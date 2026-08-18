package readinesspilot

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/journey"
)

func TestValidateSnapshotHappyPath(t *testing.T) {
	t.Parallel()

	if err := validSnapshot().Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateSnapshotRejectsClosedContractViolations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Snapshot)
		wantErr string
	}{
		{
			name: "unknown state",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0].State = State("ready")
			},
			wantErr: "unknown state",
		},
		{
			name: "unknown area id",
			mutate: func(s *Snapshot) {
				s.Areas[0].ID = AreaID("invented")
			},
			wantErr: "area",
		},
		{
			name: "unknown timing",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0].Timing = Timing("later")
			},
			wantErr: "timing",
		},
		{
			name: "unknown target class",
			mutate: func(s *Snapshot) {
				s.TargetClass = "initiative"
			},
			wantErr: "target class",
		},
		{
			name: "nil areas",
			mutate: func(s *Snapshot) {
				s.Areas = nil
			},
			wantErr: "areas must be non-nil",
		},
		{
			name: "nil attention",
			mutate: func(s *Snapshot) {
				s.Attention = nil
			},
			wantErr: "attention must be non-nil",
		},
		{
			name: "nil all concerns",
			mutate: func(s *Snapshot) {
				s.AllConcerns = nil
			},
			wantErr: "all concerns must be non-nil",
		},
		{
			name: "nil witnesses",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0].Witnesses = nil
			},
			wantErr: "witnesses must be non-nil",
		},
		{
			name: "nil cli",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0].Destination.CLI = nil
			},
			wantErr: "CLI must be non-nil",
		},
		{
			name: "invalid concern id empty component",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0].ID = "shape/question/"
			},
			wantErr: "invalid id",
		},
		{
			name: "invalid concern id control",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0].ID = "shape/question/bad\nquestion"
			},
			wantErr: "invalid id",
		},
		{
			name: "accepts unknown concern identity vocabulary",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0].ID = "shape/invented"
			},
			wantErr: "not in the closed concern identity vocabulary",
		},
		{
			name: "accepts empty witness element",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0].Witnesses = []string{""}
			},
			wantErr: "empty witness",
		},
		{
			name: "unsorted witnesses",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0].Witnesses = []string{"b", "a"}
			},
			wantErr: "sorted and deduplicated",
		},
		{
			name: "duplicate witnesses",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0].Witnesses = []string{"a", "a"}
			},
			wantErr: "sorted and deduplicated",
		},
		{
			name: "non journey work class",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0].WorkClass = journey.ClassMechanical
			},
			wantErr: "non-journey concern",
		},
		{
			name: "inconsistent area state",
			mutate: func(s *Snapshot) {
				s.Areas[0].State = StateUnproven
			},
			wantErr: "derived state",
		},
		{
			name: "all concerns out of order",
			mutate: func(s *Snapshot) {
				s.AllConcerns[0], s.AllConcerns[1] = s.AllConcerns[1], s.AllConcerns[0]
			},
			wantErr: "area and id order",
		},
		{
			name: "area has no concern",
			mutate: func(s *Snapshot) {
				s.AllConcerns = s.AllConcerns[1:]
			},
			wantErr: "has no concern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := validSnapshot()
			tt.mutate(&snapshot)
			if err := snapshot.Validate(); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDestinationUnionAndTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Snapshot)
		wantErr string
	}{
		{
			name: "unresolved concern has no destination",
			mutate: func(s *Snapshot) {
				mutateConcern(s, "shape/problem", func(c *Concern) {
					c.Destination = Destination{CLI: []string{}}
				})
			},
			wantErr: "exactly one destination",
		},
		{
			name: "unresolved concern has both destinations",
			mutate: func(s *Snapshot) {
				mutateConcern(s, "shape/problem", func(c *Concern) {
					c.Destination = Destination{BoardPath: "/board/spec/example", CLI: []string{"verdi", "lint"}}
				})
			},
			wantErr: "exactly one destination",
		},
		{
			name: "board path is not root relative",
			mutate: func(s *Snapshot) {
				mutateConcern(s, "shape/problem", func(c *Concern) {
					c.Destination = Destination{BoardPath: "board/spec/example", CLI: []string{}}
				})
			},
			wantErr: "root-relative",
		},
		{
			name: "empty cli token",
			mutate: func(s *Snapshot) {
				mutateConcern(s, "shape/problem", func(c *Concern) {
					c.Destination = Destination{CLI: []string{"verdi", ""}}
				})
			},
			wantErr: "empty CLI token",
		},
		{
			name: "control bearing cli token",
			mutate: func(s *Snapshot) {
				mutateConcern(s, "shape/problem", func(c *Concern) {
					c.Destination = Destination{CLI: []string{"verdi", "lint\n--fix"}}
				})
			},
			wantErr: "control-bearing CLI token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := validUnresolvedSnapshot()
			tt.mutate(&snapshot)
			if err := snapshot.Validate(); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateProvenConcernRejectsDestination(t *testing.T) {
	t.Parallel()

	snapshot := validSnapshot()
	snapshot.AllConcerns[0].Destination.BoardPath = "/board/spec/example"
	if err := snapshot.Validate(); err == nil || !strings.Contains(err.Error(), "proven concern") {
		t.Fatalf("Validate() error = %v, want proven-concern destination refusal", err)
	}
}

func TestValidateJourneyWorkClass(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		workClass journey.BlockerClass
		wantErr   string
	}{
		{name: "missing", workClass: "", wantErr: "journey concern work class"},
		{name: "invalid", workClass: journey.BlockerClass("easy"), wantErr: "journey concern work class"},
		{name: "valid", workClass: journey.ClassGovernance},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := validOrderedQueueSnapshot()
			mutateConcern(&snapshot, "review/blocker/principal-resolution-unproven/review", func(c *Concern) {
				c.WorkClass = tt.workClass
			})
			err := snapshot.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFocusAndAttentionLosslessness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Snapshot)
		wantErr string
	}{
		{
			name: "missing current focus",
			mutate: func(s *Snapshot) {
				s.CurrentFocus = ""
			},
			wantErr: "current focus",
		},
		{
			name: "queue omission",
			mutate: func(s *Snapshot) {
				s.Attention = s.Attention[1:]
			},
			wantErr: "omits unresolved concern",
		},
		{
			name: "queue duplication",
			mutate: func(s *Snapshot) {
				s.Attention = append(s.Attention, s.Attention[0])
			},
			wantErr: "duplicate concern",
		},
		{
			name: "queue order",
			mutate: func(s *Snapshot) {
				s.Attention[0], s.Attention[1] = s.Attention[1], s.Attention[0]
			},
			wantErr: "attention order",
		},
		{
			name: "queue value differs",
			mutate: func(s *Snapshot) {
				s.Attention[0].Summary = "different summary"
			},
			wantErr: "differs from all concerns",
		},
		{
			name: "unresolved concern lacks witness",
			mutate: func(s *Snapshot) {
				mutateConcern(s, "context/verdict", func(c *Concern) {
					c.Witnesses = []string{}
				})
			},
			wantErr: "unresolved concern must carry a witness",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := validOrderedQueueSnapshot()
			tt.mutate(&snapshot)
			if err := snapshot.Validate(); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func validSnapshot() Snapshot {
	concerns := []Concern{
		validConcern("shape/problem", AreaShape, StateProven, true, TimingCurrent),
		validConcern("success/contributor/static", AreaSuccess, StateProven, false, TimingCurrent),
		validConcern("context/verdict", AreaContext, StateProven, true, TimingCurrent),
		validConcern("review/action", AreaReview, StateProven, true, TimingCurrent),
	}
	return Snapshot{
		TargetRef:     "spec/example",
		TargetClass:   "feature",
		Branch:        "design/example",
		Head:          "0123456789abcdef0123456789abcdef01234567",
		RequestDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Areas: []Area{
			{ID: AreaShape, Label: "Shape proposal", State: StateProven},
			{ID: AreaSuccess, Label: "Show success", State: StateProven},
			{ID: AreaContext, Label: "Check context", State: StateProven},
			{ID: AreaReview, Label: "Request review", State: StateProven},
		},
		CurrentFocus: "",
		Attention:    []Concern{},
		AllConcerns:  concerns,
		StaleNotice:  "Startup snapshot at 0123456789abcdef0123456789abcdef01234567; restart verdi serve after an edit.",
	}
}

func validUnresolvedSnapshot() Snapshot {
	snapshot := validSnapshot()
	c := &snapshot.AllConcerns[0]
	c.State = StateUnproven
	c.Witnesses = []string{"problem availability is unproven"}
	c.Destination.BoardPath = "/b/design%2Fexample/board/spec/example"
	snapshot.Areas[0].State = StateUnproven
	snapshot.CurrentFocus = AreaShape
	snapshot.Attention = []Concern{*c}
	return snapshot
}

func validOrderedQueueSnapshot() Snapshot {
	snapshot := validSnapshot()
	shape := validConcern("shape/problem", AreaShape, StateUnproven, true, TimingCurrent)
	shape.Witnesses = []string{"problem is unavailable"}
	shape.Destination.BoardPath = "/b/design%2Fexample/board/spec/example"
	success := validConcern("success/contributor/static", AreaSuccess, StateViolated, false, TimingCurrent)
	success.Witnesses = []string{"static evidence failed"}
	success.Destination.BoardPath = "/b/design%2Fexample/board/spec/example"
	contextVerdict := validConcern("context/verdict", AreaContext, StateViolated, true, TimingCurrent)
	contextVerdict.Witnesses = []string{"policy conflict"}
	contextVerdict.Destination.CLI = []string{"verdi", "context", "conflict"}
	review := validConcern("review/blocker/principal-resolution-unproven/review", AreaReview, StateViolated, false, TimingEventual)
	review.WorkClass = journey.ClassGovernance
	review.Witnesses = []string{"principal resolution"}
	review.Destination.CLI = []string{"verdi", "journey", "spec/example"}
	snapshot.AllConcerns = []Concern{shape, success, contextVerdict, review}
	snapshot.Areas[0].State = StateUnproven
	snapshot.Areas[1].State = StateProven
	snapshot.Areas[2].State = StateViolated
	snapshot.Areas[3].State = StateProven
	snapshot.CurrentFocus = AreaShape
	snapshot.Attention = []Concern{contextVerdict, shape, success, review}
	return snapshot
}

func validConcern(id string, area AreaID, state State, blocking bool, timing Timing) Concern {
	return Concern{
		ID:        id,
		Area:      area,
		State:     state,
		Blocking:  blocking,
		Timing:    timing,
		Summary:   "source-derived readiness fact",
		Witnesses: []string{},
		Destination: Destination{
			CLI: []string{},
		},
	}
}

func mutateConcern(snapshot *Snapshot, id string, mutate func(*Concern)) {
	for i := range snapshot.AllConcerns {
		if snapshot.AllConcerns[i].ID == id {
			mutate(&snapshot.AllConcerns[i])
		}
	}
	for i := range snapshot.Attention {
		if snapshot.Attention[i].ID == id {
			mutate(&snapshot.Attention[i])
		}
	}
}
