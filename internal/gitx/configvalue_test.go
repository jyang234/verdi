package gitx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// setConfig sets user.email to value with one `git config` invocation.
func setConfig(value string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		if err := exec.Command("git", "-C", dir, "config", "user.email", value).Run(); err != nil {
			t.Fatalf("git config user.email %q (test setup): %v", value, err)
		}
	}
}

// addConfig appends several values to the multi-valued user.email key.
func addConfig(values ...string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		for _, v := range values {
			if err := exec.Command("git", "-C", dir, "config", "--add", "user.email", v).Run(); err != nil {
				t.Fatalf("git config --add user.email %q (test setup): %v", v, err)
			}
		}
	}
}

// TestConfigValue_ValueShapes is the value-shape table: what ConfigValue
// returns for every way a key can carry — or fail to carry — one usable
// value once git itself has run successfully. The absent, malformed-key
// and broken-configuration shapes (git exits nonzero) have their own
// tests below.
func TestConfigValue_ValueShapes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string)
		// want is the exact value expected when no error is expected.
		want string
		// wantUnset requires errors.Is(err, ErrConfigUnset).
		wantUnset bool
		// wantErrSubs requires an operational error — never
		// ErrConfigUnset — containing every listed substring.
		wantErrSubs []string
	}{
		{
			name:  "single value",
			setup: setConfig("alice@example.com"),
			want:  "alice@example.com",
		},
		{
			// The value is an authorization input: ConfigValue returns
			// git's own bytes, so a padded identity can never be silently
			// trimmed into a different identity than the one configured.
			name:  "value padded with spaces is returned byte-exact",
			setup: setConfig("  padded  "),
			want:  "  padded  ",
		},
		{
			// `git config --get` exits 0 and prints an empty line here, so
			// only inspecting the exit code would report a usable identity
			// of "". An empty value names nobody: it is the unset state.
			name:      "set but empty is unset",
			setup:     setConfig(""),
			wantUnset: true,
		},
		{
			// `git config --get` silently returns the LAST of several
			// values. For an authorization input that is a wrong answer
			// dressed as a right one, so ConfigValue refuses instead.
			name:        "multiple values are ambiguous",
			setup:       addConfig("a@example.com", "b@example.com"),
			wantErrSubs: []string{"user.email", "ambiguous"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateGitConfig(t)
			dir := initRepo(t)
			tt.setup(t, dir)

			got, err := ConfigValue(context.Background(), dir, "user.email")
			switch {
			case tt.wantUnset:
				if !errors.Is(err, ErrConfigUnset) {
					t.Fatalf("ConfigValue = (%q, %v), want errors.Is(err, ErrConfigUnset)", got, err)
				}
			case len(tt.wantErrSubs) > 0:
				if err == nil {
					t.Fatalf("ConfigValue = (%q, nil), want an operational error", got)
				}
				if errors.Is(err, ErrConfigUnset) {
					t.Errorf("ConfigValue err = %v, want NOT ErrConfigUnset: the key is set, just not unambiguously", err)
				}
				for _, sub := range tt.wantErrSubs {
					if !strings.Contains(err.Error(), sub) {
						t.Errorf("error %q does not contain %q", err.Error(), sub)
					}
				}
			default:
				if err != nil {
					t.Fatalf("ConfigValue: %v", err)
				}
				if got != tt.want {
					t.Fatalf("ConfigValue = %q, want %q", got, tt.want)
				}
			}
		})
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
