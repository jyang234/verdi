package mcpserve

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/store"
)

// These tests use spec/widget-retry, the accepted feature spec
// buildFixture (fixture_test.go) actually carries on main — the brief's
// draft used "escrow-autopay", which this fixture does not have;
// widget-notes is the fixture's only other spec but is class "component",
// not "feature", so widget-retry is the one accepted feature spec these
// assertions need (task-3 resolution note).

func TestGetDocument_Happy(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, repo, _ := newTestBackend(t)
	res := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry"}))
	if isToolError(res) {
		t.Fatalf("tool error: %s", toolResultText(t, res))
	}
	var got struct {
		Ref, Kind, Commit, Engine, Markdown string
		Proposed                            bool
		Disclosures                         []string
	}
	if err := json.Unmarshal([]byte(toolResultText(t, res)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Ref != "spec/widget-retry" || got.Kind != "spec" || got.Commit != repo.Head || got.Proposed || !strings.HasPrefix(got.Engine, "sha256:") {
		t.Fatalf("got %+v", got)
	}
	if !strings.HasPrefix(got.Markdown, "# ") || !strings.Contains(got.Markdown, "not authority") || !strings.HasSuffix(got.Markdown, "\n") {
		t.Fatalf("markdown wrong shape:\n%s", got.Markdown)
	}
	again := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry"}))
	if toolResultText(t, res) != toolResultText(t, again) {
		t.Fatal("two calls over unchanged state must be byte-identical")
	}

	// F1 (review, fix round 2): in this fixture the checkout and HEAD
	// start byte-identical, so nothing above can fail if ModeAccepted were
	// swapped for ModeWorkingTree — dirty the checkout's own spec.md
	// (never committed) and prove the render still reflects the accepted
	// commit's title, not the dirty working tree's.
	specPath := filepath.Join(backend.Root, ".verdi", "specs", "active", "widget-retry", "spec.md")
	original, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(original), `title: "Widget retry"`) {
		t.Fatalf("fixture spec.md missing the expected title line: %s", original)
	}
	dirty := strings.Replace(string(original), `title: "Widget retry"`, `title: "UNCOMMITTED"`, 1)
	if err := os.WriteFile(specPath, []byte(dirty), 0o644); err != nil {
		t.Fatal(err)
	}
	dirtyRes := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry"}))
	if isToolError(dirtyRes) {
		t.Fatalf("tool error after dirtying the working tree: %s", toolResultText(t, dirtyRes))
	}
	var dirtyGot struct{ Markdown string }
	if err := json.Unmarshal([]byte(toolResultText(t, dirtyRes)), &dirtyGot); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dirtyGot.Markdown, "# Widget retry\n") {
		t.Fatalf("get_document(spec/widget-retry) read the dirty working tree instead of the accepted commit:\n%s", dirtyGot.Markdown)
	}
}

// TestGetDocument_MalformedManifestDegradesToNilModel proves the
// model-resolution branch tool_get_document.go's `if cfg, cerr :=
// store.Open(b.Root); cerr == nil` takes when store.Open fails: a
// present-but-malformed verdi.yaml (decodable YAML, wrong schema value)
// still satisfies specdocload.Load's own Stat-only precondition, so the
// render must still succeed with a nil model rather than erroring.
func TestGetDocument_MalformedManifestDegradesToNilModel(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, _, _ := newTestBackend(t)
	manifestPath := filepath.Join(backend.Root, ".verdi", "verdi.yaml")
	if err := os.WriteFile(manifestPath, []byte("schema: not-a-real-schema/v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry"}))
	if isToolError(res) {
		t.Fatalf("tool error with a malformed (but present) verdi.yaml: %s", toolResultText(t, res))
	}
}

func TestGetDocument_KindsAndPin(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, repo, _ := newTestBackend(t)
	plan := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry", "kind": "plan"}))
	if isToolError(plan) || !strings.Contains(toolResultText(t, plan), "## Decisions") || strings.Contains(toolResultText(t, plan), "## Problem") {
		t.Fatalf("plan kind: %s", toolResultText(t, plan))
	}
	pinned := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry@" + repo.Head}))
	if isToolError(pinned) || !strings.Contains(toolResultText(t, pinned), `"commit":"`+repo.Head+`"`) {
		t.Fatalf("pinned ref: %s", toolResultText(t, pinned))
	}
	viaArg := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry", "commit": repo.Head}))
	if toolResultText(t, pinned) != toolResultText(t, viaArg) {
		t.Fatal("ref@commit and commit arg must agree")
	}
}

func TestGetDocument_Refusals(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, repo, _ := newTestBackend(t)
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing ref", map[string]any{}, "ref is required"},
		{"not a spec", map[string]any{"ref": "adr/0001"}, "spec/<name>"},
		{"fragment", map[string]any{"ref": "spec/widget-retry#ac-1"}, "spec/<name>"},
		{"unknown field", map[string]any{"ref": "spec/widget-retry", "format": "html"}, "unknown field"},
		{"bad kind", map[string]any{"ref": "spec/widget-retry", "kind": "chapter"}, "unknown document kind"},
		{"pin and commit disagree", map[string]any{"ref": "spec/widget-retry@" + repo.Head, "commit": strings.Repeat("b", 40)}, "disagree"},
		{"unknown spec", map[string]any{"ref": "spec/nope"}, "spec/nope"},
		{"bad commit", map[string]any{"ref": "spec/widget-retry", "commit": "deadbeef"}, "deadbeef"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := backend.GetDocument(context.Background(), mustArgs(t, c.args))
			if !isToolError(res) {
				t.Fatalf("want tool error, got %s", toolResultText(t, res))
			}
			if !strings.Contains(toolResultText(t, res), c.want) {
				t.Fatalf("error %q does not name %q", toolResultText(t, res), c.want)
			}
		})
	}
}

// TestGetDocument_RejectsNonHexCommitArgument (final-review F2): the
// `commit` argument names a commit, so it is held to the SAME rule the
// pinned ref form `spec/<name>@<commit>` is held to
// (internal/artifact/ref.go's commitRe, 7-40 lowercase hex, reached
// through artifact.ValidCommit) — refused as an argument error before it
// reaches git, never handed to `git rev-parse --verify` as a revision
// expression or, worse, as an option. Without the check `commit: "HEAD"`
// and `commit: "--git-dir"` both behave unlike the pinned form: the one
// silently resolves a symbolic revision the documented contract does not
// offer, the other is consumed by git as a flag rather than a revision.
func TestGetDocument_RejectsNonHexCommitArgument(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	backend, repo, _ := newTestBackend(t)
	for _, c := range []struct {
		name   string
		commit string
	}{
		{"option-shaped", "--git-dir"},
		{"leading dash", "-n"},
		{"path traversal", "../x"},
		{"symbolic revision", "HEAD"},
		{"revision expression", "HEAD~3"},
		{"branch name", "main"},
		{"uppercase hex", strings.ToUpper(repo.Head)},
		{"too short", "abcdef"},
		{"too long", strings.Repeat("a", 41)},
		{"empty-ish whitespace", " "},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry", "commit": c.commit}))
			if !isToolError(res) {
				t.Fatalf("commit %q must be an argument error, got %s", c.commit, toolResultText(t, res))
			}
			if got := toolResultText(t, res); !strings.Contains(got, "7-40 lowercase hex") {
				t.Fatalf("commit %q: error %q must name the pinned-ref commit rule", c.commit, got)
			}
		})
	}
	// Happy path: a well-formed sha still renders, so the guard refuses a
	// shape, never a legitimate pin.
	t.Run("well-formed sha still renders", func(t *testing.T) {
		res := backend.GetDocument(context.Background(), mustArgs(t, map[string]any{"ref": "spec/widget-retry", "commit": repo.Head}))
		if isToolError(res) {
			t.Fatalf("a well-formed commit must render: %s", toolResultText(t, res))
		}
	})
}

// getDocumentFixtureStore builds this file's existing accepted-spec
// fixture (buildFixture, fixture_test.go) and returns its store root, for
// tests that only need the root rather than newTestBackend's full
// (*Backend, *fixturegit.Repo, string) triple.
func getDocumentFixtureStore(t *testing.T) string {
	t.Helper()
	backend, _, _ := newTestBackend(t)
	return backend.Root
}

// acceptedFixtureRef returns the ref this file's happy-path test
// (TestGetDocument_Happy) renders: the fixture's one accepted feature
// spec, spec/widget-retry (widget-notes, the fixture's other spec, is
// class "component", not "feature" — see this file's top-of-file note).
// Confirms root actually carries it so a fixture-shape change fails
// loudly here rather than as a confusing ref-not-found error deeper in a
// caller's test.
func acceptedFixtureRef(t *testing.T, root string) string {
	t.Helper()
	const ref = "spec/widget-retry"
	if _, err := os.Stat(store.ActiveSpecPath(root, "widget-retry")); err != nil {
		t.Fatalf("acceptedFixtureRef: %s does not carry %s: %v", root, ref, err)
	}
	return ref
}

// getDocumentDraftStoreSpec is a minimal valid spec, the same shape
// internal/designapp/conformance_test.go's conformanceSpec uses (id
// spec/sample, class feature, one AC, one constraint) — copied rather
// than imported, since conformanceSpec is unexported inside another
// package's own external _test package.
const getDocumentDraftStoreSpec = `---
id: spec/sample
kind: spec
class: feature
title: Sample
owners: [platform-team]
problem: { text: "old problem", anchor: "#problem" }
outcome: { text: "old outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "first", evidence: [static], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "bounded", anchor: "#co-1" }
---
# Sample

## Problem

Old prose stays.

## Outcome

Old prose stays.

## ac-1

First.

## co-1

Bounded.
`

// getDocumentDraftStore builds internal/designapp/conformance_test.go's
// conformanceStore recipe (committed store on main, checked out onto
// design/sample, then an UNCOMMITTED draft written directly to
// .verdi/specs/active/sample/spec.md) — without that fixture's unrelated
// ASD policy content (internal/policyauthority's testdata store), which
// GetDocument never reads. main carries only .verdi/verdi.yaml, so
// spec/sample does not exist on the default branch at all: ModeAccepted
// must refuse it, and only ModeWorkingTree (the checkout's own working
// tree, currently design/sample) can read the draft.
func getDocumentDraftStore(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n"},
		Message: "layer 1: store root",
	}})

	checkout := exec.Command("git", "checkout", "-b", "design/sample")
	checkout.Dir = repo.Dir
	if output, err := checkout.CombinedOutput(); err != nil {
		t.Fatalf("git checkout design/sample: %v\n%s", err, output)
	}

	if err := os.MkdirAll(store.ActiveSpecDir(repo.Dir, "sample"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.ActiveSpecPath(repo.Dir, "sample"), []byte(getDocumentDraftStoreSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo.Dir
}

// validReadinessConcern builds one PROVEN concern for the closed concern-
// identity vocabulary (internal/readinesspilot/schema.go's
// concernIdentity): id fixes the derived area and blocking flag, so these
// are not arbitrary — mirrors that package's own (unexported, so
// reproduced rather than imported) schema_test.go validConcern helper.
func validReadinessConcern(id string, area readinesspilot.AreaID, blocking bool) readinesspilot.Concern {
	return readinesspilot.Concern{
		ID:        id,
		Area:      area,
		State:     readinesspilot.StateProven,
		Blocking:  blocking,
		Timing:    readinesspilot.TimingCurrent,
		Summary:   "source-derived readiness fact",
		Witnesses: []string{},
		Destination: readinesspilot.Destination{
			CLI: []string{},
		},
	}
}

// validReadinessSnapshot returns a Snapshot targeting ref that passes
// Snapshot.Validate(): every one of the four fixed areas proven, no
// attention items. Mirrors internal/readinesspilot/schema_test.go's own
// (unexported) validSnapshot() — reproduced here for the same reason
// validReadinessConcern is. TargetTitle/TargetClass/Branch/RequestDigest
// are Validate()-only fields specdoc.WithReadiness never reads (it copies
// only TargetRef/Head/CurrentFocus/StaleNotice/Areas/Attention into the
// rendered document), so their exact values do not matter beyond
// satisfying Validate().
func validReadinessSnapshot(ref, head string) readinesspilot.Snapshot {
	return readinesspilot.Snapshot{
		TargetRef:     ref,
		TargetTitle:   "Readiness test target",
		TargetClass:   "feature",
		Branch:        "main",
		Head:          head,
		RequestDigest: "sha256:" + strings.Repeat("a", 64),
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaSuccess, Label: "Define success", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaContext, Label: "Check constraints", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaReview, Label: "Get approval", State: readinesspilot.StateProven},
		},
		CurrentFocus: "",
		Attention:    []readinesspilot.Concern{},
		AllConcerns: []readinesspilot.Concern{
			validReadinessConcern("shape/problem", readinesspilot.AreaShape, true),
			validReadinessConcern("success/contributor/static", readinesspilot.AreaSuccess, false),
			validReadinessConcern("context/verdict", readinesspilot.AreaContext, true),
			validReadinessConcern("review/action", readinesspilot.AreaReview, true),
		},
		StaleNotice: "Startup snapshot at " + head + "; restart verdi serve after an edit.",
	}
}

// TestGetDocument_ReadinessWhenSnapshotTargetsSpec is R-W3-3: Backend.Readiness
// reaches the loader (tool_get_document.go's Readiness: b.Readiness), which
// supplies the Readiness section only when the snapshot's TargetRef names
// the spec being rendered (internal/specdoc/readiness.go's WithReadiness,
// Wave 2) — never another spec's facts.
func TestGetDocument_ReadinessWhenSnapshotTargetsSpec(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	root := getDocumentFixtureStore(t)
	snap := validReadinessSnapshot("spec/widget-retry", strings.Repeat("a", 40))
	if err := snap.Validate(); err != nil {
		t.Fatal(err)
	}
	b := &Backend{Root: root, Readiness: &snap}
	raw, _ := json.Marshal(map[string]any{"ref": snap.TargetRef, "kind": "spec"})
	text, isErr := decodeText(t, b.GetDocument(context.Background(), raw))
	if isErr {
		t.Fatal(text)
	}
	if !strings.Contains(text, "## Readiness") || strings.Contains(text, "Readiness was not supplied for this render.") {
		t.Fatalf("readiness section not rendered from the snapshot:\n%s", text)
	}
	// A snapshot targeting another spec leaves the document untouched.
	other := snap
	other.TargetRef = "spec/other"
	b2 := &Backend{Root: root, Readiness: &other}
	text2, _ := decodeText(t, b2.GetDocument(context.Background(), raw))
	if !strings.Contains(text2, "Readiness was not supplied for this render.") {
		t.Fatalf("foreign snapshot must not leak:\n%s", text2)
	}
}

// TestGetDocument_ProposedRendersTheWorkingTreeDraft is R-W3-9 (ledger
// SI-202): proposed:true selects specdocload.ModeWorkingTree, so
// get_document can render a draft still sitting only on its design
// branch's working tree — never committed to the default branch, so the
// accepted (default) reading cannot see it at all.
func TestGetDocument_ProposedRendersTheWorkingTreeDraft(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	root := getDocumentDraftStore(t)
	b := &Backend{Root: root}
	// Accepted mode cannot see a draft main does not carry.
	raw, _ := json.Marshal(map[string]any{"ref": "spec/sample", "kind": "spec"})
	if text, isErr := decodeText(t, b.GetDocument(context.Background(), raw)); !isErr {
		t.Fatalf("accepted mode must refuse a draft absent from the default branch: %s", text)
	}
	// proposed:true renders the working tree, marked proposed.
	raw, _ = json.Marshal(map[string]any{"ref": "spec/sample", "kind": "spec", "proposed": true})
	text, isErr := decodeText(t, b.GetDocument(context.Background(), raw))
	if isErr {
		t.Fatal(text)
	}
	var res getDocumentResult
	if err := json.Unmarshal([]byte(text), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Proposed || !strings.Contains(res.Markdown, "Proposed, not accepted") {
		t.Fatalf("proposed render = %+v", res)
	}
	// proposed and commit together are refused by name.
	raw, _ = json.Marshal(map[string]any{"ref": "spec/sample", "kind": "spec", "proposed": true, "commit": strings.Repeat("a", 40)})
	if text, isErr := decodeText(t, b.GetDocument(context.Background(), raw)); !isErr || !strings.Contains(text, "proposed and commit") {
		t.Fatalf("proposed+commit: isErr %v text %q", isErr, text)
	}
	// An accepted spec with proposed:true renders unmarked (Proposed derived, R-W2-4).
	root2 := getDocumentFixtureStore(t)
	raw, _ = json.Marshal(map[string]any{"ref": acceptedFixtureRef(t, root2), "kind": "spec", "proposed": true})
	text, _ = decodeText(t, (&Backend{Root: root2}).GetDocument(context.Background(), raw))
	if strings.Contains(text, `"proposed":true`) {
		t.Fatalf("exact accepted bytes must not be marked proposed:\n%s", text)
	}
}
