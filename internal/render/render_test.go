package render

import (
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

func TestRenderMarkdown_Happy(t *testing.T) {
	out, err := RenderMarkdown("# Hello\n\nSome *text*.\n\n```mermaid\ngraph TD; A-->B;\n```\n")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if !strings.Contains(out, `<h1 id="hello">Hello</h1>`) {
		t.Fatalf("missing auto-id heading: %s", out)
	}
	if !strings.Contains(out, `<pre class="mermaid">`) {
		t.Fatalf("mermaid fence not left bare for client-side rendering: %s", out)
	}
}

func TestRenderMarkdown_Negative(t *testing.T) {
	// goldmark's Convert only errors on a writer failure, which never
	// happens against a bytes.Buffer — so RenderMarkdown has no reachable
	// error path from a well-formed string input. This test instead pins
	// the negative-path contract that arbitrary (even malformed-looking)
	// markdown never panics and always returns non-error output, since
	// goldmark is deliberately permissive about its input.
	out, err := RenderMarkdown("```unterminated fence with no closing")
	if err != nil {
		t.Fatalf("RenderMarkdown returned an error for permissive markdown input: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty rendered output")
	}
}

func TestDispositionsTable_Happy(t *testing.T) {
	ds := []artifact.Disposition{
		{Sticky: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Disposition: artifact.DispositionIncorporated, Where: "#design-notes"},
		{Sticky: "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", Disposition: artifact.DispositionContradicted, Note: "out of scope"},
		{Sticky: "a-01J8Z0K5CCCCCCCCCCCCCCCCCC", Disposition: artifact.DispositionOpenQuestion},
	}
	got := string(DispositionsTable(ds))
	if !strings.Contains(got, "<table class=\"dispositions-table\">") {
		t.Fatalf("missing table wrapper: %s", got)
	}
	if !strings.Contains(got, `href="#design-notes"`) {
		t.Fatalf("incorporated disposition missing where link: %s", got)
	}
	if !strings.Contains(got, "out of scope") {
		t.Fatalf("contradicted disposition missing note: %s", got)
	}
	if strings.Count(got, "<tr>") != 4 { // header + 3 rows
		t.Fatalf("expected 4 <tr> (1 header + 3 rows), got %d: %s", strings.Count(got, "<tr>"), got)
	}
}

func TestDispositionsTable_Negative_Empty(t *testing.T) {
	if got := DispositionsTable(nil); got != "" {
		t.Fatalf("expected empty string for no dispositions, got %q", got)
	}
}
