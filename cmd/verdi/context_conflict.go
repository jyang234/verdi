package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/store"
)

type contextConflictProviderFactory func(context.Context, string, policyconflict.Request) (policyconflict.VerdictProvider, error)

// cmdContextConflict exposes the one Task-9 evaluator as a read-only CLI
// inspection surface. Dependency construction stays behind a narrow factory
// seam so parser/output behavior can be exercised without replacing package
// globals or bypassing the real request codec.
func cmdContextConflict(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return cmdContextConflictWithFactory(args, stdin, stdout, stderr, newLocalContextConflictProvider)
}

func cmdContextConflictWithFactory(args []string, stdin io.Reader, stdout, stderr io.Writer, factory contextConflictProviderFactory) int {
	ctx := context.Background()
	requestArg, hasRequest, outArg, hasOut, rest, err := extractContextCompileFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, "context conflict:", err)
		return 2
	}
	if len(rest) != 0 {
		fmt.Fprintln(stderr, "context conflict: unexpected positional argument(s):", rest)
		return 2
	}
	if !hasRequest || requestArg == "" {
		fmt.Fprintln(stderr, "context conflict: --request is required")
		return 2
	}
	if hasOut && outArg == "" {
		fmt.Fprintln(stderr, "context conflict: --out requires a value")
		return 2
	}
	if hasOut && hasDotDotElement(outArg) {
		fmt.Fprintln(stderr, "context conflict:", errContextOutDotDot)
		return 2
	}
	if hasOut && requestArg != "-" && sameFileArg(requestArg, outArg) {
		fmt.Fprintln(stderr, "context conflict: --request and --out must not name the same path")
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "context conflict:", err)
		return 2
	}

	var outCanon string
	if hasOut {
		outCanon, err = canonicalOutPath(root, outArg)
		if err != nil {
			printContextCommandDiagnostic(stderr, "conflict", root, err)
			return 2
		}
		if err := validateContextOutputStoreZone(root, outCanon, requestArg); err != nil {
			printContextCommandDiagnostic(stderr, "conflict", root, err)
			return 2
		}
	}

	data, err := readContextRequest(requestArg, stdin)
	if err != nil {
		printContextCommandDiagnostic(stderr, "conflict", root, err)
		return 2
	}
	request, err := policyconflict.DecodeRequest(data)
	if err != nil {
		printContextCommandDiagnostic(stderr, "conflict", root, err)
		return 2
	}
	if factory == nil {
		printContextCommandDiagnostic(stderr, "conflict", root, errors.New("provider factory is nil"))
		return 2
	}
	provider, err := factory(ctx, root, request)
	if err != nil {
		printContextCommandDiagnostic(stderr, "conflict", root, err)
		return 2
	}
	if provider == nil {
		printContextCommandDiagnostic(stderr, "conflict", root, errors.New("provider is nil"))
		return 2
	}

	result, err := provider.Evaluate(ctx, request)
	if err != nil {
		printContextCommandDiagnostic(stderr, "conflict", root, err)
		if policyconflict.IsNotAdopted(err) {
			return 1
		}
		return 2
	}
	exit, err := contextConflictVerdictExit(result.Report.Verdict)
	if err != nil {
		printContextCommandDiagnostic(stderr, "conflict", root, err)
		return 2
	}
	if len(result.ReportBytes) == 0 {
		printContextCommandDiagnostic(stderr, "conflict", root, errors.New("provider returned an empty completed report"))
		return 2
	}

	if !hasOut {
		if _, err := stdout.Write(result.ReportBytes); err != nil {
			printContextCommandDiagnostic(stderr, "conflict", root, fmt.Errorf("writing report to stdout: %w", err))
			return 2
		}
		return exit
	}

	managed, err := contextConflictManagedProjectionPaths(root)
	if err != nil {
		printContextCommandDiagnostic(stderr, "conflict", root, err)
		return 2
	}
	if err := validateContextOutputProjectionPaths(root, outCanon, managed); err != nil {
		printContextCommandDiagnostic(stderr, "conflict", root, err)
		return 2
	}
	if err := atomicfile.Write(outCanon, result.ReportBytes, 0o644); err != nil {
		printContextCommandDiagnostic(stderr, "conflict", root, fmt.Errorf("writing report: %w", err))
		return 2
	}
	return exit
}

func readContextRequest(requestArg string, stdin io.Reader) ([]byte, error) {
	if requestArg == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("reading request: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(requestArg)
	if err != nil {
		return nil, fmt.Errorf("reading request: %w", err)
	}
	return data, nil
}

func contextConflictVerdictExit(verdict policyconflict.Verdict) (int, error) {
	switch verdict {
	case policyconflict.VerdictPass:
		return 0, nil
	case policyconflict.VerdictBlockedViolated, policyconflict.VerdictBlockedUnproven:
		return 1, nil
	default:
		return 0, fmt.Errorf("provider returned unknown verdict %q", verdict)
	}
}

func contextConflictManagedProjectionPaths(root string) ([]string, error) {
	policyStore, err := policyauthority.Load(root)
	if err != nil {
		return nil, fmt.Errorf("resolving managed output fence: %w", err)
	}
	paths, err := instructionprojection.ManagedPaths(policyStore.Constitution.Adapters)
	if err != nil {
		return nil, fmt.Errorf("resolving managed output fence: %w", err)
	}
	return paths, nil
}

func newLocalContextConflictProvider(ctx context.Context, root string, request policyconflict.Request) (policyconflict.VerdictProvider, error) {
	manifest, err := loadManifest(root)
	if err != nil {
		return nil, err
	}
	var primary policyconflict.Judge
	if manifest.Align != nil && len(manifest.Align.JudgeCmd) != 0 {
		timeout := align.DefaultJudgeTimeout
		if manifest.Align.JudgeTimeoutSeconds != 0 {
			timeout = time.Duration(manifest.Align.JudgeTimeoutSeconds) * time.Second
		}
		primary = policyconflict.JudgeAdapter{
			Role:    string(policyconflict.JudgePrimary),
			Adapter: contextConflictRequestAdapter(request),
			Model:   "align.judge_cmd",
			Argv:    append([]string(nil), manifest.Align.JudgeCmd...),
			Timeout: timeout,
			Root:    root,
			Runner:  contextConflictJudgeRunner{delegate: align.ExecJudgeRunner{}},
		}
	}
	actors, err := resolveConflictActors(ctx, root)
	if err != nil {
		return nil, err
	}
	return policyconflict.NewService(root, policyconflict.ServiceDeps{
		Compiler:   contextcompile.NewCompiler(),
		Refs:       contextConflictRefResolver{},
		Primary:    primary,
		TreeHasher: contextConflictTreeHasher{},
		Dates:      contextConflictDateSource{},
		Actors:     actors,
	}), nil
}

// resolveConflictActors resolves the store's local-operator actor claim
// (actorlocal.go) against the resolved governance profile, wired once here
// so `context conflict`, `build start`, `gate`, and `close` all share it
// through this one factory (2026-09-05 local-operator disposition design
// §2.2, ledger SI-178). A store that has not yet adopted a constitution
// carries no profile to consult, so it resolves no actors at all — exactly
// today's Actors: nil — and is left for Evaluate's own adoption probe to
// report as NotAdoptedError (exit 1), never preempted by an operational
// failure here (mirrors policyconflict.ProbeAdoption's own
// ErrNotAdopted-only distinction). Every other policyauthority.Load or
// profile-resolution failure (a present-but-incomplete or malformed
// store) propagates as an operational error, matching what Evaluate would
// eventually report for the same store anyway.
func resolveConflictActors(ctx context.Context, root string) ([]governanceprincipal.PrincipalResolution, error) {
	policyStore, err := policyauthority.Load(root)
	if err != nil {
		if errors.Is(err, policyauthority.ErrNotAdopted) {
			return nil, nil
		}
		return nil, err
	}
	profile, err := policyStore.SelectedProfile()
	if err != nil {
		return nil, err
	}
	return resolveLocalActors(ctx, root, profile)
}

func contextConflictRequestAdapter(request policyconflict.Request) contextcompile.AdapterRef {
	if request.Target.AcceptedContext != nil {
		return request.Target.AcceptedContext.Adapter
	}
	if request.Target.AcceptanceCandidate != nil {
		return request.Target.AcceptanceCandidate.Adapter
	}
	return contextcompile.AdapterRef{}
}

type contextConflictJudgeRunner struct{ delegate align.JudgeRunner }

func (r contextConflictJudgeRunner) Run(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
	if r.delegate == nil {
		return nil, 0, errors.New("context conflict judge runner is nil")
	}
	result, err := r.delegate.RunJudge(ctx, argv, stdin)
	return result.Stdout, result.ExitCode, err
}

type contextConflictTreeHasher struct{}

func (contextConflictTreeHasher) TreeHash(ctx context.Context, root string) (string, error) {
	services, err := store.DiscoverServices(root)
	if err != nil {
		return "", err
	}
	return store.TreeHash(ctx, root, services)
}

type contextConflictDateSource struct{}

func (contextConflictDateSource) TodayUTC(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return time.Now().UTC().Format("2006-01-02"), nil
}

// contextConflictRefResolver makes absent local graph proof explicit. Exact
// ref equality is settled before this port is called; every different pair
// remains unknown and is therefore sent to semantic evaluation, never treated
// as favorable overlap/disjointness. Managed callers may inject a stronger
// graph resolver directly into ServiceDeps.
type contextConflictRefResolver struct{}

func (contextConflictRefResolver) Relate(context.Context, string, string) (policyconflict.ScopeState, []string, error) {
	return policyconflict.ScopeUnknown, []string{"ref-relation-unproven"}, nil
}

func (contextConflictRefResolver) Covers(context.Context, string, string) (policyconflict.ProofState, []string, error) {
	return policyconflict.ProofUnproven, []string{"ref-coverage-unproven"}, nil
}
