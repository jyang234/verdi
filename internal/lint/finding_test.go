package lint

import "testing"

func TestFinding_String(t *testing.T) {
	f := Finding{Rule: "VL-001", Path: ".verdi/adr/foo.md", Message: "something broke"}
	want := "VL-001 .verdi/adr/foo.md: something broke"
	if got := f.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
