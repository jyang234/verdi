package designapp

import (
	"context"
	"os"
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

func TestGetDesignCapabilities(t *testing.T) {
	t.Run("draft-write mode reports the full closed operation vocabulary", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		result, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignCapabilities: %v", err)
		}
		if result.Schema != CapabilitiesResultSchema {
			t.Fatalf("Schema = %q, want %q", result.Schema, CapabilitiesResultSchema)
		}
		if result.PolicyMode != "draft-write" {
			t.Fatalf("PolicyMode = %q, want draft-write", result.PolicyMode)
		}
		if len(result.PermittedOperations) != len(draftWriteOperations) {
			t.Fatalf("PermittedOperations = %v, want the full %d-operation vocabulary", result.PermittedOperations, len(draftWriteOperations))
		}
		if result.SpecState != specstate.Proposed {
			t.Fatalf("SpecState = %q, want proposed (unmerged design branch)", result.SpecState)
		}
		if result.MutationSchema == "" || result.ResultSchema == "" || result.PolicyDigest == "" || result.CurrentDigest == "" {
			t.Fatalf("CapabilitiesResult missing a required identity/digest field: %+v", result)
		}
		if result.Layout {
			t.Fatal("Layout must be false in v1")
		}
		if !result.Review.SemanticPacketAvailable {
			t.Fatal("Review.SemanticPacketAvailable must be true (AC-6 is non-configurable)")
		}
		if result.DirectMarkdown.Origin != "disclose" {
			t.Fatalf("DirectMarkdown.Origin = %q, want disclose (CO-1 default)", result.DirectMarkdown.Origin)
		}
		if len(result.AvailableOperations) != 6 {
			t.Fatalf("AvailableOperations = %v, want exactly the six ASD operations", result.AvailableOperations)
		}
	})

	t.Run("proposal-only mode reports no permitted operations", func(t *testing.T) {
		root := newTestStore(t, "proposal-only")
		svc := NewService()
		result, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignCapabilities: %v", err)
		}
		if len(result.PermittedOperations) != 0 {
			t.Fatalf("PermittedOperations = %v, want empty for mode proposal-only", result.PermittedOperations)
		}
	})

	t.Run("invalid ref is input-invalid", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		_, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "nope"})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "input-invalid" {
			t.Fatalf("GetDesignCapabilities(invalid ref) = %+v, want verdict input-invalid", err)
		}
	})

	t.Run("missing spec is not-found", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		_, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/does-not-exist"})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "spec-not-found" {
			t.Fatalf("GetDesignCapabilities(missing spec) = %+v, want verdict spec-not-found", err)
		}
	})

	t.Run("no policy authority adopted is a verdict", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.Policy = staticPolicySourceFor(t, nil, errPolicyNotAdopted)
		_, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
		if err == nil || err.Classification != ClassificationVerdict {
			t.Fatalf("GetDesignCapabilities(no policy authority) = %+v, want verdict", err)
		}
	})

	t.Run("nil state projector is operational", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.State = nil
		_, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
		if err == nil || err.Classification != ClassificationOperational {
			t.Fatalf("GetDesignCapabilities(nil state) = %+v, want operational", err)
		}
	})

	t.Run("nil policy source is operational", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.Policy = nil
		_, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
		if err == nil || err.Classification != ClassificationOperational {
			t.Fatalf("GetDesignCapabilities(nil policy) = %+v, want operational", err)
		}
	})
}

// TestGetDesignCapabilitiesMutability proves the advertised write
// vocabulary tracks REAL mutability across all three of the kernel's own
// preconditions — the design/<spec-name> branch, the Git-derived proposal
// state, and the design_assistance mode (AC-3's "only draft specs accept
// semantic mutations") — and never over-advertises.
//
// Each case additionally attempts the SAME mutation through
// draftmutation's own kernel on the same repository, so the two can never
// silently drift apart: a capability response claiming a write the kernel
// refuses (CO-1's "refuse semantic mutation even if an adapter mistakenly
// advertises it") fails here, and so does the reverse.
func TestGetDesignCapabilitiesMutability(t *testing.T) {
	for _, tc := range []struct {
		name             string
		mode             string
		setup            func(t *testing.T, root string)
		branch           string
		wantMutable      bool
		wantPrecondition string
		wantMutationCode draftmutation.Code
	}{
		{
			name:        "mutable draft on its own design branch",
			mode:        "draft-write",
			branch:      "design/sample",
			wantMutable: true,
		},
		{
			name:             "wrong branch",
			mode:             "draft-write",
			setup:            func(t *testing.T, root string) { checkoutNewBranch(t, root, "design/other") },
			branch:           "design/other",
			wantPrecondition: PreconditionDesignBranch,
			wantMutationCode: draftmutation.CodeStateForbidden,
		},
		{
			name:             "non-proposal Git-derived state",
			mode:             "draft-write",
			setup:            func(t *testing.T, root string) { acceptTestSpec(t, root, []byte(testSpec)) },
			branch:           "design/sample",
			wantPrecondition: PreconditionProposalState,
			wantMutationCode: draftmutation.CodeStateForbidden,
		},
		{
			name:             "forbidden policy mode",
			mode:             "off",
			branch:           "design/sample",
			wantPrecondition: PreconditionPolicyMode,
			wantMutationCode: draftmutation.CodePolicyForbidden,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newTestStore(t, tc.mode)
			if tc.setup != nil {
				tc.setup(t, root)
			}
			svc := NewService()
			result, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
			if err != nil {
				t.Fatalf("GetDesignCapabilities: %v", err)
			}
			if result.Mutable != tc.wantMutable {
				t.Fatalf("Mutable = %v, want %v (state %q, mode %q, branch %q)",
					result.Mutable, tc.wantMutable, result.SpecState, result.PolicyMode, result.Identity.Branch)
			}

			if tc.wantMutable {
				if result.MutabilityRefusal != nil {
					t.Fatalf("MutabilityRefusal = %+v, want nil for a mutable draft", result.MutabilityRefusal)
				}
				if len(result.PermittedOperations) != len(draftWriteOperations) {
					t.Fatalf("PermittedOperations = %v, want the full %d-operation vocabulary", result.PermittedOperations, len(draftWriteOperations))
				}
				if len(result.AvailableOperations) != len(availableASDOperations) {
					t.Fatalf("AvailableOperations = %v, want all six ASD operations", result.AvailableOperations)
				}
			} else {
				if result.MutabilityRefusal == nil || result.MutabilityRefusal.Precondition != tc.wantPrecondition {
					t.Fatalf("MutabilityRefusal = %+v, want precondition %q", result.MutabilityRefusal, tc.wantPrecondition)
				}
				if result.MutabilityRefusal.Detail == "" {
					t.Fatal("MutabilityRefusal.Detail must disclose the cause (CO-1)")
				}
				if len(result.PermittedOperations) != 0 {
					t.Fatalf("PermittedOperations = %v, want empty for a non-mutable spec", result.PermittedOperations)
				}
				for _, operation := range result.AvailableOperations {
					if operation == "mutate_draft" {
						t.Fatalf("AvailableOperations = %v, must withhold mutate_draft for a non-mutable spec", result.AvailableOperations)
					}
				}
				if len(result.AvailableOperations) != len(readOnlyASDOperations) {
					t.Fatalf("AvailableOperations = %v, want the five read-only operations", result.AvailableOperations)
				}
			}

			// Cross-check: the kernel's own verdict on the same repository.
			base, readErr := os.ReadFile(store.SpecPath(root, store.ZoneActive, "sample"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			request := mutateRequest(t, root, tc.branch, gitHead(t, root), base, []map[string]any{
				{"op": "set-problem", "text": "mutability cross-check", "anchor": "#problem"},
			})
			_, typed := svc.MutateDraft(context.Background(), root, request, mutateActor(t))
			if tc.wantMutable {
				if typed != nil {
					t.Fatalf("kernel refused a mutation the capability response advertised: %v", typed)
				}
				return
			}
			if typed == nil {
				t.Fatal("kernel accepted a mutation the capability response refused")
			}
			if typed.Code != tc.wantMutationCode {
				t.Fatalf("kernel refusal code = %q, want %q", typed.Code, tc.wantMutationCode)
			}
		})
	}
}
