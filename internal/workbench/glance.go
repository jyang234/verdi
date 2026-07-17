// The home page's leading status glance (spec/home-status-glance): a
// second, additive rendering pass over the exact same entries/indexErr
// renderHome already computed from the ONE shared home.Index(ctx) call
// (index.go) — this file performs no index computation of its own (dc-1).
// Every active spec — default-branch and design-branch alike — regroups
// into three fixed, always-rendered buckets (dc-2, dc-4, ADJ-36's
// total-partition reading), each shown entry carrying only its title,
// status badge, and working links (dc-3) — never a source chip, an
// in-review chip, or any evidence-bearing state (parent workbench-
// legibility dc-4's bar). It renders immediately before the exhaustive
// Directory section, which keeps its own markup, classes, and testids
// completely unchanged (dc-5) — this file adds a second rendering of a
// subset of the same entries; it never mutates or replaces the first.
package workbench

import (
	"bytes"
	stdhtml "html"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/refindex"
)

// glanceBucket is one of the three fixed buckets dc-2 ratifies. member
// partitions refindex's closed, four-value StatusGroup vocabulary exactly
// once per value — proven total by TestGlanceBuckets_TotalPartition.
type glanceBucket struct {
	// slug is dc-5's binding testid suffix (glance-group-<slug>) — fixed
	// regardless of display wording (ADJ-36: testids are addressing
	// artifacts, not display text).
	slug string
	// label is the operator-facing heading. ADJ-36 grants free wording for
	// the trailing bucket: it holds BOTH steady-state active components
	// and winding-down (closed-awaiting-archive/superseded) specs, so
	// "settling" alone would misdescribe half its members — the slug
	// stays "settling" (the binding testid), the label reads honestly.
	label string
	// member reports whether a StatusGroup belongs in this bucket.
	member func(refindex.StatusGroup) bool
}

// glanceBuckets is the glance's fixed rendering order (ac-1): on-the-desk,
// in-flight, then settling — parent dc-1's grouping vocabulary re-grouped,
// never re-derived (dc-2).
var glanceBuckets = []glanceBucket{
	{
		slug:  "on-the-desk",
		label: "On the desk",
		member: func(g refindex.StatusGroup) bool {
			return g == refindex.StatusGroupDraftsInProgress
		},
	},
	{
		slug:  "in-flight",
		label: "In flight",
		member: func(g refindex.StatusGroup) bool {
			return g == refindex.StatusGroupAcceptedPendingBuild
		},
	},
	{
		slug:  "settling",
		label: "On the shelf",
		member: func(g refindex.StatusGroup) bool {
			return g == refindex.StatusGroupActiveComponents || g == refindex.StatusGroupTerminal
		},
	},
}

// writeGlanceSection renders the leading glance over entries — the exact
// slice renderHome already computed via the one shared home.Index(ctx)
// call — and indexErr, its matching failure (nil on success). On failure
// it renders nothing at all: CO-2 requires the SAME indexErr to degrade
// both this section and the exhaustive one identically, and the exhaustive
// section (writeDirectorySection, called right after this) already
// discloses it inline — rendering a second notice here would be exactly
// the "second, contradictory notice" CO-2 forbids, and there is no
// refs-computed population to bucket in the first place.
func writeGlanceSection(buf *bytes.Buffer, root string, entries []refindex.Entry, indexErr error) {
	if indexErr != nil {
		return
	}

	eligible := glanceEligibleEntries(entries)

	buf.WriteString(`<section class="home-glance" data-testid="home-glance"><h2>At a glance</h2><div class="glance-buckets">`)
	for _, b := range glanceBuckets {
		var members []refindex.Entry
		for _, e := range eligible {
			if b.member(e.StatusGroup) {
				members = append(members, e)
			}
		}
		writeGlanceBucket(buf, root, b, members)
	}
	buf.WriteString(`</div></section>`)
}

// glanceEligibleEntries is dc-1/dc-2's population rule: every entry the
// index returns MINUS a disclosed (no-draft-spec) design-branch entry
// (dc-1 — "no content to badge or link") MINUS any entry whose Zone is not
// ZoneActive (dc-2's zone-aware settling, ADJ-32 f1 sustained — an
// archive-zone default-branch entry is excluded from the glance in EVERY
// bucket, not merely settling, since dc-1 states the population rule once,
// up front, for the whole glance). The zone check applies uniformly
// regardless of Source: a design-branch entry is always ZoneActive by
// construction (refindex.go never reads one from anywhere else), so this
// is a no-op for it in practice, and CLAUDE.md's "unknown enum values fail
// closed" governs the one case where it would ever matter — a hand-built
// Entry whose Zone was left unset. Every excluded entry still renders,
// unchanged, in the exhaustive Directory section below (ac-2's no-loss
// bar): this function only ever narrows the GLANCE's own population, never
// the index itself.
func glanceEligibleEntries(entries []refindex.Entry) []refindex.Entry {
	var out []refindex.Entry
	for _, e := range entries {
		if e.Disclosed != nil {
			continue
		}
		if e.Zone != refindex.ZoneActive {
			continue
		}
		out = append(out, e)
	}
	return out
}

// writeGlanceBucket renders one fixed bucket: its heading, its count, and
// either its entries or dc-4's explicit empty-state notice — mirroring
// directory.go's own "None." convention for an empty group rather than
// inventing a second "nothing here" vocabulary. The bucket's own heading,
// count, and data-testid always render, regardless of population (dc-4,
// ac-3): never a silently omitted bucket.
func writeGlanceBucket(buf *bytes.Buffer, root string, b glanceBucket, members []refindex.Entry) {
	buf.WriteString(`<section class="glance-group" data-testid="glance-group-`)
	buf.WriteString(b.slug)
	buf.WriteString(`"><h3>`)
	buf.WriteString(stdhtml.EscapeString(b.label))
	buf.WriteString(` <span class="count">(`)
	buf.WriteString(strconv.Itoa(len(members)))
	buf.WriteString(`)</span></h3>`)
	if len(members) == 0 {
		buf.WriteString(`<p class="empty">None.</p></section>`)
		return
	}
	buf.WriteString(`<ul>`)
	for _, e := range members {
		writeGlanceEntry(buf, root, e)
	}
	buf.WriteString(`</ul></section>`)
}

// writeGlanceEntry renders one glance card: title (linked exactly as its
// source already links it today, dc-3), its raw status badge (the shared
// writeStatusChip vocabulary), and its working links only — never a
// source chip, an in-review chip, or receipts/gate state (dc-3's
// leaner-than-the-exhaustive-section bar).
func writeGlanceEntry(buf *bytes.Buffer, root string, e refindex.Entry) {
	name := strings.TrimPrefix(e.Ref, "spec/")

	buf.WriteString(`<li class="glance-entry" data-testid="glance-entry-`)
	buf.WriteString(stdhtml.EscapeString(name))
	buf.WriteString(`">`)

	if e.Source == refindex.SourceDefault {
		writeGlanceDefaultEntry(buf, root, e, name)
	} else {
		writeGlanceDesignEntry(buf, e, name)
	}
	buf.WriteString(`</li>`)
}

// writeGlanceDefaultEntry renders a default-branch glance card: title
// linked to its corpus page, status badge, and — only when the active-zone
// working tree actually carries the file (specWorkingTreeMeta, mirrored
// verbatim from directory.go's writeDefaultEntry, never re-derived) — its
// board link, plus matrix/verdict for a class:feature entry with a
// non-empty story field. Two distinct truth sources are in play here
// (dc-3, ADJ-35): the glance's own population already read the
// DEFAULT-BRANCH tree (this entry would not be here at all if it were
// archive-zone); specWorkingTreeMeta below is a SEPARATE, serving-
// working-tree check. In the residual case where they diverge — a
// glance-admitted active-zone entry whose file is absent from the serving
// checkout — the board link (and therefore matrix/verdict, which nest
// inside the same boardServable gate) is honestly withheld, exactly as
// the exhaustive section already degrades: a link that cannot work does
// not exist to give, never a broken link.
func writeGlanceDefaultEntry(buf *bytes.Buffer, root string, e refindex.Entry, name string) {
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

	if boardServable {
		buf.WriteString(` &middot; <a class="glance-board" href="`)
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

// writeGlanceDesignEntry renders a design-branch glance card: the entry's
// title IS its one link, to the per-branch board address (dc-3, built
// through directory.go's shared designBoardHref constructor — the same call
// writeDesignEntry makes, never a second grammar), plus its status badge.
// Never matrix/verdict (a still-drafting feature carries no built evidence
// for either to show — dc-3, ADJ-32 f3 rejected); never an in-review chip
// (dc-3's evidence-bearing-state bar).
func writeGlanceDesignEntry(buf *bytes.Buffer, e refindex.Entry, name string) {
	buf.WriteString(`<a href="`)
	buf.WriteString(stdhtml.EscapeString(designBoardHref(name)))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(e.Ref))
	buf.WriteString(`</a> `)
	writeStatusChip(buf, e.SpecStatus)
}
