#!/bin/bash
# API 测试脚本 - 验证 LLM 客户端与真实 API 的集成

set -e

echo "=========================================="
echo "Kimi-Go API 测试"
echo "=========================================="
echo ""

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查环境变量
if [ -z "$OPENAI_API_KEY" ]; then
    echo -e "${RED}错误: OPENAI_API_KEY 未设置${NC}"
    echo "请设置环境变量:"
    echo "  export OPENAI_API_KEY='your-api-key'"
    exit 1
fi

if [ -z "$OPENAI_BASE_URL" ]; then
    echo -e "${YELLOW}警告: OPENAI_BASE_URL 未设置，使用默认值${NC}"
    export OPENAI_BASE_URL="https://api.openai.com/v1"
fi

if [ -z "$OPENAI_MODEL" ]; then
    echo -e "${YELLOW}警告: OPENAI_MODEL 未设置，使用默认值 gpt-3.5-turbo${NC}"
    export OPENAI_MODEL="gpt-3.5-turbo"
fi

echo "配置信息:"
echo "  Base URL: $OPENAI_BASE_URL"
echo "  Model: $OPENAI_MODEL"
echo ""

# 运行测试
echo "=========================================="
echo "运行集成测试..."
echo "=========================================="
echo ""

cd "$(dirname "$0")/.."

go test -v ./llm -run "TestIntegration_BasicChat" -timeout 60s 2>&1

if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}测试通过!${NC}"
else
    echo ""
    echo -e "${RED}测试失败${NC}"
    exit 1
fi

echo ""
echo "=========================================="
echo "API 测试完成"
echo "=========================================="
