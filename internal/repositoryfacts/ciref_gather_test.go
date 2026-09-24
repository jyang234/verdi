package repositoryfacts

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// =========================================================================
// SI-257: gathering the CI-ref fact from the provider environment, validated
// against the checkout's own remote-tracking ref.
// =========================================================================

// ciEnvVars is every environment variable the CI-ref gather may read, plus
// GITHUB_HEAD_REF, which it must never read: each test clears all of them
// so a host CI's own environment cannot leak into a row.
var ciEnvVars = []string{
	"GITHUB_ACTIONS", "GITHUB_REF_NAME", "GITHUB_REF_TYPE", "GITHUB_HEAD_REF",
	"GITLAB_CI", "CI_COMMIT_REF_NAME", "CI_COMMIT_BRANCH", "CI_COMMIT_TAG",
}

func setCIEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for _, key := range ciEnvVars {
		t.Setenv(key, "")
	}
	for key, value := range env {
		t.Setenv(key, value)
	}
}

// noCIEnv is the env port of a run outside any CI provider.
func noCIEnv(string) string { return "" }

// recordingGitReader delegates to a real GitReader and records every
// RevParse and ResolveExactRef argument, so a test can prove which
// revisions reached git, and through which read.
type recordingGitReader struct {
	GitReader
	mu    sync.Mutex
	revs  []string // RevParse arguments
	exact []string // ResolveExactRef arguments
}

func (r *recordingGitReader) RevParse(ctx context.Context, dir, rev string) (string, error) {
	r.mu.Lock()
	r.revs = append(r.revs, rev)
	r.mu.Unlock()
	return r.GitReader.RevParse(ctx, dir, rev)
}

func (r *recordingGitReader) ResolveExactRef(ctx context.Context, dir, ref string) (string, error) {
	r.mu.Lock()
	r.exact = append(r.exact, ref)
	r.mu.Unlock()
	return r.GitReader.ResolveExactRef(ctx, dir, ref)
}

// remoteTrackingRevs returns every refs/remotes/origin/... argument that
// reached git through either read.
func (r *recordingGitReader) remoteTrackingRevs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, rev := range append(append([]string{}, r.revs...), r.exact...) {
		if strings.HasPrefix(rev, "refs/remotes/origin/") {
			out = append(out, rev)
		}
	}
	return out
}

// remoteTrackingRevParses returns the refs/remotes/origin/... arguments
// that reached RevParse, whose lookup rules must never resolve a CI ref.
func (r *recordingGitReader) remoteTrackingRevParses() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, rev := range r.revs {
		if strings.HasPrefix(rev, "refs/remotes/origin/") {
			out = append(out, rev)
		}
	}
	return out
}

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// ciRefRepo builds a real two-commit repository, A then B, detached at A
// (the CI close checkout's shape), with two remote-tracking refs:
// refs/remotes/origin/close/spec-x at A (equal to HEAD) and
// refs/remotes/origin/main at B (not HEAD — though main~1 and main^ both
// name A, so a ref name that reached rev-parse unvalidated would match).
func ciRefRepo(t *testing.T) (dir, head string) {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "a\n"}, Message: "commit A"},
		{Files: map[string]string{"b.txt": "b\n"}, Message: "commit B"},
	})
	a, b := repo.Heads[0], repo.Heads[1]
	runTestGit(t, repo.Dir, "update-ref", "refs/remotes/origin/close/spec-x", a)
	runTestGit(t, repo.Dir, "update-ref", "refs/remotes/origin/main", b)
	runTestGit(t, repo.Dir, "checkout", "--quiet", "--detach", a)
	return repo.Dir, a
}

func githubEnv(name string) map[string]string {
	return map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "branch", "GITHUB_REF_NAME": name}
}

func gitlabEnv(name string) map[string]string {
	return map[string]string{"GITLAB_CI": "true", "CI_COMMIT_REF_NAME": name, "CI_COMMIT_BRANCH": name}
}

func unknownCIRef(reason string) CIRefFact { return CIRefFact{Reason: reason} }

func TestGather_CIRef(t *testing.T) {
	type row struct {
		name string
		env  map[string]string
		want CIRefFact
		// neverRevParsed: no refs/remotes/origin/... revision may reach git.
		neverRevParsed bool
	}
	rows := []row{
		{name: "not a CI run", env: nil, want: CIRefFact{}, neverRevParsed: true},
		{name: "github branch whose remote-tracking head is HEAD", env: githubEnv("close/spec-x"), want: CIRefFact{Known: true, Provider: CIProviderGitHub, Name: "close/spec-x"}},
		{name: "gitlab branch whose remote-tracking head is HEAD", env: gitlabEnv("close/spec-x"), want: CIRefFact{Known: true, Provider: CIProviderGitLab, Name: "close/spec-x"}},
		{
			name: "both providers",
			env:  map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "branch", "GITHUB_REF_NAME": "close/spec-x", "GITLAB_CI": "true", "CI_COMMIT_REF_NAME": "close/spec-x", "CI_COMMIT_BRANCH": "close/spec-x"},
			want: unknownCIRef(CIRefReasonProvidersAmbiguous), neverRevParsed: true,
		},
		{
			name: "provider flag not exactly true",
			env:  map[string]string{"GITHUB_ACTIONS": "1", "GITHUB_REF_TYPE": "branch", "GITHUB_REF_NAME": "close/spec-x"},
			want: CIRefFact{}, neverRevParsed: true,
		},
		{
			name: "github tag ref type",
			env:  map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "tag", "GITHUB_REF_NAME": "close/spec-x"},
			want: unknownCIRef(CIRefReasonRefTypeNotBranch), neverRevParsed: true,
		},
		{
			name: "github ref type absent",
			env:  map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_NAME": "close/spec-x"},
			want: unknownCIRef(CIRefReasonRefTypeNotBranch), neverRevParsed: true,
		},
		{
			name: "github ref name absent",
			env:  map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "branch"},
			want: unknownCIRef(CIRefReasonNameMissing), neverRevParsed: true,
		},
		{
			// A pull_request run names refs/pull/<n>/merge; GITHUB_HEAD_REF
			// names the head branch, which is never consulted.
			name: "github pull request run: GITHUB_HEAD_REF is never read",
			env:  map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REF_TYPE": "branch", "GITHUB_REF_NAME": "17/merge", "GITHUB_HEAD_REF": "close/spec-x"},
			want: unknownCIRef(CIRefReasonRemoteTrackingUnresolved),
		},
		{
			name: "gitlab tag pipeline",
			env:  map[string]string{"GITLAB_CI": "true", "CI_COMMIT_REF_NAME": "close/spec-x", "CI_COMMIT_TAG": "v1.0.0"},
			want: unknownCIRef(CIRefReasonTagPipeline), neverRevParsed: true,
		},
		{
			name: "gitlab ref name mismatched with CI_COMMIT_BRANCH",
			env:  map[string]string{"GITLAB_CI": "true", "CI_COMMIT_REF_NAME": "close/spec-x", "CI_COMMIT_BRANCH": "main"},
			want: unknownCIRef(CIRefReasonBranchMismatch), neverRevParsed: true,
		},
		{
			name: "gitlab merge-request pipeline: CI_COMMIT_BRANCH absent",
			env:  map[string]string{"GITLAB_CI": "true", "CI_COMMIT_REF_NAME": "close/spec-x"},
			want: unknownCIRef(CIRefReasonBranchMismatch), neverRevParsed: true,
		},
		{
			name: "gitlab ref name absent",
			env:  map[string]string{"GITLAB_CI": "true"},
			want: unknownCIRef(CIRefReasonNameMissing), neverRevParsed: true,
		},
		{
			name: "gitlab invalid name",
			env:  gitlabEnv("main~1"),
			want: unknownCIRef(CIRefReasonNameInvalid), neverRevParsed: true,
		},
		{name: "remote-tracking ref missing", env: githubEnv("feature/absent"), want: unknownCIRef(CIRefReasonRemoteTrackingUnresolved)},
		{name: "remote-tracking head is not HEAD", env: githubEnv("main"), want: unknownCIRef(CIRefReasonRemoteTrackingNotHead)},
	}
	for _, tt := range invalidBranchNameCases {
		if tt.name == "" {
			continue // an empty name is CIRefReasonNameMissing, covered above
		}
		rows = append(rows, row{
			name: "invalid name: " + tt.rule, env: githubEnv(tt.name),
			want: unknownCIRef(CIRefReasonNameInvalid), neverRevParsed: true,
		})
	}

	for _, tt := range rows {
		t.Run(tt.name, func(t *testing.T) {
			dir, head := ciRefRepo(t)
			setCIEnv(t, tt.env)
			git := &recordingGitReader{GitReader: NewGitReader()}
			g := newGatherer(git, alwaysUnresolvedDefaultBranch, os.Getenv)

			snap, err := g.Gather(context.Background(), GatherInput{Root: dir})
			if err != nil {
				t.Fatalf("Gather: %v", err)
			}
			if snap.CIRef != tt.want {
				t.Fatalf("CIRef = %+v, want %+v", snap.CIRef, tt.want)
			}
			if err := snap.Validate(); err != nil {
				t.Fatalf("Snapshot.Validate(): %v", err)
			}
			if snap.Facts.Head != (StringFact{Known: true, Value: head}) {
				t.Fatalf("Facts.Head = %+v, want %s", snap.Facts.Head, head)
			}
			if snap.Facts.Branch.Known || !containsCode(snap.Disclosures, DisclosureBranchDetached) {
				t.Fatalf("Facts.Branch = %+v disclosures %v: the detached state must stay recorded", snap.Facts.Branch, snap.Disclosures)
			}
			if revs := git.remoteTrackingRevs(); tt.neverRevParsed && len(revs) != 0 {
				t.Fatalf("remote-tracking revisions reached git: %v", revs)
			}
			if revs := git.remoteTrackingRevParses(); len(revs) != 0 {
				t.Fatalf("remote-tracking revisions reached RevParse's lookup rules: %v", revs)
			}
		})
	}
}

// TestGather_CIRefReadsOnlyTheExactRemoteTrackingRef is the E4a review's
// M-1 probe as a regression test: with refs/remotes/origin/<name> absent,
// a tag or a branch literally named refs/remotes/origin/<name> at HEAD
// would satisfy `git rev-parse --verify` through its lookup rules. SI-257
// requires "the exact ref", so each decoy leaves the CI ref unknown, while
// the exact ref at HEAD (the control) is known.
func TestGather_CIRefReadsOnlyTheExactRemoteTrackingRef(t *testing.T) {
	tests := []struct {
		name  string
		decoy []string // git arguments creating the decoy at HEAD; nil for the control
		want  CIRefFact
	}{
		{name: "exact ref at HEAD (control)", want: CIRefFact{Known: true, Provider: CIProviderGitHub, Name: "close/spec-x"}},
		{name: "tag decoy", decoy: []string{"tag", "refs/remotes/origin/feature/decoy"}, want: unknownCIRef(CIRefReasonRemoteTrackingUnresolved)},
		{name: "branch decoy", decoy: []string{"branch", "refs/remotes/origin/feature/decoy"}, want: unknownCIRef(CIRefReasonRemoteTrackingUnresolved)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, head := ciRefRepo(t)
			name := "close/spec-x"
			if tt.decoy != nil {
				name = "feature/decoy"
				runTestGit(t, dir, append(tt.decoy, head)...)
			}
			setCIEnv(t, githubEnv(name))
			snap, err := newGatherer(NewGitReader(), alwaysUnresolvedDefaultBranch, os.Getenv).Gather(context.Background(), GatherInput{Root: dir})
			if err != nil {
				t.Fatalf("Gather: %v", err)
			}
			if snap.CIRef != tt.want {
				t.Fatalf("CIRef = %+v, want %+v", snap.CIRef, tt.want)
			}
			if tt.decoy != nil && snap.BranchBeingClosed().Known {
				t.Fatalf("BranchBeingClosed() = %+v, want unknown: a decoy must never name the branch being closed", snap.BranchBeingClosed())
			}
		})
	}
}

// TestGather_CIRefLeavesFactsUnchanged proves the CI ref changes nothing
// else a Snapshot carries: the same checkout gathered inside and outside a
// CI run yields byte-identical Facts and Disclosures.
func TestGather_CIRefLeavesFactsUnchanged(t *testing.T) {
	dir, _ := ciRefRepo(t)
	setCIEnv(t, nil)
	outside, err := newGatherer(NewGitReader(), alwaysUnresolvedDefaultBranch, os.Getenv).Gather(context.Background(), GatherInput{Root: dir})
	if err != nil {
		t.Fatalf("Gather outside CI: %v", err)
	}
	setCIEnv(t, githubEnv("close/spec-x"))
	inside, err := newGatherer(NewGitReader(), alwaysUnresolvedDefaultBranch, os.Getenv).Gather(context.Background(), GatherInput{Root: dir})
	if err != nil {
		t.Fatalf("Gather inside CI: %v", err)
	}
	if !inside.CIRef.Known {
		t.Fatalf("CIRef = %+v, want known", inside.CIRef)
	}
	if !reflect.DeepEqual(inside.Facts, outside.Facts) || !reflect.DeepEqual(inside.Disclosures, outside.Disclosures) {
		t.Fatalf("CI ref changed Facts/Disclosures:\ninside=%+v %v\noutside=%+v %v", inside.Facts, inside.Disclosures, outside.Facts, outside.Disclosures)
	}
	if got := inside.BranchBeingClosed(); got != (StringFact{Known: true, Value: "close/spec-x"}) {
		t.Fatalf("BranchBeingClosed() = %+v, want the validated CI ref", got)
	}
	if got := outside.BranchBeingClosed(); got.Known {
		t.Fatalf("BranchBeingClosed() outside CI = %+v, want unknown", got)
	}
}

// TestNewGatherer_ReadsProcessEnvironment proves the production constructor
// wires the env port to the process environment.
func TestNewGatherer_ReadsProcessEnvironment(t *testing.T) {
	dir, _ := ciRefRepo(t)
	setCIEnv(t, githubEnv("close/spec-x"))
	snap, err := NewGatherer().Gather(context.Background(), GatherInput{Root: dir})
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if want := (CIRefFact{Known: true, Provider: CIProviderGitHub, Name: "close/spec-x"}); snap.CIRef != want {
		t.Fatalf("CIRef = %+v, want %+v", snap.CIRef, want)
	}
}

// TestGather_CIRefGitFailures covers the two git failures a real repository
// cannot stage on demand: HEAD itself unresolvable (today's behavior kept —
// an unknown HEAD with its disclosure, never a Gather error, and the
// remote-tracking ref is never consulted), and an operational failure of
// the exact remote-tracking read (unknown, never a Gather error). The
// remote-tracking ref is read only through ResolveExactRef, never RevParse.
func TestGather_CIRefGitFailures(t *testing.T) {
	env := githubEnv("close/spec-x")
	getenv := func(key string) string { return env[key] }
	headOnly := func(head string, err error) func(context.Context, string, string) (string, error) {
		return func(_ context.Context, _, rev string) (string, error) {
			if rev == "HEAD" {
				return head, err
			}
			panic("RevParse consulted for a non-HEAD revision: " + rev)
		}
	}
	tests := []struct {
		name     string
		revParse func(context.Context, string, string) (string, error)
		exactRef func(context.Context, string, string) (string, error)
		want     CIRefFact
		wantHead bool
	}{
		{
			name:     "HEAD unresolved",
			revParse: headOnly("", errors.New("boom")),
			exactRef: func(_ context.Context, _, ref string) (string, error) {
				panic("remote-tracking ref consulted with HEAD unresolved: " + ref)
			},
			want: unknownCIRef(CIRefReasonHeadUnresolved),
		},
		{
			name:     "exact remote-tracking read errors",
			revParse: headOnly("headsha", nil),
			exactRef: func(context.Context, string, string) (string, error) {
				return "", errors.New("fatal: 'refs/remotes/origin/close/spec-x' - not a valid ref")
			},
			want:     unknownCIRef(CIRefReasonRemoteTrackingUnresolved),
			wantHead: true,
		},
		{
			name:     "exact remote-tracking ref at another commit",
			revParse: headOnly("headsha", nil),
			exactRef: func(context.Context, string, string) (string, error) { return "othersha", nil },
			want:     unknownCIRef(CIRefReasonRemoteTrackingNotHead),
			wantHead: true,
		},
		{
			name:     "exact remote-tracking ref at HEAD",
			revParse: headOnly("headsha", nil),
			exactRef: func(_ context.Context, _, ref string) (string, error) {
				if ref != "refs/remotes/origin/close/spec-x" {
					return "", errors.New("unexpected ref " + ref)
				}
				return "headsha", nil
			},
			want:     CIRefFact{Known: true, Provider: CIProviderGitHub, Name: "close/spec-x"},
			wantHead: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := baseGitReader()
			git.currentBranchFn = func(context.Context, string) (string, error) { return "", nil }
			git.revParseFn = tt.revParse
			git.exactRefFn = tt.exactRef
			snap, err := newGatherer(git, alwaysUnresolvedDefaultBranch, getenv).Gather(context.Background(), GatherInput{Root: t.TempDir()})
			if err != nil {
				t.Fatalf("Gather: %v", err)
			}
			if snap.CIRef != tt.want {
				t.Fatalf("CIRef = %+v, want %+v", snap.CIRef, tt.want)
			}
			if snap.Facts.Head.Known != tt.wantHead {
				t.Fatalf("Facts.Head = %+v, want known=%v", snap.Facts.Head, tt.wantHead)
			}
			if !tt.wantHead && !containsCode(snap.Disclosures, DisclosureHeadUnresolved) {
				t.Fatalf("Disclosures = %v, want %q", snap.Disclosures, DisclosureHeadUnresolved)
			}
			if err := snap.Validate(); err != nil {
				t.Fatalf("Snapshot.Validate(): %v", err)
			}
		})
	}
}

func TestGatherer_NilEnvPort_FailsClosed(t *testing.T) {
	g := newGatherer(baseGitReader(), alwaysUnresolvedDefaultBranch, nil)
	if _, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir()}); err == nil {
		t.Fatal("Gather() with a nil env port: want error")
	}
}
