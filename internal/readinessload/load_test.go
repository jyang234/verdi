// load_test.go is internal/readinessload's own RED matrix (Task 2,
// spec/readiness-recovery ac-2/ac-3). It carries every moved pin from
// cmd/verdi's old readiness_snapshot_test.go/
// readiness_snapshot_integration_test.go (deleted — their tests move here,
// re-homed on this package's own fixturegit recipes, per the task brief)
// plus the wave-1 plan's new cases: any branch (no design-branch gate),
// the optional context request, and the cache-only judge posture. package
// readinessload (white-box) so hermetic tests can construct the loader{}
// seam directly — the same technique cmd/verdi's old
// localReadinessSnapshotBuilder{...} tests used.
package readinessload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/store"
)

func areaByID(snap readinesspilot.Snapshot, id readinesspilot.AreaID) readinesspilot.Area {
	for _, area := range snap.Areas {
		if area.ID == id {
			return area
		}
	}
	return readinesspilot.Area{}
}

func concernByID(t *testing.T, snap readinesspilot.Snapshot, id string) readinesspilot.Concern {
	t.Helper()
	for _, concern := range snap.AllConcerns {
		if concern.ID == id {
			return concern
		}
	}
	t.Fatalf("snapshot has no concern %q: %+v", id, snap.AllConcerns)
	return readinesspilot.Concern{}
}

// theSemanticConcern returns the one context/semantic/<id> row a fixture
// with exactly one semantic evaluation carries. Its id is a digest
// (semanticInputDigest) computed over the real resolved claims, never a
// fixed literal string, so callers look it up by prefix rather than by an
// exact id.
func theSemanticConcern(t *testing.T, snap readinesspilot.Snapshot) readinesspilot.Concern {
	t.Helper()
	var found []readinesspilot.Concern
	for _, concern := range snap.AllConcerns {
		if strings.HasPrefix(concern.ID, "context/semantic/") {
			found = append(found, concern)
		}
	}
	if len(found) != 1 {
		t.Fatalf("semantic concerns = %+v, want exactly 1", found)
	}
	return found[0]
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if strings.Contains(v, want) {
			return true
		}
	}
	return false
}

func assertCLI(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CLI = %q, want exact token vector %q", got, want)
	}
}

func assertBoardDestination(t *testing.T, concern readinesspilot.Concern, want string) {
	t.Helper()
	if concern.Destination.BoardPath != want || len(concern.Destination.CLI) != 0 {
		t.Fatalf("concern %q destination = %+v, want board path %q and no CLI", concern.ID, concern.Destination, want)
	}
}

func testDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// TestJudgeMode_Normalize directly pins Options' single most security-
// relevant line (fix round 1, Minor 10): only the exact "run" spelling
// enables process execution. The zero value and any garbage/near-miss
// spelling both fail closed to JudgeCacheOnly — process execution can never
// be enabled by omission or a struct-literal typo.
func TestJudgeMode_Normalize(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode JudgeMode
		want JudgeMode
	}{
		{name: "zero value", mode: "", want: JudgeCacheOnly},
		{name: "exact JudgeRun spelling", mode: JudgeRun, want: JudgeRun},
		{name: "near-miss garbage", mode: JudgeMode("RUN"), want: JudgeCacheOnly},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.mode.normalize(); got != tc.want {
				t.Fatalf("JudgeMode(%q).normalize() = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

// --- R-RR1-5: the optional context request ---------------------------------

// TestLoad_AnyBranchNoRequest is the brief's own verbatim case (ac-2/ac-3):
// Load derives readiness for any active spec on any branch — the fixture is
// checked out on main, never a design/<name> branch, so this alone proves
// ac-2's gate removal — and with no --context-request, the check-context
// area carries exactly the fixed unproven witness and CLI destination
// (R-RR1-5), RequestDigest is the digest of zero bytes, and a second
// derivation is byte-identical (determinism).
func TestLoad_AnyBranchNoRequest(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	snap, err := Load(context.Background(), repo.Dir, ref, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Branch != "main" || snap.Head != repo.Head || snap.StaleNotice != "Derived at HEAD "+repo.Head+" for this request." {
		t.Fatalf("snapshot identity = %+v", snap)
	}
	ctxArea := areaByID(snap, readinesspilot.AreaContext)
	if ctxArea.State != readinesspilot.StateUnproven {
		t.Fatalf("context area = %+v", ctxArea)
	}
	v := concernByID(t, snap, "context/verdict")
	if v.State != readinesspilot.StateUnproven || !contains(v.Witnesses, "no context request supplied for this derivation") || v.Destination.CLI[0] != "verdi" || v.Destination.CLI[2] != "conflict" {
		t.Fatalf("context/verdict = %+v", v)
	}
	if snap.RequestDigest != testDigest(nil) {
		t.Fatalf("request digest = %s", snap.RequestDigest)
	}
	// Determinism: a second derivation is byte-identical.
	again, err := Load(context.Background(), repo.Dir, ref, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snap, again) {
		t.Fatal("derivation is not deterministic")
	}
}

// TestLoad_WithRequestCacheMissNeverRunsJudge proves ac-3's core guarantee:
// a request never launches the judge. align.judge_cmd is configured to a
// script that fails the test if it is ever executed; with no cache entry,
// Load must still return a valid snapshot whose semantic concern is
// unproven/judge-unavailable, and .verdi/data must gain no file at all
// (not even a cache miss leaves a trace).
func TestLoad_WithRequestCacheMissNeverRunsJudge(t *testing.T) {
	repo := buildCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": featureAlphaSpec(t),
	})
	marker := filepath.Join(t.TempDir(), "judge-ran")
	judge := writeJudgeScript(t, "touch "+marker+" && printf '%s\\n' '"+noConflictJudgeResult+"'")
	configureJudge(t, repo, judge)
	checkoutBranch(t, repo.Dir, "design/feature-alpha")
	requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign))

	before := snapshotDataTree(t, repo.Dir)
	snap, err := Load(context.Background(), repo.Dir, "spec/feature-alpha", Options{ContextRequestPath: requestPath})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("judge script ran (marker present): %v", err)
	}
	semantic := theSemanticConcern(t, snap)
	if semantic.State != readinesspilot.StateUnproven || !contains(semantic.Witnesses, "judge-unavailable") {
		t.Fatalf("semantic concern = %+v, want unproven judge-unavailable", semantic)
	}
	assertNoPersistence(t, repo.Dir, before)
}

// fakeJudgeRunner is a hermetic, deterministic policyconflict.JudgeRunner:
// every call is recorded, and fn decides the response — no real process, no
// network. internal/policyconflict's own same-named, same-shaped test
// helper is unexported and unreachable from here; this is this package's
// own small instance of the same pattern (fix round 1, Important 2: the
// pattern the dispatch named explicitly, not a copy of policyconflict's
// code).
type fakeJudgeRunner struct {
	calls int
	fn    func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error)
}

func (f *fakeJudgeRunner) Run(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
	f.calls++
	return f.fn(ctx, argv, stdin)
}

// TestLoad_WithRequestCacheHit proves the converse of
// TestLoad_WithRequestCacheMissNeverRunsJudge: a judgment the D4 cache
// already holds is used without ever running the judge a second time. It
// warms the cache through an in-process JudgeAdapter evaluation backed by
// fakeJudgeRunner (fix round 1, Important 2 — no subprocess), built the
// same way readinessload.NewConflictProvider itself builds a JudgeRun
// service (this package's own conflictTreeHasher/conflictDateSource/
// conflictRefResolver, same package, reachable), then confirms a second
// Load — whose provider wraps the identical adapter identity in
// NewCacheOnlyJudge, mirroring NewConflictProvider's own JudgeCacheOnly
// wiring — reflects the same semantic state without a second runner call.
func TestLoad_WithRequestCacheHit(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	checkoutBranch(t, repo.Dir, "design/feature-alpha")
	requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))

	runner := &fakeJudgeRunner{fn: func(context.Context, []string, []byte) ([]byte, int, error) {
		// DecodeJudgeResult requires the exact canonical re-encoding of
		// whatever it decodes (authority design §6's strict-decode rule), so
		// the fake stdout is built through EncodeJudgeResult itself — never a
		// hand-typed string literal that might drift from canonical form.
		out, err := policyconflict.EncodeJudgeResult(policyconflict.JudgeResult{
			Schema: policyconflict.JudgeResultSchema, Recommendation: policyconflict.RecommendationNoConflict, Findings: []policyconflict.JudgeFinding{},
		})
		if err != nil {
			return nil, 0, err
		}
		return out, 0, nil
	}}
	adapterFor := func(request policyconflict.Request) policyconflict.JudgeAdapter {
		return policyconflict.JudgeAdapter{
			Role: string(policyconflict.JudgePrimary), Adapter: requestAdapter(request), Model: "fake-judge",
			Argv: []string{"fake-judge"}, Root: repo.Dir, Runner: runner,
		}
	}
	serviceFor := func(primary policyconflict.Judge) ConflictProviderFunc {
		return func(_ context.Context, root string, _ policyconflict.Request) (policyconflict.VerdictProvider, error) {
			return policyconflict.NewService(root, policyconflict.ServiceDeps{
				Compiler: contextcompile.NewCompiler(), Refs: conflictRefResolver{},
				Primary: primary, TreeHasher: conflictTreeHasher{}, Dates: conflictDateSource{},
			}), nil
		}
	}

	// Warm-up: a plain JudgeAdapter — CachedJudge's own miss path (reached
	// via service.go's case JudgeAdapter: arm) runs the process and
	// publishes the result to the D4 cache, mirroring what
	// NewConflictProvider builds under JudgeRun.
	warmProvider := func(ctx context.Context, root string, request policyconflict.Request) (policyconflict.VerdictProvider, error) {
		return serviceFor(adapterFor(request))(ctx, root, request)
	}
	warm, err := Load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath, ConflictProvider: warmProvider})
	if err != nil {
		t.Fatalf("warm Load: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls after warm-up = %d, want 1", runner.calls)
	}

	// Cache-only: NewCacheOnlyJudge wrapping the identical adapter identity
	// — mirroring what NewConflictProvider builds under JudgeCacheOnly.
	cacheOnlyProvider := func(ctx context.Context, root string, request policyconflict.Request) (policyconflict.VerdictProvider, error) {
		return serviceFor(policyconflict.NewCacheOnlyJudge(adapterFor(request)))(ctx, root, request)
	}
	cached, err := Load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath, ConflictProvider: cacheOnlyProvider})
	if err != nil {
		t.Fatalf("cache-only Load: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls after cache-only Load = %d, want still 1 (found the warm-up's cache entry)", runner.calls)
	}

	warmSemantic := theSemanticConcern(t, warm)
	cachedSemantic := theSemanticConcern(t, cached)
	if cachedSemantic.State != warmSemantic.State {
		t.Fatalf("cache-only semantic state = %q, want the warmed run's %q", cachedSemantic.State, warmSemantic.State)
	}
}

// --- operational refusals ---------------------------------------------------

func TestLoad_RefusesPinnedOrFragmentRef(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	for _, tc := range []struct {
		name string
		ref  string
	}{
		{name: "pinned", ref: ref + "@" + strings.Repeat("a", 40)},
		{name: "fragment", ref: ref + "#ac-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(context.Background(), repo.Dir, tc.ref, Options{})
			if err == nil || !strings.Contains(err.Error(), "unpinned whole spec ref") {
				t.Fatalf("Load error = %v, want unpinned-whole-spec-ref refusal", err)
			}
		})
	}
}

func TestLoad_ArchivedSpecIsOperational(t *testing.T) {
	repo := buildCompileRepo(t, map[string]string{
		".verdi/specs/archived/feature-alpha/spec.md": featureAlphaSpec(t),
	})
	_, err := Load(context.Background(), repo.Dir, "spec/feature-alpha", Options{})
	if err == nil || !strings.Contains(err.Error(), "resolves to no spec") {
		t.Fatalf("Load error = %v, want journey's own operational refusal for an archived-only target", err)
	}
}

// componentAlphaSpec is a real, minimal, valid `class: component` spec
// (internal/artifact's own component grammar carries no problem/outcome/
// acceptance-criteria/open-questions/stubs at all — 02: "no object model"
// — an entirely different shape from feature/story, so this is authored
// fresh rather than patched from the feature-alpha fixture).
const componentAlphaSpec = `---
id: spec/component-alpha
kind: spec
title: "Component Alpha"
owners: [alpha-team]
class: component
status: draft
---
# Component Alpha
`

// TestLoad_RefusesClassOutsideFeatureOrStory proves the loader's own
// feature-or-story gate over a real, valid, journey-legal "component" spec
// — a real artifact.SpecClass both artifact.DecodeSpec and
// journey.GatherFacts accept on their own terms. gatherFacts is replaced
// with a minimal fake reporting the resolved class/path/repository facts
// journey.GatherFacts would (skipping its own, unrelated component-lifecycle
// machinery this test has no reason to exercise), so the loader's own
// decoded-identity cross-check passes and only the feature-or-story gate
// itself can refuse.
func TestLoad_RefusesClassOutsideFeatureOrStory(t *testing.T) {
	repo := buildCompileRepo(t, map[string]string{
		".verdi/specs/active/component-alpha/spec.md": componentAlphaSpec,
	})
	ref := "spec/component-alpha"
	relPath := store.ActiveSpecRelPath("component-alpha")
	l := loader{
		gatherFacts: func(context.Context, *store.Config, string) (journey.Facts, error) {
			return journey.Facts{
				Target: journey.Target{Ref: ref, Class: "component", Path: relPath},
				Repository: journey.RepositoryFacts{
					Branch: journey.StringFact{Known: true, Value: "main"},
					Head:   journey.StringFact{Known: true, Value: repo.Head},
				},
			}, nil
		},
	}
	_, err := l.load(context.Background(), repo.Dir, ref, Options{})
	if err == nil || !strings.Contains(err.Error(), "not feature or story") {
		t.Fatalf("load error = %v, want a feature-or-story class refusal", err)
	}
}

// --- moved pin: exactly-one request read + its digest -----------------------

func TestLoad_RequestReadExactlyOnce(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	requestData := requestBytes(t, ref, contextcompile.PhaseDesign)
	requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestData)

	reads := 0
	l := loader{readFile: func(path string) ([]byte, error) {
		if path == requestPath {
			reads++
		}
		return os.ReadFile(path)
	}}
	snap, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if reads != 1 {
		t.Fatalf("request reads = %d, want exactly 1", reads)
	}
	if snap.RequestDigest != testDigest(requestData) {
		t.Fatalf("RequestDigest = %q, want digest of the exact canonical request bytes %q", snap.RequestDigest, testDigest(requestData))
	}
}

// --- moved pin: ".."/symlink refusal before any read ------------------------

func TestLoad_RefusesDotDotAndSymlinkBeforeAnyRead(t *testing.T) {
	root := t.TempDir()
	requestData := requestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign)
	realPath := filepath.Join(root, "request.json")
	if err := os.WriteFile(realPath, requestData, 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(root, "request-link.json")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{name: "dot-dot", path: root + string(filepath.Separator) + "nested" + string(filepath.Separator) + ".." + string(filepath.Separator) + "request.json", want: `".."`},
		{name: "symlink", path: linkPath, want: "symlink"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := 0
			l := loader{readFile: func(string) ([]byte, error) {
				reads++
				return nil, errors.New("must not read")
			}}
			_, err := l.load(context.Background(), root, "spec/feature-alpha", Options{ContextRequestPath: tc.path})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("load error = %v, want %q refusal", err, tc.want)
			}
			if reads != 0 {
				t.Fatalf("read calls = %d after path refusal, want 0", reads)
			}
		})
	}
}

// --- moved pin: design-phase requirement ------------------------------------

func TestLoad_RequiresDesignPhase(t *testing.T) {
	root := t.TempDir()
	requestPath := filepath.Join(root, "build-request.json")
	if err := os.WriteFile(requestPath, requestBytes(t, "spec/feature-alpha", contextcompile.PhaseBuild), 0o644); err != nil {
		t.Fatal(err)
	}
	called := false
	l := loader{gatherFacts: func(context.Context, *store.Config, string) (journey.Facts, error) {
		called = true
		return journey.Facts{}, nil
	}}
	_, err := l.load(context.Background(), root, "spec/feature-alpha", Options{ContextRequestPath: requestPath})
	if err == nil || !strings.Contains(err.Error(), `phase "design"`) {
		t.Fatalf("load error = %v, want design-phase refusal", err)
	}
	// The request is validated, read, and decoded far enough to check its
	// phase BEFORE store.Open/gatherFacts ever runs (the same "refuse
	// before any read" ordering the ".."/symlink pin requires), so
	// gatherFacts must not be called for a non-design request.
	if called {
		t.Fatal("repository facts gathered for a non-design request")
	}
}

// --- moved pin: expected identity -------------------------------------------

func TestLoad_BindsAbsentExpectedToCurrentBranchAndHead(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	checkoutBranch(t, repo.Dir, "design/feature-alpha")
	requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))

	var captured *policyconflict.AcceptanceCandidate
	l := loader{newConflictProvider: func(_ context.Context, root string, request policyconflict.Request, mode JudgeMode, actors ActorsResolver) (policyconflict.VerdictProvider, error) {
		candidate := request.Target.AcceptanceCandidate
		if candidate != nil {
			cp := *candidate
			captured = &cp
		}
		return providerFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			return fixtureReport(t, root, request, policyconflict.VerdictPass, nil), nil
		}), nil
	}}
	snap, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if captured == nil {
		t.Fatal("provider did not receive an acceptance candidate")
	}
	if captured.Expected.Branch != "design/feature-alpha" || captured.Expected.Head != repo.Head {
		t.Fatalf("candidate expected = %+v, want current branch/HEAD", captured.Expected)
	}
	if captured.Spec != ref || snap.Branch != captured.Expected.Branch || snap.Head != captured.Expected.Head {
		t.Fatalf("candidate/snapshot identity drift: candidate=%+v snapshot=%+v", captured, snap)
	}
}

func TestLoad_RefusesExpectedMismatch(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	checkoutBranch(t, repo.Dir, "design/feature-alpha")
	mismatched := contextcompile.Expected{Branch: "design/other", Head: strings.Repeat("a", 40)}
	requestPath := writeRequestFile(t, repo.Dir, "mismatched-request.json", requestWithExpected(t, ref, &mismatched))

	called := false
	l := loader{newConflictProvider: func(context.Context, string, policyconflict.Request, JudgeMode, ActorsResolver) (policyconflict.VerdictProvider, error) {
		called = true
		return nil, errors.New("must not construct provider")
	}}
	_, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
	if err == nil || !strings.Contains(err.Error(), "expected repository") {
		t.Fatalf("load error = %v, want expected identity mismatch", err)
	}
	if called {
		t.Fatal("provider constructed after expected identity mismatch")
	}
}

// requestWithExpected builds a canonical request carrying an explicit
// Expected claim, via contextcompile.EncodeRequest's own seam.
func requestWithExpected(t *testing.T, spec string, expected *contextcompile.Expected) []byte {
	t.Helper()
	req := contextcompile.Request{
		Schema: contextcompile.RequestSchema, Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Phase: contextcompile.PhaseDesign,
		Scope: policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Spec:  spec, Expected: expected,
	}
	data, err := contextcompile.EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	return data
}

// --- moved pin: identity mismatches -----------------------------------------

func TestLoad_CrossSourceIdentityMismatches(t *testing.T) {
	t.Run("journey target mismatch", func(t *testing.T) {
		repo, ref := readinessRepo(t, "feature")
		checkoutBranch(t, repo.Dir, "design/feature-alpha")
		requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
		l := loader{projectJourney: func(ctx context.Context, cfg *store.Config, arg string, extras journey.Extras) (journey.Record, error) {
			record, err := journey.NewProjector().ProjectWith(ctx, cfg, arg, extras)
			record.Target.Ref = "spec/other"
			return record, err
		}, newConflictProvider: passProviderFactory(t, repo.Dir)}
		_, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
		if err == nil || !strings.Contains(err.Error(), "journey target") {
			t.Fatalf("load error = %v, want journey target mismatch", err)
		}
	})

	t.Run("report target mismatch", func(t *testing.T) {
		repo, ref := readinessRepo(t, "feature")
		checkoutBranch(t, repo.Dir, "design/feature-alpha")
		requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
		l := loader{newConflictProvider: func(_ context.Context, root string, request policyconflict.Request, mode JudgeMode, actors ActorsResolver) (policyconflict.VerdictProvider, error) {
			return providerFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
				result := fixtureReport(t, root, request, policyconflict.VerdictPass, nil)
				result.Report.Input.Target.Candidate.Ref = "spec/other"
				return result, nil
			}), nil
		}}
		_, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
		if err == nil || !strings.Contains(err.Error(), "conflict report target") {
			t.Fatalf("load error = %v, want report target mismatch", err)
		}
	})

	t.Run("decoded spec target mismatch", func(t *testing.T) {
		repo, ref := readinessRepo(t, "feature")
		checkoutBranch(t, repo.Dir, "design/feature-alpha")
		requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
		specPath := store.ActiveSpecPath(repo.Dir, "feature-alpha")
		l := loader{
			readFile: func(path string) ([]byte, error) {
				data, err := os.ReadFile(path)
				if err != nil || path != specPath {
					return data, err
				}
				return []byte(strings.Replace(string(data), "id: spec/feature-alpha", "id: spec/feature-other", 1)), nil
			},
			newConflictProvider: passProviderFactory(t, repo.Dir),
		}
		_, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
		if err == nil || !strings.Contains(err.Error(), "decoded spec") {
			t.Fatalf("load error = %v, want decoded spec mismatch", err)
		}
	})
}

// --- moved: feature/story targets -------------------------------------------

func TestLoad_FeatureAndStoryTargets(t *testing.T) {
	for _, tc := range []struct {
		class string
		title string
	}{
		{class: "feature", title: "Feature Alpha"},
		{class: "story", title: "Borrower appeal: exact source title"},
	} {
		t.Run(tc.class, func(t *testing.T) {
			repo, ref := readinessRepo(t, tc.class)
			name := strings.TrimPrefix(ref, "spec/")
			checkoutBranch(t, repo.Dir, "design/"+name)
			requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
			snap := mustLoadWithPassProvider(t, repo, ref, requestPath)
			if snap.TargetRef != ref || snap.TargetTitle != tc.title || snap.TargetClass != tc.class || snap.Branch != "design/"+name || snap.Head != repo.Head {
				t.Fatalf("snapshot identity = %+v, want %s %q %s at current branch/%s", snap, ref, tc.title, tc.class, repo.Head)
			}
			if err := snap.Validate(); err != nil {
				t.Fatalf("snapshot Validate: %v", err)
			}
		})
	}
}

func mustLoadWithPassProvider(t *testing.T, repo *fixturegit.Repo, ref, requestPath string) readinesspilot.Snapshot {
	t.Helper()
	l := loader{newConflictProvider: passProviderFactory(t, repo.Dir)}
	snap, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return snap
}

func passProviderFactory(t *testing.T, root string) func(context.Context, string, policyconflict.Request, JudgeMode, ActorsResolver) (policyconflict.VerdictProvider, error) {
	t.Helper()
	return func(_ context.Context, gotRoot string, request policyconflict.Request, mode JudgeMode, actors ActorsResolver) (policyconflict.VerdictProvider, error) {
		if gotRoot != root {
			t.Fatalf("provider root = %q, want %q", gotRoot, root)
		}
		return providerFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			return fixtureReport(t, root, request, policyconflict.VerdictPass, nil), nil
		}), nil
	}
}

type providerFunc func(context.Context, policyconflict.Request) (policyconflict.Result, error)

func (f providerFunc) Evaluate(ctx context.Context, request policyconflict.Request) (policyconflict.Result, error) {
	return f(ctx, request)
}

// fixtureReport builds a self-consistent policyconflict.Result over the
// real internal/policyconflict/testdata/report.json fixture, re-targeted
// at request's own acceptance candidate — mirrors cmd/verdi's old
// readinessSnapshotReport.
func fixtureReport(t *testing.T, root string, request policyconflict.Request, verdict policyconflict.Verdict, mutate func(*policyconflict.Report)) policyconflict.Result {
	t.Helper()
	candidate := request.Target.AcceptanceCandidate
	if request.Target.Kind != policyconflict.TargetAcceptanceCandidate || candidate == nil {
		t.Fatalf("provider request target = %+v, want one acceptance candidate", request.Target)
	}
	ref, err := artifact.ParseRef(candidate.Spec)
	if err != nil {
		t.Fatalf("ParseRef(%q): %v", candidate.Spec, err)
	}
	specPath := store.ActiveSpecPath(root, ref.Name)
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read report target: %v", err)
	}

	fixture, err := os.ReadFile(filepath.Join("..", "policyconflict", "testdata", "report.json"))
	if err != nil {
		t.Fatalf("read report fixture: %v", err)
	}
	report, err := policyconflict.DecodeReport(fixture)
	if err != nil {
		t.Fatalf("DecodeReport fixture: %v", err)
	}
	report.Digest = ""
	report.Input.Target = policyconflict.TargetIdentity{
		Kind: policyconflict.TargetAcceptanceCandidate,
		Candidate: &policyconflict.CandidateIdentity{
			Ref:           candidate.Spec,
			Path:          store.ActiveSpecRelPath(ref.Name),
			Branch:        candidate.Expected.Branch,
			Head:          candidate.Expected.Head,
			Blob:          strings.Repeat("b", 40),
			ContentDigest: testDigest(specBytes),
			Scope:         candidate.Scope,
			Adapter:       candidate.Adapter,
			GrantDigest:   "sha256:" + strings.Repeat("d", 64),
		},
	}
	report.Input.Repository.Branch.Known, report.Input.Repository.Branch.Value = true, candidate.Expected.Branch
	report.Input.Repository.Head.Known, report.Input.Repository.Head.Value = true, candidate.Expected.Head
	switch verdict {
	case policyconflict.VerdictPass:
		report.Semantic = []policyconflict.SemanticEvaluation{}
		report.Verdict = verdict
	case policyconflict.VerdictBlockedUnproven:
		report.Verdict = verdict
	case policyconflict.VerdictBlockedViolated:
		report.Semantic[0].State = policyconflict.ProofViolatedWithWitness
		report.Semantic[0].Reasons = []policyconflict.ReasonCode{policyconflict.ReasonDispositionEffectiveConflict}
		report.Verdict = verdict
	default:
		t.Fatalf("unsupported verdict %q", verdict)
	}
	if mutate != nil {
		mutate(&report)
	}
	encoded, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatalf("EncodeReport: %v", err)
	}
	decoded, err := policyconflict.DecodeReport(encoded)
	if err != nil {
		t.Fatalf("DecodeReport: %v", err)
	}
	return policyconflict.Result{Report: decoded, ReportBytes: encoded}
}

// snapshotDataTree captures every regular file under root's .verdi/data
// tree — its path relative to root, mapped to its size — or an empty map
// when the tree is absent (or empty). Call it immediately before the
// derivation under test runs; assertNoPersistence compares its return
// value against a fresh snapshot taken after.
func snapshotDataTree(t *testing.T, root string) map[string]int64 {
	t.Helper()
	dataRoot := filepath.Join(root, ".verdi", "data")
	files := map[string]int64{}
	err := filepath.WalkDir(dataRoot, func(path string, entry os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, ierr := entry.Info()
		if ierr != nil {
			return ierr
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		files[filepath.ToSlash(rel)] = info.Size()
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("walk .verdi/data: %v", err)
	}
	return files
}

// assertNoPersistence proves co-2 directly (fix round 1, Important 1): the
// full set of regular-file paths and sizes under root's .verdi/data tree is
// unchanged from before, the snapshotDataTree result captured immediately
// before the derivation under test ran. Its predecessor only failed on a
// path whose NAME happened to contain the substring "readiness" — a
// derivation could have written .verdi/data/anything-else.json or a stray
// lock file and that check would still pass; this one does not have that
// gap, because it compares the complete tree, not a name filter.
func assertNoPersistence(t *testing.T, root string, before map[string]int64) {
	t.Helper()
	after := snapshotDataTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf(".verdi/data changed during a derivation that must persist nothing:\nbefore=%v\nafter= %v", before, after)
	}
}

// --- moved: provenance, mutation, and scratch-board postures ---------------

func TestLoad_ProvenanceMutationAndBoard(t *testing.T) {
	t.Run("missing provenance is explicitly unproven", func(t *testing.T) {
		repo, ref := readinessRepo(t, "feature")
		checkoutBranch(t, repo.Dir, "design/feature-alpha")
		requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
		snap := mustLoadWithPassProvider(t, repo, ref, requestPath)
		concern := concernByID(t, snap, "shape/provenance")
		if concern.State != readinesspilot.StateUnproven {
			t.Fatalf("provenance state = %q, want unproven", concern.State)
		}
		if !contains(concern.Witnesses, "sidecar is absent") {
			t.Fatalf("provenance witnesses = %v, want %q", concern.Witnesses, "sidecar is absent")
		}
	})

	t.Run("malformed provenance is operational", func(t *testing.T) {
		repo, ref := readinessRepo(t, "feature")
		checkoutBranch(t, repo.Dir, "design/feature-alpha")
		requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
		path := store.DesignProvenancePath(repo.Dir, store.ZoneActive, "feature-alpha")
		if err := os.WriteFile(path, []byte("not-jsonl\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		l := loader{newConflictProvider: passProviderFactory(t, repo.Dir)}
		_, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
		if err == nil || !strings.Contains(err.Error(), "decoding design provenance") {
			t.Fatalf("load error = %v, want malformed provenance operational error", err)
		}
	})

	t.Run("direct Markdown gap stays unproven", func(t *testing.T) {
		repo, ref := readinessRepo(t, "feature")
		checkoutBranch(t, repo.Dir, "design/feature-alpha")
		requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
		if err := os.WriteFile(store.DesignProvenancePath(repo.Dir, store.ZoneActive, "feature-alpha"), []byte(readinessProvenanceV1Sidecar), 0o644); err != nil {
			t.Fatal(err)
		}
		snap := mustLoadWithPassProvider(t, repo, ref, requestPath)
		concern := concernByID(t, snap, "shape/provenance")
		if concern.State != readinesspilot.StateUnproven {
			t.Fatalf("provenance state = %q, want unproven", concern.State)
		}
		if !contains(concern.Witnesses, "direct Markdown") {
			t.Fatalf("provenance witnesses = %v, want direct-Markdown gap", concern.Witnesses)
		}
	})

	t.Run("mutation residue is disclosed without recovery", func(t *testing.T) {
		repo, ref := readinessRepo(t, "feature")
		checkoutBranch(t, repo.Dir, "design/feature-alpha")
		requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
		journal := store.DraftMutationJournalPath(repo.Dir, "feature-alpha")
		stage := store.DraftMutationSpecStagePath(repo.Dir, "feature-alpha")
		if err := os.MkdirAll(filepath.Dir(journal), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(journal, []byte("leave-unread"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stage, []byte("leave-unread"), 0o644); err != nil {
			t.Fatal(err)
		}
		snap := mustLoadWithPassProvider(t, repo, ref, requestPath)
		concern := concernByID(t, snap, "shape/mutation")
		if concern.State != readinesspilot.StateUnproven {
			t.Fatalf("mutation state = %q, want unproven", concern.State)
		}
		if !contains(concern.Witnesses, "journal") || !contains(concern.Witnesses, "spec stage") {
			t.Fatalf("mutation witnesses = %v, want journal and spec-stage witnesses", concern.Witnesses)
		}
		journalBytes, _ := os.ReadFile(journal)
		stageBytes, _ := os.ReadFile(stage)
		if string(journalBytes) != "leave-unread" || string(stageBytes) != "leave-unread" {
			t.Fatalf("loader recovered or rewrote mutation residue: journal=%q stage=%q", journalBytes, stageBytes)
		}
	})

	t.Run("scratch board open item is unproven", func(t *testing.T) {
		repo, ref := readinessRepo(t, "feature")
		checkoutBranch(t, repo.Dir, "design/feature-alpha")
		requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
		annotation := &artifact.Annotation{
			ID: "a-01ARZ3NDEKTSV4RRFFQ69G5FAV", TS: "2026-08-18T12:00:00Z", Author: "tester",
			Board: &artifact.BoardAnchor{Story: "feature-alpha", X: 1, Y: 2}, Type: artifact.AnnotationQuestion,
			Body: "Which route?", Status: artifact.AnnotationOpen,
		}
		if err := boardio.AppendAnnotation(boardio.AnnotationsDir(repo.Dir), boardio.AnnotationFileForBoard("feature-alpha"), annotation); err != nil {
			t.Fatal(err)
		}
		snap := mustLoadWithPassProvider(t, repo, ref, requestPath)
		concern := concernByID(t, snap, "shape/board/question/"+annotation.ID)
		if concern.State != readinesspilot.StateUnproven {
			t.Fatalf("open board concern = %+v", concern)
		}
	})

	t.Run("board read failure is unproven", func(t *testing.T) {
		repo, ref := readinessRepo(t, "feature")
		checkoutBranch(t, repo.Dir, "design/feature-alpha")
		requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
		l := loader{
			readAnnotations: func(string) ([]*artifact.Annotation, error) {
				return nil, &os.PathError{Op: "open", Path: boardio.AnnotationsDir(repo.Dir), Err: os.ErrPermission}
			},
			newConflictProvider: passProviderFactory(t, repo.Dir),
		}
		snap, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		concern := concernByID(t, snap, "shape/board")
		if concern.State != readinesspilot.StateUnproven {
			t.Fatalf("board state = %q, want unproven", concern.State)
		}
		if !contains(concern.Witnesses, "permission denied") {
			t.Fatalf("board witnesses = %v, want permission-denied witness", concern.Witnesses)
		}
	})

	t.Run("malformed board record is operational", func(t *testing.T) {
		repo, ref := readinessRepo(t, "feature")
		checkoutBranch(t, repo.Dir, "design/feature-alpha")
		requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
		dir := boardio.AnnotationsDir(repo.Dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "board--feature-alpha.jsonl"), []byte("not-json\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		l := loader{newConflictProvider: passProviderFactory(t, repo.Dir)}
		_, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
		if err == nil || !strings.Contains(err.Error(), "reading scratch board annotations") || !strings.Contains(err.Error(), "strict json decode") {
			t.Fatalf("load error = %v, want malformed board record operational error", err)
		}
	})
}

// readinessProvenanceV1Sidecar is one LITERAL historical
// `verdi.design-provenance/v1` sidecar line for `spec/feature-alpha`,
// captured byte-for-byte — moved verbatim from cmd/verdi's old
// readiness_snapshot_integration_test.go.
const readinessProvenanceV1Sidecar = `{"attribution":{"unauthenticated":true},"changes":[{"after_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","before_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","change":"replaced","target":"problem"}],"context":{"reason":"unavailable-before-context-compiler","state":"unavailable"},"digest":"sha256:9b93daccf5d9d75ba2a0d8d20cee8d0ac010c5a40c8898f567b0e527a24f209a","excerpts":[],"harness":"codex","operations":[{"anchor":"problem","op":"set-problem","text":"changed problem"}],"policy_digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","previous_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","result_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","schema":"verdi.design-provenance/v1","spec":"spec/feature-alpha"}
`

// --- moved: conflict verdicts and destinations ------------------------------

func TestLoad_ConflictVerdictsAndDestinations(t *testing.T) {
	tests := []struct {
		verdict policyconflict.Verdict
		state   readinesspilot.State
	}{
		{verdict: policyconflict.VerdictPass, state: readinesspilot.StateProven},
		{verdict: policyconflict.VerdictBlockedViolated, state: readinesspilot.StateViolated},
		{verdict: policyconflict.VerdictBlockedUnproven, state: readinesspilot.StateUnproven},
	}
	for _, tc := range tests {
		t.Run(string(tc.verdict), func(t *testing.T) {
			repo, ref := readinessRepo(t, "feature")
			checkoutBranch(t, repo.Dir, "design/feature-alpha")
			requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
			l := loader{newConflictProvider: func(_ context.Context, root string, request policyconflict.Request, mode JudgeMode, actors ActorsResolver) (policyconflict.VerdictProvider, error) {
				return providerFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
					return fixtureReport(t, root, request, tc.verdict, nil), nil
				}), nil
			}}
			snap, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			concern := concernByID(t, snap, "context/verdict")
			if concern.State != tc.state {
				t.Fatalf("context verdict state = %q, want %q", concern.State, tc.state)
			}
			if tc.state != readinesspilot.StateProven {
				assertCLI(t, concern.Destination.CLI, []string{"verdi", "context", "conflict", "--request", requestPath})
			}
			review := concernByID(t, snap, "review/action")
			assertCLI(t, review.Destination.CLI, []string{"verdi", "journey", ref})
		})
	}
}

// --- moved: persistence boundary --------------------------------------------

func TestLoad_PersistenceBoundary(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	checkoutBranch(t, repo.Dir, "design/feature-alpha")
	requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
	before := snapshotDataTree(t, repo.Dir)
	_ = mustLoadWithPassProvider(t, repo, ref, requestPath)
	assertNoPersistence(t, repo.Dir, before)
}

// --- moved: claimed open questions ------------------------------------------

func TestLoad_ClaimedOpenQuestion(t *testing.T) {
	t.Run("claimed question is non-blocking eventual; unclaimed stays blocking current", func(t *testing.T) {
		repo, ref, requestPath := claimedQuestionFixture(t, "feature-claim", claimedQuestionFeatureSpec)
		snap := mustLoadWithPassProvider(t, repo, ref, requestPath)

		claimed := concernByID(t, snap, "shape/question/oq-2")
		if claimed.State != readinesspilot.StateUnproven || claimed.Blocking || claimed.Timing != readinesspilot.TimingEventual {
			t.Fatalf("claimed concern = %+v, want non-blocking eventual unproven", claimed)
		}
		if !contains(claimed.Witnesses, "retry-strategy-spike") || !contains(claimed.Witnesses, "oq-2") {
			t.Fatalf("claimed witnesses = %v", claimed.Witnesses)
		}

		unclaimed := concernByID(t, snap, "shape/question/oq-1")
		if unclaimed.State != readinesspilot.StateUnproven || !unclaimed.Blocking || unclaimed.Timing != readinesspilot.TimingCurrent {
			t.Fatalf("unclaimed concern = %+v, want blocking current unproven", unclaimed)
		}

		for _, area := range snap.Areas {
			if area.ID == readinesspilot.AreaShape && area.State != readinesspilot.StateUnproven {
				t.Fatalf("shape area state = %q, want unproven (driven by the unclaimed question)", area.State)
			}
		}
	})

	t.Run("shape area is proven when the only open question is claimed", func(t *testing.T) {
		repo, ref, requestPath := claimedQuestionFixture(t, "feature-claim-only", allClaimedFeatureSpec)
		snap := mustLoadWithPassProvider(t, repo, ref, requestPath)

		claimed := concernByID(t, snap, "shape/question/oq-1")
		if claimed.Blocking || claimed.Timing != readinesspilot.TimingEventual {
			t.Fatalf("claimed concern = %+v, want non-blocking eventual", claimed)
		}
		for _, area := range snap.Areas {
			if area.ID == readinesspilot.AreaShape && area.State != readinesspilot.StateProven {
				t.Fatalf("shape area state = %q, want proven: the only open question is claimed", area.State)
			}
		}
	})
}

func claimedQuestionFixture(t *testing.T, specName, specBody string) (*fixturegit.Repo, string, string) {
	t.Helper()
	repo := buildCompileRepo(t, map[string]string{".verdi/specs/active/" + specName + "/spec.md": specBody})
	checkoutBranch(t, repo.Dir, "design/"+specName)
	ref := "spec/" + specName
	requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))
	return repo, ref, requestPath
}

const claimedQuestionFeatureSpec = `---
id: spec/feature-claim
kind: spec
title: "Feature Claim"
owners: [alpha-team]
class: feature
problem: {text: "Feature claim is unclear.", anchor: problem}
outcome: {text: "Feature claim is reviewable.", anchor: outcome}
acceptance_criteria:
  - {id: ac-1, text: "the feature works", evidence: [behavioral], anchor: ac-1}
open_questions:
  - {id: oq-1, text: "which retry strategy applies?", anchor: oq-1}
  - {id: oq-2, text: "which timeout applies?", anchor: oq-2}
stubs:
  - {slug: retry-strategy-spike, spike: true, resolves: [oq-2]}
---
# Feature Claim

## Problem

Feature claim is unclear.

## Outcome

Feature claim is reviewable.

## AC-1

The feature works.

## OQ-1

Which retry strategy applies?

## OQ-2

Which timeout applies?
`

const allClaimedFeatureSpec = `---
id: spec/feature-claim-only
kind: spec
title: "Feature Claim Only"
owners: [alpha-team]
class: feature
problem: {text: "Feature claim only is unclear.", anchor: problem}
outcome: {text: "Feature claim only is reviewable.", anchor: outcome}
acceptance_criteria:
  - {id: ac-1, text: "the feature works", evidence: [behavioral], anchor: ac-1}
open_questions:
  - {id: oq-1, text: "which timeout applies?", anchor: oq-1}
stubs:
  - {slug: timeout-spike, spike: true, resolves: [oq-1]}
---
# Feature Claim Only

## Problem

Feature claim only is unclear.

## Outcome

Feature claim only is reviewable.

## AC-1

The feature works.

## OQ-1

Which timeout applies?
`

// --- R-RR1-6: the shape-destination rewrite ---------------------------------

// TestLoad_ShapeDestinationRewrite proves R-RR1-6: with a BoardHref
// function, every unresolved shape concern routes to the board; with none,
// Derive's own CLI fallback vector stands.
func TestLoad_ShapeDestinationRewrite(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	checkoutBranch(t, repo.Dir, "design/feature-alpha")
	requestPath := writeRequestFile(t, repo.Dir, "readiness-request.json", requestBytes(t, ref, contextcompile.PhaseDesign))

	t.Run("with BoardHref", func(t *testing.T) {
		l := loader{newConflictProvider: passProviderFactory(t, repo.Dir)}
		snap, err := l.load(context.Background(), repo.Dir, ref, Options{
			ContextRequestPath: requestPath,
			BoardHref:          func(branch, name string) string { return "/b/" + branch + "/board/spec/" + name },
		})
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		provenance := concernByID(t, snap, "shape/provenance")
		assertBoardDestination(t, provenance, "/b/design/feature-alpha/board/spec/feature-alpha")
	})

	t.Run("without BoardHref", func(t *testing.T) {
		l := loader{newConflictProvider: passProviderFactory(t, repo.Dir)}
		snap, err := l.load(context.Background(), repo.Dir, ref, Options{ContextRequestPath: requestPath})
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		provenance := concernByID(t, snap, "shape/provenance")
		if provenance.Destination.BoardPath != "" || len(provenance.Destination.CLI) == 0 {
			t.Fatalf("provenance destination = %+v, want the CLI fallback vector (no BoardHref supplied)", provenance.Destination)
		}
		assertCLI(t, provenance.Destination.CLI, []string{"verdi", "journey", ref})
	})
}
