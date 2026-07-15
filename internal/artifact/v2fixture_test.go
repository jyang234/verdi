package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// This file is V1-P1's "table-driven decode goldens over the extended
// examples/showcase/, mirroring PLAN.md §5 Phase 2's pattern" (brief §Test
// strategy): the v2 fixture overlay authored by this phase's §4 appendix.

// v2CorpusDir is examples/showcase relative to this package — the same tree
// internal/corpus's v0 goldens read, extended additively (nothing v0
// removed, A8).
const v2CorpusDir = "../../examples/showcase"

// v2InvalidDir holds this phase's decode-failure twins, mirroring
// testdata/corpus-invalid/'s v0 convention (see its README).
const v2InvalidDir = "../../testdata/corpus-invalid-v2"

// --- The rung-4 supersession pair's dedicated fixturegit history ---
//
// Unlike v0's examples/showcase (one shared fixturegit repo driven by
// layers.txt), this pair gets its own small, independent history: nothing
// here needs to interleave with the v0 corpus's existing golden SHAs, and
// keeping it separate means the v0 corpus's own SHA-locked tests
// (internal/corpus) need only grow their accepted-token set, never their
// git history. "fixturegit gains no new mechanism" (brief §4 appendix) —
// this is an ordinary Build() call, same as every other package's usage.
//
// Layer 1: spec/loan-workflow, DRAFT (nothing frozen yet — no prior commit
// exists to pin). Layer 2: spec/loan-workflow FROZEN (frozen.commit = layer
// 1's head) plus spec/loan-workflow-v2, DRAFT. The resulting head (layer
// 2's SHA) is what examples/showcase's committed loan-workflow-v2/spec.md
// cites as its own frozen.commit, and what the reaffirmation fixture pins
// as "v2's commit" — both baked in below as goldenShaA/goldenShaB, per the
// same "build once, bake in, test forever" contract v0's corpus_test.go
// establishes. This test rebuilds the repo on every run and asserts the
// heads still match: a change to the layer content below is a change to
// every corpus file that cites these SHAs, so drift is loud, not silent.
const (
	goldenShaA = "b5117ecc69b6779ad75cde60d4aec206ece0950b"
	goldenShaB = "06a3f4cabb226fe9344e1645e27c344493b6b62b"
)

// public-rollout-plan Task 1.5 extends this same dedicated, unchained
// history with two more layers for the rate-lock/rate-lock-v2 supersession
// pair, for the identical reason loan-workflow needed one: VL-015
// (internal/lint/vl015.go) reads a predecessor's object manifest via
// `git show <pred.frozen.commit>:<pred.RelPath>`, which only succeeds if
// the predecessor's file already exists, with matching content, AT that
// exact commit — impossible for a file whose own committed frozen.commit
// cites an ancestor from BEFORE the file was ever written (the layers.txt
// main corpus's "layer N cites layer N-1" convention, which works for
// every OTHER frozen file here only because nothing else needs to read
// their content back through git history). Layer 3: spec/rate-lock DRAFT
// (chained after layers 1-2, so it shares this same repo rather than
// opening a third independent history). Layer 4: spec/rate-lock FROZEN
// (frozen.commit = layer 3's head) plus spec/rate-lock-v2 DRAFT. The
// resulting head (layer 4's SHA) is what examples/showcase's committed
// rate-lock-v2/spec.md cites as its own frozen.commit.
const (
	goldenShaC = "620ade86bbd810b440a0d995859745d4402d7be8"
	goldenShaD = "87c65ef5e70024c112b12e275d550f1ca8584df3"
)

const v2LoanWorkflowV1Draft = `---
id: spec/loan-workflow
kind: spec
class: feature
title: "Loan workflow (v2 fixture, supersession v1)"
status: draft
owners: [platform-team]
problem: { text: "loan officers cannot see workflow status changes in real time", anchor: "#problem" }
outcome: { text: "loan officers see workflow status changes within one minute", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "workflow status changes are visible within one minute", evidence: [runtime, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "workflow history is queryable by loan id", evidence: [static, attestation], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "must not add new synchronous cross-service calls", anchor: "#co-1" }
---
# Loan workflow (v2 fixture, supersession v1)

## Problem

Loan officers only see workflow status changes on their next manual refresh.

## Outcome

Loan officers see workflow status changes within one minute of the change.

## AC-1

Workflow status changes are visible within one minute.

## AC-2

Workflow history is queryable by loan id.

## CO-1

Must not add new synchronous cross-service calls.
`

const v2LoanWorkflowV1FrozenTemplate = `---
id: spec/loan-workflow
kind: spec
class: feature
title: "Loan workflow (v2 fixture, supersession v1)"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "loan officers cannot see workflow status changes in real time", anchor: "#problem" }
outcome: { text: "loan officers see workflow status changes within one minute", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "workflow status changes are visible within one minute", evidence: [runtime, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "workflow history is queryable by loan id", evidence: [static, attestation], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "must not add new synchronous cross-service calls", anchor: "#co-1" }
frozen: { at: 2026-06-01, commit: SHA_A_PLACEHOLDER }
---
# Loan workflow (v2 fixture, supersession v1)

## Problem

Loan officers only see workflow status changes on their next manual refresh.

## Outcome

Loan officers see workflow status changes within one minute of the change.

## AC-1

Workflow status changes are visible within one minute.

## AC-2

Workflow history is queryable by loan id.

## CO-1

Must not add new synchronous cross-service calls.
`

const v2LoanWorkflowV2Draft = `---
id: spec/loan-workflow-v2
kind: spec
class: feature
title: "Loan workflow v2 (supersedes v1)"
status: draft
owners: [platform-team]
problem: { text: "loan officers cannot see workflow status changes in real time", anchor: "#problem" }
outcome: { text: "loan officers see workflow status changes within thirty seconds", anchor: "#outcome" }
links:
  - { type: supersedes, ref: spec/loan-workflow }
acceptance_criteria:
  - { id: ac-1, text: "workflow status changes are visible within thirty seconds", evidence: [runtime, attestation], anchor: "#ac-1" }
  - { id: ac-3, text: "workflow status changes emit an audit event", evidence: [static, attestation], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: "must not add new synchronous cross-service calls", anchor: "#co-1" }
supersession:
  carried: [co-1]
  amended: [ { id: ac-1, note: "tightened the visibility threshold from one minute to thirty seconds" } ]
  amended_advisory: []
  removed: [ { id: ac-2, note: "workflow-history query moved to a separate reporting feature" } ]
  added: [ac-3]
---
# Loan workflow v2 (supersedes v1)

## Problem

Loan officers only see workflow status changes on their next manual refresh.

## Outcome

Loan officers see workflow status changes within thirty seconds of the change.

## AC-1

Workflow status changes are visible within thirty seconds.

## AC-3

Workflow status changes emit an audit event.

## CO-1

Must not add new synchronous cross-service calls.
`

const v2RateLockV1Draft = `---
id: spec/rate-lock
kind: spec
class: feature
title: "Rate lock (fixture, superseded feature)"
owners: [platform-team]
status: draft
problem: { text: "borrowers lose a good quoted rate the moment they pause the application", anchor: "#problem" }
outcome: { text: "borrowers can lock a quoted rate for a fixed window and finish later", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can lock a quoted rate for 30 days", evidence: [static, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a locked rate survives a session restart", evidence: [static], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "must not lock a rate the pricing service has already retired", anchor: "#co-1" }
---
# Rate lock (fixture, superseded feature)

## Problem

Borrowers lose a good quoted rate the moment they pause the application.

## Outcome

Borrowers can lock a quoted rate for a fixed window and finish later.

## AC-1

A borrower can lock a quoted rate for 30 days.

## AC-2

A locked rate survives a session restart.

## CO-1

Must not lock a rate the pricing service has already retired.
`

const v2RateLockV1FrozenTemplate = `---
id: spec/rate-lock
kind: spec
class: feature
title: "Rate lock (fixture, superseded feature)"
owners: [platform-team]
status: superseded
problem: { text: "borrowers lose a good quoted rate the moment they pause the application", anchor: "#problem" }
outcome: { text: "borrowers can lock a quoted rate for a fixed window and finish later", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can lock a quoted rate for 30 days", evidence: [static, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a locked rate survives a session restart", evidence: [static], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "must not lock a rate the pricing service has already retired", anchor: "#co-1" }
frozen: { at: 2026-07-11, commit: SHA_C_PLACEHOLDER }
---
# Rate lock (fixture, superseded feature)

## Problem

Borrowers lose a good quoted rate the moment they pause the application.

## Outcome

Borrowers can lock a quoted rate for a fixed window and finish later.

## AC-1

A borrower can lock a quoted rate for 30 days.

## AC-2

A locked rate survives a session restart.

## CO-1

Must not lock a rate the pricing service has already retired.
`

const v2RateLockV2Draft = `---
id: spec/rate-lock-v2
kind: spec
class: feature
title: "Rate lock v2 (fixture, supersedes rate-lock)"
owners: [platform-team]
status: draft
problem: { text: "borrowers lose a good quoted rate the moment they pause the application", anchor: "#problem" }
outcome: { text: "borrowers can lock a quoted rate for a configurable window and finish later", anchor: "#outcome" }
links:
  - { type: supersedes, ref: "spec/rate-lock" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can lock a quoted rate for a configurable window", evidence: [static, attestation], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "must not lock a rate the pricing service has already retired", anchor: "#co-1" }
supersession:
  carried: [co-1]
  amended: [ { id: ac-1, note: "the fixed 30-day window becomes a window configurable per loan program, after a first-time-buyer program's 45-day underwriting cycle routinely outran the fixed lock" } ]
  amended_advisory: []
  removed: [ { id: ac-2, note: "the session-restart guarantee is subsumed by v2's own persistence layer and no longer earns its place as a feature-level AC" } ]
  added: []
---
# Rate lock v2 (fixture, supersedes rate-lock)

## Problem

Borrowers lose a good quoted rate the moment they pause the application.

## Outcome

Borrowers can lock a quoted rate for a configurable window and finish
later.

## AC-1

A borrower can lock a quoted rate for a configurable window.

## CO-1

Must not lock a rate the pricing service has already retired.
`

// TestV2SupersessionRepo_MatchesGoldenSHAs rebuilds the supersession pair's
// dedicated fixturegit history from the layer content above and proves it
// still reproduces goldenShaA/goldenShaB — the SHAs
// examples/showcase/.verdi/specs/active/loan-workflow-v2/spec.md and
// examples/showcase/.verdi/reaffirmations/jira-loan-1483/ac-1.md cite.
func TestV2SupersessionRepo_MatchesGoldenSHAs(t *testing.T) {
	layer1 := fixturegit.Layer{
		Files:   map[string]string{".verdi/specs/active/loan-workflow/spec.md": v2LoanWorkflowV1Draft},
		Message: "v2 layer 1: loan-workflow v1 draft",
	}
	repo1 := fixturegit.Build(t, []fixturegit.Layer{layer1})
	if repo1.Head != goldenShaA {
		t.Fatalf("layer 1 head = %s, want golden %s (SHA_A)", repo1.Head, goldenShaA)
	}

	frozen := strings.Replace(v2LoanWorkflowV1FrozenTemplate, "SHA_A_PLACEHOLDER", goldenShaA, 1)
	layer2 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/specs/active/loan-workflow/spec.md":    frozen,
			".verdi/specs/active/loan-workflow-v2/spec.md": v2LoanWorkflowV2Draft,
		},
		Message: "v2 layer 2: loan-workflow v1 frozen + loan-workflow-v2 draft",
	}
	repo2 := fixturegit.Build(t, []fixturegit.Layer{layer1, layer2})
	if repo2.Head != goldenShaB {
		t.Fatalf("layer 2 head = %s, want golden %s (SHA_B)", repo2.Head, goldenShaB)
	}

	// Layers 3-4: the rate-lock/rate-lock-v2 pair, chained after layers 1-2
	// so it shares this same dedicated history rather than opening a third
	// independent one (public-rollout-plan Task 1.5).
	layer3 := fixturegit.Layer{
		Files:   map[string]string{".verdi/specs/active/rate-lock/spec.md": v2RateLockV1Draft},
		Message: "v2 layer 3: rate-lock v1 draft",
	}
	repo3 := fixturegit.Build(t, []fixturegit.Layer{layer1, layer2, layer3})
	if repo3.Head != goldenShaC {
		t.Fatalf("layer 3 head = %s, want golden %s (SHA_C)", repo3.Head, goldenShaC)
	}

	rateLockFrozen := strings.Replace(v2RateLockV1FrozenTemplate, "SHA_C_PLACEHOLDER", goldenShaC, 1)
	layer4 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/specs/active/rate-lock/spec.md":    rateLockFrozen,
			".verdi/specs/active/rate-lock-v2/spec.md": v2RateLockV2Draft,
		},
		Message: "v2 layer 4: rate-lock v1 frozen + rate-lock-v2 draft",
	}
	repo4 := fixturegit.Build(t, []fixturegit.Layer{layer1, layer2, layer3, layer4})
	if repo4.Head != goldenShaD {
		t.Fatalf("layer 4 head = %s, want golden %s (SHA_D)", repo4.Head, goldenShaD)
	}
}

// --- Decode goldens over the committed v2 corpus fixtures ---

func readCorpusFile(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(v2CorpusDir, rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return data
}

// TestV2Corpus_SpecsDecode proves every v2 spec fixture (feature spec, both
// story specs, the spike, and the rung-4 supersession pair) decodes
// strictly — the exit criterion's "every v2 corpus fixture file ...
// decodes" for the spec.md files specifically.
func TestV2Corpus_SpecsDecode(t *testing.T) {
	specs := []struct {
		rel       string
		wantClass SpecClass
		wantSpike bool
	}{
		{".verdi/specs/active/escrow-autopay/spec.md", ClassFeature, false},
		{".verdi/specs/active/borrower-update-api/spec.md", ClassStory, false},
		{".verdi/specs/active/borrower-update-mobile/spec.md", ClassStory, false},
		{".verdi/specs/active/borrower-update-mobile-spike/spec.md", ClassStory, true},
		{".verdi/specs/active/loan-workflow/spec.md", ClassFeature, false},
		{".verdi/specs/active/loan-workflow-v2/spec.md", ClassFeature, false},
		{".verdi/specs/active/rate-lock/spec.md", ClassFeature, false},
		{".verdi/specs/active/rate-lock-v2/spec.md", ClassFeature, false},
	}
	for _, sp := range specs {
		t.Run(sp.rel, func(t *testing.T) {
			data := readCorpusFile(t, sp.rel)
			fm, body, err := SplitFrontmatter(data)
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			decoded, err := DecodeSpec(fm)
			if err != nil {
				t.Fatalf("DecodeSpec: %v", err)
			}
			if decoded.Class != sp.wantClass {
				t.Fatalf("Class = %q, want %q", decoded.Class, sp.wantClass)
			}
			if decoded.Spike != sp.wantSpike {
				t.Fatalf("Spike = %v, want %v", decoded.Spike, sp.wantSpike)
			}
			if err := decoded.ResolveObjectAnchors(body); err != nil {
				t.Fatalf("ResolveObjectAnchors: %v", err)
			}
		})
	}
}

// TestV2Corpus_BoardLayoutDecodes proves the sibling layout.json decodes,
// exercising both the present-and-valid path (ac-1, ac-2, dc-1) and the
// absent-key fallback (ac-3, co-1, dc-2 are declared on the spec but have
// no stored position — 01 §notes: "an absent layout.json ... falls back to
// the zoned-incremental layout algorithm for that object").
func TestV2Corpus_BoardLayoutDecodes(t *testing.T) {
	data := readCorpusFile(t, ".verdi/specs/active/escrow-autopay/layout.json")
	bl, err := DecodeBoardLayout(data)
	if err != nil {
		t.Fatalf("DecodeBoardLayout: %v", err)
	}
	if len(bl.Positions) != 3 {
		t.Fatalf("Positions = %+v, want 3 entries (present-and-valid subset)", bl.Positions)
	}
	specData := readCorpusFile(t, ".verdi/specs/active/escrow-autopay/spec.md")
	fm, _, err := SplitFrontmatter(specData)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	declared := map[string]bool{}
	for _, ac := range spec.AcceptanceCriteria {
		declared[ac.ID] = true
	}
	for _, c := range spec.Constraints {
		declared[c.ID] = true
	}
	for _, d := range spec.Decisions {
		declared[d.ID] = true
	}
	// ac-3, co-1, dc-2 are declared but absent from positions — the
	// fallback path; every positions key must still resolve to a real
	// declared id.
	for _, wantAbsent := range []string{"ac-3", "co-1", "dc-2"} {
		if !declared[wantAbsent] {
			t.Fatalf("test setup: %s is not declared on the spec", wantAbsent)
		}
		if _, ok := bl.Positions[wantAbsent]; ok {
			t.Fatalf("test setup: %s unexpectedly has a stored position", wantAbsent)
		}
	}
	for id := range bl.Positions {
		if !declared[id] {
			t.Fatalf("layout.json positions key %q does not resolve to a declared object id", id)
		}
	}
}

// TestV2Corpus_OutcomeAttestationAndReaffirmationDecode proves the outcome
// attestation and re-affirmation record both decode, and that the
// reaffirmation's object fragment ref is pinned at the golden v2 SHA.
func TestV2Corpus_OutcomeAttestationAndReaffirmationDecode(t *testing.T) {
	t.Run("outcome attestation", func(t *testing.T) {
		data := readCorpusFile(t, ".verdi/attestations/escrow-autopay/ac-1.md")
		fm, _, err := SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("SplitFrontmatter: %v", err)
		}
		att, err := DecodeAttestation(fm)
		if err != nil {
			t.Fatalf("DecodeAttestation: %v", err)
		}
		if att.ID != "attestation/escrow-autopay--ac-1" {
			t.Fatalf("ID = %q", att.ID)
		}
	})

	t.Run("reaffirmation", func(t *testing.T) {
		data := readCorpusFile(t, ".verdi/reaffirmations/jira-loan-1483/ac-1.md")
		fm, _, err := SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("SplitFrontmatter: %v", err)
		}
		re, err := DecodeReaffirmation(fm)
		if err != nil {
			t.Fatalf("DecodeReaffirmation: %v", err)
		}
		ref, err := ParseRef(re.Object)
		if err != nil {
			t.Fatalf("ParseRef(re.Object): %v", err)
		}
		if ref.Commit != goldenShaB {
			t.Fatalf("reaffirmation object commit = %s, want golden SHA_B %s", ref.Commit, goldenShaB)
		}
		if ref.Object != "ac-1" {
			t.Fatalf("reaffirmation object fragment = %q, want ac-1", ref.Object)
		}
	})
}

// TestV2Corpus_ReaffirmationHashPair_MatchesObjectContentHash proves the
// "object IDs round-trip through the (kind, id, text) content hash" exit
// criterion against real fixture data: the reaffirmation's hash.old/new
// pair equals ObjectContentHash computed directly from loan-workflow v1's
// and loan-workflow-v2's own decoded ac-1 text.
func TestV2Corpus_ReaffirmationHashPair_MatchesObjectContentHash(t *testing.T) {
	v1Data := readCorpusFile(t, ".verdi/specs/active/loan-workflow/spec.md")
	v1FM, _, err := SplitFrontmatter(v1Data)
	if err != nil {
		t.Fatalf("SplitFrontmatter(v1): %v", err)
	}
	v1, err := DecodeSpec(v1FM)
	if err != nil {
		t.Fatalf("DecodeSpec(v1): %v", err)
	}

	v2Data := readCorpusFile(t, ".verdi/specs/active/loan-workflow-v2/spec.md")
	v2FM, _, err := SplitFrontmatter(v2Data)
	if err != nil {
		t.Fatalf("SplitFrontmatter(v2): %v", err)
	}
	v2, err := DecodeSpec(v2FM)
	if err != nil {
		t.Fatalf("DecodeSpec(v2): %v", err)
	}

	reData := readCorpusFile(t, ".verdi/reaffirmations/jira-loan-1483/ac-1.md")
	reFM, _, err := SplitFrontmatter(reData)
	if err != nil {
		t.Fatalf("SplitFrontmatter(reaffirmation): %v", err)
	}
	re, err := DecodeReaffirmation(reFM)
	if err != nil {
		t.Fatalf("DecodeReaffirmation: %v", err)
	}

	v1AC1 := findAC(t, v1.AcceptanceCriteria, "ac-1")
	v2AC1 := findAC(t, v2.AcceptanceCriteria, "ac-1")
	if v1AC1.Text == v2AC1.Text {
		t.Fatal("test setup: v1 and v2 ac-1 text must differ (this is the amended object)")
	}

	oldHash, err := ObjectContentHash(ObjectKindAcceptanceCriterion, v1AC1.ID, v1AC1.Text)
	if err != nil {
		t.Fatalf("ObjectContentHash(old): %v", err)
	}
	newHash, err := ObjectContentHash(ObjectKindAcceptanceCriterion, v2AC1.ID, v2AC1.Text)
	if err != nil {
		t.Fatalf("ObjectContentHash(new): %v", err)
	}

	if oldHash != re.Hash.Old {
		t.Fatalf("ObjectContentHash(v1 ac-1) = %s, want reaffirmation hash.old %s", oldHash, re.Hash.Old)
	}
	if newHash != re.Hash.New {
		t.Fatalf("ObjectContentHash(v2 ac-1) = %s, want reaffirmation hash.new %s", newHash, re.Hash.New)
	}
}

func findAC(t *testing.T, acs []AcceptanceCriterion, id string) AcceptanceCriterion {
	t.Helper()
	for _, ac := range acs {
		if ac.ID == id {
			return ac
		}
	}
	t.Fatalf("no acceptance criterion %q found", id)
	return AcceptanceCriterion{}
}

// TestV2Corpus_FragmentRefsParseAndReserialize is the "fragment refs parse
// and re-serialize" exit criterion against real fixture links: every
// implements/resolves/exempts/supersedes edge in the v2 story/spike
// fixtures round-trips through ParseRef -> String().
func TestV2Corpus_FragmentRefsParseAndReserialize(t *testing.T) {
	fragmentRefs := []string{
		"spec/escrow-autopay#ac-1",
		"spec/escrow-autopay#ac-2",
		"spec/escrow-autopay#dc-2",
		"spec/loan-workflow#ac-1",
		"spec/escrow-autopay#oq-1",
		"spec/loan-workflow-v2@06a3f4cabb226fe9344e1645e27c344493b6b62b#ac-1",
	}
	for _, s := range fragmentRefs {
		t.Run(s, func(t *testing.T) {
			ref, err := ParseRef(s)
			if err != nil {
				t.Fatalf("ParseRef(%q): %v", s, err)
			}
			if !ref.Fragment() {
				t.Fatalf("ParseRef(%q).Fragment() = false, want true", s)
			}
			if got := ref.String(); got != s {
				t.Fatalf("round-trip: ParseRef(%q).String() = %q", s, got)
			}
		})
	}
}

// --- Negative-path: unknown-field and mismatched-anchor twins ---

func readInvalidFile(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(v2InvalidDir, rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return data
}

// TestV2CorpusInvalid_UnknownFieldTwinsFailLoudly is the "every
// unknown-field twin fails loudly" exit criterion for the round-four
// schema surface.
func TestV2CorpusInvalid_UnknownFieldTwinsFailLoudly(t *testing.T) {
	t.Run("feature-unknown-field.md", func(t *testing.T) {
		data := readInvalidFile(t, "feature-unknown-field.md")
		fm, _, err := SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("SplitFrontmatter: %v", err)
		}
		_, err = DecodeSpec(fm)
		if err == nil {
			t.Fatal("DecodeSpec: want error for unknown field, got nil")
		}
		if !strings.Contains(err.Error(), "bogus_extra_field") {
			t.Fatalf("error = %q, want it to name the unknown field", err)
		}
	})

	t.Run("story-unknown-field.md", func(t *testing.T) {
		data := readInvalidFile(t, "story-unknown-field.md")
		fm, _, err := SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("SplitFrontmatter: %v", err)
		}
		_, err = DecodeSpec(fm)
		if err == nil {
			t.Fatal("DecodeSpec: want error for unknown field, got nil")
		}
		if !strings.Contains(err.Error(), "bogus_extra_field") {
			t.Fatalf("error = %q, want it to name the unknown field", err)
		}
	})

	t.Run("layout-unknown-field.json", func(t *testing.T) {
		data := readInvalidFile(t, "layout-unknown-field.json")
		_, err := DecodeBoardLayout(data)
		if err == nil {
			t.Fatal("DecodeBoardLayout: want error for unknown field, got nil")
		}
	})

	t.Run("reaffirmation-unknown-field.md", func(t *testing.T) {
		data := readInvalidFile(t, "reaffirmation-unknown-field.md")
		fm, _, err := SplitFrontmatter(data)
		if err != nil {
			t.Fatalf("SplitFrontmatter: %v", err)
		}
		_, err = DecodeReaffirmation(fm)
		if err == nil {
			t.Fatal("DecodeReaffirmation: want error for unknown field, got nil")
		}
		if !strings.Contains(err.Error(), "bogus_extra_field") {
			t.Fatalf("error = %q, want it to name the unknown field", err)
		}
	})
}

// TestV2CorpusInvalid_MismatchedAnchorTwinFails is the "a mismatched-anchor
// twin fails naming the anchor rule" exit criterion, run against the real
// escrow-autopay fixture's body (not a synthetic minimal example).
func TestV2CorpusInvalid_MismatchedAnchorTwinFails(t *testing.T) {
	data := readInvalidFile(t, "feature-mismatched-anchor.md")
	fm, body, err := SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	err = spec.ResolveObjectAnchors(body)
	if err == nil {
		t.Fatal("ResolveObjectAnchors: want error for mismatched anchor, got nil")
	}
	if !strings.Contains(err.Error(), "anchor") {
		t.Fatalf("error = %q, want it to name the anchor rule", err)
	}
	if !strings.Contains(err.Error(), "ac-2") {
		t.Fatalf("error = %q, want it to name the offending object ac-2", err)
	}
}
