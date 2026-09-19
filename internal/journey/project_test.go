package journey

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// eventualFixtureScaffoldLayer is layer one of TestProject_FeatureEventualFromStore's
// fixture: verdi.yaml alone (mirrors buildFactsRepo's own minimal scaffold —
// no provider config is needed since no scheme-prefixed story ref this
// fixture carries is ever resolved through a real provider).
var eventualFixtureScaffoldLayer = fixturegit.Layer{
	Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n"},
	Message: "scaffold: verdi.yaml",
}

// eventualFixtureScaffoldSHA resolves the deterministic commit SHA
// eventualFixtureScaffoldLayer builds to on its own (a throwaway prelude
// build, fixturegit.Build's own documented determinism), reused as the
// closed implementing story's frozen.commit stamp so it cites REAL git
// history in the actual two-layer build below — the same prelude-then-
// reuse trick cmd/verdi/closefeature_test.go's featureCloseScaffoldSHA
// uses for an identical chicken-and-egg reason (a committed file cannot
// embed its own future commit SHA).
func eventualFixtureScaffoldSHA(t *testing.T) string {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{eventualFixtureScaffoldLayer}).Head
}

// eventualFixtureFeatureSpecMD renders the fixture feature: two stubs, one
// per AC (stub-one -> ac-1, stub-two -> ac-2) — stub-one is realized by
// checkout-story-one (closed, implements ac-1) below; stub-two has no
// implementing story at all, so it reconciles unreconciled. Both ACs
// declare evidence: [attestation]; only ac-1's is authored on disk (see
// this file's Files map), so ac-2's outcome floor stays unsatisfied.
const eventualFixtureFeatureSpecMD = `---
id: spec/checkout
kind: spec
class: feature
title: "Checkout"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "the first fixture outcome holds", evidence: [attestation] }
  - { id: ac-2, text: "the second fixture outcome holds", evidence: [attestation] }
stubs:
  - { slug: stub-one, acceptance_criteria: [ac-1] }
  - { slug: stub-two, acceptance_criteria: [ac-2] }
---
# Checkout
`

// eventualFixtureStorySpecMD renders the one implementing story: already
// closed (archive zone, status: closed, a frozen stamp citing real
// history), implementing the feature's ac-1.
func eventualFixtureStorySpecMD(scaffoldSHA string) string {
	return `---
id: spec/checkout-story-one
kind: spec
class: story
title: "Checkout story one"
status: closed
owners: [platform-team]
story: jira:CHECKOUT-1
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/checkout#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the story's own obligation holds", evidence: [static] }
frozen: { at: 2024-01-01, commit: ` + scaffoldSHA + ` }
---
# Checkout story one
`
}

// eventualFixtureStorySpecMDCandidate renders a SECOND target for the
// "story spec ... gets only later-transition items" half of
// TestProject_FeatureEventualFromStore — a plain, un-closed story implementing
// nothing, statusless and landed exactly like the feature above (so it
// resolves accepted-pending-build too, joining the SAME candidate/later
// split the feature does), carrying only an author-vouch obligation.
const eventualFixtureStorySpecMDCandidate = `---
id: spec/checkout-story-two
kind: spec
class: story
title: "Checkout story two"
owners: [platform-team]
story: jira:CHECKOUT-2
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/checkout#ac-2" }
acceptance_criteria:
  - { id: ac-1, text: "the story's own obligation holds", evidence: [static] }
---
# Checkout story two
`

// eventualFixtureAttestationMD renders an AUTHORED (no scaffold marker)
// attestation for the feature's ac-1: evidence.LoadAttestationState itself
// only does a raw substring check over the whole file for
// evidence.UnauthoredAttestationMarker, but internal/index's own walk
// decodes EVERY file under .verdi/ (attestations/ included,
// internal/index/walk.go's "attestation" kind dispatch to
// artifact.DecodeAttestation) — including it in the fixture at all, for
// index.Build (both journey ports call it) to succeed, requires a real,
// strict-decodable attestation frontmatter, not bare prose.
func eventualFixtureAttestationMD(scaffoldSHA string) string {
	return `---
id: attestation/checkout--ac-1
kind: attestation
title: "Checkout ac-1 attestation"
owners: [platform-team]
frozen: { at: 2024-01-01, commit: ` + scaffoldSHA + ` }
---
I verified ac-1 holds by manual review.
`
}

// buildEventualFixtureRepo builds the fixturegit repo TestProject_FeatureEventualFromStore
// projects over: the feature (two stubs, one AC attested), its one closed
// implementing story, and the second (un-closed, candidate-joining) story.
func buildEventualFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	scaffoldSHA := eventualFixtureScaffoldSHA(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return fixturegit.Build(t, []fixturegit.Layer{
		eventualFixtureScaffoldLayer,
		{
			Files: map[string]string{
				".verdi/specs/active/checkout/spec.md":            eventualFixtureFeatureSpecMD,
				".verdi/specs/archive/checkout-story-one/spec.md": eventualFixtureStorySpecMD(scaffoldSHA),
				".verdi/specs/active/checkout-story-two/spec.md":  eventualFixtureStorySpecMDCandidate,
				".verdi/attestations/checkout/ac-1.md":            eventualFixtureAttestationMD(scaffoldSHA),
			},
			Message: "add checkout feature + its implementing stories",
		},
	})
}

// TestProject_FeatureEventualFromStore proves the whole wiring end to end
// over a real fixturegit-backed store, using the REAL production adapters
// (NewProjector — real stubReconciler{}/featureFolder{} over real
// matrixprojection/evidence machinery): a feature with two stubs (one
// implemented by a closed story, one with no implementing story at all)
// and one AC with no attestation derives BOTH the unreconciled stub and
// the unsatisfied outcome floor as eventual items; the record round-trips
// through Canonical/Decode; a story spec in the same store gets only
// later-transition eventual items (never the feature-only sources).
func TestProject_FeatureEventualFromStore(t *testing.T) {
	repo := buildEventualFixtureRepo(t)
	cfg := openConfig(t, repo.Dir)

	t.Run("feature target", func(t *testing.T) {
		rec, err := NewProjector().Project(context.Background(), cfg, "spec/checkout")
		if err != nil {
			t.Fatalf("Project: %v", err)
		}
		if !rec.Blockers.Eventual.Derived {
			t.Fatalf("Blockers.Eventual.Derived = false, want true: %+v", rec.Blockers.Eventual)
		}
		ids := blockerIDs(rec.Blockers.Eventual.Items)
		wantStub := findBlocker(rec.Blockers.Eventual.Items, "stub-unreconciled/stub-two")
		wantFloor := findBlocker(rec.Blockers.Eventual.Items, "outcome-floor/ac-2")
		if wantStub == nil || wantFloor == nil {
			t.Fatalf("items = %v, want both stub-unreconciled/stub-two and outcome-floor/ac-2", ids)
		}
		if findBlocker(rec.Blockers.Eventual.Items, "stub-unreconciled/stub-one") != nil {
			t.Fatalf("items = %v, stub-one is realized-by a closed story and must NOT appear", ids)
		}
		if findBlocker(rec.Blockers.Eventual.Items, "outcome-floor/ac-1") != nil {
			t.Fatalf("items = %v, ac-1 is attested and its floor must NOT appear", ids)
		}

		// Full canonical round trip: Validate, digest, strict Decode.
		data, err := Canonical(rec)
		if err != nil {
			t.Fatalf("Canonical: %v", err)
		}
		decoded, err := Decode(data)
		if err != nil {
			t.Fatalf("Decode(Canonical(rec)): %v", err)
		}
		if !decoded.Blockers.Eventual.Derived || len(decoded.Blockers.Eventual.Items) != len(rec.Blockers.Eventual.Items) {
			t.Fatalf("round-tripped eventual section = %+v, want it to match the original", decoded.Blockers.Eventual)
		}
	})

	t.Run("story target gets only later-transition items", func(t *testing.T) {
		rec, err := NewProjector().Project(context.Background(), cfg, "spec/checkout-story-two")
		if err != nil {
			t.Fatalf("Project: %v", err)
		}
		if !rec.Blockers.Eventual.Derived {
			t.Fatalf("Blockers.Eventual.Derived = false, want true: %+v", rec.Blockers.Eventual)
		}
		for _, b := range rec.Blockers.Eventual.Items {
			for _, prefix := range []string{"stub-unreconciled/", "outcome-floor/", "question-claimed/"} {
				if len(b.ID) >= len(prefix) && b.ID[:len(prefix)] == prefix {
					t.Fatalf("story target items = %v, must carry NO feature-only source item, got %q", blockerIDs(rec.Blockers.Eventual.Items), b.ID)
				}
			}
		}
		if len(rec.Blockers.Eventual.Items) == 0 {
			t.Fatalf("story target items = %v, want at least the later-transition obligation item (a non-vacuous proof)", rec.Blockers.Eventual.Items)
		}
	})
}
