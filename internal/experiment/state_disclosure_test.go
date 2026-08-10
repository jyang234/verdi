package experiment

import (
	"reflect"
	"testing"
)

func TestStateDisclosureCodeValidate(t *testing.T) {
	tests := []struct {
		name    string
		code    StateDisclosureCode
		wantErr bool
	}{
		{"registration lock witness", DisclosureRegistrationLockWitness, false},
		{"ratification actor resolution", DisclosureRatificationActorResolution, false},
		{"empty", StateDisclosureCode(""), true},
		{"unknown", StateDisclosureCode("environment-policy-receipt"), true},
		{"case-shifted", StateDisclosureCode("Registration-Lock-Human-Witness"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.code.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("StateDisclosureCode(%q).Validate() = nil error, want error", tt.code)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("StateDisclosureCode(%q).Validate() unexpected error: %v", tt.code, err)
			}
		})
	}
}

func TestStateDisclosureValidate(t *testing.T) {
	tests := []struct {
		name    string
		d       StateDisclosure
		wantErr bool
	}{
		{"lock witness", StateDisclosure{Code: DisclosureRegistrationLockWitness, Detail: "why it is unproven"}, false},
		{"actor resolution", StateDisclosure{Code: DisclosureRatificationActorResolution, Detail: "why it is unproven"}, false},
		{"zero value", StateDisclosure{}, true},
		{"unknown code", StateDisclosure{Code: StateDisclosureCode("made-up"), Detail: "d"}, true},
		{"empty detail", StateDisclosure{Code: DisclosureRegistrationLockWitness}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.d.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("StateDisclosure(%+v).Validate() = nil error, want error", tt.d)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("StateDisclosure(%+v).Validate() unexpected error: %v", tt.d, err)
			}
		})
	}
}

// disclosureCodes projects ds onto its codes, in the order returned — the
// order itself is part of what DeriveState promises (SI-44).
func disclosureCodes(ds []StateDisclosure) []StateDisclosureCode {
	codes := make([]StateDisclosureCode, 0, len(ds))
	for _, d := range ds {
		codes = append(codes, d.Code)
	}
	return codes
}

// TestDeriveStateDisclosures is SI-44's table: which authority conjuncts
// DeriveState reports as disclosed-unproven at each rung of the ladder,
// and in which order. Artifact bytes cannot prove AC-5's human/merge
// witness for a lock, nor OD-4's authenticated principal resolution for a
// ratification actor, and CO-1 forbids silently assuming either.
func TestDeriveStateDisclosures(t *testing.T) {
	// ratified builds the full artifact set including a ratification bound
	// to the result's own digest.
	ratified := func(t *testing.T, dir string) {
		t.Helper()
		doc, digest := lockedDefinitionDoc(t)
		writeExperimentFile(t, dir, "experiment.yaml", doc)
		writeCandidatePatches(t, dir)
		writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
		resultDoc := validResultJSONForDigest(digest, VerdictProvenWinner)
		writeExperimentFile(t, dir, "result.json", resultDoc)
		res, err := DecodeResult([]byte(resultDoc))
		if err != nil {
			t.Fatalf("DecodeResult() unexpected error: %v", err)
		}
		resultDigest, err := ResultDigest(res)
		if err != nil {
			t.Fatalf("ResultDigest() unexpected error: %v", err)
		}
		writeExperimentFile(t, dir, "ratification.yaml", "schema: verdi.experiment-ratification/v1\n"+
			"result_digest: "+resultDigest+"\n"+
			"actor: "+validActor+"\n"+
			"disposition: select-recommended\n")
	}

	tests := []struct {
		name      string
		setup     func(t *testing.T, dir string)
		wantState State
		wantCodes []StateDisclosureCode
	}{
		{
			name:      "exploratory discloses nothing",
			setup:     func(t *testing.T, dir string) {},
			wantState: StateExploratory,
			wantCodes: []StateDisclosureCode{},
		},
		{
			name: "unlocked definition discloses nothing",
			setup: func(t *testing.T, dir string) {
				writeExperimentFile(t, dir, "experiment.yaml", validDefinitionYAML())
			},
			wantState: StateExploratory,
			wantCodes: []StateDisclosureCode{},
		},
		{
			name: "registered discloses the lock witness",
			setup: func(t *testing.T, dir string) {
				doc, _ := lockedDefinitionDoc(t)
				writeExperimentFile(t, dir, "experiment.yaml", doc)
				writeCandidatePatches(t, dir)
			},
			wantState: StateRegistered,
			wantCodes: []StateDisclosureCode{DisclosureRegistrationLockWitness},
		},
		{
			name: "measured discloses the lock witness",
			setup: func(t *testing.T, dir string) {
				doc, digest := lockedDefinitionDoc(t)
				writeExperimentFile(t, dir, "experiment.yaml", doc)
				writeCandidatePatches(t, dir)
				writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
			},
			wantState: StateMeasured,
			wantCodes: []StateDisclosureCode{DisclosureRegistrationLockWitness},
		},
		{
			name: "recommended discloses the lock witness",
			setup: func(t *testing.T, dir string) {
				doc, digest := lockedDefinitionDoc(t)
				writeExperimentFile(t, dir, "experiment.yaml", doc)
				writeCandidatePatches(t, dir)
				writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
				writeExperimentFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictProvenWinner))
			},
			wantState: StateRecommended,
			wantCodes: []StateDisclosureCode{DisclosureRegistrationLockWitness},
		},
		{
			name: "inconclusive discloses the lock witness",
			setup: func(t *testing.T, dir string) {
				doc, digest := lockedDefinitionDoc(t)
				writeExperimentFile(t, dir, "experiment.yaml", doc)
				writeCandidatePatches(t, dir)
				writeExperimentFile(t, dir, "observations.jsonl", completeObservationsJSONLForDigest(digest))
				writeExperimentFile(t, dir, "result.json", validResultJSONForDigest(digest, VerdictDisclosedUnproven))
			},
			wantState: StateInconclusive,
			wantCodes: []StateDisclosureCode{DisclosureRegistrationLockWitness},
		},
		{
			name:      "ratified discloses the lock witness and the actor resolution, in that order",
			setup:     ratified,
			wantState: StateRatified,
			wantCodes: []StateDisclosureCode{DisclosureRegistrationLockWitness, DisclosureRatificationActorResolution},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			state, disclosures, err := DeriveState(dir, testExperimentDir, acceptResult)
			if err != nil {
				t.Fatalf("DeriveState() unexpected error: %v", err)
			}
			if state != tt.wantState {
				t.Fatalf("DeriveState() = %q, want %q", state, tt.wantState)
			}
			if got := disclosureCodes(disclosures); !reflect.DeepEqual(got, tt.wantCodes) {
				t.Errorf("disclosure codes = %v, want %v", got, tt.wantCodes)
			}
			for i, d := range disclosures {
				if err := d.Validate(); err != nil {
					t.Errorf("disclosures[%d].Validate() unexpected error: %v", i, err)
				}
			}

			// Determinism: a second derivation over the same bytes returns
			// the same disclosures in the same order.
			_, again, err := DeriveState(dir, testExperimentDir, acceptResult)
			if err != nil {
				t.Fatalf("DeriveState() second call unexpected error: %v", err)
			}
			if !reflect.DeepEqual(disclosures, again) {
				t.Errorf("disclosures differ across two derivations: %+v vs %+v", disclosures, again)
			}
		})
	}
}

// TestDeriveStateErrorCarriesNoDisclosures pins the same CO-1 discipline
// the State return already follows: an operational failure yields no
// disclosures, because there is no derived state for them to qualify.
func TestDeriveStateErrorCarriesNoDisclosures(t *testing.T) {
	dir := t.TempDir()
	writeExperimentFile(t, dir, "experiment.yaml", "not: valid: yaml: at all:\n")

	state, disclosures, err := DeriveState(dir, testExperimentDir, acceptResult)
	if err == nil {
		t.Fatalf("DeriveState() with a corrupt definition = (%q, %+v, nil error), want error", state, disclosures)
	}
	if len(disclosures) != 0 {
		t.Errorf("DeriveState() = %+v disclosures alongside an error, want none", disclosures)
	}
}
