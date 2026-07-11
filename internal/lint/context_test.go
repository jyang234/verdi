package lint

import "testing"

func TestContext_EnforceDraftGate(t *testing.T) {
	cases := []struct {
		name string
		ctx  Context
		want bool
	}{
		{"unknown default branch", Context{}, false},
		{"on the default branch", Context{DefaultBranch: "main", CurrentBranch: "main"}, true},
		{"on a design branch", Context{DefaultBranch: "main", CurrentBranch: "feature/x"}, false},
		{"MR targeting the default branch, in CI", Context{DefaultBranch: "main", CurrentBranch: "feature/x", TargetBranch: "main", InCI: true}, true},
		{"MR target matches but not in CI (untrusted signal)", Context{DefaultBranch: "main", CurrentBranch: "feature/x", TargetBranch: "main", InCI: false}, false},
		{"MR targeting a different branch", Context{DefaultBranch: "main", CurrentBranch: "feature/x", TargetBranch: "develop", InCI: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ctx.EnforceDraftGate(); got != tc.want {
				t.Fatalf("EnforceDraftGate() = %v, want %v (ctx=%+v)", got, tc.want, tc.ctx)
			}
		})
	}
}
