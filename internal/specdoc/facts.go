package specdoc

import (
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/matrixprojection"
)

// Facts are the computed inputs a document reports but never derives.
// A nil map means "not supplied for this render" and the section says so
// (spec/spec-documents co-6); an empty, non-nil map means "supplied, and
// there is nothing".
type Facts struct {
	// Coverage maps a criterion id to the sorted slugs of the stubs that
	// name it in acceptance_criteria.
	Coverage map[string][]string
	// Claims maps an open-question id to the sorted slugs of the spike
	// stubs that name it in resolves.
	Claims map[string][]string
	// Evidence maps a criterion id to its state in the matrix projection.
	Evidence map[string]ACEvidence
	// EvidenceSource names where Evidence came from, e.g. "matrix at
	// <commit>"; empty when Evidence is nil.
	EvidenceSource string
}

// ACEvidence is one criterion's evidence state as the matrix reports it.
type ACEvidence struct {
	Status  string
	Summary string
	// Stories is the sorted list of implementing story refs (feature
	// records only).
	Stories []string
	// Kinds is the per-evidence-kind state in the record's own order
	// (story records only).
	Kinds []KindEvidence
}

// KindEvidence is one evidence kind's satisfaction for a story criterion.
type KindEvidence struct {
	Kind      string
	Satisfied bool
}

// FactsFromSpec derives coverage and claims from the spec's own stubs.
// Every declared criterion and question gets an entry, so an uncovered
// criterion reads as "known: nothing covers it", not as "unknown".
// Evidence stays unavailable: the spec alone cannot know it.
func FactsFromSpec(fm *artifact.SpecFrontmatter) Facts {
	if fm == nil {
		return Facts{}
	}
	coverage := make(map[string][]string, len(fm.AcceptanceCriteria))
	for _, ac := range fm.AcceptanceCriteria {
		coverage[ac.ID] = []string{}
	}
	claims := make(map[string][]string, len(fm.OpenQuestions))
	for _, oq := range fm.OpenQuestions {
		claims[oq.ID] = []string{}
	}
	for _, st := range fm.Stubs {
		if st.Spike {
			for _, id := range st.Resolves {
				if _, declared := claims[id]; declared {
					claims[id] = append(claims[id], st.Slug)
				}
			}
			continue
		}
		for _, id := range st.AcceptanceCriteria {
			if _, declared := coverage[id]; declared {
				coverage[id] = append(coverage[id], st.Slug)
			}
		}
	}
	for id := range coverage {
		sort.Strings(coverage[id])
	}
	for id := range claims {
		sort.Strings(claims[id])
	}
	return Facts{Coverage: coverage, Claims: claims}
}

// WithMatrix copies f and fills Evidence from a matrix projection record.
// A record with neither a feature nor a story body leaves Evidence
// unavailable. source is recorded verbatim as EvidenceSource.
func WithMatrix(f Facts, rec matrixprojection.Record, source string) Facts {
	out := f
	switch {
	case rec.Feature != nil:
		out.Evidence = make(map[string]ACEvidence, len(rec.Feature.ACs))
		for _, ac := range rec.Feature.ACs {
			stories := append([]string(nil), ac.ImplementingStories...)
			sort.Strings(stories)
			out.Evidence[ac.ID] = ACEvidence{Status: ac.Status, Summary: ac.Summary, Stories: stories}
		}
	case rec.Story != nil:
		out.Evidence = make(map[string]ACEvidence, len(rec.Story.ACs))
		for _, ac := range rec.Story.ACs {
			kinds := make([]KindEvidence, 0, len(ac.Kinds))
			for _, k := range ac.Kinds {
				kinds = append(kinds, KindEvidence{Kind: k.Kind, Satisfied: k.Satisfied})
			}
			out.Evidence[ac.ID] = ACEvidence{Status: ac.Status, Summary: ac.Summary, Kinds: kinds}
		}
	default:
		return out
	}
	out.EvidenceSource = source
	return out
}
