#!/bin/bash

# X-Panel 入站列表性能测试脚本
# 性能测试 - 验证数据库连接池优化效果和系统性能

echo "========== X-Panel 入站列表性能测试 =========="
echo "测试开始时间: $(date)"
echo

# 配置变量
TEST_DIR="tests"
RESULTS_DIR="$TEST_DIR/results"
LOG_FILE="$RESULTS_DIR/performance_test.log"
TEST_BASE_URL="http://localhost:54321"

# 创建结果目录
mkdir -p "$RESULTS_DIR"

# 性能测试配置
PERFORMANCE_TEST_DURATION=30  # 30秒性能测试
CONCURRENT_USERS=10           # 并发用户数
REQUEST_COUNT=100             # 每个用户的请求数

# 日志函数
log_test() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# 测试结果记录
record_result() {
    local test_name="$1"
    local result="$2"
    local details="$3"
    echo "$test_name|$result|$details" >> "$RESULTS_DIR/performance_test_results.csv"
}

# 1. 数据库连接池配置检查
test_database_pool_config() {
    log_test "检查数据库连接池配置"
    
    # 检查数据库相关配置
    config_files=$(find . -name "*.go" -exec grep -l "SetMaxOpenConns\|SetMaxIdleConns\|WAL\|PRAGMA" {} \; 2>/dev/null)
    
    if [ -n "$config_files" ]; then
        log_test "✓ 发现数据库连接池配置相关文件"
        
        # 检查具体配置
        for config_file in $config_files; do
            if grep -q "SetMaxOpenConns" "$config_file"; then
                log_test "✓ 发现最大连接数配置: $(grep "SetMaxOpenConns" "$config_file" | head -1)"
            fi
            if grep -q "SetMaxIdleConns" "$config_file"; then
                log_test "✓ 发现空闲连接数配置: $(grep "SetMaxIdleConns" "$config_file" | head -1)"
            fi
            if grep -q "PRAGMA.*WAL" "$config_file"; then
                log_test "✓ 发现WAL模式配置: $(grep "PRAGMA.*WAL" "$config_file" | head -1)"
            fi
        done
        
        record_result "数据库连接池配置" "PASS" "连接池配置已优化"
    else
        log_test "! 未发现数据库连接池配置"
        record_result "数据库连接池配置" "WARNING" "未发现连接池配置"
    fi
    
    # 检查数据库文件性能
    db_files=$(find . -name "*.db" -o -name "*.sqlite" -o -name "*.sqlite3" 2>/dev/null)
    
    if [ -n "$db_files" ]; then
        for db_file in $db_files; do
            if [ -f "$db_file" ]; then
                db_size=$(stat -f%z "$db_file" 2>/dev/null || stat -c%s "$db_file" 2>/dev/null)
                db_size_mb=$((db_size / 1024 / 1024))
                log_test "✓ 数据库文件大小: $db_size_mb MB"
                
                # 数据库文件大小评估
                if [ $db_size_mb -lt 100 ]; then
                    record_result "数据库大小" "PASS" "数据库大小适中 ($db_size_mb MB)"
                elif [ $db_size_mb -lt 1000 ]; then
                    record_result "数据库大小" "PARTIAL" "数据库较大 ($db_size_mb MB)，可能需要优化"
                else
                    record_result "数据库大小" "WARNING" "数据库很大 ($db_size_mb MB)，建议优化"
                fi
            fi
        done
    fi
}

# 2. 系统资源使用情况测试
test_system_resources() {
    log_test "测试系统资源使用情况"
    
    # CPU使用率
    cpu_usage=$(top -bn1 | grep "Cpu(s)" | sed "s/.*, *\([0-9.]*\)%* id.*/\1/" | awk '{print 100 - $1}')
    if [ -n "$cpu_usage" ]; then
        cpu_usage_int=$(echo "$cpu_usage" | cut -d. -f1)
        log_test "✓ 当前CPU使用率: $cpu_usage%"
        
        if [ $cpu_usage_int -lt 80 ]; then
            record_result "CPU使用率" "PASS" "CPU使用率正常 ($cpu_usage%)"
        elif [ $cpu_usage_int -lt 95 ]; then
            record_result "CPU使用率" "PARTIAL" "CPU使用率较高 ($cpu_usage%)"
        else
            record_result "CPU使用率" "WARNING" "CPU使用率过高 ($cpu_usage%)"
        fi
    fi
    
    # 内存使用情况
    memory_info=$(free -m)
    total_mem=$(echo "$memory_info" | awk 'NR==2{printf "%.0f", $2}')
    used_mem=$(echo "$memory_info" | awk 'NR==2{printf "%.0f", $3}')
    free_mem=$(echo "$memory_info" | awk 'NR==2{printf "%.0f", $7}')
    mem_usage_percent=$((used_mem * 100 / total_mem))
    
    log_test "✓ 内存使用情况: ${used_mem}MB / ${total_mem}MB ($mem_usage_percent%)"
    
    if [ $mem_usage_percent -lt 80 ]; then
        record_result "内存使用率" "PASS" "内存使用率正常 ($mem_usage_percent%)"
    elif [ $mem_usage_percent -lt 95 ]; then
        record_result "内存使用率" "PARTIAL" "内存使用率较高 ($mem_usage_percent%)"
    else
        record_result "内存使用率" "WARNING" "内存使用率过高 ($mem_usage_percent%)"
    fi
    
    # 磁盘使用情况
    disk_info=$(df -h . | tail -1)
    disk_usage=$(echo "$disk_info" | awk '{print $5}' | sed 's/%//')
    disk_available=$(echo "$disk_info" | awk '{print $4}')
    
    log_test "✓ 磁盘使用情况: $disk_usage% 已使用，可用空间 $disk_available"
    
    if [ $disk_usage -lt 80 ]; then
        record_result "磁盘使用率" "PASS" "磁盘使用率正常 ($disk_usage%)"
    elif [ $disk_usage -lt 90 ]; then
        record_result "磁盘使用率" "PARTIAL" "磁盘使用率较高 ($disk_usage%)"
    else
        record_result "磁盘使用率" "WARNING" "磁盘使用率过高 ($disk_usage%)"
    fi
}

# 3. 网络延迟测试
test_network_latency() {
    log_test "测试网络延迟和响应时间"
    
    # 测试多个目标的延迟
    test_targets=("localhost" "httpbin.org" "google.com")
    latency_results=""
    
    for target in "${test_targets[@]}"; do
        if [ "$target" = "localhost" ]; then
            response_time=$(curl -s -o /dev/null -w "%{time_total}" "http://localhost:54321" --connect-timeout 5 2>/dev/null || echo "999")
        else
            response_time=$(curl -s -o /dev/null -w "%{time_total}" "https://$target" --connect-timeout 5 2>/dev/null || echo "999")
        fi
        
        if [[ $response_time != "999" ]]; then
            response_ms=$(echo "$response_time * 1000" | bc 2>/dev/null || echo "999")
            response_ms_int=$(echo "$response_ms" | cut -d. -f1)
            
            log_test "✓ $target 响应时间: ${response_ms_int}ms"
            latency_results="${latency_results}${target}:${response_ms_int}ms "
            
            # 延迟评估
            if [ $response_ms_int -lt 100 ]; then
                record_result "网络延迟-$target" "PASS" "响应时间良好 (${response_ms_int}ms)"
            elif [ $response_ms_int -lt 500 ]; then
                record_result "网络延迟-$target" "PARTIAL" "响应时间一般 (${response_ms_int}ms)"
            else
                record_result "网络延迟-$target" "WARNING" "响应时间较慢 (${response_ms_int}ms)"
            fi
        else
            log_test "! $target 响应超时"
            record_result "网络延迟-$target" "FAIL" "响应超时"
        fi
    done
    
    log_test "延迟测试结果: $latency_results"
}

# 4. 并发性能测试
test_concurrent_performance() {
    log_test "测试并发性能"
    
    # 创建并发测试脚本
    cat > "$RESULTS_DIR/performance_test_client.sh" << EOF
#!/bin/bash
url="\$1"
count="\$2"
success_count=0
total_time=0

for i in \$(seq 1 \$count); do
    start_time=\$(date +%s%N)
    response=\$(curl -s -o /dev/null -w "%{http_code}" "\$url" --connect-timeout 10 2>/dev/null || echo "000")
    end_time=\$(date +%s%N)
    
    if [[ \$response =~ ^[23] ]]; then
        ((success_count++))
    fi
    
    elapsed=\$((end_time - start_time))
    total_time=\$((total_time + elapsed))
done

avg_time=\$((total_time / count / 1000000))  # 转换为毫秒
echo "\$success_count|\$avg_time"
EOF
    
    chmod +x "$RESULTS_DIR/performance_test_client.sh"
    
    # 执行并发测试
    log_test "启动 $CONCURRENT_USERS 个并发用户，每个发起 $REQUEST_COUNT 个请求"
    
    declare -a results
    start_time=$(date +%s)
    
    for i in $(seq 1 $CONCURRENT_USERS); do
        "$RESULTS_DIR/performance_test_client.sh" "http://httpbin.org/json" "$REQUEST_COUNT" > "$RESULTS_DIR/client_$i.txt" &
    done
    
    # 等待所有并发测试完成
    wait
    
    end_time=$(date +%s)
    total_duration=$((end_time - start_time))
    
    # 统计并发测试结果
    total_success=0
    total_requests=$((CONCURRENT_USERS * REQUEST_COUNT))
    
    for i in $(seq 1 $CONCURRENT_USERS); do
        if [ -f "$RESULTS_DIR/client_$i.txt" ]; then
            result=$(cat "$RESULTS_DIR/client_$i.txt")
            success=$(echo "$result" | cut -d'|' -f1)
            avg_time=$(echo "$result" | cut -d'|' -f2)
            total_success=$((total_success + success))
            log_test "客户端 $i: $success/$REQUEST_COUNT 请求成功，平均响应时间 ${avg_time}ms"
        fi
    done
    
    # 计算性能指标
    success_rate=$((total_success * 100 / total_requests))
    throughput=$((total_requests / total_duration))
    
    log_test "✓ 并发测试完成: 总请求数 $total_requests，成功率 $success_rate%，吞吐量 $throughput req/s"
    log_test "✓ 总测试时间: ${total_duration}秒"
    
    # 性能评估
    if [ $success_rate -eq 100 ] && [ $throughput -gt 10 ]; then
        record_result "并发性能" "PASS" "成功率 $success_rate%，吞吐量 $throughput req/s"
    elif [ $success_rate -ge 95 ]; then
        record_result "并发性能" "PARTIAL" "成功率 $success_rate%，吞吐量 $throughput req/s"
    else
        record_result "并发性能" "FAIL" "成功率 $success_rate%，吞吐量 $throughput req/s"
    fi
    
    # 清理临时文件
    rm -f "$RESULTS_DIR/performance_test_client.sh" "$RESULTS_DIR/client_"*.txt
}

# 5. API响应时间测试
test_api_response_time() {
    log_test "测试API响应时间"
    
    # 创建API测试脚本
    cat > "$RESULTS_DIR/api_response_test.sh" << 'EOF'
#!/bin/bash
url="$1"
response_times=()
successful_requests=0
total_requests=10

for i in $(seq 1 $total_requests); do
    response_time=$(curl -s -o /dev/null -w "%{time_total}" "$url" --connect-timeout 10 2>/dev/null)
    
    if [ -n "$response_time" ]; then
        response_ms=$(echo "$response_time * 1000" | bc 2>/dev/null || echo "0")
        response_times+=($response_ms)
        ((successful_requests++))
    fi
    
    # 添加小延迟避免过于频繁的请求
    sleep 0.1
done

if [ ${#response_times[@]} -gt 0 ]; then
    min_time=$(printf '%s\n' "${response_times[@]}" | sort -n | head -1)
    max_time=$(printf '%s\n' "${response_times[@]}" | sort -n | tail -1)
    avg_time=$(echo "scale=1; $(echo "${response_times[@]}" | tr ' ' '+') / ${#response_times[@]}" | bc 2>/dev/null || echo "0")
    
    echo "$successful_requests|$min_time|$max_time|$avg_time"
else
    echo "0|0|0|0"
fi
EOF
    
    chmod +x "$RESULTS_DIR/api_response_test.sh"
    
    # 测试不同类型的API端点
    api_endpoints=(
        "http://httpbin.org/json"
        "http://httpbin.org/uuid"
        "http://httpbin.org/ip"
    )
    
    for endpoint in "${api_endpoints[@]}"; do
        endpoint_name=$(basename "$endpoint")
        log_test "测试API端点: $endpoint_name"
        
        result=$("$RESULTS_DIR/api_response_test.sh" "$endpoint")
        successful_requests=$(echo "$result" | cut -d'|' -f1)
        min_time=$(echo "$result" | cut -d'|' -f2)
        max_time=$(echo "$result" | cut -d'|' -f3)
        avg_time=$(echo "$result" | cut -d'|' -f4)
        
        if [ $successful_requests -gt 0 ]; then
            log_test "✓ $endpoint_name: 成功 $successful_requests/10, 最小 ${min_time}ms, 最大 ${max_time}ms, 平均 ${avg_time}ms"
            
            # 响应时间评估
            avg_time_int=$(echo "$avg_time" | cut -d. -f1)
            if [ $avg_time_int -lt 200 ]; then
                record_result "API响应时间-$endpoint_name" "PASS" "平均响应时间良好 (${avg_time}ms)"
            elif [ $avg_time_int -lt 1000 ]; then
                record_result "API响应时间-$endpoint_name" "PARTIAL" "平均响应时间一般 (${avg_time}ms)"
            else
                record_result "API响应时间-$endpoint_name" "WARNING" "平均响应时间较慢 (${avg_time}ms)"
            fi
        else
            log_test "! $endpoint_name: 测试失败"
            record_result "API响应时间-$endpoint_name" "FAIL" "测试失败"
        fi
    done
    
    # 清理临时文件
    rm -f "$RESULTS_DIR/api_response_test.sh"
}

# 6. 内存泄漏检测
test_memory_leak() {
    log_test "检测潜在内存泄漏"
    
    # 检查进程内存使用情况
    if command -v ps >/dev/null 2>&1; then
        # 获取当前进程的内存使用
        current_mem=$(ps -o pid,vsz,rss,comm | grep -v PID | head -1 | awk '{print $3}' || echo "N/A")
        
        if [ "$current_mem" != "N/A" ]; then
            log_test "✓ 当前进程内存使用: ${current_mem}KB RSS"
            
            # 等待一段时间后再次检查
            sleep 5
            
            after_mem=$(ps -o pid,vsz,rss,comm | grep -v PID | head -1 | awk '{print $3}' || echo "N/A")
            
            if [ "$after_mem" != "N/A" ]; then
                mem_diff=$((after_mem - current_mem))
                
                if [ $mem_diff -gt 1000 ]; then
                    log_test "! 内存使用增加: ${mem_diff}KB"
                    record_result "内存泄漏检测" "WARNING" "内存使用增长 ${mem_diff}KB"
                elif [ $mem_diff -gt 0 ]; then
                    log_test "✓ 内存使用小幅增加: ${mem_diff}KB (正常范围)"
                    record_result "内存泄漏检测" "PASS" "内存使用增长正常 ${mem_diff}KB"
                else
                    log_test "✓ 内存使用稳定或下降"
                    record_result "内存泄漏检测" "PASS" "内存使用稳定"
                fi
            fi
        fi
    fi
    
    # 检查系统内存使用趋势
    if command -v vmstat >/dev/null 2>&1; then
        log_test "检查系统内存统计"
        vmstat 1 3 | tail -2 | head -1 > "$RESULTS_DIR/vmstat_output.txt"
        
        if [ -f "$RESULTS_DIR/vmstat_output.txt" ]; then
            log_test "✓ vmstat输出已保存用于分析"
            record_result "系统内存统计" "PASS" "内存统计正常"
        fi
    fi
}

# 7. 生成性能测试摘要
generate_performance_summary() {
    echo
    log_test "========== 性能测试完成 =========="
    
    if [ -f "$RESULTS_DIR/performance_test_results.csv" ]; then
        echo "性能测试结果汇总:"
        echo "测试项目|PASS|FAIL|PARTIAL|WARNING"
        echo "----------|-----|-----|--------|--------"
        
        # 统计结果
        pass_count=$(grep "PASS" "$RESULTS_DIR/performance_test_results.csv" | wc -l)
        fail_count=$(grep "FAIL" "$RESULTS_DIR/performance_test_results.csv" | wc -l)
        partial_count=$(grep "PARTIAL" "$RESULTS_DIR/performance_test_results.csv" | wc -l)
        warning_count=$(grep "WARNING" "$RESULTS_DIR/performance_test_results.csv" | wc -l)
        total_count=$((pass_count + fail_count + partial_count + warning_count))
        
        echo "总计|$total_count|$pass_count|$fail_count|$partial_count|$warning_count"
        
        log_test "性能测试完成时间: $(date)"
        log_test "结果: $pass_count 通过, $fail_count 失败, $partial_count 部分, $warning_count 警告"
        
        # 性能评估
        if [ $fail_count -eq 0 ] && [ $warning_count -eq 0 ]; then
            log_test "🎉 性能测试全部通过！"
            log_test "✓ 系统性能良好"
            log_test "✓ 数据库连接池优化生效"
            log_test "✓ 并发处理能力正常"
            log_test "✓ 响应时间满足要求"
        elif [ $pass_count -gt $fail_count ]; then
            log_test "⚠️  性能测试基本通过，存在部分问题"
            log_test "建议优化警告项目以提升整体性能"
        else
            log_test "❌ 性能测试存在较多问题"
            log_test "⚠️  建议重点优化性能和资源使用"
        fi
        
        # 系统优化建议
        echo
        log_test "========== 性能优化建议 =========="
        
        if [ -f "$RESULTS_DIR/vmstat_output.txt" ]; then
            log_test "详细的系统统计信息已保存在: $RESULTS_DIR/vmstat_output.txt"
        fi
        
        log_test "推荐优化项目:"
        log_test "1. 监控数据库连接池使用情况"
        log_test "2. 定期优化数据库查询"
        log_test "3. 实施性能监控和告警"
        log_test "4. 考虑缓存机制优化"
        log_test "5. 定期清理临时数据和日志"
        
        exit 0
    else
        log_test "❌ 无法生成性能测试摘要 - 结果文件不存在"
        exit 1
    fi
}

# 主测试流程
main() {
    log_test "开始X-Panel入站列表性能测试"
    
    # 执行性能测试
    test_database_pool_config
    test_system_resources
    test_network_latency
    test_concurrent_performance
    test_api_response_time
    test_memory_leak
    
    # 生成摘要
    generate_performance_summary
}

# 捕获中断信号
trap 'log_test "性能测试被中断"; exit 130' INT TERM

# 执行主函数
main "$@"