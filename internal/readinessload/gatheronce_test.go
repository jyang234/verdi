package readinessload

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

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
