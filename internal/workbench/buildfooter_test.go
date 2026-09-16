package workbench

import (
	stdhtml "html"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/buildinfo"
)

// TestBuildIdentificationFooter (spec/uat-round-1 ac-1: "the workbench
// renders it in its page footer"): EVERY workbench shell — the shared
// read-page shell (layout.go), the v0 board, the v1 board and the
// diagram editor, each of which renders its own <html> — carries exactly
// one footer element whose text is exactly internal/buildinfo.Line(),
// the same string `verdi version` prints and `verdi serve` logs, so the
// three surfaces can never disagree (dc-8: from a worktree build this is
// the honest "verdi (devel)", never a fabricated version).
func TestBuildIdentificationFooter(t *testing.T) {
	want := buildinfo.Line()
	if want == "" {
		t.Fatal("buildinfo.Line() returned an empty string; the footer would render nothing")
	}
	cases := []struct {
		name   string
		render func() ([]byte, error)
	}{
		{
			name: "shared read-page shell",
			render: func() ([]byte, error) {
				return renderPage(pageData{Title: "T", BodyHTML: "<p>body</p>"})
			},
		},
		{
			name: "v0 board page",
			render: func() ([]byte, error) {
				return renderBoardPage(boardClientState{Key: "STORY-1"}, classWords{})
			},
		},
		{
			name: "v1 board page",
			render: func() ([]byte, error) {
				return renderBoardSpecPage(&BoardProjection{Spec: "s", Title: "S", Mode: modeReadOnly, Status: "draft"}, &boardGitState{}, testASDView())
			},
		},
		{
			name: "diagram editor page",
			render: func() ([]byte, error) {
				return renderDiagramEditorPage(&diagramEditorView{Name: "d", Status: "proposed", Mode: modeReadOnly, Raw: []byte("flowchart TD\n"), Body: []byte("flowchart TD\n")})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.render()
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			html := string(out)
			got, n := testIDElementText(html, "build-identification")
			if n != 1 {
				t.Fatalf("build-identification elements = %d, want exactly 1; page: %s", n, html)
			}
			if got != want {
				t.Fatalf("footer text = %q, want exactly buildinfo.Line() = %q", got, want)
			}
			if !strings.Contains(html, stdhtml.EscapeString(want)) {
				t.Fatalf("footer must carry the HTML-escaped line %q", stdhtml.EscapeString(want))
			}
			// The footer is page chrome inside the body, after the page's
			// own content — never in <head>, never after </body>.
			footerAt := strings.Index(html, `data-testid="build-identification"`)
			bodyEnd := strings.LastIndex(html, "</body>")
			bodyStart := strings.Index(html, "<body")
			if bodyStart < 0 || bodyEnd < 0 || footerAt < bodyStart || footerAt > bodyEnd {
				t.Fatalf("footer at %d is outside <body> (%d..%d)", footerAt, bodyStart, bodyEnd)
			}
			if strings.Count(html, "<footer") != 1 {
				t.Fatalf("<footer> elements = %d, want exactly 1", strings.Count(html, "<footer"))
			}
		})
	}
}
