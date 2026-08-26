package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lifecyclecountersign"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

type lifecycleCountersignResolver interface {
	Resolve(context.Context, lifecyclecountersign.Request) (lifecyclecountersign.Result, error)
}

func resolveLifecycleCountersign(ctx context.Context, resolver lifecycleCountersignResolver, root string, manifest *store.Manifest, mdl *model.Model, class, defaultBranch, head string) (lifecyclecountersign.Result, error) {
	if resolver == nil {
		return lifecyclecountersign.Result{}, fmt.Errorf("lifecycle countersign resolver is nil")
	}
	branch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		return lifecyclecountersign.Result{}, fmt.Errorf("resolving countersign source branch: %w", err)
	}
	return resolver.Resolve(ctx, lifecyclecountersign.Request{
		Root: root, Manifest: manifest, Model: mdl, TargetClass: class,
		DefaultBranch: defaultBranch, SourceBranch: branch, LocalCandidateSHA: head,
	})
}

func lifecycleCountersignCondition(number int, result lifecyclecountersign.Result) gateCondition {
	condition := gateCondition{Name: fmt.Sprintf("%d. forge countersign proven for current candidate", number)}
	if result.Verdict == countersign.VerdictProven && result.Record != nil {
		condition.OK = true
		condition.Extra = []string{fmt.Sprintf("       countersign record: %s", result.Record.Digest)}
		return condition
	}
	reason := fmt.Sprintf("countersign verdict is %s", result.Verdict)
	if len(result.Witnesses) > 0 {
		reason += "; witnesses: " + strings.Join(result.Witnesses, "; ")
	}
	condition.Reason = reason
	return condition
}

func countersignRollupProjection(result lifecyclecountersign.Result) (*artifact.RollupCountersign, error) {
	if result.Verdict != countersign.VerdictProven || result.Record == nil {
		return nil, fmt.Errorf("cannot project non-proven countersign verdict %q", result.Verdict)
	}
	record := result.Record
	projection := &artifact.RollupCountersign{
		RecordDigest: record.Digest, Verdict: string(record.Verdict),
		Approvals:            []artifact.RollupCountersignApproval{},
		EligibleApprovalIDs:  append([]string{}, record.Reduction.EligibleApprovalIDs...),
		DistinctPrincipalIDs: make([]string, len(record.Reduction.DistinctPrincipalIDs)),
		Witnesses:            append([]string{}, record.Witnesses...),
	}
	for i, id := range record.Reduction.DistinctPrincipalIDs {
		projection.DistinctPrincipalIDs[i] = string(id)
	}
	for _, approval := range record.Approvals {
		projection.Approvals = append(projection.Approvals, artifact.RollupCountersignApproval{
			ApprovalID: approval.ApprovalID, ApprovalRef: approval.ApprovalRef,
			PrincipalID:    string(approval.PrincipalResolution.PrincipalID),
			PrincipalState: string(approval.PrincipalResolution.State),
		})
	}
	return projection, nil
}
