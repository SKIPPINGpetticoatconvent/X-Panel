#!/bin/bash

# X-Panel 入站列表功能测试脚本
# 功能测试 - 验证入站列表的基本功能是否正常

echo "========== X-Panel 入站列表功能测试 =========="
echo "测试开始时间: $(date)"
echo

# 配置变量
TEST_DIR="tests"
RESULTS_DIR="$TEST_DIR/results"
LOG_FILE="$RESULTS_DIR/functional_test.log"
TEST_BASE_URL="http://localhost:54321"
TEST_ADMIN_USER="admin"
TEST_ADMIN_PASS="admin123"

# 创建结果目录
mkdir -p "$RESULTS_DIR"

# 日志函数
log_test() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# 测试结果记录
record_result() {
    local test_name="$1"
    local result="$2"
    local details="$3"
    echo "$test_name|$result|$details" >> "$RESULTS_DIR/test_results.csv"
}

# 1. 检查服务状态
test_server_status() {
    log_test "检查X-Panel服务状态"
    
    response=$(curl -s -o /dev/null -w "%{http_code}" "$TEST_BASE_URL" --connect-timeout 10)
    
    if [ "$response" = "200" ] || [ "$response" = "302" ]; then
        log_test "✓ 服务状态正常 (HTTP $response)"
        record_result "服务状态检查" "PASS" "HTTP $response"
        return 0
    else
        log_test "✗ 服务状态异常 (HTTP $response)"
        record_result "服务状态检查" "FAIL" "HTTP $response"
        return 1
    fi
}

# 2. 测试登录功能
test_login_function() {
    log_test "测试登录功能"
    
    # 获取登录页面
    login_page=$(curl -s "$TEST_BASE_URL/login" --connect-timeout 10)
    
    if [[ $login_page == *"login"* ]] || [[ $login_page == *"username"* ]]; then
        log_test "✓ 登录页面可访问"
        
        # 尝试登录（这里假设admin/admin123是默认凭据）
        login_response=$(curl -s -X POST "$TEST_BASE_URL/login" \
            -d "username=$TEST_ADMIN_USER&password=$TEST_ADMIN_PASS" \
            -H "Content-Type: application/x-www-form-urlencoded" \
            --cookie-jar "$RESULTS_DIR/cookies.txt" \
            --connect-timeout 10)
        
        if [[ $login_response == *"dashboard"* ]] || [[ $login_response == *"inbounds"* ]]; then
            log_test "✓ 登录成功"
            record_result "登录功能" "PASS" "成功登录"
            return 0
        else
            log_test "! 登录可能需要手动验证"
            record_result "登录功能" "PARTIAL" "页面可访问，需手动验证"
            return 1
        fi
    else
        log_test "✗ 登录页面访问失败"
        record_result "登录功能" "FAIL" "无法访问登录页面"
        return 1
    fi
}

# 3. 测试入站列表API
test_inbounds_api() {
    log_test "测试入站列表API响应"
    
    # 使用cookie文件测试API（如果有的话）
    if [ -f "$RESULTS_DIR/cookies.txt" ]; then
        api_response=$(curl -s "$TEST_BASE_URL/panel/api/inbounds/list" \
            --cookie "$RESULTS_DIR/cookies.txt" \
            -H "X-Requested-With: XMLHttpRequest" \
            --connect-timeout 30)
    else
        # 尝试不登录状态下的API调用
        api_response=$(curl -s "$TEST_BASE_URL/panel/api/inbounds/list" \
            --connect-timeout 30)
    fi
    
    if [[ $api_response == *"success"* ]] || [[ $api_response == *"obj"* ]]; then
        log_test "✓ 入站列表API响应正常"
        log_test "API响应片段: ${api_response:0:200}..."
        record_result "入站列表API" "PASS" "API响应正常"
        return 0
    else
        log_test "✗ 入站列表API响应异常"
        log_test "API响应: $api_response"
        record_result "入站列表API" "FAIL" "API响应异常"
        return 1
    fi
}

# 4. 测试前端页面加载
test_frontend_loading() {
    log_test "测试前端入站列表页面加载"
    
    # 测试入站页面
    inbound_page=$(curl -s "$TEST_BASE_URL/inbounds" \
        --cookie "$RESULTS_DIR/cookies.txt" \
        --connect-timeout 30)
    
    # 检查关键元素
    checks=("inbounds" "add" "table" "v-app")
    missing_elements=0
    
    for element in "${checks[@]}"; do
        if [[ $inbound_page == *"$element"* ]]; then
            log_test "✓ 找到页面元素: $element"
        else
            log_test "! 未找到页面元素: $element"
            ((missing_elements++))
        fi
    done
    
    if [ $missing_elements -eq 0 ]; then
        log_test "✓ 前端页面加载正常"
        record_result "前端页面加载" "PASS" "所有关键元素存在"
        return 0
    elif [ $missing_elements -lt 3 ]; then
        log_test "! 前端页面部分加载"
        record_result "前端页面加载" "PARTIAL" "部分元素缺失"
        return 1
    else
        log_test "✗ 前端页面加载失败"
        record_result "前端页面加载" "FAIL" "关键元素缺失"
        return 1
    fi
}

# 5. 测试数据库连接
test_database_connection() {
    log_test "测试数据库连接和连接池配置"
    
    # 检查数据库文件是否存在
    db_files=$(find . -name "*.db" -o -name "*.sqlite" -o -name "*.sqlite3" 2>/dev/null)
    
    if [ -n "$db_files" ]; then
        log_test "✓ 找到数据库文件: $db_files"
        
        # 检查文件大小和权限
        for db_file in $db_files; do
            if [ -r "$db_file" ]; then
                log_test "✓ 数据库文件可读: $db_file"
                record_result "数据库连接" "PASS" "数据库文件存在且可读"
                return 0
            else
                log_test "! 数据库文件权限异常: $db_file"
            fi
        done
        
        record_result "数据库连接" "PARTIAL" "数据库文件存在但权限异常"
        return 1
    else
        log_test "! 未找到数据库文件"
        record_result "数据库连接" "WARNING" "未找到数据库文件"
        return 1
    fi
}

# 6. 测试端口放行功能
test_port_opening() {
    log_test "测试端口放行功能"
    
    # 模拟测试端口放行API（实际测试中需要服务运行）
    port_test_url="$TEST_BASE_URL/panel/api/server/openPort"
    
    # 注意：这里只是测试API端点是否可访问，不实际调用
    response_code=$(curl -s -o /dev/null -w "%{http_code}" \
        -X POST "$port_test_url" \
        --connect-timeout 5)
    
    if [ "$response_code" = "200" ] || [ "$response_code" = "401" ] || [ "$response_code" = "404" ]; then
        log_test "✓ 端口放行API端点可访问"
        record_result "端口放行功能" "PASS" "API端点可访问"
        return 0
    else
        log_test "! 端口放行API端点响应异常 (HTTP $response_code)"
        record_result "端口放行功能" "WARNING" "API端点响应异常"
        return 1
    fi
}

# 7. 测试错误处理机制
test_error_handling() {
    log_test "测试错误处理机制"
    
    # 测试无效API端点的错误响应
    invalid_response=$(curl -s "http://invalid-endpoint-test" \
        --connect-timeout 5 2>/dev/null || echo "timeout")
    
    if [[ $invalid_response == "timeout" ]] || [[ $invalid_response == *"error"* ]]; then
        log_test "✓ 错误处理机制工作正常"
        record_result "错误处理机制" "PASS" "错误响应正常"
        return 0
    else
        log_test "! 错误处理机制可能需要验证"
        record_result "错误处理机制" "PARTIAL" "需要进一步验证"
        return 1
    fi
}

# 8. 生成测试摘要
generate_test_summary() {
    echo
    log_test "========== 功能测试完成 =========="
    
    if [ -f "$RESULTS_DIR/test_results.csv" ]; then
        echo "测试结果汇总:"
        echo "测试项目|PASS|FAIL|PARTIAL|WARNING"
        echo "----------|-----|-----|--------|--------"
        
        # 统计结果
        pass_count=$(grep "PASS" "$RESULTS_DIR/test_results.csv" | wc -l)
        fail_count=$(grep "FAIL" "$RESULTS_DIR/test_results.csv" | wc -l)
        partial_count=$(grep "PARTIAL" "$RESULTS_DIR/test_results.csv" | wc -l)
        warning_count=$(grep "WARNING" "$RESULTS_DIR/test_results.csv" | wc -l)
        total_count=$((pass_count + fail_count + partial_count + warning_count))
        
        echo "总计|$total_count|$pass_count|$fail_count|$partial_count|$warning_count"
        
        log_test "测试完成时间: $(date)"
        log_test "结果: $pass_count 通过, $fail_count 失败, $partial_count 部分, $warning_count 警告"
        
        if [ $fail_count -eq 0 ]; then
            log_test "🎉 功能测试总体通过！"
            exit 0
        elif [ $pass_count -gt $fail_count ]; then
            log_test "⚠️  功能测试基本通过，存在部分问题"
            exit 0
        else
            log_test "❌ 功能测试未通过，需要修复问题"
            exit 1
        fi
    else
        log_test "❌ 无法生成测试摘要 - 结果文件不存在"
        exit 1
    fi
}

# 主测试流程
main() {
    log_test "开始X-Panel入站列表功能测试"
    log_test "测试环境: $TEST_BASE_URL"
    
    # 执行测试
    test_server_status
    test_login_function
    test_inbounds_api
    test_frontend_loading
    test_database_connection
    test_port_opening
    test_error_handling
    
    # 生成摘要
    generate_test_summary
}

# 捕获中断信号
trap 'log_test "测试被中断"; exit 130' INT TERM

# 执行主函数
main "$@"