package artifact

import "testing"

const deviationHex64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

var deviationLivingYAML = `
schema: verdi.deviation/v1
covers: 7f3c2a1
findings:
  - { id: f-1, kind: computed, text: "boundary loansvc->notification-svc holds", disposition: fixed }
  - { id: f-2, kind: judged, text: "retry semantics match spec intent", disposition: accepted-deviation, note: "backoff differs, documented in PR" }
digest: sha256:` + deviationHex64 + `
`

var deviationFrozenYAML = `
schema: verdi.deviation/v1
covers: 3e91ab2
findings:
  - { id: f-1, kind: computed, text: "boundary holds", disposition: fixed }
digest: sha256:` + deviationHex64 + `
integrity: sha256:` + deviationHex64 + `
judge_integrity: { stdin_b64: cHJvbXB0, raw_result: "{\"findings\":[]}" }
frozen: { at: 2026-06-01, commit: 3e91ab2 }
provenance: { generator: verdi-align, version: v0, inputs: [spec/stale-decline@3e91ab2], digest: sha256:` + deviationHex64 + ` }
`

func TestDecodeDeviation_Happy(t *testing.T) {
	cases := map[string]string{
		"living": deviationLivingYAML,
		"frozen": deviationFrozenYAML,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDeviation([]byte(y)); err != nil {
				t.Fatalf("DecodeDeviation: %v", err)
			}
		})
	}
}

func TestDecodeDeviation_Negative(t *testing.T) {
	cases := map[string]string{
		"wrong schema":                      "schema: bogus\ncovers: 7f3c2a1\nfindings: []\n",
		"bad covers":                        "schema: verdi.deviation/v1\ncovers: not-a-sha\nfindings: []\n",
		"unknown finding kind":              "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: bogus, text: t, disposition: fixed }\n",
		"accepted-deviation without note":   "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: judged, text: t, disposition: accepted-deviation }\n",
		"duplicate finding id":              "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: computed, text: t, disposition: fixed }\n  - { id: f-1, kind: judged, text: t2, disposition: fixed }\n",
		"unknown field":                     "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings: []\nbogus: true\n",
		"unknown disposition value":         "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: computed, text: t, disposition: bogus }\n",
		"integrity without judge_integrity": "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings: []\nintegrity: sha256:" + deviationHex64 + "\n",
		"judge_integrity without integrity": "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings: []\njudge_integrity: { stdin_b64: cA==, raw_result: \"{}\" }\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDeviation([]byte(y)); err == nil {
				t.Fatalf("DecodeDeviation(%s): want error, got nil", name)
			}
		})
	}
}

// TestDecodeDeviation_Undispositioned proves an empty disposition legally
// decodes (a living report's normal pre-review state for a new or changed
// finding, PLAN.md Phase 8) and that Finding.Dispositioned reports it
// correctly — distinct from an unknown/garbage disposition value, which
// still fails closed (see the negative table above).
func TestDecodeDeviation_Undispositioned(t *testing.T) {
	y := "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: computed, text: t }\n"
	fm, err := DecodeDeviation([]byte(y))
	if err != nil {
		t.Fatalf("DecodeDeviation: %v", err)
	}
	if len(fm.Findings) != 1 || fm.Findings[0].Dispositioned() {
		t.Fatalf("Findings = %+v, want one undispositioned finding", fm.Findings)
	}
}
