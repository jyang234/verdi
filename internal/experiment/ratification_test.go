package experiment

import (
	"strings"
	"testing"
)

// validActor is a canonical governanceprincipal.PrincipalID literal
// (governanceprincipal's own TestPrincipalIDValidate fixture): "github" ⇒
// trust source, decoding to subject "user-123".
const validActor = "principal/github/dXNlci0xMjM"

func validRatificationYAML() string {
	return "schema: verdi.experiment-ratification/v1\n" +
		"result_digest: " + digestOf("a") + "\n" +
		"actor: " + validActor + "\n" +
		"disposition: select-recommended\n"
}

func mutateRatification(t *testing.T, old, replacement string) string {
	t.Helper()
	doc := validRatificationYAML()
	if !strings.Contains(doc, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(doc, old, replacement, 1)
}

func TestDecodeRatificationHappyPath(t *testing.T) {
	r, err := DecodeRatification([]byte(validRatificationYAML()))
	if err != nil {
		t.Fatalf("DecodeRatification() unexpected error: %v", err)
	}
	if r.Disposition != DispositionSelectRecommended {
		t.Errorf("r.Disposition = %q, want %q", r.Disposition, DispositionSelectRecommended)
	}
}

func TestDecodeRatificationSelectOtherHappyPath(t *testing.T) {
	doc := "schema: verdi.experiment-ratification/v1\n" +
		"result_digest: " + digestOf("a") + "\n" +
		"actor: " + validActor + "\n" +
		"disposition: select-other\n" +
		"candidate: baseline\n" +
		"reason: lower operational risk than the recommended candidate\n"
	r, err := DecodeRatification([]byte(doc))
	if err != nil {
		t.Fatalf("DecodeRatification() unexpected error: %v", err)
	}
	if r.Candidate != "baseline" {
		t.Errorf("r.Candidate = %q, want baseline", r.Candidate)
	}
}

func TestDecodeRatificationRejects(t *testing.T) {
	selectOtherDoc := "schema: verdi.experiment-ratification/v1\n" +
		"result_digest: " + digestOf("a") + "\n" +
		"actor: " + validActor + "\n" +
		"disposition: select-other\n"

	tests := []struct {
		name string
		doc  string
	}{
		{"unknown schema", mutateRatification(t, "schema: verdi.experiment-ratification/v1", "schema: verdi.experiment-ratification/v2")},
		{"unknown field", validRatificationYAML() + "unknown_field: true\n"},
		// A bare trailing scalar the parser cannot place; a second "---"
		// document is covered by strictdecode_test.go's trailing-document
		// probes.
		{"trailing data", validRatificationYAML() + "trailing-garbage-not-a-key\n"},
		{"yaml anchor", mutateRatification(t, "actor: "+validActor, "actor: &a "+validActor)},
		{"yaml alias", validRatificationYAML() + "alias_ref: *nonexistent\n"},
		{"custom tag", mutateRatification(t, "disposition: select-recommended", "disposition: !custom select-recommended")},
		{"bad result digest", mutateRatification(t, "result_digest: "+digestOf("a"), "result_digest: not-a-digest")},
		{"unknown disposition", mutateRatification(t, "disposition: select-recommended", "disposition: select-everyone")},
		{"bare name actor", mutateRatification(t, "actor: "+validActor, "actor: alice")},
		{"unauthenticated marker actor", mutateRatification(t, "actor: "+validActor, "actor: unauthenticated")},
		{"empty actor", mutateRatification(t, "actor: "+validActor, "actor: \"\"")},
		{"malformed principal actor", mutateRatification(t, "actor: "+validActor, "actor: principal/GitHub/dXNlci0xMjM")},
		{"select-other missing candidate", selectOtherDoc + "reason: because\n"},
		{"select-other missing reason", selectOtherDoc + "candidate: baseline\n"},
		{"candidate present on non-select-other", validRatificationYAML() + "candidate: baseline\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeRatification([]byte(tt.doc)); err == nil {
				t.Errorf("DecodeRatification(%s) = nil error, want error", tt.name)
			}
		})
	}
}
