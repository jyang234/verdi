package designapp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/store"
)

// errPolicyNotAdopted is policyauthority's own sentinel, re-exported under
// a package-local name only so every _test.go file in this package can
// name it without repeating the import.
var errPolicyNotAdopted = policyauthority.ErrNotAdopted

// staticPolicySourceFake is a fixed-answer draftmutation.PolicySource
// fake for the negative paths a real fixturegit-adopted policy store
// cannot easily exercise (e.g. "policy authority was never adopted").
// Mirrors internal/draftmutation/policy_test.go's own unexported
// staticPolicySource, which this package cannot reach across the package
// boundary.
type staticPolicySourceFake struct {
	policy *policyauthority.EffectivePolicy
	err    error
}

func (s staticPolicySourceFake) ResolveEffectivePolicy(context.Context, string) (*policyauthority.EffectivePolicy, error) {
	return s.policy, s.err
}

func staticPolicySourceFor(t *testing.T, policy *policyauthority.EffectivePolicy, err error) staticPolicySourceFake {
	t.Helper()
	return staticPolicySourceFake{policy: policy, err: err}
}

// testSpec is the one shared fixture spec: an accepted-shaped draft with
// one of every semantic object kind, so every operation under test has a
// problem/outcome/AC/constraint/decision/question/link/stub/context ref
// to report. It mirrors cmd/verdi/designmutate_test.go's own
// designMutateBaseSpec fixture (kept independent — the two packages must
// not share an unexported test symbol across package boundaries).
const testSpec = `---
id: spec/sample
kind: spec
class: feature
title: Sample
owners: [platform-team]
links: [ { type: depends-on, ref: spec/base } ]
problem: { text: "old problem", anchor: "#problem" }
outcome: { text: "old outcome", anchor: "#outcome" }
context: []
acceptance_criteria:
  - { id: ac-1, text: "first", evidence: [static], anchor: "#ac-1" }
constraints:
  - { id: co-1, text: "bounded", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "reuse base", anchor: "#dc-1" }
open_questions:
  - { id: oq-1, text: "which signal?", anchor: "#oq-1" }
stubs:
  - { slug: first-story, acceptance_criteria: [ac-1] }
---
# Sample

## Problem

Old prose stays.

## Outcome

Old prose stays.

## ac-1

First.

## co-1

Bounded.

## dc-1

Reuse base.

## oq-1

Which signal?

## first-story

First story.
`

// testSpecAccepted is testSpec with a different problem statement only —
// used as an independent "already accepted" baseline in
// acceptTestSpec-driven tests, so the design branch's current testSpec
// draft genuinely diverges from it (see acceptTestSpec's doc comment).
var testSpecAccepted = func() string {
	const old = `text: "old problem"`
	replaced := strings.Replace(testSpec, old, `text: "very old problem"`, 1)
	if replaced == testSpec {
		panic("designapp: fixture text " + old + " not found in testSpec")
	}
	return replaced
}()

// testADR is one committed, independently valid ADR every test store
// carries, so a pinned context reference (kind/name@commit) has a real
// historical target to resolve against — internal/index's own pinned-ref
// fixture shape (internal/index/pinned_test.go), reproduced here rather
// than shared across the package boundary.
const testADR = `---
id: adr/0001-context
kind: adr
title: "Pinned context ADR"
status: proposed
owners: [platform-team]
---
# Pinned context ADR

Body.
`

// newTestStore builds a hermetic fixturegit repository carrying the
// existing internal/policyauthority ASD policy fixture (the same
// testdata designmutate_test.go and policy_test.go already share — never
// a second, hand-rolled policy fixture), with the design_assistance mode
// overridden to mode, on a checked-out design/sample branch, with an
// unmodified copy of testSpec written at the active spec path. It returns
// the resolved, symlink-evaluated checkout root.
func newTestStore(t *testing.T, mode string) string {
	t.Helper()
	// The fixturegit repo has no "origin" remote at all, so
	// specstate.ResolveDefaultBranch's hermetic remote-tracking fallback
	// (defaultbranch.go) can never resolve one on its own. CI_DEFAULT_BRANCH
	// is that package's own documented override precedence #1, existing
	// exactly for this hermetic-test shape — never a second default-branch
	// resolution invented here.
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	files := map[string]string{
		".verdi/verdi.yaml":          "schema: verdi.layout/v1\n",
		".verdi/.gitignore":          "data/\n",
		".verdi/adr/0001-context.md": testADR,
	}
	source := filepath.Join("..", "policyauthority", "testdata", "store")
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if entry.Name() == "go-toolchain.md" {
			data = bytes.Replace(data, []byte("mode: proposal-only"), []byte("mode: "+mode), 1)
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "adopt draft mutation policy"}})

	checkout := exec.Command("git", "checkout", "-b", "design/sample")
	checkout.Dir = repo.Dir
	if output, err := checkout.CombinedOutput(); err != nil {
		t.Fatalf("git checkout design/sample: %v\n%s", err, output)
	}

	resolved, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.ToSlash(resolved)

	specDir := store.SpecDir(root, store.ZoneActive, "sample")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.SpecPath(root, store.ZoneActive, "sample"), []byte(testSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// childStorySpec is a minimal, independently valid story spec, written
// directly under a caller-chosen name via writeChildStory — used to prove
// GetDesignContext resolves an explicitly named child story ref (context.go).
const childStorySpecTemplate = `---
id: spec/%[1]s
kind: spec
class: story
title: Child story %[1]s
owners: [platform-team]
story: jira:CHILD-1
links: [ { type: implements, ref: spec/sample#ac-1 } ]
problem: { text: "child problem", anchor: "#problem" }
outcome: { text: "child outcome", anchor: "#outcome" }
---
# Child story %[1]s

## Problem

Child problem.

## Outcome

Child outcome.
`

// writeChildStory writes a minimal, valid active story spec named name
// directly to disk (no Git add/commit needed — GetDesignContext reads the
// active-zone working tree, exactly like storyresolve.LoadActiveSpec
// always has).
func writeChildStory(t *testing.T, root, name string) {
	t.Helper()
	dir := store.SpecDir(root, store.ZoneActive, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte(sprintfChildStory(name))
	if err := os.WriteFile(store.SpecPath(root, store.ZoneActive, name), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func sprintfChildStory(name string) string {
	return fmt.Sprintf(childStorySpecTemplate, name)
}

// runGit runs one git command inside root, failing the test on any
// non-zero exit — the shared shape acceptTestSpec and the branch/repo
// manipulating helpers below all use.
func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = filepath.FromSlash(root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}

// checkoutNewBranch moves the checkout onto a NEW branch cut from the
// current one, leaving the untracked active spec.md in place — the
// hermetic way to reach the "not on this spec's own design/<name> branch"
// mutability precondition.
func checkoutNewBranch(t *testing.T, root, name string) {
	t.Helper()
	runGit(t, root, "checkout", "-b", name)
}

// writePinnedContextSpec rewrites the active sample spec's `context:`
// block to the given pinned refs. The spec schema requires every context
// entry to parse as a pinned ref (artifact.ParsePinnedRef), so a caller
// proving the UNRESOLVABLE case must pass a well-formed pinned ref naming
// something that does not exist — not a malformed string, which the spec
// itself would reject long before resolvePinnedContext ran.
func writePinnedContextSpec(t *testing.T, root string, refs ...string) {
	t.Helper()
	block := "context: ["
	for i, ref := range refs {
		if i > 0 {
			block += ", "
		}
		block += ref
	}
	block += "]"
	replaced := strings.Replace(testSpec, "context: []", block, 1)
	if replaced == testSpec {
		panic("designapp: fixture text context: [] not found in testSpec")
	}
	if err := os.WriteFile(store.SpecPath(root, store.ZoneActive, "sample"), []byte(replaced), 0o644); err != nil {
		t.Fatal(err)
	}
}

// breakDefaultBranchRef repoints the default branch's ref at a real BLOB
// object instead of a commit. The result is a genuinely broken repository
// in exactly the shape that separates "path absent from the default
// branch" from "git could not answer at all": `git show-ref --verify` and
// `git rev-parse --verify` both still succeed (the object exists), so
// specstate.ResolveDefaultBranch still resolves a ref, while every tree
// read against it fails. It returns nothing: the point is the failure the
// caller then observes.
func breakDefaultBranchRef(t *testing.T, root, branch string) {
	t.Helper()
	cmd := exec.Command("git", "hash-object", "-w", filepath.Join(".verdi", "verdi.yaml"))
	cmd.Dir = filepath.FromSlash(root)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git hash-object: %v", err)
	}
	blob := strings.TrimSpace(string(out))
	// `git update-ref` refuses to point a branch at a non-commit, so the
	// loose ref file is written directly. A loose ref always wins over a
	// packed one, so this holds whether or not the fixture packed its refs.
	refPath := filepath.Join(filepath.FromSlash(root), ".git", "refs", "heads", filepath.FromSlash(branch))
	if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(refPath, []byte(blob+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// acceptTestSpec commits accepted onto the checkout's default branch
// (main) at the active spec path, then returns to design/sample with its
// own on-disk draft bytes exactly restored — establishing an "accepted"
// review baseline for prepare_design_review's diff-since-base tests.
// accepted is deliberately a caller-supplied, independent byte string
// (never just a copy of the current draft) so the two differ and the
// draft branch's Git-derived state resolves as a genuine, mutable
// Proposed divergence rather than an exact, immutable
// accepted-pending-build match (internal/specstate's own Relation
// classification).
func acceptTestSpec(t *testing.T, root string, accepted []byte) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = filepath.FromSlash(root)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	specPath := store.SpecPath(root, store.ZoneActive, "sample")
	draft, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	run("checkout", "main")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specPath, accepted, 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("-c", "user.name=Verdi Fixture", "-c", "user.email=fixture@verdi.invalid",
		"commit", "-m", "accept sample")
	run("checkout", "design/sample")
	// spec.md is tracked on main (just committed) but untracked on
	// design/sample (never committed there): switching branches removes it
	// from the working tree AND deletes the now-empty directory, since it
	// is "gone" in the target commit. Re-materialize the ORIGINAL draft
	// bytes (recreating the directory too) so this fixture step never
	// silently changes the draft branch's own on-disk content.
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specPath, draft, 0o644); err != nil {
		t.Fatal(err)
	}
}
