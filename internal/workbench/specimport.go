package workbench

// The mechanical spec importer's browser adapter (docs/superpowers/specs/
// 2026-09-14-spec-import-contract.md, "Errors and browser behavior"; plan
// Task 4 UI): the four adopted routes —
//
//	GET  /design/import                       the import page
//	POST /design/import/preview               strict Request JSON -> PreviewResult
//	POST /design/import/apply                 strict Request JSON + X-Verdi-Import-Preview -> Result
//	GET  /design/import/record?branch=&spec=  the read-only source-record view
//
// — over the accepted Task 3 service through its production wiring
// (specimport.NewService) and the existing browser-human actor
// (mintBrowserActor, boardspecdesign.go). This file composes and decodes
// strict JSON and maps the service's closed error vocabulary onto HTTP
// statuses; it never parses a source, evaluates policy, publishes Git,
// mints a second actor, or invents a finding.
//
// Transport binding (the Task 4 UI preflight): both POST bodies are the
// exact strict Request JSON, decoded by the ONE decoder
// (specimport.DecodeRequest) under the contract's 12 MiB raw envelope cap;
// apply additionally carries the previewed digest in X-Verdi-Import-Preview
// (64 lowercase hex) — a missing or malformed digest is invalid-request/400,
// and the service's own recomputation stays authoritative. No actor or
// candidate field exists on the wire. Both mutation routes sit behind Go
// 1.25's net/http.CrossOriginProtection with zero trusted origins and zero
// bypass patterns, plus explicit method guards and the bounded body read.
//
// Status mapping (contract: "HTTP errors map malformed inputs to 400,
// oversized to 413, policy/actor to 403, stale/collision/dirty to 409,
// operational I/O to 500"; a completed preview with blocking findings is
// 200 with ready:false). The two completed-refusal classes the contract
// does not place — unresolved (a not-ready preview submitted for creation)
// and provenance-mismatch (an existing target whose committed proof does
// not reconcile) — answer 409 like the other completed refusals against
// current store state; ErrImportRecordMissing reports as the existing
// provenance-mismatch code (the CLI's own adopted mapping), never a new
// wire code. Operational classes (500) render a generic message and log
// the detail; every 4xx message is the service's own correction guidance,
// with any raw git command residue cut off before it reaches a browser.

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specimport"
	"github.com/jyang234/verdi/internal/store"
)

// The four adopted routes and the script asset.
const (
	routeSpecImportPage    = "/design/import"
	routeSpecImportPreview = "/design/import/preview"
	routeSpecImportApply   = "/design/import/apply"
	routeSpecImportRecord  = "/design/import/record"
	routeSpecImportJS      = "/assets/specimport.js"

	// specImportPreviewHeader carries the previewed digest on apply.
	specImportPreviewHeader = "X-Verdi-Import-Preview"
)

// specImportDigestRe is the adapter-level shape check for the apply
// header: exactly 64 lowercase hex characters (the contract's SHA-256
// lowercase-hex digest form), mirroring the CLI's own --preview check.
var specImportDigestRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// specImportServer holds the adapter's dependencies for one store root.
type specImportServer struct {
	root  string
	model *model.Model
	svc   *specimport.Service
	// origin is the same-origin guard for the two mutation routes: the
	// zero-configuration CrossOriginProtection — no AddTrustedOrigin, no
	// AddInsecureBypassPattern — consulted through Check before any body
	// byte is read.
	origin *http.CrossOriginProtection
}

func newSpecImportServer(root string, mdl *model.Model) *specImportServer {
	return &specImportServer{root: root, model: mdl, svc: specimport.NewService(), origin: http.NewCrossOriginProtection()}
}

// registerSpecImportRoutes mounts the adapter. Patterns are registered
// method-neutral like every other workbench route (handler.go's note), so
// each handler's own method check is what produces 405.
func registerSpecImportRoutes(mux *http.ServeMux, root string, mdl *model.Model) {
	s := newSpecImportServer(root, mdl)
	mux.HandleFunc(routeSpecImportPage, s.pageHandler)
	mux.HandleFunc(routeSpecImportPreview, s.previewHandler)
	mux.HandleFunc(routeSpecImportApply, s.applyHandler)
	mux.HandleFunc(routeSpecImportRecord, s.recordHandler)
	mux.HandleFunc(routeSpecImportJS, specImportJSHandler())
}

// specImportJSHandler serves the import page's one script — same minimal,
// dependency-free posture as the board scripts.
func specImportJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/specimport.js")
}

// pageHandler answers GET /design/import with the server-rendered page.
func (s *specImportServer) pageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	out, err := renderSpecImportPage(s.model)
	if err != nil {
		renderError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(out) // response body write; post-header error is unactionable
}

// guardMutation applies the method and same-origin guards shared by both
// POST routes, answering the refusal itself and reporting false when the
// request must not proceed.
func (s *specImportServer) guardMutation(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	if err := s.origin.Check(r); err != nil {
		writeJSONError(w, http.StatusForbidden, "cross-origin request refused: "+err.Error())
		return false
	}
	return true
}

// readSpecImportBody reads one request envelope, bounded by the contract's
// raw 12 MiB cap: a body over the cap answers 413 before any decode
// allocates a copy of it. It reports false after answering a refusal.
func readSpecImportBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, specimport.MaxEnvelopeBytes+1)
	raw, err := io.ReadAll(r.Body)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) || len(raw) > specimport.MaxEnvelopeBytes {
		writeSpecImportCode(w, http.StatusRequestEntityTooLarge, "invalid-request",
			fmt.Sprintf("request envelope exceeds the %d byte limit; select fewer or smaller source files", specimport.MaxEnvelopeBytes))
		return nil, false
	}
	if err != nil {
		writeSpecImportCode(w, http.StatusBadRequest, "invalid-request", "reading request body: "+err.Error())
		return nil, false
	}
	return raw, true
}

// previewHandler: POST /design/import/preview — read-only end to end. A
// preview with blocking findings is still 200 with ready:false.
func (s *specImportServer) previewHandler(w http.ResponseWriter, r *http.Request) {
	if !s.guardMutation(w, r) {
		return
	}
	raw, ok := readSpecImportBody(w, r)
	if !ok {
		return
	}
	req, err := specimport.DecodeRequest(raw)
	if err != nil {
		writeSpecImportFailure(w, err)
		return
	}
	result, err := s.svc.Preview(r.Context(), s.root, req)
	if err != nil {
		writeSpecImportFailure(w, err)
		return
	}
	writeSpecImportJSON(w, result)
}

// applyHandler: POST /design/import/apply — the same strict body plus the
// previewed digest header, applied through the one browser-human actor.
// Both created and already-created answer 200 with the Result verbatim,
// deferral disclosures included.
func (s *specImportServer) applyHandler(w http.ResponseWriter, r *http.Request) {
	if !s.guardMutation(w, r) {
		return
	}
	digest := r.Header.Get(specImportPreviewHeader)
	if digest == "" {
		writeSpecImportCode(w, http.StatusBadRequest, "invalid-request", "missing "+specImportPreviewHeader+" header: apply must name the exact preview digest it confirms")
		return
	}
	if !specImportDigestRe.MatchString(digest) {
		writeSpecImportCode(w, http.StatusBadRequest, "invalid-request", specImportPreviewHeader+" must be 64 lowercase hex characters")
		return
	}
	raw, ok := readSpecImportBody(w, r)
	if !ok {
		return
	}
	req, err := specimport.DecodeRequest(raw)
	if err != nil {
		writeSpecImportFailure(w, err)
		return
	}
	// The explicit browser-human actor — the repository's ONE production
	// minting site (boardspecdesign.go), taking no request-derived input.
	actor, err := mintBrowserActor()
	if err != nil {
		log.Printf("workbench: spec import apply: minting browser actor: %v", err)
		writeSpecImportCode(w, http.StatusInternalServerError, "identity-unavailable", "the browser actor could not be minted; the server log names the cause")
		return
	}
	result, err := s.svc.Apply(r.Context(), s.root, req, digest, actor)
	if err != nil {
		writeSpecImportFailure(w, err)
		return
	}
	writeSpecImportJSON(w, result)
}

// recordHandler: GET /design/import/record?branch=<branch>&spec=<slug> —
// the read-only view over that branch's committed bytes. A malformed query
// is a 400 page; unavailable, malformed or mismatching committed proof is
// the provenance-mismatch refusal rendered as its own page, distinct from
// the changed-current-spec disclosure a verified record carries.
func (s *specImportServer) recordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	branch := r.URL.Query().Get("branch")
	slug := r.URL.Query().Get("spec")
	if reason := validateSpecImportRecordQuery(branch, slug); reason != "" {
		renderSpecImportRecordInvalid(w, reason)
		return
	}
	view, err := specimport.ReadRecord(r.Context(), s.root, branch, slug)
	if err != nil {
		status, code, detail := specImportFailure(err)
		if status == http.StatusInternalServerError {
			log.Printf("workbench: spec import record %s %s: %v", branch, slug, err)
			detail = ""
		}
		renderSpecImportRecordUnavailable(w, status, code, detail, branch, slug)
		return
	}
	out, err := renderSpecImportRecord(view, branch, slug)
	if err != nil {
		renderError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(out) // response body write; post-header error is unactionable
}

// validateSpecImportRecordQuery returns "" for a well-formed query, else
// the reason: the spec must be a bare spec name (the board's own
// specNameRe), the branch shaped like a name git could hold (branchboard.go's
// validBranchSegment) and never option-shaped.
func validateSpecImportRecordQuery(branch, slug string) string {
	switch {
	case slug == "":
		return "the spec query parameter is required (a bare spec name)"
	case !specNameRe.MatchString(slug):
		return fmt.Sprintf("spec %q is not a bare spec name (kebab-case, no @commit pin or #object fragment)", slug)
	case branch == "":
		return "the branch query parameter is required"
	case strings.HasPrefix(branch, "-"):
		return fmt.Sprintf("branch %q must not begin with -", branch)
	case !validBranchSegment(branch):
		return fmt.Sprintf("branch %q is not shaped like a git branch name", branch)
	}
	return ""
}

// specImportRecordHref is the read-only record view's address for slug on
// branch — the one string template the created result, the board
// affordance and the page all share.
func specImportRecordHref(branch, slug string) string {
	return routeSpecImportRecord + "?branch=" + url.QueryEscape(branch) + "&spec=" + url.QueryEscape(slug)
}

// specImportRecordHrefFor returns the record view's address for name on
// branch when the board's working tree carries a committed import record
// for it at the contract's fixed sidecar path
// (.verdi/imports/<name>/<preview-digest>/record.json), else "". This is a
// presence check only — the record view is what verifies the bytes — so
// the board affordance links to the verifying view without becoming a
// second verifier.
func specImportRecordHrefFor(root, branch, name string) string {
	if branch == "" || !specNameRe.MatchString(name) {
		return ""
	}
	entries, err := os.ReadDir(filepath.Join(root, ".verdi", "imports", name))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(store.ImportRecordPath(root, name, e.Name())); err == nil {
			return specImportRecordHref(branch, name)
		}
	}
	return ""
}

// specImportSentinel pairs one specimport sentinel with the contract's wire
// code and this adapter's HTTP status.
type specImportSentinel struct {
	err    error
	code   string
	status int
}

// specImportSentinels is the closed status table (see the file comment).
var specImportSentinels = []specImportSentinel{
	{specimport.ErrInvalidRequest, "invalid-request", http.StatusBadRequest},
	{specimport.ErrInvalidSource, "invalid-source", http.StatusBadRequest},
	{specimport.ErrUnsupportedFormat, "unsupported-format", http.StatusBadRequest},
	{specimport.ErrPolicyForbidden, "policy-forbidden", http.StatusForbidden},
	{specimport.ErrActorForbidden, "actor-forbidden", http.StatusForbidden},
	{specimport.ErrDirtyContext, "dirty-context", http.StatusConflict},
	{specimport.ErrStalePreview, "stale-preview", http.StatusConflict},
	{specimport.ErrTargetExists, "target-exists", http.StatusConflict},
	{specimport.ErrUnresolved, "unresolved", http.StatusConflict},
	{specimport.ErrProvenanceMismatch, "provenance-mismatch", http.StatusConflict},
	{specimport.ErrImportRecordMissing, "provenance-mismatch", http.StatusConflict},
	{specimport.ErrInvalidModel, "invalid-model", http.StatusInternalServerError},
	{specimport.ErrIdentityUnavailable, "identity-unavailable", http.StatusInternalServerError},
	{specimport.ErrAuthorityInvalid, "authority-invalid", http.StatusInternalServerError},
	{specimport.ErrIOFailure, "io-failure", http.StatusInternalServerError},
}

// specImportFailure classifies err (one of the service's sentinel-wrapped
// errors) into status, wire code and a browser-safe detail. An error
// matching no sentinel is a defensive io-failure.
func specImportFailure(err error) (status int, code string, detail string) {
	for _, s := range specImportSentinels {
		if errors.Is(err, s.err) {
			return s.status, s.code, specImportSafeDetail(strings.TrimPrefix(err.Error(), s.err.Error()+": "))
		}
	}
	return http.StatusInternalServerError, "io-failure", ""
}

// specImportSafeDetail keeps the service's own human-readable correction
// guidance and cuts away any raw git command residue a wrapped gitx error
// would otherwise carry into a browser (command lines, stderr, exit
// statuses). The result is plain text; every renderer escapes it.
func specImportSafeDetail(detail string) string {
	for _, marker := range []string{"gitx:", "\n", "fatal:", "exit status"} {
		if i := strings.Index(detail, marker); i >= 0 {
			detail = detail[:i]
		}
	}
	return strings.TrimRight(strings.TrimSpace(detail), ":")
}

// writeSpecImportFailure renders one service error as the adapter's JSON
// failure body {"code","error"}: the contract code plus, for a refusal,
// the service's guidance; for an operational class, a generic message
// (the detail goes to the server log, never to the browser).
func writeSpecImportFailure(w http.ResponseWriter, err error) {
	status, code, detail := specImportFailure(err)
	if status == http.StatusInternalServerError {
		log.Printf("workbench: spec import %s: %v", code, err)
		detail = "an operational failure occurred; the server log names the cause"
	}
	writeSpecImportCode(w, status, code, detail)
}

func writeSpecImportCode(w http.ResponseWriter, status int, code, detail string) {
	writeJSON(w, status, map[string]string{"code": code, "error": detail})
}

// writeSpecImportJSON renders a service DTO as this store's canonical JSON
// (the same bytes `verdi design import` prints), HTTP 200.
func writeSpecImportJSON(w http.ResponseWriter, v any) {
	encoded, err := canonjson.Marshal(v)
	if err != nil {
		log.Printf("workbench: spec import: encoding result: %v", err)
		writeSpecImportCode(w, http.StatusInternalServerError, "io-failure", "an operational failure occurred while encoding the result")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded) // response body write; post-header error is unactionable
}
