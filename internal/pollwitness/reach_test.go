package pollwitness

import (
	"context"
	"os/exec"
	"regexp"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

// fullCommitID matches a full lowercase object id.
var fullCommitID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// gitArgv records every git process a context launches.
type gitArgv struct {
	mu   sync.Mutex
	argv [][]string
}

func (g *gitArgv) Observe(_ string, args []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.argv = append(g.argv, append([]string(nil), args...))
}

// TestPollWitness_BadgeReachabilityIsOneWalk (BL-157; ledger SI-352, lane
// P1 (d)): a wall's badge compute — a corpus-wide lint run whose VL-009
// and VL-003 checks ask whether every frozen stamp and pinned commit is
// reachable from HEAD — walks each head once, and never runs a per-commit
// merge-base for a full commit id that walk reaches. Only the commits the
// walk cannot answer (unreachable, abbreviated, not commits) still take
// the per-commit path. The pinned badge bytes are TestPollWitness's.
func TestPollWitness_BadgeReachabilityIsOneWalk(t *testing.T) {
	for _, fx := range witnessFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			repo := fx.build(t)
			for _, spec := range fx.specs {
				t.Run(spec, func(t *testing.T) {
					log := &gitArgv{}
					ctx := gitx.WithObserver(context.Background(), log)
					badgesCtx(t, ctx, repo.Dir, spec)

					walks := map[string]int{}
					checked := 0
					for _, args := range log.argv {
						switch {
						case len(args) == 3 && args[0] == "rev-list" && args[2] == "--":
							walks[args[1]]++
						case len(args) == 4 && args[0] == "merge-base" && args[1] == "--is-ancestor":
							checked++
							commit, head := args[2], args[3]
							if fullCommitID.MatchString(commit) && isAncestor(t, repo.Dir, commit, head) {
								t.Errorf("the badge compute ran a merge-base for %s, which its walk of %s reaches", commit, head)
							}
						}
					}
					for head, n := range walks {
						if n != 1 {
							t.Errorf("the badge compute walked %s %d times, want once", head, n)
						}
					}
					t.Logf("walks %v; per-commit checks left %d", walks, checked)
					if len(walks) == 0 {
						t.Fatal("the badge compute walked no head: the lint run checked no reachability, so this witness proves nothing")
					}
				})
			}
		})
	}
}

func isAncestor(t *testing.T, dir, commit, head string) bool {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "merge-base", "--is-ancestor", commit, head)
	cmd.Dir = dir
	return cmd.Run() == nil
}
