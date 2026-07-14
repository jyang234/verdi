package artifactview

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
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
	// Problem is the round-four spec attribute (02 §Object model) — its
	// presence is the "real round-four feature vs grandfathered v0"
	// discriminator cmd/verdi/featurematrix.go established, reused by the
	// dex's feature lens.
	Problem *artifact.Attribute

	// ADR-only field.
	Decided string

	// Waiver-only fields.
	Reason string
	Expiry string

	// Obligation-only field (spec/obligation-artifact dc-1): the one
	// evidence kind this obligation states the specific proof for. First
	// exercised by real store obligations landing with spec/fail-loud —
	// before that, no committed store carried the kind and DecodeMeta
	// failed closed on it (dex could not build such a store at all).
	ForKind artifact.EvidenceKind
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
			Stubs: s.Stubs, Problem: s.Problem,
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

	case "reaffirmation":
		r, err := artifact.DecodeReaffirmation(fm)
		if err != nil {
			return Meta{}, err
		}
		// Base only, like an attestation: a reaffirmation's existence is
		// the record. First reached when shared-homes ac-4 healed the
		// index's silently-missing reaffirmation case — before that, dex
		// never received the kind from a real store walk.
		return Meta{Base: r.Base}, nil

	case "obligation":
		o, err := artifact.DecodeObligation(fm)
		if err != nil {
			return Meta{}, err
		}
		return Meta{Base: o.Base, ForKind: o.ForKind}, nil

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
