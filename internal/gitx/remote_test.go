package gitx

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestRemoteURL_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	const want = "https://example.invalid/acme/svcfix.git"
	if err := exec.CommandContext(ctx, "git", "-C", repo.Dir, "remote", "add", "origin", want).Run(); err != nil {
		t.Fatalf("git remote add (test setup): %v", err)
	}

	got, err := RemoteURL(ctx, repo.Dir, "origin")
	if err != nil {
		t.Fatalf("RemoteURL: %v", err)
	}
	if got != want {
		t.Fatalf("RemoteURL = %q, want %q", got, want)
	}
}

func TestRemoteURL_Negative_NoSuchRemote(t *testing.T) {
	repo := buildRepo(t)
	_, err := RemoteURL(context.Background(), repo.Dir, "does-not-exist")
	if err == nil {
		t.Fatal("RemoteURL(nonexistent remote): want error, got nil")
	}
	// A genuinely-absent remote is the benign case, marked with the
	// ErrNoSuchRemote sentinel so a caller (sync's ac-1 forge identification)
	// can tell it apart from a real read failure it must surface as
	// operational (ADJ-64: never conflate unreadable with absent).
	if !errors.Is(err, ErrNoSuchRemote) {
		t.Errorf("RemoteURL(nonexistent remote) err = %v, want errors.Is(err, ErrNoSuchRemote)", err)
	}
}

func TestRemoteURL_Negative_NoSuchRemote_LocaleIndependent(t *testing.T) {
	// git localizes its "No such remote" stderr (e.g. "No existe el remoto",
	// "Pas de serveur remote"), so absence must be detected from the
	// locale-independent `git remote` name list, never that message. Under a
	// localizing locale a benign absent remote must still be ErrNoSuchRemote,
	// not a misclassified operational read failure (ADJ-64).
	t.Setenv("LC_ALL", "fr_FR.UTF-8")
	repo := buildRepo(t)
	_, err := RemoteURL(context.Background(), repo.Dir, "does-not-exist")
	if err == nil {
		t.Fatal("RemoteURL(nonexistent remote, localized locale): want error, got nil")
	}
	if !errors.Is(err, ErrNoSuchRemote) {
		t.Errorf("RemoteURL(nonexistent remote, localized locale) err = %v, want errors.Is(err, ErrNoSuchRemote)", err)
	}
}

func TestRemoteURL_Negative_NotARepo(t *testing.T) {
	_, err := RemoteURL(context.Background(), t.TempDir(), "origin")
	if err == nil {
		t.Fatal("RemoteURL outside a git repo: want error, got nil")
	}
	// A non-repo (or any operational git failure) is NOT the benign
	// absent-remote case: it stays a plain operational error, never
	// masquerading as ErrNoSuchRemote (ADJ-64).
	if errors.Is(err, ErrNoSuchRemote) {
		t.Errorf("RemoteURL(non-repo dir) err = %v, want NOT ErrNoSuchRemote (a genuine operational failure)", err)
	}
}
