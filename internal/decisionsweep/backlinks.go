package decisionsweep

import (
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/lint"
)

// ExemptSource is one decision's `exempts` edge targeting an ADR — one
// entry in an ExemptionCount's backlink list.
type ExemptSource struct {
	SpecRef    string
	DecisionID string
	Reason     string
}

// ExemptionCount is one ADR's active-exemption backlink set (03 §Exemption
// audit: "Per-ADR exemption backlinks are computed and surfaced — a
// lens/dex page ... over every exempts edge in the live corpus that
// targets that ADR").
type ExemptionCount struct {
	ADRRef  string
	Owners  []string
	Sources []ExemptSource
}

// Count is ADRRef's active-exemption count.
func (c ExemptionCount) Count() int { return len(c.Sources) }

// ScanExemptions walks snap's decoded spec documents for every decision
// object's `exempts` link, and buckets it by the ADR it targets — dangling
// exempts edges (the target does not resolve to a real, decoded ADR in the
// corpus) are excluded here: VL-003 already flags a dangling link
// elsewhere, and this audit's job is to count LIVE, resolvable exemptions
// only, per 03's "the live corpus". Returned map is keyed by the ADR's
// unpinned ref (adr/<name>); each entry's Sources is sorted (SpecRef, then
// DecisionID) for determinism.
func ScanExemptions(snap *lint.Snapshot) map[string]*ExemptionCount {
	out := make(map[string]*ExemptionCount)
	for _, doc := range snap.Docs {
		if doc.DecodeErr != nil || doc.Spec == nil {
			continue
		}
		for _, dc := range doc.Spec.Decisions {
			for _, l := range dc.Links {
				if l.Type != artifact.LinkExempts {
					continue
				}
				ref, err := artifact.ParseRef(l.Ref)
				if err != nil || ref.Kind != artifact.KindADR {
					continue
				}
				unpinned := artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()
				targets, ok := snap.ByRef[unpinned]
				if !ok || len(targets) == 0 || targets[0].ADR == nil {
					continue // dangling — not a live, resolvable exemption
				}

				entry, ok := out[unpinned]
				if !ok {
					entry = &ExemptionCount{ADRRef: unpinned, Owners: targets[0].Base.Owners}
					out[unpinned] = entry
				}
				entry.Sources = append(entry.Sources, ExemptSource{
					SpecRef:    doc.Base.ID,
					DecisionID: dc.ID,
					Reason:     l.Note,
				})
			}
		}
	}
	for _, entry := range out {
		sort.Slice(entry.Sources, func(i, j int) bool {
			if entry.Sources[i].SpecRef != entry.Sources[j].SpecRef {
				return entry.Sources[i].SpecRef < entry.Sources[j].SpecRef
			}
			return entry.Sources[i].DecisionID < entry.Sources[j].DecisionID
		})
	}
	return out
}

// SortedADRRefs returns counts' keys sorted — the deterministic iteration
// order every caller (rendering, auto-filing) needs.
func SortedADRRefs(counts map[string]*ExemptionCount) []string {
	refs := make([]string, 0, len(counts))
	for ref := range counts {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs
}
