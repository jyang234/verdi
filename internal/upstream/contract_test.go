package upstream

import "testing"

func TestDecodeBoundaryContract_Happy(t *testing.T) {
	c, err := DecodeBoundaryContract(readCanned(t, "boundary-contract-base.json"))
	if err != nil {
		t.Fatalf("DecodeBoundaryContract: %v", err)
	}
	if c.Service != "svcfix" {
		t.Errorf("Service = %q, want %q", c.Service, "svcfix")
	}
	if len(c.Entrypoints.HTTP) != 2 {
		t.Fatalf("Entrypoints.HTTP = %d entries, want 2", len(c.Entrypoints.HTTP))
	}
	if len(c.BlindSpots) != 1 {
		t.Errorf("BlindSpots = %d entries, want 1 (real capture includes one)", len(c.BlindSpots))
	}
}

func TestDecodeBoundaryContract_Branch(t *testing.T) {
	c, err := DecodeBoundaryContract(readCanned(t, "boundary-contract-branch.json"))
	if err != nil {
		t.Fatalf("DecodeBoundaryContract: %v", err)
	}
	if len(c.Entrypoints.HTTP) != 3 {
		t.Fatalf("Entrypoints.HTTP = %d entries, want 3 (base + healthz)", len(c.Entrypoints.HTTP))
	}
}

func TestDecodeBoundaryContract_UnknownField(t *testing.T) {
	if _, err := DecodeBoundaryContract(readCanned(t, "boundary-contract-unknown-field.json")); err == nil {
		t.Fatal("DecodeBoundaryContract(unknown-field twin): want error, got nil")
	}
}

func TestDecodeBoundaryContract_Negative(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"empty", ``},
		{"not json", `not json`},
		{"wrong schema", `{"service":"svcfix","schema_version":"flowmap.boundary/v2","entrypoints":{}}`},
		{"empty service", `{"service":"","schema_version":"flowmap.boundary/v1","entrypoints":{}}`},
		{"trailing data", `{"service":"svcfix","schema_version":"flowmap.boundary/v1","entrypoints":{}}{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeBoundaryContract([]byte(tc.data)); err == nil {
				t.Fatalf("DecodeBoundaryContract(%s): want error, got nil", tc.name)
			}
		})
	}
}
