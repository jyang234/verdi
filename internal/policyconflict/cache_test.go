// Task 7 Step 1 RED matrix for the D4 immutable judgment cache (authority
// design §7, ledger SI-96/SI-101): hit/miss, every independent cache-key
// axis, bare-hex filename segments, strict inner full digests, symlink,
// corruption, mismatched key, concurrent identical writers, a
// different-winner collision, persistence failure, and the rule that
// failed/invalid attempts are never cached. Test names match
// -run 'TestPolicyConflictCache'.
package policyconflict

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/store"
)

func hex64(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

var cacheTestTreeHash = hex64("tree")

func cacheTestInput() SemanticInput {
	return SemanticInput{Prompt: []byte("prompt bytes"), Claims: nil, UnknownMechanicals: nil, Exemptions: nil}
}

// --- hit / miss ------------------------------------------------------------

func TestPolicyConflictCache_Miss_RunsJudgeAndPublishes(t *testing.T) {
	root := t.TempDir()
	want := noConflictResultBytes(t)
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) { return want, 0, nil }}
	a := baseAdapter(runner)
	a.Root = root

	got, err := CachedJudge(context.Background(), a, cacheTestInput(), cacheTestTreeHash, "solo", "profile-digest-1", "authority-digest-1")
	if err != nil {
		t.Fatalf("CachedJudge: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner.calls = %d, want 1", runner.calls)
	}
	if got.Exchange.Result.Recommendation != RecommendationNoConflict {
		t.Fatalf("Recommendation = %q, want %q", got.Exchange.Result.Recommendation, RecommendationNoConflict)
	}
	if got.RecordDigest == "" {
		t.Fatal("RecordDigest must not be empty")
	}

	entries, err := os.ReadDir(filepath.Join(root, ".verdi", "data", "cache"))
	if err != nil {
		t.Fatalf("ReadDir cache dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("cache dir entries = %d, want exactly 1", len(entries))
	}
	name := entries[0].Name()
	if !strings.HasPrefix(name, "policy-conflict-"+store.LayoutVersion+"-"+cacheTestTreeHash+"-") || !strings.HasSuffix(name, ".json") {
		t.Fatalf("cache filename = %q, does not match the D4 grammar", name)
	}
}

func TestPolicyConflictCache_Hit_DoesNotRunJudge(t *testing.T) {
	root := t.TempDir()
	want := noConflictResultBytes(t)
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) { return want, 0, nil }}
	a := baseAdapter(runner)
	a.Root = root
	input := cacheTestInput()

	first, err := CachedJudge(context.Background(), a, input, cacheTestTreeHash, "solo", "profile-digest-1", "authority-digest-1")
	if err != nil {
		t.Fatalf("first CachedJudge: %v", err)
	}
	second, err := CachedJudge(context.Background(), a, input, cacheTestTreeHash, "solo", "profile-digest-1", "authority-digest-1")
	if err != nil {
		t.Fatalf("second CachedJudge: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner.calls = %d, want 1 (second call must be a cache hit)", runner.calls)
	}
	if second.Exchange.Result.Recommendation != first.Exchange.Result.Recommendation {
		t.Fatalf("second recommendation = %q, want %q", second.Exchange.Result.Recommendation, first.Exchange.Result.Recommendation)
	}
	if second.Exchange.RawDigest != first.Exchange.RawDigest {
		t.Fatalf("second RawDigest = %q, want %q (same cached exchange)", second.Exchange.RawDigest, first.Exchange.RawDigest)
	}
}

// --- cache-key axes (unit-level, via judgeCacheKeyDigest directly) --------

func baseKeyArgs() (JudgeAdapter, []byte, []byte, string, string, string) {
	a := baseAdapter(nil)
	prompt := []byte("prompt")
	input := []byte(`{"claims":[]}`)
	return a, prompt, input, "solo", "profile-digest", "authority-digest"
}

func TestPolicyConflictCache_KeyChanges_Role(t *testing.T) {
	a, prompt, input, pID, pDig, aDig := baseKeyArgs()
	base, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	a.Role = "challenger"
	changed, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	if base == changed {
		t.Fatal("changing Role must change the cache key")
	}
}

func TestPolicyConflictCache_KeyChanges_Argv(t *testing.T) {
	a, prompt, input, pID, pDig, aDig := baseKeyArgs()
	base, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	a.Argv = []string{"judge-bin", "--other-flag"}
	changed, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	if base == changed {
		t.Fatal("changing Argv must change the cache key")
	}
}

func TestPolicyConflictCache_KeyChanges_Model(t *testing.T) {
	a, prompt, input, pID, pDig, aDig := baseKeyArgs()
	base, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	a.Model = "a-different-model"
	changed, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	if base == changed {
		t.Fatal("changing Model must change the cache key")
	}
}

func TestPolicyConflictCache_KeyChanges_Prompt(t *testing.T) {
	a, prompt, input, pID, pDig, aDig := baseKeyArgs()
	base, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := judgeCacheKeyDigest(a, []byte("a different prompt"), input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	if base == changed {
		t.Fatal("changing prompt bytes must change the cache key")
	}
}

func TestPolicyConflictCache_KeyChanges_Input(t *testing.T) {
	a, prompt, input, pID, pDig, aDig := baseKeyArgs()
	base, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := judgeCacheKeyDigest(a, prompt, []byte(`{"claims":["different"]}`), pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	if base == changed {
		t.Fatal("changing input bytes must change the cache key")
	}
}

func TestPolicyConflictCache_KeyChanges_Profile(t *testing.T) {
	a, prompt, input, pID, pDig, aDig := baseKeyArgs()
	base, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := judgeCacheKeyDigest(a, prompt, input, "team", pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	if base == changed {
		t.Fatal("changing profile id must change the cache key")
	}
	changed2, err := judgeCacheKeyDigest(a, prompt, input, pID, "a-different-profile-digest", aDig)
	if err != nil {
		t.Fatal(err)
	}
	if base == changed2 {
		t.Fatal("changing profile digest must change the cache key")
	}
}

func TestPolicyConflictCache_KeyChanges_Authority(t *testing.T) {
	a, prompt, input, pID, pDig, aDig := baseKeyArgs()
	base, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, "a-different-authority-digest")
	if err != nil {
		t.Fatal(err)
	}
	if base == changed {
		t.Fatal("changing authority digest must change the cache key")
	}
}

func TestPolicyConflictCache_KeyDigest_BareHex(t *testing.T) {
	a, prompt, input, pID, pDig, aDig := baseKeyArgs()
	got, err := judgeCacheKeyDigest(a, prompt, input, pID, pDig, aDig)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 64 || strings.Contains(got, ":") {
		t.Fatalf("judgeCacheKeyDigest = %q, want bare 64-lowercase-hex (no sha256: prefix)", got)
	}
}

// --- strict inner full digests ---------------------------------------------

func TestPolicyConflictCache_PersistedRecordCarriesFullDigests(t *testing.T) {
	root := t.TempDir()
	want := noConflictResultBytes(t)
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) { return want, 0, nil }}
	a := baseAdapter(runner)
	a.Root = root
	if _, err := CachedJudge(context.Background(), a, cacheTestInput(), cacheTestTreeHash, "solo", "d1", "d2"); err != nil {
		t.Fatalf("CachedJudge: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".verdi", "data", "cache"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("ReadDir: entries=%v err=%v", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "cache", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	judgment, err := DecodeJudgment(data)
	if err != nil {
		t.Fatalf("DecodeJudgment: %v", err)
	}
	for name, got := range map[string]string{
		"command_digest": judgment.Exchange.CommandDigest,
		"prompt_digest":  judgment.Exchange.PromptDigest,
		"input_digest":   judgment.Exchange.InputDigest,
		"raw_digest":     judgment.Exchange.RawDigest,
		"digest":         judgment.Digest,
	} {
		if !strings.HasPrefix(got, "sha256:") || len(got) != len("sha256:")+64 {
			t.Errorf("%s = %q, want full sha256:<64 hex> form", name, got)
		}
	}
	if len(judgment.TreeHash) != 64 || strings.Contains(judgment.TreeHash, ":") {
		t.Errorf("judgment.TreeHash = %q, want bare 64 hex", judgment.TreeHash)
	}
	if len(judgment.InputDigest) != 64 || strings.Contains(judgment.InputDigest, ":") {
		t.Errorf("judgment.InputDigest = %q, want bare 64 hex", judgment.InputDigest)
	}
}

// --- symlink / corruption / mismatched key ---------------------------------

func precomputedCachePath(t *testing.T, root string, a JudgeAdapter, input SemanticInput, treeHash, profileID, profileDigest, authorityDigest string) string {
	t.Helper()
	inputBytes, err := testSemanticInputBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	key, err := judgeCacheKeyDigest(a, input.Prompt, inputBytes, profileID, profileDigest, authorityDigest)
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.PolicyConflictCachePath(root, treeHash, key)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// testSemanticInputBytes mirrors CachedJudge's own private canonical
// encoding of a SemanticInput's identity-bearing fields (Prompt excluded,
// same as production) so tests can precompute the exact cache path/key a
// live CachedJudge call would use.
func testSemanticInputBytes(in SemanticInput) ([]byte, error) {
	return canonjson.Marshal(semanticInputWitnessDoc{Claims: in.Claims, UnknownMechanicals: in.UnknownMechanicals, Exemptions: in.Exemptions})
}

func TestPolicyConflictCache_Symlink(t *testing.T) {
	root := t.TempDir()
	a := baseAdapter(&fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		t.Fatal("runner must not be invoked when the cache path is a refused symlink")
		return nil, 0, nil
	}})
	a.Root = root
	input := cacheTestInput()
	path := precomputedCachePath(t, root, a, input, cacheTestTreeHash, "solo", "d1", "d2")

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

	_, err := CachedJudge(context.Background(), a, input, cacheTestTreeHash, "solo", "d1", "d2")
	if err == nil {
		t.Fatal("expected an error for a symlinked cache path")
	}
}

func TestPolicyConflictCache_Corruption(t *testing.T) {
	root := t.TempDir()
	a := baseAdapter(&fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		t.Fatal("runner must not be invoked when the existing cache record is corrupt")
		return nil, 0, nil
	}})
	a.Root = root
	input := cacheTestInput()
	path := precomputedCachePath(t, root, a, input, cacheTestTreeHash, "solo", "d1", "d2")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := CachedJudge(context.Background(), a, input, cacheTestTreeHash, "solo", "d1", "d2")
	if err == nil {
		t.Fatal("expected an error for a corrupt cache record")
	}
}

func TestPolicyConflictCache_MismatchedKey(t *testing.T) {
	root := t.TempDir()
	want := noConflictResultBytes(t)
	// Publish a legitimate record under one key.
	runnerA := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) { return want, 0, nil }}
	a := baseAdapter(runnerA)
	a.Root = root
	inputA := SemanticInput{Prompt: []byte("prompt A")}
	if _, err := CachedJudge(context.Background(), a, inputA, cacheTestTreeHash, "solo", "d1", "d2"); err != nil {
		t.Fatalf("CachedJudge: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".verdi", "data", "cache"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("ReadDir: entries=%v err=%v", entries, err)
	}
	winnerPath := filepath.Join(root, ".verdi", "data", "cache", entries[0].Name())
	winnerBytes, err := os.ReadFile(winnerPath)
	if err != nil {
		t.Fatal(err)
	}

	// Copy that legitimate (self-consistent) record's bytes to the path a
	// DIFFERENT key would compute — the record decodes fine on its own but
	// disagrees with the path it was found at.
	inputB := SemanticInput{Prompt: []byte("prompt B")}
	mismatchedPath := precomputedCachePath(t, root, a, inputB, cacheTestTreeHash, "solo", "d1", "d2")
	if err := os.WriteFile(mismatchedPath, winnerBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	runnerB := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		t.Fatal("runner must not be invoked past a mismatched-key hit-check refusal")
		return nil, 0, nil
	}}
	b := baseAdapter(runnerB)
	b.Root = root
	_, err = CachedJudge(context.Background(), b, inputB, cacheTestTreeHash, "solo", "d1", "d2")
	if err == nil {
		t.Fatal("expected an error for a cache record whose path key does not match its own content")
	}
}

// --- lock-holder refusal / persistence failure / never-cached -------------

func TestPolicyConflictCache_LockHeldIsOperational(t *testing.T) {
	root := t.TempDir()
	lockPath := store.WriterLockPath(root)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	lockFile, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("pre-acquiring writer lock: %v", err)
	}
	t.Cleanup(func() { _ = filelock.Release(lockFile, lockPath) })

	want := noConflictResultBytes(t)
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) { return want, 0, nil }}
	a := baseAdapter(runner)
	a.Root = root

	_, err = CachedJudge(context.Background(), a, cacheTestInput(), cacheTestTreeHash, "solo", "d1", "d2")
	if err == nil {
		t.Fatal("expected an error when the writer lock is already held")
	}
}

func TestPolicyConflictCache_PersistenceFailure(t *testing.T) {
	root := t.TempDir()
	// Plant a FILE where the data/ directory must go, so cache-directory
	// creation fails.
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "data"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	want := noConflictResultBytes(t)
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) { return want, 0, nil }}
	a := baseAdapter(runner)
	a.Root = root

	_, err := CachedJudge(context.Background(), a, cacheTestInput(), cacheTestTreeHash, "solo", "d1", "d2")
	if err == nil {
		t.Fatal("expected an error when the cache directory cannot be created")
	}
}

func TestPolicyConflictCache_FailedAttemptNeverCached(t *testing.T) {
	root := t.TempDir()
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		return []byte("not json"), 0, nil
	}}
	a := baseAdapter(runner)
	a.Root = root
	input := cacheTestInput()

	_, err := CachedJudge(context.Background(), a, input, cacheTestTreeHash, "solo", "d1", "d2")
	if err == nil {
		t.Fatal("expected an error for malformed judge output")
	}
	entries, rerr := os.ReadDir(filepath.Join(root, ".verdi", "data", "cache"))
	if rerr == nil && len(entries) != 0 {
		t.Fatalf("cache dir entries = %d, want 0 — a failed attempt must never be cached", len(entries))
	}
}

func TestPolicyConflictCache_InvalidWitnessAttemptNeverCached(t *testing.T) {
	root := t.TempDir()
	// A malformed-per-witness result: "conflict" with a finding naming a
	// claim the (empty) SemanticInput never carried.
	resultBytes, err := EncodeJudgeResult(JudgeResult{
		Schema: JudgeResultSchema, Recommendation: RecommendationConflict,
		Findings: []JudgeFinding{{
			Claims:      []ClaimWitness{{ID: "spec/ghost#outcome", Digest: "sha256:" + hex64("ghost2"), Category: "spec-outcome"}, {ID: "spec/ghost#problem", Digest: "sha256:" + hex64("ghost"), Category: "spec-problem"}},
			Categories:  []string{"spec-outcome", "spec-problem"},
			Explanation: "a claim that does not exist in the input",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		return resultBytes, 0, nil
	}}
	a := baseAdapter(runner)
	a.Root = root
	input := cacheTestInput() // no claims at all

	_, err = CachedJudge(context.Background(), a, input, cacheTestTreeHash, "solo", "d1", "d2")
	if err == nil {
		t.Fatal("expected an error for a result citing an unknown claim witness")
	}
	entries, rerr := os.ReadDir(filepath.Join(root, ".verdi", "data", "cache"))
	if rerr == nil && len(entries) != 0 {
		t.Fatalf("cache dir entries = %d, want 0 — an invalid witness attempt must never be cached", len(entries))
	}
}

// --- concurrent identical writers / different-winner collision ------------

// TestPolicyConflictCache_ConcurrentIdenticalWriters deterministically
// reproduces "another process published the same key before this process
// acquired the lock" (authority design §7) by having the fake runner
// itself complete a FULL, independent CachedJudge call — acquiring and
// releasing the writer lock — before returning control to the outer call,
// which then races the (already-vacated) lock and finds its own
// independently-computed, byte-identical Judgment already published.
func TestPolicyConflictCache_ConcurrentIdenticalWriters(t *testing.T) {
	root := t.TempDir()
	want := noConflictResultBytes(t)
	input := cacheTestInput()

	innerRan := false
	outer := &fakeJudgeRunner{}
	outer.fn = func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		if !innerRan {
			innerRan = true
			inner := baseAdapter(&fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) { return want, 0, nil }})
			inner.Root = root
			if _, err := CachedJudge(context.Background(), inner, input, cacheTestTreeHash, "solo", "d1", "d2"); err != nil {
				t.Fatalf("inner CachedJudge (simulated concurrent publisher): %v", err)
			}
		}
		return want, 0, nil
	}
	a := baseAdapter(outer)
	a.Root = root

	got, err := CachedJudge(context.Background(), a, input, cacheTestTreeHash, "solo", "d1", "d2")
	if err != nil {
		t.Fatalf("outer CachedJudge: %v", err)
	}
	if got.Exchange.Result.Recommendation != RecommendationNoConflict {
		t.Fatalf("Recommendation = %q, want %q", got.Exchange.Result.Recommendation, RecommendationNoConflict)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".verdi", "data", "cache"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("cache dir entries = %v (err=%v), want exactly one shared record", entries, err)
	}
}

// TestPolicyConflictCache_DifferentWinnerCollision mirrors the identical-
// writer test above, except the "concurrent" publisher used a DIFFERENT
// adapter Model — genuinely different content under the SAME cache key
// only if the differing field were NOT part of the key; since Model IS
// part of the key, this instead directly forges a mismatched pre-existing
// record via a raw write (the same technique TestPolicyConflictCache_
// MismatchedKey uses) to exercise the post-lock byte-identity check with
// a record that DOES match the path key but carries different content.
func TestPolicyConflictCache_DifferentWinnerCollision(t *testing.T) {
	root := t.TempDir()
	want := noConflictResultBytes(t)
	input := cacheTestInput()
	a := baseAdapter(nil)
	a.Root = root
	path := precomputedCachePath(t, root, a, input, cacheTestTreeHash, "solo", "d1", "d2")

	inputBytes, err := testSemanticInputBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	key, err := judgeCacheKeyDigest(a, input.Prompt, inputBytes, "solo", "d1", "d2")
	if err != nil {
		t.Fatal(err)
	}
	foreignResult, err := EncodeJudgeResult(JudgeResult{Schema: JudgeResultSchema, Recommendation: RecommendationConflict, Findings: []JudgeFinding{{
		Claims:      []ClaimWitness{{ID: "spec/a#x", Digest: "sha256:" + hex64("a"), Category: "spec-problem"}, {ID: "spec/b#y", Digest: "sha256:" + hex64("b"), Category: "spec-outcome"}},
		Categories:  []string{"spec-outcome", "spec-problem"},
		Explanation: "a foreign, different exchange",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	foreign := Judgment{
		Schema: JudgmentSchema, TreeHash: cacheTestTreeHash, InputDigest: key,
		Exchange: JudgmentExchange{
			Role: JudgePrimary, Adapter: a.Adapter, Model: a.Model,
			CommandDigest: rawContentDigest([]byte("argv")), PromptDigest: rawContentDigest(input.Prompt), InputDigest: rawContentDigest(inputBytes),
			RawResult: string(foreignResult), RawDigest: rawContentDigest(foreignResult), Result: mustDecodeJudgeResult(t, foreignResult),
		},
	}
	foreignBytes, err := EncodeJudgment(foreign)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, foreignBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) { return want, 0, nil }}
	a.Runner = runner
	// The initial hit-check will find the foreign record and, since it
	// decodes and key-matches, treat it as a legitimate HIT — proving the
	// pre-check honors whatever is already validly there. To reach the
	// post-lock collision path specifically, remove the pre-existing
	// record's discoverability by the pre-check is not meaningful (a
	// pre-check hit is a hit); the collision path is instead exercised via
	// TestPolicyConflictCache_ConcurrentIdenticalWriters's inner-call
	// technique with differing content, below.
	got, err := CachedJudge(context.Background(), a, input, cacheTestTreeHash, "solo", "d1", "d2")
	if err != nil {
		t.Fatalf("CachedJudge: %v", err)
	}
	if got.Exchange.Result.Recommendation != RecommendationConflict {
		t.Fatalf("Recommendation = %q, want the pre-existing foreign record's %q (a valid hit)", got.Exchange.Result.Recommendation, RecommendationConflict)
	}
	if runner.calls != 0 {
		t.Fatalf("runner.calls = %d, want 0 (cache hit)", runner.calls)
	}
}

// TestPolicyConflictCache_ConcurrentDifferentWinnerCollision uses the same
// inner-call technique as the identical-writers test, but the inner
// (simulated concurrent) publisher computes a genuinely DIFFERENT result
// for a component the cache key does NOT bind — impossible, since every
// bound component is part of the key by construction. Instead this proves
// the collision path directly: the inner publisher races in, publishes
// under the SAME key, and the outer call's own build (from a mutated
// SemanticInput.Prompt swapped in between the outer key computation and
// its own judge return) would collide — modeled here by having the outer
// runner itself return a DIFFERENT raw result than the inner one for the
// identical key inputs, which the process/validation layer cannot produce
// deterministically, so this test instead forges the outer side via the
// same raw-write technique the mismatched-key test uses, confirming
// byte-for-byte inequality is refused rather than silently accepted.
func TestPolicyConflictCache_ConcurrentDifferentWinnerCollision(t *testing.T) {
	root := t.TempDir()
	input := cacheTestInput()
	resultA, err := EncodeJudgeResult(JudgeResult{Schema: JudgeResultSchema, Recommendation: RecommendationNoConflict, Findings: []JudgeFinding{}})
	if err != nil {
		t.Fatal(err)
	}
	resultB, err := EncodeJudgeResult(JudgeResult{Schema: JudgeResultSchema, Recommendation: RecommendationInconclusive, Findings: []JudgeFinding{}})
	if err != nil {
		t.Fatal(err)
	}

	innerRan := false
	outer := &fakeJudgeRunner{}
	outer.fn = func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		if !innerRan {
			innerRan = true
			inner := baseAdapter(&fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) { return resultA, 0, nil }})
			inner.Root = root
			if _, err := CachedJudge(context.Background(), inner, input, cacheTestTreeHash, "solo", "d1", "d2"); err != nil {
				t.Fatalf("inner CachedJudge: %v", err)
			}
		}
		// The outer call's OWN process run returns a genuinely different
		// result for the identical key inputs — a real judge is not
		// expected to be perfectly deterministic between two independent
		// invocations, so this is exactly the case the byte-identity check
		// exists for.
		return resultB, 0, nil
	}
	a := baseAdapter(outer)
	a.Root = root

	_, err = CachedJudge(context.Background(), a, input, cacheTestTreeHash, "solo", "d1", "d2")
	if err == nil {
		t.Fatal("expected a collision error when the outer call's own content disagrees with the already-published winner")
	}
}

func mustDecodeJudgeResult(t *testing.T, data []byte) JudgeResult {
	t.Helper()
	r, err := DecodeJudgeResult(data)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
