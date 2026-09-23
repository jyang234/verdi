package main

import (
	"bytes"
	"context"
	"testing"
)

// TestRealNamedGoTestRunner_ExecPath is the brief's one hermetic
// integration test: it runs the REAL `go test -json -count=1 -run ...
// ./sample` against the tiny, dependency-free fixture module under
// testdata/gotestfixture/ (via realNamedGoTestRunner, never a fake) to
// prove the actual exec path end to end — real toolchain output flowing
// through readNamedTestOutcomes and landing on all three real outcomes
// contract 5 maps to a verdict, plus a requested-but-nonexistent test
// reading absent. No network: this execs the local `go` toolchain against
// a fixture module already on disk, exactly the one exec CLAUDE.md's "no
// exec in any test" rule is deliberately waived for here (this file's
// package doc explains why: it is the only place realNamedGoTestRunner
// itself is exercised for real).
func TestRealNamedGoTestRunner_ExecPath(t *testing.T) {
	runner := realNamedGoTestRunner{}
	pattern := goTestRunPattern([]string{"TestPass", "TestFail", "TestSkip", "TestAbsent"})

	out, err := runner.RunNamedGoTest(context.Background(), "testdata/gotestfixture", "sample", pattern)
	if err != nil {
		t.Fatalf("RunNamedGoTest: %v", err)
	}

	outcomes, err := readNamedTestOutcomes(bytes.NewReader(out), "example.com/gotestfixture/sample")
	if err != nil {
		t.Fatalf("readNamedTestOutcomes: %v\n--- raw go test -json output ---\n%s", err, out)
	}

	if outcomes["TestPass"] != testOutcomePass {
		t.Errorf("TestPass outcome = %q, want pass", outcomes["TestPass"])
	}
	if outcomes["TestFail"] != testOutcomeFail {
		t.Errorf("TestFail outcome = %q, want fail", outcomes["TestFail"])
	}
	if outcomes["TestSkip"] != testOutcomeSkip {
		t.Errorf("TestSkip outcome = %q, want skip", outcomes["TestSkip"])
	}
	if _, present := outcomes["TestAbsent"]; present {
		t.Errorf("TestAbsent present in outcomes %+v, want absent (no such test exists)", outcomes)
	}
}
