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
// amend, frozen report, or — ADJ-53's j-5 reclassification — the report's
// body and frontmatter having drifted out of agreement for the target
// finding); 2 every other operational failure, including a malformed
// invocation (dc-3 scopes verdicts to conditions of the report's own state,
// so an argument-shape/vocabulary error — bad decision enum, missing
// --rationale, wrong positional count — is operational, exactly like every
// other verb's usage check in this package, and never touches the report
// at all) and — ADJ-54's durable-writer checklist — a genuine concurrent
// modification detected by the pre-write staleness re-read.
//
// Durable-writer guarantees (ADJ-54, completing the checklist input
// hygiene/j-4, identity matching/j-2, validation, exit taxonomy/j-5 began):
// the final write goes through internal/atomicfile.Write (fsync +
// temp-then-rename), never a plain os.WriteFile, so a crash/kill/disk-full
// mid-write can never truncate this store file's one genuine, never-
// reproducible judged exchange; and an optimistic staleness check
// (re-read immediately before that write, refuse on any byte difference
// from the initial read) closes the unlocked read-modify-write's lost-
// update race, since internal/filelock's existing primitive is a
// per-checkout/per-worktree PID-liveness writer-role lock — the wrong
// granularity for this verb's brief, per-report race window.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/store"
)

const dispositionUsage = "disposition: usage: verdi disposition <spec-ref> <finding-id> <fixed|accepted-deviation> --rationale <text> [--amend]"

// dispositionTestInterleave, when non-nil, is invoked by runDisposition
// immediately before its final write — a test-only seam (ADJ-54's j-7 TDD
// ask: "simulate the interleaving... via a test seam or by driving the
// internals") letting a test deterministically inject a concurrent
// modification into the exact race window a real two-PROCESS interleaving
// would otherwise need flaky OS-level timing to hit reliably. Always nil
// in a real invocation; read and set only by this package's own tests.
var dispositionTestInterleave func(reportPath string)

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

	reportPath := store.DeviationReportPath(root, store.ZoneActive, ref.Name)
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
		// spec/finding-identity judged-not-resurfaced-mark-fixed: the id is not
		// a LIVE finding. If it lives SOLELY in not-resurfaced: (the primary
		// ac-3 shape — a prior ruling a fresh judge run never resurfaces), this
		// verb is the sanctioned human exit ramp for that entry; see
		// dispositionNotResurfaced, which draws the laundering boundary. Only a
		// genuinely absent id (in neither section) is the "finding not found"
		// verdict this branch has always returned.
		if nrIdx := findNotResurfacedIndex(decoded.NotResurfaced, findingID); nrIdx != -1 {
			return dispositionNotResurfaced(reportPath, raw, decoded, string(body), nrIdx, decision, rationale, ref, stdout, stderr)
		}
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

	// spec/finding-identity judged-amend-stale-carried-from: an --amend applies
	// the same carried-from discipline as first-writing. The stamp copied from
	// decoded above is ac-2's confirmed-reaffirmation provenance for the PRIOR
	// decision; an amend that CHANGES the decision (the escalation the live path
	// below explicitly never stamps) reaffirms nothing the stamp attested, so
	// clear it. The live reaffirmation branch below cannot do this on an amend —
	// the backing record was removed at the original confirmation, so it finds
	// nothing to correct against. A note-only amend leaves the decision, and the
	// reaffirmation the stamp attests, intact.
	if amend && decision != oldFinding.Disposition {
		updated.Findings[idx].CarriedFrom = ""
	}

	// spec/finding-identity ac-1/ac-2: this IS the "confirm a candidate as a
	// working-tree edit" step — align.ReconcileJudged never dispositions a
	// candidate itself (identity.go's frozen rule is never bypassed), it
	// only pre-fills one; this verb, already the sanctioned single place a
	// human records any disposition, is where a pending candidate's
	// confirmation actually happens. If findingID's own not-resurfaced:
	// entry (its old ruling's backing record — align.ReconcileJudged's own
	// doc comment on why it stays there rather than a new persisted
	// Candidate field) is present, this confirmation resolves it: a decision
	// EQUAL to the old ruling is a REAFFIRMATION — carried-from: <covers-sha>
	// is stamped (the report's own covering head, exactly where ac-1's
	// "confirmed ... at the covering head" places it) — while a decision
	// that DIFFERS is an escalation and is never stamped (ac-2: "nothing
	// silently carries"). Either way the old entry is removed: it has been
	// resolved, one way or the other, by a human who has now seen both
	// texts.
	//
	// Scoped strictly to a JUDGED live finding (judged-reaffirm-judged-kind-
	// scope): the reaffirmation mechanism is judged-only (ReconcileJudged,
	// identity.go), so a COMPUTED finding whose boundary-derived id merely
	// collides with a judged not-resurfaced entry must never resolve — drain
	// or reaffirm — that entry, nor ever receive carried-from provenance the
	// mechanism was never meant to stamp on it.
	//
	// A colliding slug that owns a not-resurfaced backing record never reaches
	// this live path: ReconcileJudged suffixes EVERY member of such a collision
	// ("<slug><CollisionInfix><n>", collisionMemberIDs), so the backing record
	// alone keeps the bare slug and no live finding ever shadows it. This holds
	// for a backing record persisted by a prior round
	// (judged-collision-backing-regeneration-drain) AND for one BORN this round —
	// a live dispositioned prior at the slug that the fresh collision does not
	// carry forward on its bare-id member, which now lands in not-resurfaced with
	// every member already suffixed (judged-collision-backing-same-round). A live
	// judged finding sharing an id with a not-resurfaced entry is therefore always
	// a genuine slug-only candidate (a single fresh finding under that slug,
	// ac-1), whose confirmation legitimately resolves the backing record as above;
	// a colliding slug's backing record — pre-existing or born this round — is
	// resolved solely through its own exit ramp (dispositionNotResurfaced),
	// reachable at all times because its bare id is never occupied by a live
	// member. artifact.Validate enforces the invariant from the other side — a
	// dispositioned findings: entry sharing a same-kind id with a not-resurfaced
	// entry is rejected — so this path never special-cases a collision base member.
	if oldFinding.Kind == artifact.FindingJudged {
		if nrIdx := findNotResurfacedIndex(decoded.NotResurfaced, findingID); nrIdx != -1 {
			oldEntry := decoded.NotResurfaced[nrIdx]
			if decision == oldEntry.Disposition {
				updated.Findings[idx].CarriedFrom = decoded.Covers
			}
			updated.NotResurfaced = removeFindingAt(decoded.NotResurfaced, nrIdx)
		}
	}

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
		// ADJ-53 (j-5 reclassification): a body/frontmatter desync for this
		// finding is a condition of the REPORT'S OWN STATE (dc-3's verdict
		// class), not an operational failure — nothing in the environment
		// has failed; the report itself no longer agrees with itself for
		// this finding (a hand-drifted body, or one rendered by an older
		// format). Exit 1, naming the inconsistency honestly, never
		// "internal error" (which misattributes an externally-authored
		// report condition to a bug in the tool). With j-4's source fix
		// (align.judge.go's normalizeJudgeText), the reachable cases here
		// collapse to genuinely corrupted/hand-drifted reports — still
		// honestly a verdict about that report, never this verb's fault.
		fmt.Fprintf(stderr, "disposition: finding %q's rendered line does not appear exactly once in %s's body (found %d) — the report's body and frontmatter have drifted out of agreement for this finding\n", findingID, reportPath, n)
		return 1
	}

	if rc := commitDisposition(reportPath, raw, &updated, newBody, stderr); rc != 0 {
		return rc
	}

	verb := "recorded"
	if amend {
		verb = "amended"
	}
	// spec/finding-identity: name the section the id was found in — an
	// ordinary live-finding disposition ("(findings)") versus the
	// not-resurfaced exit ramp's own "(not-resurfaced)" output
	// (dispositionNotResurfaced), so a reader always knows which one happened.
	fmt.Fprintf(stdout, "disposition: %s %s %s (findings): %s -> %s\n", verb, ref.String(), findingID, decision, rationale)
	return 0
}

// commitDisposition renders updated+newBody into deviation-report.md's full
// content and durably writes it to reportPath — the write tail shared by the
// live-finding path (runDisposition) and the not-resurfaced exit ramp
// (dispositionNotResurfaced), so both inherit ADJ-54's durability guarantees
// with no second copy (CLAUDE.md: no copy-paste). Returns 0 on a successful
// write, 2 on any operational failure. Callers print their own success line.
//
// ADJ-54 (j-7): optimistic staleness verification — re-read the report
// IMMEDIATELY before the atomic write and refuse if its bytes have changed
// since raw was read at the top of the caller. This verb's
// read-decode-mutate-write has no lock, and internal/filelock's existing
// primitive is the wrong granularity here: a per-checkout/per-worktree
// PID-liveness writer-role lock built for long-lived daemons (verdi serve) and
// managed-worktree reservations, not a per-report, single-shot CLI operation's
// brief race window — using it would mean inventing a new per-report lock-path
// convention rather than using the primitive "per its own conventions". A
// genuine concurrent modification is an operational condition (exit 2) — an
// environment fact, not a verdict about the target's own state — and the
// honest remedy is simply to re-run the command against the now-current file
// rather than silently clobber whatever changed.
//
// ADJ-54 (j-6): atomicfile.Write (MkdirAll + CreateTemp + fsync +
// Rename-into-place) — this repo's own existing crash-durability primitive —
// never a plain os.WriteFile (truncate-then-write), so an operational failure
// mid-write (disk full, kill, crash) can never leave a truncated
// deviation-report.md: the one store file whose judged exchange is declared
// never reproducible (03 §Alignment report), so content not yet committed to
// git (a just-recorded disposition, or the one genuine judge_integrity
// exchange) would otherwise be unrecoverable.
func commitDisposition(reportPath string, raw []byte, updated *artifact.DeviationFrontmatter, newBody string, stderr io.Writer) int {
	markdown := align.RenderMarkdown(updated, newBody)

	if dispositionTestInterleave != nil {
		dispositionTestInterleave(reportPath)
	}

	current, err := os.ReadFile(reportPath)
	if err != nil {
		fmt.Fprintf(stderr, "disposition: re-reading %s before write: %v\n", reportPath, err)
		return 2
	}
	if !bytes.Equal(current, raw) {
		fmt.Fprintf(stderr, "disposition: %s was modified concurrently (its bytes changed since it was read); refusing to write and risk losing that change — re-run the command against the current file\n", reportPath)
		return 2
	}

	if err := atomicfile.Write(reportPath, markdown, 0o644); err != nil {
		fmt.Fprintln(stderr, "disposition:", err)
		return 2
	}
	return 0
}

// dispositionNotResurfaced is spec/finding-identity judged-not-resurfaced-mark-fixed's
// fix: the sanctioned human exit ramp for an id that lives SOLELY in
// not-resurfaced: (absent from findings:) — the primary ac-3 shape, a prior
// ruling a fresh judge run simply never re-emits. ac-3 says such an entry
// persists "until a human explicitly marks it fixed"; this verb is that
// explicit human act.
//
// The laundering boundary (X-18) this must NOT cross: the X-18 drain was the
// JUDGE's non-reproduction automatically UNCOUNTING a standing accepted
// deviation — a finding leaving findings: because it stopped reproducing must
// never silently drain out of the spec-stale/feature-close budget (closed by
// the SpecStale/ReconcileJudged union). This function is the opposite: a HUMAN,
// through the sanctioned verb, deciding the entry's fate. That is precisely
// ac-3's "until a human explicitly marks it fixed" clause, not an automatic
// drain — the same X-16 working-tree-edit discipline every other disposition
// follows.
//
//   - fixed: the human affirms the underlying issue is genuinely resolved.
//     The entry is REMOVED, releasing its identity from the budget — never
//     zero from the judge's silence, but one by the human's own signature.
//   - accepted-deviation: the human RE-AFFIRMS the standing deviation without
//     resurfacing it. The rationale is updated in place, carried-from:
//     <covers-sha> is stamped (ac-2's reaffirmation provenance, symmetric with
//     the candidate path — judged-not-resurfaced-reaffirm-provenance), and the
//     entry STAYS, so it STAYS COUNTED in the budget — a re-affirmation is never
//     a release. A `fixed` release, by contrast, is never a reaffirmation and
//     carries no such stamp.
//
// --amend is a live-findings collision guard (undispositioned vs. already-
// dispositioned) and has no meaning here: a not-resurfaced entry is by
// definition already a prior ruling, and acting on it through this verb is
// always a deliberate, explicit human decision, so the flag is not consulted.
// Mechanics mirror the live path: value-copy the decoded original (never
// mutate it), self-validate, surgically patch the entry's OWN rendered body
// line to keep body and frontmatter in agreement, then write via the shared,
// crash-durable commitDisposition tail.
func dispositionNotResurfaced(reportPath string, raw []byte, decoded *artifact.DeviationFrontmatter, body string, nrIdx int, decision artifact.FindingDisposition, rationale string, ref artifact.Ref, stdout, stderr io.Writer) int {
	oldEntry := decoded.NotResurfaced[nrIdx]
	oldLine := align.RenderNotResurfacedLine(oldEntry)

	// Value-copy: never mutate the decoded original (dc-2). Findings is left
	// aliasing decoded's (never touched on this path); NotResurfaced gets a
	// fresh backing array before any element mutation.
	updated := *decoded
	var newBody string
	var n int
	var action string

	switch decision {
	case artifact.FindingFixed:
		// Human-sanctioned release: drop the entry entirely.
		updated.NotResurfaced = removeFindingAt(decoded.NotResurfaced, nrIdx)
		action = "released"
		if len(updated.NotResurfaced) == 0 {
			// The section is now empty — substitute renderNotResurfaced's own
			// "(none)" placeholder so the body stays byte-identical to a fresh
			// align render rather than leaving a bare, entry-less heading.
			newBody, n = replaceWholeLine(body, oldLine, "(none)")
		} else {
			newBody, n = removeWholeLine(body, oldLine)
		}
	case artifact.FindingAcceptedDeviation:
		// Re-affirm in place: update the rationale, keep it counted, and stamp
		// carried-from: <covers-sha> (judged-not-resurfaced-reaffirm-provenance).
		// This IS a confirmed reaffirmation of the standing ruling at the current
		// covering head, so ac-2's "a confirmed reaffirmation carries carried-from:
		// <covers-sha> on the disposition" applies here exactly as it does on the
		// candidate path (runDisposition's live reaffirmation) — otherwise an
		// in-place reaffirmation would be indistinguishable from an entry never
		// re-confirmed, an asymmetry the reaffirmation clause does not sanction.
		// carried-from is frontmatter-only provenance (RenderNotResurfacedLine
		// omits it, like RenderFindingLine) and excluded from the report digest,
		// so the body patch and every VerifyDigest stay unaffected.
		updated.NotResurfaced = append([]artifact.Finding(nil), decoded.NotResurfaced...)
		reaffirmed := oldEntry
		reaffirmed.Disposition = artifact.FindingAcceptedDeviation
		reaffirmed.Note = rationale
		reaffirmed.CarriedFrom = decoded.Covers
		updated.NotResurfaced[nrIdx] = reaffirmed
		action = "reaffirmed"
		newBody, n = replaceWholeLine(body, oldLine, align.RenderNotResurfacedLine(reaffirmed))
	}

	// Never fake success (CLAUDE.md): self-validate the mutated frontmatter
	// before writing.
	if err := updated.Validate(); err != nil {
		fmt.Fprintln(stderr, "disposition: internal error: updated frontmatter failed self-validation:", err)
		return 2
	}

	// The entry's own not-resurfaced body line must appear exactly once
	// (ADJ-52's j-2 whole-line discipline, symmetric with the live path). A
	// mismatch is a body/frontmatter desync — a verdict about the report's own
	// state (dc-3), never "internal error" (ADJ-53's j-5 reclassification).
	if n != 1 {
		fmt.Fprintf(stderr, "disposition: not-resurfaced entry %q's rendered line does not appear exactly once in %s's body (found %d) — the report's body and frontmatter have drifted out of agreement for this entry\n", oldEntry.ID, reportPath, n)
		return 1
	}

	if rc := commitDisposition(reportPath, raw, &updated, newBody, stderr); rc != 0 {
		return rc
	}

	fmt.Fprintf(stdout, "disposition: %s %s %s (not-resurfaced): %s -> %s\n", action, ref.String(), oldEntry.ID, decision, rationale)
	return 0
}

// findNotResurfacedIndex returns the index of the JUDGED entry in
// notResurfaced whose id equals findingID, or -1 — spec/finding-identity's own
// lookup for a live candidate's backing record (ids are unique within
// not-resurfaced:, artifact.DeviationFrontmatter.Validate). Scoped to Kind ==
// FindingJudged (judged-reaffirm-judged-kind-scope): the not-resurfaced/
// reaffirmation machinery is judged-only, so the exit-ramp path never touches a
// non-judged entry — belt-and-suspenders, since ReconcileJudged only ever
// produces judged not-resurfaced entries, but this verb operates over a
// working-tree file a human can hand-edit and must not rely on that invariant
// holding by construction.
func findNotResurfacedIndex(notResurfaced []artifact.Finding, findingID string) int {
	for i, f := range notResurfaced {
		if f.ID == findingID && f.Kind == artifact.FindingJudged {
			return i
		}
	}
	return -1
}

// removeFindingAt returns a NEW slice with the entry at idx removed —
// never mutates fs in place (mirrors this file's own value-copy discipline,
// dc-2: the caller's decoded original must never be touched).
func removeFindingAt(fs []artifact.Finding, idx int) []artifact.Finding {
	out := make([]artifact.Finding, 0, len(fs)-1)
	out = append(out, fs[:idx]...)
	out = append(out, fs[idx+1:]...)
	return out
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

// removeWholeLine removes the exactly-one line in body equal to target,
// matched as a COMPLETE LINE (split on "\n", never an arbitrary substring) —
// the removal twin of replaceWholeLine, used by the not-resurfaced `fixed`
// exit ramp (dispositionNotResurfaced) when the section retains other entries.
// Returns body unmodified alongside the match count when that count is not
// exactly 1, so the caller fails closed on a body/frontmatter desync rather
// than faking success (CLAUDE.md).
func removeWholeLine(body, target string) (newBody string, matches int) {
	lines := strings.Split(body, "\n")
	found := -1
	for i, l := range lines {
		if l == target {
			matches++
			found = i
		}
	}
	if matches != 1 {
		return body, matches
	}
	lines = append(lines[:found], lines[found+1:]...)
	return strings.Join(lines, "\n"), matches
}
