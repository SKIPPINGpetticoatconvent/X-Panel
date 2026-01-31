#!/bin/bash
set -e

# Base Config
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST_DIR="${PROJECT_ROOT}/tests/e2e"
DOCKERFILE="${TEST_DIR}/docker/Dockerfile.ubuntu22"
CONTAINER_NAME="xpanel-e2e-test"
IMAGE_NAME="xpanel-e2e-image"

# Default Mode
MODE="local"
XPANEL_VERSION=$(tr -d '\n' <"${PROJECT_ROOT}/config/version")

# Parse Args
# Parse Args
TEST_SCRIPT="verify_in_container.sh"
while [[ $# -gt 0 ]]; do
  case $1 in
  --mode)
    MODE="$2"
    shift
    ;;
  --test)
    TEST_CASE="$2"
    case $TEST_CASE in
    ip_cert)
      TEST_SCRIPT="verify_ip_cert.sh"
      ;;
    domain_cert)
      TEST_SCRIPT="verify_domain_cert.sh"
      ;;
    failover)
      TEST_SCRIPT="verify_ssl_fallback.sh"
      ;;
    install)
      TEST_SCRIPT="verify_in_container.sh"
      ;;
    *)
      echo "Unknown test case: $TEST_CASE"
      exit 1
      ;;
    esac
    shift
    ;;
  *)
    echo "Unknown parameter passed: $1"
    exit 1
    ;;
  esac
  shift
done

echo "=========================================="
echo "Starting X-Panel E2E Test"
echo "Mode: ${MODE}"
echo "Test: ${TEST_SCRIPT}"
echo "Version: ${XPANEL_VERSION}"
echo "=========================================="

cleanup() {
  echo ">> Cleaning up..."
  docker rm -f ${CONTAINER_NAME} >/dev/null 2>&1 || true
  rm -f "${PROJECT_ROOT}/x-ui-linux-amd64.tar.gz"
}
# Trap exit to ensure cleanup (optional, maybe we want to inspect failed container)
# trap cleanup EXIT

cleanup

# 1. Build Binary (Local Mode Only)
if [ "${MODE}" == "local" ]; then
  echo ">> [Host] Building x-ui binary..."
  cd "${PROJECT_ROOT}"

  # Run build command equivalent to what build.sh does, or just simple go build
  # Assuming standard build:
  GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X 'github.com/SKIPPINGpetticoatconvent/X-Panel/config.version=${XPANEL_VERSION}'" -o x-ui main.go

  # Compress as expected by install script structure
  # install.sh unzip expects: x-ui-linux-amd64.tar.gz -> x-ui-linux-amd64/x-ui ...
  # Wait, looking at install.sh:
  # tar zxvf "x-ui-linux-$(arch).tar.gz"
  # cd x-ui
  # mv bin/xray-linux...

  # We need to mimic the tar structure:
  # x-ui/
  #   x-ui (binary)
  #   bin/
  #     xray-linux-amd64 (we need to download or mock this too? install.sh expects it)
  #     geosite.dat
  #     geoip.dat

  # This is getting complex. The install.sh expects a full release package.
  # To properly simulate "local build", we should probably use the project's build.sh if available, OR mock the tarball structure.

  echo ">> [Host] Creating release tarball structure..."
  mkdir -p release_temp/x-ui/bin
  cp x-ui release_temp/x-ui/

  # Note: We need xray binary and geo files for x-ui to start successfully?
  # install.sh moves them.
  # For E2E speed, maybe we can skip xray or use a dummy file if x-ui doesn't check strict hash at startup?
  # x-ui usually orchestrates xray. It might fail if xray binary is missing.
  # Let's download a real xray binary or use the one on host if present.

  # Check if we have assets
  if [ -f "${PROJECT_ROOT}/bin/xray-linux-amd64" ]; then
    cp "${PROJECT_ROOT}/bin/xray-linux-amd64" release_temp/x-ui/bin/
  else
    echo ">> [Host] Creating mock xray binary..."
    # Create a dummy xray script
    cat <<EOF >release_temp/x-ui/bin/xray-linux-amd64
#!/bin/sh
if [ "\$1" = "-version" ] || [ "\$1" = "version" ]; then
    echo "Xray 1.8.4 (Mock) Custom"
else
    # Keep running to simulate daemon if needed, or just exit 0?
    # x-ui usually starts it as a subprocess.
    # If it exits immediately, x-ui might restart it loop.
    echo "Starting Mock Xray..."
    sleep 3600
fi
EOF
    chmod +x release_temp/x-ui/bin/xray-linux-amd64
  fi

  # Geo files
  touch release_temp/x-ui/bin/geosite.dat
  touch release_temp/x-ui/bin/geoip.dat

  # X-Panel install.sh expects x-ui, x-ui.sh, etc.
  # install.sh logic:
  # tar zxvf ...
  # cd x-ui
  # chmod +x x-ui
  # chmod +x x-ui.sh

  # We need x-ui.sh in the tar?
  # Check install.sh line 475: wget -O /usr/bin/x-ui-temp .../x-ui.sh
  # It downloads x-ui.sh separately! But also chmod +x x-ui.sh inside extracted folder?
  # Line 491: chmod +x x-ui.sh
  # So the tarball should contain x-ui.sh?
  # Actually, install.sh doesn't fail if files are missing from tar, but we should be safe.
  cp "${PROJECT_ROOT}/x-ui.sh" release_temp/x-ui/ || touch release_temp/x-ui/x-ui.sh
  cp "${PROJECT_ROOT}/x-ui.service" release_temp/x-ui/ || echo "Warning: x-ui.service not found"

  # Tar it
  cd release_temp
  tar -czf "${PROJECT_ROOT}/x-ui-linux-amd64.tar.gz" x-ui
  cd ..
  rm -rf release_temp
  rm -f x-ui # binary
fi

# 2. Build Docker Image
echo ">> [Host] Building Docker image..."
docker build -t ${IMAGE_NAME} -f "${DOCKERFILE}" .

# 3. Run Container
echo ">> [Host] Starting container..."
# Privileged is required for systemd
docker run -d --privileged --cgroupns=host -v /sys/fs/cgroup:/sys/fs/cgroup:rw --name ${CONTAINER_NAME} ${IMAGE_NAME}

# 4. Inject Files
echo ">> [Host] Injecting files..."
docker cp "${PROJECT_ROOT}/install.sh" "${CONTAINER_NAME}:/root/install.sh"
docker cp "${TEST_DIR}/${TEST_SCRIPT}" "${CONTAINER_NAME}:/root/${TEST_SCRIPT}"
# Inject assets if available
if [ -d "${TEST_DIR}/assets" ]; then
  docker cp "${TEST_DIR}/assets" "${CONTAINER_NAME}:/root/assets"
fi

# We might also need other scripts
docker cp "${TEST_DIR}/verify_in_container.sh" "${CONTAINER_NAME}:/root/verify_in_container.sh"

if [ "${MODE}" == "local" ]; then
  docker cp "${PROJECT_ROOT}/x-ui-linux-amd64.tar.gz" "${CONTAINER_NAME}:/root/x-ui-linux-amd64.tar.gz"
fi

# 5. Execute Test
echo ">> [Host] Executing test inside container..."
docker exec ${CONTAINER_NAME} chmod +x /root/${TEST_SCRIPT}
docker exec ${CONTAINER_NAME} /bin/bash /root/${TEST_SCRIPT} "${MODE}" "${XPANEL_VERSION}"
EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
  echo "✅ E2E Test Passed!"
else
  echo "❌ E2E Test Failed!"
fi

# Cleanup
# cleanup

exit $EXIT_CODE
