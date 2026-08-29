package designapp

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/store"
)

// GetDesignProvenanceRequest names the one spec whose provenance sidecar
// to return. There is no other selector: AC-4's sidecar is one committed
// JSONL file per spec, and get_design_provenance returns it only on
// explicit request (AC-8) — the request naming the spec IS the explicit
// request; there is no silent default inclusion anywhere else in this
// package (context.go deliberately excludes provenance from
// get_design_context's bounded response).
type GetDesignProvenanceRequest struct {
	Spec string
}

func (r GetDesignProvenanceRequest) validate() error {
	ref, err := artifact.ParseRef(r.Spec)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return errors.New("designapp: get_design_provenance spec must be an unpinned whole spec ref")
	}
	return nil
}

// ProvenanceResult is the exact, unflattened sidecar content (AC-4): every
// decoded entry, in file order, plus the identity every ASD response
// names (AC-8). An empty Entries slice (never null) is the honest answer
// for a draft that has never been typed-mutated yet — provenance is
// non-authoritative and its absence is not a fault (CO-1).
type ProvenanceResult struct {
	Identity draftmutation.Identity   `json:"identity"`
	Entries  []designprovenance.Entry `json:"entries"`
}

// GetDesignProvenance returns the committed design-provenance sidecar for
// one spec (AC-8), decoded and validated through
// internal/designprovenance's own strict codec — never re-parsed or
// re-interpreted here. A missing sidecar (never mutated through the typed
// core yet) is a clean empty result, not an error.
func (s Service) GetDesignProvenance(ctx context.Context, start string, req GetDesignProvenanceRequest) (*ProvenanceResult, *Error) {
	if err := req.validate(); err != nil {
		return nil, inputInvalid("input-invalid", err.Error())
	}
	identity, typed := s.resolveIdentity(ctx, start, req.Spec)
	if typed != nil {
		return nil, typed
	}
	ref, err := artifact.ParseRef(identity.Spec)
	if err != nil {
		return nil, operational("authority-invalid", "parsing canonical spec identity", err)
	}
	// The spec itself must exist before its (possibly still-empty)
	// provenance sidecar is a meaningful answer: a nonexistent spec is a
	// disclosed refusal, never silently reported as "no provenance yet"
	// (CO-1: "silence is never a pass") — the same distinction
	// GetDesignCapabilities/GetDesignContext/PrepareDesignReview already
	// draw.
	if _, err := os.Stat(store.SpecPath(identity.Checkout, store.ZoneActive, ref.Name)); errors.Is(err, os.ErrNotExist) {
		return nil, notFound("spec-not-found", "no such active spec: "+identity.Spec)
	} else if err != nil {
		return nil, operational("io-failure", "checking current spec", err)
	}
	path := store.DesignProvenancePath(identity.Checkout, store.ZoneActive, ref.Name)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &ProvenanceResult{Identity: identity, Entries: []designprovenance.Entry{}}, nil
	}
	if err != nil {
		return nil, operational("io-failure", "reading design provenance", err)
	}
	entries, err := designprovenance.DecodeLog(raw)
	if err != nil {
		return nil, operational("authority-invalid", "decoding design provenance", err)
	}
	for i, entry := range entries {
		if entry.Spec != identity.Spec {
			return nil, operational("authority-invalid", "design provenance entry names a different spec",
				fmt.Errorf("designapp: design provenance entry[%d] names %q, expected %q", i, entry.Spec, identity.Spec))
		}
	}
	if entries == nil {
		entries = []designprovenance.Entry{}
	}
	return &ProvenanceResult{Identity: identity, Entries: entries}, nil
}
