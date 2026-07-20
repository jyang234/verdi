// The board page (05 §Workbench): cards (pinned refs), stickies
// (annotations, including board-only ones per I-34), yarn (proto-links),
// positions — autosaved via POST to data/mutable/boards/<story>.json,
// never committed per-drag. The one deliberately fat page (client-side
// drag via assets/board.js); the commit-to-design action is a thin HTTP
// wrapper over internal/commitdesign.Run — the same function
// `verdi board commit` calls (internal/commitdesign's doc comment: "the
// board page can call the verb's logic over HTTP" means THIS handler
// calls that Go function directly, in-process; it never shells out to
// the CLI).
package workbench

import (
	"bytes"
	"context"
	"encoding/json"
	stdhtml "html"
	"html/template"
	"io"
	"net/http"
	"strconv"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/commitdesign"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

const boardStateSchema = "verdi.board/v1"

// boardHandler answers GET /board/{key}: the board page. mdl is the
// store's resolved operating model — the commit-to-design copy and the
// proto-sticky type chips below speak class words, which are display
// prose and resolve (vocabulary.go); nil serves bare ids.
func boardHandler(root string, mdl *model.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		key := r.PathValue("key")
		if key == "" || !boardio.ValidStoryKey(key) {
			http.NotFound(w, r)
			return
		}

		path, err := boardio.BoardStatePath(root, key)
		if err != nil {
			renderError(w, http.StatusBadRequest, err)
			return
		}
		board, err := boardio.LoadBoardState(path)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}

		annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		byID := make(map[string]*artifact.Annotation, len(annotations))
		for _, a := range annotations {
			byID[a.ID] = a
		}

		clientState := boardClientState{
			Key:      key,
			Pins:     board.Pins,
			Stickies: make([]stickyView, 0, len(board.Stickies)),
			Yarn:     board.Yarn,
		}
		if clientState.Pins == nil {
			clientState.Pins = []artifact.Pin{}
		}
		if clientState.Yarn == nil {
			clientState.Yarn = []artifact.Yarn{}
		}
		for _, s := range board.Stickies {
			// The placeholder names WHICH annotation the board file cites
			// (errors name what they're about): a bare "(annotation not
			// found)" left the reader no thread to pull.
			sv := stickyView{ID: s.ID, X: s.X, Y: s.Y, Body: "(annotation " + s.ID + " not found in this store's mutable streams)", Type: "unknown", Status: "unknown"}
			if a, ok := byID[s.ID]; ok {
				sv.Body = a.Body
				sv.Type = string(a.Type)
				sv.Author = a.Author
				sv.Status = string(a.Status)
			}
			clientState.Stickies = append(clientState.Stickies, sv)
		}

		out, err := renderBoardPage(clientState, classWords{m: mdl})
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out) // response body write; post-header error is unactionable
	}
}

// stickyView is one sticky's resolved rendering data: position plus the
// annotation content it anchors — resolved by scanning every stream under
// data/mutable/annotations/ (boardio.ReadAllAnnotations) and matching by
// id, not by any assumed file-naming convention, since a board sticky's
// annotation may live in a target-keyed stream (it was authored against
// an artifact target and only later dragged onto the board) as well as a
// board-only stream (I-34).
type stickyView struct {
	ID     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Body   string  `json:"body"`
	Type   string  `json:"type"`
	Author string  `json:"author"`
	Status string  `json:"status"`
}

// boardClientState is the JSON payload embedded into the board page for
// assets/board.js to drive.
type boardClientState struct {
	Key      string          `json:"key"`
	Pins     []artifact.Pin  `json:"pins"`
	Stickies []stickyView    `json:"stickies"`
	Yarn     []artifact.Yarn `json:"yarn"`
}

var boardPageTemplate = template.Must(template.New("board").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Board: {{.Key}} · verdi workbench</title>
<link rel="stylesheet" href="/assets/style.css">
</head>
<body class="board-page">
<header class="site-head">
<a class="wordmark" href="/"><span class="leafmark" aria-hidden="true"></span>verdi<span class="wordmark-surface">workbench</span></a>
<nav class="site-nav workbench-nav"><a href="/">index</a></nav>
</header>
<header class="page-header board-head">
<h1>Board: {{.Key}}</h1>
<div id="autosave-status" role="status" aria-live="polite"></div>
</header>
{{.Body}}
<script>
window.__BOARD_KEY__ = {{.KeyJSON}};
window.__BOARD__ = {{.StateJSON}};
</script>
<script src="/assets/board.js"></script>
</body>
</html>
`))

func renderBoardPage(state boardClientState, words classWords) ([]byte, error) {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	keyJSON, err := json.Marshal(state.Key)
	if err != nil {
		return nil, err
	}

	data := struct {
		Key       string
		Body      template.HTML
		StateJSON template.JS
		KeyJSON   template.JS
	}{
		Key:       state.Key,
		Body:      template.HTML(boardPageBody(state, words)),
		StateJSON: template.JS(stateJSON),
		KeyJSON:   template.JS(keyJSON),
	}

	var buf bytes.Buffer
	if err := boardPageTemplate.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func boardPageBody(state boardClientState, words classWords) string {
	var b bytes.Buffer
	b.WriteString(`<div class="board-layout">`)

	// The spatial canvas: index cards (pinned refs) and paper stickies at
	// their stored coordinates; board.js overlays the yarn as SVG thread.
	b.WriteString(`<div id="board-canvas" class="board-canvas">`)
	for _, p := range state.Pins {
		b.WriteString(`<div class="card" data-drag data-kind="pin" data-key="` + stdhtml.EscapeString(p.Ref) + `" style="left:` + floatStr(p.X) + `px;top:` + floatStr(p.Y) + `px">`)
		b.WriteString(`<span class="card-kind">pinned ref</span>`)
		// The full pinned form stays intact as the element's text (and in
		// title); CSS (.card-ref) only truncates it visually.
		b.WriteString(`<span class="card-ref" title="` + stdhtml.EscapeString(p.Ref) + `">` + stdhtml.EscapeString(p.Ref) + `</span>`)
		b.WriteString(`</div>`)
	}
	for _, s := range state.Stickies {
		// The visible type word of a story/spike proto-sticky resolves as
		// a class word (display prose, vocabulary.go); every other type is
		// annotation taxonomy and renders verbatim. data-type keeps the
		// bare enum id regardless.
		typeLabel := s.Type
		if s.Type == string(artifact.AnnotationStory) || s.Type == string(artifact.AnnotationSpike) {
			typeLabel = words.word(s.Type)
		}
		b.WriteString(`<div class="sticky sticky--` + stickyTypeClass(s.Type) + `" data-drag data-kind="sticky" data-key="` + stdhtml.EscapeString(s.ID) + `" data-type="` + stdhtml.EscapeString(s.Type) + `" data-status="` + stdhtml.EscapeString(s.Status) + `" style="left:` + floatStr(s.X) + `px;top:` + floatStr(s.Y) + `px">`)
		b.WriteString(`<span class="sticky-type">` + stdhtml.EscapeString(typeLabel) + `</span>`)
		b.WriteString(`<p class="sticky-body">` + stdhtml.EscapeString(s.Body) + `</p>`)
		meta := s.Status
		if s.Author != "" {
			meta = s.Author + " · " + s.Status
		}
		b.WriteString(`<span class="sticky-meta">` + stdhtml.EscapeString(meta) + `</span>`)
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)

	// The side column: the yarn ledger and the commit-to-design ritual.
	b.WriteString(`<div class="board-side">`)

	b.WriteString(`<section class="yarn"><h2>Yarn</h2>`)
	if len(state.Yarn) == 0 {
		b.WriteString(`<p class="empty">No yarn strung yet.</p>`)
	} else {
		b.WriteString(`<ul class="yarn-list">`)
		for _, y := range state.Yarn {
			b.WriteString(`<li>` + stdhtml.EscapeString(y.From) + ` &rarr; ` + stdhtml.EscapeString(y.To) + ` <span class="yarn-label">(` + stdhtml.EscapeString(y.Label) + `)</span></li>`)
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(`</section>`)

	// The ritual copy's "feature" and the tracker-ref field's "Story"
	// label are class words — display prose, resolved (vocabulary.go).
	// The form's name/story_ref field NAMES and the commit-to-design API
	// contract stay bare ids.
	b.WriteString(`<section class="commit-to-design"><h2>Commit to design</h2>` +
		`<p class="ritual-note">Freezes this board into a draft ` + stdhtml.EscapeString(words.word("feature")) + ` spec: every sticky lands in the spec's dispositions block as an open question to incorporate or contradict.</p>` +
		// No HTML5 `required` here: an empty name must exercise the
		// SERVER's own validation (boardCommitHandler's "name is
		// required" 400), the same negative path a non-browser API
		// client would hit — browser-native validation would silently
		// swallow that test case before any request is even sent.
		`<form id="commit-form">` +
		`<div class="field"><label for="commit-name">Spec name</label><input id="commit-name" name="name" autocomplete="off"></div>` +
		`<div class="field"><label for="commit-story-ref">` + stdhtml.EscapeString(words.capital("story")) + ` ref <span class="optional">(optional)</span></label><input id="commit-story-ref" name="story_ref" autocomplete="off"></div>` +
		`<button type="submit">Commit to design</button></form>` +
		`<div id="commit-result" role="status"></div></section>`)

	b.WriteString(`</div></div>`)

	return b.String()
}

// stickyTypeClass maps an annotation type to its sticky-note CSS modifier.
// Only the sticky-creatable annotation types (internal/artifact) — the
// four generic ones plus the scoping canvas's story/spike proto-stickies
// (round 5.4) — get a paper color of their own; anything else, including
// the "(annotation <id> not found …)" placeholder's "unknown", falls
// back to the neutral note.
func stickyTypeClass(t string) string {
	switch t {
	case "comment", "question", "decision-needed", "agent-task", "story", "spike":
		return t
	default:
		return "unknown"
	}
}

func floatStr(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// autosavePayload is POST /board/{key}/autosave's JSON body — pins,
// stickies, and yarn only (no schema/frozen/provenance: strict decode
// rejects any of those as unknown fields, so a caller can never
// accidentally — or maliciously — autosave a frozen snapshot's shape into
// the live mutable board).
type autosavePayload struct {
	Pins     []artifact.Pin    `json:"pins"`
	Stickies []artifact.Sticky `json:"stickies"`
	Yarn     []artifact.Yarn   `json:"yarn"`
}

// boardAutosaveHandler answers POST /board/{key}/autosave: decode, then
// atomically overwrite data/mutable/boards/<key>.json (boardio's
// temp-then-rename — D3: "never committed per-drag").
func boardAutosaveHandler(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		key := r.PathValue("key")
		if key == "" || !boardio.ValidStoryKey(key) {
			writeJSONError(w, http.StatusBadRequest, "invalid board key")
			return
		}

		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "reading request body: "+err.Error())
			return
		}
		var payload autosavePayload
		if err := artifact.DecodeStrictJSON(raw, &payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "malformed autosave payload: "+err.Error())
			return
		}

		board := &artifact.Board{
			Schema:   boardStateSchema,
			Pins:     payload.Pins,
			Stickies: payload.Stickies,
			Yarn:     payload.Yarn,
		}
		path, err := boardio.BoardStatePath(root, key)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := boardio.SaveBoardState(path, board); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// commitRequest is POST /board/{key}/commit's JSON body.
type commitRequest struct {
	Name     string `json:"name"`
	StoryRef string `json:"story_ref"`
}

// commitResponse is the JSON success shape.
type commitResponse struct {
	SpecRef      string `json:"spec_ref"`
	SpecPath     string `json:"spec_path"`
	BoardPath    string `json:"board_path"`
	Dispositions int    `json:"dispositions"`
	Commit       string `json:"commit"`
}

// boardCommitHandler answers POST /board/{key}/commit: the workbench's
// commit-to-design action — calls internal/commitdesign.Run directly
// (see this file's top doc comment).
func boardCommitHandler(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		key := r.PathValue("key")
		if key == "" {
			writeJSONError(w, http.StatusBadRequest, "board key is required")
			return
		}

		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "reading request body: "+err.Error())
			return
		}
		var req commitRequest
		if err := artifact.DecodeStrictJSON(raw, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "malformed commit request: "+err.Error())
			return
		}
		if req.Name == "" {
			writeJSONError(w, http.StatusBadRequest, "name is required")
			return
		}

		cfg, err := store.Open(root)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "resolving store config: "+err.Error())
			return
		}
		modelDigest, err := cfg.Model.Digest()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "computing model digest: "+err.Error())
			return
		}

		res, err := commitdesign.Run(context.Background(), commitdesign.Input{
			Root: root, BoardKey: key, SpecName: req.Name, StoryRef: req.StoryRef, ModelDigest: modelDigest,
		})
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, commitResponse{
			SpecRef: res.SpecRef, SpecPath: res.SpecRelPath, BoardPath: res.BoardRelPath,
			Dispositions: len(res.Dispositions), Commit: res.Commit,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // response body write; post-header error is unactionable
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
