package specstate

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// corpusScanObserver records the revision of every successor-corpus scan
// gitx runs on a context it is attached to: the recursive ls-tree over
// specZonesPrefix. BlobAt's own ls-tree names a single spec path, so it
// is not counted.
type corpusScanObserver struct {
	mu   sync.Mutex
	revs []string
}

func (o *corpusScanObserver) Observe(dir string, args []string) {
	if len(args) == 6 && args[0] == "ls-tree" && args[1] == "-r" && args[5] == specZonesPrefix {
		o.mu.Lock()
		defer o.mu.Unlock()
		o.revs = append(o.revs, args[3])
	}
}

func (o *corpusScanObserver) scans() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.revs...)
}

// TestProjector_CorpusMemo_Integration proves the memo over real git and
// the production constructor: separate NewProjector values share one
// process cache, so repeated resolves read the default-branch corpus once,
// at the commit the branch resolves to; landing a successor moves the
// branch, which is read again and now supersedes the predecessor.
func TestProjector_CorpusMemo_Integration(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{memoPredPath: memoPred}, Message: "land old-feature"},
	})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	var obs corpusScanObserver
	ctx := gitx.WithObserver(context.Background(), &obs)
	candidate := Candidate{Path: memoPredPath, Content: []byte(memoPred)}

	for i := 0; i < 3; i++ {
		result, err := NewProjector().Resolve(ctx, repo.Dir, candidate)
		if err != nil {
			t.Fatalf("Resolve %d at the first commit: %v", i, err)
		}
		if result.State != AcceptedPendingBuild {
			t.Fatalf("Resolve %d at the first commit = %+v, want %s", i, result, AcceptedPendingBuild)
		}
	}
	if got, want := obs.scans(), []string{repo.Head}; !reflect.DeepEqual(got, want) {
		t.Fatalf("corpus scans at = %q, want one scan at the resolved commit %q", got, want)
	}

	writeFileForTest(t, repo.Dir, memoSuccPath, string(validSuccessorSpec("new-feature", "old-feature")))
	runGitForTest(t, repo.Dir, "add", "-A")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "land new-feature, superseding old-feature")
	moved := strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "main"))

	for i := 0; i < 2; i++ {
		result, err := NewProjector().Resolve(ctx, repo.Dir, candidate)
		if err != nil {
			t.Fatalf("Resolve %d after the branch moved: %v", i, err)
		}
		if result.State != Superseded {
			t.Fatalf("Resolve %d after the branch moved = %+v, want %s", i, result, Superseded)
		}
	}
	if got, want := obs.scans(), []string{repo.Head, moved}; !reflect.DeepEqual(got, want) {
		t.Fatalf("corpus scans at = %q, want %q (one more scan, at the moved commit)", got, want)
	}
}

// TestRealGitReader_RevParse proves the production adapter resolves the
// default branch to its commit and fails on a revision that does not
// exist.
func TestRealGitReader_RevParse(t *testing.T) {
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"seed.txt": "seed\n"}, Message: "seed"},
	})

	tests := []struct {
		name    string
		rev     string
		want    string
		wantErr bool
	}{
		{name: "a branch resolves to its commit", rev: "main^{commit}", want: repo.Head},
		{name: "a missing branch is an error", rev: "no-such-branch^{commit}", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := realGitReader{}.RevParse(ctx, repo.Dir, tt.rev)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("RevParse(%q) = %q, want an error", tt.rev, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("RevParse(%q) = (%q, %v), want (%q, nil)", tt.rev, got, err, tt.want)
			}
		})
	}
}
