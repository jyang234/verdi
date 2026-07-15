package wallbadge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/lint"
)

// writeStoreSpec writes a story spec.md at .verdi/specs/active/<name>/
// under a fresh root, decodes it (so tests catch their own fixture
// mistakes early), and returns (root, parsed frontmatter).
func writeStoreSpec(t *testing.T, name, specMD string) (string, *artifact.SpecFrontmatter) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".verdi", "specs", "active", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(specMD), 0o644); err != nil {
		t.Fatalf("write spec.md: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "spec.md"))
	if err != nil {
		t.Fatalf("read back spec.md: %v", err)
	}
	fmBytes, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	// internal/lint's own walk decodes a spec document via
	// artifact.DecodeStrict alone — shape only, never the kind's semantic
	// Validate() (internal/lint/doc.go's design note: "every semantic
	// check is re-implemented by its own VL-xxx rule rather than
	// centralized") — so a fixture whose AC deliberately declares no
	// evidence kind (VL-006's own finding) still decodes here, exactly as
	// it does inside the real engine. Using artifact.DecodeSpec instead
	// would reject such a fixture at Validate() before VL-006 ever runs.
	var fm artifact.SpecFrontmatter
	if err := artifact.DecodeStrict(fmBytes, &fm); err != nil {
		t.Fatalf("DecodeStrict: %v", err)
	}
	return root, &fm
}

const ladderStorySpec = `---
id: spec/widget-retry
kind: spec
class: story
title: "Widget retry"
status: draft
owners: [platform-team]
story: jira:WID-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "retries the widget", evidence: [attestation], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/parent-feature#ac-1" }
---
# Widget retry

## Problem

p

## Outcome

o

## ac-1

Retries.
`

const ladderCoversSHA = "2f230011b192c5ac1c0ed5442be76fc401c4cbca"

func flaggedDeviationReportMD(covers string) string {
	return `---
schema: verdi.deviation/v1
covers: ` + covers + `
findings:
  - { id: ac-1, kind: computed, text: "own-text drift", disposition: accepted-deviation, note: "known, deferred" }
---
# Deviation report
`
}

func writeDeviationReport(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", name)
	if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write deviation-report.md: %v", err)
	}
}

func buildSnapshotFor(t *testing.T, root string) *lint.Snapshot {
	t.Helper()
	snap, err := lint.BuildSnapshot(root, lint.Options{})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	return snap
}

// TestSpecStaleBadge_FlaggedWithWitness is ac-3's trigger (a): a deviation
// report whose accepted-deviation finding id equals the story's own AC id
// flags spec-stale, and the badge names that finding id plus the
// deviation report's own `covers` sha as the input revision (dc-5).
func TestSpecStaleBadge_FlaggedWithWitness(t *testing.T) {
	root, fm := writeStoreSpec(t, "widget-retry", ladderStorySpec)
	writeDeviationReport(t, root, "widget-retry", flaggedDeviationReportMD(ladderCoversSHA))
	snap := buildSnapshotFor(t, root)

	got, err := SpecStaleBadge(root, snap, fm.ID, 3)
	if err != nil {
		t.Fatalf("SpecStaleBadge: %v", err)
	}
	if got == nil {
		t.Fatal("got nil badge, want flagged spec-stale")
	}
	if got.Source != "ladder:spec-stale" {
		t.Errorf("Source = %q, want ladder:spec-stale", got.Source)
	}
	if got.Target != "" {
		t.Errorf("Target = %q, want empty (case-file badge)", got.Target)
	}
	if len(got.Inputs) != 1 || got.Inputs[0].Revision != ladderCoversSHA {
		t.Fatalf("Inputs = %+v, want one input pinned to the report's own covers sha", got.Inputs)
	}
	found := false
	for _, r := range got.Records {
		if r == "ac-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("Records = %+v, want to name the own-text finding id ac-1", got.Records)
	}
}

// TestSpecStaleBadge_NoReport is the ordinary "never audited" case:
// absent a deviation-report.md at all, ScanSpecStale skips the story —
// proven-unflagged (trivially: nothing has accumulated), not an error and
// not a disclosure.
func TestSpecStaleBadge_NoReport(t *testing.T) {
	root, fm := writeStoreSpec(t, "widget-retry", ladderStorySpec)
	snap := buildSnapshotFor(t, root)

	got, err := SpecStaleBadge(root, snap, fm.ID, 3)
	if err != nil {
		t.Fatalf("SpecStaleBadge: %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil (no report on disk)", got)
	}
}

// TestSpecStaleBadge_ReportPresentButUnflagged proves a report that
// exists but carries no accepted-deviation (or none matching a trigger)
// yields no badge either — proven-unflagged, not merely "absent".
func TestSpecStaleBadge_ReportPresentButUnflagged(t *testing.T) {
	root, fm := writeStoreSpec(t, "widget-retry", ladderStorySpec)
	writeDeviationReport(t, root, "widget-retry", `---
schema: verdi.deviation/v1
covers: `+ladderCoversSHA+`
findings:
  - { id: ac-1, kind: computed, text: "fixed promptly", disposition: fixed }
---
# Deviation report
`)
	snap := buildSnapshotFor(t, root)

	got, err := SpecStaleBadge(root, snap, fm.ID, 3)
	if err != nil {
		t.Fatalf("SpecStaleBadge: %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil (fixed, not accepted-deviation: nothing triggers)", got)
	}
}

// TestSpecStaleBadge_UnknownRef is the operational-negative path: a
// specRef this snapshot never declares (a caller programming error, not a
// user-facing case) is simply not found — proven-unflagged, since
// ScanSpecStale's own entries are keyed by ref and an absent key behaves
// exactly like "no report".
func TestSpecStaleBadge_UnknownRef(t *testing.T) {
	root, _ := writeStoreSpec(t, "widget-retry", ladderStorySpec)
	snap := buildSnapshotFor(t, root)

	got, err := SpecStaleBadge(root, snap, "spec/does-not-exist", 3)
	if err != nil {
		t.Fatalf("SpecStaleBadge: %v", err)
	}
	if got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

// fakeSupersessionLoader is a hermetic SupersessionCandidateLoader double.
type fakeSupersessionLoader struct {
	candidates []evidence.OpenSupersessionCandidate
	ok         bool
	err        error
}

func (f fakeSupersessionLoader) LoadCandidates(ctx context.Context, featureRef, specPath string) ([]evidence.OpenSupersessionCandidate, bool, error) {
	return f.candidates, f.ok, f.err
}

func implementsLink(featureRef string) []artifact.Link {
	return []artifact.Link{{Type: artifact.LinkImplements, Ref: featureRef}}
}

// TestPendingSupersessionBadge_FlaggedWithWitness is ac-3's pending-
// supersession trigger: a loaded candidate's supersession manifest
// amends an object this story's own edges touch flags the badge, naming
// the MR id and the touched object id, with the candidate's own digest as
// the input revision (dc-5) — never the MR id itself as a revision.
func TestPendingSupersessionBadge_FlaggedWithWitness(t *testing.T) {
	loader := fakeSupersessionLoader{
		ok: true,
		candidates: []evidence.OpenSupersessionCandidate{{
			MRID:   "7",
			Digest: "sha256:cccc",
			Spec:   &artifact.SpecFrontmatter{Supersession: &artifact.Supersession{Amended: []artifact.SupersessionNote{{ID: "ac-1", Note: "tightened"}}}},
		}},
	}
	got, disclosure, err := PendingSupersessionBadge(context.Background(), loader, implementsLink("spec/parent-feature#ac-1"))
	if err != nil {
		t.Fatalf("PendingSupersessionBadge: %v", err)
	}
	if disclosure != "" {
		t.Fatalf("disclosure = %q, want empty on a flagged outcome", disclosure)
	}
	if got == nil {
		t.Fatal("got nil badge, want flagged pending-supersession")
	}
	if got.Source != "ladder:pending-supersession" {
		t.Errorf("Source = %q", got.Source)
	}
	// spec/case-file-flags dc-4: one vocabulary across surfaces — the
	// stamp wears the SAME flag name the dex story-lens badge wears
	// (internal/dex/ladder.go appends "pending-supersession", hyphenated),
	// so the flag reads identically on the wall and on the dex page.
	if got.Label != "pending-supersession" {
		t.Errorf("Label = %q, want pending-supersession (the dex story-lens's own flag name, spec/case-file-flags dc-4)", got.Label)
	}
	if len(got.Inputs) != 1 || got.Inputs[0].Revision != "sha256:cccc" {
		t.Fatalf("Inputs = %+v, want the candidate's own digest, never its MR id", got.Inputs)
	}
	wantMR, wantTouch := false, false
	for _, r := range got.Records {
		if r == "MR 7" {
			wantMR = true
		}
		if r == "touches ac-1" {
			wantTouch = true
		}
	}
	if !wantMR || !wantTouch {
		t.Errorf("Records = %+v, want to name MR 7 and touches ac-1", got.Records)
	}
}

// TestPendingSupersessionBadge_ImplementsNoFeature proves a story with no
// implements edge at all has nothing to prove — nil record, no
// disclosure, regardless of the loader (never consulted).
func TestPendingSupersessionBadge_ImplementsNoFeature(t *testing.T) {
	got, disclosure, err := PendingSupersessionBadge(context.Background(), fakeSupersessionLoader{}, nil)
	if err != nil {
		t.Fatalf("PendingSupersessionBadge: %v", err)
	}
	if got != nil || disclosure != "" {
		t.Fatalf("got (%+v, %q), want (nil, \"\")", got, disclosure)
	}
}

// TestPendingSupersessionBadge_NilLoaderDisclosesUnproven is ac-3's
// disclosed-unproven outcome: no forge configured (nil loader) on a story
// that DOES implement a feature renders a disclosure — never a badge,
// never silence.
func TestPendingSupersessionBadge_NilLoaderDisclosesUnproven(t *testing.T) {
	got, disclosure, err := PendingSupersessionBadge(context.Background(), nil, implementsLink("spec/parent-feature#ac-1"))
	if err != nil {
		t.Fatalf("PendingSupersessionBadge: %v", err)
	}
	if got != nil {
		t.Fatalf("got a badge %+v, want none (unproven never badges)", got)
	}
	if disclosure == "" {
		t.Fatal("got no disclosure, want one naming the unproven state")
	}
}

// TestPendingSupersessionBadge_LoaderNotOkDisclosesUnproven mirrors the
// nil-loader case for a configured-but-unable-to-enumerate loader (e.g.
// no default branch resolved) — same disclosed-unproven contract.
func TestPendingSupersessionBadge_LoaderNotOkDisclosesUnproven(t *testing.T) {
	got, disclosure, err := PendingSupersessionBadge(context.Background(), fakeSupersessionLoader{ok: false}, implementsLink("spec/parent-feature#ac-1"))
	if err != nil {
		t.Fatalf("PendingSupersessionBadge: %v", err)
	}
	if got != nil || disclosure == "" {
		t.Fatalf("got (%+v, %q), want (nil, non-empty disclosure)", got, disclosure)
	}
}

// TestPendingSupersessionBadge_ProvenUnflagged proves a loader that DOES
// enumerate candidates, none of which touch this story's edges, yields no
// badge and no disclosure — proven-unflagged, distinct from unproven.
func TestPendingSupersessionBadge_ProvenUnflagged(t *testing.T) {
	loader := fakeSupersessionLoader{
		ok: true,
		candidates: []evidence.OpenSupersessionCandidate{{
			MRID:   "9",
			Digest: "sha256:dddd",
			Spec:   &artifact.SpecFrontmatter{Supersession: &artifact.Supersession{Amended: []artifact.SupersessionNote{{ID: "co-1", Note: "unrelated"}}}},
		}},
	}
	got, disclosure, err := PendingSupersessionBadge(context.Background(), loader, implementsLink("spec/parent-feature#ac-1"))
	if err != nil {
		t.Fatalf("PendingSupersessionBadge: %v", err)
	}
	if got != nil || disclosure != "" {
		t.Fatalf("got (%+v, %q), want (nil, \"\") — the candidate doesn't touch ac-1", got, disclosure)
	}
}

// TestPendingSupersessionBadge_LoaderErrorPropagates is the operational-
// error negative path: a genuine transport failure propagates as an
// error, never silently swallowed into a disclosure or a false
// proven-unflagged.
func TestPendingSupersessionBadge_LoaderErrorPropagates(t *testing.T) {
	wantErr := errors.New("simulated transport failure")
	loader := fakeSupersessionLoader{err: wantErr}
	_, _, err := PendingSupersessionBadge(context.Background(), loader, implementsLink("spec/parent-feature#ac-1"))
	if err == nil {
		t.Fatal("got nil error, want the loader's transport failure to propagate")
	}
}
