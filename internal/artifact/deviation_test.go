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

// deviationIntegrityOnlyYAML models an older or hand-authored frozen report
// that predates the judge_integrity self-verification record (PLAN.md
// Phase 8): integrity: alone is still legally decodable — the pairing is
// one-directional (judge_integrity requires integrity, not the reverse) —
// though internal/align.VerifyIntegrity reports it as unverifiable rather
// than silently skipping the check.
var deviationIntegrityOnlyYAML = `
schema: verdi.deviation/v1
covers: 3e91ab2
findings: []
digest: sha256:` + deviationHex64 + `
integrity: sha256:` + deviationHex64 + `
`

func TestDecodeDeviation_Happy(t *testing.T) {
	cases := map[string]string{
		"living":         deviationLivingYAML,
		"frozen":         deviationFrozenYAML,
		"integrity only": deviationIntegrityOnlyYAML,
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

// TestDecodeDeviation_OldFixturesUnaffectedByNewFields is spec/finding-
// identity ac-2's omitempty pin: every pre-existing fixture (none of which
// carries carried-from: or not-resurfaced:) keeps decoding exactly as
// before this story — the schema-additive fields must never become
// mandatory or change any existing fixture's decoded shape.
func TestDecodeDeviation_OldFixturesUnaffectedByNewFields(t *testing.T) {
	cases := map[string]string{
		"living": deviationLivingYAML,
		"frozen": deviationFrozenYAML,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			fm, err := DecodeDeviation([]byte(y))
			if err != nil {
				t.Fatalf("DecodeDeviation: %v", err)
			}
			if fm.NotResurfaced != nil {
				t.Fatalf("NotResurfaced = %+v, want nil (old fixture names none)", fm.NotResurfaced)
			}
			for _, f := range fm.Findings {
				if f.CarriedFrom != "" {
					t.Fatalf("finding %s CarriedFrom = %q, want empty (old fixture names none)", f.ID, f.CarriedFrom)
				}
			}
		})
	}
}

// TestFinding_CarriedFrom_Validate proves carried-from's two preconditions
// (spec/finding-identity ac-2): it must accompany a disposition (a
// candidate — undispositioned by construction — never carries provenance
// for a decision that has not been made) and, when present, must be a
// well-formed commit sha (the same shape Covers/Frozen.Commit already
// require).
func TestFinding_CarriedFrom_Validate(t *testing.T) {
	validSha := "7f3c2a10000000000000000000000000000000"
	tests := []struct {
		name    string
		f       Finding
		wantErr bool
	}{
		{
			name: "carried-from with a disposition and a valid sha",
			f:    Finding{ID: "judged-a", Kind: FindingJudged, Text: "t", Disposition: FindingAcceptedDeviation, Note: "n", CarriedFrom: validSha},
		},
		{
			name:    "carried-from without a disposition",
			f:       Finding{ID: "judged-a", Kind: FindingJudged, Text: "t", CarriedFrom: validSha},
			wantErr: true,
		},
		{
			name:    "carried-from not a valid sha shape",
			f:       Finding{ID: "judged-a", Kind: FindingJudged, Text: "t", Disposition: FindingFixed, CarriedFrom: "not-a-sha"},
			wantErr: true,
		},
		{
			name: "empty carried-from is legal (the common case)",
			f:    Finding{ID: "judged-a", Kind: FindingJudged, Text: "t", Disposition: FindingFixed},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.f.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("Validate(): want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate(): %v, want nil", err)
			}
		})
	}
}

// TestDecodeDeviation_NotResurfaced_Happy proves a not-resurfaced: section
// (spec/finding-identity ac-3) decodes: one previously-dispositioned judged
// finding, absent from findings:, persisted there.
func TestDecodeDeviation_NotResurfaced_Happy(t *testing.T) {
	y := `schema: verdi.deviation/v1
covers: 7f3c2a1
findings: []
not-resurfaced:
  - { id: judged-a, kind: judged, text: "old text", disposition: accepted-deviation, note: "n" }
digest: sha256:` + deviationHex64 + "\n"
	fm, err := DecodeDeviation([]byte(y))
	if err != nil {
		t.Fatalf("DecodeDeviation: %v", err)
	}
	if len(fm.NotResurfaced) != 1 || fm.NotResurfaced[0].ID != "judged-a" {
		t.Fatalf("NotResurfaced = %+v, want one judged-a entry", fm.NotResurfaced)
	}
}

// TestDecodeDeviation_NotResurfaced_Negative covers not-resurfaced:'s own
// fail-closed preconditions: every entry must already be dispositioned (an
// undispositioned finding has no prior ruling to persist), ids must be
// unique within the section, and an id must not be DISPOSITIONED in
// findings: while still present in not-resurfaced: (a confirmed finding
// must have had its backing record removed).
func TestDecodeDeviation_NotResurfaced_Negative(t *testing.T) {
	cases := map[string]string{
		"undispositioned entry": `schema: verdi.deviation/v1
covers: 7f3c2a1
findings: []
not-resurfaced:
  - { id: judged-a, kind: judged, text: "old text" }
digest: sha256:` + deviationHex64 + "\n",
		"duplicate id within not-resurfaced": `schema: verdi.deviation/v1
covers: 7f3c2a1
findings: []
not-resurfaced:
  - { id: judged-a, kind: judged, text: "old text", disposition: fixed }
  - { id: judged-a, kind: judged, text: "old text 2", disposition: fixed }
digest: sha256:` + deviationHex64 + "\n",
		"id dispositioned in findings AND still present in not-resurfaced": `schema: verdi.deviation/v1
covers: 7f3c2a1
findings:
  - { id: judged-a, kind: judged, text: "new text", disposition: fixed }
not-resurfaced:
  - { id: judged-a, kind: judged, text: "old text", disposition: fixed }
digest: sha256:` + deviationHex64 + "\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeDeviation([]byte(y)); err == nil {
				t.Fatalf("DecodeDeviation(%s): want error, got nil", name)
			}
		})
	}
}

// TestDecodeDeviation_NotResurfaced_LiveCandidateSharesID proves the
// EXPECTED, common shape a pending candidate produces: an UNDISPOSITIONED
// findings: entry (the fresh, reworded finding) sharing an id with a
// DISPOSITIONED not-resurfaced: entry (its old ruling, the pre-fill's
// backing record) is legal — this is exactly what align.ReconcileJudged
// produces for a not-yet-confirmed candidate (spec/finding-identity ac-1).
func TestDecodeDeviation_NotResurfaced_LiveCandidateSharesID(t *testing.T) {
	y := `schema: verdi.deviation/v1
covers: 7f3c2a1
findings:
  - { id: judged-a, kind: judged, text: "new reworded text" }
not-resurfaced:
  - { id: judged-a, kind: judged, text: "old text", disposition: accepted-deviation, note: "n" }
digest: sha256:` + deviationHex64 + "\n"
	fm, err := DecodeDeviation([]byte(y))
	if err != nil {
		t.Fatalf("DecodeDeviation: %v, want a live candidate + backing record to decode cleanly", err)
	}
	if len(fm.Findings) != 1 || fm.Findings[0].Dispositioned() {
		t.Fatalf("Findings = %+v, want one undispositioned candidate", fm.Findings)
	}
	if len(fm.NotResurfaced) != 1 || !fm.NotResurfaced[0].Dispositioned() {
		t.Fatalf("NotResurfaced = %+v, want one dispositioned backing record", fm.NotResurfaced)
	}
}

// TestDecodeDeviation_NotResurfaced_ComputedFindingSharesJudgedID proves the
// judged-only scope of the not-resurfaced backing relationship (spec/finding-
// identity judged-reaffirm-judged-kind-scope): a DISPOSITIONED COMPUTED finding
// sharing an id with a JUDGED not-resurfaced entry is a legitimate cross-
// namespace slug collision — computed boundary ids (boundary-<from>-<to>-<via>)
// and judged boundary slugs share the same shape — never the "unremoved judged
// backing record" the SAME-KIND rejection catches, so it decodes cleanly. The
// same-kind collision (a judged dispositioned finding + a judged not-resurfaced
// entry) stays rejected (TestDecodeDeviation_NotResurfaced_Negative).
func TestDecodeDeviation_NotResurfaced_ComputedFindingSharesJudgedID(t *testing.T) {
	y := `schema: verdi.deviation/v1
covers: 7f3c2a1
findings:
  - { id: boundary-x, kind: computed, text: "the declared boundary holds", disposition: fixed }
not-resurfaced:
  - { id: boundary-x, kind: judged, text: "an old judged ruling under the same slug", disposition: accepted-deviation, note: "n" }
digest: sha256:` + deviationHex64 + "\n"
	fm, err := DecodeDeviation([]byte(y))
	if err != nil {
		t.Fatalf("DecodeDeviation: %v, want a computed finding colliding with a judged not-resurfaced entry to decode cleanly (the backing relationship is judged-only)", err)
	}
	if len(fm.Findings) != 1 || fm.Findings[0].Kind != FindingComputed {
		t.Fatalf("Findings = %+v, want one computed finding", fm.Findings)
	}
	if len(fm.NotResurfaced) != 1 || fm.NotResurfaced[0].Kind != FindingJudged {
		t.Fatalf("NotResurfaced = %+v, want one judged backing record left intact", fm.NotResurfaced)
	}
}
