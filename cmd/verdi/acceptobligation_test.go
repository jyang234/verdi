package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// obligationSeamStoryCleanMD is a fully lint-clean draft story spec (proven
// against the real engine by the same recipe supersedepredecessor_test.go's
// succStorySupersedesMD already proves: problem/outcome/AC anchors actually
// resolving against the ## Problem/## Outcome/## AC-N headings below, and an
// implements edge to someFeatureMD — defined in supersedepredecessor_test.go,
// reused here rather than re-invented — so the link resolves rather than
// dangling). Two ACs, two different declared evidence kinds, neither with a
// pre-existing obligation: the backstop's own core happy-path shape.
const obligationSeamStoryCleanMD = `---
id: spec/widget-story
kind: spec
title: "Widget story"
owners: [platform-team]
class: story
status: draft
story: jira:LOAN-9001
problem: { text: "widgets are stale", anchor: problem }
outcome: { text: "widgets are current", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static evidence holds", evidence: [static], anchor: ac-1 }
  - { id: ac-2, text: "behavioral evidence holds", evidence: [behavioral], anchor: ac-2 }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# Widget story

## Problem

Widgets are stale.

## Outcome

Widgets are current.

## AC-1

Static evidence holds.

## AC-2

Behavioral evidence holds.
`

// obligationSeamStoryDanglingLayoutJSON induces an UNRELATED quartet lint
// refusal (VL-018, D6-23's own witness shape) on obligationSeamStoryCleanMD:
// a positions key naming no declared object at all — never VL-020, which
// tolerates every draft regardless (co-2) — proving accept's obligation
// backstop cleanup (O-1b) fires for ANY post-scaffold refusal, not only an
// obligation-shaped one.
const obligationSeamStoryDanglingLayoutJSON = `{
  "schema": "verdi.boardlayout/v1",
  "positions": {
    "ac-1": { "x": 40, "y": 20 },
    "no-such-ac": { "x": 990, "y": 40 }
  }
}
`

const preExistingAc1StaticMD = `---
id: obligation/widget-story--ac-1--static
kind: obligation
title: "PRE-EXISTING-DISTINCTIVE-MARKER"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# PRE-EXISTING-DISTINCTIVE-MARKER

A hand-authored obligation that must never be clobbered.
`

// malformedAc2BehavioralMD sits at ac-2's exact convention path but fails
// artifact.DecodeObligation: its for_kind ("static") disagrees with its own
// id's "--behavioral" segment (DC-2's id/for_kind agreement) — the
// present-but-undecodable case spec/obligation-seam ac-2's third case
// covers.
const malformedAc2BehavioralMD = `---
id: obligation/widget-story--ac-2--behavioral
kind: obligation
title: "malformed on purpose"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/widget-story" }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# malformed on purpose

for_kind disagrees with the id's own --behavioral segment.
`

// buildObligationSeamStoryRepo builds a one-layer fixturegit repo carrying
// obligationSeamStoryCleanMD, its implements-edge target (someFeatureMD,
// supersedepredecessor_test.go), and any extra files the caller supplies
// (obligation fixtures, layout.json, ...) — mirroring
// buildPredecessorFlipRepo's own "hand-written frontmatter, no design-start
// scaffold" posture.
func buildObligationSeamStoryRepo(t *testing.T, extra map[string]string) *fixturegit.Repo {
	t.Helper()
	files := map[string]string{
		".verdi/verdi.yaml":                        phase7ManifestYAML,
		".gitattributes":                           phase7GitAttributes,
		".verdi/specs/active/some-feature/spec.md": someFeatureMD,
		".verdi/specs/active/widget-story/spec.md": obligationSeamStoryCleanMD,
	}
	for k, v := range extra {
		files[k] = v
	}
	return fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "init store with widget-story draft"}})
}

func readObligation(t *testing.T, path string) (*artifact.ObligationFrontmatter, []byte) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	fm, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("splitting frontmatter of %s: %v", path, err)
	}
	ob, err := artifact.DecodeObligation(fm)
	if err != nil {
		t.Fatalf("decoding obligation %s: %v\n%s", path, err, raw)
	}
	return ob, body
}

func obligationPathFor(root, acID, kind string) string {
	return filepath.Join(root, ".verdi", "obligations", "widget-story", acID+"--"+kind+".md")
}

// TestRunAccept_ScaffoldsMissingObligations_Happy is spec/obligation-seam
// ac-1's core proof: a draft story declaring two (ac, kind) pairs, neither
// backed by an obligation, accepts successfully, and both are scaffolded —
// stamped the captured pre-flip HEAD's own commit and committer date, owned
// by the accepting operator, carrying the O-6 disclosure line — landing
// inside the SAME accept commit as the status flip (O-2).
func TestRunAccept_ScaffoldsMissingObligations_Happy(t *testing.T) {
	t.Setenv("USER", "test-operator")
	repo := buildObligationSeamStoryRepo(t, nil)
	ctx := context.Background()

	beforeHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	wantDate, err := gitx.CommitDate(ctx, repo.Dir, beforeHead)
	if err != nil {
		t.Fatal(err)
	}
	wantAt := wantDate[:10]

	var stdout, stderr bytes.Buffer
	if got := runAccept(ctx, repo.Dir, "spec/widget-story", &stdout, &stderr); got != 0 {
		t.Fatalf("runAccept = %d, want 0; stderr=%s", got, stderr.String())
	}

	for _, tc := range []struct {
		acID, kind string
	}{
		{"ac-1", "static"},
		{"ac-2", "behavioral"},
	} {
		path := obligationPathFor(repo.Dir, tc.acID, tc.kind)
		ob, body := readObligation(t, path)
		if ob.ForKind != artifact.EvidenceKind(tc.kind) {
			t.Errorf("%s: for_kind = %q, want %q", path, ob.ForKind, tc.kind)
		}
		if ob.Frozen == nil || ob.Frozen.Commit != beforeHead {
			t.Errorf("%s: frozen.commit = %+v, want the captured pre-flip HEAD %q", path, ob.Frozen, beforeHead)
		}
		if ob.Frozen.At != wantAt {
			t.Errorf("%s: frozen.at = %q, want %q (the pre-flip commit's own committer date)", path, ob.Frozen.At, wantAt)
		}
		if len(ob.Owners) != 1 || ob.Owners[0] != "test-operator" {
			t.Errorf("%s: owners = %v, want [test-operator] (O-6: the accepting operator)", path, ob.Owners)
		}
		if !contains(string(body), obligationBackstopDisclosureLine) {
			t.Errorf("%s: body does not carry the O-6 disclosure line verbatim:\n%s", path, body)
		}
	}

	afterHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := gitx.DiffNameStatus(ctx, repo.Dir, beforeHead, afterHead)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := map[string]bool{
		".verdi/specs/active/widget-story/spec.md":            true,
		".verdi/obligations/widget-story/ac-1--static.md":     true,
		".verdi/obligations/widget-story/ac-2--behavioral.md": true,
	}
	if len(entries) != len(wantPaths) {
		t.Fatalf("accept commit diff = %+v, want exactly %v", entries, wantPaths)
	}
	for _, e := range entries {
		if !wantPaths[e.Path] {
			t.Errorf("accept commit diff contains unexpected path %q", e.Path)
		}
	}
}

// TestRunAccept_SkipsExistingObligations_NeverOverwrites is ac-2's core
// proof: a pre-existing, decodable obligation is left byte-for-byte
// untouched and out of the accept commit entirely, while the still-missing
// pair is scaffolded and staged normally.
func TestRunAccept_SkipsExistingObligations_NeverOverwrites(t *testing.T) {
	repo := buildObligationSeamStoryRepo(t, map[string]string{
		".verdi/obligations/widget-story/ac-1--static.md": preExistingAc1StaticMD,
	})
	ctx := context.Background()

	beforeHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if got := runAccept(ctx, repo.Dir, "spec/widget-story", &stdout, &stderr); got != 0 {
		t.Fatalf("runAccept = %d, want 0; stderr=%s", got, stderr.String())
	}

	ac1Path := obligationPathFor(repo.Dir, "ac-1", "static")
	got, err := os.ReadFile(ac1Path)
	if err != nil {
		t.Fatalf("reading %s: %v", ac1Path, err)
	}
	if string(got) != preExistingAc1StaticMD {
		t.Fatalf("pre-existing obligation was modified:\n--- got ---\n%s\n--- want (byte-identical) ---\n%s", got, preExistingAc1StaticMD)
	}

	ac2Path := obligationPathFor(repo.Dir, "ac-2", "behavioral")
	if _, err := os.Stat(ac2Path); err != nil {
		t.Fatalf("missing pair ac-2/behavioral was not scaffolded: %v", err)
	}

	afterHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := gitx.DiffNameStatus(ctx, repo.Dir, beforeHead, afterHead)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Path == ".verdi/obligations/widget-story/ac-1--static.md" {
			t.Fatalf("accept commit diff must not touch the pre-existing obligation, got %+v", entries)
		}
	}
	if len(entries) != 2 {
		t.Fatalf("accept commit diff = %+v, want exactly 2 entries (spec.md + the ac-2 scaffold)", entries)
	}
}

// TestRunAccept_MalformedExistingObligation_RefusesOperationally is ac-2's
// third case: a present-but-undecodable file at a declared pair's exact
// convention path refuses accept operationally (exit 2) rather than
// silently overwriting it or silently treating it as covered — and any
// obligation the backstop DID manage to scaffold for an earlier AC before
// hitting this failure is unlinked (O-1b applies even when scaffolding
// itself is what failed).
func TestRunAccept_MalformedExistingObligation_RefusesOperationally(t *testing.T) {
	repo := buildObligationSeamStoryRepo(t, map[string]string{
		".verdi/obligations/widget-story/ac-2--behavioral.md": malformedAc2BehavioralMD,
	})
	ctx := context.Background()

	beforeHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "spec/widget-story", &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runAccept(malformed existing obligation) = %d, want 2 (operational); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !contains(stderr.String(), "ac-2") {
		t.Fatalf("stderr = %q, want it to name ac-2", stderr.String())
	}

	afterHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if afterHead != beforeHead {
		t.Fatal("a refused accept must not create a commit")
	}

	// ac-1's pair, processed before ac-2 hit the decode failure, must have
	// been scaffolded-then-unlinked: nothing left behind.
	if _, err := os.Stat(obligationPathFor(repo.Dir, "ac-1", "static")); !os.IsNotExist(err) {
		t.Errorf("ac-1's scaffold was not cleaned up after the ac-2 failure (err=%v)", err)
	}

	// The malformed file itself is untouched — refused, never clobbered.
	ac2Path := obligationPathFor(repo.Dir, "ac-2", "behavioral")
	got2, err := os.ReadFile(ac2Path)
	if err != nil {
		t.Fatalf("reading %s: %v", ac2Path, err)
	}
	if string(got2) != malformedAc2BehavioralMD {
		t.Fatalf("the malformed pre-existing file was modified:\n--- got ---\n%s\n--- want (byte-identical) ---\n%s", got2, malformedAc2BehavioralMD)
	}
}

// TestRunAccept_UnrelatedRefusal_UnlinksNewlyScaffoldedObligations is ac-3's
// core proof: an unrelated quartet lint violation (VL-018's dangling
// layout.json key, D6-23's own witness shape — never an obligation-shaped
// one) refuses accept AFTER the backstop has already scaffolded both
// missing pairs; both are unlinked, the obligations directory itself
// (which did not exist before this invocation) is gone afterward, and the
// tree is otherwise exactly as it started.
func TestRunAccept_UnrelatedRefusal_UnlinksNewlyScaffoldedObligations(t *testing.T) {
	repo := buildObligationSeamStoryRepo(t, map[string]string{
		".verdi/specs/active/widget-story/layout.json": obligationSeamStoryDanglingLayoutJSON,
	})
	ctx := context.Background()

	beforeHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	_, rawBefore := readSpec(t, repo.Dir, "widget-story")

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "spec/widget-story", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAccept(dangling layout key) = %d, want 1 (verdict refusal); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !contains(stderr.String(), "VL-018") {
		t.Fatalf("stderr = %q, want it to name VL-018 verbatim", stderr.String())
	}

	afterHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if afterHead != beforeHead {
		t.Fatal("a refused accept must not create a commit")
	}
	_, rawAfter := readSpec(t, repo.Dir, "widget-story")
	if !bytes.Equal(rawBefore, rawAfter) {
		t.Fatal("a refused accept must not touch the spec")
	}

	if _, err := os.Stat(obligationPathFor(repo.Dir, "ac-1", "static")); !os.IsNotExist(err) {
		t.Errorf("ac-1's scaffold was not unlinked after the unrelated refusal (err=%v)", err)
	}
	if _, err := os.Stat(obligationPathFor(repo.Dir, "ac-2", "behavioral")); !os.IsNotExist(err) {
		t.Errorf("ac-2's scaffold was not unlinked after the unrelated refusal (err=%v)", err)
	}
	obligationDir := filepath.Join(repo.Dir, ".verdi", "obligations", "widget-story")
	if _, err := os.Stat(obligationDir); !os.IsNotExist(err) {
		t.Errorf("the obligations directory (absent before this invocation) must be gone afterward too, err=%v", err)
	}

	// The surface the backstop defers to must never be pre-empted by its
	// own orphaned scaffold (O-1b's whole point): a subsequent `obligation
	// author` for the same pair succeeds as an ordinary create.
	stdout.Reset()
	stderr.Reset()
	if got := runObligationAuthor(ctx, repo.Dir, "spec/widget-story", "ac-1", "static", "", &stdout, &stderr); got != 0 {
		t.Fatalf("obligation author after the cleaned-up refusal = %d, want 0; stderr=%s", got, stderr.String())
	}
}

// TestScaffoldMissingObligations is scaffoldMissingObligations' own
// table-driven unit test, exercised directly (no git, no accept) — happy
// paths and negative paths per CLAUDE.md's testing rule.
func TestScaffoldMissingObligations(t *testing.T) {
	frozen := artifact.NewFrozen("2026-07-21", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

	decodeFixtureSpec := func(t *testing.T, md string) *artifact.SpecFrontmatter {
		t.Helper()
		fm, _, err := artifact.SplitFrontmatter([]byte(md))
		if err != nil {
			t.Fatalf("SplitFrontmatter: %v", err)
		}
		spec, err := artifact.DecodeSpec(fm)
		if err != nil {
			t.Fatalf("DecodeSpec: %v", err)
		}
		return spec
	}

	t.Run("feature class is a no-op", func(t *testing.T) {
		root := t.TempDir()
		spec := decodeFixtureSpec(t, someFeatureMD)
		created, err := scaffoldMissingObligations(root, "some-feature", spec, frozen, "op")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if created != nil {
			t.Errorf("created = %v, want nil (dc-3: features never carry obligations)", created)
		}
	})

	t.Run("story with everything already covered is a no-op", func(t *testing.T) {
		root := t.TempDir()
		writeTestFile(t, obligationPathFor(root, "ac-1", "static"), []byte(preExistingAc1StaticMD))
		spec := decodeFixtureSpec(t, `---
id: spec/widget-story
kind: spec
class: story
title: "t"
owners: [platform-team]
status: draft
story: jira:LOAN-1
problem: { text: "p", anchor: problem }
outcome: { text: "o", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [static] }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# t
`)
		created, err := scaffoldMissingObligations(root, "widget-story", spec, frozen, "op")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if created != nil {
			t.Errorf("created = %v, want nil (already covered)", created)
		}
	})

	t.Run("scaffolds only the missing pair, multiple kinds on one AC", func(t *testing.T) {
		root := t.TempDir()
		spec := decodeFixtureSpec(t, `---
id: spec/widget-story
kind: spec
class: story
title: "t"
owners: [platform-team]
status: draft
story: jira:LOAN-1
problem: { text: "p", anchor: problem }
outcome: { text: "o", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [static, behavioral] }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# t
`)
		created, err := scaffoldMissingObligations(root, "widget-story", spec, frozen, "op")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		wantPaths := []string{
			obligationPathFor(root, "ac-1", "static"),
			obligationPathFor(root, "ac-1", "behavioral"),
		}
		if len(created) != len(wantPaths) {
			t.Fatalf("created = %v, want %v", created, wantPaths)
		}
		for _, p := range wantPaths {
			if _, err := os.Stat(p); err != nil {
				t.Errorf("expected scaffold at %s: %v", p, err)
			}
		}
	})

	t.Run("malformed existing file at the convention path errors, reporting whatever was created first", func(t *testing.T) {
		root := t.TempDir()
		writeTestFile(t, obligationPathFor(root, "ac-2", "behavioral"), []byte(malformedAc2BehavioralMD))
		spec := decodeFixtureSpec(t, `---
id: spec/widget-story
kind: spec
class: story
title: "t"
owners: [platform-team]
status: draft
story: jira:LOAN-1
problem: { text: "p", anchor: problem }
outcome: { text: "o", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [static] }
  - { id: ac-2, text: "y", evidence: [behavioral] }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# t
`)
		created, err := scaffoldMissingObligations(root, "widget-story", spec, frozen, "op")
		if err == nil {
			t.Fatal("err = nil, want a decode-failure error")
		}
		if len(created) != 1 || created[0] != obligationPathFor(root, "ac-1", "static") {
			t.Errorf("created = %v, want exactly [%s] (ac-1 scaffolded before the ac-2 failure)", created, obligationPathFor(root, "ac-1", "static"))
		}
	})
}

// TestOperatorOwner proves the $USER/fallback-sentinel contract (O-6).
func TestOperatorOwner(t *testing.T) {
	t.Run("USER set", func(t *testing.T) {
		t.Setenv("USER", "alice")
		if got := operatorOwner(); got != "alice" {
			t.Errorf("operatorOwner() = %q, want alice", got)
		}
	})
	t.Run("USER unset falls back to the disclosed sentinel", func(t *testing.T) {
		t.Setenv("USER", "")
		if got := operatorOwner(); got != fallbackOperatorOwner {
			t.Errorf("operatorOwner() = %q, want the sentinel %q", got, fallbackOperatorOwner)
		}
	})
}

// TestBackstopObligationBody proves the rendered body always carries the
// O-6 disclosure line verbatim plus the acceptance criterion's own
// declared text — never a fabricated claim about what the evidence
// specifically shows.
func TestBackstopObligationBody(t *testing.T) {
	got := backstopObligationBody("spec/widget-story", "ac-1", artifact.EvidenceBehavioral, "the retry proves end to end")
	if !contains(got, obligationBackstopDisclosureLine) {
		t.Errorf("body missing the O-6 disclosure line verbatim:\n%s", got)
	}
	if !contains(got, "ac-1") || !contains(got, "behavioral") {
		t.Errorf("body does not name the (ac, kind) pair:\n%s", got)
	}
	if !contains(got, "the retry proves end to end") {
		t.Errorf("body does not carry the acceptance criterion's own declared text:\n%s", got)
	}
}

// TestUnlinkScaffoldedObligations is unlinkScaffoldedObligations' own unit
// test: it removes exactly the given paths, tolerates one already gone,
// and removes the parent directory only when it did not pre-exist AND is
// now actually empty.
func TestUnlinkScaffoldedObligations(t *testing.T) {
	t.Run("removes exactly the given paths and the now-empty, newly-created dir", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "obligations", "widget-story")
		p1 := filepath.Join(dir, "ac-1--static.md")
		p2 := filepath.Join(dir, "ac-2--behavioral.md")
		writeTestFile(t, p1, []byte("x"))
		writeTestFile(t, p2, []byte("y"))

		var stderr bytes.Buffer
		unlinkScaffoldedObligations([]string{p1, p2}, dir, false, &stderr)

		if _, err := os.Stat(p1); !os.IsNotExist(err) {
			t.Errorf("p1 still exists: %v", err)
		}
		if _, err := os.Stat(p2); !os.IsNotExist(err) {
			t.Errorf("p2 still exists: %v", err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("newly-created, now-empty dir still exists: %v", err)
		}
	})

	t.Run("tolerates a path already gone", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "obligations", "widget-story")
		p1 := filepath.Join(dir, "ac-1--static.md")
		writeTestFile(t, p1, []byte("x"))
		if err := os.Remove(p1); err != nil {
			t.Fatal(err)
		}

		var stderr bytes.Buffer
		unlinkScaffoldedObligations([]string{p1}, dir, false, &stderr)
		if stderr.Len() != 0 {
			t.Errorf("stderr = %q, want no warning for an already-gone path", stderr.String())
		}
	})

	t.Run("never removes a directory that pre-existed this invocation", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "obligations", "widget-story")
		p1 := filepath.Join(dir, "ac-1--static.md")
		writeTestFile(t, p1, []byte("x"))

		unlinkScaffoldedObligations([]string{p1}, dir, true /* preExisted */, io.Discard)

		if _, err := os.Stat(dir); err != nil {
			t.Errorf("a pre-existing directory must survive cleanup even once empty: %v", err)
		}
	})

	t.Run("leaves a newly-created dir alone if other, pre-existing files still live there", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "obligations", "widget-story")
		p1 := filepath.Join(dir, "ac-1--static.md")
		other := filepath.Join(dir, "ac-9--static.md")
		writeTestFile(t, p1, []byte("x"))
		writeTestFile(t, other, []byte("pre-existing, not part of this invocation"))

		unlinkScaffoldedObligations([]string{p1}, dir, false, io.Discard)

		if _, err := os.Stat(dir); err != nil {
			t.Errorf("a non-empty dir must survive cleanup: %v", err)
		}
		if _, err := os.Stat(other); err != nil {
			t.Errorf("the unrelated pre-existing file must survive: %v", err)
		}
	})
}

// TestAccept_ScaffoldingPrecedesLintGateInSource is a source-order witness
// (spec/obligation-seam ac-1's own "companion test" requirement): the
// scaffolding call must textually precede the quartet lint gate call inside
// runAccept, mirroring this package's existing source-text witness style
// (TestObligationAuthor_AtomicWrite_NoDirectCreateTemp, internal/workbench).
// A behavioral test alone cannot observe "before" directly since VL-020
// itself tolerates every draft regardless of scaffolding order (co-2); this
// pins the actual control-flow fact the behavioral tests above depend on.
func TestAccept_ScaffoldingPrecedesLintGateInSource(t *testing.T) {
	src, err := os.ReadFile("accept.go")
	if err != nil {
		t.Fatalf("reading accept.go: %v", err)
	}
	scaffoldIdx := bytes.Index(src, []byte("scaffoldMissingObligations("))
	lintGateIdx := bytes.Index(src, []byte("lintQuartetOrRefuse(ctx, root, ref, spec, stderr)"))
	if scaffoldIdx < 0 {
		t.Fatal("accept.go no longer calls scaffoldMissingObligations — has the backstop moved or been removed?")
	}
	if lintGateIdx < 0 {
		t.Fatal("accept.go no longer calls lintQuartetOrRefuse — has D6-23's gate moved or been removed?")
	}
	if scaffoldIdx >= lintGateIdx {
		t.Errorf("scaffoldMissingObligations (offset %d) does not precede lintQuartetOrRefuse (offset %d) in accept.go's source — O-1's ordering requirement is violated", scaffoldIdx, lintGateIdx)
	}
}
