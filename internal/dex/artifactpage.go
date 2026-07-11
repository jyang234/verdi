package dex

import (
	"context"
	"fmt"
	"html/template"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/index"
)

// writeArtifactPage renders and writes one committed-zone permalink page
// (05 §Verdi-dex mechanics: "/a/<kind>/<name>").
func writeArtifactPage(ctx context.Context, outDir, root, buildCommit string, stamp buildStamp, ix *index.Index, known map[string]bool, lens *lensData, p *artifactPage) error {
	bodyHTML, err := renderBody(p.Entry.Kind, p.Entry.Body)
	if err != nil {
		return fmt.Errorf("dex: rendering %s: %w", p.Entry.Ref, err)
	}

	frozen := p.Meta.Base.Frozen
	class := classify(p.Entry.Kind, frozen != nil)

	var banner string
	switch class {
	case classFrozen:
		banner = frozenBanner(frozen.At, frozen.Commit)
	case classAuthoredLiving:
		c, ok, lerr := gitx.LastCommit(ctx, root, buildCommit, p.RelPath)
		if lerr != nil {
			return fmt.Errorf("dex: last-modified for %s: %w", p.Entry.Ref, lerr)
		}
		if ok {
			banner = authoredLivingBanner(c)
		} else {
			banner = noHistoryBanner
		}
	default: // classLivingGated: unreachable for a committed-zone entry, kept for totality.
		banner = livingGatedBanner(stamp)
	}

	pinCommit := stamp.SHA
	if frozen != nil {
		pinCommit = frozen.Commit
	}

	// The v2 lens surfaces (V1-P8): ladder badges + disclosure rows on
	// story pages, the paired stub/live-mapping section on round-four
	// feature pages, and the ADR page's link to its exemption page.
	ladder := storyLadder(lens, p)
	connections := allConnections(ix, p.Entry.Ref, p.Entry.Links, known)
	if c := adrExemptionsConnection(p, lens.exemptions); c != nil {
		connections = append(connections, *c)
	}

	data := pageData{
		Title:            p.Entry.Title,
		Status:           p.Entry.Status,
		LadderBadges:     ladder.Badges,
		Breadcrumb:       pageBreadcrumb(p.Entry.Kind, p.Entry.Title, isArchivedSpec(p.RelPath)),
		Banner:           banner,
		BannerClass:      bannerClass(class),
		MetaRows:         append(artifactMetaRows(p), ladder.Rows...),
		BodyHTML:         template.HTML(bodyHTML),
		DispositionsHTML: renderDispositionsTable(p.Meta.Dispositions),
		FeatureLensHTML:  featureLensHTML(ix, known, p),
		Connections:      connections,
		TOC:              extractTOC(bodyHTML),
		CopyRef:          p.Entry.Ref + "@" + pinCommit,
	}
	out, err := renderPage(data)
	if err != nil {
		return err
	}
	return writeFile(outDir, permalinkOutPath(p.Entry.Ref), out)
}

// artifactMetaRows builds the metadata card (05 §Verdi-dex page anatomy:
// "owners, decided/frozen, supersession links, provenance path").
func artifactMetaRows(p *artifactPage) []metaRow {
	var rows []metaRow
	if len(p.Meta.Base.Owners) > 0 {
		rows = append(rows, metaRow{Label: "Owners", Value: strings.Join(p.Meta.Base.Owners, ", ")})
	}
	if p.Meta.Class != "" {
		rows = append(rows, metaRow{Label: "Class", Value: string(p.Meta.Class)})
	}
	if p.Meta.Story != "" {
		rows = append(rows, metaRow{Label: "Story", Value: p.Meta.Story})
	}
	if p.Meta.Decided != "" {
		rows = append(rows, metaRow{Label: "Decided", Value: p.Meta.Decided})
	}
	if p.Meta.Base.Frozen != nil {
		rows = append(rows, metaRow{Label: "Frozen", Value: fmt.Sprintf("%s @ %s", p.Meta.Base.Frozen.At, shortSHA(p.Meta.Base.Frozen.Commit))})
	}
	if p.Meta.Reason != "" {
		rows = append(rows, metaRow{Label: "Reason", Value: p.Meta.Reason})
	}
	if p.Meta.Expiry != "" {
		rows = append(rows, metaRow{Label: "Expiry", Value: p.Meta.Expiry})
	}
	if supersedes := linkRefs(p.Entry.Links, "supersedes"); len(supersedes) > 0 {
		rows = append(rows, metaRow{Label: "Supersedes", Value: strings.Join(supersedes, ", ")})
	}
	if p.Meta.Base.Provenance != nil {
		rows = append(rows, metaRow{Label: "Provenance", Value: fmt.Sprintf("%s v%s", p.Meta.Base.Provenance.Generator, p.Meta.Base.Provenance.Version)})
	}
	rows = append(rows, metaRow{Label: "Source", Value: p.RelPath})
	return rows
}

// linkRefs returns the Ref of every link of the given type, in frontmatter
// order.
func linkRefs(links []artifact.Link, linkType string) []string {
	var refs []string
	for _, l := range links {
		if string(l.Type) == linkType {
			refs = append(refs, l.Ref)
		}
	}
	return refs
}
