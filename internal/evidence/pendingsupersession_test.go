package evidence

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
)

// erroringOpenMRForge and erroringFileForge are minimal forge.Forge
// doubles that fail with a plain (non-ErrFileNotFound) transport error —
// distinguishing LoadPendingSupersessionCandidates's two real-error
// propagation paths from its "most MRs are unrelated, skip silently"
// happy path (fake.Forge alone cannot produce a non-ErrFileNotFound
// FetchFileAtRef failure, so this phase's contract-suite fake doesn't
// exercise this branch either).
type erroringOpenMRForge struct{ fakeForgeEmbed }

func (erroringOpenMRForge) ListOpenMRs(ctx context.Context, targetBranch string) ([]forge.OpenMR, error) {
	return nil, errors.New("forge: simulated transport failure listing MRs")
}

type erroringFileForge struct{ fakeForgeEmbed }

func (f erroringFileForge) ListOpenMRs(ctx context.Context, targetBranch string) ([]forge.OpenMR, error) {
	return []forge.OpenMR{{ID: "1", SourceBranch: "design/x"}}, nil
}

func (erroringFileForge) FetchFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	return nil, errors.New("forge: simulated transport failure fetching file")
}

// fakeForgeEmbed embeds a real fake.Forge so the two error doubles above
// only need to override the one method they're testing, satisfying
// forge.Forge's remaining methods via delegation.
type fakeForgeEmbed struct{ *fake.Forge }

func newFakeForgeEmbed() fakeForgeEmbed { return fakeForgeEmbed{fake.New()} }

const pendingCandidateSpecMD = `---
id: spec/loan-workflow-v2
kind: spec
class: feature
title: "Loan workflow v2 (pending, unmerged)"
status: draft
owners: [platform-team]
links:
  - { type: supersedes, ref: spec/loan-workflow }
acceptance_criteria:
  - { id: ac-1, text: "tightened outcome", evidence: [runtime, attestation] }
supersession:
  carried: []
  amended: [ { id: ac-1, note: "tightened threshold" } ]
  amended_advisory: []
  removed: [ { id: ac-2, note: "moved elsewhere" } ]
  added: []
---
# Loan workflow v2 (pending)
`

const specPath = ".verdi/specs/active/loan-workflow-v2/spec.md"

// TestPendingSupersession_EndToEnd is the exit criterion's "a
// pending-supersession case fed by an open (unmerged) supersession-MR
// manifest pulled through the forge port's fake": an open MR carries a
// genuine `supersedes` manifest naming ac-1 amended and ac-2 removed, and
// a story whose edges touch either object gets flagged.
func TestPendingSupersession_EndToEnd(t *testing.T) {
	f := fake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "7", SourceBranch: "design/loan-workflow-v2", Title: "Supersede loan-workflow"})
	f.SeedFile("design/loan-workflow-v2", specPath, []byte(pendingCandidateSpecMD))

	candidates, err := LoadPendingSupersessionCandidates(context.Background(), f, "main", "spec/loan-workflow", specPath)
	if err != nil {
		t.Fatalf("LoadPendingSupersessionCandidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %+v, want exactly 1", candidates)
	}

	t.Run("candidate carries a content digest of the exact bytes fetched (dc-5)", func(t *testing.T) {
		want := "sha256:" + fmt.Sprintf("%x", sha256.Sum256([]byte(pendingCandidateSpecMD)))
		if candidates[0].Digest != want {
			t.Fatalf("Digest = %q, want %q (sha256 of the exact fetched bytes)", candidates[0].Digest, want)
		}
	})

	t.Run("touches amended object: flagged", func(t *testing.T) {
		got := PendingSupersession(PendingSupersessionInput{ObjectIDs: []string{"ac-1"}, Candidates: candidates})
		if !got.Flagged {
			t.Fatal("Flagged = false, want true (ac-1 is amended in the pending manifest)")
		}
		if len(got.MRIDs) != 1 || got.MRIDs[0] != "7" {
			t.Fatalf("MRIDs = %v, want [7]", got.MRIDs)
		}
	})

	t.Run("touches removed object: flagged", func(t *testing.T) {
		got := PendingSupersession(PendingSupersessionInput{ObjectIDs: []string{"ac-2"}, Candidates: candidates})
		if !got.Flagged {
			t.Fatal("Flagged = false, want true (ac-2 is removed in the pending manifest)")
		}
	})

	t.Run("touches an untouched object: not flagged", func(t *testing.T) {
		got := PendingSupersession(PendingSupersessionInput{ObjectIDs: []string{"co-1"}, Candidates: candidates})
		if got.Flagged {
			t.Fatal("Flagged = true, want false (co-1 is not amended or removed by the pending manifest)")
		}
	})
}

// TestLoadPendingSupersessionCandidates_Negative covers the "most open MRs
// are unrelated" paths: no open MRs at all, an MR that doesn't touch the
// candidate path, an MR whose file doesn't supersede the feature under
// check, and an MR whose spec carries no supersession: block yet.
func TestLoadPendingSupersessionCandidates_Negative(t *testing.T) {
	t.Run("no open MRs", func(t *testing.T) {
		f := fake.New()
		got, err := LoadPendingSupersessionCandidates(context.Background(), f, "main", "spec/loan-workflow", specPath)
		if err != nil {
			t.Fatalf("LoadPendingSupersessionCandidates: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("candidates = %+v, want none", got)
		}
	})

	t.Run("open MR that doesn't touch the candidate path", func(t *testing.T) {
		f := fake.New()
		f.SeedOpenMR("main", forge.OpenMR{ID: "1", SourceBranch: "design/unrelated-change", Title: "Unrelated"})
		got, err := LoadPendingSupersessionCandidates(context.Background(), f, "main", "spec/loan-workflow", specPath)
		if err != nil {
			t.Fatalf("LoadPendingSupersessionCandidates: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("candidates = %+v, want none (file not found on that branch)", got)
		}
	})

	t.Run("open MR superseding a different feature is not a candidate", func(t *testing.T) {
		f := fake.New()
		f.SeedOpenMR("main", forge.OpenMR{ID: "2", SourceBranch: "design/other-v2", Title: "Supersede something else"})
		f.SeedFile("design/other-v2", specPath, []byte(`---
id: spec/loan-workflow-v2
kind: spec
class: feature
title: "Unrelated supersession"
status: draft
owners: [platform-team]
links:
  - { type: supersedes, ref: spec/some-other-feature }
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [attestation] }
supersession:
  carried: []
  amended: [ { id: ac-1, note: "n" } ]
  amended_advisory: []
  removed: []
  added: []
---
# Unrelated
`))
		got, err := LoadPendingSupersessionCandidates(context.Background(), f, "main", "spec/loan-workflow", specPath)
		if err != nil {
			t.Fatalf("LoadPendingSupersessionCandidates: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("candidates = %+v, want none (supersedes a different feature)", got)
		}
	})

	t.Run("open MR whose file carries no supersession block", func(t *testing.T) {
		f := fake.New()
		f.SeedOpenMR("main", forge.OpenMR{ID: "3", SourceBranch: "design/loan-workflow-v2", Title: "Still authoring"})
		f.SeedFile("design/loan-workflow-v2", specPath, []byte(`---
id: spec/loan-workflow-v2
kind: spec
class: feature
title: "Still authoring, no manifest yet"
status: draft
owners: [platform-team]
links:
  - { type: supersedes, ref: spec/loan-workflow }
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [attestation] }
---
# Still authoring
`))
		got, err := LoadPendingSupersessionCandidates(context.Background(), f, "main", "spec/loan-workflow", specPath)
		if err != nil {
			t.Fatalf("LoadPendingSupersessionCandidates: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("candidates = %+v, want none (no supersession: block yet)", got)
		}
	})

	t.Run("real transport error listing MRs propagates", func(t *testing.T) {
		f := erroringOpenMRForge{newFakeForgeEmbed()}
		if _, err := LoadPendingSupersessionCandidates(context.Background(), f, "main", "spec/loan-workflow", specPath); err == nil {
			t.Fatal("LoadPendingSupersessionCandidates: want error when ListOpenMRs fails, got nil")
		}
	})

	t.Run("real transport error fetching a file propagates (not silently skipped)", func(t *testing.T) {
		f := erroringFileForge{newFakeForgeEmbed()}
		if _, err := LoadPendingSupersessionCandidates(context.Background(), f, "main", "spec/loan-workflow", specPath); err == nil {
			t.Fatal("LoadPendingSupersessionCandidates: want error when FetchFileAtRef fails with a real transport error, got nil")
		}
	})
}

// TestPendingSupersession_Negative_NoCandidates proves an empty candidate
// set never flags — the ordinary "nothing pending" case.
func TestPendingSupersession_Negative_NoCandidates(t *testing.T) {
	got := PendingSupersession(PendingSupersessionInput{ObjectIDs: []string{"ac-1"}})
	if got.Flagged {
		t.Fatal("Flagged = true, want false with no candidates at all")
	}
}

func TestImplementsByFeature(t *testing.T) {
	tests := []struct {
		name  string
		links []artifact.Link
		want  map[string][]string
	}{
		{
			name: "groups fragment implements edges by feature name",
			links: []artifact.Link{
				{Type: artifact.LinkImplements, Ref: "spec/escrow-autopay#ac-1"},
				{Type: artifact.LinkImplements, Ref: "spec/escrow-autopay#ac-2"},
				{Type: artifact.LinkImplements, Ref: "spec/loan-workflow#ac-1"},
			},
			want: map[string][]string{
				"escrow-autopay": {"ac-1", "ac-2"},
				"loan-workflow":  {"ac-1"},
			},
		},
		{
			name: "non-implements and non-fragment links contribute nothing",
			links: []artifact.Link{
				{Type: artifact.LinkExempts, Ref: "spec/escrow-autopay#dc-2"},
				{Type: artifact.LinkImplements, Ref: "spec/loan-workflow"}, // document-level: no object id
				{Type: artifact.LinkSupersedes, Ref: "spec/old-story"},
				{Type: artifact.LinkImplements, Ref: "::not a ref::"},
			},
			want: map[string][]string{},
		},
		{
			name:  "no links at all",
			links: nil,
			want:  map[string][]string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ImplementsByFeature(tc.links)
			if len(got) != len(tc.want) {
				t.Fatalf("ImplementsByFeature: got %d features %v, want %d %v", len(got), got, len(tc.want), tc.want)
			}
			for feature, ids := range tc.want {
				gotIDs := got[feature]
				if len(gotIDs) != len(ids) {
					t.Fatalf("feature %s: got %v, want %v", feature, gotIDs, ids)
				}
				for i := range ids {
					if gotIDs[i] != ids[i] {
						t.Fatalf("feature %s: got %v, want %v", feature, gotIDs, ids)
					}
				}
			}
		})
	}
}
