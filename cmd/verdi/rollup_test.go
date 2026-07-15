package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/provider"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/provider/jira/jiratest"
	"github.com/jyang234/verdi/internal/store"
)

// rollupFixtureSpec is a scratch (not examples/showcase) feature spec
// authored directly for this test file: PLAN.md Phase 11 explicitly warns
// against hard-coding examples/showcase golden SHAs here since another agent
// is rebaking that fixture's content, so rollup's end-to-end coverage gets
// its own small, self-contained fixturegit repo instead.
const rollupFixtureSpec = `---
id: spec/rollup-fixture
kind: spec
class: feature
title: "Rollup fixture (scratch, phase 11)"
status: accepted-pending-build
owners: [platform-team]
story: jira:ROLLUP-1
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
frozen: { at: 2024-01-01, commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }
---
# Rollup fixture
`

// buildRollupFixtureRepo builds a one-commit fixturegit repo carrying
// rollupFixtureSpec at .verdi/specs/active/rollup-fixture/spec.md and
// writes a minimal verdi.yaml directly to disk (store.FindRoot only
// requires the file to exist, not be git-tracked — same convention
// matrix_test.go's buildCorpusRepo uses).
func buildRollupFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{".verdi/specs/active/rollup-fixture/spec.md": rollupFixtureSpec},
			Message: "rollup fixture spec",
		},
	})
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".verdi"), 0o755); err != nil {
		t.Fatalf("mkdir .verdi: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\nforge: gitlab\n"), 0o644); err != nil {
		t.Fatalf("writing verdi.yaml: %v", err)
	}
	return repo
}

// evidenceRecordJSON renders a single verdi.evidence/v1 record for ac-1
// with the given verdict, bound to commit.
func evidenceRecordJSON(verdict, commit string) string {
	return fmt.Sprintf(`[
  {
    "schema": "verdi.evidence/v1",
    "evidence_for": ["ac-1"],
    "kind": "static",
    "verdict": %q,
    "witness": "obligation @ site",
    "provenance": {"source": "ci", "pipeline": "p1", "commit": %q},
    "digest": "sha256:%s"
  }
]
`, verdict, commit, strings.Repeat("a", 64))
}

// writeRollupDerived writes the derived verdicts.json for the fixture
// spec at the given commit, overwriting any prior content — used to
// simulate evidence for a commit changing across CI re-runs.
func writeRollupDerived(t *testing.T, root, commit, verdict string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug("spec/rollup-fixture"), commit)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "verdicts.json"), []byte(evidenceRecordJSON(verdict, commit)), 0o644); err != nil {
		t.Fatalf("writing verdicts.json: %v", err)
	}
}

// TestRunRollup_FakeProvider_EndToEnd drives runRollup directly against a
// scratch fixturegit store and internal/provider/fake, proving the whole
// pipeline (ref/commit resolution, I-30 story resolution, the
// authoritative-only fold, and publish) end-to-end: the first publish
// fires a comment (I-26) even though nothing "changed" yet; republishing
// the same commit is idempotent; a later publish whose fold outcome
// changed (ac-1 evidenced -> violated, simulating new CI results landing
// for the same commit) fires exactly one more comment; a further
// unchanged republish fires none.
func TestRunRollup_FakeProvider_EndToEnd(t *testing.T) {
	repo := buildRollupFixtureRepo(t)
	writeRollupDerived(t, repo.Dir, repo.Head, "pass")

	fp := fake.New()
	ctx := context.Background()
	story := provider.StoryRef("jira:ROLLUP-1")

	var stdout, stderr bytes.Buffer
	deps := rollupDeps{Registry: fp, Stdout: &stdout, Stderr: &stderr}
	if got := runRollup(ctx, repo.Dir, "jira:ROLLUP-1", deps); got != 0 {
		t.Fatalf("runRollup (first publish) = %d, want 0; stderr=%s", got, deps.Stderr)
	}
	if !strings.Contains(stdout.String(), "eligible=true") {
		t.Fatalf("stdout = %q, want it to report eligible=true", stdout.String())
	}
	if got := fp.PublishRecordCount(story); got != 1 {
		t.Fatalf("PublishRecordCount after first publish = %d, want 1", got)
	}
	if got := fp.CommentCount(story); got != 1 {
		t.Fatalf("CommentCount after first publish = %d, want 1 (I-26: first publish always fires)", got)
	}
	field, ok := fp.PublishedField(story)
	if !ok || field.Commit != repo.Head || !field.Eligible {
		t.Fatalf("PublishedField = %+v (ok=%t), want commit=%s eligible=true", field, ok, repo.Head)
	}

	// Republish the same commit with no evidence change: idempotent, no
	// new comment.
	stdout.Reset()
	if got := runRollup(ctx, repo.Dir, "jira:ROLLUP-1", deps); got != 0 {
		t.Fatalf("runRollup (unchanged republish) = %d, want 0; stderr=%s", got, deps.Stderr)
	}
	if got := fp.PublishRecordCount(story); got != 1 {
		t.Fatalf("PublishRecordCount after unchanged republish = %d, want 1 (same commit, an update not a duplicate)", got)
	}
	if got := fp.CommentCount(story); got != 1 {
		t.Fatalf("CommentCount after unchanged republish = %d, want 1 (no comment on unchanged statuses)", got)
	}

	// New CI results land for the SAME commit (a legitimate re-run:
	// evidence accrues without the git commit moving) and flip ac-1 to
	// violated: exactly one more comment, still the same publish-record
	// count (same commit is still one record).
	writeRollupDerived(t, repo.Dir, repo.Head, "fail")
	stdout.Reset()
	if got := runRollup(ctx, repo.Dir, "jira:ROLLUP-1", deps); got != 0 {
		t.Fatalf("runRollup (changed republish) = %d, want 0; stderr=%s", got, deps.Stderr)
	}
	if !strings.Contains(stdout.String(), "eligible=false") || !strings.Contains(stdout.String(), "violated=true") {
		t.Fatalf("stdout = %q, want eligible=false violated=true", stdout.String())
	}
	if got := fp.PublishRecordCount(story); got != 1 {
		t.Fatalf("PublishRecordCount after changed republish (same commit) = %d, want 1", got)
	}
	if got := fp.CommentCount(story); got != 2 {
		t.Fatalf("CommentCount after changed republish = %d, want 2", got)
	}

	// Republish again with the same (now-violated) evidence: no further
	// comment.
	stdout.Reset()
	if got := runRollup(ctx, repo.Dir, "jira:ROLLUP-1", deps); got != 0 {
		t.Fatalf("runRollup (second unchanged republish) = %d, want 0; stderr=%s", got, deps.Stderr)
	}
	if got := fp.CommentCount(story); got != 2 {
		t.Fatalf("CommentCount after second unchanged republish = %d, want 2", got)
	}
}

// TestCmdRollup_JiraAdapter_EndToEnd drives cmdRollup — the real dispatch
// entry point, not the testable core — through its full production wiring
// (manifest load, VERDI_JIRA_TOKEN, buildProviderRegistry) against an
// httptest-backed Jira mock, with a simulated CI environment.
func TestCmdRollup_JiraAdapter_EndToEnd(t *testing.T) {
	server := jiratest.NewServer("customfield_rollup")
	t.Cleanup(server.Close)

	repo := buildRollupFixtureRepo(t)
	writeRollupDerived(t, repo.Dir, repo.Head, "pass")

	manifest := "schema: verdi.layout/v1\nforge: gitlab\nproviders:\n  jira:\n    base_url: " + server.URL + "\n    rollup_field: customfield_rollup\n"
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", "verdi.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("writing verdi.yaml: %v", err)
	}

	t.Setenv("CI", "true")
	t.Setenv("VERDI_JIRA_TOKEN", "test-token")
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := cmdRollup([]string{"jira:ROLLUP-1", "--publish"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdRollup = %d, want 0; stderr=%s", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "eligible=true") {
		t.Fatalf("stdout = %q, want it to report eligible=true", stdout.String())
	}

	fieldRaw, ok := server.FieldValue("ROLLUP-1")
	if !ok {
		t.Fatal("jiratest server has no rollup field set for ROLLUP-1 after publish")
	}
	if !strings.Contains(fieldRaw, repo.Head) || !strings.Contains(fieldRaw, `"eligible":true`) {
		t.Fatalf("published field = %q, want it to carry commit %s and eligible:true", fieldRaw, repo.Head)
	}
	if got := server.CommentCount("ROLLUP-1"); got != 1 {
		t.Fatalf("CommentCount after first publish = %d, want 1 (I-26)", got)
	}

	// Republish unchanged: idempotent, no new comment.
	stdout.Reset()
	stderr.Reset()
	if got := cmdRollup([]string{"jira:ROLLUP-1", "--publish"}, &stdout, &stderr); got != 0 {
		t.Fatalf("cmdRollup (republish) = %d, want 0; stderr=%s", got, stderr.String())
	}
	if got := server.CommentCount("ROLLUP-1"); got != 1 {
		t.Fatalf("CommentCount after unchanged republish = %d, want 1", got)
	}
	if got := server.PublishedCommitCount("ROLLUP-1"); got != 1 {
		t.Fatalf("PublishedCommitCount after unchanged republish = %d, want 1", got)
	}
}

// TestCmdRollup_RefusesOutsideCI proves 04 §Semantics's "PublishRollup runs
// in CI only" is enforced before anything else runs (no store root is
// even resolved), and that --force-local is a disclosed, non-authoritative
// escape hatch rather than a silent bypass.
func TestCmdRollup_RefusesOutsideCI(t *testing.T) {
	for _, v := range []string{"CI", "GITHUB_ACTIONS", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "GITHUB_BASE_REF"} {
		t.Setenv(v, "")
	}
	t.Chdir(t.TempDir())

	t.Run("no --force-local: refused", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdRollup([]string{"jira:LOAN-1482", "--publish"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdRollup outside CI = %d, want 2", got)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty on a CI refusal", stdout.String())
		}
		if !strings.Contains(stderr.String(), "CI only") && !strings.Contains(stderr.String(), "outside CI") {
			t.Fatalf("stderr = %q, want it to explain the CI-only refusal", stderr.String())
		}
		if !strings.Contains(stderr.String(), "--force-local") {
			t.Fatalf("stderr = %q, want it to name the --force-local escape hatch", stderr.String())
		}
	})

	t.Run("--force-local: proceeds with a disclosed non-authoritative warning", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		// No store root under t.TempDir(), so this still exits 2 — but
		// past the CI check, on the store-root error, proving
		// --force-local actually let it through.
		got := cmdRollup([]string{"jira:LOAN-1482", "--publish", "--force-local"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdRollup(--force-local, no store) = %d, want 2", got)
		}
		if !strings.Contains(stderr.String(), "NON-AUTHORITATIVE") {
			t.Fatalf("stderr = %q, want a disclosed NON-AUTHORITATIVE warning", stderr.String())
		}
		if strings.Contains(stderr.String(), "refusing to publish") {
			t.Fatalf("stderr = %q, --force-local should not still be refused", stderr.String())
		}
	})
}

// TestCmdRollup_Negative covers cmdRollup's own operational-error paths
// independent of CI detection.
func TestCmdRollup_Negative(t *testing.T) {
	t.Setenv("CI", "true")

	t.Run("no story argument", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdRollup([]string{"--publish"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdRollup(no story) = %d, want 2", got)
		}
	})

	t.Run("missing --publish", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdRollup([]string{"jira:LOAN-1482"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdRollup(no --publish) = %d, want 2", got)
		}
		if !strings.Contains(stderr.String(), "--publish") {
			t.Fatalf("stderr = %q, want it to mention --publish", stderr.String())
		}
	})

	t.Run("extra positional argument", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdRollup([]string{"jira:LOAN-1482", "spec/other", "--publish"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdRollup(two positional args) = %d, want 2", got)
		}
	})

	t.Run("no store root", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var stdout, stderr bytes.Buffer
		got := cmdRollup([]string{"jira:LOAN-1482", "--publish"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdRollup(no store root) = %d, want 2", got)
		}
	})
}

// TestBuildProviderRegistry_FakeMode proves providers.jira.mode: fake
// (spec/close-verb dc-2) selects the in-process fake adapter — Resolve
// against an unseeded ref degrades to provider.ErrNotFound (the fake's own
// documented behavior), never a real HTTP call, and the registered adapter
// is concretely *fake.Provider so a caller (runClose) can seed/inspect it
// directly in hermetic tests.
func TestBuildProviderRegistry_FakeMode(t *testing.T) {
	m, err := store.DecodeManifest([]byte("schema: verdi.layout/v1\nproviders:\n  jira:\n    mode: fake\n    base_url: https://example.atlassian.net\n    rollup_field: customfield_00000\n"))
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	reg := buildProviderRegistry(m)
	p, err := reg.Provider("jira")
	if err != nil {
		t.Fatalf("reg.Provider(jira): %v", err)
	}
	if _, ok := p.(*fake.Provider); !ok {
		t.Fatalf("provider registered for jira = %T, want *fake.Provider under mode: fake", p)
	}
}

// TestBuildProviderRegistry_RealModeUnchanged proves the default (mode: ""
// or absent) still wires the real Jira adapter — this addition must not
// change existing behavior for every store that never sets mode:.
func TestBuildProviderRegistry_RealModeUnchanged(t *testing.T) {
	m, err := store.DecodeManifest([]byte("schema: verdi.layout/v1\nproviders:\n  jira:\n    base_url: https://example.atlassian.net\n    rollup_field: customfield_00000\n"))
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	reg := buildProviderRegistry(m)
	p, err := reg.Provider("jira")
	if err != nil {
		t.Fatalf("reg.Provider(jira): %v", err)
	}
	if _, ok := p.(*fake.Provider); ok {
		t.Fatal("provider registered for jira is *fake.Provider, want the real jira.Adapter when mode: is absent")
	}
}

// TestRun_RollupDispatchesToRealVerb proves dispatch.go routes "rollup" to
// the real implementation, matching the equivalent matrix/sync tests.
func TestRun_RollupDispatchesToRealVerb(t *testing.T) {
	t.Setenv("CI", "true")
	t.Chdir(t.TempDir())

	var stderr bytes.Buffer
	got := run([]string{"rollup", "jira:LOAN-1482", "--publish"}, &stderr)
	if got != 2 {
		t.Fatalf("run([rollup, jira:LOAN-1482, --publish]) outside a store = %d, want 2 (operational)", got)
	}
	if strings.Contains(stderr.String(), "usage") || strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}
