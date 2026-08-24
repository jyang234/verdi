package experiment

import (
	"strconv"
	"testing"
)

func reproductionDefinition(t *testing.T, minimum *int) Definition {
	t.Helper()
	block := ""
	if minimum != nil {
		block = "reproduction:\n  minimum_valid_runs: " + strconv.Itoa(*minimum) + "\n"
	}
	def := mustDecodeDefinition(t, validDefinitionV2YAML(block))
	// A second non-baseline candidate makes disagreement between two proven
	// winners expressible without inventing an invalid baseline winner.
	def.Candidates = append(def.Candidates, Candidate{
		ID: "final-cache", Patch: "candidates/final-cache.patch",
		Digest: digestOf("e"), Base: def.BaseCommit,
	})
	digest, err := DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest(): %v", err)
	}
	def.Lock = &Lock{DefinitionDigest: digest}
	return def
}

func reproductionResult(t *testing.T, def Definition, run, winner string) Result {
	t.Helper()
	result, err := DecodeResult([]byte(validResultJSON()))
	if err != nil {
		t.Fatalf("DecodeResult(): %v", err)
	}
	digest, err := DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest(): %v", err)
	}
	result.Experiment = def.ID
	result.DefinitionDigest = digest
	result.Run = run
	result.Winner = winner
	for i := range result.Candidates {
		if result.Candidates[i].ID == winner {
			result.Candidates[i].Eligible = true
			result.Candidates[i].Violations = nil
		}
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result fixture Validate(): %v", err)
	}
	return result
}

func TestReproductionDerivation(t *testing.T) {
	two, three := 2, 3
	tests := []struct {
		name    string
		minimum *int
		build   func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification)
		want    ReproductionStatus
		wantErr bool
	}{
		{
			name:    "absent rule",
			minimum: nil,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				one := reproductionResult(t, def, "run-1", "facts-cache")
				two := reproductionResult(t, def, "run-2", "facts-cache")
				return []ReproductionRun{{Run: "run-1", Result: &one}, {Run: "run-2", Result: &two}}, nil
			},
			want: ReproductionStatus{ValidRuns: 2},
		},
		{
			name:    "too few results",
			minimum: &two,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				one := reproductionResult(t, def, "run-1", "facts-cache")
				return []ReproductionRun{{Run: "run-1", Result: &one}}, nil
			},
			want: ReproductionStatus{ValidRuns: 1, MinimumValidRuns: 2},
		},
		{
			name:    "agreeing winners",
			minimum: &two,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				one := reproductionResult(t, def, "run-1", "facts-cache")
				two := reproductionResult(t, def, "run-2", "facts-cache")
				return []ReproductionRun{{Run: "run-1", Result: &one}, {Run: "run-2", Result: &two}}, nil
			},
			want: ReproductionStatus{Reproduced: true, Winner: "facts-cache", ValidRuns: 2, MinimumValidRuns: 2},
		},
		{
			name:    "disagreeing winners",
			minimum: &two,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				one := reproductionResult(t, def, "run-1", "facts-cache")
				two := reproductionResult(t, def, "run-2", "final-cache")
				return []ReproductionRun{{Run: "run-1", Result: &one}, {Run: "run-2", Result: &two}}, nil
			},
			want: ReproductionStatus{ValidRuns: 2, MinimumValidRuns: 2},
		},
		{
			name:    "incomplete-only rows",
			minimum: &two,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				return []ReproductionRun{{Run: "run-1"}, {Run: "run-2"}}, nil
			},
			want: ReproductionStatus{MinimumValidRuns: 2},
		},
		{
			name:    "extra visible disagreement is not filtered",
			minimum: &two,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				one := reproductionResult(t, def, "run-1", "facts-cache")
				two := reproductionResult(t, def, "run-2", "facts-cache")
				three := reproductionResult(t, def, "run-3", "final-cache")
				return []ReproductionRun{{Run: "run-1", Result: &one}, {Run: "run-2", Result: &two}, {Run: "run-3", Result: &three}}, nil
			},
			want: ReproductionStatus{ValidRuns: 3, MinimumValidRuns: 2},
		},
		{
			name:    "minimum three remains unmet by two",
			minimum: &three,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				one := reproductionResult(t, def, "run-1", "facts-cache")
				two := reproductionResult(t, def, "run-2", "facts-cache")
				return []ReproductionRun{{Run: "run-1", Result: &one}, {Run: "run-2", Result: &two}}, nil
			},
			want: ReproductionStatus{ValidRuns: 2, MinimumValidRuns: 3},
		},
		{
			name:    "ratified select-other does not replace the reproduced winner",
			minimum: &two,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				one := reproductionResult(t, def, "run-1", "facts-cache")
				two := reproductionResult(t, def, "run-2", "facts-cache")
				digest, err := ResultDigest(one)
				if err != nil {
					t.Fatalf("ResultDigest(): %v", err)
				}
				ratification := Ratification{
					Schema: RatificationSchema, ResultDigest: digest, Actor: validActor,
					Disposition: DispositionSelectOther, Candidate: "baseline", Reason: "lower operational risk",
				}
				return []ReproductionRun{{Run: "run-1", Result: &one}, {Run: "run-2", Result: &two}}, &ratification
			},
			want: ReproductionStatus{Reproduced: true, Winner: "facts-cache", ValidRuns: 2, MinimumValidRuns: 2},
		},
		{
			name:    "malformed run id",
			minimum: &two,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				return []ReproductionRun{{Run: "Run_1"}}, nil
			},
			wantErr: true,
		},
		{
			name:    "duplicate run row",
			minimum: &two,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				return []ReproductionRun{{Run: "run-1"}, {Run: "run-1"}}, nil
			},
			wantErr: true,
		},
		{
			name:    "result run mismatch",
			minimum: &two,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				one := reproductionResult(t, def, "run-2", "facts-cache")
				return []ReproductionRun{{Run: "run-1", Result: &one}}, nil
			},
			wantErr: true,
		},
		{
			name:    "result winner is not registered by definition",
			minimum: &two,
			build: func(t *testing.T, def Definition) ([]ReproductionRun, *Ratification) {
				one := reproductionResult(t, def, "run-1", "facts-cache")
				one.Winner = "ghost-cache"
				for i := range one.Candidates {
					if one.Candidates[i].ID == "facts-cache" {
						one.Candidates[i].ID = "ghost-cache"
					}
				}
				if err := one.Validate(); err != nil {
					t.Fatalf("malformed binding fixture must remain schema-valid: %v", err)
				}
				return []ReproductionRun{{Run: "run-1", Result: &one}}, nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := reproductionDefinition(t, tt.minimum)
			runs, ratification := tt.build(t, def)
			got, err := DeriveReproduction(def, runs, ratification)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DeriveReproduction() = %+v, nil error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("DeriveReproduction() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("DeriveReproduction() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
