package benchmark

import (
	"os"
	"testing"
)

func TestBenchmark_DefaultCases(t *testing.T) {
	if os.Getenv("RUN_BENCHMARK") == "" {
		t.Skip("Skipping benchmark: set RUN_BENCHMARK=1 to run (use 'make benchmark')")
	}

	runner := NewRunnerFromEnv()
	if runner == nil {
		t.Skip("Skipping benchmark: OPENAI_BASE_URL, OPENAI_API_KEY, OPENAI_MODEL not set")
	}

	cases := DefaultCases()
	report := runner.Run(cases)

	PrintReport(report)

	// Optionally save results
	outDir := os.Getenv("BENCHMARK_OUTPUT_DIR")
	if outDir != "" {
		outPath := outDir + "/benchmark_report.json"
		if err := SaveReport(report, outPath); err != nil {
			t.Logf("Warning: failed to save report: %v", err)
		} else {
			t.Logf("Report saved to: %s", outPath)
		}
	}

	// Report failures but don't fail the test — benchmark results are informational
	for _, r := range report.Results {
		if !r.Passed {
			t.Logf("BENCHMARK FAIL: %s — %v", r.Case, r.FailedChecks)
		}
	}

	t.Logf("Benchmark complete: %d/%d passed (%.0f%%)",
		report.Summary.Passed, report.Summary.Total, report.Summary.PassRate)
}

func TestBenchmark_UnitTest_Assertions(t *testing.T) {
	// Test assertion logic without real LLM

	// AssertMinResponses
	result := &Result{Responses: 3}
	err := checkAssertion(Assertion{Type: AssertMinResponses, Value: "2"}, &recorder{}, result)
	if err != nil {
		t.Errorf("expected pass for min_responses 2 with 3 responses: %v", err)
	}

	err = checkAssertion(Assertion{Type: AssertMinResponses, Value: "5"}, &recorder{}, result)
	if err == nil {
		t.Error("expected fail for min_responses 5 with 3 responses")
	}

	// AssertNoError
	err = checkAssertion(Assertion{Type: AssertNoError}, &recorder{}, &Result{})
	if err != nil {
		t.Errorf("expected pass for no_error with no errors: %v", err)
	}

	err = checkAssertion(Assertion{Type: AssertNoError}, &recorder{}, &Result{Errors: []string{"oops"}})
	if err == nil {
		t.Error("expected fail for no_error with errors")
	}

	// Unknown assertion type
	err = checkAssertion(Assertion{Type: "bogus"}, &recorder{}, &Result{})
	if err == nil {
		t.Error("expected error for unknown assertion type")
	}
}

func TestDefaultCases_Valid(t *testing.T) {
	cases := DefaultCases()
	if len(cases) == 0 {
		t.Fatal("DefaultCases should return at least one case")
	}

	for _, c := range cases {
		if c.Name == "" {
			t.Error("case name should not be empty")
		}
		if len(c.Messages) == 0 {
			t.Errorf("case %q should have at least one message", c.Name)
		}
		if len(c.Assertions) == 0 {
			t.Errorf("case %q should have at least one assertion", c.Name)
		}
		if c.Timeout == 0 {
			t.Errorf("case %q should have a timeout", c.Name)
		}
	}
}
