package dex

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// The chrome golden (spec/chrome-and-tokens-v2 ac-4; obligation
// chrome-and-tokens-v2--ac-4--static): a small committed fixture store,
// independent of examples/showcase, and the docs-site build of it.
//
// The golden under chromeGoldenSite was captured at commit 9b2ffd4e (the
// base of feature/chrome-and-tokens-v2: no production change of the story
// yet), from verdi/, with
//
//	go test -count=1 ./internal/dex/testdata/chromegolden/capture
//
// which builds the store exactly as buildChromeGoldenSite does below. No
// flag of this test rewrites it: a docs-site byte that differs from it is a
// failure, never an update.
const (
	chromeGoldenStore = "testdata/chromegolden/store"
	chromeGoldenSite  = "testdata/chromegolden/site"
	// chromeGoldenHead is the fixture repository's one commit: fixturegit's
	// fixed identity and date make it stable, and the golden stamps it.
	chromeGoldenHead = "8ab53965d70e04e8fed85f1f886ece764eddaf94"
)

// TestWorkbenchChromeLeavesDocsSiteUnchanged is ac-4's static producer. Its
// two halves: the docs-site build of the fixture store equals the golden in
// every output file, byte for byte, and the stylesheet defines --wall-edge
// and --scrim with dark-mode overrides inside workbench-only blocks (SI-322),
// which the docs build strips.
func TestWorkbenchChromeLeavesDocsSiteUnchanged(t *testing.T) {
	t.Run("docs site equals the golden", func(t *testing.T) {
		built := buildChromeGoldenSite(t)
		for _, d := range compareTrees(t, chromeGoldenSite, built) {
			t.Error(d)
		}
	})

	t.Run("tokens defined with dark-mode overrides in workbench-only blocks", func(t *testing.T) {
		css, err := os.ReadFile(filepath.Join("assets", "style.css"))
		if err != nil {
			t.Fatalf("reading assets/style.css: %v", err)
		}
		defs := scanCustomProperties(t, string(css))
		for _, tok := range []struct {
			name, light, dark string
		}{
			// The handoff's values, verbatim (docs/design/handoffs/
			// 2026-09-18-workbench-redesign/README.md, "Tokens added").
			{"--wall-edge", "#d6cdb6", "#3a3325"},
			{"--scrim", "rgba(35,41,32,.28)", "rgba(0,0,0,.5)"},
		} {
			var light, dark []customPropertyDef
			for _, d := range defs {
				if d.name != tok.name {
					continue
				}
				if !d.inBlock {
					t.Errorf("%s is defined outside a workbench-only block (value %q): the docs build would carry it", tok.name, d.value)
					continue
				}
				if d.dark {
					dark = append(dark, d)
				} else {
					light = append(light, d)
				}
			}
			if len(light) != 1 || light[0].value != tok.light {
				t.Errorf("%s: light definitions in workbench-only blocks = %v, want exactly one with value %q", tok.name, light, tok.light)
			}
			if len(dark) != 1 || dark[0].value != tok.dark {
				t.Errorf("%s: dark-mode overrides in workbench-only blocks = %v, want exactly one with value %q", tok.name, dark, tok.dark)
			}
		}
	})
}

// buildChromeGoldenSite builds the docs site of the chrome golden's store
// into a fresh directory and returns it: one fixturegit layer of every
// store file, then Build with Root and OutDir only.
func buildChromeGoldenSite(t *testing.T) string {
	t.Helper()
	neutralizeCIEnv(t)
	files := readTreeFiles(t, filepath.Join(chromeGoldenStore, ".verdi"), ".verdi")
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "chrome golden fixture store"}})
	if repo.Head != chromeGoldenHead {
		t.Fatalf("fixture HEAD = %s, want %s: the store or its commit changed, so the golden's stamps no longer describe it", repo.Head, chromeGoldenHead)
	}
	out := t.TempDir()
	if err := Build(context.Background(), Options{Root: repo.Dir, OutDir: out}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	return out
}

// compareTrees returns one line per difference between the golden tree
// and the built tree: a file only one side has, or a file whose bytes
// differ (with the first differing offset).
func compareTrees(t *testing.T, golden, built string) []string {
	t.Helper()
	want := treeFiles(t, golden)
	got := treeFiles(t, built)
	if len(want) == 0 {
		return []string{"the golden tree " + golden + " holds no files: the comparison would be vacuous"}
	}
	var diffs []string
	for rel, w := range want {
		g, ok := got[rel]
		switch {
		case !ok:
			diffs = append(diffs, rel+": in the golden, not in the build")
		case !bytes.Equal(w, g):
			diffs = append(diffs, fmt.Sprintf("%s: bytes differ from the golden at offset %d (golden %d bytes, build %d bytes)", rel, firstDiff(w, g), len(w), len(g)))
		}
	}
	for rel := range got {
		if _, ok := want[rel]; !ok {
			diffs = append(diffs, rel+": in the build, not in the golden")
		}
	}
	sort.Strings(diffs)
	return diffs
}

// treeFiles reads every regular file under dir, keyed by its slash path
// relative to dir.
func treeFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatalf("reading tree %s: %v", dir, err)
	}
	return out
}

func firstDiff(a, b []byte) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// customPropertyDef is one custom-property declaration in a stylesheet:
// its value, whether a prefers-color-scheme: dark at-rule encloses it, and
// whether it sits inside a workbench-only block.
type customPropertyDef struct {
	name, value   string
	dark, inBlock bool
}

func (d customPropertyDef) String() string {
	return fmt.Sprintf("{%s: %s dark=%t}", d.name, d.value, d.dark)
}

// The workbench-only block markers (SI-322), restated here so this test
// reads the stylesheet independently of the stripping code it checks.
const (
	testWorkbenchOnlyBegin = "/* verdi:workbench-only:begin */"
	testWorkbenchOnlyEnd   = "/* verdi:workbench-only:end */"
)

// scanCustomProperties walks css once: a marker line opens or closes a
// workbench-only block, a comment is skipped, "{" pushes the rule's
// prelude, "}" pops it, and every "--name: value" declaration is recorded
// with its context. Enough CSS for this stylesheet, which carries no
// braces or semicolons inside strings.
func scanCustomProperties(t *testing.T, css string) []customPropertyDef {
	t.Helper()
	var defs []customPropertyDef
	var stack []string
	var stmt strings.Builder
	inBlock := false
	declare := func() {
		s := strings.TrimSpace(stmt.String())
		stmt.Reset()
		if !strings.HasPrefix(s, "--") {
			return
		}
		name, value, ok := strings.Cut(s, ":")
		if !ok {
			return
		}
		dark := false
		for _, prelude := range stack {
			if strings.Contains(prelude, "prefers-color-scheme: dark") {
				dark = true
			}
		}
		defs = append(defs, customPropertyDef{name: strings.TrimSpace(name), value: strings.TrimSpace(value), dark: dark, inBlock: inBlock})
	}
	for i := 0; i < len(css); {
		if strings.HasPrefix(css[i:], "/*") {
			end := strings.Index(css[i+2:], "*/")
			if end < 0 {
				t.Fatalf("unterminated comment at offset %d", i)
			}
			comment := css[i : i+2+end+2]
			switch comment {
			case testWorkbenchOnlyBegin:
				inBlock = true
			case testWorkbenchOnlyEnd:
				inBlock = false
			}
			i += len(comment)
			continue
		}
		switch c := css[i]; c {
		case '{':
			stack = append(stack, strings.TrimSpace(stmt.String()))
			stmt.Reset()
		case '}':
			declare()
			if len(stack) == 0 {
				t.Fatalf("unbalanced } at offset %d", i)
			}
			stack = stack[:len(stack)-1]
		case ';':
			declare()
		default:
			stmt.WriteByte(c)
		}
		i++
	}
	if len(stack) != 0 {
		t.Fatalf("%d rule(s) left open at the end of the stylesheet", len(stack))
	}
	return defs
}
