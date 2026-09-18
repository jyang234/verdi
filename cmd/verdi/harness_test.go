// Real, built-binary end-to-end tests for `verdi harness render|check`
// (spec/spec-documents ac-7): mirrors this package's own
// serve_integration_test.go/gc_test.go convention — buildVerdiBinary +
// runVerdi drive the actual compiled binary, never a package-internal
// stand-in, so the proof covers cmd/verdi's real wiring (flag parsing,
// dispatch, exit codes) over internal/skillpack's Write/Check.
// TestHarnessRenderAndCheck exercises usage-error grammar, render/check
// over both hosts and a single host, and the render-repairs-drift cycle,
// all against a bare t.TempDir() via -o. TestHarnessDefaultsToStoreRoot
// proves the no -o path: the store root is found by ancestor search
// (newIntegrationStoreRoot, a real git checkout via fixturegit) and the
// render commit is stamped from that repository's real HEAD, not "none".
//
// Deliberately NOT examples/showcase: harness's own behavior (R-W3-7) is
// independent of any store or corpus content — it renders the four
// skillpack templates embedded in this binary to an explicit -o
// directory or, absent one, the ambient store root alone, and never
// reads a spec, a model, or any other store byte. A scratch tempdir (or
// a minimal fixturegit checkout carrying nothing but .verdi/verdi.yaml)
// is therefore the genuine, most direct evidence for this verb, not a
// workaround — the same disclosed-scratch-fixture posture
// rollup_test.go's rollupFixtureSpec and mcpserve/fixture_test.go's
// buildFixture already use for their own verbs.
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestHarnessRenderAndCheck(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := t.TempDir()

	// Usage errors exit 2 before any root is resolved.
	for _, args := range [][]string{{"harness"}, {"harness", "frobnicate"}, {"harness", "render", "--host", "cursor", "-o", root}, {"harness", "render", "-o"}, {"harness", "check", "extra", "-o", root}} {
		code, _, stderr := runVerdi(t, bin, root, args...)
		if code != 2 || !strings.Contains(stderr, "usage: verdi harness") {
			t.Fatalf("%v: code %d stderr %q", args, code, stderr)
		}
	}

	// -o must exist.
	if code, _, stderr := runVerdi(t, bin, root, "harness", "render", "-o", filepath.Join(root, "nope")); code != 2 || !strings.Contains(stderr, "harness render:") {
		t.Fatalf("missing -o: code %d stderr %q", code, stderr)
	}

	// check before render: every skill missing, exit 1, one line per finding on stdout.
	code, stdout, _ := runVerdi(t, bin, root, "harness", "check", "-o", root)
	if code != 1 || strings.Count(stdout, "missing  ") != 8 {
		t.Fatalf("check before render: code %d stdout %q", code, stdout)
	}

	// render all: 8 sorted "<digest>  <path>" lines on stdout, exit 0.
	code, stdout, stderr := runVerdi(t, bin, root, "harness", "render", "-o", root)
	if code != 0 {
		t.Fatalf("render: code %d stderr %q", code, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 8 {
		t.Fatalf("render printed %d lines: %q", len(lines), stdout)
	}
	paths := make([]string, 0, len(lines))
	for i, l := range lines {
		digest, path, ok := strings.Cut(l, "  ")
		if !ok || !strings.HasPrefix(digest, "sha256:") || len(digest) != len("sha256:")+64 || path == "" {
			t.Fatalf("line %d %q is not '<digest>  <path>'", i, l)
		}
		paths = append(paths, path)
	}
	if !sort.StringsAreSorted(paths) {
		t.Fatalf("render output not sorted by path: %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "verdi-clarify", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	// check clean: exit 0, empty stdout.
	if code, stdout, _ := runVerdi(t, bin, root, "harness", "check", "-o", root); code != 0 || stdout != "" {
		t.Fatalf("check clean: code %d stdout %q", code, stdout)
	}

	// --host codex only renders four; a later claude check still passes (untouched).
	code, stdout, _ = runVerdi(t, bin, root, "harness", "render", "--host", "codex", "-o", root)
	if code != 0 || strings.Count(stdout, "\n") != 4 {
		t.Fatalf("render codex: code %d stdout %q", code, stdout)
	}

	// drift: edit one file → exit 1, "drift  <path>".
	p := filepath.Join(root, ".claude", "skills", "verdi-plan", "SKILL.md")
	b, _ := os.ReadFile(p)
	if err := os.WriteFile(p, append(b, []byte("edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = runVerdi(t, bin, root, "harness", "check", "-o", root)
	if code != 1 || strings.TrimSpace(stdout) != "drift  .claude/skills/verdi-plan/SKILL.md" {
		t.Fatalf("check drift: code %d stdout %q", code, stdout)
	}
	// scoped to codex the drift is invisible.
	if code, stdout, _ := runVerdi(t, bin, root, "harness", "check", "--host", "codex", "-o", root); code != 0 || stdout != "" {
		t.Fatalf("check codex after claude drift: code %d stdout %q", code, stdout)
	}
	// a codex-host file can drift too, and a --host codex check must catch
	// it — proves Check's drift comparison isn't scoped to one host.
	p2 := filepath.Join(root, ".agents", "skills", "verdi-plan", "SKILL.md")
	b2, _ := os.ReadFile(p2)
	if err := os.WriteFile(p2, append(b2, []byte("edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = runVerdi(t, bin, root, "harness", "check", "--host", "codex", "-o", root)
	if code != 1 || strings.TrimSpace(stdout) != "drift  .agents/skills/verdi-plan/SKILL.md" {
		t.Fatalf("check codex drift: code %d stdout %q", code, stdout)
	}
	// re-render repairs it.
	runVerdi(t, bin, root, "harness", "render", "-o", root)
	if code, _, _ := runVerdi(t, bin, root, "harness", "check", "-o", root); code != 0 {
		t.Fatalf("check after re-render: code %d", code)
	}
}

func TestHarnessDefaultsToStoreRoot(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := newIntegrationStoreRoot(t)
	sub := filepath.Join(root, "cmd")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// From a subdirectory, no -o: the store root is found by ancestor search.
	if code, _, stderr := runVerdi(t, bin, sub, "harness", "render"); code != 0 {
		t.Fatalf("render from subdir: code %d stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "verdi-tasks", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	// Inside a git repository the render commit is HEAD, not none.
	b, _ := os.ReadFile(filepath.Join(root, ".claude", "skills", "verdi-tasks", "SKILL.md"))
	if strings.Contains(string(b), "verdi:render-commit none") {
		t.Fatal("render inside a git repo must stamp HEAD")
	}
	if code, _, _ := runVerdi(t, bin, sub, "harness", "check"); code != 0 {
		t.Fatalf("check from subdir: code %d", code)
	}
	// Outside any store and without -o: operational error, exit 2.
	if code, _, stderr := runVerdi(t, bin, t.TempDir(), "harness", "check"); code != 2 || !strings.Contains(stderr, "harness check:") {
		t.Fatalf("no store: code %d stderr %q", code, stderr)
	}
}
