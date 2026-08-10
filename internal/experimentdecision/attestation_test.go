package experimentdecision

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

// TestEnvironmentAttestationVerify is the direct table over the
// attestation check itself: only an attestation naming the locked
// definition's registered execution.environment_policy verifies (SI-42).
func TestEnvironmentAttestationVerify(t *testing.T) {
	def := lockDefinition(t)

	tests := []struct {
		name    string
		att     EnvironmentAttestation
		wantErr bool
	}{
		{"registered policy", EnvironmentAttestation{PolicyID: "local-isolated-v1"}, false},
		{"zero value", EnvironmentAttestation{}, true},
		{"empty policy id", EnvironmentAttestation{PolicyID: ""}, true},
		{"different policy", EnvironmentAttestation{PolicyID: "shared-host-v1"}, true},
		{"case-shifted policy", EnvironmentAttestation{PolicyID: "Local-Isolated-V1"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.att.verify(def)
			if tt.wantErr && err == nil {
				t.Fatalf("verify(%+v) = nil error, want error", tt.att)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("verify(%+v) unexpected error: %v", tt.att, err)
			}
			if tt.wantErr && err != nil && !strings.HasPrefix(err.Error(), errPrefix) {
				t.Errorf("verify() error = %q, want the %q prefix", err.Error(), errPrefix)
			}
		})
	}
}

// TestEvaluateRequiresEnvironmentAttestation proves AC-2 step 1's
// environment-policy conjunct is an INPUT requirement and not a verdict
// (SI-42, CO-6): a zero or mismatched attestation is an operational error
// carrying no Result, never a disclosed-unproven comparison.
func TestEvaluateRequiresEnvironmentAttestation(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)

	tests := []struct {
		name string
		att  EnvironmentAttestation
	}{
		{"zero attestation", EnvironmentAttestation{}},
		{"mismatched policy", EnvironmentAttestation{PolicyID: "shared-host-v1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Evaluate(def, obs, tt.att)
			if err == nil {
				t.Fatalf("Evaluate(%+v) = nil error, want an operational error", tt.att)
			}
			if !isZeroResult(res) {
				t.Fatalf("Evaluate() returned a nonzero Result alongside an error: %+v", res)
			}
			if !strings.HasPrefix(err.Error(), errPrefix) {
				t.Errorf("Evaluate() error = %q, want the %q prefix", err.Error(), errPrefix)
			}
		})
	}
}

// TestEvaluateMatchingAttestationEvaluates is the positive arm: the same
// evidence with the registered policy attested produces the ordinary
// proven-winner comparison.
func TestEvaluateMatchingAttestationEvaluates(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)

	res, err := Evaluate(def, obs, EnvironmentAttestation{PolicyID: def.Execution.EnvironmentPolicy})
	if err != nil {
		t.Fatalf("Evaluate() unexpected error: %v", err)
	}
	if res.Verdict != experiment.VerdictProvenWinner {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, experiment.VerdictProvenWinner)
	}
}

// TestEvaluateAttestationNeverEntersResult pins SI-42's scope boundary:
// the attestation gates emission, it is not recorded content. The Result
// schema is unchanged, so its canonical bytes must not mention the
// attested policy identifier anywhere.
func TestEvaluateAttestationNeverEntersResult(t *testing.T) {
	def := lockDefinition(t)
	obs := happyObservations(t, def, "run-1",
		map[string][]float64{"baseline": {40, 42, 41}, "candidate-a": {18, 19, 17}},
		map[string][]float64{"baseline": {100, 101, 99}, "candidate-a": {108, 109, 107}},
	)

	res := mustEvaluate(t, def, obs)
	rendered, err := RenderResult(res)
	if err != nil {
		t.Fatalf("RenderResult() unexpected error: %v", err)
	}
	if strings.Contains(string(rendered), def.Execution.EnvironmentPolicy) {
		t.Fatalf("rendered result mentions the attested environment policy %q:\n%s", def.Execution.EnvironmentPolicy, rendered)
	}
}
