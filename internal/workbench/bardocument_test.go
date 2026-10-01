package workbench

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// TestDocumentBarFacts_EqualTheWalls (SI-323 (2)): the Document page's bar
// states the same facts the wall states for the same spec and branch —
// built through the wall's own path, never re-derived.
func TestDocumentBarFacts_EqualTheWalls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		server func(*testing.T) *boardSpecServer
		spec   string
	}{
		{name: "authoring wall on its design branch", spec: boardFixtureName, server: func(t *testing.T) *boardSpecServer {
			return &boardSpecServer{root: newBoardFixture(t)}
		}},
		{name: "sealed record on the default branch, renamed vocabulary", spec: boardFixtureName, server: func(t *testing.T) *boardSpecServer {
			return &boardSpecServer{root: newStatuslessBoardFixture(t, false), model: vocabTestModel()}
		}},
		{name: "a design branch's own board instance (/b/)", spec: "draft-a", server: func(t *testing.T) *boardSpecServer {
			root := newBranchBoardFixture(t)
			s, err := newBranchBoards(root, Deps{}, &boardSpecServer{root: root}).server(t.Context(), "design/draft-a")
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
			doc := s.documentBarFacts(t.Context(), tc.spec)
			checkBarFacts(t, doc)
			if !reflect.DeepEqual(doc, wall) {
				t.Fatalf("Document page facts =\n%+v\n%+v\nwall's =\n%+v\n%+v", doc, doc.Spec, wall, wall.Spec)
			}
		})
	}
}

// TestDocumentBarFacts_SpecTheWallCannotLoad: a Document page whose spec
// the wall cannot load (none in the active zone) still states the
// checkout's posture, and discloses its spec facts as unproven with the
// reason — never omits them.
func TestDocumentBarFacts_SpecTheWallCannotLoad(t *testing.T) {
	root := newBoardFixture(t)
	s := &boardSpecServer{root: root}
	got := s.documentBarFacts(t.Context(), "no-such-spec")
	checkBarFacts(t, got)
	if got.Spec == nil || got.Spec.Name != "no-such-spec" || !strings.Contains(got.Spec.Unproven, "no-such-spec") {
		t.Fatalf("spec facts = %+v, want disclosed-unproven naming the spec", got.Spec)
	}
	want := branchBarFacts(t.Context(), root, "no-such-spec").Posture
	if !reflect.DeepEqual(got.Posture, want) {
		t.Fatalf("posture = %+v, want the checkout's branch-level posture %+v", got.Posture, want)
	}
}

// countingPostureReader counts the posture model's Git reads by rev.
type countingPostureReader struct {
	mu   sync.Mutex
	revs []string
}

func (c *countingPostureReader) RevParse(ctx context.Context, dir, rev string) (string, error) {
	c.mu.Lock()
	c.revs = append(c.revs, rev)
	c.mu.Unlock()
	return gitPostureReader{}.RevParse(ctx, dir, rev)
}

func (c *countingPostureReader) AheadBehind(ctx context.Context, dir, left, right string) (int, int, error) {
	return gitPostureReader{}.AheadBehind(ctx, dir, left, right)
}

// acceptedResolutions counts the accepted-HEAD resolutions recorded: every
// RevParse of a rev other than the worktree's HEAD.
func (c *countingPostureReader) acceptedResolutions() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, rev := range c.revs {
		if rev != "HEAD" {
			n++
		}
	}
	return n
}

// TestBarFacts_OneAcceptedHeadResolutionPerPage (Wave 6 §5.3, SI-323 (2)):
// the Document page shares the wall's posture resolution rather than
// adding one — each page render, the wall, its snapshot, and the Document
// page, resolves the accepted HEAD through the posture model exactly once.
func TestBarFacts_OneAcceptedHeadResolutionPerPage(t *testing.T) {
	root := newBarFixture(t)
	for _, path := range []string{
		"/board/spec/" + boardFixtureName + "/document",
		"/board/spec/" + boardFixtureName,
		"/board/spec/" + boardFixtureName + "/snapshot",
	} {
		t.Run(path, func(t *testing.T) {
			counter := &countingPostureReader{}
			s := &boardSpecServer{root: root, posture: counter}
			mux := http.NewServeMux()
			for _, rt := range boardSpecRoutes() {
				mux.HandleFunc(rt.suffix, rt.handler(s))
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d\n%s", path, rec.Code, rec.Body.String())
			}
			if got := counter.acceptedResolutions(); got != 1 {
				t.Fatalf("accepted-HEAD resolutions = %d (revs %v), want exactly 1", got, counter.revs)
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
