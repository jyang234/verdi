package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// archivedStorySpecMD renders a minimal, valid, STATUSLESS archived story
// spec that implements featureRef (e.g. "spec/my-feature#ac-1") — Task 4's
// compatibility grammar (a statusless story is exactly as closeable as an
// explicitly-flagged one), landed via fixturegit rather than carrying a
// legacy status:/frozen: pair a Git-derived "closed" reading no longer
// needs.
func archivedStorySpecMD(name, featureRef string) string {
	return `---
id: spec/` + name + `
kind: spec
class: story
title: "` + name + `"
owners: [platform-team]
story: jira:` + strings.ToUpper(name) + `-1
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
links:
  - { type: implements, ref: "` + featureRef + `" }
acceptance_criteria:
  - { id: ac-1, text: "the story's own obligation holds", evidence: [attestation] }
---
# body
`
}

// TestGatherArchivedRulings_ScopesToClosedImplementing proves the L-N14
// companion gathering reads exactly the CONFIRMED-CLOSED (Git-derived,
// internal/specstate) implementing stories' archived rulings — the same
// set the feature-close budget unions — so a seated candidate backing
// always has a matching archive to collapse against (never a budget
// inflation): a story implementing a DIFFERENT feature, a story with no
// archived report, and an archive-zone copy that never actually LANDED on
// the default branch at that path are all excluded.
//
// Replaces the pre-Task-5 fixture's explicit `status: superseded` exclusion
// case: under Git-derived state an archive-zone candidate can never resolve
// Superseded at all (gatherArchivedRulings' own doc comment) — resolveOne
// branches on zone before ever consulting the successor corpus for an
// archive-zone path, so a genuinely-superseded predecessor simply never
// leaves the active zone to be seen by this glob in the first place. The
// unlanded-story case below is the Git-derived shape that actually can
// occur and must still be excluded: a local archive/ copy this store has
// not proven reachable from the default branch.
func TestGatherArchivedRulings_ScopesToClosedImplementing(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                           "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/specs/archive/impl-story/spec.md":     archivedStorySpecMD("impl-story", "spec/my-feature#ac-1"),
				".verdi/specs/archive/other-story/spec.md":    archivedStorySpecMD("other-story", "spec/other-feature#ac-1"),
				".verdi/specs/archive/noreport-story/spec.md": archivedStorySpecMD("noreport-story", "spec/my-feature#ac-1"),
			},
			Message: "land three closed, archived stories",
		},
	})
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	// Present on disk under specs/archive/ (the glob finds and decodes it),
	// but never committed — never reachable from the default branch at this
	// path, so specstate resolves it Proposed/RelationNew, not Closed.
	writeTestFile(t, store.ArchiveSpecPath(repo.Dir, "unlanded-story"), []byte(archivedStorySpecMD("unlanded-story", "spec/my-feature#ac-1")))

	// Each story WITH a report carries a dispositioned judged ruling; noreport-story
	// deliberately has none.
	writeFeatureStaleDeviationReport(t, repo.Dir, store.ZoneArchive, "impl-story", "  - { id: judged-impl, kind: judged, text: impl ruling, disposition: accepted-deviation, note: n }\n", "")
	writeFeatureStaleDeviationReport(t, repo.Dir, store.ZoneArchive, "other-story", "  - { id: judged-other, kind: judged, text: other ruling, disposition: accepted-deviation, note: n }\n", "")
	writeFeatureStaleDeviationReport(t, repo.Dir, store.ZoneArchive, "unlanded-story", "  - { id: judged-unlanded, kind: judged, text: unlanded ruling, disposition: accepted-deviation, note: n }\n", "")

	rulings, err := gatherArchivedRulings(context.Background(), repo.Dir, "my-feature", specstate.NewProjector())
	if err != nil {
		t.Fatalf("gatherArchivedRulings: %v", err)
	}
	if len(rulings) != 1 {
		t.Fatalf("gatherArchivedRulings returned %d rulings, want exactly 1 (only the confirmed-closed, implementing story with a report): %+v", len(rulings), rulings)
	}
	if rulings[0].Finding.ID != "judged-impl" || rulings[0].Source != "spec/impl-story" {
		t.Fatalf("ruling = %+v, want judged-impl sourced from spec/impl-story", rulings[0])
	}
}

// TestGatherArchivedRulings_IncludesNotResurfacedRulings proves the gathering
// draws standing rulings from BOTH the archived report's findings: and its
// not-resurfaced: section — both are dispositioned judged rulings the budget
// counts, so both are valid cross-level carry candidates.
func TestGatherArchivedRulings_IncludesNotResurfacedRulings(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                       "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/specs/archive/impl-story/spec.md": archivedStorySpecMD("impl-story", "spec/my-feature#ac-1"),
			},
			Message: "land one closed, archived, implementing story",
		},
	})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	writeFeatureStaleDeviationReport(t, repo.Dir, store.ZoneArchive, "impl-story",
		"  - { id: judged-live, kind: judged, text: live ruling, disposition: accepted-deviation, note: n }\n",
		"  - { id: judged-persisted, kind: judged, text: persisted ruling, disposition: accepted-deviation, note: n }\n")

	rulings, err := gatherArchivedRulings(context.Background(), repo.Dir, "my-feature", specstate.NewProjector())
	if err != nil {
		t.Fatalf("gatherArchivedRulings: %v", err)
	}
	got := map[string]bool{}
	for _, r := range rulings {
		got[r.Finding.ID] = true
	}
	if !got["judged-live"] || !got["judged-persisted"] {
		t.Fatalf("gathered ids = %v, want both the findings: and not-resurfaced: rulings", got)
	}
}

// TestRunDisposition_ConfirmsArchivedCandidate_StampsCarriedFrom is item 2's
// end-to-end confirmation proof (ledger L-N14 companion): a feature report whose
// align pre-filled a CROSS-LEVEL candidate (the fresh feature finding is
// undispositioned; the archived story ruling sits in not-resurfaced as its
// backing) is confirmed by the ordinary disposition verb — UNCHANGED — which
// stamps carried-from because the confirmed decision equals the backing ruling.
// The archived ruling and this confirmed feature reaffirmation are thereby the
// same deviation; carried-from is the durable signal the union collapse reads.
func TestRunDisposition_ConfirmsArchivedCandidate_StampsCarriedFrom(t *testing.T) {
	root := t.TempDir()
	covers := strings.Repeat("a", 40)

	fresh := artifact.Finding{ID: "judged-retry-semantics", Kind: artifact.FindingJudged, Text: "NEW feature-level wording"}
	backing := artifact.Finding{ID: "judged-retry-semantics", Kind: artifact.FindingJudged, Text: "OLD story-level wording", Disposition: artifact.FindingAcceptedDeviation, Note: "owner-ratified"}
	candidates := map[string]align.JudgedCandidate{fresh.ID: {
		OldDisposition: backing.Disposition, OldText: backing.Text, OldNote: backing.Note, ArchiveSource: "spec/judge-ergonomics",
	}}
	body := align.RenderBody([]artifact.Finding{fresh}, candidates, []artifact.Finding{backing}, nil, nil, nil)
	fm := &artifact.DeviationFrontmatter{
		Schema:        "verdi.deviation/v1",
		Covers:        covers,
		Findings:      []artifact.Finding{fresh},
		NotResurfaced: []artifact.Finding{backing},
		Digest:        "sha256:" + strings.Repeat("b", 64),
	}
	if err := fm.Validate(); err != nil {
		t.Fatalf("setup: cross-level candidate report invalid: %v", err)
	}
	writeTestFile(t, store.DeviationReportPath(root, store.ZoneActive, "my-feature"), align.RenderMarkdown(fm, body))

	var out, errOut bytes.Buffer
	rc := runDisposition(root, "spec/my-feature", "judged-retry-semantics", artifact.FindingAcceptedDeviation, "reaffirmed at the feature level", false, &out, &errOut)
	if rc != 0 {
		t.Fatalf("runDisposition exit = %d, want 0; stderr=%q", rc, errOut.String())
	}

	decoded, err := loadDeviationReportIfExists(store.DeviationReportPath(root, store.ZoneActive, "my-feature"))
	if err != nil || decoded == nil {
		t.Fatalf("re-reading report: %v", err)
	}
	if decoded.Findings[0].Disposition != artifact.FindingAcceptedDeviation {
		t.Fatalf("finding disposition = %q, want accepted-deviation", decoded.Findings[0].Disposition)
	}
	if decoded.Findings[0].CarriedFrom != covers {
		t.Fatalf("finding CarriedFrom = %q, want the covering head %q (a confirmed cross-level reaffirmation)", decoded.Findings[0].CarriedFrom, covers)
	}
	if len(decoded.NotResurfaced) != 0 {
		t.Fatalf("NotResurfaced = %+v, want the backing record removed on confirmation", decoded.NotResurfaced)
	}
}

// TestCheckFeatureSpecStaleCondition_ConfirmedCrossLevelReaffirmation_CountsOnce
// closes item 2's loop at the gate: a CONFIRMED feature-level reaffirmation
// (carried-from set) of an archived story ruling and the archived ruling itself
// are the SAME deviation and must count ONCE in the feature-close union — even
// though the feature judge reworded the text under the same slug. Without the
// L-N14 union collapse the two reworded texts would count twice; the tally shows
// count 1.
func TestCheckFeatureSpecStaleCondition_ConfirmedCrossLevelReaffirmation_CountsOnce(t *testing.T) {
	root := t.TempDir()
	feature := featureStaleTestSpec("spec/my-feature", "ac-1")

	// The feature's own report: the confirmed reaffirmation, carrying carried-from.
	featureFindings := "  - { id: judged-retry-semantics, kind: judged, text: \"NEW feature-level wording\", disposition: accepted-deviation, note: reaffirmed, carried-from: " + strings.Repeat("a", 40) + " }\n"
	writeFeatureStaleDeviationReport(t, root, store.ZoneActive, "my-feature", featureFindings, "")
	// The implementing story's archive: the original ruling under the same slug, old text.
	storyFindings := "  - { id: judged-retry-semantics, kind: judged, text: \"OLD story-level wording\", disposition: accepted-deviation, note: n }\n"
	writeFeatureStaleDeviationReport(t, root, store.ZoneArchive, "my-story", storyFindings, "")

	stories := []implementingStoryEdges{{SpecRef: "spec/my-story", Closed: true}}
	manifest := &store.Manifest{Audit: &store.AuditConfig{DeviationsStaleThreshold: 6}}

	cond, err := checkFeatureSpecStaleCondition(root, feature, manifest, stories, nil, nil)
	if err != nil {
		t.Fatalf("checkFeatureSpecStaleCondition: %v", err)
	}
	if !cond.OK {
		t.Fatalf("cond = %+v, want PASS (one deviation, well under threshold)", cond)
	}
	joined := strings.Join(cond.Extra, "\n")
	if !strings.Contains(joined, "accepted-deviation count 1") {
		t.Fatalf("cond.Extra = %q, want 'accepted-deviation count 1' — the confirmed reaffirmation collapses with the archived ruling it reaffirms (not 2)", cond.Extra)
	}
}
