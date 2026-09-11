package publicrelease

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceRequiresActualCleanExactCommit(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	for _, args := range [][]string{{"git", "init", "-q"}, {"git", "config", "user.name", "Fixture"}, {"git", "config", "user.email", "fixture@example.invalid"}} {
		if _, err := commandBytes(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"git", "add", "file"}, {"git", "-c", "commit.gpgsign=false", "commit", "-qm", "Freeze fixture"}} {
		if _, err := commandBytes(ctx, root, args...); err != nil {
			t.Fatal(err)
		}
	}
	head, err := commandBytes(ctx, root, "git", "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.TrimSpace(string(head))
	got, err := source(ctx, root, sha)
	if err != nil || got.Files["file"] != digest([]byte("original")) {
		t.Fatalf("source: %+v %v", got, err)
	}
	for _, want := range []string{"HEAD", strings.Repeat("0", 40)} {
		if _, err := source(ctx, root, want); err == nil {
			t.Fatal("accepted stale/symbolic source")
		}
	}
	if err = os.WriteFile(file, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = source(ctx, root, sha); err == nil {
		t.Fatal("accepted changed source")
	}
}
func TestBoundInputsRefuseEveryChangedIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*inputs)
	}{
		{"other repo head", func(i *inputs) { i.ATC.Commit = "different" }}, {"other repo tree", func(i *inputs) { i.ATC.Tree = "different" }},
		{"binary", func(i *inputs) { i.Binaries = map[string]string{"v": "new"} }}, {"contract", func(i *inputs) { i.Contract = "new" }},
		{"story", func(i *inputs) { i.Story = map[string]string{"spec": "new"} }}, {"manifest", func(i *inputs) { i.CorpusSHA = "new" }},
		{"source bytes", func(i *inputs) { i.Verdi.Files = map[string]string{"file": "new"} }}, {"checker", func(i *inputs) { i.CheckerSHA = "new" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := inputs{}
			after := before
			if err := unchanged(before, after); err != nil {
				t.Fatal(err)
			}
			tc.change(&after)
			if err := unchanged(before, after); err == nil {
				t.Fatal("accepted stale bound input")
			}
		})
	}
}
func TestInventoryRejectsEmptyAndNonregularInputs(t *testing.T) {
	root := t.TempDir()
	if _, err := inventory(root); err == nil {
		t.Fatal("empty accepted")
	}
	if err := os.Symlink("elsewhere", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := inventory(root); err == nil {
		t.Fatal("symlink accepted")
	}
}
