package workbench

// The ASD workbench's browser design adapter (Wave 6 Task 2, design §6.2;
// SI-163/SI-167/SI-176/SI-177): the six application operations exposed as
// POST /board/spec/{name}/api/<exact-application-operation>, every domain
// board mutation routed through the injected designapp bridge as ONE
// typed draftmutation transaction, and the explicit unauthenticated-human
// browser actor minted HERE and nowhere else.
//
// This file is deliberately the ONLY internal/workbench production file
// that imports internal/draftmutation (the boundary witness in
// internal/draftmutation/boundary_test.go pins exactly that), and its
// mintBrowserActor call is the repository's ONE production caller of
// draftmutation.NewUnauthenticatedHuman
// (TestNewUnauthenticatedHumanHasExactlyOneProductionCaller).
//
// Custody: the browser, its form fields, cookies, the OS user, the Git
// author, and this server's own identity can never mint a principal
// (design §4.1). The actor minted here is the kernel's explicit
// unauthenticated-human attribution — provenance only, never authority —
// and no request field feeds its construction.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/draftmutation"
)

// designFailureSchema mirrors internal/designapp.FailureSchema byte for
// byte — the ONE versioned wire envelope for an application failure on a
// transport with no exit-code channel. workbench cannot import designapp
// (designapp imports workbench for the board projection), so the literal
// is re-declared here and pinned equal to designapp's constant by
// TestWorkbenchDesignFailureSchemaMatchesDesignApp (cmd/verdi, which may
// import both).
const designFailureSchema = "verdi.design-failure/v1"

// maxDesignActionBytes caps every design-action body — the same 1 MiB
// ceiling draftmutation.MaxRequestBytes puts on a mutation request, applied
// to the RAW incoming bytes before any allocation-heavy decode (the
// mcpserve precedent).
const maxDesignActionBytes = draftmutation.MaxRequestBytes

// DesignFailure is the bridge's typed non-clean outcome for the five read
// operations: the exact classification/code/detail designapp's own Failure
// envelope carries (§4.3: tests assert the typed classification in the
// body, never the HTTP status).
type DesignFailure struct {
	Classification string `json:"classification"`
	Code           string `json:"code"`
	Detail         string `json:"detail"`
}

// DesignReadOutcome is one read operation's result union: JSON carries the
// canonical designapp envelope bytes (the same bytes `verdi design <op>`
// prints — AC-2's byte-level adapter parity by construction) exactly when
// Failure is nil.
type DesignReadOutcome struct {
	JSON    []byte
	Failure *DesignFailure
}

// DesignCapabilitiesView is the narrow, adapter-converted slice of
// get_design_capabilities the page render consumes (agent-posture facts
// for the check-context area). It is converted field-by-field by the
// injecting adapter from designapp's own typed result — never re-decoded
// from JSON here, so there is no second wire interpretation.
type DesignCapabilitiesView struct {
	Mutable             bool
	RefusalPrecondition string
	RefusalDetail       string
	PolicyMode          string
	PolicyDigest        string
	SpecState           string
	CurrentDigest       string
	PermittedOperations []string
}

// DesignBridge is the workbench's consumer-owned port onto the one ASD
// application core (04 §port pattern). Production wiring injects
// internal/designapp behind it (cmd/verdi/servedesign.go): MutateDraft has
// designapp.Service.MutateDraft's exact signature, and each read method is
// a thin call-plus-canonical-encode over the matching designapp operation.
// nil Deps.Design means the design application service is not wired; every
// design action then discloses that operationally rather than falling back
// to any local interpretation.
type DesignBridge interface {
	MutateDraft(ctx context.Context, start string, request draftmutation.Request, actor draftmutation.Actor) (draftmutation.Response, *draftmutation.Error)
	GetBoard(ctx context.Context, root, spec string) DesignReadOutcome
	GetDesignContext(ctx context.Context, root, spec string, childStories []string) DesignReadOutcome
	GetDesignCapabilities(ctx context.Context, root, spec string) (DesignReadOutcome, *DesignCapabilitiesView)
	GetDesignProvenance(ctx context.Context, root, spec string) DesignReadOutcome
	PrepareDesignReview(ctx context.Context, root, spec string) DesignReadOutcome
}

// designOperations is SI-167's closed application-operation half of the
// action inventory: POST <page>/api/<exact-application-operation>.
var designOperations = []string{
	"get_board",
	"get_design_capabilities",
	"get_design_context",
	"get_design_provenance",
	"mutate_draft",
	"prepare_design_review",
}

// legacyBoardActions is the pre-existing non-domain half of the action
// inventory: annotation writes (boardio — the same owner MCP's
// add_annotation uses), presentation layout (boardlayout), git affordances
// (gitx), scaffolding (stubinstantiate/designscaffold), and the
// obligation-artifact arm of sticky-graduate (internal/evidence). Every
// legacy DOMAIN spec-byte action (edit-text, edge, edge-delete,
// edge-retype, stub-graduate, relates-graduate, ref-trash, object-trash,
// and sticky-graduate's spec-object kinds) is DELETED: those gestures are
// now client-built mutate_draft transactions (AC-2: no parallel
// interpretation of domain mutations remains).
var legacyBoardActions = []string{
	"annotation-delete",
	"create",
	"git-commit",
	"git-switch",
	"pin",
	"position",
	"relates",
	"sticky",
	"sticky-graduate",
	"sticky-position",
	"stub-instantiate",
}

// boardActionInventory is the exact closed union the fixed-set inventory
// test pins; the API handler refuses any action outside it before any
// other work.
func boardActionInventory() map[string]bool {
	inventory := make(map[string]bool, len(designOperations)+len(legacyBoardActions))
	for _, op := range designOperations {
		inventory[op] = true
	}
	for _, action := range legacyBoardActions {
		inventory[action] = true
	}
	return inventory
}

// mintBrowserActor is the repository's single production
// NewUnauthenticatedHuman call site: the explicit browser-human actor for
// a workbench draft mutation (SI-163/SI-176). It takes no request-derived
// input whatsoever.
func mintBrowserActor() (draftmutation.Actor, error) {
	return draftmutation.NewUnauthenticatedHuman()
}

// resolveExpectedIdentity resolves the exact canonical identity the
// mutation kernel would construct for this checkout+spec, so the page can
// hand the browser the precise `expected` values a stale action is
// verified against (adjudication 3: stale actions are refused by the
// application's existing expected-identity precondition).
func resolveExpectedIdentity(ctx context.Context, root, name string) (checkout, branch, head string, err error) {
	identity, err := draftmutation.ResolveCanonicalIdentity(ctx, root, "spec/"+name, draftmutation.GitIdentityReader{})
	if err != nil {
		return "", "", "", err
	}
	return identity.Checkout, identity.Branch, identity.Head, nil
}

// cachedCanonicalCheckout resolves — successfully at most once per server
// instance — the exact canonical CHECKOUT string the mutation kernel
// derives for this root (path canonicalization only; it never changes for
// a live server). The per-render branch/HEAD facts stay fresh; only the
// stable path resolution is cached, so every poll stops re-resolving the
// worktree toplevel through git. A failure (typically the CALLER'S
// request context aborting mid-resolution) is returned to that caller
// alone and never memoized: the next call retries with its own context
// (review fix I-2 — the sync.Once variant cached the first request's
// "context canceled" for the server's lifetime, refusing every later
// mutation as identity-invalid).
func (s *boardSpecServer) cachedCanonicalCheckout(ctx context.Context, name string) (string, error) {
	s.checkoutMu.Lock()
	defer s.checkoutMu.Unlock()
	if s.checkoutCanonical != "" {
		return s.checkoutCanonical, nil
	}
	checkout, _, _, err := resolveExpectedIdentity(ctx, s.root, name)
	if err != nil {
		return "", err
	}
	s.checkoutCanonical = checkout
	return checkout, nil
}

// designActionRequest is the strict transport envelope for POST
// api/mutate_draft. Request carries the exact verdi.draftmutation/v1
// request bytes (decoded by draftmutation.DecodeRequest — the same one
// decoder the CLI and MCP use); GraduateAnnotations and DeleteAnnotations
// are the gesture's explicit annotation-owner routing (boardio), applied
// only after a clean transaction, exactly as the legacy graduation rituals
// ordered their two halves. The actor is NEVER part of this envelope.
type designActionRequest struct {
	Request             json.RawMessage `json:"request"`
	GraduateAnnotations []string        `json:"graduate_annotations,omitempty"`
	DeleteAnnotations   []string        `json:"delete_annotations,omitempty"`
}

// designReadRequest is the strict body for the five read operations: an
// empty object for four of them; get_design_context may name child
// stories.
type designReadRequest struct {
	ChildStories []string `json:"child_stories,omitempty"`
}

// readActionBody reads and bounds one design-action body. A body larger
// than the ceiling fails BEFORE any decode allocates a copy of it.
func readActionBody(r *http.Request) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxDesignActionBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading request body: %w", err)
	}
	if len(raw) > maxDesignActionBytes {
		return nil, fmt.Errorf("request body exceeds %d bytes", maxDesignActionBytes)
	}
	return raw, nil
}

// decodeStrictActionBody enforces the full pre-application strictness
// contract (design §3.2): the body must be one JSON object with no
// unknown fields, no duplicate keys, no null values anywhere, and no
// trailing data. An empty body is permitted and reads as the empty
// object.
func decodeStrictActionBody(raw []byte, out interface{}) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = []byte("{}")
	}
	if err := scanStrictJSONValue(json.NewDecoder(bytes.NewReader(raw))); err != nil {
		return err
	}
	return artifact.DecodeStrictJSON(raw, out)
}

// scanStrictJSONValue token-walks one JSON value rejecting duplicate
// object keys and null values at every depth. artifact.DecodeStrictJSON
// (unknown fields + trailing data) runs after it; together they implement
// the closed grammar's "unknown fields, nulls, duplicate keys, oversized
// bodies, and trailing data fail" clause.
func scanStrictJSONValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("malformed request: %w", err)
	}
	return scanStrictJSONToken(dec, tok)
}

func scanStrictJSONToken(dec *json.Decoder, tok json.Token) error {
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return fmt.Errorf("malformed request: %w", err)
				}
				key, ok := keyTok.(string)
				if !ok {
					return fmt.Errorf("malformed request: non-string object key")
				}
				if seen[key] {
					return fmt.Errorf("duplicate key %q", key)
				}
				seen[key] = true
				if err := scanStrictJSONValue(dec); err != nil {
					return err
				}
			}
			_, err := dec.Token() // consume '}'
			return err
		case '[':
			for dec.More() {
				if err := scanStrictJSONValue(dec); err != nil {
					return err
				}
			}
			_, err := dec.Token() // consume ']'
			return err
		}
		return nil
	case nil:
		return fmt.Errorf("null values are not permitted")
	default:
		return nil
	}
}

// designActionHandler serves one of the six application operations, or
// reports false for every other action name. Reads never take writeMu
// (renders and reads see whole files through atomic replacement); the
// mutation takes it exactly as every legacy write action does.
func (s *boardSpecServer) designActionHandler(w http.ResponseWriter, r *http.Request, name, action string) bool {
	switch action {
	case "mutate_draft":
		s.handleMutateDraft(w, r, name)
	case "get_board":
		s.handleDesignRead(w, r, name, func(ctx context.Context) DesignReadOutcome {
			return s.design.GetBoard(ctx, s.root, "spec/"+name)
		})
	case "get_design_capabilities":
		s.handleDesignRead(w, r, name, func(ctx context.Context) DesignReadOutcome {
			outcome, _ := s.design.GetDesignCapabilities(ctx, s.root, "spec/"+name)
			return outcome
		})
	case "get_design_provenance":
		s.handleDesignRead(w, r, name, func(ctx context.Context) DesignReadOutcome {
			return s.design.GetDesignProvenance(ctx, s.root, "spec/"+name)
		})
	case "prepare_design_review":
		s.handleDesignRead(w, r, name, func(ctx context.Context) DesignReadOutcome {
			return s.design.PrepareDesignReview(ctx, s.root, "spec/"+name)
		})
	case "get_design_context":
		raw, err := readActionBody(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return true
		}
		var req designReadRequest
		if err := decodeStrictActionBody(raw, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "malformed request: "+err.Error())
			return true
		}
		s.serveDesignRead(w, r, func(ctx context.Context) DesignReadOutcome {
			return s.design.GetDesignContext(ctx, s.root, "spec/"+name, req.ChildStories)
		})
	default:
		return false
	}
	return true
}

// handleDesignRead decodes the strict empty-object body then relays one
// canonical read envelope.
func (s *boardSpecServer) handleDesignRead(w http.ResponseWriter, r *http.Request, _ string, call func(context.Context) DesignReadOutcome) {
	raw, err := readActionBody(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req struct{}
	if err := decodeStrictActionBody(raw, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	s.serveDesignRead(w, r, call)
}

// serveDesignRead renders one read outcome: clean → the canonical
// envelope verbatim with HTTP 200; verdict → 200 with the typed failure
// envelope (the response stays usable, §4.3); operational → 500 with the
// same envelope, never a favorable body. The classification lives IN the
// body; the status is transport posture only.
func (s *boardSpecServer) serveDesignRead(w http.ResponseWriter, r *http.Request, call func(context.Context) DesignReadOutcome) {
	if s.design == nil {
		writeDesignFailure(w, &DesignFailure{Classification: "operational", Code: "design-service-unwired", Detail: "the design application service is not wired on this server"})
		return
	}
	outcome := call(r.Context())
	if outcome.Failure != nil {
		writeDesignFailure(w, outcome.Failure)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(outcome.JSON) // response body write; post-header error is unactionable
}

// writeDesignFailure renders one verdi.design-failure/v1 envelope.
func writeDesignFailure(w http.ResponseWriter, failure *DesignFailure) {
	status := http.StatusOK
	if failure.Classification != "verdict" {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]string{
		"schema":         designFailureSchema,
		"classification": failure.Classification,
		"code":           failure.Code,
		"detail":         failure.Detail,
	})
}

// mutationProjection is the fresh projection every non-operational
// mutation response carries (§4.3: success renders only after the
// operation AND a fresh projection both verify; a stale refusal returns a
// fresh projection so the browser can reload honestly).
type mutationProjection struct {
	HTML        string `json:"html"`
	Revision    string `json:"revision"`
	BaseDigest  string `json:"base_digest"`
	BaseSpecB64 string `json:"base_spec_b64"`
	Dirty       bool   `json:"dirty"`
}

// handleMutateDraft is the browser mutation adapter: ONE typed
// draftmutation transaction through the designapp bridge, with the
// explicit unauthenticated-human actor, followed (on clean) by the
// gesture's declared annotation-owner routing, presentation-layout
// pruning, and one fresh projection.
func (s *boardSpecServer) handleMutateDraft(w http.ResponseWriter, r *http.Request, name string) {
	if s.design == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"failure": map[string]string{"schema": designFailureSchema, "classification": "operational", "code": "design-service-unwired", "detail": "the design application service is not wired on this server"},
		})
		return
	}
	raw, err := readActionBody(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	var envelope designActionRequest
	if err := decodeStrictActionBody(raw, &envelope); err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	if len(envelope.Request) == 0 {
		writeJSONError(w, http.StatusBadRequest, "malformed request: missing field \"request\"")
		return
	}
	// Mirror the MCP adapter's transport posture: strictly decode the
	// caller's request fields, re-marshal canonically, and hand the ONE
	// kernel decoder (draftmutation.DecodeRequest) the canonical bytes it
	// requires — the browser is a transport, not a canonical-JSON printer.
	var wire draftmutation.Request
	if err := artifact.DecodeStrictJSON([]byte(envelope.Request), &wire); err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	canonical, err := canonjson.Marshal(wire)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed request: encoding request: "+err.Error())
		return
	}
	request, err := draftmutation.DecodeRequest(canonical)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	if request.Spec != "spec/"+name {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("request spec %q does not name this board's spec %q", request.Spec, "spec/"+name))
		return
	}

	// One writer at a time within this server, exactly like every legacy
	// write action (M-2); the kernel's own per-checkout transaction mutex
	// (SI-177) serializes against MCP writers in the same process.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	proj, _, _, extras, err := s.loadBoard(r.Context(), name)
	if errors.Is(err, ErrBoardNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// CO-1's review/read-only refusal is adapter knowledge (an open MR's
	// mirror is not the kernel's to see): only the live authoring wall
	// accepts writes, exactly as before the migration.
	if proj.Mode != modeAuthoring {
		writeJSONError(w, http.StatusForbidden, fmt.Sprintf("board for %s is in %s mode; only an authoring board (%s spec on a design branch) accepts writes", name, proj.Mode, s.model.DisplayState(proj.Class, "draft")))
		return
	}
	// The kernel's design/<spec-name> branch precondition, refused HERE
	// with the same explanation the wall shows (review fix I-1): the
	// kernel remains the backstop (AuthorizeState would answer
	// state-forbidden), but the adapter never invites a doomed write.
	if proj.DomainRefusal != "" {
		writeJSONError(w, http.StatusForbidden, proj.DomainRefusal)
		return
	}
	// The gesture's annotation routing is validated against the live
	// projection BEFORE the transaction, so an unknown record refuses with
	// ZERO mutation rather than surfacing as a post-transaction surprise.
	if err := validateAnnotationRouting(proj, envelope.GraduateAnnotations, envelope.DeleteAnnotations); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Removal coverage (review fix I-3, the legacy object-trash
	// enumeration): a transaction removing a declared object must also
	// remove every OTHER declared link naming its fragment — enumerated
	// from the decoded frontmatter itself, never from what happened to be
	// rendered — or be refused with ZERO mutation, disclosing exactly
	// which links hold the object (VL-003 stays green through the typed
	// path).
	if err := validateRemovalCoverage(name, extras.fm, request.Operations); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	actor, err := mintBrowserActor()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "minting browser actor: "+err.Error())
		return
	}
	response, diagnostic := s.design.MutateDraft(r.Context(), s.root, request, actor)
	if diagnostic != nil {
		if diagnostic.Code == draftmutation.CodeStaleBase && response.Stale != nil {
			// The stale refusal keeps draftmutation's own typed projection
			// verbatim (the CO-9 conformance object), and rides beside one
			// fresh projection (adjudication 3).
			staleJSON, encErr := draftmutation.EncodeStaleRefusal(*response.Stale)
			if encErr != nil {
				writeJSONError(w, http.StatusInternalServerError, "encoding stale refusal: "+encErr.Error())
				return
			}
			s.writeMutationOutcome(w, r, name, map[string]json.RawMessage{"stale": staleJSON})
			return
		}
		classification := "operational"
		status := http.StatusInternalServerError
		if diagnostic.Verdict() {
			classification = "verdict"
			status = http.StatusOK
		}
		failureJSON, encErr := json.Marshal(map[string]string{
			"schema":         designFailureSchema,
			"classification": classification,
			"code":           string(diagnostic.Code),
			"detail":         diagnostic.Detail,
		})
		if encErr != nil {
			writeJSONError(w, http.StatusInternalServerError, encErr.Error())
			return
		}
		if classification == "verdict" {
			// A verdict left the store untouched: a fresh projection is
			// safe and gives the browser its corrective destination.
			s.writeMutationOutcome(w, r, name, map[string]json.RawMessage{"failure": failureJSON})
			return
		}
		// Operational: render the stable failure alone — never a favorable
		// or partial state (§4.3).
		writeJSON(w, status, map[string]json.RawMessage{"failure": failureJSON})
		return
	}
	if response.Result == nil {
		// vocab:identity — ASD protocol/transaction name in a machinery diagnostic
		writeJSONError(w, http.StatusInternalServerError, "draft mutation service returned an invalid response union")
		return
	}

	// Clean. The annotation-owner and presentation-owner follow-ups run in
	// the legacy gestures' exact order — spec transaction first, mutable
	// zone second — and any follow-up failure is DISCLOSED beside the
	// landed result, never hidden (§4.3: no partial action effect may be
	// hidden).
	var postErr []string
	if len(envelope.GraduateAnnotations) > 0 {
		if _, gerr := boardio.GraduateStickies(boardio.AnnotationsDir(s.root), envelope.GraduateAnnotations); gerr != nil {
			postErr = append(postErr, "graduating annotations: "+gerr.Error())
		}
	}
	if len(envelope.DeleteAnnotations) > 0 {
		if _, derr := boardio.DeleteAnnotations(boardio.AnnotationsDir(s.root), envelope.DeleteAnnotations); derr != nil {
			postErr = append(postErr, "deleting annotations: "+derr.Error())
		}
	}
	for _, op := range request.Operations {
		switch op.Op {
		case draftmutation.OpAddLink:
			// Drawing a typed edge to a pinned target IS the pin's
			// graduation (02 §Record schemas) — unchanged annotation-owner
			// semantics, applied to the endpoint form the wall renders.
			endpoint := edgeEndpoint(name, declaredBoolSet(proj), op.Ref)
			if gerr := s.graduatePinsFor(proj, endpoint); gerr != nil {
				postErr = append(postErr, "graduating pins: "+gerr.Error())
			}
		case draftmutation.OpRemoveAC, draftmutation.OpRemoveConstraint, draftmutation.OpRemoveDecision, draftmutation.OpRemoveQuestion:
			if perr := s.pruneLayoutKey(name, proj, op.ID); perr != nil {
				postErr = append(postErr, "pruning layout key: "+perr.Error())
			}
		case draftmutation.OpRemoveStub:
			if perr := s.pruneLayoutKey(name, proj, "stub:"+op.Slug); perr != nil {
				postErr = append(postErr, "pruning layout key: "+perr.Error())
			}
		}
	}

	resultJSON, encErr := draftmutation.EncodeResult(*response.Result)
	if encErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "encoding mutation result: "+encErr.Error())
		return
	}
	fields := map[string]json.RawMessage{"result": resultJSON}
	if len(postErr) > 0 {
		disclosed, derr := json.Marshal(strings.Join(postErr, "; "))
		if derr == nil {
			fields["post_transaction_error"] = disclosed
		}
	}
	s.writeMutationOutcome(w, r, name, fields)
}

// declaredBoolSet mirrors buildProjection's declared-object set for
// edgeEndpoint.
func declaredBoolSet(proj *BoardProjection) map[string]bool {
	declared := make(map[string]bool, len(proj.Cards))
	for _, c := range proj.Cards {
		declared[c.ID] = true
	}
	return declared
}

// validateAnnotationRouting refuses any graduate/delete id that is not a
// live annotation record on THIS board's projection (sticky, thread, or
// pin) — the same board-scoped custody the legacy graduation and delete
// actions enforced.
func validateAnnotationRouting(proj *BoardProjection, graduate, remove []string) error {
	if len(graduate) == 0 && len(remove) == 0 {
		return nil
	}
	live := map[string]bool{}
	for _, st := range proj.Stickies {
		live[st.ID] = true
	}
	for _, e := range proj.Edges {
		if e.AnnotationID != "" {
			live[e.AnnotationID] = true
		}
	}
	for _, rc := range proj.RefCards {
		if rc.Pinned {
			live[rc.PinID] = true
		}
	}
	for _, id := range graduate {
		if !live[id] {
			return fmt.Errorf("graduate_annotations names %q, which is not a live annotation on this board — it may have been deleted or graduated since this wall was last refreshed", id)
		}
	}
	for _, id := range remove {
		if !live[id] {
			return fmt.Errorf("delete_annotations names %q, which is not a live annotation on this board — it may have been deleted or graduated since this wall was last refreshed", id)
		}
	}
	return nil
}

// refNamesFragment reports whether a stored link ref names spec/<name>'s
// declared fragment id — the legacy edgeRefMatcher semantics: the bare
// id, the canonical spec/<name>#id form, and any parseable ref (pinned
// forms included) normalizing to either.
func refNamesFragment(name, id, ref string) bool {
	internal := "spec/" + name + "#" + id
	if ref == id || ref == internal {
		return true
	}
	r, err := artifact.ParseRef(ref)
	if err != nil {
		return false
	}
	normalized := string(r.Kind) + "/" + r.Name
	if r.Object != "" {
		normalized += "#" + r.Object
	}
	return normalized == id || normalized == internal
}

// validateRemovalCoverage enforces the legacy object-trash enumeration on
// the typed path (review fix I-3): for every remove-ac/-constraint/
// -decision/-question operation in the transaction, ALL declared links
// naming the removed fragment — every link type, from the decoded
// frontmatter, never from the rendered chips — must either belong to a
// decision removed in the same transaction or be covered by a matching
// remove-link operation (exact stored source/type/ref/note). A fragment
// named by the document-level links: block refuses outright: the board
// cannot edit that block. The refusal is a zero-mutation 400 disclosing
// every uncovered link, so no transaction can land a dangling ref
// (VL-003) through this adapter.
func validateRemovalCoverage(name string, fm *artifact.SpecFrontmatter, ops []draftmutation.Operation) error {
	var removedIDs []string
	removed := map[string]bool{}
	removedDecisions := map[string]bool{}
	coverage := map[string]int{}
	for _, op := range ops {
		switch op.Op {
		case draftmutation.OpRemoveAC, draftmutation.OpRemoveConstraint, draftmutation.OpRemoveDecision, draftmutation.OpRemoveQuestion:
			if !removed[op.ID] {
				removed[op.ID] = true
				removedIDs = append(removedIDs, op.ID)
			}
			if op.Op == draftmutation.OpRemoveDecision {
				removedDecisions[op.ID] = true
			}
		case draftmutation.OpRemoveLink:
			coverage[op.Source+"\x00"+string(op.Type)+"\x00"+op.Ref+"\x00"+op.Note]++
		}
	}
	if len(removedIDs) == 0 {
		return nil
	}
	var uncovered []string
	for _, id := range removedIDs {
		for _, l := range fm.Links {
			if refNamesFragment(name, id, l.Ref) {
				return fmt.Errorf("%s is named by the spec document's own links: block (%s %s), which the board cannot edit — the object stays", id, l.Type, l.Ref)
			}
		}
		for _, dc := range fm.Decisions {
			if dc.ID == id || removedDecisions[dc.ID] {
				continue // its own entry (or a same-batch removal) goes whole, links included
			}
			for _, l := range dc.Links {
				if !refNamesFragment(name, id, l.Ref) {
					continue
				}
				key := dc.ID + "\x00" + string(l.Type) + "\x00" + l.Ref + "\x00" + l.Note
				if coverage[key] > 0 {
					coverage[key]--
					continue
				}
				uncovered = append(uncovered, fmt.Sprintf("decision %s's %s %s", dc.ID, l.Type, l.Ref))
			}
		}
	}
	if len(uncovered) > 0 {
		return fmt.Errorf("removing a declared object would leave dangling link(s) in the spec document: %s — remove them in the same transaction (a remove-link operation per stored source/type/ref/note tuple)", strings.Join(uncovered, "; "))
	}
	return nil
}

// pruneLayoutKey drops one removed object's stored layout position
// (VL-018: a dangling layout key is a lint error the writer never
// persists) — the presentation owner's follow-up to a clean removal.
func (s *boardSpecServer) pruneLayoutKey(name string, proj *BoardProjection, key string) error {
	stored, err := boardlayout.ReadFile(s.specDir(name))
	if err != nil {
		return err
	}
	if _, had := stored[key]; !had {
		return nil
	}
	live := liveKeys(proj)
	delete(live, key)
	return boardlayout.WriteFile(s.specDir(name), stored, live)
}

// mutationSnapshotTestHook, when non-nil, injects a post-transaction
// state-resolution failure at the exact point writeMutationOutcome renders
// the fresh projection — the spliceSpecTestPause-shaped seam package tests
// use to witness the disclosure contract for a projection that fails AFTER
// a durable mutation landed. Production never stores into it.
var mutationSnapshotTestHook atomic.Pointer[func(name string) error]

// writeMutationOutcome renders one mutation response: the caller's typed
// fields plus one fresh projection (region HTML, revision token, base
// digest/bytes, dirtiness) — the fresh-projection half of §4.3's clean
// contract and adjudication 3's stale contract.
func (s *boardSpecServer) writeMutationOutcome(w http.ResponseWriter, r *http.Request, name string, fields map[string]json.RawMessage) {
	var snap *asdSnapshot
	var err error
	if hook := mutationSnapshotTestHook.Load(); hook != nil {
		err = (*hook)(name)
	}
	if err == nil {
		snap, err = s.loadSnapshot(r.Context(), name)
	}
	if err != nil {
		writeUnrefreshedOutcome(w, fields, fmt.Errorf("rendering fresh projection: %w", err))
		return
	}
	projection := mutationProjection{
		HTML:        snap.HTML,
		Revision:    snap.Revision,
		BaseDigest:  snap.BaseDigest,
		BaseSpecB64: snap.BaseSpecB64,
		Dirty:       snap.Git.Dirty,
	}
	projJSON, err := json.Marshal(projection)
	if err != nil {
		writeUnrefreshedOutcome(w, fields, fmt.Errorf("encoding fresh projection: %w", err))
		return
	}
	out := make(map[string]json.RawMessage, len(fields)+1)
	for k, v := range fields {
		out[k] = v
	}
	out["projection"] = projJSON
	writeJSON(w, http.StatusOK, out)
}

// writeUnrefreshedOutcome renders a mutation outcome whose fresh
// projection could NOT be derived (Codex correction round 1, finding 2).
// It runs AFTER the kernel transaction and every post-transaction effect
// committed, so discarding the caller's fields would present a durable
// mutation as unapplied — a direct §4.3 "no partial action effect may be
// hidden" violation. Every typed field the caller assembled (the landed
// result, any post_transaction_error disclosure, a stale refusal or
// verdict failure) is preserved verbatim, and the projection failure
// itself rides beside them as one typed operational envelope. The HTTP
// 500 is transport posture only (§4.3): the body never claims a clean,
// refreshed state — and never hides the landed facts.
func writeUnrefreshedOutcome(w http.ResponseWriter, fields map[string]json.RawMessage, err error) {
	failureJSON, encErr := json.Marshal(map[string]string{
		"schema":         designFailureSchema,
		"classification": "operational",
		"code":           "projection-unavailable",
		"detail":         err.Error(),
	})
	if encErr != nil {
		// Marshaling a map of strings cannot fail; fail closed regardless.
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make(map[string]json.RawMessage, len(fields)+1)
	for k, v := range fields {
		out[k] = v
	}
	out["projection_failure"] = failureJSON
	writeJSON(w, http.StatusInternalServerError, out)
}

// digestSpecBytes exposes the kernel's one digest derivation for the
// page/snapshot base facts (never a second hash algorithm).
func digestSpecBytes(data []byte) string {
	return draftmutation.DigestBytes(data)
}
