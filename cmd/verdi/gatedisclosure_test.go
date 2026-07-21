package main

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
)

// quarRec builds one excluded evidence record naming ac-1 with the given
// verdict/witness and (optional) sync-recorded quarantine reason.
func quarRec(verdict artifact.EvidenceVerdict, witness, quarReason string) artifact.Evidence {
	rec := artifact.Evidence{
		EvidenceFor: []string{"ac-1"},
		Kind:        artifact.EvidenceStatic,
		Verdict:     verdict,
		Witness:     witness,
		Provenance:  artifact.EvidenceProvenance{Source: artifact.SourceCI, Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
	}
	if quarReason != "" {
		rec.Quarantine = &artifact.EvidenceQuarantine{Reason: quarReason}
	}
	return rec
}

// TestQuarantineDisclosures_MetACFailAndLegibility pins
// judged-quarantine-disclosure-met-ac's exact rule: a MET AC discloses ONLY an
// excluded FAIL record (the violated->evidenced flip), never a redundant excluded
// PASS record (kept out for legibility); an UNMET AC discloses every excluded
// record naming it, a FAIL as adverse and only saying "does not read violated"
// when the AC does not itself already read violated.
func TestQuarantineDisclosures_MetACFailAndLegibility(t *testing.T) {
	cases := []struct {
		name       string
		status     evidence.Status
		rec        artifact.Evidence
		wantLines  int
		wantSubs   []string
		wantNotSub []string
	}{
		{
			name:      "met evidenced + FAIL record: the flip is disclosed",
			status:    evidence.StatusEvidenced,
			rec:       quarRec(artifact.VerdictFail, "failW", ""),
			wantLines: 1,
			wantSubs:  []string{"[gate:evidence-quarantine] ac-1", "recorded verdict fail for ac-1", "does not read violated (folded evidenced)", "failW"},
		},
		{
			name:       "met evidenced + PASS record: NO disclosure (legibility, no noise)",
			status:     evidence.StatusEvidenced,
			rec:        quarRec(artifact.VerdictPass, "passW", ""),
			wantLines:  0,
			wantNotSub: []string{"passW"},
		},
		{
			name:      "met waived + FAIL record: disclosed against the waiver too",
			status:    evidence.StatusWaived,
			rec:       quarRec(artifact.VerdictFail, "failW", ""),
			wantLines: 1,
			wantSubs:  []string{"does not read violated (folded waived)"},
		},
		{
			name:      "unmet no-signal + PASS record: would-have-evidenced",
			status:    evidence.StatusNoSignal,
			rec:       quarRec(artifact.VerdictPass, "passW", ""),
			wantLines: 1,
			wantSubs:  []string{"would have evidenced ac-1 was excluded", "passW"},
		},
		{
			name:      "unmet no-signal + FAIL record: flip disclosure",
			status:    evidence.StatusNoSignal,
			rec:       quarRec(artifact.VerdictFail, "failW", ""),
			wantLines: 1,
			wantSubs:  []string{"does not read violated (folded no-signal)"},
		},
		{
			name:       "unmet VIOLATED + FAIL record: no false flip clause",
			status:     evidence.StatusViolated,
			rec:        quarRec(artifact.VerdictFail, "failW", ""),
			wantLines:  1,
			wantSubs:   []string{"already reads violated on other evidence"},
			wantNotSub: []string{"does not read violated"},
		},
		{
			name:      "prefers the sync-recorded quarantine reason verbatim",
			status:    evidence.StatusEvidenced,
			rec:       quarRec(artifact.VerdictFail, "failW", "custom sync reason naming the deleted branch"),
			wantLines: 1,
			wantSubs:  []string{"custom sync reason naming the deleted branch"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := quarantineDisclosures([]foldedAC{{ID: "ac-1", Status: tc.status}}, []artifact.Evidence{tc.rec})
			if len(got) != tc.wantLines {
				t.Fatalf("quarantineDisclosures = %#v, want %d line(s)", got, tc.wantLines)
			}
			joined := strings.Join(got, "\n")
			for _, sub := range tc.wantSubs {
				if !strings.Contains(joined, sub) {
					t.Errorf("lines = %q, want a substring %q", joined, sub)
				}
			}
			for _, sub := range tc.wantNotSub {
				if strings.Contains(joined, sub) {
					t.Errorf("lines = %q, must NOT contain %q", joined, sub)
				}
			}
		})
	}
}
