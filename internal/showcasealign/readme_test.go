package showcasealign

import (
	"fmt"
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
		if !exitCodeMatches(b.wantExit1, code) {
			if b.wantExit1 {
				t.Errorf("README.md:%d: %q was tagged exit=1 but exited %d — exit=1 requires EXACTLY code 1 (the verdict exit under this repo's 0-clean/1-verdict/2-operational contract); an operational failure (code 2) must NOT satisfy it\nstdout:\n%s\nstderr:\n%s", b.line, cmd, code, stdout, stderr)
			} else {
				t.Errorf("README.md:%d: %q exited %d (an untagged block must exit 0 clean)\nstderr:\n%s", b.line, cmd, code, stderr)
			}
		}
		if got := normalizeTrailingWS(stdout); got != b.wantOut {
			t.Errorf("README.md:%d: %q output drifted from the README paste.\n--- want (README) ---\n%s\n--- got (fresh run) ---\n%s\n--- end ---", b.line, cmd, b.wantOut, got)
		}
	}
}

// verifyBlock is one parsed <!-- showcase-verify --> console block: the
// command to run (argv[0] is always "verdi") and its expected, normalized
// stdout, plus whether the block was tagged exit=1 (which requires EXACTLY
// exit code 1 — the verdict exit; see exitCodeMatches) and the 1-based README
// line of its tag for diagnostics.
type verifyBlock struct {
	argv      []string
	wantOut   string
	wantExit1 bool
	line      int
}

// verifyTagRe matches a well-formed showcase-verify tag line (already
// whitespace-trimmed): the bare form, or the exit=1 form. verifyMarkerRe is
// the looser "this HTML comment is TRYING to be a tag" probe: any comment
// naming "showcase" immediately followed (across at most a few separator
// characters) by a "ver..." token. It exists ONLY to turn a near-miss into a
// loud failure rather than a silent skip, and it now delivers on that for BOTH
// classes the doc formerly overstated: a correctly-spelled marker carrying bad
// grammar (exit=2) AND a MISSPELLED marker WORD (showcase-verfiy, showcase_verify
// — the exact typos a hand-authored README produces) each match the probe, fail
// verifyTagRe, and are rejected loudly (co-2: a malformed tag is never a vacuous
// green). A well-formed tag matches verifyTagRe first and never reaches the
// probe; an innocuous HTML comment (no "showcase...ver") is skipped. The probe
// deliberately keeps "showcase" intact — a first-word typo like "shocase" is
// outside its scope, documented here rather than silently claimed.
var (
	verifyTagRe    = regexp.MustCompile(`^<!--\s*showcase-verify(\s+exit=1)?\s*-->$`)
	verifyMarkerRe = regexp.MustCompile(`(?i)showcase[-_\s.]{0,3}ver`)
)

// parseVerifyBlocks is the gate's entry point: it scans README.md for
// showcase-verify tags and returns one verifyBlock per tag, or fails the test
// loudly (t.Fatalf) on any malformed block. The parsing itself lives in the
// pure parseVerifyBlocksErr so every rejection path is directly unit-testable —
// a t.Fatalf parser cannot have its rejections asserted (TestParseVerifyBlocksErr
// drives the pure form and checks each error).
func parseVerifyBlocks(t *testing.T, readme string) []verifyBlock {
	t.Helper()
	blocks, err := parseVerifyBlocksErr(readme)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return blocks
}

// parseVerifyBlocksErr is the pure, strict README parser. It is deliberately
// strict: every structural expectation the tag format sets up — tag immediately
// followed by a ```console fence, whose first line is `$ verdi <args>`, whose
// remaining lines up to the closing ``` fence are the expected stdout — is a
// returned error if unmet, because a malformed block is an authoring error that
// must fail the gate loudly, never be skipped into a false pass. It rejects, in
// particular, three near-miss shapes a hand-authored README produces:
//
//   - a comment TRYING to be a tag but not matching the exact grammar,
//     including a MISSPELLED marker word (showcase-verfiy, showcase_verify):
//     verifyMarkerRe's fuzzy probe now catches these where a literal-string
//     probe silently dropped them from the gate (co-2's "malformed tag, never a
//     vacuous green");
//   - a tagged block that is a multi-command terminal TRANSCRIPT (a second
//     `$ ` command line inside the fence) rather than the single-command,
//     stdout-verbatim grammar dc-1 defines. This reconciles the two conventions
//     the README uses: a tagged block is ONE argv plus its stdout (diffed by
//     the gate), while a transcript interleaves commands and stderr (the
//     "Start your own store" scaffold block) and must stay untagged. A
//     transcript is rejected here with a message pointing the author to untag
//     it or split it into one tagged block per command, rather than silently
//     folding the second command into the first's expected stdout;
//   - the pre-existing structural malformations (no fence, non-`$ ` first line,
//     non-verdi command, unterminated fence).
//
// Only the FIRST line inside the fence is the command; every later line is
// expected stdout. Arguments split on whitespace; no quoted arguments are
// supported because none are needed (the showcased verbs take bare refs).
func parseVerifyBlocksErr(readme string) ([]verifyBlock, error) {
	lines := strings.Split(readme, "\n")
	var blocks []verifyBlock

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !verifyTagRe.MatchString(trimmed) {
			if strings.HasPrefix(trimmed, "<!--") && verifyMarkerRe.MatchString(trimmed) {
				return nil, fmt.Errorf("README.md:%d: malformed showcase-verify tag %q; grammar is `<!-- showcase-verify -->` or `<!-- showcase-verify exit=1 -->` (a misspelled marker word or bad grammar is a hard failure, never a silent skip)", i+1, trimmed)
			}
			continue
		}
		tagLine := i + 1
		wantExit1 := strings.Contains(trimmed, "exit=1")

		// The tag must be immediately followed by a ```console fence.
		i++
		if i >= len(lines) || strings.TrimSpace(lines[i]) != "```console" {
			got := "<end of file>"
			if i < len(lines) {
				got = strings.TrimSpace(lines[i])
			}
			return nil, fmt.Errorf("README.md:%d: showcase-verify tag must be immediately followed by a ```console fence, found %q", tagLine, got)
		}

		// The first line inside the fence is the command: `$ verdi <args>`.
		i++
		if i >= len(lines) {
			return nil, fmt.Errorf("README.md:%d: showcase-verify block is unterminated (no command line after the ```console fence)", tagLine)
		}
		cmdLine := lines[i]
		if !strings.HasPrefix(cmdLine, "$ ") {
			return nil, fmt.Errorf("README.md:%d: showcase-verify command line must start with `$ `, got %q", i+1, cmdLine)
		}
		argv := strings.Fields(strings.TrimPrefix(cmdLine, "$ "))
		if len(argv) == 0 || argv[0] != "verdi" {
			return nil, fmt.Errorf("README.md:%d: showcase-verify command must invoke `verdi`, got %q", i+1, cmdLine)
		}

		// Remaining lines up to the closing ``` fence are the expected stdout.
		// A second `$ ` command line here means the block is a multi-command
		// terminal transcript, NOT the single-command stdout grammar a tagged
		// block is (dc-1 diffs stdout for ONE argv; a transcript conflates argv,
		// stdout, and stderr): reject it rather than silently fold it into the
		// expected stdout.
		var out []string
		closed := false
		for i++; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "```" {
				closed = true
				break
			}
			if strings.HasPrefix(lines[i], "$ ") {
				return nil, fmt.Errorf("README.md:%d: showcase-verify block carries a second command line %q — a tagged block is a single-command, stdout-verbatim grammar (dc-1), not a multi-command terminal transcript; untag it (transcripts stay illustrative, like the scaffold block) or split it into one tagged block per command", i+1, lines[i])
			}
			out = append(out, lines[i])
		}
		if !closed {
			return nil, fmt.Errorf("README.md:%d: showcase-verify ```console block is unterminated (no closing ``` fence)", tagLine)
		}

		blocks = append(blocks, verifyBlock{
			argv:      argv,
			wantOut:   normalizeTrailingWS(strings.Join(out, "\n")),
			wantExit1: wantExit1,
			line:      tagLine,
		})
	}
	return blocks, nil
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

// exitCodeMatches reports whether an observed process exit code satisfies a
// parsed block's expectation under this repo's exit contract (0 clean / 1
// verdict / 2 operational, ../CLAUDE.md). A block tagged exit=1 requires
// EXACTLY code 1 — the verdict exit; an operational failure (code 2), or any
// other non-1 code, does NOT satisfy it. This closes the gap where the switch
// flagged an exit=1 tag only when the code was 0, letting any non-zero code
// (including an operational crash with empty stdout) pass. An untagged block
// requires code 0.
func exitCodeMatches(wantExit1 bool, code int) bool {
	if wantExit1 {
		return code == 1
	}
	return code == 0
}

// TestExitCodeMatches is exitCodeMatches' happy- and negative-path proof: an
// exit=1 tag is satisfied by EXACTLY code 1 and by nothing else — crucially not
// by code 2, the operational failure the prior any-non-zero check let through —
// and an untagged block is satisfied only by code 0.
func TestExitCodeMatches(t *testing.T) {
	cases := []struct {
		name      string
		wantExit1 bool
		code      int
		want      bool
	}{
		{"exit=1 tag, code 1 (the verdict exit) is ok", true, 1, true},
		{"exit=1 tag, code 2 (operational) is NOT ok", true, 2, false},
		{"exit=1 tag, code 0 (clean) is NOT ok", true, 0, false},
		{"exit=1 tag, code 3 is NOT ok", true, 3, false},
		{"untagged, code 0 (clean) is ok", false, 0, true},
		{"untagged, code 1 (verdict) is NOT ok", false, 1, false},
		{"untagged, code 2 (operational) is NOT ok", false, 2, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCodeMatches(tc.wantExit1, tc.code); got != tc.want {
				t.Errorf("exitCodeMatches(%v, %d) = %v, want %v", tc.wantExit1, tc.code, got, tc.want)
			}
		})
	}
}

// TestParseVerifyBlocksErr is the parser's happy- and negative-path proof. It
// drives the pure parseVerifyBlocksErr directly so its rejection paths (which
// parseVerifyBlocks turns into t.Fatalf) are asserted — in particular the two
// robustness fixes: a MISSPELLED marker word is rejected loudly rather than
// silently skipped, and a multi-command terminal TRANSCRIPT cannot be tagged as
// a single-command stdout block.
func TestParseVerifyBlocksErr(t *testing.T) {
	const validBare = "<!-- showcase-verify -->\n```console\n$ verdi lint\n```\n"
	const validExit1 = "<!-- showcase-verify exit=1 -->\n```console\n$ verdi gate\nstory.violated: true\n```\n"

	t.Run("happy: a well-formed bare block parses to one command with empty expected stdout", func(t *testing.T) {
		blocks, err := parseVerifyBlocksErr(validBare)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(blocks) != 1 {
			t.Fatalf("got %d blocks, want 1", len(blocks))
		}
		if got := strings.Join(blocks[0].argv, " "); got != "verdi lint" {
			t.Errorf("argv = %q, want %q", got, "verdi lint")
		}
		if blocks[0].wantExit1 {
			t.Errorf("wantExit1 = true, want false for a bare tag")
		}
		if blocks[0].wantOut != "" {
			t.Errorf("wantOut = %q, want empty", blocks[0].wantOut)
		}
	})

	t.Run("happy: an exit=1 tag sets wantExit1 and captures expected stdout", func(t *testing.T) {
		blocks, err := parseVerifyBlocksErr(validExit1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(blocks) != 1 || !blocks[0].wantExit1 {
			t.Fatalf("blocks = %+v, want exactly one block with wantExit1=true", blocks)
		}
		if blocks[0].wantOut != "story.violated: true" {
			t.Errorf("wantOut = %q, want %q", blocks[0].wantOut, "story.violated: true")
		}
	})

	neg := []struct {
		name   string
		readme string
		substr string // the returned error must mention this
	}{
		{
			name:   "misspelled marker word (transposition) is rejected, not skipped",
			readme: "<!-- showcase-verfiy -->\n```console\n$ verdi lint\n```\n",
			substr: "malformed showcase-verify tag",
		},
		{
			name:   "misspelled marker word (underscore) is rejected, not skipped",
			readme: "<!-- showcase_verify -->\n```console\n$ verdi lint\n```\n",
			substr: "malformed showcase-verify tag",
		},
		{
			name:   "correctly-spelled marker carrying bad grammar (exit=2) is rejected",
			readme: "<!-- showcase-verify exit=2 -->\n```console\n$ verdi gate\n```\n",
			substr: "malformed showcase-verify tag",
		},
		{
			name:   "a multi-command terminal transcript cannot be tagged",
			readme: "<!-- showcase-verify -->\n```console\n$ verdi design start\ncreated\n$ verdi serve\n```\n",
			substr: "second command line",
		},
		{
			name:   "tag not followed by a console fence",
			readme: "<!-- showcase-verify -->\nnot a fence\n",
			substr: "immediately followed by a ```console fence",
		},
		{
			name:   "first fence line is not a $ command",
			readme: "<!-- showcase-verify -->\n```console\nverdi lint\n```\n",
			substr: "must start with `$ `",
		},
		{
			name:   "command is not verdi",
			readme: "<!-- showcase-verify -->\n```console\n$ git status\n```\n",
			substr: "must invoke `verdi`",
		},
		{
			name:   "unterminated console fence",
			readme: "<!-- showcase-verify -->\n```console\n$ verdi lint\nno closing fence\n",
			substr: "unterminated",
		},
	}
	for _, tc := range neg {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseVerifyBlocksErr(tc.readme)
			if err == nil {
				t.Fatalf("expected an error, got nil (a malformed block must fail loudly, never a silent skip)")
			}
			if !strings.Contains(err.Error(), tc.substr) {
				t.Errorf("error %q does not mention %q", err.Error(), tc.substr)
			}
			if !strings.Contains(err.Error(), "README.md:") {
				t.Errorf("error %q does not name the README line", err.Error())
			}
		})
	}

	t.Run("control: an innocuous HTML comment is skipped, not rejected", func(t *testing.T) {
		// A comment with no "showcase...ver" shape must NOT trip the fuzzy probe.
		blocks, err := parseVerifyBlocksErr("<!-- TODO: rewrite the intro -->\nprose\n")
		if err != nil {
			t.Fatalf("innocuous comment wrongly rejected: %v", err)
		}
		if len(blocks) != 0 {
			t.Fatalf("got %d blocks, want 0", len(blocks))
		}
	})
}
