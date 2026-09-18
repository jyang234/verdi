package specdoc

import "testing"

func TestBodySections(t *testing.T) {
	body := []byte("# Title\n\nintro\n\n## Problem\n\nthe problem\nspans lines\n\n## ac-1\n\nrationale one\n\n### ac-1 sub\n\nkept inside\n\n## dc-2\n\n## oq-1\ntight\n")
	got := bodySections(body)
	cases := map[string]string{
		"problem": "the problem\nspans lines",
		"ac-1":    "rationale one\n\n### ac-1 sub\n\nkept inside",
		"dc-2":    "",
		"oq-1":    "tight",
	}
	for k, want := range cases {
		if got[k] != want {
			t.Errorf("section %q = %q, want %q", k, got[k], want)
		}
	}
	if _, ok := got["title"]; ok {
		t.Errorf("an H1 must not become a section")
	}
	if len(got) != len(cases) {
		t.Errorf("sections = %v, want exactly %d entries", got, len(cases))
	}
}

func TestBodySectionsEmptyAndNoHeadings(t *testing.T) {
	if got := bodySections(nil); len(got) != 0 {
		t.Fatalf("nil body → %v", got)
	}
	if got := bodySections([]byte("just prose\n")); len(got) != 0 {
		t.Fatalf("no headings → %v", got)
	}
}

// TestBodySectionsFenceAwareness is fix round F9's proof: a "## " line
// inside a fenced code block is fence content, never a heading. The fence
// sits inside ac-1's own body, quoting a "## not-a-heading" line the way
// a spec might document this very package's own "## <id>" convention; it
// must stay nested inside ac-1's Detail rather than fracturing a new,
// bogus "not-a-heading" section.
func TestBodySectionsFenceAwareness(t *testing.T) {
	body := []byte("## ac-1\n\nrationale\n\n```\n## not-a-heading\n```\n\nafter fence\n\n## ac-2\n\nnext section\n")
	got := bodySections(body)
	wantAC1 := "rationale\n\n```\n## not-a-heading\n```\n\nafter fence"
	if got["ac-1"] != wantAC1 {
		t.Errorf("ac-1 = %q, want %q", got["ac-1"], wantAC1)
	}
	if _, ok := got["not-a-heading"]; ok {
		t.Errorf("a \"## \" line inside a fence must not become its own section, got sections %v", got)
	}
	if got["ac-2"] != "next section" {
		t.Errorf("ac-2 = %q, want %q", got["ac-2"], "next section")
	}
	if len(got) != 2 {
		t.Errorf("sections = %v, want exactly 2 entries", got)
	}
}
