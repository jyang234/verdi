package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runExactRefGit runs one fixture-setup git command in dir. It is this
// file's own helper so the test depends on no other gitx test file.
func runExactRefGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestResolveExactRef_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	first, second := repo.Heads[0], repo.Heads[1]
	runExactRefGit(t, repo.Dir, "update-ref", "refs/remotes/origin/close/spec-x", first)
	runExactRefGit(t, repo.Dir, "update-ref", "refs/remotes/origin/main", second)
	runExactRefGit(t, repo.Dir, "pack-refs", "--all")
	runExactRefGit(t, repo.Dir, "update-ref", "refs/remotes/origin/loose", second)

	tests := []struct {
		name, ref, want string
	}{
		{name: "packed remote-tracking ref", ref: "refs/remotes/origin/close/spec-x", want: first},
		{name: "packed ref at another commit", ref: "refs/remotes/origin/main", want: second},
		{name: "loose remote-tracking ref", ref: "refs/remotes/origin/loose", want: second},
		{name: "local branch by its full name", ref: "refs/heads/main", want: repo.Head},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveExactRef(ctx, repo.Dir, tt.ref)
			if err != nil {
				t.Fatalf("ResolveExactRef(%q): %v", tt.ref, err)
			}
			if got != tt.want {
				t.Fatalf("ResolveExactRef(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

// TestResolveExactRef_Negative includes the E4a review's M-1 probe: with
// the exact refs/remotes/origin/<name> absent, a tag or a branch literally
// named refs/remotes/origin/<name> at HEAD makes `git rev-parse --verify`
// resolve through its lookup rules — ResolveExactRef must not.
func TestResolveExactRef_Negative(t *testing.T) {
	ctx := context.Background()

	t.Run("lookup-rule decoys", func(t *testing.T) {
		repo := buildRepo(t)
		runExactRefGit(t, repo.Dir, "tag", "refs/remotes/origin/tag-decoy", repo.Head)
		runExactRefGit(t, repo.Dir, "branch", "refs/remotes/origin/branch-decoy", repo.Head)
		for _, ref := range []string{"refs/remotes/origin/tag-decoy", "refs/remotes/origin/branch-decoy"} {
			// The decoy is live: rev-parse, with its lookup rules, resolves it.
			if got, err := RevParse(ctx, repo.Dir, ref); err != nil || got != repo.Head {
				t.Fatalf("test setup: RevParse(%q) = %q, %v; want the decoy at HEAD", ref, got, err)
			}
			if got, err := ResolveExactRef(ctx, repo.Dir, ref); err == nil {
				t.Fatalf("ResolveExactRef(%q) = %q, nil; want an error (the exact ref is absent)", ref, got)
			}
		}
	})

	t.Run("absent ref", func(t *testing.T) {
		repo := buildRepo(t)
		if got, err := ResolveExactRef(ctx, repo.Dir, "refs/remotes/origin/absent"); err == nil {
			t.Fatalf("ResolveExactRef(absent) = %q, nil; want an error", got)
		}
	})

	t.Run("not a full refname", func(t *testing.T) {
		repo := buildRepo(t)
		for _, ref := range []string{"", "HEAD", "main", "origin/main", "remotes/origin/main", "-h", "--all", repo.Head, "main~1"} {
			if got, err := ResolveExactRef(ctx, repo.Dir, ref); err == nil {
				t.Fatalf("ResolveExactRef(%q) = %q, nil; want an error", ref, got)
			}
		}
	})

	t.Run("revision syntax under refs/", func(t *testing.T) {
		repo := buildRepo(t)
		runExactRefGit(t, repo.Dir, "update-ref", "refs/remotes/origin/main", repo.Head)
		for _, ref := range []string{"refs/remotes/origin/main~1", "refs/remotes/origin/main^", "refs/remotes/origin/main^{commit}", "refs/remotes/origin/main@{0}"} {
			if got, err := ResolveExactRef(ctx, repo.Dir, ref); err == nil {
				t.Fatalf("ResolveExactRef(%q) = %q, nil; want an error (no revision expression is evaluated)", ref, got)
			}
		}
	})

	t.Run("not a repository", func(t *testing.T) {
		if got, err := ResolveExactRef(ctx, t.TempDir(), "refs/heads/main"); err == nil {
			t.Fatalf("ResolveExactRef outside a repository = %q, nil; want an error", got)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		repo := buildRepo(t)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if got, err := ResolveExactRef(cancelled, repo.Dir, "refs/heads/main"); err == nil {
			t.Fatalf("ResolveExactRef with a cancelled context = %q, nil; want an error", got)
		}
	})
}

func TestIsFullObjectID(t *testing.T) {
	sha1 := strings.Repeat("0123456789abcdef", 2) + "01234567" // 40 lowercase hex
	sha256 := strings.Repeat("0123456789abcdef", 4)            // 64 lowercase hex
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{name: "40 lowercase hex (SHA-1)", id: sha1, want: true},
		{name: "64 lowercase hex (SHA-256)", id: sha256, want: true},
		{name: "show-ref line after the caller trims its newline", id: strings.TrimSuffix(sha1+"\n", "\n"), want: true},
		{name: "empty", id: ""},
		{name: "39 characters", id: sha1[:39]},
		{name: "41 characters", id: sha1 + "a"},
		{name: "63 characters", id: sha256[:63]},
		{name: "65 characters", id: sha256 + "a"},
		{name: "upper-case hex", id: strings.ToUpper(sha1)},
		{name: "one upper-case digit", id: "A" + sha1[1:]},
		{name: "non-hex letter", id: "g" + sha1[1:]},
		{name: "non-hex punctuation", id: sha1[:39] + "-"},
		{name: "non-ASCII rune", id: sha1[:38] + "é"},
		{name: "untrimmed show-ref line", id: sha1 + "\n"},
		{name: "leading space", id: " " + sha1[1:]},
		{name: "trailing space", id: sha1[:39] + " "},
		{name: "surrounded by whitespace", id: " " + sha1 + " "},
		{name: "two lines", id: sha1[:20] + "\n" + sha1[:19]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFullObjectID(tt.id); got != tt.want {
				t.Fatalf("isFullObjectID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

// TestResolveExactRef_ChecksPrintedObjectID drives ResolveExactRef's guard
// over what git prints. Real git's show-ref --verify --hash prints exactly
// one object id, so a stand-in git on PATH supplies the output: the one
// trimmed newline is accepted, and anything else is refused, never returned
// as an object id.
func TestResolveExactRef_ChecksPrintedObjectID(t *testing.T) {
	ctx := context.Background()
	sha1 := strings.Repeat("0123456789abcdef", 2) + "01234567"
	tests := []struct {
		name, output, want string
		ok                 bool
	}{
		{name: "one id and its newline", output: sha1 + "\n", want: sha1, ok: true},
		{name: "one id without a newline", output: sha1, want: sha1, ok: true},
		{name: "empty output", output: ""},
		{name: "only a newline", output: "\n"},
		{name: "upper-case hex", output: strings.ToUpper(sha1) + "\n"},
		{name: "39 characters", output: sha1[:39] + "\n"},
		{name: "41 characters", output: sha1 + "a\n"},
		{name: "two ids on two lines", output: sha1 + "\n" + sha1 + "\n"},
		{name: "two newlines", output: sha1 + "\n\n"},
		{name: "carriage return before the newline", output: sha1 + "\r\n"},
		{name: "trailing space", output: sha1 + " \n"},
		{name: "a ref name beside the id", output: sha1 + " refs/remotes/origin/main\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shimDir := t.TempDir()
			outputPath := filepath.Join(shimDir, "output")
			if err := os.WriteFile(outputPath, []byte(tt.output), 0o644); err != nil {
				t.Fatalf("writing stand-in output: %v", err)
			}
			script := "#!/bin/sh\ncat '" + outputPath + "'\n"
			if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
				t.Fatalf("writing stand-in git: %v", err)
			}
			t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			got, err := ResolveExactRef(ctx, t.TempDir(), "refs/remotes/origin/main")
			if tt.ok {
				if err != nil || got != tt.want {
					t.Fatalf("ResolveExactRef over %q = %q, %v; want %q, nil", tt.output, got, err, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("ResolveExactRef over %q = %q, nil; want an error (not one full object id)", tt.output, got)
			}
			if !strings.Contains(err.Error(), "not one full object id") {
				t.Fatalf("ResolveExactRef over %q: err = %v, want the object-id refusal", tt.output, err)
			}
		})
	}
}
