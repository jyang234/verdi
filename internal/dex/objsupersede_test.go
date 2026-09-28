package dex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
)

// The docs site's closed-spec object supersession surface (design §6,
// §8; SI-263): a closed spec's document renders a superseded criterion or
// decision's original text unchanged, then the objsupersede views' lines
// with their links, and a successor's decision renders its decision
// view. The views come from the shared loader (internal/specdocload),
// computed once per build from the BUILD COMMIT's records — never the
// working tree — so the four document consumers render one set of bytes.
// The six committed scenario stores (testdata/objsupersede) are built
// into real repositories and the site is built from main, as the
// published site is.

const (
	osFeatureDoc   = "a/spec/closed-feature/document/index.html"
	osStoryDoc     = "a/spec/closed-story/document/index.html"
	osSuccessorDoc = "a/spec/successor/document/index.html"
	// The §6 lines on closed-feature#dc-1 after acceptance, with S's
	// decision and the conflict linked to their corpus pages (design §6:
	// "linking to S's decision and to the conflict"; the one address every
	// consumer serves, so the bytes stay identical across them).
	osGovernedFeature = `data-testid="objsupersede-dc-1-governed" data-state="superseded">governed spec/closed-feature's completed work (closed 2024-01-10)</span>`
	osSinceFeature    = `data-testid="objsupersede-dc-1-since" data-state="superseded">superseded since 2024-02-15 by <a href="/a/spec/successor#dc-1">spec/successor#dc-1</a></span> <a class="objsupersede-conflict" data-testid="objsupersede-dc-1-conflict" href="/a/conflict/successor-closed-feature">conflict/successor-closed-feature</a>`
	osGovernedStory   = `data-testid="objsupersede-ac-1-governed" data-state="superseded">governed spec/closed-story's completed work (closed 2024-01-10)</span>`
	osSinceStory      = `data-testid="objsupersede-ac-1-since" data-state="superseded">superseded since 2024-02-15 by <a href="/a/spec/successor#dc-2">spec/successor#dc-2</a></span> <a class="objsupersede-conflict" data-testid="objsupersede-ac-1-conflict" href="/a/conflict/successor-closed-story">conflict/successor-closed-story</a>`
	// The original texts, unchanged (design §6: "the original object text, unchanged").
	osFeatureText = "the governed records are listed newest first"
	osStoryText   = "the record list renders every governed record"
)

// buildObjSupersedeSite builds scenario store into a repository and the
// docs site from main into a fresh directory. With checkoutMain the
// working tree is put on main first (the published site's shape); without
// it the checkout stays where the scenario leaves it, so a site built
// from main over a design-branch checkout can tell the build commit's
// records from the working tree's. It returns the site directory and the
// store root.
func buildObjSupersedeSite(t *testing.T, store string, checkoutMain bool) (site, root string) {
	t.Helper()
	neutralizeCIEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, store)
	if checkoutMain {
		if branch, err := gitx.CurrentBranch(ctx, repo.Dir); err != nil {
			t.Fatal(err)
		} else if branch != "main" {
			if err := gitx.Checkout(ctx, repo.Dir, "main"); err != nil {
				t.Fatal(err)
			}
		}
	}
	out := t.TempDir()
	if err := Build(ctx, Options{Root: repo.Dir, OutDir: out, Commit: "main"}); err != nil {
		t.Fatalf("Build(%s): %v", store, err)
	}
	return out, repo.Dir
}

func TestBuild_ClosedSpecObjectSupersession(t *testing.T) {
	type page struct {
		path       string
		has, lacks []string
	}
	tests := []struct {
		name, store string
		pages       []page
	}{
		{"after acceptance: both targets, S's decision and the conflict linked", "accepted", []page{
			{osFeatureDoc, []string{osFeatureText, osGovernedFeature, osSinceFeature}, []string{"objsupersede-ac-1", "objsupersede-dc-1-carry"}},
			{osStoryDoc, []string{osStoryText, osGovernedStory, osSinceStory}, []string{"objsupersede-dc-1", "objsupersede-ac-1-carry"}},
			{osSuccessorDoc, []string{`data-testid="objsupersede-dc-1-spec-closed-feature-dc-1-edge" data-state="in-force">supersedes <a href="/a/spec/closed-feature#dc-1">spec/closed-feature#dc-1</a></span>`,
				`data-testid="objsupersede-dc-2-spec-closed-story-ac-1-edge" data-state="in-force">supersedes <a href="/a/spec/closed-story#ac-1">spec/closed-story#ac-1</a></span>`}, []string{"objsupersede-ac-1", "not established", "proposed"}},
		}},
		{"carried through two revisions: S1's date kept, carried by the head", "chain", []page{
			{osFeatureDoc, []string{osGovernedFeature, osSinceFeature, `data-testid="objsupersede-dc-1-carry" data-state="superseded">carried by <a href="/a/spec/successor-v3">spec/successor-v3</a></span>`}, nil},
			{osStoryDoc, []string{osGovernedStory, osSinceStory, `data-testid="objsupersede-ac-1-carry" data-state="superseded">carried by <a href="/a/spec/successor-v3">spec/successor-v3</a></span>`}, nil},
			{"a/spec/successor-v3/document/index.html", []string{`data-testid="objsupersede-dc-2-spec-closed-story-ac-1-edge" data-state="in-force">supersedes <a href="/a/spec/closed-story#ac-1">spec/closed-story#ac-1</a></span>`,
				`data-testid="objsupersede-dc-2-spec-closed-story-ac-1-carries" data-state="in-force">carries the replacement established by <a href="/a/spec/successor">spec/successor</a> (<a href="/a/conflict/successor-closed-story">conflict/successor-closed-story</a>, since 2024-02-15)</span>`}, nil},
		}},
		{"a revision drops the edge: still superseded, no longer carried", "chain-drop", []page{
			{osFeatureDoc, []string{osGovernedFeature, osSinceFeature, `data-testid="objsupersede-dc-1-carry" data-state="superseded">no longer carried by the current revision (<a href="/a/spec/successor-v2">spec/successor-v2</a>)</span>`}, nil},
			{osStoryDoc, []string{osGovernedStory, osSinceStory, `data-testid="objsupersede-ac-1-carry" data-state="superseded">carried by <a href="/a/spec/successor-v2">spec/successor-v2</a></span>`}, nil},
		}},
		{"not yet accepted: the default branch shows nothing", "proposed", []page{
			{osFeatureDoc, []string{osFeatureText}, []string{"objsupersede"}},
			{osStoryDoc, []string{osStoryText}, []string{"objsupersede"}},
		}},
		{"not established, not yet accepted: the default branch shows nothing", "no-conflict", []page{
			{osFeatureDoc, []string{osFeatureText}, []string{"objsupersede"}},
			{osStoryDoc, []string{osStoryText}, []string{"objsupersede"}},
		}},
		{"not established after acceptance: the reason on S's decision, never a supersession on the object", "chain-not-in-force", []page{
			{osFeatureDoc, []string{osFeatureText}, []string{"objsupersede"}},
			{osStoryDoc, []string{osGovernedStory, osSinceStory}, nil},
			{osSuccessorDoc, []string{
				`data-testid="objsupersede-dc-1-spec-closed-feature-dc-1-not-established" data-state="not-established">supersession not established: spec/successor's supersession was not in force at its acceptance: no conflict challenges spec/closed-feature#ac-1</span>`,
				`data-testid="objsupersede-dc-3-spec-closed-feature-ac-1-not-established" data-state="not-established">supersession not established: no conflict challenges spec/closed-feature#ac-1</span>`,
				`data-testid="objsupersede-dc-2-spec-closed-story-ac-1-edge" data-state="in-force">supersedes <a href="/a/spec/closed-story#ac-1">spec/closed-story#ac-1</a></span>`,
			}, []string{`objsupersede-dc-1-spec-closed-feature-dc-1-edge`}},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			site, _ := buildObjSupersedeSite(t, tc.store, true)
			for _, p := range tc.pages {
				html := readFile(t, site, p.path)
				for _, want := range p.has {
					if !strings.Contains(html, want) {
						t.Errorf("%s lacks %q:\n%s", p.path, want, html)
					}
				}
				for _, bad := range p.lacks {
					if strings.Contains(html, bad) {
						t.Errorf("%s carries %q, which it must not:\n%s", p.path, bad, html)
					}
				}
			}
			if tc.store == "proposed" || tc.store == "no-conflict" {
				if _, err := os.Stat(filepath.Join(site, "a", "spec", "successor")); !os.IsNotExist(err) {
					t.Errorf("the default branch's site carries the unaccepted successor: %v", err)
				}
			}
		})
	}
}

// TestBuild_ClosedSpecObjectSupersession_BuildCommitNotWorkingTree (the
// L5 docs review's I-2 witness): the site's documents compute from the
// BUILD COMMIT's records — default-branch surfaces from default-branch
// records only (design §6) — never the working tree's. chain-not-in-force
// is built from main with the checkout LEFT on design/successor-v2, whose
// unaccepted revision would otherwise leak a "carrying unproven:
// spec/successor-v2 is not accepted" line onto main's closed-story page.
func TestBuild_ClosedSpecObjectSupersession_BuildCommitNotWorkingTree(t *testing.T) {
	site, root := buildObjSupersedeSite(t, "chain-not-in-force", false)
	if branch, err := gitx.CurrentBranch(context.Background(), root); err != nil || branch != "design/successor-v2" {
		t.Fatalf("the checkout is %q (%v), want design/successor-v2 for this witness", branch, err)
	}
	story := readFile(t, site, osStoryDoc)
	if strings.Contains(story, "objsupersede-ac-1-carry") || strings.Contains(story, "successor-v2") {
		t.Fatalf("main's closed-story document carries the design branch's revision:\n%s", story)
	}
	for _, want := range []string{osGovernedStory, osSinceStory} {
		if !strings.Contains(story, want) {
			t.Errorf("closed-story document lacks %q", want)
		}
	}
	// The design branch's own spec is not on main: no document for it.
	if _, err := os.Stat(filepath.Join(site, "a", "spec", "successor-v2", "document")); !os.IsNotExist(err) {
		t.Errorf("the site built from main carries the design branch's successor-v2 document: %v", err)
	}
}

// TestBuild_ClosedSpecObjectSupersession_AddsLinesOnly: the closed spec's
// document differs from a render with no views supplied by exactly the
// added bullet lines — its fold, rollup, evidence, and verdicts render as
// before (design §6) — and the artifact page's verbatim body is untouched.
func TestBuild_ClosedSpecObjectSupersession_AddsLinesOnly(t *testing.T) {
	site, root := buildObjSupersedeSite(t, "accepted", true)
	sha, err := gitx.RevParse(context.Background(), root, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"closed-feature", "closed-story"} {
		got := readFile(t, site, filepath.Join("a", "spec", name, "spec.md"))
		res, err := specdocload.Load(context.Background(), specdocload.Request{Root: root, Name: name, Mode: specdocload.ModeAt, At: sha, Kind: specdoc.KindSpec})
		if err != nil {
			t.Fatal(err)
		}
		res.Input.Facts.Supersession = nil // the render with no views supplied
		doc, err := specdoc.Build(res.Input)
		if err != nil {
			t.Fatal(err)
		}
		var kept []string
		added := 0
		for _, line := range strings.Split(got, "\n") {
			if strings.Contains(line, `class="objsupersede`) {
				added++
				continue
			}
			kept = append(kept, line)
		}
		if added != 2 {
			t.Errorf("%s: %d added lines, want the two §6 lines", name, added)
		}
		// A decision's bullets end with a blank line so its rationale is
		// not lazily absorbed into the list; blank-line runs carry no
		// content, so both sides are compared with them collapsed.
		if want := squeezeBlank(specdoc.RenderMarkdown(doc)); squeezeBlank(strings.Join(kept, "\n")) != want {
			t.Errorf("%s: the document changed beyond the added lines:\n--- with views, lines removed ---\n%s\n--- without views ---\n%s", name, squeezeBlank(strings.Join(kept, "\n")), want)
		}
	}
	page := readFile(t, site, "a/spec/closed-feature/index.html")
	if !strings.Contains(page, "The governed records are listed newest first.") {
		t.Errorf("closed-feature's artifact page lost its verbatim body")
	}
	if strings.Contains(page, `data-testid="objsupersede-dc-1-`) {
		t.Errorf("the artifact page's verbatim body carries the decision's supersession markup")
	}
}

// TestObjectSupersessionHTML (the L5 docs review's M-1): a closed feature's
// artifact page lists each criterion's text in the feature lens, so a
// superseded criterion carries the same lines there, from the same views.
// No scenario supersedes a closed FEATURE's criterion in force (the
// stores supersede the feature's decision and the story's criterion), so
// the row renderer is proven on the closed story's criterion — the views
// do not care which class declares the object — and the page-level
// negative on the accepted store's closed feature, whose ac-1 is not
// superseded and whose page therefore carries no lines.
func TestObjectSupersessionHTML(t *testing.T) {
	neutralizeCIEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "accepted")
	views, err := specdocload.CommitViews(ctx, repo.Dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	got, err := objectSupersessionHTML(views, "closed-story", "ac-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<ul class="objsupersede-lines" data-testid="lens-supersession-ac-1" data-state="superseded">`,
		`<li><span class="objsupersede objsupersede--superseded" data-testid="objsupersede-ac-1-governed" data-state="superseded">governed spec/closed-story's completed work (closed 2024-01-10)</span></li>`,
		`<li><span class="objsupersede objsupersede--superseded" data-testid="objsupersede-ac-1-since" data-state="superseded">superseded since 2024-02-15 by <a href="/a/spec/successor#dc-2">spec/successor#dc-2</a></span> <a class="objsupersede-conflict" data-testid="objsupersede-ac-1-conflict" href="/a/conflict/successor-closed-story">conflict/successor-closed-story</a></li>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("lens lines lack %q:\n%s", want, got)
		}
	}
	for _, tc := range []struct{ spec, id string }{{"closed-feature", "ac-1"}, {"closed-feature", "dc-9"}, {"nowhere", "ac-1"}} {
		if got, err := objectSupersessionHTML(views, tc.spec, tc.id); err != nil || got != "" {
			t.Errorf("objectSupersessionHTML(%s#%s) = %q, %v; want nothing", tc.spec, tc.id, got, err)
		}
	}
	if got, err := objectSupersessionHTML(nil, "closed-story", "ac-1"); err != nil || got != "" {
		t.Errorf("nil views gave %q, %v", got, err)
	}
	site, _ := buildObjSupersedeSite(t, "accepted", true)
	page := readFile(t, site, "a/spec/closed-feature/index.html")
	if !strings.Contains(page, `<td><code>ac-1</code></td>`) {
		t.Fatalf("closed-feature's page has no feature lens mapping row for ac-1:\n%s", page)
	}
	if strings.Contains(page, "objsupersede") {
		t.Errorf("closed-feature#ac-1 is not superseded on the accepted store, yet its page carries supersession markup:\n%s", page)
	}
}

// squeezeBlank collapses every run of blank lines to one.
func squeezeBlank(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

// TestBuild_ClosedSpecObjectSupersession_OncePerBuild (the L5 docs
// review's M-4): one build of the site computes the view index exactly
// once, however many spec documents it renders, and a rebuild of the
// same commit computes nothing anew (the shared cache); two builds write
// byte-identical documents (Phase 12's determinism).
func TestBuild_ClosedSpecObjectSupersession_OncePerBuild(t *testing.T) {
	neutralizeCIEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "chain")
	before := specdocload.ViewBuilds()
	first := t.TempDir()
	if err := Build(ctx, Options{Root: repo.Dir, OutDir: first, Commit: "main"}); err != nil {
		t.Fatal(err)
	}
	if got := specdocload.ViewBuilds() - before; got != 1 {
		t.Fatalf("the build computed the view index %d times, want exactly once", got)
	}
	again := t.TempDir()
	if err := Build(ctx, Options{Root: repo.Dir, OutDir: again, Commit: "main"}); err != nil {
		t.Fatal(err)
	}
	if got := specdocload.ViewBuilds() - before; got != 1 {
		t.Fatalf("a rebuild of the same commit computed the view index again (%d builds)", got)
	}
	for _, p := range []string{osFeatureDoc, osStoryDoc, "a/spec/successor-v3/document/index.html", "a/spec/closed-feature/spec.md", "a/spec/closed-feature/index.html"} {
		if readFile(t, first, p) != readFile(t, again, p) {
			t.Errorf("%s differs between two builds of the same commit", p)
		}
	}
}

// TestFeatureLensHTML_RendersSupersededCriterionLines (the L5 docs
// review's M-1 closure, fix pass 2 item 10): the feature lens's criterion
// ROW carries the lines — a lens driven by a class-feature page whose
// criterion the views supersede must put them in the row's Text cell,
// right after the criterion's text. The views do not care which class
// declares the object, so the accepted store's closed story stands in as
// the feature; a lens that drops the lines fails here.
func TestFeatureLensHTML_RendersSupersededCriterionLines(t *testing.T) {
	neutralizeCIEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "accepted")
	views, err := specdocload.CommitViews(ctx, repo.Dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	ix, err := index.Build(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	story := views.Records.Specs["closed-story"].FM
	p := &artifactPage{
		Entry:   &index.Entry{Ref: "spec/closed-story", Kind: "spec", Title: story.Title},
		Meta:    meta{Class: artifact.ClassFeature, Problem: &artifact.Attribute{Text: "p"}, AcceptanceCriteria: story.AcceptanceCriteria},
		RelPath: ".verdi/specs/archive/closed-story/spec.md",
	}
	html, err := featureLensHTML(ix, knownRefs(ix), nil, views, p)
	if err != nil {
		t.Fatal(err)
	}
	row := `<tr><td><code>ac-1</code></td><td>` + osStoryText + `<ul class="objsupersede-lines" data-testid="lens-supersession-ac-1" data-state="superseded">`
	if !strings.Contains(string(html), row) {
		t.Fatalf("the lens row lacks the criterion's lines right after its text:\nwant %s\n--- got ---\n%s", row, html)
	}
	if !strings.Contains(string(html), `data-testid="objsupersede-ac-1-since" data-state="superseded">superseded since 2024-02-15 by <a href="/a/spec/successor#dc-2">spec/successor#dc-2</a></span>`) {
		t.Errorf("the lens row lacks the since line:\n%s", html)
	}
	// A criterion the views do not touch renders its bare row.
	feature := views.Records.Specs["closed-feature"].FM
	p = &artifactPage{
		Entry:   &index.Entry{Ref: "spec/closed-feature", Kind: "spec", Title: feature.Title},
		Meta:    meta{Class: artifact.ClassFeature, Problem: &artifact.Attribute{Text: "p"}, AcceptanceCriteria: feature.AcceptanceCriteria},
		RelPath: ".verdi/specs/archive/closed-feature/spec.md",
	}
	html, err = featureLensHTML(ix, knownRefs(ix), nil, views, p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `<tr><td><code>ac-1</code></td><td>an operator can read the governed records</td><td>`) || strings.Contains(string(html), "objsupersede") {
		t.Errorf("an untouched criterion's row changed:\n%s", html)
	}
}
