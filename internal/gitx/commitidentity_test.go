package gitx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// clearGitIdentEnv removes the identity variables git reads from the
// environment, so a case that means "this checkout declares no identity"
// is never quietly satisfied by the developer's own shell — or reddened
// by a gate command that exports them empty to reproduce a CI host.
// t.Setenv cannot express absence: setting GIT_COMMITTER_NAME to "" is
// the OPPOSITE of unset, a name git refuses outright. So the variables
// are removed directly and restored by Cleanup, which is safe because
// nothing in this package runs in parallel.
func clearGitIdentEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "EMAIL"} {
		old, had := os.LookupEnv(key)
		if !had {
			continue
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unsetting %s (test setup): %v", key, err)
		}
		t.Cleanup(func() {
			if err := os.Setenv(key, old); err != nil {
				t.Errorf("restoring %s: %v", key, err)
			}
		})
	}
}

// setIdentity configures both halves of a repository-scoped git identity.
// ConfigValue's own setConfig sets user.email alone, which is not enough
// here: git needs a NAME as well before it will mint an ident.
func setIdentity(name, email string) func(*testing.T, string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		for _, kv := range [][2]string{{"user.name", name}, {"user.email", email}} {
			if err := exec.Command("git", "-C", dir, "config", kv[0], kv[1]).Run(); err != nil {
				t.Fatalf("git config %s %q (test setup): %v", kv[0], kv[1], err)
			}
		}
	}
}

// corruptConfig makes the repository's own config file unparseable (an
// unterminated section header), mirroring
// TestConfigValue_Negative_BrokenRepository's setup.
func corruptConfig(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("[user\nemail = a@b.com\n"), 0o644); err != nil {
		t.Fatalf("corrupting .git/config (test setup): %v", err)
	}
}

// TestCommitIdentityAvailable is the availability table: every way a
// checkout can — or cannot — give git an identity to record a commit
// with, once git itself has run.
//
// Deliberately absent: a "no identity configured anywhere" case with the
// environment merely cleared. Git then derives a name from the OS
// account's full-name field, which a developer's machine fills in and a
// CI runner's does not, so its answer is a property of the HOST rather
// than of this function. The negative cases below pin the state
// explicitly (GIT_COMMITTER_NAME exported empty), which every host
// answers the same way.
func TestCommitIdentityAvailable(t *testing.T) {
	tests := []struct {
		name string
		// setup configures the repository before the probe; nil leaves
		// the fresh repository's own identity unset.
		setup func(*testing.T, string)
		// env is applied after the identity environment is cleared.
		env  map[string]string
		want bool
	}{
		{
			// The ordinary developer checkout, and what fixturegit
			// provisions: git mints its ident from the repository's own
			// user.name/user.email with nothing in the environment.
			name:  "repository identity alone",
			setup: setIdentity("Repo Author", "author@example.invalid"),
			want:  true,
		},
		{
			// The state `verdi policy adopt` must accept on a CI runner
			// that configures no identity in any scope: the environment
			// alone carries one.
			name: "environment identity alone",
			env:  map[string]string{"GIT_COMMITTER_NAME": "Verdi Fixture", "GIT_COMMITTER_EMAIL": "fixture@verdi.invalid"},
			want: true,
		},
		{
			// An exported-empty GIT_COMMITTER_NAME overrides every config
			// scope, so even a repository that configures a perfectly good
			// identity cannot commit. Reporting it available would be the
			// favorable-default answer that the caller then discovers is
			// wrong only after writing.
			name:  "empty committer name overrides the repository identity",
			setup: setIdentity("Repo Author", "author@example.invalid"),
			env:   map[string]string{"GIT_COMMITTER_NAME": ""},
			want:  false,
		},
		{
			// The CI shape the policy adopt preflight exists for: no
			// identity in any config scope and an empty name in the
			// environment.
			name: "empty committer name with no repository identity",
			env:  map[string]string{"GIT_COMMITTER_NAME": ""},
			want: false,
		},
		{
			// A config file git cannot parse is folded into "unavailable"
			// rather than reported operational, and deliberately so: git
			// exits 128 for this exactly as it does for an unusable ident,
			// and the two are distinguishable only by matching git's own
			// localized prose — which ConfigValue's ADJ-64 rule forbids.
			// The fold is the fail-closed direction and the true answer to
			// the question asked: git cannot mint an identity from a
			// configuration it cannot read, so a commit here would fail.
			name:  "unparseable repository configuration",
			setup: corruptConfig,
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateGitConfig(t)
			clearGitIdentEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			dir := initRepo(t)
			if tt.setup != nil {
				tt.setup(t, dir)
			}

			got, err := CommitIdentityAvailable(context.Background(), dir)
			if err != nil {
				t.Fatalf("CommitIdentityAvailable: %v", err)
			}
			if got != tt.want {
				t.Fatalf("CommitIdentityAvailable = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCommitIdentityAvailable_MatchesWhetherGitCanCommit pins the
// function to the only claim that makes it worth calling: its answer
// agrees with what `git commit` itself then does. A probe that says
// "available" where a commit dies is worse than no probe at all — the
// caller writes on the strength of it.
func TestCommitIdentityAvailable_MatchesWhetherGitCanCommit(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
	}{
		{name: "identity available", env: map[string]string{"GIT_COMMITTER_NAME": "Verdi Fixture", "GIT_COMMITTER_EMAIL": "fixture@verdi.invalid", "GIT_AUTHOR_NAME": "Verdi Fixture", "GIT_AUTHOR_EMAIL": "fixture@verdi.invalid"}},
		{name: "identity unavailable", env: map[string]string{"GIT_COMMITTER_NAME": "", "GIT_AUTHOR_NAME": ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateGitConfig(t)
			clearGitIdentEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			dir := initRepo(t)

			available, err := CommitIdentityAvailable(context.Background(), dir)
			if err != nil {
				t.Fatalf("CommitIdentityAvailable: %v", err)
			}

			if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := exec.Command("git", "-C", dir, "add", "a.txt").Run(); err != nil {
				t.Fatalf("git add (test setup): %v", err)
			}
			committed := exec.Command("git", "-C", dir, "commit", "-m", "probe").Run() == nil

			if available != committed {
				t.Fatalf("CommitIdentityAvailable = %v but `git commit` succeeded = %v", available, committed)
			}
		})
	}
}

// TestCommitIdentityAvailable_Negative_Operational covers the split the
// availability answer depends on: a git that RAN and refused reports an
// unavailable identity, while a git that never ran at all reports an
// operational error. The second must never be reported as "no identity
// configured" — that would tell an operator to configure user.name when
// the real fault is that git could not be executed.
func TestCommitIdentityAvailable_Negative_Operational(t *testing.T) {
	isolateGitConfig(t)

	t.Run("directory does not exist", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "no-such-directory")
		available, err := CommitIdentityAvailable(context.Background(), missing)
		if err == nil {
			t.Fatalf("CommitIdentityAvailable(%s) = (%v, nil), want an operational error", missing, available)
		}
		if available {
			t.Fatal("CommitIdentityAvailable reported an available identity alongside an error")
		}
	})

	t.Run("context already cancelled", func(t *testing.T) {
		dir := initRepo(t)
		setIdentity("Repo Author", "author@example.invalid")(t, dir)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		available, err := CommitIdentityAvailable(ctx, dir)
		if err == nil {
			t.Fatalf("CommitIdentityAvailable(cancelled ctx) = (%v, nil), want an operational error", available)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("CommitIdentityAvailable(cancelled ctx) err = %v, want errors.Is(err, context.Canceled)", err)
		}
		if available {
			t.Fatal("CommitIdentityAvailable reported an available identity alongside an error")
		}
	})
}
