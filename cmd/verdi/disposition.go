// verdi disposition <spec-ref> <finding-id> <fixed|accepted-deviation>
// --rationale <text> [--amend] (05 §CLI, spec/disposition-verb dc-1):
// records a reviewer's decision on a deviation-report.md finding IN PLACE —
// the sanctioned replacement for the round-6 hand-edit flow (D6-25).
//
// Mechanics mirror align.FreezeInPlace's own discipline exactly (dc-2):
// decode, value-copy (never mutate the decoded original), set only the
// target finding's Disposition/Note, self-validate, then re-render via
// align.RenderMarkdown — the report's own deterministic re-renderer, never a
// generic yaml.Marshal, never internal/artifact/splice (spec.md-only). The
// verb never calls align.Compute, align.PreserveDispositions, or the judge
// (dc-5): it is a pure, local, offline read-mutate-write over a report that
// already exists, so digest/integrity/judge_integrity are carried over
// byte-for-byte (co-2) and remain independently reverifiable.
//
// <spec-ref> is resolved directly against
// .verdi/specs/active/<name>/deviation-report.md — never inferred from the
// checked-out branch the way `verdi align` does (dc-4).
//
// Exit contract (CLAUDE.md 0/1/2; dc-3): 0 written; 1 a verdict about the
// report's own state (unknown finding, disposition collision, nothing to
// amend, frozen report); 2 every other operational failure, including a
// malformed invocation (dc-3 scopes the three named verdicts to report-state
// problems, so an argument-shape/vocabulary error — bad decision enum,
// missing --rationale, wrong positional count — is operational, exactly like
// every other verb's usage check in this package, and never touches the
// report at all).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

const dispositionUsage = "disposition: usage: verdi disposition <spec-ref> <finding-id> <fixed|accepted-deviation> --rationale <text> [--amend]"

// cmdDisposition is `verdi disposition`'s entry point, invoked by dispatch.go.
func cmdDisposition(args []string, stdout, stderr io.Writer) int {
	positional, decision, rationale, amend, rc := parseDispositionArgs(args, stderr)
	if rc != 0 {
		return rc
	}
	specArg, findingID := positional[0], positional[1]

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "disposition:", err)
		return 2
	}
	return runDisposition(root, specArg, findingID, decision, rationale, amend, stdout, stderr)
}

// parseDispositionArgs hand-parses args (mirroring cmd/verdi/align.go's
// cmdAlign loop-based style rather than the stdlib flag package, so
// --rationale/--amend may appear in any order relative to the three
// positionals). Every failure here is a usage/argument-shape problem —
// exit 2, never one of ac-2's three report-state verdicts — and returns
// before any file is touched.
func parseDispositionArgs(args []string, stderr io.Writer) (positional []string, decision artifact.FindingDisposition, rationale string, amend bool, rc int) {
	var rationaleSet bool
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--rationale":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "disposition: --rationale requires a <text> argument")
				return nil, "", "", false, 2
			}
			i++
			rationale = args[i]
			rationaleSet = true
		case "--amend":
			amend = true
		default:
			if strings.HasPrefix(a, "--") {
				fmt.Fprintf(stderr, "disposition: unrecognized flag %q\n", a)
				return nil, "", "", false, 2
			}
			positional = append(positional, a)
		}
	}

	if len(positional) != 3 {
		fmt.Fprintln(stderr, dispositionUsage)
		return nil, "", "", false, 2
	}
	if !rationaleSet || strings.TrimSpace(rationale) == "" {
		fmt.Fprintln(stderr, "disposition: --rationale <text> is required and must not be empty")
		return nil, "", "", false, 2
	}
	// ADJ-52 (j-3): a rationale renders as one line of a markdown bullet
	// (align.RenderFindingLine's raw " — <note>" interpolation, never
	// escaped the way the frontmatter note: field is); a newline or other
	// control character would silently break that single-line invariant
	// with no prior check catching it. Refused here, at argument-shape time
	// (exit 2), before the report is ever touched.
	if r, bad := firstControlRune(rationale); bad {
		fmt.Fprintf(stderr, "disposition: --rationale must not contain control characters (found %U); a disposition renders as a single-line body bullet by design\n", r)
		return nil, "", "", false, 2
	}

	decision = artifact.FindingDisposition(positional[2])
	if decision != artifact.FindingFixed && decision != artifact.FindingAcceptedDeviation {
		fmt.Fprintf(stderr, "disposition: %q is not a known decision (want %q or %q)\n", positional[2], artifact.FindingFixed, artifact.FindingAcceptedDeviation)
		return nil, "", "", false, 2
	}

	return positional[:2], decision, rationale, amend, 0
}

// firstControlRune returns the first Unicode control-character rune in s
// (if any) and whether one was found — ADJ-52's j-3 check backing
// --rationale's single-line-bullet constraint: newlines (\n, \r), tabs, and
// other C0/C1 control characters would each, if embedded raw, corrupt the
// one-line body bullet a disposition's rationale renders as.
func firstControlRune(s string) (r rune, found bool) {
	for _, c := range s {
		if unicode.IsControl(c) {
			return c, true
		}
	}
	return 0, false
}

// runDisposition is the testable core: given an already-resolved root,
// record decision/rationale on findingID in specArg's living
// deviation-report.md.
func runDisposition(root, specArg, findingID string, decision artifact.FindingDisposition, rationale string, amend bool, stdout, stderr io.Writer) int {
	ref, err := artifact.ParseRef(specArg)
	if err != nil || ref.Pinned() || ref.Kind != artifact.KindSpec {
		fmt.Fprintf(stderr, "disposition: %q is not a spec ref (want spec/<name>, e.g. spec/stale-decline)\n", specArg)
		return 2
	}

	reportPath := filepath.Join(root, ".verdi", "specs", "active", ref.Name, "deviation-report.md")
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		fmt.Fprintf(stderr, "disposition: reading %s: %v\n", reportPath, err)
		return 2
	}
	fm, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		fmt.Fprintf(stderr, "disposition: %s: %v\n", reportPath, err)
		return 2
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		fmt.Fprintf(stderr, "disposition: %s: %v\n", reportPath, err)
		return 2
	}

	// co-3: a frozen report is immutable to every verb, this one included —
	// no flag, including --amend, ever overrides this refusal. Checked
	// before the finding lookup: frozen-ness is a report-wide precondition.
	if decoded.Frozen != nil {
		fmt.Fprintf(stderr, "disposition: %s is already frozen (at %s, commit %s); a frozen report is immutable\n", reportPath, decoded.Frozen.At, decoded.Frozen.Commit)
		return 1
	}

	idx := -1
	for i, f := range decoded.Findings {
		if f.ID == findingID {
			idx = i
			break
		}
	}
	if idx == -1 {
		fmt.Fprintf(stderr, "disposition: finding %q not found in %s\n", findingID, reportPath)
		return 1
	}

	oldFinding := decoded.Findings[idx]
	already := oldFinding.Dispositioned()
	if already && !amend {
		fmt.Fprintf(stderr, "disposition: finding %q already carries a disposition (%s); pass --amend to replace it\n", findingID, oldFinding.Disposition)
		return 1
	}
	if !already && amend {
		fmt.Fprintf(stderr, "disposition: finding %q has no existing disposition; --amend has nothing to amend\n", findingID)
		return 1
	}

	// Value-copy: never mutate the decoded original (mirrors
	// align.FreezeInPlace's own discipline, dc-2). A fresh backing array for
	// Findings so mutating the copy's element never touches decoded's.
	updated := *decoded
	updated.Findings = append([]artifact.Finding(nil), decoded.Findings...)
	updated.Findings[idx].Disposition = decision
	updated.Findings[idx].Note = rationale

	// Never fake success (CLAUDE.md): self-validate before writing.
	if err := updated.Validate(); err != nil {
		fmt.Fprintln(stderr, "disposition: internal error: updated frontmatter failed self-validation:", err)
		return 2
	}

	// Keep the human-legible body in agreement with the frontmatter write
	// (dc-2): locate the target finding's OLD rendered bullet line and
	// replace it with its NEW one — both computed via align.RenderFindingLine,
	// the SAME formatting rule renderFindings itself uses — leaving every
	// other line (including the Boundary-diff/Diagram-alignment subsections
	// this verb has no data to regenerate) byte-for-byte untouched.
	//
	// ADJ-52 (j-2): matched as a WHOLE LINE (replaceWholeLine, anchored to
	// line boundaries), never as a raw substring — a prior rationale that
	// happens to quote another finding's full rendered bullet verbatim
	// embeds that quoted text INSIDE its own, longer line, which is never
	// itself equal to the quoted finding's own, shorter, standalone line.
	// A substring count over the whole body (the pre-fix approach) could
	// not tell the two apart, permanently bricking the quoted finding's own
	// later disposition with a false "found 2".
	oldLine := align.RenderFindingLine(oldFinding)
	newLine := align.RenderFindingLine(updated.Findings[idx])
	newBody, n := replaceWholeLine(string(body), oldLine, newLine)
	if n != 1 {
		fmt.Fprintf(stderr, "disposition: internal error: expected exactly one occurrence of finding %q's rendered line in %s, found %d\n", findingID, reportPath, n)
		return 2
	}

	markdown := align.RenderMarkdown(&updated, newBody)
	if err := os.WriteFile(reportPath, markdown, 0o644); err != nil {
		fmt.Fprintln(stderr, "disposition:", err)
		return 2
	}

	verb := "recorded"
	if amend {
		verb = "amended"
	}
	fmt.Fprintf(stdout, "disposition: %s %s %s: %s -> %s\n", verb, ref.String(), findingID, decision, rationale)
	return 0
}

// replaceWholeLine replaces the exactly-one line in body that equals
// oldLine with newLine, matched as a COMPLETE LINE — anchored to line
// boundaries via a split on "\n" — never as an arbitrary substring
// (ADJ-52's j-2 fix). Returns body unmodified alongside the match count
// when that count is not exactly 1, so the caller can fail closed rather
// than fake success (CLAUDE.md); every finding's rendered line begins with
// its own unique "- **<id>**" prefix (Finding IDs are unique, enforced at
// decode), so two DIFFERENT findings' whole lines can never collide —
// only a raw substring search could confuse an embedded quotation for the
// line it quotes.
func replaceWholeLine(body, oldLine, newLine string) (newBody string, matches int) {
	lines := strings.Split(body, "\n")
	found := -1
	for i, l := range lines {
		if l == oldLine {
			matches++
			found = i
		}
	}
	if matches != 1 {
		return body, matches
	}
	lines[found] = newLine
	return strings.Join(lines, "\n"), matches
}
