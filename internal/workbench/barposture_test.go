package workbench

import (
	"fmt"
	stdhtml "html"
	"maps"
	"slices"
	"strings"
	"testing"
)

// legacyASDPosture is writeASDPosture exactly as it stood before the top
// bar's facts model (base 0e888109, boardshellrender.go), rendering from
// the projection and the asdView. It is the reference the byte-identity
// test below holds the facts-driven row to: the row's markup is
// unchanged, so the facts carry everything the row shows
// (spec/chrome-and-tokens-v2, SI-323 (3)).
//
// noWorktree marks a remote-only branch's sealed wall, whose row changes
// in exactly two facts, the one visible change SI-323 (1) authorizes (as
// refined at c32f185c): the working tree reads "unproven" (data-dirty
// "unproven") instead of a false "clean", and the base digest reads
// "unproven" instead of empty.
func legacyASDPosture(p *BoardProjection, asd *asdView, noWorktree bool) string {
	var b strings.Builder
	esc := stdhtml.EscapeString
	b.WriteString(`<section class="asd-posture" id="asd-posture" data-testid="asd-posture" aria-label="Repository posture">`)
	b.WriteString(`<span class="board-mode-tag board-mode-tag--` + esc(string(p.Mode)) + `">` + esc(modeStampLabel(p)) + `</span>`)
	if badge := terminalStatusBadge(p.Status); badge != "" {
		label := badge
		if p.StatusLabel != "" {
			label = p.StatusLabel
		}
		b.WriteString(`<span class="badge badge-` + esc(badge) + ` board-status-badge" data-testid="board-status-badge">` + esc(label) + `</span>`)
	}
	b.WriteString(`<span class="asd-posture-bytes" data-testid="asd-posture-bytes" data-state="` + esc(asd.StateFormal) + `">displayed bytes: ` + esc(postureByteWord(asd.StateFormal)) + ` <span class="asd-posture-formal">(` + esc(asd.StateFormal) + `)</span></span>`)
	dirtyWord, dirtyState := "clean", "clean"
	if asd.Dirty {
		dirtyWord, dirtyState = "uncommitted changes", "dirty"
	}
	if noWorktree {
		dirtyWord, dirtyState = "unproven", "unproven"
	}
	b.WriteString(`<span class="asd-posture-tree" data-testid="asd-posture-tree" data-dirty="` + dirtyState + `">working tree: ` + dirtyWord + `</span>`)
	b.WriteString(`<button type="button" class="asd-refresh" id="asd-refresh" data-testid="asd-refresh">Refresh</button>`)
	b.WriteString(`<details class="readiness-tech asd-posture-tech" data-testid="asd-posture-tech"><summary>Repository details</summary><dl class="readiness-tech-facts">`)
	writeReadinessFact(&b, "Checkout", asd.Checkout)
	writeReadinessFact(&b, "Branch", asd.Branch)
	writeReadinessFact(&b, "Worktree HEAD", legacyOrUnproven(asd.WorktreeHead))
	writeReadinessFact(&b, "Accepted branch", legacyOrUnproven(asd.DefaultBranch))
	writeReadinessFact(&b, "Accepted HEAD", legacyOrUnproven(asd.AcceptedHead))
	if asd.AheadBehindKnown {
		writeReadinessFact(&b, "Ahead/behind", fmt.Sprintf("%d ahead, %d behind %s", asd.Ahead, asd.Behind, asd.DefaultBranch))
		if asd.Ahead > 0 && asd.Behind > 0 {
			writeReadinessFact(&b, "Divergence", "diverged: both sides carry commits the other lacks")
		}
	} else {
		writeReadinessFact(&b, "Ahead/behind", "unproven: the accepted branch could not be resolved")
	}
	digest := asd.BaseDigest
	if noWorktree {
		digest = "unproven"
	}
	writeReadinessFact(&b, "Base digest", digest)
	b.WriteString(`</dl></details>`)
	b.WriteString(`</section>`)
	return b.String()
}

// TestASDPosture_RendersFromBarFactsByteIdentically: today's posture row,
// rendered from the bar's facts, is byte-identical to the row rendered
// from the projection and asdView — over synthetic postures covering
// every branch of the row, and over the wall fixtures' real loads (an
// authoring wall, a sealed record on the default branch, and a
// remote-only branch's sealed wall, which differs in exactly its working
// tree and base digest, now disclosed-unproven).
func TestASDPosture_RendersFromBarFactsByteIdentically(t *testing.T) {
	type wallCase struct {
		name string
		p    *BoardProjection
		asd  *asdView
		// noWorktree marks the remote-only sealed wall (legacyASDPosture).
		noWorktree bool
	}
	var cases []wallCase
	postures := map[string]branchPosture{
		"proven, dirty, ahead":   {Checkout: "/srv/a<b>", Branch: "design/s", DefaultBranch: "main", Dirty: true, WorktreeHead: "h1", AcceptedHead: "a1", Ahead: 2, AheadBehindKnown: true},
		"diverged":               {Checkout: "/c", Branch: "design/s", DefaultBranch: "trunk", WorktreeHead: "h", AcceptedHead: "a", Ahead: 1, Behind: 5, AheadBehindKnown: true},
		"behind only":            {Checkout: "/c", Branch: "main", DefaultBranch: "main", WorktreeHead: "h", AcceptedHead: "a", Behind: 2, AheadBehindKnown: true},
		"default branch unknown": {Checkout: "/c", Branch: "x", WorktreeHead: "h"},
		"detached, nothing":      {Checkout: "/c"},
	}
	projections := map[string]*BoardProjection{
		"authoring":            {Spec: "s", Mode: modeAuthoring, Status: "draft"},
		"review":               {Spec: "s", Mode: modeReview, Status: "draft"},
		"sealed":               {Spec: "s", Mode: modeReadOnly, Status: "accepted-pending-build"},
		"not accepted":         {Spec: "s", Mode: modeReadOnly, Status: "draft"},
		"unproven":             {Spec: "s", Mode: modeReadOnly, Status: "unproven"},
		"superseded":           {Spec: "s", Mode: modeReadOnly, Status: "superseded"},
		"superseded, renamed":  {Spec: "s", Mode: modeReadOnly, Status: "superseded", StatusLabel: "Replaced & gone"},
		"closed spike, labels": {Spec: "s", Mode: modeReadOnly, Status: "closed", Class: "story", Spike: true, ClassLabel: "Probe"},
	}
	states := []string{"proposed", "draft", "accepted-pending-build", "closed", "superseded", "unproven", "", `odd"state`}
	// Paired in sorted order, so every run renders the same pairs of
	// projection, posture, and state.
	i := 0
	for _, pn := range slices.Sorted(maps.Keys(projections)) {
		for _, bn := range slices.Sorted(maps.Keys(postures)) {
			state := states[i%len(states)]
			i++
			cases = append(cases, wallCase{name: pn + " / " + bn + " / " + state, p: projections[pn], asd: &asdView{branchPosture: postures[bn], StateFormal: state, BaseDigest: "sha256:" + pn}})
		}
	}

	// The wall fixtures' real loads.
	for _, f := range []struct {
		name string
		root func(*testing.T) string
	}{
		{"authoring wall", newBoardFixture},
		{"sealed record on main", func(t *testing.T) string { return newStatuslessBoardFixture(t, false) }},
	} {
		root := f.root(t)
		proj, _, asd, err := (&boardSpecServer{root: root}).loadASD(t.Context(), boardFixtureName)
		if err != nil {
			t.Fatalf("%s: loadASD: %v", f.name, err)
		}
		cases = append(cases, wallCase{name: f.name, p: proj, asd: asd})
	}
	branchRoot := newBranchBoardFixture(t)
	b := newBranchBoards(branchRoot, Deps{}, &boardSpecServer{root: branchRoot})
	proj, _, err := b.loadSealed(t.Context(), "design/remote-only", "origin/design/remote-only", "remote-spec")
	if err != nil {
		t.Fatalf("loadSealed: %v", err)
	}
	cases = append(cases, wallCase{name: "remote-only sealed wall", p: proj, asd: sealedASDView("design/remote-only", "origin/design/remote-only", proj), noWorktree: true})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := legacyASDPosture(tc.p, tc.asd, tc.noWorktree)
			bar := specBarFacts(tc.p, tc.asd)
			if got := asdPostureHTML(&bar); got != want {
				t.Fatalf("posture row from the bar's facts differs from today's row\n got: %s\nwant: %s", got, want)
			}
			region := renderBoardRegion(tc.p, &boardGitState{}, withTestASDDefaults(tc.asd))
			if !strings.Contains(region, want) {
				t.Fatalf("the board region does not carry today's posture row byte for byte\nregion: %s\n  want: %s", region, want)
			}
		})
	}
}

// legacyOrUnproven is the row's former orUnproven, kept with its legacy
// renderer: the model now carries that word (barfacts.go's valueOr).
func legacyOrUnproven(v string) string {
	if v == "" {
		return "unproven"
	}
	return v
}

// withTestASDDefaults fills the render-only maps a synthetic asdView
// leaves nil, so the whole region renders around its posture.
func withTestASDDefaults(asd *asdView) *asdView {
	v := *asd
	base := testASDView()
	if v.NextIDs == nil {
		v.NextIDs = base.NextIDs
	}
	if v.ObjectAnchors == nil {
		v.ObjectAnchors, v.ObjectEvidence, v.StickySlugs, v.EdgeFacts = base.ObjectAnchors, base.ObjectEvidence, base.StickySlugs, base.EdgeFacts
	}
	if v.SlugPattern == "" {
		v.SlugPattern = base.SlugPattern
	}
	return &v
}
