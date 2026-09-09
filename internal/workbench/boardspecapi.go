package workbench

// The v1 board's write surface: POST /board/spec/{name}/api/{action}.
// Since Wave 6 Task 2 every DOMAIN spec write is a typed mutate_draft
// transaction through the one designapp application core
// (boardspecdesign.go) — the legacy splice path is deleted (AC-2: the
// migrated workbench retains no parallel interpretation of domain
// mutations). This file keeps only the pre-existing NON-domain
// affordances on their existing owners: annotation writes go to the
// mutable zone (boardio, the same owner MCP's add_annotation uses) and
// never dirty the spec tree; layout positions go to boardlayout; git
// acts are explicit rituals (gitx); scaffolding rides
// stubinstantiate/designscaffold; the obligation arm of sticky-graduate
// rides internal/evidence. Everything but stub-instantiate/create is
// authoring-mode-only: review is a mirror, read-only is a document.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/stubinstantiate"
)

// boardAPIRequest is the one strict-decoded body shape every action
// reads its fields from; unknown fields fail closed.
type boardAPIRequest struct {
	ID string `json:"id,omitempty"`
	// IDs is annotation-delete's batch form (Wave 6 Task 2): one gesture
	// deleting several mutable-zone records (a trashed reference card's
	// pin plus its threads) in one action — same owner, same semantics,
	// plural addressing.
	IDs     []string `json:"ids,omitempty"`
	Ref     string   `json:"ref,omitempty"`
	Text    string   `json:"text,omitempty"`
	From    string   `json:"from,omitempty"`
	To      string   `json:"to,omitempty"`
	Type    string   `json:"type,omitempty"`
	NewType string   `json:"newType,omitempty"`
	Note    string   `json:"note,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	X       float64  `json:"x,omitempty"`
	Y       float64  `json:"y,omitempty"`
	Message string   `json:"message,omitempty"`
	Branch  string   `json:"branch,omitempty"`
	// Name, Values, and ACs are the create action's inputs
	// (spec/creation-form ac-2): the new spec's kebab-case name, the
	// form's submitted values keyed by the enumerated field descriptors
	// (designscaffold.Fields — unknown keys refuse by name), and the
	// declared acceptance criteria the new story implements.
	Name   string            `json:"name,omitempty"`
	Values map[string]string `json:"values,omitempty"`
	ACs    []string          `json:"acs,omitempty"`
}

// boardAPIResponse reports the working tree's dirtiness after the
// action — the uncommitted-changes indicator's live signal.
type boardAPIResponse struct {
	Dirty bool `json:"dirty"`
}

// boardSpecAPIHandler answers POST /board/spec/{name}/api/{action}.
func (s *boardSpecServer) boardSpecAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.PathValue("name")
		action := r.PathValue("action")
		// The closed route/action grammar (SI-167): anything outside the
		// exact inventory union fails before ANY other work.
		if !boardActionInventory()[action] {
			http.NotFound(w, r)
			return
		}
		// The six application operations (design §3.2) have their own
		// strict transport and dispatch (boardspecdesign.go).
		if s.designActionHandler(w, r, name, action) {
			return
		}

		// Serialize every mutation against this server's other in-flight
		// mutations: each action is a read-modify-write of the working tree
		// or the mutable zone, and two racing writers would otherwise lose
		// an update (M-2). Held across loadBoard (the read half) through the
		// action's write so the projection an action edits cannot go stale
		// under a concurrent commit.
		s.writeMu.Lock()
		defer s.writeMu.Unlock()

		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "reading request body: "+err.Error())
			return
		}
		var req boardAPIRequest
		if err := artifact.DecodeStrictJSON(raw, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "malformed request: "+err.Error())
			return
		}

		proj, _, _, _, err := s.loadBoard(r.Context(), name)
		if errors.Is(err, ErrBoardNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// stub-instantiate and create are deliberately EXEMPT from the
		// authoring-only gate (spec/scoping-canvas ac-6's flagged judgment
		// call; create inherits the identical posture, spec/creation-form
		// ac-2): neither edits the SERVED spec at all — each scaffolds an
		// unrelated new story spec on a fresh, un-checked-out branch via
		// git plumbing — so an accepted-pending-build wall (permanently
		// sealed, never authoring) must still be able to run them. Their
		// own guard (class feature, status accepted-pending-build) is
		// enforced inside each action, against the wall's own state rather
		// than the generic writes-need-authoring-mode posture every other
		// action shares.
		if action != "stub-instantiate" && action != "create" && proj.Mode != modeAuthoring {
			// The parenthetical's state word is display and resolves
			// (L-M13a(6)); the mode word and the board name are the
			// route's own taxonomy/identity, kept bare.
			writeJSONError(w, http.StatusForbidden, fmt.Sprintf("board for %s is in %s mode; only an authoring board (%s spec on a design branch) accepts writes", name, proj.Mode, s.model.DisplayState(proj.Class, "draft")))
			return
		}

		ctx := r.Context()
		switch action {
		case "sticky":
			err = s.actionSticky(name, proj, req)
		case "sticky-graduate":
			// Only the obligation arm survives here (spec/obligation-
			// artifact ac-3, an evidence-artifact write through the one
			// shared internal/evidence seam). A spec-object graduation
			// (the object menu's ac/co/dc/oq) is a DOMAIN mutation and is
			// a typed mutate_draft transaction now (Wave 6 Task 2).
			if strings.HasPrefix(req.Kind, obligationGraduatePrefix) {
				err = s.actionObligationGraduate(ctx, name, proj, req)
			} else {
				err = fmt.Errorf("sticky-graduate graduates only evidence obligations (kind obligation:<for-kind>) now; graduating a sticky into a declared spec object is a typed mutate_draft transaction")
			}
		case "stub-instantiate":
			err = s.actionStubInstantiate(ctx, name, proj, req)
		case "create":
			err = s.actionCreate(ctx, name, proj, req)
		case "relates":
			err = s.actionRelates(ctx, name, proj, req)
		case "pin":
			err = s.actionPin(ctx, name, proj, req)
		case "annotation-delete":
			err = s.actionAnnotationDelete(name, proj, req)
		case "position":
			err = s.actionPosition(name, proj, req)
		case "sticky-position":
			err = s.actionStickyPosition(name, proj, req)
		case "git-commit":
			err = s.actionGitCommit(ctx, req)
		case "git-switch":
			s.actionGitSwitch(ctx, w, req)
			return
		default:
			// Unreachable: the inventory guard above already refused
			// every unknown action. Kept as the fail-closed backstop.
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		dirty, derr := gitx.StatusDirty(ctx, s.root)
		if derr != nil {
			writeJSONError(w, http.StatusInternalServerError, derr.Error())
			return
		}
		writeJSON(w, http.StatusOK, boardAPIResponse{Dirty: dirty})
	}
}

// declaredKindsOf indexes a projection's cards by id → kind.
func declaredKindsOf(proj *BoardProjection) map[string]string {
	kinds := make(map[string]string, len(proj.Cards))
	for _, c := range proj.Cards {
		kinds[c.ID] = c.Kind
	}
	return kinds
}

// stubKeyFor returns the declared stub's own "stub:<slug>" layout key when
// slug names one of proj.StubViews, else "" — the board id ↔ layout.json
// key mapping the position action and liveKeys share (round 5.5 dc-6).
func stubKeyFor(proj *BoardProjection, slug string) (string, bool) {
	for _, sv := range proj.StubViews {
		if sv.Slug == slug {
			return "stub:" + sv.Slug, true
		}
	}
	return "", false
}

// liveKeys is the full set of layout.json keys currently backed by
// something real on this board: every declared object id (declaredKindsOf)
// plus every declared stub's "stub:<slug>" key (round 5.5 dc-6 amendment:
// stubs are draggable now, mirroring how a stored object position works).
// It is the writer's live set for Prune (VL-018: a dangling key, object or
// stub, is a lint error the writer never persists).
func liveKeys(proj *BoardProjection) map[string]bool {
	live := make(map[string]bool, len(proj.Cards)+len(proj.StubViews))
	for id := range declaredKindsOf(proj) {
		live[id] = true
	}
	for _, sv := range proj.StubViews {
		live["stub:"+sv.Slug] = true
	}
	return live
}

// Sticky landing geometry: the rendered sticky footprint estimate
// (mirrors canvasMinHeight's) and the append gap.
const (
	stickyEstHeight = 150
	stickyLaneGap   = 24
)

// stickyLaneColumn maps a sticky's type to the wall band it files
// into: a question queues beneath the open-questions column it may
// graduate into, a decision-needed beneath the decisions, a story or
// spike proto-sticky parks in the stubs band it will typeset into
// (spec/scoping-canvas dc-6: "its parking spot a claim about where the
// stub will land"); comments and agent tasks take the scratch lane past
// the references.
func stickyLaneColumn(typ artifact.AnnotationType) boardlayout.ZoneColumn {
	var want boardlayout.ZoneKind
	switch typ {
	case artifact.AnnotationQuestion:
		want = boardlayout.ZoneOpenQuestion
	case artifact.AnnotationDecisionNeeded:
		want = boardlayout.ZoneDecision
	case artifact.AnnotationStory, artifact.AnnotationSpike:
		want = boardlayout.ZoneStub
	default:
		return boardlayout.ScratchColumn()
	}
	for _, c := range boardlayout.ZoneColumns() {
		if c.Kind == want {
			return c
		}
	}
	return boardlayout.ScratchColumn() // unreachable: zoneOrder covers all three
}

// stickyLanePosition appends a new sticky to the BOTTOM of its type's
// lane (owner directive): below every element whose footprint
// intersects the lane's band, or the lane's first slot when it is
// empty. Deterministic given the projection; the lane is only the
// landing spot — stickies drag anywhere afterwards.
func stickyLanePosition(proj *BoardProjection, typ artifact.AnnotationType) (float64, float64) {
	lane := stickyLaneColumn(typ)
	left := float64(lane.X)
	right := float64(lane.X + lane.Width)
	inLane := func(x, w float64) bool { return x < right && left < x+w }
	bottom := -1.0
	for _, c := range proj.Cards {
		if inLane(c.X, boardlayout.CardWidth) && c.Y+boardlayout.CardHeight > bottom {
			bottom = c.Y + boardlayout.CardHeight
		}
	}
	for _, rc := range proj.RefCards {
		if inLane(rc.X, boardlayout.CardWidth) && rc.Y+boardlayout.RefCardHeight > bottom {
			bottom = rc.Y + boardlayout.RefCardHeight
		}
	}
	for _, sv := range proj.StubViews {
		if inLane(sv.X, boardlayout.CardWidth) && sv.Y+boardlayout.StubCardHeight > bottom {
			bottom = sv.Y + boardlayout.StubCardHeight
		}
	}
	for _, st := range proj.Stickies {
		if inLane(st.X, boardlayout.CardWidth) && st.Y+stickyEstHeight > bottom {
			bottom = st.Y + stickyEstHeight
		}
	}
	if bottom < 0 {
		return left, boardlayout.ZoneOriginY
	}
	return left, bottom + stickyLaneGap
}

// annotationAuthor names the local author for board-created annotation
// records. The mutable zone is per-checkout state, so the OS user is
// honest attribution; "board" is the fallback.
func annotationAuthor() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "board"
}

// newAnnotation mints a fresh annotation record shell (a-<ULID> id,
// RFC3339 stamp). The id/timestamp are declared stamps on a mutable-zone
// record, not generated-artifact content.
func newAnnotation(typ artifact.AnnotationType, body string) (*artifact.Annotation, error) {
	id, err := artifact.NewAnnotationID()
	if err != nil {
		return nil, fmt.Errorf("workbench: minting annotation id: %w", err)
	}
	return &artifact.Annotation{
		ID:     id,
		TS:     time.Now().UTC().Format(time.RFC3339),
		Author: annotationAuthor(),
		Type:   typ,
		Body:   body,
		Status: artifact.AnnotationOpen,
	}, nil
}

// stickyCreatableTypes is the closed set of annotation types an author
// can pin as a free-floating sticky (02 §Record schemas; relates is a
// thread and review is the MR's voice — neither is sticky-creatable).
// story/spike (round 5.4) are NOT in this generic set — they are
// feature-class-only proto-stickies (protoStickyTypes below), gated
// separately since this set alone cannot see the wall's class.
var stickyCreatableTypes = map[artifact.AnnotationType]bool{
	artifact.AnnotationComment:        true,
	artifact.AnnotationQuestion:       true,
	artifact.AnnotationDecisionNeeded: true,
	artifact.AnnotationAgentTask:      true,
}

// protoStickyTypes is the scoping canvas's typed proto-sticky set (02
// §Record schemas, round 5.4, DC-5): legal ONLY on a feature-class wall
// (spec/scoping-canvas item 5a) — a story sticky's yarn reads as AC
// coverage, a spike sticky's as open-question resolution, neither of
// which means anything on a story wall.
var protoStickyTypes = map[artifact.AnnotationType]bool{
	artifact.AnnotationStory: true,
	artifact.AnnotationSpike: true,
}

// actionSticky: "Add sticky" — a free-floating sticky of the author's
// explicitly chosen type (owner UAT round 6, item 2: choosing is part
// of creating; nothing defaults silently, unknown types fail closed) in
// the annotation layer; it never dirties the spec working tree (05
// §Workbench "The scratch tier"). story/spike additionally require a
// feature-class wall (proj.Class, already carried by the projection) —
// a plain-language refusal everywhere else.
func (s *boardSpecServer) actionSticky(name string, proj *BoardProjection, req boardAPIRequest) error {
	if req.Text == "" {
		return fmt.Errorf("sticky requires text")
	}
	typ := artifact.AnnotationType(req.Type)
	if req.Type == "" {
		// vocab:identity — sticky/annotation TYPE enum values (wire)
		return fmt.Errorf("sticky requires a type (one of comment, question, decision-needed, agent-task, story, spike)")
	}
	if !stickyCreatableTypes[typ] && !protoStickyTypes[typ] {
		// vocab:identity — sticky/annotation TYPE enum values (wire)
		return fmt.Errorf("sticky type %q is not creatable (one of comment, question, decision-needed, agent-task, story, spike); fail closed", req.Type)
	}
	if protoStickyTypes[typ] && proj.Class != string(artifact.ClassFeature) {
		// The spoken class words are display and resolve (L-M13a(6));
		// the echoed sticky TYPE %q is a wire enum value — identity. The
		// class COMPARISON stays on bare ids.
		return fmt.Errorf("sticky type %q is only creatable on %s-class wall (the scoping canvas, 02 §Record schemas); this wall is class %s", req.Type, model.Indefinite(s.model.DisplayClass("feature")), s.model.DisplayClass(proj.Class))
	}
	a, err := newAnnotation(typ, req.Text)
	if err != nil {
		return err
	}
	x, y := stickyLanePosition(proj, typ)
	a.Board = &artifact.BoardAnchor{Story: name, X: x, Y: y}
	return boardio.AppendAnnotation(boardio.AnnotationsDir(s.root), boardio.AnnotationFileForBoard(store.RefSlug(name)), a)
}

// stubInstantiatePlaceholderStoryRef re-exports stubinstantiate's own
// PlaceholderStoryRef under this package's established name (a handful of
// other workbench call sites, e.g. actionCreate's own fallback, already
// reference the local identifier) — never a second, independently-typed
// constant that could drift from the shared package's own value.
const stubInstantiatePlaceholderStoryRef = stubinstantiate.PlaceholderStoryRef

// actionStubInstantiate scaffolds a declared stub's story (or spike) spec
// on a fresh design/<slug> branch, built entirely via git plumbing so the
// SERVING checkout's HEAD, working tree, and real index are never touched
// (spec/scoping-canvas ac-6) — the operator checks the new branch out
// themselves. Guarded by the wall's own class and status (class feature,
// status accepted-pending-build: "the owner's rule: implementations
// build accepted specs only") rather than the generic authoring-mode
// gate — see the handler's own comment on why this action is exempted
// from it.
//
// A thin wrapper over internal/stubinstantiate.Instantiate (spec/
// cli-creation ac-3, ledger L-N7): this action's own guard, stub lookup,
// link-building, template render, self-validation, and git-plumbing
// commit all moved into that shared package so `verdi design start
// --from-stub` calls the IDENTICAL core rather than a second CLI-side
// reimplementation — closing the ADJ-65 asymmetry at the mechanism. This
// function now only translates the board's own proj/req shapes into the
// shared core's plain arguments and back; every message and behavior is
// unchanged (the extraction is behavior-preserving, proven by this
// package's own existing handler tests passing unmodified).
func (s *boardSpecServer) actionStubInstantiate(ctx context.Context, name string, proj *BoardProjection, req boardAPIRequest) error {
	stubs := make([]artifact.Stub, len(proj.StubViews))
	for i, sv := range proj.StubViews {
		stubs[i] = artifact.Stub{Slug: sv.Slug, Spike: sv.Spike, Resolves: sv.Resolves, AcceptanceCriteria: sv.AcceptanceCriteria}
	}
	_, err := stubinstantiate.Instantiate(ctx, s.root, name, artifact.SpecClass(proj.Class), proj.Status, stubs, req.ID, s.model)
	return err
}

// createFormFields enumerates the story class's resolved template into
// the creation form's field descriptors (spec/creation-form ac-1/ac-2):
// ONE contract for the server-rendered form and the submitted-values
// validation — the two halves cannot drift. The store's own template
// override wins, exactly as LoadTemplate resolves everywhere else.
func (s *boardSpecServer) createFormFields() ([]byte, []designscaffold.Field, error) {
	cfg, err := store.Open(s.root)
	if err != nil {
		return nil, nil, fmt.Errorf("workbench: resolving store config: %w", err)
	}
	class, ok := cfg.Model.Classes[string(artifact.ClassStory)]
	if !ok {
		return nil, nil, fmt.Errorf("workbench: internal error: resolved model has no %q class", artifact.ClassStory)
	}
	tmpl, err := designscaffold.LoadTemplate(s.root, class.Template)
	if err != nil {
		return nil, nil, fmt.Errorf("workbench: %w", err)
	}
	fields, err := designscaffold.Fields(tmpl)
	if err != nil {
		return nil, nil, fmt.Errorf("workbench: enumerating template %s: %w", class.Template, err)
	}
	return tmpl, fields, nil
}

// normalizeOwners maps a submitted Owners value to the template's YAML
// flow-sequence position (owners: {{safe .Owners}}): names split on
// commas and trimmed, rendered as the [a, b] list literal; an
// already-bracketed value normalizes through the identical path (so
// "[]" and "[ , ]" refuse exactly like " , "). Zero names after
// normalization refuses naming the field and the required shape — the
// artifact rule is at least one owner.
func normalizeOwners(v string) (string, error) {
	inner := strings.TrimSpace(v)
	if strings.HasPrefix(inner, "[") && strings.HasSuffix(inner, "]") {
		inner = inner[1 : len(inner)-1]
	}
	var names []string
	for _, p := range strings.Split(inner, ",") {
		if p = strings.TrimSpace(p); p != "" {
			names = append(names, p)
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("the Owners field must list at least one owner — e.g. alice, bob (rendered as the list [alice, bob])")
	}
	return "[" + strings.Join(names, ", ") + "]", nil
}

// actionCreate scaffolds a free story spec from the creation form's
// submitted values (spec/creation-form ac-2) — stub-instantiate's
// sibling for a story no declared stub planned: same wall guards, same
// self-validate + CheckClass gate before any git object is written, same
// pure-plumbing branch cut. Fields are keyed by the enumerated
// descriptors of the story class's own resolved template (unknown keys
// refuse by name; the template's STATEMENT fields are required and
// refuse by name when empty — the field contract's other half, so no
// caller can mint a silent placeholder behind the ritual); implements
// edges bind to the caller-chosen declared acceptance criteria of this
// wall (at least one, each validated); unfilled INPUT fields fall back
// to the same disclosed placeholder defaults every other scaffold
// consumer uses, Owners normalizing through normalizeOwners first.
func (s *boardSpecServer) actionCreate(ctx context.Context, name string, proj *BoardProjection, req boardAPIRequest) error {
	if err := stubinstantiate.SealedFeatureWallGuard(artifact.SpecClass(proj.Class), proj.Status, "create", s.model); err != nil {
		return err
	}
	slug := req.Name
	if slug == "" {
		// The refusal speaks the class word as display prose (L-M13a(6)).
		return fmt.Errorf("create requires a kebab-case name for the new %s spec", s.model.DisplayClass("story"))
	}
	if !specNameRe.MatchString(slug) {
		return fmt.Errorf("spec name %q must be kebab-case (02 §Identity)", slug)
	}
	if _, err := os.Stat(store.ActiveSpecDir(s.root, slug)); err == nil {
		return fmt.Errorf("spec %s already exists under specs/active/ — pick another name", slug)
	}
	if _, err := os.Stat(store.ArchiveSpecDir(s.root, slug)); err == nil {
		return fmt.Errorf("spec %s already exists under specs/archive/ — names are unique across active and archived specs (guide 6.1)", slug)
	}
	// A plain-language pre-check on the branch (the form surfaces this
	// message verbatim); UpdateRef stays the atomic create-only guard.
	if _, err := gitx.RevParse(ctx, s.root, "refs/heads/design/"+slug); err == nil {
		return fmt.Errorf("branch design/%s already exists — the name is taken; check that branch out instead", slug)
	}

	tmpl, fields, err := s.createFormFields()
	if err != nil {
		return err
	}
	// Submitted values must key into the template's own enumerated
	// input/statement fields — the form and this validation share one
	// contract, so a drifted (or hand-crafted) client fails loudly by
	// name rather than silently dropping input.
	askable := make(map[string]bool, len(fields))
	for _, f := range fields {
		if f.Kind == designscaffold.FieldInput || f.Kind == designscaffold.FieldStatement {
			askable[f.Name] = true
		}
	}
	for key := range req.Values {
		if !askable[key] {
			return fmt.Errorf("value key %q is not a fillable field of the %s class's template (fields are enumerated from the template's own placeholders, guide 5.3)", key, s.model.DisplayClass("story"))
		}
	}

	// The chosen acceptance criteria become the story's real implements
	// edges — at least one, each a declared AC of this wall, never design
	// start's placeholder edge.
	if len(req.ACs) == 0 {
		return fmt.Errorf("create requires at least one acceptance criterion for the new %s to implement (choose from this wall's declared acceptance criteria)", s.model.DisplayClass("story"))
	}
	kinds := declaredKindsOf(proj)
	seen := map[string]bool{}
	var links []designscaffold.StoryLink
	for _, ac := range req.ACs {
		if kinds[ac] != string(boardlayout.ZoneAC) {
			return fmt.Errorf("%q is not a declared acceptance criterion on this wall", ac)
		}
		if seen[ac] {
			continue
		}
		seen[ac] = true
		links = append(links, designscaffold.StoryLink{Type: artifact.LinkImplements, Ref: "spec/" + name + "#" + ac})
	}

	// Required-ness is the same field contract's other half (judged-
	// create-statement-required-enforcement, adjudicated): every
	// STATEMENT field the resolved template enumerates must arrive
	// non-empty — the server refuses by name, mirroring the form's own
	// refusal copy, and never silently mints the TODO placeholder behind
	// the ritual. Deliberate TODO deferral is the CLI's explicit
	// --defer-statements contract (spec/creation-surfaces ac-3, plan Task
	// 12) with its disclosure line — never a silent server default.
	var missingStatements []string
	for _, f := range fields {
		if f.Kind == designscaffold.FieldStatement && strings.TrimSpace(req.Values[f.Name]) == "" {
			missingStatements = append(missingStatements, f.Name)
		}
	}
	if len(missingStatements) > 0 {
		return fmt.Errorf("create requires a non-empty %s — a work item with no stated problem or outcome is not an artifact yet; the creation surface collects these before it exists (guide 6.1), never a silent placeholder", strings.Join(missingStatements, " and "))
	}

	valueOr := func(key, fallback string) string {
		if v, ok := req.Values[key]; ok && v != "" {
			return v
		}
		return fallback
	}
	// Owners maps deterministically to the template's YAML list shape
	// (judged-create-owners-value-shape, adjudicated): natural input
	// normalizes, a bracketed literal passes through, and a value that
	// normalizes to zero names refuses HERE by name — never surfaced as
	// the self-validation's strict-decode internal error. Absent input
	// keeps the disclosed placeholder default (ac-3's unfilled-field
	// posture).
	owners := designscaffold.DefaultOwners
	if v, ok := req.Values["Owners"]; ok && strings.TrimSpace(v) != "" {
		owners, err = normalizeOwners(v)
		if err != nil {
			return err
		}
	}
	content, err := designscaffold.Render(tmpl, designscaffold.ScaffoldData{
		Ref:      "spec/" + slug,
		Title:    valueOr("Title", designscaffold.HumanizeName(slug)),
		Owners:   owners,
		StoryRef: valueOr("StoryRef", stubInstantiatePlaceholderStoryRef),
		Problem:  req.Values["Problem"],
		Outcome:  req.Values["Outcome"],
		Links:    links,
	})
	if err != nil {
		return fmt.Errorf("workbench: rendering the %s class's template: %w", artifact.ClassStory, err)
	}

	// Self-validate + CheckClass before ever touching the object database
	// — stub-instantiate's inherited posture (K1): class.Template is
	// DATA, so a misconfigured binding or override can render the wrong
	// class and still strict-decode clean.
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		return fmt.Errorf("workbench: create scaffold failed self-validation: %w", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return fmt.Errorf("workbench: create scaffold failed self-validation: %w", err)
	}
	if err := designscaffold.CheckClass(spec, artifact.ClassStory); err != nil {
		return fmt.Errorf("workbench: create scaffold failed self-validation (the resolved template's class binding is misconfigured): %w", err)
	}

	msg := fmt.Sprintf("create: scaffold spec/%s from the creation form of spec/%s", slug, name)
	return stubinstantiate.CommitScaffoldBranch(ctx, s.root, slug, content, msg)
}

// relatesTarget builds a relates endpoint's pinned target record.
func (s *boardSpecServer) relatesTarget(ctx context.Context, name string, proj *BoardProjection, endpoint string) (*artifact.Target, error) {
	// A live sticky on this board (round 5.4, 02 §Record schemas: "a
	// relates endpoint may name a board annotation by id") — most
	// relevantly a story/spike proto-sticky's attribution yarn, but
	// legal for any sticky, matching the amendment's own general wording.
	// Stored as the bare annotation id, no selector: this is exactly what
	// relatesEndpoint (projection.go) recognizes on the read side.
	for _, st := range proj.Stickies {
		if st.ID == endpoint {
			return &artifact.Target{Ref: endpoint}, nil
		}
	}
	head, err := gitx.RevParse(ctx, s.root, "HEAD")
	if err != nil {
		return nil, err
	}
	if _, ok := declaredKindsOf(proj)[endpoint]; ok {
		return &artifact.Target{
			Ref:      "spec/" + name + "@" + head,
			Selector: artifact.Selector{Heading: endpoint},
		}, nil
	}
	r, err := artifact.ParseRef(endpoint)
	if err != nil {
		return nil, fmt.Errorf("relates endpoint %q is neither a declared object nor a ref: %w", endpoint, err)
	}
	pinned := string(r.Kind) + "/" + r.Name + "@" + head
	if r.Object != "" {
		pinned += "#" + r.Object
	}
	return &artifact.Target{Ref: pinned}, nil
}

// checkProtoYarnLegal re-checks the scoping canvas's type-directed
// attribution rule (spec/scoping-canvas dc-5) server-side, exactly as
// checkEdgeLegal re-checks the type picker's table: the drag handler can
// only OFFER what this function permits, but the server never trusts the
// menu. Without it a direct API caller could mint a story-sticky→open-
// question (or spike-sticky→acceptance-criterion) thread that
// stub-graduate's prefix filter then silently ignores, leaving a dead
// thread in the annotation stream that nothing ever reviews.
//
// The rule itself lives in ONE place — protoYarnTargetKind (edgetypes.go)
// — shared with the client's routeProtoYarn (assets/boardspec.js), which
// applies the same two rules as the picker's fast path.
//
// Only a pair with a story/spike proto-sticky ENDPOINT is type-checked:
// every other untyped relates thread keeps the open vocabulary dc-5 gives
// it. The check is symmetric in the endpoints — the drag handler only ever
// posts the sticky as `from`, but a direct caller may swap them and
// actionStubGraduate reads a thread in either direction.
func (s *boardSpecServer) checkProtoYarnLegal(proj *BoardProjection, from, to string) error {
	kinds := declaredKindsOf(proj)
	stickyTypes := make(map[string]string, len(proj.Stickies))
	for _, st := range proj.Stickies {
		stickyTypes[st.ID] = st.Type
	}
	for _, pair := range [][2]string{{from, to}, {to, from}} {
		sticky, target := pair[0], pair[1]
		want, typed := protoYarnTargetKind(stickyTypes[sticky])
		if !typed || kinds[target] == want {
			continue
		}
		// The refusal is the server-side voice of routeProtoYarn's own
		// wording: name the thread's one meaning, then — when the pair is
		// merely CROSSED (a story sticky aimed at an open question, or the
		// mirror) — redirect to the sticky type that thought wants to be.
		// The class words are display prose and resolve (L-M13a(6)); the
		// endpoint ids and object kinds stay bare identity.
		claim, plural, singular := "claims coverage", "acceptance criteria", "acceptance criterion"
		crossKind, crossType, crossVerb := string(boardlayout.ZoneOpenQuestion), "spike", "answers open questions"
		if want == string(boardlayout.ZoneOpenQuestion) {
			claim, plural, singular = "claims an answer", "open questions", "open question"
			crossKind, crossType, crossVerb = string(boardlayout.ZoneAC), "story", "delivers an acceptance criterion"
		}
		msg := fmt.Sprintf("%s sticky's thread %s — it ties only to %s, and %q is not a declared %s on this wall",
			model.Indefinite(s.model.DisplayClass(stickyTypes[sticky])), claim, plural, target, singular)
		if kinds[target] == crossKind {
			msg += fmt.Sprintf(". If this thought %s, it wants to be %s sticky instead", crossVerb, model.Indefinite(s.model.DisplayClass(crossType)))
		}
		return fmt.Errorf("%s (spec/scoping-canvas dc-5)", msg)
	}
	return nil
}

// actionRelates: the scratch tier's untyped thread — annotation layer,
// never the document (02 §Record schemas: type relates).
func (s *boardSpecServer) actionRelates(ctx context.Context, name string, proj *BoardProjection, req boardAPIRequest) error {
	if req.From == "" || req.To == "" {
		return fmt.Errorf("relates requires from and to")
	}
	if err := s.checkProtoYarnLegal(proj, req.From, req.To); err != nil {
		return err
	}
	a, err := newAnnotation(artifact.AnnotationRelates, "relates: "+req.From+" ~ "+req.To)
	if err != nil {
		return err
	}
	if a.Target, err = s.relatesTarget(ctx, name, proj, req.From); err != nil {
		return err
	}
	if a.TargetB, err = s.relatesTarget(ctx, name, proj, req.To); err != nil {
		return err
	}
	return boardio.AppendAnnotation(boardio.AnnotationsDir(s.root), boardio.AnnotationFileForTarget(artifact.Ref{Kind: artifact.KindSpec, Name: name}), a)
}

// actionAnnotationDelete: a scratch sticky or an untyped relates thread
// dies from the mutable stream (05 §Workbench: they graduate or they
// die; owner UAT round 6, item 3). Only records this board actually
// presents are deletable, and the spec document is never touched.
// Refusals name WHICH annotation and WHERE it was looked for (the board
// and the stream directory) — the owner-reported "annotations were
// missing, unclear where" popups were these messages firing on stale
// double-deletes without naming their board.
func (s *boardSpecServer) actionAnnotationDelete(name string, proj *BoardProjection, req boardAPIRequest) error {
	ids := req.IDs
	if req.ID != "" {
		ids = append([]string{req.ID}, ids...)
	}
	if len(ids) == 0 {
		return fmt.Errorf("annotation-delete requires id or ids")
	}
	live := map[string]bool{}
	for _, st := range proj.Stickies {
		live[st.ID] = true
	}
	for _, e := range proj.Edges {
		if e.AnnotationID != "" {
			live[e.AnnotationID] = true
		}
	}
	// A trashed reference card's own pin record dies with its threads
	// (the ref-trash gesture's annotation half rides this action now).
	for _, rc := range proj.RefCards {
		if rc.Pinned {
			live[rc.PinID] = true
		}
	}
	for _, id := range ids {
		if !live[id] {
			return fmt.Errorf("no annotation %q on the board for spec/%s — it may already have been deleted or graduated since this wall was last refreshed", id, name)
		}
	}
	dir := boardio.AnnotationsDir(s.root)
	n, err := boardio.DeleteAnnotations(dir, ids)
	if err != nil {
		return err
	}
	if n != len(ids) {
		return fmt.Errorf("only %d of %d annotation record(s) were found to delete in the mutable streams under %s", n, len(ids), dir)
	}
	return nil
}

// actionPosition: a card drag landed — resolve the drop against every
// other card's footprint (nearest non-overlapping position; the board is
// collision-free by construction) and store ONLY the dragged card's
// coordinate in layout.json (positions only, never content; autosaved,
// never committed per-drag; no other stored position is ever touched).
// The write prunes orphaned keys (VL-018, the adjudicated policy). The id
// is either a declared object id, or — round 5.5 dc-6 — "stub:<slug>"
// naming a declared stub; either way the layout key and the zone kind
// (hence the footprint) are resolved the same way and fed to the same
// drop-resolution machinery.
func (s *boardSpecServer) actionPosition(name string, proj *BoardProjection, req boardAPIRequest) error {
	kinds := declaredKindsOf(proj)
	layoutKey := req.ID
	var kind boardlayout.ZoneKind
	switch {
	case kinds[req.ID] != "":
		kind = boardlayout.ZoneKind(kinds[req.ID])
	default:
		slug, isStub := strings.CutPrefix(req.ID, "stub:")
		if !isStub {
			return fmt.Errorf("position target %q is not a declared object id or a declared stub (layout.json keys must resolve, VL-018)", req.ID)
		}
		key, ok := stubKeyFor(proj, slug)
		if !ok {
			return fmt.Errorf("position target %q is not a declared object id or a declared stub (layout.json keys must resolve, VL-018)", req.ID)
		}
		layoutKey = key
		kind = boardlayout.ZoneStub
	}
	stored, err := boardlayout.ReadFile(s.specDir(name))
	if err != nil {
		return err
	}
	obstacles := make([]boardlayout.Rect, 0, len(proj.Cards)+len(proj.RefCards)+len(proj.StubViews))
	for _, c := range proj.Cards {
		if c.ID == req.ID {
			continue
		}
		w, h := boardlayout.FootprintFor(boardlayout.ZoneKind(c.Kind))
		obstacles = append(obstacles, boardlayout.Rect{X: c.X, Y: c.Y, W: w, H: h})
	}
	for _, rc := range proj.RefCards {
		w, h := boardlayout.FootprintFor(boardlayout.ZoneReference)
		obstacles = append(obstacles, boardlayout.Rect{X: rc.X, Y: rc.Y, W: w, H: h})
	}
	for _, sv := range proj.StubViews {
		if "stub:"+sv.Slug == layoutKey {
			continue
		}
		w, h := boardlayout.FootprintFor(boardlayout.ZoneStub)
		obstacles = append(obstacles, boardlayout.Rect{X: sv.X, Y: sv.Y, W: w, H: h})
	}
	w, h := boardlayout.FootprintFor(kind)
	stored[layoutKey] = boardlayout.ResolveDrop(artifact.Position{X: req.X, Y: req.Y}, w, h, obstacles)
	return boardlayout.WriteFile(s.specDir(name), stored, liveKeys(proj))
}

// actionStickyPosition: a sticky (or pinned-reference) drag landed — the
// position lives inside the annotation record (02 §Record schemas:
// board {story, x, y}); pins drag like stickies. The refusal names the
// annotation AND its board (errors name what they're about).
func (s *boardSpecServer) actionStickyPosition(name string, proj *BoardProjection, req boardAPIRequest) error {
	for _, st := range proj.Stickies {
		if st.ID == req.ID {
			return boardio.RepositionSticky(boardio.AnnotationsDir(s.root), req.ID, req.X, req.Y)
		}
	}
	for _, rc := range proj.RefCards {
		if rc.Pinned && rc.PinID == req.ID {
			return boardio.RepositionSticky(boardio.AnnotationsDir(s.root), req.ID, req.X, req.Y)
		}
	}
	return fmt.Errorf("no sticky or pin %q on the board for spec/%s — it may have been deleted or graduated while the drag was in flight", req.ID, name)
}

// actionGitCommit: the board-owned commit/push (05 §Workbench: "message
// prompt, executes git on the design branch underneath"). Push runs when
// an origin exists; a purely local checkout still commits durably.
func (s *boardSpecServer) actionGitCommit(ctx context.Context, req boardAPIRequest) error {
	if req.Message == "" {
		return fmt.Errorf("git-commit requires a commit message")
	}
	if err := gitx.AddAll(ctx, s.root); err != nil {
		return err
	}
	if _, err := gitx.CreateCommit(ctx, s.root, req.Message); err != nil {
		return err
	}
	hasOrigin, err := gitx.HasRemote(ctx, s.root, "origin")
	if err != nil {
		return err
	}
	if hasOrigin {
		if err := gitx.Push(ctx, s.root); err != nil {
			return fmt.Errorf("committed locally, but push failed: %w", err)
		}
	}
	return nil
}

// actionGitSwitch: the branch switcher, guarded server-side too — a
// dirty tree refuses to switch (409), whatever the client shows.
func (s *boardSpecServer) actionGitSwitch(ctx context.Context, w http.ResponseWriter, req boardAPIRequest) {
	if req.Branch == "" {
		writeJSONError(w, http.StatusBadRequest, "git-switch requires a branch")
		return
	}
	if s.fixedBranch != "" {
		// A per-branch draft board (spec/draft-boards dc-1): the branch is
		// the address, so "switch branch" here would silently re-point the
		// managed worktree the worktree-manager seam owns for fixedBranch —
		// the surprise mutation feature dc-1 forbids. The other branch's
		// board is one directory click away at its own /b/ address.
		writeJSONError(w, http.StatusForbidden, fmt.Sprintf(
			"this board serves branch %s at its own /b/ address — the branch is the address here, so switching this working tree is not available; open the other branch's board from the directory instead", s.fixedBranch))
		return
	}
	dirty, err := gitx.StatusDirty(ctx, s.root)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if dirty {
		writeJSONError(w, http.StatusConflict, "uncommitted changes on this working tree; commit before switching branches (branch-switch guard)")
		return
	}
	if err := gitx.Checkout(ctx, s.root, req.Branch); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, boardAPIResponse{Dirty: false})
}
