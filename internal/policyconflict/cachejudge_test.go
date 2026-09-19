// Task 2 (spec/readiness-recovery ac-3, plan R-RR1-4) RED matrix for the
// cache-only judge: a hit returns the validated exchange exactly as
// CachedJudge would, a miss reports ErrJudgeCacheMiss without ever running
// adapter.Runner, and a corrupt cache record is an operational error rather
// than a miss. Test names match -run 'CacheOnly|CacheMiss'.
package policyconflict

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/store"
)

// seedCacheOnlyJudgment writes a canonical Judgment at the exact path/key
// cacheOnlyLookup computes for (adapter, input, treeHash, profileID,
// profileDigest, authorityDigest) — built with this package's own
// judgeCacheKeyDigest, per the task brief ("same package, so reachable"),
// never a hand-rolled path.
func seedCacheOnlyJudgment(t *testing.T, adapter JudgeAdapter, input SemanticInput, treeHash, profileID, profileDigest, authorityDigest string, resultBytes []byte) JudgmentExchange {
	t.Helper()
	inputBytes, err := testSemanticInputBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	key, err := judgeCacheKeyDigest(adapter, input.Prompt, inputBytes, profileID, profileDigest, authorityDigest)
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.PolicyConflictCachePath(adapter.Root, treeHash, key)
	if err != nil {
		t.Fatal(err)
	}
	result := mustDecodeJudgeResult(t, resultBytes)
	exchange := JudgmentExchange{
		Role: JudgeRole(adapter.Role), Adapter: adapter.Adapter, Model: adapter.Model,
		CommandDigest: rawContentDigest([]byte(joinArgv(adapter.Argv))),
		PromptDigest:  rawContentDigest(input.Prompt), InputDigest: rawContentDigest(inputBytes),
		RawResult: string(resultBytes), RawDigest: rawContentDigest(resultBytes), Result: result,
	}
	judgment := Judgment{
		Schema: JudgmentSchema, TreeHash: treeHash, InputDigest: key,
		ProfileID: profileID, ProfileDigest: profileDigest, AuthorityDigest: authorityDigest,
		Exchange: exchange,
	}
	encoded, err := EncodeJudgment(judgment)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	return exchange
}

func neverCalledRunner(t *testing.T) *fakeJudgeRunner {
	return &fakeJudgeRunner{fn: func(context.Context, []string, []byte) ([]byte, int, error) {
		t.Fatal("cache-only judge must never run adapter.Argv")
		return nil, 0, nil
	}}
}

func TestNewCacheOnlyJudge_HitReturnsExchangeWithoutRunning(t *testing.T) {
	root := t.TempDir()
	adapter := baseAdapter(neverCalledRunner(t))
	adapter.Root = root
	input := cacheTestInput()
	want := noConflictResultBytes(t)
	seedCacheOnlyJudgment(t, adapter, input, standaloneTreeHash, standaloneProfileID, standaloneProfileDigest, standaloneAuthorityDigest, want)

	got, err := NewCacheOnlyJudge(adapter).Judge(context.Background(), input.Prompt, mustSemanticInputBytes(t, input))
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if got.Result.Recommendation != RecommendationNoConflict {
		t.Fatalf("Recommendation = %q, want %q", got.Result.Recommendation, RecommendationNoConflict)
	}
	if got.Role != JudgePrimary || got.Adapter != adapter.Adapter || got.Model != adapter.Model {
		t.Fatalf("exchange transport identity = %+v, want the adapter's own", got)
	}
}

func TestNewCacheOnlyJudge_MissReportsErrJudgeCacheMissWithoutRunning(t *testing.T) {
	root := t.TempDir()
	adapter := baseAdapter(neverCalledRunner(t))
	adapter.Root = root
	input := cacheTestInput()

	_, err := NewCacheOnlyJudge(adapter).Judge(context.Background(), input.Prompt, mustSemanticInputBytes(t, input))
	if !errors.Is(err, ErrJudgeCacheMiss) {
		t.Fatalf("Judge error = %v, want ErrJudgeCacheMiss", err)
	}
}

func TestNewCacheOnlyJudge_CorruptCacheIsOperationalNotMiss(t *testing.T) {
	root := t.TempDir()
	adapter := baseAdapter(neverCalledRunner(t))
	adapter.Root = root
	input := cacheTestInput()

	inputBytes, err := testSemanticInputBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	key, err := judgeCacheKeyDigest(adapter, input.Prompt, inputBytes, standaloneProfileID, standaloneProfileDigest, standaloneAuthorityDigest)
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.PolicyConflictCachePath(root, standaloneTreeHash, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = NewCacheOnlyJudge(adapter).Judge(context.Background(), input.Prompt, mustSemanticInputBytes(t, input))
	if err == nil || errors.Is(err, ErrJudgeCacheMiss) {
		t.Fatalf("Judge error = %v, want an operational error (not a miss) for a corrupt cache record", err)
	}
	if !errors.Is(err, ErrCacheOperational) {
		t.Fatalf("Judge error = %v, want it to wrap ErrCacheOperational", err)
	}
}

func TestNewCacheOnlyJudge_SymlinkedCachePathIsOperational(t *testing.T) {
	root := t.TempDir()
	adapter := baseAdapter(neverCalledRunner(t))
	adapter.Root = root
	input := cacheTestInput()

	inputBytes, err := testSemanticInputBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	key, err := judgeCacheKeyDigest(adapter, input.Prompt, inputBytes, standaloneProfileID, standaloneProfileDigest, standaloneAuthorityDigest)
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.PolicyConflictCachePath(root, standaloneTreeHash, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(root, "elsewhere.json")
	if err := os.WriteFile(elsewhere, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, path); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	_, err = NewCacheOnlyJudge(adapter).Judge(context.Background(), input.Prompt, mustSemanticInputBytes(t, input))
	if err == nil || errors.Is(err, ErrJudgeCacheMiss) {
		t.Fatalf("Judge error = %v, want an operational refusal (not a miss) for a symlinked cache path", err)
	}
}

// mustSemanticInputBytes mirrors the canonical normalized-input encoding
// runValidatedJudge itself computes (service.go's inputBytes, derived from
// SemanticInput via semanticInputWitnessDoc) — the exact bytes a cache-only
// judge's plain Judge(ctx, prompt, input) method receives as its "input"
// argument.
func mustSemanticInputBytes(t *testing.T, in SemanticInput) []byte {
	t.Helper()
	data, err := testSemanticInputBytes(in)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestNewCacheOnlyJudge_FindsARealRunsCacheEntry proves R-RR1-4's own
// substantive claim beyond the brief's direct unit test: "the cache key is
// computed by the same judgeCacheKeyDigest path a real run uses, so a
// judgment cached by verdi context conflict or by serve --context-request
// is found by a request." It drives runValidatedJudge (via Service.Evaluate)
// twice against the SAME checkout/adapter identity: once with a concrete
// JudgeAdapter (a real run, publishing through CachedJudge with the
// evaluation's own resolved tree-hash/profile/authority axes), then again
// with NewCacheOnlyJudge wrapping the identical adapter — the second
// evaluation's semantic row must reflect the cached judgment without ever
// invoking the runner a second time.
func TestNewCacheOnlyJudge_FindsARealRunsCacheEntry(t *testing.T) {
	service, repo := newServiceFixture(t, nil)
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", ".gitignore"), []byte("data/\n"), 0o644); err != nil {
		t.Fatalf("write cache ignore: %v", err)
	}
	runOperandGit(t, repo.Dir, "add", ".verdi/.gitignore")
	runOperandGit(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "ignore data cache")
	repo.Head = strings.TrimSpace(runOperandGit(t, repo.Dir, "rev-parse", "HEAD"))

	runs := 0
	runner := &fakeJudgeRunner{fn: func(context.Context, []string, []byte) ([]byte, int, error) {
		runs++
		return noConflictResultBytes(t), 0, nil
	}}
	adapter := baseAdapter(runner)
	adapter.Root = repo.Dir
	hasher := &serviceTreeHasher{hash: cacheTestTreeHash}
	service.Deps.TreeHasher = hasher

	service.Deps.Primary = &adapter
	realRun, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
	if err != nil {
		t.Fatalf("real-run Evaluate: %v", err)
	}
	if runs != 1 {
		t.Fatalf("runner calls after the real run = %d, want 1", runs)
	}

	service.Deps.Primary = NewCacheOnlyJudge(adapter)
	cacheOnlyRun, err := service.Evaluate(context.Background(), serviceAcceptedRequest())
	if err != nil {
		t.Fatalf("cache-only Evaluate: %v", err)
	}
	if runs != 1 {
		t.Fatalf("runner calls after the cache-only run = %d, want still 1 (found the real run's cache entry)", runs)
	}
	if len(cacheOnlyRun.Report.Semantic) != 1 || cacheOnlyRun.Report.Semantic[0].State != realRun.Report.Semantic[0].State {
		t.Fatalf("cache-only semantic row = %+v, want it to match the real run's %+v", cacheOnlyRun.Report.Semantic, realRun.Report.Semantic)
	}
	if cacheOnlyRun.Report.Semantic[0].Primary == nil || cacheOnlyRun.Report.Semantic[0].Primary.RawDigest != realRun.Report.Semantic[0].Primary.RawDigest {
		t.Fatalf("cache-only primary exchange = %+v, want the real run's cached exchange", cacheOnlyRun.Report.Semantic[0].Primary)
	}
}
