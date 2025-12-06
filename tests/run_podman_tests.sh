#!/bin/bash
# Podman防火墙测试运行脚本
# 用于在隔离的容器环境中测试防火墙自动安装功能

set -e

# 脚本配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DOCKER_DIR="$SCRIPT_DIR/docker"
TEST_RESULTS_DIR="$SCRIPT_DIR/test-results"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."
    
    if ! command -v podman &> /dev/null; then
        log_error "Podman未安装，请先安装Podman"
        exit 1
    fi
    
    if ! command -v go &> /dev/null; then
        log_error "Go未安装，请先安装Go"
        exit 1
    fi
    
    if [ ! -f "$PROJECT_ROOT/go.mod" ]; then
        log_error "项目根目录未找到go.mod文件，请确保在正确的项目目录中运行"
        exit 1
    fi
    
    log_success "依赖检查完成"
}

# 创建测试结果目录
create_test_results_dir() {
    mkdir -p "$TEST_RESULTS_DIR"
    log_info "测试结果目录: $TEST_RESULTS_DIR"
}

# 构建Docker镜像
build_image() {
    local distro=$1
    local dockerfile=$2
    local image_name="x-ui-test-$distro"
    
    log_info "构建 $distro 测试镜像..."
    
    cd "$PROJECT_ROOT"
    
    if podman build -f "$dockerfile" -t "$image_name" .; then
        log_success "$distro 镜像构建完成: $image_name"
        return 0
    else
        log_error "$distro 镜像构建失败"
        return 1
    fi
}

# 运行容器测试
run_container_test() {
    local distro=$1
    local image_name="x-ui-test-$distro"
    local test_name="$2"
    local privileged=$3
    local container_name="x-ui-test-$distro-$(date +%s)"
    
    log_info "运行 $distro 容器测试: $test_name"
    
    # 设置容器运行参数
    local run_args=(
        --rm
        --name "$container_name"
        -v "$PROJECT_ROOT:/app:rw"
        -v "$TEST_RESULTS_DIR:/app/test-results:rw"
        -e TEST_OS_TYPE="$distro"
        -e SKIP_REAL_INSTALL_TESTS=true
    )
    
    # 如果需要特权模式
    if [ "$privileged" = "true" ]; then
        run_args+=(--privileged)
        log_warning "使用特权模式运行容器"
    fi
    
    # 运行测试
    if podman run "${run_args[@]}" "$image_name" "$test_name" > "$TEST_RESULTS_DIR/${distro}_${test_name}_output.log" 2>&1; then
        log_success "$distro 测试 '$test_name' 完成"
        return 0
    else
        log_error "$distro 测试 '$test_name' 失败，查看日志: $TEST_RESULTS_DIR/${distro}_${test_name}_output.log"
        return 1
    fi
}

# 运行所有测试
run_all_tests() {
    local privileged=${1:-false}
    local test_name=${2:-""}
    
    log_info "开始运行所有测试 (特权模式: $privileged)"
    
    local failed_tests=()
    
    # 测试矩阵
    local test_matrix=(
        "ubuntu:tests/docker/Dockerfile.ubuntu"
        "centos:tests/docker/Dockerfile.centos"
    )
    
    for entry in "${test_matrix[@]}"; do
        IFS=':' read -r distro dockerfile <<< "$entry"
        
        # 构建镜像
        if ! build_image "$distro" "$dockerfile"; then
            failed_tests+=("$distro-build")
            continue
        fi
        
        # 运行测试
        if [ -n "$test_name" ]; then
            # 运行特定测试
            if ! run_container_test "$distro" "$test_name" "$privileged"; then
                failed_tests+=("$distro-$test_name")
            fi
        else
            # 运行所有防火墙测试
            if ! run_container_test "$distro" "" "$privileged"; then
                failed_tests+=("$distro-all")
            fi
        fi
    done
    
    # 显示测试结果摘要
    echo
    log_info "=== 测试结果摘要 ==="
    if [ ${#failed_tests[@]} -eq 0 ]; then
        log_success "所有测试通过!"
    else
        log_error "失败的测试:"
        for failed_test in "${failed_tests[@]}"; do
            echo "  - $failed_test"
        done
    fi
}

# 清理函数
cleanup() {
    log_info "清理容器和镜像..."
    
    # 停止并删除所有测试容器
    podman ps -a --filter "name=x-ui-test" --format "{{.Names}}" | xargs -r podman rm -f
    
    # 删除测试镜像
    podman images --filter "reference=x-ui-test*" --format "{{.Repository}}:{{.Tag}}" | xargs -r podman rmi -f
    
    log_success "清理完成"
}

# 显示帮助信息
show_help() {
    echo "用法: $0 [选项] [测试名称]"
    echo
    echo "选项:"
    echo "  -h, --help              显示此帮助信息"
    echo "  -p, --privileged        使用特权模式运行容器 (用于测试systemd服务)"
    echo "  -c, --cleanup           清理所有测试容器和镜像"
    echo "  -l, --list              列出可用的测试"
    echo "  --ubuntu-only           只运行Ubuntu测试"
    echo "  --centos-only           只运行CentOS测试"
    echo
    echo "示例:"
    echo "  $0                                    # 运行所有测试"
    echo "  $0 -p                                 # 使用特权模式运行所有测试"
    echo "  $0 TestNewFirewallService            # 运行特定测试"
    echo "  $0 -p TestPackageManagerDetection    # 使用特权模式运行特定测试"
    echo "  $0 -c                                 # 清理测试环境"
}

# 列出可用测试
list_tests() {
    log_info "可用的防火墙测试:"
    echo
    echo "单元测试:"
    echo "  TestNewFirewallServiceWithAutoInstall    - 测试防火墙自动安装功能"
    echo "  TestPackageManagerDetection             - 测试包管理器检测"
    echo "  TestFirewallRecommendation              - 测试防火墙推荐逻辑"
    echo "  TestUFWInstallation                     - 测试UFW安装流程"
    echo "  TestFirewalldInstallation               - 测试Firewalld安装流程"
    echo "  TestSSHPortDetection                    - 测试SSH端口检测"
    echo "  TestMultiOSEnvironment                  - 测试多操作系统环境"
    echo "  TestErrorHandling                       - 测试错误处理"
    echo "  TestConcurrentOperations                - 测试并发操作"
    echo
    echo "性能测试:"
    echo "  BenchmarkFirewallDetection              - 防火墙检测性能测试"
}

# 解析命令行参数
parse_args() {
    local privileged=false
    local cleanup_only=false
    local list_only=false
    local ubuntu_only=false
    local centos_only=false
    local test_name=""
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -p|--privileged)
                privileged=true
                shift
                ;;
            -c|--cleanup)
                cleanup_only=true
                shift
                ;;
            -l|--list)
                list_only=true
                shift
                ;;
            --ubuntu-only)
                ubuntu_only=true
                shift
                ;;
            --centos-only)
                centos_only=true
                shift
                ;;
            -*)
                log_error "未知选项: $1"
                show_help
                exit 1
                ;;
            *)
                test_name="$1"
                shift
                ;;
        esac
    done
    
    # 处理选项
    if [ "$cleanup_only" = true ]; then
        cleanup
        exit 0
    fi
    
    if [ "$list_only" = true ]; then
        list_tests
        exit 0
    fi
    
    # 检查互斥选项
    if [ "$ubuntu_only" = true ] && [ "$centos_only" = true ]; then
        log_error "不能同时指定 --ubuntu-only 和 --centos-only"
        exit 1
    fi
    
    # 执行测试
    check_dependencies
    create_test_results_dir
    
    if [ "$ubuntu_only" = true ]; then
        run_ubuntu_only_test "$test_name" "$privileged"
    elif [ "$centos_only" = true ]; then
        run_centos_only_test "$test_name" "$privileged"
    else
        run_all_tests "$privileged" "$test_name"
    fi
}

# 只运行Ubuntu测试
run_ubuntu_only_test() {
    local test_name=$1
    local privileged=$2
    
    log_info "运行Ubuntu测试..."
    
    if ! build_image "ubuntu" "tests/docker/Dockerfile.ubuntu"; then
        log_error "Ubuntu镜像构建失败"
        exit 1
    fi
    
    if ! run_container_test "ubuntu" "$test_name" "$privileged"; then
        log_error "Ubuntu测试失败"
        exit 1
    fi
}

# 只运行CentOS测试
run_centos_only_test() {
    local test_name=$1
    local privileged=$2
    
    log_info "运行CentOS测试..."
    
    if ! build_image "centos" "tests/docker/Dockerfile.centos"; then
        log_error "CentOS镜像构建失败"
        exit 1
    fi
    
    if ! run_container_test "centos" "$test_name" "$privileged"; then
        log_error "CentOS测试失败"
        exit 1
    fi
}

# 主函数
main() {
    log_info "=== X-UI防火墙测试环境 ==="
    log_info "项目根目录: $PROJECT_ROOT"
    log_info "Docker文件目录: $DOCKER_DIR"
    log_info "测试结果目录: $TEST_RESULTS_DIR"
    
    parse_args "$@"
}

# 脚本入口
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi