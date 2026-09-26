package dex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
)

// The docs site's closed-spec object supersession surface (design §6,
// §8; SI-263): a closed spec's document renders a superseded criterion or
// decision's original text unchanged, then the objsupersede views' lines
// with their links, and a successor's decision renders its decision
// view. The six committed scenario stores (testdata/objsupersede) are
// built into real repositories; the site is built from main, as the
// published site is, and the working tree is put on main first where a
// scenario's checkout is a design branch (default-branch surfaces compute
// from default-branch records only).

const (
	osFeatureDoc   = "a/spec/closed-feature/document/index.html"
	osStoryDoc     = "a/spec/closed-story/document/index.html"
	osSuccessorDoc = "a/spec/successor/document/index.html"
	// The §6 lines on closed-feature#dc-1 after acceptance, with S's
	// decision and the conflict linked (design §6: "linking to S's
	// decision and to the conflict").
	osGovernedFeature = `data-testid="objsupersede-dc-1-governed" data-state="superseded">governed spec/closed-feature's completed work (closed 2024-01-10)</span>`
	osSinceFeature    = `data-testid="objsupersede-dc-1-since" data-state="superseded">superseded since 2024-02-15 by <a href="/a/spec/successor/document/#dc-1">spec/successor#dc-1</a></span> <a class="objsupersede-conflict" data-testid="objsupersede-dc-1-conflict" href="/a/conflict/successor-closed-feature/">conflict/successor-closed-feature</a>`
	osGovernedStory   = `data-testid="objsupersede-ac-1-governed" data-state="superseded">governed spec/closed-story's completed work (closed 2024-01-10)</span>`
	osSinceStory      = `data-testid="objsupersede-ac-1-since" data-state="superseded">superseded since 2024-02-15 by <a href="/a/spec/successor/document/#dc-2">spec/successor#dc-2</a></span> <a class="objsupersede-conflict" data-testid="objsupersede-ac-1-conflict" href="/a/conflict/successor-closed-story/">conflict/successor-closed-story</a>`
	// The original texts, unchanged (design §6: "the original object text, unchanged").
	osFeatureText = "the governed records are listed newest first"
	osStoryText   = "the record list renders every governed record"
)

// buildObjSupersedeSite builds scenario store into a repository, puts its
// working tree on main, and builds the docs site from main into a fresh
// directory. It returns the site directory and the store root.
func buildObjSupersedeSite(t *testing.T, store string) (site, root string) {
	t.Helper()
	neutralizeCIEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, store)
	if branch, err := gitx.CurrentBranch(ctx, repo.Dir); err != nil {
		t.Fatal(err)
	} else if branch != "main" {
		if err := gitx.Checkout(ctx, repo.Dir, "main"); err != nil {
			t.Fatal(err)
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
			{osSuccessorDoc, []string{`data-testid="objsupersede-dc-1-spec-closed-feature-dc-1-edge" data-state="in-force">supersedes <a href="/a/spec/closed-feature/document/#dc-1">spec/closed-feature#dc-1</a></span>`,
				`data-testid="objsupersede-dc-2-spec-closed-story-ac-1-edge" data-state="in-force">supersedes <a href="/a/spec/closed-story/document/#ac-1">spec/closed-story#ac-1</a></span>`}, []string{"objsupersede-ac-1", "not established", "proposed"}},
		}},
		{"carried through two revisions: S1's date kept, carried by the head", "chain", []page{
			{osFeatureDoc, []string{osGovernedFeature, osSinceFeature, `data-testid="objsupersede-dc-1-carry" data-state="superseded">carried by <a href="/a/spec/successor-v3/document/">spec/successor-v3</a></span>`}, nil},
			{osStoryDoc, []string{osGovernedStory, osSinceStory, `data-testid="objsupersede-ac-1-carry" data-state="superseded">carried by <a href="/a/spec/successor-v3/document/">spec/successor-v3</a></span>`}, nil},
			{"a/spec/successor-v3/document/index.html", []string{`data-testid="objsupersede-dc-2-spec-closed-story-ac-1-edge" data-state="in-force">supersedes <a href="/a/spec/closed-story/document/#ac-1">spec/closed-story#ac-1</a></span>`,
				`data-testid="objsupersede-dc-2-spec-closed-story-ac-1-carries" data-state="in-force">carries the replacement established by <a href="/a/spec/successor/document/">spec/successor</a> (<a href="/a/conflict/successor-closed-story/">conflict/successor-closed-story</a>, since 2024-02-15)</span>`}, nil},
		}},
		{"a revision drops the edge: still superseded, no longer carried", "chain-drop", []page{
			{osFeatureDoc, []string{osGovernedFeature, osSinceFeature, `data-testid="objsupersede-dc-1-carry" data-state="superseded">no longer carried by the current revision (<a href="/a/spec/successor-v2/document/">spec/successor-v2</a>)</span>`}, nil},
			{osStoryDoc, []string{osGovernedStory, osSinceStory, `data-testid="objsupersede-ac-1-carry" data-state="superseded">carried by <a href="/a/spec/successor-v2/document/">spec/successor-v2</a></span>`}, nil},
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
				`data-testid="objsupersede-dc-2-spec-closed-story-ac-1-edge" data-state="in-force">supersedes <a href="/a/spec/closed-story/document/#ac-1">spec/closed-story#ac-1</a></span>`,
			}, []string{`objsupersede-dc-1-spec-closed-feature-dc-1-edge`}},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			site, _ := buildObjSupersedeSite(t, tc.store)
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

// TestBuild_ClosedSpecObjectSupersession_AddsLinesOnly: the closed spec's
// document differs from a render with no views supplied by exactly the
// added bullet lines — its fold, rollup, evidence, and verdicts render as
// before (design §6) — and the artifact page's verbatim body is untouched.
func TestBuild_ClosedSpecObjectSupersession_AddsLinesOnly(t *testing.T) {
	site, root := buildObjSupersedeSite(t, "accepted")
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
		page := readFile(t, site, filepath.Join("a", "spec", name, "index.html"))
		if strings.Contains(page, "objsupersede") {
			t.Errorf("%s: the artifact page (the verbatim body) carries supersession markup", name)
		}
	}
	if !strings.Contains(readFile(t, site, "a/spec/closed-feature/index.html"), "The governed records are listed newest first.") {
		t.Errorf("closed-feature's artifact page lost its verbatim body")
	}
}

// squeezeBlank collapses every run of blank lines to one.
func squeezeBlank(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

// TestBuild_ClosedSpecObjectSupersession_Deterministic: two builds of the
// same commit write byte-identical documents (Phase 12's determinism), and
// the view index is computed once per build (the cost note: measured and
// logged, never per spec).
func TestBuild_ClosedSpecObjectSupersession_Deterministic(t *testing.T) {
	site, root := buildObjSupersedeSite(t, "chain")
	again := t.TempDir()
	start := time.Now()
	if err := Build(context.Background(), Options{Root: root, OutDir: again, Commit: "main"}); err != nil {
		t.Fatal(err)
	}
	t.Logf("second build of the chain store: %s", time.Since(start).Round(time.Millisecond))
	for _, p := range []string{osFeatureDoc, osStoryDoc, "a/spec/successor-v3/document/index.html", "a/spec/closed-feature/spec.md"} {
		if readFile(t, site, p) != readFile(t, again, p) {
			t.Errorf("%s differs between two builds of the same commit", p)
		}
	}
	start = time.Now()
	ix, err := supersessionIndex(context.Background(), root, "main")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("view index of the chain store: %s", time.Since(start).Round(time.Millisecond))
	if v := ix.Object("closed-feature", "dc-1"); v.Since != "2024-02-15" {
		t.Errorf("index Object(closed-feature, dc-1).Since = %q, want 2024-02-15", v.Since)
	}
}

func TestSupersessionLink(t *testing.T) {
	known := map[string]bool{"spec/s": true, "spec/nodoc": true, "conflict/c": true}
	docs := documentSet{"spec/s": true}
	tests := []struct {
		ref, want string
	}{
		{"spec/s#dc-1", "/a/spec/s/document/#dc-1"},
		{"spec/s", "/a/spec/s/document/"},
		{"conflict/c", "/a/conflict/c/"},
		{"spec/nodoc#dc-1", ""}, // a page but no document: objects render only in documents
		{"spec/nodoc", ""},
		{"spec/absent#ac-1", ""},
		{"conflict/absent", ""},
		{"spec/s@0123abc#dc-1", ""}, // a view never names a pinned ref; none is linked
		{"", ""},
		{"not a ref", ""},
	}
	for _, tc := range tests {
		if got := supersessionLink(tc.ref, known, docs); got != tc.want {
			t.Errorf("supersessionLink(%q) = %q, want %q", tc.ref, got, tc.want)
		}
	}
}

// TestSupersessionIndex_Errors: an unresolvable commit is the build's
// error (fail closed: a site never renders a superseded object as
// untouched because its records could not be read).
func TestSupersessionIndex_Errors(t *testing.T) {
	neutralizeCIEnv(t)
	repo := scenario.Build(t, "accepted")
	if _, err := supersessionIndex(context.Background(), repo.Dir, "no-such-commit"); err == nil {
		t.Fatal("supersessionIndex accepted an unresolvable commit")
	}
	if _, err := supersessionIndex(context.Background(), t.TempDir(), "HEAD"); err == nil {
		t.Fatal("supersessionIndex accepted a directory that is not a repository")
	}
}
