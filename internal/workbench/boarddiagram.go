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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/diagrambase"
	"github.com/jyang234/verdi/internal/diagramedit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/wtmanager"
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

	// Exit is the tool view's resolved return-target state (spec/tool-view-
	// exit ac-1/dc-2/dc-3): page-chrome-only state the page handler
	// resolves once, server-side, from the incoming board= query parameter
	// (resolveDiagramExit) and sets on the view before rendering. Never
	// populated by loadDiagram itself (loadDiagram has no request in
	// scope) and never read by the fragment or API routes.
	Exit diagramExitTarget
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
		v.Exit = resolveDiagramExit(s.root, r.URL.Query().Get("board"))
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
//
// boardName is the rendering spec board's own name and fixedBranch its own
// branch address — both already in scope at the call site (the board-load
// path knows which spec it is loading and, for a per-branch board instance,
// on which branch). The link rides a request-scoped board=<origin-path>
// query parameter (spec/tool-view-exit dc-2, controller adjudication
// ADJ-38): the value is the ORIGINATING BOARD PATH the operator was on —
// the serving checkout's unprefixed /board/spec/<name>, or a per-branch
// board's /b/<branch>/board/spec/<name> — so the editor's exit affordance
// returns to that EXACT board rather than the serving checkout's same-named
// board (the mislabeling ADJ-38 removes). The path is query-escaped; nothing
// is persisted — the parameter exists only for the length of the one request
// the link is followed on.
func attachDiagramEditorHrefs(proj *BoardProjection, root, boardName, fixedBranch string) {
	origin := boardOriginPath(fixedBranch, boardName)
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
		proj.RefCards[i].EditorHref = "/board/diagram/" + ref.Name + "?board=" + url.QueryEscape(origin)
	}
}

// boardOriginPath is the board route the rendering session is on — the value
// the diagram editor's board= parameter carries so its exit affordance can
// return there (spec/tool-view-exit dc-2, ADJ-38). fixedBranch is the
// rendering board instance's own branch: "" for the serving checkout's
// unprefixed board, the design branch for a per-branch draft board. The path
// mirrors the exact route grammar the operator followed — the branch rides
// one percent-encoded path segment, as handler.go's /b/{branch} mount and
// resolveDiagramExit's parser both expect — and is built through the shared
// constructors (directory.go), never a second grammar.
func boardOriginPath(fixedBranch, boardName string) string {
	if fixedBranch == "" {
		return boardSpecPrefix + boardName
	}
	return branchBoardHref(fixedBranch, boardName)
}

// diagramExitTarget is the diagram designer's resolved return-target
// state (spec/tool-view-exit dc-2/dc-3): where its exit affordance and its
// Escape binding navigate, computed once per page render.
type diagramExitTarget struct {
	// Href is where the exit affordance and Escape both navigate.
	Href string
	// Label is the affordance's visible text. Per dc-3 it always names
	// which case produced it — the real originating board, an unresolved
	// name, or no name at all — rather than collapsing them into one
	// unexplained state.
	Label string
	// Known is true iff Href resolves to the real originating spec board
	// (as opposed to the index fallback).
	Known bool
}

// resolveDiagramExit resolves origin — the incoming board= query parameter,
// the ORIGINATING BOARD PATH the operator was on (spec/tool-view-exit dc-2,
// controller adjudication ADJ-38) — against the actual board-route grammars
// and the store each addresses (dc-3: never derived or guessed, only
// carried; a path that does not check out is never trusted as a link). Two
// grammars are recognized, each resolved against its own store:
//
//   - /board/spec/<name>              the serving checkout's own tree (root);
//   - /b/<branch>/board/spec/<name>   the branch's managed worktree tree
//     (wtmanager.WorktreePath(root, branch)) — the exact store that produced
//     the link, since a branch-prefixed origin is emitted only by a
//     per-branch board instance whose root IS that path.
//
// A name that resolves to a real active spec in the store its grammar
// addresses (boards serve only specs/active/, boardspec.go's specDir doc
// comment) is the one case this renders a live board link for, echoing the
// validated origin path so the operator returns to the EXACT board they came
// from — never the serving checkout's same-named board (the branch-board
// mislabeling ADJ-38 removes). Anything else falls back to the index (dc-3),
// honestly labeled with which honest-degradation case produced it: no origin
// supplied at all (a direct URL, or the corpus page's editor link — neither
// carries board=), a well-formed board path whose spec does not resolve in
// the addressed store (stale, mistyped, or foreign to that tree), or a path
// that is not one of the two recognized board routes at all (only these two
// grammars are ever honored — never an open redirect). Every path component
// is validated (specNameRe for the name, validBranchSegment for the branch)
// before it reaches the filesystem, exactly like loadDiagram's own name
// parameter — a malformed value is treated as merely unresolvable, never a
// path to stat.
func resolveDiagramExit(root, origin string) diagramExitTarget {
	if origin == "" {
		return diagramExitTarget{
			Href:  "/",
			Label: "no originating board is known — back to index",
		}
	}
	if store, name, ok := diagramExitStore(root, origin); ok {
		if _, err := os.Stat(filepath.Join(store, ".verdi", "specs", "active", name, "spec.md")); err == nil {
			return diagramExitTarget{
				Href:  origin,
				Label: "back to board: " + name,
				Known: true,
			}
		}
		// A recognized board route whose spec does not resolve in the store
		// it addresses: named by the spec it failed to find (dc-3: the label
		// names which case it is, never one unexplained state).
		return diagramExitTarget{
			Href:  "/",
			Label: fmt.Sprintf("board %q is not known — back to index", name),
		}
	}
	// Not a recognized board route at all (a foreign or malformed path,
	// never followed as a link): disclosed by the raw value it carried.
	return diagramExitTarget{
		Href:  "/",
		Label: fmt.Sprintf("board %q is not known — back to index", origin),
	}
}

// diagramExitStore parses origin against the two board-route grammars and
// returns the filesystem store root that grammar addresses plus the spec
// name it targets. ok is false for anything that is not one of the two
// recognized board routes with valid components — the strict gate that keeps
// resolveDiagramExit from ever honoring a foreign path (no open redirect)
// and from letting a hostile branch or name segment reach the filesystem.
func diagramExitStore(root, origin string) (store, name string, ok bool) {
	// Unprefixed: /board/spec/<name> — the serving checkout's own tree.
	if rest, found := strings.CutPrefix(origin, boardSpecPrefix); found {
		if specNameRe.MatchString(rest) {
			return root, rest, true
		}
		return "", "", false
	}
	// Branch-prefixed: /b/<branch>/board/spec/<name>. The branch is one path
	// segment with its slashes percent-encoded (handler.go's /b/{branch}
	// mount), exactly as the operator's URL carried it — so splitting on the
	// first boardSpecPrefix segment is unambiguous (an encoded branch can
	// contain no such literal, and a spec name can contain no slash). These
	// are the same two prefixes branchBoardHref (directory.go) builds with.
	if rest, found := strings.CutPrefix(origin, branchBoardPrefix); found {
		seg, tail, cut := strings.Cut(rest, boardSpecPrefix)
		if !cut || seg == "" || !specNameRe.MatchString(tail) {
			return "", "", false
		}
		branch, err := url.PathUnescape(seg)
		if err != nil || !validBranchSegment(branch) {
			return "", "", false
		}
		return wtmanager.WorktreePath(root, branch), tail, true
	}
	return "", "", false
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
