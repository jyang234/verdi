package workbench

import (
	"strings"
	"testing"
)

// TestBoardSpecPage_SupersededStatusBadge proves spec/feature-supersession-
// state ac-2 on the board surface at BOTH rungs: a superseded spec's terminal
// `status` is legible on its own wall (03 §rung 3: "without consulting
// backlinks"), stamped as a status badge on the board head beside the mode
// tag — the same `.badge-superseded` vocabulary the index list and dex page
// already carry, so status reads the same on every surface that renders a
// spec.
func TestBoardSpecPage_SupersededStatusBadge(t *testing.T) {
	// A superseded feature and a superseded story both stamp the badge — the
	// render is class-agnostic (it reads Status), so both rungs are covered.
	for _, class := range []string{"feature", "story"} {
		t.Run("superseded "+class+" wall stamps the badge", func(t *testing.T) {
			proj := &BoardProjection{
				Spec: "spec/rate-lock", Title: "Rate lock", Mode: modeReadOnly,
				Status: "superseded", Class: class,
				Problem: "borrowers lose a good rate when they pause",
				Outcome: "borrowers can lock a rate for a window",
			}
			out, err := renderBoardSpecPage(proj, &boardGitState{})
			if err != nil {
				t.Fatalf("renderBoardSpecPage: %v", err)
			}
			page := string(out)
			for _, want := range []string{
				`data-testid="board-status-badge"`,
				`class="badge badge-superseded board-status-badge"`,
				`>superseded</span>`,
			} {
				if !strings.Contains(page, want) {
					t.Errorf("superseded %s board head missing %q; got:\n%s", class, want, page)
				}
			}
		})
	}

	// An accepted-pending-build wall must NOT stamp the badge: its lifecycle is
	// already spoken by the mode stamp, and only the terminal `superseded`
	// state gets the badge (ac-2 scope; the `closed` case is deferred by dc-2).
	t.Run("accepted-pending-build wall stamps no status badge", func(t *testing.T) {
		proj := &BoardProjection{
			Spec: "spec/rate-lock-v2", Title: "Rate lock v2", Mode: modeReadOnly,
			Status: "accepted-pending-build", Class: "feature",
			Problem: "p", Outcome: "o",
		}
		out, err := renderBoardSpecPage(proj, &boardGitState{})
		if err != nil {
			t.Fatalf("renderBoardSpecPage: %v", err)
		}
		if strings.Contains(string(out), "board-status-badge") {
			t.Errorf("accepted-pending-build board must not render a status badge; got:\n%s", out)
		}
	})
}
