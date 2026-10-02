package workbench

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
	"github.com/jyang234/verdi/internal/store"
)

// The Document page's chrome facts (spec/document-page-v2 ac-1, ac-2;
// ledger SI-340 (4)–(7), (10)): the stamp, the identity card, the contents
// rail, and the id chips, as one value per render built from what the
// page's own load resolved.

const factsSpecName = "rail-fixture"

// factsSpec declares every object kind the document enumerates, so each
// counted section has a count other than zero or one.
const factsSpec = `---
id: spec/rail-fixture
kind: spec
title: "Rail fixture"
owners: [platform-team, docs-team]
class: feature
problem: { text: "A problem.", anchor: problem }
outcome: { text: "An outcome.", anchor: outcome }
decisions:
  - { id: dc-1, text: "First decision.", anchor: dc-1 }
  - { id: dc-2, text: "Second decision.", anchor: dc-2 }
constraints:
  - { id: co-1, text: "One constraint.", anchor: co-1 }
acceptance_criteria:
  - { id: ac-1, text: "First.", evidence: [static], anchor: ac-1 }
  - { id: ac-2, text: "Second.", evidence: [static], anchor: ac-2 }
  - { id: ac-3, text: "Third.", evidence: [static], anchor: ac-3 }
open_questions:
  - { id: oq-1, text: "First question?", anchor: oq-1 }
  - { id: oq-2, text: "Second question?", anchor: oq-2 }
stubs:
  - { slug: first-story, acceptance_criteria: [ac-1, ac-2, ac-3] }
---
`

const factsCommit = "0123456789abcdef0123456789abcdef01234567"

// factsDoc builds the document the page's load would build for spec under
// kind (with mutate applied to the load's Input first), its HTML, and the
// load's Result as the active zone holds it.
func factsDoc(t *testing.T, spec, body string, kind specdoc.Kind, mutate func(*specdoc.Input)) (specdoc.Document, string, specdocload.Result) {
	t.Helper()
	fmBytes, _, err := artifact.SplitFrontmatter([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	fm, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		t.Fatal(err)
	}
	in := specdoc.Input{
		Spec:  fm,
		Body:  []byte(body),
		Stamp: specdoc.Stamp{Ref: "spec/" + factsSpecName, Commit: factsCommit},
		Facts: specdoc.FactsFromSpec(fm),
		Kind:  kind,
	}
	if mutate != nil {
		mutate(&in)
	}
	doc, err := specdoc.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	html, err := specdoc.RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	return doc, html, specdocload.Result{Input: in, RelPath: store.ActiveSpecRelPath(factsSpecName)}
}

// withEvidenceAndReadiness supplies evidence for the three criteria and a
// readiness snapshot whose attention queue holds two concerns.
func withEvidenceAndReadiness(in *specdoc.Input) {
	in.Facts.Evidence = map[string]specdoc.ACEvidence{
		"ac-1": {Status: "violated"}, "ac-2": {Status: "pending"}, "ac-3": {Status: "evidenced"},
	}
	in.Facts.EvidenceSource = "matrix over the working tree at 0123456789ab"
	in.Facts.Readiness = &specdoc.ReadinessFacts{
		TargetRef: "spec/" + factsSpecName, Head: factsCommit, CurrentFocus: "shape",
		Areas: []specdoc.ReadinessArea{{ID: "shape", Label: "Define the work", State: "attention"}, {ID: "success", Label: "Define success", State: "proven"}},
		Attention: []specdoc.ReadinessConcern{
			{ID: "shape/question/oq-1", Area: "shape", State: "open", Summary: "oq-1 is unclaimed", Timing: "now"},
			{ID: "shape/question/oq-2", Area: "shape", State: "open", Summary: "oq-2 is unclaimed", Timing: "now"},
		},
	}
}

// railEntry is an expected rail entry; count < 0 means no count.
func railEntry(id, text string, count int) documentRailEntry {
	e := documentRailEntry{ID: id, Text: text}
	if count >= 0 {
		e.Count = &count
	}
	return e
}

func railString(rail []documentRailEntry) string {
	var b strings.Builder
	for _, e := range rail {
		b.WriteString(e.ID + "(" + e.Text + ")")
		if e.Count != nil {
			b.WriteString("=" + strings.Repeat("|", *e.Count))
		}
		b.WriteString(" ")
	}
	return b.String()
}

// TestDocumentPageFacts_Rail (SI-340 (7)): the rail lists the body's h2
// sections in order, with the ids the body carries; a section that
// enumerates items shows the number it renders, and no other does.
func TestDocumentPageFacts_Rail(t *testing.T) {
	for _, tc := range []struct {
		name, spec, body string
		kind             specdoc.Kind
		mutate           func(*specdoc.Input)
		want             []documentRailEntry
	}{
		{
			name: "every section, counted where it enumerates items", spec: factsSpec, kind: specdoc.KindSpec, mutate: withEvidenceAndReadiness,
			want: []documentRailEntry{
				railEntry("identity", "Identity", -1), railEntry("problem", "Problem", -1), railEntry("outcome", "Outcome", -1),
				railEntry("decisions", "Decisions", 2), railEntry("constraints", "Constraints", 1),
				railEntry("acceptance-criteria", "Acceptance criteria", 3), railEntry("open-questions", "Open questions", 2),
				railEntry("plan", "Plan", 1), railEntry("evidence", "Evidence", 3), railEntry("readiness", "Readiness", 2),
			},
		},
		{
			name: "evidence and readiness not supplied show no count", spec: factsSpec, kind: specdoc.KindSpec,
			want: []documentRailEntry{
				railEntry("identity", "Identity", -1), railEntry("problem", "Problem", -1), railEntry("outcome", "Outcome", -1),
				railEntry("decisions", "Decisions", 2), railEntry("constraints", "Constraints", 1),
				railEntry("acceptance-criteria", "Acceptance criteria", 3), railEntry("open-questions", "Open questions", 2),
				railEntry("plan", "Plan", 1), railEntry("evidence", "Evidence", -1), railEntry("readiness", "Readiness", -1),
			},
		},
		{
			name: "a section with nothing declared counts zero", kind: specdoc.KindSpec,
			spec: strings.Replace(strings.Replace(factsSpec, "decisions:\n  - { id: dc-1, text: \"First decision.\", anchor: dc-1 }\n  - { id: dc-2, text: \"Second decision.\", anchor: dc-2 }\n", "", 1),
				"open_questions:\n  - { id: oq-1, text: \"First question?\", anchor: oq-1 }\n  - { id: oq-2, text: \"Second question?\", anchor: oq-2 }\n", "", 1),
			want: []documentRailEntry{
				railEntry("identity", "Identity", -1), railEntry("problem", "Problem", -1), railEntry("outcome", "Outcome", -1),
				railEntry("decisions", "Decisions", 0), railEntry("constraints", "Constraints", 1),
				railEntry("acceptance-criteria", "Acceptance criteria", 3), railEntry("open-questions", "Open questions", 0),
				railEntry("plan", "Plan", 1), railEntry("evidence", "Evidence", -1), railEntry("readiness", "Readiness", -1),
			},
		},
		{
			name: "the plan kind's sections", spec: factsSpec, kind: specdoc.KindPlan, mutate: withEvidenceAndReadiness,
			want: []documentRailEntry{
				railEntry("identity", "Identity", -1), railEntry("decisions", "Decisions", 2),
				railEntry("constraints", "Constraints", 1), railEntry("plan", "Plan", 1),
			},
		},
		{
			name: "the tasks kind's sections", spec: factsSpec, kind: specdoc.KindTasks, mutate: withEvidenceAndReadiness,
			want: []documentRailEntry{
				railEntry("identity", "Identity", -1), railEntry("plan", "Plan", 1),
				railEntry("evidence", "Evidence", 3), railEntry("readiness", "Readiness", 2),
			},
		},
		{
			// The h1 takes the bare id, so goldmark de-duplicates the
			// section's: the rail reads the id the body carries.
			name: "a title that collides with a section heading", kind: specdoc.KindPlan,
			spec: strings.Replace(factsSpec, `title: "Rail fixture"`, `title: "Plan"`, 1),
			want: []documentRailEntry{
				railEntry("identity", "Identity", -1), railEntry("decisions", "Decisions", 2),
				railEntry("constraints", "Constraints", 1), railEntry("plan-1", "Plan", 1),
			},
		},
		{
			// A rationale whose own text renders an h2 (a setext heading)
			// is in the body, so it is in the rail, with no count; the
			// sections around it keep theirs.
			name: "a heading the body's prose adds", spec: factsSpec, kind: specdoc.KindPlan,
			body: "# Rail fixture\n\n## dc-2\n\nAn aside\n---\n\nThe rationale.\n",
			want: []documentRailEntry{
				railEntry("identity", "Identity", -1), railEntry("decisions", "Decisions", 2), railEntry("an-aside", "An aside", -1),
				railEntry("constraints", "Constraints", 1), railEntry("plan", "Plan", 1),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, html, res := factsDoc(t, tc.spec, tc.body, tc.kind, tc.mutate)
			got := newDocumentPageFacts(factsSpecName, doc, html, res, documentCheckout{git: &boardGitState{Branch: "main"}}, nil).Rail
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("rail =\n%s\nwant\n%s", railString(got), railString(tc.want))
			}
			// Every rail id is an id the body carries.
			for _, e := range got {
				if !strings.Contains(html, `<h2 id="`+e.ID+`">`) {
					t.Errorf("rail id %q is not a heading id in the body", e.ID)
				}
			}
		})
	}
}

// TestDocumentPageFacts_Chips (SI-340 (10)): chips only where they can
// work — the active-zone spec the wall serves, at the anchors the body
// renders for its acceptance criteria, constraints, decisions, and open
// questions; never at readiness anchors, and none on an archived spec.
func TestDocumentPageFacts_Chips(t *testing.T) {
	dc := func(id string) documentChip { return documentChip{ID: id, Kind: "decision"} }
	co := func(id string) documentChip { return documentChip{ID: id, Kind: "constraint"} }
	ac := func(id string) documentChip { return documentChip{ID: id, Kind: "acceptance-criterion"} }
	oq := func(id string) documentChip { return documentChip{ID: id, Kind: "open-question"} }
	for _, tc := range []struct {
		name    string
		kind    specdoc.Kind
		relPath string
		want    []documentChip
	}{
		{name: "the active spec's objects, in document order", kind: specdoc.KindSpec, relPath: store.ActiveSpecRelPath(factsSpecName),
			want: []documentChip{dc("dc-1"), dc("dc-2"), co("co-1"), ac("ac-1"), ac("ac-2"), ac("ac-3"), oq("oq-1"), oq("oq-2")}},
		{name: "only the anchors the plan kind renders", kind: specdoc.KindPlan, relPath: store.ActiveSpecRelPath(factsSpecName),
			want: []documentChip{dc("dc-1"), dc("dc-2"), co("co-1")}},
		{name: "the tasks kind renders no object anchor", kind: specdoc.KindTasks, relPath: store.ActiveSpecRelPath(factsSpecName),
			want: []documentChip{}},
		{name: "an archived spec, whose wall answers 404", kind: specdoc.KindSpec, relPath: store.SpecRelPath(store.ZoneArchive, factsSpecName),
			want: []documentChip{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, html, res := factsDoc(t, factsSpec, "", tc.kind, withEvidenceAndReadiness)
			res.RelPath = tc.relPath
			got := newDocumentPageFacts(factsSpecName, doc, html, res, documentCheckout{git: &boardGitState{Branch: "main"}}, nil).Chips
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("chips = %+v, want %+v", got, tc.want)
			}
			for _, c := range got {
				if !strings.Contains(html, `<a id="`+c.ID+`"></a>`) {
					t.Errorf("chip %q has no anchor in the body", c.ID)
				}
			}
		})
	}
}

// TestDocumentPageFacts_Stamp (SI-340 (5)–(6)): the state words and the
// commit shown; the refreshed time is the browser's (dc-2), never a field.
func TestDocumentPageFacts_Stamp(t *testing.T) {
	for _, tc := range []struct {
		name     string
		proposed bool
		want     documentStamp
	}{
		{name: "proposed", proposed: true, want: documentStamp{State: "proposed", Words: "proposed, not accepted", Commit: factsCommit}},
		{name: "accepted", proposed: false, want: documentStamp{State: "accepted", Words: "accepted", Commit: factsCommit}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, html, res := factsDoc(t, factsSpec, "", specdoc.KindSpec, func(in *specdoc.Input) { in.Stamp.Proposed = tc.proposed })
			if got := newDocumentPageFacts(factsSpecName, doc, html, res, documentCheckout{git: &boardGitState{Branch: "main"}}, nil).Stamp; got != tc.want {
				t.Fatalf("stamp = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestDocumentPageFacts_Identity (SI-340 (4)): the ref, class, branch,
// owners, and the files the load rendered from; a branch that cannot be
// read is disclosed-unproven, and a detached HEAD is marked as such.
func TestDocumentPageFacts_Identity(t *testing.T) {
	const spike = `---
id: spec/rail-fixture
kind: spec
class: story
title: "Rail spike"
owners: [platform-team, docs-team]
problem: { text: "p", anchor: problem }
outcome: { text: "o", anchor: outcome }
spike: true
story: jira:LOAN-1490
links:
  - { type: resolves, ref: "spec/loan-update#oq-1" }
---
`
	for _, tc := range []struct {
		name     string
		spec     string
		checkout documentCheckout
		model    *model.Model
		want     documentIdentity
	}{
		{
			name: "on a branch", spec: factsSpec, checkout: documentCheckout{git: &boardGitState{Branch: "design/rail-fixture"}},
			want: documentIdentity{Ref: "spec/rail-fixture", Class: "feature", ClassLabel: "feature", Branch: provenFact("design/rail-fixture"),
				Owners: []string{"platform-team", "docs-team"}, Files: []string{".verdi/specs/active/rail-fixture/spec.md"}},
		},
		{
			name: "renamed vocabulary", spec: factsSpec, checkout: documentCheckout{git: &boardGitState{Branch: "main"}}, model: vocabTestModel(),
			want: documentIdentity{Ref: "spec/rail-fixture", Class: "feature", ClassLabel: "Initiative", Branch: provenFact("main"),
				Owners: []string{"platform-team", "docs-team"}, Files: []string{".verdi/specs/active/rail-fixture/spec.md"}},
		},
		{
			name: "a spike story", spec: spike, checkout: documentCheckout{git: &boardGitState{Branch: "main"}}, model: vocabTestModel(),
			want: documentIdentity{Ref: "spec/rail-fixture", Class: "spike", ClassLabel: "Deep Dive", Branch: provenFact("main"),
				Owners: []string{"platform-team", "docs-team"}, Files: []string{".verdi/specs/active/rail-fixture/spec.md"}},
		},
		{
			name: "a detached HEAD", spec: factsSpec, checkout: documentCheckout{git: &boardGitState{}},
			want: documentIdentity{Ref: "spec/rail-fixture", Class: "feature", ClassLabel: "feature", Branch: provenFact(""), Detached: true,
				Owners: []string{"platform-team", "docs-team"}, Files: []string{".verdi/specs/active/rail-fixture/spec.md"}},
		},
		{
			name: "a Git state that cannot be read", spec: factsSpec, checkout: documentCheckout{err: errors.New("git exploded")},
			want: documentIdentity{Ref: "spec/rail-fixture", Class: "feature", ClassLabel: "feature", Branch: unprovenFact("the checkout's Git state could not be read: git exploded"),
				Owners: []string{"platform-team", "docs-team"}, Files: []string{".verdi/specs/active/rail-fixture/spec.md"}},
		},
		{
			name: "a Git state never read", spec: factsSpec, checkout: documentCheckout{},
			want: documentIdentity{Ref: "spec/rail-fixture", Class: "feature", ClassLabel: "feature", Branch: unprovenFact("the checkout's Git state could not be read: no load read it"),
				Owners: []string{"platform-team", "docs-team"}, Files: []string{".verdi/specs/active/rail-fixture/spec.md"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, html, res := factsDoc(t, tc.spec, "", specdoc.KindSpec, nil)
			if got := newDocumentPageFacts(factsSpecName, doc, html, res, tc.checkout, tc.model).Identity; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("identity =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

// TestDocumentPageFacts_EqualTheBar: within one render, the identity
// card's class and branch are the bar's (SI-323 (2)), the files are the
// ones the load rendered from, and the stamp is the document's own; the
// facts are the ones the page's own load built, so the snapshot carries
// the same value.
func TestDocumentPageFacts_EqualTheBar(t *testing.T) {
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
		{name: "a detached HEAD", spec: documentWallName, server: func(t *testing.T) *boardSpecServer {
			_, repo, _ := newAcceptedWallFixture(t)
			gitOut(t, repo.Dir, "checkout", "-q", "--detach")
			return &boardSpecServer{root: repo.Dir}
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
			snap, res, err := s.loadDocument(t.Context(), tc.spec, specdoc.KindSpec)
			if err != nil {
				t.Fatalf("loadDocument: %v", err)
			}
			bar := s.documentBarFacts(t.Context(), tc.spec, res, snap.checkout)
			f := snap.Facts
			if f.Identity.Class != bar.Spec.Class || f.Identity.ClassLabel != bar.Spec.ClassLabel {
				t.Errorf("identity class %q/%q, bar class %q/%q", f.Identity.Class, f.Identity.ClassLabel, bar.Spec.Class, bar.Spec.ClassLabel)
			}
			if f.Identity.Branch != bar.Posture.Branch || f.Identity.Detached != bar.Posture.Detached {
				t.Errorf("identity branch %+v detached %t, bar branch %+v detached %t", f.Identity.Branch, f.Identity.Detached, bar.Posture.Branch, bar.Posture.Detached)
			}
			if !reflect.DeepEqual(f.Identity.Files, []string{res.RelPath}) || !reflect.DeepEqual(f.Identity.Owners, res.Input.Spec.Owners) || f.Identity.Ref != "spec/"+tc.spec {
				t.Errorf("identity %+v, want ref spec/%s, owners %v, files [%s]", f.Identity, tc.spec, res.Input.Spec.Owners, res.RelPath)
			}
			if f.Stamp.Commit != res.Input.Stamp.Commit || (f.Stamp.State == "proposed") != snap.Proposed || (bar.Spec.Bytes.Word == "proposed") != snap.Proposed {
				t.Errorf("stamp %+v, document proposed %t, bar bytes %q", f.Stamp, snap.Proposed, bar.Spec.Bytes.Word)
			}
			if len(f.Rail) == 0 || len(f.Chips) == 0 {
				t.Errorf("an active spec's facts carry a rail and chips: %+v", f)
			}
		})
	}
}

// TestDocumentPageFacts_ChipsFollowTheWall (SI-340 (10)): over the
// accepted scenario store, an archived spec's Document page serves while
// its wall answers 404, and it carries no chip; the active successor's
// wall serves, and its page carries chips.
func TestDocumentPageFacts_ChipsFollowTheWall(t *testing.T) {
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME"} {
		t.Setenv(key, "")
	}
	repo := scenario.Build(t, "accepted")
	h := NewHandler(repo.Dir)
	s := &boardSpecServer{root: repo.Dir}
	for _, tc := range []struct {
		name  string
		wall  int
		chips bool
	}{
		{name: "closed-feature", wall: http.StatusNotFound, chips: false},
		{name: "successor", wall: http.StatusOK, chips: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := getStatus(t, h, "/board/spec/"+tc.name).code; got != tc.wall {
				t.Fatalf("wall answered %d, want %d", got, tc.wall)
			}
			if got := getStatus(t, h, "/board/spec/"+tc.name+"/document").code; got != http.StatusOK {
				t.Fatalf("Document page answered %d", got)
			}
			snap, _, err := s.loadDocument(t.Context(), tc.name, specdoc.KindSpec)
			if err != nil {
				t.Fatalf("loadDocument: %v", err)
			}
			if got := len(snap.Facts.Chips) > 0; got != tc.chips {
				t.Fatalf("chips %+v, want present=%t", snap.Facts.Chips, tc.chips)
			}
		})
	}
}

// TestDocumentPageFacts_CountsTheLoadsOwnDocument (SI-340 (1)): the
// evidence and readiness counts come from the document the page's own
// load built — readiness wired through the page's own loader.
func TestDocumentPageFacts_CountsTheLoadsOwnDocument(t *testing.T) {
	snapshot := readinesspilot.Snapshot{
		TargetRef: "spec/" + documentWallName, CurrentFocus: readinesspilot.AreaShape,
		Areas: []readinesspilot.Area{{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateProven}},
		Attention: []readinesspilot.Concern{
			{ID: "shape/one", Area: readinesspilot.AreaShape, Summary: "one"},
			{ID: "shape/two", Area: readinesspilot.AreaShape, Summary: "two"},
			{ID: "shape/three", Area: readinesspilot.AreaShape, Summary: "three"},
		},
	}
	_, repo, _ := newAcceptedWallFixtureWithReadiness(t, &snapshot)
	s := &boardSpecServer{root: repo.Dir, readinessLoader: fixedSnapshotLoader{snap: snapshot}}
	snap, _, err := s.loadDocument(t.Context(), documentWallName, specdoc.KindSpec)
	if err != nil {
		t.Fatalf("loadDocument: %v", err)
	}
	counts := map[string]int{}
	for _, e := range snap.Facts.Rail {
		if e.Count != nil {
			counts[e.ID] = *e.Count
		}
	}
	if counts["readiness"] != 3 {
		t.Errorf("readiness count %d, want the three concerns the body lists (rail %s)", counts["readiness"], railString(snap.Facts.Rail))
	}
	if n, ok := counts["evidence"]; ok && n != 1 {
		t.Errorf("evidence count %d, want the one criterion's row", n)
	}
	if !strings.Contains(snap.Markdown, "3. three — ") {
		t.Fatalf("the body lists the three concerns:\n%s", snap.Markdown)
	}
}

// TestDocumentSectionHeadings pins the rail's section headings to the
// shared renderer's own: every section of every kind is written as an h2
// with exactly that text, in the kind's order; an unknown section has none.
func TestDocumentSectionHeadings(t *testing.T) {
	for _, kind := range []specdoc.Kind{specdoc.KindSpec, specdoc.KindPlan, specdoc.KindTasks} {
		t.Run(string(kind), func(t *testing.T) {
			doc, _, _ := factsDoc(t, factsSpec, "", kind, withEvidenceAndReadiness)
			md := specdoc.RenderMarkdown(doc)
			at := 0
			for _, s := range doc.Sections {
				h := documentSectionHeading(s)
				i := strings.Index(md[at:], "\n## "+h+"\n")
				if h == "" || i < 0 {
					t.Fatalf("section %q: heading %q is not the renderer's next h2 after byte %d:\n%s", s, h, at, md)
				}
				at += i + 1
			}
		})
	}
	if h := documentSectionHeading(specdoc.SectionID("glossary")); h != "" {
		t.Fatalf("an unknown section's heading = %q, want none", h)
	}
}
