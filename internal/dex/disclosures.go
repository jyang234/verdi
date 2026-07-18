// The dex's read-only disclosures edition (spec/disclosures-panel ac-3;
// 05 §Lenses: "the dex ships their read-only, main-only editions,
// computed the same way — no separate logic path"): the same
// internal/disclosureview enumeration and item markup the workbench's
// /disclosures page serves live, baked at build time under the dex's own
// temporal honesty (a living-gated build stamp).
//
// Disclosed input note: the enumeration is a function of the CHECKOUT's
// state (notably the never-committed mutable zone's presence, 01 §Zones),
// not of the committed tree alone — the second build input, after
// Options.Forge, that sits outside "a pure function of main's tree". In
// the dex's own CI home (a bare pipeline clone) that state is fixed, so
// published sites stay deterministic; a local build honestly reflects the
// local checkout instead. Recorded in the round-5 divergence log rather
// than silently absorbed.
package dex

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/disclosureview"
	"github.com/jyang234/verdi/internal/model"
)

// dexDisclosuresNote is this edition's compute-provenance line — the one
// piece of chrome that legitimately differs from the workbench edition,
// because the two editions' temporal claims differ (build stamp vs. live
// render).
const dexDisclosuresNote = "Enumerated from the checkout's state at this page's build stamp. For the live edition, open /disclosures on a running `verdi serve`. An entry here is a claim verdi is honestly not proving, not a failure."

// writeDisclosuresPage emits /disclosures/ through the shared view.
func writeDisclosuresPage(ctx context.Context, root, outDir string, stamp buildStamp, mdl *model.Model) error {
	items, err := disclosureview.Current(ctx, root)
	if err != nil {
		return fmt.Errorf("dex: enumerating disclosures: %w", err)
	}
	out, err := renderPage(mdl, pageData{
		Title:      "Disclosures",
		Breadcrumb: []breadcrumbEntry{{Label: "Home", URL: "/"}, {Label: "Disclosures", URL: ""}},
		Banner:     livingGatedBanner(stamp),
		BodyHTML:   disclosureview.HTML(items, dexDisclosuresNote),
	})
	if err != nil {
		return err
	}
	return writeFile(outDir, "disclosures/index.html", out)
}
