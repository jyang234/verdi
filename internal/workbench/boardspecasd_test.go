package workbench

// Unit coverage for the ASD workbench's derivation and strictness seams:
// the four-area shell projection (SI-125 idioms over this board's typed
// facts), the wiring that feeds that projection from one real stored spec
// (buildASDView), the strict pre-application body grammar (design §3.2),
// and the fixed-set route/action inventory (SI-167).

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func TestDeriveASDShell(t *testing.T) {
	baseInput := func() asdShellInput {
		return asdShellInput{
			ProblemPresent: true,
			OutcomePresent: true,
			ACs:            []asdACFact{{ID: "ac-1", EvidenceCount: 1}},
			Mode:           "authoring",
			Branch:         "design/x",
			StateFormal:    "proposed",
			DesignWired:    true,
			Caps:           &DesignCapabilitiesView{PolicyMode: "proposal-only", PolicyDigest: "sha256:abc", RefusalPrecondition: "policy-mode", RefusalDetail: "mode forbids agent writes"},
			SpikeWord:      "spike",
		}
	}

	t.Run("every area carries an explicit anchor and ordering is deterministic", func(t *testing.T) {
		shell := deriveASDShell(baseInput())
		if len(shell.Areas) != 4 {
			t.Fatalf("areas = %d, want 4", len(shell.Areas))
		}
		for i, id := range asdAreaOrder {
			if shell.Areas[i].ID != id {
				t.Fatalf("area order[%d] = %s, want %s", i, shell.Areas[i].ID, id)
			}
		}
		// The clean-tree proposed board: shape/success/context proven,
		// review unproven (human review pending) — focus is request-review.
		if shell.CurrentFocus != asdAreaReview {
			t.Fatalf("focus = %s, want %s", shell.CurrentFocus, asdAreaReview)
		}
		second := deriveASDShell(baseInput())
		if len(second.All) != len(shell.All) {
			t.Fatal("derivation is not deterministic")
		}
		for i := range shell.All {
			if shell.All[i].ID != second.All[i].ID {
				t.Fatal("concern ordering is not deterministic")
			}
		}
	})

	t.Run("missing problem is a blocking violation with source guidance", func(t *testing.T) {
		in := baseInput()
		in.ProblemPresent = false
		shell := deriveASDShell(in)
		if shell.CurrentFocus != asdAreaShape {
			t.Fatalf("focus = %s, want shape", shell.CurrentFocus)
		}
		found := false
		for _, c := range shell.All {
			if c.ID == "shape/problem" {
				found = true
				if c.State != asdStateViolated || !c.Blocking || c.Guidance == "" || len(c.Witnesses) == 0 {
					t.Fatalf("shape/problem = %+v, want blocking violated with guidance and witness", c)
				}
			}
		}
		if !found {
			t.Fatal("no shape/problem concern")
		}
		// Attention leads with the current area's rows (SI-125).
		if len(shell.Attention) == 0 || shell.Attention[0].Area != asdAreaShape {
			t.Fatalf("attention head = %+v, want a shape-area row first", shell.Attention)
		}
	})

	t.Run("open questions are blocking unproven and never suppressed", func(t *testing.T) {
		in := baseInput()
		in.OpenQuestions = []asdObjectFact{{ID: "oq-1", Text: "t1"}, {ID: "oq-2", Text: "t2"}}
		shell := deriveASDShell(in)
		got := 0
		for _, c := range shell.All {
			if strings.HasPrefix(c.ID, "shape/question/") {
				got++
				if c.State != asdStateUnproven || !c.Blocking || c.Dest == "" {
					t.Fatalf("question concern = %+v", c)
				}
			}
		}
		if got != 2 {
			t.Fatalf("question concerns = %d, want 2 (lossless)", got)
		}
	})

	t.Run("a spike-claimed open question is non-blocking with claim-aware guidance", func(t *testing.T) {
		in := baseInput()
		in.OpenQuestions = []asdObjectFact{{ID: "oq-1", Text: "t1", ClaimedBySlugs: []string{"retry-strategy-spike"}}}
		shell := deriveASDShell(in)
		var found *asdConcern
		for i := range shell.All {
			if shell.All[i].ID == "shape/question/oq-1" {
				found = &shell.All[i]
			}
		}
		if found == nil {
			t.Fatalf("no shape/question/oq-1 concern in %+v", shell.All)
		}
		c := *found
		if c.State != asdStateUnproven || c.Blocking {
			t.Fatalf("claimed question concern = %+v, want unproven non-blocking", c)
		}
		if !strings.Contains(c.Summary, "retry-strategy-spike") || !strings.Contains(c.Summary, "claimed") || !strings.Contains(c.Summary, "unresolved") {
			t.Fatalf("summary = %q, want it to name the claiming stub and say claimed+unresolved", c.Summary)
		}
		if c.Guidance == "" || strings.Contains(c.Guidance, "edit or remove") {
			t.Fatalf("guidance = %q, want claim-aware guidance, not the unclaimed edit-or-remove text", c.Guidance)
		}
		wantWitnesses := []string{"declared open question oq-1", "retry-strategy-spike"}
		sort.Strings(wantWitnesses)
		gotWitnesses := append([]string(nil), c.Witnesses...)
		sort.Strings(gotWitnesses)
		if !reflect.DeepEqual(gotWitnesses, wantWitnesses) {
			t.Fatalf("witnesses = %q, want %q", c.Witnesses, wantWitnesses)
		}
		if c.Dest == "" {
			t.Fatalf("claimed question concern lost its destination: %+v", c)
		}
	})

	t.Run("multiple claiming stubs are named, witnessed, and spoken in the plural", func(t *testing.T) {
		in := baseInput()
		in.OpenQuestions = []asdObjectFact{{ID: "oq-1", Text: "t1", ClaimedBySlugs: []string{"alpha-spike", "zeta-spike"}}}
		shell := deriveASDShell(in)
		var c asdConcern
		for _, row := range shell.All {
			if row.ID == "shape/question/oq-1" {
				c = row
			}
		}
		if !strings.Contains(c.Summary, "alpha-spike") || !strings.Contains(c.Summary, "zeta-spike") {
			t.Fatalf("summary = %q, want both claiming stub slugs", c.Summary)
		}
		for _, want := range []string{"alpha-spike", "zeta-spike"} {
			ok := false
			for _, w := range c.Witnesses {
				if w == want {
					ok = true
				}
			}
			if !ok {
				t.Fatalf("witnesses = %q, missing %q", c.Witnesses, want)
			}
		}
		// The sentences are pinned exactly (lane review F2): the head
		// noun and its verb agree with the number of claiming stubs. The
		// class word stays the attributive singular every sibling
		// surface speaks (readinesspilot's "<word> stubs", the stub
		// cards' "<word> stub"); only "stub"/"stubs" and "answers"/
		// "answer" move.
		wantPluralSummary := "Open question oq-1 is claimed by spike stubs alpha-spike, zeta-spike and remains unresolved: t1"
		if c.Summary != wantPluralSummary {
			t.Fatalf("two-slug summary =\n  %q\nwant\n  %q", c.Summary, wantPluralSummary)
		}
		wantPluralGuidance := "No wall edit is required to accept: the claiming spike stubs answer it after acceptance."
		if c.Guidance != wantPluralGuidance {
			t.Fatalf("two-slug guidance =\n  %q\nwant\n  %q", c.Guidance, wantPluralGuidance)
		}

		one := baseInput()
		one.OpenQuestions = []asdObjectFact{{ID: "oq-1", Text: "t1", ClaimedBySlugs: []string{"alpha-spike"}}}
		var single asdConcern
		for _, row := range deriveASDShell(one).All {
			if row.ID == "shape/question/oq-1" {
				single = row
			}
		}
		wantSingleSummary := "Open question oq-1 is claimed by spike stub alpha-spike and remains unresolved: t1"
		if single.Summary != wantSingleSummary {
			t.Fatalf("one-slug summary =\n  %q\nwant\n  %q", single.Summary, wantSingleSummary)
		}
		wantSingleGuidance := "No wall edit is required to accept: the claiming spike stub answers it after acceptance."
		if single.Guidance != wantSingleGuidance {
			t.Fatalf("one-slug guidance =\n  %q\nwant\n  %q", single.Guidance, wantSingleGuidance)
		}
	})

	t.Run("an unclaimed question among a claimed one stays blocking", func(t *testing.T) {
		in := baseInput()
		in.OpenQuestions = []asdObjectFact{
			{ID: "oq-1", Text: "t1"},
			{ID: "oq-2", Text: "t2", ClaimedBySlugs: []string{"retry-strategy-spike"}},
		}
		shell := deriveASDShell(in)
		var unclaimed, claimed asdConcern
		for _, row := range shell.All {
			switch row.ID {
			case "shape/question/oq-1":
				unclaimed = row
			case "shape/question/oq-2":
				claimed = row
			}
		}
		if !unclaimed.Blocking {
			t.Fatalf("unclaimed concern = %+v, want blocking", unclaimed)
		}
		if claimed.Blocking {
			t.Fatalf("claimed concern = %+v, want non-blocking", claimed)
		}
	})

	t.Run("downstream violations count exactly and areas never suppress", func(t *testing.T) {
		in := baseInput()
		in.ProblemPresent = false // focus: shape
		in.ACs = nil              // success violated downstream
		shell := deriveASDShell(in)
		if shell.CurrentFocus != asdAreaShape {
			t.Fatalf("focus = %s", shell.CurrentFocus)
		}
		if shell.DownstreamViolated != 1 {
			t.Fatalf("downstream violated = %d, want 1 (success/criteria)", shell.DownstreamViolated)
		}
		// The downstream violated row stays in Attention (prominence, not
		// suppression).
		seen := false
		for _, c := range shell.Attention {
			if c.ID == "success/criteria" {
				seen = true
			}
		}
		if !seen {
			t.Fatal("downstream violation missing from the queue")
		}
	})

	t.Run("design-branch and proposal-state refusals speak to humans and agents alike", func(t *testing.T) {
		// Review fix I-1: only the policy-mode precondition is agent-
		// specific (AuthorizePolicy's human bypass); the design-branch and
		// proposal-state preconditions refuse the browser human too
		// (AuthorizeState runs for every actor), so agent-only wording
		// would be dishonest.
		for _, tc := range []struct{ precondition, detail string }{
			{"design-branch", "branch main is not mutable design branch design/x"},
			{"proposal-state", "Git-derived state accepted-pending-build is not mutable proposal state"},
		} {
			in := baseInput()
			in.Caps = &DesignCapabilitiesView{PolicyMode: "draft-write", PolicyDigest: "sha256:abc", RefusalPrecondition: tc.precondition, RefusalDetail: tc.detail}
			shell := deriveASDShell(in)
			found := false
			for _, c := range shell.All {
				if c.ID != "context/draft-writes" {
					continue
				}
				found = true
				if strings.Contains(c.Summary, "Delegated agents") {
					t.Fatalf("%s: summary %q is agent-only wording; the refusal binds the human writer too", tc.precondition, c.Summary)
				}
				if !strings.Contains(c.Summary, tc.precondition) {
					t.Fatalf("%s: summary %q does not name the failing precondition", tc.precondition, c.Summary)
				}
				witnessed := false
				for _, w := range c.Witnesses {
					if w == tc.detail {
						witnessed = true
					}
				}
				if !witnessed {
					t.Fatalf("%s: witnesses %q do not carry the kernel detail %q", tc.precondition, c.Witnesses, tc.detail)
				}
			}
			if !found {
				t.Fatalf("%s: no context/draft-writes concern in %+v", tc.precondition, shell.All)
			}
		}
		// The policy-mode precondition keeps its agent-specific label.
		shell := deriveASDShell(baseInput())
		for _, c := range shell.All {
			if c.ID == "context/agent-writes" && !strings.Contains(c.Summary, "Delegated agents") {
				t.Fatalf("policy-mode summary %q lost its agent-specific labeling", c.Summary)
			}
		}
	})

	t.Run("not-adopted policy is honest absence, not a violation", func(t *testing.T) {
		in := baseInput()
		in.Caps = nil
		in.CapsFailure = &DesignFailure{Classification: "verdict", Code: "policy-forbidden", Detail: "project has not adopted policy authority"}
		shell := deriveASDShell(in)
		for _, c := range shell.All {
			if c.ID == "context/policy" {
				if c.State != asdStateUnproven || c.Blocking {
					t.Fatalf("context/policy = %+v, want nonblocking unproven", c)
				}
				if !strings.Contains(c.Summary, "not-applicable") {
					t.Fatalf("summary %q does not name the not-applicable posture", c.Summary)
				}
				return
			}
		}
		t.Fatal("no context/policy concern")
	})

	t.Run("human review is plainly labeled with the formal obligation secondary", func(t *testing.T) {
		shell := deriveASDShell(baseInput())
		for _, c := range shell.All {
			if c.ID == "review/acceptance" {
				if !c.HumanReview {
					t.Fatalf("review/acceptance = %+v, want HumanReview", c)
				}
				joined := strings.Join(c.Witnesses, " ")
				if !strings.Contains(joined, "AC-6") {
					t.Fatalf("formal secondary evidence missing: %v", c.Witnesses)
				}
				return
			}
		}
		t.Fatal("no review/acceptance concern")
	})

	t.Run("dirty tree blocks review with the commit guidance", func(t *testing.T) {
		in := baseInput()
		in.Dirty = true
		shell := deriveASDShell(in)
		for _, c := range shell.All {
			if c.ID == "review/worktree" {
				if c.State != asdStateUnproven || !c.Blocking || !strings.Contains(c.Guidance, "Commit") {
					t.Fatalf("review/worktree = %+v", c)
				}
				return
			}
		}
		t.Fatal("no review/worktree concern")
	})
}

func TestDecodeStrictActionBody(t *testing.T) {
	type body struct {
		A string   `json:"a,omitempty"`
		B []string `json:"b,omitempty"`
	}
	cases := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{"empty body is the empty object", "", ""},
		{"plain object decodes", `{"a":"x"}`, ""},
		{"unknown field fails closed", `{"zzz":1}`, "unknown field"},
		{"duplicate key fails closed", `{"a":"x","a":"y"}`, "duplicate key"},
		{"null value fails closed", `{"a":null}`, "null values"},
		{"nested null fails closed", `{"b":["x",null]}`, "null values"},
		{"trailing data fails closed", `{"a":"x"}{}`, "trailing data"},
		{"non-object fails closed", `[1,2]`, "cannot unmarshal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out body
			err := decodeStrictActionBody([]byte(tc.raw), &out)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

// TestBoardActionInventory is SI-167's fixed-set witness: the exact
// closed union of application operations and surviving non-domain
// affordances. ANY growth or loss fails here first — the inventory is the
// authority the handler refuses against, so this table IS the route
// grammar's action half.
func TestBoardActionInventory(t *testing.T) {
	wantOperations := []string{
		"get_board",
		"get_design_capabilities",
		"get_design_context",
		"get_design_provenance",
		"mutate_draft",
		"prepare_design_review",
	}
	wantLegacy := []string{
		"annotation-delete",
		"create",
		"git-commit",
		"git-switch",
		"pin",
		"position",
		"relates",
		"revise",
		"sticky",
		"sticky-graduate",
		"sticky-position",
		"stub-instantiate",
	}
	if len(designOperations) != len(wantOperations) {
		t.Fatalf("designOperations = %v, want %v", designOperations, wantOperations)
	}
	for i, op := range wantOperations {
		if designOperations[i] != op {
			t.Fatalf("designOperations[%d] = %q, want %q", i, designOperations[i], op)
		}
	}
	if len(legacyBoardActions) != len(wantLegacy) {
		t.Fatalf("legacyBoardActions = %v, want %v", legacyBoardActions, wantLegacy)
	}
	for i, action := range wantLegacy {
		if legacyBoardActions[i] != action {
			t.Fatalf("legacyBoardActions[%d] = %q, want %q", i, legacyBoardActions[i], action)
		}
	}
	inventory := boardActionInventory()
	if len(inventory) != len(wantOperations)+len(wantLegacy) {
		t.Fatalf("inventory size = %d, want %d", len(inventory), len(wantOperations)+len(wantLegacy))
	}
	// Every deleted DOMAIN action is genuinely out of the union.
	for _, gone := range []string{"edit-text", "edge", "edge-delete", "edge-retype", "stub-graduate", "relates-graduate", "ref-trash", "object-trash", "spliceSpec"} {
		if inventory[gone] {
			t.Errorf("deleted action %q is still in the inventory", gone)
		}
	}
}

// TestBoardActionInventoryRefusesUnknownGrowth proves the handler refuses
// an action outside the closed union BEFORE any other work (404), and the
// snapshot route exists in the shared route table (both mounts).
func TestBoardActionInventoryRefusesUnknownGrowth(t *testing.T) {
	suffixes := map[string]bool{}
	for _, rt := range boardSpecRoutes() {
		suffixes[rt.suffix] = true
	}
	for _, want := range []string{routeBoardPage, routeBoardFragment, routeBoardSnapshot, routeBoardAPI, routeBoardPeek, routeBoardPinSearch, routeBoardDocument, routeBoardDocumentSnapshot} {
		if !suffixes[want] {
			t.Errorf("route table missing %s", want)
		}
	}
	if len(suffixes) != 8 {
		t.Errorf("route table has %d rows, want exactly 8 (fixed set: the six board rows plus spec/spec-documents' document page and snapshot)", len(suffixes))
	}
}

// --- the wall-side claim wiring, from stored frontmatter -----------------

// claimWallSpec is a draft feature wall carrying the exact shape ac-10
// speaks about: two declared open questions, one of them claimed by TWO
// spike stubs (declared zeta-first, so the claim ordering is the
// derivation's own sort and not the file's order), the other claimed by
// nothing, plus one plain coverage stub that claims an acceptance
// criterion and never a question.
const claimWallSpec = `---
id: spec/claim-wall
kind: spec
class: feature
title: "Claim wall"
status: draft
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "ac one", evidence: [attestation], anchor: "#ac-1" }
open_questions:
  - { id: oq-1, text: "which decline reasons may be shown verbatim?", anchor: "#oq-1" }
  - { id: oq-2, text: "what refresh-window SLA applies?", anchor: "#oq-2" }
stubs:
  - { slug: plain-coverage-stub, acceptance_criteria: [ac-1] }
  - { slug: zeta-spike, spike: true, resolves: [oq-2] }
  - { slug: alpha-spike, spike: true, resolves: [oq-2] }
---
# Claim wall

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## oq-1

Prose.

## oq-2

Prose.
`

const claimWallName = "claim-wall"

const claimWallPlainStubEntry = "{ slug: plain-coverage-stub, acceptance_criteria: [ac-1] }"

func newClaimWallFixture(t *testing.T) string {
	t.Helper()
	return buildAuthoringFixture(t, "design/"+claimWallName,
		map[string]string{".verdi/.gitignore": "data/\n"},
		map[string]string{
			".verdi/specs/active/" + claimWallName + "/spec.md": claimWallSpec,
		})
}

// shellConcerns indexes a rendered shell's rows by concern id.
func shellConcerns(shell asdShell) map[string]asdConcern {
	byID := make(map[string]asdConcern, len(shell.All))
	for _, c := range shell.All {
		byID[c.ID] = c
	}
	return byID
}

// TestBuildASDView_SpikeClaimedQuestions is the WIRING witness for ac-10
// on the wall (PLAN.md §7 I-128 option (a)): deriveASDShell's own unit
// tests hand it asdObjectFacts directly, so nothing proved that the view
// builder reads `stubs:` out of the stored frontmatter at all. This case
// starts at one real spec.md in a real store and asserts the rendered
// shell — the claimed question's exact sentences and witnesses, the
// unclaimed question's unchanged blocking guidance, and the plain stub's
// silence.
func TestBuildASDView_SpikeClaimedQuestions(t *testing.T) {
	root := newClaimWallFixture(t)
	s := &boardSpecServer{root: root}
	ctx := context.Background()

	_, _, asd, err := s.loadASD(ctx, claimWallName)
	if err != nil {
		t.Fatalf("loadASD: %v", err)
	}
	byID := shellConcerns(asd.Shell)

	claimed, ok := byID["shape/question/oq-2"]
	if !ok {
		t.Fatalf("no shape/question/oq-2 concern in %+v", asd.Shell.All)
	}
	if claimed.State != asdStateUnproven || claimed.Blocking {
		t.Fatalf("claimed question = %+v, want unproven and non-blocking", claimed)
	}
	wantSummary := "Open question oq-2 is claimed by spike stubs alpha-spike, zeta-spike and remains unresolved: what refresh-window SLA applies?"
	if claimed.Summary != wantSummary {
		t.Fatalf("claimed summary =\n  %q\nwant\n  %q", claimed.Summary, wantSummary)
	}
	wantGuidance := "No wall edit is required to accept: the claiming spike stubs answer it after acceptance."
	if claimed.Guidance != wantGuidance {
		t.Fatalf("claimed guidance =\n  %q\nwant\n  %q", claimed.Guidance, wantGuidance)
	}
	wantWitnesses := []string{"declared open question oq-2", "alpha-spike", "zeta-spike"}
	if !reflect.DeepEqual(claimed.Witnesses, wantWitnesses) {
		t.Fatalf("claimed witnesses = %q, want %q", claimed.Witnesses, wantWitnesses)
	}

	unclaimed, ok := byID["shape/question/oq-1"]
	if !ok {
		t.Fatalf("no shape/question/oq-1 concern in %+v", asd.Shell.All)
	}
	if !unclaimed.Blocking {
		t.Fatalf("unclaimed question = %+v, want blocking", unclaimed)
	}
	wantUnclaimedGuidance := "Resolve it on the wall: edit or remove oq-1, or graduate a decision that answers it."
	if unclaimed.Guidance != wantUnclaimedGuidance {
		t.Fatalf("unclaimed guidance =\n  %q\nwant\n  %q", unclaimed.Guidance, wantUnclaimedGuidance)
	}
	if len(unclaimed.Witnesses) != 1 || unclaimed.Witnesses[0] != "declared open question oq-1" {
		t.Fatalf("unclaimed witnesses = %q, want the declaration alone", unclaimed.Witnesses)
	}

	// The plain coverage stub claims an acceptance criterion, so it never
	// reaches a question row — not in a summary, not as a witness.
	for _, c := range asd.Shell.All {
		if strings.Contains(c.Summary, "plain-coverage-stub") {
			t.Fatalf("%s summary names the plain stub: %q", c.ID, c.Summary)
		}
		for _, w := range c.Witnesses {
			if w == "plain-coverage-stub" {
				t.Fatalf("%s witnesses the plain stub: %q", c.ID, c.Witnesses)
			}
		}
	}
}

// TestBuildASDView_NonSpikeStubNeverClaims pins the view builder's
// fail-closed half. A plain stub CANNOT legally carry `resolves` — the
// decode seam refuses that frontmatter outright (02 §Kind registry, DC-4,
// asserted here) — so the builder's spike test is the defense behind a
// refused state: were such a stub to reach it anyway, the question it
// names stays the ordinary blocking unclaimed row.
func TestBuildASDView_NonSpikeStubNeverClaims(t *testing.T) {
	fmBytes, _, err := artifact.SplitFrontmatter([]byte(claimWallSpec))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	illegal := strings.Replace(string(fmBytes), claimWallPlainStubEntry,
		"{ slug: plain-coverage-stub, acceptance_criteria: [ac-1], resolves: [oq-1] }", 1)
	if illegal == string(fmBytes) {
		t.Fatal("the plain-stub entry moved: this case never built the refused frontmatter")
	}
	if _, err := artifact.DecodeSpec([]byte(illegal)); err == nil || !strings.Contains(err.Error(), "resolves requires spike") {
		t.Fatalf("DecodeSpec(plain stub with resolves) err = %v, want the DC-4 refusal", err)
	}

	root := newClaimWallFixture(t)
	s := &boardSpecServer{root: root}
	ctx := context.Background()
	proj, git, _, extras, err := s.loadBoard(ctx, claimWallName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	// The refused shape, injected past the decode seam.
	extras.fm.Stubs = append(extras.fm.Stubs, artifact.Stub{
		Slug:               "smuggled-plain-stub",
		AcceptanceCriteria: []string{"ac-1"},
		Resolves:           []string{"oq-1"},
	})
	view, err := s.buildASDView(ctx, claimWallName, proj, git, extras.raw, extras.fm, extras.state)
	if err != nil {
		t.Fatalf("buildASDView: %v", err)
	}
	unclaimed, ok := shellConcerns(view.Shell)["shape/question/oq-1"]
	if !ok {
		t.Fatalf("no shape/question/oq-1 concern in %+v", view.Shell.All)
	}
	if !unclaimed.Blocking {
		t.Fatalf("oq-1 = %+v, want the blocking unclaimed row: a non-spike stub claims nothing", unclaimed)
	}
	if strings.Contains(unclaimed.Summary, "smuggled-plain-stub") || strings.Contains(unclaimed.Summary, "claimed") {
		t.Fatalf("oq-1 summary = %q, want the unclaimed sentence", unclaimed.Summary)
	}
	for _, w := range unclaimed.Witnesses {
		if w == "smuggled-plain-stub" {
			t.Fatalf("oq-1 witnesses = %q, want no claim witness", unclaimed.Witnesses)
		}
	}
}
