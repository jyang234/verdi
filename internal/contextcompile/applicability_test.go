package contextcompile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/policyartifact"
)

func TestEvaluateApplicability(t *testing.T) {
	t.Parallel()

	universal := explicitUniversalScope()
	tests := []struct {
		name        string
		in          ApplicabilityInput
		want        Applicability
		wantDisc    []DisclosureCode
		wantErrText string
	}{
		{
			name: "both universal need no candidate operands",
			in: ApplicabilityInput{
				Policy: universal, Request: universal, Phase: PhaseBuild,
			},
			want: ApplicabilityApplicable,
		},
		{
			name: "one universal and one exact phase",
			in: ApplicabilityInput{
				Policy: scopeWithPhases("build"), Request: universal, Phase: PhaseBuild,
			},
			want: ApplicabilityApplicable,
		},
		{
			name: "directory marker matches descendant",
			in: ApplicabilityInput{
				Policy: scopeWithPaths("cmd/"), Request: universal,
				CandidatePath: "cmd/verdi/main.go", Phase: PhaseBuild,
			},
			want: ApplicabilityApplicable,
		},
		{
			name: "directory marker rejects same prefix sibling",
			in: ApplicabilityInput{
				Policy: scopeWithPaths("cmd/"), Request: universal,
				CandidatePath: "cmdx/main.go", Phase: PhaseBuild,
			},
			want: ApplicabilityInapplicable,
		},
		{
			name: "directory marker does not match directory entry itself",
			in: ApplicabilityInput{
				Policy: scopeWithPaths("cmd/"), Request: universal,
				CandidatePath: "cmd", Phase: PhaseBuild,
			},
			want: ApplicabilityInapplicable,
		},
		{
			name: "exact file path matches",
			in: ApplicabilityInput{
				Policy: scopeWithPaths("go.mod"), Request: universal,
				CandidatePath: "go.mod", Phase: PhaseBuild,
			},
			want: ApplicabilityApplicable,
		},
		{
			name: "exact file path rejects descendant",
			in: ApplicabilityInput{
				Policy: scopeWithPaths("cmd"), Request: universal,
				CandidatePath: "cmd/main.go", Phase: PhaseBuild,
			},
			want: ApplicabilityInapplicable,
		},
		{
			name: "exact environment and ref match",
			in: ApplicabilityInput{
				Policy: scopeWithEnvironmentAndRef("production", "spec/widget"), Request: universal,
				CandidateRef: "spec/widget", Environment: "production", Phase: PhaseReview,
			},
			want: ApplicabilityApplicable,
		},
		{
			name: "disjoint policy and request phase sets are known inapplicable",
			in: ApplicabilityInput{
				Policy: scopeWithPhases("design"), Request: scopeWithPhases("build"), Phase: PhaseBuild,
			},
			want: ApplicabilityInapplicable,
		},
		{
			name: "disjoint environment sets need no candidate operand",
			in: ApplicabilityInput{
				Policy: scopeWithEnvironments("production"), Request: scopeWithEnvironments("staging"), Phase: PhaseBuild,
			},
			want: ApplicabilityInapplicable,
		},
		{
			name: "disjoint ref sets need no candidate operand",
			in: ApplicabilityInput{
				Policy: scopeWithRefs("spec/alpha"), Request: scopeWithRefs("spec/beta"), Phase: PhaseBuild,
			},
			want: ApplicabilityInapplicable,
		},
		{
			name: "disjoint path sets need no candidate operand",
			in: ApplicabilityInput{
				Policy: scopeWithPaths("cmd/"), Request: scopeWithPaths("docs/"), Phase: PhaseBuild,
			},
			want: ApplicabilityInapplicable,
		},
		{
			name: "missing path needed for comparison is unknown",
			in: ApplicabilityInput{
				Policy: scopeWithPaths("cmd/"), Request: universal, Phase: PhaseBuild,
			},
			want:     ApplicabilityUnknown,
			wantDisc: []DisclosureCode{DisclosureApplicabilityUnknown},
		},
		{
			name: "missing ref needed for comparison is unknown",
			in: ApplicabilityInput{
				Policy: universal, Request: scopeWithRefs("spec/widget"), Phase: PhaseBuild,
			},
			want:     ApplicabilityUnknown,
			wantDisc: []DisclosureCode{DisclosureApplicabilityUnknown},
		},
		{
			name: "missing environment needed for comparison is unknown",
			in: ApplicabilityInput{
				Policy: scopeWithEnvironments("production"), Request: universal, Phase: PhaseBuild,
			},
			want:     ApplicabilityUnknown,
			wantDisc: []DisclosureCode{DisclosureApplicabilityUnknown},
		},
		{
			name: "known empty dimension dominates a different unknown dimension",
			in: ApplicabilityInput{
				Policy: scopeWithPhaseAndRef("review", "spec/widget"), Request: universal, Phase: PhaseBuild,
			},
			want: ApplicabilityInapplicable,
		},
		{
			name: "unknown phase enum fails closed",
			in: ApplicabilityInput{
				Policy: universal, Request: universal, Phase: Phase("deploy"),
			},
			wantErrText: "unknown phase",
		},
		{
			name: "invalid scope path fails closed",
			in: ApplicabilityInput{
				Policy: scopeWithPaths("cmd//"), Request: universal, Phase: PhaseBuild,
			},
			wantErrText: "scope.paths",
		},
		{
			name: "invalid concrete candidate path fails closed",
			in: ApplicabilityInput{
				Policy: universal, Request: universal, CandidatePath: "../secret", Phase: PhaseBuild,
			},
			wantErrText: "candidate path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := EvaluateApplicability(tt.in)
			if tt.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
					t.Fatalf("EvaluateApplicability() error = %v, want containing %q", err, tt.wantErrText)
				}
				return
			}
			if err != nil {
				t.Fatalf("EvaluateApplicability() error = %v", err)
			}
			if got.State != tt.want {
				t.Errorf("State = %q, want %q", got.State, tt.want)
			}
			if want := nonNilDisclosures(tt.wantDisc); !reflect.DeepEqual(got.Disclosures, want) {
				t.Errorf("Disclosures = %#v, want %#v", got.Disclosures, want)
			}
		})
	}
}

func explicitUniversalScope() policyartifact.Scope {
	return policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
}

func scopeWithPhases(phases ...string) policyartifact.Scope {
	scope := explicitUniversalScope()
	scope.Phases = phases
	return scope
}

func scopeWithPaths(paths ...string) policyartifact.Scope {
	scope := explicitUniversalScope()
	scope.Paths = paths
	return scope
}

func scopeWithRefs(refs ...string) policyartifact.Scope {
	scope := explicitUniversalScope()
	scope.Refs = refs
	return scope
}

func scopeWithEnvironments(environments ...string) policyartifact.Scope {
	scope := explicitUniversalScope()
	scope.Environments = environments
	return scope
}

func scopeWithEnvironmentAndRef(environment, ref string) policyartifact.Scope {
	scope := explicitUniversalScope()
	scope.Environments = []string{environment}
	scope.Refs = []string{ref}
	return scope
}

func scopeWithPhaseAndRef(phase, ref string) policyartifact.Scope {
	scope := explicitUniversalScope()
	scope.Phases = []string{phase}
	scope.Refs = []string{ref}
	return scope
}

func nonNilDisclosures(in []DisclosureCode) []DisclosureCode {
	if in == nil {
		return []DisclosureCode{}
	}
	return in
}
