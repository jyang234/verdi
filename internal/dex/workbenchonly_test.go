package dex

import (
	"bytes"
	"context"
	"strconv"
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
// workbench-only block, and the docs site's (docsStyleCSS, which strips
// the committed bytes before composing) is exactly StyleCSS with the
// blocks removed.
func TestDocsStyleCSS_StripsWhatTheWorkbenchServes(t *testing.T) {
	full, err := StyleCSS()
	if err != nil {
		t.Fatalf("StyleCSS: %v", err)
	}
	raw, err := embeddedStyleCSS()
	if err != nil {
		t.Fatalf("embeddedStyleCSS: %v", err)
	}
	docs, err := docsStyleCSS(raw)
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

// TestBuild_RefusesAMalformedWorkbenchOnlyBlock (F1A-A3, F1A-A2): a
// stylesheet whose workbench-only marker is refused fails the whole docs
// build as an operational error naming the committed file's own line (the
// stripping runs before the chroma palettes are composed in), and the build
// writes no docs stylesheet.
func TestBuild_RefusesAMalformedWorkbenchOnlyBlock(t *testing.T) {
	raw, err := embeddedStyleCSS()
	if err != nil {
		t.Fatalf("embeddedStyleCSS: %v", err)
	}
	lines := bytes.Count(raw, []byte("\n"))
	for _, tc := range []struct {
		name, appended, want string
	}{
		{name: "a block never closed", appended: workbenchOnlyBegin + "\n:root { --x: 1; }\n", want: "has no end marker"},
		{name: "an end with no open block", appended: workbenchOnlyEnd + "\n", want: "no open block"},
		{name: "a misspelled marker", appended: "/* verdi:workbench-only:start */\n", want: "malformed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := chromeGoldenRepo(t)
			out := t.TempDir()
			broken := append(append([]byte{}, raw...), tc.appended...)
			err := build(context.Background(), Options{Root: repo.Dir, OutDir: out}, func() ([]byte, error) { return broken, nil })
			if err == nil {
				t.Fatal("build accepted a malformed workbench-only marker")
			}
			wantLine := "style.css line " + strconv.Itoa(lines+1) + ":"
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), wantLine) {
				t.Fatalf("build error %q, want it to name %q at the committed file's %q", err, tc.want, wantLine)
			}
			if fileExists(out, "assets/style.css") {
				t.Fatal("a refused build still wrote the docs stylesheet")
			}
		})
	}
}
