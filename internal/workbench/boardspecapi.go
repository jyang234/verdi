package workbench

// The v1 board's write surface: POST /board/spec/{name}/api/{action}.
// Every spec write goes through internal/artifact/splice (surgical splice +
// validate-before-write, S7) and lands in the working tree only;
// annotation writes go to the mutable zone and never dirty the spec
// tree; git acts are explicit rituals (05 §Workbench "Authoring").
// Everything is authoring-mode-only: review is a mirror, read-only is a
// document.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifact/splice"
	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// boardAPIRequest is the one strict-decoded body shape every action
// reads its fields from; unknown fields fail closed.
type boardAPIRequest struct {
	ID      string  `json:"id,omitempty"`
	Ref     string  `json:"ref,omitempty"`
	Text    string  `json:"text,omitempty"`
	From    string  `json:"from,omitempty"`
	To      string  `json:"to,omitempty"`
	Type    string  `json:"type,omitempty"`
	NewType string  `json:"newType,omitempty"`
	Note    string  `json:"note,omitempty"`
	Kind    string  `json:"kind,omitempty"`
	X       float64 `json:"x,omitempty"`
	Y       float64 `json:"y,omitempty"`
	Message string  `json:"message,omitempty"`
	Branch  string  `json:"branch,omitempty"`
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

		proj, _, _, err := s.loadBoard(r.Context(), name)
		if errors.Is(err, ErrBoardNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// stub-instantiate is deliberately EXEMPT from the authoring-only
		// gate (spec/scoping-canvas ac-6, flagged judgment call): it never
		// edits the SERVED spec at all — it scaffolds an unrelated new
		// story spec on a fresh, un-checked-out branch via git plumbing —
		// so an accepted-pending-build wall (permanently sealed, never
		// authoring) must still be able to run it. Its own guard (class
		// feature, status accepted-pending-build) is enforced inside the
		// action itself, against the wall's own state rather than the
		// generic writes-need-authoring-mode posture every other action
		// shares.
		if action != "stub-instantiate" && proj.Mode != modeAuthoring {
			writeJSONError(w, http.StatusForbidden, fmt.Sprintf("board for %s is in %s mode; only an authoring board (draft spec on a design branch) accepts writes", name, proj.Mode))
			return
		}

		ctx := r.Context()
		switch action {
		case "edit-text":
			err = s.actionEditText(name, req)
		case "edge":
			err = s.actionEdge(name, proj, req)
		case "sticky":
			err = s.actionSticky(name, proj, req)
		case "sticky-graduate":
			// A sticky graduates into a declared spec object (the object
			// menu's ac/co/dc/oq) OR — on a story wall, kind
			// "obligation:<for-kind>" — into an evidence-obligation artifact
			// bound to the story AC it was dropped on (spec/obligation-
			// artifact ac-3). One action, one graduation ritual; the kind
			// prefix selects the destination.
			if strings.HasPrefix(req.Kind, obligationGraduatePrefix) {
				err = s.actionObligationGraduate(ctx, name, proj, req)
			} else {
				err = s.actionStickyGraduate(name, proj, req)
			}
		case "stub-graduate":
			err = s.actionStubGraduate(name, proj, req)
		case "stub-instantiate":
			err = s.actionStubInstantiate(ctx, name, proj, req)
		case "relates":
			err = s.actionRelates(ctx, name, proj, req)
		case "relates-graduate":
			err = s.actionRelatesGraduate(name, proj, req)
		case "pin":
			err = s.actionPin(ctx, name, proj, req)
		case "ref-trash":
			err = s.actionRefTrash(name, proj, req)
		case "object-trash":
			err = s.actionObjectTrash(name, proj, req)
		case "annotation-delete":
			err = s.actionAnnotationDelete(proj, req)
		case "edge-delete":
			err = s.actionEdgeDelete(name, proj, req)
		case "edge-retype":
			err = s.actionEdgeRetype(name, proj, req)
		case "position":
			err = s.actionPosition(name, proj, req)
		case "sticky-position":
			err = s.actionStickyPosition(proj, req)
		case "git-commit":
			err = s.actionGitCommit(ctx, req)
		case "git-switch":
			s.actionGitSwitch(ctx, w, req)
			return
		default:
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

// spliceSpec runs one splice transaction against the spec's document:
// parse the pristine buffer, compute edits, apply tail-to-head, strict
// re-decode (validate-before-write), then atomically replace the file.
// An invalid result never touches the working tree (S7 §5).
func (s *boardSpecServer) spliceSpec(name string, mutate func(d *splice.Doc) ([]splice.Edit, error)) error {
	path := filepath.Join(s.specDir(name), "spec.md")
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("workbench: reading spec %s: %w", name, err)
	}
	doc, err := splice.Parse(src)
	if err != nil {
		return err
	}
	edits, err := mutate(doc)
	if err != nil {
		return err
	}
	out, err := doc.Apply(edits)
	if err != nil {
		return err
	}
	if err := splice.Validate(out); err != nil {
		return err
	}
	// atomicfile.Write (MkdirAll + CreateTemp + fsync + Rename-into-place)
	// — this repo's one shared crash-durability primitive — never a private
	// CreateTemp->Write->Close->Rename copy, so a crash mid-write can never
	// leave a torn spec.md nor lose the fsync that copy lacked
	// (CLEANUP-BEFORE #1).
	if err := atomicfile.Write(path, out, 0o644); err != nil {
		return fmt.Errorf("workbench: %w", err)
	}
	return nil
}

// actionEditText: the inline card editor's blur — editing the card IS
// editing the spec object (05 §Workbench: bidirectional authoring).
func (s *boardSpecServer) actionEditText(name string, req boardAPIRequest) error {
	if req.ID == "" || req.Text == "" {
		return fmt.Errorf("edit-text requires id and text")
	}
	return s.spliceSpec(name, func(d *splice.Doc) ([]splice.Edit, error) {
		e, err := d.SetObjectText(req.ID, req.Text)
		if err != nil {
			return nil, err
		}
		return []splice.Edit{e}, nil
	})
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

// checkEdgeLegal re-checks the picker's own table server-side: the menu
// can only OFFER what this function permits, but the server never
// trusts the menu.
func checkEdgeLegal(proj *BoardProjection, from, to, edgeType string) error {
	kinds := declaredKindsOf(proj)
	sourceKind, ok := kinds[from]
	if !ok {
		return fmt.Errorf("edge source %q is not a declared object", from)
	}
	targetKind := targetKindOf(kinds, to)
	for _, t := range legalEdgeTypes(sourceKind, targetKind) {
		if t == edgeType {
			return nil
		}
	}
	return fmt.Errorf("edge type %q is not legal for a (%s, %s) pair (02 §Link taxonomy)", edgeType, sourceKind, targetKind)
}

// edgeRefFor renders a yarn target endpoint as the link ref the spec
// document stores: an internal object becomes a same-spec fragment; an
// external endpoint is stored as written.
func edgeRefFor(proj *BoardProjection, name, to string) string {
	if _, ok := declaredKindsOf(proj)[to]; ok {
		return "spec/" + name + "#" + to
	}
	return to
}

// actionEdge: the type picker's commit — a declared typed edge lands in
// the decision's own links: via splice.
func (s *boardSpecServer) actionEdge(name string, proj *BoardProjection, req boardAPIRequest) error {
	if req.From == "" || req.To == "" || req.Type == "" {
		return fmt.Errorf("edge requires from, to, and type")
	}
	if err := checkEdgeLegal(proj, req.From, req.To, req.Type); err != nil {
		return err
	}
	link := artifact.Link{Type: artifact.LinkType(req.Type), Ref: edgeRefFor(proj, name, req.To), Note: req.Note}
	if err := s.spliceSpec(name, func(d *splice.Doc) ([]splice.Edit, error) {
		e, err := d.AppendDecisionLink(req.From, link)
		if err != nil {
			return nil, err
		}
		return []splice.Edit{e}, nil
	}); err != nil {
		return err
	}
	// Drawing a typed edge to a pinned target IS the pin's graduation
	// (02 §Record schemas): the record flips, the card stays.
	return s.graduatePinsFor(proj, req.To)
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
		return fmt.Errorf("sticky requires a type (one of comment, question, decision-needed, agent-task, story, spike)")
	}
	if !stickyCreatableTypes[typ] && !protoStickyTypes[typ] {
		return fmt.Errorf("sticky type %q is not creatable (one of comment, question, decision-needed, agent-task, story, spike); fail closed", req.Type)
	}
	if protoStickyTypes[typ] && proj.Class != string(artifact.ClassFeature) {
		return fmt.Errorf("sticky type %q is only creatable on a feature-class wall (the scoping canvas, 02 §Record schemas); this wall is class %s", req.Type, proj.Class)
	}
	a, err := newAnnotation(typ, req.Text)
	if err != nil {
		return err
	}
	x, y := stickyLanePosition(proj, typ)
	a.Board = &artifact.BoardAnchor{Story: name, X: x, Y: y}
	return boardio.AppendAnnotation(boardio.AnnotationsDir(s.root), boardio.AnnotationFileForBoard(store.RefSlug(name)), a)
}

// graduationBlocks maps the graduate menu's object kinds to id prefixes.
var graduationBlocks = map[string]string{
	string(boardlayout.ZoneAC):           "ac",
	string(boardlayout.ZoneConstraint):   "co",
	string(boardlayout.ZoneDecision):     "dc",
	string(boardlayout.ZoneOpenQuestion): "oq",
}

// actionStickyGraduate: graduation is an ordinary edit — the sticky's
// text becomes a declared object (05 §Workbench: "a sticky becomes a
// real object ... or they die"), and the record flips to graduated.
// A graduated acceptance criterion declares the outcome-evidence floor
// (attestation) as its expected evidence; the author refines it in the
// document like any other spec edit.
func (s *boardSpecServer) actionStickyGraduate(name string, proj *BoardProjection, req boardAPIRequest) error {
	prefix, ok := graduationBlocks[req.Kind]
	if !ok {
		return fmt.Errorf("unknown graduation kind %q", req.Kind)
	}
	var sticky *scratchStickyView
	for i := range proj.Stickies {
		if proj.Stickies[i].ID == req.ID {
			sticky = &proj.Stickies[i]
			break
		}
	}
	if sticky == nil {
		return fmt.Errorf("no sticky %q on this board", req.ID)
	}

	var existing []string
	for _, c := range proj.Cards {
		existing = append(existing, c.ID)
	}
	objectID := splice.NextID(existing, prefix)
	var evidence []artifact.EvidenceKind
	if prefix == "ac" {
		evidence = []artifact.EvidenceKind{artifact.EvidenceAttestation}
	}

	if err := s.spliceSpec(name, func(d *splice.Doc) ([]splice.Edit, error) {
		return d.AppendObject(objectID, sticky.Body, evidence)
	}); err != nil {
		return err
	}
	_, err := boardio.GraduateStickies(boardio.AnnotationsDir(s.root), []string{req.ID})
	return err
}

// actionStubGraduate: a story (or spike) proto-sticky plus its coverage
// (or resolution) yarn graduates into a declared stub (spec/scoping-
// canvas ac-2/ac-5, DC-1/DC-2) — a story sticky's relates-threads to
// acceptance criteria become the stub's acceptance_criteria list; a spike
// sticky's threads to open questions become its resolves list. The slug
// is RefSlug of the sticky's own body (its working title): a body that
// does not produce a usable kebab-case slug is refused by the splice
// write's own validate-before-write step, honestly, rather than silently
// repaired here. Refuses with a plain-language error when the sticky has
// no attribution yarn at all, or a slug collision with an already-
// declared stub. On success, the sticky and every thread that fed the
// stub flip to graduated — the same GraduateStickies machinery
// sticky-graduate uses.
func (s *boardSpecServer) actionStubGraduate(name string, proj *BoardProjection, req boardAPIRequest) error {
	if req.ID == "" {
		return fmt.Errorf("stub-graduate requires id")
	}
	var sticky *scratchStickyView
	for i := range proj.Stickies {
		if proj.Stickies[i].ID == req.ID {
			sticky = &proj.Stickies[i]
			break
		}
	}
	if sticky == nil {
		return fmt.Errorf("no sticky %q on this board", req.ID)
	}

	var spike bool
	switch artifact.AnnotationType(sticky.Type) {
	case artifact.AnnotationStory:
		spike = false
	case artifact.AnnotationSpike:
		spike = true
	default:
		return fmt.Errorf("sticky %q is a %s, not a story or spike proto-sticky; stub-graduate does not apply", req.ID, sticky.Type)
	}

	wantPrefix, noun := "ac-", "acceptance criteria"
	if spike {
		wantPrefix, noun = "oq-", "open questions"
	}
	seen := map[string]bool{}
	var ids []string
	var threadIDs []string
	for _, e := range proj.Edges {
		if e.Layer != "annotation" {
			continue
		}
		var other string
		switch {
		case e.From == req.ID:
			other = e.To
		case e.To == req.ID:
			other = e.From
		default:
			continue
		}
		if !strings.HasPrefix(other, wantPrefix) {
			continue
		}
		threadIDs = append(threadIDs, e.AnnotationID)
		if !seen[other] {
			seen[other] = true
			ids = append(ids, other)
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("sticky %q has no attribution yarn to %s yet; draw coverage yarn first", req.ID, noun)
	}
	sort.Strings(ids)

	slug := store.RefSlug(sticky.Body)
	for _, sv := range proj.StubViews {
		if sv.Slug == slug {
			return fmt.Errorf("a stub named %q already exists on this spec (slug collision)", slug)
		}
	}

	if err := s.spliceSpec(name, func(d *splice.Doc) ([]splice.Edit, error) {
		var e splice.Edit
		var err error
		if spike {
			e, err = d.AppendSpikeStub(slug, ids)
		} else {
			e, err = d.AppendStub(slug, ids)
		}
		if err != nil {
			return nil, err
		}
		return []splice.Edit{e}, nil
	}); err != nil {
		return err
	}

	graduate := append([]string{req.ID}, threadIDs...)
	_, err := boardio.GraduateStickies(boardio.AnnotationsDir(s.root), graduate)
	return err
}

// stubInstantiatePlaceholderStoryRef is the `story:` tracker scalar a
// stub-instantiated story spec carries: the story class requires one
// unconditionally (validateStory), but stub-instantiate has no real
// tracker ref of its own to give it (ac-6: bound to its stub by slug,
// "with no new provenance record") — an explicit, scheme-shaped
// placeholder rather than an empty field that would fail self-validation.
const stubInstantiatePlaceholderStoryRef = "todo:REPLACE-ME"

// actionStubInstantiate scaffolds a declared stub's story (or spike) spec
// on a fresh design/<slug> branch, built entirely via git plumbing so the
// SERVING checkout's HEAD, working tree, and real index are never touched
// (spec/scoping-canvas ac-6) — the operator checks the new branch out
// themselves. Guarded by the wall's own class and status (class feature,
// status accepted-pending-build: "the owner's rule: implementations
// build accepted specs only") rather than the generic authoring-mode
// gate — see the handler's own comment on why this action is exempted
// from it. Fails closed if the branch already exists (gitx.UpdateRef's
// own atomicity).
func (s *boardSpecServer) actionStubInstantiate(ctx context.Context, name string, proj *BoardProjection, req boardAPIRequest) error {
	slug := req.ID
	if slug == "" {
		return fmt.Errorf("stub-instantiate requires a stub slug (id)")
	}
	if proj.Class != string(artifact.ClassFeature) {
		return fmt.Errorf("stub-instantiate is only available on a feature-class wall; this wall is class %s", proj.Class)
	}
	if proj.Status != "accepted-pending-build" {
		return fmt.Errorf("stub-instantiate is only available on an accepted-pending-build spec (implementations build accepted specs only); this wall's status is %s", proj.Status)
	}
	var stub *StubView
	for i := range proj.StubViews {
		if proj.StubViews[i].Slug == slug {
			stub = &proj.StubViews[i]
			break
		}
	}
	if stub == nil {
		return fmt.Errorf("no stub %q declared on this spec", slug)
	}

	var links []designscaffold.StoryLink
	if stub.Spike {
		for _, oq := range stub.Resolves {
			links = append(links, designscaffold.StoryLink{Type: artifact.LinkResolves, Ref: "spec/" + name + "#" + oq})
		}
	} else {
		for _, ac := range stub.AcceptanceCriteria {
			links = append(links, designscaffold.StoryLink{Type: artifact.LinkImplements, Ref: "spec/" + name + "#" + ac})
		}
	}

	// A plain-language pre-check on the branch (the wall surfaces this
	// message verbatim): UpdateRef below stays the atomic create-only
	// guard — this only makes the common refusal legible, it does not
	// replace the fail-closed write.
	if _, err := gitx.RevParse(ctx, s.root, "refs/heads/design/"+slug); err == nil {
		return fmt.Errorf("branch design/%s already exists — this stub was already instantiated (or the name is taken); check that branch out instead", slug)
	}

	content := designscaffold.Story("spec/"+slug, stubInstantiatePlaceholderStoryRef, designscaffold.HumanizeName(slug), stub.Spike, links)

	// Self-validate before ever touching the object database (CLAUDE.md:
	// "never fake success" — mirrors design start's own pre-write check).
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		return fmt.Errorf("workbench: internal error: stub-instantiate scaffold failed self-validation: %w", err)
	}
	if _, err := artifact.DecodeSpec(fm); err != nil {
		return fmt.Errorf("workbench: internal error: stub-instantiate scaffold failed self-validation: %w", err)
	}

	baseCommit, err := gitx.RevParse(ctx, s.root, "HEAD")
	if err != nil {
		return err
	}
	blobSHA, err := gitx.WriteBlob(ctx, s.root, []byte(content))
	if err != nil {
		return err
	}
	path := store.ActiveSpecRelPath(slug)
	tree, err := gitx.BuildTreeWithFile(ctx, s.root, baseCommit+"^{tree}", path, blobSHA)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("stub-instantiate: scaffold spec/%s from stub %q of spec/%s", slug, slug, name)
	commit, err := gitx.CommitTree(ctx, s.root, tree, baseCommit, msg)
	if err != nil {
		return err
	}
	return gitx.UpdateRef(ctx, s.root, "refs/heads/design/"+slug, commit)
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

// actionRelates: the scratch tier's untyped thread — annotation layer,
// never the document (02 §Record schemas: type relates).
func (s *boardSpecServer) actionRelates(ctx context.Context, name string, proj *BoardProjection, req boardAPIRequest) error {
	if req.From == "" || req.To == "" {
		return fmt.Errorf("relates requires from and to")
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

// actionRelatesGraduate: the thread's graduation to a typed edge via the
// picker — an ordinary spec edit replacing the annotation (05
// §Workbench; 02 §Record schemas: "graduation to a real object edge ...
// is an ordinary spec edit, not an automatic promotion").
func (s *boardSpecServer) actionRelatesGraduate(name string, proj *BoardProjection, req boardAPIRequest) error {
	if req.ID == "" || req.Type == "" {
		return fmt.Errorf("relates-graduate requires id and type")
	}
	var thread *edgeView
	for i := range proj.Edges {
		if proj.Edges[i].AnnotationID == req.ID {
			thread = &proj.Edges[i]
			break
		}
	}
	if thread == nil {
		return fmt.Errorf("no relates thread %q on this board", req.ID)
	}
	if err := checkEdgeLegal(proj, thread.From, thread.To, req.Type); err != nil {
		return err
	}
	link := artifact.Link{Type: artifact.LinkType(req.Type), Ref: edgeRefFor(proj, name, thread.To), Note: req.Note}
	if err := s.spliceSpec(name, func(d *splice.Doc) ([]splice.Edit, error) {
		e, err := d.AppendDecisionLink(thread.From, link)
		if err != nil {
			return nil, err
		}
		return []splice.Edit{e}, nil
	}); err != nil {
		return err
	}
	if _, err := boardio.GraduateStickies(boardio.AnnotationsDir(s.root), []string{req.ID}); err != nil {
		return err
	}
	// The graduated thread's typed edge also graduates any pin holding
	// its target (02 §Record schemas).
	return s.graduatePinsFor(proj, thread.To)
}

// actionAnnotationDelete: a scratch sticky or an untyped relates thread
// dies from the mutable stream (05 §Workbench: they graduate or they
// die; owner UAT round 6, item 3). Only records this board actually
// presents are deletable, and the spec document is never touched.
func (s *boardSpecServer) actionAnnotationDelete(proj *BoardProjection, req boardAPIRequest) error {
	if req.ID == "" {
		return fmt.Errorf("annotation-delete requires id")
	}
	onBoard := false
	for _, st := range proj.Stickies {
		if st.ID == req.ID {
			onBoard = true
			break
		}
	}
	if !onBoard {
		for _, e := range proj.Edges {
			if e.AnnotationID == req.ID {
				onBoard = true
				break
			}
		}
	}
	if !onBoard {
		return fmt.Errorf("no annotation %q on this board", req.ID)
	}
	n, err := boardio.DeleteAnnotations(boardio.AnnotationsDir(s.root), []string{req.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("annotation %q was not found in the mutable stream", req.ID)
	}
	return nil
}

// edgeRefMatcher matches a stored link ref against a board endpoint the
// way the projection derives endpoints (edgeEndpoint): verbatim, the
// same-spec fragment form, or the pin-dropped kind/name#object form —
// so a pinned stored ref still matches the chip's unpinned data-to.
func edgeRefMatcher(name, to string) func(string) bool {
	internal := "spec/" + name + "#" + to
	return func(ref string) bool {
		if ref == to || ref == internal {
			return true
		}
		r, err := artifact.ParseRef(ref)
		if err != nil {
			return false
		}
		normalized := string(r.Kind) + "/" + r.Name
		if r.Object != "" {
			normalized += "#" + r.Object
		}
		return normalized == to || normalized == internal
	}
}

// actionEdgeDelete: removing a spec-layer typed edge is the exact
// inverse of drawing it — an ordinary spec edit through the splice
// write path (owner UAT round 6, item 3; the gate-bearing confirmation
// is the client's ritual, mirroring creation).
func (s *boardSpecServer) actionEdgeDelete(name string, proj *BoardProjection, req boardAPIRequest) error {
	if req.From == "" || req.To == "" || req.Type == "" {
		return fmt.Errorf("edge-delete requires from, to, and type")
	}
	if _, ok := declaredKindsOf(proj)[req.From]; !ok {
		return fmt.Errorf("edge source %q is not a declared object (a document-level edge lives in the frontmatter links: block, which the board cannot edit)", req.From)
	}
	return s.spliceSpec(name, func(d *splice.Doc) ([]splice.Edit, error) {
		e, err := d.RemoveDecisionLink(req.From, req.Type, edgeRefMatcher(name, req.To))
		if err != nil {
			return nil, err
		}
		return []splice.Edit{e}, nil
	})
}

// actionEdgeRetype: the relationship's type is updatable in place
// (owner directive, round 6 UAT follow-up) — one splice edit replacing
// only the type scalar, so the stored ref (pins included) and note
// survive verbatim and the document never passes through a linkless
// state. The new type must be legal for the pair, same table as
// creation.
func (s *boardSpecServer) actionEdgeRetype(name string, proj *BoardProjection, req boardAPIRequest) error {
	if req.From == "" || req.To == "" || req.Type == "" || req.NewType == "" {
		return fmt.Errorf("edge-retype requires from, to, type, and newType")
	}
	if err := checkEdgeLegal(proj, req.From, req.To, req.NewType); err != nil {
		return err
	}
	return s.spliceSpec(name, func(d *splice.Doc) ([]splice.Edit, error) {
		e, err := d.RetypeDecisionLink(req.From, req.Type, edgeRefMatcher(name, req.To), req.NewType)
		if err != nil {
			return nil, err
		}
		return []splice.Edit{e}, nil
	})
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
// board {story, x, y}); pins drag like stickies.
func (s *boardSpecServer) actionStickyPosition(proj *BoardProjection, req boardAPIRequest) error {
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
	return fmt.Errorf("no sticky %q on this board", req.ID)
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
