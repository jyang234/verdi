package recovery

import "testing"

// validProjection returns a well-formed Projection fixture: one
// ref-scoped state with an executable choice and one store-scoped state
// with a manual choice, mirroring internal/journey/record_test.go's
// validRecord builder.
func validProjection(t *testing.T) Projection {
	t.Helper()
	return Projection{
		Schema: SchemaID,
		Ref:    "spec/checkout",
		Branch: "design/checkout",
		Head:   "abc123",
		States: []RecognizedState{
			{
				Code:           StateEmptyBranchCut,
				Scope:          ScopeRef,
				Target:         "close/checkout",
				Facts:          []string{"close/checkout tip abc123 is an ancestor of main"},
				Uncertainties:  []Uncertainty{},
				StepsCompleted: []string{"close/checkout was checked out"},
				InvariantsHeld: []string{"index is empty"},
				Choices: []Choice{
					{
						ID:             "unwind-branch-cut:close/checkout",
						Summary:        "unwind the empty close/checkout branch cut",
						Preconditions:  []string{"close/checkout still points at abc123"},
						Effects:        []string{"switch back to main", "delete close/checkout with git branch -d"},
						Reversibility:  ReversibilityNoneNeeded,
						Confirmation:   "--apply unwind-branch-cut:close/checkout",
						Postconditions: []string{"close/checkout does not exist"},
						Executor:       "branchcut.Unwind",
						ManualCommands: []string{},
					},
				},
			},
			{
				Code:           StateStaleLock,
				Scope:          ScopeStore,
				Target:         "/root/.verdi/data/writer.lock",
				Facts:          []string{"pid 4242 is not alive"},
				Uncertainties:  []Uncertainty{},
				StepsCompleted: []string{},
				InvariantsHeld: []string{"HEAD is abc123 and no ritual branch was modified by this run"},
				Choices: []Choice{
					{
						ID:             "remove-stale-lock:/root/.verdi/data/writer.lock",
						Summary:        "remove the stale writer lock",
						Preconditions:  []string{"the lock still names pid 4242"},
						Effects:        []string{"delete /root/.verdi/data/writer.lock"},
						Reversibility:  ReversibilityIrreversible,
						Confirmation:   "none: no executor",
						Postconditions: []string{"/root/.verdi/data/writer.lock does not exist"},
						Executor:       "none",
						ManualCommands: []string{"rm /root/.verdi/data/writer.lock"},
					},
				},
			},
		},
		Disclosures: []string{},
	}
}

func TestProjectionValidateHappyPath(t *testing.T) {
	p := validProjection(t)
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate() on well-formed fixture: unexpected error: %v", err)
	}
	if !p.Recognized() {
		t.Fatal("Recognized() = false on a projection with states, want true")
	}
}

func TestProjectionRecognizedEmpty(t *testing.T) {
	p := validProjection(t)
	p.States = []RecognizedState{}
	if p.Recognized() {
		t.Fatal("Recognized() = true on a projection with no states, want false")
	}
}

func TestProjectionValidateNegative(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Projection)
	}{
		{
			name:   "unknown schema id",
			mutate: func(p *Projection) { p.Schema = "verdi.recovery-projection/v2" },
		},
		{
			name: "states unsorted",
			mutate: func(p *Projection) {
				p.States[0], p.States[1] = p.States[1], p.States[0]
			},
		},
		{
			name: "duplicate code+target",
			mutate: func(p *Projection) {
				p.States[1] = p.States[0]
			},
		},
		{
			name: "executable choice with manual commands",
			mutate: func(p *Projection) {
				p.States[0].Choices[0].ManualCommands = []string{"git branch -d close/checkout"}
			},
		},
		{
			name: "manual choice with no manual commands",
			mutate: func(p *Projection) {
				p.States[1].Choices[0].ManualCommands = []string{}
			},
		},
		{
			name: "executable choice id lacks its target after the colon",
			mutate: func(p *Projection) {
				p.States[0].Choices[0].ID = "unwind-branch-cut"
				p.States[0].Choices[0].Confirmation = "--apply unwind-branch-cut"
			},
		},
		{
			name: "uncertainty with empty witness",
			mutate: func(p *Projection) {
				p.States[0].Uncertainties = []Uncertainty{{Text: "something is unclear", Witness: ""}}
			},
		},
		{
			name: "control character in a string",
			mutate: func(p *Projection) {
				p.States[0].Facts = []string{"close/checkout tip abc123\x00 is an ancestor of main"}
			},
		},
		{
			name:   "empty head",
			mutate: func(p *Projection) { p.Head = "" },
		},
		{
			name:   "code outside the closed set",
			mutate: func(p *Projection) { p.States[0].Code = StateCode("something-else") },
		},
		{
			name:   "empty facts",
			mutate: func(p *Projection) { p.States[0].Facts = []string{} },
		},
		{
			name:   "empty invariants_held",
			mutate: func(p *Projection) { p.States[0].InvariantsHeld = []string{} },
		},
		{
			name: "duplicate executable choice id across different states",
			mutate: func(p *Projection) {
				// Same target, different code (never colliding on
				// (code,target) or ordering) but an identical executable
				// choice id — R-RR3-3 makes that id the byte-for-byte
				// --apply key, so two states sharing one is ambiguous.
				p.States[1].Code = StateStrandedResidue
				p.States[1].Target = "close/checkout"
				p.States[1].Choices[0] = Choice{
					ID:             "unwind-branch-cut:close/checkout",
					Summary:        "duplicate id collision",
					Preconditions:  []string{"x"},
					Effects:        []string{"y"},
					Reversibility:  ReversibilityIrreversible,
					Confirmation:   "--apply unwind-branch-cut:close/checkout",
					Postconditions: []string{"z"},
					Executor:       "branchcut.Unwind",
					ManualCommands: []string{},
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validProjection(t)
			tc.mutate(&p)
			if err := p.Validate(); err == nil {
				t.Fatalf("Validate() on mutated projection (%s): want error, got nil", tc.name)
			}
		})
	}
}
