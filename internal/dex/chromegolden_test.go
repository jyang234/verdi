package dex

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// The chrome golden (spec/chrome-and-tokens-v2 ac-4; obligation
// chrome-and-tokens-v2--ac-4--static): a small committed fixture store,
// independent of examples/showcase, and the docs-site build of it.
//
// The golden under chromeGoldenSite was captured at commit 3a9ded5c, the
// commit that adds it, whose production code is its parent 9b2ffd4e's
// (the base of feature/chrome-and-tokens-v2: no production change of the
// story yet), from verdi/, with
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

// TestWorkbenchChromeLeavesDocsSiteUnchanged is ac-4's static producer: the
// docs-site build of the fixture store equals the golden in every output
// file, byte for byte; the stylesheet defines --wall-edge and --scrim with
// dark-mode overrides inside workbench-only blocks (SI-322), which the docs
// build strips; and every rule in those blocks uses tokens only, except the
// pushpin highlights and shadows (SI-326).
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
		defs := scanDeclarations(t, string(css))
		for _, tok := range []struct {
			name, light, dark string
		}{
			// The handoff's values, verbatim (docs/design/handoffs/
			// 2026-09-18-workbench-redesign/README.md, "Tokens added").
			{"--wall-edge", "#d6cdb6", "#3a3325"},
			{"--scrim", "rgba(35,41,32,.28)", "rgba(0,0,0,.5)"},
		} {
			var light, dark []cssDecl
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

	t.Run("new rules use tokens only, except the pushpin highlights and shadows (SI-326)", func(t *testing.T) {
		css, err := os.ReadFile(filepath.Join("assets", "style.css"))
		if err != nil {
			t.Fatalf("reading assets/style.css: %v", err)
		}
		decls := scanDeclarations(t, string(css))
		inBlocks := 0
		for _, d := range decls {
			if d.inBlock {
				inBlocks++
			}
		}
		if inBlocks == 0 {
			t.Fatal("no declaration sits inside a workbench-only block: the check would be vacuous")
		}
		for _, v := range tokenRuleViolations(decls) {
			t.Error(v)
		}
	})
}

// buildChromeGoldenSite builds the docs site of the chrome golden's store
// into a fresh directory and returns it: one fixturegit layer of every
// store file, then Build with Root and OutDir only.
func buildChromeGoldenSite(t *testing.T) string {
	t.Helper()
	repo := chromeGoldenRepo(t)
	out := t.TempDir()
	if err := Build(context.Background(), Options{Root: repo.Dir, OutDir: out}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	return out
}

// chromeGoldenRepo is the chrome golden's store as its one-commit
// fixturegit repository, its HEAD pinned.
func chromeGoldenRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	neutralizeCIEnv(t)
	files := readTreeFiles(t, filepath.Join(chromeGoldenStore, ".verdi"), ".verdi")
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "chrome golden fixture store"}})
	if repo.Head != chromeGoldenHead {
		t.Fatalf("fixture HEAD = %s, want %s: the store or its commit changed, so the golden's stamps no longer describe it", repo.Head, chromeGoldenHead)
	}
	return repo
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

// cssDecl is one declaration in a stylesheet: its property and value, the
// innermost rule's prelude (its selectors, or an at-rule), whether a
// prefers-color-scheme: dark at-rule encloses it, and whether it sits
// inside a workbench-only block.
type cssDecl struct {
	name, value, rule string
	dark, inBlock     bool
}

func (d cssDecl) String() string {
	return fmt.Sprintf("{%s: %s dark=%t}", d.name, d.value, d.dark)
}

// The workbench-only block markers (SI-322), restated here so this test
// reads the stylesheet independently of the stripping code it checks.
const (
	testWorkbenchOnlyBegin = "/* verdi:workbench-only:begin */"
	testWorkbenchOnlyEnd   = "/* verdi:workbench-only:end */"
)

// scanDeclarations walks css once: a marker line opens or closes a
// workbench-only block, a comment is skipped, "{" pushes the rule's
// prelude, "}" pops it, and every "name: value" declaration is recorded
// with its context. Enough CSS for this stylesheet, which carries no
// braces or semicolons inside strings.
func scanDeclarations(t *testing.T, css string) []cssDecl {
	t.Helper()
	var defs []cssDecl
	var stack []string
	var stmt strings.Builder
	inBlock := false
	declare := func() {
		s := strings.TrimSpace(stmt.String())
		stmt.Reset()
		name, value, ok := strings.Cut(s, ":")
		if !ok || len(stack) == 0 {
			return
		}
		dark := false
		for _, prelude := range stack {
			if strings.Contains(prelude, "prefers-color-scheme: dark") {
				dark = true
			}
		}
		defs = append(defs, cssDecl{name: strings.TrimSpace(name), value: strings.TrimSpace(value), rule: stack[len(stack)-1], dark: dark, inBlock: inBlock})
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

// tokenRuleViolations returns one line per declaration inside a
// workbench-only block that breaks ac-4's "new rules use tokens only,
// except the pushpin highlights and shadows" as ledger SI-326 reads it:
//   - a colour literal (hex, a colour function, or a named colour other
//     than transparent, currentColor, and the CSS-wide keywords) is
//     allowed only in a custom-property definition (a token), in a
//     box-shadow or text-shadow value, or in a rule whose selectors all
//     target the pushpin (pushpinRule). A var() fallback is scanned like
//     any value: only the token's name is set aside;
//   - a font-family value is exactly one font token, var(--…), and so is
//     a font shorthand's family, unless the whole shorthand is a CSS-wide
//     keyword (fontShorthandOK).
//
// Declarations outside workbench-only blocks are the docs site's shared
// rules, which this story does not add, and are not checked.
func tokenRuleViolations(decls []cssDecl) []string {
	var out []string
	for _, d := range decls {
		if !d.inBlock {
			continue
		}
		prop := strings.ToLower(d.name)
		switch {
		case strings.HasPrefix(prop, "--"):
			continue
		case prop == "font-family":
			if !fontTokenRe.MatchString(strings.TrimSpace(d.value)) {
				out = append(out, fmt.Sprintf("%s: %s in %q uses no font token (var(--…))", d.name, d.value, d.rule))
			}
			continue
		case prop == "font":
			if !fontShorthandOK(d.value) {
				out = append(out, fmt.Sprintf("%s: %s in %q has a family that is no font token (var(--…))", d.name, d.value, d.rule))
			}
			continue
		case prop == "box-shadow" || prop == "text-shadow", pushpinRule(d.rule):
			continue
		}
		if lit := colourLiteral(d.value); lit != "" {
			out = append(out, fmt.Sprintf("%s: %s in %q carries the colour literal %q, not a token", d.name, d.value, d.rule, lit))
		}
	}
	return out
}

// pushpinRule reports whether every selector of a rule's prelude targets
// the pushpin: its subject — the last compound selector, with :has() set
// aside — carries the handoff's .yarn-handle class or a class naming the
// pushpin. A rule that merely mentions the pushpin (.board:has(.yarn-handle),
// .yarn-handle ~ .card) styles something else.
func pushpinRule(prelude string) bool {
	if strings.HasPrefix(prelude, "@") {
		return false
	}
	for _, sel := range splitTopLevel(prelude, func(c byte) bool { return c == ',' }) {
		compounds := splitTopLevel(removeHas(sel), func(c byte) bool { return c == ' ' || c == '>' || c == '+' || c == '~' || c == '\t' || c == '\n' })
		if len(compounds) == 0 || !pushpinClassRe.MatchString(compounds[len(compounds)-1]) {
			return false
		}
	}
	return true
}

// splitTopLevel splits s at every byte sep accepts outside parentheses,
// dropping empty parts.
func splitTopLevel(s string, sep func(byte) bool) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '(':
			depth++
		case c == ')':
			depth--
		case depth == 0 && sep(c):
			if p := strings.TrimSpace(s[start:i]); p != "" {
				parts = append(parts, p)
			}
			start = i + 1
		}
	}
	if p := strings.TrimSpace(s[start:]); p != "" {
		parts = append(parts, p)
	}
	return parts
}

// removeHas sets every :has(…) aside from a selector, its parentheses
// balanced.
func removeHas(sel string) string {
	for {
		i := strings.Index(sel, ":has(")
		if i < 0 {
			return sel
		}
		depth, j := 0, i+len(":has")
		for ; j < len(sel); j++ {
			if sel[j] == '(' {
				depth++
			} else if sel[j] == ')' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		if j >= len(sel) {
			return sel[:i]
		}
		sel = sel[:i] + sel[j+1:]
	}
}

// fontShorthandOK reports whether a font shorthand keeps SI-326's font
// rule: its whole value is a CSS-wide keyword, or its family — what
// follows the style, variant, weight, stretch, and size/line-height
// components — is exactly one font token, var(--…), with no family list.
func fontShorthandOK(value string) bool {
	v := strings.TrimSpace(strings.ToLower(value))
	switch v {
	case "inherit", "initial", "unset", "revert", "revert-layer":
		return true
	}
	// A family list (a comma) leaves a token that is no component before
	// the last, or a last token that is no font token: refused below.
	tokens := strings.Fields(v)
	if len(tokens) < 2 || !fontTokenRe.MatchString(tokens[len(tokens)-1]) {
		return false
	}
	for _, t := range tokens[:len(tokens)-1] {
		if !fontComponentRe.MatchString(t) {
			return false
		}
	}
	return true
}

var (
	fontTokenRe      = regexp.MustCompile(`^var\(--[A-Za-z0-9_-]+\)$`)
	hexColourRe      = regexp.MustCompile(`#(?:[0-9a-f]{8}|[0-9a-f]{6}|[0-9a-f]{3,4})\b`)
	colourFunctionRe = regexp.MustCompile(`\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color)\(`)
	// pushpinClassRe is a compound selector carrying the pushpin's class:
	// the handoff's .yarn-handle, or a class naming the pushpin.
	pushpinClassRe = regexp.MustCompile(`\.(?:yarn-handle|[a-z0-9_-]*pushpin[a-z0-9_-]*)(?:[^a-z0-9_-]|$)`)
	// tokenNameRe is a var() reference's token name, which is set aside;
	// its fallback is scanned.
	tokenNameRe = regexp.MustCompile(`var\(\s*--[a-z0-9_-]+`)
	// opaqueRe is what can carry no colour of a value's own: a url() and
	// a string.
	opaqueRe = regexp.MustCompile(`url\([^()]*\)|"[^"]*"|'[^']*'`)
	// identRe is one whole identifier token, and whether a function's
	// opening parenthesis follows it.
	identRe = regexp.MustCompile(`-?[a-z_][a-z0-9_-]*(\()?`)
	// fontComponentRe is a font shorthand component before the family: a
	// style, variant, weight, or stretch keyword, a number, a size with an
	// optional line height, or a token.
	fontComponentRe = regexp.MustCompile(`^(?:normal|italic|oblique|small-caps|bold|bolder|lighter|(?:ultra-|extra-|semi-)?(?:condensed|expanded)|xx-small|x-small|small|medium|large|x-large|xx-large|xxx-large|smaller|larger|[0-9.]+(?:[a-z]+|%)?(?:/[0-9.]+(?:[a-z]+|%)?)?|var\(--[a-z0-9_-]+\))$`)
)

// colourLiteral returns the first colour literal value spells — a token's
// name, url()s, and strings set aside, a var() fallback scanned — or ""
// when it spells none. A named colour counts only as a whole identifier
// token, never inside a longer one or as a function's name (tan()).
func colourLiteral(value string) string {
	v := opaqueRe.ReplaceAllString(tokenNameRe.ReplaceAllString(strings.ToLower(value), "var("), " ")
	if m := hexColourRe.FindString(v); m != "" {
		return m
	}
	if m := colourFunctionRe.FindString(v); m != "" {
		return m
	}
	named := namedColours()
	for _, m := range identRe.FindAllStringSubmatch(v, -1) {
		if m[1] == "" && named[m[0]] {
			return m[0]
		}
	}
	return ""
}

// namedColours is CSS Color Module 4's named colours and its system
// colours (Canvas, CanvasText, ...; F1B-A7), lowercased — transparent,
// currentColor, and the CSS-wide keywords are no colour literal and are
// absent.
func namedColours() map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields(`canvas canvastext linktext visitedtext activetext buttonface buttontext
		buttonborder field fieldtext highlight highlighttext selecteditem selecteditemtext mark marktext
		graytext accentcolor accentcolortext`) {
		m[n] = true
	}
	for _, n := range strings.Fields(`aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond
		blue blueviolet brown burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk crimson cyan
		darkblue darkcyan darkgoldenrod darkgray darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange
		darkorchid darkred darksalmon darkseagreen darkslateblue darkslategray darkslategrey darkturquoise darkviolet
		deeppink deepskyblue dimgray dimgrey dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro
		ghostwhite gold goldenrod gray green greenyellow grey honeydew hotpink indianred indigo ivory khaki
		lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan lightgoldenrodyellow lightgray
		lightgreen lightgrey lightpink lightsalmon lightseagreen lightskyblue lightslategray lightslategrey
		lightsteelblue lightyellow lime limegreen linen magenta maroon mediumaquamarine mediumblue mediumorchid
		mediumpurple mediumseagreen mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue
		mintcream mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered orchid palegoldenrod
		palegreen paleturquoise palevioletred papayawhip peachpuff peru pink plum powderblue purple rebeccapurple
		red rosybrown royalblue saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue slateblue
		slategray slategrey snow springgreen steelblue tan teal thistle tomato turquoise violet wheat white
		whitesmoke yellow yellowgreen`) {
		m[n] = true
	}
	return m
}
