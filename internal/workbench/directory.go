// The whole-store directory (spec/directory-home): the home page's
// organizing content. It renders the computed directory index — every spec
// on the default branch and every draft on a design branch — grouped by
// status per spec/workbench-directory dc-2, every entry status-chipped and
// linked per the ratified address grammars (dc-3), disclosed by source, and
// chipped "in review" from a per-render, non-blocking forge consultation
// (dc-4). The index itself is CONSUMED through the sibling ref-index
// story's seam (refindex.ComputeIndex) — this file performs no git ref
// enumeration of its own and holds no second copy of the grouping rules
// (dc-2): grouping keys off each entry's StatusGroup field, never its
// address or on-disk path.
package workbench

import (
	"bytes"
	"context"
	"fmt"
	stdhtml "html"
	"net/url"
	"strconv"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/refindex"
	"github.com/jyang234/verdi/internal/store"
)

// OpenMRLister is the directory's in-review consultation port (dc-4): the
// source branches of every open MR/PR targeting the store's default
// branch, consulted fresh per render. It is a consumer-defined interface
// (04 §port pattern) so this package never imports internal/forge — the
// caller (cmd/verdi's serve.go) adapts the forge port's ListOpenMRs onto
// it, and the hermetic harness/test doubles implement it directly (co-2).
type OpenMRLister interface {
	// OpenMRSourceBranches returns the source (head) branch of every open
	// merge/pull request targeting the store's default branch.
	OpenMRSourceBranches(ctx context.Context) ([]string, error)
}

// HomeDeps carries the home page's injected collaborators. It is a
// separate struct from Deps (boardspec.go) because the directory is the
// home page's own concern, not the board's — and its zero value is the
// full production wiring, so existing NewHandlerWith callers need no
// change.
type HomeDeps struct {
	// Index computes the whole-store directory index (dc-2: the ref-index
	// story's seam, consumed as an input, never re-derived). nil means
	// production: refindex.ComputeIndex over the store root through Git
	// below. Tests inject canned entries here to drive the renderer alone.
	Index func(ctx context.Context) ([]refindex.Entry, error)

	// Git is the read-only ref plumbing behind the production Index above.
	// nil means the production refindex adapter. Its method set contains
	// nothing capable of mutating a checkout, so a directory read mutates
	// nothing by construction (co-1).
	Git refindex.GitRunner

	// OpenMRs is the in-review chip's forge consultation (dc-4). nil means
	// no forge is configured: chips are silently absent, which is honest —
	// there is no second source to consult. A non-nil lister that errors
	// (unreachable forge, missing credentials, transport failure) degrades
	// to the disclosed "MR status unavailable" notice, never a blocked or
	// partial directory.
	OpenMRs OpenMRLister

	// Model is the store's resolved operating model
	// (spec/vocabulary-surfaces ac-2): the status chips this page renders
	// resolve their visible words through its display vocabulary. nil
	// means resolve() fills it from the store root once at handler
	// construction (store.Open — never per render); a store whose model
	// cannot be resolved renders bare ids, exactly like a model with no
	// renames.
	Model *model.Model

	// Clock is the "now" seam behind the quiet-draft decision
	// (spec/index-data ac-2, dc-3: refindex.IsQuiet is always handed an
	// injected now, never reading the wall clock itself). nil means
	// production: resolve() fills it with time.Now, whose EVERY call
	// still reads the real wall clock fresh — resolve() runs once, at
	// handler construction, not per render, so assigning the function
	// value here (never a captured time.Time snapshot) is what keeps "nil
	// means the wall clock read at render time" true across the whole
	// server's lifetime. Tests inject a fixed func instead (ac-2's own
	// "tests ... set it"), and `verdi serve` injects VERDI_NOW's fixed
	// instant when that variable is set — the e2e harness's clock
	// (SI-296) — so no quiet decision ever depends on when a test
	// happened to run. renderHome reads it once per render.
	Clock func() time.Time

	// Corpus builds the working tree's corpus index: the other-records
	// listing and the index cards' backlinks (the New story call to
	// action's implementing stories, a superseded spec's successor). nil
	// means production: index.Build. renderHome calls it exactly once per
	// render (spec/index-v2 ac-4; SI-366 (12)), before the directory, so a
	// test counts its calls here.
	Corpus func(root string) (*index.Index, error)
}

// resolve fills production defaults for any nil field, rooted at root.
func (h HomeDeps) resolve(root string) HomeDeps {
	if h.Git == nil {
		h.Git = refindex.NewGitRunner()
	}
	if h.Index == nil {
		git := h.Git
		// The effective-state resolver rides beside the ref plumbing: the
		// production seam over specstate.NewProjector, constructed HERE (the
		// I/O layer) and passed in — refindex batches its ResolveMany calls
		// internally, so the directory never triggers a per-entry corpus scan.
		resolver := refindex.NewStateResolver()
		h.Index = func(ctx context.Context) ([]refindex.Entry, error) {
			return refindex.ComputeIndex(ctx, root, git, resolver)
		}
	}
	if h.Model == nil {
		if cfg, err := store.Open(root); err == nil {
			h.Model = cfg.Model
		}
	}
	if h.Clock == nil {
		h.Clock = time.Now
	}
	if h.Corpus == nil {
		h.Corpus = index.Build
	}
	return h
}

// openMRConsultTimeout bounds the per-render forge consultation (dc-4:
// the refs-computed directory is never blocked on the network — a hung
// forge degrades to the disclosed absence instead of delaying the page).
const openMRConsultTimeout = 2 * time.Second

// consultOpenMRs performs the per-render, non-blocking in-review
// consultation (dc-4). It returns the set of design branches with an open
// MR and, when the consultation failed, the disclosed notice text — the
// caller renders the notice and the refs-computed directory in full either
// way. A nil lister (no forge configured) is the silent, legitimate
// absence: no chips, no notice.
func consultOpenMRs(ctx context.Context, mrs OpenMRLister) (inReview map[string]bool, notice string) {
	if mrs == nil {
		return nil, ""
	}
	ctx, cancel := context.WithTimeout(ctx, openMRConsultTimeout)
	defer cancel()
	branches, err := mrs.OpenMRSourceBranches(ctx)
	if err != nil {
		d := disclosure.New(
			"workbench:mr-status",
			"",
			fmt.Sprintf("MR status unavailable (%v) — in-review chips cannot be shown; the directory renders from git refs alone", err),
		)
		return nil, disclosure.Render(d)
	}
	inReview = make(map[string]bool, len(branches))
	for _, b := range branches {
		inReview[b] = true
	}
	return inReview, ""
}

// statusGroupOrder is the page's fixed group order — feature dc-2's four
// buckets, most-in-motion first. Rendering order is a presentation choice
// of this page; the vocabulary itself is refindex's (dc-2: no second copy
// of the grouping rules lives here).
var statusGroupOrder = []refindex.StatusGroup{
	refindex.StatusGroupDraftsInProgress,
	refindex.StatusGroupAcceptedPendingBuild,
	refindex.StatusGroupActiveComponents,
	refindex.StatusGroupTerminal,
}

// designPrefix is the branch-namespace convention `verdi design start`
// cuts every design branch under (cmd/verdi/design.go) — the same
// derivation refindex uses to name a design-branch entry, applied in
// reverse here to address the entry's branch.
const designPrefix = "design/"

// writeDirectorySection renders the whole-store directory from the
// entries' card facts (indexcards.go: homeCards, index-aligned with the
// computed index; nil when indexErr is set) as the index's four columns
// (indexcolumns.go; spec/index-v2 ac-1, parent dc-12). indexErr is the
// index-computation failure, if any (dc-5: it renders as a disclosed
// inline notice in a still-served page, never a dead-end — and no column,
// heading or count beside it, since there is no computed population to
// draw); mrNotice comes from consultOpenMRs, whose in-review answer each
// card carries; mrConfigured gates the second-source provenance line. now
// is the render's one clock reading (HomeDeps.Clock, read once per render
// by the caller): every entry's quiet carrier is decided against it
// (spec/index-data ac-2, dc-3; SI-297).
func writeDirectorySection(buf *bytes.Buffer, cards []cardFacts, indexErr error, mrNotice string, mrConfigured bool, mdl *model.Model, now time.Time) {
	buf.WriteString(`<section class="home-directory">`)
	// vocab:identity — the directory's own StatusGroup taxonomy word (L-M8 genus), not the lifecycle state
	buf.WriteString(`<p class="dir-provenance">Computed from git refs: every spec on the default branch and every draft on a design branch, grouped by status.`)
	if mrConfigured {
		// dc-4: the in-review chip's input is a second, non-ref source —
		// disclosed as such on the page.
		// vocab:identity — non-vocabulary homograph: the forge's merge request/merge gate, never the `merge` lifecycle transition word
		buf.WriteString(` &ldquo;In review&rdquo; chips are consulted per render from the forge's open merge requests &mdash; a second source beside the refs.`)
	}
	buf.WriteString(`</p>`)

	if mrNotice != "" {
		buf.WriteString(`<p class="notice dir-mr-unavailable" data-testid="mr-status-unavailable">`)
		buf.WriteString(stdhtml.EscapeString(mrNotice))
		buf.WriteString(`</p>`)
	}

	if indexErr != nil {
		// dc-5: the home page is the one landing surface that must never
		// itself be a dead end — the failure is disclosed inline and the
		// rest of the page still serves.
		buf.WriteString(`<p class="notice dir-index-failed">Could not compute the directory index: `)
		buf.WriteString(stdhtml.EscapeString(indexErr.Error()))
		buf.WriteString(`</p></section>`)
		return
	}

	byGroup := map[refindex.StatusGroup][]cardFacts{}
	for _, c := range cards {
		byGroup[c.entry.StatusGroup] = append(byGroup[c.entry.StatusGroup], c)
	}

	buf.WriteString(`<div class="dir-columns">`)
	for _, g := range statusGroupOrder {
		writeDirectoryColumn(buf, g, byGroup[g], mdl, now)
	}
	buf.WriteString(`</div></section>`)
}

// sourceChipLabels render each entry's ref source (feature dc-5 via
// refindex's Source enum: "each entry disclosed by source").
var sourceChipLabels = map[refindex.Source]string{
	refindex.SourceDefault: "default branch",
	refindex.SourceLocal:   "local branch",
	refindex.SourceRemote:  "remote-tracking",
	refindex.SourceBoth:    "local + remote",
}

// writeDirectoryEntry renders one index entry as a card (spec/index-v2
// ac-2; the card stays the li.dir-entry root, SI-366 (14)): a disclosed
// notice entry (ac-3's no-draft-spec shape — listed and explained, never
// linked as if a board existed), a default-branch spec (today's
// unprefixed addresses, dc-3), or a design-branch draft (the draft-boards
// story's per-branch address grammar, dc-3 — emitted, never invented).
// The open tag keeps its pinned attribute order — class, data-testid,
// data-source, the date carriers — then carries the card's review state
// and disclosed fact (indexcards.go) for the filters to read.
func writeDirectoryEntry(buf *bytes.Buffer, c cardFacts, mdl *model.Model, now time.Time) {
	e, name := c.entry, c.name

	buf.WriteString(`<li class="dir-entry`)
	if e.Disclosed != nil {
		buf.WriteString(` dir-entry-disclosed`)
	}
	buf.WriteString(`" data-testid="dir-entry-`)
	buf.WriteString(stdhtml.EscapeString(name))
	buf.WriteString(`" data-source="`)
	buf.WriteString(string(e.Source))
	buf.WriteString(`"`)
	writeDateCarriers(buf, e, now)
	buf.WriteString(` data-review="`)
	buf.WriteString(string(c.review))
	buf.WriteString(`" data-disclosed="`)
	buf.WriteString(strconv.FormatBool(c.disclosed))
	buf.WriteString(`">`)

	switch {
	// A DEFAULT-BRANCH entry keeps its full identity even when its
	// effective lifecycle state is unproven (Task 6 fix round 2, finding
	// 1): content DOES exist at this entry's ref — title, corpus link, an
	// honest unproven status treatment, and the read-only board link all
	// still hold — so a Disclosure here rides BESIDE the ordinary render
	// (writeDefaultEntry appends it), never replaces it. Collapsing it
	// into the bare notice shape below would strip every affordance off
	// the row — and one malformed corpus spec makes EVERY default entry
	// unproven at once.
	case e.Source == refindex.SourceDefault:
		writeDefaultEntry(buf, c, mdl)
	case e.Disclosed != nil:
		writeNoticeEntry(buf, c)
	default:
		writeDesignEntry(buf, c, mdl)
	}
	buf.WriteString(`</li>`)
}

// writeNoticeEntry renders ac-3's no-draft-spec card: it names the branch
// in the title position (there is genuinely no content to title), states
// the absence through the shared disclosure vocabulary, and carries no
// link — its next move, inspect the branch, is text alone.
func writeNoticeEntry(buf *bytes.Buffer, c cardFacts) {
	buf.WriteString(`<div class="dir-card-title"><span class="dir-ref">`)
	buf.WriteString(stdhtml.EscapeString(c.entry.Ref))
	buf.WriteString(`</span></div>`)
	writeCardMeta(buf, c, "")
	writeCardMove(buf, c.move)
	writeCardDisclosure(buf, c.entry)
}

// writeCardMeta renders the card's chip row: the status badge (label is
// the model's display word for it, "" when no rename differs), the source
// chip, the age chip (SI-366 (13)), and the in-review chip when the forge
// lists an open MR from the draft's branch (dc-4; SI-366 (3)).
func writeCardMeta(buf *bytes.Buffer, c cardFacts, statusLabel string) {
	buf.WriteString(`<div class="dir-meta">`)
	if c.entry.SpecStatus != "" {
		writeStatusChip(buf, c.entry.SpecStatus, statusLabel)
		buf.WriteString(` `)
	}
	writeSourceChip(buf, c.entry.Source)
	buf.WriteString(` `)
	writeAgeChip(buf, c.age)
	if c.review == reviewOpen {
		// dc-4: chipped from the forge port's open-MR listing — the
		// disclosed second source, never part of the index computation.
		buf.WriteString(` <span class="badge badge-open dir-inreview">in review</span>`)
	}
	buf.WriteString(`</div>`)
}

// writeAgeChip renders the age fact: its text, the quiet modifier when
// index-data's IsQuiet says so, and — when no age can be stated — the
// disclosed unproven treatment carrying the reason.
func writeAgeChip(buf *bytes.Buffer, a ageFact) {
	switch {
	case a.unproven != "":
		buf.WriteString(`<span class="dir-age dir-age-unproven dir-unproven" title="`)
		buf.WriteString(stdhtml.EscapeString(a.unproven))
		buf.WriteString(`">`)
	case a.quiet:
		buf.WriteString(`<span class="dir-age dir-age-quiet">`)
	default:
		buf.WriteString(`<span class="dir-age">`)
	}
	buf.WriteString(stdhtml.EscapeString(a.text))
	buf.WriteString(`</span>`)
}

// writeCardMove renders the next move (SI-366 (2)): nothing when the table
// names none, a link only for a successor the corpus names.
func writeCardMove(buf *bytes.Buffer, m nextMove) {
	if m.kind == "" {
		return
	}
	buf.WriteString(`<div class="dir-move">`)
	if m.href != "" {
		buf.WriteString(`<a href="`)
		buf.WriteString(stdhtml.EscapeString(m.href))
		buf.WriteString(`">&rarr; `)
		buf.WriteString(stdhtml.EscapeString(m.text))
		buf.WriteString(`</a>`)
	} else {
		buf.WriteString(`&rarr; `)
		buf.WriteString(stdhtml.EscapeString(m.text))
	}
	buf.WriteString(`</div>`)
}

// writeCardDisclosure renders the entry's own disclosure, when it carries
// one, in the shared disclosure vocabulary — beside the card's facts,
// never replacing them.
func writeCardDisclosure(buf *bytes.Buffer, e refindex.Entry) {
	if e.Disclosed == nil {
		return
	}
	buf.WriteString(`<span class="dir-disclosed">`)
	buf.WriteString(stdhtml.EscapeString(disclosure.Render(*e.Disclosed)))
	buf.WriteString(`</span>`)
}

// writeDateCarriers writes SI-297's non-visible date carriers onto an
// entry's own <li> — attributes only, no visible text, so the index story
// (F7) can render ages and the quiet mark, and the e2e harness can assert
// them, from the served page itself:
//
//   - data-last-change: the entry's committer date (refindex.LastChange
//     reads it), or
//   - data-date-unproven: the reason no date is readable — the entry's own
//     date disclosure, or this renderer's when an entry carries neither
//     (silence is never a pass); and
//   - data-quiet ("true" or "false"): only on a drafts-in-progress entry
//     with a readable date, decided by refindex.IsQuiet against now. An
//     absent data-quiet means unproven, never not-quiet (dc-2: no entry
//     outside that group reads quiet).
func writeDateCarriers(buf *bytes.Buffer, e refindex.Entry, now time.Time) {
	if _, ok := refindex.LastChange(e); !ok {
		buf.WriteString(` data-date-unproven="`)
		buf.WriteString(stdhtml.EscapeString(dateUnprovenReason(e)))
		buf.WriteString(`"`)
		return
	}
	buf.WriteString(` data-last-change="`)
	buf.WriteString(stdhtml.EscapeString(e.Date))
	buf.WriteString(`"`)
	if e.StatusGroup == refindex.StatusGroupDraftsInProgress {
		buf.WriteString(` data-quiet="`)
		buf.WriteString(strconv.FormatBool(refindex.IsQuiet(e, now)))
		buf.WriteString(`"`)
	}
}

// dateUnprovenReason renders why e has no readable date, in the shared
// disclosure vocabulary: its own DateDisclosed when refindex disclosed one,
// otherwise this renderer's own disclosure naming the gap (an entry built
// outside refindex's computation — never zero, never now).
func dateUnprovenReason(e refindex.Entry) string {
	if e.DateDisclosed != nil {
		return disclosure.Render(*e.DateDisclosed)
	}
	text := "no last-change date was computed for this entry"
	if e.Date != "" {
		text = fmt.Sprintf("last-change date %q is not a committer date", e.Date)
	}
	return disclosure.Render(disclosure.New("workbench:date-unproven", e.Ref, text))
}

// writeDefaultEntry renders a default-branch card: its title linked to its
// corpus page (the card's first link), its ref, the chip row, and — only
// where the routing serves it — the unprefixed board address (dc-3) plus
// the feature spec's matrix and verdict links for a `story:` tracker
// field (SI-366 (20)), the same affordances the pre-directory home
// carried; then its next move and any disclosure. Title/class/story are
// PRESENTATION enrichment read from the serving working tree (the card's
// one working-tree read, readSpecTreeMeta — the same artifactview seam
// the old home used); the entry's existence, grouping, and status all
// come from the computed index alone, so a missing or undecodable
// working-tree file degrades the trimmings, never the entry.
func writeDefaultEntry(buf *bytes.Buffer, c cardFacts, mdl *model.Model) {
	e, name, title, class, story, boardServable := c.entry, c.name, c.title, c.class, c.story, c.boardServable

	buf.WriteString(`<div class="dir-card-title"><a class="dir-title" href="`)
	buf.WriteString(stdhtml.EscapeString(defaultCorpusHref(name)))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(title))
	buf.WriteString(`</a></div><span class="dir-ref">`)
	buf.WriteString(stdhtml.EscapeString(e.Ref))
	buf.WriteString(`</span>`)
	writeCardMeta(buf, c, statusChipLabel(mdl, string(class), e.SpecStatus))

	if boardServable {
		// The board route serves the working tree's active zone only; an
		// archive-zone (or working-tree-absent) spec gets no board link —
		// dc-3: the directory emits only addresses the routing serves, so
		// a link on this page is live by construction.
		buf.WriteString(`<div class="dir-links"><a class="dir-board" href="`)
		buf.WriteString(stdhtml.EscapeString(defaultBoardHref(name)))
		buf.WriteString(`">board</a>`)
		if class == artifact.ClassFeature && story != "" {
			buf.WriteString(` <a href="`)
			buf.WriteString(stdhtml.EscapeString(matrixHref(story)))
			buf.WriteString(`">matrix</a> <a href="`)
			buf.WriteString(stdhtml.EscapeString(verdictHref(story)))
			buf.WriteString(`">verdict</a>`)
		}
		buf.WriteString(`</div>`)
	}

	writeCardMove(buf, c.move)
	// An unproven default-branch entry's disclosure rides beside the full
	// identity render, in the shared disclosure vocabulary — never a
	// replacement for it (fix round 2, finding 1).
	writeCardDisclosure(buf, e)
}

// defaultCorpusHref, defaultBoardHref, matrixHref, verdictHref (here),
// BranchBoardHref (the shared per-branch constructor below) and
// designBoardHref (below writeDesignEntry) are the directory's address
// grammar, each computed in exactly one place (the "never a third
// grammar" bar the retired home-status-glance set, kept by parent dc-12).
// Each is a pure string join of the same literals/escapes the entry
// writers always wrote inline; stdhtml.EscapeString is a per-rune,
// context-free replacement, so escaping the whole joined string equals
// escaping its parts and concatenating — proven by
// TestRenderHome_DirectoryGroupsChipsAndLinks and friends asserting the
// literal hrefs.
func defaultCorpusHref(name string) string { return "/a/spec/" + name }
func defaultBoardHref(name string) string  { return boardSpecPrefix + name }
func matrixHref(story string) string       { return "/matrix/" + story }
func verdictHref(story string) string      { return "/verdict/" + story }

// branchBoardPrefix and boardSpecPrefix are the two literals of the board
// address grammar: the per-branch mount prefix (draft-boards dc-1) and the
// board-spec route segment the unprefixed mount and the /b/{branch} mount
// serve alike (draft-boards ac-1 — "the SAME route table beneath the
// prefix"). Every href constructor here and diagramExitStore's parser
// reference these, so the grammar lives in exactly one place (dc-3).
const (
	branchBoardPrefix = "/b/"
	boardSpecPrefix   = "/board/spec/"
)

// BranchBoardHref is THE single constructor of the per-branch board address
// (draft-boards dc-1): the branch rides one path segment with its slashes
// percent-encoded, the spec name beneath it emitted verbatim (always a
// valid slug). The directory's design entries (designBoardHref) and the
// diagram editor's origin path (boardOriginPath, boarddiagram.go) both build
// the address through here; diagramExitStore parses the same two prefixes
// back out.
func BranchBoardHref(branch, name string) string {
	return branchBoardPrefix + url.PathEscape(branch) + boardSpecPrefix + name
}

// writeDesignEntry renders a design-branch draft's card: its one link is
// its board, under the sibling draft-boards story's ratified grammar —
// /b/<branch-escaped>/board/spec/<name>, the branch riding one path
// segment with its slashes percent-encoded (draft-boards dc-1) — one
// grammar for local and remote-tracking entries alike; the routing story
// behind it enforces feature dc-5's authoring/sealed split, never this
// page's link shapes (dc-3). The link reads the draft's decoded title
// (SI-366 (1)); when none was decoded it reads the ref, the only identity
// the entry has, beside the disclosed title-unproven chip.
func writeDesignEntry(buf *bytes.Buffer, c cardFacts, mdl *model.Model) {
	e, name := c.entry, c.name

	buf.WriteString(`<div class="dir-card-title"><a class="dir-board dir-title" href="`)
	buf.WriteString(stdhtml.EscapeString(designBoardHref(name)))
	buf.WriteString(`">`)
	if c.title != "" {
		buf.WriteString(stdhtml.EscapeString(c.title))
	} else {
		buf.WriteString(stdhtml.EscapeString(e.Ref))
	}
	buf.WriteString(`</a>`)
	if c.titleUnproven != "" {
		buf.WriteString(` <span class="dir-unproven dir-title-unproven" title="`)
		buf.WriteString(stdhtml.EscapeString(c.titleUnproven))
		buf.WriteString(`">title unproven</span>`)
	}
	buf.WriteString(`</div><span class="dir-ref">`)
	buf.WriteString(stdhtml.EscapeString(e.Ref))
	buf.WriteString(`</span>`)
	writeCardMeta(buf, c, statusChipLabel(mdl, "", e.SpecStatus))
	writeCardMove(buf, c.move)
}

// designBoardHref is the directory's per-branch board address for a design
// branch: the branch is always designPrefix+name (feature dc-1), and the
// address is built through the shared BranchBoardHref constructor — the
// name segment is a valid slug emitted verbatim, never containing a
// character url.PathEscape would touch.
func designBoardHref(name string) string {
	return BranchBoardHref(designPrefix+name, name)
}

// writeStatusChip renders the entry's spec status in the same
// badge-<status> vocabulary the board head, the old home listing, and the
// dex's listing pages share, so a draft reads ochre and an accepted spec
// green on every surface. label is the model's display word for the
// status (spec/vocabulary-surfaces ac-2; statusChipLabel below) — the
// chip's VISIBLE text only; "" falls back to the raw status, and the
// badge-<status> CSS class keeps the bare id either way (addressing,
// never display).
func writeStatusChip(buf *bytes.Buffer, status, label string) {
	if status == "" {
		return
	}
	if label == "" {
		label = status
	}
	buf.WriteString(`<span class="badge badge-`)
	buf.WriteString(stdhtml.EscapeString(status))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(label))
	buf.WriteString(`</span>`)
}

// statusChipLabel resolves status through the model's display vocabulary
// (the identical DisplayState lookup every other surface uses), returning
// "" when no rename differs — so callers hand writeStatusChip a label
// only when the model actually renames, keeping the no-model/no-rename
// render byte-identical. Nil-safe (model.Model's nil-receiver contract).
// class is the entry's own spec class per DisplayState's Q2 caller
// convention — default-branch entries read it from the working tree
// (specWorkingTreeMeta); design-branch entries pass "" (the computed
// index deliberately carries no class, and a degraded draft may have no
// readable content at all).
func statusChipLabel(m *model.Model, class, status string) string {
	if label := m.DisplayState(class, status); label != status {
		return label
	}
	return ""
}

// writeSourceChip renders the entry's ref source disclosure (feature dc-5).
func writeSourceChip(buf *bytes.Buffer, src refindex.Source) {
	buf.WriteString(`<span class="badge badge-src badge-src-`)
	buf.WriteString(string(src))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(sourceChipLabels[src]))
	buf.WriteString(`</span>`)
}

// specWorkingTreeMeta reads name's spec frontmatter from the serving
// working tree — active zone first, then archive — for presentation
// enrichment only (title, feature class, scalar story ref) plus whether
// the ACTIVE-zone file exists, which is what makes /board/spec/<name>
// servable. Every failure degrades to zero values: the directory entry
// itself never depends on the working tree (dc-2 — the index is computed
// from refs; this is trim, not truth). It is readSpecTreeMeta's trim
// (indexcardsread.go), the one working-tree read the index cards share.
func specWorkingTreeMeta(root, name string) (title string, class artifact.SpecClass, story string, boardServable bool) {
	m := readSpecTreeMeta(root, name)
	return m.title, m.class, m.story, m.boardServable
}
