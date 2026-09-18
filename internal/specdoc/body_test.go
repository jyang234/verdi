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
