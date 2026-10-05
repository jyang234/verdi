package headings

import (
	"reflect"
	"testing"
)

func TestExtract(t *testing.T) {
	for _, tc := range []struct {
		name, html string
		want       []Entry
	}{
		{
			name: "h2 to h4 in document order, h1 excluded",
			html: `<h1 id="title">Title</h1><p>x</p><h2 id="a">Section A</h2><h3 id="b">Sub B</h3><h4 id="c">Deep C</h4>`,
			want: []Entry{{Level: 2, ID: "a", Text: "Section A"}, {Level: 3, ID: "b", Text: "Sub B"}, {Level: 4, ID: "c", Text: "Deep C"}},
		},
		{
			name: "inline markup stripped and entities decoded",
			html: "<h2 id=\"x\">Use <code>a &amp; b</code>\n now</h2>",
			want: []Entry{{Level: 2, ID: "x", Text: "Use a & b\n now"}},
		},
		{
			name: "deduplicated ids are read as rendered",
			html: `<h1 id="plan">Plan</h1><h2 id="plan-1">Plan</h2>`,
			want: []Entry{{Level: 2, ID: "plan-1", Text: "Plan"}},
		},
		{name: "no headings", html: "<p>no headings here</p>", want: []Entry{}},
		{name: "a heading without an id is not an entry", html: `<h2>No id</h2><h5 id="e">Too deep</h5>`, want: []Entry{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Extract(tc.html); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Extract = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSections(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []Entry
		want []Entry
	}{
		{
			name: "only the level-2 entries, in order",
			in:   []Entry{{Level: 2, ID: "identity", Text: "Identity"}, {Level: 3, ID: "dc-1", Text: "A long decision"}, {Level: 4, ID: "x", Text: "x"}, {Level: 2, ID: "problem", Text: "Problem"}},
			want: []Entry{{Level: 2, ID: "identity", Text: "Identity"}, {Level: 2, ID: "problem", Text: "Problem"}},
		},
		{name: "no level-2 entry", in: []Entry{{Level: 3, ID: "dc-1", Text: "x"}}, want: nil},
		{name: "nil", in: nil, want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Sections(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Sections = %+v, want %+v", got, tc.want)
			}
		})
	}
}
