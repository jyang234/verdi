package main

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/ritualwitness"
)

// resolvedCutBound bounds one built-binary ritual run below, so a hung
// child fails its own case instead of the package timeout.
const resolvedCutBound = 2 * time.Minute

// TestResolvedBaseCuts_WriteNoUpstream pins ledger SI-341 (6) (backlog
// BL-141): design start, its --supersedes path, and policy adopt, driven as
// the built binary over a fixture with a local bare remote and HEAD on
// another branch, cut their branch at the resolved default branch's commit
// (origin/main), never at HEAD, and the branch's one commit sits directly
// on that commit. Cut at the commit, not at origin/main's name, git sets
// up no upstream: the run leaves the local configuration exactly as it
// found it (no branch.<b>.remote or branch.<b>.merge, which no declaration
// of these rituals admits, SI-325 (7)), and the harness judges no config
// effect outside the declaration.
func TestResolvedBaseCuts_WriteNoUpstream(t *testing.T) {
	bin := buildVerdiBinary(t)
	manifest := map[string]string{".verdi/verdi.yaml": minimalManifestYAML}
	tests := []struct {
		name   string
		ritual string
		branch string
		base   map[string]string
		args   []string
	}{
		{
			name: "design start", ritual: "design_start", branch: "refs/heads/design/resolved-cut",
			base: manifest,
			args: []string{"design", "start", "--kind", "feature", "--name", "resolved-cut", "--defer-statements"},
		},
		{
			name: "design start --supersedes", ritual: "design_start", branch: "refs/heads/design/lockbox-v2",
			base: map[string]string{
				".verdi/verdi.yaml":                   minimalManifestYAML,
				".verdi/specs/active/lockbox/spec.md": lockboxPredecessor,
				".verdi/adr/0099-lockbox-exempt.md":   lockboxExemptADR,
			},
			args: []string{"design", "start", "--supersedes", "spec/lockbox", "--name", "lockbox-v2"},
		},
		{
			name: "policy adopt", ritual: "policy_adopt", branch: "refs/heads/policy/adopt",
			base: manifest,
			args: []string{"policy", "adopt", "--starter"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), resolvedCutBound)
			defer cancel()
			fx := ritualwitness.BuildWith(t, ctx, ritualwitness.SeedClean, tt.base)
			elsewhere := divergeHead(t, fx)
			res := ritualwitness.RunOn(t, ctx, fx, ritualwitness.Binary{Path: bin, Args: tt.args}, ritualDeclaration(t, tt.ritual))
			logVerdicts(t, res)
			requireCleanRun(t, res)

			if got := res.Before.Refs["refs/remotes/origin/main"].Object; got != fx.BaseCommit {
				t.Fatalf("the resolved default branch origin/main is at %s, want the seed commit %s", got, fx.BaseCommit)
			}
			if _, existed := res.Before.Refs[tt.branch]; existed {
				t.Fatalf("%s existed before the run", tt.branch)
			}
			tip, ok := res.After.Refs[tt.branch]
			if !ok {
				t.Fatalf("the run created no %s", tt.branch)
			}
			parents := res.After.Commits[tip.Object].Parents
			if len(parents) != 1 || parents[0] != fx.BaseCommit {
				t.Errorf("%s's commit %s has parents %v, want the resolved default branch's commit %s (HEAD was at %s)", tt.branch, tip.Object, parents, fx.BaseCommit, elsewhere)
			}
			if !reflect.DeepEqual(res.Before.Config, res.After.Config) {
				t.Errorf("the cut changed the local configuration (an upstream, SI-341 (6)):\nbefore %v\nafter  %v", res.Before.Config, res.After.Config)
			}
			for _, v := range res.Verdicts {
				if v.Field == "config" {
					t.Errorf("config verdict %s: the run must leave the configuration untouched", v)
				}
			}
		})
	}
}
