// The New story dialog's data (spec/new-story-dialog-v2 ac-2, dc-2;
// SI-369 (1), (3)): the coverage chip text the wall's AC cards and the
// dialog's criteria share, and the dialog's criterion coverage, computed
// through featurecoverage.Compute — the function the wall and the index
// already use — so the three never disagree about which criteria are
// covered.
package workbench

import "strconv"

// coverageChipText is the wall's coverage chip text for a criterion that
// stubs distinct declared stubs list (spec/scoping-canvas ac-4): "no
// stub", "covered by 1 stub", or "covered by N stubs". It is the stub
// half only; index-coverage ac-1 keeps these texts unchanged, and the New
// story dialog shows them verbatim (SI-369 (1)), so the wall's receipts
// and the dialog word a count through this one function.
func coverageChipText(stubs int) string {
	switch stubs {
	case 0:
		return "no stub"
	case 1:
		return "covered by 1 stub"
	default:
		return "covered by " + strconv.Itoa(stubs) + " stubs"
	}
}
