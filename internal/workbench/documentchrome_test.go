package workbench

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
	"github.com/jyang234/verdi/internal/store"
)

// The Document page's chrome refresh (spec/document-page-v2 ac-1; ledger
// SI-340 (8); Wave 6 §5.1): the snapshot carries the page facts, and its
// revision token covers every fact the page renders, so a poll that swaps
// the body brings the stamp, identity card, and rail of the same revision.

// cloneSnapshot deep-copies snap's facts, so a mutation never reaches the
// value it was cloned from.
func cloneSnapshot(t *testing.T, snap documentSnapshot) documentSnapshot {
	t.Helper()
	c := snap
	data, err := json.Marshal(snap.Facts)
	if err != nil {
		t.Fatal(err)
	}
	c.Facts = documentPageFacts{}
	if err := json.Unmarshal(data, &c.Facts); err != nil {
		t.Fatal(err)
	}
	return c
}

// TestDocumentRevision_CoversEveryPageFact: a change to any fact the page
// renders — the body's Markdown, its ref and kind, and every chrome fact,
// including the owners, branch, and files the Markdown does not carry —
// moves the token; the same facts give the same token.
func TestDocumentRevision_CoversEveryPageFact(t *testing.T) {
	_, repo, name := newAcceptedWallFixture(t)
	base, _, err := (&boardSpecServer{root: repo.Dir}).loadDocument(t.Context(), name, specdoc.KindSpec)
	if err != nil {
		t.Fatalf("loadDocument: %v", err)
	}
	counted := -1
	for i, e := range base.Facts.Rail {
		if e.Count != nil {
			counted = i
			break
		}
	}
	if counted < 0 || len(base.Facts.Chips) == 0 || len(base.Facts.Identity.Owners) == 0 {
		t.Fatalf("the base facts must carry a counted rail entry, a chip, and an owner: %+v", base.Facts)
	}
	baseRev, err := documentRevision(base)
	if err != nil {
		t.Fatal(err)
	}
	if baseRev != base.Revision {
		t.Fatalf("the load's revision %q is not documentRevision's %q", base.Revision, baseRev)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*documentSnapshot)
	}{
		{"the Markdown", func(s *documentSnapshot) { s.Markdown += " " }},
		{"the ref", func(s *documentSnapshot) { s.Ref += "x" }},
		{"the kind", func(s *documentSnapshot) { s.Kind = string(specdoc.KindPlan) }},
		{"the stamp's state", func(s *documentSnapshot) {
			s.Facts.Stamp.State, s.Facts.Stamp.Words = documentStateProposed, documentProposedWords
		}},
		{"the stamp's commit", func(s *documentSnapshot) { s.Facts.Stamp.Commit = strings.Repeat("f", 40) }},
		{"the owners", func(s *documentSnapshot) { s.Facts.Identity.Owners[0] = "another-team" }},
		{"an added owner", func(s *documentSnapshot) {
			s.Facts.Identity.Owners = append(s.Facts.Identity.Owners, "another-team")
		}},
		{"the branch", func(s *documentSnapshot) { s.Facts.Identity.Branch = provenFact("design/elsewhere") }},
		{"a detached HEAD", func(s *documentSnapshot) {
			s.Facts.Identity.Branch, s.Facts.Identity.Detached = provenFact(""), true
		}},
		{"an unproven branch", func(s *documentSnapshot) { s.Facts.Identity.Branch = unprovenFact("git exploded") }},
		{"the files", func(s *documentSnapshot) {
			s.Facts.Identity.Files = []string{store.SpecRelPath(store.ZoneArchive, name)}
		}},
		{"the class", func(s *documentSnapshot) { s.Facts.Identity.ClassLabel = "Initiative" }},
		{"a rail count", func(s *documentSnapshot) { *s.Facts.Rail[counted].Count++ }},
		{"a rail id", func(s *documentSnapshot) { s.Facts.Rail[0].ID += "-1" }},
		{"the chips", func(s *documentSnapshot) { s.Facts.Chips = s.Facts.Chips[1:] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := cloneSnapshot(t, base)
			tc.mutate(&changed)
			rev, err := documentRevision(changed)
			if err != nil {
				t.Fatal(err)
			}
			if rev == baseRev {
				t.Fatalf("changing %s left the revision at %s", tc.name, rev)
			}
			if again, _ := documentRevision(base); again != baseRev {
				t.Fatalf("the mutation reached the base snapshot")
			}
		})
	}
	t.Run("the same facts", func(t *testing.T) {
		same := cloneSnapshot(t, base)
		same.HTML, same.Disclosures, same.Revision = "", nil, ""
		if rev, _ := documentRevision(same); rev != baseRev {
			t.Fatalf("an equal snapshot gave %s, want %s", rev, baseRev)
		}
	})
}

// TestDocumentSnapshot_CarriesThePageFacts: the /snapshot projection
// carries the page facts on the wire (strictly decoded), the page embeds
// the same revision, and the facts are the page's own.
func TestDocumentSnapshot_CarriesThePageFacts(t *testing.T) {
	h, repo, name := newAcceptedWallFixture(t)
	page := getStatus(t, h, "/board/spec/"+name+"/document")
	snap := getStatus(t, h, "/board/spec/"+name+"/document/snapshot")
	if page.code != http.StatusOK || snap.code != http.StatusOK {
		t.Fatalf("page %d, snapshot %d\n%s", page.code, snap.code, snap.body)
	}
	var decoded struct {
		Revision    string            `json:"revision"`
		HTML        string            `json:"html"`
		Markdown    string            `json:"markdown"`
		Kind        string            `json:"kind"`
		Ref         string            `json:"ref"`
		Proposed    bool              `json:"proposed"`
		Disclosures []string          `json:"disclosures"`
		Facts       documentPageFacts `json:"facts"`
	}
	if err := decodeStrictJSON(t, snap.body, &decoded); err != nil {
		t.Fatalf("snapshot decode: %v\n%s", err, snap.body)
	}
	if !strings.Contains(page.body, `data-revision="`+decoded.Revision+`"`) {
		t.Fatalf("the page does not embed the snapshot's revision %s", decoded.Revision)
	}
	head := gitOut(t, repo.Dir, "rev-parse", "HEAD")
	f := decoded.Facts
	if f.Stamp != (documentStamp{State: documentStateAccepted, Words: documentStateAccepted, Commit: head}) {
		t.Errorf("stamp %+v, want accepted at %s", f.Stamp, head)
	}
	wantIdentity := documentIdentity{Ref: "spec/" + name, Class: "feature", ClassLabel: "feature", Branch: provenFact("main"),
		Owners: []string{"platform-team"}, Files: []string{store.ActiveSpecRelPath(name)}}
	if !reflect.DeepEqual(f.Identity, wantIdentity) {
		t.Errorf("identity %+v, want %+v", f.Identity, wantIdentity)
	}
	if !reflect.DeepEqual(f.Chips, []documentChip{{ID: "ac-1", Kind: "acceptance-criterion"}}) {
		t.Errorf("chips %+v, want the one criterion", f.Chips)
	}
	if len(f.Rail) == 0 || f.Rail[0].ID != "identity" {
		t.Errorf("rail %s, want the body's sections", railString(f.Rail))
	}
	for _, e := range f.Rail {
		if !strings.Contains(decoded.HTML, `<h2 id="`+e.ID+`">`) {
			t.Errorf("rail id %q is not a heading of the snapshot's own body", e.ID)
		}
	}
}

// TestDocumentSnapshot_ChromeMovesWithTheBody: a poll brings the chrome of
// the revision it brings the body of. A branch change at the same commit
// leaves the Markdown byte-identical and still moves the token, so the
// identity card's branch refreshes; an owners edit arrives with the
// proposed stamp of the very bytes it edited.
func TestDocumentSnapshot_ChromeMovesWithTheBody(t *testing.T) {
	h, repo, name := newAcceptedWallFixture(t)
	path := "/board/spec/" + name + "/document/snapshot"
	type wire struct {
		Revision string            `json:"revision"`
		Markdown string            `json:"markdown"`
		Facts    documentPageFacts `json:"facts"`
	}
	poll := func(t *testing.T, etag string) (int, wire, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("If-None-Match", etag)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var w wire
		if rec.Code == http.StatusOK {
			if err := json.Unmarshal(rec.Body.Bytes(), &w); err != nil {
				t.Fatal(err)
			}
		}
		return rec.Code, w, rec.Header().Get("ETag")
	}
	_, first, etag := poll(t, "")
	if code, _, _ := poll(t, etag); code != http.StatusNotModified {
		t.Fatalf("an unchanged page must answer 304, got %d", code)
	}

	gitOut(t, repo.Dir, "checkout", "-q", "-b", "design/elsewhere")
	code, moved, etag2 := poll(t, etag)
	if code != http.StatusOK || etag2 == etag {
		t.Fatalf("a branch change must move the token: %d, %s -> %s", code, etag, etag2)
	}
	if moved.Markdown != first.Markdown {
		t.Fatalf("the fixture must change the branch alone; the Markdown moved too")
	}
	if moved.Facts.Identity.Branch != provenFact("design/elsewhere") {
		t.Fatalf("the polled identity branch %+v, want design/elsewhere", moved.Facts.Identity.Branch)
	}

	spec := filepath.Join(repo.Dir, store.ActiveSpecRelPath(name))
	data, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spec, []byte(strings.Replace(string(data), "owners: [platform-team]", "owners: [platform-team, docs-team]", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	code, edited, etag3 := poll(t, etag2)
	if code != http.StatusOK || etag3 == etag2 {
		t.Fatalf("an owners edit must move the token: %d", code)
	}
	if !reflect.DeepEqual(edited.Facts.Identity.Owners, []string{"platform-team", "docs-team"}) || edited.Facts.Stamp.State != documentStateProposed || !strings.Contains(edited.Markdown, "Proposed, not accepted") {
		t.Fatalf("the polled chrome must be the edited revision's: owners %v, stamp %+v", edited.Facts.Identity.Owners, edited.Facts.Stamp)
	}
}

// TestDocumentSnapshot_AddsNoResolution (Wave 6 §5.3): the poll's one
// projection reads the checkout's Git state for the identity card's
// branch, which resolves no accepted ref and runs no specstate: its
// counted reads equal the shared loader's alone.
func TestDocumentSnapshot_AddsNoResolution(t *testing.T) {
	_, repo, name := newAcceptedWallFixture(t)
	s := &boardSpecServer{root: repo.Dir}
	git, _, err := s.gitState(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	req := specdocload.Request{Root: repo.Dir, Name: name, Mode: specdocload.ModeWorkingTree, Kind: specdoc.KindSpec}
	if _, err := specdocload.Load(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	specPath := store.ActiveSpecRelPath(name)
	alone := &gitCounts{acceptedRef: git.acceptedRef(), specPath: specPath}
	if _, err := specdocload.Load(gitx.WithObserver(t.Context(), alone), req); err != nil {
		t.Fatal(err)
	}
	poll := &gitCounts{acceptedRef: git.acceptedRef(), specPath: specPath}
	mux := http.NewServeMux()
	for _, rt := range boardSpecRoutes() {
		mux.HandleFunc(rt.suffix, rt.handler(s))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(gitx.WithObserver(t.Context(), poll), http.MethodGet, "/board/spec/"+name+"/document/snapshot", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("snapshot: %d\n%s", rec.Code, rec.Body.String())
	}
	aloneParses, aloneStates := alone.counts()
	pollParses, pollStates := poll.counts()
	t.Logf("accepted-ref rev-parses: loader %d, poll %d; specstate runs: loader %d, poll %d", aloneParses, pollParses, aloneStates, pollStates)
	if aloneParses == 0 || aloneStates == 0 {
		t.Fatalf("the loader alone made %d accepted-ref rev-parses and %d specstate runs: the count would be vacuous", aloneParses, aloneStates)
	}
	if pollParses != aloneParses || pollStates != aloneStates {
		t.Fatalf("the poll made %d accepted-ref rev-parses and %d specstate runs, the loader alone %d and %d", pollParses, pollStates, aloneParses, aloneStates)
	}
}
