package main

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"testing/fstest"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lifecyclecountersign"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// countersignPolicyDir is the constitution store's root inside any tree —
// the ONE subtree the selected governance profile can live in.
const countersignPolicyDir = ".verdi/policy"

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
	request := lifecyclecountersign.Request{
		Root: root, Manifest: manifest, Model: mdl, TargetClass: class,
		DefaultBranch: defaultBranch, SourceBranch: branch, LocalCandidateSHA: head,
	}
	// Governance authority is acceptance truth (I-121): pin the selected
	// profile to the exact Git tree at the SAME resolved default branch
	// acceptance already reads, never to this mutable checkout — which the
	// policy-conflict adoption probe keeps reading, unchanged, because it
	// asks the different question of what the candidate being mutated has
	// adopted. A default branch that resolves to no ref, or a ref that
	// resolves to no commit, leaves the operand absent so the reducer
	// classifies it unproven; only a resolved commit whose tree cannot be
	// read is operational.
	if accepted, ok := specstate.ResolveDefaultBranch(ctx, root); ok {
		if commit, revErr := gitx.RevParse(ctx, root, accepted.Ref); revErr == nil {
			source, err := countersignAcceptedPolicySource(ctx, root, commit)
			if err != nil {
				return lifecyclecountersign.Result{}, fmt.Errorf("reading accepted governance tree at %s: %w", commit, err)
			}
			request.AcceptedBranch, request.AcceptedCommit, request.AcceptedProfileSource = accepted.Name, commit, source
		}
	}
	return resolver.Resolve(ctx, request)
}

// countersignAcceptedPolicySource materializes the accepted tree's
// constitution-store subtree at commit as a read-only fs.FS, following
// experimentAcceptedTreeFS's entry-kind discipline (a mode-120000 entry
// stays a symlink so policyauthority applies its own refusal instead of
// this adapter silently following a link out of the accepted tree) but
// scoped to .verdi/policy/: every lifecycle gate calls this, and reading
// every blob in the repository to decode one profile is not a cost a gate
// can carry.
func countersignAcceptedPolicySource(ctx context.Context, root, commit string) (fs.FS, error) {
	entries, err := gitx.LsTreeEntries(ctx, root, commit)
	if err != nil {
		return nil, err
	}
	source := fstest.MapFS{}
	for _, entry := range entries {
		if entry.Type != "blob" || !countersignPolicyPath(entry.Path) {
			continue
		}
		var mode fs.FileMode
		switch entry.Mode {
		case "100644", "100755":
			mode = 0o444
		case "120000":
			mode = fs.ModeSymlink | 0o444
		default:
			return nil, fmt.Errorf("accepted tree entry %s has unsupported blob mode %s", entry.Path, entry.Mode)
		}
		data, readErr := gitx.Show(ctx, root, commit, entry.Path)
		if readErr != nil {
			return nil, readErr
		}
		source[entry.Path] = &fstest.MapFile{Data: data, Mode: mode}
	}
	return source, nil
}

// countersignPolicyPath reports whether path is the constitution store
// root itself (a symlink entry there must survive into the source so it
// is refused, never skipped) or a file inside it.
func countersignPolicyPath(path string) bool {
	return path == countersignPolicyDir || strings.HasPrefix(path, countersignPolicyDir+"/")
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
