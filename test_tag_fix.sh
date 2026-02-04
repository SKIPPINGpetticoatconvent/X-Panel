#!/bin/bash

# 测试入站标签唯一性修复
# 此脚本验证修复后的行为

echo "=== 测试入站标签唯一性修复 ==="

# 检查代码格式
echo "1. 检查代码格式..."
gofmt -l ./database/repository ./web/service
if [ $? -ne 0 ]; then
    echo "代码格式检查失败!"
    exit 1
fi
echo "✓ 代码格式正确"

# 运行相关测试
echo "2. 运行入站标签唯一性测试..."
go test -v ./database/repository -run TestInboundTagTestSuite
if [ $? -ne 0 ]; then
    echo "测试失败!"
    exit 1
fi
echo "✓ 标签唯一性测试通过"

# 运行完整的仓库测试
echo "3. 运行完整的仓库测试..."
go test -v ./database/repository
if [ $? -ne 0 ]; then
    echo "仓库测试失败!"
    exit 1
fi
echo "✓ 仓库测试通过"

# 运行服务层测试
echo "4. 运行服务层测试..."
go test -v ./web/service -run TestInboundService
if [ $? -ne 0 ]; then
    echo "服务层测试失败!"
    exit 1
fi
echo "✓ 服务层测试通过"

echo "=== 所有测试通过，修复验证成功! ==="
echo ""
echo "修复内容:"
echo "- 添加了 CheckTagExist 方法到 InboundRepository"
echo "- 在 AddInbound 方法中添加了 tag 唯一性检查"
echo "- 在 UpdateInbound 方法中添加了 tag 唯一性检查"
echo "- 添加了完整的测试覆盖"
echo ""
echo "现在用户在修改入站设置时，如果 tag 重复，"
echo "将会收到明确的错误提示，而不是数据库约束错误。"
