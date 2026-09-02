package constitutionapp

import (
	"context"
	"os"
)

// InspectRequest is Inspect's strict request. Beyond its own envelope
// version it carries no field: Inspect always reports the WHOLE constitution
// (every source layer and the complete effective rule ledger) for the
// caller's own checkout, never a caller-selected subset or a caller-named
// root — root is resolved by the adapter (CLI: the current checkout; MCP:
// the configured server root), exactly as every other operation in this
// repository resolves it, never accepted as request content.
type InspectRequest struct {
	Schema string `json:"schema"`
}

func (r InspectRequest) validate() error { return nil }

// InspectResult is Inspect's exact envelope: the accepted/proposed Git
// identity and both constitution snapshots, each carrying its own source
// layers and effective rule ledger without flattening (Wave 6 Task 3:
// "Expose source layers, effective rule ledger, applicability derivation,
// conflict witnesses, exemptions, dispositions, and affected consumers
// without flattening provenance"). Conflict witnesses and affected
// consumers are ImpactReview's own envelope — Inspect is a pure read of
// what is currently adopted, never a conflict evaluation.
type InspectResult struct {
	Schema   string   `json:"schema"`
	Identity Identity `json:"identity"`
	Accepted Snapshot `json:"accepted"`
	Proposed Snapshot `json:"proposed"`
}

// Inspect reports the accepted and proposed constitution states at their
// exact Git identities. The accepted state is read from the resolved
// default branch's exact tree via a Git-tree-backed source (authority.go's
// acceptedSource) — no second checkout. The proposed state is read from the
// current checkout's real filesystem, exactly as internal/policyauthority.
// Load already does — the current checkout IS the proposal, mirroring
// internal/draftmutation's own posture for ASD drafts.
func (s Service) Inspect(ctx context.Context, root string, req InspectRequest) (*InspectResult, *Error) {
	if root == "" {
		return nil, inputInvalid("input-invalid", errRootRequired.Error())
	}
	if err := req.validate(); err != nil {
		return nil, inputInvalid("input-invalid", err.Error())
	}
	identity, typed := s.resolveIdentity(ctx, root)
	if typed != nil {
		return nil, typed
	}

	proposed, typed := loadSnapshot(s.Authority, os.DirFS(root), identity.Head, "corrupted-policy")
	if typed != nil {
		return nil, typed
	}

	var accepted Snapshot
	if identity.AcceptedKnown {
		source, err := s.acceptedSource(ctx, root, identity.AcceptedHead)
		if err != nil {
			return nil, operational("io-failure", "reading accepted constitution tree", err)
		}
		accepted, typed = loadSnapshot(s.Authority, source, identity.AcceptedHead, "corrupted-policy")
		if typed != nil {
			return nil, typed
		}
	} else {
		accepted = Snapshot{Adopted: false, Reason: "the accepted default branch is unresolved"}
	}

	return &InspectResult{Schema: InspectResultSchema, Identity: identity, Accepted: accepted, Proposed: proposed}, nil
}
