package artifact

import "testing"

func TestDecodeGuideClaims_Happy(t *testing.T) {
	cases := map[string]string{
		"exists row, no caveat, no cite": "schema: verdi.guideclaims/v1\n" +
			"rows:\n" +
			"  - id: 12-mcp-tools\n" +
			"    section: \"12\"\n" +
			"    capability: \"MCP: nine tools\"\n" +
			"    status: EXISTS\n" +
			"    witnesses:\n" +
			"      - name: TestMCPToolInventory\n",
		"partial row with caveat and cite": "schema: verdi.guideclaims/v1\n" +
			"rows:\n" +
			"  - id: 9.5-model-check\n" +
			"    section: \"9.5\"\n" +
			"    capability: \"model check\"\n" +
			"    status: PARTIAL\n" +
			"    caveat: \"the live-store migration guard awaits stage 3\"\n" +
			"    cite: \"docs/design/plans/2026-07-20-extensibility-phase2-design.md#9. Explicitly out of scope\"\n" +
			"    witnesses:\n" +
			"      - name: TestModelCheck_FrontierViolation_Exit1_PinnedText\n",
		"invented row with cite, no witnesses": "schema: verdi.guideclaims/v1\n" +
			"rows:\n" +
			"  - id: 15-preset-directories\n" +
			"    section: \"15\"\n" +
			"    capability: \"Preset directories\"\n" +
			"    status: INVENTED\n" +
			"    cite: \"docs/design/plans/2026-07-17-extensibility-chronicle.md#PHASE 1 ARCHIVED\"\n",
		"row with multiple witnesses (one witness SET)": "schema: verdi.guideclaims/v1\n" +
			"rows:\n" +
			"  - id: 13-kernel-rules-frontier\n" +
			"    section: \"13\"\n" +
			"    capability: \"Kernel rules; frontier errors\"\n" +
			"    status: EXISTS\n" +
			"    witnesses:\n" +
			"      - name: TestDecodeModel_KernelViolations\n" +
			"      - name: TestDecodeModel_Frontier\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			m, err := DecodeGuideClaims([]byte(y))
			if err != nil {
				t.Fatalf("DecodeGuideClaims: %v", err)
			}
			if len(m.Rows) != 1 {
				t.Fatalf("Rows = %d, want 1", len(m.Rows))
			}
		})
	}
}

func TestDecodeGuideClaims_Negative(t *testing.T) {
	const preamble = "schema: verdi.guideclaims/v1\nrows:\n"
	cases := map[string]string{
		"bad schema literal": "schema: verdi.guideclaims/v0\n" +
			"rows:\n" +
			"  - id: x\n    section: \"1\"\n    capability: c\n    status: EXISTS\n",
		"unknown top-level field": preamble +
			"  - id: x\n    section: \"1\"\n    capability: c\n    status: EXISTS\n" +
			"extra_field: nope\n",
		"unknown row field": preamble +
			"  - id: x\n    section: \"1\"\n    capability: c\n    status: EXISTS\n    notes: not a real field\n",
		"unknown status enum value": preamble +
			"  - id: x\n    section: \"1\"\n    capability: c\n    status: SORTA\n",
		"missing id": preamble +
			"  - section: \"1\"\n    capability: c\n    status: EXISTS\n",
		"duplicate id": preamble +
			"  - id: dup\n    section: \"1\"\n    capability: c\n    status: EXISTS\n" +
			"  - id: dup\n    section: \"2\"\n    capability: d\n    status: EXISTS\n",
		"missing section": preamble +
			"  - id: x\n    capability: c\n    status: EXISTS\n",
		"missing capability": preamble +
			"  - id: x\n    section: \"1\"\n    status: EXISTS\n",
		"PARTIAL row without caveat reds (ac-3)": preamble +
			"  - id: x\n    section: \"1\"\n    capability: c\n    status: PARTIAL\n    cite: \"docs/x.md#Y\"\n",
		"non-EXISTS (PARTIAL) row without cite reds (ac-3)": preamble +
			"  - id: x\n    section: \"1\"\n    capability: c\n    status: PARTIAL\n    caveat: \"narrower than it sounds\"\n",
		"non-EXISTS (INVENTED) row without cite reds (ac-3)": preamble +
			"  - id: x\n    section: \"1\"\n    capability: c\n    status: INVENTED\n",
		"witness with empty name": preamble +
			"  - id: x\n    section: \"1\"\n    capability: c\n    status: EXISTS\n    witnesses:\n      - name: \"\"\n",
		"bundled multi-capability row shape (sub_claims list) rejected at decode (ac-1)": preamble +
			"  - id: x\n" +
			"    section: \"7.2\"\n" +
			"    sub_claims:\n" +
			"      - capability: \"verdi sync\"\n" +
			"        status: EXISTS\n" +
			"      - capability: \"fold\"\n" +
			"        status: EXISTS\n",
		"bundled row shape (capability as a list, not a scalar) rejected at decode (ac-1)": preamble +
			"  - id: x\n" +
			"    section: \"7.2\"\n" +
			"    capability:\n      - \"verdi sync\"\n      - \"fold\"\n" +
			"    status: EXISTS\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeGuideClaims([]byte(y)); err == nil {
				t.Fatalf("DecodeGuideClaims(%s): want error, got nil", name)
			}
		})
	}
}
