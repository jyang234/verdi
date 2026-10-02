// spec/strict-lint-gate-v2 ac-3, its source half (dc-4; ledger SI-328,
// SI-330): a //nolint directive in the module's Go source that names a gated
// linter is an exclusion too. It carries a reason after its own //, and the
// number of such directives is pinned here, apart from the configuration's
// exclusions, so every added one is a visible change to this file. A
// directive that suppresses every linter, the gated ones included, is
// refused whatever else it names.
//
// The witness reads the directives itself: nolintlint, which could require
// named linters and reasons, would be a sixth linter (strict-lint-target-v2
// dc-2). It reads them as golangci-lint 2.5.0, the Makefile's pin, does in
// pkg/result/processors/nolint_filter.go (NewNolintFilter's pattern and
// NolintFilter.extractInlineRangeFromComment), over every comment of a file
// parsed with go/parser and parser.ParseComments:
//
//   - a comment is a directive when its text, with every leading '/' and ' '
//     trimmed, matches ^nolint( |:|$), so `// nolint` is one and a
//     mid-comment mention, a block comment, or `//nolintx` is not (nor may a
//     line of this file's comments begin with the name, or it would be one);
//   - it suppresses every linter when that text begins with "nolint:all" or
//     does not begin with "nolint:" (a bare //nolint, or //nolint and a
//     space);
//   - otherwise its list is the text after "nolint:" up to the first "//",
//     split at commas, each name trimmed and lowercased, and a name "all"
//     suppresses every linter wherever it stands.
//
// A name matches a linter exactly: no linter in golangci-lint 2.5.0 declares
// an alternative name (lintersdb's builders never call WithAlternativeNames).
// The reason is the text after that first "//", trimmed; nolintlint's
// require-explanation likewise wants more than "//" there.
//
// contextcheck reads directives of its own, which golangci-lint's nolint
// filter never sees (ledger SI-337): a line of a function declaration's doc
// comment that it reads makes it skip that function, or check it as a
// handler, and drop its finding at the function's call sites. The witness
// reads those lines as contextcheck does (contextcheckDocDirective), each as
// a directive naming contextcheck, counted and needing a reason after its
// own // like any other. A line both readings recognize is one directive.
//
// The files read are the module's linux/amd64 lint set, the configuration
// witness's own universe (moduleGoFiles; ledger SI-315): the files make
// lint-strict analyzes, test files included, testdata directories and nested
// modules excluded. The gated linters are the strict configuration's
// linters.enable, read from the file, never copied here.
package specalign

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// strictLintDirectiveCount is the pinned number of //nolint directives in the
// module's linux/amd64 lint set that name a gated linter. Adding one means
// changing this number in the same change (ac-3).
const strictLintDirectiveCount = 1

// nolintProbeDir is the //nolint probe module (internal/lintratchet's
// TestLintStrict_NolintProbe), relative to the repository root: each file
// holds one gated finding under one directive shape, and its
// name says whether the pinned golangci-lint suppresses that finding
// (suppressed_*.go) or reports it (reported_*.go).
const nolintProbeDir = "internal/lintratchet/testdata/nolintprobe"

// nolintDirective is one //nolint directive, read as golangci-lint 2.5.0
// reads it.
type nolintDirective struct {
	// at is the directive's file:line, the file relative to the module root.
	at string
	// text is the comment as written, its // included.
	text string
	// all reports that it suppresses every linter: it has no list, or its
	// list contains all.
	all bool
	// linters are the names in its list, trimmed and lowercased, as
	// golangci-lint matches them against linter names.
	linters []string
	// reason is the text after its own //, trimmed, or "" when it has none.
	reason string
	// contextcheck reports that contextcheck itself reads it, a line of a
	// function declaration's doc comment, so it names contextcheck whatever
	// its list says (contextcheckDocDirective; ledger SI-337).
	contextcheck bool
}

// contextcheckLinter is the gated linter that reads directives of its own,
// and the name it looks for in them.
const contextcheckLinter = "contextcheck"

// contextcheckNolintPattern is contextcheck v1.1.6's own pattern for a doc
// line that makes it skip a function (nolintRe, contextcheck.go:272).
var contextcheckNolintPattern = regexp.MustCompile(`^//\s?nolint:`)

// contextcheckRequestFlag is contextcheck v1.1.6's other doc flag
// (runner.docFlag, contextcheck.go:265), which makes it check a function
// taking an *http.Request as a handler; the pinned golangci-lint then drops
// that function's call-site finding (the probe module's
// suppressed_contextcheck_request.go).
const contextcheckRequestFlag = "// @contextcheck(req_has_ctx)"

// contextcheckDocDirective reports whether contextcheck reads comment, one
// line of a function declaration's doc comment as go/ast holds it (its //
// included), as one of its own directives, and returns the text after the
// directive's opening, where its reason is sought after its own //. It reads
// as contextcheck v1.1.6, bundled in the pinned golangci-lint 2.5.0, does in
// github.com/kkHAIKE/contextcheck@v1.1.6/contextcheck.go:261-292: for each
// line of a function declaration's doc (FuncDecl.Doc, in getDocFromFunc; a
// method is one too), a line matching nolintRe, ^//\s?nolint:, that holds the
// case-sensitive substring contextcheck anywhere skips the function, and
// otherwise one beginning with the request flag marks it a handler
// (docFlag).
func contextcheckDocDirective(comment string) (string, bool) {
	if loc := contextcheckNolintPattern.FindStringIndex(comment); loc != nil && strings.Contains(comment, contextcheckLinter) {
		return comment[loc[1]:], true
	}
	if rest, ok := strings.CutPrefix(comment, contextcheckRequestFlag); ok {
		return rest, true
	}
	return "", false
}

// parseNolintDirective reads comment, one comment's text as go/ast holds it
// (its // or /* included), as golangci-lint 2.5.0's nolint filter reads it,
// and reports whether it is a directive at all.
func parseNolintDirective(comment string) (nolintDirective, bool) {
	text := strings.TrimLeft(comment, "/ ")
	rest, ok := strings.CutPrefix(text, "nolint")
	if !ok || (rest != "" && rest[0] != ' ' && rest[0] != ':') {
		return nolintDirective{}, false
	}
	list, hasList := strings.CutPrefix(rest, ":")
	list, reason, _ := strings.Cut(list, "//")
	d := nolintDirective{text: comment, reason: strings.TrimSpace(reason)}
	if !hasList {
		d.all = true
		return d, true
	}
	d.all = strings.HasPrefix(list, "all")
	for item := range strings.SplitSeq(list, ",") {
		name := strings.ToLower(strings.TrimSpace(item))
		d.all = d.all || name == "all"
		d.linters = append(d.linters, name)
	}
	return d, true
}

// fileNolintDirectives returns the //nolint directives among the comments of
// src, the Go file at rel, parsed as golangci-lint parses a file for them
// (go/parser with parser.ParseComments), and contextcheck's own directives
// among its function declarations' doc lines, one directive per comment. A
// file that does not parse is an error, never a file without directives.
func fileNolintDirectives(rel string, src []byte) ([]nolintDirective, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing %s for //nolint directives: %w", rel, err)
	}
	funcDoc := map[*ast.Comment]bool{}
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Doc != nil {
			for _, c := range fd.Doc.List {
				funcDoc[c] = true
			}
		}
	}
	var out []nolintDirective
	for _, g := range f.Comments {
		for _, c := range g.List {
			d, ok := parseNolintDirective(c.Text)
			if rest, read := contextcheckDocDirective(c.Text); read && funcDoc[c] {
				if !ok {
					_, reason, _ := strings.Cut(rest, "//")
					d, ok = nolintDirective{text: c.Text, reason: strings.TrimSpace(reason)}, true
				}
				d.contextcheck = true
			}
			if ok {
				d.at = fmt.Sprintf("%s:%d", rel, fset.Position(c.Pos()).Line)
				out = append(out, d)
			}
		}
	}
	return out, nil
}

// lintSetNolintDirectives returns the //nolint directives in the linux/amd64
// lint set of the module at root: the files moduleGoFiles lists, the universe
// the configuration witness measures (ledger SI-315). A lint set that cannot
// be listed or is empty is an error, never a set without directives.
func lintSetNolintDirectives(root string) ([]nolintDirective, error) {
	files, err := moduleGoFiles(root)
	if err != nil {
		return nil, fmt.Errorf("listing the module's linux/amd64 lint set: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("the module at %s has no Go file in its linux/amd64 lint set, so no //nolint directive can be read", root)
	}
	var out []nolintDirective
	for _, rel := range files {
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", rel, err)
		}
		ds, err := fileNolintDirectives(rel, src)
		if err != nil {
			return nil, err
		}
		out = append(out, ds...)
	}
	return out, nil
}

// strictConfigGatedLinters returns, sorted, the linters raw, a strict
// configuration's source, enables: the gated set (strict-lint-target-v2
// dc-2), which TestStrictLintTargetIsWired holds the configuration to. A
// configuration that enables none is an error, since no directive could then
// name a gated linter.
func strictConfigGatedLinters(raw string) ([]string, error) {
	top, err := decodeStrictLintConfig(raw)
	if err != nil {
		return nil, err
	}
	enabled := asStringSlice(lookupOrNil(top, "linters", "enable"))
	if len(enabled) == 0 {
		return nil, fmt.Errorf("%s enables no linter, so no //nolint directive can be classified against the gated set", strictLintConfigFile)
	}
	return slices.Sorted(slices.Values(enabled)), nil
}

// gatedNamed returns, sorted and once each, the gated linters d names: those
// in its list, and contextcheck when contextcheck itself reads it.
func gatedNamed(d nolintDirective, gated []string) []string {
	names := d.linters
	if d.contextcheck {
		names = append(slices.Clone(names), contextcheckLinter)
	}
	var out []string
	for _, l := range names {
		if slices.Contains(gated, l) && !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	slices.Sort(out)
	return out
}

// strictLintDirectiveProblems returns every way directives depart from ac-3,
// classified against gated: a directive that suppresses every linter (no
// list, or all in it), which is refused; one that names a gated linter
// without a reason; and a number of directives naming a gated linter other
// than pinned. A directive naming only ungated linters is neither counted nor
// refused.
func strictLintDirectiveProblems(directives []nolintDirective, gated []string, pinned int) []string {
	var out []string
	count := 0
	for _, d := range directives {
		if d.all {
			out = append(out, fmt.Sprintf("%s: %q suppresses every linter, the gated ones included (no list, or all in it), so it is refused: name the linters it excludes (spec/strict-lint-gate-v2 ac-3, dc-4)", d.at, d.text))
		}
		named := gatedNamed(d, gated)
		if len(named) == 0 {
			continue
		}
		count++
		if d.reason == "" {
			out = append(out, fmt.Sprintf("%s: %q names gated linter(s) %v but carries no reason: write one after its own // (spec/strict-lint-gate-v2 ac-3)", d.at, d.text, named))
		}
	}
	if count != pinned {
		out = append(out, fmt.Sprintf("the lint set holds %d //nolint directive(s) naming a gated linter, but strictLintDirectiveCount pins %d: change the pin in the same change, so every added exclusion is visible", count, pinned))
	}
	return out
}

// realStrictGatedLinters reads the gated set from the committed strict
// configuration.
func realStrictGatedLinters(t *testing.T) []string {
	t.Helper()
	gated, err := strictConfigGatedLinters(readStrictLintConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	return gated
}

// synthesizeDirectives parses decls, Go declarations and their comments, as
// the body of a file synthetic.go, and returns its //nolint directives.
func synthesizeDirectives(t *testing.T, decls string) []nolintDirective {
	t.Helper()
	ds, err := fileNolintDirectives("synthetic.go", []byte("package p\n\n"+decls+"\n"))
	if err != nil {
		t.Fatalf("synthetic source: %v", err)
	}
	return ds
}

// testStrictLintSourceDirectives is TestStrictLintExclusionsCounted's source
// half: the committed lint set's directives, then each falsifier shape of a
// synthetic directive, then the probe module's shapes, each of which the
// pinned golangci-lint suppresses or reports as its file name says.
func testStrictLintSourceDirectives(t *testing.T) {
	gated := realStrictGatedLinters(t)
	committed, err := lintSetNolintDirectives(verdiRepoRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("the committed lint set", func(t *testing.T) {
		for _, p := range strictLintDirectiveProblems(committed, gated, strictLintDirectiveCount) {
			t.Error(p)
		}
	})

	// The S3 review's reproduction of finding S3-1: this file, added to the
	// lint set at internal/lintratchet/zz_slip.go, suppressed contextcheck's
	// finding at line 11 under the pinned golangci-lint while the witness
	// passed. It is read here beside the committed lint set, never added to
	// it: counted, it breaks the pin; without a reason, it is named.
	t.Run("the review's contextcheck slip", func(t *testing.T) {
		const (
			slipFile      = "internal/lintratchet/zz_slip.go"
			slipDirective = "//nolint:unused // contextcheck: deliberately starts afresh"
			slipSource    = "package lintratchet\n\nimport \"context\"\n\nfunc slipWait(ctx context.Context) error { return ctx.Err() }\n\n" +
				slipDirective + "\nfunc slipFresh() error { return slipWait(context.Background()) }\n\n" +
				"// SlipCaller has a context but calls slipFresh, which does not take it.\n" +
				"func SlipCaller(ctx context.Context) error { _ = ctx; return slipFresh() }\n"
		)
		cases := []struct {
			name, directive, want string
		}{
			{name: "as reproduced", directive: slipDirective, want: fmt.Sprintf("holds %d //nolint directive(s) naming a gated linter, but strictLintDirectiveCount pins %d", strictLintDirectiveCount+1, strictLintDirectiveCount)},
			{name: "without a reason", directive: "//nolint:contextchecks", want: slipFile + ":7: \"//nolint:contextchecks\" names gated linter(s) [contextcheck] but carries no reason"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				slip, err := fileNolintDirectives(slipFile, []byte(strings.Replace(slipSource, slipDirective, tc.directive, 1)))
				if err != nil {
					t.Fatal(err)
				}
				problems := strictLintDirectiveProblems(append(slices.Clone(committed), slip...), gated, strictLintDirectiveCount)
				if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, tc.want) }) {
					t.Fatalf("problems = %q, want one containing %q", problems, tc.want)
				}
			})
		}
	})

	cases := []struct {
		name   string
		decls  string
		pinned int
		want   string // a substring of one problem; "" wants none
	}{
		// Counted: a directive naming a gated linter, which carries a reason.
		{name: "a gated linter, with a reason", decls: "var x int //nolint:gochecknoglobals // a reason", pinned: 1},
		{name: "a gated linter on the line above, with a reason", decls: "//nolint:gochecknoglobals // a reason\nvar x int", pinned: 1},
		{name: "a gated linter among ungated ones, with a reason", decls: "var x int //nolint:unused,gochecknoglobals,staticcheck // a reason", pinned: 1},
		{name: "a gated linter, the reason unspaced", decls: "var x int //nolint:gochecknoglobals//a reason", pinned: 1},
		{name: "a gated linter named twice, counted once", decls: "var x int //nolint:errorlint,errorlint // a reason", pinned: 1},
		{name: "two directives", decls: "var x int //nolint:noctx // a reason\nvar y int //nolint:containedctx // a reason", pinned: 2},
		{name: "a count other than the pin", decls: "var x int //nolint:gochecknoglobals // a reason", pinned: 0, want: "holds 1 //nolint directive(s) naming a gated linter, but strictLintDirectiveCount pins 0"},
		{name: "a second directive beyond the pin", decls: "var x int //nolint:noctx // a reason\nvar y int //nolint:containedctx // a reason", pinned: 1, want: "holds 2 //nolint directive(s) naming a gated linter, but strictLintDirectiveCount pins 1"},
		// Counted, but without a reason.
		{name: "a gated linter, without a reason", decls: "var x int //nolint:errorlint", pinned: 1, want: "synthetic.go:3: \"//nolint:errorlint\" names gated linter(s) [errorlint] but carries no reason"},
		{name: "a gated linter, an empty reason", decls: "var x int //nolint:errorlint //  ", pinned: 1, want: "carries no reason"},
		{name: "a gated linter among ungated ones, without a reason", decls: "var x int //nolint:unused,contextcheck", pinned: 1, want: "names gated linter(s) [contextcheck] but carries no reason"},
		{name: "a gated linter capitalised, without a reason", decls: "var x int //nolint:ErrorLint", pinned: 1, want: "names gated linter(s) [errorlint] but carries no reason"},
		{name: "a gated linter in a spaced directive, without a reason", decls: "var x int // nolint:noctx", pinned: 1, want: "names gated linter(s) [noctx] but carries no reason"},
		{name: "a gated linter, the list spaced, without a reason", decls: "var x int //nolint: noctx , unused", pinned: 1, want: "names gated linter(s) [noctx] but carries no reason"},
		// Refused: a directive that suppresses every linter.
		{name: "a bare directive", decls: "var x int //nolint", pinned: 0, want: "synthetic.go:3: \"//nolint\" suppresses every linter"},
		{name: "a bare directive, with a reason", decls: "var x int //nolint // a reason", pinned: 0, want: "suppresses every linter"},
		{name: "a bare spaced directive", decls: "var x int // nolint", pinned: 0, want: "suppresses every linter"},
		{name: "a directive and a space before its colon", decls: "var x int //nolint :errorlint // a reason", pinned: 0, want: "suppresses every linter"},
		{name: "all", decls: "var x int //nolint:all // a reason", pinned: 0, want: "\"//nolint:all // a reason\" suppresses every linter"},
		{name: "all, not first", decls: "var x int //nolint:unused,all // a reason", pinned: 0, want: "\"//nolint:unused,all // a reason\" suppresses every linter"},
		{name: "all, last of three", decls: "var x int //nolint:unused,staticcheck, all // a reason", pinned: 0, want: "suppresses every linter"},
		{name: "all capitalised, not first", decls: "var x int //nolint:unused,ALL // a reason", pinned: 0, want: "suppresses every linter"},
		{name: "all beside a gated linter", decls: "var x int //nolint:errorlint,all // a reason", pinned: 1, want: "\"//nolint:errorlint,all // a reason\" suppresses every linter"},
		{name: "a list beginning with all", decls: "var x int //nolint:allx // a reason", pinned: 0, want: "suppresses every linter"},
		{name: "a comment line of a group beginning with nolint", decls: "// x is a counter.\n// nolint begins this line, so golangci-lint reads it as a directive.\nvar x int", pinned: 0, want: "synthetic.go:4: \"// nolint begins this line, so golangci-lint reads it as a directive.\" suppresses every linter"},
		// Neither counted nor refused.
		{name: "only ungated linters, without a reason", decls: "var x int //nolint:unused,staticcheck", pinned: 0},
		{name: "an empty list", decls: "var x int //nolint:", pinned: 0},
		{name: "a gated linter's name in the reason only", decls: "var x int //nolint:unused // not errorlint, all", pinned: 0},
		{name: "a gated linter and a reason without its //", decls: "var x int //nolint:errorlint because", pinned: 0},
		{name: "a mention mid-comment", decls: "var x int // a mention of //nolint mid-comment", pinned: 0},
		{name: "a word beginning with nolint", decls: "var x int //nolintx", pinned: 0},
		{name: "a block comment", decls: "var x int /* nolint */", pinned: 0},
		{name: "a tab before nolint", decls: "var x int //\tnolint", pinned: 0},
		// contextcheck's own reading of a function declaration's doc (ledger
		// SI-337), counted as naming contextcheck.
		{name: "contextcheck's name in the reason of a function's doc directive", decls: "//nolint:unused // contextcheck: deliberately starts afresh\nfunc fresh() {}", pinned: 1},
		{name: "contextcheck's name in a method's doc directive", decls: "type T struct{}\n\n//nolint:unused // contextcheck: a reason\nfunc (T) m() {}", pinned: 1},
		{name: "contextcheck's name in a later line of a function's doc", decls: "// fresh starts afresh.\n//\n//nolint:unused // contextcheck: a reason\nfunc fresh() {}", pinned: 1},
		{name: "contextcheck's name in a spaced doc directive's reason", decls: "// nolint:staticcheck // contextcheck too\nfunc fresh() {}", pinned: 1},
		{name: "contextcheck named in a function's doc by both readings, counted once", decls: "//nolint:contextcheck // a reason\nfunc fresh() {}", pinned: 1},
		{name: "contextcheck's request flag, with a reason", decls: "// @contextcheck(req_has_ctx) // a reason\nfunc h() {}", pinned: 1},
		{name: "contextcheck's name in a function's doc, beyond the pin", decls: "//nolint:unused // contextcheck: a reason\nfunc fresh() {}", pinned: 0, want: "holds 1 //nolint directive(s) naming a gated linter, but strictLintDirectiveCount pins 0"},
		{name: "contextcheck's name inside another in a function's doc, without a reason", decls: "//nolint:contextchecks\nfunc fresh() {}", pinned: 1, want: "synthetic.go:3: \"//nolint:contextchecks\" names gated linter(s) [contextcheck] but carries no reason"},
		{name: "contextcheck after a tab in a function's doc, without a reason", decls: "//\tnolint:contextcheck\nfunc fresh() {}", pinned: 1, want: "synthetic.go:3: \"//\\tnolint:contextcheck\" names gated linter(s) [contextcheck] but carries no reason"},
		{name: "contextcheck's request flag, without a reason", decls: "// @contextcheck(req_has_ctx)\nfunc h() {}", pinned: 1, want: "synthetic.go:3: \"// @contextcheck(req_has_ctx)\" names gated linter(s) [contextcheck] but carries no reason"},
		{name: "contextcheck's request flag and a reason without its //", decls: "// @contextcheck(req_has_ctx) because\nfunc h() {}", pinned: 1, want: "names gated linter(s) [contextcheck] but carries no reason"},
		{name: "all in a function's doc beside contextcheck's name", decls: "//nolint:all // contextcheck\nfunc fresh() {}", pinned: 1, want: "\"//nolint:all // contextcheck\" suppresses every linter"},
		// Not contextcheck's reading: neither counted nor refused.
		{name: "contextcheck's name capitalised in a function's doc", decls: "//nolint:unused // ContextCheck: a reason\nfunc fresh() {}", pinned: 0},
		{name: "contextcheck's name in a variable's doc directive", decls: "//nolint:unused // contextcheck: a reason\nvar x int", pinned: 0},
		{name: "contextcheck's name above a function, a blank line between", decls: "//nolint:unused // contextcheck: a reason\n\nfunc fresh() {}", pinned: 0},
		{name: "contextcheck's name inside a function", decls: "func fresh() {\n\t//nolint:unused // contextcheck: a reason\n}", pinned: 0},
		{name: "contextcheck's name after a function, on its line", decls: "func fresh() {} //nolint:unused // contextcheck: a reason", pinned: 0},
		{name: "contextcheck's name in a function's doc, two spaces before nolint", decls: "//  nolint:unused // contextcheck: a reason\nfunc fresh() {}", pinned: 0},
		{name: "contextcheck's name in a function's doc, a third slash before nolint", decls: "///nolint:unused // contextcheck: a reason\nfunc fresh() {}", pinned: 0},
		{name: "contextcheck's name in a function's doc that holds no directive", decls: "// fresh is contextcheck's concern.\nfunc fresh() {}", pinned: 0},
		{name: "contextcheck's name in a function's block-comment doc", decls: "/*nolint:unused // contextcheck */\nfunc fresh() {}", pinned: 0},
		{name: "contextcheck's request flag on a type's doc", decls: "// @contextcheck(req_has_ctx)\ntype T struct{}", pinned: 0},
		{name: "contextcheck's request flag, unspaced", decls: "//@contextcheck(req_has_ctx)\nfunc h() {}", pinned: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := strictLintDirectiveProblems(synthesizeDirectives(t, tc.decls), gated, tc.pinned)
			if tc.want == "" {
				if len(problems) != 0 {
					t.Fatalf("problems = %q, want none", problems)
				}
				return
			}
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, tc.want) }) {
				t.Fatalf("problems = %q, want one containing %q", problems, tc.want)
			}
		})
	}

	t.Run("the probe module", func(t *testing.T) {
		for _, p := range nolintProbeProblems(t, filepath.Join(verdiRepoRoot, filepath.FromSlash(nolintProbeDir)), gated) {
			t.Error(p)
		}
	})
}

// nolintProbeProblems returns each probe file, under dir, whose directives
// the witness reads otherwise than the pinned golangci-lint treats them: a
// suppressed_*.go file needs a directive, and the witness must refuse or
// count each of its directives; a reported_*.go file's directives must be
// neither, as the finding stands. Any other Go file but doc.go is a problem.
func nolintProbeProblems(t *testing.T, dir string, gated []string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the probe module: %v", err)
	}
	var out []string
	seen := map[string]int{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || name == "doc.go" {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		ds, err := fileNolintDirectives(name, src)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.HasPrefix(name, "suppressed_"):
			seen["suppressed"]++
			if len(ds) == 0 {
				out = append(out, fmt.Sprintf("%s holds no //nolint directive the witness reads, but golangci-lint suppresses its finding", name))
			}
			for _, d := range ds {
				if !d.all && len(gatedNamed(d, gated)) == 0 {
					out = append(out, fmt.Sprintf("%s: golangci-lint suppresses the gated finding under %q, which the witness neither refuses nor counts", d.at, d.text))
				}
			}
		case strings.HasPrefix(name, "reported_"):
			seen["reported"]++
			for _, d := range ds {
				if d.all || len(gatedNamed(d, gated)) != 0 {
					out = append(out, fmt.Sprintf("%s: golangci-lint reports the gated finding under %q, which the witness nonetheless refuses or counts", d.at, d.text))
				}
			}
		default:
			out = append(out, fmt.Sprintf("%s is named neither suppressed_*.go nor reported_*.go, so its expected outcome is unknown", name))
		}
	}
	for _, kind := range []string{"suppressed", "reported"} {
		if seen[kind] == 0 {
			out = append(out, fmt.Sprintf("the probe module holds no %s_*.go file", kind))
		}
	}
	return out
}

// TestParseNolintDirective covers the recognition rule and the reading of a
// directive's list and reason, over comment texts as go/ast holds them.
func TestParseNolintDirective(t *testing.T) {
	cases := []struct {
		comment string
		ok      bool
		want    nolintDirective // text is the comment itself
	}{
		{comment: "//nolint", ok: true, want: nolintDirective{all: true}},
		{comment: "// nolint", ok: true, want: nolintDirective{all: true}},
		{comment: "///nolint", ok: true, want: nolintDirective{all: true}},
		{comment: "//nolint // why", ok: true, want: nolintDirective{all: true, reason: "why"}},
		{comment: "//nolint:all", ok: true, want: nolintDirective{all: true, linters: []string{"all"}}},
		{comment: "//nolint:allx", ok: true, want: nolintDirective{all: true, linters: []string{"allx"}}},
		{comment: "//nolint:unused,all // why", ok: true, want: nolintDirective{all: true, linters: []string{"unused", "all"}, reason: "why"}},
		{comment: "//nolint:unused, ALL", ok: true, want: nolintDirective{all: true, linters: []string{"unused", "all"}}},
		{comment: "//nolint:errorlint", ok: true, want: nolintDirective{linters: []string{"errorlint"}}},
		{comment: "//nolint:ErrorLint , noctx // a reason // more", ok: true, want: nolintDirective{linters: []string{"errorlint", "noctx"}, reason: "a reason // more"}},
		{comment: "//nolint:errorlint//why", ok: true, want: nolintDirective{linters: []string{"errorlint"}, reason: "why"}},
		{comment: "//nolint:errorlint //", ok: true, want: nolintDirective{linters: []string{"errorlint"}}},
		{comment: "//nolint:errorlint because", ok: true, want: nolintDirective{linters: []string{"errorlint because"}}},
		{comment: "//nolint:", ok: true, want: nolintDirective{linters: []string{""}}},
		// Not directives.
		{comment: "// see nolint"},
		{comment: "//nolintx"},
		{comment: "//NOLINT"},
		{comment: "//\tnolint"},
		{comment: "/* nolint */"},
		{comment: "/*nolint*/"},
		{comment: "// a mention of //nolint mid-comment"},
	}
	for _, tc := range cases {
		t.Run(tc.comment, func(t *testing.T) {
			got, ok := parseNolintDirective(tc.comment)
			if ok != tc.ok {
				t.Fatalf("parseNolintDirective(%q) is a directive: %v, want %v", tc.comment, ok, tc.ok)
			}
			want := tc.want
			if tc.ok {
				want.text = tc.comment
			}
			if got.at != "" || got.text != want.text || got.all != want.all || !slices.Equal(got.linters, want.linters) || got.reason != want.reason {
				t.Fatalf("parseNolintDirective(%q) = %+v, want %+v", tc.comment, got, want)
			}
		})
	}
}

// TestFileNolintDirectives covers reading a file's directives: every
// comment, each at its own line; and a file that does not parse is an error,
// never a file without directives.
func TestFileNolintDirectives(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    []string // each directive's at and text
		wantErr bool
	}{
		{name: "none", src: "package p\n\n// a comment\nvar x int\n"},
		{name: "several", src: "//nolint:errorlint // a\npackage p\n\n/*\nnolint\n*/\nvar x int //nolint\n\nfunc f() {\n\t// nolint:noctx\n}\n", want: []string{"f.go:1 //nolint:errorlint // a", "f.go:7 //nolint", "f.go:10 // nolint:noctx"}},
		{
			name: "contextcheck's reading of function declarations' docs",
			src: "package p\n\n// T is a type.\n//\n//nolint:unused // contextcheck: a type's doc\ntype T struct{}\n\n" +
				"// m is a method.\n//\n//nolint:unused // contextcheck: a reason\nfunc (T) m() {}\n\n" +
				"//\tnolint:contextcheck\nfunc f() {\n\t//nolint:unused // contextcheck: in a body\n}\n\n" +
				"// @contextcheck(req_has_ctx) // a reason\nfunc g() {}\n\n" +
				"//nolint:unused // ContextCheck: capitalised\nfunc h() {}\n",
			want: []string{
				"f.go:5 //nolint:unused // contextcheck: a type's doc",
				"f.go:10 //nolint:unused // contextcheck: a reason [contextcheck]",
				"f.go:13 //\tnolint:contextcheck [contextcheck]",
				"f.go:15 //nolint:unused // contextcheck: in a body",
				"f.go:18 // @contextcheck(req_has_ctx) // a reason [contextcheck]",
				"f.go:21 //nolint:unused // ContextCheck: capitalised",
			},
		},
		{name: "not Go", src: "package p\n\nvar x int //nolint\nfunc {\n", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ds, err := fileNolintDirectives("f.go", []byte(tc.src))
			if (err != nil) != tc.wantErr {
				t.Fatalf("fileNolintDirectives error = %v, want error %v", err, tc.wantErr)
			}
			var got []string
			for _, d := range ds {
				line := d.at + " " + d.text
				if slices.Contains(gatedNamed(d, []string{"contextcheck"}), "contextcheck") && !slices.Contains(d.linters, "contextcheck") {
					line += " [contextcheck]" // named by contextcheck's own reading alone
				}
				got = append(got, line)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("fileNolintDirectives = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestLintSetNolintDirectives covers the files the directives are read from:
// the module's linux/amd64 lint set, test files included, and never a file
// golangci-lint does not analyze there (another platform's, testdata's,
// another module's, or one the go tool ignores). A lint set that cannot be
// listed, is empty, or holds a file that does not parse is an error.
func TestLintSetNolintDirectives(t *testing.T) {
	const pkg = "package p\n\nvar x int //nolint\n"
	write := func(t *testing.T, root string, files map[string]string) {
		t.Helper()
		for f, content := range files {
			p := filepath.Join(root, filepath.FromSlash(f))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	module := t.TempDir()
	write(t, module, map[string]string{
		"go.mod": "module m\n", "a.go": pkg, "a_test.go": pkg, "pkg/r_linux.go": pkg,
		"pkg/r_darwin.go": pkg, "pkg/tagged_darwin_only.go": "//go:build darwin\n\n" + pkg,
		"testdata/x.go": pkg, "nested/go.mod": "module nested\n", "nested/x.go": pkg, "pkg/_x.go": pkg,
	})
	broken := t.TempDir()
	write(t, broken, map[string]string{"go.mod": "module m\n", "a.go": "package p\n\nfunc {\n"})
	empty := t.TempDir()
	write(t, empty, map[string]string{"go.mod": "module m\n", "doc.md": ""})

	cases := []struct {
		name    string
		root    string
		want    []string
		wantErr string
	}{
		{name: "a module tree", root: module, want: []string{"a.go:3", "a_test.go:3", "pkg/r_linux.go:3"}},
		{name: "a file that does not parse", root: broken, wantErr: "parsing a.go"},
		{name: "no Go file", root: empty, wantErr: "no Go file"},
		{name: "a root that does not exist", root: filepath.Join(module, "absent"), wantErr: "listing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ds, err := lintSetNolintDirectives(tc.root)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("lintSetNolintDirectives(%s) error = %v, want one containing %q", tc.root, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("lintSetNolintDirectives(%s): %v", tc.root, err)
			}
			var got []string
			for _, d := range ds {
				got = append(got, d.at)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("lintSetNolintDirectives(%s) = %q, want %q", tc.root, got, tc.want)
			}
		})
	}
}

// TestStrictConfigGatedLinters covers reading the gated set from a strict
// configuration: its linters.enable, sorted; a configuration that enables
// none, or is not YAML, is an error, never an empty gated set.
func TestStrictConfigGatedLinters(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    []string
		wantErr string
	}{
		{name: "the exclusion fixture", raw: strictExclusionFixture, want: []string{"containedctx", "contextcheck", "errorlint", "gochecknoglobals", "noctx"}},
		{name: "an unsorted enable list", raw: "version: \"2\"\nlinters:\n  enable:\n    - noctx\n    - errorlint\n", want: []string{"errorlint", "noctx"}},
		{name: "no linters.enable", raw: "version: \"2\"\nlinters:\n  default: none\n", wantErr: "enables no linter"},
		{name: "an empty linters.enable", raw: "version: \"2\"\nlinters:\n  enable: []\n", wantErr: "enables no linter"},
		{name: "not YAML", raw: "linters: [", wantErr: strictLintConfigFile},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := strictConfigGatedLinters(tc.raw)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("strictConfigGatedLinters error = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("strictConfigGatedLinters = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

// TestNolintProbeProblems covers the tie between the probe module's file
// names and the witness's reading (review finding S3-3): a sound module has
// no problem, and each way a probe file departs from its name is one,
// naming the file.
func TestNolintProbeProblems(t *testing.T) {
	const (
		counted  = "package p\n\nvar a int //nolint:gochecknoglobals // a reason\n"
		refused  = "package p\n\nvar a int //nolint:unused,all\n"
		ungated  = "package p\n\nvar a int //nolint:unused\n"
		noneRead = "package p\n\nvar a int // a mention of nolint mid-comment\n"
		ownRead  = "package p\n\n//nolint:contextchecks\nfunc f() {}\n"
	)
	cases := []struct {
		name  string
		files map[string]string
		want  []string // a substring of each problem, one each
	}{
		{name: "a sound module", files: map[string]string{"doc.go": "package p\n", "suppressed_counted.go": counted, "suppressed_refused.go": refused, "suppressed_contextcheck.go": ownRead, "reported_ungated.go": ungated, "reported_none.go": noneRead, "notes.txt": "", "sub/x.go": "package sub\n"}},
		{name: "a suppressed file holding an ungated directive", files: map[string]string{"suppressed_counted.go": counted, "suppressed_ungated.go": ungated, "reported_none.go": noneRead}, want: []string{"suppressed_ungated.go:3: golangci-lint suppresses the gated finding under \"//nolint:unused\", which the witness neither refuses nor counts"}},
		{name: "a suppressed file holding no directive", files: map[string]string{"suppressed_none.go": noneRead, "reported_none.go": noneRead}, want: []string{"suppressed_none.go holds no //nolint directive the witness reads"}},
		{name: "a reported file holding a counted directive", files: map[string]string{"suppressed_counted.go": counted, "reported_counted.go": counted}, want: []string{"reported_counted.go:3: golangci-lint reports the gated finding under \"//nolint:gochecknoglobals // a reason\", which the witness nonetheless refuses or counts"}},
		{name: "a reported file holding a refused directive", files: map[string]string{"suppressed_counted.go": counted, "reported_refused.go": refused}, want: []string{"reported_refused.go:3: golangci-lint reports the gated finding under \"//nolint:unused,all\""}},
		{name: "a file whose name has neither kind", files: map[string]string{"suppressed_counted.go": counted, "reported_none.go": noneRead, "probe_x.go": counted}, want: []string{"probe_x.go is named neither suppressed_*.go nor reported_*.go"}},
		{name: "no reported file", files: map[string]string{"suppressed_counted.go": counted}, want: []string{"the probe module holds no reported_*.go file"}},
		{name: "no suppressed file", files: map[string]string{"reported_none.go": noneRead}, want: []string{"the probe module holds no suppressed_*.go file"}},
		{name: "no probe file", files: map[string]string{"doc.go": "package p\n"}, want: []string{"holds no suppressed_*.go file", "holds no reported_*.go file"}},
	}
	gated := []string{"contextcheck", "gochecknoglobals"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				p := filepath.Join(dir, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			problems := nolintProbeProblems(t, dir, gated)
			if len(problems) != len(tc.want) {
				t.Fatalf("problems = %q, want %d, one containing each of %q", problems, len(tc.want), tc.want)
			}
			for _, w := range tc.want {
				if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, w) }) {
					t.Fatalf("problems = %q, want one containing %q", problems, w)
				}
			}
		})
	}
}

// TestContextcheckDocDirective covers contextcheck's own reading of one line
// of a function declaration's doc comment (ledger SI-337): its nolint
// pattern, one optional whitespace character after the //, with its name
// anywhere in the line, case-sensitively; or its request flag as a prefix.
// What follows the flag is where the line's reason is sought.
func TestContextcheckDocDirective(t *testing.T) {
	cases := []struct {
		comment string
		ok      bool
		rest    string
	}{
		{comment: "//nolint:unused // contextcheck: a reason", ok: true, rest: "unused // contextcheck: a reason"},
		{comment: "//nolint:contextcheck", ok: true, rest: "contextcheck"},
		{comment: "//nolint:contextchecks", ok: true, rest: "contextchecks"},
		{comment: "// nolint:staticcheck // contextcheck", ok: true, rest: "staticcheck // contextcheck"},
		{comment: "//\tnolint:contextcheck", ok: true, rest: "contextcheck"},
		{comment: "//nolint:all // contextcheck", ok: true, rest: "all // contextcheck"},
		{comment: "// @contextcheck(req_has_ctx)", ok: true, rest: ""},
		{comment: "// @contextcheck(req_has_ctx) // a reason", ok: true, rest: " // a reason"},
		{comment: "// @contextcheck(req_has_ctx)x", ok: true, rest: "x"},
		// Not contextcheck's.
		{comment: "//nolint:unused // a reason"},
		{comment: "//nolint:unused // ContextCheck"},
		{comment: "//NOLINT:contextcheck"},
		{comment: "//  nolint:contextcheck"},
		{comment: "///nolint:contextcheck"},
		{comment: "//nolint contextcheck"},
		{comment: "//nolint"},
		{comment: "/*nolint:contextcheck*/"},
		{comment: "// contextcheck"},
		{comment: "//@contextcheck(req_has_ctx)"},
		{comment: "// @contextcheck(req_has_ctx"},
		{comment: "// see @contextcheck(req_has_ctx)"},
	}
	for _, tc := range cases {
		t.Run(tc.comment, func(t *testing.T) {
			rest, ok := contextcheckDocDirective(tc.comment)
			if ok != tc.ok || rest != tc.rest {
				t.Fatalf("contextcheckDocDirective(%q) = %q, %v; want %q, %v", tc.comment, rest, ok, tc.rest, tc.ok)
			}
		})
	}
}
