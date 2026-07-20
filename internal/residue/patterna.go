package residue

import "sort"

// PatternA is AC-1 pattern (a)'s finding: a stranded closure ritual — an
// active-zone spec still status: accepted-pending-build whose own
// close/<name> branch already performed the archive move on its own tip,
// but never landed (unmerged into the default branch).
type PatternA struct {
	SpecName string // "<name>"
	Branch   string // "close/<name>"
	Tip      string // the close branch's own tip commit sha
}

// findPatternA derives AC-1 pattern (a)'s findings from closeBranches (the
// SAME shared pass AC-2 classifies from — ac-2's static obligation) and
// activeStatus (name -> raw status string, built from the active-zone spec
// set with dc-2's superseded-exclusion already applied by the caller
// before this runs): a close/<name> branch whose own tip already contains
// archive/<name>, where <name> is STILL an active-zone spec at status:
// accepted-pending-build — all three of pattern (a)'s own conditions
// (dc-1's static obligation): the branch exists (it is IN closeBranches at
// all), it is unmerged (closeBranches already excludes merged branches),
// and its own tip already archived <name>.
func findPatternA(closeBranches []CloseBranch, activeStatus map[string]string) []PatternA {
	var out []PatternA
	for _, cb := range closeBranches {
		if !cb.ArchivedOnOwnTip {
			continue
		}
		if activeStatus[cb.Name] != "accepted-pending-build" {
			continue
		}
		out = append(out, PatternA{SpecName: cb.Name, Branch: cb.Branch, Tip: cb.Tip})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SpecName < out[j].SpecName })
	return out
}
