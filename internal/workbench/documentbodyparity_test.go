package workbench_test

// The Document page's body parity (spec/document-page-v2 ac-3, obligation
// document-page-v2--ac-3--static; ledger SI-340 (14), (15)). An external
// test package, because internal/mcpserve — one of the renders compared —
// imports internal/workbench. The built-binary parity of all four renders
// stays with cmd/verdi/document_parity_e2e_test.go; this test reaches the
// CLI leg through the shared loader the CLI calls, with the CLI's own
// request.

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/dex"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/workbench"
)

// regionIDRe matches an id attribute naming the body's region (never a
// data-testid, whose name only ends in "id").
var regionIDRe = regexp.MustCompile(`\sid="document-region"`)

// regionOpenRe matches the start tag of the element that holds the shared
// body, whatever its tag name and attribute order.
var regionOpenRe = regexp.MustCompile(`<([a-z][a-z0-9]*)\s[^>]*?\bid="document-region"[^>]*>`)

// sourceOpenRe matches the start tag of the hidden Markdown source.
var sourceOpenRe = regexp.MustCompile(`<pre\s[^>]*?\bid="document-markdown"[^>]*>`)

// regionParity requires the page's #document-region to hold exactly want,
// the shared load's HTML: want is the region's start, byte for byte, and
// the region's end tag follows at once. No byte is normalized on either
// side.
func regionParity(page, want string) error {
	if n := len(regionIDRe.FindAllStringIndex(page, -1)); n != 1 {
		return fmt.Errorf("the page carries %d #document-region elements, want 1", n)
	}
	m := regionOpenRe.FindStringSubmatchIndex(page)
	if m == nil {
		return fmt.Errorf("no #document-region start tag")
	}
	rest := page[m[1]:]
	end := "</" + page[m[2]:m[3]] + ">"
	if !strings.HasPrefix(rest, want) {
		return fmt.Errorf("#document-region differs from the shared load's HTML at byte %d", firstDifference(rest, want))
	}
	if !strings.HasPrefix(rest[len(want):], end) {
		return fmt.Errorf("#document-region carries bytes after the shared load's HTML, before %s: %.40q", end, rest[len(want):])
	}
	return nil
}

// hiddenMarkdown is the page's hidden Markdown source as the HTML parser
// reads it, which is what Copy hands over: the one newline right after
// the <pre> start tag dropped, then character references decoded.
func hiddenMarkdown(page string) (string, error) {
	m := sourceOpenRe.FindStringIndex(page)
	if m == nil {
		return "", fmt.Errorf("no #document-markdown source")
	}
	rest := page[m[1]:]
	end := strings.Index(rest, "</pre>")
	if end < 0 {
		return "", fmt.Errorf("#document-markdown is never closed")
	}
	return html.UnescapeString(strings.TrimPrefix(rest[:end], "\n")), nil
}

// markdownParity requires every page-side Markdown to equal every other
// render's, byte for byte.
func markdownParity(page, legs map[string]string) error {
	for _, p := range sortedKeys(page) {
		for _, l := range sortedKeys(legs) {
			if page[p] != legs[l] {
				return fmt.Errorf("the page's %s differs from %s at byte %d", p, l, firstDifference(page[p], legs[l]))
			}
		}
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func firstDifference(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func get(t *testing.T, h http.Handler, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// otherRenders is spec name's Markdown as the three other renders carry it
// with their shells stripped: verdi spec doc --no-readiness (the CLI's own
// request to the shared loader: the accepted reading, the store's model,
// no readiness), MCP get_document's markdown field, and the docs site's
// spec.md.
func otherRenders(t *testing.T, ctx context.Context, root, site string, mdl *model.Model, name string) map[string]string {
	t.Helper()
	res, err := specdocload.Load(ctx, specdocload.Request{Root: root, Name: name, Mode: specdocload.ModeAccepted, Kind: specdoc.KindSpec, Model: mdl})
	if err != nil {
		t.Fatalf("the CLI's load: %v", err)
	}
	doc, err := specdoc.Build(res.Input)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal((&mcpserve.Backend{Root: root}).GetDocument(ctx, json.RawMessage(`{"ref":"spec/`+name+`"}`)))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || result.IsError || len(result.Content) != 1 {
		t.Fatalf("MCP get_document: %v\n%s", err, raw)
	}
	var mcpDoc struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &mcpDoc); err != nil {
		t.Fatal(err)
	}

	dexBytes, err := os.ReadFile(filepath.Join(site, "a", "spec", name, "spec.md"))
	if err != nil {
		t.Fatalf("the docs site's spec.md: %v", err)
	}
	return map[string]string{
		"verdi spec doc":   specdoc.RenderMarkdown(doc),
		"MCP get_document": mcpDoc.Markdown,
		"the docs site":    string(dexBytes),
	}
}

// sharedHTML is the shared load's HTML for the page's own resolution: the
// root mount's request (the serving checkout's working tree, its corpus
// links servable, no readiness loader wired).
func sharedHTML(t *testing.T, ctx context.Context, root string, mdl *model.Model, name string) string {
	t.Helper()
	res, err := specdocload.Load(ctx, specdocload.Request{Root: root, Name: name, Mode: specdocload.ModeWorkingTree, Kind: specdoc.KindSpec, Model: mdl})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := specdoc.Build(res.Input)
	if err != nil {
		t.Fatal(err)
	}
	out, err := specdoc.RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// storeSpecs lists every spec the store holds, in either zone.
func storeSpecs(t *testing.T, root string) []string {
	t.Helper()
	var names []string
	for _, zone := range []string{store.ZoneActive, store.ZoneArchive} {
		entries, err := os.ReadDir(filepath.Join(root, ".verdi", "specs", zone))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if _, err := os.Stat(store.SpecPath(root, zone, e.Name())); err == nil {
				names = append(names, e.Name())
			}
		}
	}
	sort.Strings(names)
	return names
}

// TestDocumentPage_BodyByteParity: over the accepted scenario store's
// root mount, at clean accepted bytes, and for every spec it holds —
// including the closed-spec object supersession lines — the Document
// page's Markdown (its hidden source, the ?format=md download, and the
// snapshot's markdown) equals what verdi spec doc, MCP get_document, and
// the docs site carry with their shells stripped, and its
// #document-region (and the snapshot's html) is the shared load's HTML for
// the same resolution, written unmodified. Each half fails on a one-byte
// change to either side.
func TestDocumentPage_BodyByteParity(t *testing.T) {
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME"} {
		t.Setenv(key, "")
	}
	ctx := t.Context()
	repo := scenario.Build(t, "accepted")
	if status, err := gitStatus(ctx, repo.Dir); err != nil || status != "" {
		t.Fatalf("the fixture must be clean accepted bytes: %v %q", err, status)
	}
	cfg, err := store.Open(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	site := t.TempDir()
	if err := dex.Build(ctx, dex.Options{Root: repo.Dir, OutDir: site, Commit: "main"}); err != nil {
		t.Fatal(err)
	}
	h := workbench.NewHandlerWith(repo.Dir, workbench.Deps{})

	specs := storeSpecs(t, repo.Dir)
	if len(specs) < 2 {
		t.Fatalf("the fixture store holds %v, want several specs", specs)
	}
	superseding := 0
	for _, name := range specs {
		t.Run(name, func(t *testing.T) {
			base := "/board/spec/" + name + "/document"
			page := get(t, h, base)
			var snap struct {
				HTML     string `json:"html"`
				Markdown string `json:"markdown"`
				Proposed bool   `json:"proposed"`
			}
			if err := json.Unmarshal([]byte(get(t, h, base+"/snapshot")), &snap); err != nil {
				t.Fatal(err)
			}
			if snap.Proposed {
				t.Fatalf("spec/%s does not render as the accepted reading", name)
			}
			hidden, err := hiddenMarkdown(page)
			if err != nil {
				t.Fatal(err)
			}
			pageSide := map[string]string{"hidden source": hidden, "?format=md download": get(t, h, base+"?format=md"), "snapshot markdown": snap.Markdown}
			legs := otherRenders(t, ctx, repo.Dir, site, cfg.Model, name)
			if !strings.Contains(hidden, "## Identity") || !strings.Contains(hidden, "not authority") {
				t.Fatalf("the compared Markdown is not spec/%s's document:\n%s", name, hidden)
			}
			if strings.Contains(hidden, "data-testid=\"objsupersede-") {
				superseding++
			}
			if err := markdownParity(pageSide, legs); err != nil {
				t.Fatal(err)
			}

			want := sharedHTML(t, ctx, repo.Dir, cfg.Model, name)
			if err := regionParity(page, want); err != nil {
				t.Fatal(err)
			}
			if snap.HTML != want {
				t.Fatalf("the snapshot's html differs from the shared load's at byte %d", firstDifference(snap.HTML, want))
			}

			t.Run("a one-byte change fails each half", func(t *testing.T) {
				flip := func(s string, i int) string { return s[:i] + string(s[i]^0x20) + s[i+1:] }
				for label, side := range map[string]map[string]string{
					"the page's Markdown": {"hidden source": flip(hidden, len(hidden)/2)},
					"a render's Markdown": {"hidden source": hidden},
				} {
					others := legs
					if label == "a render's Markdown" {
						others = map[string]string{"MCP get_document": flip(legs["MCP get_document"], len(hidden)/3)}
					}
					if markdownParity(side, others) == nil {
						t.Errorf("Markdown half: changing one byte of %s still passes", label)
					}
				}
				region := page[regionOpenRe.FindStringIndex(page)[1]:]
				nl := strings.Index(region, "\n")
				for label, mutated := range map[string]struct{ page, want string }{
					// A whitespace byte: any normalized comparison would accept it.
					"a newline in the region turned to a space": {page: strings.Replace(page, region, region[:nl]+" "+region[nl+1:], 1), want: want},
					"a byte the page adds after the body":       {page: strings.Replace(page, want+"</", want+" </", 1), want: want},
					"one byte of the shared load's HTML":        {page: page, want: flip(want, len(want)/2)},
				} {
					if regionParity(mutated.page, mutated.want) == nil {
						t.Errorf("HTML half: %s still passes", label)
					}
				}
			})
		})
	}
	if superseding == 0 {
		t.Fatal("no compared document carries a closed-spec object supersession line: the fixture's purpose is lost")
	}
}

// gitStatus is the working tree's porcelain status.
func gitStatus(ctx context.Context, dir string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain").Output()
	return strings.TrimSpace(string(out)), err
}
