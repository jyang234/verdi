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
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
	"github.com/OWNER/verdi/internal/boardlayout"
	"github.com/OWNER/verdi/internal/gitx"
)

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
}

// boardSpecServer holds the board's dependencies for one store root.
type boardSpecServer struct {
	root string
	feed CommentFeed

	// reviewUnavailable, when non-empty, is a disclosed reason the review
	// feed is CONFIGURED (a forge is named in verdi.yaml) but cannot be
	// consulted — no credentials to build a live adapter at startup. The
	// board renders a visible notice in its chrome rather than silently
	// reading the missing input as "not under review" (I-1(b): a
	// configured-but-unavailable forge is disclosed, never silent;
	// constitution 2/10). Empty means either no forge is configured (silent
	// not-under-review is legitimate) or a live feed is wired.
	reviewUnavailable string

	// writeMu serializes board MUTATIONS within this process. D3's
	// process-level writer lock (I-12) keeps other processes out, but the
	// board's HTTP handlers run as concurrent goroutines against the same
	// files; without this, two overlapping read-modify-write actions
	// (spliceSpec on spec.md, actionPosition on layout.json, the boardio
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
func (s *boardSpecServer) loadBoard(ctx context.Context, name string) (*BoardProjection, *boardGitState, string, error) {
	if !specNameRe.MatchString(name) {
		return nil, nil, "", ErrBoardNotFound
	}
	raw, err := os.ReadFile(filepath.Join(s.specDir(name), "spec.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, "", ErrBoardNotFound
		}
		return nil, nil, "", fmt.Errorf("workbench: reading spec %s: %w", name, err)
	}
	fmBytes, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, nil, "", fmt.Errorf("workbench: spec %s: %w", name, err)
	}
	fm, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		return nil, nil, "", fmt.Errorf("workbench: spec %s: %w", name, err)
	}

	stored, err := boardlayout.ReadFile(s.specDir(name))
	if err != nil {
		return nil, nil, "", err
	}
	annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(s.root))
	if err != nil {
		return nil, nil, "", err
	}

	// The review feed is NON-BLOCKING on every render (I-2, 04 §Semantics'
	// degradation posture: never block rendering). A configured-but-erroring
	// feed degrades to a disclosed notice and underReview=false — authoring
	// and read-only boards render fully without a feed; a review board
	// renders the projection plus the disclosure. The startup-time
	// disclosure (forge configured but no credentials, s.reviewUnavailable)
	// seeds the notice; a render-time transport error overrides it with the
	// live reason.
	var comments []MRComment
	underReview := false
	reviewNotice := s.reviewUnavailable
	if s.feed != nil {
		c, ur, ferr := s.feed.ListMRComments(ctx, name)
		if ferr != nil {
			// Configured AND reachable enough to attempt, but the call
			// failed: disclose, never silence, never a 500 (I-1(b)/I-2).
			reviewNotice = "review feed unavailable: " + ferr.Error()
		} else {
			comments, underReview = c, ur
		}
	}

	git, gitNotice, err := s.gitState(ctx)
	if err != nil {
		return nil, nil, "", err
	}

	mode := modeReadOnly
	switch {
	case underReview:
		// A spec with an open spec-MR: the board is a mirror of the MR
		// (05 §Workbench "Review").
		mode = modeReview
	case fm.Status == "draft" && git.Branch != "" && git.Branch != git.DefaultBranch:
		mode = modeAuthoring
	}
	if mode != modeReview {
		comments = nil // the feed is a review-mode input only
	}

	proj, err := buildProjection(name, fm, stored, annotations, comments, mode)
	if err != nil {
		return nil, nil, "", err
	}
	if reviewNotice != "" {
		proj.Notices = append(proj.Notices, reviewNotice)
	}
	if gitNotice != "" {
		proj.Notices = append(proj.Notices, gitNotice)
	}
	return proj, git, reviewNotice, nil
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
// unreachable disclosure Deps.ReviewUnavailable does. The returned
// reviewNotice is the review-feed disclosure alone (see loadBoard's doc
// comment) — get_board surfaces it as its own review_unavailable field,
// never folded silently into the generic notices a human board's chrome
// shows.
func LoadProjection(ctx context.Context, root, name string, feed CommentFeed, reviewUnavailable string) (proj *BoardProjection, reviewNotice string, err error) {
	s := &boardSpecServer{root: root, feed: feed, reviewUnavailable: reviewUnavailable}
	proj, _, reviewNotice, err = s.loadBoard(ctx, name)
	return proj, reviewNotice, err
}

// gitState queries the working tree's branch and dirtiness. When the
// default branch cannot be resolved (no origin/HEAD configured) it falls
// back to "main" — the board needs a non-empty "are we on a design branch"
// signal to key authoring-vs-read-only mode — but the assumption is
// DISCLOSED, never silent (M-4): the returned notice names it, since a
// repo whose real default is e.g. "master" would otherwise misread a
// checkout literally on "main" as the default branch and deny authoring
// mode. The notice feeds the board's rendered chrome at the call site.
func (s *boardSpecServer) gitState(ctx context.Context) (*boardGitState, string, error) {
	branch, err := gitx.CurrentBranch(ctx, s.root)
	if err != nil {
		return nil, "", err
	}
	def, err := gitx.DefaultBranch(ctx, s.root)
	if err != nil {
		return nil, "", err
	}
	notice := ""
	if def == "" {
		def = "main"
		notice = `default branch could not be resolved (no origin/HEAD configured); assuming "main" — authoring-mode detection may be wrong if this repo's real default differs`
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
		proj, git, _, err := s.loadBoard(r.Context(), r.PathValue("name"))
		if err == ErrBoardNotFound {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
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
		proj, git, _, err := s.loadBoard(r.Context(), r.PathValue("name"))
		if err == ErrBoardNotFound {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderBoardRegion(proj, git)))
	}
}
