package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"testing/fstest"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentapp"
	"github.com/jyang234/verdi/internal/experimentdecision"
	"github.com/jyang234/verdi/internal/experimentevaluator"
	"github.com/jyang234/verdi/internal/experimentpolicy"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/specstate"
)

type experimentAcceptedGit struct{}

func (experimentAcceptedGit) ResolveDefaultBranch(ctx context.Context, root string) (experimentapp.DefaultBranch, error) {
	branch, ok := specstate.ResolveDefaultBranch(ctx, root)
	if !ok {
		return experimentapp.DefaultBranch{}, fmt.Errorf("experiment CLI: default branch is unresolved")
	}
	head, err := gitx.RevParse(ctx, root, branch.Ref)
	if err != nil {
		return experimentapp.DefaultBranch{}, err
	}
	return experimentapp.DefaultBranch{Name: branch.Name, Ref: branch.Ref, Head: head}, nil
}

func (experimentAcceptedGit) ListTree(ctx context.Context, root, commit string) ([]experimentapp.GitTreeEntry, error) {
	entries, err := gitx.LsTreeEntries(ctx, root, commit)
	if err != nil {
		return nil, err
	}
	result := make([]experimentapp.GitTreeEntry, len(entries))
	for index, entry := range entries {
		result[index] = experimentapp.GitTreeEntry{Mode: entry.Mode, Type: entry.Type, Object: entry.Object, Path: entry.Path}
	}
	return result, nil
}

func (experimentAcceptedGit) ReadBlob(ctx context.Context, root, commit, object, path string) ([]byte, error) {
	resolved, err := gitx.RevParse(ctx, root, commit+":"+path)
	if err != nil {
		return nil, err
	}
	if resolved != object {
		return nil, fmt.Errorf("experiment CLI: blob %s at %s resolved to %s, want %s", path, commit, resolved, object)
	}
	return gitx.Show(ctx, root, commit, path)
}

type experimentPolicyResolver struct{}

func (experimentPolicyResolver) ResolvePolicy(ctx context.Context, request experimentapp.PolicyRequest) (*experimentpolicy.Decision, error) {
	var store *policyauthority.Store
	var err error
	if request.AcceptedCommit == "" {
		store, err = policyauthority.Load(request.CheckoutRoot)
	} else {
		var source fs.FS
		source, err = experimentAcceptedTreeFS(ctx, request.CheckoutRoot, request.AcceptedCommit)
		if err == nil {
			store, err = policyauthority.LoadFromSource(source)
		}
	}
	if err != nil {
		return nil, err
	}
	effective, err := policyauthority.Resolve(store)
	if err != nil {
		return nil, err
	}
	selection, err := contextcompile.SelectApplicablePayloads(effective, experimentpolicy.PayloadKind, contextcompile.PayloadSelectionInput{
		Request:       policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		CandidatePath: request.ExperimentPath, CandidateRef: request.Spike,
		Phase: contextcompile.PhaseDesign, Environment: "local",
	})
	if err != nil {
		return nil, err
	}
	return experimentpolicy.Resolve(selection)
}

type experimentCapabilityDiscoverer struct{}

func (experimentCapabilityDiscoverer) DiscoverCapabilities(ctx context.Context, request experimentapp.CapabilityRequest) (experimentapp.CapabilityDiscovery, error) {
	if len(request.Definition.Evaluator.Argv) == 0 {
		return experimentapp.CapabilityDiscovery{}, fmt.Errorf("experiment CLI: evaluator argv is empty")
	}
	envRoot, err := os.MkdirTemp("", "verdi-experiment-env-")
	if err != nil {
		return experimentapp.CapabilityDiscovery{}, err
	}
	defer os.RemoveAll(envRoot)
	grants := execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantNetwork},
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{request.Definition.Evaluator.Argv[0]}},
		{Kind: execworkspace.GrantTimeouts, Seconds: 30},
	}}
	profile, _, err := execworkspace.BuildProfile(request.CheckoutRoot, envRoot, grants, map[string]string{})
	if err != nil {
		return experimentapp.CapabilityDiscovery{}, err
	}
	discovery, err := experimentevaluator.Discover(ctx, profile, experimentevaluator.DiscoverInput{
		Launch:             experimentevaluator.Launch{Directory: request.CheckoutRoot, Argv: request.Definition.Evaluator.Argv, Digest: request.Definition.Evaluator.Digest},
		CapabilitiesDigest: request.Definition.Evaluator.CapabilitiesDigest,
	})
	if err != nil {
		return experimentapp.CapabilityDiscovery{}, err
	}
	return experimentapp.CapabilityDiscovery{Bytes: discovery.Bytes}, nil
}

type experimentResultVerifier struct{}

func (experimentResultVerifier) VerifyResult(definition experiment.Definition, observations []experiment.Observation, receipt *experiment.ExecutionReceipt, result experiment.Result) error {
	return experimentdecision.VerifyResult(definition, observations, receipt, result)
}

func newExperimentService(root string) (*experimentapp.Service, error) {
	materializer, err := execworkspace.NewMaterializer(root, root, execworkspace.NewGitReconciler(root))
	if err != nil {
		return nil, err
	}
	runner, err := experimentapp.NewRunDelegate(experimentapp.RunDependencies{
		Materializer: materializer,
		Versions:     experiment.ReceiptVersions{Verdi: "dev", RecommendationEngine: string(experiment.AlgorithmV1)},
	})
	if err != nil {
		return nil, err
	}
	return experimentapp.NewService(experimentPolicyResolver{}, experimentAcceptedGit{}, experimentCapabilityDiscoverer{}, experimentResultVerifier{}, runner)
}

func experimentAcceptedTreeFS(ctx context.Context, root, commit string) (fs.FS, error) {
	entries, err := gitx.LsTreeEntries(ctx, root, commit)
	if err != nil {
		return nil, err
	}
	source := fstest.MapFS{}
	for _, entry := range entries {
		if entry.Type != "blob" {
			continue
		}
		// Preserve the accepted entry's file kind: a mode-120000 entry is a
		// symlink and must stay one so consumers apply their own refusal
		// (its blob data is the link target and is never followed here).
		var mode fs.FileMode
		switch entry.Mode {
		case "100644", "100755":
			mode = 0o444
		case "120000":
			mode = fs.ModeSymlink | 0o444
		default:
			return nil, fmt.Errorf("experiment CLI: accepted tree entry %s has unsupported blob mode %s", entry.Path, entry.Mode)
		}
		data, readErr := gitx.Show(ctx, root, commit, entry.Path)
		if readErr != nil {
			return nil, readErr
		}
		source[entry.Path] = &fstest.MapFile{Data: data, Mode: mode}
	}
	return source, nil
}
