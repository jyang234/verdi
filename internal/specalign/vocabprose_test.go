// The L-M13a(6) mechanical prose witness (ledger, docs/design/plans/
// 2026-07-17-extensibility-phase1-plan.md: "a mechanical prose-witness
// (production string literals scanned for class words; every hit routed
// or classification-marked) — sweeps don't converge, enforcement does").
//
// TestVocabProseWitness scans every PRODUCTION Go string literal under
// cmd/ and internal/ for the operating model's class words (canonical
// classes + the spike pseudo-class, singular and display-plural) and
// lifecycle state words spoken as prose, and requires every hit to be
// either
//
//   - ROUTED: its own enclosing statement (or top-level declaration)
//     visibly engages the display chain — a Display*/display* call,
//     model.Indefinite/Article/Capitalize, the workbench classWords
//     methods (word/plural/capital/indefinite), or an identifier ending
//     in Word/Words (the house convention for resolved display-word
//     locals: classWord, featureWord, ...); or
//   - MARKED: the one mechanical classification marker comment,
//     `// vocab:identity — <why>`, on the literal's own starting line or
//     the line directly above it, placed AT the producing site (never in
//     a consumer package). The marker asserts the bare word is
//     deliberate: an identity-layer id (ref/usage grammar, wire enum
//     value, frontmatter field, branch/CSS/testid fragment, commit
//     subject), a machinery diagnostic speaking ids, or a non-vocabulary
//     homograph — never unclassified display prose.
//
// The word lists are DERIVED from the embedded canonical model at run
// time (model.Canonical()), never hand-maintained: declared class ids
// plus "spike" (the L-M13 pseudo-class) with their display plurals, and
// the union of every lifecycle's state ids.
//
// Mechanical rules, and their disclosed limits:
//
//   - Only string literals with INTERIOR whitespace are scanned: a
//     single-token literal ("draft", "feature/", " stubcard--spike") is
//     an id in every legitimate grammar (enum value, ref, prefix, CSS
//     class), never prose. A bare display word smuggled out as a padded
//     token would evade this — the witness is a ratchet against prose
//     leaks, not an adversarial-proof gate.
//   - A match flanked by compound punctuation (_ . / : % \ < >) is an
//     identity compound (story:, story.violated, feature/<name>,
//     spec/<story>, %sstory) and is skipped. '-' is deliberately NOT in
//     that set: hyphenated display compounds ("a feature-class wall",
//     the ledgered boardspecapi hit) must not hide from the witness, so
//     hyphen-joined identity fragments (--story-ref, stub-story-link)
//     carry markers instead.
//   - The stock posture phrases "fail closed"/"fails closed"/"failing
//     closed"/"fail-closed" are stripped before matching: that "closed"
//     is the house fail-closed rule's own word, never the lifecycle
//     state.
//   - The routed heuristic reads the enclosing statement's SOURCE TEXT
//     with its string literals and comments blanked; an author binding a
//     display word to an oddly-named variable in a separate statement
//     defeats it and must use the marker instead.
//   - Scope is production .go files under cmd/ and internal/ only.
//     Excluded, each deliberately: _test.go files and testdata/ (test
//     fixtures legitimately pin bare ids); cmd/e2eharness and
//     internal/provider/providertest (test scaffolding, not shipped
//     surface); internal/model itself (the display chain's own home —
//     its kernel diagnostics speak class/state IDS by construction).
//     Out of mechanical reach entirely — JS assets (internal/workbench/
//     assets), HTML template literals already covered as Go strings
//     aside, e2e TypeScript — the client-side prose seam is instead
//     proven by the vocabulary e2e suite (45-vocabulary.spec.ts).
package specalign

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
)

// vocabIdentityMarker is the ONE mechanical classification marker
// (documented in model.DisplayClass's ENUMERATION RULE): its presence on
// a hit literal's starting line, or the line directly above, classifies
// that literal's bare vocabulary words as deliberate identity-layer /
// non-display usage.
const vocabIdentityMarker = "vocab:identity"

// vocabStockPhrases are stripped (replaced by spaces) before word
// matching — the house fail-closed posture's own "closed", never the
// lifecycle state.
var vocabStockPhrases = []string{"fail closed", "fails closed", "failing closed", "fail-closed"}

// vocabRouteRe recognizes a statement that visibly engages the display
// chain (see the file doc comment's ROUTED bullet). Applied to statement
// source text whose string literals and comments have been blanked, so
// literal or comment text can never self-route a statement.
var vocabRouteRe = regexp.MustCompile(`\b[Dd]isplay[A-Za-z]*\s*\(|\bIndefinite\(|\bArticle\(|\bCapitalize\(|\bclassWords\b|\.word\(|\.plural\(|\.capital\(|\.indefinite\(|\b[A-Za-z0-9_]*[Ww]ords?\b`)

// vocabProseWords derives the witness's word list from the embedded
// canonical model: every declared class id and its display plural, the
// spike pseudo-class and its plural (L-M13 rule 3), and the union of
// every lifecycle's state ids. Sorted longest-first so a longer word is
// matched (and blanked) before any shorter word could hit inside it.
func vocabProseWords() []string {
	mdl := model.Canonical()
	seen := map[string]bool{}
	var words []string
	add := func(w string) {
		w = strings.ToLower(w)
		if w != "" && !seen[w] {
			seen[w] = true
			words = append(words, w)
		}
	}
	for id := range mdl.Classes {
		add(id)
		add(mdl.DisplayClassPlural(id))
	}
	add("spike")
	add(mdl.DisplayClassPlural("spike"))
	for _, lc := range mdl.Lifecycle {
		for _, s := range lc.States {
			add(s)
		}
	}
	sort.Slice(words, func(i, j int) bool {
		if len(words[i]) != len(words[j]) {
			return len(words[i]) > len(words[j])
		}
		return words[i] < words[j]
	})
	return words
}

func isVocabWordChar(r byte) bool {
	return r == '_' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isVocabCompoundChar reports punctuation that welds a vocabulary word
// into an identity compound. '-' is deliberately absent — see the file
// doc comment.
func isVocabCompoundChar(r byte) bool {
	switch r {
	case '_', '.', '/', ':', '%', '\\', '<', '>':
		return true
	}
	return false
}

func stripVocabStockPhrases(lower string) string {
	for _, p := range vocabStockPhrases {
		for {
			i := strings.Index(lower, p)
			if i < 0 {
				break
			}
			lower = lower[:i] + strings.Repeat(" ", len(p)) + lower[i+len(p):]
		}
	}
	return lower
}

// vocabMatchWords returns every vocabulary word spoken bare in s (case-
// insensitive, whole-word, compound punctuation excluded, stock phrases
// stripped). words must be sorted longest-first (vocabProseWords);
// each match is blanked so a shorter word never re-hits inside it.
func vocabMatchWords(s string, words []string) []string {
	lower := stripVocabStockPhrases(strings.ToLower(s))
	var hits []string
	for _, w := range words {
		for i := 0; ; {
			j := strings.Index(lower[i:], w)
			if j < 0 {
				break
			}
			start := i + j
			end := start + len(w)
			okBefore := start == 0 || (!isVocabWordChar(lower[start-1]) && !isVocabCompoundChar(lower[start-1]))
			okAfter := end == len(lower) || (!isVocabWordChar(lower[end]) && !isVocabCompoundChar(lower[end]))
			if okBefore && okAfter {
				hits = append(hits, w)
				lower = lower[:start] + strings.Repeat(" ", len(w)) + lower[end:]
			}
			i = end
		}
	}
	return hits
}

// vocabViolation is one unrouted, unmarked bare vocabulary hit.
type vocabViolation struct {
	File    string // module-root-relative, slash-separated
	Line    int    // the literal's STARTING line
	Words   []string
	Literal string
}

// scanVocabProse scans one production Go source file and returns its
// violations. rel is the module-root-relative path used in reports.
func scanVocabProse(rel string, src []byte, words []string) ([]vocabViolation, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", rel, err)
	}
	lines := strings.Split(string(src), "\n")
	lineHasMarker := func(n int) bool {
		return n >= 1 && n <= len(lines) && strings.Contains(lines[n-1], vocabIdentityMarker)
	}

	var out []vocabViolation
	var stack []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		val, uerr := strconv.Unquote(lit.Value)
		if uerr != nil {
			return true
		}
		// Single tokens (padding aside) are ids, never prose — the
		// interior-whitespace rule.
		if !strings.ContainsAny(strings.TrimSpace(val), " \t\n") {
			return true
		}
		hits := vocabMatchWords(val, words)
		if len(hits) == 0 {
			return true
		}

		// The enclosing unit: innermost statement, else the enclosing
		// top-level declaration (const/var blocks).
		var encl ast.Node
		for i := len(stack) - 2; i >= 0; i-- {
			if _, isStmt := stack[i].(ast.Stmt); isStmt {
				encl = stack[i]
				break
			}
		}
		if encl == nil {
			for i := len(stack) - 2; i >= 0; i-- {
				if _, isDecl := stack[i].(ast.Decl); isDecl {
					encl = stack[i]
					break
				}
			}
		}
		if encl != nil && vocabRouteRe.MatchString(blankedUnitText(fset, f, src, encl)) {
			return true
		}

		pos := fset.Position(lit.Pos())
		if lineHasMarker(pos.Line) || lineHasMarker(pos.Line-1) {
			return true
		}
		out = append(out, vocabViolation{File: rel, Line: pos.Line, Words: hits, Literal: val})
		return true
	})
	return out, nil
}

// blankedUnitText returns unit's source text with every string literal
// and every comment inside its span blanked to spaces (newlines kept),
// so only real code tokens can satisfy vocabRouteRe.
func blankedUnitText(fset *token.FileSet, f *ast.File, src []byte, unit ast.Node) string {
	s := fset.Position(unit.Pos()).Offset
	e := fset.Position(unit.End()).Offset
	if s < 0 || e > len(src) || s >= e {
		return ""
	}
	buf := []byte(string(src[s:e]))
	blank := func(from, to token.Pos) {
		bs := fset.Position(from).Offset - s
		be := fset.Position(to).Offset - s
		for k := max(bs, 0); k < be && k < len(buf); k++ {
			if buf[k] != '\n' {
				buf[k] = ' '
			}
		}
	}
	ast.Inspect(unit, func(m ast.Node) bool {
		if bl, ok := m.(*ast.BasicLit); ok && bl.Kind == token.STRING {
			blank(bl.Pos(), bl.End())
		}
		return true
	})
	for _, cg := range f.Comments {
		if cg.End() < unit.Pos() || cg.Pos() > unit.End() {
			continue
		}
		blank(cg.Pos(), cg.End())
	}
	return string(buf)
}

// vocabProseSkipDir reports directories the witness deliberately does
// not scan — each exclusion justified in the file doc comment.
func vocabProseSkipDir(root, path string) bool {
	switch filepath.Base(path) {
	case "testdata", "node_modules":
		return true
	}
	switch path {
	case filepath.Join(root, "cmd", "e2eharness"),
		filepath.Join(root, "internal", "model"),
		filepath.Join(root, "internal", "provider", "providertest"):
		return true
	}
	return false
}

// TestVocabProseWitness is the module-wide gate: zero unrouted, unmarked
// bare vocabulary words in production string literals.
func TestVocabProseWitness(t *testing.T) {
	root := verdiRepoRoot
	words := vocabProseWords()
	if len(words) == 0 {
		t.Fatal("vocabProseWords() returned no words — the canonical model resolved empty, the witness would vacuously pass")
	}

	var violations []vocabViolation
	for _, tree := range []string{"cmd", "internal"} {
		err := filepath.Walk(filepath.Join(root, tree), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if vocabProseSkipDir(root, path) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			vs, serr := scanVocabProse(filepath.ToSlash(rel), src, words)
			if serr != nil {
				return serr
			}
			violations = append(violations, vs...)
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", tree, err)
		}
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].File != violations[j].File {
			return violations[i].File < violations[j].File
		}
		return violations[i].Line < violations[j].Line
	})
	for _, v := range violations {
		lit := v.Literal
		if len(lit) > 120 {
			lit = lit[:120] + "..."
		}
		t.Errorf("%s:%d: bare vocabulary word(s) %v in production string literal %q — route the word through the model display chain on this statement (model.DisplayClass and friends, spec/vocabulary-surfaces), or classify the site with `// %s — <why>` on the literal's line or the line above (model.DisplayClass's ENUMERATION RULE; ledger L-M13a(6))", v.File, v.Line, v.Words, lit, vocabIdentityMarker)
	}
}

// TestVocabProseWords proves the derived word list carries the canonical
// classes, the spike pseudo-class, display plurals, and every canonical
// lifecycle state — and is longest-first (the matcher's precondition).
func TestVocabProseWords(t *testing.T) {
	words := vocabProseWords()
	got := map[string]bool{}
	for _, w := range words {
		got[w] = true
	}
	for _, want := range []string{
		"feature", "features", "story", "stories", "spike", "spikes",
		"draft", "accepted-pending-build", "closed", "superseded",
	} {
		if !got[want] {
			t.Errorf("vocabProseWords() is missing %q (have %v)", want, words)
		}
	}
	for i := 1; i < len(words); i++ {
		if len(words[i-1]) < len(words[i]) {
			t.Fatalf("vocabProseWords() not sorted longest-first: %q before %q", words[i-1], words[i])
		}
	}
}

// TestVocabMatchWords is the matcher's own happy/negative table.
func TestVocabMatchWords(t *testing.T) {
	words := vocabProseWords()
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"display prose hit", "only a story spec can be accepted", []string{"story"}},
		{"capitalized display hit", "each Story is its own spec", []string{"story"}},
		{"plural display hit", "a feature never lists its stories", []string{"stories", "feature"}},
		{"state word as display", "an accepted-pending-build spec here", []string{"accepted-pending-build"}},
		{"hyphenated display compound", "only creatable on a feature-class wall", []string{"feature"}},
		{"frontmatter field compound skipped", "no active spec has story: %s", nil},
		{"ref grammar compound skipped", "resolves the whole spec/<story> ref", nil},
		{"dotted wire key skipped", "per-AC status plus story.violated here", nil},
		{"stock phrase stripped", "an empty sequence cannot exist; fail closed", nil},
		{"hyphenated stock phrase stripped", "does not map to any group (fail-closed) here", nil},
		{"embedded substring not a word", "the FoldFeature seam and disclosed text", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vocabMatchWords(tt.in, words)
			if len(got) != len(tt.want) {
				t.Fatalf("vocabMatchWords(%q) = %v, want %v", tt.in, got, tt.want)
			}
			gotSet := map[string]bool{}
			for _, w := range got {
				gotSet[w] = true
			}
			for _, w := range tt.want {
				if !gotSet[w] {
					t.Fatalf("vocabMatchWords(%q) = %v, want it to include %q", tt.in, got, w)
				}
			}
		})
	}
}

// TestScanVocabProse_Classifier proves the three-way classification on
// synthetic sources: a routed statement passes, a marked literal passes
// (both marker positions), tokens pass, and an unmarked bare display
// literal FAILS — the witness's negative path, the same failure a live
// mutation of a routed production site produces.
func TestScanVocabProse_Classifier(t *testing.T) {
	words := vocabProseWords()
	tests := []struct {
		name      string
		src       string
		wantFiles int // number of violations
		wantLine  int // when wantFiles == 1
	}{
		{
			name: "unmarked bare display literal fails",
			src: `package p
func f() string {
	return "only a story spec can be accepted"
}
`,
			wantFiles: 1,
			wantLine:  3,
		},
		{
			name: "statement routed through DisplayClass passes",
			src: `package p
func f(mdl M) string {
	return sprintf("only %s spec can be accepted (no story, no acceptance criteria)", mdl.DisplayClass("story"))
}
`,
			wantFiles: 0,
		},
		{
			name: "statement referencing a resolved xxxWord local passes",
			src: `package p
func f(storyWord string) string {
	return sprintf("no active spec has that story here %s", storyWord)
}
`,
			wantFiles: 0,
		},
		{
			name: "marker on the literal's line passes",
			src: `package p
func f() string {
	return "usage: verdi feature start <ref>" // vocab:identity — CLI verb name
}
`,
			wantFiles: 0,
		},
		{
			name: "marker on the line above passes",
			src: `package p
func f() string {
	// vocab:identity — CLI verb name
	return "usage: verdi feature start <ref>"
}
`,
			wantFiles: 0,
		},
		{
			name: "marker inside the literal itself does not classify",
			src: `package p
func f() string {
	return "a story spec mentioning vocab:identity" +
		"continues here"
}
`,
			// The literal's own text contains the marker token on its
			// line, so the line-based check clears it — this documents
			// that the marker's grain is the LINE, and a literal
			// speaking the marker's name is self-classifying. Accepted:
			// no production literal speaks the marker.
			wantFiles: 0,
		},
		{
			name: "single-token literal passes untouched",
			src: `package p
var x = map[string]bool{"story": true}
func f(s string) bool { return s == "draft" }
`,
			wantFiles: 0,
		},
		{
			name: "padded token literal passes (interior-whitespace rule)",
			src: `package p
func f(b buf) {
	b.WriteString(" stubcard--spike")
}
`,
			wantFiles: 0,
		},
		{
			name: "top-level const with bare display prose fails",
			src: `package p
const usage = "operate on a story spec"
`,
			wantFiles: 1,
			wantLine:  2,
		},
		{
			name: "comment text inside the statement cannot self-route",
			src: `package p
func f() string {
	return sprintf(
		// these words are display, someone claims
		"only a story spec can be accepted")
}
`,
			wantFiles: 1,
			wantLine:  5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scanVocabProse("synth.go", []byte(tt.src), words)
			if err != nil {
				t.Fatalf("scanVocabProse: %v", err)
			}
			if len(got) != tt.wantFiles {
				t.Fatalf("scanVocabProse violations = %d (%v), want %d", len(got), got, tt.wantFiles)
			}
			if tt.wantFiles == 1 && got[0].Line != tt.wantLine {
				t.Fatalf("violation line = %d, want %d (%v)", got[0].Line, tt.wantLine, got[0])
			}
		})
	}
}
