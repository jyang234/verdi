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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifactview"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/refindex"
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

	// Git is the read-only ref plumbing behind the production Index above
	// and the stale-entry 404 surface's branch-existence probe
	// (notfound.go). nil means the production refindex adapter. Its method
	// set contains nothing capable of mutating a checkout, so a directory
	// read mutates nothing by construction (co-1).
	Git refindex.GitRunner

	// OpenMRs is the in-review chip's forge consultation (dc-4). nil means
	// no forge is configured: chips are silently absent, which is honest —
	// there is no second source to consult. A non-nil lister that errors
	// (unreachable forge, missing credentials, transport failure) degrades
	// to the disclosed "MR status unavailable" notice, never a blocked or
	// partial directory.
	OpenMRs OpenMRLister
}

// resolve fills production defaults for any nil field, rooted at root.
func (h HomeDeps) resolve(root string) HomeDeps {
	if h.Git == nil {
		h.Git = refindex.NewGitRunner()
	}
	if h.Index == nil {
		git := h.Git
		h.Index = func(ctx context.Context) ([]refindex.Entry, error) {
			return refindex.ComputeIndex(ctx, root, git)
		}
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

// statusGroupLabels are the four groups' human headings.
var statusGroupLabels = map[refindex.StatusGroup]string{
	refindex.StatusGroupDraftsInProgress:     "Drafts in progress",
	refindex.StatusGroupAcceptedPendingBuild: "Accepted, pending build",
	refindex.StatusGroupActiveComponents:     "Active components",
	refindex.StatusGroupTerminal:             "Terminal",
}

// designPrefix is the branch-namespace convention `verdi design start`
// cuts every design branch under (cmd/verdi/design.go) — the same
// derivation refindex uses to name a design-branch entry, applied in
// reverse here to address the entry's branch.
const designPrefix = "design/"

// writeDirectorySection renders the whole-store directory. indexErr is the
// index-computation failure, if any (dc-5: it renders as a disclosed
// inline notice in a still-served page, never a dead-end); inReview and
// mrNotice come from consultOpenMRs; mrConfigured gates the second-source
// provenance line.
func writeDirectorySection(buf *bytes.Buffer, root string, entries []refindex.Entry, indexErr error, inReview map[string]bool, mrNotice string, mrConfigured bool) {
	buf.WriteString(`<section class="home-directory"><h2>Directory</h2>`)
	buf.WriteString(`<p class="dir-provenance">Computed from git refs: every spec on the default branch and every draft on a design branch, grouped by status.`)
	if mrConfigured {
		// dc-4: the in-review chip's input is a second, non-ref source —
		// disclosed as such on the page.
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

	byGroup := map[refindex.StatusGroup][]refindex.Entry{}
	for _, e := range entries {
		byGroup[e.StatusGroup] = append(byGroup[e.StatusGroup], e)
	}

	for _, g := range statusGroupOrder {
		group := byGroup[g]
		buf.WriteString(`<section class="dir-group" data-testid="dir-group-`)
		buf.WriteString(string(g))
		buf.WriteString(`"><h3>`)
		buf.WriteString(stdhtml.EscapeString(statusGroupLabels[g]))
		buf.WriteString(` <span class="count">(`)
		fmt.Fprintf(buf, "%d", len(group))
		buf.WriteString(`)</span></h3>`)
		if len(group) == 0 {
			buf.WriteString(`<p class="empty">None.</p></section>`)
			continue
		}
		buf.WriteString(`<ul>`)
		for _, e := range group {
			writeDirectoryEntry(buf, root, e, inReview)
		}
		buf.WriteString(`</ul></section>`)
	}
	buf.WriteString(`</section>`)
}

// sourceChipLabels render each entry's ref source (feature dc-5 via
// refindex's Source enum: "each entry disclosed by source").
var sourceChipLabels = map[refindex.Source]string{
	refindex.SourceDefault: "default branch",
	refindex.SourceLocal:   "local branch",
	refindex.SourceRemote:  "remote-tracking",
	refindex.SourceBoth:    "local + remote",
}

// writeDirectoryEntry renders one index entry: a disclosed notice entry
// (ac-3's no-draft-spec shape — listed and explained, never linked as if a
// board existed), a default-branch spec (today's unprefixed addresses,
// dc-3), or a design-branch draft (the draft-boards story's per-branch
// address grammar, dc-3 — emitted, never invented).
func writeDirectoryEntry(buf *bytes.Buffer, root string, e refindex.Entry, inReview map[string]bool) {
	name := strings.TrimPrefix(e.Ref, "spec/")

	buf.WriteString(`<li class="dir-entry`)
	if e.Disclosed != nil {
		buf.WriteString(` dir-entry-disclosed`)
	}
	buf.WriteString(`" data-testid="dir-entry-`)
	buf.WriteString(stdhtml.EscapeString(name))
	buf.WriteString(`" data-source="`)
	buf.WriteString(string(e.Source))
	buf.WriteString(`">`)

	switch {
	case e.Disclosed != nil:
		// ac-3: a design branch with no draft spec is a notice entry — it
		// names the branch and states the absence, and carries no link.
		buf.WriteString(`<span class="dir-ref">`)
		buf.WriteString(stdhtml.EscapeString(e.Ref))
		buf.WriteString(`</span> `)
		writeSourceChip(buf, e.Source)
		buf.WriteString(` <span class="dir-disclosed">`)
		buf.WriteString(stdhtml.EscapeString(disclosure.Render(*e.Disclosed)))
		buf.WriteString(`</span>`)

	case e.Source == refindex.SourceDefault:
		writeDefaultEntry(buf, root, e, name)

	default:
		writeDesignEntry(buf, e, name, inReview)
	}
	buf.WriteString(`</li>`)
}

// writeDefaultEntry renders a default-branch entry: title linked to its
// corpus page, status and source chips, and the unprefixed board address
// (dc-3) — plus the feature spec's matrix/verdict links, the same
// affordances the pre-directory home carried. Title/class/story are
// PRESENTATION enrichment read from the serving working tree (the same
// artifactview seam the old home used); the entry's existence, grouping,
// and status all come from the computed index alone, so a missing or
// undecodable working-tree file degrades the trimmings, never the entry.
func writeDefaultEntry(buf *bytes.Buffer, root string, e refindex.Entry, name string) {
	title, class, story, boardServable := specWorkingTreeMeta(root, name)
	if title == "" {
		title = e.Ref
	}

	buf.WriteString(`<a href="`)
	buf.WriteString(stdhtml.EscapeString(defaultCorpusHref(name)))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(title))
	buf.WriteString(`</a> `)
	writeStatusChip(buf, e.SpecStatus)
	buf.WriteString(` `)
	writeSourceChip(buf, e.Source)

	if boardServable {
		// The board route serves the working tree's active zone only; an
		// archive-zone (or working-tree-absent) spec gets no board link —
		// dc-3: the directory emits only addresses the routing serves, so
		// a link on this page is live by construction.
		buf.WriteString(` &middot; <a class="dir-board" href="`)
		buf.WriteString(stdhtml.EscapeString(defaultBoardHref(name)))
		buf.WriteString(`">board</a>`)
		if class == artifact.ClassFeature && story != "" {
			buf.WriteString(` <a href="`)
			buf.WriteString(stdhtml.EscapeString(matrixHref(story)))
			buf.WriteString(`">matrix</a> <a href="`)
			buf.WriteString(stdhtml.EscapeString(verdictHref(story)))
			buf.WriteString(`">verdict</a>`)
		}
	}
}

// defaultCorpusHref, defaultBoardHref, matrixHref, verdictHref (here) and
// designBoardHref (below writeDesignEntry) are the directory's address
// grammar, each computed in exactly one place and shared verbatim with the
// home-status-glance leading section (glance.go) — the "mirrors exactly,
// never a third grammar" bar spec/home-status-glance dc-3 sets. Extracting
// them changes no rendered byte here (each is a pure string join of the
// same literals/escapes writeDefaultEntry/writeDesignEntry always wrote
// inline; stdhtml.EscapeString is a per-rune, context-free replacement, so
// escaping the whole joined string equals escaping its parts and
// concatenating — proven by TestRenderHome_DirectoryGroupsChipsAndLinks
// and friends continuing to assert the identical literal hrefs unchanged).
func defaultCorpusHref(name string) string { return "/a/spec/" + name }
func defaultBoardHref(name string) string  { return "/board/spec/" + name }
func matrixHref(story string) string       { return "/matrix/" + story }
func verdictHref(story string) string      { return "/verdict/" + story }

// writeDesignEntry renders a design-branch draft: the entry links to its
// per-branch board address under the sibling draft-boards story's ratified
// grammar — /b/<branch-escaped>/board/spec/<name>, the branch riding one
// path segment with its slashes percent-encoded (draft-boards dc-1) — one
// grammar for local and remote-tracking entries alike; the routing story
// behind it enforces feature dc-5's authoring/sealed split, never this
// page's link shapes (dc-3).
func writeDesignEntry(buf *bytes.Buffer, e refindex.Entry, name string, inReview map[string]bool) {
	branch := designPrefix + name

	buf.WriteString(`<a class="dir-board" href="`)
	buf.WriteString(stdhtml.EscapeString(designBoardHref(name)))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(e.Ref))
	buf.WriteString(`</a> `)
	writeStatusChip(buf, e.SpecStatus)
	buf.WriteString(` `)
	writeSourceChip(buf, e.Source)

	if inReview[branch] {
		// dc-4: chipped from the forge port's open-MR listing — the
		// disclosed second source, never part of the index computation.
		buf.WriteString(` <span class="badge badge-open dir-inreview">in review</span>`)
	}
}

// designBoardHref is the per-branch board address grammar (draft-boards
// dc-1): the branch rides one path segment with its slashes percent-
// encoded; the name segment beneath it is emitted verbatim (always a
// valid slug, never containing a character url.PathEscape would touch).
func designBoardHref(name string) string {
	return "/b/" + url.PathEscape(designPrefix+name) + "/board/spec/" + name
}

// writeStatusChip renders the entry's raw spec status in the same
// badge-<status> vocabulary the board head, the old home listing, and the
// dex's listing pages share, so a draft reads ochre and an accepted spec
// green on every surface.
func writeStatusChip(buf *bytes.Buffer, status string) {
	if status == "" {
		return
	}
	buf.WriteString(`<span class="badge badge-`)
	buf.WriteString(stdhtml.EscapeString(status))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(status))
	buf.WriteString(`</span>`)
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
// from refs; this is trim, not truth).
func specWorkingTreeMeta(root, name string) (title string, class artifact.SpecClass, story string, boardServable bool) {
	path := filepath.Join(root, ".verdi", "specs", "active", name, "spec.md")
	data, err := os.ReadFile(path)
	if err == nil {
		boardServable = true
	} else {
		path = filepath.Join(root, ".verdi", "specs", "archive", name, "spec.md")
		if data, err = os.ReadFile(path); err != nil {
			return "", "", "", false
		}
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return "", "", "", boardServable
	}
	m, err := artifactview.DecodeMeta("spec", fm)
	if err != nil {
		return "", "", "", boardServable
	}
	return m.Base.Title, m.Class, m.Story, boardServable
}
