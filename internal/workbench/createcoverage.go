// The New story dialog's data (spec/new-story-dialog-v2 ac-2, dc-2;
// SI-369 (1), (3)): the coverage chip text the wall's AC cards and the
// dialog's criteria share, and the dialog's criterion coverage, computed
// through featurecoverage.Compute — the function the wall and the index
// already use — so the three never disagree about which criteria are
// covered.
package workbench

import (
	"strconv"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/featurecoverage"
)

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

// createCoverageView is the New story dialog's criterion coverage: one
// row per declared acceptance criterion of the feature, in declared
// order, and the count the dialog's legend reads. It is computed only for
// a wall that renders the dialog (loadBoard), and the zero value — no
// rows — is every other wall's.
type createCoverageView struct {
	// Criteria are the feature's declared criteria, in declared order.
	Criteria []createCriterionView
	// Uncovered counts the rows whose Uncovered flag is set: the legend's
	// "n AC unclaimed" (SI-369 (3)).
	Uncovered int
	// Unproven is set when any row carries a disclosure. The legend then
	// reads "coverage unproven" rather than a count, because an input
	// that could not be read is never counted as no coverage (SI-369 (3)).
	Unproven bool
}

// createCriterionView is one criterion's coverage in the dialog.
type createCriterionView struct {
	// ID is the criterion's declared id.
	ID string
	// Stubs counts the distinct declared stubs that list the criterion:
	// the wall chip's count, worded by coverageChipText.
	Stubs int
	// Stories are the refs of the stories whose implements edge names the
	// criterion, sorted; a superseded story still counts (SI-369 (14)).
	Stories []string
	// Disclosed are the reasons an input that might have covered the
	// criterion could not be read.
	Disclosed []string
	// Uncovered is featurecoverage's Uncovered(): no stub, no story and
	// no disclosure.
	Uncovered bool
}

// createCoverageOf computes the New story dialog's coverage of the
// feature featureRef declared by fm: featurecoverage.Compute over the
// wall's stubs and the corpus's implements backlinks (dc-2), with the
// story half assembled by storyLinksOf, as the index's coverageOf does.
// A corpus that could not be read discloses every criterion's story half
// rather than reading it as none: no row is then uncovered, and the view
// is unproven.
func createCoverageOf(featureRef string, fm *artifact.SpecFrontmatter, corpus corpusRead) createCoverageView {
	ids := make([]string, len(fm.AcceptanceCriteria))
	for i, ac := range fm.AcceptanceCriteria {
		ids[i] = ac.ID
	}
	var links []featurecoverage.StoryLink
	if reason := corpus.unreadReason(); reason != "" {
		for _, id := range ids {
			links = append(links, featurecoverage.StoryLink{CriterionID: id, Unreadable: reason})
		}
	} else {
		links = storyLinksOf(corpus.links, featureRef, ids)
	}
	coverage := featurecoverage.Compute(ids, featurecoverage.StubDecls(fm.Stubs), links)

	var view createCoverageView
	for _, id := range ids {
		c := coverage[id]
		row := createCriterionView{
			ID:        id,
			Stubs:     len(c.Stubs),
			Stories:   c.Stories,
			Disclosed: c.Disclosed,
			Uncovered: c.Uncovered(),
		}
		if row.Uncovered {
			view.Uncovered++
		}
		if len(row.Disclosed) > 0 {
			view.Unproven = true
		}
		view.Criteria = append(view.Criteria, row)
	}
	return view
}
