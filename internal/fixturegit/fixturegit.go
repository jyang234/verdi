// Package fixturegit builds deterministic git repositories for tests, from
// a declarative list of (files, commit-message) layers. It is a test
// helper, not a production package (PLAN.md §4 "fixturegit (a Go test
// helper, not data)"), de-risking spike S2: frozen stamps, pinned refs,
// ancestry checks, and cross-commit diffs elsewhere in the plan all need
// real git history with commit SHAs that are byte-stable across machines
// and runs.
package fixturegit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Layer is one commit's worth of file writes.
type Layer struct {
	// Files maps repo-relative slash paths to full file content. Only the
	// named files are written; nothing is deleted between layers (layers
	// are additive, matching how the fixture corpus is authored in stages).
	Files map[string]string
	// Message is the commit message for this layer. Required non-empty so
	// every commit has a legible message (mirrors D1's legibility goal).
	Message string
}

// Repo is a built, deterministic git repository ready for a test to read.
type Repo struct {
	// Dir is the repository's working directory (a fresh t.TempDir()).
	Dir string
	// Head is the HEAD commit SHA after every layer was committed.
	Head string
	// Heads holds each layer's own commit SHA, in the order layers were
	// built (Heads[len(Heads)-1] == Head). Callers that pin frontmatter
	// refs or frozen stamps at a specific, earlier layer's commit (rather
	// than always the final head) need these (PLAN.md §4: "Pins inside
	// corpus files must be the literal deterministic SHAs").
	Heads []string
}

// Fixed author/committer identity and commit timestamp so that a repo built
// from the same layers always produces byte-identical commit SHAs,
// regardless of the host machine's clock, timezone, or git config
// (PLAN.md risk R2 / spike S2).
const (
	identityName  = "Verdi Fixture"
	identityEmail = "fixture@verdi.invalid"
	// 2024-01-01T00:00:00Z as git's "<unix-seconds> <tz-offset>" date form.
	fixedDate = "1704067200 +0000"
)

// Build creates a fresh temp-dir git repository and commits each layer in
// order, in a fixed timezone under a fixed author/committer identity and
// date. It fails the calling test (via t.Fatal) on any git or filesystem
// error, or if a layer is empty.
func Build(t testing.TB, layers []Layer) *Repo {
	t.Helper()

	if len(layers) == 0 {
		t.Fatal("fixturegit: Build called with no layers")
	}

	dir := t.TempDir()
	runGit(t, dir, nil, "init", "--quiet", "--initial-branch=main")
	runGit(t, dir, nil, "config", "user.name", identityName)
	runGit(t, dir, nil, "config", "user.email", identityEmail)
	// Deterministic fixtures never need signing, and a machine with commit
	// signing configured globally would otherwise hang or fail here.
	runGit(t, dir, nil, "config", "commit.gpgsign", "false")

	commitEnv := commitEnvironment()
	heads := make([]string, 0, len(layers))

	for i, layer := range layers {
		if len(layer.Files) == 0 {
			t.Fatalf("fixturegit: layer %d has no files", i)
		}
		if strings.TrimSpace(layer.Message) == "" {
			t.Fatalf("fixturegit: layer %d has an empty commit message", i)
		}

		for path, content := range layer.Files {
			writeFile(t, dir, path, content)
		}

		runGit(t, dir, nil, "add", "-A")
		runGit(t, dir, commitEnv, "commit", "--quiet", "--no-verify", "-m", layer.Message)

		layerHead := strings.TrimSpace(runGitOutput(t, dir, nil, "rev-parse", "HEAD"))
		heads = append(heads, layerHead)
	}

	head := heads[len(heads)-1]
	return &Repo{Dir: dir, Head: head, Heads: heads}
}

// commitEnvironment returns the fixed environment overrides applied to
// every `git commit` invocation: identical author/committer name, email,
// and date, plus a fixed TZ so no local timezone leaks into the commit
// object's date field.
func commitEnvironment() []string {
	return []string{
		"TZ=UTC",
		"GIT_AUTHOR_NAME=" + identityName,
		"GIT_AUTHOR_EMAIL=" + identityEmail,
		"GIT_AUTHOR_DATE=" + fixedDate,
		"GIT_COMMITTER_NAME=" + identityName,
		"GIT_COMMITTER_EMAIL=" + identityEmail,
		"GIT_COMMITTER_DATE=" + fixedDate,
	}
}

// writeFile writes content to dir/path, creating parent directories as
// needed. path uses forward slashes regardless of host OS.
func writeFile(t testing.TB, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("fixturegit: mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("fixturegit: write %s: %v", path, err)
	}
}

// runGit runs git in dir, overriding env with extraEnv (base env vars with
// the same key are shadowed, never left ambiguous to exec.Cmd's last-wins
// behavior). It fails the test on a non-zero exit.
func runGit(t testing.TB, dir string, extraEnv []string, args ...string) {
	t.Helper()
	runGitOutput(t, dir, extraEnv, args...)
}

// runGitOutput is runGit plus the combined stdout+stderr, for callers that
// need the output (e.g. `rev-parse HEAD`).
func runGitOutput(t testing.TB, dir string, extraEnv []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = mergeEnv(os.Environ(), extraEnv)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixturegit: git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// mergeEnv returns base with every key present in overrides removed first,
// so the override is unambiguous rather than relying on exec.Cmd's
// last-duplicate-wins behavior.
func mergeEnv(base, overrides []string) []string {
	overrideKeys := make(map[string]bool, len(overrides))
	for _, kv := range overrides {
		overrideKeys[envKey(kv)] = true
	}

	merged := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		if !overrideKeys[envKey(kv)] {
			merged = append(merged, kv)
		}
	}
	return append(merged, overrides...)
}

// envKey returns the "NAME" half of a "NAME=value" environment entry.
func envKey(kv string) string {
	if i := strings.IndexByte(kv, '='); i >= 0 {
		return kv[:i]
	}
	return kv
}

// String implements fmt.Stringer for readable test failure output.
func (r *Repo) String() string {
	return fmt.Sprintf("fixturegit.Repo{Dir: %s, Head: %s}", r.Dir, r.Head)
}
