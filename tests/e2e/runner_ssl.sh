#!/bin/bash
set -e

# Base Config
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST_DIR="${PROJECT_ROOT}/tests/e2e"
DOCKERFILE="${TEST_DIR}/docker/Dockerfile.ubuntu22"
CONTAINER_NAME="xpanel-ssl-test"
IMAGE_NAME="xpanel-e2e-image"

# Default Mode
MODE="local"
XPANEL_VERSION=$(cat "${PROJECT_ROOT}/config/version" | tr -d '\n')

# Parse Args
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --mode) MODE="$2"; shift ;;
        *) echo "Unknown parameter passed: $1"; exit 1 ;;
    esac
    shift
done

echo "=========================================="
echo "Starting X-Panel SSL Fallback Test"
echo "Mode: ${MODE}"
echo "Version: ${XPANEL_VERSION}"
echo "=========================================="

cleanup() {
    echo ">> Cleaning up..."
    docker rm -f ${CONTAINER_NAME} >/dev/null 2>&1 || true
    rm -f "${PROJECT_ROOT}/x-ui-linux-amd64.tar.gz"
}
cleanup

# 1. Build Binary (Local Mode Only)
if [ "${MODE}" == "local" ]; then
    echo ">> [Host] Building x-ui binary..."
    cd "${PROJECT_ROOT}"
    
    # Simple Build
    GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X 'github.com/SKIPPINGpetticoatconvent/X-Panel/config.version=${XPANEL_VERSION}'" -o x-ui main.go
    
    echo ">> [Host] Creating release tarball structure..."
    mkdir -p release_temp/x-ui/bin
    cp x-ui release_temp/x-ui/
    
    # Mock Xray
    if [ -f "${PROJECT_ROOT}/bin/xray-linux-amd64" ]; then
        cp "${PROJECT_ROOT}/bin/xray-linux-amd64" release_temp/x-ui/bin/
    else 
         echo ">> [Host] Creating mock xray binary..."
         cat <<EOF > release_temp/x-ui/bin/xray-linux-amd64
#!/bin/sh
if [ "\$1" = "-version" ] || [ "\$1" = "version" ]; then
    echo "Xray 1.8.4 (Mock) Custom"
else
    # Keep alive
    sleep 3600
fi
EOF
         chmod +x release_temp/x-ui/bin/xray-linux-amd64
    fi

    # Geo files
    touch release_temp/x-ui/bin/geosite.dat
    touch release_temp/x-ui/bin/geoip.dat

    # Helper scripts
    cp "${PROJECT_ROOT}/x-ui.sh" release_temp/x-ui/ || touch release_temp/x-ui/x-ui.sh
    cp "${PROJECT_ROOT}/x-ui.service" release_temp/x-ui/ || echo "Warning: x-ui.service not found"
    
    # Tar it
    cd release_temp
    tar -czf "${PROJECT_ROOT}/x-ui-linux-amd64.tar.gz" x-ui
    cd ..
    rm -rf release_temp
    rm -f x-ui # binary
fi

# 2. Build Docker Image (Reuse existing image name)
echo ">> [Host] Ensuring Docker image exists..."
docker build -t ${IMAGE_NAME} -f ${DOCKERFILE} . >/dev/null

# 3. Run Container
echo ">> [Host] Starting container..."
docker run -d --privileged --cgroupns=host -v /sys/fs/cgroup:/sys/fs/cgroup:rw --name ${CONTAINER_NAME} ${IMAGE_NAME}

# 4. Inject Files
echo ">> [Host] Injecting files..."
docker cp "${PROJECT_ROOT}/install.sh" "${CONTAINER_NAME}:/root/install.sh"
docker cp "${TEST_DIR}/verify_ssl_fallback.sh" "${CONTAINER_NAME}:/root/verify_ssl_fallback.sh"

if [ "${MODE}" == "local" ]; then
    docker cp "${PROJECT_ROOT}/x-ui-linux-amd64.tar.gz" "${CONTAINER_NAME}:/root/x-ui-linux-amd64.tar.gz"
fi

# 5. Execute Test settings permission first
docker exec ${CONTAINER_NAME} chmod +x /root/verify_ssl_fallback.sh

echo ">> [Host] Executing SSL verification..."
docker exec ${CONTAINER_NAME} /bin/bash /root/verify_ssl_fallback.sh "${MODE}" "${XPANEL_VERSION}"
EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ SSL Fallback Test Passed!"
else
    echo "❌ SSL Fallback Test Failed!"
fi

# Cleanup
# cleanup

exit $EXIT_CODE
