package specstate

import (
	"context"
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

	t.Run("archive exact bytes: closed, exact", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		path := ".verdi/specs/archive/legacy-thing/spec.md"
		content := []byte("---\nid: spec/legacy-thing\n---\nbody\n")

		p := newProjector(stubGit{
			blobAt: exactBlobAt(path, fakeOID),
			show:   exactShow(path, content),
			fpbl:   exactLanding(path, fakeOID, fakeLanding),
			lsTree: onlyPath(), // archive-zone candidates never need the active corpus
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
		predecessorContent := []byte("---\nid: spec/old-feature\n---\nbody\n")
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
