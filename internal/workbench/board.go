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

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardio"
	"github.com/OWNER/verdi/internal/commitdesign"
)

const boardStateSchema = "verdi.board/v1"

// boardHandler answers GET /board/{key}: the board page.
func boardHandler(root string) http.HandlerFunc {
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
			http.Error(w, "workbench: "+err.Error(), http.StatusBadRequest)
			return
		}
		board, err := boardio.LoadBoardState(path)
		if err != nil {
			http.Error(w, "workbench: "+err.Error(), http.StatusInternalServerError)
			return
		}

		annotations, err := boardio.ReadAllAnnotations(boardio.AnnotationsDir(root))
		if err != nil {
			http.Error(w, "workbench: "+err.Error(), http.StatusInternalServerError)
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
			sv := stickyView{ID: s.ID, X: s.X, Y: s.Y, Body: "(annotation not found)", Type: "unknown", Status: "unknown"}
			if a, ok := byID[s.ID]; ok {
				sv.Body = a.Body
				sv.Type = string(a.Type)
				sv.Author = a.Author
				sv.Status = string(a.Status)
			}
			clientState.Stickies = append(clientState.Stickies, sv)
		}

		out, err := renderBoardPage(clientState)
		if err != nil {
			http.Error(w, "workbench: "+err.Error(), http.StatusInternalServerError)
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
</head>
<body>
<nav class="workbench-nav"><a href="/">workbench</a></nav>
<header class="page-header"><h1>Board: {{.Key}}</h1></header>
<div class="page-body">
{{.Body}}
</div>
<script>
window.__BOARD_KEY__ = {{.KeyJSON}};
window.__BOARD__ = {{.StateJSON}};
</script>
<script src="/assets/board.js"></script>
</body>
</html>
`))

func renderBoardPage(state boardClientState) ([]byte, error) {
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
		Body:      template.HTML(boardPageBody(state)),
		StateJSON: template.JS(stateJSON),
		KeyJSON:   template.JS(keyJSON),
	}

	var buf bytes.Buffer
	if err := boardPageTemplate.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func boardPageBody(state boardClientState) string {
	var b bytes.Buffer
	b.WriteString(`<div id="autosave-status"></div>`)
	b.WriteString(`<div id="board-canvas" class="board-canvas">`)
	for _, p := range state.Pins {
		b.WriteString(`<div class="card" data-drag data-kind="pin" data-key="` + stdhtml.EscapeString(p.Ref) + `" style="left:` + floatStr(p.X) + `px;top:` + floatStr(p.Y) + `px">`)
		b.WriteString(`<strong>pin</strong> ` + stdhtml.EscapeString(p.Ref))
		b.WriteString(`</div>`)
	}
	for _, s := range state.Stickies {
		b.WriteString(`<div class="sticky" data-drag data-kind="sticky" data-key="` + stdhtml.EscapeString(s.ID) + `" style="left:` + floatStr(s.X) + `px;top:` + floatStr(s.Y) + `px" data-status="` + stdhtml.EscapeString(s.Status) + `">`)
		b.WriteString(`<strong>` + stdhtml.EscapeString(s.Type) + `</strong> ` + stdhtml.EscapeString(s.Body))
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)

	b.WriteString(`<section class="yarn"><h2>Yarn</h2><ul>`)
	for _, y := range state.Yarn {
		b.WriteString(`<li>` + stdhtml.EscapeString(y.From) + ` &rarr; ` + stdhtml.EscapeString(y.To) + ` (` + stdhtml.EscapeString(y.Label) + `)</li>`)
	}
	b.WriteString(`</ul></section>`)

	b.WriteString(`<section class="commit-to-design"><h2>Commit to design</h2>` +
		// No HTML5 `required` here: an empty name must exercise the
		// SERVER's own validation (boardCommitHandler's "name is
		// required" 400), the same negative path a non-browser API
		// client would hit — browser-native validation would silently
		// swallow that test case before any request is even sent.
		`<form id="commit-form"><label>Spec name <input id="commit-name" name="name"></label> ` +
		`<label>Story ref (optional) <input id="commit-story-ref" name="story_ref"></label> ` +
		`<button type="submit">Commit to design</button></form>` +
		`<div id="commit-result"></div></section>`)

	return b.String()
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

		res, err := commitdesign.Run(context.Background(), commitdesign.Input{
			Root: root, BoardKey: key, SpecName: req.Name, StoryRef: req.StoryRef,
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
