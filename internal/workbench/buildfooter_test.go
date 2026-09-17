package workbench

import (
	"go/ast"
	"go/parser"
	"go/token"
	stdhtml "html"
	"html/template"
	"os"
	"strconv"
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

// TestBuildIdentificationFooter_EveryShellTemplateCarriesIt is the drift
// guard the wave-1 whole-wave review asked for (2026-09-16 ledger, minor
// 3; it mirrors cmd/verdi's TestVerbUsageRegistry_CoversEveryVerb).
// TestBuildIdentificationFooter above enumerates the four shells BY HAND,
// so a fifth <!doctype html>-emitting template added later would ship
// footerless while that test still passed — a content-free failure no
// one skimming test output would notice. This test derives the shell set
// mechanically instead: it parses every non-test Go source in this
// package, collects every string literal that emits a document shell
// (contains "<!doctype html>" or "<html", case-insensitive), and requires
// each to (a) contain the {{buildFooter}} action and (b) parse as an
// html/template with shellFuncs, the one seam that supplies it. A shell
// assembled outside that seam (a builder writing "<!doctype html>" by
// hand) fails (a) by construction — the ratchet's intent. The floor of
// four pins the scan itself: finding fewer shells than the known set
// means the scan broke, never that the shells are fine.
func TestBuildIdentificationFooter_EveryShellTemplateCarriesIt(t *testing.T) {
	const knownShells = 4 // layout.go, board.go, boardspecrender.go, boarddiagramrender.go
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package sources: %v", err)
	}
	type shell struct {
		at  string
		src string
	}
	var shells []shell
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				s, uerr := strconv.Unquote(lit.Value)
				if uerr != nil {
					t.Fatalf("%s: unquote string literal: %v", fset.Position(lit.Pos()), uerr)
				}
				low := strings.ToLower(s)
				if strings.Contains(low, "<!doctype html>") || strings.Contains(low, "<html") {
					shells = append(shells, shell{at: fset.Position(lit.Pos()).String(), src: s})
				}
				return true
			})
		}
	}
	if len(shells) < knownShells {
		t.Fatalf("found %d document-shell literals, want at least the %d known shells — the scan is broken, not the shells", len(shells), knownShells)
	}
	for _, sh := range shells {
		t.Run(sh.at, func(t *testing.T) {
			if !strings.Contains(sh.src, "{{buildFooter}}") {
				t.Fatalf("shell template at %s emits a document (<!doctype html>/<html>) but carries no {{buildFooter}} action — it would ship without the build-identification footer (spec/uat-round-1 ac-1); add {{buildFooter}} inside its <body> and parse it with shellFuncs", sh.at)
			}
			if _, perr := template.New("shell").Funcs(shellFuncs).Parse(sh.src); perr != nil {
				t.Fatalf("shell template at %s does not parse with shellFuncs: %v", sh.at, perr)
			}
		})
	}
}
