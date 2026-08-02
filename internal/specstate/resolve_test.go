package specstate

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// stubGit is the in-process fake gitReader every projector table test
// constructs a Projector over (via newProjector), so the projector's own
// decision logic is characterized without executing real git. Each
// function field panics if called unset — a test that expects a given
// gitReader method never to run (e.g. a diverged candidate should never
// need FirstParentBlobLanding) gets a loud failure instead of a silent
// nil-pointer deref if that expectation is ever violated.
type stubGit struct {
	show   func(ctx context.Context, dir, commit, path string) ([]byte, error)
	blobAt func(ctx context.Context, dir, ref, path string) (string, bool, error)
	fpbl   func(ctx context.Context, dir, ref, path, oid string) (string, bool, error)
	lsTree func(ctx context.Context, dir, ref, path string) ([]string, error)
}

func (s stubGit) Show(ctx context.Context, dir, commit, path string) ([]byte, error) {
	if s.show == nil {
		panic("stubGit: unexpected Show call")
	}
	return s.show(ctx, dir, commit, path)
}

func (s stubGit) BlobAt(ctx context.Context, dir, ref, path string) (string, bool, error) {
	if s.blobAt == nil {
		panic("stubGit: unexpected BlobAt call")
	}
	return s.blobAt(ctx, dir, ref, path)
}

func (s stubGit) FirstParentBlobLanding(ctx context.Context, dir, ref, path, oid string) (string, bool, error) {
	if s.fpbl == nil {
		panic("stubGit: unexpected FirstParentBlobLanding call")
	}
	return s.fpbl(ctx, dir, ref, path, oid)
}

func (s stubGit) LsTree(ctx context.Context, dir, ref, path string) ([]string, error) {
	if s.lsTree == nil {
		panic("stubGit: unexpected LsTree call")
	}
	return s.lsTree(ctx, dir, ref, path)
}

// buildResolvableRepo builds a fixturegit repo and points CI_DEFAULT_BRANCH
// at its local "main" branch, so the real ResolveDefaultBranch that
// ResolveMany/Resolve call internally legitimately succeeds — letting
// every OTHER test in this file drive the projector's own decision logic
// through a fake gitReader without needing real git content at the
// candidate's path (that content never exists in these fixture repos at
// all; only the stubGit fake ever answers Show/BlobAt/FirstParentBlobLanding/
// LsTree calls).
func buildResolvableRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"seed.txt": "seed\n"}, Message: "seed"},
	})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return repo
}

// buildUnresolvableRepo builds a fixturegit repo with no origin remote and
// no CI_DEFAULT_BRANCH — the real ResolveDefaultBranch legitimately fails
// for it.
func buildUnresolvableRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"seed.txt": "seed\n"}, Message: "seed"},
	})
	t.Setenv("CI_DEFAULT_BRANCH", "")
	return repo
}

const (
	fakeOID     = "1111111111111111111111111111111111aaaa"
	fakeLanding = "2222222222222222222222222222222222bbbb"
)

// TestProjector_ResolveMany covers every row of the task-3 brief's step-3
// state table.
func TestProjector_ResolveMany(t *testing.T) {
	ctx := context.Background()

	t.Run("no resolvable default ref: unproven, disclosure", func(t *testing.T) {
		repo := buildUnresolvableRepo(t)
		// A panicking stub proves branch resolution short-circuits before
		// this projector ever touches its gitReader.
		p := newProjector(stubGit{})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{
			Path:    ".verdi/specs/active/payments/spec.md",
			Content: []byte("---\nid: spec/payments\n---\n"),
		})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Unproven || result.Relation != RelationUnproven {
			t.Fatalf("Resolve = %+v, want Unproven/unproven", result)
		}
		if len(result.Disclosures) == 0 {
			t.Fatal("Resolve: want a disclosure naming the missing default branch")
		}
		if result.Baseline != nil {
			t.Fatalf("Resolve: want nil baseline, got %+v", result.Baseline)
		}
	})

	t.Run("path absent on default: proposed, new", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		p := newProjector(stubGit{
			blobAt: func(ctx context.Context, dir, ref, path string) (string, bool, error) {
				return "", false, nil
			},
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{
			Path:    ".verdi/specs/active/payments/spec.md",
			Content: []byte("---\nid: spec/payments\n---\n"),
		})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Proposed || result.Relation != RelationNew {
			t.Fatalf("Resolve = %+v, want Proposed/new", result)
		}
		if result.Baseline != nil {
			t.Fatalf("Resolve: want nil baseline, got %+v", result.Baseline)
		}
	})

	t.Run("active exact bytes, omitted status: accepted-pending-build, exact", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		path := ".verdi/specs/active/payments/spec.md"
		// Otherwise schema-valid except for the omitted status: line —
		// proving probeLegacyStatus's DecodeSpec failure here is due
		// specifically to the (today, ahead of the sibling task) required
		// status field, not some unrelated shape problem.
		content := []byte("---\nid: spec/payments\nkind: spec\nclass: feature\ntitle: Payments\nowners: [platform]\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(path, fakeOID),
			show:   exactShow(path, content),
			fpbl:   exactLanding(path, fakeOID, fakeLanding),
			lsTree: onlyPath(path),
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: path, Content: content})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		wantBaseline := &Baseline{Path: path, Blob: fakeOID, LandingCommit: fakeLanding}
		if result.State != AcceptedPendingBuild || result.Relation != RelationExact {
			t.Fatalf("Resolve = %+v, want AcceptedPendingBuild/exact", result)
		}
		if *result.Baseline != *wantBaseline {
			t.Fatalf("Resolve baseline = %+v, want %+v", result.Baseline, wantBaseline)
		}
		if len(result.Disclosures) != 0 {
			t.Fatalf("Resolve: want no disclosures for an omitted status, got %v", result.Disclosures)
		}
	})

	t.Run("active exact bytes, legacy draft: accepted-pending-build + migration disclosure, exact", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		path := ".verdi/specs/active/payments/spec.md"
		// Schema-valid (status: draft needs no frozen stamp) so
		// probeLegacyStatus's artifact.DecodeSpec call actually reads
		// status back, rather than failing for an unrelated reason.
		content := []byte("---\nid: spec/payments\nkind: spec\nclass: feature\nstatus: draft\ntitle: Payments\nowners: [platform]\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(path, fakeOID),
			show:   exactShow(path, content),
			fpbl:   exactLanding(path, fakeOID, fakeLanding),
			lsTree: onlyPath(path),
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: path, Content: content})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != AcceptedPendingBuild || result.Relation != RelationExact {
			t.Fatalf("Resolve = %+v, want AcceptedPendingBuild/exact", result)
		}
		if len(result.Disclosures) != 1 {
			t.Fatalf("Resolve: want exactly one migration disclosure, got %v", result.Disclosures)
		}
	})

	t.Run("active exact bytes, legacy accepted: accepted-pending-build, exact", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		path := ".verdi/specs/active/payments/spec.md"
		// Schema-valid: status: accepted-pending-build requires a frozen
		// stamp, so probeLegacyStatus's artifact.DecodeSpec call actually
		// reads status back, rather than failing for an unrelated reason.
		content := []byte("---\nid: spec/payments\nkind: spec\nclass: feature\nstatus: accepted-pending-build\ntitle: Payments\nowners: [platform]\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\nfrozen: { at: \"2024-01-01\", commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(path, fakeOID),
			show:   exactShow(path, content),
			fpbl:   exactLanding(path, fakeOID, fakeLanding),
			lsTree: onlyPath(path),
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: path, Content: content})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != AcceptedPendingBuild || result.Relation != RelationExact {
			t.Fatalf("Resolve = %+v, want AcceptedPendingBuild/exact", result)
		}
		if len(result.Disclosures) != 0 {
			t.Fatalf("Resolve: want no disclosures for an explicit legacy-accepted status, got %v", result.Disclosures)
		}
	})

	// fix-round-2 (Finding 4): a landed, exact, active-zone candidate whose
	// OWN frontmatter fails to decode must project Unproven with a
	// disclosure naming the failure — never silently AcceptedPendingBuild,
	// even though its (unreadable) legacy status field might have said
	// superseded or closed. Single-candidate (non-batched) proof,
	// mirroring TestProjector_ResolveMany_Batching's own batched sibling
	// subtest below.
	t.Run("active exact bytes, own content fails to decode: unproven, fail-closed", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		path := ".verdi/specs/active/broken-solo/spec.md"
		content := []byte("---\nid: spec/broken-solo\nkind: spec\nclass: feature\nunknown_field: true\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(path, fakeOID),
			show:   exactShow(path, content),
			fpbl:   exactLanding(path, fakeOID, fakeLanding),
			lsTree: onlyPath(path),
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: path, Content: content})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Unproven || result.Relation != RelationUnproven {
			t.Fatalf("Resolve = %+v, want Unproven/unproven (fail-closed: this candidate's own decode failure must block its own verdict)", result)
		}
		if len(result.Disclosures) != 1 || !strings.Contains(result.Disclosures[0], path) || !strings.Contains(result.Disclosures[0], "failed to decode") {
			t.Fatalf("Resolve disclosures = %v, want exactly one naming %s as the decode witness", result.Disclosures, path)
		}
		if result.Baseline != nil {
			t.Fatalf("Resolve: want nil baseline for an unproven result, got %+v", result.Baseline)
		}
	})

	t.Run("active exact predecessor named by a valid landed successor: superseded, exact", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		predecessorPath := ".verdi/specs/active/old-feature/spec.md"
		predecessorContent := []byte("---\nid: spec/old-feature\nkind: spec\nclass: feature\ntitle: Old Feature\nowners: [platform]\n---\nbody\n")
		successorPath := ".verdi/specs/active/new-feature/spec.md"
		successorContent := []byte(`---
id: spec/new-feature
kind: spec
class: feature
title: New Feature
owners: [platform]
status: accepted-pending-build
frozen: { at: "2024-01-01", commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }
acceptance_criteria:
  - { id: ac-1, text: works, evidence: [static] }
links:
  - { type: supersedes, ref: spec/old-feature }
supersession:
  added: [ac-1]
---
body
`)

		p := newProjector(stubGit{
			blobAt: exactBlobAt(predecessorPath, fakeOID),
			show: func(ctx context.Context, dir, commit, path string) ([]byte, error) {
				switch path {
				case predecessorPath:
					return predecessorContent, nil
				case successorPath:
					return successorContent, nil
				default:
					t.Fatalf("unexpected Show(%s)", path)
					return nil, nil
				}
			},
			fpbl:   exactLanding(predecessorPath, fakeOID, fakeLanding),
			lsTree: onlyPath(predecessorPath, successorPath),
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: predecessorPath, Content: predecessorContent})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Superseded || result.Relation != RelationExact {
			t.Fatalf("Resolve = %+v, want Superseded/exact", result)
		}
		wantBaseline := &Baseline{Path: predecessorPath, Blob: fakeOID, LandingCommit: fakeLanding}
		if *result.Baseline != *wantBaseline {
			t.Fatalf("Resolve baseline = %+v, want %+v", result.Baseline, wantBaseline)
		}
	})

	// Final fix wave I3: design §Authority names archive records as
	// authority — a successor that has itself CLOSED (moved to
	// specs/archive/) still supersedes its predecessor. Before this fix the
	// corpus scan globbed specs/active/ only, so archiving a successor
	// silently reverted its predecessor to AcceptedPendingBuild. The lsTree
	// stub here is PREFIX-FAITHFUL (underPrefix, not onlyPath) so the scan's
	// own requested prefix decides what it sees — exactly what makes this
	// row RED against an active-only scan.
	t.Run("active exact predecessor whose ONLY valid successor lives in the archive zone: superseded, exact", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		predecessorPath := ".verdi/specs/active/old-feature/spec.md"
		predecessorContent := []byte("---\nid: spec/old-feature\nkind: spec\nclass: feature\ntitle: Old Feature\nowners: [platform]\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\n---\nbody\n")
		successorPath := ".verdi/specs/archive/new-feature/spec.md"
		successorContent := validSuccessorSpec("new-feature", "old-feature")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(predecessorPath, fakeOID),
			show: func(ctx context.Context, dir, commit, path string) ([]byte, error) {
				switch path {
				case predecessorPath:
					return predecessorContent, nil
				case successorPath:
					return successorContent, nil
				default:
					t.Fatalf("unexpected Show(%s)", path)
					return nil, nil
				}
			},
			fpbl:   exactLanding(predecessorPath, fakeOID, fakeLanding),
			lsTree: underPrefix(predecessorPath, successorPath),
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: predecessorPath, Content: predecessorContent})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Superseded || result.Relation != RelationExact {
			t.Fatalf("Resolve = %+v, want Superseded/exact (archive-zone successors are authority — design §Authority)", result)
		}
		wantBaseline := &Baseline{Path: predecessorPath, Blob: fakeOID, LandingCommit: fakeLanding}
		if result.Baseline == nil || *result.Baseline != *wantBaseline {
			t.Fatalf("Resolve baseline = %+v, want %+v", result.Baseline, wantBaseline)
		}
	})

	// Final fix wave I4: a successor naming this predecessor via a
	// links: supersedes edge WITHOUT a validatable supersession: block (the
	// story-class shape, which can never carry the block) used to be
	// silently DISCARDED, projecting the predecessor AcceptedPendingBuild
	// as though no supersession claim existed at all. The honest shape is
	// disclosed-unproven: no invented mechanism (never Superseded from one
	// signal), no silent acceptance — the disclosure names the successor
	// and the missing proof.
	t.Run("active exact predecessor named by a supersedes link whose successor carries no supersession block: unproven + disclosure naming the successor", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		predecessorPath := ".verdi/specs/active/story-v1/spec.md"
		predecessorContent := []byte("---\nid: spec/story-v1\nkind: spec\nclass: story\ntitle: Story\nowners: [platform]\nstory: jira:S-1\nproblem: { text: x, anchor: problem }\noutcome: { text: y, anchor: outcome }\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\nlinks:\n  - { type: implements, ref: \"spec/some-feature#ac-1\" }\n---\nbody\n")
		successorPath := ".verdi/specs/active/story-v2/spec.md"
		// A story-class successor: internal/artifact's validateStory rejects
		// a supersession: block outright, so this shape can never carry the
		// second signal.
		successorContent := []byte("---\nid: spec/story-v2\nkind: spec\nclass: story\ntitle: Story\nowners: [platform]\nstory: jira:S-1\nproblem: { text: x, anchor: problem }\noutcome: { text: y, anchor: outcome }\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\nlinks:\n  - { type: implements, ref: \"spec/some-feature#ac-1\" }\n  - { type: supersedes, ref: \"spec/story-v1\" }\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(predecessorPath, fakeOID),
			show: func(ctx context.Context, dir, commit, path string) ([]byte, error) {
				switch path {
				case predecessorPath:
					return predecessorContent, nil
				case successorPath:
					return successorContent, nil
				default:
					t.Fatalf("unexpected Show(%s)", path)
					return nil, nil
				}
			},
			fpbl:   exactLanding(predecessorPath, fakeOID, fakeLanding),
			lsTree: underPrefix(predecessorPath, successorPath),
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: predecessorPath, Content: predecessorContent})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Unproven || result.Relation != RelationUnproven {
			t.Fatalf("Resolve = %+v, want Unproven/unproven (one-signal supersession is disclosed, never silently accepted or silently superseded)", result)
		}
		if len(result.Disclosures) != 1 || !strings.Contains(result.Disclosures[0], successorPath) || !strings.Contains(result.Disclosures[0], "supersession") {
			t.Fatalf("Resolve disclosures = %v, want exactly one naming the one-signal successor %s and the missing supersession proof", result.Disclosures, successorPath)
		}

		// The successor's own resolution is unaffected: nothing names IT as
		// a predecessor, and its own one-signal edge outbound never taints
		// its own verdict.
		p2 := newProjector(stubGit{
			blobAt: exactBlobAt(successorPath, fakeOID),
			show: func(ctx context.Context, dir, commit, path string) ([]byte, error) {
				switch path {
				case predecessorPath:
					return predecessorContent, nil
				case successorPath:
					return successorContent, nil
				default:
					t.Fatalf("unexpected Show(%s)", path)
					return nil, nil
				}
			},
			fpbl:   exactLanding(successorPath, fakeOID, fakeLanding),
			lsTree: underPrefix(predecessorPath, successorPath),
		})
		succ, err := p2.Resolve(ctx, repo.Dir, Candidate{Path: successorPath, Content: successorContent})
		if err != nil {
			t.Fatalf("Resolve(successor): %v", err)
		}
		if succ.State != AcceptedPendingBuild {
			t.Fatalf("Resolve(successor) = %+v, want AcceptedPendingBuild", succ)
		}
	})

	// I4's own negative: an OBJECT-FRAGMENT supersedes edge (spec/x#object)
	// is a decision-level override, never a whole-spec supersession claim —
	// it must neither supersede nor un-prove the predecessor.
	t.Run("active exact predecessor named only by a FRAGMENT supersedes edge with no supersession block: accepted-pending-build", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		predecessorPath := ".verdi/specs/active/frag-pred/spec.md"
		predecessorContent := []byte("---\nid: spec/frag-pred\nkind: spec\nclass: feature\ntitle: Frag Pred\nowners: [platform]\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\n---\nbody\n")
		successorPath := ".verdi/specs/active/frag-succ/spec.md"
		successorContent := []byte("---\nid: spec/frag-succ\nkind: spec\nclass: feature\ntitle: Frag Succ\nowners: [platform]\nstatus: draft\nacceptance_criteria:\n  - { id: ac-1, text: corrected, evidence: [static] }\nlinks:\n  - { type: supersedes, ref: \"spec/frag-pred#ac-1\" }\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(predecessorPath, fakeOID),
			show: func(ctx context.Context, dir, commit, path string) ([]byte, error) {
				switch path {
				case predecessorPath:
					return predecessorContent, nil
				case successorPath:
					return successorContent, nil
				default:
					t.Fatalf("unexpected Show(%s)", path)
					return nil, nil
				}
			},
			fpbl:   exactLanding(predecessorPath, fakeOID, fakeLanding),
			lsTree: underPrefix(predecessorPath, successorPath),
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: predecessorPath, Content: predecessorContent})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != AcceptedPendingBuild || len(result.Disclosures) != 0 {
			t.Fatalf("Resolve = %+v, want AcceptedPendingBuild with no disclosures (a fragment edge is a decision override, not a supersession claim)", result)
		}
	})

	// fix-round-1 finding 1: an active-zone candidate whose exact bytes are
	// landed but whose Git-derived successor proof finds NOTHING (no
	// active-zone spec validly names it as a predecessor) must still read
	// its own legacy EXPLICIT terminal status — the disclosure-seam live
	// witness shape: a story-class predecessor whose successor has
	// already been closed (moved to the archive zone, invisible to the
	// active-zone-only successor scan) so the corpus can never
	// independently confirm it.
	t.Run("active exact bytes, legacy superseded with no corpus-provable successor: superseded + compatibility disclosure, exact", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		path := ".verdi/specs/active/disclosure-seam/spec.md"
		content := []byte("---\nid: spec/disclosure-seam\nkind: spec\nclass: story\nstatus: superseded\ntitle: Disclosure seam\nowners: [platform]\nstory: jira:DS-1\nproblem: { text: x, anchor: problem }\noutcome: { text: y, anchor: outcome }\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\nlinks:\n  - { type: implements, ref: \"spec/some-feature#ac-1\" }\nfrozen: { at: \"2024-01-01\", commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(path, fakeOID),
			show:   exactShow(path, content),
			fpbl:   exactLanding(path, fakeOID, fakeLanding),
			// The corpus scan finds only this candidate itself — no
			// successor anywhere in the active zone (its successor already
			// moved to archive, per the live disclosure-seam witness).
			lsTree: onlyPath(path),
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: path, Content: content})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Superseded || result.Relation != RelationExact {
			t.Fatalf("Resolve = %+v, want Superseded/exact (legacy status compatibility read)", result)
		}
		wantBaseline := &Baseline{Path: path, Blob: fakeOID, LandingCommit: fakeLanding}
		if result.Baseline == nil || *result.Baseline != *wantBaseline {
			t.Fatalf("Resolve baseline = %+v, want %+v", result.Baseline, wantBaseline)
		}
		if len(result.Disclosures) != 1 {
			t.Fatalf("Resolve: want exactly one legacy-terminal-status compatibility disclosure, got %v", result.Disclosures)
		}
	})

	t.Run("active exact bytes, legacy closed with no corpus-provable successor: closed + compatibility disclosure, exact", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		path := ".verdi/specs/active/legacy-closed-in-place/spec.md"
		content := []byte("---\nid: spec/legacy-closed-in-place\nkind: spec\nclass: feature\nstatus: closed\ntitle: Legacy closed in place\nowners: [platform]\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\nfrozen: { at: \"2024-01-01\", commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(path, fakeOID),
			show:   exactShow(path, content),
			fpbl:   exactLanding(path, fakeOID, fakeLanding),
			lsTree: onlyPath(path),
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: path, Content: content})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Closed || result.Relation != RelationExact {
			t.Fatalf("Resolve = %+v, want Closed/exact (legacy status compatibility read, still active zone)", result)
		}
		if len(result.Disclosures) != 1 {
			t.Fatalf("Resolve: want exactly one legacy-terminal-status compatibility disclosure, got %v", result.Disclosures)
		}
	})

	t.Run("archive exact bytes: closed, exact", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		path := ".verdi/specs/archive/legacy-thing/spec.md"
		content := []byte("---\nid: spec/legacy-thing\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(path, fakeOID),
			show:   exactShow(path, content),
			fpbl:   exactLanding(path, fakeOID, fakeLanding),
			// lsTree left unset: archive-zone candidates never need the
			// active corpus, so a stray LsTree call must panic — the same
			// proof-by-unset-stub pattern the diverged case above uses for
			// FirstParentBlobLanding.
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: path, Content: content})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Closed || result.Relation != RelationExact {
			t.Fatalf("Resolve = %+v, want Closed/exact", result)
		}
	})

	t.Run("default path exists but candidate bytes differ: proposed with baseline, diverged", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		path := ".verdi/specs/active/payments/spec.md"
		defaultContent := []byte("---\nid: spec/payments\n---\ndefault body\n")
		candidateContent := []byte("---\nid: spec/payments\n---\ncandidate body\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(path, fakeOID),
			show:   exactShow(path, defaultContent),
			// FirstParentBlobLanding must never be called for a divergence
			// that is already known not to be accepted (left unset — a
			// stub call panics).
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: path, Content: candidateContent})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Proposed || result.Relation != RelationDiverged {
			t.Fatalf("Resolve = %+v, want Proposed/diverged", result)
		}
		if result.Baseline == nil || result.Baseline.Blob != fakeOID {
			t.Fatalf("Resolve baseline = %+v, want blob %s", result.Baseline, fakeOID)
		}
	})

	t.Run("blob exists but landing commit cannot be proven: unproven, unproven", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		path := ".verdi/specs/active/payments/spec.md"
		content := []byte("---\nid: spec/payments\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(path, fakeOID),
			show:   exactShow(path, content),
			fpbl: func(ctx context.Context, dir, ref, path, oid string) (string, bool, error) {
				return "", false, nil
			},
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: path, Content: content})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Unproven || result.Relation != RelationUnproven {
			t.Fatalf("Resolve = %+v, want Unproven/unproven", result)
		}
		if len(result.Disclosures) == 0 {
			t.Fatal("Resolve: want a disclosure naming the unprovable landing commit")
		}
		if result.Baseline != nil {
			t.Fatalf("Resolve: want nil baseline, got %+v", result.Baseline)
		}
	})

	t.Run("malformed default-branch successor prevents a complete supersession scan: unproven + decode disclosure, unproven", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		predecessorPath := ".verdi/specs/active/old-feature/spec.md"
		// fix-round-2 (Finding 4): must decode CLEANLY on its own — this
		// subtest characterizes the OTHER (malformedPath) corpus entry's
		// decode failure blocking the predecessor's supersession proof, not
		// the predecessor's own content. Before Finding 4's fix this content
		// was missing `kind: spec` (itself undecodable) and it went
		// unnoticed only because resolveOne never checked a candidate's own
		// corpus.failures entry at all; now it does, so a predecessor whose
		// own bytes don't decode would short-circuit to a ONE-disclosure
		// Unproven (its own failure) before ever reaching the multi-witness
		// scan-incompleteness path this subtest means to exercise.
		predecessorContent := []byte("---\nid: spec/old-feature\nkind: spec\nclass: feature\ntitle: Old Feature\nowners: [platform]\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\n---\nbody\n")
		malformedPath := ".verdi/specs/active/broken-successor/spec.md"
		malformedContent := []byte("---\nid: spec/broken-successor\nkind: spec\nclass: feature\nunknown_field: true\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(predecessorPath, fakeOID),
			show: func(ctx context.Context, dir, commit, path string) ([]byte, error) {
				switch path {
				case predecessorPath:
					return predecessorContent, nil
				case malformedPath:
					return malformedContent, nil
				default:
					t.Fatalf("unexpected Show(%s)", path)
					return nil, nil
				}
			},
			fpbl:   exactLanding(predecessorPath, fakeOID, fakeLanding),
			lsTree: onlyPath(predecessorPath, malformedPath),
		})

		result, err := p.Resolve(ctx, repo.Dir, Candidate{Path: predecessorPath, Content: predecessorContent})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Unproven || result.Relation != RelationUnproven {
			t.Fatalf("Resolve = %+v, want Unproven/unproven", result)
		}
		if len(result.Disclosures) < 2 {
			t.Fatalf("Resolve: want a scan-incompleteness disclosure plus the decode witness, got %v", result.Disclosures)
		}
		foundWitness := false
		for _, d := range result.Disclosures {
			if strings.Contains(d, malformedPath) && strings.Contains(d, "failed to decode") {
				foundWitness = true
			}
		}
		if !foundWitness {
			t.Fatalf("Resolve disclosures = %v, want one naming %s as the decode witness", result.Disclosures, malformedPath)
		}
		if result.Baseline != nil {
			t.Fatalf("Resolve: want nil baseline, got %+v", result.Baseline)
		}
	})
}

// validSuccessorSpec builds a schema-valid feature spec's bytes: accepted,
// frozen, carrying a `links: {type: supersedes}` edge to predecessorName
// plus a validated `supersession:` block — the two signals scanSuccessors
// requires before treating a spec as a real successor.
func validSuccessorSpec(name, predecessorName string) []byte {
	return []byte(`---
id: spec/` + name + `
kind: spec
class: feature
title: ` + name + `
owners: [platform]
status: accepted-pending-build
frozen: { at: "2024-01-01", commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }
acceptance_criteria:
  - { id: ac-1, text: works, evidence: [static] }
links:
  - { type: supersedes, ref: spec/` + predecessorName + `}
supersession:
  added: [ac-1]
---
body
`)
}

// TestProjector_ResolveMany_Batching proves ResolveMany's batching
// contract directly (fix-round-1 findings 1 and 2): per-candidate self-
// exclusion from the shared corpus scan, never batch-wide exclusion, and
// exactly one corpus scan per ResolveMany call regardless of how many
// candidates are in the batch.
func TestProjector_ResolveMany_Batching(t *testing.T) {
	ctx := context.Background()

	t.Run("predecessor and successor batched together both resolve correctly, agreeing with single Resolve calls", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		predecessorPath := ".verdi/specs/active/old-feature/spec.md"
		// Schema-valid: with finding 1's fix, the corpus scan now decodes
		// EVERY active-zone path unconditionally (no scan-time exclusion),
		// so the predecessor's own bytes must decode cleanly too — only
		// its OWN evaluation self-excludes its own path from the corpus's
		// failure/successor lookups, not the scan itself.
		predecessorContent := []byte(`---
id: spec/old-feature
kind: spec
class: feature
title: Old Feature
owners: [platform]
status: accepted-pending-build
frozen: { at: "2024-01-01", commit: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb }
acceptance_criteria:
  - { id: ac-1, text: works, evidence: [static] }
---
body
`)
		successorPath := ".verdi/specs/active/new-feature/spec.md"
		successorContent := validSuccessorSpec("new-feature", "old-feature")

		const predOID = "1111111111111111111111111111111111aaaa"
		const succOID = "2222222222222222222222222222222222bbbb"
		const predLanding = "3333333333333333333333333333333333cccc"
		const succLanding = "4444444444444444444444444444444444dddd"

		content := map[string][]byte{predecessorPath: predecessorContent, successorPath: successorContent}
		oid := map[string]string{predecessorPath: predOID, successorPath: succOID}
		landing := map[string]string{predecessorPath: predLanding, successorPath: succLanding}

		newStub := func() stubGit {
			return stubGit{
				show: func(ctx context.Context, dir, commit, path string) ([]byte, error) {
					b, ok := content[path]
					if !ok {
						t.Fatalf("unexpected Show(%s)", path)
					}
					return b, nil
				},
				blobAt: func(ctx context.Context, dir, ref, path string) (string, bool, error) {
					o, ok := oid[path]
					if !ok {
						return "", false, nil
					}
					return o, true, nil
				},
				fpbl: func(ctx context.Context, dir, ref, path, o string) (string, bool, error) {
					l, ok := landing[path]
					if !ok || oid[path] != o {
						t.Fatalf("unexpected FirstParentBlobLanding(%s, %s)", path, o)
					}
					return l, true, nil
				},
				lsTree: onlyPath(predecessorPath, successorPath),
			}
		}

		p := newProjector(newStub())
		results, err := p.ResolveMany(ctx, repo.Dir, []Candidate{
			{Path: predecessorPath, Content: predecessorContent},
			{Path: successorPath, Content: successorContent},
		})
		if err != nil {
			t.Fatalf("ResolveMany: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("ResolveMany returned %d results, want 2", len(results))
		}
		if results[0].State != Superseded || results[0].Relation != RelationExact {
			t.Fatalf("ResolveMany predecessor result = %+v, want Superseded/exact", results[0])
		}
		if results[1].State != AcceptedPendingBuild || results[1].Relation != RelationExact {
			t.Fatalf("ResolveMany successor result = %+v, want AcceptedPendingBuild/exact", results[1])
		}

		// A fresh Projector per single-candidate Resolve call (a stub
		// panics on out-of-scope calls, and a single-candidate call must
		// still see the OTHER path during its own corpus scan).
		p2 := newProjector(newStub())
		singlePred, err := p2.Resolve(ctx, repo.Dir, Candidate{Path: predecessorPath, Content: predecessorContent})
		if err != nil {
			t.Fatalf("Resolve(predecessor): %v", err)
		}
		if singlePred.State != results[0].State || singlePred.Relation != results[0].Relation {
			t.Fatalf("Resolve(predecessor) = %+v, disagrees with ResolveMany's %+v", singlePred, results[0])
		}

		p3 := newProjector(newStub())
		singleSucc, err := p3.Resolve(ctx, repo.Dir, Candidate{Path: successorPath, Content: successorContent})
		if err != nil {
			t.Fatalf("Resolve(successor): %v", err)
		}
		if singleSucc.State != results[1].State || singleSucc.Relation != results[1].Relation {
			t.Fatalf("Resolve(successor) = %+v, disagrees with ResolveMany's %+v", singleSucc, results[1])
		}
	})

	t.Run("a batched candidate's own malformed bytes block OTHER candidates' supersession scans, AND its own verdict (fix-round-2 Finding 4)", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		malformedPath := ".verdi/specs/active/broken-thing/spec.md"
		malformedContent := []byte("---\nid: spec/broken-thing\nkind: spec\nclass: feature\nunknown_field: true\n---\nbody\n")
		otherPath := ".verdi/specs/active/predecessor-b/spec.md"
		// Schema-valid (see the sibling subtest's identical note): only
		// malformedPath's own bytes are meant to fail decode here.
		otherContent := []byte(`---
id: spec/predecessor-b
kind: spec
class: feature
title: Predecessor B
owners: [platform]
status: accepted-pending-build
frozen: { at: "2024-01-01", commit: cccccccccccccccccccccccccccccccccccccccc }
acceptance_criteria:
  - { id: ac-1, text: works, evidence: [static] }
---
body
`)

		const malformedOID = "5555555555555555555555555555555555eeee"
		const otherOID = "6666666666666666666666666666666666ffff"
		const malformedLanding = "7777777777777777777777777777777777aaaa"
		const otherLanding = "8888888888888888888888888888888888bbbb"

		content := map[string][]byte{malformedPath: malformedContent, otherPath: otherContent}
		oid := map[string]string{malformedPath: malformedOID, otherPath: otherOID}
		landing := map[string]string{malformedPath: malformedLanding, otherPath: otherLanding}

		p := newProjector(stubGit{
			show: func(ctx context.Context, dir, commit, path string) ([]byte, error) {
				b, ok := content[path]
				if !ok {
					t.Fatalf("unexpected Show(%s)", path)
				}
				return b, nil
			},
			blobAt: func(ctx context.Context, dir, ref, path string) (string, bool, error) {
				o, ok := oid[path]
				if !ok {
					return "", false, nil
				}
				return o, true, nil
			},
			fpbl: func(ctx context.Context, dir, ref, path, o string) (string, bool, error) {
				l, ok := landing[path]
				if !ok || oid[path] != o {
					t.Fatalf("unexpected FirstParentBlobLanding(%s, %s)", path, o)
				}
				return l, true, nil
			},
			lsTree: onlyPath(malformedPath, otherPath),
		})

		results, err := p.ResolveMany(ctx, repo.Dir, []Candidate{
			{Path: otherPath, Content: otherContent},
			{Path: malformedPath, Content: malformedContent},
		})
		if err != nil {
			t.Fatalf("ResolveMany: %v", err)
		}

		// otherPath: unrelated to malformedPath's supersedes graph, but the
		// scan can't rule out that the UNREADABLE malformedPath declares
		// `supersedes: spec/predecessor-b` — so otherPath must be unproven,
		// naming malformedPath as the decode witness.
		if results[0].State != Unproven || results[0].Relation != RelationUnproven {
			t.Fatalf("ResolveMany[otherPath] = %+v, want Unproven/unproven (blocked by malformedPath's own decode failure)", results[0])
		}
		foundWitness := false
		for _, d := range results[0].Disclosures {
			if strings.Contains(d, malformedPath) && strings.Contains(d, "failed to decode") {
				foundWitness = true
			}
		}
		if !foundWitness {
			t.Fatalf("ResolveMany[otherPath] disclosures = %v, want one naming %s as the decode witness", results[0].Disclosures, malformedPath)
		}

		// malformedPath's own resolution: fix-round-2 Finding 4 — its own
		// decode failure is NOT self-excluded the way the SUPERSESSION
		// lookup is (a spec cannot supersede itself, but it very much CAN
		// fail to prove its own state): it must project Unproven too,
		// naming ITS OWN path as the decode witness, never silently
		// AcceptedPendingBuild just because probeLegacyStatus's tolerant
		// read would have swallowed the same failure.
		if results[1].State != Unproven || results[1].Relation != RelationUnproven {
			t.Fatalf("ResolveMany[malformedPath] = %+v, want Unproven/unproven (fail-closed: its own decode failure must block its own verdict)", results[1])
		}
		if len(results[1].Disclosures) != 1 || !strings.Contains(results[1].Disclosures[0], malformedPath) || !strings.Contains(results[1].Disclosures[0], "failed to decode") {
			t.Fatalf("ResolveMany[malformedPath] disclosures = %v, want exactly one naming %s as its own decode witness", results[1].Disclosures, malformedPath)
		}
	})

	t.Run("corpus is scanned exactly once per ResolveMany call, regardless of batch size", func(t *testing.T) {
		repo := buildResolvableRepo(t)

		const numCandidates = 12
		candidates := make([]Candidate, numCandidates)
		candidateContent := map[string][]byte{}
		for i := 0; i < numCandidates; i++ {
			path := fmt.Sprintf(".verdi/specs/active/predecessor-%02d/spec.md", i)
			body := []byte(fmt.Sprintf("---\nid: spec/predecessor-%02d\nkind: spec\nclass: feature\ntitle: P%02d\nowners: [platform]\n---\nbody\n", i, i))
			candidates[i] = Candidate{Path: path, Content: body}
			candidateContent[path] = body
		}

		// Two corpus-only specs, NOT among the batch's own candidates —
		// scanSuccessors must decode each exactly once, never once per
		// candidate.
		otherPaths := []string{
			".verdi/specs/active/other-a/spec.md",
			".verdi/specs/active/other-b/spec.md",
		}
		otherContent := []byte("---\nid: spec/other\nkind: spec\nclass: feature\ntitle: Other\nowners: [platform]\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\n---\nbody\n")

		var lsTreeCalls int
		showCounts := map[string]int{}
		var allPaths []string
		allPaths = append(allPaths, otherPaths...)
		for _, c := range candidates {
			allPaths = append(allPaths, c.Path)
		}

		p := newProjector(stubGit{
			lsTree: func(ctx context.Context, dir, ref, prefix string) ([]string, error) {
				lsTreeCalls++
				return allPaths, nil
			},
			show: func(ctx context.Context, dir, commit, path string) ([]byte, error) {
				showCounts[path]++
				if b, ok := candidateContent[path]; ok {
					return b, nil
				}
				return otherContent, nil
			},
			blobAt: func(ctx context.Context, dir, ref, path string) (string, bool, error) {
				return fakeOID, true, nil
			},
			fpbl: func(ctx context.Context, dir, ref, path, oid string) (string, bool, error) {
				return fakeLanding, true, nil
			},
		})

		results, err := p.ResolveMany(ctx, repo.Dir, candidates)
		if err != nil {
			t.Fatalf("ResolveMany: %v", err)
		}
		if len(results) != numCandidates {
			t.Fatalf("ResolveMany returned %d results, want %d", len(results), numCandidates)
		}
		if lsTreeCalls != 1 {
			t.Fatalf("LsTree called %d times, want exactly 1 per ResolveMany call", lsTreeCalls)
		}
		for _, path := range otherPaths {
			if showCounts[path] != 1 {
				t.Fatalf("Show(%s) called %d times, want exactly 1 (the corpus scan must decode each successor once, never once per candidate)", path, showCounts[path])
			}
		}
	})
}

// TestParseCandidatePath proves the zone/name derivation refuses any
// shape other than .verdi/specs/{active,archive}/<name>/spec.md.
func TestParseCandidatePath(t *testing.T) {
	t.Run("active", func(t *testing.T) {
		zn, name, err := parseCandidatePath(".verdi/specs/active/payments/spec.md")
		if err != nil || zn != zoneActive || name != "payments" {
			t.Fatalf("parseCandidatePath = (%v, %q, %v), want (zoneActive, \"payments\", nil)", zn, name, err)
		}
	})

	t.Run("archive", func(t *testing.T) {
		zn, name, err := parseCandidatePath(".verdi/specs/archive/legacy-thing/spec.md")
		if err != nil || zn != zoneArchive || name != "legacy-thing" {
			t.Fatalf("parseCandidatePath = (%v, %q, %v), want (zoneArchive, \"legacy-thing\", nil)", zn, name, err)
		}
	})

	badShapes := []string{
		".verdi/specs/active/payments/board.json",
		".verdi/specs/active/payments/nested/spec.md",
		".verdi/specs/staging/payments/spec.md",
		".verdi/specs/active/spec.md",
		"payments/spec.md",
		"",
	}
	for _, path := range badShapes {
		t.Run("refuses "+path, func(t *testing.T) {
			if _, _, err := parseCandidatePath(path); err == nil {
				t.Fatalf("parseCandidatePath(%q): want error, got nil", path)
			}
		})
	}
}

// exactBlobAt, exactShow, exactLanding, and onlyPath build stubGit
// function fields for the common case in this file's table: exactly one
// candidate path, with a fixed oid/content/landing.

func exactBlobAt(path, oid string) func(context.Context, string, string, string) (string, bool, error) {
	return func(ctx context.Context, dir, ref, p string) (string, bool, error) {
		if p != path {
			return "", false, nil
		}
		return oid, true, nil
	}
}

func exactShow(path string, content []byte) func(context.Context, string, string, string) ([]byte, error) {
	return func(ctx context.Context, dir, commit, p string) ([]byte, error) {
		if p != path {
			panic("exactShow: unexpected path " + p)
		}
		return content, nil
	}
}

func exactLanding(path, oid, landing string) func(context.Context, string, string, string, string) (string, bool, error) {
	return func(ctx context.Context, dir, ref, p, o string) (string, bool, error) {
		if p != path || o != oid {
			panic("exactLanding: unexpected path/oid " + p + "/" + o)
		}
		return landing, true, nil
	}
}

func onlyPath(paths ...string) func(context.Context, string, string, string) ([]string, error) {
	return func(ctx context.Context, dir, ref, prefix string) ([]string, error) {
		return paths, nil
	}
}

// underPrefix is onlyPath's PREFIX-FAITHFUL sibling: it answers each LsTree
// call with only the paths under the requested prefix, exactly as the real
// `git ls-tree -r -- <prefix>` does — load-bearing for the archive-zone
// successor rows, where WHAT the scan asks for decides what it can see.
func underPrefix(paths ...string) func(context.Context, string, string, string) ([]string, error) {
	return func(ctx context.Context, dir, ref, prefix string) ([]string, error) {
		var out []string
		for _, p := range paths {
			if strings.HasPrefix(p, prefix+"/") {
				out = append(out, p)
			}
		}
		return out, nil
	}
}
