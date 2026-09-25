package objsupersede

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// hermetic keeps an ambient CI_DEFAULT_BRANCH from choosing the branch.
func hermetic(t *testing.T) { t.Setenv("CI_DEFAULT_BRANCH", "") }

func obj(spec, id string) artifact.Ref {
	return artifact.Ref{Kind: artifact.KindSpec, Name: spec, Object: id}
}

// TestHistory_Acceptance pins SI-270: the earliest first-parent commit whose
// tree holds the spec in either zone, dated in UTC; a later in-place edit
// does not move it; a missing default branch or shallow history is unproven.
func TestHistory_Acceptance(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	chain := scenario.Build(t, "chain")
	proposed := scenario.Build(t, "proposed")
	unresolved := scenario.Build(t, "accepted")
	gitIn(t, unresolved.Dir, "update-ref", "-d", "refs/remotes/origin/main")
	shallow := fixturegit.ShallowClone(t, &fixturegit.Repo{Dir: chain.Dir}, 2)
	tests := []struct {
		name, dir, spec string
		want            Fact
	}{
		{"accepted by merge, dated by its committer date; the in-place edit does not move it", chain.Dir, "successor", Fact{State: FactProven, Commit: chain.Steps[1], Date: "2024-02-15"}},
		{"a revision", chain.Dir, "successor-v2", Fact{State: FactProven, Commit: chain.Steps[4], Date: "2024-03-15"}},
		{"accepted in the active zone; the later archive move does not move it", chain.Dir, "closed-feature", Fact{State: FactProven, Commit: chain.Base[1], Date: "2024-01-01"}},
		{"only on a design branch", proposed.Dir, "successor", Fact{State: FactAbsent}},
		{"never written", chain.Dir, "nowhere", Fact{State: FactAbsent}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewHistory(ctx, tc.dir).Acceptance(ctx, tc.spec); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
	for name, dir := range map[string]string{"no default branch": unresolved.Dir, "shallow history": shallow} {
		t.Run(name, func(t *testing.T) {
			got := NewHistory(ctx, dir).Acceptance(ctx, "successor")
			if got.State != FactUnproven || got.Witness == "" || got.Date != "" {
				t.Fatalf("got %+v, want unproven with a witness and no date", got)
			}
		})
	}
}

func TestUTCDay(t *testing.T) {
	tests := []struct{ iso, want string }{
		{"2024-02-15T09:00:00+00:00", "2024-02-15"},
		{"2024-02-15T23:30:00-05:00", "2024-02-16"},
		{"2024-02-16T01:00:00+09:00", "2024-02-15"},
	}
	for _, tc := range tests {
		if got, err := utcDay(tc.iso); err != nil || got != tc.want {
			t.Errorf("utcDay(%q) = %q, %v; want %q", tc.iso, got, err, tc.want)
		}
	}
	if got, err := utcDay("yesterday"); err == nil {
		t.Errorf("utcDay accepted a non-date: %q", got)
	}
}

// TestHistory_Closed pins SI-270's closed date: the committer date of the
// landing of specstate's Closed baseline, never invented from a shallow
// history (review a X-1) or an unresolved default branch.
func TestHistory_Closed(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	repo := scenario.Build(t, "accepted")
	recs := mustRead(t, CommitTree{Root: repo.Dir, Commit: "main"})
	shallow := fixturegit.ShallowClone(t, &fixturegit.Repo{Dir: repo.Dir}, 1)
	unresolved := scenario.Build(t, "accepted")
	gitIn(t, unresolved.Dir, "update-ref", "-d", "refs/remotes/origin/main")
	tests := []struct {
		name, dir, spec string
		want            Fact
		witness         string // unproven: the witness's required substring
	}{
		{"closed feature: the archive move, not its acceptance", repo.Dir, "closed-feature", Fact{State: FactProven, Commit: repo.Base[2], Date: "2024-01-10"}, ""},
		{"closed story: the archive move, by its committer date", repo.Dir, "closed-story", Fact{State: FactProven, Commit: repo.Base[2], Date: "2024-01-10"}, ""},
		{"an accepted, unclosed spec", repo.Dir, "other-feature", Fact{State: FactAbsent}, ""},
		{"shallow history", shallow, "closed-feature", Fact{State: FactUnproven}, "shallow history"},
		{"no default branch", unresolved.Dir, "closed-feature", Fact{State: FactUnproven}, "no default branch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewHistory(ctx, tc.dir).Closed(ctx, recs.Specs[tc.spec])
			witnessOK := strings.Contains(got.Witness, tc.witness) && (tc.witness == "") == (got.Witness == "")
			if got.State != tc.want.State || got.Commit != tc.want.Commit || got.Date != tc.want.Date || !witnessOK {
				t.Fatalf("got %+v, want %+v with witness containing %q", got, tc.want, tc.witness)
			}
		})
	}
}

// TestHistory_Establishment pins SI-270's in-force rule: the new-replacement
// match evaluated on the acceptance commit's tree.
func TestHistory_Establishment(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	accepted := scenario.Build(t, "accepted")
	noBranchRepo := func(t *testing.T, dir string) {
		gitIn(t, dir, "update-ref", "-d", "refs/remotes/origin/main")
	}
	// unreadable drops one record blob of the acceptance commit from the
	// object store, so reading that commit's tree fails.
	unreadable := func(t *testing.T, dir string) {
		oid := gitOut(t, dir, "rev-parse", "main:.verdi/conflicts/successor-closed-story.md")
		if err := os.Remove(filepath.Join(dir, ".git", "objects", oid[:2], oid[2:])); err != nil {
			t.Fatal(err)
		}
	}
	// acceptBroken and acceptOddName accept design/successor together with
	// one more record: an undecodable spec (review b I-1's witness), or a
	// conflict under a name git quotes whose id disagrees with it (review a
	// I-1's witness).
	acceptBroken := func(t *testing.T, dir string) {
		acceptRecord(t, dir, ".verdi/specs/active/zz-broken/spec.md", "---\nid: [\n---\n")
	}
	acceptOddName := func(t *testing.T, dir string) {
		raw, err := os.ReadFile(filepath.Join(dir, ".verdi/conflicts/successor-closed-feature.md"))
		if err != nil {
			t.Fatal(err)
		}
		acceptRecord(t, dir, ".verdi/conflicts/successor-closed-feature-2é.md", string(raw))
	}
	tests := []struct {
		name, scenario, successor string
		prep                      func(*testing.T, string)

		object artifact.Ref
		want   Establishment // Detail: its required prefix
	}{
		{"in force", "", "successor", nil, obj("closed-feature", "dc-1"), Establishment{Commit: accepted.Steps[1], Date: "2024-02-15"}},
		{"in force, criterion", "", "successor", nil, obj("closed-story", "ac-1"), Establishment{Commit: accepted.Steps[1], Date: "2024-02-15"}},
		{"no edge to the object at acceptance", "", "successor", nil, obj("closed-feature", "ac-1"), Establishment{Reason: ReasonEstablisherNotInForce, Detail: "spec/successor carries no edge to spec/closed-feature#ac-1"}},
		{"not accepted", "proposed", "successor", nil, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonEstablisherNotAccepted}},
		{"records did not match at acceptance", "unrelated-accepted", "unrelated", nil, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonEstablisherNotInForce, Detail: "the object spec/closed-feature#dc-1 is already superseded by spec/successor (conflict/successor-closed-feature)"}},
		{"acceptance unproven: no default branch", "accepted", "successor", noBranchRepo, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonAcceptanceUnproven, Detail: noBranch}},
		{"acceptance unproven: a record fails decode at acceptance", "proposed", "successor", acceptBroken, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonAcceptanceUnproven, Detail: "records do not decode at the acceptance commit: .verdi/specs/active/zz-broken/spec.md: "}},
		{"acceptance unproven: a record under a quoted name fails at acceptance", "proposed", "successor", acceptOddName, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonAcceptanceUnproven, Detail: "records do not decode at the acceptance commit: .verdi/conflicts/successor-closed-feature-2é.md: id conflict/successor-closed-feature disagrees"}},
		{"acceptance unproven: an unreadable acceptance commit", "accepted", "successor", unreadable, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonAcceptanceUnproven, Detail: "objsupersede: reading .verdi/conflicts/successor-closed-story.md: "}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := accepted.Dir
			if tc.scenario != "" {
				dir = scenario.Build(t, tc.scenario).Dir
			}
			if tc.prep != nil {
				tc.prep(t, dir)
			}
			got := NewHistory(ctx, dir).Establishment(ctx, tc.successor, tc.object)
			if got.Reason != tc.want.Reason || got.Commit != tc.want.Commit || got.Date != tc.want.Date ||
				!strings.HasPrefix(got.Detail, tc.want.Detail) || (tc.want.Detail == "") != (got.Detail == "") {
				t.Fatalf("got %+v, want %+v (Detail as a prefix)", got, tc.want)
			}
		})
	}
}

// acceptRecord commits one more record on design/successor and accepts
// that branch into main with a --no-ff merge.
func acceptRecord(t *testing.T, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "--no-verify", "-m", "Add "+path)
	gitIn(t, dir, "checkout", "-q", "main")
	gitIn(t, dir, "merge", "-q", "--no-ff", "--no-verify", "-m", "Accept spec/successor", "design/successor")
	gitIn(t, dir, "update-ref", "refs/remotes/origin/main", "main")
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// TestEvaluate_Scenarios drives Evaluate over built scenario stores with the
// real history: §8's proposed, carried, and unrelated-reuse paths.
func TestEvaluate_Scenarios(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	chain := scenario.Build(t, "chain")
	tests := []struct {
		name, scenario, branch, spec, dc string
		want                             string
	}{
		{"proposed successor", "proposed", "", "successor", "dc-1", "records match; takes effect when spec/successor is accepted"},
		{"S2 carries S1's replacement", "chain", "design/successor-v2", "successor-v2", "dc-1", "carries the replacement established by spec/successor (conflict/successor-closed-feature, since 2024-02-15)"},
		{"S3 amends and still carries it", "chain", "design/successor-v3", "successor-v3", "dc-1", "carries the replacement established by spec/successor (conflict/successor-closed-feature, since 2024-02-15)"},
		{"S3 amends the closed criterion's replacement and still carries it", "chain", "design/successor-v3", "successor-v3", "dc-2", "carries the replacement established by spec/successor (conflict/successor-closed-story, since 2024-02-15)"},
		{"S1 was not in force for T at acceptance: a sibling edge had no conflict", "chain-not-in-force", "", "successor-v2", "dc-1", "spec/successor's supersession was not in force at its acceptance: no conflict challenges spec/closed-feature#ac-1"},
		{"the same S1 was in force for another closed spec", "chain-not-in-force", "", "successor-v2", "dc-2", "carries the replacement established by spec/successor (conflict/successor-closed-story, since 2024-02-15)"},
		{"unrelated reuse", "unrelated", "", "unrelated", "dc-1", "the object spec/closed-feature#dc-1 is already superseded by spec/successor (conflict/successor-closed-feature)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := chain.Dir
			if tc.scenario != "chain" {
				dir = scenario.Build(t, tc.scenario).Dir
			}
			if tc.branch != "" {
				gitIn(t, dir, "checkout", "-q", tc.branch)
			}
			res, err := Evaluate(ctx, mustRead(t, WorkTree{Root: dir}), tc.spec, NewHistory(ctx, dir))
			if err != nil {
				t.Fatal(err)
			}
			got, err := result(t, res, tc.dc).Text()
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}
