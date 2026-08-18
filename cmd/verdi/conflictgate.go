package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyconflict"
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
	if err := rejectConflictRequestSymlinks(root, input.RequestPath); err != nil {
		return conflictGateResult{}, err
	}
	data, err := os.ReadFile(input.RequestPath)
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

// rejectConflictRequestSymlinks refuses both a linked request file and any
// linked caller-selected ancestor. Paths inside the checkout start at the
// already-resolved store root, avoiding false positives from platform-level
// aliases above the checkout while still proving every request component.
func rejectConflictRequestSymlinks(root, requestPath string) error {
	requestAbs, err := filepath.Abs(requestPath)
	if err != nil {
		return fmt.Errorf("resolving --context-request path: %w", err)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolving store root: %w", err)
	}
	rootInfo, err := os.Stat(rootAbs)
	if err != nil {
		return fmt.Errorf("inspecting store root: %w", err)
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
				return nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return errors.New("--context-request must not contain a symlink path component")
			}
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("inspecting --context-request path: %w", statErr)
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
				return nil
			}
			return fmt.Errorf("inspecting --context-request path: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("--context-request must not contain a symlink path component")
		}
	}
	return nil
}

// localLifecycleConflictProvider preserves the VerdictProvider boundary while
// reusing Task 10's exact request-aware production dependency construction.
type localLifecycleConflictProvider struct{ root string }

func (p localLifecycleConflictProvider) Evaluate(ctx context.Context, request policyconflict.Request) (policyconflict.Result, error) {
	provider, err := newLocalContextConflictProvider(p.root, request)
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
