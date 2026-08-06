package experiment

import (
	"strings"
	"testing"
)

func TestCheckNoDuplicateJSONKeys(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantErr bool
	}{
		{"flat duplicate", `{"a": 1, "a": 2}`, true},
		{"duplicate inside a nested object", `{"outer": {"a": 1, "a": 2}}`, true},
		{"duplicate inside an array element", `{"list": [{"a": 1, "a": 2}]}`, true},
		{"duplicate separated by an intervening object", `{"a": {"b": 1}, "a": 2}`, true},
		{"duplicate separated by an intervening array", `{"a": [1, 2], "a": 3}`, true},
		// Repeated key NAMES at different object levels (and in sibling
		// objects) are ordinary, correct JSON: the guard rejects a repeat
		// within ONE object only.
		{"same name at different levels", `{"id": "a", "inner": {"id": "b"}, "list": [{"id": "c"}, {"id": "d"}]}`, false},
		{"top-level array of objects", `[{"a": 1}, {"a": 2}]`, false},
		{"top-level scalar", `42`, false},
		{"empty object", `{}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkNoDuplicateJSONKeys([]byte(tt.doc))
			if tt.wantErr && err == nil {
				t.Errorf("checkNoDuplicateJSONKeys(%s) = nil error, want error", tt.doc)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("checkNoDuplicateJSONKeys(%s) = %v, want nil", tt.doc, err)
			}
		})
	}
}

// TestDecodeResultRejectsTwoFacedVerdict is the two-faced-result probe: a
// document whose textual verdict is "disclosed-unproven" but which the
// standard library's last-key-wins decode would read as "proven-winner".
// Either reading is a lie about the other, so the decode must FAIL rather
// than pick one.
func TestDecodeResultRejectsTwoFacedVerdict(t *testing.T) {
	doc := mutateResult(t, validResultJSON(),
		`"verdict": "proven-winner",`,
		`"verdict": "disclosed-unproven",
  "verdict": "proven-winner",`)
	res, err := DecodeResult([]byte(doc))
	if err == nil {
		t.Fatalf("DecodeResult() with a duplicated verdict key = nil error (verdict %q, winner %q), want error", res.Verdict, res.Winner)
	}
	if !strings.Contains(err.Error(), "verdict") {
		t.Errorf("DecodeResult() error = %v, want an error naming the duplicated key", err)
	}
}

func TestDecodeRejectsDuplicateKeys(t *testing.T) {
	t.Run("observation round", func(t *testing.T) {
		doc := mutateObservation(t, `"round": 4,`, `"round": 4,
  "round": 9,`)
		if _, err := DecodeObservation([]byte(doc)); err == nil {
			t.Errorf("DecodeObservation() with a duplicated round key = nil error, want error")
		}
	})

	t.Run("observation nested measurement key", func(t *testing.T) {
		doc := mutateObservation(t,
			`{"id": "request-latency", "value": 18.0, "unit": "ms", "source": "evaluator-measured"}`,
			`{"id": "request-latency", "value": 18.0, "value": 99.0, "unit": "ms", "source": "evaluator-measured"}`)
		if _, err := DecodeObservation([]byte(doc)); err == nil {
			t.Errorf("DecodeObservation() with a duplicated nested measurement key = nil error, want error")
		}
	})

	t.Run("observations.jsonl line", func(t *testing.T) {
		line := `{"schema": "verdi.experiment-observation/v1", "experiment_digest": "` + digestOf("a") +
			`", "run": "run-1", "run": "run-2", "candidate": "facts-cache", "round": 4, "guards": [], "measurements": [], "disclosures": []}`
		if _, err := DecodeObservations([]byte(line + "\n")); err == nil {
			t.Errorf("DecodeObservations() with a duplicated key on a line = nil error, want error")
		}
	})

	t.Run("capabilities", func(t *testing.T) {
		doc := mutateCapabilities(t, `"requires_network": false,`, `"requires_network": false,
  "requires_network": true,`)
		if _, err := DecodeCapabilities([]byte(doc)); err == nil {
			t.Errorf("DecodeCapabilities() with a duplicated key = nil error, want error")
		}
	})

	t.Run("capsule manifest", func(t *testing.T) {
		doc := mutateCapsule(t, `"selected": "facts-cache",`, `"selected": "facts-cache",
  "selected": "baseline",`)
		if _, err := DecodeCapsuleManifest([]byte(doc)); err == nil {
			t.Errorf("DecodeCapsuleManifest() with a duplicated key = nil error, want error")
		}
	})
}

func TestCheckSingleYAMLDocument(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantErr bool
	}{
		{"single document", "a: 1\n", false},
		{"single document with explicit start", "---\na: 1\n", false},
		{"single document with explicit end", "a: 1\n...\n", false},
		{"second document", "a: 1\n---\nb: 2\n", true},
		{"second empty document", "a: 1\n---\n", true},
		{"second unparseable document", "a: 1\n---\n: : :\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSingleYAMLDocument([]byte(tt.doc))
			if tt.wantErr && err == nil {
				t.Errorf("checkSingleYAMLDocument(%q) = nil error, want error", tt.doc)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("checkSingleYAMLDocument(%q) = %v, want nil", tt.doc, err)
			}
		})
	}
}

// TestDecodeDefinitionRejectsSecondDocument is the registration-spoof
// probe: appending a second YAML document leaves the first document (and
// therefore Locked/DefinitionDigest) reporting an intact registration
// while the file on disk carries content the schema never saw.
func TestDecodeDefinitionRejectsSecondDocument(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"second document with unknown fields", validDefinitionYAML() + "---\ntotally: unknown\n"},
		{"second empty document", validDefinitionYAML() + "---\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeDefinition([]byte(tt.doc)); err == nil {
				t.Errorf("DecodeDefinition(%s) = nil error, want error", tt.name)
			}
		})
	}
}

func TestDecodeRatificationRejectsSecondDocument(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"second document with unknown fields", validRatificationYAML() + "---\ntotally: unknown\n"},
		{"second empty document", validRatificationYAML() + "---\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeRatification([]byte(tt.doc)); err == nil {
				t.Errorf("DecodeRatification(%s) = nil error, want error", tt.name)
			}
		})
	}
}
