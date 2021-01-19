import json
import os
import time

import pytest

from tests.helpers.assertions import has_datapoint, process_is_running_as_user, run_container_cmd, service_is_running
from tests.helpers.formatting import print_dp_or_event
from tests.helpers.util import (
    DEB_DISTROS,
    REPO_DIR,
    RPM_DISTROS,
    SERVICE_NAME,
    SERVICE_OWNER,
    SERVICE_PROCESS,
    copy_file_into_container,
    run_distro_image,
    wait_for,
)

INSTALLER_PATH = REPO_DIR / "internal" / "buildscripts" / "packaging" / "installer" / "install.sh"

# Override default test parameters with the following env vars
STAGE = os.environ.get("STAGE", "release")
VERSIONS = os.environ.get("VERSIONS", "latest").split(",")

# If set, test the collector with CONFIG_PATH instead of the config in the package
CONFIG_PATH = os.environ.get("CONFIG_PATH")

# Default installer script options
ACCESS_TOKEN = "testing123"
REALM = "us0"
BALLAST = "64"
HEC_TOKEN = "testing456"

# Default hostmetrics collection interval is 1m
METRICS_TIMEOUT = 70

# Paths for installed files
COLLECTOR_CONFIG_DIR = "/etc/otel/collector"
COLLECTOR_CONFIG_PATH = f"{COLLECTOR_CONFIG_DIR}/splunk_config_linux.yaml"
COLLECTOR_ENV_PATH = f"{COLLECTOR_CONFIG_DIR}/splunk_env"
FLUENTD_CONFIG_DIR = f"{COLLECTOR_CONFIG_DIR}/fluentd"
FLUENTD_CONFIG_PATH = f"{FLUENTD_CONFIG_DIR}/fluent.conf"
FLUENTD_SOURCE_PATH = f"{FLUENTD_CONFIG_DIR}/conf.d/journald.conf"
FLUENTD_OVERRIDE_PATH = "/etc/systemd/system/td-agent.service.d/splunk-otel-collector.conf"


def get_installer_options(version):
    opts = f"-- {ACCESS_TOKEN} --realm {REALM} --ballast {BALLAST} --hec-token {HEC_TOKEN} --insecure"

    if version != "latest":
        opts += f" --collector-version {version.lstrip('v')}"

    if STAGE != "release":
        assert STAGE.lower() in ("test", "beta"), f"Unsupported stage '{STAGE}'!"
        opts += f" --{STAGE.lower()}"

    return opts


def verify_env_file(cont):
    def _has_key_value(key, value):
        return run_container_cmd(cont, f"grep '^{key}={value}$' {COLLECTOR_ENV_PATH}")

    api_url = f"https://api.{REALM}.signalfx.com"
    ingest_url = f"https://ingest.{REALM}.signalfx.com"
    trace_url = f"{ingest_url}/v2/trace"
    hec_url = f"{ingest_url}/v1/log"

    assert _has_key_value("SPLUNK_ACCESS_TOKEN", ACCESS_TOKEN)
    assert _has_key_value("SPLUNK_REALM", REALM)
    assert _has_key_value("SPLUNK_BALLAST_SIZE_MIB", BALLAST)
    assert _has_key_value("SPLUNK_API_URL", api_url)
    assert _has_key_value("SPLUNK_INGEST_URL", ingest_url)
    assert _has_key_value("SPLUNK_TRACE_URL", trace_url)
    assert _has_key_value("SPLUNK_HEC_URL", hec_url)
    assert _has_key_value("SPLUNK_HEC_TOKEN", HEC_TOKEN)


def install_fluent_plugin_systemd(cont, distro):
    cmd = "td-agent-gem list fluent-plugin-systemd --exact"
    code, output = cont.exec_run(cmd)
    assert code == 0, f"{cmd}:\n{output.decode('utf-8')}"

    if "fluent-plugin-systemd" not in output.decode("utf-8"):
        if distro in DEB_DISTROS:
            assert run_container_cmd(cont, "apt update")
            assert run_container_cmd(cont, "apt install -y build-essential")
        else:
            assert run_container_cmd(cont, "yum group install -y 'Development Tools'")

        assert run_container_cmd(cont, "td-agent-gem install fluent-plugin-systemd")


def has_log_event(backend):
    for log in backend.logs:
        if log.get("event", "").strip():
            return True
    return False


@pytest.mark.installer
@pytest.mark.parametrize(
    "distro",
    [pytest.param(distro, marks=pytest.mark.deb) for distro in DEB_DISTROS]
    + [pytest.param(distro, marks=pytest.mark.rpm) for distro in RPM_DISTROS],
)
@pytest.mark.parametrize("version", VERSIONS)
def test_installer_with_fluentd(distro, version):
    print(f"Testing installation on {distro} from {STAGE} stage ...")
    with run_distro_image(distro) as [cont, backend]:
        if CONFIG_PATH:
            run_container_cmd(cont, f"mkdir -p {COLLECTOR_CONFIG_DIR}")
            copy_file_into_container(CONFIG_PATH, cont, COLLECTOR_CONFIG_PATH)

        copy_file_into_container(INSTALLER_PATH, cont, "/test/install.sh")

        installer_cmd = "sh -x /test/install.sh " + get_installer_options(version)

        try:
            # run installer script
            assert run_container_cmd(cont, installer_cmd)
            time.sleep(5)

            # verify env file created with configured parameters
            verify_env_file(cont)

            # verify collector service
            assert service_is_running(cont, SERVICE_NAME)
            assert process_is_running_as_user(cont, SERVICE_PROCESS, SERVICE_OWNER)

            # verify datapoints
            assert wait_for(lambda: has_datapoint(backend, metric_name="system.cpu.time"), METRICS_TIMEOUT)
            assert wait_for(lambda: has_datapoint(backend, dimensions={"exporter": "signalfx"}), METRICS_TIMEOUT)

            # the td-agent service should only be running when installing
            # collector packages that have our custom fluentd config
            if run_container_cmd(cont, f"test -f {COLLECTOR_CONFIG_DIR}/fluentd/fluent.conf"):
                assert service_is_running(cont, "td-agent")

                # test the journald example if it exists
                if run_container_cmd(cont, f"test -f {FLUENTD_SOURCE_PATH}.example"):
                    run_container_cmd(cont, f"cp -f {FLUENTD_SOURCE_PATH}.example {FLUENTD_SOURCE_PATH}")

                    install_fluent_plugin_systemd(cont, distro)

                    # ensure the td-agent user has access to the journald logs in the container
                    assert run_container_cmd(cont, "usermod -a -G systemd-journal td-agent")
                    assert run_container_cmd(cont, "chgrp -R systemd-journal /run/log/journal")

                    assert run_container_cmd(cont, "systemctl restart td-agent")

                    assert wait_for(lambda: has_log_event(backend), METRICS_TIMEOUT)
            else:
                assert not service_is_running(cont, "td-agent")
        finally:
            print("\nDatapoints received:")
            for dp in backend.datapoints:
                print_dp_or_event(dp)
            print("\nEvents received:")
            for event in backend.events:
                print_dp_or_event(event)
            print("\nLogs received:")
            for log in backend.logs:
                print(json.dumps(log))
            print(f"\nDimensions set: {backend.dims}")
            run_container_cmd(cont, "journalctl -u td-agent --no-pager")
            run_container_cmd(cont, "cat /var/log/td-agent/td-agent.log")
            run_container_cmd(cont, "journalctl -u splunk-otel-collector --no-pager")


@pytest.mark.installer
@pytest.mark.parametrize(
    "distro",
    [pytest.param(distro, marks=pytest.mark.deb) for distro in DEB_DISTROS]
    + [pytest.param(distro, marks=pytest.mark.rpm) for distro in RPM_DISTROS],
)
@pytest.mark.parametrize("version", VERSIONS)
def test_installer_without_fluentd(distro, version):
    print(f"Testing installation on {distro} from {STAGE} stage ...")
    with run_distro_image(distro) as [cont, backend]:
        if CONFIG_PATH:
            run_container_cmd(cont, f"mkdir -p {COLLECTOR_CONFIG_DIR}")
            copy_file_into_container(CONFIG_PATH, cont, COLLECTOR_CONFIG_PATH)

        copy_file_into_container(INSTALLER_PATH, cont, "/test/install.sh")

        installer_cmd = "sh -x /test/install.sh " + get_installer_options(version) + " --without-fluentd"

        try:
            # run installer script
            assert run_container_cmd(cont, f"{installer_cmd}")
            time.sleep(5)

            # verify env file created with configured parameters
            verify_env_file(cont)

            # verify collector service
            assert service_is_running(cont, SERVICE_NAME)
            assert process_is_running_as_user(cont, SERVICE_PROCESS, SERVICE_OWNER)

            # fluentd should not be installed or running
            assert not run_container_cmd(cont, "test -e /etc/td-agent")
            assert not run_container_cmd(cont, "test -e /opt/td-agent")
            assert not run_container_cmd(cont, f"test -e {FLUENTD_OVERRIDE_PATH}")
            assert not service_is_running(cont, "td-agent")

            # verify datapoints
            assert wait_for(lambda: has_datapoint(backend, metric_name="system.cpu.time"), METRICS_TIMEOUT)
            assert wait_for(lambda: has_datapoint(backend, dimensions={"exporter": "signalfx"}), METRICS_TIMEOUT)
        finally:
            print("\nDatapoints received:")
            for dp in backend.datapoints:
                print_dp_or_event(dp)
            print("\nEvents received:")
            for event in backend.events:
                print_dp_or_event(event)
            print("\nLogs received:")
            for log in backend.logs:
                print(json.dumps(log))
            print(f"\nDimensions set: {backend.dims}")
            run_container_cmd(cont, "journalctl -u splunk-otel-collector --no-pager")


@pytest.mark.installer
@pytest.mark.parametrize(
    "distro",
    [pytest.param(distro, marks=pytest.mark.deb) for distro in DEB_DISTROS]
    + [pytest.param(distro, marks=pytest.mark.rpm) for distro in RPM_DISTROS],
    )
@pytest.mark.parametrize("version", VERSIONS)
def test_installer_service_owner(distro, version):
    print(f"Testing installation on {distro} from {STAGE} stage ...")
    with run_distro_image(distro) as [cont, backend]:
        if CONFIG_PATH:
            run_container_cmd(cont, f"mkdir -p {COLLECTOR_CONFIG_DIR}")
            copy_file_into_container(CONFIG_PATH, cont, COLLECTOR_CONFIG_PATH)

        copy_file_into_container(INSTALLER_PATH, cont, "/test/install.sh")

        service_owner = "testuser"
        installer_cmd = "sh -x /test/install.sh " + get_installer_options(version)
        installer_cmd += f" --service-user {service_owner} --service-group {service_owner}"

        try:
            # run installer script
            assert run_container_cmd(cont, f"{installer_cmd}")
            time.sleep(5)

            # verify env file created with configured parameters
            verify_env_file(cont)

            # verify collector service
            assert service_is_running(cont, SERVICE_NAME)
            assert process_is_running_as_user(cont, SERVICE_PROCESS, service_owner)

            # verify datapoints
            assert wait_for(lambda: has_datapoint(backend, metric_name="system.cpu.time"), METRICS_TIMEOUT)
            assert wait_for(lambda: has_datapoint(backend, dimensions={"exporter": "signalfx"}), METRICS_TIMEOUT)
        finally:
            print("\nDatapoints received:")
            for dp in backend.datapoints:
                print_dp_or_event(dp)
            print("\nEvents received:")
            for event in backend.events:
                print_dp_or_event(event)
            print("\nLogs received:")
            for log in backend.logs:
                print(json.dumps(log))
            print(f"\nDimensions set: {backend.dims}")
            run_container_cmd(cont, "journalctl -u splunk-otel-collector --no-pager")
