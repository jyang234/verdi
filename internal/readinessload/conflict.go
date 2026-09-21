// conflict.go is the one home for policy-conflict provider construction
// (moved from cmd/verdi/context_conflict.go's old
// newLocalContextConflictProvider) and for the --context-request path
// safety check (moved from cmd/verdi/conflictgate.go's old
// validatedConflictRequestPath). cmd/verdi's context-conflict-evaluating
// verbs call NewConflictProvider/ValidatedContextRequestPath directly
// rather than keeping a second copy.
package readinessload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/store"
)

// NewConflictProvider constructs the one production policy-conflict
// VerdictProvider every lifecycle surface shares: cmd/verdi's `context
// conflict`, `build start`, `gate`, and `close` verbs (always JudgeRun) and
// this package's own Load (JudgeCacheOnly for a per-request derivation,
// JudgeRun only for serve's startup pre-run). Under JudgeRun the primary
// judge is the manifest's configured align.judge_cmd process, exactly as
// cmd/verdi's old newLocalContextConflictProvider built it; under
// JudgeCacheOnly it is policyconflict.NewCacheOnlyJudge wrapping the
// identical adapter, so a request never executes the judge
// (spec/readiness-recovery ac-3) — R-RR1-4's cache key is therefore
// computed from the exact same adapter identity either way. actors
// resolves the store's opt-in local-operator actor claim (nil resolves
// none, today's exact behavior for a store that declares none); cmd/verdi
// supplies its own resolveConflictActors (actorlocal.go).
func NewConflictProvider(ctx context.Context, root string, request policyconflict.Request, mode JudgeMode, actors ActorsResolver) (policyconflict.VerdictProvider, error) {
	cfg, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	manifest := cfg.Manifest

	var primary policyconflict.Judge
	if manifest.Align != nil && len(manifest.Align.JudgeCmd) != 0 {
		timeout := align.DefaultJudgeTimeout
		if manifest.Align.JudgeTimeoutSeconds != 0 {
			timeout = time.Duration(manifest.Align.JudgeTimeoutSeconds) * time.Second
		}
		adapter := policyconflict.JudgeAdapter{
			Role:    string(policyconflict.JudgePrimary),
			Adapter: requestAdapter(request),
			Model:   "align.judge_cmd",
			Argv:    append([]string(nil), manifest.Align.JudgeCmd...),
			Timeout: timeout,
			Root:    root,
			Runner:  execJudgeRunner{delegate: align.ExecJudgeRunner{}},
		}
		if mode.normalize() == JudgeRun {
			primary = adapter
		} else {
			primary = policyconflict.NewCacheOnlyJudge(adapter)
		}
	}

	var resolvedActors []governanceprincipal.PrincipalResolution
	if actors != nil {
		resolvedActors, err = actors(ctx, root)
		if err != nil {
			return nil, err
		}
	}

	return policyconflict.NewService(root, policyconflict.ServiceDeps{
		Compiler:   contextcompile.NewCompiler(),
		Refs:       conflictRefResolver{},
		Primary:    primary,
		TreeHasher: conflictTreeHasher{},
		Dates:      conflictDateSource{},
		Actors:     resolvedActors,
	}), nil
}

func requestAdapter(request policyconflict.Request) contextcompile.AdapterRef {
	if request.Target.AcceptedContext != nil {
		return request.Target.AcceptedContext.Adapter
	}
	if request.Target.AcceptanceCandidate != nil {
		return request.Target.AcceptanceCandidate.Adapter
	}
	return contextcompile.AdapterRef{}
}

type execJudgeRunner struct{ delegate align.JudgeRunner }

func (r execJudgeRunner) Run(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
	if r.delegate == nil {
		return nil, 0, errors.New("readinessload: judge runner is nil")
	}
	result, err := r.delegate.RunJudge(ctx, argv, stdin)
	return result.Stdout, result.ExitCode, err
}

type conflictTreeHasher struct{}

func (conflictTreeHasher) TreeHash(ctx context.Context, root string) (string, error) {
	services, err := store.DiscoverServices(root)
	if err != nil {
		return "", err
	}
	return store.TreeHash(ctx, root, services)
}

type conflictDateSource struct{}

func (conflictDateSource) TodayUTC(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return time.Now().UTC().Format("2006-01-02"), nil
}

// conflictRefResolver makes absent local graph proof explicit. Exact ref
// equality is settled before this port is called; every different pair
// remains unknown and is therefore sent to semantic evaluation, never
// treated as favorable overlap/disjointness. Managed callers may inject a
// stronger graph resolver directly into ServiceDeps.
type conflictRefResolver struct{}

func (conflictRefResolver) Relate(context.Context, string, string) (policyconflict.ScopeState, []string, error) {
	return policyconflict.ScopeUnknown, []string{"ref-relation-unproven"}, nil
}

func (conflictRefResolver) Covers(context.Context, string, string) (policyconflict.ProofState, []string, error) {
	return policyconflict.ProofUnproven, []string{"ref-coverage-unproven"}, nil
}

// ValidatedContextRequestPath returns the one absolute request path used by
// both validation and reading (moved verbatim from
// cmd/verdi/conflictgate.go's old validatedConflictRequestPath). It refuses
// traversal elements before Abs can collapse them lexically: the kernel
// follows a symlink before applying "..", so validating the cleaned
// spelling but reading the original could select a different file. It also
// refuses a linked request file or caller-selected ancestor. Paths inside
// the checkout start at the already-resolved store root, avoiding false
// positives from platform-level aliases above the checkout.
func ValidatedContextRequestPath(root, requestPath string) (string, error) {
	if store.HasDotDotElement(requestPath) {
		return "", errors.New(`--context-request must not contain a ".." path element`)
	}
	requestAbs, err := filepath.Abs(requestPath)
	if err != nil {
		return "", fmt.Errorf("resolving --context-request path: %w", err)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolving store root: %w", err)
	}
	rootInfo, err := os.Stat(rootAbs)
	if err != nil {
		return "", fmt.Errorf("inspecting store root: %w", err)
	}

	// Walk upward from the caller-selected file until the physical store root
	// inode is reached. This checks every selectable component below the root,
	// but deliberately stops before platform aliases above it (macOS commonly
	// exposes /var through /private/var). A lexical Rel check cannot distinguish
	// that harmless host alias from a symlink selected inside the checkout.
	current := requestAbs
	for {
		info, statErr := os.Lstat(current)
		if statErr == nil {
			if os.SameFile(rootInfo, info) {
				return requestAbs, nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return "", errors.New("--context-request must not contain a symlink path component")
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return "", fmt.Errorf("inspecting --context-request path: %w", statErr)
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	start := filepath.Clean(string(filepath.Separator))
	var remainder string
	if rel, relErr := filepath.Rel(rootAbs, requestAbs); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		start = rootAbs
		remainder = rel
	} else if volume := filepath.VolumeName(requestAbs); volume != "" {
		start = volume + string(filepath.Separator)
		remainder = strings.TrimPrefix(requestAbs, start)
	} else {
		remainder = strings.TrimPrefix(requestAbs, string(filepath.Separator))
	}

	current = start
	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return requestAbs, nil
			}
			return "", fmt.Errorf("inspecting --context-request path: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("--context-request must not contain a symlink path component")
		}
	}
	return requestAbs, nil
}

// ContextRequestSpec safely validates path (refusing ".." and symlink path
// components, exactly as Load's own request handling does), reads it
// exactly once, and decodes it — reporting the request's declared target
// spec ref (Load's own required ref parameter) alongside a Predecoded
// bundle of the exact bytes and decoded value. cmd/verdi's serve startup
// pre-run uses ref to learn which spec to warm before calling Load, and
// passes predecoded back through Options.PredecodedRequest so Load does not
// read the same file a second time (fix round 1, Minor 6: "exactly one
// canonical read" now holds per startup, not merely per Load call — see
// TestContextRequestSpec_PredecodedRequestAvoidsASecondRead). A caller that
// only needs ref may discard predecoded.
func ContextRequestSpec(root, path string) (ref string, predecoded *PredecodedRequest, err error) {
	if path == "-" {
		return "", nil, errors.New("readinessload: --context-request does not accept stdin ('-')")
	}
	validated, err := ValidatedContextRequestPath(root, path)
	if err != nil {
		return "", nil, fmt.Errorf("readinessload: %w", err)
	}
	data, err := os.ReadFile(validated)
	if err != nil {
		return "", nil, fmt.Errorf("readinessload: reading --context-request: %w", err)
	}
	request, err := contextcompile.DecodeRequest(data)
	if err != nil {
		return "", nil, fmt.Errorf("readinessload: decoding --context-request: %w", err)
	}
	return request.Spec, &PredecodedRequest{Bytes: data, Request: request}, nil
}
