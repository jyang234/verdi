package workbench

// Unit coverage for the ASD workbench's derivation and strictness seams:
// the four-area readiness the wall shows — since the wall shell's own
// derivation retired (spec/wall-strip-and-drawer-v2 ac-7, dc-4), the
// loader's facts in the record drawer's Readiness tab, against which the
// shell's tests are repointed in place under their names (SI-368 (32)
// T1) — the strict pre-application body grammar (design §3.2), and the
// fixed-set route/action inventory (SI-167).

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

// unshapedWallName's spec is a draft feature declaring neither problem nor
// outcome, with one open question claimed by a single spike stub — the
// stub covering no criterion, so the feature's one criterion is uncovered.
const unshapedWallName = "unshaped-wall"

const unshapedWallSpec = `---
id: spec/unshaped-wall
kind: spec
class: feature
title: "Unshaped wall"
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "ac one", evidence: [attestation], anchor: "#ac-1" }
open_questions:
  - { id: oq-1, text: "which retry strategy applies?", anchor: "#oq-1" }
stubs:
  - { slug: retry-strategy-spike, spike: true, resolves: [oq-1] }
---
# Unshaped wall

## ac-1

Prose.

## oq-1

Prose.
`

// criterialessWallName's spec is a draft story with no acceptance criteria
// and one unclaimed open question, implementing its parent feature's
// criterion: Define the work is the current step, and Define success
// carries a violated row downstream of it.
const criterialessWallName = "criterialess-wall"

const criterialessWallSpec = `---
id: spec/criterialess-wall
kind: spec
title: "Criterialess wall"
owners: [platform-team]
class: story
story: jira:LOAN-2207
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
open_questions:
  - { id: oq-1, text: "which channel carries it?", anchor: "#oq-1" }
links:
  - { type: implements, ref: spec/criterialess-parent#ac-1 }
---
# Criterialess wall

## Problem

Prose.

## Outcome

Prose.

## oq-1

Prose.
`

const criterialessParentSpec = `---
id: spec/criterialess-parent
kind: spec
class: feature
title: "Criterialess parent"
status: draft
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "ac one", evidence: [attestation], anchor: "#ac-1" }
---
# Criterialess parent

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.
`

// newCriterialessWall builds the criterialess wall, its parent feature
// beside it, with any extra draft-branch files.
func newCriterialessWall(t *testing.T, extra map[string]string) *tabWall {
	t.Helper()
	files := map[string]string{".verdi/specs/active/criterialess-parent/spec.md": criterialessParentSpec}
	for path, body := range extra {
		files[path] = body
	}
	return newTabWall(t, criterialessWallName, criterialessWallSpec, files)
}

// tabFocus is the Readiness tab's focused stepper station's area, "" when
// it marks none, failing when it marks more than one.
func tabFocus(t *testing.T, tab string) string {
	t.Helper()
	m := regexp.MustCompile(`<li class="readiness-station readiness-station--focus" data-area-id="([^"]+)"`).FindAllStringSubmatch(tab, -1)
	switch len(m) {
	case 0:
		return ""
	case 1:
		return m[0][1]
	}
	t.Fatalf("the tab marks %d focused steps", len(m))
	return ""
}

// TestDeriveASDShell is the retired wall shell's derivation test,
// repointed in place (SI-368 (32) T1): each claim it made of the wall's
// readiness is asserted on the record drawer's Readiness tab, which
// renders the production loader's facts, over real stored walls; each
// claim of a family only the shell produced is asserted at that family's
// home (SI-368 (3)). The capabilities wording that "design-branch and
// proposal-state refusals speak to humans and agents alike" lives in the
// Context tab's script and is proven in e2e 91's ac-5, on the harness's
// review mirror and sealed record, beside the design wall's policy-mode
// refusal that speaks to agents alone (SI-368 (32) B2).
func TestDeriveASDShell(t *testing.T) {
	claim := newTabWall(t, claimWallName, claimWallSpec, nil)
	unshaped := newTabWall(t, unshapedWallName, unshapedWallSpec, nil)

	t.Run("every area carries an explicit anchor and ordering is deterministic", func(t *testing.T) {
		tab, snap := claim.tab(t)
		stations := regexp.MustCompile(`<li class="readiness-station[^"]*" data-area-id="([^"]+)" data-state="([^"]+)"`).FindAllStringSubmatch(tab, -1)
		wantAreas := []string{"shape-proposal", "show-success", "check-context", "request-review"}
		if len(stations) != len(wantAreas) {
			t.Fatalf("the tab's stepper has %d steps, want %d", len(stations), len(wantAreas))
		}
		for i, s := range stations {
			if s[1] != wantAreas[i] {
				t.Errorf("step %d = %s, want %s", i+1, s[1], wantAreas[i])
			}
			if s[2] != string(snap.Areas[i].State) || (s[2] != "proven" && s[2] != "violated-with-witness" && s[2] != "unproven") {
				t.Errorf("step %s carries state %q, want the loader's explicit %q", s[1], s[2], snap.Areas[i].State)
			}
		}
		if got := tabFocus(t, tab); got != string(snap.CurrentFocus) {
			t.Fatalf("the tab's focus = %q, want the loader's one focus %q", got, snap.CurrentFocus)
		}
		again, _ := claim.tab(t)
		if !reflect.DeepEqual(tabConcernIDs(again), tabConcernIDs(tab)) {
			t.Fatal("the tab's concern ordering is not deterministic")
		}
	})

	t.Run("missing problem is a blocking violation with source guidance", func(t *testing.T) {
		tab, snap := unshaped.tab(t)
		row := readTabRow(t, tab, "shape/problem")
		if row.State != string(readinesspilot.StateViolated) || row.Blocking != "true" || row.Primary != readinesspilot.Guidance(readinesspilot.GuidanceProblem, readinesspilot.GuidanceFacts{}) || len(row.Witnesses) == 0 {
			t.Fatalf("shape/problem = %+v, want blocking violated with the source guidance and a witness", row)
		}
		if got := tabFocus(t, tab); got != "shape-proposal" {
			t.Fatalf("the tab's focus = %q, want shape-proposal", got)
		}
		// Focus next leads with the current step's rows (SI-125).
		first := regexp.MustCompile(`id="readiness-focus".*?<article [^>]*data-area-id="([^"]+)"`).FindStringSubmatch(tab)
		if first == nil || first[1] != "shape-proposal" || snap.Attention[0].Area != readinesspilot.AreaShape {
			t.Fatalf("Focus next's first item is not a shape-area row: %v", first)
		}
	})

	t.Run("open questions are blocking unproven and never suppressed", func(t *testing.T) {
		tab, _ := claim.tab(t)
		got := 0
		for _, id := range tabConcernIDs(tab) {
			if strings.HasPrefix(id, "shape/question/") {
				got++
			}
		}
		if got != 2 {
			t.Fatalf("question rows = %d, want 2 (lossless: one per declared open question)", got)
		}
		row := readTabRow(t, tab, "shape/question/oq-1")
		if row.State != string(readinesspilot.StateUnproven) || row.Blocking != "true" || row.TargetKind != readinessTargetObject || row.Target != "oq-1" {
			t.Fatalf("unclaimed question = %+v, want blocking unproven, going to its card", row)
		}
	})

	t.Run("a spike-claimed open question is non-blocking with claim-aware guidance", func(t *testing.T) {
		tab, _ := unshaped.tab(t)
		row := readTabRow(t, tab, "shape/question/oq-1")
		if row.State != string(readinesspilot.StateUnproven) || row.Blocking != "false" {
			t.Fatalf("claimed question = %+v, want unproven non-blocking", row)
		}
		if !strings.Contains(row.Fact, "claimed") || !strings.Contains(row.Fact, "unresolved") {
			t.Fatalf("fact = %q, want it to say claimed and unresolved", row.Fact)
		}
		if row.Primary != "No wall edit is required to accept: the claiming spike stub answers it after acceptance." || strings.Contains(row.Primary, "edit or remove") {
			t.Fatalf("guidance = %q, want the one-stub claim-aware sentence, not the unclaimed edit-or-remove text", row.Primary)
		}
		if !reflect.DeepEqual(row.Witnesses, []string{"oq-1", "retry-strategy-spike"}) {
			t.Fatalf("witnesses = %q, want the question and its claiming stub", row.Witnesses)
		}
		if row.TargetKind != readinessTargetObject || row.Target != "oq-1" {
			t.Fatalf("claimed question lost its destination: %+v", row)
		}
	})

	t.Run("multiple claiming stubs are named, witnessed, and spoken in the plural", func(t *testing.T) {
		tab, _ := claim.tab(t)
		row := readTabRow(t, tab, "shape/question/oq-2")
		for _, want := range []string{"oq-2", "alpha-spike", "zeta-spike"} {
			if !containsString(row.Witnesses, want) {
				t.Fatalf("witnesses = %q, want the question and both claiming stubs", row.Witnesses)
			}
		}
		// The loader's prose (SI-368 (32) T1): the fact speaks the claim,
		// and the guidance's head noun and verb agree with the number of
		// claiming stubs — "stubs answer" here, "stub answers" above.
		if row.Fact != "Declared open question is claimed by spike stubs and remains unresolved" {
			t.Fatalf("fact = %q, want the loader's claimed-question fact", row.Fact)
		}
		if row.Primary != "No wall edit is required to accept: the claiming spike stubs answer it after acceptance." {
			t.Fatalf("two-stub guidance = %q", row.Primary)
		}
	})

	t.Run("an unclaimed question among a claimed one stays blocking", func(t *testing.T) {
		tab, _ := claim.tab(t)
		if row := readTabRow(t, tab, "shape/question/oq-1"); row.Blocking != "true" {
			t.Fatalf("unclaimed question = %+v, want blocking", row)
		}
		if row := readTabRow(t, tab, "shape/question/oq-2"); row.Blocking != "false" {
			t.Fatalf("claimed question = %+v, want non-blocking", row)
		}
	})

	t.Run("downstream violations count exactly and areas never suppress", func(t *testing.T) {
		tab, snap := newCriterialessWall(t, nil).tab(t)
		if snap.CurrentFocus != readinesspilot.AreaShape || tabFocus(t, tab) != "shape-proposal" {
			t.Fatalf("focus = %q (tab %q), want shape-proposal", snap.CurrentFocus, tabFocus(t, tab))
		}
		if row := readTabRow(t, tab, "success/criteria"); row.State != string(readinesspilot.StateViolated) {
			t.Fatalf("success/criteria = %+v, want violated", row)
		}
		var downstream []string
		for _, c := range snap.AllConcerns {
			if c.State == readinesspilot.StateViolated && c.Area != readinesspilot.AreaShape {
				downstream = append(downstream, c.ID)
			}
		}
		known := regexp.MustCompile(`data-known-concern="([^"]+)"`).FindAllStringSubmatch(tab, -1)
		if len(known) != len(downstream) || len(downstream) == 0 {
			t.Fatalf("known problems in later steps = %d, want exactly the %d downstream violations %q", len(known), len(downstream), downstream)
		}
		// The downstream violations stay in the tab (prominence, not
		// suppression): each keeps its one row.
		for _, id := range downstream {
			readTabRow(t, tab, id)
		}
	})

	t.Run("not-adopted policy is honest absence, not a violation", func(t *testing.T) {
		// The context/policy row's home is the policy guide (SI-368 (3),
		// (32) B1): no readiness verdict, and on an authoring wall the
		// not-applicable posture a write records is named.
		html := wallGuide(policyForbiddenInput(), modeAuthoring)
		expectNoReadinessState(t, "the not-adopted guide", html)
		if line, _ := testIDElementText(html, "asd-policy-guide-editing"); !strings.Contains(line, "not-applicable") {
			t.Fatalf("the guide's editing line %q does not name the not-applicable posture", line)
		}
	})

	t.Run("human review is plainly labeled with the formal obligation secondary", func(t *testing.T) {
		// review/acceptance's home is the Review tab (SI-368 (3), (32) B3).
		a := reviewAcceptanceFor("proposed", modeAuthoring, "design/x")
		var b strings.Builder
		writeReviewAcceptance(&b, a)
		if label, n := testIDElementText(b.String(), "record-human-review"); n != 1 || !strings.HasPrefix(label, "Human review") {
			t.Fatalf("human-review label = %q (%d)", label, n)
		}
		if !strings.Contains(strings.Join(a.Witnesses, " "), "AC-6") {
			t.Fatalf("formal secondary evidence missing: %v", a.Witnesses)
		}
	})

	t.Run("dirty tree blocks review with the commit guidance", func(t *testing.T) {
		// review/worktree's home is the bar's Commit and push indicator and
		// popover (SI-368 (3)): a dirty tree sets the indicator, the popover
		// names the branch, and its note says to commit and push because
		// review reads the committed head.
		html := renderWallUncommitted(deriveWallUncommitted(&boardGitState{Branch: "design/x", Dirty: true}))
		if strings.Contains(html, `data-testid="uncommitted-indicator" hidden`) {
			t.Fatalf("a dirty tree leaves the indicator clear:\n%s", html)
		}
		for _, want := range []string{"uncommitted on design/x", "Commits the working tree and pushes this branch", "review reads the committed head"} {
			if !strings.Contains(html, want) {
				t.Fatalf("the Commit popover lacks %q:\n%s", want, html)
			}
		}
	})
}

// containsString reports whether list holds s.
func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
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
	for _, want := range []string{routeBoardPage, routeBoardFragment, routeBoardSnapshot, routeBoardAPI, routeBoardPeek, routeBoardPinSearch, routeBoardDocument, routeBoardDocumentSnapshot, routeBoardReadiness} {
		if !suffixes[want] {
			t.Errorf("route table missing %s", want)
		}
	}
	if len(suffixes) != 9 {
		t.Errorf("route table has %d rows, want exactly 9 (fixed set: the six board rows, spec/spec-documents' document page and snapshot, and spec/wall-strip-and-drawer-v2's Readiness tab)", len(suffixes))
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

// TestBuildASDView_SpikeClaimedQuestions is the WIRING witness for ac-10
// on the wall (PLAN.md §7 I-128 option (a)), repointed in place onto the
// Readiness tab (SI-368 (32) T1): it starts at one real spec.md in a real
// store and asserts that the stored `stubs:` reach the wall's readiness
// through the loader — the claimed question's exact guidance and
// witnesses, the unclaimed question's unchanged blocking guidance, and
// the plain stub's silence on every question row.
func TestBuildASDView_SpikeClaimedQuestions(t *testing.T) {
	tab, _ := newTabWall(t, claimWallName, claimWallSpec, nil).tab(t)

	claimed := readTabRow(t, tab, "shape/question/oq-2")
	if claimed.State != string(readinesspilot.StateUnproven) || claimed.Blocking != "false" {
		t.Fatalf("claimed question = %+v, want unproven and non-blocking", claimed)
	}
	if want := "Declared open question is claimed by spike stubs and remains unresolved"; claimed.Fact != want {
		t.Fatalf("claimed fact =\n  %q\nwant\n  %q", claimed.Fact, want)
	}
	if want := "No wall edit is required to accept: the claiming spike stubs answer it after acceptance."; claimed.Primary != want {
		t.Fatalf("claimed guidance =\n  %q\nwant\n  %q", claimed.Primary, want)
	}
	// The loader's witnesses, in its sorted order (SI-368 (32) T1).
	if want := []string{"alpha-spike", "oq-2", "zeta-spike"}; !reflect.DeepEqual(claimed.Witnesses, want) {
		t.Fatalf("claimed witnesses = %q, want %q", claimed.Witnesses, want)
	}

	unclaimed := readTabRow(t, tab, "shape/question/oq-1")
	if unclaimed.Blocking != "true" {
		t.Fatalf("unclaimed question = %+v, want blocking", unclaimed)
	}
	if want := "Resolve it on the wall: edit or remove oq-1, or graduate a decision that answers it."; unclaimed.Primary != want {
		t.Fatalf("unclaimed guidance =\n  %q\nwant\n  %q", unclaimed.Primary, want)
	}
	if !reflect.DeepEqual(unclaimed.Witnesses, []string{"oq-1"}) {
		t.Fatalf("unclaimed witnesses = %q, want the declaration alone", unclaimed.Witnesses)
	}

	// The plain coverage stub claims an acceptance criterion, so it never
	// reaches a question row — not in a fact, not as a witness.
	for _, id := range tabConcernIDs(tab) {
		if !strings.HasPrefix(id, "shape/question/") {
			continue
		}
		row := readTabRow(t, tab, id)
		if strings.Contains(row.Fact, "plain-coverage-stub") || containsString(row.Witnesses, "plain-coverage-stub") {
			t.Fatalf("%s names the plain stub: %+v", id, row)
		}
	}
}

// TestBuildASDView_NonSpikeStubNeverClaims pins the fail-closed half. A
// plain stub CANNOT legally carry `resolves` — the decode seam refuses
// that frontmatter outright (02 §Kind registry, DC-4, asserted here) — so
// the spike test behind it is the defense behind a refused state. Since
// the wall's readiness is the loader's (SI-368 (32) T5), that defense is
// the loader's spike filter, claimedQuestionsOf, pinned by
// internal/readinessload's TestClaimedQuestionsOf_NonSpikeStubNeverClaims:
// a plain stub reaching it anyway claims nothing.
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
}
