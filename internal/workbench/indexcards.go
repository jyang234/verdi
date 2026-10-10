// The index's per-card facts (spec/index-v2 ac-2, ac-4; ledger SI-366):
// one pure projection of a directory entry onto what its card states
// beyond the index row itself — its title, age, review state, next move,
// the New story call to action, and whether it carries a disclosure.
//
// Nothing in this file reads git, the forge, the clock, or the disk. The
// inputs are each read ONCE per render by renderHome (index.go) — the one
// directory index, the one forge consultation, the one corpus index, the
// one clock reading — plus each default entry's working-tree read
// (indexcardsread.go), and handed in here, so the facts add no
// computation to the page's one index computation (dc-1; SI-366 (12)).
// Age and quiet come from index-data (refindex.LastChange, IsQuiet) and
// coverage from index-coverage (featurecoverage.Compute, Uncovered): this
// file reads them and derives neither (dc-3). No fact is evidence-bearing
// (dc-1): an age is a committer date, a review state a forge listing, a
// move a fixed table, and coverage family structure only.
package workbench

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/featurecoverage"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/refindex"
)

// cardFacts is one directory entry's card facts.
type cardFacts struct {
	// entry is the card's directory-index row, unchanged.
	entry refindex.Entry
	// name is the entry's spec name: its ref without "spec/".
	name string
	// title is the card's title. A default-branch entry's is its
	// working-tree title, else its ref (the directory's rule since
	// spec/directory-home). A design-branch draft's is the title the index
	// decoded from its content (SI-366 (1)), never its ref: when that is
	// empty, title stays empty and titleUnproven says why. A degraded
	// (no-draft-spec) entry has no content to title; its own disclosure
	// says so.
	title         string
	titleUnproven string
	// class, story and boardServable are a default-branch entry's
	// working-tree read (specTreeMeta); zero for a design-branch entry.
	class         artifact.SpecClass
	story         string
	boardServable bool
	age           ageFact
	review        reviewState
	move          nextMove
	// cta is the New story call to action, nil unless the entry is an
	// accepted feature with an uncovered criterion or unprovable coverage
	// (SI-366 (10)).
	cta *callToAction
	// disclosed is the card-level disclosure the "disclosed" filter
	// selects (SI-366 (19), (21)(b)): any unproven fact the card itself
	// states — the entry's Disclosed or DateDisclosed, an unproven age, or
	// an unproven draft title — so the filter never hides one.
	// Call-to-action coverage disclosures are excluded.
	disclosed bool
}

// cardContext is the render's one reading of every input a card shares
// with every other card.
type cardContext struct {
	review reviewConsultation
	corpus corpusRead
	// now is the render's one clock reading (HomeDeps.Clock, read once per
	// render; spec/index-data ac-2).
	now   time.Time
	words classWords
}

// projectCard is the one pure per-card projection: e is the entry, tree
// its working-tree read (zero for a design-branch entry).
func projectCard(e refindex.Entry, tree specTreeMeta, cc cardContext) cardFacts {
	c := cardFacts{
		entry:  e,
		name:   strings.TrimPrefix(e.Ref, "spec/"),
		age:    ageOf(e, cc.now),
		review: reviewOf(e, cc.review),
	}
	if e.Source == refindex.SourceDefault {
		c.title = tree.title
		if c.title == "" {
			c.title = e.Ref
		}
		c.class, c.story, c.boardServable = tree.class, tree.story, tree.boardServable
	} else {
		c.title = e.Title
		if c.title == "" && e.Disclosed == nil {
			c.titleUnproven = disclosure.Render(disclosure.New("workbench:title-unproven", e.Ref, "no title was decoded from this design branch's spec"))
		}
	}
	c.disclosed = e.Disclosed != nil || e.DateDisclosed != nil || c.age.unproven != "" || c.titleUnproven != ""
	c.move = moveOf(e, c.review, cc.corpus, cc.words)
	c.cta = ctaFrom(c.name, coverageOf(e, tree, cc.corpus), cc.words)
	return c
}

// ageFact is a card's age (SI-366 (13)).
type ageFact struct {
	// text is "today", "<n> d ago", "quiet <n> d", or "age unproven".
	text string
	// days is the whole days since the last change; meaningful only when
	// unproven is empty.
	days int
	// quiet is refindex.IsQuiet against the render's clock: only a
	// drafts-in-progress entry, and only past QuietAfterDays (exclusive).
	quiet bool
	// unproven is the disclosed reason no age can be stated, empty when
	// the age is proven.
	unproven string
}

// ageUnprovenText is the age chip's text when no age can be stated.
const ageUnprovenText = "age unproven"

// ageOf is e's age against now: index-data's readable last change
// (refindex.LastChange) and quiet decision (refindex.IsQuiet), never a
// second date rule. An unreadable date is disclosed with the entry's own
// reason (dateUnprovenReason); so is a last change after now, which no
// age can honestly name.
func ageOf(e refindex.Entry, now time.Time) ageFact {
	last, ok := refindex.LastChange(e)
	if !ok {
		return ageFact{text: ageUnprovenText, unproven: dateUnprovenReason(e)}
	}
	since := now.Sub(last)
	if since < 0 {
		reason := fmt.Sprintf("last-change date %s is after this render's clock %s", e.Date, now.Format(time.RFC3339))
		return ageFact{text: ageUnprovenText, unproven: disclosure.Render(disclosure.New("workbench:age-unproven", e.Ref, reason))}
	}
	days := int(since / (24 * time.Hour))
	a := ageFact{days: days, quiet: refindex.IsQuiet(e, now)}
	switch {
	case a.quiet:
		a.text = fmt.Sprintf("quiet %d d", days)
	case days == 0:
		a.text = "today"
	default:
		a.text = fmt.Sprintf("%d d ago", days)
	}
	return a
}

// reviewState is a card's review state (SI-366 (4)) — a closed enum.
type reviewState string

const (
	// reviewOpen: the forge lists an open MR from the draft's branch.
	reviewOpen reviewState = "open"
	// reviewNotOpen: the forge answered and lists none, or the entry is a
	// default-branch spec, which has no design branch to be in review.
	reviewNotOpen reviewState = "not-open"
	// reviewUnavailable: the forge could not be consulted this render, so
	// the draft's review state is unknown — never "not open".
	reviewUnavailable reviewState = "unavailable"
	// reviewUnconfigured: no forge is configured to consult.
	reviewUnconfigured reviewState = "unconfigured"
)

// reviewConsultation is the render's one forge consultation
// (consultOpenMRs), as the cards read it.
type reviewConsultation struct {
	// configured is whether a forge lister is wired (HomeDeps.OpenMRs).
	configured bool
	// failed is whether the consultation failed (its notice disclosed).
	failed bool
	// inReview maps each design branch with an open MR to its open
	// requests' forge-native ids, when it succeeded.
	inReview map[string][]string
}

// reviewOf is e's review state. A design-branch entry reads the
// consultation; a default-branch entry is never in review. An unknown
// source fails closed to unavailable: the card claims neither open nor
// not open.
func reviewOf(e refindex.Entry, rc reviewConsultation) reviewState {
	switch e.Source {
	case refindex.SourceDefault:
		return reviewNotOpen
	case refindex.SourceLocal, refindex.SourceRemote, refindex.SourceBoth:
	default:
		return reviewUnavailable
	}
	switch {
	case !rc.configured:
		return reviewUnconfigured
	case rc.failed:
		return reviewUnavailable
	case len(rc.inReview[reviewBranch(e)]) > 0:
		return reviewOpen
	default:
		return reviewNotOpen
	}
}

// reviewBranch is the design branch whose open MRs put e in review: the
// branch `verdi design start` cut for the draft (designPrefix).
func reviewBranch(e refindex.Entry) string {
	return designPrefix + strings.TrimPrefix(e.Ref, "spec/")
}

// moveKind names a next move (SI-366 (2)'s table) — a closed enum; the
// zero value, no kind, is "no move".
type moveKind string

const (
	moveOpenWall      moveKind = "open-wall"
	moveInspectBranch moveKind = "inspect-branch"
	moveAwaitingMerge moveKind = "awaiting-merge"
	moveSealedWall    moveKind = "sealed-wall"
	moveObligations   moveKind = "obligations"
	moveSeeSuccessor  moveKind = "see-successor"
	moveArchive       moveKind = "archive"
)

// nextMove is a card's next move. It never claims evidence (SI-366 (2)).
type nextMove struct {
	kind moveKind
	text string
	// href is set only for see-successor, and only when a superseded-by
	// backlink names the successor: its corpus page.
	href string
}

// The entry statuses the next move reads off refindex.Entry.SpecStatus
// (specstate's projected artifact status for a feature or story, the raw
// field for a component).
const (
	entryStatusClosed     = "closed"
	entryStatusSuperseded = "superseded"
	entryStatusUnproven   = "unproven"
)

// moveOf is e's next move, per SI-366 (2)'s table. An archived entry has
// none; so, failing closed, does an entry whose zone, group, or terminal
// status is not one the table names.
func moveOf(e refindex.Entry, review reviewState, corpus corpusRead, words classWords) nextMove {
	if e.Zone != refindex.ZoneActive {
		return nextMove{}
	}
	switch e.StatusGroup {
	case refindex.StatusGroupDraftsInProgress:
		switch {
		case e.Disclosed != nil || e.SpecStatus == entryStatusUnproven:
			return nextMove{kind: moveInspectBranch, text: "inspect the branch"}
		case review == reviewOpen:
			return nextMove{kind: moveAwaitingMerge, text: "awaiting " + words.verb("merge")}
		default:
			return nextMove{kind: moveOpenWall, text: "open the wall"}
		}
	case refindex.StatusGroupAcceptedPendingBuild:
		return nextMove{kind: moveSealedWall, text: "sealed wall"}
	case refindex.StatusGroupActiveComponents:
		return nextMove{kind: moveObligations, text: "obligations"}
	case refindex.StatusGroupTerminal:
		switch e.SpecStatus {
		case entryStatusSuperseded:
			return nextMove{kind: moveSeeSuccessor, text: "see successor", href: successorHref(e.Ref, corpus)}
		case entryStatusClosed:
			return nextMove{kind: moveArchive, text: "archive"}
		}
	}
	return nextMove{}
}

// successorHref is the corpus page of the spec a superseded-by backlink
// names for ref — the first, backlinks being sorted — or "" when the
// corpus is unreadable or names none.
func successorHref(ref string, corpus corpusRead) string {
	if corpus.links == nil {
		return ""
	}
	for _, bl := range corpus.links.Backlinks(ref) {
		if bl.Type != "superseded-by" {
			continue
		}
		if kind, name, ok := splitSimpleRef(bl.From); ok {
			return "/a/" + kind + "/" + name
		}
	}
	return ""
}

// backlinker is the corpus index's backlink lookup the cards read
// (*index.Index's own method, narrowed at the consumer: 04 §port).
type backlinker interface {
	Backlinks(ref string) []index.Backlink
}

// corpusRead is the render's one corpus index (index.Build): its
// backlinks, or why it could not be built.
type corpusRead struct {
	links backlinker // nil when err is set
	err   error
}

// newCorpusRead wraps index.Build's answer, never holding a nil index
// behind a non-nil interface.
func newCorpusRead(ix *index.Index, err error) corpusRead {
	switch {
	case err != nil:
		return corpusRead{err: err}
	case ix == nil:
		return corpusRead{err: errors.New("the corpus index was not built")}
	}
	return corpusRead{links: ix}
}

// unreadReason is the coverage disclosure for a corpus index that could
// not be built, or "" when it was: the one wording the index's coverageOf
// and the New story dialog's createCoverageOf both disclose.
func (c corpusRead) unreadReason() string {
	if c.err == nil {
		return ""
	}
	return "the corpus index could not be built: " + c.err.Error()
}

// coverageRead is an accepted feature's criterion coverage as the call
// to action reads it.
type coverageRead struct {
	// applies is false when the entry is not an accepted feature: no call
	// to action at all.
	applies bool
	// criteria are the feature's declared criterion ids, in declared order.
	criteria []string
	coverage map[string]featurecoverage.Coverage
	// unproven is why coverage could not be read (SI-366 (10)).
	unproven string
}

// coverageOf assembles an accepted default-branch feature's coverage
// from its working-tree declaration (criteria and stubs) and the corpus's
// implements backlinks, through featurecoverage.Compute — the function
// the wall and the New story dialog share (index-coverage co-2). Any
// input it cannot read — the working-tree file missing or undecodable, no
// active-zone file to serve the dialog, or the corpus index unbuilt — is
// disclosed as unproven, never counted as no coverage.
func coverageOf(e refindex.Entry, tree specTreeMeta, corpus corpusRead) coverageRead {
	if e.Source != refindex.SourceDefault || e.StatusGroup != refindex.StatusGroupAcceptedPendingBuild {
		return coverageRead{}
	}
	if tree.unreadable != "" {
		return coverageRead{applies: true, unproven: tree.unreadable}
	}
	if tree.class != artifact.ClassFeature {
		return coverageRead{}
	}
	if !tree.boardServable {
		return coverageRead{applies: true, unproven: "no active-zone working-tree file exists to serve its wall"}
	}
	if reason := corpus.unreadReason(); reason != "" {
		return coverageRead{applies: true, unproven: reason}
	}
	links := storyLinksOf(corpus.links, e.Ref, tree.criteria)
	return coverageRead{
		applies:  true,
		criteria: tree.criteria,
		coverage: featurecoverage.Compute(tree.criteria, featurecoverage.StubDecls(tree.stubs), links),
	}
}

// callToAction is the New story call to action (SI-366 (10), (11)).
type callToAction struct {
	// unclaimed counts the criteria no stub lists and no story implements.
	unclaimed int
	// criterion is the first of them, in declared order.
	criterion string
	// href opens the feature's wall with the New story dialog claiming
	// criterion: /board/spec/<feature>?new-story=<criterion>.
	href string
	// text is "<n> AC unclaimed · <criterion> · New <story word>", or
	// "coverage unproven".
	text string
	// unproven is the disclosed reason coverage could not be read; when
	// set, unclaimed, criterion and href are empty — never a zero count.
	unproven string
}

// coverageUnprovenText is the call to action's text when coverage could
// not be read.
const coverageUnprovenText = "coverage unproven"

// ctaFrom is the call to action for the feature named name from its
// coverage: nil when none applies or every criterion is covered; the
// coverage-unproven form when any input was unreadable or any criterion
// carries a disclosure (SI-366 (10): never counted, never a zero).
func ctaFrom(name string, cov coverageRead, words classWords) *callToAction {
	if !cov.applies {
		return nil
	}
	if cov.unproven != "" {
		return &callToAction{text: coverageUnprovenText, unproven: cov.unproven}
	}
	var uncovered []string
	for _, id := range cov.criteria {
		c := cov.coverage[id]
		if len(c.Disclosed) > 0 {
			return &callToAction{text: coverageUnprovenText, unproven: id + ": " + strings.Join(c.Disclosed, "; ")}
		}
		if c.Uncovered() {
			uncovered = append(uncovered, id)
		}
	}
	if len(uncovered) == 0 {
		return nil
	}
	first := uncovered[0]
	storyWord := words.word("story")
	return &callToAction{
		unclaimed: len(uncovered),
		criterion: first,
		href:      defaultBoardHref(name) + "?new-story=" + url.QueryEscape(first),
		text:      fmt.Sprintf("%d AC unclaimed · %s · New %s", len(uncovered), first, storyWord),
	}
}
