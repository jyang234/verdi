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

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
	"github.com/OWNER/verdi/internal/boardlayout"
	"github.com/OWNER/verdi/internal/gitx"
)

// Deps carries the workbench's injected collaborators (04 §port
// pattern: interfaces defined at the consumer, wired by the caller).
type Deps struct {
	// CommentFeed is review mode's comment source. nil means no forge is
	// wired: no spec is ever under review and the board renders
	// authoring/read-only purely from branch state.
	CommentFeed CommentFeed
}

// boardSpecServer holds the board's dependencies for one store root.
type boardSpecServer struct {
	root string
	feed CommentFeed
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

// loadBoard assembles the projection's four inputs and the git state.
func (s *boardSpecServer) loadBoard(ctx context.Context, name string) (*boardProjection, *boardGitState, error) {
	if !specNameRe.MatchString(name) {
		return nil, nil, errBoardNotFound
	}
	raw, err := os.ReadFile(filepath.Join(s.specDir(name), "spec.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, errBoardNotFound
		}
		return nil, nil, fmt.Errorf("workbench: reading spec %s: %w", name, err)
	}
	fmBytes, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("workbench: spec %s: %w", name, err)
	}
	fm, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("workbench: spec %s: %w", name, err)
	}

	stored, err := boardlayout.ReadFile(s.specDir(name))
	if err != nil {
		return nil, nil, err
	}
	annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(s.root))
	if err != nil {
		return nil, nil, err
	}

	var comments []MRComment
	underReview := false
	if s.feed != nil {
		comments, underReview, err = s.feed.ListMRComments(ctx, name)
		if err != nil {
			return nil, nil, fmt.Errorf("workbench: comment feed for %s: %w", name, err)
		}
	}

	git, err := s.gitState(ctx)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}
	return proj, git, nil
}

// gitState queries the working tree's branch and dirtiness. An unknown
// default branch (no origin HEAD configured) falls back to "main" — the
// board only needs a best-effort "are we on a design branch" signal.
func (s *boardSpecServer) gitState(ctx context.Context) (*boardGitState, error) {
	branch, err := gitx.CurrentBranch(ctx, s.root)
	if err != nil {
		return nil, err
	}
	def, err := gitx.DefaultBranch(ctx, s.root)
	if err != nil {
		return nil, err
	}
	if def == "" {
		def = "main"
	}
	dirty, err := gitx.StatusDirty(ctx, s.root)
	if err != nil {
		return nil, err
	}
	branches, err := gitx.LocalBranches(ctx, s.root)
	if err != nil {
		return nil, err
	}
	return &boardGitState{Branch: branch, DefaultBranch: def, Branches: branches, Dirty: dirty}, nil
}

// errBoardNotFound distinguishes 404 from operational failures.
var errBoardNotFound = fmt.Errorf("workbench: no such spec board")

// boardSpecPageHandler answers GET /board/spec/{name}: the full page.
func (s *boardSpecServer) boardSpecPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		proj, git, err := s.loadBoard(r.Context(), r.PathValue("name"))
		if err == errBoardNotFound {
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
		proj, git, err := s.loadBoard(r.Context(), r.PathValue("name"))
		if err == errBoardNotFound {
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
