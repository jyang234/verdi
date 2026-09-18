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

// documentLoadStatus maps a loadDocument failure for name to its HTTP
// status: a spec the working tree does not hold (in either zone) is 404,
// every other failure is operational (500). The loader names a missing
// spec in its error text rather than with a sentinel ("spec/<name> not
// found in either zone of the working tree"), so the match is anchored
// to that exact phrase — never a bare "not found", which would turn an
// operational failure such as a missing git executable into a 404.
func documentLoadStatus(name string, err error) int {
	if errors.Is(err, ErrBoardNotFound) || strings.Contains(err.Error(), "spec/"+name+" not found") {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

// documentFormatFromQuery reads ?format=: empty (the page or the JSON
// projection) or "md" (the Markdown download on the page route; accepted
// and ignored on /snapshot, which is always JSON). Anything else is
// refused the same way on both routes.
func documentFormatFromQuery(r *http.Request) (string, error) {
	format := r.URL.Query().Get("format")
	if format != "" && format != "md" {
		return "", fmt.Errorf("format must be md, got %q", format)
	}
	return format, nil
}

// documentNotModified reports whether r's If-None-Match names etag. Both
// document responses that stamp a validator — the ?format=md download
// and the /snapshot projection — answer the SAME revision token for the
// same (spec, kind), so a client may carry one between them and the two
// routes must agree on when it is unchanged; hence one comparison, not a
// copy per handler.
func documentNotModified(r *http.Request, etag string) bool {
	match := r.Header.Get("If-None-Match")
	return match != "" && match == etag
}

// boardDocumentPageHandler answers GET /board/spec/{name}/document: the
// Document tab, or — under ?format=md — the raw Markdown as a download
// (the exact bytes the snapshot carries, stamped with the same ETag and
// answering the same conditional request). The tab page itself stamps no
// validator and is unconditional.
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
			renderError(w, http.StatusBadRequest, err)
			return
		}
		format, err := documentFormatFromQuery(r)
		if err != nil {
			renderError(w, http.StatusBadRequest, err)
			return
		}
		snap, err := s.loadDocument(r.Context(), name, kind)
		if err != nil {
			// The HTML route fails as the board does: renderError's page,
			// never a plain-text body (/snapshot keeps JSON errors).
			renderError(w, documentLoadStatus(name, err), err)
			return
		}
		if format == "md" {
			// Stamping a validator obliges honouring the conditional
			// request it invites: an unchanged token answers 304 with no
			// body, exactly as /snapshot does, so re-downloading the same
			// document does not re-transfer it. The entity headers below
			// are deliberately not set on the 304 — RFC 7232 requires only
			// the validator there, which is set first so both arms carry
			// it.
			etag := `"` + snap.Revision + `"`
			w.Header().Set("ETag", etag)
			if documentNotModified(r, etag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.md"`, name, kind))
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
		if _, err := documentFormatFromQuery(r); err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		name := r.PathValue("name")
		snap, err := s.loadDocument(r.Context(), name, kind)
		if err != nil {
			writeJSONError(w, documentLoadStatus(name, err), err.Error())
			return
		}
		etag := `"` + snap.Revision + `"`
		w.Header().Set("ETag", etag)
		if documentNotModified(r, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		writeJSON(w, http.StatusOK, snap)
	}
}
