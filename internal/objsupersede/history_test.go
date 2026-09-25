package objsupersede

import (
	"context"
	"os/exec"
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
		{"accepted by merge; the in-place edit does not move it", chain.Dir, "successor", Fact{State: FactProven, Commit: chain.Steps[1], Date: "2024-02-15"}},
		{"a revision", chain.Dir, "successor-v2", Fact{State: FactProven, Commit: chain.Steps[4], Date: "2024-03-15"}},
		{"archive zone", chain.Dir, "closed-feature", Fact{State: FactProven, Commit: chain.Base[1], Date: "2024-01-01"}},
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

func TestHistory_Closed(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	repo := scenario.Build(t, "accepted")
	recs := mustRead(t, CommitTree{Root: repo.Dir, Commit: "main"})
	h := NewHistory(ctx, repo.Dir)
	if got, want := h.Closed(ctx, recs.Specs["closed-feature"]), (Fact{State: FactProven, Commit: repo.Base[1], Date: "2024-01-01"}); got != want {
		t.Errorf("closed-feature: got %+v, want %+v", got, want)
	}
	if got := h.Closed(ctx, recs.Specs["other-feature"]); got.State != FactAbsent {
		t.Errorf("an accepted, unclosed spec: got %+v", got)
	}
	gitIn(t, repo.Dir, "update-ref", "-d", "refs/remotes/origin/main")
	if got := NewHistory(ctx, repo.Dir).Closed(ctx, recs.Specs["closed-feature"]); got.State != FactUnproven || got.Witness == "" {
		t.Errorf("no default branch: got %+v", got)
	}
}

// TestHistory_Establishment pins SI-270's in-force rule: the new-replacement
// match evaluated on the acceptance commit's tree.
func TestHistory_Establishment(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	accepted := scenario.Build(t, "accepted")
	tests := []struct {
		name, scenario, successor string
		object                    artifact.Ref
		want                      Establishment
	}{
		{"in force", "", "successor", obj("closed-feature", "dc-1"), Establishment{Commit: accepted.Steps[1], Date: "2024-02-15"}},
		{"in force, criterion", "", "successor", obj("closed-story", "ac-1"), Establishment{Commit: accepted.Steps[1], Date: "2024-02-15"}},
		{"no edge to the object at acceptance", "", "successor", obj("closed-feature", "ac-1"), Establishment{Reason: ReasonEstablisherNotInForce, Detail: "spec/successor carries no edge to spec/closed-feature#ac-1"}},
		{"not accepted", "proposed", "successor", obj("closed-feature", "dc-1"), Establishment{Reason: ReasonEstablisherNotAccepted}},
		{"records did not match at acceptance", "unrelated-accepted", "unrelated", obj("closed-feature", "dc-1"), Establishment{Reason: ReasonEstablisherNotInForce, Detail: "the object spec/closed-feature#dc-1 is already superseded by spec/successor (conflict/successor-closed-feature)"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := accepted.Dir
			if tc.scenario != "" {
				dir = scenario.Build(t, tc.scenario).Dir
			}
			if got := NewHistory(ctx, dir).Establishment(ctx, tc.successor, tc.object); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestEvaluate_Scenarios drives Evaluate over built scenario stores with the
// real history: §8's proposed, carried, and unrelated-reuse paths.
func TestEvaluate_Scenarios(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	chain := scenario.Build(t, "chain")
	tests := []struct {
		name, scenario, branch, spec string
		want                         string
	}{
		{"proposed successor", "proposed", "", "successor", "records match; takes effect when spec/successor is accepted"},
		{"S2 carries S1's replacement", "chain", "design/successor-v2", "successor-v2", "carries the replacement established by spec/successor (conflict/successor-closed-feature, since 2024-02-15)"},
		{"S3 amends and still carries it", "chain", "design/successor-v3", "successor-v3", "carries the replacement established by spec/successor (conflict/successor-closed-feature, since 2024-02-15)"},
		{"unrelated reuse", "unrelated", "", "unrelated", "the object spec/closed-feature#dc-1 is already superseded by spec/successor (conflict/successor-closed-feature)"},
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
			got, err := result(t, res, "dc-1").Text()
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}
