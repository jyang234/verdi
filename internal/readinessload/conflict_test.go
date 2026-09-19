// conflict_test.go directly exercises the moved provider construction
// (NewConflictProvider), the moved path safety check
// (ValidatedContextRequestPath), and this package's own ContextRequestSpec
// helper — independent of Load's own ordering, since cmd/verdi's serve.go
// and context_conflict.go call these directly.
package readinessload

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// policyartifactUniversalScope builds an explicitly empty (universal) scope
// — mirrors requestBytes's own nil-means-universal scope handling, for
// callers that construct a policyconflict.Request by hand rather than
// through contextcompile.EncodeRequest.
func policyartifactUniversalScope(t *testing.T) policyartifact.Scope {
	t.Helper()
	return policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
}

func TestValidatedContextRequestPath_RefusesDotDotAndSymlink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "request.json")
	if err := os.WriteFile(real, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.json")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	if _, err := ValidatedContextRequestPath(root, root+string(filepath.Separator)+".."+string(filepath.Separator)+"request.json"); err == nil || !strings.Contains(err.Error(), `".."`) {
		t.Fatalf("error = %v, want a \"..\" refusal", err)
	}
	if _, err := ValidatedContextRequestPath(root, link); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %v, want a symlink refusal", err)
	}
	if got, err := ValidatedContextRequestPath(root, real); err != nil || got != real {
		t.Fatalf("ValidatedContextRequestPath(%q) = (%q, %v), want (%q, nil)", real, got, err, real)
	}
}

func TestContextRequestSpec_ReportsTheDeclaredTarget(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	requestPath := writeRequestFile(t, repo.Dir, "request.json", requestBytes(t, ref, contextcompile.PhaseDesign))

	got, err := ContextRequestSpec(repo.Dir, requestPath)
	if err != nil {
		t.Fatalf("ContextRequestSpec: %v", err)
	}
	if got != ref {
		t.Fatalf("ContextRequestSpec = %q, want %q", got, ref)
	}
}

func TestContextRequestSpec_RefusesStdin(t *testing.T) {
	if _, err := ContextRequestSpec(t.TempDir(), "-"); err == nil || !strings.Contains(err.Error(), "stdin") {
		t.Fatalf("error = %v, want a stdin refusal", err)
	}
}

func TestContextRequestSpec_NeverReadsPastAPathRefusal(t *testing.T) {
	root := t.TempDir()
	// filepath.Join would lexically collapse "..", so the literal element
	// is built by string concatenation instead — the same technique
	// load_test.go's own ".."/symlink pin uses.
	traversal := root + string(filepath.Separator) + "nested" + string(filepath.Separator) + ".." + string(filepath.Separator) + "request.json"
	if _, err := ContextRequestSpec(root, traversal); err == nil || !strings.Contains(err.Error(), `".."`) {
		t.Fatalf("error = %v, want a \"..\" refusal", err)
	}
}

func TestNewConflictProvider_JudgeRunLaunchesOnMiss(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	ran := filepath.Join(t.TempDir(), "ran")
	judge := writeJudgeScript(t, "printf x >> "+ran+" && printf '%s\\n' '"+noConflictJudgeResult+"'")
	configureJudge(t, repo, judge)
	checkoutBranch(t, repo.Dir, "design/feature-alpha")

	request := policyconflict.Request{
		Schema: policyconflict.RequestSchema,
		Target: policyconflict.Target{
			Kind: policyconflict.TargetAcceptanceCandidate,
			AcceptanceCandidate: &policyconflict.AcceptanceCandidate{
				Adapter:  contextcompile.AdapterRef{ID: "codex", Version: "1"},
				Expected: contextcompile.Expected{Branch: "design/feature-alpha", Head: repo.Head},
				Scope:    policyartifactUniversalScope(t),
				Spec:     ref,
			},
		},
	}
	provider, err := NewConflictProvider(context.Background(), repo.Dir, request, JudgeRun, nil)
	if err != nil {
		t.Fatalf("NewConflictProvider: %v", err)
	}
	if _, err := provider.Evaluate(context.Background(), request); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	data, rerr := os.ReadFile(ran)
	if rerr != nil || len(data) != 1 {
		t.Fatalf("judge run marker = %q (err=%v), want exactly one run under JudgeRun", data, rerr)
	}
}

func TestNewConflictProvider_JudgeCacheOnlyNeverLaunches(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	judge := writeJudgeScript(t, "echo must-not-run-under-cache-only >&2; exit 1")
	configureJudge(t, repo, judge)
	checkoutBranch(t, repo.Dir, "design/feature-alpha")

	request := policyconflict.Request{
		Schema: policyconflict.RequestSchema,
		Target: policyconflict.Target{
			Kind: policyconflict.TargetAcceptanceCandidate,
			AcceptanceCandidate: &policyconflict.AcceptanceCandidate{
				Adapter:  contextcompile.AdapterRef{ID: "codex", Version: "1"},
				Expected: contextcompile.Expected{Branch: "design/feature-alpha", Head: repo.Head},
				Scope:    policyartifactUniversalScope(t),
				Spec:     ref,
			},
		},
	}
	provider, err := NewConflictProvider(context.Background(), repo.Dir, request, JudgeCacheOnly, nil)
	if err != nil {
		t.Fatalf("NewConflictProvider: %v", err)
	}
	// A miss resolves the semantic row unproven/judge-unavailable rather
	// than failing Evaluate — this proves the judge script (which would
	// fail the process if run) was never launched.
	result, err := provider.Evaluate(context.Background(), request)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(result.Report.Semantic) != 1 || result.Report.Semantic[0].Primary != nil {
		t.Fatalf("semantic rows = %+v, want one row with no primary exchange (judge never ran)", result.Report.Semantic)
	}
}
