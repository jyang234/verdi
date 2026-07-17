// The spec-stale and pending-supersession ladder badges (spec/badge-
// computes ac-3, co-3): computed through the EXACT exported entry points
// internal/dex/lens.go's computeLensData calls — decisionsweep.
// ScanSpecStale over a lint.BuildSnapshot, and evidence.PendingSupersession
// fed by evidence.LoadPendingSupersessionCandidates (via
// SupersessionCandidateLoader, the port abstraction) and
// evidence.ImplementsByFeature — never a local reimplementation of either
// fold. Both preserve the lens's own three-valued outcome: flagged-with-
// witness (a DerivationRecord), proven-unflagged (nil, nil), or disclosed-
// unproven (a disclosure string, never a badge and never silence).
package wallbadge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/decisionsweep"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// readStoreFile reads a store-root-relative, slash-separated path.
func readStoreFile(root, relPath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
}

// SpecStaleBadge computes the spec-stale ladder badge for one story spec
// (specRef, e.g. "spec/widget-retry") via decisionsweep.ScanSpecStale over
// snap — the exact entry point internal/dex/lens.go's computeLensData
// calls (co-3). snap is a lint.BuildSnapshot the caller built (this
// function never re-walks the store). Returns (nil, nil) when the story
// has no deviation report yet, or is proven-unflagged; a DerivationRecord
// when the ladder flags it. root is needed only to read the deviation
// report's own `covers` field back out for the derivation record's input
// revision (dc-5) — ScanSpecStale's fold itself is never re-derived here.
// mdl is accepted for symmetry with the model-aware badge pipeline but is
// NOT consulted for the flag's display — the label is a fixed case-file id
// (see the Label site below; finding judged-ladder-flags-share-state-namespace).
func SpecStaleBadge(root string, snap *lint.Snapshot, specRef string, threshold int, mdl *model.Model) (*DerivationRecord, error) {
	entries, err := decisionsweep.ScanSpecStale(root, snap, threshold)
	if err != nil {
		return nil, fmt.Errorf("wallbadge: spec-stale: %w", err)
	}

	var result *evidence.SpecStaleResult
	for i := range entries {
		if entries[i].StoryRef == specRef {
			result = &entries[i].Result
			break
		}
	}
	if result == nil || !result.Flagged {
		return nil, nil // no report yet, or proven-unflagged: no badge either way
	}

	ref, err := artifact.ParseRef(specRef)
	if err != nil {
		return nil, fmt.Errorf("wallbadge: spec-stale: %s: %w", specRef, err)
	}
	reportRelPath := store.DeviationReportRelPath(store.ZoneActive, ref.Name)
	covers, err := readDeviationCovers(root, reportRelPath)
	if err != nil {
		return nil, fmt.Errorf("wallbadge: spec-stale: reading %s: %w", reportRelPath, err)
	}

	ids := append([]string(nil), result.OwnTextFindingIDs...)
	sort.Strings(ids)
	records := append(ids, fmt.Sprintf("%d accepted-deviation disposition(s) accumulated (threshold exceeded: %v)", result.AcceptedDeviationCount, result.TriggeredByThreshold))

	return &DerivationRecord{
		Source: "ladder:spec-stale",
		// FIXED label — NOT vocabulary-resolved. `spec-stale` is a case-
		// file FLAG (03 §The amendment ladder), not a lifecycle state, so
		// its display is NOT vocabulary-addressable in v1: flags are case-
		// file taxonomy, a namespace disjoint from vocabulary.states, and
		// mdl is deliberately not consulted for the flag label here.
		// Finding judged-ladder-flags-share-state-namespace: routing this
		// id through mdl.DisplayState let a states entry keyed `spec-stale`
		// silently rename the flag. Genuine lifecycle-state badges (e.g.
		// dex terminal-status badges) stay DisplayState-resolved; flags do
		// not. Source, inputs, and records stay bare ids too — receipts.
		Label:   "spec-stale",
		Inputs:  []InputRecord{{Name: "deviation-report", Path: reportRelPath, Revision: covers}},
		Records: records,
	}, nil
}

// readDeviationCovers reads and strict-decodes reportRelPath (under root)
// and returns its `covers` field — an already-pinned commit sha (dc-5) —
// WITHOUT re-deriving anything ScanSpecStale itself computes. Called only
// after ScanSpecStale has already proven the report exists and decodes
// (SpecStaleBadge's own gate), so a failure here names a genuine race
// (the file changed or vanished between the two reads) rather than an
// ordinary "no report" case.
func readDeviationCovers(root, reportRelPath string) (string, error) {
	data, err := readStoreFile(root, reportRelPath)
	if err != nil {
		return "", err
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return "", err
	}
	dev, err := artifact.DecodeDeviation(fm)
	if err != nil {
		return "", err
	}
	return dev.Covers, nil
}

// PendingSupersessionBadge computes the pending-supersession ladder badge
// for one story spec via evidence.PendingSupersession fed by
// evidence.LoadPendingSupersessionCandidates (through loader, the port
// abstraction keeping internal/forge out of this package) and
// evidence.ImplementsByFeature — internal/dex/lens.go's own
// computePendingStates call sequence (co-3), never a second open-MR
// supersession fold. links is the story's OWN declared links (its
// implements edges) — the same field lens.go reads off each story page.
//
// Returns exactly one of (record, disclosure) non-empty on a nil error:
// nil record and "" disclosure when the story implements no feature
// (nothing to prove) or is proven-unflagged; a record when the ladder
// flags it; a non-empty disclosure — never a record, never silence, per
// ac-3's three-valued outcome — when loader is nil (no forge configured)
// or a candidate load reports ok=false (open MRs could not be
// enumerated, e.g. no default branch resolved). mdl is accepted for
// symmetry with the model-aware badge pipeline but is NOT consulted for
// the flag's display — the label is fixed (finding
// judged-ladder-flags-share-state-namespace; see the Label site below).
func PendingSupersessionBadge(ctx context.Context, loader SupersessionCandidateLoader, links []artifact.Link, mdl *model.Model) (*DerivationRecord, string, error) {
	byFeature := evidence.ImplementsByFeature(links)
	if len(byFeature) == 0 {
		return nil, "", nil
	}
	if loader == nil {
		return nil, "pending-supersession is disclosed-unproven: no forge is configured to enumerate open MRs", nil
	}

	featureNames := make([]string, 0, len(byFeature))
	for n := range byFeature {
		featureNames = append(featureNames, n)
	}
	sort.Strings(featureNames)

	var merged evidence.PendingSupersessionResult
	var inputs []InputRecord
	for _, featureName := range featureNames {
		candidatePath := store.ActiveSpecRelPath(featureName + "-v2")
		candidates, ok, err := loader.LoadCandidates(ctx, "spec/"+featureName, candidatePath)
		if err != nil {
			return nil, "", fmt.Errorf("wallbadge: pending-supersession: loading candidates for %s: %w", featureName, err)
		}
		if !ok {
			return nil, "pending-supersession is disclosed-unproven: open MRs could not be enumerated (no default branch resolved)", nil
		}
		r := evidence.PendingSupersession(evidence.PendingSupersessionInput{ObjectIDs: byFeature[featureName], Candidates: candidates})
		if r.Flagged {
			merged.Flagged = true
			merged.Touched = append(merged.Touched, r.Touched...)
			merged.MRIDs = append(merged.MRIDs, r.MRIDs...)
		}
		for _, c := range candidates {
			inputs = append(inputs, InputRecord{Name: "candidate:" + c.MRID, Path: candidatePath, Revision: c.Digest})
		}
	}
	if !merged.Flagged {
		return nil, "", nil
	}

	sort.Strings(merged.Touched)
	sort.Strings(merged.MRIDs)
	sort.Slice(inputs, func(i, j int) bool {
		if inputs[i].Name != inputs[j].Name {
			return inputs[i].Name < inputs[j].Name
		}
		return inputs[i].Path < inputs[j].Path
	})

	var records []string
	for _, mr := range merged.MRIDs {
		records = append(records, "MR "+mr)
	}
	for _, id := range merged.Touched {
		records = append(records, "touches "+id)
	}

	return &DerivationRecord{
		Source: "ladder:pending-supersession",
		// FIXED label — NOT vocabulary-resolved, for the identical reason
		// as spec-stale above (finding judged-ladder-flags-share-state-
		// namespace): `pending-supersession` is a case-file FLAG, not a
		// lifecycle state, so its display is not vocabulary-addressable in
		// v1. spec/case-file-flags dc-4 still holds — the same computation
		// wears the same FIXED name on every surface (dex's
		// internal/dex/ladder.go renders the identical fixed id); mdl is
		// deliberately not consulted here. Source/inputs/records stay bare
		// ids — receipts.
		Label:   "pending-supersession",
		Inputs:  inputs,
		Records: records,
	}, "", nil
}
