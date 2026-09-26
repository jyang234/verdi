package main

// objSupersedeFixture (design docs/superpowers/specs/2026-09-24-closed-
// spec-object-supersession-design.md §8; L3c report's "Item 5 handoff"):
// the surface lane's e2e proof needs a docs site and a board per
// closed-spec object supersession scenario — a criterion target and a
// decision target, in force, carried, dropped, not yet accepted, and not
// established — and the shared harness store carries none of that
// authority shape. So this provisions EACH scenario as its own scratch git
// repository with scenario.Materialize (never reimplementing the replay),
// reading the committed fixture from moduleRoot/testdata/objsupersede
// (never the source file's location, which a -trimpath build loses), and
// gives each store, on loopback only:
//
//   - a board: one `verdi serve` subprocess over the store's checkout,
//     the same build-then-exec seam as unprovenboard.go and
//     specimportfixture.go, one binary build shared by every store, run
//     under hermeticServeEnv as they are (no ambient review, open-MR, or
//     diagram-verification feed and no CI identity reaches it). Before
//     it starts, .verdi/data/ goes into the repository's .git/info/exclude
//     — never a commit, so no SHA moves — so the serve's own lock files and
//     managed worktrees never read as uncommitted changes (nothing under
//     .verdi/data/ is ever committed, CLAUDE.md).
//   - a docs site: internal/dex's Build — the site the published docs use —
//     over a DETACHED worktree of main at <scratch>/<store>/main (never a
//     named-branch worktree of main: the workbench would then find main
//     "already checked out" and answer /b/main/... from the serving
//     checkout), built at main's own commit as dex.Options.Commit (its only
//     clock; never the wall clock), and served by a loopback file server.
//
// Everything starts lazily on the control server's first GET
// /objsupersede-fixture, is reused thereafter, and stops with the harness
// (main.go defers stop, which also removes the run's scratch). A failed
// start reaps whatever it started, removes its scratch, and caches
// nothing. Test-only.
//
// Consumer note: a cold first GET includes a full `go build` of the verdi
// binary plus six materializations, docs builds, and serves — warm the
// fixture in a beforeAll with its own timeout allowance (as
// e2e/tests/49-readiness-pilot.spec.ts does) so the default per-request
// budget never cancels it mid-build.
//
// # JSON contract
//
// GET /objsupersede-fixture answers {"stores": {"<store>": S}}, keyed by
// store (= scenario) name, where S is (every key always present):
//
//	scenario                the scenario name
//	url                     the store's verdi serve base URL ("http://127.0.0.1:<port>/")
//	docs_url                the store's docs site base URL, built from main
//	docs_commit             main's full commit, the one the docs site was built at
//	checkout                the branch the serve's checkout has
//	main_branch             "main", the default branch
//	design_branch           the design branch that proposed "successor"
//	successor               the spec under test at the checkout
//	establishing_successor  the spec every conflict's resolved_by names: S in
//	                        design §6's "superseded since … by spec/S#<decision-id>"
//	conflicts               the conflict refs at the checkout, sorted
//	supersessions           every decision-to-object supersedes edge, sorted by object:
//	  object                  the closed object, "spec/<closed>#<id>"
//	  object_docs_url         its document page on the docs site, with its anchor
//	  decision                the successor's deciding decision, "" when the
//	                          successor no longer carries the edge
//	  establishing_decision   the establishing successor's decision on the edge
//	  conflict                the conflict challenging the object, "" when none does
//	boards                  {"checkout": V, "design": V, "main": V}, where V is
//	                        {"branch", "spec", "url", "not_a_surface"} and exactly
//	                        one of url and not_a_surface is non-empty
//	docs                    spec ref -> its document page on the docs site: both
//	                        closed specs, and each successor spec main carries
//
// These are record facts, not outcomes: whether an edge is in force,
// carried, proposed, or refused is the objsupersede views' answer, which
// the surfaces render. Per store:
//
//	store               checkout             successor          main board
//	accepted            main                 spec/successor     the checkout's board
//	chain               main                 spec/successor-v3  the checkout's board
//	chain-drop          main                 spec/successor-v2  the checkout's board
//	proposed            design/successor     spec/successor     not a surface
//	no-conflict         design/successor     spec/successor     not a surface
//	chain-not-in-force  design/successor-v2  spec/successor-v2  /b/main/board/spec/successor
//
// Boards: internal/workbench mounts the board twice — /board/spec/<name>
// for the serving checkout's branch and /b/<branch>/board/spec/<name> (the
// branch percent-encoded, as BranchBoardHref does) for any other branch,
// cut as a managed worktree on first request (branchboard.go) — so one
// serve per store answers every branch. The design board of accepted,
// chain, and chain-drop is their merged design branch, which still
// resolves. For proposed and no-conflict, main has no board that shows a
// closed object: the closed specs are archived and archived specs have no
// board (ADJ-39), and a closed object renders on a board only as a
// reference card on a board whose spec links it (SI-278), which no spec on
// main does. So their main board is reported as not a surface, never as a
// URL that 404s by design; "the default branch shows nothing" is asserted
// on the docs site's closed-spec document pages, where the object itself
// renders. The one /b/main URL handed out, chain-not-in-force's main board
// (main carries its accepted spec/successor), relies on the workbench
// cutting a managed worktree named "main" for the default branch — outside
// the managed-worktree domain's design-branch naming (wtmanager naming.go
// dc-1; `verdi gc` would map that directory back to design/main).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jyang234/verdi/internal/dex"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

// objSupersedeStores is the fixed, deterministic set of scenario stores
// the surface lane needs (L3c report's Item 5 handoff): the happy path
// (accepted), the carried and dropped whole-spec-revision chains (chain,
// chain-drop), the not-yet-accepted pair (proposed, no-conflict), and the
// broken-chain not-in-force case (chain-not-in-force). Order is fixed so
// the fixture's provisioning is deterministic, not map iteration.
var objSupersedeStores = []string{"accepted", "chain", "chain-drop", "proposed", "no-conflict", "chain-not-in-force"}

// objSupersedeDataZone is the store's runtime data zone, excluded (never
// committed) in every provisioned repository.
const objSupersedeDataZone = ".verdi/data/"

// objSupersedeNoMainBoard is why proposed's and no-conflict's main board is
// not a surface (see the file doc).
const objSupersedeNoMainBoard = "no board on main shows a closed object here: spec/closed-feature and spec/closed-story are archived and archived specs have no board (ADJ-39), and a closed object renders on a board only as a reference card on a board whose spec links it (SI-278), which no spec on main does; assert the default branch's absence on the docs pages"

// The committed records' refs (testdata/objsupersede/records).
const (
	osEstablisher      = "spec/successor"
	osClosedFeature    = "spec/closed-feature"
	osClosedStory      = "spec/closed-story"
	osConflictFeature  = "conflict/successor-closed-feature"
	osConflictStory    = "conflict/successor-closed-story"
	osFeatureDecision  = osClosedFeature + "#dc-1"
	osFeatureCriterion = osClosedFeature + "#ac-1"
	osStoryCriterion   = osClosedStory + "#ac-1"
)

// objSupersedeFacts is one store's static record facts, read off the
// committed records and scenarios.json; the Happy test re-derives every
// one from the served repository's own records.
type objSupersedeFacts struct {
	designBranch string
	successor    string
	// mainSpec is the successor spec whose board is main's surface, ""
	// when main carries none.
	mainSpec string
	// mainSuccessors are the successor specs main carries, each with a
	// document on the docs site beside the two closed specs'.
	mainSuccessors []string
	// pairs carry object, decision, establishing decision, and conflict;
	// newObjSupersedeStoreInfo adds each object's docs URL.
	pairs []objSupersedeSupersession
}

// osPair is one decision-to-object edge's facts.
func osPair(object, decision, establishing, conflict string) objSupersedeSupersession {
	return objSupersedeSupersession{Object: object, Decision: decision, EstablishingDecision: establishing, Conflict: conflict}
}

// objSupersedeFactsByStore holds every store's facts.
var objSupersedeFactsByStore = map[string]objSupersedeFacts{
	"accepted": {
		designBranch: "design/successor", successor: "spec/successor", mainSpec: "spec/successor",
		mainSuccessors: []string{"spec/successor"},
		pairs: []objSupersedeSupersession{
			osPair(osFeatureDecision, "spec/successor#dc-1", "spec/successor#dc-1", osConflictFeature),
			osPair(osStoryCriterion, "spec/successor#dc-2", "spec/successor#dc-2", osConflictStory),
		},
	},
	"chain": {
		designBranch: "design/successor-v3", successor: "spec/successor-v3", mainSpec: "spec/successor-v3",
		mainSuccessors: []string{"spec/successor", "spec/successor-v2", "spec/successor-v3"},
		pairs: []objSupersedeSupersession{
			osPair(osFeatureDecision, "spec/successor-v3#dc-1", "spec/successor#dc-1", osConflictFeature),
			osPair(osStoryCriterion, "spec/successor-v3#dc-2", "spec/successor#dc-2", osConflictStory),
		},
	},
	"chain-drop": {
		designBranch: "design/successor-v2", successor: "spec/successor-v2", mainSpec: "spec/successor-v2",
		mainSuccessors: []string{"spec/successor", "spec/successor-v2"},
		pairs: []objSupersedeSupersession{
			osPair(osFeatureDecision, "", "spec/successor#dc-1", osConflictFeature),
			osPair(osStoryCriterion, "spec/successor-v2#dc-2", "spec/successor#dc-2", osConflictStory),
		},
	},
	"proposed": {
		designBranch: "design/successor", successor: "spec/successor",
		pairs: []objSupersedeSupersession{
			osPair(osFeatureDecision, "spec/successor#dc-1", "spec/successor#dc-1", osConflictFeature),
			osPair(osStoryCriterion, "spec/successor#dc-2", "spec/successor#dc-2", osConflictStory),
		},
	},
	"no-conflict": {
		designBranch: "design/successor", successor: "spec/successor",
		pairs: []objSupersedeSupersession{
			osPair(osFeatureDecision, "spec/successor#dc-1", "spec/successor#dc-1", ""),
			osPair(osStoryCriterion, "spec/successor#dc-2", "spec/successor#dc-2", osConflictStory),
		},
	},
	"chain-not-in-force": {
		designBranch: "design/successor-v2", successor: "spec/successor-v2", mainSpec: "spec/successor",
		mainSuccessors: []string{"spec/successor"},
		pairs: []objSupersedeSupersession{
			osPair(osFeatureCriterion, "spec/successor-v2#dc-3", "spec/successor#dc-3", ""),
			osPair(osFeatureDecision, "spec/successor-v2#dc-1", "spec/successor#dc-1", osConflictFeature),
			osPair(osStoryCriterion, "spec/successor-v2#dc-2", "spec/successor#dc-2", osConflictStory),
		},
	},
}

// objSupersedeStoreInfo is one store's JSON (the file doc's S).
type objSupersedeStoreInfo struct {
	Scenario              string                     `json:"scenario"`
	URL                   string                     `json:"url"`
	DocsURL               string                     `json:"docs_url"`
	DocsCommit            string                     `json:"docs_commit"`
	Checkout              string                     `json:"checkout"`
	MainBranch            string                     `json:"main_branch"`
	DesignBranch          string                     `json:"design_branch"`
	Successor             string                     `json:"successor"`
	EstablishingSuccessor string                     `json:"establishing_successor"`
	Conflicts             []string                   `json:"conflicts"`
	Supersessions         []objSupersedeSupersession `json:"supersessions"`
	Boards                objSupersedeBoards         `json:"boards"`
	Docs                  map[string]string          `json:"docs"`
}

// objSupersedeSupersession is one decision-to-object supersedes edge.
type objSupersedeSupersession struct {
	Object               string `json:"object"`
	ObjectDocsURL        string `json:"object_docs_url"`
	Decision             string `json:"decision"`
	EstablishingDecision string `json:"establishing_decision"`
	Conflict             string `json:"conflict"`
}

// objSupersedeBoards is a store's three board views.
type objSupersedeBoards struct {
	Checkout objSupersedeBoard `json:"checkout"`
	Design   objSupersedeBoard `json:"design"`
	Main     objSupersedeBoard `json:"main"`
}

// objSupersedeBoard is one board view: a URL, or why there is none.
type objSupersedeBoard struct {
	Branch      string `json:"branch"`
	Spec        string `json:"spec"`
	URL         string `json:"url"`
	NotASurface string `json:"not_a_surface"`
}

// objSupersedeFixtureInfo is the endpoint's whole JSON body: every store,
// keyed by scenario name.
type objSupersedeFixtureInfo struct {
	Stores map[string]objSupersedeStoreInfo `json:"stores"`
}

// newObjSupersedeStoreInfo assembles store name's JSON from its facts, its
// manifest checkout and default branch, main's commit, and the two base
// URLs — every view URL composed here, so no consumer composes one.
func newObjSupersedeStoreInfo(name, checkout, mainBranch, mainCommit, serveURL, docsURL string) (objSupersedeStoreInfo, error) {
	facts, ok := objSupersedeFactsByStore[name]
	if !ok {
		return objSupersedeStoreInfo{}, fmt.Errorf("objsupersede fixture: store %q has no recorded facts", name)
	}
	board := func(branch, spec string) objSupersedeBoard {
		slug := strings.TrimPrefix(spec, "spec/")
		if branch == checkout {
			return objSupersedeBoard{Branch: branch, Spec: spec, URL: serveURL + "board/spec/" + slug}
		}
		return objSupersedeBoard{Branch: branch, Spec: spec, URL: serveURL + "b/" + url.PathEscape(branch) + "/board/spec/" + slug}
	}
	document := func(ref string) string { return docsURL + "a/" + ref + "/document/" }

	info := objSupersedeStoreInfo{
		Scenario: name, URL: serveURL, DocsURL: docsURL, DocsCommit: mainCommit,
		Checkout: checkout, MainBranch: mainBranch, DesignBranch: facts.designBranch,
		Successor: facts.successor, EstablishingSuccessor: osEstablisher,
		Conflicts: []string{}, Supersessions: []objSupersedeSupersession{},
		Boards: objSupersedeBoards{
			Checkout: board(checkout, facts.successor),
			Design:   board(facts.designBranch, facts.successor),
			Main:     objSupersedeBoard{Branch: mainBranch, NotASurface: objSupersedeNoMainBoard},
		},
		Docs: map[string]string{osClosedFeature: document(osClosedFeature), osClosedStory: document(osClosedStory)},
	}
	if facts.mainSpec != "" {
		info.Boards.Main = board(mainBranch, facts.mainSpec)
	}
	for _, ref := range facts.mainSuccessors {
		info.Docs[ref] = document(ref)
	}
	conflicts := map[string]bool{}
	for _, p := range facts.pairs {
		spec, id, _ := strings.Cut(p.Object, "#")
		p.ObjectDocsURL = document(spec) + "#" + id
		info.Supersessions = append(info.Supersessions, p)
		if p.Conflict != "" && !conflicts[p.Conflict] {
			conflicts[p.Conflict] = true
			info.Conflicts = append(info.Conflicts, p.Conflict)
		}
	}
	sort.Strings(info.Conflicts)
	return info, nil
}

// objSupersedeStore is one started store: its JSON, the repository its
// serve checks out, the detached main checkout its docs site was built
// from, and the two handles stop reaps.
type objSupersedeStore struct {
	info     objSupersedeStoreInfo
	root     string
	docsRoot string
	serve    *objSupersedeProc
	site     *objSupersedeSite
}

// stop reaps the store's serve and closes its docs site. Nil-safe and
// idempotent, like both handles.
func (s *objSupersedeStore) stop() {
	s.serve.stop()
	s.site.stop()
}

// objSupersedeRun is one successful start: every store, and the scratch
// directory that holds them.
type objSupersedeRun struct {
	scratch string
	stores  map[string]*objSupersedeStore
}

// objSupersedeSteps are startAll's steps. A nil field is the real step;
// tests substitute one to fail a chosen store, or wrap one to observe what
// started, and so prove that a partial failure reaps everything.
type objSupersedeSteps struct {
	buildBinary func(ctx context.Context, moduleRoot, out string) error
	materialize func(ctx context.Context, fixtureDir, repoDir, name string) (*scenario.Repo, error)
	buildSite   func(ctx context.Context, opts dex.Options) error
	serveSite   func(dir string) (*objSupersedeSite, error)
	startServe  func(ctx context.Context, binPath, root string) (*objSupersedeProc, error)
}

// withDefaults fills every unset step with the real one.
func (s objSupersedeSteps) withDefaults() objSupersedeSteps {
	if s.buildBinary == nil {
		s.buildBinary = buildBinary
	}
	if s.materialize == nil {
		s.materialize = scenario.Materialize
	}
	if s.buildSite == nil {
		s.buildSite = dex.Build
	}
	if s.serveSite == nil {
		s.serveSite = func(dir string) (*objSupersedeSite, error) { return serveObjSupersedeSite(objSupersedeLoopback, dir) }
	}
	if s.startServe == nil {
		s.startServe = startObjSupersedeServe
	}
	return s
}

// objSupersedeFixture lazily provisions every store and serves it — the
// same start-once cache shape as the other subprocess fixtures. The zero
// value is usable.
type objSupersedeFixture struct {
	moduleRoot string
	// names are the stores to provision; nil means objSupersedeStores.
	names []string
	// tmpRoot is where the run's scratch directory is made; "" means the
	// system temporary directory.
	tmpRoot string
	// steps are startAll's steps (objSupersedeSteps).
	steps objSupersedeSteps

	// start performs the whole start (startAll). Tests substitute a fake
	// to pin the handler's lazy, start-once contract without subprocesses.
	start func(ctx context.Context) (*objSupersedeRun, error)

	mu  sync.Mutex
	run *objSupersedeRun
}

func newObjSupersedeFixture(moduleRoot string) *objSupersedeFixture {
	f := &objSupersedeFixture{moduleRoot: moduleRoot}
	f.start = f.startAll
	return f
}

// handler answers GET with the fixture's JSON body (objSupersedeFixtureInfo),
// provisioning and starting every store on the first call.
func (f *objSupersedeFixture) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	info, err := f.ensureStarted(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

// ensureStarted runs start once and returns the same info on every call
// thereafter, unchanged. A failed start caches nothing, so the next call
// retries. ctx bounds provisioning, the build, the docs builds, and every
// store's readiness wait — the request's own lifetime; the servers
// themselves live until stop().
func (f *objSupersedeFixture) ensureStarted(ctx context.Context) (objSupersedeFixtureInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.run != nil {
		return f.infoLocked(), nil
	}
	if f.start == nil {
		f.start = f.startAll
	}
	run, err := f.start(ctx)
	if err != nil {
		return objSupersedeFixtureInfo{}, err
	}
	if run == nil || len(run.stores) == 0 {
		return objSupersedeFixtureInfo{}, errors.New("objsupersede fixture: start returned no stores")
	}
	f.run = run
	return f.infoLocked(), nil
}

// infoLocked assembles the JSON body from the started stores. Callers
// hold f.mu.
func (f *objSupersedeFixture) infoLocked() objSupersedeFixtureInfo {
	info := objSupersedeFixtureInfo{Stores: make(map[string]objSupersedeStoreInfo, len(f.run.stores))}
	for name, s := range f.run.stores {
		info.Stores[name] = s.info
	}
	return info
}

// stop reaps every store's serve (SIGTERM, then the WaitDelay force-kill,
// waiting for each), closes every docs site, and removes the run's
// scratch. Safe when never started, and idempotent.
func (f *objSupersedeFixture) stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.run == nil {
		return
	}
	stopObjSupersedeStores(f.run.stores)
	if f.run.scratch != "" {
		_ = os.RemoveAll(f.run.scratch)
		f.run.scratch = ""
	}
}

// stopObjSupersedeStores reaps every store in stores.
func stopObjSupersedeStores(stores map[string]*objSupersedeStore) {
	for _, s := range stores {
		s.stop()
	}
}

// startAll loads the scenario manifest from moduleRoot, builds the verdi
// binary once, and starts every store (startObjSupersedeStore) in order.
// On any failure it reaps every store it already started, removes its
// scratch, and returns the error.
func (f *objSupersedeFixture) startAll(ctx context.Context) (_ *objSupersedeRun, err error) {
	steps := f.steps.withDefaults()
	names := f.names
	if names == nil {
		names = objSupersedeStores
	}
	fixtureDir := filepath.Join(f.moduleRoot, "testdata", "objsupersede")
	m, err := scenario.Load(fixtureDir)
	if err != nil {
		return nil, fmt.Errorf("loading the objsupersede scenario manifest: %w", err)
	}

	scratch, err := os.MkdirTemp(f.tmpRoot, "verdi-e2e-objsupersede-*")
	if err != nil {
		return nil, err
	}
	run := &objSupersedeRun{scratch: scratch, stores: make(map[string]*objSupersedeStore, len(names))}
	defer func() {
		if err != nil {
			stopObjSupersedeStores(run.stores)
			_ = os.RemoveAll(scratch)
		}
	}()

	binPath := filepath.Join(scratch, "verdi")
	if err := steps.buildBinary(ctx, f.moduleRoot, binPath); err != nil {
		return nil, fmt.Errorf("building verdi binary for the objsupersede fixture: %w", err)
	}
	for _, name := range names {
		s, err := startObjSupersedeStore(ctx, steps, m, fixtureDir, binPath, filepath.Join(scratch, name), name)
		if err != nil {
			return nil, err
		}
		run.stores[name] = s
	}
	return run, nil
}

// startObjSupersedeStore provisions store name in dir: the materialized
// repository (dir/repo) with its data zone excluded, a detached checkout
// of main (dir/main), the docs site built from it at main's own commit
// (dir/site) and served, and the store's serve. On failure it reaps
// whatever of its own it started.
func startObjSupersedeStore(ctx context.Context, steps objSupersedeSteps, m *scenario.Manifest, fixtureDir, binPath, dir, name string) (_ *objSupersedeStore, err error) {
	sc, ok := m.Scenarios[name]
	if !ok {
		return nil, fmt.Errorf("objsupersede fixture: scenario %q is not defined in the manifest", name)
	}
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		return nil, err
	}
	if _, err := steps.materialize(ctx, fixtureDir, repo, name); err != nil {
		return nil, fmt.Errorf("materializing objsupersede scenario %q: %w", name, err)
	}
	if err := excludeObjSupersedeDataZone(ctx, repo); err != nil {
		return nil, fmt.Errorf("excluding the data zone of objsupersede scenario %q: %w", name, err)
	}
	mainBranch := m.Commit.InitialBranch
	mainCommit, err := gitOutput(ctx, repo, "rev-parse", "--verify", "refs/heads/"+mainBranch+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("resolving %s of objsupersede scenario %q: %w", mainBranch, name, err)
	}
	docsRoot := filepath.Join(dir, "main")
	if err := runGit(ctx, repo, nil, "worktree", "add", "--detach", "--quiet", docsRoot, mainCommit); err != nil {
		return nil, fmt.Errorf("checking out %s of objsupersede scenario %q: %w", mainBranch, name, err)
	}
	siteDir := filepath.Join(dir, "site")
	if err := steps.buildSite(ctx, dex.Options{Root: docsRoot, OutDir: siteDir, Commit: mainCommit, DefaultBranch: mainBranch}); err != nil {
		return nil, fmt.Errorf("building the docs site for objsupersede scenario %q: %w", name, err)
	}

	s := &objSupersedeStore{root: repo, docsRoot: docsRoot}
	defer func() {
		if err != nil {
			s.stop()
		}
	}()
	if s.site, err = steps.serveSite(siteDir); err != nil {
		return nil, fmt.Errorf("serving the docs site for objsupersede scenario %q: %w", name, err)
	}
	if s.serve, err = steps.startServe(ctx, binPath, repo); err != nil {
		return nil, fmt.Errorf("starting verdi serve for objsupersede scenario %q: %w", name, err)
	}
	if s.info, err = newObjSupersedeStoreInfo(name, sc.Checkout, mainBranch, mainCommit, s.serve.url, s.site.url); err != nil {
		return nil, err
	}
	return s, nil
}

// excludeObjSupersedeDataZone appends the data zone to repo's
// .git/info/exclude — the repository's own, uncommitted ignore file,
// shared by its linked worktrees — so the serve's runtime state never
// reads as uncommitted changes and no commit (so no SHA) changes.
func excludeObjSupersedeDataZone(ctx context.Context, repo string) error {
	exclude, err := gitOutput(ctx, repo, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(exclude) {
		exclude = filepath.Join(repo, exclude)
	}
	if err := os.MkdirAll(filepath.Dir(exclude), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString("\n" + objSupersedeDataZone + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// objSupersedeSite is one store's docs site file server.
type objSupersedeSite struct {
	url string
	srv *http.Server
}

// objSupersedeLoopback is where every docs site listens: an ephemeral
// loopback port.
const objSupersedeLoopback = "127.0.0.1:0"

// serveObjSupersedeSite serves the built site in dir on addr (production:
// objSupersedeLoopback). The listener is bound before this returns, so the
// site answers at once; a bind failure is an error, with nothing started.
func serveObjSupersedeSite(addr, dir string) (*objSupersedeSite, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Handler: http.FileServer(http.Dir(dir)), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return &objSupersedeSite{url: "http://" + ln.Addr().String() + "/", srv: srv}, nil
}

// stop closes the site's listener and connections. Nil-safe and idempotent.
func (s *objSupersedeSite) stop() {
	if s == nil || s.srv == nil {
		return
	}
	_ = s.srv.Close()
	s.srv = nil
}

// objSupersedeProc is one store's `verdi serve` subprocess.
type objSupersedeProc struct {
	url    string
	pid    int
	cancel context.CancelFunc
	// exited is closed once the process has been waited for; err is
	// cmd.Wait's result, read only after exited is closed.
	exited chan struct{}
	err    error
}

// stop terminates the subprocess (SIGTERM, then the WaitDelay force-kill)
// and waits for it. Nil-safe and idempotent.
func (p *objSupersedeProc) stop() {
	if p == nil || p.cancel == nil {
		return
	}
	p.cancel()
	<-p.exited
	p.cancel = nil
}

// startObjSupersedeServe starts one isolated `verdi serve` subprocess of
// binPath over root, loopback only, and waits for its healthz — the same
// build-then-exec seam as unprovenboard.go and specimportfixture.go. A
// serve that exits before answering fails at once, naming its exit.
func startObjSupersedeServe(ctx context.Context, binPath, root string) (*objSupersedeProc, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	// The subprocess outlives this request: its context is the fixture's
	// own, cancelled by stop() — SIGTERM first (main.go's exact posture),
	// then the stdlib's force-kill after WaitDelay.
	childCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(childCtx, binPath, "serve", "--http", addr)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = root
	cmd.Env = hermeticServeEnv(os.Environ())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	p := &objSupersedeProc{url: "http://" + addr + "/", pid: cmd.Process.Pid, cancel: cancel, exited: make(chan struct{})}
	go func() {
		p.err = cmd.Wait()
		close(p.exited)
	}()

	waitCtx, stopWaiting := context.WithCancel(ctx)
	defer stopWaiting()
	go func() {
		select {
		case <-p.exited:
			stopWaiting()
		case <-waitCtx.Done():
		}
	}()
	if err := waitHealthy(waitCtx, p.url+"healthz", 20*time.Second); err != nil {
		exitedEarly := false
		select {
		case <-p.exited:
			exitedEarly = true
		default:
		}
		p.stop()
		if exitedEarly {
			if p.err == nil {
				return nil, errors.New("verdi serve exited before answering healthz (status 0)")
			}
			return nil, fmt.Errorf("verdi serve exited before answering healthz: %w", p.err)
		}
		return nil, fmt.Errorf("waiting for healthz: %w", err)
	}
	return p, nil
}
