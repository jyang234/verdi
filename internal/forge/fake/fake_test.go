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

func (h harness) SeedOpenMR(t *testing.T, targetBranch, sourceBranch, title string) {
	t.Helper()
	h.f.SeedOpenMR(targetBranch, forge.OpenMR{SourceBranch: sourceBranch, Title: title})
}

func (h harness) SeedFile(t *testing.T, ref, path string, content []byte) {
	t.Helper()
	h.f.SeedFile(ref, path, content)
}

func (h harness) SeedComment(t *testing.T, mrID string, c forge.Comment) {
	t.Helper()
	h.f.SeedComment(mrID, c)
}

func (h harness) SeedThreadResolution(t *testing.T, mrID string, tr forge.ThreadResolution) {
	t.Helper()
	h.f.SeedThreadResolution(mrID, tr)
}

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
	if _, err := f.ListOpenMRs(ctx, "main"); err == nil {
		t.Fatal("ListOpenMRs with cancelled context: want error, got nil")
	}
	if _, err := f.FetchFileAtRef(ctx, "main", "path"); err == nil {
		t.Fatal("FetchFileAtRef with cancelled context: want error, got nil")
	}
	if _, err := f.ListComments(ctx, "mr-1"); err == nil {
		t.Fatal("ListComments with cancelled context: want error, got nil")
	}
	if _, err := f.PostComment(ctx, "mr-1", "body", nil); err == nil {
		t.Fatal("PostComment with cancelled context: want error, got nil")
	}
	if _, err := f.GetThreadResolution(ctx, "mr-1"); err == nil {
		t.Fatal("GetThreadResolution with cancelled context: want error, got nil")
	}
}

func TestForge_ListOpenMRs_NoneSeeded(t *testing.T) {
	f := New()
	mrs, err := f.ListOpenMRs(context.Background(), "main")
	if err != nil {
		t.Fatalf("ListOpenMRs: %v", err)
	}
	if len(mrs) != 0 {
		t.Fatalf("ListOpenMRs with nothing seeded = %+v, want empty", mrs)
	}
}

func TestForge_Negative_NoBundleWrapsErrNoBundle(t *testing.T) {
	f := New()
	_, err := f.FetchEvidenceBundle(context.Background(), "spec/x", "abc123")
	if !errors.Is(err, forge.ErrNoBundle) {
		t.Fatalf("error = %v, want errors.Is(err, forge.ErrNoBundle)", err)
	}
}
