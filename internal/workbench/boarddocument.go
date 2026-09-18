package workbench

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
)

// The board's Document tab (spec/spec-documents ac-4, wave 2 task 5): a
// reading of the served spec's objects rendered through the SAME loader
// the CLI, the docs site, and MCP use (internal/specdocload), so the
// Markdown is byte-identical across every consumer (ac-6). Two GET-only
// routes in the shared board route table — the page (which doubles as
// the Markdown download under ?format=md) and the conditional /snapshot
// projection the page polls. Nothing here is authority (co-2): the
// document's own footer says so, and no route writes.

const (
	routeBoardDocument         = "/board/spec/{name}/document"
	routeBoardDocumentSnapshot = "/board/spec/{name}/document/snapshot"
)

// documentSnapshot is the Document tab's conditional projection: the
// rendered HTML fragment, the canonical Markdown it was rendered from,
// and a revision token over the ref, kind, and Markdown. The Markdown
// already carries the stamp (commit, proposed posture, engine), so any
// change a reader could see moves the token.
type documentSnapshot struct {
	Revision string `json:"revision"`
	HTML     string `json:"html"`
	Markdown string `json:"markdown"`
	Kind     string `json:"kind"`
	Ref      string `json:"ref"`
	Proposed bool   `json:"proposed"`
}

// loadDocument renders the working tree of the checkout this server
// serves (root mount: the serving checkout; branch mount: that branch's
// worktree), stamped with its HEAD and marked proposed unless the bytes
// are the exact accepted bytes on the default branch — the loader's own
// ModeWorkingTree rule, never re-derived here. Readiness reaches the
// document only when the served snapshot targets this spec (the loader
// gates on TargetRef). Loader disclosures are not surfaced separately:
// the document says, section by section, which facts were unavailable.
func (s *boardSpecServer) loadDocument(ctx context.Context, name string, kind specdoc.Kind) (documentSnapshot, error) {
	if !specNameRe.MatchString(name) {
		return documentSnapshot{}, fmt.Errorf("workbench: spec %q not found: %w", name, ErrBoardNotFound)
	}
	res, err := specdocload.Load(ctx, specdocload.Request{Root: s.root, Name: name, Mode: specdocload.ModeWorkingTree, Kind: kind, Model: s.model, Readiness: s.readiness})
	if err != nil {
		return documentSnapshot{}, err
	}
	doc, err := specdoc.Build(res.Input)
	if err != nil {
		return documentSnapshot{}, err
	}
	md := specdoc.RenderMarkdown(doc)
	html, err := specdoc.RenderHTML(doc)
	if err != nil {
		return documentSnapshot{}, err
	}
	snap := documentSnapshot{HTML: html, Markdown: md, Kind: string(kind), Ref: doc.Stamp.Ref, Proposed: doc.Stamp.Proposed}
	rev, err := canonjson.Digest(struct {
		Ref, Kind, Markdown string
	}{snap.Ref, snap.Kind, snap.Markdown})
	if err != nil {
		return documentSnapshot{}, err
	}
	snap.Revision = rev
	return snap, nil
}

// documentKindFromQuery reads ?kind=, defaulting to the spec document;
// an unknown kind is the loader's own refusal (fail closed).
func documentKindFromQuery(r *http.Request) (specdoc.Kind, error) {
	k := r.URL.Query().Get("kind")
	if k == "" {
		return specdoc.KindSpec, nil
	}
	return specdoc.ParseKind(k)
}

// documentLoadStatus maps a loadDocument failure to its HTTP status: a
// spec the working tree does not hold (in either zone) is 404, every
// other failure is operational. The loader names a missing spec in its
// error text rather than with a sentinel, so the not-found shape is
// recognized by that text — confined to this one function.
func documentLoadStatus(err error) int {
	if errors.Is(err, ErrBoardNotFound) || strings.Contains(err.Error(), "not found") {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

// boardDocumentPageHandler answers GET /board/spec/{name}/document: the
// Document tab, or — under ?format=md — the raw Markdown as a download
// (the exact bytes the snapshot carries, stamped with the same ETag).
func (s *boardSpecServer) boardDocumentPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.PathValue("name")
		kind, err := documentKindFromQuery(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		format := r.URL.Query().Get("format")
		if format != "" && format != "md" {
			http.Error(w, fmt.Sprintf("format must be md, got %q", format), http.StatusBadRequest)
			return
		}
		snap, err := s.loadDocument(r.Context(), name, kind)
		if err != nil {
			http.Error(w, err.Error(), documentLoadStatus(err))
			return
		}
		if format == "md" {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.md"`, name, kind))
			w.Header().Set("ETag", `"`+snap.Revision+`"`)
			_, _ = w.Write([]byte(snap.Markdown)) // response body write; post-header error is unactionable
			return
		}
		// EscapedPath, not Path: under the /b/{branch} mount the branch
		// rides one segment with its slashes percent-encoded, and every
		// sibling link on the page must keep that encoding to resolve.
		page, err := renderBoardDocumentPage(r.URL.EscapedPath(), name, snap)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page) // response body write; post-header error is unactionable
	}
}

// boardDocumentSnapshotHandler answers GET /board/spec/{name}/document/
// snapshot: the conditional projection. The ETag carries the revision
// token and an unchanged If-None-Match answers 304 with no body, so a
// poll leaves the page untouched (the boardSpecSnapshotHandler idiom).
func (s *boardSpecServer) boardDocumentSnapshotHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		kind, err := documentKindFromQuery(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		snap, err := s.loadDocument(r.Context(), r.PathValue("name"), kind)
		if err != nil {
			writeJSONError(w, documentLoadStatus(err), err.Error())
			return
		}
		etag := `"` + snap.Revision + `"`
		w.Header().Set("ETag", etag)
		if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		writeJSON(w, http.StatusOK, snap)
	}
}
