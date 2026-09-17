package supersede

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// minimalPredecessor is the smallest valid, decodable, accepted-shaped
// feature spec Compose can compose from: one acceptance criterion, no
// constraints/decisions/open_questions/links/stubs at all — proves Compose
// handles the all-blocks-empty-except-AC edge case without any special
// casing (02 §Kind registry: only acceptance_criteria is required).
const minimalPredecessor = `---
id: spec/widget
kind: spec
class: feature
title: "Widget"
owners: [platform-team]
problem: { text: "widgets are hard to find", anchor: "#problem" }
outcome: { text: "widgets are easy to find", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a user can find a widget", evidence: [attestation], anchor: "#ac-1" }
---
# Widget

## Problem

Widgets are hard to find.

## Outcome

Widgets are easy to find.

## Ac 1

A user can find a widget.
`

// TestCompose_MinimalPredecessor_CarriesTheOneAC is the RED-first proof:
// the smallest predecessor Compose can be asked to compose from, with the
// smallest assertion set (the composed manifest carries the one AC id, the
// supersedes ref names the predecessor, and the rendered content carries
// the successor's own id).
func TestCompose_MinimalPredecessor_CarriesTheOneAC(t *testing.T) {
	got, err := Compose(ComposeInput{
		PredecessorName: "widget",
		PredecessorRaw:  []byte(minimalPredecessor),
		SuccessorName:   "widget-v2",
	})
	if err != nil {
		t.Fatalf("Compose = %v, want no error", err)
	}
	if len(got.CarriedIDs) != 1 || got.CarriedIDs[0] != "ac-1" {
		t.Fatalf("CarriedIDs = %v, want [ac-1]", got.CarriedIDs)
	}
	if got.SupersedesRef != "spec/widget" {
		t.Fatalf("SupersedesRef = %q, want spec/widget", got.SupersedesRef)
	}
	if !strings.Contains(string(got.Content), "id: spec/widget-v2") {
		t.Fatalf("Content = %s, want the successor id spec/widget-v2", got.Content)
	}
}

// richPredecessor exercises every object kind, both stub shapes (plain and
// spike), a fragment (non-supersedes) link, and a non-default frontmatter
// field (impacts, owners with two entries) — so a single test proves
// verbatim copying survives every field Compose must not touch.
const richPredecessor = `---
id: spec/rich
kind: spec
class: feature
title: "Rich feature (fixture)"
owners: [platform-team, qa-lead]
impacts: [loansvc, notification-svc]
problem: { text: "borrowers cannot self-serve", anchor: "#problem" }
outcome: { text: "borrowers can self-serve", anchor: "#outcome" }
links:
  - { type: exempts, ref: "adr/0012-something" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can update their application", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a borrower can see the change reflected", evidence: [behavioral, attestation], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "must not allow a duplicate submission", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "use optimistic locking on the application row", anchor: "#dc-1" }
open_questions:
  - { id: oq-1, text: "should a partial update be allowed", anchor: "#oq-1" }
stubs:
  - { slug: rich-api, acceptance_criteria: [ac-1] }
  - { slug: rich-spike, spike: true, resolves: [oq-1] }
---
# Rich feature (fixture)

Body prose the successor must carry byte-for-byte.
`

// TestCompose_RichPredecessor_CopiesEverythingVerbatimExceptTheExceptions
// proves: carried ids collect every object id across all four blocks, in
// 02 §Object model's own fixed block order; impacts/owners (non-default
// fields) survive byte-for-byte; both stub shapes survive verbatim; the
// fragment exempts link is KEPT (never a whole-spec predecessor, so
// WholeSpecSupersedesRefs never targets it); exactly one supersedes link
// exists in the output, naming the predecessor; and the body is carried
// byte-for-byte.
func TestCompose_RichPredecessor_CopiesEverythingVerbatimExceptTheExceptions(t *testing.T) {
	got, err := Compose(ComposeInput{
		PredecessorName: "rich",
		PredecessorRaw:  []byte(richPredecessor),
		SuccessorName:   "rich-v2",
	})
	if err != nil {
		t.Fatalf("Compose = %v, want no error", err)
	}

	wantCarried := []string{"ac-1", "ac-2", "co-1", "dc-1", "oq-1"}
	if len(got.CarriedIDs) != len(wantCarried) {
		t.Fatalf("CarriedIDs = %v, want %v", got.CarriedIDs, wantCarried)
	}
	for i, id := range wantCarried {
		if got.CarriedIDs[i] != id {
			t.Fatalf("CarriedIDs[%d] = %q, want %q (declaration order: ac/co/dc/oq) — got %v", i, got.CarriedIDs[i], id, got.CarriedIDs)
		}
	}

	content := string(got.Content)
	for _, verbatim := range []string{
		"impacts: [loansvc, notification-svc]",
		"owners: [platform-team, qa-lead]",
		"- { slug: rich-api, acceptance_criteria: [ac-1] }",
		"- { slug: rich-spike, spike: true, resolves: [oq-1] }",
		"- { id: ac-1, text: \"a borrower can update their application\", evidence: [attestation], anchor: \"#ac-1\" }",
		"- { id: dc-1, text: \"use optimistic locking on the application row\", anchor: \"#dc-1\" }",
		"Body prose the successor must carry byte-for-byte.",
		"title: \"Rich feature (fixture)\"",
	} {
		if !strings.Contains(content, verbatim) {
			t.Errorf("Content missing verbatim span %q\ngot:\n%s", verbatim, content)
		}
	}

	if !strings.Contains(content, `{ type: exempts, ref: "adr/0012-something" }`) {
		t.Errorf("Content dropped the kept fragment/non-supersedes link:\n%s", content)
	}
	if got := strings.Count(content, "type: supersedes"); got != 1 {
		t.Errorf("supersedes link count = %d, want exactly 1:\n%s", got, content)
	}
	if !strings.Contains(content, `ref: "spec/rich"`) {
		t.Errorf("Content missing the new supersedes ref to the predecessor:\n%s", content)
	}
	if !strings.Contains(content, "id: spec/rich-v2") {
		t.Errorf("Content missing the successor's own id:\n%s", content)
	}
}

// widgetV2SupersedingV1 is a "v2" predecessor that itself already
// superseded a "v1": it carries its OWN inherited supersedes link and
// supersession block, classifying v1's objects. Composing v3 from it must
// REPLACE — never append to — both: v3 names v2 as its sole predecessor
// and classifies v2's OWN objects, never v1's.
const widgetV2SupersedingV1 = `---
id: spec/widget-v2
kind: spec
class: feature
title: "Widget v2"
owners: [platform-team]
problem: { text: "widgets are hard to find", anchor: "#problem" }
outcome: { text: "widgets are easy to find and reservable", anchor: "#outcome" }
links:
  - { type: supersedes, ref: "spec/widget" }
acceptance_criteria:
  - { id: ac-1, text: "a user can find a widget", evidence: [attestation], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "a reservation expires after 10 minutes", anchor: "#co-1" }
supersession:
  carried: [co-1]
  amended: [ { id: ac-1, note: "widened to include reservation" } ]
  amended_advisory: []
  removed: []
  added: []
---
# Widget v2

Body.
`

// TestCompose_PredecessorAlreadySuperseding_ReplacesNotAppends is the
// v2->v3 case (dispatch contract, part A's "predecessor already carrying a
// supersedes link and supersession block"): a table row asserting SUCCESS
// with specific replace-not-append assertions, not an error — see this
// lane's final report for why this reads as a positive transformation-
// correctness proof rather than a refusal (the contract's own "negative
// cases" enumeration groups shape-edge-cases together; this is the one
// entry among them that Compose must actually SUCCEED at, since chained
// supersession — v1->v2->v3 — is an ordinary, legal history, not an error
// condition; R3-4 controller ruling: "a predecessor that itself carries
// supersession: and a supersedes link has both replaced").
func TestCompose_PredecessorAlreadySuperseding_ReplacesNotAppends(t *testing.T) {
	got, err := Compose(ComposeInput{
		PredecessorName: "widget-v2",
		PredecessorRaw:  []byte(widgetV2SupersedingV1),
		SuccessorName:   "widget-v3",
	})
	if err != nil {
		t.Fatalf("Compose = %v, want no error", err)
	}

	wantCarried := []string{"ac-1", "co-1"}
	if len(got.CarriedIDs) != len(wantCarried) || got.CarriedIDs[0] != wantCarried[0] || got.CarriedIDs[1] != wantCarried[1] {
		t.Fatalf("CarriedIDs = %v, want %v (v2's OWN objects, not v1's)", got.CarriedIDs, wantCarried)
	}
	if got.SupersedesRef != "spec/widget-v2" {
		t.Fatalf("SupersedesRef = %q, want spec/widget-v2", got.SupersedesRef)
	}

	content := string(got.Content)
	if strings.Contains(content, `ref: "spec/widget"`) {
		t.Errorf("Content still carries v2's own inherited supersedes link to v1 — must be replaced, not appended:\n%s", content)
	}
	if got := strings.Count(content, "type: supersedes"); got != 1 {
		t.Errorf("supersedes link count = %d, want exactly 1 (the old v1 link must be replaced):\n%s", got, content)
	}
	if strings.Contains(content, "widened to include reservation") {
		t.Errorf("Content still carries v2's own inherited amended note about v1 — the fresh supersession: block must classify v2's OWN objects, not describe v1's:\n%s", content)
	}
	if !strings.Contains(content, "carried: [ac-1, co-1]") {
		t.Errorf("Content missing the fresh carried list classifying v2's own objects:\n%s", content)
	}
	if !strings.Contains(content, "amended: []") {
		t.Errorf("Content must classify nothing amended at scaffold time (author reclassifies later):\n%s", content)
	}
}

// widgetWithLegacyFields is a predecessor shaped like a v0/legacy-era
// accepted spec: a persisted `status:` line and a `frozen: {...}` stamp
// (both still decode-legal — 02: "OPTIONAL for the feature and story
// classes ... A persisted legacy value ... still decodes"). Compose must
// drop both: the successor is a fresh draft, and copying the predecessor's
// OWN acceptance stamp forward would misrepresent the successor as already
// accepted.
const widgetWithLegacyFields = `---
id: spec/widget-legacy
kind: spec
class: feature
title: "Widget (legacy fields fixture)"
owners: [platform-team]
status: accepted-pending-build
problem: { text: "widgets are hard to find", anchor: "#problem" }
outcome: { text: "widgets are easy to find", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a user can find a widget", evidence: [attestation], anchor: "#ac-1" }
frozen: { at: 2026-01-01, commit: 0123456789abcdef0123456789abcdef01234567 }
---
# Widget (legacy fields fixture)

Body.
`

// TestCompose_PredecessorWithLegacyStatusAndFrozen_DropsBoth proves the
// drop rule for both legacy fields, regardless of their position relative
// to other top-level keys (status: sits before problem/outcome here,
// frozen: sits after acceptance_criteria — non-adjacent, exactly like the
// real examples/showcase rate-lock-v2 fixture's own field order).
func TestCompose_PredecessorWithLegacyStatusAndFrozen_DropsBoth(t *testing.T) {
	got, err := Compose(ComposeInput{
		PredecessorName: "widget-legacy",
		PredecessorRaw:  []byte(widgetWithLegacyFields),
		SuccessorName:   "widget-legacy-v2",
	})
	if err != nil {
		t.Fatalf("Compose = %v, want no error", err)
	}
	content := string(got.Content)
	if strings.Contains(content, "status:") {
		t.Errorf("Content still carries the predecessor's legacy status: field:\n%s", content)
	}
	if strings.Contains(content, "frozen:") {
		t.Errorf("Content still carries the predecessor's legacy frozen: stamp:\n%s", content)
	}
}

// storyPredecessor is a class: story spec — supersession is feature-only
// (02 §Kind registry) — used to prove Compose's own self-validation
// (designscaffold.CheckClass) refuses it as a defense-in-depth backstop,
// independent of Resolve's own purpose-built class guard (resolve_test.go).
const storyPredecessor = `---
id: spec/some-story
kind: spec
class: story
title: "Some story"
owners: [platform-team]
story: jira:LOAN-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "a", evidence: [static], anchor: "#ac-1" }
---
# Some story

## Problem

p.

## Outcome

o.

## Ac 1

a.
`

// TestCompose_Negative is Compose's own table of shape-edge and error
// cases (dispatch contract part A's enumerated list): a story predecessor
// (feature-only), non-decodable bytes (no frontmatter delimiters at all),
// and a predecessor whose frontmatter fails DecodeSpec's own duplicate-id
// validation (the "duplicate ids" case: a malformed predecessor never
// reaches this package's own carried-id logic at all — DecodeSpec rejects
// it first, so this proves Compose surfaces that failure rather than
// panicking or silently producing a manifest with a duplicated id).
func TestCompose_Negative(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantErrSub string
	}{
		{
			name:       "story predecessor",
			raw:        storyPredecessor,
			wantErrSub: "feature",
		},
		{
			name:       "non-decodable bytes: no frontmatter delimiters",
			raw:        "# just a markdown doc\n\nno frontmatter here\n",
			wantErrSub: "frontmatter delimiter",
		},
		{
			name: "non-decodable bytes: duplicate acceptance criterion id",
			raw: `---
id: spec/dup
kind: spec
class: feature
title: "Dup"
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "first", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-1, text: "second", evidence: [attestation], anchor: "#ac-1" }
---
# Dup
`,
			wantErrSub: "duplicated",
		},
		{
			name: "non-decodable bytes: unknown field",
			raw: `---
id: spec/unknownfield
kind: spec
class: feature
title: "Unknown field"
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a", evidence: [attestation], anchor: "#ac-1" }
not_a_real_field: true
---
# Unknown field
`,
			wantErrSub: "strict decode",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compose(ComposeInput{
				PredecessorName: "pred",
				PredecessorRaw:  []byte(tc.raw),
				SuccessorName:   "succ",
			})
			if err == nil {
				t.Fatal("Compose = nil error, want a refusal")
			}
			if !strings.Contains(err.Error(), tc.wantErrSub) {
				t.Fatalf("Compose error = %q, want it to contain %q", err.Error(), tc.wantErrSub)
			}
		})
	}
}

// TestDeclarationOrderIDs_EmptyBlocksAreFine is a direct unit test of the
// "predecessor with zero objects of some kind" edge case at the helper
// level: constraints/decisions/open_questions are all individually
// optional (02 §Kind registry) — declarationOrderIDs must not panic or
// misbehave on nil slices, the common case for most real feature specs.
func TestDeclarationOrderIDs_EmptyBlocksAreFine(t *testing.T) {
	spec := &artifact.SpecFrontmatter{
		AcceptanceCriteria: []artifact.AcceptanceCriterion{{ID: "ac-1"}, {ID: "ac-2"}},
	}
	got := declarationOrderIDs(spec)
	want := []string{"ac-1", "ac-2"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("declarationOrderIDs = %v, want %v", got, want)
	}
}

// quotedKeyPredecessor writes the very fields Compose must rewrite or drop
// with QUOTED keys — `"id":`, `"status":`, `'frozen':` — which is ordinary,
// legal YAML that artifact.DecodeSpec accepts unchanged (proven by the
// decode assertions below, which read the predecessor's own values back).
// Before the fix round that added this test, topLevelKeyRe recognized only
// bare keys, so these three spans were copied VERBATIM: the successor
// silently inherited the predecessor's own acceptance stamp and id, and
// Compose's self-validation passed (a spec carrying a quoted `"status":` is
// decode-valid), so the CLI exited 0 on a successor that claimed its
// predecessor's identity and acceptance.
const quotedKeyPredecessor = `---
"id": spec/quoted
kind: spec
class: feature
title: "Quoted keys (fixture)"
owners: [platform-team]
"status": accepted-pending-build
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a", evidence: [attestation], anchor: "#ac-1" }
'frozen': { at: 2026-01-01, commit: 0123456789abcdef0123456789abcdef01234567 }
---
# Quoted keys (fixture)

Body.
`

// TestCompose_QuotedTopLevelKeys_AreHandledLikeUnquotedOnes proves the
// quoted spelling of every key Compose special-cases is handled exactly
// like the bare spelling — asserted on the DECODED successor (the shape
// that actually matters: what the next reader of these bytes sees), not on
// a substring of the rendered text.
func TestCompose_QuotedTopLevelKeys_AreHandledLikeUnquotedOnes(t *testing.T) {
	// The predecessor really does decode with both legacy fields set —
	// otherwise this fixture would prove nothing about dropping them.
	predFM, _, err := artifact.SplitFrontmatter([]byte(quotedKeyPredecessor))
	if err != nil {
		t.Fatalf("SplitFrontmatter(predecessor) = %v, want no error", err)
	}
	predSpec, err := artifact.DecodeSpec(predFM)
	if err != nil {
		t.Fatalf("DecodeSpec(predecessor) = %v, want no error (quoted keys are legal YAML)", err)
	}
	if predSpec.Status == "" || predSpec.Frozen == nil || predSpec.ID != "spec/quoted" {
		t.Fatalf("predecessor fixture decoded as status=%q frozen=%v id=%q, want all three set through their quoted keys", predSpec.Status, predSpec.Frozen, predSpec.ID)
	}

	got, err := Compose(ComposeInput{
		PredecessorName: "quoted",
		PredecessorRaw:  []byte(quotedKeyPredecessor),
		SuccessorName:   "quoted-v2",
	})
	if err != nil {
		t.Fatalf("Compose = %v, want no error", err)
	}

	outFM, _, err := artifact.SplitFrontmatter(got.Content)
	if err != nil {
		t.Fatalf("SplitFrontmatter(successor) = %v, want no error", err)
	}
	outSpec, err := artifact.DecodeSpec(outFM)
	if err != nil {
		t.Fatalf("DecodeSpec(successor) = %v, want no error", err)
	}
	if outSpec.ID != "spec/quoted-v2" {
		t.Errorf("successor id = %q, want spec/quoted-v2 (a quoted \"id\": key must be rewritten too)", outSpec.ID)
	}
	if outSpec.Status != "" {
		t.Errorf("successor status = %q, want it dropped (a quoted \"status\": key must be dropped too)", outSpec.Status)
	}
	if outSpec.Frozen != nil {
		t.Errorf("successor frozen = %+v, want it dropped (a quoted 'frozen': key must be dropped too)", outSpec.Frozen)
	}
}

// TestCheckComposedPostconditions is the direct table test of Compose's
// own post-condition assertion — the backstop that makes any future text
// surgery bug fail CLOSED with the offending field named, instead of
// silently shipping a successor that inherits what it must not. Each row
// hand-builds an "already composed" decode result and asserts the field
// name appears in the refusal.
func TestCheckComposedPostconditions(t *testing.T) {
	// good is the shape a correct Compose produces for predecessor
	// spec/pred -> successor spec/succ carrying objects ac-1 and co-1.
	good := func() *artifact.SpecFrontmatter {
		return &artifact.SpecFrontmatter{
			Base: artifact.Base{
				ID:    "spec/succ",
				Links: []artifact.Link{{Type: artifact.LinkSupersedes, Ref: "spec/pred"}},
			},
			Supersession: &artifact.Supersession{Carried: []string{"ac-1", "co-1"}},
		}
	}
	carried := []string{"ac-1", "co-1"}

	t.Run("a correctly composed successor passes", func(t *testing.T) {
		if err := checkComposedPostconditions(good(), "succ", "spec/pred", carried); err != nil {
			t.Fatalf("checkComposedPostconditions = %v, want nil", err)
		}
	})

	cases := []struct {
		name    string
		mutate  func(*artifact.SpecFrontmatter)
		wantSub string
	}{
		{
			name:    "successor keeps the predecessor's id",
			mutate:  func(s *artifact.SpecFrontmatter) { s.ID = "spec/pred" },
			wantSub: "id",
		},
		{
			name:    "successor inherits a status stamp",
			mutate:  func(s *artifact.SpecFrontmatter) { s.Status = artifact.Status("accepted-pending-build") },
			wantSub: "status",
		},
		{
			name: "successor inherits a frozen stamp",
			mutate: func(s *artifact.SpecFrontmatter) {
				s.Frozen = &artifact.Frozen{At: "2026-01-01", Commit: "0123456789abcdef0123456789abcdef01234567"}
			},
			wantSub: "frozen",
		},
		{
			name:    "no whole-spec supersedes link at all",
			mutate:  func(s *artifact.SpecFrontmatter) { s.Links = nil },
			wantSub: "supersedes",
		},
		{
			name: "the predecessor's own inherited supersedes link survived",
			mutate: func(s *artifact.SpecFrontmatter) {
				s.Links = append(s.Links, artifact.Link{Type: artifact.LinkSupersedes, Ref: "spec/ancient"})
			},
			wantSub: "supersedes",
		},
		{
			name: "the one supersedes link names the wrong predecessor",
			mutate: func(s *artifact.SpecFrontmatter) {
				s.Links = []artifact.Link{{Type: artifact.LinkSupersedes, Ref: "spec/somebody-else"}}
			},
			wantSub: "supersedes",
		},
		{
			name:    "no supersession block at all",
			mutate:  func(s *artifact.SpecFrontmatter) { s.Supersession = nil },
			wantSub: "supersession",
		},
		{
			name: "a predecessor object went unclassified",
			mutate: func(s *artifact.SpecFrontmatter) {
				s.Supersession.Carried = []string{"ac-1"}
			},
			wantSub: "co-1",
		},
		{
			name: "carried lists an object the predecessor never declared",
			mutate: func(s *artifact.SpecFrontmatter) {
				s.Supersession.Carried = []string{"ac-1", "co-1", "ac-99"}
			},
			wantSub: "carried",
		},
		{
			name: "a fresh scaffold classifies something amended",
			mutate: func(s *artifact.SpecFrontmatter) {
				s.Supersession.Amended = []artifact.SupersessionNote{{ID: "ac-1", Note: "widened"}}
			},
			wantSub: "amended",
		},
		{
			name: "a fresh scaffold classifies something amended_advisory",
			mutate: func(s *artifact.SpecFrontmatter) {
				s.Supersession.AmendedAdvisory = []artifact.SupersessionNote{{ID: "ac-1", Note: "widened"}}
			},
			wantSub: "amended_advisory",
		},
		{
			name: "a fresh scaffold classifies something removed",
			mutate: func(s *artifact.SpecFrontmatter) {
				s.Supersession.Removed = []artifact.SupersessionNote{{ID: "co-1", Note: "gone"}}
			},
			wantSub: "removed",
		},
		{
			name: "a fresh scaffold classifies something added",
			mutate: func(s *artifact.SpecFrontmatter) {
				s.Supersession.Added = []string{"ac-9"}
			},
			wantSub: "added",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := good()
			tc.mutate(spec)
			err := checkComposedPostconditions(spec, "succ", "spec/pred", carried)
			if err == nil {
				t.Fatal("checkComposedPostconditions = nil, want a fail-closed refusal")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("checkComposedPostconditions = %q, want it to name %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// pinnedSupersedesPredecessor is a conforming v2 whose own inherited
// whole-spec supersedes link is PINNED (`spec/ancient@3e91ab2`) — the form
// 02 §Identity calls "the only form permitted in context manifests,
// evidence records, and board pins", and which internal/artifact's own
// WholeSpecSupersedesRefs treats as whole-spec (supersession_test.go).
// It also carries a FRAGMENT supersedes edge, which is a decision-level
// override and never a whole-spec predecessor (I-47), so Compose must keep
// that one untouched.
const pinnedSupersedesPredecessor = `---
id: spec/pinned
kind: spec
class: feature
title: "Pinned predecessor (fixture)"
owners: [platform-team]
links:
  - { type: supersedes, ref: "spec/ancient@3e91ab2" }
  - { type: supersedes, ref: "spec/other-thing#dc-4" }
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a", evidence: [attestation], anchor: "#ac-1" }
supersession:
  carried: [ac-1]
  amended: []
  amended_advisory: []
  removed: []
  added: []
---
# Pinned predecessor (fixture)

Body.
`

// TestCompose_PredecessorWithPinnedSupersedesLink_ReplacesIt proves the
// v2->v3 replacement rule holds for a PINNED inherited link too. Before the
// fix round that added this test, renderLinksBlock matched the link to drop
// by string equality against WholeSpecSupersedesRefs' commit-stripped
// rendering ("spec/ancient" != "spec/ancient@3e91ab2"), so the pinned link
// survived alongside the new one: two whole-spec predecessors, which I-47
// rejects at the decode seam — a conforming accepted revision could not be
// superseded at all.
func TestCompose_PredecessorWithPinnedSupersedesLink_ReplacesIt(t *testing.T) {
	got, err := Compose(ComposeInput{
		PredecessorName: "pinned",
		PredecessorRaw:  []byte(pinnedSupersedesPredecessor),
		SuccessorName:   "pinned-v2",
	})
	if err != nil {
		t.Fatalf("Compose = %v, want no error", err)
	}

	content := string(got.Content)
	if strings.Contains(content, "spec/ancient") {
		t.Errorf("Content still carries the predecessor's own pinned supersedes link to spec/ancient@3e91ab2 — it must be replaced:\n%s", content)
	}
	if !strings.Contains(content, `{ type: supersedes, ref: "spec/other-thing#dc-4" }`) {
		t.Errorf("Content dropped the fragment supersedes edge, which is never a whole-spec predecessor (I-47):\n%s", content)
	}

	outFM, _, err := artifact.SplitFrontmatter(got.Content)
	if err != nil {
		t.Fatalf("SplitFrontmatter(successor) = %v, want no error", err)
	}
	outSpec, err := artifact.DecodeSpec(outFM)
	if err != nil {
		t.Fatalf("DecodeSpec(successor) = %v, want no error (two whole-spec predecessors would fail here, I-47)", err)
	}
	refs := artifact.WholeSpecSupersedesRefs(outSpec.Links)
	if len(refs) != 1 || refs[0].String() != "spec/pinned" {
		t.Fatalf("whole-spec supersedes refs = %v, want exactly [spec/pinned]", refs)
	}
}
