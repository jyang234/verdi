package fake

import (
	"context"
	"errors"
	"testing"

	"github.com/OWNER/verdi/internal/forge"
	"github.com/OWNER/verdi/internal/forge/forgetest"
)

type harness struct{ f *Forge }

func (h harness) Forge() forge.Forge { return h.f }

func (h harness) SeedBundle(t *testing.T, ref, commit string, bundle forge.EvidenceBundle) {
	t.Helper()
	h.f.SeedBundle(ref, commit, bundle)
}

func (h harness) WantGeneratedAttribute() string { return "fake-generated" }

// TestFake_ContractSuite proves the fake satisfies the same behavioral
// contract the gitlab and github adapters must (04 §Testing's pattern).
func TestFake_ContractSuite(t *testing.T) {
	forgetest.Run(t, func(t *testing.T) forgetest.Harness {
		return harness{f: New()}
	})
}

func TestForge_CIContext(t *testing.T) {
	f := New()
	f.SetCIContext(forge.CIInfo{DefaultBranch: "main", IsMergeRequest: true, TargetBranch: "main"})

	got, err := f.CIContext(context.Background())
	if err != nil {
		t.Fatalf("CIContext: %v", err)
	}
	if got.DefaultBranch != "main" || !got.IsMergeRequest || got.TargetBranch != "main" {
		t.Errorf("CIContext = %+v", got)
	}
}

func TestForge_Negative_CancelledContext(t *testing.T) {
	f := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := f.FetchEvidenceBundle(ctx, "ref", "commit"); err == nil {
		t.Fatal("FetchEvidenceBundle with cancelled context: want error, got nil")
	}
	if _, err := f.CIContext(ctx); err == nil {
		t.Fatal("CIContext with cancelled context: want error, got nil")
	}
}

func TestForge_Negative_NoBundleWrapsErrNoBundle(t *testing.T) {
	f := New()
	_, err := f.FetchEvidenceBundle(context.Background(), "spec/x", "abc123")
	if !errors.Is(err, forge.ErrNoBundle) {
		t.Fatalf("error = %v, want errors.Is(err, forge.ErrNoBundle)", err)
	}
}
