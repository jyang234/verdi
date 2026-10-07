package workbench

// The workbench top bar's facts (spec/chrome-and-tokens-v2 ac-1, ac-2;
// ledger SI-323 (1), (2)): one typed model of everything the bar states
// about the page it heads, carried by every workbench page's view data.
// Each fact is a proven value or disclosed-unproven with its reason — the
// three-valued semantics today's posture row speaks (its "unproven" word
// for an unresolved value, the unresolved ahead/behind text,
// postureByteWord), moved here, so the row,
// the bar, and the wall's snapshot read one model and nothing re-derives
// it (dc-2).
//
// Two builders fill it, over one shared Git path:
//   - specBarFacts, for a page about one spec (the wall, served or sealed,
//     and its Document page), from the posture model the wall's posture
//     row renders (asdView, sealedASDView);
//   - branchBarFacts, for every other page, from the same gitState and
//     resolveBranchPosture path the wall's posture rides, with no spec and
//     no displayed bytes (SI-323 (1)).

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/gitx"
)

// barFacts is everything the top bar states about one page: its title,
// the spec it is about (nil on a page not about one spec), and the
// checkout's posture.
type barFacts struct {
	Title   string     `json:"title"`
	Spec    *barSpec   `json:"spec,omitempty"`
	Posture barPosture `json:"posture"`
}

// barSpec is what the bar names about the one spec a page is about: its
// name, class, mode, terminal status, and what the displayed bytes are.
type barSpec struct {
	Name string `json:"name"`
	// Unproven, when non-empty, says why the spec's facts could not be
	// computed — a Document page for a spec the wall cannot load — and
	// every field below is then empty: disclosed, never omitted.
	Unproven string `json:"unproven,omitempty"`
	// Class is the class chip's id (the spec's class, or "spike" for a
	// spike story); ClassLabel is the model's display word for it. Both
	// are empty for a spec that declares no class.
	Class      string `json:"class,omitempty"`
	ClassLabel string `json:"classLabel,omitempty"`
	// Mode is the room's id and ModeLabel its stamp's words
	// (modeStampLabel). ModeDisclosure, when non-empty, is the review
	// feed's disclosure: the feed is configured but could not be
	// consulted, so the mode is stated without the review state it would
	// have read (SI-323 (2); the wall's board notice says the same).
	Mode           string `json:"mode,omitempty"`
	ModeLabel      string `json:"modeLabel,omitempty"`
	ModeDisclosure string `json:"modeDisclosure,omitempty"`
	// StatusBadge is the terminal status badge's id (terminalStatusBadge),
	// empty when the posture row draws none; StatusBadgeLabel is its words.
	StatusBadge      string   `json:"statusBadge,omitempty"`
	StatusBadgeLabel string   `json:"statusBadgeLabel,omitempty"`
	Bytes            barBytes `json:"bytes"`
}

// barBytes is what the displayed bytes are (design §4.2): the formal state
// and its plain word (postureByteWord), which is "unproven" whenever the
// formal state proves nothing.
type barBytes struct {
	State string `json:"state"`
	Word  string `json:"word"`
}

// barFact is one fact as the bar states it. Text is the exact words
// today's posture row prints for it; Unproven, when non-empty, marks the
// fact disclosed-unproven and says why.
type barFact struct {
	Text     string `json:"text"`
	Unproven string `json:"unproven,omitempty"`
}

// barTree is the working tree's state: State is "clean", "dirty", or
// "unproven" (the posture row's data-dirty), Text its words.
type barTree struct {
	State string `json:"state"`
	barFact
}

// barPosture is the full posture one action away from the bar (ac-2):
// checkout, branch, working tree, worktree HEAD, accepted branch and HEAD,
// ahead/behind, divergence when both sides carry commits, and — on a page
// about one spec, where today's row has it — the base digest.
type barPosture struct {
	Checkout barFact `json:"checkout"`
	// Branch is the checked-out branch; on a detached HEAD its Text is
	// empty, as today's row prints it, and Detached says so.
	Branch         barFact  `json:"branch"`
	Detached       bool     `json:"detached,omitempty"`
	Tree           barTree  `json:"tree"`
	WorktreeHead   barFact  `json:"worktreeHead"`
	AcceptedBranch barFact  `json:"acceptedBranch"`
	AcceptedHead   barFact  `json:"acceptedHead"`
	AheadBehind    barFact  `json:"aheadBehind"`
	Divergence     *barFact `json:"divergence,omitempty"`
	BaseDigest     *barFact `json:"baseDigest,omitempty"`
}

// The words today's posture row prints for facts it cannot prove.
const (
	unprovenWord = "unproven"
	// aheadBehindUnresolved is the row's ahead/behind reason, verbatim.
	aheadBehindUnresolved = "the accepted branch could not be resolved"
	// defaultBranchUnresolved is the reason an unresolved default branch
	// leaves the accepted branch and HEAD unproven.
	defaultBranchUnresolved = "the default branch could not be resolved"
)

func provenFact(text string) barFact { return barFact{Text: text} }

func unprovenFact(reason string) barFact {
	return barFact{Text: unprovenWord, Unproven: reason}
}

// branchPosture is the branch-level half of the posture model (design
// §4.2): the checkout's Git facts, resolved once per page by
// resolveBranchPosture. The wall's asdView embeds it; every other page's
// bar reads it directly. The why fields say why an empty fact is
// unresolved; treeWhy, when set, says why the working tree's state is
// (there is none: a remote-only branch's sealed wall), and Dirty is then
// meaningless.
type branchPosture struct {
	Checkout         string
	Branch           string
	DefaultBranch    string
	Dirty            bool
	WorktreeHead     string
	AcceptedHead     string
	Ahead, Behind    int
	AheadBehindKnown bool

	defaultBranchWhy string
	worktreeHeadWhy  string
	acceptedHeadWhy  string
	aheadBehindWhy   string
	treeWhy          string
}

// postureHeads is one page's resolution of the worktree HEAD and the
// accepted HEAD, each empty with the reason when unresolved. The wall
// resolves them itself (resolvePostureHeads); the Document page takes
// the ones its document load already made (SI-323 (2)).
type postureHeads struct {
	worktree, worktreeWhy string
	accepted, acceptedWhy string
}

// postureReader is the posture model's Git port (04 §port pattern:
// defined at its consumer): the worktree HEAD, the accepted HEAD, and
// ahead/behind. gitPostureReader is production; a package test wraps it
// to count a page's accepted-HEAD resolutions (Wave 6 §5.3).
type postureReader interface {
	RevParse(ctx context.Context, dir, rev string) (string, error)
	AheadBehind(ctx context.Context, dir, left, right string) (ahead, behind int, err error)
}

// gitPostureReader reads the posture through internal/gitx.
type gitPostureReader struct{}

func (gitPostureReader) RevParse(ctx context.Context, dir, rev string) (string, error) {
	return gitx.RevParse(ctx, dir, rev)
}

func (gitPostureReader) AheadBehind(ctx context.Context, dir, left, right string) (int, int, error) {
	return gitx.AheadBehind(ctx, dir, left, right)
}

// resolveBranchPosture resolves the posture's Git facts for the checkout
// at root whose gitState is git (resolvePostureHeads, then
// branchPostureFrom). A nil rd reads through gitx.
func resolveBranchPosture(ctx context.Context, root string, git *boardGitState, rd postureReader) branchPosture {
	return branchPostureFrom(ctx, root, git, resolvePostureHeads(ctx, root, git, rd), rd)
}

// resolvePostureHeads resolves the worktree HEAD and — when the default
// branch resolved — the accepted HEAD at its AUTHORITATIVE rev
// (acceptedRef: origin/<name> when it exists, the same rev the state
// projector reads accepted bytes at; keying on the display NAME would
// ride a possibly-stale local shadow while acceptance moves on the
// remote-tracking ref — Codex correction round 1, finding 1, closure
// reopen). It is the page's one accepted-HEAD resolution (Wave 6 §5.3) —
// or, in a request that pinned its accepted HEAD, that pin's id, read
// again by nothing. A nil rd reads through gitx.
func resolvePostureHeads(ctx context.Context, root string, git *boardGitState, rd postureReader) postureHeads {
	if rd == nil {
		rd = gitPostureReader{}
	}
	var h postureHeads
	if head, err := rd.RevParse(ctx, root, "HEAD"); err == nil {
		h.worktree = head
	} else {
		h.worktreeWhy = "the worktree HEAD could not be resolved: " + err.Error()
	}
	if git.DefaultBranch == "" {
		h.acceptedWhy = defaultBranchUnresolved
		return h
	}
	if git.acceptedTip != "" {
		// The request pinned its accepted HEAD: the posture states that
		// one resolution (ledger SI-356).
		h.accepted = git.acceptedTip
		return h
	}
	ref := git.acceptedRef()
	if accepted, err := rd.RevParse(ctx, root, ref); err == nil {
		h.accepted = accepted
	} else {
		h.acceptedWhy = fmt.Sprintf("the accepted HEAD (%s) could not be resolved: %v", ref, err)
	}
	return h
}

// branchPostureFrom completes the branch-level posture from git and the
// heads the page resolved: ahead/behind is counted against that very
// accepted HEAD, so the two facts can never describe different commits.
// A nil rd reads through gitx.
func branchPostureFrom(ctx context.Context, root string, git *boardGitState, heads postureHeads, rd postureReader) branchPosture {
	if rd == nil {
		rd = gitPostureReader{}
	}
	bp := branchPosture{
		Checkout:        root,
		Branch:          git.Branch,
		DefaultBranch:   git.DefaultBranch,
		Dirty:           git.Dirty,
		WorktreeHead:    heads.worktree,
		worktreeHeadWhy: heads.worktreeWhy,
		AcceptedHead:    heads.accepted,
		acceptedHeadWhy: heads.acceptedWhy,
	}
	switch {
	case git.DefaultBranch == "":
		bp.defaultBranchWhy = defaultBranchUnresolved
		bp.aheadBehindWhy = aheadBehindUnresolved
	case heads.accepted == "":
		bp.aheadBehindWhy = aheadBehindUnresolved + ": " + heads.acceptedWhy
	default:
		if ahead, behind, err := rd.AheadBehind(ctx, root, "HEAD", heads.accepted); err == nil {
			bp.Ahead, bp.Behind, bp.AheadBehindKnown = ahead, behind, true
		} else {
			bp.aheadBehindWhy = aheadBehindUnresolved + ": " + err.Error()
		}
	}
	return bp
}

// facts states bp as the bar's posture, with today's row's words: a fact
// with no value reads "unproven" (the row's former orUnproven) with the
// reason resolution recorded, ahead/behind reads the row's own unresolved
// sentence with its reason, and the divergence fact exists only when both
// sides carry commits. Checkout and branch print as they are, as the row
// prints them; an empty branch is a detached HEAD, which Detached marks.
func (bp *branchPosture) facts() barPosture {
	p := barPosture{
		Checkout:       provenFact(bp.Checkout),
		Branch:         provenFact(bp.Branch),
		Detached:       bp.Branch == "",
		Tree:           barTree{State: "clean", barFact: provenFact("clean")},
		WorktreeHead:   valueOr(bp.WorktreeHead, bp.worktreeHeadWhy, "the worktree HEAD could not be resolved"),
		AcceptedBranch: valueOr(bp.DefaultBranch, bp.defaultBranchWhy, defaultBranchUnresolved),
		AcceptedHead:   valueOr(bp.AcceptedHead, bp.acceptedHeadWhy, "the accepted HEAD could not be resolved"),
		AheadBehind:    barFact{Text: unprovenWord + ": " + aheadBehindUnresolved, Unproven: aheadBehindUnresolved},
	}
	if bp.aheadBehindWhy != "" {
		p.AheadBehind.Unproven = bp.aheadBehindWhy
	}
	switch {
	case bp.treeWhy != "":
		p.Tree = barTree{State: unprovenWord, barFact: unprovenFact(bp.treeWhy)}
	case bp.Dirty:
		p.Tree = barTree{State: "dirty", barFact: provenFact("uncommitted changes")}
	}
	if bp.AheadBehindKnown {
		p.AheadBehind = provenFact(fmt.Sprintf("%d ahead, %d behind %s", bp.Ahead, bp.Behind, bp.DefaultBranch))
		if bp.Ahead > 0 && bp.Behind > 0 {
			d := provenFact("diverged: both sides carry commits the other lacks")
			p.Divergence = &d
		}
	}
	return p
}

// valueOr is v proven, or — when v is empty — unproven with why (or
// fallback when resolution recorded no reason).
func valueOr(v, why, fallback string) barFact {
	if v != "" {
		return provenFact(v)
	}
	if why == "" {
		why = fallback
	}
	return unprovenFact(why)
}

// specBarFacts is the bar's facts for a page about one spec, from the
// posture model today's posture row renders: p names the spec, its class,
// and its mode, and asd carries the posture. A pure function — nothing is
// resolved here.
func specBarFacts(p *BoardProjection, asd *asdView) barFacts {
	spec := &barSpec{
		Name:           p.Spec,
		Mode:           string(p.Mode),
		ModeLabel:      modeStampLabel(p),
		ModeDisclosure: asd.reviewNotice,
		Bytes:          barBytes{State: asd.StateFormal, Word: postureByteWord(asd.StateFormal)},
	}
	if p.Class != "" {
		spec.Class = p.classChipID()
		spec.ClassLabel = p.ClassLabel
		if spec.ClassLabel == "" {
			spec.ClassLabel = spec.Class
		}
	}
	if badge := terminalStatusBadge(p.Status); badge != "" {
		spec.StatusBadge, spec.StatusBadgeLabel = badge, badge
		if p.StatusLabel != "" {
			spec.StatusBadgeLabel = p.StatusLabel
		}
	}
	posture := asd.facts() // the embedded branchPosture's
	digest := provenFact(asd.BaseDigest)
	if asd.baseDigestWhy != "" {
		digest = unprovenFact(asd.baseDigestWhy)
	}
	posture.BaseDigest = &digest
	return barFacts{Title: p.Title, Spec: spec, Posture: posture}
}

// branchBarFacts is the bar's facts for a page not about one spec (SI-323
// (1)): the checkout at root's branch, working tree, and branch-level
// posture, read through the wall's own gitState path. A fact it cannot
// obtain is disclosed-unproven with the reason: no root at all, or a Git
// state that cannot be read.
func branchBarFacts(ctx context.Context, root, title string) barFacts {
	if root == "" {
		return unprovenBarFacts(title, "", "no store root is known to this page")
	}
	git, _, err := (&boardSpecServer{root: root}).gitState(ctx)
	if err != nil {
		return unprovenBarFacts(title, root, "the checkout's Git state could not be read: "+err.Error())
	}
	return branchBarFactsFor(ctx, root, title, git)
}

// branchBarFactsFor is branchBarFacts over a gitState the page already
// read (the diagram editor's), so the page reads its Git state once.
func branchBarFactsFor(ctx context.Context, root, title string, git *boardGitState) barFacts {
	bp := resolveBranchPosture(ctx, root, git, nil)
	return barFacts{Title: title, Posture: bp.facts()}
}

// unprovenBarFacts is a page's facts when its Git state cannot be read:
// every posture fact disclosed-unproven with reason, the checkout proven
// when the page knows it.
func unprovenBarFacts(title, checkout, reason string) barFacts {
	u := unprovenFact(reason)
	p := barPosture{
		Checkout:       u,
		Branch:         u,
		Tree:           barTree{State: unprovenWord, barFact: u},
		WorktreeHead:   u,
		AcceptedBranch: u,
		AcceptedHead:   u,
		AheadBehind:    u,
	}
	if checkout != "" {
		p.Checkout = provenFact(checkout)
	}
	return barFacts{Title: title, Posture: p}
}

// barProbeKey is the request-context key of a bar probe.
type barProbeKey struct{}

// barProbe records the bar facts each page render puts in its view data:
// a package test seam, so a test reads which facts a page carries without
// parsing its markup. It holds data only — no function value — so nothing
// a probe carries can run code inside a route's reach (ledger SI-318's
// write-scope witness). Production requests carry none, and no request
// can set one: its context key is unexported.
type barProbe struct {
	mu   sync.Mutex
	seen []barFacts
}

// facts returns a copy of every bar-facts value recorded so far, in
// render order.
func (p *barProbe) facts() []barFacts {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]barFacts(nil), p.seen...)
}

// withBarProbe returns ctx carrying probe, which observeBar records each
// page render's bar facts into.
func withBarProbe(ctx context.Context, probe *barProbe) context.Context {
	return context.WithValue(ctx, barProbeKey{}, probe)
}

// observeBar records f in ctx's bar probe, if it carries one.
func observeBar(ctx context.Context, f barFacts) {
	if probe, ok := ctx.Value(barProbeKey{}).(*barProbe); ok {
		probe.mu.Lock()
		probe.seen = append(probe.seen, f)
		probe.mu.Unlock()
	}
}

// classChipID is the class chip's id: the spec's class, or "spike" for a
// spike story — the same rule the case-file class tag and the model's
// class display word follow.
func (p *BoardProjection) classChipID() string {
	if p.Spike {
		return "spike"
	}
	return p.Class
}

// String renders f for test failure output.
func (f barFact) String() string {
	if f.Unproven == "" {
		return f.Text
	}
	return f.Text + " (" + strings.TrimSpace(f.Unproven) + ")"
}
