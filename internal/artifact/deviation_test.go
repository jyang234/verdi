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
		"wrong schema":                    "schema: bogus\ncovers: 7f3c2a1\nfindings: []\n",
		"bad covers":                      "schema: verdi.deviation/v1\ncovers: not-a-sha\nfindings: []\n",
		"unknown finding kind":            "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: bogus, text: t, disposition: fixed }\n",
		"accepted-deviation without note": "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: judged, text: t, disposition: accepted-deviation }\n",
		"duplicate finding id":            "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: computed, text: t, disposition: fixed }\n  - { id: f-1, kind: judged, text: t2, disposition: fixed }\n",
		"unknown field":                   "schema: verdi.deviation/v1\ncovers: 7f3c2a1\nfindings: []\nbogus: true\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDeviation([]byte(y)); err == nil {
				t.Fatalf("DecodeDeviation(%s): want error, got nil", name)
			}
		})
	}
}
