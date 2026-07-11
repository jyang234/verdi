// Top-level `verdi audit` orchestration (05 §CLI, R4-I-10): the exemption
// audit (backlinks.go, autofile.go) plus V1-P3's spec-stale surfacing
// (internal/evidence.SpecStale), one pass over the corpus.
package decisionsweep

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/evidence"
	"github.com/OWNER/verdi/internal/lint"
)

// SpecStaleEntry is one story spec's spec-stale computation.
type SpecStaleEntry struct {
	StoryRef string
	Result   evidence.SpecStaleResult
}

// AuditResult is Audit's full output: the corpus's per-ADR exemption
// counts, the paths newly auto-filed this run, and every story's
// spec-stale result.
type AuditResult struct {
	Exemptions []*ExemptionCount
	Filed      []string
	SpecStale  []SpecStaleEntry
}

// Audit runs the full `verdi audit` pipeline: walk the corpus once
// (lint.BuildSnapshot), scan exemption backlinks and auto-file any
// conflict that crosses exemptsThreshold, and compute spec-stale for every
// story spec found (V1-P3's evidence.SpecStale, surfaced against
// deviationsThreshold — 05 §CLI's `audit` row). Both thresholds <= 0 use
// their documented defaults (DefaultExemptsConflictThreshold,
// evidence.DefaultDeviationsStaleThreshold).
func Audit(root string, exemptsThreshold, deviationsThreshold int) (*AuditResult, error) {
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

	specStale, err := scanSpecStale(root, snap, deviationsThreshold)
	if err != nil {
		return nil, err
	}

	exemptions := make([]*ExemptionCount, 0, len(counts))
	for _, ref := range SortedADRRefs(counts) {
		exemptions = append(exemptions, counts[ref])
	}

	return &AuditResult{Exemptions: exemptions, Filed: filed, SpecStale: specStale}, nil
}

// scanSpecStale computes evidence.SpecStale for every story-class spec
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
func scanSpecStale(root string, snap *lint.Snapshot, threshold int) ([]SpecStaleEntry, error) {
	var out []SpecStaleEntry
	for _, doc := range snap.Docs {
		if doc.DecodeErr != nil || doc.Spec == nil || doc.Spec.Class != artifact.ClassStory {
			continue
		}
		ref, err := artifact.ParseRef(doc.Spec.ID)
		if err != nil {
			return nil, fmt.Errorf("decisionsweep: story %s has an invalid id: %w", doc.Spec.ID, err)
		}

		findings, found, err := readDeviationFindings(root, ref.Name)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		acIDs := storyOwnACIDs(doc.Spec)
		result := evidence.SpecStale(evidence.SpecStaleInput{Findings: findings, StoryACIDs: acIDs, Threshold: threshold})
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
func readDeviationFindings(root, name string) ([]artifact.Finding, bool, error) {
	for _, statusDir := range []string{"active", "archive"} {
		path := filepath.Join(root, ".verdi", "specs", statusDir, name, "deviation-report.md")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, false, fmt.Errorf("decisionsweep: reading %s: %w", path, err)
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			return nil, false, fmt.Errorf("decisionsweep: %s: %w", path, err)
		}
		decoded, err := artifact.DecodeDeviation(fm)
		if err != nil {
			return nil, false, fmt.Errorf("decisionsweep: %s: %w", path, err)
		}
		return decoded.Findings, true, nil
	}
	return nil, false, nil
}
