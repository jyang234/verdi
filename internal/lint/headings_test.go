package lint

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Design notes", "design-notes"},
		{"Simple", "simple"},
		{"With, Punctuation!", "with-punctuation"},
		{"  leading and trailing  ", "leading-and-trailing"},
		{"Already-hyphenated", "already-hyphenated"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := slugify(tc.in); got != tc.want {
				t.Fatalf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestHeadingAnchors(t *testing.T) {
	body := "# Title\n\nSome text.\n\n## Design notes\n\nMore text.\n\n### Sub heading\n"
	anchors := headingAnchors(body)

	for _, want := range []string{"title", "design-notes", "sub-heading"} {
		if !anchors[want] {
			t.Errorf("anchors missing %q; got %v", want, anchors)
		}
	}
	if len(anchors) != 3 {
		t.Errorf("got %d anchors, want 3: %v", len(anchors), anchors)
	}
}

func TestHeadingAnchors_IgnoresNonHeadingHashLines(t *testing.T) {
	body := "#no-space-not-a-heading\n\nregular text with a # in it\n"
	anchors := headingAnchors(body)
	if len(anchors) != 0 {
		t.Fatalf("got anchors %v, want none", anchors)
	}
}

func TestResolveAnchor(t *testing.T) {
	anchors := map[string]bool{"design-notes": true}
	if !resolveAnchor(anchors, "#design-notes") {
		t.Fatal("resolveAnchor(#design-notes) = false, want true")
	}
	if resolveAnchor(anchors, "#does-not-exist") {
		t.Fatal("resolveAnchor(#does-not-exist) = true, want false")
	}
}
