package ritualwitness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

// Synthetic rituals: in-process Ritual functions, each exercising one
// verdict. None is a real ritual (that is spec/ritual-effect-witness's R3
// lane). A ritual that mutates through plain git, not gitx, does so on
// purpose: that is a mutation made outside gitx (parent dc-2).

// writeAndStage writes content to dir/path and stages it through gitx.
func writeAndStage(ctx context.Context, dir, path, content string) error {
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return err
	}
	return gitx.AddPaths(ctx, dir, path)
}

// plainGit runs git outside gitx, so no observer sees it.
func plainGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", errWithOutput(err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func errWithOutput(err error, out []byte) error {
	return fmt.Errorf("%w: %s", err, out)
}

// steps runs each step in order, exiting 2 with the first error.
func steps(fns ...func(ctx context.Context, dir string) error) Ritual {
	return func(ctx context.Context, dir string) (int, error) {
		for _, fn := range fns {
			if err := fn(ctx, dir); err != nil {
				return 2, err
			}
		}
		return 0, nil
	}
}

func newBranch(name string) func(context.Context, string) error {
	return func(ctx context.Context, dir string) error { return gitx.CheckoutNewBranch(ctx, dir, name) }
}

func checkout(ref string) func(context.Context, string) error {
	return func(ctx context.Context, dir string) error { return gitx.CheckoutExisting(ctx, dir, ref) }
}

func stage(path, content string) func(context.Context, string) error {
	return func(ctx context.Context, dir string) error { return writeAndStage(ctx, dir, path, content) }
}

func addPaths(paths ...string) func(context.Context, string) error {
	return func(ctx context.Context, dir string) error { return gitx.AddPaths(ctx, dir, paths...) }
}

func commitPaths(paths ...string) func(context.Context, string) error {
	return func(ctx context.Context, dir string) error {
		_, err := gitx.CreateCommitPaths(ctx, dir, "commit "+strings.Join(paths, " "), paths...)
		return err
	}
}

func commitIndex(ctx context.Context, dir string) error {
	_, err := gitx.CreateCommit(ctx, dir, "commit the whole index")
	return err
}

func plain(args ...string) func(context.Context, string) error {
	return func(ctx context.Context, dir string) error {
		_, err := plainGit(ctx, dir, args...)
		return err
	}
}

func writeFile(path, content string) func(context.Context, string) error {
	return func(_ context.Context, dir string) error {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		return os.WriteFile(full, []byte(content), 0o644)
	}
}

func removeFile(path string) func(context.Context, string) error {
	return func(_ context.Context, dir string) error {
		return os.Remove(filepath.Join(dir, filepath.FromSlash(path)))
	}
}

// refuseIfStaged exits the ritual with 2 when any entry is staged: a guard.
func refuseIfStaged(ctx context.Context, dir string) error {
	staged, err := gitx.StagedPaths(ctx, dir)
	if err != nil {
		return err
	}
	if len(staged) > 0 {
		return errors.New("refusing: an entry is already staged")
	}
	return nil
}

// ritualScoped stays within a scoped declaration: a branch and a commit
// of its own path.
func ritualScoped(ctx context.Context, dir string) (int, error) {
	return steps(newBranch("ritual/scoped-ok"), stage("owned/new.txt", "new\n"), commitPaths("owned/new.txt"))(ctx, dir)
}

// ritualMovesUndeclaredRef commits on the checked-out branch.
func ritualMovesUndeclaredRef(ctx context.Context, dir string) (int, error) {
	return steps(stage("owned/x.txt", "x\n"), commitPaths("owned/x.txt"))(ctx, dir)
}

// ritualCarriesForeign commits the whole index, so a pre-staged foreign
// entry rides into its commit.
func ritualCarriesForeign(ctx context.Context, dir string) (int, error) {
	return steps(newBranch("ritual/carries"), stage("owned/c.txt", "c\n"), commitIndex)(ctx, dir)
}

// ritualScopedButRefuses guards on a staged entry before any mutation,
// and otherwise makes a scoped commit.
func ritualScopedButRefuses(ctx context.Context, dir string) (int, error) {
	return steps(refuseIfStaged, newBranch("ritual/guard-then-scope"), stage("owned/f.txt", "f\n"), commitPaths("owned/f.txt"))(ctx, dir)
}

// ritualBypassesGitx creates a ref with plain git: no observer sees it.
func ritualBypassesGitx(ctx context.Context, dir string) (int, error) {
	return steps(func(ctx context.Context, dir string) error {
		head, err := gitx.RevParse(ctx, dir, "HEAD")
		if err != nil {
			return err
		}
		_, err = plainGit(ctx, dir, "update-ref", "refs/heads/ritual/sneaky", head)
		return err
	})(ctx, dir)
}

// noLog is a Driver that cannot supply a command log, like a Binary run
// whose VERDI_GITLOG file it cannot read whole.
type noLog struct{ fn Ritual }

func (d noLog) Run(ctx context.Context, dir string) (int, CommandLog, error) {
	exit, err := d.fn(ctx, dir)
	return exit, CommandLog{}, err
}

// --- expectations ---------------------------------------------------------

func v(field string, status Status, detail string) Verdict {
	return Verdict{Field: field, Status: status, Detail: detail}
}

func formatVerdicts(vs []Verdict) string {
	var b strings.Builder
	for _, x := range vs {
		b.WriteString("  " + x.String() + "\n")
	}
	if b.Len() == 0 {
		return "  (no verdicts)\n"
	}
	return b.String()
}

func sortedVerdicts(vs []Verdict) []Verdict {
	out := append([]Verdict(nil), vs...)
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// verdictDiff compares the exact verdict multisets — every field, status,
// and detail, nothing missing and nothing extra — and returns "" when they
// are equal, or both lists.
func verdictDiff(got, want []Verdict) string {
	g, w := sortedVerdicts(got), sortedVerdicts(want)
	if len(g) == len(w) {
		same := true
		for i := range g {
			if g[i] != w[i] {
				same = false
				break
			}
		}
		if same {
			return ""
		}
	}
	return "verdicts differ\n got:\n" + formatVerdicts(g) + " want:\n" + formatVerdicts(w)
}

// requireVerdict asserts that one verdict with field and status has a
// detail containing substr.
func requireVerdict(t *testing.T, vs []Verdict, field string, status Status, substr string) {
	t.Helper()
	for _, x := range vs {
		if x.Field == field && x.Status == status && strings.Contains(x.Detail, substr) {
			return
		}
	}
	t.Fatalf("no %s verdict with status %s (detail containing %q) among:\n%s", field, status, substr, formatVerdicts(vs))
}

// createdCommits returns the short ids of the commits a run created,
// sorted.
func createdCommits(res Result) []string {
	var ids []string
	for id := range res.After.Commits {
		if _, old := res.Before.Commits[id]; !old {
			ids = append(ids, short(id))
		}
	}
	sort.Strings(ids)
	return ids
}

// onlyCommit returns the short id of the one commit a run created.
func onlyCommit(t *testing.T, res Result) string {
	t.Helper()
	ids := createdCommits(res)
	if len(ids) != 1 {
		t.Fatalf("want exactly one created commit, got %v", ids)
	}
	return ids[0]
}
