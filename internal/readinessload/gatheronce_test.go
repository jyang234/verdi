package readinessload

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/gitx"
)

// argvLog records the argv of every git process a context launches.
type argvLog struct {
	mu   sync.Mutex
	argv [][]string
}

func (l *argvLog) Observe(_ string, args []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.argv = append(l.argv, append([]string(nil), args...))
}

// count returns how many launched processes ran exactly want.
func (l *argvLog) count(want ...string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, args := range l.argv {
		if reflect.DeepEqual(args, want) {
			n++
		}
	}
	return n
}

// TestLoad_GathersFactsOncePerLoad (ledger SI-352, lane P1 (a)): one
// readiness load gathers the target's repository and lifecycle facts once
// and projects its journey over those same facts. The repository-facts
// gather begins with `git remote`, which nothing else in a load runs, so
// it counts gathers: one per load, where projecting the journey used to
// gather a second time.
func TestLoad_GathersFactsOncePerLoad(t *testing.T) {
	for _, class := range []string{"feature", "story"} {
		t.Run(class, func(t *testing.T) {
			repo, ref := readinessRepo(t, class)
			log := &argvLog{}
			if _, err := Load(gitx.WithObserver(context.Background(), log), repo.Dir, ref, Options{}); err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := log.count("remote"); got != 1 {
				t.Fatalf("one load ran `git remote` %d times, want once: the facts were gathered %d times", got, got)
			}
		})
	}
}

// TestLoad_FactsAreFreshEachLoad (ledger SI-352, lane P1 (a); co-2): the
// facts a load gathers live exactly as long as that load. A commit between
// two loads must move the second snapshot to the new HEAD — facts carried
// over from the first load would leave it at the old one.
func TestLoad_FactsAreFreshEachLoad(t *testing.T) {
	repo, ref := readinessRepo(t, "story")
	ctx := context.Background()
	before, err := Load(ctx, repo.Dir, ref, Options{})
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "unrelated.txt"), []byte("moves HEAD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	head := commitAll(t, repo.Dir, "move HEAD between two loads")
	after, err := Load(ctx, repo.Dir, ref, Options{})
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if before.Head == after.Head || after.Head != head {
		t.Fatalf("second load's HEAD = %s (first %s), want the new HEAD %s", after.Head, before.Head, head)
	}
	if !strings.Contains(after.StaleNotice, head) {
		t.Fatalf("second load's derivation stamp = %q, want it to name the new HEAD %s", after.StaleNotice, head)
	}
}

// TestLoad_NeverWritesTheIndex (BL-105; ledger SI-343 (3), SI-352; Wave 6
// §5.3 "no mutation"): a readiness load is a read. After a tracked file's
// mtime moves with its bytes unchanged — the stat drift a plain `git
// status` refreshes and writes back — a load leaves .git/index byte for
// byte as it was.
func TestLoad_NeverWritesTheIndex(t *testing.T) {
	for _, class := range []string{"feature", "story"} {
		t.Run(class, func(t *testing.T) {
			repo, ref := readinessRepo(t, class)
			spec := filepath.Join(repo.Dir, ".verdi", "specs", "active", strings.TrimPrefix(ref, "spec/"), "spec.md")
			stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			if err := os.Chtimes(spec, stale, stale); err != nil {
				t.Fatal(err)
			}
			index := filepath.Join(repo.Dir, ".git", "index")
			before, err := os.ReadFile(index)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Load(context.Background(), repo.Dir, ref, Options{}); err != nil {
				t.Fatalf("Load: %v", err)
			}
			after, err := os.ReadFile(index)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("a readiness load rewrote .git/index: a read path took git's optional index lock")
			}
		})
	}
}

// TestLoad_ReadsThroughOneSession (ledger SI-352, lane P1 (c)): a load's
// object reads go through its read session's one batch process — no
// per-path `git show` or `git ls-tree <rev> -- <path>` — and no identical
// ref-read argv runs twice in one load.
func TestLoad_ReadsThroughOneSession(t *testing.T) {
	for _, class := range []string{"feature", "story"} {
		t.Run(class, func(t *testing.T) {
			repo, ref := readinessRepo(t, class)
			log := &argvLog{}
			if _, err := Load(gitx.WithObserver(context.Background(), log), repo.Dir, ref, Options{}); err != nil {
				t.Fatalf("Load: %v", err)
			}
			seen := map[string]bool{}
			batches := 0
			for _, args := range log.argv {
				key := strings.Join(args, " ")
				switch {
				case args[0] == "cat-file":
					batches++
				case args[0] == "show", args[0] == "ls-tree" && args[1] != "-r":
					t.Errorf("the load ran `git %s` outside its read session", key)
				case args[0] == "symbolic-ref" || args[0] == "show-ref" || args[0] == "rev-parse" && args[1] == "--verify":
					if seen[key] {
						t.Errorf("the load resolved `git %s` more than once", key)
					}
					seen[key] = true
				}
			}
			if batches != 1 {
				t.Fatalf("the load started %d batch processes, want one: %v", batches, log.argv)
			}
		})
	}
}
