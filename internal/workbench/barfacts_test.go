package workbench

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// checkBarFacts asserts the model's own invariants on f: every fact is
// proven or carries its reason, an unproven fact never prints a value,
// and a spec whose facts could not be computed carries nothing else.
func checkBarFacts(t *testing.T, f barFacts) {
	t.Helper()
	p := f.Posture
	facts := map[string]barFact{
		"checkout": p.Checkout, "branch": p.Branch, "tree": p.Tree.barFact,
		"worktree head": p.WorktreeHead, "accepted branch": p.AcceptedBranch,
		"accepted head": p.AcceptedHead, "ahead/behind": p.AheadBehind,
	}
	if p.Divergence != nil {
		facts["divergence"] = *p.Divergence
	}
	if p.BaseDigest != nil {
		facts["base digest"] = *p.BaseDigest
	}
	for name, fact := range facts {
		if fact.Unproven != "" && !strings.HasPrefix(fact.Text, unprovenWord) {
			t.Errorf("%s is disclosed-unproven (%s) but prints %q", name, fact.Unproven, fact.Text)
		}
		if fact.Unproven == "" && fact.Text == unprovenWord {
			t.Errorf("%s prints %q with no reason", name, unprovenWord)
		}
	}
	switch p.Tree.State {
	case "clean", "dirty":
		if p.Tree.Unproven != "" {
			t.Errorf("tree state %q carries an unproven reason", p.Tree.State)
		}
	case unprovenWord:
		if p.Tree.Unproven == "" {
			t.Error("tree state unproven carries no reason")
		}
	default:
		t.Errorf("tree state %q is none of clean, dirty, unproven", p.Tree.State)
	}
	if f.Spec != nil && f.Spec.Unproven != "" && (f.Spec.Class != "" || f.Spec.Mode != "" || f.Spec.Bytes != (barBytes{})) {
		t.Errorf("spec facts are disclosed-unproven (%s) yet carry values: %+v", f.Spec.Unproven, f.Spec)
	}
}

func TestSpecBarFacts_FromThePostureModel(t *testing.T) {
	proven := branchPosture{
		Checkout: "/srv/store", Branch: "design/s", DefaultBranch: "main", Dirty: true,
		WorktreeHead: "aaa111", AcceptedHead: "bbb222", Ahead: 2, AheadBehindKnown: true,
	}
	digest := func(s string) *barFact { f := provenFact(s); return &f }
	for _, tc := range []struct {
		name string
		p    *BoardProjection
		asd  *asdView
		want barFacts
	}{
		{
			name: "authoring wall, every fact proven",
			p:    &BoardProjection{Spec: "s", Title: "S title", Mode: modeAuthoring, Status: "draft", Class: "story"},
			asd:  &asdView{branchPosture: proven, StateFormal: "proposed", BaseDigest: "sha256:abc"},
			want: barFacts{
				Title: "S title",
				Spec:  &barSpec{Name: "s", Class: "story", ClassLabel: "story", Mode: "authoring", ModeLabel: "authoring · live wall", Bytes: barBytes{State: "proposed", Word: "proposed"}},
				Posture: barPosture{
					Checkout: provenFact("/srv/store"), Branch: provenFact("design/s"),
					Tree:         barTree{State: "dirty", barFact: provenFact("uncommitted changes")},
					WorktreeHead: provenFact("aaa111"), AcceptedBranch: provenFact("main"), AcceptedHead: provenFact("bbb222"),
					AheadBehind: provenFact("2 ahead, 0 behind main"), BaseDigest: digest("sha256:abc"),
				},
			},
		},
		{
			name: "superseded sealed record with a renamed status and a spike label",
			p:    &BoardProjection{Spec: "s", Title: "S", Mode: modeReadOnly, Status: "superseded", StatusLabel: "Replaced", Class: "story", Spike: true, ClassLabel: "Probe"},
			asd: &asdView{branchPosture: branchPosture{
				Checkout: "/c", Branch: "main", DefaultBranch: "main", WorktreeHead: "h", AcceptedHead: "a", Ahead: 3, Behind: 4, AheadBehindKnown: true,
			}, StateFormal: "superseded", BaseDigest: "sha256:d"},
			want: barFacts{
				Title: "S",
				Spec: &barSpec{Name: "s", Class: "spike", ClassLabel: "Probe", Mode: "readonly", ModeLabel: "read-only · sealed record",
					StatusBadge: "superseded", StatusBadgeLabel: "Replaced", Bytes: barBytes{State: "superseded", Word: "superseded"}},
				Posture: barPosture{
					Checkout: provenFact("/c"), Branch: provenFact("main"),
					Tree:         barTree{State: "clean", barFact: provenFact("clean")},
					WorktreeHead: provenFact("h"), AcceptedBranch: provenFact("main"), AcceptedHead: provenFact("a"),
					AheadBehind: provenFact("3 ahead, 4 behind main"),
					Divergence:  digest("diverged: both sides carry commits the other lacks"), BaseDigest: digest("sha256:d"),
				},
			},
		},
		{
			name: "nothing resolvable: every unresolved fact disclosed with its reason",
			p:    &BoardProjection{Spec: "u", Title: "U", Mode: modeReadOnly, Status: "unproven"},
			asd: &asdView{branchPosture: branchPosture{
				Checkout: "/c", Branch: "feature-x",
				defaultBranchWhy: "the default branch could not be resolved", acceptedHeadWhy: "the default branch could not be resolved",
				worktreeHeadWhy: "the worktree HEAD could not be resolved: unborn",
			}, StateFormal: "unproven", BaseDigest: "sha256:u"},
			want: barFacts{
				Title: "U",
				Spec:  &barSpec{Name: "u", Mode: "readonly", ModeLabel: "read-only · lifecycle unproven", Bytes: barBytes{State: "unproven", Word: "unproven"}},
				Posture: barPosture{
					Checkout: provenFact("/c"), Branch: provenFact("feature-x"),
					Tree:           barTree{State: "clean", barFact: provenFact("clean")},
					WorktreeHead:   unprovenFact("the worktree HEAD could not be resolved: unborn"),
					AcceptedBranch: unprovenFact("the default branch could not be resolved"),
					AcceptedHead:   unprovenFact("the default branch could not be resolved"),
					AheadBehind:    barFact{Text: "unproven: the accepted branch could not be resolved", Unproven: "the accepted branch could not be resolved"},
					BaseDigest:     digest("sha256:u"),
				},
			},
		},
		{
			name: "a posture with no recorded reasons still discloses one",
			p:    &BoardProjection{Spec: "r", Title: "R", Mode: modeReview, Status: "draft", Class: "feature"},
			asd:  &asdView{branchPosture: branchPosture{Checkout: "/c", Branch: "design/r"}, StateFormal: "proposed"},
			want: barFacts{
				Title: "R",
				Spec:  &barSpec{Name: "r", Class: "feature", ClassLabel: "feature", Mode: "review", ModeLabel: "review · mirror of the MR", Bytes: barBytes{State: "proposed", Word: "proposed"}},
				Posture: barPosture{
					Checkout: provenFact("/c"), Branch: provenFact("design/r"),
					Tree:           barTree{State: "clean", barFact: provenFact("clean")},
					WorktreeHead:   unprovenFact("the worktree HEAD could not be resolved"),
					AcceptedBranch: unprovenFact("the default branch could not be resolved"),
					AcceptedHead:   unprovenFact("the accepted HEAD could not be resolved"),
					AheadBehind:    barFact{Text: "unproven: the accepted branch could not be resolved", Unproven: "the accepted branch could not be resolved"},
					BaseDigest:     digest(""),
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := specBarFacts(tc.p, tc.asd)
			checkBarFacts(t, got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("specBarFacts =\n%+v\n%+v\nwant\n%+v\n%+v", got, *got.Spec, tc.want, *tc.want.Spec)
			}
		})
	}
}

// TestSpecBarFacts_ServedWall builds the bar's facts for real served walls
// — an authoring wall on its design branch and the sealed record on the
// default branch — from the wall's own load (loadASD), and checks each
// fact against Git directly.
func TestSpecBarFacts_ServedWall(t *testing.T) {
	for _, tc := range []struct {
		name, branch, mode, bytes, aheadBehind string
		root                                   func(*testing.T) string
	}{
		{name: "authoring", root: newBoardFixture, branch: "design/" + boardFixtureName, mode: "authoring", bytes: "proposed", aheadBehind: "1 ahead, 0 behind main"},
		{name: "sealed record on main", root: func(t *testing.T) string { return newStatuslessBoardFixture(t, false) }, branch: "main", mode: "readonly", bytes: "accepted", aheadBehind: "0 ahead, 0 behind main"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.root(t)
			s := &boardSpecServer{root: root}
			proj, _, asd, err := s.loadASD(t.Context(), boardFixtureName)
			if err != nil {
				t.Fatalf("loadASD: %v", err)
			}
			got := specBarFacts(proj, asd)
			checkBarFacts(t, got)
			if got.Title != "Refi test flow" || got.Spec == nil || got.Spec.Name != boardFixtureName || got.Spec.Class != "feature" || got.Spec.Mode != tc.mode || got.Spec.Bytes.Word != tc.bytes {
				t.Fatalf("spec facts = %q %+v", got.Title, got.Spec)
			}
			p := got.Posture
			if p.Checkout.Text != root || p.Branch.Text != tc.branch || p.Tree.State != "clean" {
				t.Fatalf("checkout/branch/tree = %v / %v / %v", p.Checkout, p.Branch, p.Tree)
			}
			if p.WorktreeHead.Text != gitOut(t, root, "rev-parse", "HEAD") || p.AcceptedHead.Text != gitOut(t, root, "rev-parse", "main") || p.AcceptedBranch.Text != "main" {
				t.Fatalf("heads = %v / %v / %v", p.WorktreeHead, p.AcceptedHead, p.AcceptedBranch)
			}
			if p.AheadBehind.Text != tc.aheadBehind || p.Divergence != nil {
				t.Fatalf("ahead/behind = %v, divergence %v", p.AheadBehind, p.Divergence)
			}
			if p.BaseDigest == nil || p.BaseDigest.Text != asd.BaseDigest || asd.BaseDigest == "" {
				t.Fatalf("base digest = %v, want the wall's %q", p.BaseDigest, asd.BaseDigest)
			}
		})
	}
}

// TestSpecBarFacts_SealedWall: a remote-only design branch's sealed wall
// (sealedASDView) has no working tree, so its heads, accepted branch,
// ahead/behind, working-tree state, and base digest are disclosed-unproven
// with the reason, never omitted and never a false "clean" (SI-323 (1), as
// refined at c32f185c).
func TestSpecBarFacts_SealedWall(t *testing.T) {
	root := newBranchBoardFixture(t)
	b := newBranchBoards(root, Deps{}, &boardSpecServer{root: root})
	proj, _, err := b.loadSealed(t.Context(), "design/remote-only", "origin/design/remote-only", "remote-spec")
	if err != nil {
		t.Fatalf("loadSealed: %v", err)
	}
	got := specBarFacts(proj, sealedASDView("design/remote-only", "origin/design/remote-only", proj))
	checkBarFacts(t, got)
	p := got.Posture
	if !strings.HasPrefix(p.Checkout.Text, "origin/design/remote-only (remote-tracking ref") || p.Branch.Text != "design/remote-only" {
		t.Fatalf("checkout/branch = %v / %v", p.Checkout, p.Branch)
	}
	if p.BaseDigest == nil {
		t.Fatal("the sealed wall's base digest is omitted, want disclosed-unproven")
	}
	for name, f := range map[string]barFact{"worktree head": p.WorktreeHead, "accepted branch": p.AcceptedBranch, "accepted head": p.AcceptedHead, "working tree": p.Tree.barFact, "base digest": *p.BaseDigest} {
		if f.Unproven == "" || !strings.Contains(f.Unproven, "remote-only") {
			t.Errorf("%s = %v, want disclosed-unproven naming the remote-only render", name, f)
		}
	}
	if p.AheadBehind.Unproven == "" {
		t.Errorf("ahead/behind = %v, want disclosed-unproven", p.AheadBehind)
	}
	if got.Spec == nil || got.Spec.Mode != "readonly" || got.Spec.Bytes.State != "draft" || got.Spec.Bytes.Word != "proposed" {
		t.Fatalf("spec facts = %+v", got.Spec)
	}
	if p.Tree.State != unprovenWord || p.Tree.Text != unprovenWord || p.BaseDigest.Text != unprovenWord {
		t.Fatalf("tree / base digest = %+v / %v, want both disclosed-unproven", p.Tree, p.BaseDigest)
	}
}

func TestBranchBarFacts(t *testing.T) {
	t.Run("a checkout's branch-level posture", func(t *testing.T) {
		root := newBoardFixture(t)
		got := branchBarFacts(t.Context(), root, "Workbench")
		checkBarFacts(t, got)
		if got.Title != "Workbench" || got.Spec != nil || got.Posture.BaseDigest != nil {
			t.Fatalf("title/spec/base digest = %q / %+v / %v, want no spec facts off a spec page", got.Title, got.Spec, got.Posture.BaseDigest)
		}
		p := got.Posture
		want := barPosture{
			Checkout: provenFact(root), Branch: provenFact("design/" + boardFixtureName),
			Tree:         barTree{State: "clean", barFact: provenFact("clean")},
			WorktreeHead: provenFact(gitOut(t, root, "rev-parse", "HEAD")), AcceptedBranch: provenFact("main"),
			AcceptedHead: provenFact(gitOut(t, root, "rev-parse", "main")), AheadBehind: provenFact("1 ahead, 0 behind main"),
		}
		if !reflect.DeepEqual(p, want) {
			t.Fatalf("posture =\n%+v\nwant\n%+v", p, want)
		}
	})

	t.Run("equals the wall's posture on the same checkout (one path, dc-2)", func(t *testing.T) {
		root := newBoardFixture(t)
		s := &boardSpecServer{root: root}
		proj, _, asd, err := s.loadASD(t.Context(), boardFixtureName)
		if err != nil {
			t.Fatalf("loadASD: %v", err)
		}
		wall := specBarFacts(proj, asd).Posture
		wall.BaseDigest = nil
		if got := branchBarFacts(t.Context(), root, "x").Posture; !reflect.DeepEqual(got, wall) {
			t.Fatalf("branch-level posture =\n%+v\nwall's\n%+v", got, wall)
		}
	})

	t.Run("no store root: every fact disclosed-unproven", func(t *testing.T) {
		got := branchBarFacts(t.Context(), "", "Error")
		checkBarFacts(t, got)
		if got.Title != "Error" || got.Posture.Checkout.Unproven == "" || got.Posture.Branch.Unproven == "" || got.Posture.Tree.State != unprovenWord || got.Posture.AcceptedHead.Unproven == "" {
			t.Fatalf("facts = %+v, want every fact disclosed-unproven", got)
		}
	})

	t.Run("a root that is no Git checkout: the checkout proven, the rest disclosed-unproven", func(t *testing.T) {
		root := t.TempDir()
		got := branchBarFacts(t.Context(), root, "Error")
		checkBarFacts(t, got)
		p := got.Posture
		if p.Checkout.Text != root || p.Checkout.Unproven != "" {
			t.Fatalf("checkout = %v, want %s proven", p.Checkout, root)
		}
		if !strings.Contains(p.Branch.Unproven, "Git state could not be read") || p.Tree.State != unprovenWord || p.AheadBehind.Unproven == "" {
			t.Fatalf("posture = %+v, want the Git facts disclosed-unproven with the read failure", p)
		}
	})
}

// gitOut runs git in dir and returns its trimmed output.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}
