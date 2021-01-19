import glob
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
    copy_file_content_into_container,
    run_distro_image,
    wait_for,
)

PKG_NAME = "splunk-otel-collector"
PKG_DIR = REPO_DIR / "dist"
CONFIG_PATH = "/etc/otel/collector/splunk_config_linux.yaml"
ENV_PATH = "/etc/otel/collector/splunk_env"

# Default hostmetrics collection interval is 1m
METRICS_TIMEOUT = 70


def get_package(distro, name, path):
    if distro in DEB_DISTROS:
        pkg_paths = glob.glob(str(path / f"{name}*amd64.deb"))
    else:
        pkg_paths = glob.glob(str(path / f"{name}*x86_64.rpm"))

    if pkg_paths:
        return sorted(pkg_paths)[-1]
    else:
        return None


def create_env_file(cont, realm="us0"):
    api_url = f"https://api.{realm}.signalfx.com"
    ingest_url = f"https://ingest.{realm}.signalfx.com"

    content = f"""
SPLUNK_ACCESS_TOKEN=testing123
SPLUNK_REALM={realm}
SPLUNK_BALLAST_SIZE_MIB=64
SPLUNK_API_URL={api_url}
SPLUNK_INGEST_URL={ingest_url}
SPLUNK_TRACE_URL={ingest_url}/v2/trace
SPLUNK_HEC_URL={ingest_url}/v1/log
SPLUNK_HEC_TOKEN=testing456
"""
    copy_file_content_into_container(content, cont, ENV_PATH)


@pytest.mark.parametrize(
    "distro",
    [pytest.param(distro, marks=pytest.mark.deb) for distro in DEB_DISTROS]
    + [pytest.param(distro, marks=pytest.mark.rpm) for distro in RPM_DISTROS],
)
def test_collector_package_install(distro):
    pkg_path = get_package(distro, PKG_NAME, PKG_DIR)
    assert pkg_path, f"{PKG_NAME} package not found in {PKG_DIR}"

    pkg_base = os.path.basename(pkg_path)

    with run_distro_image(distro) as [cont, backend]:
        copy_file_into_container(pkg_path, cont, f"/test/{pkg_base}")

        # install package
        if distro in DEB_DISTROS:
            assert run_container_cmd(cont, f"dpkg -i /test/{pkg_base}")
        else:
            assert run_container_cmd(cont, f"rpm -i /test/{pkg_base}")

        try:
            # verify service is not running after install without env file
            time.sleep(5)
            assert not service_is_running(cont, SERVICE_NAME)

            # verify service restart with default config and custom env file
            create_env_file(cont)
            assert run_container_cmd(cont, f"systemctl restart {SERVICE_NAME}")
            time.sleep(5)
            assert service_is_running(cont, SERVICE_NAME)
            assert process_is_running_as_user(cont, SERVICE_PROCESS, SERVICE_OWNER)

            assert wait_for(lambda: has_datapoint(backend, metric_name="system.cpu.time"), METRICS_TIMEOUT)
            assert wait_for(lambda: has_datapoint(backend, dimensions={"exporter": "signalfx"}), METRICS_TIMEOUT)

            # verify service stop
            assert run_container_cmd(cont, f"systemctl stop {SERVICE_NAME}")
            time.sleep(5)
            assert not service_is_running(cont, SERVICE_NAME)
        finally:
            print("\nDatapoints received:")
            for dp in backend.datapoints:
                print_dp_or_event(dp)
            print("\nEvents received:")
            for event in backend.events:
                print_dp_or_event(event)
            print(f"\nDimensions set: {backend.dims}")
            run_container_cmd(cont, f"journalctl -u {SERVICE_NAME} --no-pager")

        # restart service and verify uninstall
        assert run_container_cmd(cont, f"systemctl restart {SERVICE_NAME}")

        time.sleep(5)

        if distro in DEB_DISTROS:
            assert run_container_cmd(cont, f"dpkg -P {PKG_NAME}")
        else:
            assert run_container_cmd(cont, f"rpm -e {PKG_NAME}")

        time.sleep(5)
        assert not service_is_running(cont, SERVICE_NAME)

        # verify env file is preserved
        assert run_container_cmd(cont, f"test -f {ENV_PATH}")
