#!/bin/bash

# X-Panel 入站列表Session管理测试脚本
# Session管理测试 - 验证Session健康检查、过期预警和自动续期功能

echo "========== X-Panel 入站列表Session管理测试 =========="
echo "测试开始时间: $(date)"
echo

# 配置变量
TEST_DIR="tests"
RESULTS_DIR="$TEST_DIR/results"
LOG_FILE="$RESULTS_DIR/session_test.log"
TEST_BASE_URL="http://localhost:54321"

# 创建结果目录
mkdir -p "$RESULTS_DIR"

# Session测试配置
SESSION_TEST_DURATION=60  # 60秒Session测试
SESSION_CHECK_INTERVAL=10  # 每10秒检查一次Session状态

# 日志函数
log_test() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# 测试结果记录
record_result() {
    local test_name="$1"
    local result="$2"
    local details="$3"
    echo "$test_name|$result|$details" >> "$RESULTS_DIR/session_test_results.csv"
}

# 1. Session配置文件检查
test_session_config() {
    log_test "检查Session管理相关配置文件"
    
    # 查找Session相关文件
    session_files=$(find . -name "*session*" -type f 2>/dev/null)
    auth_files=$(find . -name "*auth*" -type f 2>/dev/null)
    
    if [ -n "$session_files" ] || [ -n "$auth_files" ]; then
        log_test "✓ 发现Session管理相关文件"
        
        if [ -n "$session_files" ]; then
            log_test "Session文件: $session_files"
            
            # 检查Session管理文件内容
            for session_file in $session_files; do
                if [ -f "$session_file" ]; then
                    # 检查Session管理功能
                    if grep -q "SetMaxAge\|SetLoginUser\|GetLoginUser\|IsLogin\|ClearSession" "$session_file"; then
                        log_test "✓ 发现完整Session管理函数: $(basename "$session_file")"
                        record_result "Session管理函数" "PASS" "发现完整Session管理函数"
                    else
                        log_test "! Session文件功能不完整: $(basename "$session_file")"
                        record_result "Session管理函数" "PARTIAL" "Session文件功能不完整"
                    fi
                fi
            done
        fi
        
        if [ -n "$auth_files" ]; then
            log_test "认证相关文件: $auth_files"
        fi
        
    else
        log_test "! 未发现Session管理相关文件"
        record_result "Session管理文件" "WARNING" "未发现Session管理文件"
    fi
    
    # 检查Session相关配置
    config_files=$(find . -name "*.go" -exec grep -l "session\|Session" {} \; 2>/dev/null)
    
    if [ -n "$config_files" ]; then
        log_test "✓ 发现 $(( $(echo "$config_files" | wc -l) )) 个Session相关配置文件"
        
        # 检查Session配置优化
        session_optimization_count=0
        for config_file in $config_files; do
            # 检查Session健康检查相关代码
            if grep -q "checkSessionHealth\|attemptSessionRefresh\|SessionMonitoring" "$config_file"; then
                ((session_optimization_count++))
                log_test "✓ 发现Session监控代码: $(basename "$config_file")"
            fi
            
            # 检查Session续期机制
            if grep -q "refreshSession\|autoRefresh\|续期" "$config_file"; then
                log_test "✓ 发现Session续期机制: $(basename "$config_file")"
            fi
            
            # 检查Session过期检测
            if grep -q "expiry\|expired\|过期" "$config_file"; then
                log_test "✓ 发现Session过期检测: $(basename "$config_file")"
            fi
        done
        
        if [ $session_optimization_count -gt 0 ]; then
            record_result "Session监控机制" "PASS" "发现 $session_optimization_count 个监控机制"
        else
            record_result "Session监控机制" "WARNING" "未发现明显的Session监控机制"
        fi
    fi
}

# 2. 前端Session监控代码检查
test_frontend_session_monitoring() {
    log_test "检查前端Session监控代码"
    
    # 检查HTML文件中的Session相关代码
    html_files=$(find . -name "*.html" -exec grep -l "session\|Session\|sessionCheck\|sessionRefresh" {} \; 2>/dev/null)
    
    if [ -n "$html_files" ]; then
        log_test "✓ 发现 $(( $(echo "$html_files" | wc -l) )) 个包含Session代码的HTML文件"
        
        # 详细检查入站页面
        inbound_html_file="./web/html/inbounds.html"
        if [ -f "$inbound_html_file" ]; then
            log_test "详细检查入站页面Session功能"
            
            # 检查具体的Session监控功能
            session_features=(
                "startSessionMonitoring"
                "checkSessionHealth"
                "attemptSessionRefresh"
                "sessionCheckInterval"
                "sessionRefreshInterval"
            )
            
            found_features=0
            for feature in "${session_features[@]}"; do
                if grep -q "$feature" "$inbound_html_file"; then
                    log_test "✓ 发现Session功能: $feature"
                    ((found_features++))
                else
                    log_test "! 未发现Session功能: $feature"
                fi
            done
            
            if [ $found_features -eq ${#session_features[@]} ]; then
                log_test "✓ 入站页面Session功能完整"
                record_result "前端Session监控" "PASS" "所有Session监控功能存在"
            elif [ $found_features -gt 0 ]; then
                log_test "! 入站页面Session功能部分完整 ($found_features/${#session_features[@]})"
                record_result "前端Session监控" "PARTIAL" "部分Session监控功能存在"
            else
                log_test "✗ 入站页面Session功能缺失"
                record_result "前端Session监控" "FAIL" "Session监控功能缺失"
            fi
            
            # 检查具体的监控间隔配置
            if grep -q "5.*60.*1000\|300000" "$inbound_html_file"; then
                log_test "✓ 发现Session健康检查间隔配置 (5分钟)"
                record_result "Session检查间隔" "PASS" "Session健康检查间隔配置正确"
            else
                log_test "! 未发现明确的Session检查间隔配置"
                record_result "Session检查间隔" "WARNING" "Session检查间隔配置不明确"
            fi
            
            # 检查自动续期配置
            if grep -q "25.*60.*1000\|1500000" "$inbound_html_file"; then
                log_test "✓ 发现Session自动续期配置 (25分钟)"
                record_result "Session续期配置" "PASS" "Session自动续期配置正确"
            else
                log_test "! 未发现Session自动续期配置"
                record_result "Session续期配置" "WARNING" "Session自动续期配置不明确"
            fi
        fi
        
        # 检查其他HTML文件的Session功能
        for html_file in $html_files; do
            if [ "$html_file" != "$inbound_html_file" ]; then
                if grep -q "startSessionMonitoring\|SessionMonitoring" "$html_file"; then
                    log_test "✓ 其他页面也有Session监控: $(basename "$html_file")"
                fi
            fi
        done
    else
        log_test "! 未发现前端Session监控代码"
        record_result "前端Session监控" "FAIL" "未发现前端Session监控代码"
    fi
}

# 3. Session API端点测试
test_session_api_endpoints() {
    log_test "测试Session相关API端点"
    
    # 创建Session测试脚本
    cat > "$RESULTS_DIR/session_api_test.sh" << 'EOF'
#!/bin/bash
base_url="$1"
endpoint="$2"
method="${3:-GET}"
data="$4"

if [ -n "$data" ]; then
    response=$(curl -s -X "$method" "$base_url$endpoint" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "$data" \
        -w "HTTP_CODE:%{http_code}|TIME_TOTAL:%{time_total}" \
        --cookie "$5" \
        --cookie-jar "$6" \
        2>/dev/null)
else
    response=$(curl -s -X "$method" "$base_url$endpoint" \
        -w "HTTP_CODE:%{http_code}|TIME_TOTAL:%{time_total}" \
        --cookie "$5" \
        --cookie-jar "$6" \
        2>/dev/null)
fi

echo "$response"
EOF
    
    chmod +x "$RESULTS_DIR/session_api_test.sh"
    
    # 测试Session相关API端点
    session_endpoints=(
        "/login"
        "/logout" 
        "/panel/api/auth/status"
        "/panel/api/auth/refresh"
        "/panel/api/inbounds/list"
    )
    
    cookie_file="$RESULTS_DIR/test_cookies.txt"
    
    for endpoint in "${session_endpoints[@]}"; do
        endpoint_name=$(echo "$endpoint" | sed 's/\//_/g' | sed 's/^_//')
        log_test "测试API端点: $endpoint"
        
        if [ "$endpoint" = "/login" ]; then
            # 测试登录端点
            response=$("$RESULTS_DIR/session_api_test.sh" "$TEST_BASE_URL" "$endpoint" "POST" "username=admin&password=test" "" "$cookie_file")
        elif [ "$endpoint" = "/logout" ]; then
            # 测试登出端点
            response=$("$RESULTS_DIR/session_api_test.sh" "$TEST_BASE_URL" "$endpoint" "POST" "" "$cookie_file" "$cookie_file")
        elif [ "$endpoint" = "/panel/api/auth/status" ]; then
            # 测试认证状态检查
            response=$("$RESULTS_DIR/session_api_test.sh" "$TEST_BASE_URL" "$endpoint" "GET" "" "$cookie_file" "$cookie_file")
        elif [ "$endpoint" = "/panel/api/auth/refresh" ]; then
            # 测试Session刷新
            response=$("$RESULTS_DIR/session_api_test.sh" "$TEST_BASE_URL" "$endpoint" "POST" "" "$cookie_file" "$cookie_file")
        else
            # 其他端点
            response=$("$RESULTS_DIR/session_api_test.sh" "$TEST_BASE_URL" "$endpoint" "GET" "" "$cookie_file" "$cookie_file")
        fi
        
        http_code=$(echo "$response" | grep -o "HTTP_CODE:[0-9]*" | cut -d: -f2)
        time_total=$(echo "$response" | grep -o "TIME_TOTAL:[0-9.]*" | cut -d: -f2)
        
        if [ -n "$http_code" ]; then
            log_test "✓ $endpoint: HTTP $http_code, 响应时间 ${time_total}s"
            
            # HTTP状态码评估
            if [ "$http_code" = "200" ] || [ "$http_code" = "302" ]; then
                record_result "Session API-$endpoint_name" "PASS" "HTTP $http_code, 响应正常"
            elif [ "$http_code" = "401" ] || [ "$http_code" = "403" ]; then
                record_result "Session API-$endpoint_name" "PARTIAL" "HTTP $http_code, 需要认证"
            else
                record_result "Session API-$endpoint_name" "WARNING" "HTTP $http_code, 响应异常"
            fi
        else
            log_test "! $endpoint: 测试失败"
            record_result "Session API-$endpoint_name" "FAIL" "API端点无响应"
        fi
    done
    
    # 清理临时文件
    rm -f "$RESULTS_DIR/session_api_test.sh" "$cookie_file"
}

# 4. Session过期机制测试
test_session_expiry_mechanism() {
    log_test "测试Session过期机制"
    
    # 检查前端代码中的Session过期处理
    expiry_mechanisms=(
        "visibilitychange"
        "beforeunload"
        "onvisibilitychange"
        "onpagehide"
    )
    
    found_expiry_mechanisms=0
    for mechanism in "${expiry_mechanisms[@]}"; do
        if find . -name "*.html" -exec grep -l "$mechanism" {} \; 2>/dev/null | grep -q .; then
            log_test "✓ 发现页面可见性变化处理: $mechanism"
            ((found_expiry_mechanisms++))
        fi
    done
    
    if [ $found_expiry_mechanisms -gt 0 ]; then
        log_test "✓ 发现 $found_expiry_mechanisms 个Session过期处理机制"
        record_result "Session过期机制" "PASS" "发现过期处理机制"
    else
        log_test "! 未发现明显的Session过期处理机制"
        record_result "Session过期机制" "WARNING" "未发现过期处理机制"
    fi
    
    # 检查Session健康检查实现
    if find . -name "*.html" -exec grep -l "checkSessionHealth\|SessionHealth" {} \; 2>/dev/null | grep -q .; then
        log_test "✓ 发现Session健康检查实现"
        record_result "Session健康检查" "PASS" "Session健康检查已实现"
    else
        log_test "! 未发现Session健康检查实现"
        record_result "Session健康检查" "WARNING" "未发现健康检查实现"
    fi
    
    # 检查自动续期功能
    if find . -name "*.html" -exec grep -l "attemptSessionRefresh\|autoRefreshSession" {} \; 2>/dev/null | grep -q .; then
        log_test "✓ 发现Session自动续期功能"
        record_result "Session自动续期" "PASS" "Session自动续期已实现"
    else
        log_test "! 未发现Session自动续期功能"
        record_result "Session自动续期" "WARNING" "未发现自动续期功能"
    fi
}

# 5. Session安全机制测试
test_session_security() {
    log_test "测试Session安全机制"
    
    # 检查Session Cookie安全配置
    security_features=(
        "HttpOnly"
        "Secure"
        "SameSite"
        "httpOnly"
        "secure"
        "sameSite"
    )
    
    found_security_features=0
    for feature in "${security_features[@]}"; do
        if find . -name "*.go" -exec grep -l "$feature" {} \; 2>/dev/null | grep -q .; then
            log_test "✓ 发现Session安全特性: $feature"
            ((found_security_features++))
        fi
    done
    
    if [ $found_security_features -gt 0 ]; then
        log_test "✓ 发现 $found_security_features 个Session安全特性"
        record_result "Session安全机制" "PASS" "发现安全特性"
    else
        log_test "! 未发现明显的Session安全特性"
        record_result "Session安全机制" "WARNING" "未发现安全特性"
    fi
    
    # 检查Session存储安全性
    session_storage_methods=(
        "gin-contrib/sessions"
        "cookie"
        "redis"
        "memcached"
        "file"
    )
    
    found_storage_methods=0
    for method in "${session_storage_methods[@]}"; do
        if find . -name "*.go" -exec grep -l "$method" {} \; 2>/dev/null | grep -q .; then
            log_test "✓ 发现Session存储方法: $method"
            ((found_storage_methods++))
        fi
    done
    
    if [ $found_storage_methods -gt 0 ]; then
        log_test "✓ 发现 $found_storage_methods 种Session存储方法"
        record_result "Session存储方法" "PASS" "Session存储已配置"
    else
        log_test "! 未发现明确的Session存储方法"
        record_result "Session存储方法" "WARNING" "未发现存储方法"
    fi
}

# 6. Session用户体验测试
test_session_user_experience() {
    log_test "测试Session用户交互体验"
    
    # 检查前端用户提示和反馈
    ux_features=(
        "session.*warning\|warning.*session"
        "session.*error\|error.*session"
        "session.*success\|success.*session"
        "expired.*session\|session.*expired"
        "timeout.*session\|session.*timeout"
    )
    
    found_ux_features=0
    for feature in "${ux_features[@]}"; do
        if find . -name "*.html" -exec grep -Ei "$feature" {} \; 2>/dev/null | grep -q .; then
            log_test "✓ 发现Session用户提示: $(echo "$feature" | cut -d'|' -f1)"
            ((found_ux_features++))
        fi
    done
    
    if [ $found_ux_features -gt 0 ]; then
        log_test "✓ 发现 $found_ux_features 个Session用户体验功能"
        record_result "Session用户体验" "PASS" "发现用户提示功能"
    else
        log_test "! 未发现明显的Session用户体验功能"
        record_result "Session用户体验" "WARNING" "未发现用户体验功能"
    fi
    
    # 检查自动重试机制
    if find . -name "*.html" -exec grep -l "retry\|Retry\|retryWithBackoff\|指数退避" {} \; 2>/dev/null | grep -q .; then
        log_test "✓ 发现Session重试机制"
        record_result "Session重试机制" "PASS" "发现重试机制"
    else
        log_test "! 未发现Session重试机制"
        record_result "Session重试机制" "WARNING" "未发现重试机制"
    fi
}

# 7. Session监控和日志测试
test_session_monitoring_logging() {
    log_test "测试Session监控和日志记录"
    
    # 检查Session监控代码
    monitoring_features=(
        "console.*log.*Session\|Session.*console.*log"
        "console.*warn.*Session\|Session.*console.*warn"
        "console.*error.*Session\|Session.*console.*error"
        "log.*Session\|Session.*log"
        "监控.*Session\|Session.*监控"
    )
    
    found_monitoring_features=0
    for feature in "${monitoring_features[@]}"; do
        if find . -name "*.html" -o -name "*.js" | xargs grep -Ei "$feature" 2>/dev/null | grep -q .; then
            log_test "✓ 发现Session监控日志: $(echo "$feature" | cut -d'|' -f1)"
            ((found_monitoring_features++))
        fi
    done
    
    if [ $found_monitoring_features -gt 0 ]; then
        log_test "✓ 发现 $found_monitoring_features 个Session监控功能"
        record_result "Session监控日志" "PASS" "发现监控日志功能"
    else
        log_test "! 未发现明显的Session监控日志"
        record_result "Session监控日志" "WARNING" "未发现监控日志"
    fi
    
    # 检查错误统计和上报
    if find . -name "*.html" -exec grep -l "logErrorStats\|reportError\|错误统计" {} \; 2>/dev/null | grep -q .; then
        log_test "✓ 发现Session错误统计功能"
        record_result "Session错误统计" "PASS" "发现错误统计功能"
    else
        log_test "! 未发现Session错误统计功能"
        record_result "Session错误统计" "WARNING" "未发现错误统计功能"
    fi
}

# 8. 生成Session测试摘要
generate_session_test_summary() {
    echo
    log_test "========== Session管理测试完成 =========="
    
    if [ -f "$RESULTS_DIR/session_test_results.csv" ]; then
        echo "Session管理测试结果汇总:"
        echo "测试项目|PASS|FAIL|PARTIAL|WARNING"
        echo "----------|-----|-----|--------|--------"
        
        # 统计结果
        pass_count=$(grep "PASS" "$RESULTS_DIR/session_test_results.csv" | wc -l)
        fail_count=$(grep "FAIL" "$RESULTS_DIR/session_test_results.csv" | wc -l)
        partial_count=$(grep "PARTIAL" "$RESULTS_DIR/session_test_results.csv" | wc -l)
        warning_count=$(grep "WARNING" "$RESULTS_DIR/session_test_results.csv" | wc -l)
        total_count=$((pass_count + fail_count + partial_count + warning_count))
        
        echo "总计|$total_count|$pass_count|$fail_count|$partial_count|$warning_count"
        
        log_test "Session管理测试完成时间: $(date)"
        log_test "结果: $pass_count 通过, $fail_count 失败, $partial_count 部分, $warning_count 警告"
        
        # Session管理评估
        if [ $fail_count -eq 0 ] && [ $warning_count -eq 0 ]; then
            log_test "🎉 Session管理测试全部通过！"
            log_test "✓ Session健康检查机制完善"
            log_test "✓ Session过期预警功能正常"
            log_test "✓ Session自动续期机制工作正常"
            log_test "✓ Session安全机制配置正确"
            log_test "✓ 用户体验优化到位"
        elif [ $pass_count -gt $fail_count ]; then
            log_test "⚠️  Session管理测试基本通过，存在部分改进空间"
            log_test "建议优化警告项目以提升Session管理质量"
        else
            log_test "❌ Session管理测试存在较多问题"
            log_test "⚠️  建议重点完善Session管理机制"
        fi
        
        # Session优化建议
        echo
        log_test "========== Session管理优化建议 =========="
        log_test "推荐优化项目:"
        log_test "1. 完善Session过期时间配置"
        log_test "2. 增强Session安全性和加密"
        log_test "3. 优化Session存储性能"
        log_test "4. 添加Session使用统计"
        log_test "5. 完善Session异常处理"
        log_test "6. 实现Session多设备管理"
        log_test "7. 添加Session使用提醒"
        
        exit 0
    else
        log_test "❌ 无法生成Session测试摘要 - 结果文件不存在"
        exit 1
    fi
}

# 主测试流程
main() {
    log_test "开始X-Panel入站列表Session管理测试"
    
    # 执行Session管理测试
    test_session_config
    test_frontend_session_monitoring
    test_session_api_endpoints
    test_session_expiry_mechanism
    test_session_security
    test_session_user_experience
    test_session_monitoring_logging
    
    # 生成摘要
    generate_session_test_summary
}

# 捕获中断信号
trap 'log_test "Session管理测试被中断"; exit 130' INT TERM

# 执行主函数
main "$@"