package benchmark

import "time"

// DefaultCases returns the standard benchmark case suite.
func DefaultCases() []Case {
	return []Case{
		{
			Name:        "simple_chat",
			Description: "Basic text conversation — no tools",
			Messages:    []string{"用一句话介绍你自己"},
			Assertions: []Assertion{
				{Type: AssertNoError},
				{Type: AssertMinResponses, Value: "1"},
			},
			Timeout: 30 * time.Second,
		},
		{
			Name:        "shell_single",
			Description: "Single shell tool call",
			Messages:    []string{"请使用 shell 工具执行: echo benchmark_ok"},
			Assertions: []Assertion{
				{Type: AssertNoError},
				{Type: AssertToolCalled, Value: "shell"},
				{Type: AssertMinResponses, Value: "1"},
			},
			Timeout: 60 * time.Second,
		},
		{
			Name:        "file_write_read",
			Description: "File tool: write then read",
			Messages:    []string{"请用 file 工具在当前目录写一个文件 bench_test.txt 内容为 hello_bench，然后用 file 工具读取它"},
			Assertions: []Assertion{
				{Type: AssertNoError},
				{Type: AssertToolCalled, Value: "file"},
				{Type: AssertMinResponses, Value: "1"},
			},
			Timeout: 60 * time.Second,
		},
		{
			Name:        "error_recovery",
			Description: "Tool fails, agent should handle gracefully",
			Messages:    []string{"请用 shell 工具执行这个不存在的命令: __benchmark_no_such_cmd__"},
			Assertions: []Assertion{
				{Type: AssertNoError},
				{Type: AssertToolCalled, Value: "shell"},
				{Type: AssertMinResponses, Value: "1"},
			},
			Timeout: 60 * time.Second,
		},
		{
			Name:        "multi_turn",
			Description: "Multi-turn conversation with context retention",
			Messages: []string{
				"请记住这个数字: 42",
				"我刚才让你记住的数字是多少?",
			},
			Assertions: []Assertion{
				{Type: AssertNoError},
				{Type: AssertMinResponses, Value: "2"},
				{Type: AssertResponseContains, Value: "42"},
			},
			Timeout: 60 * time.Second,
		},
		{
			Name:        "multi_tool_chain",
			Description: "Multiple tool calls in a sequence",
			Messages:    []string{"请用 shell 工具先执行 echo step1，再执行 echo step2，分别告诉我两次的输出"},
			Assertions: []Assertion{
				{Type: AssertNoError},
				{Type: AssertToolCalled, Value: "shell"},
				{Type: AssertMinResponses, Value: "1"},
			},
			Timeout: 90 * time.Second,
		},
		{
			Name:        "complex_task",
			Description: "Composite: create file, verify with shell, report",
			Messages:    []string{"请完成以下任务: 1) 用 file 工具创建文件 report.txt 内容为 benchmark_pass 2) 用 shell 工具执行 cat report.txt 确认内容 3) 告诉我最终结果"},
			Assertions: []Assertion{
				{Type: AssertNoError},
				{Type: AssertToolCalled, Value: "file"},
				{Type: AssertToolCalled, Value: "shell"},
				{Type: AssertMinResponses, Value: "1"},
			},
			Timeout: 120 * time.Second,
		},
	}
}
