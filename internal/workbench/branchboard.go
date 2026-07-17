// Per-branch draft boards (spec/draft-boards): the /b/{branch}/ prefix in
// front of the existing board addresses (dc-1). Beneath the prefix the
// EXISTING board route table (handler.go's boardSpecRoutes) is served
// rooted at the branch's managed working tree — one boardSpecServer
// instance per branch, constructed over a root obtained from the
// worktree-manager story's seam (wtmanager.EnsureWorktree), never a second
// board implementation and never a worktree lifecycle of this package's
// own (co-1). The branch rides ONE path segment with its slashes
// percent-encoded (design/foo -> design%2Ffoo); Go 1.22+ ServeMux segment
// semantics keep the escaped slash inside the segment and PathValue
// decodes it back.
//
// Resolution routes each request to a mode that already exists (dc-4 —
// this file adds routing, never a mode):
//
//   - local design branch        -> its managed worktree's own board
//     instance, mode computed by the UNCHANGED branch-state law
//     (loadBoard's switch) against that instance's own tree (ac-3);
//   - branch checked out at the serving root -> the serving checkout's own
//     board instance: that checkout IS the branch's working tree, so the
//     one instance (and its write serialization) serves both addresses;
//   - remote-tracking ref only   -> a sealed read-only render of that
//     ref's committed content, remoteness disclosed, no worktree cut, no
//     local branch minted (dc-4);
//   - no ref at all              -> the disclosed 404 notice page naming
//     the vanished branch, with a way back (dc-4);
//   - a cut that fails           -> a disclosed error page naming the
//     failure, never a dead link (dc-2).
package workbench

import (
	"context"
	"errors"
	"fmt"
	stdhtml "html"
	"net/http"
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// ensureWorktree is wtmanager.EnsureWorktree behind a package-level var so
// a test can inject a FAILING cut (dc-2's disclosed-error path, which no
// hermetic fixture can provoke through the real seam) — the same
// seam-preserving pattern wtmanager itself uses for gitx.WorktreeAdd.
// Production never rebinds it: EnsureWorktree is the one call into the
// worktree-manager seam, and this package runs no `git worktree` of its
// own (co-1).
var ensureWorktree = wtmanager.EnsureWorktree

// branchBoards routes /b/{branch}/board/spec/... requests to a per-branch
// boardSpecServer instance rooted at the branch's managed worktree.
type branchBoards struct {
	root string
	deps Deps

	// serving is the unprefixed routes' own board instance. When a /b/
	// branch is already checked out at the serving root itself
	// (wtmanager.ErrCheckedOutHere), that checkout IS the branch's working
	// tree, so requests dispatch into this same instance — same tree, same
	// writeMu — rather than erroring or minting a second writer over the
	// same files.
	serving *boardSpecServer

	// mu guards servers. Per-branch instances are singletons: the board's
	// intra-process write serialization (boardSpecServer.writeMu, M-2)
	// only holds if every request for a branch reaches the SAME instance.
	mu      sync.Mutex
	servers map[string]*boardSpecServer
}

func newBranchBoards(root string, deps Deps, serving *boardSpecServer) *branchBoards {
	return &branchBoards{root: root, deps: deps, serving: serving, servers: map[string]*boardSpecServer{}}
}

// server resolves branch to its managed worktree through the
// worktree-manager seam (lazy and synchronous on first request, dc-2:
// EnsureWorktree cuts the worktree and this call blocks until it is done;
// reuse afterwards is a stat) and returns the branch's singleton board
// instance rooted there. Every wtmanager refusal is returned typed for
// dispatch to route.
func (b *branchBoards) server(ctx context.Context, branch string) (*boardSpecServer, error) {
	path, err := ensureWorktree(ctx, b.root, branch)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.servers[branch]; ok {
		return s, nil
	}
	s := &boardSpecServer{
		root:              path,
		feed:              b.deps.CommentFeed,
		reviewUnavailable: b.deps.ReviewUnavailable,
		supersession:      b.deps.SupersessionCandidates,
		model:             b.deps.Model,
		fixedBranch:       branch,
	}
	b.servers[branch] = s
	return s, nil
}

// dispatch wraps one board route (rt) for the /b/{branch} prefix: resolve
// the branch, then hand the request to that branch's own instance of the
// EXISTING handler — the routing layer contains no board logic of its own.
func (b *branchBoards) dispatch(rt boardSpecRoute) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		branch := r.PathValue("branch")
		if !validBranchSegment(branch) {
			// Fail closed on anything no git branch can be named (empty,
			// "." or ".." segments): never let a hostile path segment
			// reach the filesystem-path mapping behind the seam.
			b.renderBranchGone(w, r, branch, rt)
			return
		}
		s, err := b.server(r.Context(), branch)
		switch {
		case err == nil:
			rt.handler(s)(w, r)
		case errors.Is(err, wtmanager.ErrCheckedOutHere):
			rt.handler(b.serving)(w, r)
		case errors.Is(err, wtmanager.ErrNotLocalBranch):
			b.serveRemoteOrGone(w, r, branch, rt)
		default:
			// dc-2: a failed cut is disclosed, naming the failure — never
			// a dead link, never a silent 500.
			b.renderCutFailure(w, r, branch, err, rt)
		}
	}
}

// validBranchSegment reports whether branch (the decoded path segment) is
// shaped like a name git could hold: no empty, "." or ".." path segments.
// git-check-ref-format forbids all of these, so rejecting them refuses no
// real branch — it only closes the path-traversal door a decoded %2F
// would otherwise open into the worktree path mapping.
func validBranchSegment(branch string) bool {
	if branch == "" {
		return false
	}
	for _, seg := range strings.Split(branch, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// serveRemoteOrGone routes wtmanager's ErrNotLocalBranch (dc-4): a branch
// resolving to a remote-tracking ref renders sealed from that ref; a
// branch resolving to no ref at all renders the disclosed 404 notice.
func (b *branchBoards) serveRemoteOrGone(w http.ResponseWriter, r *http.Request, branch string, rt boardSpecRoute) {
	ref := "origin/" + branch
	if _, err := gitx.RevParse(r.Context(), b.root, "refs/remotes/"+ref); err != nil {
		b.renderBranchGone(w, r, branch, rt)
		return
	}
	b.serveSealed(w, r, branch, ref, rt)
}

// serveSealed serves rt for a remote-only branch: the page and fragment
// routes render the board sealed from the remote-tracking ref's committed
// content (dc-4: read-only, remoteness disclosed, no worktree cut, no
// local branch minted); every other route needs a working tree none
// exists for, so it refuses with the same disclosure instead of lying
// with an empty success.
func (b *branchBoards) serveSealed(w http.ResponseWriter, r *http.Request, branch, ref string, rt boardSpecRoute) {
	switch rt.suffix {
	case routeBoardPage, routeBoardFragment:
	default:
		msg := fmt.Sprintf("branch %s resolves only to remote-tracking ref %s: its board is a sealed read-only render of that ref, and this route needs a working tree (none is cut for a remote-only branch)", branch, ref)
		if rt.json {
			writeJSONError(w, http.StatusForbidden, msg)
		} else {
			http.Error(w, msg, http.StatusForbidden)
		}
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.PathValue("name")
	proj, git, err := b.loadSealed(r.Context(), branch, ref, name)
	if errors.Is(err, ErrBoardNotFound) {
		b.renderSpecNotOnRef(w, name, ref)
		return
	}
	if err != nil {
		renderError(w, http.StatusInternalServerError, err)
		return
	}
	if rt.suffix == routeBoardFragment {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderBoardRegion(proj, git)))
		return
	}
	out, err := renderBoardSpecPage(proj, git)
	if err != nil {
		renderError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(out) // response body write; post-header error is unactionable
}

// loadSealed assembles the sealed board for a remote-only branch from the
// ref's COMMITTED content — spec.md and layout.json read via git plumbing
// (gitx.Show), no tree materialized anywhere. It calls the same
// buildProjection every board render uses (never a second projection);
// the mode passed is the EXISTING modeReadOnly value — dc-4's routing to
// a mode that already exists, not a new mode or a new mode rule. The
// scratch annotation tier and obligation enrichment are working-tree
// state a remote-only branch does not have here; their absence is part of
// the appended disclosure rather than silently implied.
func (b *branchBoards) loadSealed(ctx context.Context, branch, ref, name string) (*BoardProjection, *boardGitState, error) {
	if !specNameRe.MatchString(name) {
		return nil, nil, ErrBoardNotFound
	}
	specPath := store.ActiveSpecRelPath(name)
	raw, err := gitx.Show(ctx, b.root, ref, specPath)
	if err != nil {
		// `git show` cannot distinguish "no such path at this ref" from
		// other failures without a second probe; the ref itself was
		// verified by the caller, so a failed read here means the spec is
		// not on the ref — the board-not-found shape.
		return nil, nil, ErrBoardNotFound
	}
	fmBytes, bodyBytes, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("workbench: spec %s at %s: %w", name, ref, err)
	}
	fm, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("workbench: spec %s at %s: %w", name, ref, err)
	}

	stored := map[string]artifact.Position{}
	if lraw, lerr := gitx.Show(ctx, b.root, ref, ".verdi/specs/active/"+name+"/layout.json"); lerr == nil {
		bl, derr := artifact.DecodeBoardLayout(lraw)
		if derr != nil {
			return nil, nil, fmt.Errorf("workbench: layout for %s at %s: %w", name, ref, derr)
		}
		if bl.Positions != nil {
			stored = bl.Positions
		}
	}

	proj, err := buildProjection(name, fm, bodyBytes, stored, nil, nil, modeReadOnly)
	if err != nil {
		return nil, nil, err
	}
	// Display vocabulary (spec/vocabulary-surfaces ac-2): the sealed
	// remote-ref render resolves the same store model the served
	// instances carry.
	proj.applyModelVocabulary(b.deps.Model)
	proj.Notices = append(proj.Notices, fmt.Sprintf(
		"branch %s exists only as remote-tracking ref %s: this board is rendered sealed (read-only) from that ref's committed content — no worktree was cut and no local branch was created; fetch the branch as a local branch to author. The scratch annotation tier and obligation enrichment are working-tree state and are not read from a remote ref.",
		branch, ref))
	git := &boardGitState{Branch: ref, DefaultBranch: "", Branches: nil, Dirty: false}
	return proj, git, nil
}

// renderCutFailure is dc-2's disclosed error page: the worktree cut for a
// real local branch failed, and the failure is named to the human instead
// of buried in a log behind a dead link.
func (b *branchBoards) renderCutFailure(w http.ResponseWriter, r *http.Request, branch string, err error, rt boardSpecRoute) {
	msg := fmt.Sprintf("could not prepare the working tree for branch %s: %v", branch, err)
	if rt.json {
		writeJSONError(w, http.StatusInternalServerError, msg)
		return
	}
	_ = r
	renderError(w, http.StatusInternalServerError, errors.New(msg))
}

// renderBranchGone is dc-4's no-ref shape: the disclosed notice page —
// HTTP 404, a body naming the branch that resolves to no ref (local or
// remote-tracking), and a working link back to the directory. It IS the
// stale-entry surface the directory-home story discloses (its dc-5) —
// notfound.go's renderStaleEntryNotice, reused verbatim: with these
// routes registered, the grammar-matching paths its "/" catch-all used
// to answer resolve here instead, and both surfaces must stay one.
func (b *branchBoards) renderBranchGone(w http.ResponseWriter, r *http.Request, branch string, rt boardSpecRoute) {
	if rt.json {
		writeJSONError(w, http.StatusNotFound, fmt.Sprintf("design branch %s no longer resolves to any ref in this store", branch))
		return
	}
	_ = r
	renderStaleEntryNotice(w, branch)
}

// renderSpecNotOnRef is the sealed path's own 404: the branch's
// remote-tracking ref exists but carries no such spec — still a legible
// page with a way back (notfound.go's shared shell), never a bare
// NotFound.
func (b *branchBoards) renderSpecNotOnRef(w http.ResponseWriter, name, ref string) {
	var body strings.Builder
	body.WriteString(`<div class="error-page" role="alert" data-testid="stale-entry-notice">`)
	body.WriteString(`<p class="error-message"><strong>No such spec on this branch.</strong></p>`)
	body.WriteString(`<p>The remote-tracking ref <code>`)
	body.WriteString(stdhtml.EscapeString(ref))
	body.WriteString(`</code> exists, but carries no spec named <code>`)
	body.WriteString(stdhtml.EscapeString(name))
	body.WriteString(`</code> under <code>.verdi/specs/active/</code>.</p>`)
	writeBackToDirectory(&body)
	body.WriteString(`</div>`)
	writeNotFoundPage(w, body.String())
}
