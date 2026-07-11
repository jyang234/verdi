package artifact

import "testing"

const decisionConflictHex64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

var decisionConflictLivingYAML = `
schema: verdi.decisionconflict/v1
covers: 7f3c2a1
findings:
  - { id: edge-dc-1-supersedes-adr-exactly-once, kind: computed, text: "dc-1 supersedes adr/exactly-once: unresolved (adr status is accepted, want superseded)" }
  - { id: judged-1, kind: judged, text: "dc-2 may conflict with adr/retry-policy", disposition: exempt, note: "reviewed, exemption stands", target_ref: adr/retry-policy, routed_owners: [platform-team] }
sweep_provenance: { adr_corpus_digest: "sha256:` + decisionConflictHex64 + `", decisions_scanned: [dc-1, dc-2] }
digest: sha256:` + decisionConflictHex64 + `
`

var decisionConflictFrozenYAML = `
schema: verdi.decisionconflict/v1
covers: 3e91ab2
findings:
  - { id: edge-dc-1-exempts-adr-x, kind: computed, text: "dc-1 exempts adr/x: resolved", disposition: exempt, note: "reason on the link" }
digest: sha256:` + decisionConflictHex64 + `
integrity: sha256:` + decisionConflictHex64 + `
judge_integrity: { stdin_b64: cHJvbXB0, raw_result: "{\"findings\":[]}" }
frozen: { at: 2026-06-01, commit: 3e91ab2 }
provenance: { generator: verdi-align, version: v1, inputs: [spec/stale-decline@3e91ab2], digest: sha256:` + decisionConflictHex64 + ` }
`

func TestDecodeDecisionConflict_Happy(t *testing.T) {
	cases := map[string]string{
		"living": decisionConflictLivingYAML,
		"frozen": decisionConflictFrozenYAML,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDecisionConflict([]byte(y)); err != nil {
				t.Fatalf("DecodeDecisionConflict: %v", err)
			}
		})
	}
}

func TestDecodeDecisionConflict_Negative(t *testing.T) {
	cases := map[string]string{
		"wrong schema":                      "schema: bogus\ncovers: 7f3c2a1\nfindings: []\n",
		"bad covers":                        "schema: verdi.decisionconflict/v1\ncovers: not-a-sha\nfindings: []\n",
		"unknown finding kind":              "schema: verdi.decisionconflict/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: bogus, text: t }\n",
		"unknown disposition value":         "schema: verdi.decisionconflict/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: computed, text: t, disposition: fixed, note: n }\n",
		"disposition without note":          "schema: verdi.decisionconflict/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: judged, text: t, disposition: no-conflict }\n",
		"duplicate finding id":              "schema: verdi.decisionconflict/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: computed, text: t }\n  - { id: f-1, kind: judged, text: t2 }\n",
		"unknown field":                     "schema: verdi.decisionconflict/v1\ncovers: 7f3c2a1\nfindings: []\nbogus: true\n",
		"judge_integrity without integrity": "schema: verdi.decisionconflict/v1\ncovers: 7f3c2a1\nfindings: []\njudge_integrity: { stdin_b64: cA==, raw_result: \"{}\" }\n",
		"bad adr corpus digest":             "schema: verdi.decisionconflict/v1\ncovers: 7f3c2a1\nfindings: []\nsweep_provenance: { adr_corpus_digest: bogus, decisions_scanned: [] }\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDecisionConflict([]byte(y)); err == nil {
				t.Fatalf("DecodeDecisionConflict(%s): want error, got nil", name)
			}
		})
	}
}

// TestDecodeDecisionConflict_AllFourDispositions proves every one of the
// four disposition values (03 §Decision-conflict gate) decodes legally.
func TestDecodeDecisionConflict_AllFourDispositions(t *testing.T) {
	for _, d := range []ConflictDisposition{ConflictSuperseded, ConflictExempt, ConflictRejected, ConflictNoConflict} {
		t.Run(string(d), func(t *testing.T) {
			y := "schema: verdi.decisionconflict/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: judged, text: t, disposition: " + string(d) + ", note: n }\n"
			fm, err := DecodeDecisionConflict([]byte(y))
			if err != nil {
				t.Fatalf("DecodeDecisionConflict: %v", err)
			}
			if !fm.Findings[0].Dispositioned() || fm.Findings[0].Disposition != d {
				t.Fatalf("Findings[0] = %+v, want disposition %q", fm.Findings[0], d)
			}
		})
	}
}

// TestDecodeDecisionConflict_Undispositioned proves an empty disposition
// legally decodes — a living report's normal pre-review state, matching
// deviation.go's own Finding rule.
func TestDecodeDecisionConflict_Undispositioned(t *testing.T) {
	y := "schema: verdi.decisionconflict/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: computed, text: t }\n"
	fm, err := DecodeDecisionConflict([]byte(y))
	if err != nil {
		t.Fatalf("DecodeDecisionConflict: %v", err)
	}
	if len(fm.Findings) != 1 || fm.Findings[0].Dispositioned() {
		t.Fatalf("Findings = %+v, want one undispositioned finding", fm.Findings)
	}
}
