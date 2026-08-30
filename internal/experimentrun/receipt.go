package experimentrun

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
)

// ReceiptInput is the complete already-validated execution boundary from
// which one durable CSE receipt is derived. It deliberately has no process,
// workspace, or persistence capability.
type ReceiptInput struct {
	Definition        experiment.Definition
	Run               string
	Capabilities      experiment.Capabilities
	CapabilitiesBytes []byte
	Authorization     AuthorizedExecution
	Inputs            ResolvedInputs
	CandidatePatches  map[string][]byte
	Fingerprint       []byte
	Enforcement       execworkspace.EnforcementReport
	Versions          experiment.ReceiptVersions
}

type hostRuntimeFacts struct {
	os             string
	arch           string
	runtimeVersion string
}

// CandidateReceipts derives sorted receipt candidate rows from the registered
// candidates and their exact raw patch bytes.
func CandidateReceipts(def experiment.Definition, experimentDigest, run string, patches map[string][]byte) ([]experiment.ReceiptCandidate, error) {
	if err := def.Validate(); err != nil {
		return nil, fmt.Errorf("experimentrun: candidate receipts definition: %w", err)
	}
	if err := experiment.ValidateDigest(experimentDigest); err != nil {
		return nil, fmt.Errorf("experimentrun: candidate receipts experiment digest: %w", err)
	}
	if err := experiment.ValidateID(run); err != nil {
		return nil, fmt.Errorf("experimentrun: candidate receipts run: %w", err)
	}
	if len(patches) != len(def.Candidates) {
		return nil, fmt.Errorf("experimentrun: candidate patches have %d entries, want exactly %d", len(patches), len(def.Candidates))
	}
	registered := make(map[string]experiment.Candidate, len(def.Candidates))
	for _, candidate := range def.Candidates {
		registered[candidate.ID] = candidate
	}
	for id := range patches {
		if _, ok := registered[id]; !ok {
			return nil, fmt.Errorf("experimentrun: candidate patches include unregistered candidate %q", id)
		}
	}
	rows := make([]experiment.ReceiptCandidate, 0, len(def.Candidates))
	for _, candidate := range def.Candidates {
		patch, ok := patches[candidate.ID]
		if !ok {
			return nil, fmt.Errorf("experimentrun: candidate patches omit %q", candidate.ID)
		}
		identityRunID, err := experiment.WorkspaceRunID(experimentDigest, run, candidate.ID)
		if err != nil {
			return nil, fmt.Errorf("experimentrun: candidate %q workspace run id: %w", candidate.ID, err)
		}
		identity, err := execworkspace.NewPatchIdentity(identityRunID, candidate.Base, patch)
		if err != nil {
			return nil, fmt.Errorf("experimentrun: candidate %q materialization identity: %w", candidate.ID, err)
		}
		if "sha256:"+identity.PatchSHA256 != candidate.Digest {
			return nil, fmt.Errorf("experimentrun: candidate %q patch bytes do not match registered digest", candidate.ID)
		}
		rows = append(rows, experiment.ReceiptCandidate{
			ID:             candidate.ID,
			BaseCommit:     candidate.Base,
			PatchDigest:    candidate.Digest,
			WorkspaceRunID: identityRunID,
			Materialization: experiment.WorkspaceIdentity{
				Shape:       experiment.WorkspaceBasePlusPatch,
				RunID:       identity.RunID,
				CommitSHA:   identity.CommitSHA,
				PatchSHA256: identity.PatchSHA256,
			},
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}

// BuildExecutionReceipt derives one strict durable receipt. It rechecks every
// authority input instead of trusting an earlier validation pass.
func BuildExecutionReceipt(input ReceiptInput) (experiment.ExecutionReceipt, error) {
	return buildExecutionReceipt(input, hostRuntimeFacts{
		os:             runtime.GOOS,
		arch:           runtime.GOARCH,
		runtimeVersion: runtime.Version(),
	})
}

func buildExecutionReceipt(input ReceiptInput, host hostRuntimeFacts) (experiment.ExecutionReceipt, error) {
	if err := input.Definition.Validate(); err != nil {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt definition: %w", err)
	}
	locked, err := experiment.Locked(input.Definition)
	if err != nil {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt lock: %w", err)
	}
	if !locked {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt requires a locked definition")
	}
	if err := experiment.ValidateID(input.Run); err != nil {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt run: %w", err)
	}
	capabilities, err := experiment.DecodeCapabilities(input.CapabilitiesBytes)
	if err != nil {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt capabilities bytes: %w", err)
	}
	if !reflect.DeepEqual(capabilities, input.Capabilities) {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt capabilities value does not match canonical capability bytes")
	}
	if digestBytes(input.CapabilitiesBytes) != input.Definition.Evaluator.CapabilitiesDigest {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt capabilities bytes do not match locked digest")
	}
	authorized, err := validateAuthorization(input.Definition, capabilities, input.Authorization.Authorization)
	if err != nil {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt authorization: %w", err)
	}
	if !reflect.DeepEqual(authorized.Grants, input.Authorization.Grants) {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt decoded grants do not match authorization")
	}
	if err := validateResolvedInputs(input.Definition, input.Inputs); err != nil {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt inputs: %w", err)
	}
	experimentDigest, err := experiment.DefinitionDigest(input.Definition)
	if err != nil {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt definition digest: %w", err)
	}
	schedule, err := DeriveSchedule(input.Definition)
	if err != nil {
		return experiment.ExecutionReceipt{}, err
	}
	scheduleDigest, err := ScheduleDigest(schedule)
	if err != nil {
		return experiment.ExecutionReceipt{}, err
	}
	candidates, err := CandidateReceipts(input.Definition, experimentDigest, input.Run, input.CandidatePatches)
	if err != nil {
		return experiment.ExecutionReceipt{}, err
	}
	fingerprint, err := decodeFingerprint(input.Fingerprint)
	if err != nil {
		return experiment.ExecutionReceipt{}, err
	}
	if err := validateFingerprint(input.Definition, capabilities, authorized.Authorization, input.Inputs, input.Versions, fingerprint, host); err != nil {
		return experiment.ExecutionReceipt{}, err
	}
	enforcement, network, disclosures, err := receiptEnforcement(authorized.Grants, input.Enforcement)
	if err != nil {
		return experiment.ExecutionReceipt{}, err
	}
	receipt := experiment.ExecutionReceipt{
		Schema:             experiment.ExecutionReceiptSchemaV2,
		ExperimentDigest:   experimentDigest,
		Run:                input.Run,
		EnvironmentPolicy:  authorized.Authorization.EnvironmentPolicy,
		AuthorityDigest:    authorized.Authorization.AuthorityDigest,
		CapabilitiesDigest: input.Definition.Evaluator.CapabilitiesDigest,
		ScheduleDigest:     scheduleDigest,
		GrantsDigest:       digestBytes(authorized.Authorization.GrantBytes),
		Fingerprint:        fingerprint,
		Inputs:             receiptInputs(input.Inputs),
		Enforcement:        enforcement,
		Network:            network,
		Candidates:         candidates,
		Versions:           input.Versions,
		Disclosures:        disclosures,
	}
	if err := receipt.Validate(); err != nil {
		return experiment.ExecutionReceipt{}, fmt.Errorf("experimentrun: build receipt validation: %w", err)
	}
	return receipt, nil
}

func receiptInputs(inputs ResolvedInputs) *experiment.ReceiptInputs {
	fixtures := make([]experiment.ResolvedArtifact, len(inputs.Fixtures))
	copy(fixtures, inputs.Fixtures)
	return &experiment.ReceiptInputs{
		Workload: inputs.Workload,
		Fixtures: fixtures,
		Contract: inputs.Contract,
	}
}

// VerifyExecutionReceipt proves receipt is byte-identical to the complete
// receipt derivable from input. A mismatch is operational rather than a
// favorable interpretation of changed execution facts.
func VerifyExecutionReceipt(input ReceiptInput, receipt experiment.ExecutionReceipt) error {
	return verifyExecutionReceipt(input, receipt, hostRuntimeFacts{
		os:             runtime.GOOS,
		arch:           runtime.GOARCH,
		runtimeVersion: runtime.Version(),
	})
}

func verifyExecutionReceipt(input ReceiptInput, receipt experiment.ExecutionReceipt, host hostRuntimeFacts) error {
	if err := receipt.Validate(); err != nil {
		return fmt.Errorf("experimentrun: verify receipt: %w", err)
	}
	want, err := buildExecutionReceipt(input, host)
	if err != nil {
		return fmt.Errorf("experimentrun: verify receipt inputs: %w", err)
	}
	wantBytes, err := experiment.EncodeExecutionReceipt(want)
	if err != nil {
		return fmt.Errorf("experimentrun: encode expected receipt: %w", err)
	}
	gotBytes, err := experiment.EncodeExecutionReceipt(receipt)
	if err != nil {
		return fmt.Errorf("experimentrun: encode receipt: %w", err)
	}
	if !bytes.Equal(wantBytes, gotBytes) {
		return fmt.Errorf("experimentrun: receipt does not match current locked execution inputs")
	}
	return nil
}

func decodeFingerprint(raw []byte) (experiment.ExecutionFingerprint, error) {
	var fingerprint experiment.ExecutionFingerprint
	if err := artifact.DecodeStrictJSON(raw, &fingerprint); err != nil {
		return experiment.ExecutionFingerprint{}, fmt.Errorf("experimentrun: decode fingerprint: %w", err)
	}
	if err := fingerprint.Validate(); err != nil {
		return experiment.ExecutionFingerprint{}, fmt.Errorf("experimentrun: validate fingerprint: %w", err)
	}
	canonical, err := canonjson.Marshal(fingerprint)
	if err != nil {
		return experiment.ExecutionFingerprint{}, fmt.Errorf("experimentrun: encode fingerprint: %w", err)
	}
	if !bytes.Equal(canonical, raw) {
		return experiment.ExecutionFingerprint{}, fmt.Errorf("experimentrun: fingerprint bytes are not canonical")
	}
	return fingerprint, nil
}

func validateFingerprint(def experiment.Definition, capabilities experiment.Capabilities, authorization ExecutionAuthorization, inputs ResolvedInputs, versions experiment.ReceiptVersions, fingerprint experiment.ExecutionFingerprint, host hostRuntimeFacts) error {
	if fingerprint.OS != host.os {
		return fmt.Errorf("experimentrun: fingerprint OS %q does not match host %q", fingerprint.OS, host.os)
	}
	if fingerprint.Arch != host.arch {
		return fmt.Errorf("experimentrun: fingerprint architecture %q does not match host %q", fingerprint.Arch, host.arch)
	}
	if fingerprint.ToolVersions["runtime"] != host.runtimeVersion {
		return fmt.Errorf("experimentrun: fingerprint tool version %q does not match host runtime %q", "runtime", host.runtimeVersion)
	}
	if host.os != "linux" {
		return fmt.Errorf("experimentrun: authoritative CSE execution requires linux, host is %q", host.os)
	}
	if err := versions.Validate(); err != nil {
		return fmt.Errorf("experimentrun: receipt versions: %w", err)
	}
	if versions.RecommendationEngine != string(def.Algorithm) {
		return fmt.Errorf("experimentrun: receipt recommendation engine %q does not match definition %q", versions.RecommendationEngine, def.Algorithm)
	}
	for name, want := range map[string]string{
		"verdi":                 versions.Verdi,
		"evaluator":             capabilities.EvaluatorVersion,
		"recommendation-engine": string(def.Algorithm),
	} {
		if fingerprint.ToolVersions[name] != want {
			return fmt.Errorf("experimentrun: fingerprint tool version %q does not match required value", name)
		}
	}
	if len(fingerprint.Env) != len(authorization.DeclaredEnv) {
		return fmt.Errorf("experimentrun: fingerprint environment has %d entries, want %d", len(fingerprint.Env), len(authorization.DeclaredEnv))
	}
	for name, want := range authorization.DeclaredEnv {
		got, ok := fingerprint.Env[name]
		if !ok || got == nil || *got != want {
			return fmt.Errorf("experimentrun: fingerprint environment %q does not match authorization", name)
		}
	}
	wantInputs := map[string]string{
		"evaluator:" + def.Evaluator.Argv[0]: strings.TrimPrefix(def.Evaluator.Digest, "sha256:"),
		inputs.Workload.Path:                 strings.TrimPrefix(inputs.Workload.Digest, "sha256:"),
		inputs.Contract.Path:                 strings.TrimPrefix(inputs.Contract.Digest, "sha256:"),
	}
	for _, fixture := range inputs.Fixtures {
		wantInputs[fixture.Path] = strings.TrimPrefix(fixture.Digest, "sha256:")
	}
	if len(fingerprint.InputDigests) != len(wantInputs) {
		return fmt.Errorf("experimentrun: fingerprint input set has %d entries, want %d", len(fingerprint.InputDigests), len(wantInputs))
	}
	for path, want := range wantInputs {
		if fingerprint.InputDigests[path] != want {
			return fmt.Errorf("experimentrun: fingerprint input %q does not match locked identity", path)
		}
	}
	return nil
}

func receiptEnforcement(grants execworkspace.GrantSet, report execworkspace.EnforcementReport) ([]experiment.ReceiptEnforcement, experiment.ReceiptNetwork, []experiment.ReceiptDisclosure, error) {
	network, err := receiptNetwork(report.Network)
	if err != nil {
		return nil, experiment.ReceiptNetwork{}, nil, err
	}
	if len(report.Rows) != len(grants.Grants) {
		return nil, experiment.ReceiptNetwork{}, nil, fmt.Errorf("experimentrun: enforcement has %d rows, want one for each of %d grants", len(report.Rows), len(grants.Grants))
	}
	granted := make(map[execworkspace.GrantKind]bool, len(grants.Grants))
	for _, grant := range grants.Grants {
		granted[grant.Kind] = true
	}
	enforcement := make([]experiment.ReceiptEnforcement, len(report.Rows))
	resourceCeilingsApplied := false
	seen := make(map[execworkspace.GrantKind]bool, len(report.Rows))
	for i, row := range report.Rows {
		if !granted[row.Kind] || seen[row.Kind] {
			return nil, experiment.ReceiptNetwork{}, nil, fmt.Errorf("experimentrun: enforcement row %q does not project exactly one granted control", row.Kind)
		}
		seen[row.Kind] = true
		if !row.Applied {
			return nil, experiment.ReceiptNetwork{}, nil, fmt.Errorf("experimentrun: enforcement %q was not applied", row.Kind)
		}
		enforcement[i] = experiment.ReceiptEnforcement{Kind: row.Kind.String(), Applied: row.Applied, Reason: row.Reason}
		if row.Kind == execworkspace.GrantResourceCeilings {
			resourceCeilingsApplied = true
		}
	}
	if len(seen) != len(granted) {
		return nil, experiment.ReceiptNetwork{}, nil, fmt.Errorf("experimentrun: enforcement does not project every granted control")
	}
	if network.Mode == experiment.NetworkDeny {
		for _, row := range enforcement {
			if row.Kind == execworkspace.GrantNetwork.String() {
				return nil, experiment.ReceiptNetwork{}, nil, fmt.Errorf("experimentrun: denied network posture cannot project a network grant")
			}
		}
	}
	if network.Mode == experiment.NetworkAllow {
		found := false
		for _, row := range enforcement {
			if row.Kind == execworkspace.GrantNetwork.String() {
				found = true
			}
		}
		if !found {
			return nil, experiment.ReceiptNetwork{}, nil, fmt.Errorf("experimentrun: allowed network posture requires projected network grant")
		}
	}
	disclosures := []experiment.ReceiptDisclosure{}
	if !resourceCeilingsApplied {
		disclosures = []experiment.ReceiptDisclosure{experiment.DisclosureCPUAllocationUnproven, experiment.DisclosureMemoryAllocationUnproven}
	}
	return enforcement, network, disclosures, nil
}

func receiptNetwork(network execworkspace.NetworkEnforcement) (experiment.ReceiptNetwork, error) {
	if !network.Configured {
		return experiment.ReceiptNetwork{}, fmt.Errorf("experimentrun: network enforcement is unavailable")
	}
	var mode experiment.NetworkMode
	switch network.Mode {
	case execworkspace.NetworkDeny:
		mode = experiment.NetworkDeny
	case execworkspace.NetworkAllow:
		mode = experiment.NetworkAllow
	default:
		return experiment.ReceiptNetwork{}, fmt.Errorf("experimentrun: unknown execution-workspace network mode %q", network.Mode)
	}
	result := experiment.ReceiptNetwork{Mode: mode, Configured: network.Configured, Reason: network.Reason}
	if err := result.Validate(); err != nil {
		return experiment.ReceiptNetwork{}, fmt.Errorf("experimentrun: receipt network: %w", err)
	}
	return result, nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
