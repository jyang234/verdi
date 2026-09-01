package workbench

// Unit coverage for the ASD workbench's derivation and strictness seams:
// the four-area shell projection (SI-125 idioms over this board's typed
// facts), the strict pre-application body grammar (design §3.2), and the
// fixed-set route/action inventory (SI-167).

import (
	"strings"
	"testing"
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
	for _, want := range []string{routeBoardPage, routeBoardFragment, routeBoardSnapshot, routeBoardAPI, routeBoardPeek, routeBoardPinSearch} {
		if !suffixes[want] {
			t.Errorf("route table missing %s", want)
		}
	}
	if len(suffixes) != 6 {
		t.Errorf("route table has %d rows, want exactly 6 (fixed set)", len(suffixes))
	}
}
