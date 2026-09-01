package workbench

// The ASD workbench's rendered-fact assembly (Wave 6 Task 2, design §6.2):
// the revision/posture header facts, the promoted four-area shell's
// concern derivation (the Wave 3.5 pilot's presentation idioms — SI-125's
// plain labels, exactly-three preview, current-area-first ordering —
// applied to this board's own typed facts), and the per-render client
// facts (base digest/bytes, expected identity, grammar pattern, next
// object ids, stored link tuples) the browser needs to construct typed
// mutate_draft transactions without interpreting spec bytes itself.
//
// Everything here is presentation over facts the application owners
// already returned: the decoded frontmatter the projection was built
// from, the projection itself, Git facts from gitx, and the capabilities
// view the designapp bridge converted. Nothing derives lifecycle truth,
// scores, or authority (design §3's adapter boundary).

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifact/splice"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// The four presentation areas, byte-identical to the readiness pilot's
// promoted shell (SI-124/SI-125): ids are addressing, labels are the
// participant-ratified plain words. Presentation concepts, never
// lifecycle states.
type asdAreaID string

const (
	asdAreaShape   asdAreaID = "shape-proposal"
	asdAreaSuccess asdAreaID = "show-success"
	asdAreaContext asdAreaID = "check-context"
	asdAreaReview  asdAreaID = "request-review"
)

var asdAreaOrder = []asdAreaID{asdAreaShape, asdAreaSuccess, asdAreaContext, asdAreaReview}

var asdAreaLabels = map[asdAreaID]string{
	asdAreaShape:   "Define the work",
	asdAreaSuccess: "Define success",
	asdAreaContext: "Check constraints",
	asdAreaReview:  "Get approval",
}

// The pilot's exact three-valued vocabulary (never a fourth word).
const (
	asdStateProven   = "proven"
	asdStateViolated = "violated-with-witness"
	asdStateUnproven = "unproven"
)

// asdConcern is one shell row: a source fact, its honest state, its
// explicit timing/dependency (F-02), and its source-derived corrective
// guidance (F-03). HumanReview marks the plainly-labeled human-review
// rows (F-05); the formal obligation stays in Witnesses.
type asdConcern struct {
	ID          string
	Area        asdAreaID
	State       string
	Blocking    bool
	Summary     string
	Guidance    string
	Witnesses   []string
	Dest        string // in-page fragment href ("" when none applies)
	HumanReview bool
}

type asdArea struct {
	ID    asdAreaID
	Label string
	State string
}

// asdShell is the derived presentation: lossless (All), prominence-only
// ordering (Attention), one deterministic focus.
type asdShell struct {
	Areas              []asdArea
	CurrentFocus       asdAreaID
	Attention          []asdConcern
	All                []asdConcern
	DownstreamViolated int
}

// asdShellInput is deriveASDShell's complete typed input — assembled from
// the decoded frontmatter, projection, Git facts, and capabilities view;
// the derivation itself reads nothing else.
type asdShellInput struct {
	ProblemPresent  bool
	OutcomePresent  bool
	OpenQuestions   []asdObjectFact
	OpenStickyCount int
	ACs             []asdACFact
	UncoveredACs    []string // feature walls: declared ACs no stub covers
	Class           string
	Mode            string
	Dirty           bool
	Branch          string
	StateFormal     string
	UnderReview     bool
	DesignWired     bool
	Caps            *DesignCapabilitiesView
	CapsFailure     *DesignFailure
	PinnedContext   int
}

type asdObjectFact struct{ ID, Text string }

type asdACFact struct {
	ID            string
	EvidenceCount int
}

// deriveASDShell maps the board's typed facts onto the four-area shell.
// Every area receives an explicit positive or unresolved anchor (a proven
// area is never vacuous), every unresolved row carries source-derived
// guidance, and ordering follows SI-125 exactly: current-area unresolved
// rows first, then blocking before nonblocking, violated before unproven,
// area order, id — prominence only, nothing suppressed.
func deriveASDShell(in asdShellInput) asdShell {
	var all []asdConcern
	add := func(c asdConcern) { all = append(all, c) }

	// -- shape-proposal: Define the work --------------------------------
	if in.ProblemPresent {
		add(asdConcern{ID: "shape/problem", Area: asdAreaShape, State: asdStateProven, Blocking: true,
			Summary: "The problem statement is present."})
	} else {
		add(asdConcern{ID: "shape/problem", Area: asdAreaShape, State: asdStateViolated, Blocking: true,
			Summary:   "No problem statement is declared.",
			Guidance:  "State the problem (typed operation set-problem) — the case file opens with it.",
			Witnesses: []string{"spec.md frontmatter declares no problem attribute"},
			Dest:      "#asd-forms"})
	}
	if in.OutcomePresent {
		add(asdConcern{ID: "shape/outcome", Area: asdAreaShape, State: asdStateProven, Blocking: true,
			Summary: "The outcome statement is present."})
	} else {
		add(asdConcern{ID: "shape/outcome", Area: asdAreaShape, State: asdStateViolated, Blocking: true,
			Summary:   "No outcome statement is declared.",
			Guidance:  "State the intended outcome (typed operation set-outcome).",
			Witnesses: []string{"spec.md frontmatter declares no outcome attribute"},
			Dest:      "#asd-forms"})
	}
	for _, oq := range in.OpenQuestions {
		add(asdConcern{ID: "shape/question/" + oq.ID, Area: asdAreaShape, State: asdStateUnproven, Blocking: true,
			Summary:   "Open question " + oq.ID + " is unresolved: " + oq.Text,
			Guidance:  "Resolve it on the wall: edit or remove " + oq.ID + ", or graduate a decision that answers it.",
			Witnesses: []string{"declared open question " + oq.ID},
			Dest:      "#obj-" + oq.ID})
	}
	if in.OpenStickyCount > 0 {
		add(asdConcern{ID: "shape/board", Area: asdAreaShape, State: asdStateUnproven, Blocking: false,
			Summary:   fmt.Sprintf("%d open scratch record(s) sit on the wall.", in.OpenStickyCount),
			Guidance:  "Graduate each sticky into the spec, or delete it — scratch never enters the record by itself.",
			Witnesses: []string{fmt.Sprintf("%d open annotation record(s) on this board", in.OpenStickyCount)}})
	}

	// -- show-success: Define success -----------------------------------
	if len(in.ACs) == 0 {
		add(asdConcern{ID: "success/criteria", Area: asdAreaSuccess, State: asdStateViolated, Blocking: true,
			Summary:   "No acceptance criteria are declared.",
			Guidance:  "Declare what must be true when this lands (typed operation add-ac).",
			Witnesses: []string{"spec.md frontmatter declares no acceptance_criteria"},
			Dest:      "#asd-forms"})
	} else {
		add(asdConcern{ID: "success/criteria", Area: asdAreaSuccess, State: asdStateProven, Blocking: true,
			Summary: fmt.Sprintf("%d acceptance criteria are declared.", len(in.ACs))})
		for _, ac := range in.ACs {
			if ac.EvidenceCount == 0 {
				add(asdConcern{ID: "success/evidence/" + ac.ID, Area: asdAreaSuccess, State: asdStateUnproven, Blocking: false,
					Summary:   "Acceptance criterion " + ac.ID + " declares no evidence kinds.",
					Guidance:  "Declare how " + ac.ID + " will be proven (typed operation set-ac-evidence).",
					Witnesses: []string{ac.ID + " evidence list is empty"},
					Dest:      "#obj-" + ac.ID})
			}
		}
	}
	for _, acID := range in.UncoveredACs {
		add(asdConcern{ID: "success/coverage/" + acID, Area: asdAreaSuccess, State: asdStateUnproven, Blocking: false,
			Summary: "No stub covers acceptance criterion " + acID + " yet.",
			// vocab:identity — "story sticky" names the annotation TYPE id (02 §Record schemas' proto-sticky enum), not display class prose
			Guidance:  "Plan the delivery: graduate a story sticky into a stub claiming " + acID + ".",
			Witnesses: []string{"declared stub coverage count for " + acID + " is 0"},
			Dest:      "#obj-" + acID})
	}

	// -- check-context: Check constraints -------------------------------
	switch {
	case !in.DesignWired:
		add(asdConcern{ID: "context/capabilities", Area: asdAreaContext, State: asdStateUnproven, Blocking: false,
			Summary:   "Design capabilities are unavailable: the design application service is not wired on this server.",
			Guidance:  "Run the workbench through `verdi serve`, which wires the design application service.",
			Witnesses: []string{"design-service-unwired"}})
	case in.Caps != nil:
		witnesses := []string{"design_assistance mode " + in.Caps.PolicyMode, "effective policy digest " + in.Caps.PolicyDigest}
		add(asdConcern{ID: "context/policy", Area: asdAreaContext, State: asdStateProven, Blocking: true,
			Summary:   "Design-assistance policy is adopted (mode " + in.Caps.PolicyMode + ").",
			Witnesses: witnesses})
		if in.Caps.Mutable {
			add(asdConcern{ID: "context/agent-writes", Area: asdAreaContext, State: asdStateProven, Blocking: false,
				// vocab:identity — "draft writes"/"draft-write" is the design_assistance mode's own enum family (AC-3), not the lifecycle state word
				Summary:   "Delegated agents may apply typed draft writes here.",
				Witnesses: []string{"mutable=true for the delegated-agent posture"}})
		} else {
			add(asdConcern{ID: "context/agent-writes", Area: asdAreaContext, State: asdStateProven, Blocking: false,
				// vocab:identity — "this draft" names AC-1's canonical draft (ASD protocol term), not a lifecycle state word
				Summary:   "Delegated agents cannot write to this draft (" + in.Caps.RefusalPrecondition + ").",
				Witnesses: []string{in.Caps.RefusalDetail}})
		}
	case in.CapsFailure != nil && in.CapsFailure.Code == "policy-forbidden":
		add(asdConcern{ID: "context/policy", Area: asdAreaContext, State: asdStateUnproven, Blocking: false,
			Summary:   "No policy authority is adopted; browser editing proceeds and records the explicit not-applicable policy posture.",
			Guidance:  "Adopt a project constitution (.verdi/policy) to govern agent design assistance; human editing does not require one.",
			Witnesses: []string{in.CapsFailure.Code + ": " + in.CapsFailure.Detail}})
	default:
		detail := "capabilities unavailable"
		if in.CapsFailure != nil {
			detail = in.CapsFailure.Code + ": " + in.CapsFailure.Detail
		}
		add(asdConcern{ID: "context/capabilities", Area: asdAreaContext, State: asdStateUnproven, Blocking: false,
			Summary:   "Design capabilities could not be derived for this wall.",
			Guidance:  "Resolve the named failure, then refresh.",
			Witnesses: []string{detail}})
	}
	if in.PinnedContext > 0 {
		add(asdConcern{ID: "context/pinned", Area: asdAreaContext, State: asdStateProven, Blocking: false,
			Summary:   fmt.Sprintf("%d pinned context reference(s) are declared.", in.PinnedContext),
			Witnesses: []string{fmt.Sprintf("context: declares %d pinned ref(s)", in.PinnedContext)}})
	}

	// -- request-review: Get approval -----------------------------------
	if in.Dirty {
		add(asdConcern{ID: "review/worktree", Area: asdAreaReview, State: asdStateUnproven, Blocking: true,
			Summary:   "Uncommitted changes sit on this working tree.",
			Guidance:  "Commit & push to file the proposal on " + in.Branch + " — review reads the committed head.",
			Witnesses: []string{"git status reports a dirty working tree"},
			Dest:      "#asd-git"})
	} else {
		add(asdConcern{ID: "review/worktree", Area: asdAreaReview, State: asdStateProven, Blocking: true,
			Summary: "The working tree is clean: the proposal is filed on " + in.Branch + "."})
	}
	switch {
	case in.StateFormal == string(specstate.AcceptedPendingBuild):
		add(asdConcern{ID: "review/acceptance", Area: asdAreaReview, State: asdStateProven, Blocking: true,
			// vocab:identity — non-vocabulary homograph: the forge's merge (owner's merge of the PR), never the `merge` lifecycle transition word
			Summary:     "This revision is accepted: the owner's merge made it reachable from the default branch.",
			HumanReview: true,
			Witnesses:   []string{"Git-derived state " + in.StateFormal}})
	case in.UnderReview:
		add(asdConcern{ID: "review/acceptance", Area: asdAreaReview, State: asdStateUnproven, Blocking: true,
			// vocab:identity — non-vocabulary homograph: the forge's merge request/owner's merge, never the `merge` lifecycle transition word
			Summary: "Human review is open: this wall mirrors the proposal's merge request.",
			// vocab:identity — non-vocabulary homograph: the forge's merge request/owner's merge, never the `merge` lifecycle transition word
			Guidance:    "The owner's merge of the open merge request is the single acceptance decision — no second ceremony.",
			HumanReview: true,
			// vocab:identity — non-vocabulary homograph: the forge's merge request/authorizes-merge, never the `merge` lifecycle transition word
			Witnesses: []string{"an open merge request mirrors this spec", "AC-6/DC-15: the profile-required review of the exact proposed head authorizes merge"}})
	default:
		add(asdConcern{ID: "review/acceptance", Area: asdAreaReview, State: asdStateUnproven, Blocking: true,
			Summary: "Human review has not accepted this proposal yet.",
			// vocab:identity — non-vocabulary homograph: the forge's owner's-merge of the pull request, never the `merge` lifecycle transition word
			Guidance:    "Derive the semantic review packet (Semantic review, below), open a pull request from " + in.Branch + ", and request the owner's review — the owner's merge is the single acceptance decision.",
			HumanReview: true,
			// vocab:identity — non-vocabulary homograph: authorizes-merge is the forge gate, never the `merge` lifecycle transition word
			Witnesses: []string{"Git-derived state " + in.StateFormal, "AC-6/DC-15: the profile-required review of the exact proposed head authorizes merge; no separate acceptance command exists"}})
	}

	return assembleASDShell(all)
}

// assembleASDShell computes area states, focus, ordering, and the
// downstream-violated count from the complete concern list.
func assembleASDShell(all []asdConcern) asdShell {
	areaIndex := map[asdAreaID]int{}
	for i, id := range asdAreaOrder {
		areaIndex[id] = i
	}
	// Lossless ordering for All: area order, then concern id.
	sort.SliceStable(all, func(i, j int) bool {
		if areaIndex[all[i].Area] != areaIndex[all[j].Area] {
			return areaIndex[all[i].Area] < areaIndex[all[j].Area]
		}
		return all[i].ID < all[j].ID
	})

	stateRank := map[string]int{asdStateViolated: 0, asdStateUnproven: 1}
	areas := make([]asdArea, 0, len(asdAreaOrder))
	for _, id := range asdAreaOrder {
		state := asdStateProven
		for _, c := range all {
			if c.Area != id || !c.Blocking {
				continue
			}
			if c.State == asdStateViolated {
				state = asdStateViolated
				break
			}
			if c.State == asdStateUnproven {
				state = asdStateUnproven
			}
		}
		areas = append(areas, asdArea{ID: id, Label: asdAreaLabels[id], State: state})
	}
	var focus asdAreaID
	for _, a := range areas {
		if a.State != asdStateProven {
			focus = a.ID
			break
		}
	}

	var attention []asdConcern
	for _, c := range all {
		if c.State != asdStateProven {
			attention = append(attention, c)
		}
	}
	focusIdx, focused := areaIndex[focus], focus != ""
	sort.SliceStable(attention, func(i, j int) bool {
		a, b := attention[i], attention[j]
		// SI-125: current-area unresolved rows first.
		if focused {
			ai, bi := a.Area == focus, b.Area == focus
			if ai != bi {
				return ai
			}
		}
		if a.Blocking != b.Blocking {
			return a.Blocking
		}
		if stateRank[a.State] != stateRank[b.State] {
			return stateRank[a.State] < stateRank[b.State]
		}
		if areaIndex[a.Area] != areaIndex[b.Area] {
			return areaIndex[a.Area] < areaIndex[b.Area]
		}
		return a.ID < b.ID
	})

	downstream := 0
	if focused {
		for _, c := range all {
			if c.State == asdStateViolated && areaIndex[c.Area] > focusIdx {
				downstream++
			}
		}
	}
	return asdShell{Areas: areas, CurrentFocus: focus, Attention: attention, All: all, DownstreamViolated: downstream}
}

// asdEdgeFact is one stored spec-layer link tuple, keyed by the rendered
// edge's (from, type, endpoint) triple so the chip can carry the EXACT
// bytes remove-link/add-link address (splice matches exact tuples).
type asdEdgeFact struct {
	Ref  string
	Note string
}

// asdView carries every ASD-specific rendered fact for one page render.
type asdView struct {
	// Posture header (design §4.2).
	Checkout         string
	Branch           string
	DefaultBranch    string
	WorktreeHead     string
	AcceptedHead     string
	Ahead, Behind    int
	AheadBehindKnown bool
	Dirty            bool
	StateFormal      string
	StateLabel       string
	RelationDiverged bool

	// Shell.
	Shell asdShell

	// Capabilities facts (agent posture; browser gating rides Mode).
	Caps        *DesignCapabilitiesView
	CapsFailure *DesignFailure
	DesignWired bool

	// Client mutation facts.
	BaseDigest       string
	BaseSpecB64      string
	ExpectedCheckout string
	ExpectedBranch   string
	ExpectedHead     string
	ExpectedKnown    bool
	SlugPattern      string
	NextIDs          map[string]string
	ProblemAnchor    string
	OutcomeAnchor    string
	ObjectAnchors    map[string]string
	ObjectEvidence   map[string]string
	StickySlugs      map[string]string
	StubSlugs        []string
	EdgeFacts        map[string][]asdEdgeFact
}

// asdEdgeKey builds the chip-fact lookup key.
func asdEdgeKey(from, edgeType, to string) string {
	return from + "\x00" + edgeType + "\x00" + to
}

// buildASDView assembles the complete ASD render facts for one loaded
// board. It performs the page's one capabilities consultation (a cited
// designapp predecessor API — SI-168 disclosure) and the header's Git
// fact reads; everything else is a pure function of the already-decoded
// inputs.
func (s *boardSpecServer) buildASDView(ctx context.Context, name string, proj *BoardProjection, git *boardGitState, raw []byte, fm *artifact.SpecFrontmatter, st specstate.Result) (*asdView, error) {
	v := &asdView{
		Checkout:      s.root,
		Branch:        git.Branch,
		DefaultBranch: git.DefaultBranch,
		Dirty:         git.Dirty,
		StateFormal:   string(st.State),
		StateLabel:    s.model.DisplayState(proj.Class, string(st.ArtifactStatus())),
		BaseDigest:    digestSpecBytes(raw),
		BaseSpecB64:   base64.StdEncoding.EncodeToString(raw),
		SlugPattern:   specNameRe.String(),
	}
	v.RelationDiverged = st.State == specstate.Proposed && st.Relation == specstate.RelationDiverged

	worktreeHead := ""
	if head, err := gitx.RevParse(ctx, s.root, "HEAD"); err == nil {
		worktreeHead = head
		v.WorktreeHead = head
	}
	if git.DefaultBranch != "" {
		if accepted, err := gitx.RevParse(ctx, s.root, git.DefaultBranch); err == nil {
			v.AcceptedHead = accepted
		}
		if ahead, behind, err := gitx.AheadBehind(ctx, s.root, "HEAD", git.DefaultBranch); err == nil {
			v.Ahead, v.Behind, v.AheadBehindKnown = ahead, behind, true
		}
	}

	if s.design != nil {
		v.DesignWired = true
		view, failure := s.cachedCapabilities(ctx, name, git.Branch, worktreeHead, v.BaseDigest)
		v.Caps = view
		v.CapsFailure = failure
	}
	if proj.Mode == modeAuthoring && worktreeHead != "" && git.Branch != "" {
		// The kernel's canonical checkout path is resolved once and cached
		// (stable per server); branch and HEAD are this render's own fresh
		// Git facts — together the exact expected identity the mutation
		// kernel verifies.
		if checkout, err := s.cachedCanonicalCheckout(ctx, name); err == nil {
			v.ExpectedCheckout, v.ExpectedBranch, v.ExpectedHead, v.ExpectedKnown = checkout, git.Branch, worktreeHead, true
		}
	}

	// Object facts for typed client operations.
	v.NextIDs = map[string]string{}
	var existing []string
	for _, c := range proj.Cards {
		existing = append(existing, c.ID)
	}
	for _, prefix := range []string{"ac", "co", "dc", "oq"} {
		v.NextIDs[prefix] = splice.NextID(existing, prefix)
	}
	v.ObjectAnchors = map[string]string{}
	v.ObjectEvidence = map[string]string{}
	if fm.Problem != nil {
		v.ProblemAnchor = fm.Problem.Anchor
	}
	if fm.Outcome != nil {
		v.OutcomeAnchor = fm.Outcome.Anchor
	}
	for _, ac := range fm.AcceptanceCriteria {
		v.ObjectAnchors[ac.ID] = ac.Anchor
		kinds := make([]string, len(ac.Evidence))
		for i, k := range ac.Evidence {
			kinds[i] = string(k)
		}
		v.ObjectEvidence[ac.ID] = strings.Join(kinds, ",")
	}
	for _, co := range fm.Constraints {
		v.ObjectAnchors[co.ID] = co.Anchor
	}
	for _, dc := range fm.Decisions {
		v.ObjectAnchors[dc.ID] = dc.Anchor
	}
	for _, oq := range fm.OpenQuestions {
		v.ObjectAnchors[oq.ID] = oq.Anchor
	}

	v.StickySlugs = map[string]string{}
	for _, sticky := range proj.Stickies {
		v.StickySlugs[sticky.ID] = store.RefSlug(sticky.Body)
	}
	for _, sv := range proj.StubViews {
		v.StubSlugs = append(v.StubSlugs, sv.Slug)
	}

	// Stored link tuples for the spec-layer chips, in the projection's own
	// iteration order (buildProjection 1b), consumed by the renderer in
	// the same order per key.
	declared := declaredBoolSet(proj)
	v.EdgeFacts = map[string][]asdEdgeFact{}
	for _, dc := range fm.Decisions {
		for _, l := range dc.Links {
			if !closedEdgeType(l.Type) {
				continue
			}
			key := asdEdgeKey(dc.ID, string(l.Type), edgeEndpoint(name, declared, l.Ref))
			v.EdgeFacts[key] = append(v.EdgeFacts[key], asdEdgeFact{Ref: l.Ref, Note: l.Note})
		}
	}

	// Shell input.
	in := asdShellInput{
		ProblemPresent:  proj.Problem != "",
		OutcomePresent:  proj.Outcome != "",
		OpenStickyCount: len(proj.Stickies),
		Class:           proj.Class,
		Mode:            string(proj.Mode),
		Dirty:           git.Dirty,
		Branch:          git.Branch,
		StateFormal:     string(st.State),
		UnderReview:     proj.Mode == modeReview,
		DesignWired:     v.DesignWired,
		Caps:            v.Caps,
		CapsFailure:     v.CapsFailure,
		PinnedContext:   len(fm.Context),
	}
	for _, oq := range fm.OpenQuestions {
		in.OpenQuestions = append(in.OpenQuestions, asdObjectFact{ID: oq.ID, Text: oq.Text})
	}
	for _, ac := range fm.AcceptanceCriteria {
		in.ACs = append(in.ACs, asdACFact{ID: ac.ID, EvidenceCount: len(ac.Evidence)})
	}
	if proj.Class == string(artifact.ClassFeature) {
		for _, c := range proj.Cards {
			if boardlayout.ZoneKind(c.Kind) == boardlayout.ZoneAC && proj.ACCoverage[c.ID] == 0 {
				in.UncoveredACs = append(in.UncoveredACs, c.ID)
			}
		}
		sort.Strings(in.UncoveredACs)
	}
	v.Shell = deriveASDShell(in)
	return v, nil
}

// asdSnapshot is the /snapshot projection: one rendered region plus the
// exact machine facts the browser's conditional refresh and typed
// mutations consume. Revision is the deterministic token over every
// rendered fact (SI-165) — the digest of this snapshot's own canonical
// content.
type asdSnapshot struct {
	Revision    string          `json:"revision"`
	HTML        string          `json:"html"`
	BaseDigest  string          `json:"base_digest"`
	BaseSpecB64 string          `json:"base_spec_b64"`
	Git         *boardGitState  `json:"git"`
	Expected    asdExpectedWire `json:"expected"`
}

type asdExpectedWire struct {
	Checkout string `json:"checkout"`
	Branch   string `json:"branch"`
	Head     string `json:"head"`
}

// loadSnapshot builds one complete snapshot: one composed page projection
// (loadASD) rendered once, then the revision token over the whole
// serialized content.
func (s *boardSpecServer) loadSnapshot(ctx context.Context, name string) (*asdSnapshot, error) {
	proj, git, asd, err := s.loadASD(ctx, name)
	if err != nil {
		return nil, err
	}
	snap := &asdSnapshot{
		HTML:        renderBoardRegion(proj, git, asd),
		BaseDigest:  asd.BaseDigest,
		BaseSpecB64: asd.BaseSpecB64,
		Git:         git,
		Expected:    asdExpectedWire{Checkout: asd.ExpectedCheckout, Branch: asd.ExpectedBranch, Head: asd.ExpectedHead},
	}
	snap.Revision = snapshotRevision(snap)
	return snap, nil
}

// snapshotRevision digests every rendered fact of one snapshot (the
// revision field itself excluded). Deterministic: the render is a pure
// function of store state, and the machine fields are exact copies of it.
func snapshotRevision(snap *asdSnapshot) string {
	h := sha256.New()
	enc := json.NewEncoder(h)
	_ = enc.Encode(struct {
		HTML        string          `json:"html"`
		BaseDigest  string          `json:"base_digest"`
		BaseSpecB64 string          `json:"base_spec_b64"`
		Git         *boardGitState  `json:"git"`
		Expected    asdExpectedWire `json:"expected"`
	}{snap.HTML, snap.BaseDigest, snap.BaseSpecB64, snap.Git, snap.Expected})
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// asdClientPayload is the ASD slice of the embedded page state — the same
// facts the snapshot serves, so the initial page needs no bootstrap fetch.
type asdClientPayload struct {
	Revision    string            `json:"revision"`
	BaseDigest  string            `json:"baseDigest"`
	BaseSpecB64 string            `json:"baseSpecB64"`
	Expected    asdExpectedWire   `json:"expected"`
	SlugPattern string            `json:"slugPattern"`
	NextIDs     map[string]string `json:"nextIds"`
}

// asdCountLabel is a tiny helper for exact-count copy.
func asdCountLabel(n int, singular, plural string) string {
	if n == 1 {
		return strconv.Itoa(n) + " " + singular
	}
	return strconv.Itoa(n) + " " + plural
}

// capsCacheEntry is one memoized capabilities consultation.
type capsCacheEntry struct {
	view    *DesignCapabilitiesView
	failure *DesignFailure
}

// policyStamp fingerprints the working tree's policy authority inputs
// (.verdi/policy file names, sizes, and mtimes) so a live policy edit
// invalidates the capabilities memo without re-running the full policy
// resolution per poll. Errors and absence stamp distinctly.
func (s *boardSpecServer) policyStamp() string {
	h := sha256.New()
	root := filepath.Join(s.root, ".verdi", "policy")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		fmt.Fprintf(h, "%s|%d|%d\n", path, info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		fmt.Fprintf(h, "err|%v", err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// cachedCapabilities memoizes the designapp capabilities consultation per
// exact fact key. The consultation itself stays designapp's alone — this
// is transport-level memoization of an unchanged operation over unchanged
// inputs, never a second derivation.
func (s *boardSpecServer) cachedCapabilities(ctx context.Context, name, branch, head, digest string) (*DesignCapabilitiesView, *DesignFailure) {
	key := name + "\x00" + branch + "\x00" + head + "\x00" + digest + "\x00" + s.policyStamp()
	s.capsMu.Lock()
	if entry, ok := s.capsCache[key]; ok {
		s.capsMu.Unlock()
		return entry.view, entry.failure
	}
	s.capsMu.Unlock()
	outcome, view := s.design.GetDesignCapabilities(ctx, s.root, "spec/"+name)
	s.capsMu.Lock()
	if s.capsCache == nil {
		s.capsCache = map[string]capsCacheEntry{}
	}
	// Bound the memo: one live entry per spec name (the key embeds every
	// volatile fact, so replacing is correct and keeps the map from
	// growing with history).
	for k := range s.capsCache {
		if strings.HasPrefix(k, name+"\x00") {
			delete(s.capsCache, k)
		}
	}
	s.capsCache[key] = capsCacheEntry{view: view, failure: outcome.Failure}
	s.capsMu.Unlock()
	return view, outcome.Failure
}
