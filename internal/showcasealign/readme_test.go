package showcasealign

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestReadmeExamplesFresh re-runs every root-README console block tagged
// <!-- showcase-verify --> against a freshly provisioned showcase store and
// requires byte-identical (trailing-whitespace-normalized) output — the
// README-freshness drift gate spec/public-showcase ac-3 requires (ledger
// L-D, docs/design/plans/2026-07-14-public-rollout-plan.md). A stale pasted
// example — one that no longer reproduces because the showcase, a verb's
// output, or the corpus content moved underneath it — is a defect this test
// exists to catch, exactly as TestShowcaseLintClean catches a lint
// regression and TestShowcaseCoverage catches an unshowcased capability.
//
// The store the blocks run against is provisionShowcaseStore's fixturegit
// reconstruction (helpers_test.go) — the canonical showcase store with real
// git history — NOT a raw `cd examples/showcase`, whose frozen pins do not
// resolve (see the README's own note and examples/showcase/README.md). Every
// tagged command must therefore be one whose output is identical on both:
// `verdi matrix` reads the working tree and reproduces on either; `verdi
// lint`'s zero-findings result is the gate's, framed as such in the README.
//
// Tag format, in README.md:
//
//	<!-- showcase-verify -->
//	```console
//	$ verdi lint
//	<expected stdout, verbatim — empty here means the store lints clean>
//	```
//
// and, for a command expected to exit non-zero:
//
//	<!-- showcase-verify exit=1 -->
//	```console
//	$ verdi <args>
//	<expected stdout, verbatim>
//	```
func TestReadmeExamplesFresh(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join(verdiRepoRoot, "README.md"))
	if err != nil {
		t.Fatalf("README.md unreadable: %v", err)
	}
	blocks := parseVerifyBlocks(t, string(readme))
	if len(blocks) == 0 {
		t.Fatal("README has no <!-- showcase-verify --> blocks; the quick start must be verified against the showcase store (spec/public-showcase ac-3, design §3)")
	}
	store := provisionShowcaseStore(t)
	for _, b := range blocks {
		cmd := strings.Join(b.argv, " ")
		stdout, stderr, code := runBinary(t, store, b.argv[1:]...) // argv[0] == "verdi"
		switch {
		case b.expectNonzero && code == 0:
			t.Errorf("README.md:%d: %q was tagged exit=1 but exited 0\nstdout:\n%s", b.line, cmd, stdout)
		case !b.expectNonzero && code != 0:
			t.Errorf("README.md:%d: %q exited %d (untagged for non-zero)\nstderr:\n%s", b.line, cmd, code, stderr)
		}
		if got := normalizeTrailingWS(stdout); got != b.wantOut {
			t.Errorf("README.md:%d: %q output drifted from the README paste.\n--- want (README) ---\n%s\n--- got (fresh run) ---\n%s\n--- end ---", b.line, cmd, b.wantOut, got)
		}
	}
}

// verifyBlock is one parsed <!-- showcase-verify --> console block: the
// command to run (argv[0] is always "verdi") and its expected, normalized
// stdout, plus whether the block was tagged exit=1 (an expected non-zero
// exit) and the 1-based README line of its tag for diagnostics.
type verifyBlock struct {
	argv          []string
	wantOut       string
	expectNonzero bool
	line          int
}

// verifyTagRe matches a well-formed showcase-verify tag line (already
// whitespace-trimmed): the bare form, or the exit=1 form. verifyMarkerRe is
// the looser "this HTML comment is TRYING to be a tag" probe used only to
// turn a near-miss (a typo like exit=2, or a misspelled marker) into a loud
// t.Fatalf rather than a silent skip.
var (
	verifyTagRe    = regexp.MustCompile(`^<!--\s*showcase-verify(\s+exit=1)?\s*-->$`)
	verifyMarkerRe = regexp.MustCompile(`showcase-verify`)
)

// parseVerifyBlocks scans README.md line by line for showcase-verify tags
// and returns one verifyBlock per tag. It is deliberately strict: every
// structural expectation the tag format sets up — tag immediately followed
// by a ```console fence, whose first line is `$ verdi <args>`, whose
// remaining lines up to the closing ``` fence are the expected stdout — is a
// t.Fatalf if unmet, because a malformed block is an authoring error in the
// README that must fail the gate loudly, never be skipped into a false pass.
// An HTML comment that mentions "showcase-verify" but does not match the
// exact grammar is likewise a fatal malformed tag.
//
// Only the FIRST line inside the fence is the command; every later line is
// expected output. One command per block (a block with two `$ ` lines makes
// the second line part of the expected stdout, which will simply not match —
// caught, not silently accepted). Arguments split on whitespace; no quoted
// arguments are supported because none are needed (the showcased verbs take
// bare refs).
func parseVerifyBlocks(t *testing.T, readme string) []verifyBlock {
	t.Helper()
	lines := strings.Split(readme, "\n")
	var blocks []verifyBlock

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !verifyTagRe.MatchString(trimmed) {
			if strings.HasPrefix(trimmed, "<!--") && verifyMarkerRe.MatchString(trimmed) {
				t.Fatalf("README.md:%d: malformed showcase-verify tag %q; grammar is `<!-- showcase-verify -->` or `<!-- showcase-verify exit=1 -->`", i+1, trimmed)
			}
			continue
		}
		tagLine := i + 1
		expectNonzero := strings.Contains(trimmed, "exit=1")

		// The tag must be immediately followed by a ```console fence.
		i++
		if i >= len(lines) || strings.TrimSpace(lines[i]) != "```console" {
			got := "<end of file>"
			if i < len(lines) {
				got = strings.TrimSpace(lines[i])
			}
			t.Fatalf("README.md:%d: showcase-verify tag must be immediately followed by a ```console fence, found %q", tagLine, got)
		}

		// The first line inside the fence is the command: `$ verdi <args>`.
		i++
		if i >= len(lines) {
			t.Fatalf("README.md:%d: showcase-verify block is unterminated (no command line after the ```console fence)", tagLine)
		}
		cmdLine := lines[i]
		if !strings.HasPrefix(cmdLine, "$ ") {
			t.Fatalf("README.md:%d: showcase-verify command line must start with `$ `, got %q", i+1, cmdLine)
		}
		argv := strings.Fields(strings.TrimPrefix(cmdLine, "$ "))
		if len(argv) == 0 || argv[0] != "verdi" {
			t.Fatalf("README.md:%d: showcase-verify command must invoke `verdi`, got %q", i+1, cmdLine)
		}

		// Remaining lines up to the closing ``` fence are the expected stdout.
		var out []string
		closed := false
		for i++; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "```" {
				closed = true
				break
			}
			out = append(out, lines[i])
		}
		if !closed {
			t.Fatalf("README.md:%d: showcase-verify ```console block is unterminated (no closing ``` fence)", tagLine)
		}

		blocks = append(blocks, verifyBlock{
			argv:          argv,
			wantOut:       normalizeTrailingWS(strings.Join(out, "\n")),
			expectNonzero: expectNonzero,
			line:          tagLine,
		})
	}
	return blocks
}

// normalizeTrailingWS strips trailing spaces/tabs from every line and any
// trailing blank lines from the whole string, so a byte-for-byte comparison
// of command output against a README paste is not defeated by an invisible
// trailing space a table's column padding or an editor's final-newline habit
// introduced. Internal spacing (the column alignment `verdi matrix` emits) is
// significant and preserved — that is real output structure, not whitespace
// noise. Applied identically to the fresh run's stdout and to every parsed
// wantOut so both sides are normalized the same way.
func normalizeTrailingWS(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}
