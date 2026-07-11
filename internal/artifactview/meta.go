package artifactview

import (
	"fmt"

	"github.com/OWNER/verdi/internal/artifact"
)

// Meta is the render-fidelity metadata a page needs beyond index.Entry's
// generic view.
type Meta struct {
	Base   artifact.Base
	Status string

	// Feature-spec-only fields (02 §feature-spec frontmatter additions).
	Class              artifact.SpecClass
	Story              string
	Impacts            []string
	Context            []string
	Declares           *artifact.Declares
	AcceptanceCriteria []artifact.AcceptanceCriterion
	Dispositions       []artifact.Disposition
	// Stubs is the feature spec's acceptance-time story plan (02 §Common
	// frontmatter `stubs:`), carried through for the feature lens' dex
	// edition (05 §Lenses: stubs always rendered paired with the computed
	// live mapping, never alone).
	Stubs []artifact.Stub

	// ADR-only field.
	Decided string

	// Waiver-only fields.
	Reason string
	Expiry string
}

// DecodeMeta dispatches to internal/artifact's typed decoder for kind and
// projects the result into Meta's kind-agnostic-plus-extras shape. fm is
// the artifact's raw frontmatter bytes (artifact.SplitFrontmatter's first
// return value).
func DecodeMeta(kind string, fm []byte) (Meta, error) {
	switch kind {
	case "spec":
		s, err := artifact.DecodeSpec(fm)
		if err != nil {
			return Meta{}, err
		}
		return Meta{
			Base: s.Base, Status: string(s.Status),
			Class: s.Class, Story: s.Story, Impacts: s.Impacts, Context: s.Context,
			Declares: s.Declares, AcceptanceCriteria: s.AcceptanceCriteria, Dispositions: s.Dispositions,
			Stubs: s.Stubs,
		}, nil

	case "adr":
		a, err := artifact.DecodeADR(fm)
		if err != nil {
			return Meta{}, err
		}
		return Meta{Base: a.Base, Status: string(a.Status), Decided: a.Decided}, nil

	case "diagram":
		d, err := artifact.DecodeDiagram(fm)
		if err != nil {
			return Meta{}, err
		}
		return Meta{Base: d.Base, Status: string(d.Status)}, nil

	case "attestation":
		at, err := artifact.DecodeAttestation(fm)
		if err != nil {
			return Meta{}, err
		}
		return Meta{Base: at.Base}, nil

	case "waiver":
		w, err := artifact.DecodeWaiver(fm)
		if err != nil {
			return Meta{}, err
		}
		return Meta{Base: w.Base, Status: string(w.Status), Reason: w.Reason, Expiry: w.Expiry}, nil

	case "conflict":
		c, err := artifact.DecodeConflict(fm)
		if err != nil {
			return Meta{}, err
		}
		return Meta{Base: c.Base, Status: string(c.Status)}, nil

	default:
		return Meta{}, fmt.Errorf("artifactview: unhandled kind %q", kind)
	}
}
