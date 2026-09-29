package gitx

import (
	"bufio"
	"strings"
	"testing"
)

func TestParseGitOffset(t *testing.T) {
	tests := []struct {
		tz     string
		want   int
		wantOK bool
	}{
		{tz: "+0000", want: 0, wantOK: true},
		{tz: "-0000", want: 0, wantOK: true},
		{tz: "+0530", want: 5*3600 + 30*60, wantOK: true},
		{tz: "-0800", want: -8 * 3600, wantOK: true},
		{tz: "+1400", want: 14 * 3600, wantOK: true},
		{tz: "0000", wantOK: false},    // no sign
		{tz: "+000", wantOK: false},    // too short
		{tz: "+00:00", wantOK: false},  // ISO form, not git's
		{tz: "*0000", wantOK: false},   // bad sign
		{tz: "+0a00", wantOK: false},   // non-digit hours
		{tz: "+000b", wantOK: false},   // non-digit minutes
		{tz: "+0060", wantOK: false},   // minutes out of range
		{tz: "", wantOK: false},        // empty
		{tz: "Z", wantOK: false},       // zulu is not git's form
		{tz: "+00000", wantOK: false},  // too long
		{tz: "-12345", wantOK: false},  // too long, signed
		{tz: "+ 100", wantOK: false},   // embedded space
		{tz: "+-100", wantOK: false},   // doubled sign
		{tz: "--100", wantOK: false},   // doubled sign
		{tz: "+1-00", wantOK: false},   // sign inside hours
		{tz: "+10-0", wantOK: false},   // sign inside minutes
		{tz: "+10+0", wantOK: false},   // sign inside minutes
		{tz: "+99a9", wantOK: false},   // letter inside minutes
		{tz: "\t0000", wantOK: false},  // tab for a sign
		{tz: "+0000\n", wantOK: false}, // trailing newline
	}
	for _, tt := range tests {
		t.Run(tt.tz, func(t *testing.T) {
			got, ok := parseGitOffset(tt.tz)
			if ok != tt.wantOK {
				t.Fatalf("parseGitOffset(%q) ok = %v, want %v", tt.tz, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("parseGitOffset(%q) = %d, want %d", tt.tz, got, tt.want)
			}
		})
	}
}

func TestCommitterDate(t *testing.T) {
	const tree = "tree 4b825dc642cb6eb9a060e54bf8d69288fbee4904\n"
	tests := []struct {
		name   string
		body   string
		want   string
		wantOK bool
	}{
		{
			name:   "well-formed, UTC",
			body:   tree + "author A <a@x> 1704067200 +0000\ncommitter C <c@x> 1704067200 +0000\n\nmsg\n",
			want:   "2024-01-01T00:00:00+00:00",
			wantOK: true,
		},
		{
			name:   "committer offset preserved, never the author's",
			body:   tree + "author A <a@x> 1 +0000\ncommitter C <c@x> 1705276800 +0530\n\nmsg\n",
			want:   "2024-01-15T05:30:00+05:30",
			wantOK: true,
		},
		{
			name:   "a name containing '>' still parses from the last '>'",
			body:   tree + "committer C > D <c@x> 1706745600 -0800\n\nmsg\n",
			want:   "2024-01-31T16:00:00-08:00",
			wantOK: true,
		},
		{
			name:   "a committer line inside the message is never read",
			body:   tree + "author A <a@x> 1 +0000\n\ncommitter C <c@x> 1704067200 +0000\n",
			wantOK: false,
		},
		{name: "no committer line at all", body: tree + "author A <a@x> 1 +0000\n\nmsg\n", wantOK: false},
		{name: "empty body", body: "", wantOK: false},
		{name: "committer without an email bracket", body: tree + "committer C 1704067200 +0000\n\n", wantOK: false},
		{name: "missing offset", body: tree + "committer C <c@x> 1704067200\n\n", wantOK: false},
		{name: "extra stamp field", body: tree + "committer C <c@x> 1704067200 +0000 x\n\n", wantOK: false},
		{name: "non-numeric seconds", body: tree + "committer C <c@x> soon +0000\n\n", wantOK: false},
		{name: "malformed offset", body: tree + "committer C <c@x> 1704067200 +00:00\n\n", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := committerDate([]byte(tt.body))
			if ok != tt.wantOK {
				t.Fatalf("committerDate ok = %v (date %q), want %v", ok, got, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("committerDate = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadBatchAnswer(t *testing.T) {
	tests := []struct {
		name      string
		stream    string
		wantBody  string
		wantFound bool
		wantErr   bool
	}{
		{name: "found commit", stream: "abc commit 5\nhello\n", wantBody: "hello", wantFound: true},
		{name: "found empty commit body", stream: "abc commit 0\n\n", wantBody: "", wantFound: true},
		{name: "missing", stream: "design/x^{commit} missing\n", wantFound: false},
		{name: "ambiguous", stream: "abc^{commit} ambiguous\n", wantFound: false},
		{name: "a rev that looks like a header is still missing", stream: "a commit 5^{commit} missing\n", wantFound: false},
		{name: "empty stream", stream: "", wantErr: true},
		{name: "header without newline", stream: "abc commit 5", wantErr: true},
		{name: "non-commit type", stream: "abc blob 5\nhello\n", wantErr: true},
		{name: "too few header fields", stream: "abc commit\n", wantErr: true},
		{name: "non-numeric size", stream: "abc commit five\nhello\n", wantErr: true},
		{name: "negative size", stream: "abc commit -1\n\n", wantErr: true},
		{name: "truncated content", stream: "abc commit 10\nhello\n", wantErr: true},
		{name: "content not newline-terminated", stream: "abc commit 5\nhelloX", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, found, err := readBatchAnswer(bufio.NewReader(strings.NewReader(tt.stream)))
			if (err != nil) != tt.wantErr {
				t.Fatalf("readBatchAnswer err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if found != tt.wantFound {
				t.Fatalf("readBatchAnswer found = %v, want %v", found, tt.wantFound)
			}
			if string(body) != tt.wantBody {
				t.Fatalf("readBatchAnswer body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}
