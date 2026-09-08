package constitutionapp

import (
	"context"
	"fmt"
	"time"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/store"
)

// ConflictEvidence is one existing accepted-context compiler result paired
// with the existing conflict gate's result for the identical request.
type ConflictEvidence struct {
	AcceptedManifestBytes []byte
	Result                policyconflict.Result
}

// ConflictEvaluator is constitutionapp's consumer-owned port over the one
// existing context compiler and conflict gate. The manifest bytes are needed
// by constitutionimpact to bind each result to its exact accepted-context
// operands; this package never reconstructs those bytes from a report.
type ConflictEvaluator interface {
	Evaluate(ctx context.Context, root string, request policyconflict.Request) (ConflictEvidence, error)
}

// localConflictEvaluator constructs one internal/policyconflict.Service per
// call, rooted at the caller-supplied root, mirroring
// cmd/verdi/context_conflict.go's own newLocalContextConflictProvider
// wiring exactly (real compiler, a conservative "unproven" ref-relation
// resolver, the real D4 tree hasher, a real UTC date source). Primary and
// Challenger stay nil in v1: this package supplies no manifest-driven
// align.judge_cmd wiring of its own (a narrow, disclosed limitation — every
// semantic-evaluation-required target therefore reports
// policyconflict.ReasonJudgeUnavailable, a legitimate three-valued-unproven
// outcome policyconflict itself already defines, never a fabricated pass).
type localConflictEvaluator struct{}

func (localConflictEvaluator) Evaluate(ctx context.Context, root string, request policyconflict.Request) (ConflictEvidence, error) {
	if request.Target.Kind != policyconflict.TargetAcceptedContext || request.Target.AcceptedContext == nil {
		return ConflictEvidence{}, fmt.Errorf("constitutionapp: conflict evidence requires an accepted-context request")
	}
	compiled, err := contextcompile.NewCompiler().Compile(ctx, root, *request.Target.AcceptedContext)
	if err != nil {
		return ConflictEvidence{}, err
	}
	svc := policyconflict.NewService(root, policyconflict.ServiceDeps{
		Compiler:   contextcompile.NewCompiler(),
		Refs:       constitutionRefResolver{},
		TreeHasher: constitutionTreeHasher{},
		Dates:      constitutionDateSource{},
	})
	result, err := svc.Evaluate(ctx, request)
	if err != nil {
		return ConflictEvidence{}, err
	}
	return ConflictEvidence{
		AcceptedManifestBytes: append([]byte(nil), compiled.ManifestBytes...),
		Result:                result,
	}, nil
}

// constitutionRefResolver makes absent local graph proof explicit — every
// ref pair is unknown, never a favorable overlap/disjointness guess.
// Byte-identical posture to cmd/verdi/context_conflict.go's own
// contextConflictRefResolver.
type constitutionRefResolver struct{}

func (constitutionRefResolver) Relate(context.Context, string, string) (policyconflict.ScopeState, []string, error) {
	return policyconflict.ScopeUnknown, []string{"ref-relation-unproven"}, nil
}

func (constitutionRefResolver) Covers(context.Context, string, string) (policyconflict.ProofState, []string, error) {
	return policyconflict.ProofUnproven, []string{"ref-coverage-unproven"}, nil
}

type constitutionTreeHasher struct{}

func (constitutionTreeHasher) TreeHash(ctx context.Context, root string) (string, error) {
	services, err := store.DiscoverServices(root)
	if err != nil {
		return "", err
	}
	return store.TreeHash(ctx, root, services)
}

type constitutionDateSource struct{}

func (constitutionDateSource) TodayUTC(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return time.Now().UTC().Format("2006-01-02"), nil
}
