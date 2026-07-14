package wallbadge

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/lint"
)

// BoardBadges is ComputeBadges' full result for one spec's board.
type BoardBadges struct {
	// ByObject maps a rendered board object id (an acceptance criterion,
	// constraint, decision, or open-question id, or a declared stub's
	// "stub:<slug>" key) to the badge(s) anchored to its card.
	ByObject map[string][]DerivationRecord
	// CaseFile carries every spec-level badge — the ladder flags (ac-3)
	// and every VL finding that declared a spec-level locus (ac-2).
	CaseFile []DerivationRecord
	// Disclosures are ladder outcomes that could not be proven (ac-3's
	// disclosed-unproven case) — never a badge, never silence.
	Disclosures []string
	// EvidenceSlots is, per STORY acceptance-criterion id, the
	// fold-derived record state of each DECLARED evidence kind in the
	// AC's own declared order (spec/evidence-slot ac-1) — the data the
	// card's per-kind obligation rows wear as record-state chips (ac-3).
	// Nil on non-story specs and on specs declaring no evidence kinds.
	// The matching fold:empty-slot badges ride ByObject like every other
	// card badge (dc-3: one attachment path, one record shape).
	EvidenceSlots map[string][]SlotState
}

// ComputeBadges runs the full v1 badge set (dc-1) for one spec: the
// VL-finding partition (ac-2), scoped to specRelPath; the size-smell
// observation (spec/case-file-flags ac-2) on ANY spec wall that declares
// acceptance criteria, feature and story alike (its dc-3); plus — on a
// STORY-class spec only, mirroring internal/dex/lens.go's own
// isStoryPage/computeLensData gate — the spec-stale and pending-
// supersession ladder badges (ac-3).
//
// ctx/root are the caller's own inputs; specRelPath/specRevision/fm are
// internal/workbench's loadBoard's ALREADY-loaded spec document (its
// store-relative path, the sha256 of the exact bytes it read, and the
// parsed frontmatter) — this function never re-reads the spec document
// itself. superseLoader may be nil (no forge configured; every ladder
// pending-supersession outcome then disclosed-unproven rather than
// silently "not flagged").
func ComputeBadges(ctx context.Context, root, specRelPath, specRevision string, fm *artifact.SpecFrontmatter, superseLoader SupersessionCandidateLoader) (*BoardBadges, error) {
	findings, err := lint.NewEngine().Run(ctx, root, lint.BuildContext(ctx, root), lint.Options{})
	if err != nil {
		return nil, fmt.Errorf("wallbadge: running lint: %w", err)
	}

	out := &BoardBadges{ByObject: make(map[string][]DerivationRecord)}
	for _, b := range VLBadges(findings, specRelPath, specRevision) {
		if b.Target == "" {
			out.CaseFile = append(out.CaseFile, b)
			continue
		}
		out.ByObject[b.Target] = append(out.ByObject[b.Target], b)
	}

	// The size-smell observation (spec/case-file-flags ac-2/dc-3): class-
	// blind — any spec wall that declares acceptance criteria can outgrow
	// a screen — and a pure function of the already-decoded frontmatter's
	// AC count plus declared constants (dc-1), so it needs no store I/O.
	if smell := SizeSmellBadge(specRelPath, specRevision, len(fm.AcceptanceCriteria)); smell != nil {
		out.CaseFile = append(out.CaseFile, *smell)
	}

	if fm.Class != artifact.ClassStory {
		return out, nil // the ladder flags are a story-wall concern only (lens.go's isStoryPage)
	}

	snap, err := lint.BuildSnapshot(root, lint.Options{})
	if err != nil {
		return nil, fmt.Errorf("wallbadge: building snapshot: %w", err)
	}
	// Threshold resolution mirrors internal/dex/lens.go's computeLensData
	// verbatim: 0 unless the manifest configures one, letting
	// decisionsweep.ScanSpecStale itself apply
	// evidence.DefaultDeviationsStaleThreshold when it sees <= 0.
	threshold := 0
	if snap.Manifest != nil && snap.Manifest.Audit != nil {
		threshold = snap.Manifest.Audit.DeviationsStaleThreshold
	}

	stale, err := SpecStaleBadge(root, snap, fm.ID, threshold)
	if err != nil {
		return nil, err
	}
	if stale != nil {
		out.CaseFile = append(out.CaseFile, *stale)
	}

	pending, disclosure, err := PendingSupersessionBadge(ctx, superseLoader, fm.Links)
	if err != nil {
		return nil, err
	}
	if pending != nil {
		out.CaseFile = append(out.CaseFile, *pending)
	}
	if disclosure != "" {
		out.Disclosures = append(out.Disclosures, disclosure)
	}

	// The evidence-slot compute (spec/evidence-slot ac-1/ac-2): a story
	// AC card's per-declared-kind record state, plus a fold:empty-slot
	// badge on each AC holding an empty slot — attached through this one
	// entry point and ByObject like every other card badge (dc-3), never
	// a second attachment path.
	slots, slotBadges, err := EmptySlotBadges(ctx, root, specRelPath, specRevision, fm)
	if err != nil {
		return nil, err
	}
	out.EvidenceSlots = slots
	for _, b := range slotBadges {
		out.ByObject[b.Target] = append(out.ByObject[b.Target], b)
	}

	return out, nil
}
