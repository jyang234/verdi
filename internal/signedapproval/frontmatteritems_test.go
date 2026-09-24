package signedapproval

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/jyang234/verdi/internal/artifact"
)

// These tests cover artifact.FrontmatterMappingItems, the one seam helper
// this lane adds under internal/artifact. They live here, beside the
// helper's only consumer, because the lane's write set admits exactly one
// new internal/artifact file.

func TestFrontmatterMappingItems_Happy(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []artifact.FrontmatterItem
	}{
		{
			name: "block rows with surrounding keys and body",
			doc: "---\nschema: x\napprovals:\n  - role: policy-owner\n    principal: principal/sc/MTAwMQ\n" +
				"  - role: escalation\n    principal: principal/sc/MTAwMg\nexpiry: \"2026-12-31\"\n---\nbody\n",
			want: []artifact.FrontmatterItem{
				{Fields: map[string]string{"role": "policy-owner", "principal": "principal/sc/MTAwMQ"}, FirstLine: 4, LastLine: 5},
				{Fields: map[string]string{"role": "escalation", "principal": "principal/sc/MTAwMg"}, FirstLine: 6, LastLine: 7},
			},
		},
		{
			name: "flow rows share one line",
			doc:  "---\napprovals: [{role: a, principal: p}, {role: b, principal: q}]\n---\n",
			want: []artifact.FrontmatterItem{
				{Fields: map[string]string{"role": "a", "principal": "p"}, FirstLine: 2, LastLine: 2},
				{Fields: map[string]string{"role": "b", "principal": "q"}, FirstLine: 2, LastLine: 2},
			},
		},
		{
			name: "comment line inside a row is inside its span",
			doc:  "---\napprovals:\n  - role: a\n    # note\n    principal: p\n---\n",
			want: []artifact.FrontmatterItem{
				{Fields: map[string]string{"role": "a", "principal": "p"}, FirstLine: 3, LastLine: 5},
			},
		},
		{
			name: "a lone dash line is outside the span",
			doc:  "---\napprovals:\n  -\n    role: a\n    principal: p\n---\n",
			want: []artifact.FrontmatterItem{
				{Fields: map[string]string{"role": "a", "principal": "p"}, FirstLine: 4, LastLine: 5},
			},
		},
		{
			name: "quoted scalars",
			doc:  "---\napprovals:\n  - role: \"a\"\n    principal: 'p'\n---\n",
			want: []artifact.FrontmatterItem{
				{Fields: map[string]string{"role": "a", "principal": "p"}, FirstLine: 3, LastLine: 4},
			},
		},
		{name: "absent key", doc: "---\nschema: x\n---\n", want: nil},
		{name: "null value", doc: "---\napprovals:\n---\n", want: nil},
		{name: "explicit null", doc: "---\napprovals: null\n---\n", want: nil},
		{name: "tilde null", doc: "---\napprovals: ~\n---\n", want: nil},
		{name: "empty sequence", doc: "---\napprovals: []\n---\n", want: []artifact.FrontmatterItem{}},
		{name: "empty frontmatter", doc: "---\n---\nbody\n", want: nil},
		{name: "comment-only frontmatter", doc: "---\n# nothing\n---\n", want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := artifact.FrontmatterMappingItems([]byte(tc.doc), "approvals")
			if err != nil {
				t.Fatalf("FrontmatterMappingItems: %v", err)
			}
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("FrontmatterMappingItems =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

func TestFrontmatterMappingItems_Negative(t *testing.T) {
	tests := []struct {
		name, doc, key, wantSubstr string
	}{
		{name: "no frontmatter", doc: "approvals: []\n", wantSubstr: "frontmatter"},
		{name: "unclosed frontmatter", doc: "---\napprovals: []\n", wantSubstr: "frontmatter"},
		{name: "malformed yaml", doc: "---\napprovals: [\n---\n", wantSubstr: "yaml"},
		{name: "anchor", doc: "---\napprovals:\n  - &r {role: a, principal: p}\n---\n", wantSubstr: "anchor"},
		{name: "alias", doc: "---\nx: &r {role: a, principal: p}\napprovals:\n  - *r\n---\n", wantSubstr: "anchor"},
		{name: "custom tag", doc: "---\napprovals:\n  - !row {role: a, principal: p}\n---\n", wantSubstr: "custom tag"},
		{name: "top level is a sequence", doc: "---\n- a\n---\n", wantSubstr: "mapping"},
		{name: "top level is a scalar", doc: "---\njust text\n---\n", wantSubstr: "mapping"},
		{name: "duplicate top-level key", doc: "---\napprovals: []\napprovals: []\n---\n", wantSubstr: "duplicate"},
		{name: "non-string top-level key", doc: "---\n1: x\napprovals: []\n---\n", wantSubstr: "key"},
		{name: "value is a mapping", doc: "---\napprovals: {role: a}\n---\n", wantSubstr: "sequence"},
		{name: "value is a string", doc: "---\napprovals: none\n---\n", wantSubstr: "sequence"},
		{name: "item is a scalar", doc: "---\napprovals:\n  - a\n---\n", wantSubstr: "mapping"},
		{name: "item is a sequence", doc: "---\napprovals:\n  - [a, b]\n---\n", wantSubstr: "mapping"},
		{name: "item value is an integer", doc: "---\napprovals:\n  - {role: 1, principal: p}\n---\n", wantSubstr: "plain string"},
		{name: "item value is a mapping", doc: "---\napprovals:\n  - {role: {x: y}, principal: p}\n---\n", wantSubstr: "plain string"},
		{name: "item has a duplicate key", doc: "---\napprovals:\n  - {role: a, role: b}\n---\n", wantSubstr: "duplicated"},
		{name: "empty key argument", doc: "---\napprovals: []\n---\n", key: " ", wantSubstr: "key"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := tc.key
			if key == "" {
				key = "approvals"
			}
			if key == " " {
				key = ""
			}
			got, err := artifact.FrontmatterMappingItems([]byte(tc.doc), key)
			if err == nil {
				t.Fatalf("FrontmatterMappingItems: want error, got %+v", got)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("error %q does not mention %q", err, tc.wantSubstr)
			}
		})
	}
}

// utf16LE encodes s as UTF-16LE with a byte-order mark, the encoding YAML
// switches to on that mark.
func utf16LE(s string) string {
	var b strings.Builder
	b.WriteString("\xff\xfe")
	for _, u := range utf16.Encode([]rune(s)) {
		b.WriteByte(byte(u))
		b.WriteByte(byte(u >> 8))
	}
	return b.String()
}

// TestFrontmatterMappingItems_LineBreakConvention pins the one line-break
// convention (SI-256): the frontmatter's only line breaks are LF and CRLF.
// YAML also counts a lone CR, NEL, LS, and PS as line breaks and Git does
// not, so each would make the reported spans disagree with the document's
// lines; each is refused with ErrNonstandardLineBreak. A frontmatter that is
// not UTF-8 is refused too: YAML reads a UTF-16 frontmatter by its
// byte-order mark, and there a byte Git counts as a line break can sit
// inside a character YAML does not break on. The body is never parsed for
// spans and may carry any bytes.
func TestFrontmatterMappingItems_LineBreakConvention(t *testing.T) {
	// The reviewer's layout: row b sits alone on document line 5 and
	// "extra: x" on line 6 when brk is not a Git line break.
	probe := func(brk string) string {
		return "---\napprovals:\n  - role: a\n    principal: p\n  - role: b" + brk + "    principal: q\nextra: x\n---\n"
	}
	refused := []struct {
		name, doc string
	}{
		{name: "lone CR", doc: probe("\r")},
		{name: "NEL", doc: probe("\u0085")},
		{name: "LS", doc: probe("\u2028")},
		{name: "PS", doc: probe("\u2029")},
		{name: "two lone CRs push past the closing delimiter", doc: "---\napprovals:\n  - role: a\n    principal: p\n  - role: b\r\r    principal: q\n---\nEVIL\n---\nbody\n"},
		{name: "lone CR before the final CRLF", doc: "---\napprovals:\n  - role: a\n    principal: p\r\r\n---\n"},
		{name: "lone CR in a comment", doc: "---\n# a\rb\napprovals:\n  - role: a\n    principal: p\n---\n"},
		{name: "raw LS inside a quoted scalar", doc: "---\ntitle: \"a\u2028b\"\napprovals:\n  - role: a\n    principal: p\n---\n"},
	}
	for _, tc := range refused {
		t.Run("refused/"+tc.name, func(t *testing.T) {
			got, err := artifact.FrontmatterMappingItems([]byte(tc.doc), "approvals")
			if !errors.Is(err, artifact.ErrNonstandardLineBreak) {
				t.Fatalf("FrontmatterMappingItems = %+v, %v: want ErrNonstandardLineBreak", got, err)
			}
		})
	}

	notUTF8 := []struct {
		name, doc string
	}{
		{name: "UTF-16LE frontmatter", doc: "---\n" + utf16LE("approvals:\n  - role: a\n    principal: p\n  - role: b\u0a41\n    principal: q\n") + "\n---\n"},
		{name: "invalid UTF-8 byte", doc: "---\napprovals:\n  - role: a\xff\n    principal: p\n---\n"},
	}
	for _, tc := range notUTF8 {
		t.Run("not UTF-8/"+tc.name, func(t *testing.T) {
			got, err := artifact.FrontmatterMappingItems([]byte(tc.doc), "approvals")
			if err == nil || !strings.Contains(err.Error(), "UTF-8") {
				t.Fatalf("FrontmatterMappingItems = %+v, %v: want a UTF-8 refusal", got, err)
			}
		})
	}

	accepted := []struct {
		name, doc string
		want      []artifact.FrontmatterItem
	}{
		{
			name: "CRLF throughout",
			doc:  strings.ReplaceAll(probe("\n"), "\n", "\r\n"),
			want: []artifact.FrontmatterItem{
				{Fields: map[string]string{"role": "a", "principal": "p"}, FirstLine: 3, LastLine: 4},
				{Fields: map[string]string{"role": "b", "principal": "q"}, FirstLine: 5, LastLine: 6},
			},
		},
		{
			name: "CRLF on the last frontmatter line only",
			doc:  "---\napprovals:\n  - role: a\n    principal: p\r\n---\n",
			want: []artifact.FrontmatterItem{{Fields: map[string]string{"role": "a", "principal": "p"}, FirstLine: 3, LastLine: 4}},
		},
		{
			name: "body carries every nonstandard break",
			doc:  "---\napprovals:\n  - role: a\n    principal: p\n---\nx\ry\u0085z\u2028w\u2029v\r\r\n",
			want: []artifact.FrontmatterItem{{Fields: map[string]string{"role": "a", "principal": "p"}, FirstLine: 3, LastLine: 4}},
		},
		{
			name: "escaped LS is text, not a break",
			doc:  "---\ntitle: \"a\\u2028b\"\napprovals:\n  - role: a\n    principal: p\n---\n",
			want: []artifact.FrontmatterItem{{Fields: map[string]string{"role": "a", "principal": "p"}, FirstLine: 4, LastLine: 5}},
		},
		{
			name: "UTF-8 byte-order mark",
			doc:  "---\n\ufeffapprovals:\n  - role: a\n    principal: p\n---\n",
			want: []artifact.FrontmatterItem{{Fields: map[string]string{"role": "a", "principal": "p"}, FirstLine: 3, LastLine: 4}},
		},
	}
	for _, tc := range accepted {
		t.Run("accepted/"+tc.name, func(t *testing.T) {
			got, err := artifact.FrontmatterMappingItems([]byte(tc.doc), "approvals")
			if err != nil {
				t.Fatalf("FrontmatterMappingItems: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("FrontmatterMappingItems =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}
