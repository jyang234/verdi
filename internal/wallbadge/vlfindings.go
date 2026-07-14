package wallbadge

import (
	"sort"

	"github.com/jyang234/verdi/internal/lint"
)

// vlLabelMaxRunes bounds a VL badge's short label (dc-2: "label: the
// chip's short text") — a badge is a compact chip (dc-4), not a
// paragraph; the finding's full message still rides Records for the
// derivation drawer.
const vlLabelMaxRunes = 60

// VLBadges computes badge derivation records (dc-2) from a corpus-wide
// lint finding list — the caller runs lint.NewEngine().Run itself; this
// function never re-walks the store — partitioned SOLELY by each
// finding's own wall-locus declaration (lint.Finding.Locus, dc-3) and
// scoped to exactly this one spec's own document: a finding whose Locus
// is nil (plumbing, decode failures — dc-3's fail-closed default) or
// whose Path is not specRelPath contributes nothing, regardless of
// whether that Path lies inside this spec's own directory (badge-
// computes ac-2's explicit fail-closed case: a spec-local plumbing
// finding, e.g. VL-018's dangling layout.json key, still declares no
// locus and so still never badges).
//
// specRelPath/specRevision name the ONE pinned input every VL badge
// cites — the spec document itself, already read by the caller
// (internal/workbench's loadBoard) before this function is ever called —
// so VLBadges never re-reads or re-hashes the file.
//
// The returned records are grouped by (rule, target) so a rule that fires
// more than once against the same card (or the case file) renders one
// chip carrying every firing message, not a chip per finding — and are
// returned in a fully deterministic order (by target, then rule; each
// record's own Records sorted) with no dependency on map iteration order
// (ac-4).
func VLBadges(findings []lint.Finding, specRelPath, specRevision string) []DerivationRecord {
	type key struct{ rule, target string }
	byKey := make(map[key]*DerivationRecord)
	var order []key

	for _, f := range findings {
		if f.Locus == nil || f.Path != specRelPath {
			continue
		}
		k := key{rule: f.Rule, target: f.Locus.Object}
		rec, ok := byKey[k]
		if !ok {
			rec = &DerivationRecord{
				Source: "lint:" + f.Rule,
				Label:  vlLabel(f.Message),
				Target: f.Locus.Object,
				Inputs: []InputRecord{{Name: "spec", Path: specRelPath, Revision: specRevision}},
			}
			byKey[k] = rec
			order = append(order, k)
		}
		rec.Records = append(rec.Records, f.Message)
	}

	sort.Slice(order, func(i, j int) bool {
		if order[i].target != order[j].target {
			return order[i].target < order[j].target
		}
		return order[i].rule < order[j].rule
	})

	out := make([]DerivationRecord, 0, len(order))
	for _, k := range order {
		rec := byKey[k]
		sort.Strings(rec.Records)
		out = append(out, *rec)
	}
	return out
}

// vlLabel derives a badge's short label directly from the finding's own
// message — deliberately NOT a per-rule-id lookup table: ac-2's static
// obligation forbids any switch or map over VL rule ids deciding
// anything about a badge, and a message-derived label means a brand new
// rule that declares a locus gets a legible chip with no registration
// step here at all.
func vlLabel(message string) string {
	r := []rune(message)
	if len(r) <= vlLabelMaxRunes {
		return message
	}
	return string(r[:vlLabelMaxRunes-1]) + "…"
}
