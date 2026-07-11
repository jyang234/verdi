package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/forge"
	forgefake "github.com/OWNER/verdi/internal/forge/fake"
)

func TestCheckReviewThreadsCondition_NilForge_Disclosed(t *testing.T) {
	cond, err := checkReviewThreadsCondition(context.Background(), nil, "main", "design/stale-decline")
	if err != nil {
		t.Fatalf("checkReviewThreadsCondition: %v", err)
	}
	if !cond.Disclosed {
		t.Fatal("Disclosed = false, want true (nil forge — constitution 2/10: silence is never a pass)")
	}
	if cond.OK {
		t.Fatal("OK = true, want false: a disclosed condition is neither a pass nor a fail")
	}
}

func TestCheckReviewThreadsCondition_NoOpenMR_PassesTrivially(t *testing.T) {
	f := forgefake.New()
	// No MR seeded at all: nothing to prove — no MR means no review
	// threads exist yet (mirrors closuregate.go's "nothing to implement,
	// nothing to prove" trivial pass).
	cond, err := checkReviewThreadsCondition(context.Background(), f, "main", "design/stale-decline")
	if err != nil {
		t.Fatalf("checkReviewThreadsCondition: %v", err)
	}
	if cond.Disclosed {
		t.Fatal("Disclosed = true, want false: a reachable forge with no matching MR is a genuine pass, not a disclosure")
	}
	if !cond.OK {
		t.Fatalf("OK = false (%s), want true (no open MR for this branch)", cond.Reason)
	}
}

func TestCheckReviewThreadsCondition_UnresolvedThreadFails(t *testing.T) {
	f := forgefake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "7", SourceBranch: "design/stale-decline", Title: "Stale decline"})
	f.SeedComment("7", forge.Comment{ID: "c1", ThreadID: "t1", Body: "[vd:ac-2] outcome AC reads implementation-scoped — reword?"})

	cond, err := checkReviewThreadsCondition(context.Background(), f, "main", "design/stale-decline")
	if err != nil {
		t.Fatalf("checkReviewThreadsCondition: %v", err)
	}
	if cond.OK {
		t.Fatal("OK = true, want false (thread t1 is unresolved)")
	}
	if !strings.Contains(cond.Reason, "t1") {
		t.Errorf("Reason = %q, want it to name the unresolved thread t1", cond.Reason)
	}
}

func TestCheckReviewThreadsCondition_AllResolvedPasses(t *testing.T) {
	f := forgefake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "7", SourceBranch: "design/stale-decline", Title: "Stale decline"})
	f.SeedComment("7", forge.Comment{ID: "c1", ThreadID: "t1", Body: "[vd:ac-2] outcome AC reads implementation-scoped — reword?"})
	f.SeedThreadResolution("7", forge.ThreadResolution{ThreadID: "t1", Resolved: true, ResolvedBy: "reviewer"})

	cond, err := checkReviewThreadsCondition(context.Background(), f, "main", "design/stale-decline")
	if err != nil {
		t.Fatalf("checkReviewThreadsCondition: %v", err)
	}
	if !cond.OK {
		t.Fatalf("OK = false (%s), want true (every thread resolved)", cond.Reason)
	}
}

// TestCheckReviewThreadsCondition_GeneralCommentNeverBlocks proves a
// token-free general/conversation comment (no ThreadID — belongs to no
// substantive/resolvable thread at all) never blocks the gate: it is
// inbox-tray material, not a review thread 05's readiness rule governs.
func TestCheckReviewThreadsCondition_GeneralCommentNeverBlocks(t *testing.T) {
	f := forgefake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "7", SourceBranch: "design/stale-decline", Title: "Stale decline"})
	f.SeedComment("7", forge.Comment{ID: "c1", Body: "nit: general conversation, no vd token, no thread at all"})

	cond, err := checkReviewThreadsCondition(context.Background(), f, "main", "design/stale-decline")
	if err != nil {
		t.Fatalf("checkReviewThreadsCondition: %v", err)
	}
	if !cond.OK {
		t.Fatalf("OK = false (%s), want true (a general comment is not a substantive thread)", cond.Reason)
	}
}

// TestSpecMRGate_ReviewThreads_UnresolvedBlocks proves runSpecMRGate
// itself (not just the condition function in isolation) fails the gate
// when an injected forge reports an unresolved substantive thread, even
// though the declared-decision-conflict condition passes.
func TestSpecMRGate_ReviewThreads_UnresolvedBlocks(t *testing.T) {
	repo := buildDesignGateRepo(t)
	writeDecisionConflictReport(t, repo.Dir, repo.Head,
		"  - { id: f-1, kind: computed, text: \"exempts edge to adr/decline-policy\", disposition: exempt, note: \"excused, see witness\" }\n")

	f := forgefake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "9", SourceBranch: "design/stale-decline", Title: "Stale decline"})
	f.SeedComment("9", forge.Comment{ID: "c1", ThreadID: "t1", Body: "[vd:ac-2] reword?"})

	var stdout, stderr bytes.Buffer
	got := runSpecMRGate(context.Background(), repo.Dir, "design/stale-decline", f, "main", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runSpecMRGate = %d, want 1; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "review threads resolved") {
		t.Fatalf("stdout = %q, want it to name the review-thread condition", stdout.String())
	}
	if !strings.Contains(stdout.String(), "gate: FAIL") {
		t.Fatalf("stdout = %q, want a final gate: FAIL line", stdout.String())
	}
}

// TestSpecMRGate_ReviewThreads_ResolvedPasses proves runSpecMRGate passes
// once both spec-MR conditions clear: declared decision conflicts
// dispositioned AND every substantive review thread resolved.
func TestSpecMRGate_ReviewThreads_ResolvedPasses(t *testing.T) {
	repo := buildDesignGateRepo(t)
	writeDecisionConflictReport(t, repo.Dir, repo.Head,
		"  - { id: f-1, kind: computed, text: \"exempts edge to adr/decline-policy\", disposition: exempt, note: \"excused, see witness\" }\n")

	f := forgefake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "9", SourceBranch: "design/stale-decline", Title: "Stale decline"})
	f.SeedComment("9", forge.Comment{ID: "c1", ThreadID: "t1", Body: "[vd:ac-2] reword?"})
	f.SeedThreadResolution("9", forge.ThreadResolution{ThreadID: "t1", Resolved: true, ResolvedBy: "reviewer"})

	var stdout, stderr bytes.Buffer
	got := runSpecMRGate(context.Background(), repo.Dir, "design/stale-decline", f, "main", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runSpecMRGate = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "gate: PASS") {
		t.Fatalf("stdout = %q, want a final gate: PASS line", stdout.String())
	}
}

func TestBuildForgeBestEffort_NoManifest_ReturnsNil(t *testing.T) {
	root := t.TempDir()
	if f := buildForgeBestEffort(context.Background(), root); f != nil {
		t.Fatalf("buildForgeBestEffort with no verdi.yaml = %v, want nil", f)
	}
}

// TestForgeBestEffort_BareFixtureRepo_YieldsNil is the R4-I-28 hermeticity
// fence (I-1(a)): a fixture repo that NAMES a forge in verdi.yaml but
// exports no credentials must still yield a nil live forge — so `go test`
// never dials out — while reporting the configured kind so the read
// surfaces can disclose (I-1(b)). Credentials are explicitly cleared so
// the assertion holds even inside this project's own CI.
func TestForgeBestEffort_BareFixtureRepo_YieldsNil(t *testing.T) {
	t.Setenv("CI_PROJECT_ID", "")
	t.Setenv("CI_JOB_TOKEN", "")
	t.Setenv("GITHUB_REPOSITORY", "")
	t.Setenv("GITHUB_TOKEN", "")

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\nforge: gitlab\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, kind := forgeBestEffort(context.Background(), root)
	if f != nil {
		t.Fatalf("forgeBestEffort on a bare fixture repo = %v, want nil live forge (no credentials — go test must never dial out)", f)
	}
	if kind != "gitlab" {
		t.Fatalf("configuredKind = %q, want gitlab (a forge IS named in verdi.yaml, for I-1(b) disclosure)", kind)
	}
}

func TestForgeCredentialsPresent(t *testing.T) {
	t.Setenv("CI_PROJECT_ID", "")
	t.Setenv("CI_JOB_TOKEN", "")
	t.Setenv("GITHUB_REPOSITORY", "")
	t.Setenv("GITHUB_TOKEN", "")
	if forgeCredentialsPresent("gitlab") {
		t.Error(`forgeCredentialsPresent("gitlab") = true with no gitlab credentials, want false`)
	}
	if forgeCredentialsPresent("github") {
		t.Error(`forgeCredentialsPresent("github") = true with no github credentials, want false`)
	}
	if forgeCredentialsPresent("unknown") {
		t.Error(`forgeCredentialsPresent("unknown") = true, want false`)
	}

	// The identifier alone is not enough — the auth token is required too,
	// so we never build a forge doomed to 401 (and a fixture that leaks an
	// identifier still yields nil).
	t.Setenv("CI_PROJECT_ID", "42")
	if forgeCredentialsPresent("gitlab") {
		t.Error(`forgeCredentialsPresent("gitlab") = true with CI_PROJECT_ID but no CI_JOB_TOKEN, want false`)
	}
	t.Setenv("CI_JOB_TOKEN", "secret")
	if !forgeCredentialsPresent("gitlab") {
		t.Error(`forgeCredentialsPresent("gitlab") = false with both CI_PROJECT_ID and CI_JOB_TOKEN set, want true`)
	}
}
