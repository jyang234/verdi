package dex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
)

// TestBuild_WritesSpecDocuments is spec/spec-documents ac-4's dex witness:
// every spec page gets the three Markdown files beside it plus a Document
// view, all rendered at the site's build commit through the shared loader,
// and a page that is not a spec gets nothing. buildDexFixtureRepo's
// escrow-autopay is the showcase corpus's accepted feature spec on main.
func TestBuild_WritesSpecDocuments(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	out := t.TempDir()
	if err := Build(context.Background(), Options{Root: repo.Dir, OutDir: out}); err != nil {
		t.Fatal(err)
	}
	specDir := filepath.Join(out, "a", "spec", "escrow-autopay")
	for _, name := range []string{"spec.md", "plan.md", "tasks.md"} {
		data, err := os.ReadFile(filepath.Join(specDir, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		md := string(data)
		if !strings.HasPrefix(md, "# ") || !strings.Contains(md, "not authority") || !strings.Contains(md, "commit `"+repo.Head+"`") {
			t.Errorf("%s wrong shape:\n%s", name, md)
		}
		if strings.Contains(md, "Proposed, not accepted") {
			t.Errorf("%s: a build-commit render is never a working-tree preview, yet it carries the proposed header", name)
		}
	}
	page, err := os.ReadFile(filepath.Join(specDir, "document", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, want := range []string{`<h2 id="identity">`, "not authority", `href="../spec.md"`, `href="../plan.md"`, `href="../tasks.md"`, `href="../"`} {
		if !strings.Contains(html, want) {
			t.Errorf("document page missing %q", want)
		}
	}
	if n := strings.Count(html, "<h1"); n != 1 {
		t.Errorf("document page has %d <h1> elements, want exactly one (the shell's; the fragment's own title is folded into it)", n)
	}
	// The rail lists section headings only (fix round 1, F2): no h3/h4
	// entries, and every entry is a short section name, never a
	// sentence-long decision or criterion heading.
	if strings.Contains(html, `class="toc-level-3"`) || strings.Contains(html, `class="toc-level-4"`) {
		t.Errorf("document page TOC carries h3/h4 entries")
	}
	for _, m := range tocEntryRe.FindAllStringSubmatch(html, -1) {
		if len(m[1]) > 32 {
			t.Errorf("document page TOC entry %q is longer than a section name", m[1])
		}
	}
	if len(tocEntryRe.FindAllString(html, -1)) < 3 {
		t.Errorf("document page TOC has fewer than three section entries — the F2 filter assertion would be vacuous")
	}
	specPage, err := os.ReadFile(filepath.Join(specDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(specPage), `href="document/"`) {
		t.Errorf("spec page must link to its document view")
	}
	// Non-spec pages get no documents.
	adrDir := filepath.Join(out, "a", "adr")
	if _, err := os.Stat(adrDir); err != nil {
		t.Fatalf("fixture has no adr pages to prove the negative path: %v", err)
	}
	for _, glob := range []string{"*/spec.md", "*/document/index.html"} {
		entries, _ := filepath.Glob(filepath.Join(adrDir, glob))
		if len(entries) != 0 {
			t.Errorf("adr pages must not carry spec documents: %v", entries)
		}
	}
}

// tocEntryRe matches one side-rail TOC entry's visible text.
var tocEntryRe = regexp.MustCompile(`<li class="toc-level-[2-4]"><a href="#[^"]*">([^<]*)</a></li>`)

// wtOnlySpec is a component spec that exists only in the working tree of
// the fixture (never committed): indexed, so it gets a page, but absent at
// the build commit, so it gets no document (ac-4: "the site's pinned
// commit").
const wtOnlySpec = `---
id: spec/wt-only-notes
kind: spec
class: component
title: "Working-tree-only notes"
status: active
owners: [platform-team]
---
# Working-tree-only notes

Not yet committed.
`

func TestBuild_SkipsSpecsAbsentAtBuildCommit(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "wt-only-notes", "spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specPath, []byte(wtOnlySpec), 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := Build(context.Background(), Options{Root: repo.Dir, OutDir: out}); err != nil {
		t.Fatalf("a working-tree-only spec must not abort the build: %v", err)
	}
	// The uncommitted spec: a page, no files, no link.
	wt := filepath.Join(out, "a", "spec", "wt-only-notes")
	page, err := os.ReadFile(filepath.Join(wt, "index.html"))
	if err != nil {
		t.Fatalf("working-tree-only spec page: %v", err)
	}
	if strings.Contains(string(page), `href="document/"`) {
		t.Errorf("working-tree-only spec page links to a document it does not have")
	}
	for _, name := range []string{"spec.md", "plan.md", "tasks.md", filepath.Join("document", "index.html")} {
		if _, err := os.Stat(filepath.Join(wt, name)); err == nil {
			t.Errorf("working-tree-only spec wrote %s", name)
		}
	}
	// The committed spec beside it: page, link, and files, as before.
	committed := filepath.Join(out, "a", "spec", "escrow-autopay")
	page, err = os.ReadFile(filepath.Join(committed, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `href="document/"`) {
		t.Errorf("committed spec page lost its document link")
	}
	for _, name := range []string{"spec.md", "plan.md", "tasks.md", filepath.Join("document", "index.html")} {
		if _, err := os.Stat(filepath.Join(committed, name)); err != nil {
			t.Errorf("committed spec missing %s: %v", name, err)
		}
	}
}

func TestDocumentedSpecs(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	pages := []*artifactPage{
		{Entry: &index.Entry{Ref: "spec/escrow-autopay", Kind: "spec"}},
		{Entry: &index.Entry{Ref: "spec/loan-refi-2023", Kind: "spec"}}, // archive zone
		{Entry: &index.Entry{Ref: "spec/wt-only-notes", Kind: "spec"}},
		{Entry: &index.Entry{Ref: "adr/0001-outbox-events", Kind: "adr"}},
	}
	docs, err := documentedSpecs(context.Background(), repo.Dir, repo.Head, pages)
	if err != nil {
		t.Fatal(err)
	}
	want := documentSet{"spec/escrow-autopay": true, "spec/loan-refi-2023": true}
	if len(docs) != len(want) {
		t.Fatalf("documentedSpecs = %v, want %v", docs, want)
	}
	for ref := range want {
		if !docs[ref] {
			t.Errorf("%s missing from the set", ref)
		}
	}
	if _, err := documentedSpecs(context.Background(), repo.Dir, "0000000000000000000000000000000000000000", pages); err == nil {
		t.Errorf("an unresolvable build commit must be an error, not an empty set")
	}
}

func TestSpecDocumentURL(t *testing.T) {
	docs := documentSet{"spec/a": true}
	cases := []struct{ ref, want string }{
		{"spec/a", "document/"},
		{"spec/b", ""},
		{"adr/a", ""},
	}
	for _, c := range cases {
		if got := specDocumentURL(c.ref, docs); got != c.want {
			t.Errorf("specDocumentURL(%q) = %q, want %q", c.ref, got, c.want)
		}
	}
}

func TestDocumentTOC(t *testing.T) {
	in := []TOCEntry{{Level: 2, ID: "identity", Text: "Identity"}, {Level: 3, ID: "dc-1", Text: "A very long decision sentence"}, {Level: 4, ID: "x", Text: "x"}, {Level: 2, ID: "problem", Text: "Problem"}}
	got := documentTOC(in)
	if len(got) != 2 || got[0].ID != "identity" || got[1].ID != "problem" {
		t.Fatalf("documentTOC = %+v, want the two level-2 entries in order", got)
	}
	if got := documentTOC(nil); got != nil {
		t.Fatalf("documentTOC(nil) = %+v, want nil", got)
	}
}

func TestWriteSpecDocuments_NonSpecIsNoop(t *testing.T) {
	out := t.TempDir()
	p := &artifactPage{Entry: &index.Entry{Ref: "adr/0001-outbox-events", Kind: "adr", Title: "Outbox events"}}
	if err := writeSpecDocuments(context.Background(), out, t.TempDir(), buildStamp{SHA: "0000000000000000000000000000000000000000"}, nil, p); err != nil {
		t.Fatalf("non-spec page: %v", err)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("non-spec page wrote %d entries, want none", len(entries))
	}
}

func TestWriteSpecDocuments_UnknownSpecFails(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	p := &artifactPage{Entry: &index.Entry{Ref: "spec/no-such-spec", Kind: "spec", Title: "No such spec"}, RelPath: ".verdi/specs/active/no-such-spec/spec.md"}
	err := writeSpecDocuments(context.Background(), t.TempDir(), repo.Dir, buildStamp{SHA: repo.Head}, nil, p)
	if err == nil || !strings.Contains(err.Error(), "spec/no-such-spec") {
		t.Fatalf("unknown spec: err = %v, want an error naming spec/no-such-spec", err)
	}
}

func TestStripLeadingH1(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"goldmark title", "<h1 id=\"t\">T</h1>\n<h2 id=\"identity\">Identity</h2>\n", "<h2 id=\"identity\">Identity</h2>\n"},
		{"no h1", "<h2 id=\"identity\">Identity</h2>\n", "<h2 id=\"identity\">Identity</h2>\n"},
		{"h1 not first", "<p>x</p>\n<h1>T</h1>\n", "<p>x</p>\n<h1>T</h1>\n"},
		{"unterminated h1", "<h1>T", "<h1>T"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stripLeadingH1(c.in); got != c.want {
				t.Fatalf("stripLeadingH1(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestWriteSpecDocuments_KindsShareOneLoad proves the one-load-per-spec
// economy is byte-neutral: the plan.md and tasks.md dex writes from the
// spec-kind load's Input equal a per-kind load — the path the CLI and MCP
// take, and what the ac-6 parity test compares against.
func TestWriteSpecDocuments_KindsShareOneLoad(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	out := t.TempDir()
	ref := "spec/escrow-autopay"
	p := &artifactPage{Entry: &index.Entry{Ref: ref, Kind: "spec", Title: "Escrow autopay enrollment"}, RelPath: ".verdi/specs/active/escrow-autopay/spec.md"}
	if err := writeSpecDocuments(context.Background(), out, repo.Dir, buildStamp{SHA: repo.Head, Date: "2026-01-01"}, nil, p); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []specdoc.Kind{specdoc.KindSpec, specdoc.KindPlan, specdoc.KindTasks} {
		res, err := specdocload.Load(context.Background(), specdocload.Request{Root: repo.Dir, Name: "escrow-autopay", Mode: specdocload.ModeAt, At: repo.Head, Kind: kind})
		if err != nil {
			t.Fatalf("per-kind load %s: %v", kind, err)
		}
		doc, err := specdoc.Build(res.Input)
		if err != nil {
			t.Fatalf("per-kind build %s: %v", kind, err)
		}
		want := specdoc.RenderMarkdown(doc)
		got, err := os.ReadFile(filepath.Join(out, "a", "spec", "escrow-autopay", string(kind)+".md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s.md differs from a per-kind load:\n--- dex ---\n%s\n--- per-kind ---\n%s", kind, got, want)
		}
		if kind != specdoc.KindSpec && strings.Contains(string(got), "## Problem") {
			t.Errorf("%s.md carries the spec kind's Problem section — the kind was not applied", kind)
		}
	}
}

// TestWriteAllSpecDocuments_FirstErrorWins: an unknown spec among the pages
// fails the whole pass with an error naming it, and non-spec pages are
// skipped without touching the filesystem.
func TestWriteAllSpecDocuments_FirstErrorWins(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	out := t.TempDir()
	pages := []*artifactPage{
		{Entry: &index.Entry{Ref: "adr/0001-outbox-events", Kind: "adr", Title: "Outbox events"}},
		{Entry: &index.Entry{Ref: "spec/no-such-spec", Kind: "spec", Title: "No such spec"}, RelPath: ".verdi/specs/active/no-such-spec/spec.md"},
	}
	err := writeAllSpecDocuments(context.Background(), out, repo.Dir, buildStamp{SHA: repo.Head}, nil, pages, documentSet{"spec/no-such-spec": true})
	if err == nil || !strings.Contains(err.Error(), "spec/no-such-spec") {
		t.Fatalf("err = %v, want an error naming spec/no-such-spec", err)
	}
	if entries, _ := filepath.Glob(filepath.Join(out, "a", "adr", "*")); len(entries) != 0 {
		t.Fatalf("adr page wrote %v", entries)
	}
}

// fakeSpecPages returns n spec pages for pool tests that stand in the
// render step (renderSpecDocuments) and never touch the store.
func fakeSpecPages(n int) ([]*artifactPage, documentSet) {
	pages := make([]*artifactPage, 0, n)
	docs := make(documentSet, n)
	for i := 0; i < n; i++ {
		ref := fmt.Sprintf("spec/fake-%03d", i)
		pages = append(pages, &artifactPage{Entry: &index.Entry{Ref: ref, Kind: "spec", Title: ref}})
		docs[ref] = true
	}
	return pages, docs
}

// stubRender swaps the pool's render step for the test's duration.
func stubRender(t *testing.T, fn func(ctx context.Context, outDir, root string, stamp buildStamp, mdl *model.Model, p *artifactPage) error) {
	t.Helper()
	prev := renderSpecDocuments
	renderSpecDocuments = fn
	t.Cleanup(func() { renderSpecDocuments = prev })
}

// TestWriteAllSpecDocuments_CallerCancel pins the cancellation path (fix
// round 1, F5): the caller cancels after the first page is rendered; the
// pass returns the caller's own cancellation, and fewer than all pages
// were rendered.
func TestWriteAllSpecDocuments_CallerCancel(t *testing.T) {
	pages, docs := fakeSpecPages(64)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var rendered atomic.Int32
	stubRender(t, func(ctx context.Context, _, _ string, _ buildStamp, _ *model.Model, _ *artifactPage) error {
		if rendered.Add(1) == 1 {
			cancel()
		}
		return nil
	})
	err := writeAllSpecDocuments(ctx, t.TempDir(), t.TempDir(), buildStamp{SHA: "0000000000000000000000000000000000000000"}, nil, pages, docs)
	if err != context.Canceled {
		t.Fatalf("err = %v, want the caller's context.Canceled", err)
	}
	if n := int(rendered.Load()); n < 1 || n >= len(pages) {
		t.Fatalf("rendered %d of %d pages, want at least one and fewer than all", n, len(pages))
	}
}

// TestWriteAllSpecDocuments_PanicBecomesError (fix round 1, F4): a render
// that panics fails the pass with an error naming the spec — the build
// exits 2 through the normal path — and does not kill the process.
func TestWriteAllSpecDocuments_PanicBecomesError(t *testing.T) {
	pages, docs := fakeSpecPages(8)
	stubRender(t, func(_ context.Context, _, _ string, _ buildStamp, _ *model.Model, p *artifactPage) error {
		if p.Entry.Ref == "spec/fake-003" {
			panic("render exploded")
		}
		return nil
	})
	err := writeAllSpecDocuments(context.Background(), t.TempDir(), t.TempDir(), buildStamp{SHA: "0000000000000000000000000000000000000000"}, nil, pages, docs)
	if err == nil || !strings.Contains(err.Error(), "spec/fake-003") || !strings.Contains(err.Error(), "render exploded") {
		t.Fatalf("err = %v, want an error naming spec/fake-003 and the panic", err)
	}
}
