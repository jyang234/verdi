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
	"reflect"
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
	data := requestBytes(t, ref, contextcompile.PhaseDesign)
	requestPath := writeRequestFile(t, repo.Dir, "request.json", data)

	got, predecoded, err := ContextRequestSpec(repo.Dir, requestPath)
	if err != nil {
		t.Fatalf("ContextRequestSpec: %v", err)
	}
	if got != ref {
		t.Fatalf("ContextRequestSpec ref = %q, want %q", got, ref)
	}
	if predecoded == nil {
		t.Fatal("ContextRequestSpec predecoded = nil, want a bundle")
	}
	if !reflect.DeepEqual(predecoded.Bytes, data) {
		t.Fatalf("predecoded.Bytes = %q, want the exact canonical request bytes %q", predecoded.Bytes, data)
	}
	if predecoded.Request.Spec != ref {
		t.Fatalf("predecoded.Request.Spec = %q, want %q", predecoded.Request.Spec, ref)
	}
}

func TestContextRequestSpec_RefusesStdin(t *testing.T) {
	if _, _, err := ContextRequestSpec(t.TempDir(), "-"); err == nil || !strings.Contains(err.Error(), "stdin") {
		t.Fatalf("error = %v, want a stdin refusal", err)
	}
}

func TestContextRequestSpec_NeverReadsPastAPathRefusal(t *testing.T) {
	root := t.TempDir()
	// filepath.Join would lexically collapse "..", so the literal element
	// is built by string concatenation instead — the same technique
	// load_test.go's own ".."/symlink pin uses.
	traversal := root + string(filepath.Separator) + "nested" + string(filepath.Separator) + ".." + string(filepath.Separator) + "request.json"
	if _, _, err := ContextRequestSpec(root, traversal); err == nil || !strings.Contains(err.Error(), `".."`) {
		t.Fatalf("error = %v, want a \"..\" refusal", err)
	}
}

// TestContextRequestSpec_PredecodedRequestAvoidsASecondRead re-pins fix
// round 1's Minor 6 fix: Load, given the Predecoded bundle ContextRequestSpec
// already read once, does not read the file again. It corrupts the file on
// disk immediately after the one read ContextRequestSpec performs — if Load
// re-read it, decoding would fail; instead Load succeeds and its
// RequestDigest is the digest of the ORIGINAL (predecoded) bytes, proving
// the corrupted on-disk bytes were never touched.
func TestContextRequestSpec_PredecodedRequestAvoidsASecondRead(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	checkoutBranch(t, repo.Dir, "design/feature-alpha")
	requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))

	gotRef, predecoded, err := ContextRequestSpec(repo.Dir, requestPath)
	if err != nil {
		t.Fatalf("ContextRequestSpec: %v", err)
	}
	if gotRef != ref {
		t.Fatalf("ContextRequestSpec ref = %q, want %q", gotRef, ref)
	}

	if err := os.WriteFile(requestPath, []byte("not json, and not even the same length"), 0o644); err != nil {
		t.Fatal(err)
	}

	fakeProvider := func(_ context.Context, root string, request policyconflict.Request) (policyconflict.VerdictProvider, error) {
		return providerFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			return fixtureReport(t, root, request, policyconflict.VerdictPass, nil), nil
		}), nil
	}
	snap, err := Load(context.Background(), repo.Dir, ref, Options{
		ContextRequestPath: requestPath, PredecodedRequest: predecoded, ConflictProvider: fakeProvider,
	})
	if err != nil {
		t.Fatalf("Load with PredecodedRequest after the file was corrupted on disk: %v", err)
	}
	if snap.RequestDigest != testDigest(predecoded.Bytes) {
		t.Fatalf("RequestDigest = %q, want the digest of the predecoded bytes %q", snap.RequestDigest, testDigest(predecoded.Bytes))
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
