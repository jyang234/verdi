package signedapproval

import (
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

func TestWithoutRowLines(t *testing.T) {
	doc := "l1\nl2\nl3\nl4\nl5\n"
	tests := []struct {
		name string
		doc  string
		rows []approvalRow
		want string
	}{
		{name: "no rows keeps every byte", doc: doc, want: doc},
		{name: "one interior row", doc: doc, rows: []approvalRow{{first: 2, last: 3}}, want: "l1\nl4\nl5\n"},
		{name: "two rows", doc: doc, rows: []approvalRow{{first: 1, last: 1}, {first: 4, last: 5}}, want: "l2\nl3\n"},
		{name: "overlapping rows", doc: doc, rows: []approvalRow{{first: 2, last: 3}, {first: 3, last: 4}}, want: "l1\nl5\n"},
		{name: "last line without a newline", doc: "l1\nl2\nl3", rows: []approvalRow{{first: 2, last: 2}}, want: "l1\nl3"},
		{name: "CRLF terminators survive", doc: "l1\r\nl2\r\nl3\r\n", rows: []approvalRow{{first: 2, last: 2}}, want: "l1\r\nl3\r\n"},
		{name: "span beyond the document removes nothing extra", doc: "l1\n", rows: []approvalRow{{first: 5, last: 9}}, want: "l1\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(withoutRowLines([]byte(tc.doc), tc.rows)); got != tc.want {
				t.Fatalf("withoutRowLines = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCarriesRow(t *testing.T) {
	const c = "0123456789abcdef0123456789abcdef01234567"
	blamed := func(orig ...int) []gitx.BlameLine {
		lines := make([]gitx.BlameLine, 0, len(orig))
		for i, o := range orig {
			lines = append(lines, gitx.BlameLine{Commit: c, OrigLine: o, FinalLine: 10 + i})
		}
		return lines
	}
	atCommit := []approvalRow{
		{role: "a", principal: "p", first: 4, last: 5},
		{role: "b", principal: "q", first: 6, last: 7},
	}
	r := approvalRow{role: "a", principal: "p"}
	tests := []struct {
		name   string
		row    approvalRow
		lines  []gitx.BlameLine
		noRows bool
		want   bool
	}{
		{name: "same row on the same lines", row: r, lines: blamed(4, 5), want: true},
		{name: "same lines, another role", row: approvalRow{role: "b", principal: "p"}, lines: blamed(4, 5)},
		{name: "same lines, another principal", row: approvalRow{role: "a", principal: "q"}, lines: blamed(4, 5)},
		{name: "lines of two rows", row: r, lines: blamed(4, 7)},
		{name: "non-contiguous lines", row: r, lines: blamed(4, 6)},
		{name: "part of a row", row: r, lines: blamed(5)},
		{name: "lines outside every row", row: r, lines: blamed(1, 2)},
		{name: "no rows at the commit", row: r, lines: blamed(4, 5), noRows: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := atCommit
			if tc.noRows {
				rows = nil
			}
			if got := carriesRow(rows, tc.row, tc.lines); got != tc.want {
				t.Fatalf("carriesRow = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDistinctCommits(t *testing.T) {
	a, b := "aaaa", "bbbb"
	tests := []struct {
		name  string
		lines []gitx.BlameLine
		want  []string
	}{
		{name: "none", want: nil},
		{name: "one commit", lines: []gitx.BlameLine{{Commit: a}, {Commit: a}}, want: []string{a}},
		{name: "first-seen order", lines: []gitx.BlameLine{{Commit: b}, {Commit: a}, {Commit: b}}, want: []string{b, a}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := distinctCommits(tc.lines); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("distinctCommits = %v, want %v", got, tc.want)
			}
		})
	}
}
