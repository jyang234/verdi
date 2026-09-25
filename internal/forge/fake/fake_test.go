package fake

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/forgetest"
)

type harness struct{ f *Forge }

func (h harness) Forge() forge.Forge { return h.f }

func (h harness) SeedBundle(t *testing.T, ref, commit string, tree forge.DerivedTree) {
	t.Helper()
	h.f.SeedBundle(ref, commit, tree)
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

func TestForge_ListApprovals_Negative_NotSeeded(t *testing.T) {
	f := New()
	if _, err := f.ListApprovals(context.Background(), "17"); err == nil {
		t.Fatal("ListApprovals with nothing seeded: want error, got nil")
	}
}

const (
	seedCommit      = "6dcb09b5b57875f334f61aebed695e2e4193db5e"
	seedOtherCommit = "e5bd3914e2e596debea16f433f57875b5b90bcd6"
)

func seedMergeRecordFacts(t *testing.T, commit string) forge.MergeRecordFacts {
	t.Helper()
	facts, err := forge.NewMergeRecordFacts(forge.MergeRecordFacts{
		Supported: true, Repository: "acme/svcfix", Commit: commit, DefaultBranch: "main",
		Changes: []forge.ChangeRequestMerge{
			{ChangeID: "3", State: forge.ChangeRequestOpen, TargetBranch: "main"},
			{ChangeID: "7", State: forge.ChangeRequestMerged, TargetBranch: "main", MergeCommitSHA: commit, MergedAt: "2026-09-20T10:15:30Z"},
		},
	}, time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return facts
}

func TestForge_MergeRecords_SeededFactsAreIndependentClones(t *testing.T) {
	f := New()
	seed := seedMergeRecordFacts(t, seedCommit)
	if err := f.SeedMergeRecordFacts(seedCommit, seed); err != nil {
		t.Fatalf("SeedMergeRecordFacts: %v", err)
	}
	want := seedMergeRecordFacts(t, seedCommit)
	seed.Changes[1].TargetBranch = "release" // the caller's copy, after seeding

	first, err := f.MergeRecords(context.Background(), seedCommit)
	if err != nil {
		t.Fatalf("MergeRecords: %v", err)
	}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("MergeRecords = %+v, want the seeded facts %+v", first, want)
	}
	first.Changes[1].MergeCommitSHA = seedOtherCommit
	first.Changes = append(first.Changes, forge.ChangeRequestMerge{ChangeID: "9"})

	second, err := f.MergeRecords(context.Background(), seedCommit)
	if err != nil {
		t.Fatalf("MergeRecords second: %v", err)
	}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("MergeRecords returned aliased facts %+v, want %+v", second, want)
	}
	if err := second.Validate(); err != nil {
		t.Fatalf("returned facts do not validate: %v", err)
	}
}

func TestForge_MergeRecords_UnsupportedFactsCanBeSeeded(t *testing.T) {
	facts, err := forge.NewMergeRecordFacts(forge.MergeRecordFacts{
		Supported: false, UnsupportedReason: "fake: merge records unsupported", Repository: "acme/svcfix", Commit: seedCommit,
	}, time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	f := New()
	if err := f.SeedMergeRecordFacts(seedCommit, facts); err != nil {
		t.Fatalf("SeedMergeRecordFacts: %v", err)
	}
	got, err := f.MergeRecords(context.Background(), seedCommit)
	if err != nil || got.Supported {
		t.Fatalf("MergeRecords = %+v, %v; want the seeded unsupported facts", got, err)
	}
}

func TestForge_SeedMergeRecordFacts_Refusals(t *testing.T) {
	valid := seedMergeRecordFacts(t, seedCommit)
	tampered := seedMergeRecordFacts(t, seedCommit)
	tampered.DefaultBranch = "trunk" // changed after its digest
	tests := []struct {
		name   string
		commit string
		facts  forge.MergeRecordFacts
	}{
		{"facts never validated", seedCommit, forge.MergeRecordFacts{Supported: true, Commit: seedCommit}},
		{"zero facts", seedCommit, forge.MergeRecordFacts{}},
		{"facts changed after their digest", seedCommit, tampered},
		{"facts for another commit", seedOtherCommit, valid},
		{"an abbreviated commit key", seedCommit[:12], valid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New()
			if err := f.SeedMergeRecordFacts(tt.commit, tt.facts); err == nil {
				t.Fatalf("SeedMergeRecordFacts(%s): want error, got nil", tt.name)
			}
			if got, err := f.MergeRecords(context.Background(), tt.commit); err == nil {
				t.Fatalf("MergeRecords after a refused seed = %+v, want the unseeded error", got)
			}
		})
	}
}

func TestForge_MergeRecords_Negative(t *testing.T) {
	t.Run("unseeded commit", func(t *testing.T) {
		if got, err := New().MergeRecords(context.Background(), seedCommit); err == nil {
			t.Fatalf("MergeRecords unseeded = %+v, want error", got)
		}
	})
	t.Run("cancelled context", func(t *testing.T) {
		f := New()
		if err := f.SeedMergeRecordFacts(seedCommit, seedMergeRecordFacts(t, seedCommit)); err != nil {
			t.Fatalf("SeedMergeRecordFacts: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if got, err := f.MergeRecords(ctx, seedCommit); err == nil {
			t.Fatalf("MergeRecords with a cancelled context = %+v, want error", got)
		}
	})
}
