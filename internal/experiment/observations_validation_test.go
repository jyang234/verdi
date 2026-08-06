package experiment

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const (
	obsGuardsOK       = `[{"id":"behavioral-equivalence","verdict":"pass","witness":null},{"id":"tenant-isolation","verdict":"pass","witness":null}]`
	obsMeasurementsOK = `[{"id":"request-latency","value":18.0,"unit":"ms","source":"evaluator-measured"},{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`
)

// smallRoundsDefinition returns a locked definition identical to the
// shared fixture but with rounds reduced to 2, keeping observation-set
// fixtures small, plus its computed digest.
func smallRoundsDefinition(t *testing.T) (def Definition, digest string) {
	t.Helper()
	unlocked := mutate(t, "rounds: 10", "rounds: 2")
	def = mustDecodeDefinition(t, unlocked)
	digest, err := DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	lockedDoc := unlocked + "lock:\n  definition_digest: " + digest + "\n"
	def = mustDecodeDefinition(t, lockedDoc)
	return def, digest
}

func obsRecord(defDigest, run, candidate string, round int, guards, measurements, disclosures string) string {
	return fmt.Sprintf(`{"schema": "verdi.experiment-observation/v1", "experiment_digest": %q, "run": %q, "candidate": %q, "round": %d, "guards": %s, "measurements": %s, "disclosures": %s}`,
		defDigest, run, candidate, round, guards, measurements, disclosures)
}

func decodeObs(t *testing.T, lines ...string) []Observation {
	t.Helper()
	data := strings.Join(lines, "\n") + "\n"
	obs, err := DecodeObservations([]byte(data))
	if err != nil {
		t.Fatalf("DecodeObservations() unexpected error: %v", err)
	}
	return obs
}

func validObservationLines(defDigest string) []string {
	return []string{
		obsRecord(defDigest, "run-1", "baseline", 1, obsGuardsOK, obsMeasurementsOK, "[]"),
		obsRecord(defDigest, "run-1", "baseline", 2, obsGuardsOK, obsMeasurementsOK, "[]"),
		obsRecord(defDigest, "run-1", "facts-cache", 1, obsGuardsOK, obsMeasurementsOK, "[]"),
		obsRecord(defDigest, "run-1", "facts-cache", 2, obsGuardsOK, obsMeasurementsOK, "[]"),
	}
}

func TestValidateObservationsHappyPath(t *testing.T) {
	def, digest := smallRoundsDefinition(t)
	obs := decodeObs(t, validObservationLines(digest)...)
	if err := ValidateObservations(def, obs); err != nil {
		t.Fatalf("ValidateObservations() unexpected error: %v", err)
	}
}

func TestValidateObservationsNotLocked(t *testing.T) {
	unlocked := mustDecodeDefinition(t, mutate(t, "rounds: 10", "rounds: 2"))
	digest, err := DefinitionDigest(unlocked)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	obs := decodeObs(t, validObservationLines(digest)...)
	if err := ValidateObservations(unlocked, obs); err == nil {
		t.Errorf("ValidateObservations() on an unlocked definition = nil error, want error")
	}
}

func TestValidateObservationsRejects(t *testing.T) {
	def, digest := smallRoundsDefinition(t)

	tests := []struct {
		name  string
		lines []string
	}{
		{"empty set", nil},
		{"mismatched experiment_digest", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digestOf("0"), "run-1", "baseline", 1, obsGuardsOK, obsMeasurementsOK, "[]")
			return l
		}()},
		{"mixed run ids", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-2", "baseline", 1, obsGuardsOK, obsMeasurementsOK, "[]")
			return l
		}()},
		{"unregistered candidate", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "nonexistent", 1, obsGuardsOK, obsMeasurementsOK, "[]")
			return l
		}()},
		{"round out of range", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 3, obsGuardsOK, obsMeasurementsOK, "[]")
			return l
		}()},
		{"duplicate candidate/round", append(validObservationLines(digest), obsRecord(digest, "run-1", "baseline", 1, obsGuardsOK, obsMeasurementsOK, "[]"))},
		{"unregistered guard id (bound guard as verdict)", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1,
				`[{"id":"behavioral-equivalence","verdict":"pass","witness":null},{"id":"tenant-isolation","verdict":"pass","witness":null},{"id":"peak-rss","verdict":"pass","witness":null}]`,
				obsMeasurementsOK, "[]")
			return l
		}()},
		{"missing required guard verdict", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1,
				`[{"id":"behavioral-equivalence","verdict":"pass","witness":null}]`,
				obsMeasurementsOK, "[]")
			return l
		}()},
		{"primary metric measurement missing", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
		{"primary metric only candidate-reported", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"request-latency","value":18.0,"unit":"ms","source":"candidate-reported"},{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
		{"primary metric unit mismatch", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"request-latency","value":18.0,"unit":"seconds","source":"evaluator-measured"},{"id":"peak-rss","value":109,"unit":"MiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
		{"bound guard measurement missing", func() []string {
			l := validObservationLines(digest)
			l[0] = obsRecord(digest, "run-1", "baseline", 1, obsGuardsOK,
				`[{"id":"request-latency","value":18.0,"unit":"ms","source":"evaluator-measured"}]`, "[]")
			return l
		}()},
		{"bound guard unit inconsistent across records", func() []string {
			l := validObservationLines(digest)
			l[1] = obsRecord(digest, "run-1", "baseline", 2, obsGuardsOK,
				`[{"id":"request-latency","value":18.0,"unit":"ms","source":"evaluator-measured"},{"id":"peak-rss","value":109,"unit":"KiB","source":"harness-measured"}]`, "[]")
			return l
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obs []Observation
			if tt.lines != nil {
				obs = decodeObs(t, tt.lines...)
			}
			err := ValidateObservations(def, obs)
			if err == nil {
				t.Fatalf("ValidateObservations(%s) = nil error, want error", tt.name)
			}
			if !errors.Is(err, ErrObservationIntegrity) {
				t.Errorf("ValidateObservations(%s) error %v does not wrap ErrObservationIntegrity", tt.name, err)
			}
		})
	}
}

func TestValidateCompleteHappyPath(t *testing.T) {
	def, digest := smallRoundsDefinition(t)
	obs := decodeObs(t, validObservationLines(digest)...)
	if err := ValidateComplete(def, obs); err != nil {
		t.Fatalf("ValidateComplete() unexpected error: %v", err)
	}
}

func TestValidateCompleteMissingEntry(t *testing.T) {
	def, digest := smallRoundsDefinition(t)
	lines := validObservationLines(digest)
	obs := decodeObs(t, lines[:3]...) // drop facts-cache round 2

	err := ValidateComplete(def, obs)
	if err == nil {
		t.Fatalf("ValidateComplete() with a missing (candidate, round) = nil error, want error")
	}
	if !errors.Is(err, ErrObservationIncomplete) {
		t.Errorf("ValidateComplete() error %v does not wrap ErrObservationIncomplete", err)
	}
	if !strings.Contains(err.Error(), "facts-cache") {
		t.Errorf("ValidateComplete() error %v does not name the missing candidate", err)
	}
}

func TestValidateCompletePropagatesIntegrityErrors(t *testing.T) {
	def, digest := smallRoundsDefinition(t)
	lines := validObservationLines(digest)
	lines[0] = obsRecord(digestOf("0"), "run-1", "baseline", 1, obsGuardsOK, obsMeasurementsOK, "[]")
	obs := decodeObs(t, lines...)

	err := ValidateComplete(def, obs)
	if err == nil {
		t.Fatalf("ValidateComplete() with an integrity violation = nil error, want error")
	}
	if !errors.Is(err, ErrObservationIntegrity) {
		t.Errorf("ValidateComplete() error %v does not wrap ErrObservationIntegrity (integrity failures must propagate, not be reported as incompleteness)", err)
	}
	if errors.Is(err, ErrObservationIncomplete) {
		t.Errorf("ValidateComplete() error %v wraps ErrObservationIncomplete, want only ErrObservationIntegrity for an integrity-violating input", err)
	}
}
