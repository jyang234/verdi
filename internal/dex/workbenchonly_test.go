package dex

import (
	"bytes"
	"strings"
	"testing"
)

func TestStripWorkbenchOnly_Happy(t *testing.T) {
	const begin, end = workbenchOnlyBegin + "\n", workbenchOnlyEnd + "\n"
	for _, tc := range []struct {
		name, in, want string
	}{
		{name: "no block", in: "a { color: red; }\nb { color: blue; }\n", want: "a { color: red; }\nb { color: blue; }\n"},
		{name: "no block, no final newline", in: "a { color: red; }", want: "a { color: red; }"},
		{name: "empty", in: "", want: ""},
		{name: "one block", in: "a {}\n" + begin + ":root { --x: 1; }\n" + end + "b {}\n", want: "a {}\nb {}\n"},
		{name: "one block at the end, no final newline", in: "a {}\n" + begin + ":root { --x: 1; }\n" + workbenchOnlyEnd, want: "a {}\n"},
		{name: "indented marker lines", in: ":root {\n  --a: 1;\n  " + begin + "  --x: 1;\n  " + end + "}\n", want: ":root {\n  --a: 1;\n}\n"},
		{name: "several blocks", in: "a {}\n" + begin + "x {}\n" + end + "b {}\n" + begin + "y {}\nz {}\n" + end + "c {}\n", want: "a {}\nb {}\nc {}\n"},
		{name: "empty block", in: "a {}\n" + begin + end + "b {}\n", want: "a {}\nb {}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stripWorkbenchOnly([]byte(tc.in))
			if err != nil {
				t.Fatalf("stripWorkbenchOnly: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("stripWorkbenchOnly = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStripWorkbenchOnly_Negative(t *testing.T) {
	const begin, end = workbenchOnlyBegin + "\n", workbenchOnlyEnd + "\n"
	for _, tc := range []struct {
		name, in, wantErr string
	}{
		{name: "begin never closed", in: "a {}\n" + begin + "x {}\n", wantErr: "line 2"},
		{name: "end with no open block", in: "a {}\n" + end + "x {}\n", wantErr: "line 2"},
		{name: "nested begin", in: begin + "x {}\n" + begin + "y {}\n" + end + end, wantErr: "nested"},
		{name: "second end after a closed block", in: begin + end + end, wantErr: "line 3"},
		{name: "marker sharing a line with CSS", in: "a {} " + begin + end, wantErr: "line 1"},
		{name: "both markers on one line", in: workbenchOnlyBegin + workbenchOnlyEnd + "\n", wantErr: "line 1"},
		{name: "misspelled marker", in: "/* verdi:workbench-only:start */\nx {}\n" + end, wantErr: "line 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stripWorkbenchOnly([]byte(tc.in))
			if err == nil {
				t.Fatalf("stripWorkbenchOnly = %q, want an error", got)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not name %q", err, tc.wantErr)
			}
		})
	}
}

// TestDocsStyleCSS_StripsWhatTheWorkbenchServes proves the two surfaces'
// split (SI-322): the workbench's stylesheet (StyleCSS) keeps every
// workbench-only block, and the docs site's (docsStyleCSS) is exactly
// StyleCSS with the blocks removed.
func TestDocsStyleCSS_StripsWhatTheWorkbenchServes(t *testing.T) {
	full, err := StyleCSS()
	if err != nil {
		t.Fatalf("StyleCSS: %v", err)
	}
	docs, err := docsStyleCSS()
	if err != nil {
		t.Fatalf("docsStyleCSS: %v", err)
	}
	if !bytes.Contains(full, []byte(workbenchOnlyBegin)) || !bytes.Contains(full, []byte("--wall-edge")) {
		t.Fatal("the workbench stylesheet carries no workbench-only block with --wall-edge")
	}
	if bytes.Contains(docs, []byte("verdi:workbench-only")) || bytes.Contains(docs, []byte("--wall-edge")) || bytes.Contains(docs, []byte("--scrim")) {
		t.Fatal("the docs stylesheet still carries a workbench-only block")
	}
	want, err := stripWorkbenchOnly(full)
	if err != nil {
		t.Fatalf("stripWorkbenchOnly: %v", err)
	}
	if !bytes.Equal(docs, want) {
		t.Fatal("docsStyleCSS is not StyleCSS with its workbench-only blocks stripped")
	}
}
