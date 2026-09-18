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
	res, _ = Load(context.Background(), Request{Root: repo.Dir, Name: "lockbox", Mode: ModeWorkingTree, Kind: specdoc.KindSpec, Readiness: &snap})
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
		{"no store", Request{Root: t.TempDir(), Name: "lockbox", Mode: ModeAccepted, Kind: specdoc.KindSpec}, ""},
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
