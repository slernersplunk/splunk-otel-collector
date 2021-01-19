import asyncio
import glob
import io
import logging
import os
import random
import re
import socket
import subprocess
import tarfile
import threading
import time
from contextlib import contextmanager
from io import BytesIO
from pathlib import Path
from typing import Any, Dict, List, TypeVar, cast

import docker
import netifaces as ni
import requests

from tests.helpers import fake_backend

TESTS_DIR = Path(__file__).parent.parent.resolve()
DOCKERFILES_DIR = TESTS_DIR / "images"
REPO_DIR = TESTS_DIR.parent.parent.parent.parent.resolve()
DEB_DISTROS = [df.split(".")[-1] for df in glob.glob(str(DOCKERFILES_DIR / "deb" / "Dockerfile.*"))]
RPM_DISTROS = [df.split(".")[-1] for df in glob.glob(str(DOCKERFILES_DIR / "rpm" / "Dockerfile.*"))]
SERVICE_NAME = "splunk-otel-collector"
SERVICE_OWNER = "splunk-otel-collector"
SERVICE_PROCESS = "otelcol"
DEFAULT_TIMEOUT = int(os.environ.get("DEFAULT_TIMEOUT", 30))
DOCKER_API_VERSION = "1.34"


T = TypeVar("T")


def retry_on_ebadf(func: T) -> T:
    max_tries = 10

    def wrap(*args, **kwargs):
        tries = 0
        while True:
            try:
                return func(*args, **kwargs)
            except requests.exceptions.ConnectionError as e:
                if "bad file descriptor" in str(e).lower():
                    tries += 1
                    if tries >= max_tries:
                        raise

                    logging.error("Retrying ConnectionError EBADF")
                    continue
                raise
            except OSError as e:
                if e.errno == 9:
                    tries += 1
                    if tries >= max_tries:
                        raise

                    logging.error("Retrying OSError EBADF")
                    continue
                raise

    return cast(T, wrap)


def get_docker_client():
    return docker.from_env(version=DOCKER_API_VERSION)


def has_docker_image(client, name):
    return name in [t for image in client.images.list() for t in image.tags]


def assert_wait_for(test, timeout_seconds=DEFAULT_TIMEOUT, interval_seconds=0.2, on_fail=None):
    """
    Runs `wait_for` but raises an assertion if it fails, optionally calling
    `on_fail` before raising an AssertionError
    """
    if not wait_for(test, timeout_seconds, interval_seconds):
        if on_fail:
            on_fail()

        raise AssertionError("test '%s' still failng after %d seconds" % (test, timeout_seconds))


def wait_for(test, timeout_seconds=DEFAULT_TIMEOUT, interval_seconds=0.2):
    """
    Repeatedly calls the test function for timeout_seconds until either test
    returns a truthy value, at which point the function returns True -- or the
    timeout is exceeded, at which point it will return False.
    """
    start = time.time()
    while True:
        if test():
            return True
        if time.time() - start > timeout_seconds:
            return False
        time.sleep(interval_seconds)


def wait_for_value(func, timeout_seconds=DEFAULT_TIMEOUT, interval_seconds=0.2):
    """
    Waits for func to return a non-None value and returns that value.  If the
    func is still returning None after the timeout, returns None to the caller.
    """
    start = time.time()
    while True:
        val = func()
        if val is not None:
            return val
        if time.time() - start > timeout_seconds:
            return None
        time.sleep(interval_seconds)


def wait_for_assertion(test, timeout_seconds=DEFAULT_TIMEOUT, interval_seconds=0.2):
    """
    Waits for the given `test` function passed in to not raise an
    AssertionError.  It is is still raising such an error after the
    timeout_seconds, that exception will be raised by this function itself.
    """
    e = None

    def wrap():
        nonlocal e
        try:
            test()
        except AssertionError as err:
            e = err
            return False
        return True

    if not wait_for(wrap, timeout_seconds, interval_seconds):
        raise e  # pylint: disable=raising-bad-type


def ensure_always(test, timeout_seconds=DEFAULT_TIMEOUT, interval_seconds=0.2):
    """
    Repeatedly calls the given test.  If it ever returns false before the timeout
    given is completed, returns False, otherwise True.
    """
    start = time.time()
    while True:
        if not test():
            return False
        if time.time() - start > timeout_seconds:
            return True
        time.sleep(interval_seconds)


def ensure_never(test, timeout_seconds=DEFAULT_TIMEOUT):
    """
    Repeatedly calls the given test.  If it ever returns true before the timeout
    given is completed, returns False, otherwise True.
    """
    start = time.time()
    while True:
        if test():
            return False
        if time.time() - start > timeout_seconds:
            return True
        time.sleep(0.2)


def print_lines(msg):
    """
    Print each line separately to make it easier to read in pytest output
    """
    for line in msg.splitlines():
        print(line)


def container_ip(container):
    container.reload()
    return container.attrs["NetworkSettings"]["IPAddress"]


def container_hostname(container):
    container.reload()
    return container.attrs["Config"]["Hostname"]


LOCALHOST_COUNTER = 0

# Ensure a unique internal status server host address.  This supports up to
# 255 concurrent agents on the same pytest worker process, and up to 255
# pytest workers, which should be plenty
def get_unique_localhost():
    worker = int(re.sub(r"\D", "", os.environ.get("PYTEST_XDIST_WORKER", "0")))

    global LOCALHOST_COUNTER  # pylint:disable=global-statement
    LOCALHOST_COUNTER += 1

    return "127.%d.%d.0" % (worker, LOCALHOST_COUNTER % 255)


@contextmanager
def run_subprocess(command: List[str], env: Dict[Any, Any] = None, **kwargs):
    # subprocess on Windows has a bug where it doesn't like Path.
    proc = retry_on_ebadf(
        lambda: subprocess.Popen(
            [str(c) for c in command], env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, **kwargs
        )
    )()

    get_output = pull_from_reader_in_background(proc.stdout)

    try:
        yield [get_output, proc.pid]
    finally:
        proc.terminate()
        proc.wait(15)


@contextmanager
def run_container(image_name, wait_for_ip=True, print_logs=True, **kwargs):
    files = kwargs.pop("files", [])
    client = get_docker_client()

    if not image_name.startswith("sha256"):
        try:
            client.images.get(image_name)
        except docker.errors.APIError:
            client.images.pull(image_name)
    container = retry_on_ebadf(
        lambda: retry(lambda: client.containers.create(image_name, **kwargs), docker.errors.DockerException)
    )()

    for src, dst in files:
        copy_file_into_container(src, container, dst)

    try:
        container.start()

        def has_ip_addr():
            container.reload()
            return container.attrs["NetworkSettings"]["IPAddress"]

        if wait_for_ip:
            wait_for(has_ip_addr, timeout_seconds=5)
            yield container
    finally:
        try:
            if print_logs:
                print_lines(
                    "Container %s/%s logs:\n%s" % (image_name, container.name, container.logs().decode("utf-8"))
                )
            container.remove(force=True, v=True)
        except docker.errors.NotFound:
            pass


@contextmanager
def run_service(service_name, buildargs=None, print_logs=True, path=None, dockerfile="./Dockerfile", **kwargs):
    if buildargs is None:
        buildargs = {}
    if path is None:
        path = os.path.join(TEST_SERVICES_DIR, service_name)

    client = get_docker_client()
    image, _ = retry_on_ebadf(
        lambda: retry(
            lambda: client.images.build(
                path=str(path), dockerfile=dockerfile, rm=True, forcerm=True, buildargs=buildargs
            ),
            docker.errors.BuildError,
        )
    )()
    with run_container(image.id, print_logs=print_logs, **kwargs) as cont:
        yield cont


def get_host_ip():
    gws = ni.gateways()
    interface = gws["default"][ni.AF_INET][1]
    return ni.ifaddresses(interface)[ni.AF_INET][0]["addr"]


def send_udp_message(host, port, msg):
    """
    Send a datagram to the given host/port
    """
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)  # Internet  # UDP
    sock.sendto(msg.encode("utf-8"), (host, port))


def retry(function, exception, max_attempts=5, interval_seconds=5):
    """
    Retry function up to max_attempts if exception is caught
    """
    for attempt in range(max_attempts):
        try:
            return function()
        except exception as e:
            assert attempt < (max_attempts - 1), "%s failed after %d attempts!\n%s" % (function, max_attempts, str(e))
        time.sleep(interval_seconds)


def pull_from_reader_in_background(reader):
    output = io.BytesIO()

    def pull_output():
        while True:
            # If any output is waiting, grab it.
            try:
                byt = reader.read(1)
            except OSError:
                return
            if not byt:
                return
            if isinstance(byt, str):
                byt = byt.encode("utf-8")
            output.write(byt)

    threading.Thread(target=pull_output).start()

    def get_output():
        return output.getvalue().decode("utf-8")

    return get_output


def random_hex(bits=64):
    """Return random hex number as a string with the given number of bits (default 64)"""
    return hex(random.getrandbits(bits))[2:]


def copy_file_content_into_container(content, container, target_path):
    copy_file_object_into_container(
        BytesIO(content.encode("utf-8")), container, target_path, size=len(content.encode("utf-8"))
    )


# This is more convoluted that it should be but seems to be the simplest way in
# the face of docker-in-docker environments where volume bind mounting is hard.
@retry_on_ebadf
def copy_file_object_into_container(fd, container, target_path, size=None):
    tario = BytesIO()
    tar = tarfile.TarFile(fileobj=tario, mode="w")

    info = tarfile.TarInfo(name=target_path)
    if size is None:
        size = os.fstat(fd.fileno()).st_size
    info.size = size

    tar.addfile(info, fd)

    tar.close()

    container.put_archive("/", tario.getvalue())
    # Apparently when the above `put_archive` call returns, the file isn't
    # necessarily fully written in the container, so wait a bit to ensure it
    # is.
    time.sleep(2)


def copy_file_into_container(path, container, target_path):
    with open(path, "rb") as fd:
        copy_file_object_into_container(fd, container, target_path)


def path_exists_in_container(container, path):
    code, _ = container.exec_run("test -e %s" % path)
    return code == 0


def get_container_file_content(container, path):
    assert path_exists_in_container(container, path), "File %s does not exist!" % path
    return container.exec_run("cat %s" % path)[1].decode("utf-8")


def get_stripped_container_id(container_id):
    return container_id.replace("docker://", "").replace("cri-o://", "")


@contextmanager
def run_simple_sanic_app(app):
    app_sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    app_sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    app_sock.bind(("127.0.0.1", 0))

    port = app_sock.getsockname()[1]

    loop = asyncio.new_event_loop()

    async def start_server():
        server = app.create_server(sock=app_sock, access_log=False)
        loop.create_task(server)

    loop.create_task(start_server())
    threading.Thread(target=loop.run_forever).start()

    try:
        yield f"http://127.0.0.1:{port}"
    finally:
        app_sock.close()
        loop.stop()


def build_base_image(name, path=DOCKERFILES_DIR, dockerfile=None, buildargs=None):
    client = get_docker_client()
    dockerfile = dockerfile or Path(path) / f"Dockerfile.{name}"
    image, _ = client.images.build(
        path=str(path), dockerfile=str(dockerfile), pull=True, rm=True, forcerm=True, buildargs=buildargs
    )

    return image.id


# Run an HTTPS proxy inside the container with socat so that our fake backend
# doesn't have to worry about HTTPS.  The cert file must be trusted by the
# container running the agent.
# This is pretty hacky but docker makes it hard to communicate from a container
# back to the host machine (and we don't want to use the host network stack in
# the container due to init systems).  The idea is to bind mount a shared
# folder from the test host to the container that two socat instances use to
# communicate using a file to make the bytes flow between the HTTPS proxy and
# the fake backend.
@contextmanager
def socat_https_proxy(container, target_host, target_port, source_host, bind_addr):
    cert = "/%s.cert" % source_host
    key = "/%s.key" % source_host

    socat_bin = DOCKERFILES_DIR / "socat"
    stopped = False
    socket_path = "/tmp/scratch/%s-%s" % (source_host, container.id[:12])

    # Keep the socat instance in the container running across container restarts
    def keep_running_in_container(cont, sock):
        while not stopped:
            try:
                _, output = cont.exec_run(
                    [
                        "socat",
                        "-v",
                        "OPENSSL-LISTEN:443,cert=%s,key=%s,verify=0,bind=%s,fork" % (cert, key, bind_addr),
                        "UNIX-CONNECT:%s" % sock,
                    ]
                )
                print(output.decode("utf-8"))
            except docker.errors.APIError:
                print("socat died, restarting...")
                time.sleep(0.1)

    threading.Thread(target=keep_running_in_container, args=(container, socket_path)).start()

    proc = retry_on_ebadf(
        lambda: subprocess.Popen(
            [socat_bin, "-v", "UNIX-LISTEN:%s,fork" % socket_path, "TCP4:%s:%d" % (target_host, target_port)],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            close_fds=False,
        )
    )()

    get_local_out = pull_from_reader_in_background(proc.stdout)

    try:
        yield
    finally:
        stopped = True
        # The socat instance in the container will die with the container
        proc.kill()
        print(get_local_out())


@contextmanager
def run_init_system_image(
    base_image,
    with_socat=True,
    path=DOCKERFILES_DIR,
    dockerfile=None,
    ingest_host="ingest.us0.signalfx.com",  # Whatever value is used here needs a self-signed cert in ./images/certs/
    api_host="api.us0.signalfx.com",  # Whatever value is used here needs a self-signed cert in ./images/certs/
    command=None,
    buildargs=None,
):  # pylint: disable=too-many-arguments
    image_id = retry(lambda: build_base_image(base_image, path, dockerfile, buildargs), docker.errors.BuildError)
    print("Image ID: %s" % image_id)
    if with_socat:
        backend_ip = "127.0.0.1"
    else:
        backend_ip = get_host_ip()
    with fake_backend.start(ip_addr=backend_ip) as backend:
        container_options = {
            # Init systems running in the container want permissions
            "privileged": True,
            "volumes": {
                "/sys/fs/cgroup": {"bind": "/sys/fs/cgroup", "mode": "ro"},
                "/tmp/scratch": {"bind": "/tmp/scratch", "mode": "rw"},
            },
            "extra_hosts": {
                # Socat will be running on localhost to forward requests to
                # these hosts to the fake backend
                ingest_host: backend.ingest_host,
                api_host: backend.api_host,
            },
        }

        if command:
            container_options["command"] = command

        with run_container(image_id, wait_for_ip=True, **container_options) as cont:
            # Workaround for https://bugzilla.redhat.com/show_bug.cgi?id=1769831 which
            # causes yum/dnf to exit with error code 141 when importing GPG keys.
            cont.exec_run("mkdir -p /run/user/0")

            if with_socat:
                # Proxy the backend calls through a fake HTTPS endpoint so that we
                # don't have to change the default configuration default by the
                # package.  The base_image used should trust the self-signed certs
                # default in the images dir so that the agent doesn't throw TLS
                # verification errors.
                with socat_https_proxy(
                    cont, backend.ingest_host, backend.ingest_port, ingest_host, "127.0.0.1"
                ), socat_https_proxy(cont, backend.api_host, backend.api_port, api_host, "127.0.0.2"):
                    yield [cont, backend]
            else:
                yield [cont, backend]


@contextmanager
def run_distro_image(distro):
    assert distro in DEB_DISTROS + RPM_DISTROS, f"'{distro}' distro not supported!"

    if distro in DEB_DISTROS:
        dockerfile = DOCKERFILES_DIR / "deb" / f"Dockerfile.{distro}"
    else:
        dockerfile = DOCKERFILES_DIR / "rpm" / f"Dockerfile.{distro}"

    with run_init_system_image(distro, with_socat=True, dockerfile=dockerfile) as [cont, backend]:
        yield [cont, backend]
