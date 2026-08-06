package governanceprincipal

import (
	"strings"
	"testing"
)

func TestCanonicalPrincipalID(t *testing.T) {
	tests := []struct {
		name     string
		sourceID string
		subject  string
		want     PrincipalID
	}{
		{"ascii subject", "github", "user-123", "principal/github/dXNlci0xMjM"},
		{"subject with slash", "github", "org/team", "principal/github/b3JnL3RlYW0"},
		{"unicode subject", "corporate-idp", "山田太郎", "principal/corporate-idp/5bGx55Sw5aSq6YOO"},
		{"emoji subject", "github", "user-🔐", "principal/github/dXNlci3wn5SQ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanonicalPrincipalID(tt.sourceID, tt.subject)
			if err != nil {
				t.Fatalf("CanonicalPrincipalID(%q, %q): %v", tt.sourceID, tt.subject, err)
			}
			if got != tt.want {
				t.Errorf("CanonicalPrincipalID(%q, %q) = %q, want %q", tt.sourceID, tt.subject, got, tt.want)
			}
			if err := got.Validate(); err != nil {
				t.Errorf("derived principal ID %q does not validate: %v", got, err)
			}
		})
	}
}

func TestCanonicalPrincipalIDRejects(t *testing.T) {
	tests := []struct {
		name     string
		sourceID string
		subject  string
	}{
		{"empty source", "", "user-123"},
		{"uppercase source", "GitHub", "user-123"},
		{"source starting with digit", "9lives", "user-123"},
		{"source starting with dash", "-github", "user-123"},
		{"source with slash", "git/hub", "user-123"},
		{"empty subject", "github", ""},
		{"invalid utf-8 subject", "github", "user-\xff"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := CanonicalPrincipalID(tt.sourceID, tt.subject); err == nil {
				t.Errorf("CanonicalPrincipalID(%q, %q) = %q, want error", tt.sourceID, tt.subject, got)
			}
		})
	}
}

// TestCanonicalPrincipalIDCollisionResistance: distinct (source, subject)
// pairs — including pairs whose naive concatenations collide — derive
// distinct principal IDs.
func TestCanonicalPrincipalIDCollisionResistance(t *testing.T) {
	pairs := []struct{ source, subject string }{
		{"github", "user-123"},
		{"github", "user-12"},
		{"gitlab", "user-123"},
		{"git", "hub-user-123"},
		{"github", "user/123"},
		{"github", "user"},
		{"github-user", "123"},
		{"g", "ithub-user-123"},
		{"github", "USER-123"},
	}
	seen := make(map[PrincipalID]string, len(pairs))
	for _, p := range pairs {
		id, err := CanonicalPrincipalID(p.source, p.subject)
		if err != nil {
			t.Fatalf("CanonicalPrincipalID(%q, %q): %v", p.source, p.subject, err)
		}
		if prev, dup := seen[id]; dup {
			t.Errorf("collision: (%s, %s) and %s both derive %q", p.source, p.subject, prev, id)
		}
		seen[id] = p.source + "/" + p.subject
	}
}

func TestPrincipalIDValidate(t *testing.T) {
	valid := []PrincipalID{
		"principal/github/dXNlci0xMjM",
		"principal/corporate-idp/5bGx55Sw5aSq6YOO",
	}
	for _, id := range valid {
		if err := id.Validate(); err != nil {
			t.Errorf("Validate(%q): unexpected error %v", id, err)
		}
	}

	invalid := []struct {
		name string
		id   PrincipalID
	}{
		{"empty", ""},
		{"missing prefix", "github/dXNlci0xMjM"},
		{"wrong prefix", "principle/github/dXNlci0xMjM"},
		{"missing segments", "principal/github"},
		{"extra segment", "principal/github/dXNlci0xMjM/extra"},
		{"invalid source id", "principal/GitHub/dXNlci0xMjM"},
		{"empty encoded subject", "principal/github/"},
		{"base64 padding", "principal/github/dXNlcg=="},
		{"standard base64 alphabet", "principal/github/dXNl+g"},
		{"non-base64 bytes", "principal/github/not base64!"},
		{"encoded invalid utf-8", "principal/github/_w"},
		// "YR" decodes to the same byte as canonical "YQ" under permissive
		// decoding; only the exact CanonicalPrincipalID output may validate.
		{"non-canonical trailing bits", "principal/github/YR"},
		{"non-canonical trailing bits long", "principal/github/dXNlci0xMjN"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.id.Validate()
			if err == nil {
				t.Fatalf("Validate(%q): expected error, got nil", tt.id)
			}
			if !strings.Contains(err.Error(), "principal") {
				t.Errorf("Validate(%q) error %q does not mention principal", tt.id, err)
			}
		})
	}
}
