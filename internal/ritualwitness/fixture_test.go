package ritualwitness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildWith_TracksTheBaseFilesAtTheSeedCommit: a real ritual's store
// is tracked at the one seed commit the default branch resolves to, under
// the same seeded working state Build lays over it.
func TestBuildWith_TracksTheBaseFilesAtTheSeedCommit(t *testing.T) {
	ctx := context.Background()
	base := map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", "docs/readme.md": "a store file\n"}
	for _, state := range both() {
		t.Run(state.String(), func(t *testing.T) {
			fx := BuildWith(t, ctx, state, base)
			snap, err := Capture(ctx, fx.Dir, fx.Bare)
			if err != nil {
				t.Fatal(err)
			}
			for path := range base {
				if _, ok := snap.Trees[fx.BaseCommit][path]; !ok {
					t.Errorf("%s is not tracked at the seed commit", path)
				}
				if _, ok := snap.HeadTree[path]; !ok {
					t.Errorf("%s is not in HEAD's tree", path)
				}
			}
			if got := snap.RemoteRefs["refs/heads/main"].Object; got != fx.BaseCommit {
				t.Fatalf("the remote's main = %s, want the seed commit %s", got, fx.BaseCommit)
			}
			if got := staged(snap, ForeignFile); got != (state == SeedFull) {
				t.Fatalf("%s staged = %v in the %s state", ForeignFile, got, state)
			}
			if _, err := os.Stat(filepath.Join(fx.Dir, UntrackedFile)); err != nil {
				t.Fatalf("the seeded untracked file is missing: %v", err)
			}
		})
	}
}

// TestBuildWith_RefusesABaseFileTheSeedCommitDoesNotTrack (R4-B4): a base
// file the seed commit leaves untracked — here one a base-supplied nested
// .gitignore ignores — fails BuildWith, naming it, rather than leaving the
// ritual a store that is not at the default branch's commit.
func TestBuildWith_RefusesABaseFileTheSeedCommitDoesNotTrack(t *testing.T) {
	ctx := context.Background()
	rec := &fatalRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		BuildWith(rec, ctx, SeedClean, map[string]string{
			".verdi/.gitignore":       "*.json\n",
			".verdi/store-state.json": "{}\n",
			".verdi/verdi.yaml":       "schema: verdi.layout/v1\n",
		})
	}()
	<-done
	if got := rec.message(); !strings.Contains(got, ".verdi/store-state.json") || strings.Contains(got, ".verdi/verdi.yaml") {
		t.Fatalf("BuildWith failed with %q, want it to name only the untracked .verdi/store-state.json", got)
	}
}

// TestBuildWith_NilBaseIsBuild: Build is BuildWith with no base files, so
// both seed the same commit.
func TestBuildWith_NilBaseIsBuild(t *testing.T) {
	ctx := context.Background()
	if a, b := Build(t, ctx, SeedClean).BaseCommit, BuildWith(t, ctx, SeedClean, nil).BaseCommit; a != b {
		t.Fatalf("Build's seed commit %s differs from BuildWith(nil)'s %s", a, b)
	}
}

// TestBaseCollision: a base file may not replace a file the fixture
// itself seeds or lays over the base, nor sit where git would ignore it.
func TestBaseCollision(t *testing.T) {
	tests := []struct {
		name string
		base map[string]string
		want string
	}{
		{"none", nil, ""},
		{"store files", map[string]string{".verdi/verdi.yaml": "x", ".verdi/specs/active/a/spec.md": "y"}, ""},
		{"the fixture's .gitignore", map[string]string{".gitignore": "x"}, ".gitignore"},
		{"the tracked file", map[string]string{TrackedFile: "x"}, TrackedFile},
		{"the owned directory's file", map[string]string{OwnedDir + "/keep.txt": "x"}, OwnedDir + "/keep.txt"},
		{"the untracked file", map[string]string{UntrackedFile: "x"}, UntrackedFile},
		{"the foreign file", map[string]string{ForeignFile: "x"}, ForeignFile},
		{"an ignored path", map[string]string{IgnoredDir + "mutable/x.json": "x"}, IgnoredDir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := baseCollision(tt.base)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("baseCollision = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("baseCollision = %v, want an error naming %q", err, tt.want)
			}
		})
	}
}
