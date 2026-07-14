// The board's diagram-proposal editor surface (spec/board-editor dc-1):
// GET /board/diagram/{name} (page), GET /board/diagram/{name}/fragment,
// and POST /board/diagram/{name}/api/{action} — deliberately the same
// routing grammar as the spec board's /board/spec/{name} trio, so the
// workbench keeps one routing idiom. The surface serves class: proposal
// diagram artifacts (.verdi/diagrams/<name>.mermaid) only: an incumbent
// authored-living diagram has no editor (it is not a proposal), and a
// missing or non-proposal name 404s.
//
// Write posture (co-1): EVERY write path on this surface — the code-pane
// save, each structural op, and reset — reassembles the file as the
// UNTOUCHED frontmatter prefix bytes plus the new body bytes, verbatim.
// Nothing normalizes, reflows, or round-trips the text through a graph
// representation; the structural ops are internal/diagramedit's pure
// byte-splices and reset's body is internal/diagrambase's recovered base.
package workbench

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/diagrambase"
	"github.com/jyang234/verdi/internal/diagramedit"
	"github.com/jyang234/verdi/internal/gitx"
)

// boardDiagramServer holds the editor's dependencies for one store root.
type boardDiagramServer struct {
	root string

	// verifier is the dc-4 consumer port. nil means no extractor is wired
	// (the two stories build in either order): the rail renders the
	// disclosed verification-unavailable state, and nothing else on the
	// surface changes — verification informs, it never blocks.
	verifier DiagramVerifier

	// writeMu serializes this surface's read-modify-write mutations within
	// the process, exactly like boardSpecServer.writeMu (M-2): two racing
	// saves against the same file would otherwise lose an update. Reads
	// (page/fragment) do not take it — atomic rename keeps every read a
	// whole file.
	writeMu sync.Mutex
}

// diagramEditorView is everything one render of the editor needs — the
// load result of the artifact plus the surface's computed state.
type diagramEditorView struct {
	Name  string
	Title string
	// Status is the proposal's authored status (proposed/accepted).
	Status string
	Mode   boardModeKind
	// Raw is the artifact file's full bytes; Body is its mermaid source
	// (the byte suffix after the frontmatter block). bodyStart is where
	// Body begins in Raw — the byte-preserving write seam.
	Raw       []byte
	Body      []byte
	bodyStart int

	// Ops state: Doc is non-nil iff the body is within diagramedit's
	// flowchart subset; otherwise OpsUnavailable carries the disclosure
	// (ac-2: disclosed unavailable, the code pane stays live).
	Doc            *diagramedit.Doc
	OpsUnavailable string

	// Derived provenance (ac-4): nil for a from-scratch proposal — the
	// before-peek/reset affordances are then not offered at all.
	DerivedFrom *artifact.DiagramDerivedFrom

	// Rail state (ac-5): exactly one of Verification (the report,
	// rendered verbatim) or VerificationUnavailable (the disclosed
	// unavailable reason) is set.
	Verification            *DiagramVerification
	VerificationUnavailable string

	Git       *boardGitState
	GitNotice string
}

// diagramPath is the proposal's file in the working tree (01 §Directory
// layout: diagrams/<name>.mermaid).
func (s *boardDiagramServer) diagramPath(name string) string {
	return filepath.Join(s.root, ".verdi", "diagrams", name+".mermaid")
}

// errNotAProposal distinguishes "exists but is not a class: proposal
// diagram" — surfaced as 404 like a missing artifact (this surface only
// exists for proposals) but with its own message.
var errNotAProposal = errors.New("workbench: diagram is not a class: proposal artifact; the editor serves proposals only")

// loadDiagram reads, strict-decodes, and classifies the surface's state.
func (s *boardDiagramServer) loadDiagram(ctx context.Context, name string) (*diagramEditorView, error) {
	if !specNameRe.MatchString(name) {
		return nil, ErrBoardNotFound
	}
	raw, err := os.ReadFile(s.diagramPath(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrBoardNotFound
		}
		return nil, fmt.Errorf("workbench: reading diagram %s: %w", name, err)
	}
	fmBytes, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, fmt.Errorf("workbench: diagram %s: %w", name, err)
	}
	fm, err := artifact.DecodeDiagram(fmBytes)
	if err != nil {
		return nil, fmt.Errorf("workbench: diagram %s: %w", name, err)
	}
	if fm.Class != artifact.DiagramClassProposal {
		return nil, errNotAProposal
	}

	v := &diagramEditorView{
		Name:        name,
		Title:       fm.Title,
		Status:      string(fm.Status),
		Raw:         raw,
		Body:        body,
		bodyStart:   len(raw) - len(body),
		DerivedFrom: fm.DerivedFrom,
	}

	// Ops availability is a pure function of the current source (ac-2):
	// within the dc-2 subset the ops surface is live; outside it the ops
	// are disclosed unavailable and the code pane stays fully editable.
	doc, perr := diagramedit.Parse(body)
	if perr != nil {
		var outside *diagramedit.OutsideSubsetError
		if !errors.As(perr, &outside) {
			return nil, fmt.Errorf("workbench: diagram %s: %w", name, perr)
		}
		v.OpsUnavailable = outside.Error()
	} else {
		v.Doc = doc
	}

	// The rail consumes, never computes (dc-4): unwired, or any error,
	// renders the disclosed unavailable state — NON-BLOCKING by
	// construction (the load carries on either way).
	if s.verifier == nil {
		v.VerificationUnavailable = "no verification extractor is wired for this checkout"
	} else if report, verr := s.verifier.VerifyDiagram(ctx, name); verr != nil {
		v.VerificationUnavailable = verr.Error()
	} else if rerr := report.Validate(); rerr != nil {
		// A malformed report is an extractor error, disclosed the same
		// way — never rendered as a fabricated tier.
		v.VerificationUnavailable = rerr.Error()
	} else {
		v.Verification = report
	}

	// The same authoring-mode gate as spec-board writes (dc-1): a
	// proposal is authored on a design branch while still proposed; an
	// accepted (frozen) proposal, or any checkout on the default branch,
	// is a read-only record.
	git, notice, err := (&boardSpecServer{root: s.root}).gitState(ctx)
	if err != nil {
		return nil, err
	}
	v.Git, v.GitNotice = git, notice
	v.Mode = modeReadOnly
	if fm.Status == "proposed" && git.Branch != "" && git.Branch != git.DefaultBranch {
		v.Mode = modeAuthoring
	}
	return v, nil
}

// writeBody persists newBody as the artifact's mermaid source: the
// frontmatter prefix bytes are spliced back UNTOUCHED and newBody lands
// verbatim (ac-3: a save stores the pane's bytes exactly; no trimming,
// no newline normalization, no graph round-trip anywhere). Atomic
// temp+rename via the shared atomicfile seam.
func (s *boardDiagramServer) writeBody(v *diagramEditorView, newBody []byte) error {
	out := make([]byte, 0, v.bodyStart+len(newBody))
	out = append(out, v.Raw[:v.bodyStart]...)
	out = append(out, newBody...)
	if err := atomicfile.Write(s.diagramPath(v.Name), out, 0o644); err != nil {
		return fmt.Errorf("workbench: writing diagram %s: %w", v.Name, err)
	}
	return nil
}

// diagramAPIRequest is the one strict-decoded body shape every editor
// action reads its fields from. It deliberately declares NO position
// field of any kind (co-2): a request carrying x/y/position keys fails
// closed at decode (DecodeStrictJSON's DisallowUnknownFields) — the
// schema is the refusal.
type diagramAPIRequest struct {
	Source *string `json:"source,omitempty"` // save: the pane's exact bytes
	Label  string  `json:"label,omitempty"`  // add-node, rename
	ID     string  `json:"id,omitempty"`     // rename, delete-node
	From   string  `json:"from,omitempty"`   // connect, delete-edge
	To     string  `json:"to,omitempty"`     // connect, delete-edge
}

// diagramAPIResponse reports the surface's post-action state: the
// artifact's body (the pane swaps to it), the working tree's dirtiness,
// and the recomputed ops availability with the current node/edge model
// (the client's gesture layer maps rendered SVG elements to these ids —
// it never re-parses the source itself).
type diagramAPIResponse struct {
	Dirty          bool               `json:"dirty"`
	Source         string             `json:"source"`
	OpsAvailable   bool               `json:"opsAvailable"`
	OpsUnavailable string             `json:"opsUnavailable,omitempty"`
	Nodes          []diagramedit.Node `json:"nodes"`
	Edges          []diagramedit.Edge `json:"edges"`
}

// diagramPeekResponse is the peek action's read-only result: the
// digest-verified base source, carrying no state of its own (ac-4).
type diagramPeekResponse struct {
	Base string `json:"base"`
}

func opsStateOf(v *diagramEditorView) (available bool, reason string, nodes []diagramedit.Node, edges []diagramedit.Edge) {
	if v.Doc == nil {
		return false, v.OpsUnavailable, []diagramedit.Node{}, []diagramedit.Edge{}
	}
	return true, "", v.Doc.Nodes(), v.Doc.Edges()
}

// boardDiagramPageHandler answers GET /board/diagram/{name}.
func (s *boardDiagramServer) boardDiagramPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		v, err := s.loadDiagram(r.Context(), r.PathValue("name"))
		if errors.Is(err, ErrBoardNotFound) || errors.Is(err, errNotAProposal) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		out, err := renderDiagramEditorPage(v)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out) // response body write; post-header error is unactionable
	}
}

// boardDiagramFragmentHandler answers GET /board/diagram/{name}/fragment:
// the re-rendered editor region (dc-1's routing grammar) — one renderer
// for page and fragment, no client-side duplicate.
func (s *boardDiagramServer) boardDiagramFragmentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		v, err := s.loadDiagram(r.Context(), r.PathValue("name"))
		if errors.Is(err, ErrBoardNotFound) || errors.Is(err, errNotAProposal) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderDiagramEditorRegion(v)))
	}
}

// boardDiagramAPIHandler answers POST /board/diagram/{name}/api/{action}.
func (s *boardDiagramServer) boardDiagramAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.PathValue("name")
		action := r.PathValue("action")

		s.writeMu.Lock()
		defer s.writeMu.Unlock()

		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "reading request body: "+err.Error())
			return
		}
		var req diagramAPIRequest
		if err := artifact.DecodeStrictJSON(raw, &req); err != nil {
			// Unknown fields — a position key included (co-2) — land here.
			writeJSONError(w, http.StatusBadRequest, "malformed request: "+err.Error())
			return
		}

		v, err := s.loadDiagram(r.Context(), name)
		if errors.Is(err, ErrBoardNotFound) || errors.Is(err, errNotAProposal) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// peek is the one read-only action (it renders, it never writes);
		// everything else is a write and sits behind the authoring gate
		// (dc-1: a read-only checkout refuses mutations exactly as the
		// spec board does).
		if action != "peek" && v.Mode != modeAuthoring {
			writeJSONError(w, http.StatusForbidden, fmt.Sprintf("diagram editor for %s is in %s mode; only an authoring editor (proposed proposal on a design branch) accepts writes", name, v.Mode))
			return
		}

		switch action {
		case "peek":
			s.actionDiagramPeek(r.Context(), w, v)
			return
		case "save":
			if req.Source == nil {
				writeJSONError(w, http.StatusBadRequest, "save requires source")
				return
			}
			err = s.writeBody(v, []byte(*req.Source))
		case "reset":
			err = s.actionDiagramReset(r.Context(), v)
		case "add-node", "connect", "rename", "delete-node", "delete-edge":
			err = s.actionDiagramOp(v, action, req)
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			status := http.StatusBadRequest
			var outside *diagramedit.OutsideSubsetError
			var mismatch *diagrambase.DigestMismatchError
			var noSrc *diagrambase.NoSourceDigestError
			if errors.As(err, &outside) || errors.As(err, &mismatch) || errors.As(err, &noSrc) {
				// Disclosed refusals with their own vocabulary: ops
				// unavailable on this source; a base that does not verify;
				// a derived proposal with no source_digest to gate on
				// (ADJ-16). Each fails visible and writes nothing.
				status = http.StatusConflict
			}
			writeJSONError(w, status, err.Error())
			return
		}

		s.writeDiagramState(r.Context(), w, name)
	}
}

// writeDiagramState reloads the artifact and answers with the post-action
// response — the pane's new source of truth.
func (s *boardDiagramServer) writeDiagramState(ctx context.Context, w http.ResponseWriter, name string) {
	v, err := s.loadDiagram(ctx, name)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dirty, derr := gitx.StatusDirty(ctx, s.root)
	if derr != nil {
		writeJSONError(w, http.StatusInternalServerError, derr.Error())
		return
	}
	available, reason, nodes, edges := opsStateOf(v)
	writeJSON(w, http.StatusOK, diagramAPIResponse{
		Dirty:          dirty,
		Source:         string(v.Body),
		OpsAvailable:   available,
		OpsUnavailable: reason,
		Nodes:          nodes,
		Edges:          edges,
	})
}

// actionDiagramOp runs one structural operation as dc-2's deterministic
// source-text edit: the server computes the byte splice against the
// artifact's CURRENT source (one writer, no client-side duplicate of the
// edit logic) and persists it through the same byte-preserving write
// path the save uses.
func (s *boardDiagramServer) actionDiagramOp(v *diagramEditorView, action string, req diagramAPIRequest) error {
	var (
		newBody []byte
		err     error
	)
	switch action {
	case "add-node":
		newBody, _, err = diagramedit.AddNode(v.Body, req.Label)
	case "connect":
		if req.From == "" || req.To == "" {
			return fmt.Errorf("connect requires from and to")
		}
		newBody, err = diagramedit.Connect(v.Body, req.From, req.To)
	case "rename":
		if req.ID == "" {
			return fmt.Errorf("rename requires id")
		}
		newBody, err = diagramedit.Rename(v.Body, req.ID, req.Label)
	case "delete-node":
		if req.ID == "" {
			return fmt.Errorf("delete-node requires id")
		}
		newBody, err = diagramedit.DeleteNode(v.Body, req.ID)
	case "delete-edge":
		if req.From == "" || req.To == "" {
			return fmt.Errorf("delete-edge requires from and to")
		}
		newBody, err = diagramedit.DeleteEdge(v.Body, req.From, req.To)
	}
	if err != nil {
		return err
	}
	return s.writeBody(v, newBody)
}

// actionDiagramPeek answers the before-peek: the digest-verified base
// source, read-only (ac-4/dc-5). Every recovery failure — mismatch
// included — is a disclosed error and nothing is written (there is no
// write path in this function at all).
func (s *boardDiagramServer) actionDiagramPeek(ctx context.Context, w http.ResponseWriter, v *diagramEditorView) {
	base, err := diagrambase.Recover(ctx, s.root, v.DerivedFrom)
	if err != nil {
		status := http.StatusBadRequest
		var mismatch *diagrambase.DigestMismatchError
		var noSrc *diagrambase.NoSourceDigestError
		if errors.As(err, &mismatch) || errors.As(err, &noSrc) {
			// A base that does not verify, or a derived proposal with no
			// source_digest to gate on (ADJ-16): disclosed unavailable,
			// painted in the peek panel's failure slot, nothing written.
			status = http.StatusConflict
		}
		writeJSONError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, diagramPeekResponse{Base: string(base)})
}

// attachDiagramEditorHrefs enriches a spec board's diagram reference
// cards with their editor links (dc-1: the editor is reachable from a
// spec board's pinned diagram reference card). For each ref card whose
// target is a diagram, the target's file is decoded; a class: proposal
// gains EditorHref, anything else — an incumbent diagram, a dangling
// ref, a malformed file — is silently left link-less (the card itself
// already discloses what it references; a proposal-only surface simply
// is not offered for a non-proposal). Store-derived enrichment in the
// I/O layer, mirroring attachObligations' posture.
func attachDiagramEditorHrefs(proj *BoardProjection, root string) {
	for i := range proj.RefCards {
		ref, err := artifact.ParseRef(proj.RefCards[i].Ref)
		if err != nil || ref.Kind != artifact.KindDiagram {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, ".verdi", "diagrams", ref.Name+".mermaid"))
		if err != nil {
			continue
		}
		fmBytes, _, err := artifact.SplitFrontmatter(raw)
		if err != nil {
			continue
		}
		fm, err := artifact.DecodeDiagram(fmBytes)
		if err != nil || fm.Class != artifact.DiagramClassProposal {
			continue
		}
		proj.RefCards[i].EditorHref = "/board/diagram/" + ref.Name
	}
}

// actionDiagramReset replaces the working source with the digest-verified
// base bytes THROUGH THE ORDINARY SAVE PATH (dc-5: reset is byte-exact,
// not render-equivalent, and carries no state of its own). A recovery
// failure writes nothing.
func (s *boardDiagramServer) actionDiagramReset(ctx context.Context, v *diagramEditorView) error {
	base, err := diagrambase.Recover(ctx, s.root, v.DerivedFrom)
	if err != nil {
		return err
	}
	return s.writeBody(v, base)
}
