package gitx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// isolateGitConfig prevents the host machine's own global/system git
// config from leaking into a test that depends on a key being absent. A
// real developer machine legitimately has user.email/user.name configured
// globally, which would otherwise make TestConfigValue_Negative_Absent
// flaky and host-dependent (mirrors the isolation already used by
// cmd/verdi's worktree-contract tests and internal/workbench's handler
// tests).
func isolateGitConfig(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// initRepo creates a fresh, empty git repository with no committed
// history and no identity configured — deliberately NOT fixturegit.Build,
// which always sets user.name/user.email locally (fixed fixture
// identity), the opposite of what the "absent" test case needs.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "--quiet").Run(); err != nil {
		t.Fatalf("git init (test setup): %v", err)
	}
	return dir
}

func TestConfigValue_Present(t *testing.T) {
	isolateGitConfig(t)
	dir := initRepo(t)
	ctx := context.Background()

	const want = "alice@example.com"
	if err := exec.Command("git", "-C", dir, "config", "user.email", want).Run(); err != nil {
		t.Fatalf("git config (test setup): %v", err)
	}

	got, err := ConfigValue(ctx, dir, "user.email")
	if err != nil {
		t.Fatalf("ConfigValue: %v", err)
	}
	if got != want {
		t.Fatalf("ConfigValue = %q, want %q", got, want)
	}
}

func TestConfigValue_Negative_Absent(t *testing.T) {
	isolateGitConfig(t)
	dir := initRepo(t)
	ctx := context.Background()

	_, err := ConfigValue(ctx, dir, "user.email")
	if err == nil {
		t.Fatal("ConfigValue(unset key): want error, got nil")
	}
	if !errors.Is(err, ErrConfigUnset) {
		t.Errorf("ConfigValue(unset key) err = %v, want errors.Is(err, ErrConfigUnset)", err)
	}
}

// TestConfigValue_Negative_Absent_LocaleIndependent proves the absent
// determination never rests on matching git's own (locale-dependent)
// stderr text: under a localizing locale, an absent key still reports
// ErrConfigUnset.
func TestConfigValue_Negative_Absent_LocaleIndependent(t *testing.T) {
	isolateGitConfig(t)
	t.Setenv("LC_ALL", "fr_FR.UTF-8")
	dir := initRepo(t)
	ctx := context.Background()

	_, err := ConfigValue(ctx, dir, "user.email")
	if err == nil {
		t.Fatal("ConfigValue(unset key, localized locale): want error, got nil")
	}
	if !errors.Is(err, ErrConfigUnset) {
		t.Errorf("ConfigValue(unset key, localized locale) err = %v, want errors.Is(err, ErrConfigUnset)", err)
	}
}

func TestConfigValue_Negative_BrokenRepository(t *testing.T) {
	isolateGitConfig(t)
	dir := initRepo(t)
	ctx := context.Background()

	// Corrupt the repository's own config file (an unterminated section
	// header) so `git config --get` fails to parse the file at all — a
	// broken repository, categorically different from a merely-unset key.
	configPath := filepath.Join(dir, ".git", "config")
	if err := os.WriteFile(configPath, []byte("[user\nemail = a@b.com\n"), 0o644); err != nil {
		t.Fatalf("corrupting .git/config (test setup): %v", err)
	}

	_, err := ConfigValue(ctx, dir, "user.email")
	if err == nil {
		t.Fatal("ConfigValue(broken config file): want error, got nil")
	}
	if errors.Is(err, ErrConfigUnset) {
		t.Errorf("ConfigValue(broken config file) err = %v, want NOT ErrConfigUnset (a genuine operational failure)", err)
	}
}

// TestConfigValue_Negative_MalformedKey proves a syntactically invalid
// key (git itself refuses it — no section separator at all) is never
// misreported as the benign "absent" case, even though git also exits 1
// for it: the two are told apart by git's own stderr being present
// (malformed key) or empty (genuinely unset), never by matching its text.
func TestConfigValue_Negative_MalformedKey(t *testing.T) {
	isolateGitConfig(t)
	dir := initRepo(t)
	ctx := context.Background()

	_, err := ConfigValue(ctx, dir, "justkey")
	if err == nil {
		t.Fatal("ConfigValue(malformed key): want error, got nil")
	}
	if errors.Is(err, ErrConfigUnset) {
		t.Errorf("ConfigValue(malformed key) err = %v, want NOT ErrConfigUnset (git rejected the key itself)", err)
	}
}
