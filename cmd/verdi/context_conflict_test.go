package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyconflict"
)

type contextConflictProviderFunc func(context.Context, policyconflict.Request) (policyconflict.Result, error)

func (f contextConflictProviderFunc) Evaluate(ctx context.Context, request policyconflict.Request) (policyconflict.Result, error) {
	return f(ctx, request)
}

func contextConflictRequestBytes(t *testing.T, spec string) []byte {
	t.Helper()
	accepted, err := contextcompile.DecodeRequest(contextRequestBytes(t, spec, contextcompile.PhaseDesign, nil))
	if err != nil {
		t.Fatalf("DecodeRequest fixture: %v", err)
	}
	data, err := policyconflict.EncodeRequest(policyconflict.Request{
		Schema: policyconflict.RequestSchema,
		Target: policyconflict.Target{Kind: policyconflict.TargetAcceptedContext, AcceptedContext: &accepted},
	})
	if err != nil {
		t.Fatalf("EncodeRequest fixture: %v", err)
	}
	return data
}

func contextConflictResult(verdict policyconflict.Verdict) policyconflict.Result {
	return policyconflict.Result{
		Report:      policyconflict.Report{Verdict: verdict},
		ReportBytes: []byte("report-" + verdict + "\n"),
	}
}

func contextConflictFixture(t *testing.T) (root, requestPath string) {
	t.Helper()
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	requestPath = writeContextRequestFile(t, repo.Dir, "conflict-request.json", contextConflictRequestBytes(t, "spec/feature-alpha"))
	return repo.Dir, requestPath
}

func TestCmdContextConflictFlagGrammar(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"missing request", nil},
		{"missing request value", []string{"--request"}},
		{"duplicate request", []string{"--request", "a", "--request", "b"}},
		{"duplicate output", []string{"--request", "a", "--out", "b", "--out", "c"}},
		{"unknown flag", []string{"--request", "a", "--actor", "p"}},
		{"challenger flag absent", []string{"--request", "a", "--challenger", "judge"}},
		{"positional", []string{"--request", "a", "extra"}},
		{"empty output", []string{"--request", "a", "--out="}},
		{"dot dot output", []string{"--request", "a", "--out", "notes/../report.json"}},
		{"same request output", []string{"--request", "same.json", "--out", "./same.json"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			var stdout, stderr bytes.Buffer
			called := false
			code := cmdContextConflictWithFactory(tc.args, strings.NewReader(""), &stdout, &stderr,
				func(string, policyconflict.Request) (policyconflict.VerdictProvider, error) {
					called = true
					return nil, errors.New("must not construct")
				})
			if code != 2 {
				t.Fatalf("exit = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("stdout=%q stderr=%q, want empty stdout and diagnostic", stdout.String(), stderr.String())
			}
			if called {
				t.Fatal("provider factory called for invalid grammar")
			}
		})
	}
}

func TestCmdContextConflictExitAndOutputMapping(t *testing.T) {
	root, requestPath := contextConflictFixture(t)
	t.Chdir(root)

	cases := []struct {
		name       string
		result     policyconflict.Result
		err        error
		wantCode   int
		wantOutput string
	}{
		{"pass", contextConflictResult(policyconflict.VerdictPass), nil, 0, "report-pass\n"},
		{"blocked violated", contextConflictResult(policyconflict.VerdictBlockedViolated), nil, 1, "report-blocked-violated\n"},
		{"blocked unproven", contextConflictResult(policyconflict.VerdictBlockedUnproven), nil, 1, "report-blocked-unproven\n"},
		{"not adopted", policyconflict.Result{}, &policyconflict.NotAdoptedError{Err: errors.New("not adopted")}, 1, ""},
		{"operational", policyconflict.Result{}, &policyconflict.OperationalError{Op: "test", Err: errors.New("boom")}, 2, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := cmdContextConflictWithFactory([]string{"--request", requestPath}, strings.NewReader(""), &stdout, &stderr,
				func(_ string, got policyconflict.Request) (policyconflict.VerdictProvider, error) {
					if got.Schema != policyconflict.RequestSchema {
						t.Fatalf("request schema = %q", got.Schema)
					}
					return contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
						return tc.result, tc.err
					}), nil
				})
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q", code, tc.wantCode, stdout.String(), stderr.String())
			}
			if stdout.String() != tc.wantOutput {
				t.Fatalf("stdout = %q, want %q", stdout.String(), tc.wantOutput)
			}
			if tc.err == nil && stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty for a completed report", stderr.String())
			}
			if tc.err != nil && stderr.Len() == 0 {
				t.Fatal("stderr empty for refusal/failure")
			}
		})
	}
}

func TestCmdContextConflictFileStdinAndExplicitOutput(t *testing.T) {
	root, requestPath := contextConflictFixture(t)
	t.Chdir(root)
	want := contextConflictResult(policyconflict.VerdictBlockedUnproven)
	factory := func(string, policyconflict.Request) (policyconflict.VerdictProvider, error) {
		return contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			return want, nil
		}), nil
	}

	for _, tc := range []struct {
		name  string
		args  []string
		stdin []byte
	}{
		{"file to stdout", []string{"--request", requestPath}, nil},
		{"stdin to stdout", []string{"--request", "-"}, contextConflictRequestBytes(t, "spec/feature-alpha")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := cmdContextConflictWithFactory(tc.args, bytes.NewReader(tc.stdin), &stdout, &stderr, factory); code != 1 {
				t.Fatalf("exit = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !bytes.Equal(stdout.Bytes(), want.ReportBytes) || stderr.Len() != 0 {
				t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}

	out := filepath.Join(root, "conflict-report.json")
	var stdout, stderr bytes.Buffer
	if code := cmdContextConflictWithFactory([]string{"--request", requestPath, "--out", out}, strings.NewReader(""), &stdout, &stderr, factory); code != 1 {
		t.Fatalf("--out exit = %d, want 1; stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("--out stdout=%q stderr=%q, want both empty", stdout.String(), stderr.String())
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile(out): %v", err)
	}
	if !bytes.Equal(got, want.ReportBytes) {
		t.Fatalf("out bytes = %q, want %q", got, want.ReportBytes)
	}
}

func TestCmdContextConflictStrictRequestAndOutputFence(t *testing.T) {
	root, requestPath := contextConflictFixture(t)
	t.Chdir(root)
	managedBefore, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read managed projection: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, ".verdi"), filepath.Join(root, "policy-link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	providerCalls := 0
	factory := func(string, policyconflict.Request) (policyconflict.VerdictProvider, error) {
		return contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			providerCalls++
			return contextConflictResult(policyconflict.VerdictPass), nil
		}), nil
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"store destination", []string{"--request", requestPath, "--out", ".verdi/report.json"}},
		{"symlink into store", []string{"--request", requestPath, "--out", "policy-link/report.json"}},
		{"managed projection", []string{"--request", requestPath, "--out", "AGENTS.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := cmdContextConflictWithFactory(tc.args, strings.NewReader(""), &stdout, &stderr, factory); code != 2 {
				t.Fatalf("exit = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
	after, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read managed projection after: %v", err)
	}
	if !bytes.Equal(after, managedBefore) {
		t.Fatal("managed projection changed despite output refusal")
	}
	if providerCalls != 1 {
		// Store and symlink destinations are rejected before evaluation;
		// the managed path set is resolved after the sealed evaluation.
		t.Fatalf("provider calls = %d, want 1", providerCalls)
	}

	malformed := writeContextRequestFile(t, root, "malformed-conflict.json", []byte("{not json"))
	var stdout, stderr bytes.Buffer
	if code := cmdContextConflictWithFactory([]string{"--request", malformed}, strings.NewReader(""), &stdout, &stderr, factory); code != 2 {
		t.Fatalf("malformed exit = %d, want 2", code)
	}
	canonical := contextConflictRequestBytes(t, "spec/feature-alpha")
	noncanonical := append([]byte(" \n"), canonical...)
	noncanonicalPath := writeContextRequestFile(t, root, "noncanonical-conflict.json", noncanonical)
	stdout.Reset()
	stderr.Reset()
	if code := cmdContextConflictWithFactory([]string{"--request", noncanonicalPath}, strings.NewReader(""), &stdout, &stderr, factory); code != 2 {
		t.Fatalf("noncanonical exit = %d, want 2", code)
	}
}

func TestCmdContextConflictNoPartialReportOnFailure(t *testing.T) {
	root, requestPath := contextConflictFixture(t)
	t.Chdir(root)
	out := filepath.Join(root, "report.json")
	var stdout, stderr bytes.Buffer
	code := cmdContextConflictWithFactory([]string{"--request", requestPath, "--out", out}, strings.NewReader(""), &stdout, &stderr,
		func(string, policyconflict.Request) (policyconflict.VerdictProvider, error) {
			return contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
				return policyconflict.Result{}, &policyconflict.OperationalError{Op: "judge", Err: errors.New("failed")}
			}), nil
		})
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("output exists after evaluation failure: %v", err)
	}
}
