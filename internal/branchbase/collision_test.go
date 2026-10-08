package branchbase

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestCollision (UAT-031, ledger SI-333): a fresh branch collides when it
// exists locally, or as a remote-tracking branch of the remote the base
// resolves from; a local or HEAD-fallback base resolves from no remote; a
// directory that is not a repository is an error.
func TestCollision(t *testing.T) {
	const branch = "feature/widget-story"
	originMain := Resolution{Kind: ResolvedDefault, Ref: "origin/main", BranchName: "main"}
	localMain := Resolution{Kind: ResolvedDefault, Ref: "main", BranchName: "main"}
	headFallback := Resolution{Kind: HeadFallback, Ref: "HEAD"}
	unresolvable := Resolution{Kind: Unresolvable}
	tests := []struct {
		name    string
		seed    []string // a ref to create at HEAD, by full name
		base    Resolution
		want    string
		wantErr bool
	}{
		{"no collision", nil, originMain, "", false},
		{"a local branch", []string{"refs/heads/" + branch}, originMain, "refs/heads/" + branch, false},
		{"the base's remote", []string{"refs/remotes/origin/" + branch}, originMain, "refs/remotes/origin/" + branch, false},
		{"another remote is not the base's", []string{"refs/remotes/upstream/" + branch}, originMain, "", false},
		{"a local base asks no remote", []string{"refs/remotes/origin/" + branch}, localMain, "", false},
		{"the HEAD fallback asks no remote", []string{"refs/remotes/origin/" + branch}, headFallback, "", false},
		{"the HEAD fallback still asks the local branch", []string{"refs/heads/" + branch}, headFallback, "refs/heads/" + branch, false},
		{"an unresolvable base asks no remote", []string{"refs/remotes/origin/" + branch}, unresolvable, "", false},
		{"not a repository", nil, originMain, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tt.wantErr {
				repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "seed"}})
				dir = repo.Dir
				for _, ref := range tt.seed {
					fixturegit.CreateRef(t, dir, ref, repo.Head)
				}
			}
			got, err := Collision(context.Background(), dir, branch, tt.base)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("Collision = %q, %v; want %q (error %v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}
