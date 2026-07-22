// Top-level `verdi audit` orchestration (05 §CLI, R4-I-10): the exemption
// audit (backlinks.go, autofile.go) plus V1-P3's spec-stale surfacing
// (internal/evidence.SpecStale), one pass over the corpus.
package decisionsweep

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
)

// SpecStaleEntry is one story spec's spec-stale computation.
type SpecStaleEntry struct {
	StoryRef string
	Result   evidence.SpecStaleResult
}

// AuditResult is Audit's full output: the corpus's per-ADR exemption
// counts, the paths newly auto-filed this run, every story's spec-stale
// result, and every story's waiver-audit result (spec/verb-surfaces ac-3)
// — its own clearly-separated section, never merged into SpecStale's own
// accepted-deviation count.
type AuditResult struct {
	Exemptions  []*ExemptionCount
	Filed       []string
	SpecStale   []SpecStaleEntry
	WaiverStale []WaiverStaleEntry
}

// Audit runs the full `verdi audit` pipeline: walk the corpus once
// (lint.BuildSnapshot), scan exemption backlinks and auto-file any
// conflict that crosses exemptsThreshold, compute spec-stale for every
// story spec found (V1-P3's evidence.SpecStale, surfaced against
// deviationsThreshold — 05 §CLI's `audit` row), and compute the waiver
// audit for every story spec found (spec/verb-surfaces ac-3, surfaced
// against waiversThreshold — the X-18 counterweight's own site, extended).
// All three thresholds <= 0 use their documented defaults
// (DefaultExemptsConflictThreshold, evidence.DefaultDeviationsStaleThreshold,
// DefaultWaiversStaleThreshold). now is read once by the caller (mirroring
// attest.go's own stamp-at-the-boundary convention) and threaded through to
// ScanWaiverStale, never re-read here — the whole pipeline stays
// deterministic given (root, thresholds, now).
func Audit(root string, exemptsThreshold, deviationsThreshold, waiversThreshold int, now time.Time) (*AuditResult, error) {
	snap, err := lint.BuildSnapshot(root, lint.Options{})
	if err != nil {
		return nil, fmt.Errorf("decisionsweep: %w", err)
	}

	counts := ScanExemptions(snap)
	filings, err := PlanAutoFilings(root, counts, exemptsThreshold)
	if err != nil {
		return nil, err
	}
	filed, err := WriteFilings(root, filings)
	if err != nil {
		return nil, err
	}

	specStale, err := ScanSpecStale(root, snap, deviationsThreshold)
	if err != nil {
		return nil, err
	}

	waiverStale, err := ScanWaiverStale(root, snap, waiversThreshold, now)
	if err != nil {
		return nil, err
	}

	exemptions := make([]*ExemptionCount, 0, len(counts))
	for _, ref := range SortedADRRefs(counts) {
		exemptions = append(exemptions, counts[ref])
	}

	return &AuditResult{Exemptions: exemptions, Filed: filed, SpecStale: specStale, WaiverStale: waiverStale}, nil
}

// ScanSpecStale computes evidence.SpecStale for every story-class spec
// snap found, reading its sibling deviation-report.md (active or archive)
// when one exists — a story with no report yet (never built, or built but
// never `verdi align`-ed) is skipped, not flagged: there is no
// accepted-deviation to have accumulated. StoryACIDs (SpecStale's join key,
// "the set of AC ids the story's OWN spec declares" — specstale.go's own
// doc comment) is built from the story's own `acceptance_criteria:` block
// (02 §Kind registry: round-four story specs carry their own ACs), exactly
// as the closure gate's spec-stale condition builds it (closuregate.go), so
// `verdi audit` surfaces the same trigger (a) the gate enforces — never the
// feature AC ids the story implements.
//
// Exported (V1-P8): internal/dex's story-page ladder badges render THIS
// computation's result — 05 §Lenses' anti-hairball law ("computed the same
// way — no separate logic path"), so the dex's spec-stale badge and `verdi
// audit`'s flag can never disagree.
func ScanSpecStale(root string, snap *lint.Snapshot, threshold int) ([]SpecStaleEntry, error) {
	var out []SpecStaleEntry
	for _, doc := range snap.Docs {
		if doc.DecodeErr != nil || doc.Spec == nil || doc.Spec.Class != artifact.ClassStory {
			continue
		}
		ref, err := artifact.ParseRef(doc.Spec.ID)
		if err != nil {
			// vocab:identity — operational diagnostic naming ids (machinery, not verdict prose)
			return nil, fmt.Errorf("decisionsweep: story %s has an invalid id: %w", doc.Spec.ID, err)
		}

		findings, notResurfaced, found, err := readDeviationFindings(root, ref.Name)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		acIDs := storyOwnACIDs(doc.Spec)
		// spec/finding-identity ac-3: unioned with the story's OWN
		// not-resurfaced: by unique identity, so a finding a fresh judge run
		// simply does not re-emit never drains out of the budget just because
		// it moved out of findings: (the X-18 laundering drain). OwnNotResurfaced
		// feeds trigger (b) only — not trigger (a), which is unreachable over a
		// judged-only section whose ids can never equal an AC id
		// (judged-spec-stale-own-text-judged-id-prefix) — mirrors closuregate.go's
		// checkSpecStaleCondition, 05 §Lenses' anti-hairball law.
		result := evidence.SpecStale(evidence.SpecStaleInput{
			Findings:         findings,
			OwnNotResurfaced: notResurfaced,
			StoryACIDs:       acIDs,
			Threshold:        threshold,
		})
		out = append(out, SpecStaleEntry{StoryRef: doc.Spec.ID, Result: result})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StoryRef < out[j].StoryRef })
	return out, nil
}

// storyOwnACIDs is the set of AC ids the story's OWN spec declares in its
// `acceptance_criteria:` block — SpecStale's trigger-(a) join key,
// identical to the closure gate's own construction (closuregate.go).
func storyOwnACIDs(story *artifact.SpecFrontmatter) map[string]bool {
	ids := make(map[string]bool, len(story.AcceptanceCriteria))
	for _, ac := range story.AcceptanceCriteria {
		ids[ac.ID] = true
	}
	return ids
}

// readDeviationFindings reads and strict-decodes <name>/deviation-report.md
// from either specs/active/ or specs/archive/, returning (nil, false, nil)
// when neither exists.
func readDeviationFindings(root, name string) (findings, notResurfaced []artifact.Finding, found bool, err error) {
	for _, statusDir := range []string{"active", "archive"} {
		path := store.DeviationReportPath(root, statusDir, name)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return nil, nil, false, fmt.Errorf("decisionsweep: reading %s: %w", path, readErr)
		}
		fm, _, splitErr := artifact.SplitFrontmatter(data)
		if splitErr != nil {
			return nil, nil, false, fmt.Errorf("decisionsweep: %s: %w", path, splitErr)
		}
		decoded, decodeErr := artifact.DecodeDeviation(fm)
		if decodeErr != nil {
			return nil, nil, false, fmt.Errorf("decisionsweep: %s: %w", path, decodeErr)
		}
		return decoded.Findings, decoded.NotResurfaced, true, nil
	}
	return nil, nil, false, nil
}
