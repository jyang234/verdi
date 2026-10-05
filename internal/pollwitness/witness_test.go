package pollwitness

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/wallbadge"
	"github.com/jyang234/verdi/internal/workbench"
)

// updateEnv re-captures every golden instead of comparing against it.
const updateEnv = "VERDI_POLLWITNESS_UPDATE"

// witnessFixture is one fixture and the walls pinned on it: one story and
// one feature.
type witnessFixture struct {
	name  string
	build func(*testing.T) *fixturegit.Repo
	specs []string
}

func witnessFixtures() []witnessFixture {
	return []witnessFixture{
		{name: "e2e", build: buildE2EStore, specs: []string{"borrower-update-api", "stale-decline"}},
		{name: "real-shaped", build: buildRealShapedStore, specs: []string{realShapedStory, "stale-decline", realShapedFeature}},
	}
}

// TestPollWitness pins, per fixture and per wall, the exact bytes of the
// readiness snapshot, the Readiness section on every surface that renders
// it, the readiness page, and the wall badges. Each surface derives
// through its own independently constructed loader, as in production.
func TestPollWitness(t *testing.T) {
	for _, fx := range witnessFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			repo := fx.build(t)
			for _, spec := range fx.specs {
				t.Run(spec, func(t *testing.T) {
					pinWall(t, repo.Dir, filepath.Join(fx.name, spec))
				})
			}
		})
	}
}

func newLoader(root string) readinessload.Loader {
	return readinessload.Loader{Root: root, Opts: readinessload.Options{BoardHref: workbench.BranchBoardHref}}
}

func pinWall(t *testing.T, root, golden string) {
	t.Helper()
	ctx := context.Background()
	name := filepath.Base(golden)
	ref := "spec/" + name

	snap, err := newLoader(root).Load(ctx, ref)
	if err != nil {
		t.Fatalf("readiness load: %v", err)
	}
	snapJSON := marshal(t, snap)
	checkGolden(t, golden+".snapshot.json", snapJSON)
	again, err := newLoader(root).Load(ctx, ref)
	if err != nil {
		t.Fatalf("second readiness load: %v", err)
	}
	if !bytes.Equal(marshal(t, again), snapJSON) {
		t.Fatal("two loads of the same ref at the same HEAD derived different snapshots (readiness-recovery-v2 ac-2)")
	}

	section := cliReadiness(t, root, name)
	checkGolden(t, golden+".readiness-section.md", []byte(section))
	if got := mcpReadiness(t, root, ref); got != section {
		t.Errorf("MCP get_document's Readiness section differs from the CLI composition's:\n--- mcp ---\n%s\n--- cli ---\n%s", got, section)
	}
	h := workbench.NewHandlerWith(root, workbench.Deps{ReadinessLoader: newLoader(root)})
	if got := readinessSection(t, get(t, h, "/board/spec/"+name+"/document?format=md")); got != section {
		t.Errorf("the board Document tab's Readiness section differs from the CLI composition's:\n--- board ---\n%s\n--- cli ---\n%s", got, section)
	}
	checkGolden(t, golden+".readiness-page.html", []byte(mainRegion(t, get(t, h, "/readiness?spec="+name))))

	checkGolden(t, golden+".badges.json", marshal(t, badges(t, root, name)))
}

// cliReadiness mirrors `verdi spec doc spec/<name>` (cmd/verdi/specdoc.go):
// its own loader, the accepted reading when the spec has landed and the
// working-tree reading (--proposed) when it has not, then specdoc's
// Markdown render.
func cliReadiness(t *testing.T, root, name string) string {
	t.Helper()
	ctx := context.Background()
	cfg, err := store.Open(root)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	snap, err := newLoader(root).Load(ctx, "spec/"+name)
	if err != nil {
		t.Fatalf("CLI readiness load: %v", err)
	}
	req := specdocload.Request{Root: root, Name: name, Mode: specdocload.ModeAccepted, Kind: specdoc.KindSpec, Model: cfg.Model, Readiness: &snap}
	res, err := specdocload.Load(ctx, req)
	if err != nil {
		req.Mode = specdocload.ModeWorkingTree
		if res, err = specdocload.Load(ctx, req); err != nil {
			t.Fatalf("specdocload: %v", err)
		}
	}
	doc, err := specdoc.Build(res.Input)
	if err != nil {
		t.Fatalf("specdoc.Build: %v", err)
	}
	return readinessSection(t, specdoc.RenderMarkdown(doc))
}

// mcpReadiness is MCP get_document's Readiness section, through a backend
// wired with its own loader the way cmd/verdi's mcp.go wires it: the
// accepted reading when the spec has landed, the proposed one when not.
func mcpReadiness(t *testing.T, root, ref string) string {
	t.Helper()
	backend := &mcpserve.Backend{Root: root, ReadinessLoader: newLoader(root)}
	type payload struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	call := func(args string) (payload, []byte) {
		raw, err := json.Marshal(backend.GetDocument(context.Background(), json.RawMessage(args)))
		if err != nil {
			t.Fatalf("marshalling the MCP result: %v", err)
		}
		var p payload
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("decoding the MCP result: %v\n%s", err, raw)
		}
		return p, raw
	}
	result, raw := call(`{"ref":"` + ref + `"}`)
	if result.IsError {
		result, raw = call(`{"ref":"` + ref + `","proposed":true}`)
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("MCP get_document result: %s", raw)
	}
	var doc struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &doc); err != nil {
		t.Fatalf("decoding the MCP document: %v", err)
	}
	return readinessSection(t, doc.Markdown)
}

// badges is the wall's badge set, computed with exactly the inputs
// internal/workbench's attachBadges passes: the spec's store-relative path,
// the sha256 of its working-tree bytes, its frontmatter, no forge, and the
// store's operating model.
func badges(t *testing.T, root, name string) *wallbadge.BoardBadges {
	t.Helper()
	return badgesCtx(t, context.Background(), root, name)
}

// badgesCtx is badges under ctx.
func badgesCtx(t *testing.T, ctx context.Context, root, name string) *wallbadge.BoardBadges {
	t.Helper()
	cfg, err := store.Open(root)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	rel := store.ActiveSpecRelPath(name)
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	out, err := wallbadge.ComputeBadges(ctx, root, rel, "sha256:"+hex.EncodeToString(sum[:]), spec, nil, cfg.Model)
	if err != nil {
		t.Fatalf("ComputeBadges: %v", err)
	}
	return out
}

// readinessSection extracts the "## Readiness" section: from its heading
// to the next level-two heading or thematic break.
func readinessSection(t *testing.T, doc string) string {
	t.Helper()
	const heading = "## Readiness"
	start := strings.Index(doc, heading)
	if start < 0 {
		t.Fatalf("document has no %q heading:\n%s", heading, doc)
	}
	rest := doc[start+len(heading):]
	end := len(rest)
	for _, terminator := range []string{"\n## ", "\n---\n"} {
		if i := strings.Index(rest, terminator); i >= 0 && i < end {
			end = i
		}
	}
	section := doc[start : start+len(heading)+end]
	if strings.Contains(section, "Readiness was not supplied for this render.") {
		t.Fatalf("the Readiness section was not populated:\n%s", section)
	}
	return section
}

// mainRegion is the page's <main> element: the readiness page's own
// content. The shared top bar above it carries the serving checkout's path,
// a fact about the machine and not about the derivation.
func mainRegion(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, "<main")
	end := strings.Index(page, "</main>")
	if start < 0 || end < start {
		t.Fatalf("the page has no <main> region:\n%s", page)
	}
	return page[start:end+len("</main>")] + "\n"
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

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	return append(data, '\n')
}

// checkGolden compares got with testdata/<rel>, or re-captures it when
// VERDI_POLLWITNESS_UPDATE=1. A mismatch names the first differing line.
func checkGolden(t *testing.T, rel string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", filepath.FromSlash(rel))
	if os.Getenv(updateEnv) == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v", path, err)
	}
	if bytes.Equal(got, want) {
		return
	}
	gotLines, wantLines := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Errorf("%s drifted at line %d:\n got: %q\nwant: %q", path, i+1, g, w)
			return
		}
	}
}
