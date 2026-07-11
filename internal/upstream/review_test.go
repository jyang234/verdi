package upstream

import "testing"

func TestDecodeReview_AllThreeVerdicts(t *testing.T) {
	cases := []struct {
		file    string
		verdict ReviewVerdict
	}{
		{"review-structurally-clear.json", ReviewStructurallyClear},
		{"review-block.json", ReviewBlock},
		{"review-no-structural-signal.json", ReviewNoStructuralSignal},
	}
	for _, tc := range cases {
		t.Run(string(tc.verdict), func(t *testing.T) {
			r, err := DecodeReview(readCanned(t, tc.file))
			if err != nil {
				t.Fatalf("DecodeReview(%s): %v", tc.file, err)
			}
			if r.Verdict != tc.verdict {
				t.Errorf("Verdict = %q, want %q", r.Verdict, tc.verdict)
			}
			if r.Service != "svcfix" {
				t.Errorf("Service = %q, want %q", r.Service, "svcfix")
			}
			if r.Blocking() != (tc.verdict == ReviewBlock) {
				t.Errorf("Blocking() = %v, want %v", r.Blocking(), tc.verdict == ReviewBlock)
			}
		})
	}
}

func TestDecodeReview_Block_HasViolation(t *testing.T) {
	r, err := DecodeReview(readCanned(t, "review-block.json"))
	if err != nil {
		t.Fatalf("DecodeReview: %v", err)
	}
	if len(r.NewViolations) != 1 {
		t.Fatalf("NewViolations = %d entries, want 1", len(r.NewViolations))
	}
	if r.NewViolations[0].Rule != "layering" {
		t.Errorf("NewViolations[0].Rule = %q, want %q", r.NewViolations[0].Rule, "layering")
	}
	if len(r.ReachableFrom) == 0 {
		t.Error("ReachableFrom is empty, want at least one entry for a BLOCK verdict")
	}
}

func TestDecodeReview_Clear_HasContractChange(t *testing.T) {
	r, err := DecodeReview(readCanned(t, "review-structurally-clear.json"))
	if err != nil {
		t.Fatalf("DecodeReview: %v", err)
	}
	if len(r.ContractChanges) != 1 {
		t.Fatalf("ContractChanges = %d entries, want 1", len(r.ContractChanges))
	}
	if r.ContractChanges[0].Name != "GET /healthz" {
		t.Errorf("ContractChanges[0].Name = %q, want %q", r.ContractChanges[0].Name, "GET /healthz")
	}
}

func TestDecodeReview_UnknownField(t *testing.T) {
	if _, err := DecodeReview(readCanned(t, "review-unknown-field.json")); err == nil {
		t.Fatal("DecodeReview(unknown-field twin): want error, got nil")
	}
}

func TestDecodeReview_Negative(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"empty", ``},
		{"not json", `not json`},
		{"empty service", `{"service":"","verdict":"BLOCK"}`},
		{"unknown verdict", `{"service":"svcfix","verdict":"MAYBE"}`},
		{"trailing data", `{"service":"svcfix","verdict":"BLOCK"}{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeReview([]byte(tc.data)); err == nil {
				t.Fatalf("DecodeReview(%s): want error, got nil", tc.name)
			}
		})
	}
}
