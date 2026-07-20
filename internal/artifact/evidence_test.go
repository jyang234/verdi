package artifact

import "testing"

func TestDecodeEvidence_Happy(t *testing.T) {
	cases := map[string]string{
		"static pass ci": `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"pass",
			"witness":"retryWorker -> charge.Post","provenance":{"source":"ci","pipeline":"913","commit":"7f3c2a1"},
			"digest":"sha256:` + hex64 + `"}`,
		"behavioral fail local": `{"schema":"verdi.evidence/v1","evidence_for":["ac-3"],"kind":"behavioral","verdict":"fail",
			"witness":"golden mismatch","provenance":{"source":"local","pipeline":"","commit":"9c41f2e"},
			"digest":"sha256:` + hex64 + `"}`,
		"runtime abstain": `{"schema":"verdi.evidence/v1","evidence_for":["ac-4"],"kind":"runtime","verdict":"abstain",
			"witness":"","provenance":{"source":"ci","pipeline":"914","commit":"7f3c2a1"},
			"digest":"sha256:` + hex64 + `"}`,
		"static pass ci with job and producer (I-25)": `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"pass",
			"witness":"retryWorker -> charge.Post","producer":"retryWorker",
			"provenance":{"source":"ci","pipeline":"913","job":"42","commit":"7f3c2a1"},
			"digest":"sha256:` + hex64 + `"}`,
		"quarantined record (spec/evidence-resilience ac-1)": `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"pass",
			"witness":"retryWorker -> charge.Post",
			"provenance":{"source":"ci","pipeline":"913","commit":"7f3c2a1"},
			"quarantine":{"reason":"provenance.commit 7f3c2a1 is not reachable from HEAD"},
			"digest":"sha256:` + hex64 + `"}`,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeEvidence([]byte(y)); err != nil {
				t.Fatalf("DecodeEvidence: %v", err)
			}
		})
	}
}

// TestDecodeEvidence_Quarantine_RoundTrips proves the ac-1 schema addition
// end to end: a quarantined record decodes with its reason accessible on
// the Go struct (never just silently tolerated as an unknown-but-ignored
// blob), and a record with no quarantine block at all decodes with a nil
// Quarantine — the ordinary, non-quarantined case, unaffected — so every
// pre-existing fixture without this field keeps decoding exactly as
// before (schema-additive, omitempty).
func TestDecodeEvidence_Quarantine_RoundTrips(t *testing.T) {
	quarantined := `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"pass",
		"witness":"w","provenance":{"source":"ci","pipeline":"913","commit":"deadbeef"},
		"quarantine":{"reason":"provenance.commit deadbeef is not reachable from HEAD"},
		"digest":"sha256:` + hex64 + `"}`
	rec, err := DecodeEvidence([]byte(quarantined))
	if err != nil {
		t.Fatalf("DecodeEvidence(quarantined): %v", err)
	}
	if rec.Quarantine == nil {
		t.Fatal("rec.Quarantine = nil, want the decoded quarantine block")
	}
	if rec.Quarantine.Reason != "provenance.commit deadbeef is not reachable from HEAD" {
		t.Errorf("rec.Quarantine.Reason = %q, want the decoded reason", rec.Quarantine.Reason)
	}

	plain := `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"pass",
		"witness":"w","provenance":{"source":"ci","pipeline":"913","commit":"7f3c2a1"},
		"digest":"sha256:` + hex64 + `"}`
	rec2, err := DecodeEvidence([]byte(plain))
	if err != nil {
		t.Fatalf("DecodeEvidence(plain): %v", err)
	}
	if rec2.Quarantine != nil {
		t.Errorf("rec2.Quarantine = %+v, want nil (no quarantine block present)", rec2.Quarantine)
	}
}

// TestDecodeEvidence_Quarantine_Negative proves the quarantine block is
// itself strict-decoded (an unknown field inside it fails closed, matching
// every other block in this schema).
func TestDecodeEvidence_Quarantine_Negative(t *testing.T) {
	bad := `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"pass",
		"witness":"w","provenance":{"source":"ci","pipeline":"913","commit":"7f3c2a1"},
		"quarantine":{"reason":"x","bogus":true},
		"digest":"sha256:` + hex64 + `"}`
	if _, err := DecodeEvidence([]byte(bad)); err == nil {
		t.Fatal("DecodeEvidence(quarantine with unknown field): want error, got nil")
	}
}

func TestDecodeEvidence_Negative(t *testing.T) {
	cases := map[string]string{
		"wrong schema":       `{"schema":"bogus","evidence_for":["ac-2"],"kind":"static","verdict":"pass","witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"7f3c2a1"},"digest":"sha256:` + hex64 + `"}`,
		"empty evidence_for": `{"schema":"verdi.evidence/v1","evidence_for":[],"kind":"static","verdict":"pass","witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"7f3c2a1"},"digest":"sha256:` + hex64 + `"}`,
		"bad ac id":          `{"schema":"verdi.evidence/v1","evidence_for":["not-an-ac"],"kind":"static","verdict":"pass","witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"7f3c2a1"},"digest":"sha256:` + hex64 + `"}`,
		"unknown kind":       `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"bogus","verdict":"pass","witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"7f3c2a1"},"digest":"sha256:` + hex64 + `"}`,
		"unknown verdict":    `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"bogus","witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"7f3c2a1"},"digest":"sha256:` + hex64 + `"}`,
		"unknown source":     `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"pass","witness":"w","provenance":{"source":"bogus","pipeline":"1","commit":"7f3c2a1"},"digest":"sha256:` + hex64 + `"}`,
		"bad commit":         `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"pass","witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"xyz"},"digest":"sha256:` + hex64 + `"}`,
		"bad digest":         `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"pass","witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"7f3c2a1"},"digest":"not-sha256"}`,
		"unknown field":      `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"pass","witness":"w","provenance":{"source":"ci","pipeline":"1","commit":"7f3c2a1"},"digest":"sha256:` + hex64 + `","bogus":true}`,
		"unknown field in provenance (I-25 job typo)": `{"schema":"verdi.evidence/v1","evidence_for":["ac-2"],"kind":"static","verdict":"pass","witness":"w","provenance":{"source":"ci","pipeline":"1","jobb":"1","commit":"7f3c2a1"},"digest":"sha256:` + hex64 + `"}`,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeEvidence([]byte(y)); err == nil {
				t.Fatalf("DecodeEvidence(%s): want error, got nil", name)
			}
		})
	}
}
