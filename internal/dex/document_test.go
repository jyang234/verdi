package dex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/index"
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
