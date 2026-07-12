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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/artifact/splice"
	"github.com/OWNER/verdi/internal/boardio"
	"github.com/OWNER/verdi/internal/boardlayout"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/store"
)

// boardAPIRequest is the one strict-decoded body shape every action
// reads its fields from; unknown fields fail closed.
type boardAPIRequest struct {
	ID      string  `json:"id,omitempty"`
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
		if err == ErrBoardNotFound {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if proj.Mode != modeAuthoring {
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
			err = s.actionStickyGraduate(name, proj, req)
		case "relates":
			err = s.actionRelates(ctx, name, proj, req)
		case "relates-graduate":
			err = s.actionRelatesGraduate(name, proj, req)
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
	tmp, err := os.CreateTemp(s.specDir(name), ".spec-*.md")
	if err != nil {
		return fmt.Errorf("workbench: temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("workbench: writing %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("workbench: closing %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("workbench: replacing %s: %w", path, err)
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
	return s.spliceSpec(name, func(d *splice.Doc) ([]splice.Edit, error) {
		e, err := d.AppendDecisionLink(req.From, link)
		if err != nil {
			return nil, err
		}
		return []splice.Edit{e}, nil
	})
}

// Sticky landing geometry: the rendered sticky footprint estimate
// (mirrors canvasMinHeight's) and the append gap.
const (
	stickyEstHeight = 150
	stickyLaneGap   = 24
)

// stickyLaneColumn maps a sticky's type to the wall band it files
// into: a question queues beneath the open-questions column it may
// graduate into, a decision-needed beneath the decisions; comments and
// agent tasks take the scratch lane past the references.
func stickyLaneColumn(typ artifact.AnnotationType) boardlayout.ZoneColumn {
	var want boardlayout.ZoneKind
	switch typ {
	case artifact.AnnotationQuestion:
		want = boardlayout.ZoneOpenQuestion
	case artifact.AnnotationDecisionNeeded:
		want = boardlayout.ZoneDecision
	default:
		return boardlayout.ScratchColumn()
	}
	for _, c := range boardlayout.ZoneColumns() {
		if c.Kind == want {
			return c
		}
	}
	return boardlayout.ScratchColumn() // unreachable: zoneOrder covers both
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
var stickyCreatableTypes = map[artifact.AnnotationType]bool{
	artifact.AnnotationComment:        true,
	artifact.AnnotationQuestion:       true,
	artifact.AnnotationDecisionNeeded: true,
	artifact.AnnotationAgentTask:      true,
}

// actionSticky: "Add sticky" — a free-floating sticky of the author's
// explicitly chosen type (owner UAT round 6, item 2: choosing is part
// of creating; nothing defaults silently, unknown types fail closed) in
// the annotation layer; it never dirties the spec working tree (05
// §Workbench "The scratch tier").
func (s *boardSpecServer) actionSticky(name string, proj *BoardProjection, req boardAPIRequest) error {
	if req.Text == "" {
		return fmt.Errorf("sticky requires text")
	}
	typ := artifact.AnnotationType(req.Type)
	if req.Type == "" {
		return fmt.Errorf("sticky requires a type (one of comment, question, decision-needed, agent-task)")
	}
	if !stickyCreatableTypes[typ] {
		return fmt.Errorf("sticky type %q is not creatable (one of comment, question, decision-needed, agent-task); fail closed", req.Type)
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

// relatesTarget builds a relates endpoint's pinned target record.
func (s *boardSpecServer) relatesTarget(ctx context.Context, name string, proj *BoardProjection, endpoint string) (*artifact.Target, error) {
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
	_, err := boardio.GraduateStickies(boardio.AnnotationsDir(s.root), []string{req.ID})
	return err
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
// The write prunes orphaned keys (VL-018, the adjudicated policy).
func (s *boardSpecServer) actionPosition(name string, proj *BoardProjection, req boardAPIRequest) error {
	kinds := declaredKindsOf(proj)
	if _, ok := kinds[req.ID]; !ok {
		return fmt.Errorf("position target %q is not a declared object id (layout.json keys must resolve, VL-018)", req.ID)
	}
	stored, err := boardlayout.ReadFile(s.specDir(name))
	if err != nil {
		return err
	}
	obstacles := make([]boardlayout.Rect, 0, len(proj.Cards)+len(proj.RefCards))
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
	w, h := boardlayout.FootprintFor(boardlayout.ZoneKind(kinds[req.ID]))
	stored[req.ID] = boardlayout.ResolveDrop(artifact.Position{X: req.X, Y: req.Y}, w, h, obstacles)
	live := make(map[string]bool, len(kinds))
	for id := range kinds {
		live[id] = true
	}
	return boardlayout.WriteFile(s.specDir(name), stored, live)
}

// actionStickyPosition: a sticky drag landed — the position lives inside
// the annotation record (02 §Record schemas: board {story, x, y}).
func (s *boardSpecServer) actionStickyPosition(proj *BoardProjection, req boardAPIRequest) error {
	for _, st := range proj.Stickies {
		if st.ID == req.ID {
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
