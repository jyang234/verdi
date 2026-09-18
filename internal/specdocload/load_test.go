package specdocload

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/store"
)

const manifestYAML = "schema: verdi.config/v1\nforge: none\n"

const lockboxSpec = `---
id: spec/lockbox
kind: spec
title: "Lockbox"
owners: [platform-team]
class: feature
problem: { text: "Keys are shared.", anchor: problem }
outcome: { text: "Each key has one holder.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A key opens one box.", evidence: [behavioral, attestation], anchor: ac-1 }
stubs:
  - { slug: key-holder, acceptance_criteria: [ac-1] }
---
# Lockbox

## Problem

Keys are shared today.

## Outcome

One holder.

## ac-1

Proven by opening.
`

// archivedLockboxSpec is lockboxSpec's twin for the archive zone
// (fix-round-1 F2's zone-precedence witnesses): a distinguishable problem
// statement so a test can prove WHICH zone's bytes were actually read,
// not just that some read succeeded.
const archivedLockboxSpec = `---
id: spec/lockbox
kind: spec
title: "Lockbox"
owners: [platform-team]
class: feature
problem: { text: "ARCHIVED: keys were shared.", anchor: problem }
outcome: { text: "Each key has one holder.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A key opens one box.", evidence: [behavioral, attestation], anchor: ac-1 }
stubs:
  - { slug: key-holder, acceptance_criteria: [ac-1] }
---
# Lockbox

## Problem

Keys were shared once, now archived.

## Outcome

One holder.

## ac-1

Proven by opening.
`

func buildRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return fixturegit.Build(t, []fixturegit.Layer{{
		Message: "adopt store with one accepted spec",
		Files: map[string]string{
			".verdi/verdi.yaml":                   manifestYAML,
			".verdi/specs/active/lockbox/spec.md": lockboxSpec,
		},
	}})
}

// buildRepoWithFiles is buildRepo generalized over the store's own files,
// for fixtures that need a specific zone layout (fix-round-1 F2) rather
// than buildRepo's fixed one-active-spec shape. Always carries the
// manifest; the caller supplies everything else.
func buildRepoWithFiles(t *testing.T, files map[string]string) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	all := map[string]string{".verdi/verdi.yaml": manifestYAML}
	for k, v := range files {
		all[k] = v
	}
	return fixturegit.Build(t, []fixturegit.Layer{{Message: "adopt store", Files: all}})
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid", "GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestLoadAcceptedMode(t *testing.T) {
	repo := buildRepo(t)
	res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	in := res.Input
	if in.Spec == nil || in.Spec.Title != "Lockbox" || in.Stamp.Ref != "spec/lockbox" || in.Stamp.Commit != repo.Head || in.Stamp.Proposed {
		t.Fatalf("input = %+v", in.Stamp)
	}
	if in.Status != "accepted-pending-build" {
		t.Fatalf("status = %q", in.Status)
	}
	if in.Facts.Coverage == nil || in.Facts.Coverage["ac-1"][0] != "key-holder" {
		t.Fatalf("coverage = %v", in.Facts.Coverage)
	}
	if in.Facts.Evidence == nil || !strings.HasPrefix(in.Facts.EvidenceSource, "matrix over the working tree at "+repo.Head[:12]) {
		t.Fatalf("evidence = %v / %q (disclosures %v)", in.Facts.Evidence, in.Facts.EvidenceSource, res.Disclosures)
	}
	if in.Facts.Readiness != nil {
		t.Fatal("no readiness was supplied")
	}
	if res.RelPath != ".verdi/specs/active/lockbox/spec.md" || res.Head != repo.Head {
		t.Fatalf("relpath/head = %q / %q", res.RelPath, res.Head)
	}
}

func TestLoadAtAndWorkingTreeModes(t *testing.T) {
	repo := buildRepo(t)
	git(t, repo.Dir, "checkout", "-q", "-b", "design/lockbox-edit")
	edited := strings.Replace(lockboxSpec, "Keys are shared.", "Keys are shared widely.", 1)
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi/specs/active/lockbox/spec.md"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo.Dir, "commit", "-qam", "edit")
	head := git(t, repo.Dir, "rev-parse", "HEAD")

	at, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAt, At: head, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if at.Input.Spec.Problem.Text != "Keys are shared widely." || at.Input.Stamp.Commit != head || at.Input.Stamp.Proposed {
		t.Fatalf("at: %+v", at.Input.Stamp)
	}

	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi/specs/active/lockbox/spec.md"), []byte(strings.Replace(edited, "widely", "UNCOMMITTED", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	wt, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wt.Input.Spec.Problem.Text, "UNCOMMITTED") || wt.Input.Stamp.Commit != head || !wt.Input.Stamp.Proposed {
		t.Fatalf("working tree: %+v / %q", wt.Input.Stamp, wt.Input.Spec.Problem.Text)
	}

	accepted, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Input.Spec.Problem.Text != "Keys are shared." || accepted.Input.Stamp.Commit != repo.Head {
		t.Fatalf("accepted must read main: %+v", accepted.Input.Stamp)
	}
}

func TestLoadWorkingTreeOnMainIsNotProposed(t *testing.T) {
	repo := buildRepo(t)
	res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if res.Input.Stamp.Proposed {
		t.Fatal("the exact accepted bytes on the default branch are not proposed")
	}
}

// TestLoadWorkingTreeExactBytesMatchAcceptedEvidence is the controller's
// preview-flag ruling's witness: matrixprojection.Project's preview
// argument is the derived `proposed` value, never the raw `req.Mode ==
// ModeWorkingTree` test. On a working tree checked out exactly on the
// default branch (Relation == RelationExact, so proposed is false),
// ModeWorkingTree and ModeAccepted must therefore pass the identical
// preview flag (false) to the matrix projection and so fold identical
// Facts.Evidence/EvidenceSource — an accepted reading folds evidence the
// same way whichever consumer asks for it. A `req.Mode == ModeWorkingTree`
// implementation would instead always pass preview=true for the working
// tree call and could diverge from the accepted reading even when the
// bytes are byte-identical.
func TestLoadWorkingTreeExactBytesMatchAcceptedEvidence(t *testing.T) {
	repo := buildRepo(t)
	wt, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	if wt.Input.Stamp.Proposed {
		t.Fatal("exact accepted bytes in the working tree must not be proposed")
	}
	if wt.Input.Facts.Evidence == nil || accepted.Input.Facts.Evidence == nil {
		t.Fatalf("both loads must compute evidence: working tree %v, accepted %v", wt.Input.Facts.Evidence, accepted.Input.Facts.Evidence)
	}
	if wt.Input.Facts.EvidenceSource != accepted.Input.Facts.EvidenceSource {
		t.Fatalf("evidence source diverged: working tree %q vs accepted %q", wt.Input.Facts.EvidenceSource, accepted.Input.Facts.EvidenceSource)
	}
	if !reflect.DeepEqual(wt.Input.Facts.Evidence, accepted.Input.Facts.Evidence) {
		t.Fatalf("evidence facts diverged:\nworking tree: %+v\naccepted:     %+v", wt.Input.Facts.Evidence, accepted.Input.Facts.Evidence)
	}
}

func TestLoadReadinessGating(t *testing.T) {
	repo := buildRepo(t)
	snap := readinesspilot.Snapshot{TargetRef: "spec/lockbox", Head: repo.Head, Areas: []readinesspilot.Area{{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateProven}}, CurrentFocus: readinesspilot.AreaShape}
	res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec, Readiness: &snap})
	if err != nil {
		t.Fatal(err)
	}
	if res.Input.Facts.Readiness == nil {
		t.Fatal("matching snapshot must be supplied")
	}
	snap.TargetRef = "spec/other"
	res, err = Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec, Readiness: &snap})
	if err != nil {
		t.Fatal(err)
	}
	if res.Input.Facts.Readiness != nil {
		t.Fatal("a snapshot for another spec must not be supplied")
	}
}

func TestLoadRefusals(t *testing.T) {
	repo := buildRepo(t)
	cases := []struct {
		name string
		req  Request
		want string
	}{
		{"unknown spec", Request{Root: repo.Dir, Name: "nope", Mode: ModeAccepted, Kind: specdoc.KindSpec}, "spec/nope not found"},
		{"bad commit", Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAt, At: "deadbeef", Kind: specdoc.KindSpec}, "deadbeef"},
		{"bad kind", Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: "chapter"}, "unknown document kind"},
		{"empty name", Request{Root: repo.Dir, Mode: ModeAccepted, Kind: specdoc.KindSpec}, "spec name"},
		{"no store", Request{Root: t.TempDir(), Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec}, "is not a verdi store root"},
		{"ModeAt without a commit", Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAt, Kind: specdoc.KindSpec}, "ModeAt requires a commit"},
		{"unknown mode", Request{Root: repo.Dir, Name: "lockbox", Mode: Mode(99), Kind: specdoc.KindSpec}, "unknown mode 99"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load(context.Background(), c.req)
			if err == nil {
				t.Fatal("want error")
			}
			if c.want != "" && !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err %q does not name %q", err, c.want)
			}
		})
	}
}

// TestLoadAcceptedModeSurvivesUnresolvableHEAD is fix-round-1 F1's ruled
// witness: a HEAD that cannot be resolved must not block a ModeAccepted
// (or ModeAt) render when the default branch itself resolves via a real
// ref. refs/remotes/origin/main stands in for a genuine remote-tracking
// ref — gitx.HasRemoteTrackingBranch only checks the ref's existence, no
// configured "origin" remote required (internal/gitx/branch.go) — and
// repointing HEAD at a branch that was never created reproduces an
// unborn HEAD (the reachable case the finding names: a checkout whose
// HEAD points at a branch that was never checked out) without disturbing
// the already-committed "main" branch specstate.ResolveDefaultBranch can
// still find locally too; the remote-tracking ref is what actually
// resolves it here, per resolveBranchRef's remote-wins-over-local
// precedence, proving this isn't accidentally passing via the local
// branch instead.
func TestLoadAcceptedModeSurvivesUnresolvableHEAD(t *testing.T) {
	repo := buildRepo(t)
	git(t, repo.Dir, "update-ref", "refs/remotes/origin/main", repo.Head)
	git(t, repo.Dir, "symbolic-ref", "HEAD", "refs/heads/nonexistent-branch")

	res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatalf("Load must still render when only HEAD (not the default branch) is unresolvable: %v", err)
	}
	if res.Input.Spec == nil || res.Input.Spec.Title != "Lockbox" {
		t.Fatalf("the render itself must still succeed: %+v", res.Input.Spec)
	}
	if res.Head != "" {
		t.Fatalf("Result.Head = %q, want empty when HEAD could not be resolved", res.Head)
	}
	if res.Input.Facts.Evidence != nil {
		t.Fatalf("Facts.Evidence = %v, want nil — WithMatrix must be skipped", res.Input.Facts.Evidence)
	}
	found := false
	for _, d := range res.Disclosures {
		if strings.HasPrefix(d, "evidence not computed: resolving HEAD: ") {
			found = true
		}
	}
	if !found {
		t.Fatalf("disclosures = %v, want one starting \"evidence not computed: resolving HEAD: \"", res.Disclosures)
	}
}

// TestLoadZonePrecedence is fix-round-1 F2's witness: loadSource's
// active-then-archive precedence is duplicated (once for the
// ModeWorkingTree disk read, once for the ModeAccepted/ModeAt git-show
// read), so swapping either list alone stays green under the existing
// tests — none of them ever loads from the archive zone at all.
func TestLoadZonePrecedence(t *testing.T) {
	t.Run("archive-only spec resolves from the archive zone", func(t *testing.T) {
		repo := buildRepoWithFiles(t, map[string]string{
			".verdi/specs/archive/lockbox/spec.md": archivedLockboxSpec,
		})
		wantRel := store.SpecRelPath(store.ZoneArchive, "lockbox")

		wt, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec})
		if err != nil {
			t.Fatal(err)
		}
		if wt.RelPath != wantRel {
			t.Fatalf("ModeWorkingTree RelPath = %q, want %q", wt.RelPath, wantRel)
		}

		accepted, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec})
		if err != nil {
			t.Fatal(err)
		}
		if accepted.RelPath != wantRel {
			t.Fatalf("ModeAccepted RelPath = %q, want %q", accepted.RelPath, wantRel)
		}
	})

	t.Run("working tree not-found names both zones tried", func(t *testing.T) {
		repo := buildRepo(t)
		_, err := Load(context.Background(), Request{Root: repo.Dir, Name: "nope", Mode: ModeWorkingTree, Kind: specdoc.KindSpec})
		if err == nil {
			t.Fatal("want error")
		}
		for _, want := range []string{
			"not found in either zone of the working tree",
			store.ActiveSpecPath(repo.Dir, "nope"),
			store.ArchiveSpecPath(repo.Dir, "nope"),
		} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("err %q does not name %q", err, want)
			}
		}
	})

	t.Run("both zones present: the active zone wins", func(t *testing.T) {
		repo := buildRepoWithFiles(t, map[string]string{
			".verdi/specs/active/lockbox/spec.md":  lockboxSpec,
			".verdi/specs/archive/lockbox/spec.md": archivedLockboxSpec,
		})
		wantRel := store.ActiveSpecRelPath("lockbox")

		wt, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec})
		if err != nil {
			t.Fatal(err)
		}
		if wt.RelPath != wantRel || wt.Input.Spec.Problem.Text != "Keys are shared." {
			t.Fatalf("ModeWorkingTree must read the active zone: relpath %q, problem %q", wt.RelPath, wt.Input.Spec.Problem.Text)
		}

		accepted, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec})
		if err != nil {
			t.Fatal(err)
		}
		if accepted.RelPath != wantRel || accepted.Input.Spec.Problem.Text != "Keys are shared." {
			t.Fatalf("ModeAccepted must read the active zone: relpath %q, problem %q", accepted.RelPath, accepted.Input.Spec.Problem.Text)
		}
	})
}

// TestLoadDisclosuresOrderAndContent is fix-round-1 F4's witness: the
// Disclosures contract (what content an entry carries, and that it is
// Disclosures[0] when evidence is the only thing that degrades) is
// otherwise asserted nowhere. Deleting the working tree's own copy of
// the spec leaves ModeAccepted's content read (git-show against main)
// unaffected, but matrixprojection.Project's storyresolve.Resolve reads
// the working tree directly and so fails — degrading evidence alone;
// status still resolves (specstate never reads the working tree), so
// this fixture cannot also independently pin the status-before-evidence
// append ORDER Load's source carries when both degrade at once — that
// relative order is a source-reading invariant, not something a live
// dual-failure fixture could cheaply force here (specstate.Resolve's
// only genuine error paths are git-plumbing failures in successor
// scanning, which risk breaking loadSource's own git-show/rev-parse
// calls on the same repo; a merely-unresolvable default branch is itself
// a disclosed Result, not a Go error, so it never reaches this path at
// all). Disclosed, not silently assumed proven.
func TestLoadDisclosuresOrderAndContent(t *testing.T) {
	repo := buildRepo(t)
	if err := os.Remove(filepath.Join(repo.Dir, ".verdi/specs/active/lockbox/spec.md")); err != nil {
		t.Fatal(err)
	}
	res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec})
	if err != nil {
		t.Fatalf("git-show must still find the spec on main even though the working tree's copy is gone: %v", err)
	}
	if len(res.Disclosures) == 0 || !strings.HasPrefix(res.Disclosures[0], "evidence not computed: ") {
		t.Fatalf("disclosures = %v, want Disclosures[0] to start \"evidence not computed: \"", res.Disclosures)
	}
	if res.Input.Facts.Evidence != nil {
		t.Fatalf("Facts.Evidence = %v, want nil", res.Input.Facts.Evidence)
	}
	if res.Input.Status != "accepted-pending-build" {
		t.Fatalf("status must still resolve (it never reads the working tree): %q", res.Input.Status)
	}
}

// TestLoadDecodesFrontmatterNotWholeFile pins the decode seam to the split
// frontmatter bytes (every other DecodeSpec caller in the module already
// does this). Decoding the whole file let the YAML parser peek one token
// past the closing "---" to find the document end, so a body whose first
// token opens with "*" (Markdown bold — examples/showcase's
// borrower-update-mobile) failed as a bogus alias; dex, the first consumer
// to render every spec, found it (wave 2 task 4).
func TestLoadDecodesFrontmatterNotWholeFile(t *testing.T) {
	spec := strings.Replace(lockboxSpec, "# Lockbox\n", "# Lockbox\n\n**Deviating fixture** opens the body with an emphasis marker.\n", 1)
	if spec == lockboxSpec {
		t.Fatal("fixture body was not rewritten")
	}
	repo := buildRepoWithFiles(t, map[string]string{".verdi/specs/active/lockbox/spec.md": spec})
	for _, mode := range []Mode{ModeAccepted, ModeAt, ModeWorkingTree} {
		res, err := Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: mode, At: repo.Head, Kind: specdoc.KindSpec})
		if err != nil {
			t.Fatalf("mode %d: %v", mode, err)
		}
		if !strings.Contains(string(res.Input.Body), "**Deviating fixture**") {
			t.Fatalf("mode %d: body lost the bold opener: %q", mode, res.Input.Body)
		}
		if res.Input.Spec.Title != "Lockbox" {
			t.Fatalf("mode %d: title = %q", mode, res.Input.Spec.Title)
		}
	}
}
