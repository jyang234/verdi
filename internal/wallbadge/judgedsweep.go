// The judged-findings case-file chip (spec/derivation-drawer ac-3, dc-2):
// ONE chip per spec — the surface wall-receipts ac-6 itself requires —
// reading the spec's own decision-conflict-report.md, an artifact `verdi
// align` already writes, so no computation is invented (wall-receipts
// dc-1). The chip rides the same canonical DerivationRecord as every
// badge: source is the namespaced sweep id (align:judged-sweep), the
// pinned inputs are the report itself (its content digest) and the covers
// sha the sweep pinned, and the firing records are the findings — each
// with its disposition state, or an explicit undispositioned disclosure,
// never a silently omitted finding. The sweep-provenance block (covers,
// adr_corpus_digest, decisions_scanned) rides Record.Provenance so the
// drawer stamps it once at its head.
//
// Staleness legibility is comparison, not verdict (dc-3): this file
// computes only deterministic equality/set comparisons over pinned
// inputs — the report's covers contrasted against the current spec
// content digest, and decisions_scanned contrasted against the decision
// ids the spec currently declares — rendered as disclosure lines on the
// record. No blocking rule, no computed "stale" verdict badge, and no
// clock read anywhere on this path (ac-4/co-1).
package wallbadge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// JudgedSweepSource is the judged-findings chip's namespaced rule id
// (spec/derivation-drawer dc-2).
const JudgedSweepSource = "align:judged-sweep"

// CoversResolver resolves the spec document's content digest at a pinned
// commit — a consumer-defined port (04 §port pattern, mirroring
// SupersessionCandidateLoader) so this package never execs git itself;
// internal/workbench wires the gitx-backed implementation.
//
// SpecDigestAtCommit returns the "sha256:<hex>" content digest of
// relPath's bytes at commit. ok is false when the pinned commit (or the
// path at it) cannot be resolved in this checkout — the disclosed-
// unproven case (dc-3's comparison then discloses its own inability
// rather than claiming a mismatch): never an error, and never a silent
// "no mismatch". err is operational only (git itself failed to run).
type CoversResolver interface {
	SpecDigestAtCommit(ctx context.Context, commit, relPath string) (revision string, ok bool, err error)
}

// JudgedSweepBadge computes the judged-findings case-file chip for one
// spec from its own decision-conflict-report.md
// (.verdi/specs/active/<specName>/decision-conflict-report.md), strict-
// decoded through artifact.DecodeDecisionConflict — the one decoder,
// never a local YAML parse. specRevision is the CURRENT spec document's
// content digest, already computed by the caller (internal/workbench's
// attachBadges hashes the bytes loadBoard read) — dc-3's second operand,
// never re-derived here. fm is that same already-loaded spec's
// frontmatter (its declared decision ids are dc-3's set-comparison
// operand).
//
// Returns exactly one of (record, disclosure) non-empty on a nil error,
// the ladder computes' own three-valued posture: (nil, "", nil) when the
// spec has no report at all (dc-2: absence of a sweep is not a finding —
// no chip, and inventing a "no sweep yet" verdict would be a new
// computation); a record when the report decodes; a non-empty disclosure
// — never a chip, never silence — when a report EXISTS but cannot be
// strict-decoded (three-valued honesty: an unreadable receipt is
// disclosed, not skipped).
func JudgedSweepBadge(ctx context.Context, root, specName, specRevision string, fm *artifact.SpecFrontmatter, resolver CoversResolver) (*DerivationRecord, string, error) {
	reportRelPath := ".verdi/specs/active/" + specName + "/decision-conflict-report.md"
	specRelPath := ".verdi/specs/active/" + specName + "/spec.md"

	data, err := readStoreFile(root, reportRelPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil // no sweep yet: no chip (dc-2), legitimately silent
		}
		return nil, "", fmt.Errorf("wallbadge: judged-sweep: reading %s: %w", reportRelPath, err)
	}

	fmBytes, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Sprintf("judged findings are disclosed-unproven: %s: %v", reportRelPath, err), nil
	}
	report, err := artifact.DecodeDecisionConflict(fmBytes)
	if err != nil {
		return nil, fmt.Sprintf("judged findings are disclosed-unproven: %s failed to decode: %v", reportRelPath, err), nil
	}

	rec := &DerivationRecord{
		Source: JudgedSweepSource,
		Label:  judgedSweepLabel(report.Findings),
		// Target "" — the case-file chip (dc-2: judged findings enter the
		// wall as ONE case-file chip).
		Inputs: []InputRecord{
			// The covers sha: the sweep's own pinned claim of the spec
			// revision it read, carried verbatim (badge-computes dc-5's
			// already-pinned-field form, the ladder badges' precedent).
			{Name: "covers", Path: specRelPath, Revision: report.Covers},
			// The report itself, at the exact bytes read.
			{Name: "decision-conflict-report", Path: reportRelPath, Revision: judgedContentDigest(data)},
		},
		Records:     judgedSweepRecords(report.Findings),
		Provenance:  judgedSweepProvenance(report),
		Disclosures: judgedSweepDisclosures(ctx, report, fm, specRelPath, specRevision, resolver),
	}
	return rec, "", nil
}

// judgedSweepLabel is the chip's short text: the judged-finding count —
// the number a case file's reader triages by. A report whose findings
// are all computed-kind (or empty) still chips as "0 judged findings":
// the sweep ran, and its provenance must stay openable (a stale CLEAN
// sweep has to look stale too, ac-3).
func judgedSweepLabel(findings []artifact.ConflictFinding) string {
	n := 0
	for _, f := range findings {
		if f.Kind == artifact.FindingJudged {
			n++
		}
	}
	if n == 1 {
		return "1 judged finding"
	}
	return fmt.Sprintf("%d judged findings", n)
}

// judgedSweepRecords lists EVERY finding in the report, in the report's
// own pinned order (never re-sorted: the artifact's order is part of the
// receipt), each with its disposition state — a dispositioned finding
// shows its disposition and note, an undispositioned one is explicitly
// disclosed as undispositioned (dc-2: never a silently omitted finding).
func judgedSweepRecords(findings []artifact.ConflictFinding) []string {
	records := make([]string, 0, len(findings))
	for _, f := range findings {
		if f.Dispositioned() {
			records = append(records, fmt.Sprintf("%s [%s] %s — note: %s", f.ID, f.Disposition, f.Text, f.Note))
			continue
		}
		records = append(records, fmt.Sprintf("%s [undispositioned] %s", f.ID, f.Text))
	}
	return records
}

// judgedSweepProvenance builds the drawer-head provenance block (ac-3:
// the drawer names the report's covers sha,
// sweep_provenance.adr_corpus_digest, and decisions_scanned) — every
// line a pinned field read verbatim from the decoded report, never a
// clock and never a recomputation. A report without a sweep_provenance
// block contributes only the covers line; its absence is disclosed by
// judgedSweepDisclosures, never papered over.
func judgedSweepProvenance(report *artifact.DecisionConflictFrontmatter) []string {
	lines := []string{"sweep covers " + report.Covers}
	if report.SweepProvenance == nil {
		return lines
	}
	lines = append(lines, "adr_corpus_digest "+report.SweepProvenance.ADRCorpusDigest)
	scanned := "(none)"
	if len(report.SweepProvenance.DecisionsScanned) > 0 {
		scanned = strings.Join(report.SweepProvenance.DecisionsScanned, ", ")
	}
	return append(lines, "decisions_scanned: "+scanned)
}

// judgedSweepDisclosures computes dc-3's staleness-legibility lines:
// deterministic equality/set comparisons over pinned inputs, rendered as
// disclosure lines — never a blocking rule, never a computed "stale"
// verdict of its own; the reader judges staleness from the disclosed
// contrast.
//
//   - covers vs the wall: the report's covers sha pins the spec revision
//     the sweep read; resolver recovers that revision's content digest
//     and it is compared, by plain string equality, against
//     specRevision — the content digest of the spec THIS wall renders.
//     Equal: no line (a fresh sweep wears no mismatch). Different: the
//     contrast line names both pinned identifiers. Unresolvable (or no
//     resolver wired): disclosed-unproven, never a silent pass and never
//     a claimed mismatch.
//   - decisions_scanned vs the declared set: every decision id the spec
//     currently declares (qualified "<spec-id>#<dc-id>", the exact form
//     internal/align's scannedDecisionIDs writes) must appear in
//     decisions_scanned; each miss is named. A report with no
//     sweep_provenance block cannot be compared at all — that absence is
//     disclosed instead.
func judgedSweepDisclosures(ctx context.Context, report *artifact.DecisionConflictFrontmatter, fm *artifact.SpecFrontmatter, specRelPath, specRevision string, resolver CoversResolver) []string {
	var lines []string

	if resolver == nil {
		lines = append(lines, fmt.Sprintf("sweep covers %s, which this process cannot resolve (no git access wired); this wall renders %s", report.Covers, specRevision))
	} else if coversDigest, ok, err := resolver.SpecDigestAtCommit(ctx, report.Covers, specRelPath); err != nil || !ok {
		// Operational git failures land here too, disclosed rather than
		// failing the whole wall render: the drawer is a reading aid
		// (co-2 — disclosure never blocks), and "could not prove" is the
		// honest label for both an unresolvable pin and a broken resolver.
		lines = append(lines, fmt.Sprintf("sweep covers %s, which this checkout cannot resolve; this wall renders %s", report.Covers, specRevision))
	} else if coversDigest != specRevision {
		lines = append(lines, fmt.Sprintf("sweep covers %s; this wall renders %s", report.Covers, specRevision))
	}

	if report.SweepProvenance == nil {
		return append(lines, "sweep_provenance is absent from this report; adr_corpus_digest and decisions_scanned cannot be compared")
	}
	scanned := make(map[string]bool, len(report.SweepProvenance.DecisionsScanned))
	for _, id := range report.SweepProvenance.DecisionsScanned {
		scanned[id] = true
	}
	for _, dc := range fm.Decisions {
		if !scanned[fm.ID+"#"+dc.ID] {
			lines = append(lines, dc.ID+" is not in decisions_scanned")
		}
	}
	return lines
}

// judgedContentDigest is "sha256:<hex>" over b — the honest, recomputable
// revision (badge-computes dc-5) of the report input's exact bytes read.
func judgedContentDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
