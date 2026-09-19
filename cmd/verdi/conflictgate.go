package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/readinessload"
)

type conflictGateInput struct {
	RequestPath string
	Phase       contextcompile.Phase
	Spec        string
	Candidate   bool
	Branch      string
	Head        string
}

type conflictGateResult struct {
	Adopted bool
	Result  policyconflict.Result
}

// extractConflictRequestFlag removes the one additive lifecycle request flag
// without reading it. The remaining arguments retain their original order so
// each legacy verb's existing parser remains the sole owner of its grammar.
func extractConflictRequestFlag(args []string) (string, []string, error) {
	var requestPath string
	found := false
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] != "--context-request" {
			rest = append(rest, args[i])
			continue
		}
		if found {
			return "", nil, errors.New("--context-request may be supplied only once")
		}
		found = true
		if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
			return "", nil, errors.New("--context-request requires a filesystem path")
		}
		i++
		requestPath = args[i]
		if requestPath == "-" {
			return "", nil, errors.New("--context-request does not accept stdin ('-')")
		}
	}
	return requestPath, rest, nil
}

// runConflictGate is the one lifecycle request loader and adapter. It probes
// adoption before touching caller operands, strict-decodes only through the
// context compiler seam, replaces the optional expected claim with computed
// lifecycle facts, and invokes the one policy-conflict provider port.
func runConflictGate(ctx context.Context, root string, input conflictGateInput, provider policyconflict.VerdictProvider) (conflictGateResult, error) {
	adopted, err := probeConflictGate(root, input.RequestPath)
	if err != nil {
		return conflictGateResult{}, err
	}
	if !adopted {
		return conflictGateResult{Adopted: false}, nil
	}
	if input.RequestPath == "-" {
		return conflictGateResult{}, errors.New("--context-request does not accept stdin ('-')")
	}
	requestPath, err := readinessload.ValidatedContextRequestPath(root, input.RequestPath)
	if err != nil {
		return conflictGateResult{}, err
	}
	data, err := os.ReadFile(requestPath)
	if err != nil {
		return conflictGateResult{}, fmt.Errorf("reading --context-request: %w", err)
	}
	request, err := contextcompile.DecodeRequest(data)
	if err != nil {
		return conflictGateResult{}, fmt.Errorf("decoding --context-request: %w", err)
	}
	if request.Phase != input.Phase {
		return conflictGateResult{}, fmt.Errorf("--context-request phase %q does not match lifecycle phase %q", request.Phase, input.Phase)
	}
	if request.Spec != input.Spec {
		return conflictGateResult{}, fmt.Errorf("--context-request spec %q does not match lifecycle spec %q", request.Spec, input.Spec)
	}

	computed := contextcompile.Expected{Branch: input.Branch, Head: input.Head}
	if request.Expected != nil && *request.Expected != computed {
		return conflictGateResult{}, fmt.Errorf("--context-request expected repository %+v does not match computed repository %+v", *request.Expected, computed)
	}
	request.Expected = &computed

	conflictRequest := policyconflict.Request{Schema: policyconflict.RequestSchema}
	if input.Candidate {
		if input.Phase != contextcompile.PhaseDesign {
			return conflictGateResult{}, fmt.Errorf("acceptance-candidate lifecycle target requires phase %q, got %q", contextcompile.PhaseDesign, input.Phase)
		}
		conflictRequest.Target = policyconflict.Target{
			Kind: policyconflict.TargetAcceptanceCandidate,
			AcceptanceCandidate: &policyconflict.AcceptanceCandidate{
				Adapter:  request.Adapter,
				Expected: computed,
				Grants:   request.Grants,
				Scope:    request.Scope,
				Spec:     request.Spec,
			},
		}
	} else {
		conflictRequest.Target = policyconflict.Target{
			Kind:            policyconflict.TargetAcceptedContext,
			AcceptedContext: &request,
		}
	}
	if err := conflictRequest.Validate(); err != nil {
		return conflictGateResult{}, fmt.Errorf("constructing lifecycle conflict request: %w", err)
	}
	if provider == nil {
		return conflictGateResult{}, errors.New("policy-conflict provider is nil")
	}
	result, err := provider.Evaluate(ctx, conflictRequest)
	if err != nil {
		return conflictGateResult{}, fmt.Errorf("evaluating policy conflicts: %w", err)
	}
	if _, err := contextConflictVerdictExit(result.Report.Verdict); err != nil {
		return conflictGateResult{}, err
	}
	return conflictGateResult{Adopted: true, Result: result}, nil
}

// probeConflictGate owns the adoption/flag compatibility decision used by the
// adapter and by wrappers that must preserve an older parser's exact legacy
// output before resolving target operands.
func probeConflictGate(root, requestPath string) (bool, error) {
	adopted, err := policyconflict.ProbeAdoption(root)
	if err != nil {
		return false, fmt.Errorf("probing policy-conflict adoption: %w", err)
	}
	if !adopted {
		if requestPath != "" {
			return false, errors.New("--context-request is invalid before constitution adoption")
		}
		return false, nil
	}
	if requestPath == "" {
		return false, errors.New("--context-request is required after constitution adoption")
	}
	return true, nil
}

// localLifecycleConflictProvider preserves the VerdictProvider boundary while
// reusing Task 10's exact request-aware production dependency construction.
type localLifecycleConflictProvider struct{ root string }

func (p localLifecycleConflictProvider) Evaluate(ctx context.Context, request policyconflict.Request) (policyconflict.Result, error) {
	// The lifecycle gate is a JudgeRun caller (`build start`, `gate`,
	// `close`), exactly like `verdi context conflict` — provider
	// construction moved to internal/readinessload.NewConflictProvider
	// (spec/readiness-recovery Task 2) so there is one home for it.
	provider, err := readinessload.NewConflictProvider(ctx, p.root, request, readinessload.JudgeRun, resolveConflictActors)
	if err != nil {
		return policyconflict.Result{}, err
	}
	return provider.Evaluate(ctx, request)
}

func conflictCondition(result policyconflict.Result) gateCondition {
	reasons, witnesses := conflictSummaryValues(result.Report)
	condition := gateCondition{
		Name: "constitutional conflict verdict",
		OK:   result.Report.Verdict == policyconflict.VerdictPass,
	}
	if condition.OK {
		condition.Extra = append(condition.Extra, "       state: "+string(result.Report.Verdict))
	} else {
		condition.Reason = "state: " + string(result.Report.Verdict)
	}
	condition.Extra = append(condition.Extra, "       report digest: "+result.Report.Digest)
	if len(reasons) != 0 {
		condition.Extra = append(condition.Extra, fmt.Sprintf("       reasons: %v", reasons))
	}
	if len(witnesses) != 0 {
		condition.Extra = append(condition.Extra, fmt.Sprintf("       witness IDs: %v", witnesses))
	}
	return condition
}

func renderConflictSummary(w io.Writer, result policyconflict.Result) {
	reasons, witnesses := conflictSummaryValues(result.Report)
	fmt.Fprintf(w, "constitutional conflict: state: %s\n", result.Report.Verdict)
	fmt.Fprintf(w, "constitutional conflict: report digest: %s\n", result.Report.Digest)
	if len(reasons) != 0 {
		fmt.Fprintf(w, "constitutional conflict: reasons: %v\n", reasons)
	}
	if len(witnesses) != 0 {
		fmt.Fprintf(w, "constitutional conflict: witness IDs: %v\n", witnesses)
	}
}

func conflictSummaryValues(report policyconflict.Report) ([]string, []string) {
	reasonSet := make(map[string]struct{})
	witnessSet := make(map[string]struct{})
	for _, row := range report.Mechanical {
		if row.ID != "" {
			witnessSet[row.ID] = struct{}{}
		}
		for _, reason := range row.Reasons {
			reasonSet[string(reason)] = struct{}{}
		}
	}
	for _, row := range report.Semantic {
		if row.ID != "" {
			witnessSet[row.ID] = struct{}{}
		}
		for _, reason := range row.Reasons {
			reasonSet[string(reason)] = struct{}{}
		}
	}
	for _, disclosure := range report.Disclosures {
		reasonSet[string(disclosure.Code)] = struct{}{}
		for _, witness := range disclosure.Witnesses {
			witnessSet[witness] = struct{}{}
		}
	}
	reasons := make([]string, 0, len(reasonSet))
	for reason := range reasonSet {
		reasons = append(reasons, reason)
	}
	witnesses := make([]string, 0, len(witnessSet))
	for witness := range witnessSet {
		witnesses = append(witnesses, witness)
	}
	sort.Strings(reasons)
	sort.Strings(witnesses)
	return reasons, witnesses
}
