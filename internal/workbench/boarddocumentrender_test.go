package workbench

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specdoc"
)

// The Document page's chrome (spec/document-page-v2 ac-1, ac-4; ledger
// SI-340 (2)–(7), (10), (13); SI-343): the temporal stamp, the identity
// card, and the contents rail are server-rendered from the page's facts
// around the unchanged body, the chip anchors ride a JSON script for the
// page's script to draw, and no chip is ever rendered here.

// renderedDocumentPage loads name's Document page through s and renders
// it under its root-mount path.
func renderedDocumentPage(t *testing.T, s *boardSpecServer, name string) (string, documentSnapshot) {
	t.Helper()
	snap, res, err := s.loadDocumentPage(t.Context(), name, specdoc.KindSpec)
	if err != nil {
		t.Fatalf("loadDocumentPage: %v", err)
	}
	bar := s.documentBarFacts(t.Context(), name, res, snap.checkout)
	page, err := renderBoardDocumentPage(t.Context(), "/board/spec/"+name+"/document", name, snap, bar)
	if err != nil {
		t.Fatalf("renderBoardDocumentPage: %v", err)
	}
	return string(page), snap
}

// elementRe matches the whole element carrying the test id, up to its
// first closing tag of the same name (no nesting of the same tag inside
// the chrome's cells).
func elementByTestID(t *testing.T, page, id string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<([a-z0-9]+)[^>]*\sdata-testid="` + regexp.QuoteMeta(id) + `"[^>]*>.*?</`)
	m := re.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("the page has no element data-testid=%q\n%s", id, page)
	}
	// Extend to the matching close tag of the opening element.
	start := strings.Index(page, m[0])
	rest := page[start:]
	end := strings.Index(rest, "</"+m[1]+">")
	if end < 0 {
		t.Fatalf("element %q never closes", id)
	}
	return rest[:end+len("</"+m[1]+">")]
}

// textOf strips tags from a fragment and unescapes nothing: the chrome's
// cells carry plain words, which is what the eye reads.
func textOf(fragment string) string {
	return strings.TrimSpace(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(fragment, ""))
}

func TestDocumentPage_RendersTheChrome(t *testing.T) {
	for _, tc := range []struct {
		name   string
		spec   string
		server func(*testing.T) *boardSpecServer
		state  string
		words  string
	}{
		{name: "the accepted reading", spec: documentWallName, server: func(t *testing.T) *boardSpecServer {
			_, repo, _ := newAcceptedWallFixture(t)
			return &boardSpecServer{root: repo.Dir}
		}, state: documentStateAccepted, words: documentStateAccepted},
		{name: "a proposal on its design branch", spec: boardFixtureName, server: func(t *testing.T) *boardSpecServer {
			return &boardSpecServer{root: newBarFixture(t)}
		}, state: documentStateProposed, words: documentProposedWords},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, snap := renderedDocumentPage(t, tc.server(t), tc.spec)
			f := snap.Facts

			// The temporal stamp (SI-340 (2), (6)): its state as a token
			// class and a data attribute, the state words, the commit in
			// full as data and short to the eye, and an empty refreshed
			// slot the browser fills (dc-2).
			stamp := elementByTestID(t, page, "document-stamp")
			for _, want := range []string{
				`id="document-stamp"`,
				`class="document-stamp document-stamp--` + tc.state + `"`,
				`data-state="` + tc.state + `"`,
				`data-commit="` + f.Stamp.Commit + `"`,
				`data-testid="document-stamp-words">` + tc.words + `<`,
				`data-testid="document-stamp-commit" title="` + f.Stamp.Commit + `">` + shortCommit(f.Stamp.Commit) + `<`,
				`id="document-refreshed" class="document-stamp-refreshed" data-testid="document-refreshed"></span>`,
			} {
				if !strings.Contains(stamp, want) {
					t.Errorf("stamp lacks %q\n%s", want, stamp)
				}
			}
			if f.Stamp.State != tc.state || len(f.Stamp.Commit) != 40 {
				t.Fatalf("the fixture's stamp is %+v, want %s at a full commit", f.Stamp, tc.state)
			}

			// The identity card (SI-340 (4)): ref, class, branch, owners,
			// and the files behind the spec, each in its own cell.
			if got := textOf(elementByTestID(t, page, "document-identity-ref")); got != f.Identity.Ref {
				t.Errorf("ref cell %q, want %q", got, f.Identity.Ref)
			}
			if got := textOf(elementByTestID(t, page, "document-identity-class")); got != f.Identity.ClassLabel {
				t.Errorf("class cell %q, want %q", got, f.Identity.ClassLabel)
			}
			branch := elementByTestID(t, page, "document-identity-branch")
			if !strings.Contains(branch, `data-state="proven"`) || textOf(branch) != f.Identity.Branch.Text {
				t.Errorf("branch cell %s, want proven %q", branch, f.Identity.Branch.Text)
			}
			if got := textOf(elementByTestID(t, page, "document-identity-owners")); got != strings.Join(f.Identity.Owners, ", ") {
				t.Errorf("owners cell %q, want %q", got, strings.Join(f.Identity.Owners, ", "))
			}
			files := elementByTestID(t, page, "document-identity-files")
			for _, file := range f.Identity.Files {
				if !strings.Contains(files, "<code>"+file+"</code>") {
					t.Errorf("files cell lacks %q: %s", file, files)
				}
			}

			// The contents rail (SI-340 (7); SI-343 (1)): the body's h2
			// sections in order, each linking its heading id, counted
			// exactly where the facts count it.
			rail := elementByTestID(t, page, "document-contents")
			last := -1
			counted := 0
			for _, e := range f.Rail {
				entry := elementByTestID(t, rail, "document-contents-"+e.ID)
				at := strings.Index(rail, entry)
				if at <= last {
					t.Errorf("rail entry %q is out of order", e.ID)
				}
				last = at
				if !strings.Contains(entry, `href="#`+e.ID+`"`) || !strings.Contains(entry, `<span class="document-contents-text">`+e.Text+`</span>`) {
					t.Errorf("rail entry %q lacks its link or text: %s", e.ID, entry)
				}
				hasCount := strings.Contains(entry, `data-testid="document-contents-`+e.ID+`-count"`)
				if hasCount != (e.Count != nil) {
					t.Errorf("rail entry %q counted=%v, want %v: %s", e.ID, hasCount, e.Count != nil, entry)
				}
				if e.Count != nil {
					counted++
					if got := textOf(elementByTestID(t, entry, "document-contents-"+e.ID+"-count")); got != strconv.Itoa(*e.Count) {
						t.Errorf("rail entry %q count %q, want %d", e.ID, got, *e.Count)
					}
				}
			}
			if counted == 0 || len(f.Rail) == 0 {
				t.Fatalf("the fixture's rail must carry entries and a count: %+v", f.Rail)
			}

			// The chip anchors ride a JSON script (SI-340 (10); dc-1), the
			// wall's href rides the region, and no chip is rendered here.
			chips := decodeChipsScript(t, page)
			if !reflect.DeepEqual(chips, f.Chips) || len(chips) == 0 {
				t.Errorf("chips script %+v, want the facts' %+v", chips, f.Chips)
			}
			if strings.Contains(page, `class="document-chip`) {
				t.Errorf("the server rendered a chip")
			}
			if !strings.Contains(page, `data-board-href="/board/spec/`+tc.spec+`"`) {
				t.Errorf("the region lacks the wall's href\n%s", page)
			}

			// The scripts, in order: the document script, then the chrome's.
			if !strings.Contains(page, `<script src="/assets/specdocument.js"></script>`+"\n"+`<script src="/assets/documentpage.js"></script>`) {
				t.Errorf("the page lacks the two scripts in order\n%s", page)
			}

			// Copy and Download sit in the bar's controls slot (SI-340 (13));
			// Refresh and the status line stay in the page head; the body's
			// own not-authority footer stays.
			controls := elementByTestID(t, page, "topbar-controls")
			for _, id := range []string{`id="document-copy"`, `id="document-download"`} {
				if !strings.Contains(controls, id) {
					t.Errorf("the bar's controls lack %s: %s", id, controls)
				}
			}
			head := page[strings.Index(page, `<header class="document-head">`):strings.Index(page, `</header>`+"\n"+`<div class="document-layout">`)]
			if !strings.Contains(head, `id="document-refresh"`) || !strings.Contains(head, `id="document-status"`) || strings.Contains(head, `id="document-copy"`) {
				t.Errorf("the page head must keep Refresh and the status line and not Copy: %s", head)
			}
			if !strings.Contains(page, "not authority") {
				t.Errorf("the body's not-authority footer is gone")
			}
		})
	}
}

// TestDocumentPage_ChromeEdges: the chrome's edge states — a branch that
// could not be read, a detached HEAD, a spec without a class or owners, a
// section counted at zero, and no facts at all — each render as their
// own words, escaped, and never panic.
func TestDocumentPage_ChromeEdges(t *testing.T) {
	zero := 0
	facts := func(mut func(*documentPageFacts)) documentPageFacts {
		f := documentPageFacts{
			Stamp:    documentStamp{State: documentStateProposed, Words: documentProposedWords, Commit: strings.Repeat("a", 40)},
			Identity: documentIdentity{Ref: "spec/x", ClassLabel: "feature", Branch: provenFact("main"), Owners: []string{"a-team", "b-team"}, Files: []string{".verdi/specs/active/x/spec.md"}},
			Rail:     []documentRailEntry{{ID: "identity", Text: "Identity"}, {ID: "decisions", Text: "Decisions", Count: &zero}},
			Chips:    []documentChip{},
		}
		mut(&f)
		return f
	}
	for _, tc := range []struct {
		name  string
		facts documentPageFacts
		want  []string
		never []string
	}{
		{name: "an unproven branch, escaped", facts: facts(func(f *documentPageFacts) {
			f.Identity.Branch = unprovenFact(`git <exploded> "quietly"`)
		}), want: []string{
			`data-testid="document-identity-branch" data-state="unproven" title="git &lt;exploded&gt; &#34;quietly&#34;">unproven<span class="document-identity-why">git &lt;exploded&gt; &#34;quietly&#34;</span></dd>`,
		}, never: []string{`<exploded>`}},
		{name: "a detached HEAD", facts: facts(func(f *documentPageFacts) {
			f.Identity.Branch, f.Identity.Detached = provenFact(""), true
		}), want: []string{`data-testid="document-identity-branch" data-state="proven" data-detached="true">detached HEAD</dd>`}},
		{name: "no class and no owners", facts: facts(func(f *documentPageFacts) {
			f.Identity.ClassLabel, f.Identity.Owners = "", nil
		}), want: []string{
			`data-testid="document-identity-class" data-state="none">not declared</dd>`,
			`data-testid="document-identity-owners" data-state="none">none declared</dd>`,
		}},
		{name: "a section counted at zero", facts: facts(func(*documentPageFacts) {}), want: []string{
			`<li data-testid="document-contents-decisions" data-count="0"><a href="#decisions"><span class="document-contents-text">Decisions</span><span class="document-contents-count" data-testid="document-contents-decisions-count">0</span></a></li>`,
			`<li data-testid="document-contents-identity"><a href="#identity"><span class="document-contents-text">Identity</span></a></li>`,
		}},
		{name: "an owner and a chip id, escaped", facts: facts(func(f *documentPageFacts) {
			f.Identity.Owners = []string{`<b>x</b>`}
			f.Chips = []documentChip{{ID: `</script><script>`, Kind: "decision"}}
		}), want: []string{`&lt;b&gt;x&lt;/b&gt;`}, never: []string{`<b>x</b>`, `</script><script>`}},
		{name: "the accepted token", facts: facts(func(f *documentPageFacts) {
			f.Stamp = documentStamp{State: documentStateAccepted, Words: documentStateAccepted, Commit: "abc"}
		}), want: []string{`class="document-stamp document-stamp--accepted"`, `data-testid="document-stamp-commit" title="abc">abc<`}},
		{name: "no facts", facts: documentPageFacts{}, want: []string{
			`id="document-contents-list" class="document-contents-list"></ol>`,
			`data-testid="document-identity-files"></dd>`,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap := documentSnapshot{Kind: "spec", Revision: "r", HTML: "<h1>X</h1>", Markdown: "# X", Facts: tc.facts}
			page, err := renderBoardDocumentPage(t.Context(), "/board/spec/x/document", "x", snap, barFacts{Title: "X"})
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			// The chips script round-trips the facts' chips exactly, the
			// escaped id included, and no facts gives JSON null.
			if got := decodeChipsScript(t, string(page)); !reflect.DeepEqual(got, tc.facts.Chips) {
				t.Errorf("chips script decodes to %#v, want %#v", got, tc.facts.Chips)
			}
			for _, w := range tc.want {
				if !strings.Contains(string(page), w) {
					t.Errorf("page lacks %q\n%s", w, page)
				}
			}
			for _, n := range tc.never {
				if strings.Contains(string(page), n) {
					t.Errorf("page carries %q unescaped\n%s", n, page)
				}
			}
		})
	}
}

// decodeChipsScript decodes the page's chips JSON script (SI-340 (10)).
func decodeChipsScript(t *testing.T, page string) []documentChip {
	t.Helper()
	m := regexp.MustCompile(`<script type="application/json" id="document-chips">(.*?)</script>`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("no chips script\n%s", page)
	}
	var chips []documentChip
	if err := json.Unmarshal([]byte(m[1]), &chips); err != nil {
		t.Fatalf("chips script is not JSON: %v: %s", err, m[1])
	}
	return chips
}

func TestShortCommit(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"abc", "abc"},
		{"abcdefgh", "abcdefgh"},
		{strings.Repeat("0123456789", 4), "01234567"},
	} {
		if got := shortCommit(tc.in); got != tc.want {
			t.Errorf("shortCommit(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestDocumentPageAssetBudget: the chrome's own script is a new asset of
// at most 64 KiB (Wave 6 §5.3; spec/workbench-redesign co-1), served at
// its route; the document script is untouched in size class, and
// boardspec.js did not grow for it.
func TestDocumentPageAssetBudget(t *testing.T) {
	const ceiling = 64 * 1024
	data, err := embeddedAssets.ReadFile("assets/documentpage.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > ceiling || len(data) == 0 {
		t.Fatalf("assets/documentpage.js is %d bytes; want 1..%d", len(data), ceiling)
	}
	rec := httptest.NewRecorder()
	NewHandler(t.TempDir()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/documentpage.js", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") || rec.Body.String() != string(data) {
		t.Fatalf("asset route: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = httptest.NewRecorder()
	NewHandler(t.TempDir()).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/assets/documentpage.js", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST on the asset: %d, want 405", rec.Code)
	}
}
