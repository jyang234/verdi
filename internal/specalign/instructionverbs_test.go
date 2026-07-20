// AC-2 (spec/instruction-conformance): every `verdi <verb>` command
// reference inside an enumerated instruction file's backtick-delimited
// span — inline code and fenced code blocks alike (DC-1) — is extracted
// and validated against dispatch.go's own recognized-verb set, by execing
// the real built verdi binary with the extracted word as its sole
// argument from an empty, rootless temp directory (DC-2). Mirrors
// helpers_test.go's runBinary + package TestMain build-once precedent
// exactly: never `go run`, never importing cmd/verdi as a package.
//
// DC-1 scopes extraction to the literal `verdi <verb>` invocation shape
// only — never a bare backticked verb name with no "verdi " prefix. This
// repo's own root CLAUDE.md names CLI verbs as bare backticked words, no
// "verdi " prefix (its "gate", "board", ... sentence), so it passes this
// check VACUOUSLY today (zero references found, not zero references
// checked-and-clean) — a disclosed, deliberate scope limit, not a gap
// this file's tests paper over.
//
// DC-2 classifies a verb "known" unless stderr is EXACTLY dispatch.go's
// own top-level unknown-verb usage banner — captured LIVE (never a
// hardcoded duplicate of dispatch.go's `usage` const, which would drift
// silently if that text ever changes). This is deliberately coarser than
// helpers_test.go's own assertNotOutOfV0 (which asks "is this verb
// real/implemented"): it asks "does dispatch.go recognize this word at
// all", the right question for prose that may accurately describe a
// recognized-but-out-of-scope verb (waivers/verify-artifact).
package specalign

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// fencedCodeBlockRe matches a standard triple-backtick fenced code block
// — an optional language tag on the opening fence line, DOTALL so the
// body can span multiple lines — and captures its body in group 1.
// Four-or-more-backtick fences (a CommonMark extension for fencing a
// block that itself contains a triple-backtick example) are out of
// scope: neither of this repo's two real instruction files use one
// today, and AC-2's text only asks for "fenced code blocks", not every
// CommonMark fence variant.
var fencedCodeBlockRe = regexp.MustCompile("(?s)```[^\n]*\n(.*?)```")

// inlineCodeRe matches a single-backtick inline code span and captures
// its body. Double-backtick delimiters (CommonMark's escape for a
// literal backtick inside inline code) are out of scope for the same
// reason as fencedCodeBlockRe's four-backtick fences.
var inlineCodeRe = regexp.MustCompile("`([^`\n]+)`")

// extractBacktickSpans returns the text content of every backtick-
// delimited span in doc: fenced code blocks first (so a fenced block's
// own interior backticks, if any, are blanked out of the working copy
// and never double-counted as a separate inline span), then inline code
// spans over what remains. AC-2's exact extraction surface: "inline code
// and fenced code blocks alike". Order is fenced-blocks-first (in their
// own document order), then inline spans (in their own document order)
// — not strict document order across both kinds — since the only
// consumer, extractVerdiVerbRefs, treats its result as an unordered set
// of candidate verb tokens.
func extractBacktickSpans(doc string) []string {
	var spans []string

	working := []byte(doc)
	for _, idx := range fencedCodeBlockRe.FindAllStringSubmatchIndex(doc, -1) {
		spans = append(spans, doc[idx[2]:idx[3]])
		for i := idx[0]; i < idx[1]; i++ {
			if working[i] != '\n' {
				working[i] = ' '
			}
		}
	}

	for _, m := range inlineCodeRe.FindAllStringSubmatch(string(working), -1) {
		spans = append(spans, m[1])
	}
	return spans
}

// verdiVerbInvocationRe recognizes DC-1's literal `verdi <verb>`
// invocation shape: the word "verdi" followed by whitespace and a
// candidate verb token (the next contiguous run of non-whitespace
// characters) — never a bare backticked verb name with no "verdi "
// prefix.
var verdiVerbInvocationRe = regexp.MustCompile(`\bverdi\s+(\S+)`)

// extractVerdiVerbRefs returns every candidate verb token named by a
// literal `verdi <verb>` invocation inside doc's backtick-delimited spans
// (AC-2), possibly with duplicates (repeated mentions of the same verb
// are each returned) — a caller that wants a per-file de-duped set (the
// gate's own reporting shape) dedupes itself.
func extractVerdiVerbRefs(doc string) []string {
	var verbs []string
	for _, span := range extractBacktickSpans(doc) {
		for _, m := range verdiVerbInvocationRe.FindAllStringSubmatch(span, -1) {
			verbs = append(verbs, m[1])
		}
	}
	return verbs
}

// unknownVerbBanner captures dispatch.go's own top-level unknown-verb
// usage banner LIVE, by execing the once-built binary with a token that
// cannot plausibly ever be a real verb, from a fresh rootless temp
// directory — never a hardcoded duplicate of dispatch.go's `usage`
// const, so this classifier tracks dispatch.go's own text even if it
// changes (DC-2).
func unknownVerbBanner(t *testing.T) string {
	t.Helper()
	const probe = "__specalign_definitely_not_a_real_verdi_verb__"
	_, stderr, code := runBinary(t, t.TempDir(), probe)
	if code != 2 {
		t.Fatalf("capturing the unknown-verb banner via %q: exit = %d, want 2", probe, code)
	}
	if stderr == "" {
		t.Fatalf("capturing the unknown-verb banner via %q: stderr was empty", probe)
	}
	return stderr
}

// classifyVerb reports whether dispatch.go recognizes verb AT ALL (DC-2's
// coarse question — not helpers_test.go's assertNotOutOfV0, which asks
// the different "known AND implemented" question). known is true unless
// execing the binary with verb as its sole argument from a fresh
// rootless temp directory produces stderr EXACTLY equal to banner — a
// verb-specific usage error, an operational store-root failure,
// dispatch.go's own distinct "not implemented (out of v0 scope)" message
// (waivers/verify-artifact), or genuine success all count as known.
func classifyVerb(t *testing.T, verb, banner string) (known bool) {
	t.Helper()
	_, stderr, _ := runBinary(t, t.TempDir(), verb)
	return stderr != banner
}

func TestExtractBacktickSpans(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []string
	}{
		{"no backticks", "plain prose here, nothing to see", nil},
		{"empty doc", "", nil},
		{"single inline span", "run `verdi lint` now", []string{"verdi lint"}},
		{"multiple inline spans", "`verdi lint` and `verdi board`", []string{"verdi lint", "verdi board"}},
		{
			"fenced block, no language tag",
			"```\nverdi build start --kind feature\n```",
			[]string{"verdi build start --kind feature\n"},
		},
		{
			"fenced block with language tag",
			"```bash\nverdi lint\n```",
			[]string{"verdi lint\n"},
		},
		{
			"fenced block's own interior backtick is not double-counted as a separate inline span",
			"```\nsee `foo` inside a fence\n```",
			[]string{"see `foo` inside a fence\n"},
		},
		{
			"mixed inline and fenced",
			"inline `verdi lint` then a fence:\n```\nverdi board commit\n```\nand another inline `verdi gate`.",
			[]string{"verdi lint", "verdi board commit\n", "verdi gate"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sortedCopy(extractBacktickSpans(tc.doc))
			want := sortedCopy(tc.want)
			if len(got) != len(want) {
				t.Fatalf("extractBacktickSpans(%q) = %v, want %v", tc.doc, got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Errorf("extractBacktickSpans(%q) = %v, want %v", tc.doc, got, want)
				}
			}
		})
	}
}

func TestExtractVerdiVerbRefs(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want []string
	}{
		{"no verdi mentions", "just some prose about the project", nil},
		{"inline invocation, bare verb", "run `verdi lint` daily", []string{"lint"}},
		{
			"inline invocation with subcommand and flags — only the FIRST word after verdi is the verb (DC-1)",
			"driven by `verdi board commit <board-key> --name <spec-name>`",
			[]string{"board"},
		},
		{
			"fenced block invocation",
			"```\nverdi build start --kind feature --name foo\n```",
			[]string{"build"},
		},
		{
			"bare backticked verb with NO verdi prefix is NOT extracted (DC-1) — this repo's own CLAUDE.md shape",
			"CLI verbs: `gate`, `board`, `audit`, `close`, `gc`, `waivers`, `verify-artifact`.",
			nil,
		},
		{
			"verdi mentioned OUTSIDE any backtick span is NOT extracted (AC-2 scope: backtick-delimited spans only)",
			"run verdi lint before you push",
			nil,
		},
		{
			"verdi as a substring of another word does not match (word boundary)",
			"`averdiword lint`",
			nil,
		},
		{
			"multiple refs across inline and fenced spans in one doc",
			"inline `verdi lint`, then:\n```\nverdi gate\n```\nand `verdi audit` too.",
			[]string{"gate", "lint", "audit"},
		},
		{"empty doc", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sortedCopy(extractVerdiVerbRefs(tc.doc))
			want := sortedCopy(tc.want)
			if len(got) != len(want) {
				t.Fatalf("extractVerdiVerbRefs(%q) = %v, want %v", tc.doc, got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Errorf("extractVerdiVerbRefs(%q) = %v, want %v", tc.doc, got, want)
				}
			}
		})
	}
}

func sortedCopy(ss []string) []string {
	out := append([]string(nil), ss...)
	sort.Strings(out)
	return out
}

func TestUnknownVerbBanner(t *testing.T) {
	banner := unknownVerbBanner(t)
	if !strings.HasPrefix(banner, "usage: verdi <verb>") {
		t.Errorf("unknownVerbBanner() = %q, want it to start with dispatch.go's usage preamble", banner)
	}
}

// TestClassifyVerb is AC-2's both-directions proof at the classifier
// layer: a verb dispatch.go does not recognize at all classifies unknown;
// every shape of "recognized" DC-2 names — the special-cased "lint", an
// ordinary phase-gated verb, and the two verbs explicitly out of v0 scope
// (their own distinct "not implemented" message, never the banner) —
// classifies known.
func TestClassifyVerb(t *testing.T) {
	banner := unknownVerbBanner(t)

	tests := []struct {
		name string
		verb string
		want bool // known
	}{
		{"lint is real and special-cased ahead of the verbPhase lookup", "lint", true},
		{"board is real and dispatched (still — see AC-3, which alone catches the motivating defect)", "board", true},
		{"design is real, phase-gated, argument-parsing-first", "design", true},
		{"waivers is recognized but explicitly out of v0 scope (DC-2's own carve-out)", "waivers", true},
		{"verify-artifact is recognized but explicitly out of v0 scope", "verify-artifact", true},
		{"a definitely-fake verb is unknown", "frobnicate-nonexistent-verb", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyVerb(t, tc.verb, banner)
			if got != tc.want {
				t.Errorf("classifyVerb(%q) = %v, want %v", tc.verb, got, tc.want)
			}
		})
	}
}
