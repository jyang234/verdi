package policyartifact

import (
	"strings"
	"testing"
)

func TestClassifyPolicyPath(t *testing.T) {
	tests := []struct {
		rel      string
		wantKind string
		wantName string
		wantErr  string
	}{
		{"constitution.md", KindConstitution, "constitution", ""},
		{"policies/go-toolchain.md", KindPolicy, "go-toolchain", ""},
		{"overlays/frontend-go.md", KindOverlay, "frontend-go", ""},
		{"exemptions/legacy-service.md", KindExemption, "legacy-service", ""},
		{"profiles/solo-default.md", KindProfileStorage, "solo-default", ""},
		{"profiles/solo.v2.md", KindProfileStorage, "solo.v2", ""},
		// Unknown entries fail closed: the directory is a versioned
		// schema, not a convention (store-layout Principle 3).
		{"notes.txt", "", "", "unrecognized"},
		{"policies/nested/dir.md", "", "", "unrecognized"},
		{"policies/Bad_Name.md", "", "", "kebab"},
		{"profiles/Bad Name.md", "", "", "name"},
		{"other/constitution.md", "", "", "unrecognized"},
		{"policies/x.txt", "", "", "unrecognized"},
	}
	for _, tt := range tests {
		t.Run(tt.rel, func(t *testing.T) {
			kind, name, err := ClassifyPolicyPath(tt.rel)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ClassifyPolicyPath(%q) err = %v, want containing %q", tt.rel, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ClassifyPolicyPath(%q): %v", tt.rel, err)
			}
			if kind != tt.wantKind || name != tt.wantName {
				t.Fatalf("ClassifyPolicyPath(%q) = (%q, %q), want (%q, %q)", tt.rel, kind, name, tt.wantKind, tt.wantName)
			}
		})
	}
}
