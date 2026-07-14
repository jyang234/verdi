package artifact

import "testing"

const diagramSweepHex64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

var diagramSweepLivingYAML = `
schema: verdi.diagramsweep/v1
covers: 7f3c2a1
findings:
  - { id: judged-1, kind: judged, text: "new sync edge collides with the outbox mandate", disposition: exempt, note: "reviewed, exemption stands", target_ref: adr/outbox-mandate, routed_owners: [platform-team] }
sweep_provenance: { adr_corpus_digest: "sha256:` + diagramSweepHex64 + `", decisions_scanned: [spec/proposal-artifact#dc-1] }
integrity: sha256:` + diagramSweepHex64 + `
judge_integrity: { stdin_b64: cHJvbXB0, raw_result: "{\"findings\":[]}" }
provenance: { generator: verdi-align, version: v1, inputs: [diagram/loansvc-future@7f3c2a1], digest: sha256:` + diagramSweepHex64 + ` }
`

var diagramSweepAbsenceYAML = `
schema: verdi.diagramsweep/v1
covers: 3e91ab2
findings:
  - { id: judged-diagram-sweep-coverage-absent, kind: judged, text: "judged diagram-sweep coverage absent: no align.judge_cmd configured" }
provenance: { generator: verdi-align, version: v1, inputs: [diagram/loansvc-future@3e91ab2], digest: sha256:` + diagramSweepHex64 + ` }
`

func TestDecodeDiagramSweep_Happy(t *testing.T) {
	cases := map[string]string{
		"living":  diagramSweepLivingYAML,
		"absence": diagramSweepAbsenceYAML,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDiagramSweep([]byte(y)); err != nil {
				t.Fatalf("DecodeDiagramSweep: %v", err)
			}
		})
	}
}

func TestDecodeDiagramSweep_Negative(t *testing.T) {
	cases := map[string]string{
		"wrong schema":                      "schema: bogus\ncovers: 7f3c2a1\nfindings: []\n",
		"bad covers":                        "schema: verdi.diagramsweep/v1\ncovers: not-a-sha\nfindings: []\n",
		"unknown finding kind":              "schema: verdi.diagramsweep/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: bogus, text: t }\n",
		"unknown disposition value":         "schema: verdi.diagramsweep/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: judged, text: t, disposition: fixed, note: n }\n",
		"disposition without note":          "schema: verdi.diagramsweep/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: judged, text: t, disposition: no-conflict }\n",
		"duplicate finding id":              "schema: verdi.diagramsweep/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: judged, text: t }\n  - { id: f-1, kind: judged, text: t2 }\n",
		"unknown field":                     "schema: verdi.diagramsweep/v1\ncovers: 7f3c2a1\nfindings: []\nbogus: true\n",
		"judge_integrity without integrity": "schema: verdi.diagramsweep/v1\ncovers: 7f3c2a1\nfindings: []\njudge_integrity: { stdin_b64: cA==, raw_result: \"{}\" }\n",
		"bad adr corpus digest":             "schema: verdi.diagramsweep/v1\ncovers: 7f3c2a1\nfindings: []\nsweep_provenance: { adr_corpus_digest: bogus, decisions_scanned: [] }\n",
		"bad integrity shape":               "schema: verdi.diagramsweep/v1\ncovers: 7f3c2a1\nfindings: []\nintegrity: bogus\n",
		"no frozen field accepted (not a violation, sanity only)": "schema: verdi.diagramsweep/v1\ncovers: 7f3c2a1\nfindings: []\nfrozen: { at: 2026-01-01, commit: 7f3c2a1 }\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeDiagramSweep([]byte(y))
			if name == "no frozen field accepted (not a violation, sanity only)" {
				// This report's schema declares no frozen field at all
				// (spec/judged-sweep dc-3's exact field list) — strict
				// decode must REJECT an unknown frozen: key exactly like
				// any other unrecognized field, proving the type really
				// carries no Frozen field rather than silently ignoring
				// one.
				if err == nil {
					t.Fatalf("DecodeDiagramSweep(%s): want error (unknown field frozen), got nil", name)
				}
				return
			}
			if err == nil {
				t.Fatalf("DecodeDiagramSweep(%s): want error, got nil", name)
			}
		})
	}
}

// TestDecodeDiagramSweep_AllFourDispositions proves every one of the
// existing four ConflictDisposition values (reused unchanged,
// spec/judged-sweep dc-2) decodes legally on a sweep report.
func TestDecodeDiagramSweep_AllFourDispositions(t *testing.T) {
	for _, d := range []ConflictDisposition{ConflictSuperseded, ConflictExempt, ConflictRejected, ConflictNoConflict} {
		t.Run(string(d), func(t *testing.T) {
			y := "schema: verdi.diagramsweep/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: judged, text: t, disposition: " + string(d) + ", note: n }\n"
			fm, err := DecodeDiagramSweep([]byte(y))
			if err != nil {
				t.Fatalf("DecodeDiagramSweep: %v", err)
			}
			if !fm.Findings[0].Dispositioned() || fm.Findings[0].Disposition != d {
				t.Fatalf("Findings[0] = %+v, want disposition %q", fm.Findings[0], d)
			}
		})
	}
}

// TestDecodeDiagramSweep_Undispositioned proves an empty disposition
// legally decodes — a fresh sweep's normal state before a human looks at
// it, matching DecisionConflictFrontmatter's own rule.
func TestDecodeDiagramSweep_Undispositioned(t *testing.T) {
	y := "schema: verdi.diagramsweep/v1\ncovers: 7f3c2a1\nfindings:\n  - { id: f-1, kind: judged, text: t }\n"
	fm, err := DecodeDiagramSweep([]byte(y))
	if err != nil {
		t.Fatalf("DecodeDiagramSweep: %v", err)
	}
	if len(fm.Findings) != 1 || fm.Findings[0].Dispositioned() {
		t.Fatalf("Findings = %+v, want one undispositioned finding", fm.Findings)
	}
}
