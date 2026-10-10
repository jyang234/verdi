package workbench

import "strconv"

// The index's in-review chip copy (spec/workbench-redesign dc-4: "A pull
// request shows as open, with its number, and no review state"; ledger
// SI-376 (2); BL-177): one pure projection of a draft's open requests,
// as the render's one forge consultation listed them, onto the chip's
// words. The words "PR" and "MR" are the forges' own names for the
// request, never the operating model's class words, so they are not
// routed through the display vocabulary.

// ForgeKind is the store's configured forge, as `verdi serve`'s forge
// wiring resolves it (forgeBestEffort's configuredKind: verdi.yaml's
// forge: key, else auto-detected from the origin remote). It decides the
// notation the chip writes a request's number in. A closed enum: any
// other value, the empty one included, fails closed — the chip then
// cannot write the number in its forge's notation and discloses that
// instead of guessing one.
type ForgeKind string

// The forge kinds the chip knows a notation for.
const (
	// ForgeGitHub: a pull request, "PR #<number>".
	ForgeGitHub ForgeKind = "github"
	// ForgeGitLab: a merge request, "MR !<iid>".
	ForgeGitLab ForgeKind = "gitlab"
)

// reviewChip is an in-review card's chip: its visible text and, when the
// chip cannot state a number, the disclosed reason (its title), empty
// otherwise.
type reviewChip struct {
	text  string
	title string
}

// numberUnavailableText is the chip's text when no number can be stated:
// still in review, the number disclosed as unavailable, never invented.
const numberUnavailableText = "in review · number unavailable"

// inReviewChip is the chip for a draft whose branch has open requests
// with the given forge-native ids (one per open request, "" for a request
// the forge listed without an id). It names the lowest numbered request
// in kind's notation, then " · +<k>" for the k other open requests; with
// no usable number, or no notation for kind, it says so instead. It
// states no review state (dc-4).
func inReviewChip(kind ForgeKind, ids []string) reviewChip {
	var prefix string
	switch kind {
	case ForgeGitHub:
		prefix = "PR #"
	case ForgeGitLab:
		prefix = "MR !"
	default:
		return reviewChip{
			text:  numberUnavailableText,
			title: "the store names no forge kind this page can write a request number for (github or gitlab), so the open request's number is not shown",
		}
	}
	lowest, ok := lowestRequestNumber(ids)
	if !ok {
		return reviewChip{
			text:  numberUnavailableText,
			title: "the forge listed this branch's open request without a usable number, so none is shown",
		}
	}
	text := prefix + lowest + " open"
	if rest := len(ids) - 1; rest > 0 {
		text += " · +" + strconv.Itoa(rest)
	}
	return reviewChip{text: text}
}

// lowestRequestNumber is the numerically lowest of ids that is a request
// number in the forges' own form — a positive decimal with no sign, no
// leading zero and no padding, within uint64, as both adapters format it
// — and false when none is. Any other id ("", "0", "07", "mr-9") is no
// number: the chip discloses it rather than writing it in a number's
// notation.
func lowestRequestNumber(ids []string) (string, bool) {
	var lowest string
	var lowestN uint64
	found := false
	for _, id := range ids {
		n, ok := requestNumber(id)
		if !ok {
			continue
		}
		if !found || n < lowestN {
			lowest, lowestN, found = id, n, true
		}
	}
	return lowest, found
}

// requestNumber parses id as a request number in the forges' own form.
func requestNumber(id string) (uint64, bool) {
	if id == "" || id[0] < '1' || id[0] > '9' {
		return 0, false
	}
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
