package evidence

import (
	"context"
	"errors"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/forge"
)

// OpenSupersessionCandidate is one open MR's decoded, confirmed candidate
// v2 spec: a spec found on an open MR's source branch that genuinely
// `supersedes` featureRef and carries a `supersession:` block — i.e. it
// survived LoadPendingSupersessionCandidates's filtering, not just any
// file found at the probed path.
type OpenSupersessionCandidate struct {
	MRID string
	Spec *artifact.SpecFrontmatter
}

// LoadPendingSupersessionCandidates lists MRs open against targetBranch
// through f, and for each, probes specPath on that MR's source branch
// (03 §The amendment ladder: "the fold's input set includes open
// supersession MRs"). specPath is caller-supplied — the port deliberately
// does not enumerate an MR's changed files (openmr.go's doc comment), and
// the caller already knows the conventional candidate path for the
// feature it is checking (R4-I-14: a superseding spec is a NEW ref,
// typically `<name>-v2`, so the caller derives
// `.verdi/specs/active/<name>-v2/spec.md` before calling this).
//
// Most open MRs do not touch specPath at all — FetchFileAtRef returning
// forge.ErrFileNotFound for a given MR is the ordinary case and is
// silently skipped, not an error. A file found at specPath that either
// fails to decode as a spec, does not carry a `supersedes` edge to
// featureRef, or carries no `supersession:` block at all is also skipped
// (it is either unrelated content coincidentally living at that path, or
// a supersession-shaped MR still mid-authoring with no manifest yet) —
// this function's job is narrowing to *confirmed* candidates, not
// flagging every anomaly.
func LoadPendingSupersessionCandidates(ctx context.Context, f forge.Forge, targetBranch, featureRef, specPath string) ([]OpenSupersessionCandidate, error) {
	mrs, err := f.ListOpenMRs(ctx, targetBranch)
	if err != nil {
		return nil, fmt.Errorf("evidence: LoadPendingSupersessionCandidates: listing open MRs: %w", err)
	}

	var out []OpenSupersessionCandidate
	for _, mr := range mrs {
		data, err := f.FetchFileAtRef(ctx, mr.SourceBranch, specPath)
		if err != nil {
			if errors.Is(err, forge.ErrFileNotFound) {
				continue
			}
			return nil, fmt.Errorf("evidence: LoadPendingSupersessionCandidates: fetching %s at %s: %w", specPath, mr.SourceBranch, err)
		}

		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			continue
		}
		spec, err := artifact.DecodeSpec(fm)
		if err != nil {
			continue
		}
		if spec.Supersession == nil {
			continue
		}
		if !supersedesRef(spec, featureRef) {
			continue
		}
		out = append(out, OpenSupersessionCandidate{MRID: mr.ID, Spec: spec})
	}
	return out, nil
}

func supersedesRef(spec *artifact.SpecFrontmatter, featureRef string) bool {
	for _, l := range spec.Links {
		if l.Type == artifact.LinkSupersedes && l.Ref == featureRef {
			return true
		}
	}
	return false
}

// ImplementsByFeature groups links' `implements`-edge fragment object ids
// by target feature spec name — the caller-side input builder for
// PendingSupersessionInput.ObjectIDs (and the cascade fold's per-story
// edge sets). Links whose ref is not a fragment ref, or fails to parse,
// contribute nothing: a document-level implements edge names no object a
// supersession manifest could classify. Promoted here from cmd/verdi once
// internal/dex became its second consumer (CLAUDE.md: shared code lives in
// a shared internal/ package; never copy-paste across packages).
func ImplementsByFeature(links []artifact.Link) map[string][]string {
	out := make(map[string][]string)
	for _, l := range links {
		if l.Type != artifact.LinkImplements {
			continue
		}
		ref, err := artifact.ParseRef(l.Ref)
		if err != nil || !ref.Fragment() {
			continue
		}
		out[ref.Name] = append(out[ref.Name], ref.Object)
	}
	return out
}

// PendingSupersessionInput is PendingSupersession's input for one story.
type PendingSupersessionInput struct {
	// ObjectIDs are the object ids (fragment targets) the story's edges
	// name within the feature the candidates are checked against.
	ObjectIDs []string
	// Candidates are the confirmed open supersession candidates
	// (LoadPendingSupersessionCandidates's output) for that same feature.
	Candidates []OpenSupersessionCandidate
}

// PendingSupersessionResult is PendingSupersession's outcome.
type PendingSupersessionResult struct {
	Flagged bool
	// MRIDs are the open MR(s) whose pending manifest triggered the flag.
	MRIDs []string
	// Touched are the object ids that triggered it (amended or removed in
	// at least one triggering candidate).
	Touched []string
}

// PendingSupersession implements 03 §The amendment ladder's
// pending-supersession flag: "a story whose edges touch objects listed
// amended/removed in a pending manifest gets an advisory
// pending-supersession flag." Cascade verdicts themselves only bind at
// supersession *merge* (FoldCascade); this is the race-window check over
// *open*, unmerged manifests.
func PendingSupersession(in PendingSupersessionInput) PendingSupersessionResult {
	touchedSet := make(map[string]bool)
	mrSet := make(map[string]bool)
	var result PendingSupersessionResult

	for _, c := range in.Candidates {
		if c.Spec == nil || c.Spec.Supersession == nil {
			continue
		}
		amended := make(map[string]bool, len(c.Spec.Supersession.Amended))
		for _, n := range c.Spec.Supersession.Amended {
			amended[n.ID] = true
		}
		removed := make(map[string]bool, len(c.Spec.Supersession.Removed))
		for _, n := range c.Spec.Supersession.Removed {
			removed[n.ID] = true
		}

		for _, id := range in.ObjectIDs {
			if amended[id] || removed[id] {
				result.Flagged = true
				if !touchedSet[id] {
					touchedSet[id] = true
					result.Touched = append(result.Touched, id)
				}
				if !mrSet[c.MRID] {
					mrSet[c.MRID] = true
					result.MRIDs = append(result.MRIDs, c.MRID)
				}
			}
		}
	}
	return result
}
