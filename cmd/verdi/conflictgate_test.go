package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyconflict"
)

func contextLifecycleRequestFile(t *testing.T, root, name, spec string, phase contextcompile.Phase, expected *contextcompile.Expected) string {
	t.Helper()
	req, err := contextcompile.DecodeRequest(contextRequestBytes(t, spec, phase, nil))
	if err != nil {
		t.Fatalf("DecodeRequest fixture: %v", err)
	}
	req.Expected = expected
	data, err := contextcompile.EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest fixture: %v", err)
	}
	return writeContextRequestFile(t, root, name, data)
}

func adoptedConflictGateRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	return repo.Dir, repo.Head
}

func installConflictPolicyStore(t *testing.T, root string) {
	t.Helper()
	for rel, content := range contextPolicyStoreFiles(t) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir policy parent: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write policy fixture %s: %v", rel, err)
		}
	}
}

func lifecycleConflictResult(verdict policyconflict.Verdict) policyconflict.Result {
	result := contextConflictResult(verdict)
	result.Report.Digest = "sha256:" + strings.Repeat("c", 64)
	if verdict != policyconflict.VerdictPass {
		result.Report.Mechanical = []policyconflict.MechanicalEvaluation{{
			ID: "mechanical-policy-conflict",
			Reasons: []policyconflict.ReasonCode{
				policyconflict.ReasonMechanicalConflict,
			},
		}}
	}
	return result
}

type conflictLifecycleSnapshot struct {
	Branches []byte
	Head     []byte
	Index    []byte
	Worktree []byte
	Status   []byte
	Files    map[string][]byte
}

func takeConflictLifecycleSnapshot(t *testing.T, root string, paths ...string) conflictLifecycleSnapshot {
	t.Helper()
	git := func(args ...string) []byte {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
		return out
	}
	snapshot := conflictLifecycleSnapshot{
		Branches: git("for-each-ref", "--format=%(refname):%(objectname)", "refs/heads"),
		Head:     git("rev-parse", "HEAD"),
		Index:    git("diff", "--cached", "--binary"),
		Worktree: git("diff", "--binary"),
		Status:   git("status", "--porcelain=v1", "-z", "--untracked-files=all"),
		Files:    make(map[string][]byte, len(paths)),
	}
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if errors.Is(err, os.ErrNotExist) {
			snapshot.Files[path] = nil
			continue
		}
		if err != nil {
			t.Fatalf("read snapshot path %s: %v", path, err)
		}
		snapshot.Files[path] = data
	}
	return snapshot
}

func assertConflictLifecycleSnapshot(t *testing.T, root string, before conflictLifecycleSnapshot) {
	t.Helper()
	paths := make([]string, 0, len(before.Files))
	for path := range before.Files {
		paths = append(paths, path)
	}
	after := takeConflictLifecycleSnapshot(t, root, paths...)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("repository changed across conflict refusal\nbefore=%+v\nafter=%+v", before, after)
	}
}

// TestConflictGateRequestGrammar catches a lifecycle command consuming the
// flag only in one position, accepting two request sources, or treating a
// missing value/stdin as an ordinary positional operand.
func TestConflictGateRequestGrammar(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
		wantRest []string
		wantErr  bool
	}{
		{name: "absent", args: []string{"spec/feature-alpha"}, wantRest: []string{"spec/feature-alpha"}},
		{name: "first", args: []string{"--context-request", "request.json", "spec/feature-alpha"}, wantPath: "request.json", wantRest: []string{"spec/feature-alpha"}},
		{name: "middle", args: []string{"--prepare", "--context-request", "request.json", "spec/feature-alpha"}, wantPath: "request.json", wantRest: []string{"--prepare", "spec/feature-alpha"}},
		{name: "last", args: []string{"spec/feature-alpha", "--context-request", "request.json"}, wantPath: "request.json", wantRest: []string{"spec/feature-alpha"}},
		{name: "duplicate", args: []string{"--context-request", "one.json", "--context-request", "two.json"}, wantErr: true},
		{name: "missing value", args: []string{"--context-request"}, wantErr: true},
		{name: "next flag is not a value", args: []string{"--context-request", "--prepare", "spec/feature-alpha"}, wantErr: true},
		{name: "stdin forbidden", args: []string{"--context-request", "-", "spec/feature-alpha"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotRest, err := extractConflictRequestFlag(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("extractConflictRequestFlag(%q) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if gotPath != tt.wantPath || !reflect.DeepEqual(gotRest, tt.wantRest) {
				t.Fatalf("extractConflictRequestFlag(%q) = (%q, %q), want (%q, %q)", tt.args, gotPath, gotRest, tt.wantPath, tt.wantRest)
			}
		})
	}
}

// TestConflictGateAdoption catches either side of the compatibility break:
// reading operands in a legacy checkout, or letting an adopted checkout run
// without the explicit existing context request.
func TestConflictGateAdoption(t *testing.T) {
	t.Run("legacy without flag does not read operands or call provider", func(t *testing.T) {
		root := t.TempDir()
		called := false
		provider := contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			called = true
			return contextConflictResult(policyconflict.VerdictPass), nil
		})
		got, err := runConflictGate(context.Background(), root, conflictGateInput{
			Phase: contextcompile.Phase("not-a-phase"), Spec: "not a ref", Branch: "", Head: "",
		}, provider)
		if err != nil || got.Adopted {
			t.Fatalf("runConflictGate legacy = (%+v, %v), want Adopted=false and nil error", got, err)
		}
		if called {
			t.Fatal("provider called before adoption")
		}
	})

	t.Run("legacy with flag is misuse before file read", func(t *testing.T) {
		root := t.TempDir()
		missing := filepath.Join(root, "does-not-exist.json")
		_, err := runConflictGate(context.Background(), root, conflictGateInput{RequestPath: missing}, nil)
		if err == nil || strings.Contains(err.Error(), "no such file") {
			t.Fatalf("runConflictGate legacy flag error = %v, want adoption misuse before a file read", err)
		}
	})

	t.Run("adopted without flag refuses before provider", func(t *testing.T) {
		root, head := adoptedConflictGateRepo(t)
		called := false
		provider := contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			called = true
			return contextConflictResult(policyconflict.VerdictPass), nil
		})
		_, err := runConflictGate(context.Background(), root, conflictGateInput{
			Phase: contextcompile.PhaseDesign, Spec: "spec/feature-alpha", Branch: "main", Head: head,
		}, provider)
		if err == nil || !strings.Contains(err.Error(), "--context-request") {
			t.Fatalf("runConflictGate adopted without flag error = %v, want missing flag", err)
		}
		if called {
			t.Fatal("provider called without required adopted request")
		}
	})
}

// TestConflictGateRequestValidation catches any permissive read/decode path,
// any caller claim replacing lifecycle facts, and any symlink-following read.
func TestConflictGateRequestValidation(t *testing.T) {
	root, head := adoptedConflictGateRepo(t)
	valid := contextLifecycleRequestFile(t, root, "valid.json", "spec/feature-alpha", contextcompile.PhaseDesign, nil)

	tests := []struct {
		name      string
		path      string
		phase     contextcompile.Phase
		spec      string
		branch    string
		head      string
		wantError string
	}{
		{name: "valid", path: valid, phase: contextcompile.PhaseDesign, spec: "spec/feature-alpha", branch: "design/feature-alpha", head: head},
		{name: "malformed", path: writeContextRequestFile(t, root, "malformed.json", []byte("{not-json\n")), phase: contextcompile.PhaseDesign, spec: "spec/feature-alpha", branch: "design/feature-alpha", head: head, wantError: "decoding request"},
		{name: "noncanonical", path: writeContextRequestFile(t, root, "noncanonical.json", append([]byte(" "), contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil)...)), phase: contextcompile.PhaseDesign, spec: "spec/feature-alpha", branch: "design/feature-alpha", head: head, wantError: "canonical"},
		{name: "phase mismatch", path: valid, phase: contextcompile.PhaseBuild, spec: "spec/feature-alpha", branch: "main", head: head, wantError: "phase"},
		{name: "spec mismatch", path: valid, phase: contextcompile.PhaseDesign, spec: "spec/other", branch: "design/feature-alpha", head: head, wantError: "spec"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			provider := contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
				calls++
				return contextConflictResult(policyconflict.VerdictPass), nil
			})
			_, err := runConflictGate(context.Background(), root, conflictGateInput{
				RequestPath: tt.path, Phase: tt.phase, Spec: tt.spec, Candidate: true, Branch: tt.branch, Head: tt.head,
			}, provider)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("runConflictGate: %v", err)
				}
				if calls != 1 {
					t.Fatalf("provider calls = %d, want 1", calls)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("runConflictGate error = %v, want substring %q", err, tt.wantError)
			}
			if calls != 0 {
				t.Fatalf("provider calls = %d after invalid request, want 0", calls)
			}
		})
	}

	t.Run("optional expected mismatch", func(t *testing.T) {
		mismatched := contextLifecycleRequestFile(t, root, "mismatch.json", "spec/feature-alpha", contextcompile.PhaseDesign, &contextcompile.Expected{
			Branch: "design/other", Head: strings.Repeat("a", 40),
		})
		_, err := runConflictGate(context.Background(), root, conflictGateInput{
			RequestPath: mismatched, Phase: contextcompile.PhaseDesign, Spec: "spec/feature-alpha", Candidate: true, Branch: "design/feature-alpha", Head: head,
		}, contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			t.Fatal("provider called after expected mismatch")
			return policyconflict.Result{}, nil
		}))
		if err == nil || !strings.Contains(err.Error(), "expected") {
			t.Fatalf("runConflictGate error = %v, want expected mismatch", err)
		}
	})

	t.Run("symlink request", func(t *testing.T) {
		link := filepath.Join(root, "request-link.json")
		if err := os.Symlink(valid, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		calls := 0
		_, err := runConflictGate(context.Background(), root, conflictGateInput{
			RequestPath: link, Phase: contextcompile.PhaseDesign, Spec: "spec/feature-alpha", Candidate: true, Branch: "design/feature-alpha", Head: head,
		}, contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			calls++
			return contextConflictResult(policyconflict.VerdictPass), nil
		}))
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("runConflictGate symlink error = %v, want symlink refusal", err)
		}
		if calls != 0 {
			t.Fatalf("provider calls = %d after symlink refusal, want 0", calls)
		}
	})

	t.Run("symlink ancestor", func(t *testing.T) {
		dir := filepath.Join(root, "request-dir")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		request := contextLifecycleRequestFile(t, dir, "request.json", "spec/feature-alpha", contextcompile.PhaseDesign, nil)
		link := filepath.Join(root, "request-dir-link")
		if err := os.Symlink(dir, link); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		calls := 0
		_, err := runConflictGate(context.Background(), root, conflictGateInput{
			RequestPath: filepath.Join(link, filepath.Base(request)), Phase: contextcompile.PhaseDesign, Spec: "spec/feature-alpha", Candidate: true, Branch: "design/feature-alpha", Head: head,
		}, contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			calls++
			return contextConflictResult(policyconflict.VerdictPass), nil
		}))
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("runConflictGate symlink-ancestor error = %v, want symlink refusal", err)
		}
		if calls != 0 {
			t.Fatalf("provider calls = %d after symlink-ancestor refusal, want 0", calls)
		}
	})

	t.Run("system var alias", func(t *testing.T) {
		resolvedRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatalf("EvalSymlinks(%q): %v", root, err)
		}
		if resolvedRoot == root {
			t.Skip("checkout root has no system path alias")
		}
		rel, err := filepath.Rel(root, valid)
		if err != nil {
			t.Fatalf("Rel(%q, %q): %v", root, valid, err)
		}
		calls := 0
		_, err = runConflictGate(context.Background(), resolvedRoot, conflictGateInput{
			RequestPath: valid, Phase: contextcompile.PhaseDesign, Spec: "spec/feature-alpha", Candidate: true, Branch: "design/feature-alpha", Head: head,
		}, contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			calls++
			return contextConflictResult(policyconflict.VerdictPass), nil
		}))
		if err != nil {
			t.Fatalf("runConflictGate root alias %q request alias %q: %v", resolvedRoot, filepath.Join(root, rel), err)
		}
		if calls != 1 {
			t.Fatalf("provider calls = %d through system path alias, want 1", calls)
		}
	})

	t.Run("unreadable request", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("chmod permissions differ on windows")
		}
		path := contextLifecycleRequestFile(t, root, "unreadable.json", "spec/feature-alpha", contextcompile.PhaseDesign, nil)
		if err := os.Chmod(path, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
		_, err := runConflictGate(context.Background(), root, conflictGateInput{
			RequestPath: path, Phase: contextcompile.PhaseDesign, Spec: "spec/feature-alpha", Candidate: true, Branch: "design/feature-alpha", Head: head,
		}, nil)
		if err == nil || !strings.Contains(err.Error(), "reading") {
			t.Fatalf("runConflictGate unreadable error = %v, want read refusal", err)
		}
	})
}

// TestConflictGateRequestPathIdentity catches validation and reading using
// different pathname identities. filepath.Abs cleans "symlink-dir/.."
// lexically, while the kernel follows symlink-dir before applying "..". A
// request path containing that sequence must fail operationally before the
// provider is constructed or called, never decode an external request file.
func TestConflictGateRequestPathIdentity(t *testing.T) {
	root, _ := adoptedConflictGateRepo(t)
	writeContextRequestFile(t, root, "request.json", []byte("{not-the-request\n"))

	externalParent := t.TempDir()
	externalChild := filepath.Join(externalParent, "child")
	if err := os.Mkdir(externalChild, 0o755); err != nil {
		t.Fatal(err)
	}
	contextLifecycleRequestFile(t, externalParent, "request.json", "spec/feature-alpha", contextcompile.PhaseReview, nil)
	link := filepath.Join(root, "symlink-dir")
	if err := os.Symlink(externalChild, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	// filepath.Join would erase the traversal element this regression needs.
	requestPath := root + string(filepath.Separator) + filepath.FromSlash("symlink-dir/../request.json")
	calls := 0
	provider := contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
		calls++
		return contextConflictResult(policyconflict.VerdictPass), nil
	})
	var stdout, stderr bytes.Buffer
	got := runCloseConflictGate(context.Background(), root, "spec/feature-alpha", requestPath, provider, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runCloseConflictGate = %d, want operational exit 2; stdout=%q stderr=%q", got, stdout.String(), stderr.String())
	}
	if calls != 0 {
		t.Fatalf("provider calls = %d after path-identity refusal, want 0", calls)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), `".." path element`) {
		t.Fatalf("stdout=%q stderr=%q, want path-safe operational refusal", stdout.String(), stderr.String())
	}
}

// TestConflictGateTarget catches drift among lifecycle consumers: design must
// build the candidate union arm, while build/review must preserve the decoded
// request as the accepted arm after binding computed branch and HEAD.
func TestConflictGateTarget(t *testing.T) {
	root, head := adoptedConflictGateRepo(t)
	tests := []struct {
		name      string
		phase     contextcompile.Phase
		candidate bool
		wantKind  policyconflict.TargetKind
	}{
		{name: "design candidate", phase: contextcompile.PhaseDesign, candidate: true, wantKind: policyconflict.TargetAcceptanceCandidate},
		{name: "build accepted", phase: contextcompile.PhaseBuild, wantKind: policyconflict.TargetAcceptedContext},
		{name: "review accepted", phase: contextcompile.PhaseReview, wantKind: policyconflict.TargetAcceptedContext},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := contextLifecycleRequestFile(t, root, strings.ReplaceAll(tt.name, " ", "-")+".json", "spec/feature-alpha", tt.phase, nil)
			var captured policyconflict.Request
			provider := contextConflictProviderFunc(func(_ context.Context, req policyconflict.Request) (policyconflict.Result, error) {
				captured = req
				return contextConflictResult(policyconflict.VerdictPass), nil
			})
			got, err := runConflictGate(context.Background(), root, conflictGateInput{
				RequestPath: path,
				Phase:       tt.phase,
				Spec:        "spec/feature-alpha",
				Candidate:   tt.candidate,
				Branch:      "topic/feature-alpha",
				Head:        head,
			}, provider)
			if err != nil {
				t.Fatalf("runConflictGate: %v", err)
			}
			if !got.Adopted || got.Result.Report.Verdict != policyconflict.VerdictPass {
				t.Fatalf("runConflictGate result = %+v", got)
			}
			if captured.Target.Kind != tt.wantKind {
				t.Fatalf("target kind = %q, want %q", captured.Target.Kind, tt.wantKind)
			}
			wantExpected := contextcompile.Expected{Branch: "topic/feature-alpha", Head: head}
			switch tt.wantKind {
			case policyconflict.TargetAcceptanceCandidate:
				if captured.Target.AcceptanceCandidate == nil || captured.Target.AcceptedContext != nil {
					t.Fatalf("candidate target arms = %+v", captured.Target)
				}
				if captured.Target.AcceptanceCandidate.Expected != wantExpected || captured.Target.AcceptanceCandidate.Spec != "spec/feature-alpha" {
					t.Fatalf("candidate = %+v, want computed expected %+v and exact spec", captured.Target.AcceptanceCandidate, wantExpected)
				}
			case policyconflict.TargetAcceptedContext:
				if captured.Target.AcceptedContext == nil || captured.Target.AcceptanceCandidate != nil {
					t.Fatalf("accepted target arms = %+v", captured.Target)
				}
				if captured.Target.AcceptedContext.Expected == nil || *captured.Target.AcceptedContext.Expected != wantExpected {
					t.Fatalf("accepted expected = %+v, want computed %+v", captured.Target.AcceptedContext.Expected, wantExpected)
				}
				if captured.Target.AcceptedContext.Phase != tt.phase || captured.Target.AcceptedContext.Spec != "spec/feature-alpha" {
					t.Fatalf("accepted request = %+v", captured.Target.AcceptedContext)
				}
			}
		})
	}
}

// TestLifecycleConflictBuiltBinary proves that the real close dispatcher
// threads the one request adapter through each review-mode entry point. The
// real provider's conservative verdict must arrive before any close effect.
func TestLifecycleConflictBuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	tests := []struct {
		name string
		args func(string) []string
	}{
		{name: "close", args: func(path string) []string {
			return []string{"close", "--force-local", "spec/feature-alpha", "--context-request", path}
		}},
		{name: "preflight", args: func(path string) []string {
			return []string{"close", "--preflight", "--context-request", path, "spec/feature-alpha"}
		}},
		{name: "prepare", args: func(path string) []string {
			return []string{"close", "--context-request", path, "--prepare", "spec/feature-alpha"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := buildContextCompileRepo(t, map[string]string{
				".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
			})
			requestPath := contextLifecycleRequestFile(t, repo.Dir, "review-context.json", "spec/feature-alpha", contextcompile.PhaseReview, nil)
			before := takeConflictLifecycleSnapshot(t, repo.Dir,
				".verdi/specs/active/feature-alpha/spec.md",
				".verdi/specs/active/feature-alpha/deviation-report.md",
				".verdi/specs/active/feature-alpha/rollup.json",
				".verdi/specs/archive/feature-alpha/spec.md",
				"AGENTS.md",
			)

			stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, tt.args(requestPath)...)
			if code != 1 {
				t.Fatalf("exit=%d, want conflict verdict exit 1; stdout=%s stderr=%s", code, stdout, stderr)
			}
			if !strings.Contains(stdout, "constitutional conflict: state: blocked-unproven") {
				t.Fatalf("stdout=%q, want closed conflict summary", stdout)
			}
			assertConflictLifecycleSnapshot(t, repo.Dir, before)
		})
	}
}

func TestConflictGateTargetProviderError(t *testing.T) {
	root, head := adoptedConflictGateRepo(t)
	path := contextLifecycleRequestFile(t, root, "provider-error.json", "spec/feature-alpha", contextcompile.PhaseBuild, nil)
	want := errors.New("provider failed")
	_, err := runConflictGate(context.Background(), root, conflictGateInput{
		RequestPath: path, Phase: contextcompile.PhaseBuild, Spec: "spec/feature-alpha", Branch: "main", Head: head,
	}, contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
		return policyconflict.Result{}, want
	}))
	if !errors.Is(err, want) {
		t.Fatalf("runConflictGate error = %v, want wrapped provider error", err)
	}
}

// TestConflictGateSummaryRender catches a lifecycle consumer leaking the full
// report or dropping a closed reason/witness while translating the one result.
func TestConflictGateSummaryRender(t *testing.T) {
	result := policyconflict.Result{Report: policyconflict.Report{
		Verdict: policyconflict.VerdictBlockedViolated,
		Digest:  "sha256:" + strings.Repeat("d", 64),
		Mechanical: []policyconflict.MechanicalEvaluation{{
			ID: "mechanical-z", Reasons: []policyconflict.ReasonCode{policyconflict.ReasonMechanicalConflict},
		}},
		Semantic: []policyconflict.SemanticEvaluation{{
			ID: "semantic-a", Reasons: []policyconflict.ReasonCode{policyconflict.ReasonDispositionRequired},
		}},
		Disclosures: []policyconflict.Disclosure{{
			Code: contextcompile.DisclosureApplicabilityUnknown, Witnesses: []string{"source-b", "source-a"},
		}},
	}}

	condition := conflictCondition(result)
	if condition.Name != "constitutional conflict verdict" || condition.OK || condition.Reason != "state: blocked-violated" {
		t.Fatalf("condition = %+v", condition)
	}
	wantExtra := []string{
		"       report digest: " + result.Report.Digest,
		"       reasons: [applicability-unknown disposition-required mechanical-conflict]",
		"       witness IDs: [mechanical-z semantic-a source-a source-b]",
	}
	if !reflect.DeepEqual(condition.Extra, wantExtra) {
		t.Fatalf("condition extra = %q, want %q", condition.Extra, wantExtra)
	}

	var out bytes.Buffer
	renderConflictSummary(&out, result)
	want := "constitutional conflict: state: blocked-violated\n" +
		"constitutional conflict: report digest: " + result.Report.Digest + "\n" +
		"constitutional conflict: reasons: [applicability-unknown disposition-required mechanical-conflict]\n" +
		"constitutional conflict: witness IDs: [mechanical-z semantic-a source-a source-b]\n"
	if out.String() != want {
		t.Fatalf("summary = %q, want %q", out.String(), want)
	}
	for _, forbidden := range []string{"accepted-context", "policy_entries", "raw_result", "claim_digest"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("summary leaked full report field %q: %q", forbidden, out.String())
		}
	}
}

func TestConflictGateSummaryPass(t *testing.T) {
	result := policyconflict.Result{Report: policyconflict.Report{Verdict: policyconflict.VerdictPass, Digest: "sha256:" + strings.Repeat("e", 64)}}
	condition := conflictCondition(result)
	if !condition.OK || condition.Reason != "" {
		t.Fatalf("pass condition = %+v", condition)
	}
	want := []string{"       state: pass", "       report digest: " + result.Report.Digest}
	if !reflect.DeepEqual(condition.Extra, want) {
		t.Fatalf("pass condition extra = %q, want %q", condition.Extra, want)
	}
}
