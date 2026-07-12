package lint

import (
	"testing"

	"github.com/OWNER/verdi/internal/disclosure"
)

func TestFinding_String(t *testing.T) {
	f := Finding{Rule: "VL-001", Path: ".verdi/adr/foo.md", Message: "something broke"}
	want := "VL-001 .verdi/adr/foo.md: something broke"
	if got := f.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

// TestFinding_Disclosure pins the one Finding->Disclosure mapping both
// renderers share: String()'s disclosure branch and the disclosures-view
// enumeration (spec/disclosures-panel ac-1) must consume the same seam
// value, never two independently-authored mappings.
func TestFinding_Disclosure(t *testing.T) {
	tests := []struct {
		name string
		f    Finding
		want disclosure.Disclosure
	}{
		{
			name: "disclosure finding maps rule to source and path to scope",
			f:    Finding{Rule: "VL-017", Path: ".verdi/specs/active/x/spec.md", Message: "unproven: no mutable zone", Severity: SeverityDisclosure},
			want: disclosure.Disclosure{
				ID:       "lint:VL-017/.verdi/specs/active/x/spec.md",
				Source:   "lint:VL-017",
				Scope:    ".verdi/specs/active/x/spec.md",
				Text:     "unproven: no mutable zone",
				Severity: disclosure.SeverityDisclosedUnproven,
			},
		},
		{
			name: "empty path yields a scopeless disclosure",
			f:    Finding{Rule: "VL-017", Message: "checkout-wide", Severity: SeverityDisclosure},
			want: disclosure.Disclosure{
				ID:       "lint:VL-017",
				Source:   "lint:VL-017",
				Text:     "checkout-wide",
				Severity: disclosure.SeverityDisclosedUnproven,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.Disclosure(); got != tt.want {
				t.Fatalf("Disclosure() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestFinding_String_DisclosureRendersThroughSeam proves String()'s
// disclosure branch is Render(f.Disclosure()) — the same value the
// enumeration collects — so the CLI line and an enumerated item can never
// drift (the exact silent-drift failure conflict/disclosure-seam-rename-
// insufficient witnessed).
func TestFinding_String_DisclosureRendersThroughSeam(t *testing.T) {
	f := Finding{Rule: "VL-017", Path: "p", Message: "m", Severity: SeverityDisclosure}
	if got, want := f.String(), disclosure.Render(f.Disclosure()); got != want {
		t.Fatalf("String() = %q, want Render(Disclosure()) = %q", got, want)
	}
}
