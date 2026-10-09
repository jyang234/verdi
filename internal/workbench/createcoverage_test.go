package workbench

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/boardlayout"
)

// Tests for the New story dialog's data (spec/new-story-dialog-v2 ac-2,
// dc-2; SI-369 (1), (3), (13), (14)): the coverage chip text the wall and
// the dialog share, and the dialog's criterion coverage.

// TestCoverageChipText: the one chip text is the wall's, word for word —
// "no stub" for a criterion no stub lists, the singular for one stub, and
// the plural for more (index-coverage ac-1 keeps these texts; SI-369 (1)).
func TestCoverageChipText(t *testing.T) {
	for _, tc := range []struct {
		stubs int
		want  string
	}{
		{0, "no stub"},
		{1, "covered by 1 stub"},
		{2, "covered by 2 stubs"},
		{12, "covered by 12 stubs"},
	} {
		if got := coverageChipText(tc.stubs); got != tc.want {
			t.Errorf("coverageChipText(%d) = %q, want %q", tc.stubs, got, tc.want)
		}
	}
}

// TestWriteScopingReceipts_ChipBytesUnchanged: the AC card's chip, now
// worded by coverageChipText, is byte-identical to the literals the
// receipts wrote before the helper existed — the class, the testid, the
// count and the text — for no stub, one stub, and many; an open-question
// card claimed once still wears nothing.
func TestWriteScopingReceipts_ChipBytesUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kind  boardlayout.ZoneKind
		stubs int
		want  string
	}{
		{"no stub", boardlayout.ZoneAC, 0, `<span class="coverage-chip coverage-chip--none" data-testid="coverage-ac-1" data-coverage="0">no stub</span>`},
		{"one stub", boardlayout.ZoneAC, 1, `<span class="coverage-chip coverage-chip--covered" data-testid="coverage-ac-1" data-coverage="1">covered by 1 stub</span>`},
		{"three stubs", boardlayout.ZoneAC, 3, `<span class="coverage-chip coverage-chip--covered" data-testid="coverage-ac-1" data-coverage="3">covered by 3 stubs</span>`},
		{"an open question claimed once wears nothing", boardlayout.ZoneOpenQuestion, 1, ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &BoardProjection{ACCoverage: map[string]int{"ac-1": tc.stubs}, OQClaims: map[string]int{"ac-1": tc.stubs}}
			var b strings.Builder
			writeScopingReceipts(&b, p, cardView{ID: "ac-1", Kind: string(tc.kind)})
			if got := b.String(); got != tc.want {
				t.Errorf("writeScopingReceipts =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}
