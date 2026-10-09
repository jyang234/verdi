package workbench

// The ASD workbench's rendered-fact assembly (Wave 6 Task 2, design §6.2):
// the revision/posture header facts, the policy setup guide's choice from
// the capabilities consultation, and the per-render client facts (base
// digest/bytes, expected identity, grammar pattern, next object ids,
// stored link tuples) the browser needs to construct typed mutate_draft
// transactions without interpreting spec bytes itself. The wall shell's
// own readiness derivation is retired (spec/wall-strip-and-drawer-v2 ac-7,
// dc-4): the wall's readiness is the record drawer's Readiness tab, which
// renders the per-request loader's facts (wallreadiness.go).
//
// Everything here is presentation over facts the application owners
// already returned: the decoded frontmatter the projection was built
// from, the projection itself, and Git facts from gitx. Nothing derives
// lifecycle truth, scores, or authority (design §3's adapter boundary).

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifact/splice"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// policyGuideKind selects the policy guide variant from the refusal's own
// discriminant. draftmutation raises policy-forbidden from TWO distinct
// conditions (internal/draftmutation/policy.go's ResolvePolicyGrant): the
// canonical not-adopted condition (policyIdentityNotAdopted, the ONLY
// failure "a caller may read as genuine non-adoption") and an adopted,
// sealed effective policy that carries no design_assistance payload. The
// code alone therefore never justifies "no policy is adopted".
type policyGuideKind string

const (
	policyGuideNone policyGuideKind = ""
	// policyGuideNotAdopted: the refusal detail carries draftmutation's
	// exact not-adopted discriminant — initial manual setup applies.
	policyGuideNotAdopted policyGuideKind = "not-adopted"
	// policyGuideNoDesignAssistance: every other policy-forbidden refusal —
	// policy authority resolved, but design assistance is not granted by
	// it. The guide must never describe that policy as absent or
	// unaccepted.
	policyGuideNoDesignAssistance policyGuideKind = "no-design-assistance"
)

// policyNotAdoptedDetail is draftmutation's exact not-adopted detail
// (policy.go's policyIdentityNotAdopted), forwarded by designapp as
// DesignFailure's bare Detail — never re-embedding the "policy-forbidden:"
// prefix Code already carries (ac-5, spec/uat-round-1: a package/code
// prefix appears exactly once, at the outermost layer that owns it;
// internal/designapp/outcome.go's translateDraftmutationError is that
// layer). Matched by containment, not equality, so this constant stays a
// robust discriminant rather than a second place the exact wire shape must
// be kept in sync.
const policyNotAdoptedDetail = "project has not adopted policy authority"

// policyGuide is the policy setup guide's facts: its variant, and the
// refusal it quotes — the code and the bare detail, verbatim.
type policyGuide struct {
	Kind   policyGuideKind
	Code   string
	Detail string
}

// policyGuideFor selects the policy setup guide from the capabilities
// consultation alone (SI-368 (3): the guide is capabilities, not
// readiness), so it needs no readiness load: no guide while the design
// application service is unwired or capabilities were derived; the
// not-adopted variant only when a policy-forbidden refusal carries
// draftmutation's own not-adopted discriminant; the no-design-assistance
// variant for every other policy-forbidden refusal; and none for any other
// failure. The Readiness tab carries the guide it chooses (SI-368 (3)).
func policyGuideFor(designWired bool, caps *DesignCapabilitiesView, failure *DesignFailure) policyGuide {
	if !designWired || caps != nil || failure == nil || failure.Code != "policy-forbidden" {
		return policyGuide{}
	}
	kind := policyGuideNoDesignAssistance
	if strings.Contains(failure.Detail, policyNotAdoptedDetail) {
		kind = policyGuideNotAdopted
	}
	return policyGuide{Kind: kind, Code: failure.Code, Detail: failure.Detail}
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
	// Posture header (design §4.2): the branch-level Git facts every
	// page's top bar shares (branchPosture, resolved by
	// resolveBranchPosture), then the spec's own.
	branchPosture
	StateFormal      string
	StateLabel       string
	RelationDiverged bool

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

	// reviewNotice is the review feed's disclosure for this render, when
	// the feed is configured but could not be consulted (consultReview):
	// the mode the posture states was derived without review state.
	reviewNotice string

	// baseDigestWhy, when set, says why BaseDigest is unresolved: a
	// remote-only branch's sealed wall has no working-tree spec bytes to
	// digest (sealedASDView).
	baseDigestWhy string

	// ImportRecordHref is the read-only source-record view's address when
	// this board's working tree carries a committed import record for the
	// spec (spec-import-contract: "The review UI must show an adjacent
	// verified source-record link"); "" for a never-imported spec. A
	// presence fact from the tree — the record view does the verifying.
	ImportRecordHref string

	// Marks is the wall's readiness marks (spec/wall-canvas-v2 ac-1, dc-1;
	// ledger SI-360 (4)): per card, the Focus next concerns naming it, or
	// the unavailable notice's one reason. It is render data for the
	// canvas, nil when this render composed no marks (the fragment, a
	// mutation's fresh projection).
	Marks *wallMarks

	// Pill is the readiness pill's facts (spec/wall-strip-and-drawer-v2
	// ac-4; ledger SI-368 (2)), from the readiness the composed refresh
	// already loaded, or the marks' fixed reason where the marks are fixed
	// (SI-364 (3)); nil when this render composed none (SI-362 (2)).
	Pill *wallPill

	// readinessRevision digests what this view's projection read beyond
	// the wall itself — the readiness it loaded, or the load's failure,
	// and the marks and pill derived from it — so the snapshot's revision
	// covers it (Wave 6 §5.1; SI-360 (2)); "" when the projection loaded
	// no readiness.
	readinessRevision string
}

// asdEdgeKey builds the chip-fact lookup key.
func asdEdgeKey(from, edgeType, to string) string {
	return from + "\x00" + edgeType + "\x00" + to
}

// postureView is the posture model for one served spec (design §4.2):
// the branch-level Git facts completed from the page's heads
// (branchPostureFrom) plus the spec's displayed-bytes state and base
// digest. The wall's asdView starts from it over heads it resolves
// itself; the Document page's top bar builds it over the heads, state,
// and bytes its document load already resolved — one path, so both pages
// state one posture for the same spec and branch (SI-323 (2)).
func (s *boardSpecServer) postureView(ctx context.Context, proj *BoardProjection, git *boardGitState, raw []byte, st specstate.Result, heads postureHeads) *asdView {
	return &asdView{
		branchPosture:    branchPostureFrom(ctx, s.root, git, heads, s.posture),
		StateFormal:      string(st.State),
		StateLabel:       s.model.DisplayState(proj.Class, string(st.ArtifactStatus())),
		RelationDiverged: st.State == specstate.Proposed && st.Relation == specstate.RelationDiverged,
		BaseDigest:       digestSpecBytes(raw),
	}
}

// buildASDView assembles the complete ASD render facts for one loaded
// board. It performs the header's Git fact reads; everything else is a
// pure function of the already-decoded inputs. The wall consults no
// capabilities: the policy setup guide those chose is the Readiness tab's
// (SI-368 (3), (30)(c)), which consults them when it opens.
func (s *boardSpecServer) buildASDView(ctx context.Context, name string, proj *BoardProjection, git *boardGitState, raw []byte, fm *artifact.SpecFrontmatter, st specstate.Result) (*asdView, error) {
	v := s.postureView(ctx, proj, git, raw, st, resolvePostureHeads(ctx, s.root, git, s.posture))
	v.BaseSpecB64 = base64.StdEncoding.EncodeToString(raw)
	v.SlugPattern = specNameRe.String()
	v.ImportRecordHref = specImportRecordHrefFor(s.root, git.Branch, name)
	worktreeHead := v.WorktreeHead

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

	return v, nil
}

// asdSnapshot is the /snapshot projection: one rendered region plus the
// exact machine facts the browser's conditional refresh and typed
// mutations consume. Revision is the deterministic token over every
// rendered fact (SI-165) — the digest of this snapshot's own canonical
// content, plus the top bar's facts.
type asdSnapshot struct {
	Revision string `json:"revision"`
	HTML     string `json:"html"`
	// Posture is the top bar's posture group rendered from the bar's facts,
	// as its own field (SI-323 (3)): the posture lives in the top bar,
	// outside the region, and a refresh swaps it from here.
	Posture     string          `json:"posture"`
	BaseDigest  string          `json:"base_digest"`
	BaseSpecB64 string          `json:"base_spec_b64"`
	Git         *boardGitState  `json:"git"`
	Expected    asdExpectedWire `json:"expected"`
	// Marks is the wall's readiness marks (SI-360 (4)), when the
	// projection composed them: the snapshot route and the page do, a
	// mutation's fresh projection does not.
	Marks *wallMarks `json:"marks,omitempty"`
	// Pill is the readiness pill's facts (SI-368 (2)), when the projection
	// composed them, or the instance's fixed reason (SI-364 (3)); a
	// mutation's fresh projection on a serving-root wall carries none, so
	// the pill stays as the last poll left it (SI-362 (2)).
	Pill *wallPill `json:"pill,omitempty"`
	// Uncommitted is the Commit and push fragment (SI-368 (8);
	// wallUncommittedFragment) rendered from Git's changes summary, on the
	// authoring wall only. It needs no readiness load, so every snapshot
	// of that wall carries it, a mutation's fresh one included, and the
	// revision covers it.
	Uncommitted string `json:"uncommitted,omitempty"`

	// bar is the top bar's facts the region and Posture render from.
	// Not on the wire: the revision hashes it explicitly, so the accepted
	// head and ahead/behind move the token wherever the posture renders.
	bar barFacts
	// readiness is the composed readiness's digest (asdView's
	// readinessRevision), which the revision hashes; "" when the
	// projection loaded no readiness.
	readiness string
}

type asdExpectedWire struct {
	Checkout string `json:"checkout"`
	Branch   string `json:"branch"`
	Head     string `json:"head"`
}

// openProjection opens one application projection's reads of the served
// checkout (Wave 6 §5.3; ledger SI-356): one read session, and one
// accepted-HEAD resolution every consumer reads at. The caller defers the
// release; nothing either holds outlives it (co-2).
func (s *boardSpecServer) openProjection(ctx context.Context) (context.Context, func()) {
	ctx, release := gitx.WithReadSession(ctx, s.root)
	return specstate.WithAcceptedHead(ctx, s.root), release
}

// wallRefresh is one projection of the wall: its loaded projection and
// snapshot and — when the refresh composes it — the spec's readiness,
// derived in the same read session against the same accepted HEAD, and
// the marks derived from it.
type wallRefresh struct {
	proj *BoardProjection
	git  *boardGitState
	asd  *asdView
	snap *asdSnapshot
	// readiness is the readiness snapshot when the refresh composed it
	// and the loader derived it; readinessErr is the loader's failure.
	// Both are zero when the refresh loaded no readiness.
	readiness    *readinesspilot.Snapshot
	readinessErr error
}

// readinessRevision digests what a composed refresh read beyond the
// wall's own projection: the readiness it loaded, or the load's failure,
// and the marks and pill derived from it (Wave 6 §5.1; SI-360 (2);
// SI-368 (2)).
func readinessRevision(readiness *readinesspilot.Snapshot, readinessErr error, marks *wallMarks, pill *wallPill) (string, error) {
	failure := ""
	if readinessErr != nil {
		failure = readinessErr.Error()
	}
	return canonjson.Digest(struct {
		Readiness        *readinesspilot.Snapshot
		ReadinessFailure string
		Marks            *wallMarks
		Pill             *wallPill
	}{readiness, failure, marks, pill})
}

// projectWallRefresh is the wall's one application projection per
// conditional refresh (Wave 6 §5.3; ledger SI-356): one read session for
// the checkout and one accepted-HEAD resolution
// (specstate.WithAcceptedHead) around everything the refresh derives, so
// the wall's own projection, its badges and — when composeReadiness is
// set — the spec's readiness all read at the one commit id, enumerate the
// accepted tree at most once between them, and resolve nothing again; its
// snapshot's revision covers all of it. The wall's snapshot route and page
// compose (SI-360 (2)); a plain refresh is the wall alone. Nothing
// outlives the call (co-2).
func (s *boardSpecServer) projectWallRefresh(ctx context.Context, name string, composeReadiness bool) (wallRefresh, error) {
	ctx, release := s.openProjection(ctx)
	defer release()
	out, err := s.composeWall(ctx, name, composeReadiness)
	if err != nil {
		return wallRefresh{}, err
	}
	out.snap = newASDSnapshot(out.proj, out.git, out.asd)
	return out, nil
}

// composeWall loads the wall under ctx — which carries the caller's read
// session and pinned accepted HEAD — and, when composeReadiness is set,
// composes its readiness marks (SI-360): on a wall served from the
// serving root, the spec's readiness loaded in that session and the marks
// and the pill (SI-368 (2)) derived from that one load, which the
// snapshot's revision then covers. On a wall
// whose marks are fixed for this server instance (instanceMarks: a /b/
// wall whose branch is not the serving root's, or no loader wired) the
// load already carries them and nothing more is loaded; the snapshot's
// own revision determines them (SI-362 (1)).
func (s *boardSpecServer) composeWall(ctx context.Context, name string, composeReadiness bool) (wallRefresh, error) {
	proj, git, asd, err := s.loadASD(ctx, name)
	if err != nil {
		return wallRefresh{}, err
	}
	out := wallRefresh{proj: proj, git: git, asd: asd}
	if !composeReadiness || s.instanceMarks() != nil {
		return out, nil
	}
	readiness, err := s.readinessLoader.Load(ctx, "spec/"+name)
	if err != nil {
		out.readinessErr = err
	} else {
		out.readiness = &readiness
	}
	in := wallMarksInputFor(name, proj, asd, out.readiness, out.readinessErr)
	marks, pill := deriveWallMarks(in), deriveWallPill(in)
	asd.Marks, asd.Pill = &marks, &pill
	if asd.readinessRevision, err = readinessRevision(out.readiness, out.readinessErr, asd.Marks, asd.Pill); err != nil {
		return wallRefresh{}, fmt.Errorf("workbench: the composed refresh's revision: %w", err)
	}
	return out, nil
}

// loadSnapshot builds one complete snapshot: one composed page projection
// (loadASD) rendered once, then the revision token over the whole
// serialized content. It loads no readiness, so its marks are this
// instance's fixed ones, or none on a serving-root wall (SI-364 (3)).
func (s *boardSpecServer) loadSnapshot(ctx context.Context, name string) (*asdSnapshot, error) {
	proj, git, asd, err := s.loadASD(ctx, name)
	if err != nil {
		return nil, err
	}
	return newASDSnapshot(proj, git, asd), nil
}

// newASDSnapshot builds one snapshot of a loaded wall: the region, the top
// bar's facts (specBarFacts, the same pure function of p and asd the
// region's posture header renders from), the posture fragment rendered
// from them, and the revision over all of it. Three callers each build
// their own from their own load: the /snapshot route (loadSnapshot), the
// mutation response's fresh projection (writeMutationOutcome, which
// carries its HTML, posture, and revision), and the page, which embeds
// its revision and puts its facts in the template's view data.
func newASDSnapshot(p *BoardProjection, git *boardGitState, asd *asdView) *asdSnapshot {
	bar := specBarFacts(p, asd)
	snap := &asdSnapshot{
		HTML:        renderBoardRegion(p, git, asd),
		Posture:     asdPostureHTML(&bar),
		BaseDigest:  asd.BaseDigest,
		BaseSpecB64: asd.BaseSpecB64,
		Git:         git,
		Expected:    asdExpectedWire{Checkout: asd.ExpectedCheckout, Branch: asd.ExpectedBranch, Head: asd.ExpectedHead},
		Marks:       asd.Marks,
		Pill:        asd.Pill,
		Uncommitted: wallUncommittedFragment(p, git),
		bar:         bar,
		readiness:   asd.readinessRevision,
	}
	snap.Revision = snapshotRevision(snap)
	return snap
}

// snapshotRevision digests every rendered fact of one snapshot (the
// revision field itself excluded), the top bar's facts explicitly (SI-323
// (3)), the Commit and push fragment where the wall has one (SI-368 (8)),
// and, on a refresh that loaded readiness, that readiness and the marks
// derived from it (SI-360 (2)) — omitted otherwise, so a snapshot that
// loaded none keeps the token it always had. Deterministic: the render is
// a pure function of store state, and the machine fields are exact copies
// of it.
func snapshotRevision(snap *asdSnapshot) string {
	h := sha256.New()
	enc := json.NewEncoder(h)
	_ = enc.Encode(struct {
		HTML        string          `json:"html"`
		Posture     string          `json:"posture"`
		Bar         barFacts        `json:"bar"`
		BaseDigest  string          `json:"base_digest"`
		BaseSpecB64 string          `json:"base_spec_b64"`
		Git         *boardGitState  `json:"git"`
		Expected    asdExpectedWire `json:"expected"`
		Uncommitted string          `json:"uncommitted,omitempty"`
		Readiness   string          `json:"readiness,omitempty"`
	}{snap.HTML, snap.Posture, snap.bar, snap.BaseDigest, snap.BaseSpecB64, snap.Git, snap.Expected, snap.Uncommitted, snap.readiness})
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
