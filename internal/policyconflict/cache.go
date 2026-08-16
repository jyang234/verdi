// cache.go is the D4 immutable judgment-reuse adapter (authority design §7,
// ledger SI-96/SI-101): CachedJudge composes the pure Judge port (judge.go)
// and the witness cross-check (semantic.go's ValidateJudgeResult) around
// the checkout-scoped cache described in store-layout D4 —
// .verdi/data/cache/policy-conflict-<layout-version>-<tree-hash>-
// <input-digest>.json. The judge invocation itself runs WITHOUT D3's
// checkout writer lock; only cache-directory creation and immutable
// publication acquire that existing nonblocking lock, and only for the
// narrow window CreateImmutable needs.
package policyconflict

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/store"
)

// ErrCacheOperational is the one sentinel every D4 cache-path failure
// wraps: lock-holder refusal, a malformed/noncanonical/mismatched/
// symlinked existing record, a different-winner collision, or a
// persistence failure (authority design §7/§12 — all Exit 2, never a
// favorable fallback).
var ErrCacheOperational = errors.New("policyconflict: policy-conflict cache operational failure")

// judgeCacheKeyDoc is the private composite the D4 cache filename's
// input-digest segment binds (authority design §7): "the role (primary or
// challenger), adapter-declared transport/model posture, argv digest,
// prompt digest, normalized input digest, profile/challenger posture, and
// effective authority digest." It is never decoded — only ever hashed by
// this package and re-hashed the same way to verify a cache path — so
// plain field names (canonjson sorts and fixes encoding regardless) are
// enough; it needs no json tags or cross-version wire stability.
type judgeCacheKeyDoc struct {
	Role            JudgeRole
	AdapterID       string
	AdapterVersion  string
	Model           string
	ArgvDigest      string
	PromptDigest    string
	InputDigest     string
	ProfileID       string
	ProfileDigest   string
	AuthorityDigest string
}

// CachedJudge is the D4 cache-aware entry point one primary or challenger
// invocation goes through (authority design §7): on a cache hit it
// strict-decodes, canonical-reencodes, verifies the path key, and returns
// the recorded exchange WITHOUT running adapter's process; on a miss it
// runs adapter.Judge (unlocked), cross-checks the result via
// ValidateJudgeResult, and publishes the validated exchange as an
// immutable D4 record under D3's writer.lock, held only around cache
// directory creation and publication.
//
// treeHash is the D4 tree hash (store.TreeHash's own bare-hex return
// form); profileID/profileDigest and authorityDigest are the
// governanceprincipal.Profile identity and effective-policy digest the
// evaluation snapshot carries — axes the fixed SemanticInput/JudgeAdapter
// contracts have no field for, so the caller (the not-yet-built Task 9
// service, which already resolves a ConflictView.Snapshot) supplies them
// explicitly rather than this package inventing a new exported carrier
// type for them.
func CachedJudge(ctx context.Context, adapter JudgeAdapter, input SemanticInput, treeHash, profileID, profileDigest, authorityDigest string) (ValidatedExchange, error) {
	role := JudgeRole(adapter.Role)
	if err := role.Validate(); err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: adapter role: %v", ErrCacheOperational, err)
	}
	if err := validateNonEmpty("adapter.id", adapter.Adapter.ID); err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}
	if err := validateNonEmpty("adapter.version", adapter.Adapter.Version); err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}
	if err := validateNonEmpty("adapter.model", adapter.Model); err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}
	if len(adapter.Argv) == 0 {
		return ValidatedExchange{}, fmt.Errorf("%w: adapter argv is empty", ErrCacheOperational)
	}

	promptBytes := input.Prompt
	inputBytes, err := canonjson.Marshal(semanticInputWitnessDoc{
		Claims:        input.Claims,
		UnknownScopes: input.UnknownScopes,
		Exemptions:    input.Exemptions,
	})
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: encoding normalized semantic input: %v", ErrCacheOperational, err)
	}

	bareKeyDigest, err := judgeCacheKeyDigest(adapter, promptBytes, inputBytes, profileID, profileDigest, authorityDigest)
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}
	path, err := store.PolicyConflictCachePath(adapter.Root, treeHash, bareKeyDigest)
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}

	recordDigest, err := semanticInputDigest(input)
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}

	// Cache hit check, before running any process (authority design §7:
	// "returns the recorded result without launching a process").
	if judgment, found, err := loadCachedJudgment(path, treeHash, bareKeyDigest); err != nil {
		return ValidatedExchange{}, err
	} else if found {
		return ValidatedExchange{Exchange: judgment.Exchange, RecordDigest: recordDigest}, nil
	}

	// Miss: run the judge WITHOUT the checkout writer lock.
	exchange, err := adapter.Judge(ctx, promptBytes, inputBytes)
	if err != nil {
		return ValidatedExchange{}, err
	}
	// Defend against a Judge implementation (real or, in tests, hand-built)
	// asserting a transport identity or digest other than what was
	// actually configured/sent — "Judge output cannot override
	// adapter-declared model identity" (authority design §6), generalized
	// to every transport-provenance field this package itself supplies.
	if exchange.Role != role {
		return ValidatedExchange{}, fmt.Errorf("%w: judge returned role %q, want %q", ErrCacheOperational, exchange.Role, role)
	}
	if exchange.Adapter != adapter.Adapter {
		return ValidatedExchange{}, fmt.Errorf("%w: judge returned adapter %+v, want %+v", ErrCacheOperational, exchange.Adapter, adapter.Adapter)
	}
	if exchange.Model != adapter.Model {
		return ValidatedExchange{}, fmt.Errorf("%w: judge returned model %q, want %q", ErrCacheOperational, exchange.Model, adapter.Model)
	}
	wantPromptDigest := rawContentDigest(promptBytes)
	if exchange.PromptDigest != wantPromptDigest {
		return ValidatedExchange{}, fmt.Errorf("%w: judge returned prompt_digest %q, want %q", ErrCacheOperational, exchange.PromptDigest, wantPromptDigest)
	}
	wantInputDigest := rawContentDigest(inputBytes)
	if exchange.InputDigest != wantInputDigest {
		return ValidatedExchange{}, fmt.Errorf("%w: judge returned input_digest %q, want %q", ErrCacheOperational, exchange.InputDigest, wantInputDigest)
	}

	if _, err := ValidateJudgeResult(input, exchange.Result); err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}

	judgment := Judgment{Schema: JudgmentSchema, TreeHash: treeHash, InputDigest: bareKeyDigest, Exchange: exchange}
	encoded, err := EncodeJudgment(judgment)
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: encoding judgment: %v", ErrCacheOperational, err)
	}

	lockPath := store.WriterLockPath(adapter.Root)
	// filelock.Acquire needs data/ to already exist (it opens the lock
	// file with O_CREATE|O_EXCL, never creating parent directories) — a
	// fresh checkout's first cache write is exactly the case where data/
	// does not exist yet, so this package (not filelock, a shared,
	// intentionally minimal primitive) owns creating it.
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: creating %s: %v", ErrCacheOperational, filepath.Dir(lockPath), err)
	}
	lockFile, err := filelock.Acquire(lockPath)
	if err != nil {
		var held *filelock.ErrHeld
		if errors.As(err, &held) {
			return ValidatedExchange{}, fmt.Errorf("%w: writer lock held: %v", ErrCacheOperational, err)
		}
		return ValidatedExchange{}, fmt.Errorf("%w: acquiring writer lock: %v", ErrCacheOperational, err)
	}
	defer func() { _ = filelock.Release(lockFile, lockPath) }()

	created, existing, err := atomicfile.CreateImmutable(path, encoded, 0o644)
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: persisting judgment: %v", ErrCacheOperational, err)
	}
	if created {
		return ValidatedExchange{Exchange: exchange, RecordDigest: recordDigest}, nil
	}

	// Another process published this exact key before we acquired the
	// lock (authority design §7): accepted only after strict decode,
	// canonical re-encoding, path-key verification, and byte-identity with
	// what we ourselves validated; anything else is a collision.
	winner, err := DecodeJudgment(existing)
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: existing cache record at %s is malformed: %v", ErrCacheOperational, path, err)
	}
	if winner.TreeHash != treeHash || winner.InputDigest != bareKeyDigest {
		return ValidatedExchange{}, fmt.Errorf("%w: existing cache record at %s does not carry the expected path key", ErrCacheOperational, path)
	}
	if !bytes.Equal(existing, encoded) {
		return ValidatedExchange{}, fmt.Errorf("%w: existing cache record at %s is a different winner (content mismatch)", ErrCacheOperational, path)
	}
	return ValidatedExchange{Exchange: winner.Exchange, RecordDigest: recordDigest}, nil
}

// judgeCacheKeyDigest computes the D4 filename's bare-hex input-digest
// segment from every axis authority design §7 names.
func judgeCacheKeyDigest(adapter JudgeAdapter, promptBytes, inputBytes []byte, profileID, profileDigest, authorityDigest string) (string, error) {
	argvDigest := rawContentDigest([]byte(joinArgv(adapter.Argv)))
	full, err := canonjson.Digest(judgeCacheKeyDoc{
		Role:            JudgeRole(adapter.Role),
		AdapterID:       adapter.Adapter.ID,
		AdapterVersion:  adapter.Adapter.Version,
		Model:           adapter.Model,
		ArgvDigest:      argvDigest,
		PromptDigest:    rawContentDigest(promptBytes),
		InputDigest:     rawContentDigest(inputBytes),
		ProfileID:       profileID,
		ProfileDigest:   profileDigest,
		AuthorityDigest: authorityDigest,
	})
	if err != nil {
		return "", fmt.Errorf("digesting cache key: %w", err)
	}
	bare, ok := strippedDigest(full)
	if !ok {
		return "", fmt.Errorf("computed cache key digest %q is not sha256:<64 hex> form", full)
	}
	return bare, nil
}

func joinArgv(argv []string) string {
	var b bytes.Buffer
	for i, a := range argv {
		if i > 0 {
			b.WriteByte(0)
		}
		b.WriteString(a)
	}
	return b.String()
}

// strippedDigest strips the "sha256:" prefix from a full canonical digest,
// reporting false if full is not in that exact form.
func strippedDigest(full string) (string, bool) {
	const prefix = "sha256:"
	if len(full) != len(prefix)+64 || full[:len(prefix)] != prefix {
		return "", false
	}
	return full[len(prefix):], true
}

// loadCachedJudgment reports the cached Judgment at path if one exists and
// is valid: found=false, err=nil means a genuine miss (nothing at path
// yet); err!=nil means path names something that must never be silently
// treated as a miss or a hit — a symlink, a non-regular entry, a
// malformed/noncanonical record, or a record whose own path-key fields
// disagree with the path it was read from.
func loadCachedJudgment(path, treeHash, inputDigest string) (Judgment, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Judgment{}, false, nil
		}
		return Judgment{}, false, fmt.Errorf("%w: inspecting cache path %s: %v", ErrCacheOperational, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Judgment{}, false, fmt.Errorf("%w: refusing existing symlink at cache path %s", ErrCacheOperational, path)
	}
	if !info.Mode().IsRegular() {
		return Judgment{}, false, fmt.Errorf("%w: refusing existing non-regular entry at cache path %s (mode %s)", ErrCacheOperational, path, info.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Judgment{}, false, fmt.Errorf("%w: reading cache path %s: %v", ErrCacheOperational, path, err)
	}
	judgment, err := DecodeJudgment(data)
	if err != nil {
		return Judgment{}, false, fmt.Errorf("%w: cache record at %s is malformed: %v", ErrCacheOperational, path, err)
	}
	if judgment.TreeHash != treeHash || judgment.InputDigest != inputDigest {
		return Judgment{}, false, fmt.Errorf("%w: cache record at %s does not carry the expected path key", ErrCacheOperational, path)
	}
	return judgment, true, nil
}
