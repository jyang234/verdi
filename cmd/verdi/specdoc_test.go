package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

const specDocFixture = `---
id: spec/lockbox
kind: spec
title: "Lockbox"
owners: [platform-team]
class: feature
problem: { text: "Keys are shared.", anchor: problem }
outcome: { text: "Each key has one holder.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A key opens one box.", evidence: [behavioral, attestation], anchor: ac-1 }
constraints:
  - { id: co-1, text: "No network.", anchor: co-1 }
decisions:
  - { id: dc-1, text: "One holder per key.", anchor: dc-1 }
open_questions:
  - { id: oq-1, text: "Who audits holders?", anchor: oq-1 }
stubs:
  - { slug: key-holder, acceptance_criteria: [ac-1] }
  - { slug: audit-probe, spike: true, resolves: [oq-1] }
---
# Lockbox

## Problem

Keys are shared today.

## Outcome

One holder.

## ac-1

Proven by opening.

## co-1

## dc-1

Because two holders means no holder.

## oq-1
`

func buildSpecDocRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Message: "adopt store with one accepted spec",
		Files: map[string]string{
			".verdi/verdi.yaml":                   supersedeManifestYAML, // the package's shared minimal manifest (designsupersede_test.go:17)
			".verdi/specs/active/lockbox/spec.md": specDocFixture,
		},
	}})
}

func TestSpecDoc_RendersAcceptedSpecToStdout(t *testing.T) {
	repo := buildSpecDocRepo(t)
	bin := buildVerdiBinary(t)
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "spec", "doc", "spec/lockbox")
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	for _, want := range []string{"# Lockbox", "## Acceptance criteria", "**ac-1** A key opens one box.", "covered by", "`key-holder`", "commit `" + repo.Head + "`", "not authority"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "Proposed, not") {
		t.Errorf("accepted render must not carry the proposed header")
	}
}

func TestSpecDoc_KindsFormatsAndOutputFile(t *testing.T) {
	repo := buildSpecDocRepo(t)
	bin := buildVerdiBinary(t)
	out := filepath.Join(t.TempDir(), "tasks.html")
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "spec", "doc", "spec/lockbox", "--kind", "tasks", "--format", "html", "-o", out)
	if code != 0 {
		t.Fatalf("exit %d, stderr:\n%s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("-o must leave stdout empty, got %q", stdout)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	if !strings.Contains(html, "<h2") || strings.Contains(html, "Acceptance criteria") || !strings.Contains(html, "Plan") {
		t.Errorf("tasks html wrong shape:\n%s", html)
	}
	stdout, _, code = runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "spec", "doc", "spec/lockbox", "--kind=plan")
	if code != 0 || !strings.Contains(stdout, "## Decisions") || strings.Contains(stdout, "## Problem") {
		t.Errorf("plan kind: exit %d, stdout:\n%s", code, stdout)
	}
}

func TestSpecDoc_ProposedAndAt(t *testing.T) {
	repo := buildSpecDocRepo(t)
	bin := buildVerdiBinary(t)
	// A design branch with a COMMITTED edit, then a further UNCOMMITTED
	// edit on top (fix round 1, F2). Without the second, uncommitted
	// layer, an implementation that silently read HEAD instead of the
	// actual working tree for --proposed would still see "widely" and
	// pass a weaker version of this test; only the uncommitted marker
	// text can catch that, since it exists nowhere in git history.
	branch := "design/lockbox-edit"
	run := func(args ...string) (string, string, int) {
		return runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, args...)
	}
	specPath := filepath.Join(repo.Dir, ".verdi/specs/active/lockbox/spec.md")
	runGitCmd(t, repo.Dir, "checkout", "-q", "-b", branch)
	edited := strings.Replace(specDocFixture, "Keys are shared.", "Keys are shared widely.", 1)
	if err := os.WriteFile(specPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, repo.Dir, "commit", "-qam", "edit problem")
	head := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "HEAD"))

	uncommitted := strings.Replace(edited, "Keys are shared widely.", "Keys are shared UNCOMMITTED.", 1)
	if err := os.WriteFile(specPath, []byte(uncommitted), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := run("spec", "doc", "spec/lockbox", "--proposed")
	if code != 0 {
		t.Fatalf("--proposed exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Proposed, not accepted") || !strings.Contains(stdout, "Keys are shared UNCOMMITTED.") || !strings.Contains(stdout, "commit `"+head+"`") {
		t.Errorf("--proposed render must read the dirty working tree, not HEAD:\n%s", stdout)
	}

	stdout, stderr, code = run("spec", "doc", "spec/lockbox")
	if code != 0 {
		t.Fatalf("default exit %d: %s", code, stderr)
	}
	if strings.Contains(stdout, "widely") || strings.Contains(stdout, "UNCOMMITTED") || !strings.Contains(stdout, "commit `"+repo.Head+"`") {
		t.Errorf("default render must read main's bytes, not the branch's:\n%s", stdout)
	}

	stdout, stderr, code = run("spec", "doc", "spec/lockbox", "--at", head)
	if code != 0 || !strings.Contains(stdout, "widely") || strings.Contains(stdout, "UNCOMMITTED") || !strings.Contains(stdout, "commit `"+head+"`") {
		t.Errorf("--at render must read the committed blob, not the dirty working tree (exit %d, %s):\n%s", code, stderr, stdout)
	}
}

// TestSpecDoc_EvidenceSourceNamesHEADNotAt is the F1 CLI proof (spec-
// documents wave-1 final fix round): matrixprojection.Project always
// evaluates the working tree's current HEAD, never the (possibly older)
// commit the rendered spec text itself came from under --at, so the
// Evidence section's Source line must always name HEAD's own prefix, not
// --at's.
func TestSpecDoc_EvidenceSourceNamesHEADNotAt(t *testing.T) {
	revised := strings.Replace(specDocFixture, "Keys are shared.", "Keys are shared, revised.", 1)
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Message: "adopt store with one accepted spec",
			Files: map[string]string{
				".verdi/verdi.yaml":                   supersedeManifestYAML,
				".verdi/specs/active/lockbox/spec.md": specDocFixture,
			},
		},
		{
			Message: "revise lockbox problem statement",
			Files: map[string]string{
				".verdi/specs/active/lockbox/spec.md": revised,
			},
		},
	})
	old := repo.Heads[0]
	if old == repo.Head {
		t.Fatal("fixture must have two distinct commits for this proof")
	}
	bin := buildVerdiBinary(t)
	stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, "spec", "doc", "spec/lockbox", "--at", old)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	wantSource := "Source: matrix over the working tree at " + repo.Head[:12]
	if !strings.Contains(stdout, wantSource) {
		t.Errorf("Source line must name HEAD's prefix %q, got:\n%s", wantSource, stdout)
	}
	dontWantSource := "Source: matrix over the working tree at " + old[:12]
	if strings.Contains(stdout, dontWantSource) {
		t.Errorf("Source line must not name --at's commit %q, got:\n%s", dontWantSource, stdout)
	}
}

func TestSpecDoc_Refusals(t *testing.T) {
	repo := buildSpecDocRepo(t)
	bin := buildVerdiBinary(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing ref", []string{"spec", "doc"}, "usage: verdi spec doc"},
		{"not a spec ref", []string{"spec", "doc", "adr/x"}, "is not a valid spec/<name> ref"},
		{"fragment ref", []string{"spec", "doc", "spec/lockbox#ac-1"}, "is not a valid spec/<name> ref"},
		{"unknown spec", []string{"spec", "doc", "spec/nope"}, "spec/nope"},
		{"bad kind", []string{"spec", "doc", "spec/lockbox", "--kind", "chapter"}, "unknown document kind"},
		{"bad format", []string{"spec", "doc", "spec/lockbox", "--format", "pdf"}, "--format must be md or html"},
		{"bad commit", []string{"spec", "doc", "spec/lockbox", "--at", "deadbeef"}, "deadbeef"},
		{"at and proposed", []string{"spec", "doc", "spec/lockbox", "--at", repo.Head, "--proposed"}, "--at and --proposed cannot be combined"},
		{"extra positional", []string{"spec", "doc", "spec/lockbox", "spec/bogus"}, "usage: verdi spec doc"},
		{"o inside store", []string{"spec", "doc", "spec/lockbox", "-o", filepath.Join(repo.Dir, ".verdi", "evil.md")}, "-o must not point inside the store"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stdout, stderr, code := runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, c.args...)
			if code != 2 {
				t.Fatalf("exit %d, want 2; stdout %q stderr %q", code, stdout, stderr)
			}
			if !strings.Contains(stderr, c.want) {
				t.Errorf("stderr %q does not name %q", stderr, c.want)
			}
		})
	}
}

// TestOutPathInStore is the F3 unit-level table proof for outPathInStore
// itself: happy path (outside the store, allowed), negative path (inside
// the store, refused, at both a direct child and a not-yet-existing
// nested path), the store directory itself, and — the specific case
// filepath.Rel-based containment exists to get right where a bare
// strings.HasPrefix(out, storeDir) would not — a sibling directory
// (".verdi-other") that shares the store directory's name as a string
// prefix without being inside it.
func TestOutPathInStore(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".verdi-other"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	tests := []struct {
		name string
		out  string
		want bool
	}{
		{"direct child of the store", filepath.Join(root, ".verdi", "evil.md"), true},
		{"a not-yet-existing nested path under the store", filepath.Join(root, ".verdi", "specs", "active", "evil.md"), true},
		{"the store directory itself", filepath.Join(root, ".verdi"), true},
		{"a sibling directory sharing the store's name as a string prefix", filepath.Join(root, ".verdi-other", "fine.md"), false},
		{"outside the root entirely", filepath.Join(outside, "fine.md"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := outPathInStore(root, tt.out)
			if err != nil {
				t.Fatalf("outPathInStore(%q) error: %v", tt.out, err)
			}
			if got != tt.want {
				t.Errorf("outPathInStore(%q) = %v, want %v", tt.out, got, tt.want)
			}
		})
	}
}

// TestSpecDoc_FlagsAfterRef is fix round 1, F1's regression: a flag
// placed AFTER the positional <spec-ref> must still take effect, not be
// silently dropped. Uses a two-commit fixture (rather than
// buildSpecDocRepo's single layer) so --at has a genuinely OLD commit,
// distinct from HEAD, to prove it actually read.
func TestSpecDoc_FlagsAfterRef(t *testing.T) {
	revised := strings.Replace(specDocFixture, "Keys are shared.", "Keys are shared, revised.", 1)
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Message: "adopt store with one accepted spec",
			Files: map[string]string{
				".verdi/verdi.yaml":                   supersedeManifestYAML,
				".verdi/specs/active/lockbox/spec.md": specDocFixture,
			},
		},
		{
			Message: "revise lockbox problem statement",
			Files: map[string]string{
				".verdi/specs/active/lockbox/spec.md": revised,
			},
		},
	})
	old := repo.Heads[0]
	bin := buildVerdiBinary(t)
	run := func(args ...string) (string, string, int) {
		return runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, args...)
	}

	t.Run("--at after ref", func(t *testing.T) {
		stdout, stderr, code := run("spec", "doc", "--format", "md", "spec/lockbox", "--at", old)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		if !strings.Contains(stdout, "Keys are shared.") || strings.Contains(stdout, "revised") || !strings.Contains(stdout, "commit `"+old+"`") {
			t.Errorf("--at after the ref must still render the OLD bytes/commit, not HEAD's:\n%s", stdout)
		}
	})

	t.Run("--proposed after ref", func(t *testing.T) {
		stdout, stderr, code := run("spec", "doc", "--format", "md", "spec/lockbox", "--proposed")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		if !strings.Contains(stdout, "Proposed, not accepted") {
			t.Errorf("--proposed after the ref must still be applied:\n%s", stdout)
		}
	})

	t.Run("-o after ref", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "plan.md")
		stdout, stderr, code := run("spec", "doc", "--kind", "plan", "spec/lockbox", "-o", out)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		if stdout != "" {
			t.Errorf("-o after the ref must still suppress stdout, got %q", stdout)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("-o after the ref must still write the file: %v", err)
		}
		if !strings.Contains(string(data), "## Plan") {
			t.Errorf("-o file wrong shape:\n%s", data)
		}
	})
}

func TestSpecDoc_NoStoreExitsOperational(t *testing.T) {
	bin := buildVerdiBinary(t)
	_, stderr, code := runVerdiBinary(t, bin, t.TempDir(), nil, "spec", "doc", "spec/lockbox")
	if code != 2 || stderr == "" {
		t.Fatalf("exit %d stderr %q, want 2 with a message", code, stderr)
	}
}
