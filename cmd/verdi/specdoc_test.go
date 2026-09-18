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
	// A design branch with an edited spec.
	branch := "design/lockbox-edit"
	run := func(args ...string) (string, string, int) {
		return runVerdiBinary(t, bin, repo.Dir, []string{"CI_DEFAULT_BRANCH=main"}, args...)
	}
	runGitCmd(t, repo.Dir, "checkout", "-q", "-b", branch)
	edited := strings.Replace(specDocFixture, "Keys are shared.", "Keys are shared widely.", 1)
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi/specs/active/lockbox/spec.md"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, repo.Dir, "commit", "-qam", "edit problem")
	head := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "HEAD"))

	stdout, stderr, code := run("spec", "doc", "spec/lockbox", "--proposed")
	if code != 0 {
		t.Fatalf("--proposed exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Proposed, not accepted") || !strings.Contains(stdout, "Keys are shared widely.") || !strings.Contains(stdout, "commit `"+head+"`") {
		t.Errorf("--proposed render wrong:\n%s", stdout)
	}

	stdout, stderr, code = run("spec", "doc", "spec/lockbox")
	if code != 0 {
		t.Fatalf("default exit %d: %s", code, stderr)
	}
	if strings.Contains(stdout, "widely") || !strings.Contains(stdout, "commit `"+repo.Head+"`") {
		t.Errorf("default render must read main's bytes, not the branch's:\n%s", stdout)
	}

	stdout, stderr, code = run("spec", "doc", "spec/lockbox", "--at", head)
	if code != 0 || !strings.Contains(stdout, "widely") || !strings.Contains(stdout, "commit `"+head+"`") {
		t.Errorf("--at render wrong (exit %d, %s):\n%s", code, stderr, stdout)
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

func TestSpecDoc_NoStoreExitsOperational(t *testing.T) {
	bin := buildVerdiBinary(t)
	_, stderr, code := runVerdiBinary(t, bin, t.TempDir(), nil, "spec", "doc", "spec/lockbox")
	if code != 2 || stderr == "" {
		t.Fatalf("exit %d stderr %q, want 2 with a message", code, stderr)
	}
}
