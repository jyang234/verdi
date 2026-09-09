package workbench

// The import/pin surface and the trash gesture's server half.
//
// Pinning (02 §Record schemas round-5.2, 05 §The scratch tier): an
// existing artifact goes on the wall as planning material — an
// annotation record of type pin, mutable-zone only, never the spec
// document. Its graduation is drawing a typed edge to the pinned target
// (actionEdge calls graduatePinsFor); its death is the trash.
//
// Trashing (owner directive, round 7): dropping a wall element on the
// trash removes it per tier — scratch records die from the mutable
// stream; a reference card's typed edges leave the spec document in one
// splice batch; a declared object's frontmatter entry goes together with
// every link naming its fragment (VL-003 stays green), its layout key
// (VL-018), and its scratch threads. Body prose is NEVER deleted.
// Everything here sits behind the same authoring-only 403 gate as every
// other board write.

import (
	"context"
	"errors"
	"fmt"
	stdhtml "html"
	"net/http"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/store"
)

// referenceLane is the references column band — the pin's landing lane.
func referenceLane() boardlayout.ZoneColumn {
	for _, c := range boardlayout.ZoneColumns() {
		if c.Kind == boardlayout.ZoneReference {
			return c
		}
	}
	return boardlayout.ScratchColumn() // unreachable: zoneOrder holds it
}

// laneBottomPosition appends a new element of the given footprint height
// to the BOTTOM of a lane — the same landing policy stickyLanePosition
// established: below every element whose footprint intersects the band,
// or the lane's first slot when it is empty. Deterministic given the
// projection.
func laneBottomPosition(proj *BoardProjection, lane boardlayout.ZoneColumn) (float64, float64) {
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

// actionPin: the supply toolbox's commit — pin an existing corpus
// artifact to this wall as planning material. The ref must resolve in
// the working tree's index (fail closed), must not be this board's own
// spec, and must not already have a card (one card per ref, ever). The
// record stores the ref pinned at HEAD (02 §Identity: board pins carry
// the pinned form) and lands at the bottom of the references lane.
func (s *boardSpecServer) actionPin(ctx context.Context, name string, proj *BoardProjection, req boardAPIRequest) error {
	if req.Ref == "" {
		return fmt.Errorf("pin requires a ref")
	}
	r, err := artifact.ParseRef(req.Ref)
	if err != nil {
		return fmt.Errorf("pin: %w", err)
	}
	if r.Pinned() {
		return fmt.Errorf("pin takes an unpinned ref (the server pins it at HEAD)")
	}
	if r.Fragment() {
		return fmt.Errorf("a pin names a whole artifact, not a fragment (02 §Record schemas)")
	}
	if r.Kind == artifact.KindSpec && r.Name == name {
		return fmt.Errorf("spec/%s is this board's own spec — the wall already is it", name)
	}
	for _, rc := range proj.RefCards {
		if rc.Ref == req.Ref {
			return fmt.Errorf("%s already has a card on this wall (one card per ref)", req.Ref)
		}
	}
	ix, err := index.Build(s.root)
	if err != nil {
		return fmt.Errorf("pin: building the corpus index: %w", err)
	}
	entry, ok := ix.Get(req.Ref)
	if !ok || entry.Kind == "external" {
		return fmt.Errorf("%s is not in this corpus; nothing to pin", req.Ref)
	}
	head, err := gitx.RevParse(ctx, s.root, "HEAD")
	if err != nil {
		return err
	}

	a, err := newAnnotation(artifact.AnnotationPin, req.Text) // body optional: the why
	if err != nil {
		return err
	}
	a.Target = &artifact.Target{Ref: req.Ref + "@" + head}
	x, y := laneBottomPosition(proj, referenceLane())
	a.Board = &artifact.BoardAnchor{Story: name, X: x, Y: y}
	return boardio.AppendAnnotation(boardio.AnnotationsDir(s.root), boardio.AnnotationFileForBoard(store.RefSlug(name)), a)
}

// graduatePinsFor flips the pin record(s) whose card just earned a typed
// edge to graduated (02 §Record schemas: "its graduation is drawing a
// typed edge to the pinned target"). The card stays — the edge projects
// it now — and files into the references lane on the next render.
func (s *boardSpecServer) graduatePinsFor(proj *BoardProjection, endpoint string) error {
	var ids []string
	for _, rc := range proj.RefCards {
		if rc.Pinned && rc.Ref == endpoint {
			ids = append(ids, rc.PinID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	_, err := boardio.GraduateStickies(boardio.AnnotationsDir(s.root), ids)
	return err
}

// pinSearchLimit caps one fragment's rows; the remainder is disclosed.
const pinSearchLimit = 50

// boardPinSearchHandler answers GET /board/spec/{name}/pinsearch?q= —
// the supply toolbox's picker over the corpus index (index.Build, like
// the peek): kind + title + ref per row, deterministic ordering (the
// search's score-then-ref order; the whole corpus sorted by ref when the
// query is empty). The board's own spec, external refs, and refs already
// on the wall are excluded. Authoring-only, like every board write
// affordance (the picker exists to mutate).
func (s *boardSpecServer) boardPinSearchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.PathValue("name")
		proj, _, _, _, err := s.loadBoard(r.Context(), name)
		if errors.Is(err, ErrBoardNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if proj.Mode != modeAuthoring {
			http.Error(w, "the pin toolbox exists only on an authoring board", http.StatusForbidden)
			return
		}
		ix, err := index.Build(s.root)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		onWall := map[string]bool{"spec/" + name: true}
		for _, rc := range proj.RefCards {
			onWall[rc.Ref] = true
		}
		eligible := func(e *index.Entry) bool {
			return e.Kind != "external" && !onWall[e.Ref]
		}

		var entries []*index.Entry
		query := r.URL.Query().Get("q")
		if strings.TrimSpace(query) == "" {
			for _, e := range ix.All() { // sorted by ref: deterministic
				if eligible(e) {
					entries = append(entries, e)
				}
			}
		} else {
			for _, hit := range ix.Search(query) { // score desc, ref asc
				if e, ok := ix.Get(hit.Ref); ok && eligible(e) {
					entries = append(entries, e)
				}
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderPinResults(entries, query)))
	}
}

// renderPinResults renders the picker fragment: one button per artifact
// (kind chip, title, ref), or the disclosed empty state.
func renderPinResults(entries []*index.Entry, query string) string {
	esc := stdhtml.EscapeString
	var b strings.Builder
	if len(entries) == 0 {
		b.WriteString(`<p class="pin-results-empty" data-testid="pin-results-empty">Nothing in this corpus matches`)
		if q := strings.TrimSpace(query); q != "" {
			b.WriteString(` &#8220;` + esc(q) + `&#8221;`)
		}
		b.WriteString(`.</p>`)
		return b.String()
	}
	shown := entries
	if len(shown) > pinSearchLimit {
		shown = shown[:pinSearchLimit]
	}
	for _, e := range shown {
		b.WriteString(`<button type="button" class="pin-result" data-testid="pin-result-` + esc(strings.ReplaceAll(e.Ref, "/", "-")) + `" data-ref="` + esc(e.Ref) + `">`)
		b.WriteString(`<span class="pin-result-kind">` + esc(e.Kind) + `</span>`)
		b.WriteString(`<span class="pin-result-title">` + esc(e.Title) + `</span>`)
		b.WriteString(`<span class="pin-result-ref">` + esc(e.Ref) + `</span>`)
		b.WriteString(`</button>`)
	}
	if rest := len(entries) - len(shown); rest > 0 {
		b.WriteString(`<p class="pin-results-more">` + esc(fmt.Sprintf("%d more — narrow the search.", rest)) + `</p>`)
	}
	return b.String()
}
