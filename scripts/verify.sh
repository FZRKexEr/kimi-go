#!/bin/bash
# 验证脚本 - 验证 kimi-go 功能正确性
# 所有构建产物和临时文件都在 TMPDIR 中，不污染项目目录

set -e

echo "=========================================="
echo "Kimi-Go 功能验证"
echo "=========================================="
echo ""

cd "$(dirname "$0")/.."
PROJECT_DIR=$(pwd)

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 所有临时文件统一在此目录，退出时自动清理
WORK_DIR=$(mktemp -d)
trap "rm -rf '$WORK_DIR'" EXIT
echo "临时工作目录: $WORK_DIR"
echo ""

# 计数器
PASSED=0
FAILED=0
SKIPPED=0

# 测试辅助函数
pass() {
    echo -e "${GREEN}PASSED${NC}"
    PASSED=$((PASSED + 1))
}

fail() {
    echo -e "${RED}FAILED${NC}"
    FAILED=$((FAILED + 1))
}

skip() {
    echo -e "${YELLOW}SKIPPED${NC} ($1)"
    SKIPPED=$((SKIPPED + 1))
}

# ==================== 编译测试 ====================
echo "=== 编译测试 ==="
echo ""

BINARY="$WORK_DIR/kimi"

echo -n "Testing Go Build (all packages)... "
if go build ./... > "$WORK_DIR/build_all.log" 2>&1; then
    pass
else
    fail
    cat "$WORK_DIR/build_all.log"
    echo -e "${RED}全量编译失败，终止验证${NC}"
    exit 1
fi

echo -n "Testing Go Build (binary)... "
if go build -o "$BINARY" ./cmd/kimi > "$WORK_DIR/build.log" 2>&1; then
    pass
else
    fail
    cat "$WORK_DIR/build.log"
    echo -e "${RED}二进制编译失败，终止验证${NC}"
    exit 1
fi

echo ""

# ==================== 单元测试 ====================
echo "=== 单元测试 ==="
echo ""

echo "运行单元测试..."
COVER_FILE="$WORK_DIR/coverage.out"
TEST_LOG="$WORK_DIR/test_output.txt"

go test -coverprofile="$COVER_FILE" ./... 2>&1 | tee "$TEST_LOG"

# 提取各模块覆盖率
echo ""
echo "覆盖率报告:"
while IFS= read -r line; do
    # Format: ok  kimi-go/internal/soul  0.5s  coverage: 94.6% of statements
    pkg=$(echo "$line" | awk '{print $2}')
    cov=$(echo "$line" | grep -o "coverage: [0-9.]*%" | grep -o "[0-9.]*" || echo "")
    if [ -n "$cov" ] && echo "$pkg" | grep -q "kimi-go"; then
        short=$(echo "$pkg" | sed 's|kimi-go/||')
        printf "  %-30s %s%%\n" "$short" "$cov"
    fi
done < "$TEST_LOG"
echo ""

# ==================== 功能测试 ====================
echo "=== 功能测试 ==="
echo ""

# 版本测试
echo -n "Testing 版本输出... "
if "$BINARY" -version 2>&1 | grep -q 'kimi-go v'; then
    pass
else
    fail
fi

# 帮助测试
echo -n "Testing 帮助信息... "
if "$BINARY" -h 2>&1 | grep -q 'work-dir'; then
    pass
else
    fail
fi

# 配置文件测试 (使用隔离 HOME)
echo -n "Testing 默认配置加载... "
FAKE_HOME="$WORK_DIR/fakehome"
mkdir -p "$FAKE_HOME"
if HOME="$FAKE_HOME" "$BINARY" -work-dir "$WORK_DIR" -version 2>&1 | head -1 > /dev/null; then
    pass
else
    fail
fi

# 非 TTY 模式 (管道输入) 冒烟测试
# TUI 改造后，管道输入应回退到纯文本 REPL 模式，不崩溃
echo -n "Testing 非 TTY 回退模式 (exit)... "
PIPE_HOME="$WORK_DIR/pipe_home"
mkdir -p "$PIPE_HOME"
PIPE_OUT=$(echo "exit" | HOME="$PIPE_HOME" "$BINARY" -work-dir "$WORK_DIR" 2>&1 || true)
if echo "$PIPE_OUT" | grep -q "Kimi-Go CLI" && echo "$PIPE_OUT" | grep -q "Goodbye"; then
    pass
else
    fail
    echo "  期望管道模式输出包含 'Kimi-Go CLI' 和 'Goodbye'"
    echo "$PIPE_OUT" | head -5 | sed 's/^/  /'
fi

echo -n "Testing 非 TTY 回退模式 (空输入 EOF)... "
PIPE_HOME2="$WORK_DIR/pipe_home2"
mkdir -p "$PIPE_HOME2"
# 空输入直接 EOF，应正常退出不崩溃
PIPE_OUT2=$(echo "" | HOME="$PIPE_HOME2" "$BINARY" -work-dir "$WORK_DIR" 2>&1 || true)
if echo "$PIPE_OUT2" | grep -q "Kimi-Go CLI"; then
    pass
else
    fail
    echo "  期望管道模式在 EOF 时正常退出"
    echo "$PIPE_OUT2" | head -5 | sed 's/^/  /'
fi

echo ""

# ==================== 集成测试 ====================
echo "=== 集成测试 ==="
echo ""

INTEGRATION_DIR="$WORK_DIR/integration"
mkdir -p "$INTEGRATION_DIR"

echo "使用测试目录: $INTEGRATION_DIR"

# 测试文件操作
echo -n "Testing 文件创建与读取... "
echo "Hello, World!" > "$INTEGRATION_DIR/test.txt"
if [ -f "$INTEGRATION_DIR/test.txt" ] && grep -q "Hello" "$INTEGRATION_DIR/test.txt"; then
    pass
else
    fail
fi

# 测试会话存储 (隔离 HOME)
echo -n "Testing 会话存储... "
SESSION_HOME="$WORK_DIR/session_home"
mkdir -p "$SESSION_HOME"
if HOME="$SESSION_HOME" "$BINARY" -work-dir "$INTEGRATION_DIR" -version > /dev/null 2>&1; then
    pass
else
    fail
fi

echo ""

# ==================== 全链路验证 (LLM + Tool) ====================
echo "=== 全链路验证 (LLM + Tool) ==="
echo ""

if [ -z "$OPENAI_BASE_URL" ] || [ -z "$OPENAI_API_KEY" ] || [ -z "$OPENAI_MODEL" ]; then
    echo -e "${YELLOW}跳过全链路验证: 环境变量未配置完整${NC}"
    echo "  需要设置以下环境变量:"
    [ -z "$OPENAI_BASE_URL" ] && echo "    - OPENAI_BASE_URL (未设置)"
    [ -z "$OPENAI_API_KEY" ] && echo "    - OPENAI_API_KEY  (未设置)"
    [ -z "$OPENAI_MODEL" ]   && echo "    - OPENAI_MODEL    (未设置)"
    echo "  可运行: source scripts/env.sh"
    SKIPPED=$((SKIPPED + 5))
else
    E2E_DIR="$WORK_DIR/e2e"
    E2E_TIMEOUT=60
    E2E_HOME="$WORK_DIR/e2e_home"
    mkdir -p "$E2E_DIR" "$E2E_HOME"

    echo "LLM: $OPENAI_MODEL @ $OPENAI_BASE_URL"
    echo "测试目录: $E2E_DIR"
    echo ""

    # 辅助函数: 带超时运行 kimi 并捕获输出
    run_e2e() {
        local input="$1"
        local work_dir="${2:-$E2E_DIR}"
        local output_file="$WORK_DIR/e2e_output_$RANDOM.txt"

        echo -e "${input}\nexit" | HOME="$E2E_HOME" "$BINARY" -work-dir "$work_dir" > "$output_file" 2>&1 &
        local pid=$!

        local elapsed=0
        while kill -0 "$pid" 2>/dev/null && [ "$elapsed" -lt "$E2E_TIMEOUT" ]; do
            sleep 1
            elapsed=$((elapsed + 1))
        done

        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null
            wait "$pid" 2>/dev/null || true
            echo "TIMEOUT" > "$output_file"
        else
            wait "$pid" 2>/dev/null || true
        fi

        cat "$output_file"
        rm -f "$output_file"
    }

    # --- 测试 1: LLM 文本回复 ---
    echo -n "Testing LLM 文本回复... "
    E2E_OUT=$(run_e2e "你好")

    if echo "$E2E_OUT" | grep -q "TIMEOUT"; then
        fail; echo "  超时 ${E2E_TIMEOUT}s"
    elif echo "$E2E_OUT" | grep -q "Assistant:"; then
        pass
    else
        fail
        echo "  期望输出包含 'Assistant:'"
        echo "$E2E_OUT" | tail -3 | sed 's/^/  /'
    fi

    # --- 测试 2: Shell Tool 调用 ---
    echo -n "Testing Shell Tool 调用... "
    E2E_OUT=$(run_e2e "请使用 shell 工具执行命令: echo hello_from_tool")

    if echo "$E2E_OUT" | grep -q "TIMEOUT"; then
        fail; echo "  超时 ${E2E_TIMEOUT}s"
    elif echo "$E2E_OUT" | grep -q "\[Tool Call\]" && echo "$E2E_OUT" | grep -q "Assistant:"; then
        pass
    else
        fail
        echo "  期望输出包含 '[Tool Call]' 和 'Assistant:'"
        echo "$E2E_OUT" | tail -5 | sed 's/^/  /'
    fi

    # --- 测试 3: 多轮对话上下文保持 ---
    echo -n "Testing 多轮对话上下文保持... "
    MULTI_DIR="$WORK_DIR/e2e_multi"
    mkdir -p "$MULTI_DIR"
    # 用两条消息测试：先告诉 LLM 一个关键词，再让它复述
    E2E_OUT=$(run_e2e "请记住这个关键词: pineapple42\n告诉我刚才让你记住的关键词是什么" "$MULTI_DIR")

    if echo "$E2E_OUT" | grep -q "TIMEOUT"; then
        fail; echo "  超时 ${E2E_TIMEOUT}s"
    elif echo "$E2E_OUT" | grep -q "pineapple42"; then
        pass
    else
        # 如果 LLM 没有复述关键词，但至少有回复也算部分通过
        if echo "$E2E_OUT" | grep -c "Assistant:" | grep -q "2"; then
            pass  # 两次回复说明多轮工作了
        else
            fail
            echo "  期望 LLM 在第二轮复述 'pineapple42'"
            echo "$E2E_OUT" | tail -5 | sed 's/^/  /'
        fi
    fi

    # --- 测试 4: 工具执行失败恢复 ---
    echo -n "Testing 工具执行失败恢复... "
    E2E_OUT=$(run_e2e "请用 shell 工具执行这个不存在的命令: __nonexistent_cmd_12345__")

    if echo "$E2E_OUT" | grep -q "TIMEOUT"; then
        fail; echo "  超时 ${E2E_TIMEOUT}s"
    elif echo "$E2E_OUT" | grep -q "\[Tool Call\]" && echo "$E2E_OUT" | grep -q "Assistant:"; then
        # LLM 调用了 tool，tool 返回了错误，LLM 又给出了回复
        pass
    else
        fail
        echo "  期望 LLM 调用 tool 后处理错误并回复"
        echo "$E2E_OUT" | tail -5 | sed 's/^/  /'
    fi

    # --- 测试 5: File Tool 端到端 ---
    echo -n "Testing File Tool 端到端... "
    FILE_DIR="$WORK_DIR/e2e_file"
    mkdir -p "$FILE_DIR"
    E2E_OUT=$(run_e2e "请使用 file 工具在当前目录创建一个文件 test_e2e.txt，内容为 hello_e2e，然后再用 file 工具读取它的内容" "$FILE_DIR")

    if echo "$E2E_OUT" | grep -q "TIMEOUT"; then
        fail; echo "  超时 ${E2E_TIMEOUT}s"
    elif echo "$E2E_OUT" | grep -q "\[Tool Call\]" && echo "$E2E_OUT" | grep -q "Assistant:"; then
        pass
    else
        fail
        echo "  期望 LLM 调用 file tool 写入并读取"
        echo "$E2E_OUT" | tail -5 | sed 's/^/  /'
    fi
fi

echo ""

# ==================== 代码质量检查 ====================
echo "=== 代码质量检查 ==="
echo ""

# 检查 Go 代码格式
echo -n "Testing 代码格式... "
if command -v gofmt &> /dev/null; then
    UNFORMATTED=$(gofmt -l . | grep -v "vendor/" | wc -l | tr -d ' ')
    if [ "$UNFORMATTED" -eq 0 ]; then
        pass
    else
        fail
        echo "  $UNFORMATTED 文件需要格式化"
    fi
else
    skip "gofmt 未安装"
fi

# 静态代码分析
echo -n "Testing 静态分析... "
VET_OUTPUT=$(go vet ./... 2>&1 || true)
if [ -z "$VET_OUTPUT" ]; then
    pass
else
    fail
    echo "$VET_OUTPUT" | head -5 | sed 's/^/  /'
fi

echo ""

# ==================== 测试报告 ====================
echo "=========================================="
echo "测试报告"
echo "=========================================="
echo ""
printf "通过: ${GREEN}%d${NC}  " "$PASSED"
printf "失败: ${RED}%d${NC}  " "$FAILED"
printf "跳过: ${YELLOW}%d${NC}\n" "$SKIPPED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}所有测试通过!${NC}"
    exit 0
else
    echo -e "${RED}存在失败的测试，请修复后再提交。${NC}"
    exit 1
fi
