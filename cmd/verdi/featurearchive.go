// Cross-level re-recording awareness (ledger L-N14 companion, the D6-35
// slug-drift residual's cross-level case): gathers the dispositioned judged
// rulings of a feature's CLOSED, non-superseded implementing stories' ARCHIVED
// deviation reports so a feature-context `verdi align` can recognize a
// re-recorded ruling as a carry candidate (align.applyArchivedRulings) instead
// of a brand-new finding — stopping cross-level re-recordings from consuming
// fresh spec-stale budget and controller time. Feature-context align only;
// story aligns never call this (runAlignForSpec gates on spec.Class).
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// gatherArchivedRulings collects, for the feature featureName, the dispositioned
// judged rulings of its CLOSED implementing stories' ARCHIVED deviation reports
// (findings: + not-resurfaced:), each tagged with its source archive ref — the
// cross-level reaffirmation source align.ReconcileJudged consults (ledger L-N14
// companion).
//
// Scope matches the feature-close budget's own AdditionalSets exactly (the closed
// implementing stories' archives), so a candidate's seated backing always has a
// matching archive to collapse against in the union — never a budget inflation
// from an unrelated archive. The filter is: an archived spec that declares an
// `implements` edge into featureName AND whose Git-derived effective state
// (internal/specstate, resolved for every archive-zone candidate in ONE
// ResolveMany call rather than once per spec — Task 5) is CONFIRMED Closed. A
// superseded predecessor is excluded structurally, not by an in-loop status
// check (L-N12's original concern): specstate.Superseded applies only to an
// ACTIVE-zone candidate reachable via a validated successor's supersedes edge
// (resolve.go's resolveOne branches on zone BEFORE ever consulting the
// successor corpus for an archive-zone path) — a spec this glob ever sees at
// all already sits in the archive zone, so it can never resolve Superseded;
// the old legacy `status: superseded` field on an archived spec, if one ever
// existed, is no longer authority.
//
// Resilience: an archive whose spec.md is absent or does not decode, that does
// not implement this feature, or whose effective state cannot be CONFIRMED
// Closed (Proposed/Superseded/Unproven — e.g. this archive-zone copy has not
// actually landed, or diverges from what the default branch holds there) is
// not a confirmable implementing story of featureName and is skipped — this
// advisory pre-fill never couples a feature align to an unrelated or unproven
// archive's health. A CONFIRMED implementing story whose deviation-report.md
// fails to DECODE is this feature's own concern and is surfaced as an
// operational error (never silently treated as "no rulings").
func gatherArchivedRulings(ctx context.Context, root, featureName string, resolver specStateResolver) ([]align.ArchivedRuling, error) {
	specPaths, err := filepath.Glob(store.ArchiveSpecPath(root, "*"))
	if err != nil {
		return nil, fmt.Errorf("align: gathering archived rulings: %w", err)
	}
	sort.Strings(specPaths) // deterministic candidate order feeding the batch resolve below

	type archivedCandidate struct {
		name string
		spec *artifact.SpecFrontmatter
	}
	var implementing []archivedCandidate
	var candidates []specstate.Candidate
	for _, specPath := range specPaths {
		content, spec := loadArchivedSpecIfDecodes(specPath)
		if spec == nil {
			continue
		}
		if len(evidence.ImplementsByFeature(spec.Links)[featureName]) == 0 {
			continue // does not implement this feature
		}
		name := filepath.Base(filepath.Dir(specPath))
		implementing = append(implementing, archivedCandidate{name: name, spec: spec})
		candidates = append(candidates, specstate.Candidate{Path: store.SpecRelPath(store.ZoneArchive, name), Content: content})
	}

	results, err := resolver.ResolveMany(ctx, root, candidates)
	if err != nil {
		// vocab:identity — operational diagnostic naming ids (exit-2 machinery, not verdict prose)
		return nil, fmt.Errorf("align: resolving Git-derived state for archived implementing stories of spec/%s: %w", featureName, err)
	}

	var out []align.ArchivedRuling
	for i, ic := range implementing {
		if results[i].State != specstate.Closed {
			continue // not confirmed closed via Git — see this function's resilience note
		}
		report, err := loadDeviationReportIfExists(store.DeviationReportPath(root, store.ZoneArchive, ic.name))
		if err != nil {
			// vocab:identity — operational diagnostic naming ids (exit-2 machinery, not verdict prose)
			return nil, fmt.Errorf("align: gathering archived rulings for implementing story spec/%s: %w", ic.name, err)
		}
		if report == nil {
			continue // an implementing story that never produced a report
		}
		source := "spec/" + ic.name
		for _, set := range [][]artifact.Finding{report.Findings, report.NotResurfaced} {
			for _, f := range set {
				if f.Kind == artifact.FindingJudged && f.Dispositioned() {
					out = append(out, align.ArchivedRuling{Finding: f, Source: source})
				}
			}
		}
	}
	// Deterministic order (by source, then id) so applyArchivedRulings' first-per-
	// slug pick and the rendered candidate order never depend on Glob's order.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Finding.ID < out[j].Finding.ID
	})
	return out, nil
}

// loadArchivedSpecIfDecodes reads and decodes an archived spec.md, returning
// its raw bytes alongside the decoded frontmatter (the exact content a
// specstate.Candidate needs), or (nil, nil) when it is absent or does not
// decode — this advisory cross-level pre-fill (L-N14) skips an archive it
// cannot confirm as an implementing story rather than coupling a feature
// align to an unrelated archive's health (see gatherArchivedRulings'
// resilience note).
func loadArchivedSpecIfDecodes(specPath string) ([]byte, *artifact.SpecFrontmatter) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, nil
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, nil
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return nil, nil
	}
	return data, spec
}
