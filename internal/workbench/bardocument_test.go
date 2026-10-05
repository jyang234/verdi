package workbench

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specdoc"
)

// TestDocumentBarFacts_EqualTheWalls (SI-323 (2)): the Document page's bar
// states the same facts the wall states for the same spec and branch,
// built from the document load's own resolutions.
func TestDocumentBarFacts_EqualTheWalls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		server func(*testing.T) *boardSpecServer
		spec   string
	}{
		{name: "authoring wall on its design branch", spec: boardFixtureName, server: func(t *testing.T) *boardSpecServer {
			return &boardSpecServer{root: newBarFixture(t)}
		}},
		{name: "sealed record on the default branch, renamed vocabulary", spec: documentWallName, server: func(t *testing.T) *boardSpecServer {
			_, repo, _ := newAcceptedWallFixture(t)
			return &boardSpecServer{root: repo.Dir, model: vocabTestModel()}
		}},
		{name: "a design branch's own board instance (/b/)", spec: boardFixtureName, server: func(t *testing.T) *boardSpecServer {
			root := newBarFixture(t)
			gitOut(t, root, "checkout", "-q", "main")
			s, err := newBranchBoards(root, Deps{}, &boardSpecServer{root: root}).server(t.Context(), "design/"+boardFixtureName)
			if err != nil {
				t.Fatalf("branch server: %v", err)
			}
			return s
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.server(t)
			proj, _, asd, err := s.loadASD(t.Context(), tc.spec)
			if err != nil {
				t.Fatalf("loadASD: %v", err)
			}
			wall := specBarFacts(proj, asd)
			snap, res, err := s.loadDocumentPage(t.Context(), tc.spec, specdoc.KindSpec)
			if err != nil {
				t.Fatalf("loadDocumentPage: %v", err)
			}
			doc := s.documentBarFacts(t.Context(), tc.spec, res, snap.checkout)
			checkBarFacts(t, doc)
			if !reflect.DeepEqual(doc, wall) {
				t.Fatalf("Document page facts =\n%+v\n%+v\nwall's =\n%+v\n%+v", doc, doc.Spec, wall, wall.Spec)
			}
		})
	}
}

// TestDocumentBarFacts_BytesAgreeWithTheStamp: within one render, the
// bar's displayed-bytes word and the document's stamp read the same
// projection — proposed exactly when the stamp says proposed.
func TestDocumentBarFacts_BytesAgreeWithTheStamp(t *testing.T) {
	for _, tc := range []struct {
		name, spec, word string
		proposed         bool
		root             func(*testing.T) string
	}{
		{name: "a new spec on its design branch", spec: boardFixtureName, word: "proposed", proposed: true, root: newBarFixture},
		{name: "the accepted bytes on the default branch", spec: documentWallName, word: "accepted", proposed: false, root: func(t *testing.T) string {
			_, repo, _ := newAcceptedWallFixture(t)
			return repo.Dir
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &boardSpecServer{root: tc.root(t)}
			snap, res, err := s.loadDocumentPage(t.Context(), tc.spec, specdoc.KindSpec)
			if err != nil {
				t.Fatalf("loadDocumentPage: %v", err)
			}
			bar := s.documentBarFacts(t.Context(), tc.spec, res, snap.checkout)
			if bar.Spec == nil || bar.Spec.Bytes.Word != tc.word || snap.Proposed != tc.proposed {
				t.Fatalf("bar bytes %+v, stamp proposed %t; want %q and %t", bar.Spec, snap.Proposed, tc.word, tc.proposed)
			}
			if (bar.Spec.Bytes.Word == "proposed") != snap.Proposed {
				t.Fatal("the bar's displayed bytes disagree with the document stamp in one render")
			}
		})
	}
}

// TestDocumentBarFacts_StateTheLoadCouldNotResolve: when the document load
// could not make its effective-state projection, the bar's spec facts are
// disclosed-unproven with the load's own reason — never resolved again —
// and the posture still states the checkout.
func TestDocumentBarFacts_StateTheLoadCouldNotResolve(t *testing.T) {
	root := newBarFixture(t)
	s := &boardSpecServer{root: root}
	snap, res, err := s.loadDocumentPage(t.Context(), boardFixtureName, specdoc.KindSpec)
	if err != nil {
		t.Fatalf("loadDocumentPage: %v", err)
	}
	res.State = nil
	res.Disclosures = []string{"status not resolved: the projector failed"}
	got := s.documentBarFacts(t.Context(), boardFixtureName, res, snap.checkout)
	checkBarFacts(t, got)
	if got.Spec == nil || got.Spec.Name != boardFixtureName || !strings.Contains(got.Spec.Unproven, "the projector failed") {
		t.Fatalf("spec facts = %+v, want disclosed-unproven with the load's reason", got.Spec)
	}
	if got.Posture.BaseDigest == nil || got.Posture.BaseDigest.Text != digestSpecBytes(res.Content) {
		t.Fatalf("base digest = %v, want the rendered bytes' digest", got.Posture.BaseDigest)
	}
	if got.Posture.Branch != provenFact("design/"+boardFixtureName) || got.Posture.AcceptedHead.Text != gitOut(t, root, "rev-parse", "main") {
		t.Fatalf("posture = %+v, want the checkout stated", got.Posture)
	}
}

// gitCounts counts, through gitx's observer, the two Git reads a page's
// accepted-HEAD budget is about (Wave 6 §5.3): rev-parses of the accepted
// ref (with or without ^{commit}), and specstate runs (each begins with a
// BlobAt, `git ls-tree <ref> -- <spec path>`).
type gitCounts struct {
	mu                     sync.Mutex
	acceptedRef, specPath  string
	acceptedParses, states int
}

func (g *gitCounts) Observe(_ string, args []string) {
	if len(args) == 0 {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	last := strings.TrimSuffix(args[len(args)-1], "^{commit}")
	switch {
	case args[0] == "rev-parse" && last == g.acceptedRef:
		g.acceptedParses++
	case args[0] == "ls-tree" && last == g.specPath:
		g.states++
	}
}

func (g *gitCounts) counts() (int, int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.acceptedParses, g.states
}

// TestDocumentPage_BarAddsNoResolution (SI-323 (2), as refined at
// 23083dae; Wave 6 §5.3): the Document page's real Git reads — counted at
// the exec, not at a port — resolve the accepted ref and run specstate
// exactly as often as its document load alone does: the bar reuses the
// load's resolutions and adds none.
func TestDocumentPage_BarAddsNoResolution(t *testing.T) {
	for _, tc := range []struct {
		name, spec string
		root       func(*testing.T) string
	}{
		{name: "a new spec on its design branch", spec: boardFixtureName, root: newBarFixture},
		{name: "the accepted bytes on the default branch", spec: documentWallName, root: func(t *testing.T) string {
			_, repo, _ := newAcceptedWallFixture(t)
			return repo.Dir
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.root(t)
			s := &boardSpecServer{root: root}
			git, _, err := s.gitState(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			specPath := ".verdi/specs/active/" + tc.spec + "/spec.md"

			// One unobserved load first, so the process-wide caches the
			// load consults (the views and the specstate corpus) are equally
			// warm for both counted renders.
			if _, _, err := s.loadDocument(t.Context(), tc.spec, specdoc.KindSpec); err != nil {
				t.Fatalf("loadDocument: %v", err)
			}
			alone := &gitCounts{acceptedRef: git.acceptedRef(), specPath: specPath}
			if _, _, err := s.loadDocument(gitx.WithObserver(t.Context(), alone), tc.spec, specdoc.KindSpec); err != nil {
				t.Fatalf("loadDocument: %v", err)
			}

			page := &gitCounts{acceptedRef: git.acceptedRef(), specPath: specPath}
			mux := http.NewServeMux()
			for _, rt := range boardSpecRoutes() {
				mux.HandleFunc(rt.suffix, rt.handler(s))
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(gitx.WithObserver(t.Context(), page), http.MethodGet, "/board/spec/"+tc.spec+"/document", nil)
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET Document page = %d\n%s", rec.Code, rec.Body.String())
			}

			aloneParses, aloneStates := alone.counts()
			pageParses, pageStates := page.counts()
			t.Logf("accepted-ref rev-parses: load %d, page %d; specstate runs: load %d, page %d", aloneParses, pageParses, aloneStates, pageStates)
			if aloneParses == 0 || aloneStates == 0 {
				t.Fatalf("the load alone made %d accepted-ref rev-parses and %d specstate runs: the count would be vacuous", aloneParses, aloneStates)
			}
			if pageParses != aloneParses || pageStates != aloneStates {
				t.Fatalf("the Document page made %d accepted-ref rev-parses and %d specstate runs, the load alone %d and %d: the bar added a resolution", pageParses, pageStates, aloneParses, aloneStates)
			}
		})
	}
}

// newBarFixture is the board fixture as an adopted store (a verdi.yaml, so
// the Document page renders too): main carries the config and an ADR, and
// the spec is authored on its design branch, checked out.
func newBarFixture(t *testing.T) string {
	t.Helper()
	return buildAuthoringFixture(t, "design/"+boardFixtureName,
		map[string]string{
			".verdi/verdi.yaml":                "schema: verdi.config/v1\nforge: none\n",
			".verdi/adr/0001-outbox-events.md": boardFixtureADR,
			".verdi/.gitignore":                "data/\n",
		},
		map[string]string{
			".verdi/specs/active/" + boardFixtureName + "/spec.md":     boardFixtureSpec,
			".verdi/specs/active/" + boardFixtureName + "/layout.json": boardFixtureLayout,
		})
}

// TestBarFacts_ModeCarriesTheReviewFeedDisclosure (F1A-B3; SI-323 (2)):
// when the review feed is configured but cannot be consulted, the mode is
// stated with the feed's disclosure — on the wall and on its Document
// page alike — never as a proven review-free mode. A wall whose feed is
// unconfigured, or answered, carries none.
func TestBarFacts_ModeCarriesTheReviewFeedDisclosure(t *testing.T) {
	root := newBarFixture(t)
	for _, tc := range []struct {
		name string
		s    *boardSpecServer
		want string
	}{
		{name: "a feed that errors", s: &boardSpecServer{root: root, feed: erroringFeed{}}, want: "review"},
		{name: "a configured forge with no credentials", s: &boardSpecServer{root: root, reviewUnavailable: "forge configured but no credentials"}, want: "no credentials"},
		{name: "no feed configured", s: &boardSpecServer{root: root}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proj, _, asd, err := tc.s.loadASD(t.Context(), boardFixtureName)
			if err != nil {
				t.Fatalf("loadASD: %v", err)
			}
			wall := specBarFacts(proj, asd)
			snap, res, err := tc.s.loadDocumentPage(t.Context(), boardFixtureName, specdoc.KindSpec)
			if err != nil {
				t.Fatalf("loadDocumentPage: %v", err)
			}
			doc := tc.s.documentBarFacts(t.Context(), boardFixtureName, res, snap.checkout)
			for page, f := range map[string]barFacts{"wall": wall, "Document page": doc} {
				got := f.Spec.ModeDisclosure
				switch {
				case tc.want == "" && got != "":
					t.Errorf("%s: mode disclosure %q, want none", page, got)
				case tc.want != "" && !strings.Contains(strings.ToLower(got), tc.want):
					t.Errorf("%s: mode disclosure %q, want the feed's disclosure (%q)", page, got, tc.want)
				}
			}
			if wall.Spec.ModeDisclosure != doc.Spec.ModeDisclosure {
				t.Fatalf("wall %q and Document page %q disagree", wall.Spec.ModeDisclosure, doc.Spec.ModeDisclosure)
			}
		})
	}
}
