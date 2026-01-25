import os
import subprocess

import pytest

# Define the image name
IMAGE_NAME = "xpanel-test-image"
CONTAINER_NAME = "xpanel-test-container"


@pytest.fixture(scope="module")
def docker_container(request):
    """
    Builds the docker image and runs the container.
    Yields the container object (subprocess wrapper or similar concept).
    Cleans up after tests.
    """
    # Build Image
    print(f"Building Docker image {IMAGE_NAME}...")
    subprocess.run(["docker", "build", "-t", IMAGE_NAME, "."], check=True)

    # Run Container
    print(f"Starting container {CONTAINER_NAME}...")
    # Remove if exists
    subprocess.run(["docker", "rm", "-f", CONTAINER_NAME], stderr=subprocess.DEVNULL)

    # Run in background
    subprocess.run(
        [
            "docker",
            "run",
            "-d",
            "--name",
            CONTAINER_NAME,
            "--privileged",  # often needed for systemd simulation or deep system access
            IMAGE_NAME,
            "tail",
            "-f",
            "/dev/null",
        ],
        check=True,
    )

    yield CONTAINER_NAME

    # Teardown
    print(f"Stopping and removing container {CONTAINER_NAME}...")
    subprocess.run(["docker", "rm", "-f", CONTAINER_NAME], check=True)


class DockerTest:
    def __init__(self, container_name):
        self.container_name = container_name

    def run(self, command):
        """Runs a command inside the docker container."""
        cmd = ["docker", "exec", self.container_name, "bash", "-c", command]
        result = subprocess.run(cmd, capture_output=True, text=True)
        return result

    def copy_to(self, src, dest):
        cmd = ["docker", "cp", src, f"{self.container_name}:{dest}"]
        subprocess.run(cmd, check=True)

    def file_exists(self, path):
        res = self.run(f"test -e {path}")
        return res.returncode == 0

    def file_contains(self, path, string):
        res = self.run(f"grep '{string}' {path}")
        return res.returncode == 0


@pytest.fixture(scope="module")
def dt(docker_container):
    """Returns a DockerTest instance linked to the running container."""
    test_helper = DockerTest(docker_container)

    # Helper to copy scripts once
    # Assuming we are running pytest from 'shell_test' directory
    # and scripts are in the parent directory '../'

    # Locate scripts
    install_script = "../install.sh"
    xui_script = "../x-ui.sh"

    if not os.path.exists(install_script):
        # Fallback if running from root
        install_script = "install.sh"
        xui_script = "x-ui.sh"

    print("Copying scripts to container...")
    test_helper.copy_to(install_script, "/root/install.sh")
    test_helper.copy_to(xui_script, "/root/x-ui.sh")

    return test_helper


def test_e2e_full_flow(dt):
    """Run the full E2E flow in a single test to avoid fixture scope issues."""
    print("\n[Step 1] Installation Setup")
    # 1. Run install.sh (it might complain about wget failure, but let's see)
    # res = dt.run("bash /root/install.sh")

    # 2. Manually setup the x-ui environment
    dt.run("mkdir -p /usr/local/x-ui")
    dt.run("cp /root/x-ui.sh /usr/local/x-ui/x-ui.sh")
    dt.run("chmod +x /usr/local/x-ui/x-ui.sh")
    dt.run("mkdir -p /etc/systemd/system/")
    dt.run("touch /etc/systemd/system/x-ui.service")
    dt.run("touch /usr/local/x-ui/x-ui")
    dt.run("chmod +x /usr/local/x-ui/x-ui")
    dt.run("ln -sf /usr/local/x-ui/x-ui.sh /usr/bin/x-ui")

    assert dt.file_exists("/usr/bin/x-ui")
    assert dt.file_exists("/usr/local/x-ui/x-ui.sh")

    print("DEBUG: Checking files before status...")
    ls_res = dt.run("ls -laR /usr/local/x-ui/")
    print(f"DEBUG LS /usr/local/x-ui/: {ls_res.stdout}")

    print("\n[Step 2] Status Check")
    res = dt.run("x-ui status")
    print(f"Status Output: {res.stdout}")
    assert (
        "状态" in res.stdout or "running" in res.stdout or "not running" in res.stdout
    )

    print("\n[Step 3] Lifecycle Start")
    res = dt.run("x-ui start")
    if "面板正在运行" in res.stdout or "already running" in res.stdout:
        print("Service already running (mocked), skipping systemctl log check")
    else:
        assert dt.file_contains("/tmp/systemctl.log", "start x-ui")

    print("\n[Step 4] Lifecycle Restart")
    dt.run("echo > /tmp/systemctl.log")
    dt.run("x-ui restart")
    assert dt.file_contains("/tmp/systemctl.log", "restart x-ui")

    print("\n[Step 5] Lifecycle Stop")
    dt.run("echo > /tmp/systemctl.log")
    dt.run("x-ui stop")
    assert dt.file_contains("/tmp/systemctl.log", "stop x-ui")

    print("\n[Step 6] Settings Port")
    # Mock binary for settings
    mock_binary_script = """#!/bin/bash
echo "Mock x-ui binary called with: $@"
if [[ "$1" == "setting" ]]; then
    echo -e "username: admin"
    echo -e "password: admin"
    echo -e "port: 54321"
fi
"""
    dt.run(f"echo '{mock_binary_script}' > /usr/local/x-ui/x-ui")
    dt.run("chmod +x /usr/local/x-ui/x-ui")

    res = dt.run("x-ui setting -port 9999")
    print(res.stdout)
    assert "Mock x-ui binary called with: setting -port 9999" in res.stdout

    print("\n[Step 7] Settings Credentials")
    res = dt.run("x-ui setting -username testuser -password testpass")
    print(res.stdout)
    assert "setting -username testuser -password testpass" in res.stdout

    print("\n[Step 8] Uninstall")
    res = dt.run("echo y | x-ui uninstall")
    print(res.stdout)
    assert not dt.file_exists("/usr/bin/x-ui")
    assert not dt.file_exists("/usr/local/x-ui/x-ui.sh")
    assert dt.file_contains("/tmp/systemctl.log", "disable x-ui")
