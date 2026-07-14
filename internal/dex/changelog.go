package dex

import (
	"context"
	"fmt"
	"html/template"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
)

// writeChangelog emits dex's "what changed" feed (05 §Verdi-dex mechanics:
// "each publish emits a 'what changed' feed from the git log of .verdi/"):
// every commit reachable from buildCommit that touched .verdi/, most
// recent first.
func writeChangelog(ctx context.Context, root, outDir string, stamp buildStamp, buildCommit string) error {
	commits, err := gitx.Log(ctx, root, buildCommit, ".verdi")
	if err != nil {
		return fmt.Errorf("dex: changelog: %w", err)
	}

	var b strings.Builder
	if len(commits) == 0 {
		b.WriteString("<p>No commits touching .verdi/ yet.</p>\n")
	} else {
		b.WriteString(`<ul class="entry-list">` + "\n")
		for _, c := range commits {
			fmt.Fprintf(&b, "<li><code>%s</code> · %s · %s — %s</li>\n",
				template.HTMLEscapeString(shortSHA(c.SHA)),
				template.HTMLEscapeString(dateOnly(c.Date)),
				template.HTMLEscapeString(c.Author),
				template.HTMLEscapeString(c.Subject))
		}
		b.WriteString("</ul>\n")
	}

	data := pageData{
		Title:      "What changed",
		Breadcrumb: []breadcrumbEntry{{Label: "Home", URL: "/"}, {Label: "What changed", URL: ""}},
		Banner:     livingGatedBanner(stamp),
		BodyHTML:   template.HTML(b.String()),
	}
	out, err := renderPage(data)
	if err != nil {
		return err
	}
	return writeFile(outDir, "changelog/index.html", out)
}
