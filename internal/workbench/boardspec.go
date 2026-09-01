// The v1 board (05 §Workbench, R4 "Board as projection"): GET
// /board/spec/{name} renders the deterministic projection of a spec —
// the spec document is the source of truth; the board is a view of it,
// plus the annotation layer's scratch tier and, under an open spec-MR,
// the review-comment mirror. The v0 board page (board.go) is superseded
// but stays reachable at /board/{key} for grandfathered v0 board.json
// state (R4-I-9: history is never rewritten).
package workbench

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/wallbadge"
)

// StateResolver is the board loader's effective-lifecycle-state port
// (04 §port pattern: defined at the consumer): the served spec's mode and
// display status derive from specstate's git-derived verdict, never from a
// persisted status: field (merge-signaled acceptance — a feature/story may
// omit status entirely). specstate.Projector satisfies it; package tests
// may inject a fake to characterize the mode mapping without git.
type StateResolver interface {
	Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error)
}

// Deps carries the workbench's injected collaborators (04 §port
// pattern: interfaces defined at the consumer, wired by the caller).
type Deps struct {
	// CommentFeed is review mode's comment source. nil means no LIVE feed
	// is wired: either no forge is configured (the board renders
	// authoring/read-only purely from branch state, silently — legitimate)
	// or a forge IS configured but unreachable, in which case the caller
	// sets ReviewUnavailable below so the board discloses rather than
	// silently omits the review input (I-1(b)).
	CommentFeed CommentFeed

	// ReviewUnavailable, when non-empty, is the disclosed reason a
	// configured forge could not be reached to build a live CommentFeed
	// (e.g. named in verdi.yaml but no credentials). The board renders a
	// visible notice in its chrome — review or authoring mode alike —
	// distinguishing "configured but unavailable" (disclosed) from "no
	// forge configured" (silent). Only the caller that knows the manifest
	// (cmd/verdi's serve.go/mcp.go) can set it; unit tests wiring a feed
	// directly leave it empty.
	ReviewUnavailable string

	// Disclosures carries the serving process's own disclosed context as
	// structured seam values (e.g. the review-feed-unavailable state
	// behind ReviewUnavailable, as its Disclosure rather than its rendered
	// line) for the /disclosures page (spec/disclosures-panel ac-1): the
	// page appends these to its fresh per-render enumeration, so
	// process-context disclosures and store-derived ones read in the one
	// view. Only the caller that computes that context (cmd/verdi's
	// serve.go) can supply them.
	Disclosures []disclosure.Disclosure

	// Model is the store's resolved operating model
	// (spec/vocabulary-surfaces ac-2): board renders resolve their class
	// and state display words through it. nil means route registration
	// fills it from the store root once (store.Open at construction —
	// never per render); a store whose model cannot be resolved renders
	// bare ids, exactly like a model with no renames.
	Model *model.Model

	// SupersessionCandidates is the pending-supersession wall badge's
	// forge access (spec/badge-computes ac-3) — a consumer-defined port
	// (wallbadge.SupersessionCandidateLoader, 04 §port pattern) so this
	// package never imports internal/forge, exactly like CommentFeed
	// above. nil means no forge is configured: every pending-
	// supersession outcome on this checkout renders as a disclosed-
	// unproven notice rather than a badge (ac-3's three-valued outcome)
	// — never silently "not flagged". Only the caller that builds the
	// real forge-backed adapter (cmd/verdi's serve.go) sets it.
	SupersessionCandidates wallbadge.SupersessionCandidateLoader

	// DiagramVerifier is the diagram editor's verification-rail seam
	// (spec/board-editor dc-4, 04 §port pattern): the rail consumes the
	// extractor's tier and findings through this interface and never
	// computes them. nil means no extractor is wired — the rail renders
	// its disclosed verification-unavailable state, and nothing on the
	// editor blocks (verification informs, never gates).
	DiagramVerifier DiagramVerifier

	// Readiness is the Wave 3.5 pilot's startup snapshot: derived once by
	// the caller (cmd/verdi's serve path via readinesspilot.Derive) and
	// injected immutable — the cockpit page renders it verbatim and never
	// recomputes readiness. nil means the pilot is not wired: GET
	// /readiness discloses that honestly with a 503 page rather than
	// rendering anything vacuous.
	Readiness *readinesspilot.Snapshot

	// Design is the ASD application bridge (Wave 6 Task 2): the one typed
	// application core (internal/designapp, injected by cmd/verdi's serve
	// wiring) behind a consumer-owned port (boardspecdesign.go). Every
	// domain board mutation and every api/<application-operation> action
	// routes through it. nil means the design service is not wired: the
	// six design actions disclose that operationally and the shell renders
	// the honest unwired posture — never a local fallback interpretation.
	Design DesignBridge
}

// boardSpecServer holds the board's dependencies for one store root.
type boardSpecServer struct {
	root string
	feed CommentFeed

	// supersession is the pending-supersession wall badge's forge access
	// (Deps.SupersessionCandidates, see that field's doc comment). nil is
	// a fully valid zero value, mirroring feed above.
	supersession wallbadge.SupersessionCandidateLoader

	// model is the store's resolved operating model (Deps.Model — set at
	// construction, the manifest-derived-config pattern this struct's
	// other fields already follow; never re-opened per render). nil
	// renders bare ids everywhere (spec/vocabulary-surfaces' fallback).
	model *model.Model

	// reviewUnavailable, when non-empty, is a disclosed reason the review
	// feed is CONFIGURED (a forge is named in verdi.yaml) but cannot be
	// consulted — no credentials to build a live adapter at startup. The
	// board renders a visible notice in its chrome rather than silently
	// reading the missing input as "not under review" (I-1(b): a
	// configured-but-unavailable forge is disclosed, never silent;
	// constitution 2/10). Empty means either no forge is configured (silent
	// not-under-review is legitimate) or a live feed is wired.
	reviewUnavailable string

	// state resolves the served spec's effective lifecycle state (the
	// StateResolver port above). nil means production: specstate's real
	// projector, constructed lazily per load — the same posture the other
	// nil-meaning-production fields here take. Package tests inject fakes
	// to characterize the mode mapping for every state without git.
	state StateResolver

	// fixedBranch, when non-empty, marks this instance as a per-branch
	// draft board (spec/draft-boards): it serves exactly one branch's
	// managed worktree at that branch's own /b/ address, so the branch IS
	// the address. The branch switcher's git-switch action refuses on such
	// an instance — switching the worktree underneath would break the
	// branch<->worktree mapping the worktree-manager seam owns (its reuse
	// contract keys purely on the deterministic path) and mutate the tree
	// under any other tab on the same address (feature dc-1's
	// no-surprise-mutation law). The serving checkout's own instance
	// leaves this empty and keeps today's switch semantics.
	fixedBranch string

	// design is the ASD application bridge (Deps.Design — see that field's
	// doc comment). nil means the design service is not wired.
	design DesignBridge

	// checkoutOnce/checkoutCanonical/checkoutErr cache the kernel's
	// canonical checkout-path resolution for this root (stable for the
	// server's lifetime) so per-poll renders stop re-resolving it.
	checkoutOnce      sync.Once
	checkoutCanonical string
	checkoutErr       error

	// capsMu/capsCache memoize one spec's capabilities consultation per
	// exact fact key (branch, worktree HEAD, current spec digest, policy
	// tree stamp): the conditional 2s poll re-derives capabilities only
	// when one of the facts they depend on can have moved, instead of
	// re-running the full identity/state/policy resolution every tick.
	capsMu    sync.Mutex
	capsCache map[string]capsCacheEntry

	// writeMu serializes board MUTATIONS within this process. D3's
	// process-level writer lock (I-12) keeps other processes out, but the
	// board's HTTP handlers run as concurrent goroutines against the same
	// files; without this, two overlapping read-modify-write actions
	// (the designapp mutation transaction, actionPosition on layout.json, the boardio
	// full-file JSONL rewrites) could lose an update (last writer wins).
	// Atomic temp+rename already prevents a torn file, so this closes the
	// remaining intra-process lost-update window (M-2). Reads (page/
	// fragment) do not take it: atomic rename makes every read see one
	// whole file, old or new.
	writeMu sync.Mutex
}

var specNameRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// specDir is the spec's directory in the working tree. The board
// projects the ACTIVE working-tree state (authoring edits what the next
// commit will contain), so only specs/active/ is served.
func (s *boardSpecServer) specDir(name string) string {
	return filepath.Join(s.root, ".verdi", "specs", "active", name)
}

// boardGitState is the git half of the page model: what the affordance
// shows (05 §Workbench: indicator, commit/push, branch switcher).
type boardGitState struct {
	Branch        string   `json:"branch"`
	DefaultBranch string   `json:"defaultBranch"`
	Branches      []string `json:"branches"`
	Dirty         bool     `json:"dirty"`
}

// loadBoard assembles the projection's four inputs and the git state. The
// third return value is the review-feed disclosure ALONE (I-1(b)'s three
// states — configured-and-live silent, configured-but-unavailable
// disclosed, or not-configured silent), separated out from proj.Notices
// (which also folds in unrelated chrome banners like an assumed default
// branch) so a caller that wants ONLY the review state — get_board
// (internal/mcpserve), via the exported LoadProjection below — can surface
// it as its own field, matching list_annotations' review_unavailable
// pattern (commit 1348e79) rather than parsing prose notices.
func (s *boardSpecServer) loadBoard(ctx context.Context, name string) (*BoardProjection, *boardGitState, string, *boardLoadExtras, error) {
	if !specNameRe.MatchString(name) {
		return nil, nil, "", nil, ErrBoardNotFound
	}
	raw, err := os.ReadFile(filepath.Join(s.specDir(name), "spec.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, "", nil, ErrBoardNotFound
		}
		return nil, nil, "", nil, fmt.Errorf("workbench: reading spec %s: %w", name, err)
	}
	fmBytes, bodyBytes, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("workbench: spec %s: %w", name, err)
	}
	fm, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("workbench: spec %s: %w", name, err)
	}

	stored, err := boardlayout.ReadFile(s.specDir(name))
	if err != nil {
		return nil, nil, "", nil, err
	}
	annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(s.root))
	if err != nil {
		return nil, nil, "", nil, err
	}

	// The review feed is NON-BLOCKING on every render (I-2, 04 §Semantics'
	// degradation posture: never block rendering). A configured-but-erroring
	// feed degrades to a disclosed notice and underReview=false — authoring
	// and read-only boards render fully without a feed; a review board
	// renders the projection plus the disclosure. The startup-time
	// disclosure (forge configured but no credentials, s.reviewUnavailable)
	// seeds the notice; a render-time transport error overrides it with the
	// live reason. Both are rendered through the same internal/disclosure
	// seam and the same review-feed source, so the override changes which
	// cause is named — never the vocabulary a reader has learned
	// (spec/disclosure-seam-v2 ac-1/ac-2).
	var comments []MRComment
	underReview := false
	reviewNotice := s.reviewUnavailable
	if s.feed != nil {
		c, ur, ferr := s.feed.ListMRComments(ctx, name)
		if ferr != nil {
			// Configured AND reachable enough to attempt, but the call
			// failed: disclose, never silence, never a 500 (I-1(b)/I-2).
			reviewNotice = disclosure.Render(disclosure.ReviewUnavailableTransport(ferr))
		} else {
			comments, underReview = c, ur
		}
	}

	git, gitNotice, err := s.gitState(ctx)
	if err != nil {
		return nil, nil, "", nil, err
	}

	// The spec's effective lifecycle state (merge-signaled acceptance),
	// resolved HERE — the I/O loader — and passed inward as plain values:
	// buildProjection stays a pure function and never executes Git. The
	// candidate is the working tree's exact bytes at the spec's canonical
	// store path, compared against what the default branch holds.
	st, err := s.resolveState(ctx, name, raw)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("workbench: resolving effective state for %s: %w", name, err)
	}

	// Mode is keyed by EFFECTIVE state plus branch state, never by a
	// persisted status: field (a feature/story may omit one entirely):
	// a NEW PROPOSED spec on a design branch is the live authoring wall;
	// everything else fails closed to the sealed read-only document —
	// accepted-pending-build (exact on default), superseded, closed, and
	// unproven alike (an unprovable state is never editable; its
	// disclosures surface as board notices below rather than silence).
	// Relation==RelationNew is load-bearing (final fix wave I6): a
	// DIVERGED candidate also resolves Proposed, but its bytes are a
	// modified ACCEPTED revision — a frozen legacy spec hand-edited in the
	// working tree — which VL-010 refuses at merge; opening an editable
	// authoring wall over it invites edits no merge can legally accept, so
	// it renders read-only with a divergence notice instead (below).
	mode := modeReadOnly
	switch {
	case underReview:
		// A spec with an open spec-MR: the board is a mirror of the MR
		// (05 §Workbench "Review").
		mode = modeReview
	case st.State == specstate.Proposed && st.Relation == specstate.RelationNew && git.Branch != "" && git.Branch != git.DefaultBranch:
		mode = modeAuthoring
	}
	if mode != modeReview {
		comments = nil // the feed is a review-mode input only
	}

	proj, err := buildProjection(name, fm, bodyBytes, stored, annotations, comments, mode, string(st.ArtifactStatus()))
	if err != nil {
		return nil, nil, "", nil, err
	}
	// The state resolution's own disclosures (an unproven default branch,
	// an incomplete corpus scan, a legacy-status migration note) render in
	// the board chrome — never silently swallowed (three-valued honesty:
	// unproven is disclosed, not dressed as either mode's certainty).
	proj.Notices = append(proj.Notices, st.Disclosures...)
	// A diverged candidate's read-only render names WHY it is not the
	// authoring wall (I6): the working tree holds a modified accepted
	// revision, which VL-010 refuses — silence here would read as an
	// inexplicably sealed board.
	if st.State == specstate.Proposed && st.Relation == specstate.RelationDiverged {
		proj.Notices = append(proj.Notices, fmt.Sprintf(
			// vocab:identity — non-vocabulary homograph: the forge's merge request/merge gate, never the `merge` lifecycle transition word
			"spec/%s's working-tree bytes diverge from the accepted revision on the default branch — a modified accepted revision (VL-010 refuses it at merge); the board renders read-only. Start a new proposal (a successor spec) instead of editing the accepted revision in place", name))
	}
	// Display vocabulary (spec/vocabulary-surfaces ac-2): resolved-model
	// enrichment on the I/O layer, exactly like attachObligations below —
	// the pure projector stays model-free.
	proj.applyModelVocabulary(s.model)
	// Obligations are a store-derived enrichment, loaded and attached HERE
	// (the I/O layer) rather than inside the pure projector — the same
	// posture proj.Notices takes below (spec/obligation-wall ac-2/dc-3).
	if err := attachObligations(proj, s.root, name, fm); err != nil {
		return nil, nil, "", nil, err
	}
	// Wall badges (spec/badge-computes dc-1): the SAME store-derived I/O
	// enrichment posture as attachObligations above — runs after
	// buildProjection, never inside it — so the full page, the post-
	// mutation fragment, and get_board's LoadProjection all see the same
	// badges (ac-1).
	if err := attachBadges(ctx, proj, s.root, name, raw, fm, s.supersession, s.model); err != nil {
		return nil, nil, "", nil, err
	}
	// s.fixedBranch distinguishes the two board modes the one call site
	// serves: "" for the serving checkout's unprefixed board, the design
	// branch for a per-branch draft board instance — so the editor link
	// carries the operator's OWN board path (spec/tool-view-exit dc-2 / ADJ-38).
	attachDiagramEditorHrefs(proj, s.root, name, s.fixedBranch)
	// Family navigation (spec/family-board-links): the story-to-feature
	// affordance, the feature-stub-to-story link(s), and the live
	// in-between disclosure — the SAME I/O-enrichment posture as every
	// attach* call above, run last only because it is the newest of the
	// four; order among them is otherwise immaterial (each touches its
	// own disjoint fields). s.fixedBranch threads through exactly as it
	// does into attachDiagramEditorHrefs above (ADJ-70): a per-branch
	// board's family links stay inside the branch they resolved from.
	if err := attachFamilyLinks(ctx, proj, s.root, s.fixedBranch); err != nil {
		return nil, nil, "", nil, err
	}
	// The creation form's field descriptors (spec/creation-form ac-2/ac-3):
	// the same I/O-enrichment posture as every attach* call above. Only a
	// sealed accepted-pending-build feature wall carries the affordance
	// (the create action's own guard, mirrored at render); a template that
	// cannot be resolved or enumerated degrades to a disclosed notice and
	// no affordance — never silence, never a broken board.
	if proj.Class == string(artifact.ClassFeature) && proj.Status == "accepted-pending-build" {
		if _, fields, cerr := s.createFormFields(); cerr != nil {
			proj.Notices = append(proj.Notices, "creation form unavailable: "+cerr.Error())
		} else {
			proj.CreateFields = fields
		}
	}
	if reviewNotice != "" {
		proj.Notices = append(proj.Notices, reviewNotice)
	}
	if gitNotice != "" {
		proj.Notices = append(proj.Notices, gitNotice)
	}
	return proj, git, reviewNotice, &boardLoadExtras{raw: raw, fm: fm, state: st}, nil
}

// boardLoadExtras carries the exact already-decoded inputs loadBoard
// consumed, so the ASD view (buildASDView) is a pure function of them
// rather than a second read/decode/resolve pass.
type boardLoadExtras struct {
	raw   []byte
	fm    *artifact.SpecFrontmatter
	state specstate.Result
}

// loadASD is the ASD page projection: one loadBoard plus the ASD rendered
// facts (posture header, shell, capabilities view, client mutation
// facts). The page, fragment, snapshot, and mutation-response renders all
// come from exactly this one composed projection.
func (s *boardSpecServer) loadASD(ctx context.Context, name string) (*BoardProjection, *boardGitState, *asdView, error) {
	proj, git, _, extras, err := s.loadBoard(ctx, name)
	if err != nil {
		return nil, nil, nil, err
	}
	asd, err := s.buildASDView(ctx, name, proj, git, extras.raw, extras.fm, extras.state)
	if err != nil {
		return nil, nil, nil, err
	}
	return proj, git, asd, nil
}

// attachObligations enriches a STORY board's AC cards with their evidence
// obligations (spec/obligation-wall ac-2), so what each AC demands is read on
// the wall itself (feature co-3, legible-without-the-sidecar) rather than
// recovered by opening the obligation file. For each evidence kind an AC
// DECLARES it projects onto that card either the kind's authored obligation
// (title + prose) or a disclosed "no obligation" marker — the wall-receipts
// posture (dc-2): the read surface DISCLOSES, it never refuses; the
// activation gate (obligation-gate) is what refuses at accept, so a draft in
// progress renders legibly.
//
// Obligations are loaded by (spec-name, ac-id) through the ONE reader both
// this surface and `verdi matrix` consume (evidence.Obligations, dc-1: "a
// small loader ... both surfaces consume it, not two readers"). Genuine
// absence (no obligation authored yet for a declared kind) is the ordinary
// case and reads as Present=false, never an error; only a malformed
// obligation on disk is a surfaced (operational) error — the loader's own
// three-valued posture, propagated here as loadBoard's error, never silently
// swallowed.
//
// This is a no-op on any non-story wall: obligations attach to STORY
// acceptance criteria (a feature AC wears its coverage receipt instead), so
// gating on class here mirrors the projection's own feature/story split. It
// runs AFTER buildProjection — the projector stays a pure function of its
// four in-memory inputs; this store-derived enrichment lives in the I/O
// layer, exactly like proj.Notices.
func attachObligations(proj *BoardProjection, root, specName string, fm *artifact.SpecFrontmatter) error {
	if fm.Class != artifact.ClassStory {
		return nil
	}
	declaredKinds := make(map[string][]artifact.EvidenceKind, len(fm.AcceptanceCriteria))
	for _, ac := range fm.AcceptanceCriteria {
		declaredKinds[ac.ID] = ac.Evidence
	}
	for i := range proj.Cards {
		c := &proj.Cards[i]
		if boardlayout.ZoneKind(c.Kind) != boardlayout.ZoneAC {
			continue
		}
		kinds := declaredKinds[c.ID]
		if len(kinds) == 0 {
			continue
		}
		obs, err := evidence.Obligations(root, specName, c.ID)
		if err != nil {
			return fmt.Errorf("workbench: obligations for %s %s: %w", specName, c.ID, err)
		}
		views := make([]obligationView, 0, len(kinds))
		for _, k := range kinds {
			ov := obligationView{Kind: string(k)}
			if ob, ok := obs[k]; ok {
				ov.Present = true
				ov.Title = ob.Title
				ov.Body = ob.Body
			}
			views = append(views, ov)
		}
		c.Obligations = views
	}
	return nil
}

// LoadProjection computes the deterministic board projection for a spec —
// the SAME four-input computation (loadBoard, above) that renders the HTTP
// board page — exposed for a non-HTTP caller (05 §MCP server's get_board
// row: "the same element taxonomy, computed badges, and mode-appropriate
// annotations a human sees in `verdi serve`, so agents work from what
// humans see rather than a second-hand summary"). This is the ONLY
// entrypoint mcpserve's get_board tool uses — it never reimplements the
// projection. feed may be nil (no live review population, matching a nil
// Deps.CommentFeed); reviewUnavailable carries the same configured-but-
// unreachable disclosure Deps.ReviewUnavailable does. superseLoader mirrors
// Deps.SupersessionCandidates — nil is a fully valid "no forge configured"
// zero value (spec/badge-computes ac-3's disclosed-unproven case). The
// returned reviewNotice is the review-feed disclosure alone (see loadBoard's
// doc comment) — get_board surfaces it as its own review_unavailable field,
// never folded silently into the generic notices a human board's chrome
// shows.
func LoadProjection(ctx context.Context, root, name string, feed CommentFeed, reviewUnavailable string, superseLoader wallbadge.SupersessionCandidateLoader) (proj *BoardProjection, reviewNotice string, err error) {
	s := &boardSpecServer{root: root, feed: feed, reviewUnavailable: reviewUnavailable, supersession: superseLoader}
	proj, _, reviewNotice, _, err = s.loadBoard(ctx, name)
	return proj, reviewNotice, err
}

// resolveState projects the served spec's effective lifecycle state
// through the StateResolver port: the working tree's exact bytes at the
// spec's canonical active-zone path, classified against the default
// branch. nil s.state means production (specstate's real projector) —
// the same nil-is-production posture the struct's other fields take.
func (s *boardSpecServer) resolveState(ctx context.Context, name string, raw []byte) (specstate.Result, error) {
	resolver := s.state
	if resolver == nil {
		resolver = specstate.NewProjector()
	}
	return resolver.Resolve(ctx, s.root, specstate.Candidate{Path: store.ActiveSpecRelPath(name), Content: raw})
}

// unresolvedDefaultBranchNotice is the board chrome's ONE story for a
// repository whose default branch cannot be resolved (fix round 2,
// finding 3): what was tried — specstate.ResolveDefaultBranch's D6-6
// precedence chain, the same resolution the effective-state projection
// itself uses, so the notice and the (unproven, read-only) render can
// never contradict each other — plus the remedy. Deliberately NO assumed
// fallback and no new configuration key: purely-local fail-closed
// behavior is design-mandated; the remedy is to make the default branch
// provable, never to guess one.
const unresolvedDefaultBranchNotice = "default branch could not be resolved: CI_DEFAULT_BRANCH is unset, origin/HEAD is not configured, and neither refs/remotes/origin/main nor refs/remotes/origin/master alone identifies it — the spec's effective lifecycle state is unproven and the board renders read-only. Remedy: set CI_DEFAULT_BRANCH, or run `git remote set-head origin <branch>` to configure origin/HEAD"

// gitState queries the working tree's branch and dirtiness. The default
// branch comes from specstate.ResolveDefaultBranch — the ONE shared
// resolution (CI_DEFAULT_BRANCH, then origin/HEAD, then the D6-6
// remote-tracking fallback) the effective-state projection also routes
// through, so the git panel and the projected lifecycle state always
// tell the same story. When it cannot resolve, DefaultBranch stays empty
// and the returned notice carries the honest unproven story plus its
// remedy (unresolvedDefaultBranchNotice) — never an assumed "main"
// beside a read-only render (M-4 as amended by fix round 2, finding 3:
// one consistent story, not two contradictory ones). Mode selection
// stays fail-closed either way: with no provable default branch,
// specstate projects Unproven and loadBoard's switch never reaches the
// authoring case.
func (s *boardSpecServer) gitState(ctx context.Context) (*boardGitState, string, error) {
	branch, err := gitx.CurrentBranch(ctx, s.root)
	if err != nil {
		return nil, "", err
	}
	def := ""
	notice := ""
	if resolved, ok := specstate.ResolveDefaultBranch(ctx, s.root); ok {
		def = resolved.Name
	} else {
		notice = unresolvedDefaultBranchNotice
	}
	dirty, err := gitx.StatusDirty(ctx, s.root)
	if err != nil {
		return nil, "", err
	}
	branches, err := gitx.LocalBranches(ctx, s.root)
	if err != nil {
		return nil, "", err
	}
	return &boardGitState{Branch: branch, DefaultBranch: def, Branches: branches, Dirty: dirty}, notice, nil
}

// ErrBoardNotFound distinguishes 404 from operational failures.
var ErrBoardNotFound = fmt.Errorf("workbench: no such spec board")

// boardSpecPageHandler answers GET /board/spec/{name}: the full page.
func (s *boardSpecServer) boardSpecPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		proj, git, asd, err := s.loadASD(r.Context(), r.PathValue("name"))
		if errors.Is(err, ErrBoardNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		out, err := renderBoardSpecPage(proj, git, asd)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out) // response body write; post-header error is unactionable
	}
}

// boardSpecFragmentHandler answers GET /board/spec/{name}/fragment: the
// re-rendered board region the client swaps in after every mutation, so
// the DOM is always the server's own projection — one renderer, no
// client-side duplicate.
func (s *boardSpecServer) boardSpecFragmentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		proj, git, asd, err := s.loadASD(r.Context(), r.PathValue("name"))
		if errors.Is(err, ErrBoardNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderBoardRegion(proj, git, asd)))
	}
}

// boardSpecSnapshotHandler answers GET /board/spec/{name}/snapshot: the
// conditional projection route (SI-165, SI-167). The response is one
// complete rendered projection plus its machine facts; the ETag carries
// the deterministic revision token and an unchanged If-None-Match answers
// 304 with no body, so a poll can leave the page — and its unsaved state
// — completely untouched. GET only; no polling result ever writes.
func (s *boardSpecServer) boardSpecSnapshotHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		snap, err := s.loadSnapshot(r.Context(), r.PathValue("name"))
		if errors.Is(err, ErrBoardNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		etag := `"` + snap.Revision + `"`
		w.Header().Set("ETag", etag)
		if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		writeJSON(w, http.StatusOK, snap)
	}
}
