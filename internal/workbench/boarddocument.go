package workbench

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/readinesspilot"
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
// the page's chrome facts (spec/document-page-v2, SI-340 (8)), and a
// revision token over every fact the page renders (documentRevision).
type documentSnapshot struct {
	Revision    string            `json:"revision"`
	HTML        string            `json:"html"`
	Markdown    string            `json:"markdown"`
	Kind        string            `json:"kind"`
	Ref         string            `json:"ref"`
	Proposed    bool              `json:"proposed"`
	Disclosures []string          `json:"disclosures"`
	Facts       documentPageFacts `json:"facts"`

	// checkout is what the load read of the checkout's Git state, which
	// the page's top bar reuses (documentBarFacts); never part of the wire
	// projection.
	checkout documentCheckout
}

// documentCheckout is the serving checkout as one Document load read it.
// Every load reads the checked-out branch, which the identity card states
// and the revision token covers. Only the page's load reads the full Git
// state (git), through the wall's own gitState path, for its top bar: the
// bar and the identity card then state one branch, and the bar reads no
// Git state of its own (SI-323 (2)). A poll and a download read the branch
// alone, one exec, and never a status scan, which can rewrite the served
// checkout's index (SI-343 (3); Wave 6 §5.3). err, when set, says why the
// read failed; read is false only for a value no load produced.
type documentCheckout struct {
	read   bool
	branch string
	git    *boardGitState
	err    error
}

// The reasons a documentCheckout carries no Git state.
var (
	errCheckoutNotRead = errors.New("no load read it")
	errBranchOnly      = errors.New("the load read the branch alone")
)

// readBranch reads the checked-out branch alone ("" on a detached HEAD):
// the snapshot's and the download's read.
func (s *boardSpecServer) readBranch(ctx context.Context) documentCheckout {
	branch, err := gitx.CurrentBranch(ctx, s.root)
	if err != nil {
		return documentCheckout{err: err}
	}
	return documentCheckout{read: true, branch: branch}
}

// readCheckout reads the full Git state, whose branch the identity card
// states: the page's read, which its top bar reuses.
func (s *boardSpecServer) readCheckout(ctx context.Context) documentCheckout {
	git, _, err := s.gitState(ctx)
	if err != nil {
		return documentCheckout{err: err}
	}
	return documentCheckout{read: true, branch: git.Branch, git: git}
}

// state is the full Git state the load read, or why there is none.
func (c documentCheckout) state() (*boardGitState, error) {
	switch {
	case c.err != nil:
		return nil, c.err
	case !c.read:
		return nil, errCheckoutNotRead
	case c.git == nil:
		return nil, errBranchOnly
	}
	return c.git, nil
}

// branchFact is the checked-out branch as the bar states it (barPosture's
// Branch and Detached): proven, an empty text that is a detached HEAD, or
// disclosed-unproven when it could not be read.
func (c documentCheckout) branchFact() (barFact, bool) {
	switch {
	case c.err != nil:
		return unprovenFact(checkoutUnreadable(c.err)), false
	case !c.read:
		return unprovenFact(checkoutUnreadable(errCheckoutNotRead)), false
	}
	return provenFact(c.branch), c.branch == ""
}

// checkoutUnreadable is the reason every fact the checkout's Git state
// would have proven is disclosed-unproven.
func checkoutUnreadable(err error) string {
	return "the checkout's Git state could not be read: " + err.Error()
}

// documentRevision is the snapshot's revision token: a digest over every
// fact the page renders (Wave 6 §5.1) — the ref, the kind, the Markdown
// (which carries the body's own stamp: commit, proposed posture, engine),
// and the chrome facts, whose owners, branch, and files the Markdown does
// not carry. So any change a reader could see moves the token, and a poll
// that swaps the body brings the chrome of the same revision.
func documentRevision(snap documentSnapshot) (string, error) {
	return canonjson.Digest(struct {
		Ref, Kind, Markdown string
		Facts               documentPageFacts
	}{snap.Ref, snap.Kind, snap.Markdown, snap.Facts})
}

// loadDocument renders the working tree of the checkout this server
// serves (root mount: the serving checkout; branch mount: that branch's
// worktree), stamped with its HEAD and marked proposed unless the bytes
// are the exact accepted bytes on the default branch — the loader's own
// ModeWorkingTree rule, never re-derived here. When a ReadinessLoader is
// wired, readiness is derived fresh for THIS document's own ref
// (spec/readiness-recovery ac-4, R-RR1-8: every consumer asks for its own
// ref) — never a shared or foreign snapshot. A derivation error never
// fails the render: it becomes a "readiness: <err>" disclosure and the
// section states its own absence, exactly like any other degraded fact a
// document is not a verdict over (R-RR1-9). The load's Result comes back
// too: the page's top bar reuses its resolutions (documentBarFacts). The
// load also reads the checkout's branch, for the identity card, and keeps
// the document it built, from which the chrome facts are drawn (SI-340
// (1)). loadDocument is the poll's and the download's load: it reads the
// branch alone (readBranch). The page loads through loadDocumentPage.
func (s *boardSpecServer) loadDocument(ctx context.Context, name string, kind specdoc.Kind) (documentSnapshot, specdocload.Result, error) {
	return s.loadDocumentReading(ctx, name, kind, s.readBranch)
}

// loadDocumentPage is the page's load: loadDocument, reading the full Git
// state (readCheckout) for the identity card's branch and the top bar
// alike. Its revision equals a poll's for the same state, since both read
// the branch through gitx.CurrentBranch; only when the rest of the state
// cannot be read does the page disclose the branch unproven where a poll
// proves it, and the first poll then brings the proven branch.
func (s *boardSpecServer) loadDocumentPage(ctx context.Context, name string, kind specdoc.Kind) (documentSnapshot, specdocload.Result, error) {
	return s.loadDocumentReading(ctx, name, kind, s.readCheckout)
}

// loadDocumentReading is the shared load, reading the checkout through
// read once the document has loaded. It is the Document page's one
// application projection per conditional refresh (Wave 6 §5.3; ledger
// SI-356; BL-158): one read session for the checkout, and one accepted-HEAD
// resolution (openProjection) that the readiness load, the
// document's own lifecycle state and closed-spec views, and the checkout's
// default branch all read at — the snapshot, its revision token and its
// readiness derived together. Both end with the call; nothing outlives it
// (spec/readiness-recovery-v2 co-2).
func (s *boardSpecServer) loadDocumentReading(ctx context.Context, name string, kind specdoc.Kind, read func(context.Context) documentCheckout) (documentSnapshot, specdocload.Result, error) {
	if !specNameRe.MatchString(name) {
		return documentSnapshot{}, specdocload.Result{}, fmt.Errorf("workbench: spec %q not found: %w", name, ErrBoardNotFound)
	}
	ctx, release := s.openProjection(ctx)
	defer release()

	var readiness *readinesspilot.Snapshot
	var readinessDisclosure string
	if s.readinessLoader != nil {
		snap, rerr := s.readinessLoader.Load(ctx, "spec/"+name)
		if rerr != nil {
			readinessDisclosure = "readiness: " + rerr.Error()
		} else {
			readiness = &snap
		}
	}

	// A per-branch (/b/<branch>) instance serves the branch's tree, but the
	// corpus route (/a/{kind}/{name}) serves the serving checkout's, so the
	// §6 lines' corpus links could 404 there: the document takes the board
	// cards' posture and links no corpus page on a per-branch board.
	res, err := specdocload.Load(ctx, specdocload.Request{Root: s.root, Name: name, Mode: specdocload.ModeWorkingTree, Kind: kind, Model: s.model, Readiness: readiness, CorpusUnservable: s.fixedBranch != ""})
	if err != nil {
		return documentSnapshot{}, specdocload.Result{}, err
	}
	doc, err := specdoc.Build(res.Input)
	if err != nil {
		return documentSnapshot{}, specdocload.Result{}, err
	}
	md := specdoc.RenderMarkdown(doc)
	html, err := specdoc.RenderHTML(doc)
	if err != nil {
		return documentSnapshot{}, specdocload.Result{}, err
	}
	disclosures := append([]string(nil), res.Disclosures...)
	if readinessDisclosure != "" {
		disclosures = append(disclosures, readinessDisclosure)
	}
	checkout := read(ctx)
	snap := documentSnapshot{
		HTML: html, Markdown: md, Kind: string(kind), Ref: doc.Stamp.Ref, Proposed: doc.Stamp.Proposed, Disclosures: disclosures,
		Facts:    newDocumentPageFacts(name, doc, html, res, checkout, s.model),
		checkout: checkout,
	}
	rev, err := documentRevision(snap)
	if err != nil {
		return documentSnapshot{}, specdocload.Result{}, err
	}
	snap.Revision = rev
	return snap, res, nil
}

// documentBarFacts is the Document page's top bar facts (SI-323 (2), as
// refined at 23083dae): the facts the wall shows for the same spec and
// branch, adding no resolution of its own. The displayed bytes and the
// mode come from the effective-state projection the document load made,
// the base digest from the bytes it rendered, and the worktree and
// accepted HEADs from the heads it resolved — so the page keeps its one
// accepted-HEAD resolution (Wave 6 §5.3) and the bar can never disagree
// with the document's stamp. The checkout's gitState (branch, working
// tree) is the one the page's load read (checkout, from loadDocumentPage),
// so the bar and the identity card state the same branch; only the review
// feed, which the load does not read, is read here, through the wall's own
// path. A fact that cannot
// be obtained is disclosed-unproven: a projection the load could not make
// leaves the spec's facts unproven with its reason.
func (s *boardSpecServer) documentBarFacts(ctx context.Context, name string, res specdocload.Result, checkout documentCheckout) barFacts {
	title := name
	if res.Input.Spec != nil {
		title = res.Input.Spec.Title
	}
	git, err := checkout.state()
	if err != nil {
		f := unprovenBarFacts(title, s.root, checkoutUnreadable(err))
		f.Spec = &barSpec{Name: name, Unproven: checkoutUnreadable(err)}
		return f
	}
	heads := documentHeads(git, res)
	if res.State == nil || res.Input.Spec == nil {
		bp := branchPostureFrom(ctx, s.root, git, heads, s.posture)
		f := barFacts{Title: title, Posture: bp.facts()}
		digest := provenFact(digestSpecBytes(res.Content))
		f.Posture.BaseDigest = &digest
		f.Spec = &barSpec{Name: name, Unproven: "the effective state could not be resolved: " + strings.Join(res.Disclosures, "; ")}
		return f
	}
	_, underReview, notice := s.consultReview(ctx, name)
	p := projectionHead(name, res.Input.Spec, effectiveMode(underReview, *res.State, git), string(res.State.ArtifactStatus()))
	p.applyModelVocabulary(s.model)
	asd := s.postureView(ctx, p, git, res.Content, *res.State, heads)
	asd.reviewNotice = notice
	return specBarFacts(p, asd)
}

// documentHeads states the heads the document load resolved as the
// posture's: its HEAD, and the default branch's head it resolved for its
// views' history (specdocload.Result.Accepted).
func documentHeads(git *boardGitState, res specdocload.Result) postureHeads {
	h := postureHeads{worktree: res.Head}
	if res.Head == "" {
		h.worktreeWhy = "the worktree HEAD could not be resolved"
	}
	switch {
	case git.DefaultBranch == "" || res.Accepted.Ref == "":
		h.acceptedWhy = defaultBranchUnresolved
	case res.Accepted.Commit == "":
		h.acceptedWhy = fmt.Sprintf("the accepted HEAD (%s) could not be resolved", res.Accepted.Ref)
	default:
		h.accepted = res.Accepted.Commit
	}
	return h
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
			renderError(r.Context(), w, s.root, http.StatusBadRequest, err)
			return
		}
		format, err := documentFormatFromQuery(r)
		if err != nil {
			renderError(r.Context(), w, s.root, http.StatusBadRequest, err)
			return
		}
		// The download reads the checkout's branch alone, as a poll does;
		// the page reads the full Git state its top bar states.
		load := s.loadDocumentPage
		if format == "md" {
			load = s.loadDocument
		}
		snap, res, err := load(r.Context(), name, kind)
		if err != nil {
			// The HTML route fails as the board does: renderError's page,
			// never a plain-text body (/snapshot keeps JSON errors).
			renderError(r.Context(), w, s.root, documentLoadStatus(name, err), err)
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
		bar := s.documentBarFacts(r.Context(), name, res, snap.checkout)
		page, err := renderBoardDocumentPage(r.Context(), r.URL.EscapedPath(), name, snap, bar)
		if err != nil {
			renderError(r.Context(), w, s.root, http.StatusInternalServerError, err)
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
		snap, _, err := s.loadDocument(r.Context(), name, kind)
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
