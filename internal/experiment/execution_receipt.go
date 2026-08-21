package experiment

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
)

const ExecutionReceiptSchema = "verdi.experiment-execution/v1"

type NetworkMode string

const (
	NetworkDeny  NetworkMode = "deny"
	NetworkAllow NetworkMode = "allow"
)

type ReceiptNetwork struct {
	Mode       NetworkMode `json:"mode"`
	Configured bool        `json:"configured"`
	Reason     string      `json:"reason"`
}

func (n ReceiptNetwork) Validate() error {
	if n.Mode != NetworkDeny && n.Mode != NetworkAllow {
		return fmt.Errorf("experiment: unknown receipt network mode %q", n.Mode)
	}
	if n.Mode == NetworkAllow && !n.Configured {
		return fmt.Errorf("experiment: allowed network must be configured")
	}
	return nonemptyString("receipt.network.reason", n.Reason)
}

type ExecutionFingerprint struct {
	OS           string             `json:"os"`
	Arch         string             `json:"arch"`
	ToolVersions map[string]string  `json:"tool_versions"`
	Env          map[string]*string `json:"env"`
	InputDigests map[string]string  `json:"input_digests"`
}

func (f ExecutionFingerprint) Validate() error {
	if f.OS == "" || f.Arch == "" {
		return fmt.Errorf("experiment: receipt fingerprint os and arch must be nonempty")
	}
	if f.ToolVersions == nil || f.Env == nil || f.InputDigests == nil {
		return fmt.Errorf("experiment: receipt fingerprint maps must be present")
	}
	for k, v := range f.ToolVersions {
		if k == "" || v == "" {
			return fmt.Errorf("experiment: receipt fingerprint tool versions require nonempty names and values")
		}
	}
	for k, v := range f.Env {
		if k == "" || strings.ContainsAny(k, "=\x00") {
			return fmt.Errorf("experiment: receipt fingerprint environment name %q is invalid", k)
		}
		if v != nil && strings.IndexByte(*v, 0) >= 0 {
			return fmt.Errorf("experiment: receipt fingerprint environment %q contains NUL", k)
		}
	}
	for k, v := range f.InputDigests {
		if k == "" || v == "" {
			return fmt.Errorf("experiment: receipt fingerprint input digests require nonempty names and values")
		}
		if _, err := hex.DecodeString(v); err != nil || strings.ToLower(v) != v {
			return fmt.Errorf("experiment: receipt fingerprint input %q digest is not canonical lowercase hex", k)
		}
	}
	return nil
}

type ReceiptEnforcement struct {
	Kind    string `json:"kind"`
	Applied bool   `json:"applied"`
	Reason  string `json:"reason"`
}

var enforcementOrder = map[string]int{"network": 0, "path-read": 1, "path-write": 2, "process-execution": 3, "resource-ceilings": 4, "timeouts": 5}

func (r ReceiptEnforcement) Validate() error {
	if _, ok := enforcementOrder[r.Kind]; !ok {
		return fmt.Errorf("experiment: unknown enforcement kind %q", r.Kind)
	}
	return nonemptyString("receipt.enforcement.reason", r.Reason)
}

type WorkspaceShape string

const WorkspaceBasePlusPatch WorkspaceShape = "base-plus-patch"

type WorkspaceIdentity struct {
	Shape       WorkspaceShape `json:"shape"`
	RunID       string         `json:"run_id"`
	CommitSHA   string         `json:"commit_sha"`
	PatchSHA256 string         `json:"patch_sha256"`
}

func (i WorkspaceIdentity) Validate() error {
	if i.Shape != WorkspaceBasePlusPatch {
		return fmt.Errorf("experiment: unknown CSE workspace shape %q", i.Shape)
	}
	if len(i.RunID) != 64 || !lowerHex(i.RunID) {
		return fmt.Errorf("experiment: workspace identity run_id must be full lowercase sha256")
	}
	if err := ValidateCommit(i.CommitSHA); err != nil {
		return err
	}
	if len(i.PatchSHA256) != 64 || !lowerHex(i.PatchSHA256) {
		return fmt.Errorf("experiment: workspace identity patch_sha256 must be full lowercase sha256")
	}
	return nil
}

func lowerHex(s string) bool {
	for _, c := range []byte(s) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

type ReceiptCandidate struct {
	ID              string            `json:"id"`
	BaseCommit      string            `json:"base_commit"`
	PatchDigest     string            `json:"patch_digest"`
	WorkspaceRunID  string            `json:"workspace_run_id"`
	Materialization WorkspaceIdentity `json:"materialization"`
}

func (c ReceiptCandidate) Validate(experimentDigest, run string) error {
	if err := ValidateID(c.ID); err != nil {
		return err
	}
	if err := ValidateCommit(c.BaseCommit); err != nil {
		return err
	}
	if err := ValidateDigest(c.PatchDigest); err != nil {
		return err
	}
	want, err := WorkspaceRunID(experimentDigest, run, c.ID)
	if err != nil {
		return err
	}
	if c.WorkspaceRunID != want {
		return fmt.Errorf("experiment: candidate %q workspace_run_id %q, want %q", c.ID, c.WorkspaceRunID, want)
	}
	if err := c.Materialization.Validate(); err != nil {
		return err
	}
	if c.Materialization.RunID != c.WorkspaceRunID || c.Materialization.CommitSHA != c.BaseCommit || "sha256:"+c.Materialization.PatchSHA256 != c.PatchDigest {
		return fmt.Errorf("experiment: candidate %q materialization identity does not match receipt row", c.ID)
	}
	return nil
}

type ReceiptVersions struct {
	Verdi                string `json:"verdi"`
	RecommendationEngine string `json:"recommendation_engine"`
}

func (v ReceiptVersions) Validate() error {
	if v.Verdi == "" {
		return fmt.Errorf("experiment: receipt versions.verdi must be nonempty")
	}
	if AlgorithmVersion(v.RecommendationEngine).Validate() != nil {
		return fmt.Errorf("experiment: receipt versions.recommendation_engine is unknown")
	}
	return nil
}

type ReceiptDisclosure string

const (
	DisclosureCPUAllocationUnproven    ReceiptDisclosure = "cpu-allocation-unproven"
	DisclosureMemoryAllocationUnproven ReceiptDisclosure = "memory-allocation-unproven"
)

func (d ReceiptDisclosure) Validate() error {
	if d != DisclosureCPUAllocationUnproven && d != DisclosureMemoryAllocationUnproven {
		return fmt.Errorf("experiment: unknown receipt disclosure %q", d)
	}
	return nil
}

type ExecutionReceipt struct {
	Schema             string               `json:"schema"`
	ExperimentDigest   string               `json:"experiment_digest"`
	Run                string               `json:"run"`
	EnvironmentPolicy  string               `json:"environment_policy"`
	AuthorityDigest    string               `json:"authority_digest"`
	CapabilitiesDigest string               `json:"capabilities_digest"`
	ScheduleDigest     string               `json:"schedule_digest"`
	GrantsDigest       string               `json:"grants_digest"`
	Fingerprint        ExecutionFingerprint `json:"fingerprint"`
	Enforcement        []ReceiptEnforcement `json:"enforcement"`
	Network            ReceiptNetwork       `json:"network"`
	Candidates         []ReceiptCandidate   `json:"candidates"`
	Versions           ReceiptVersions      `json:"versions"`
	Disclosures        []ReceiptDisclosure  `json:"disclosures"`
}

func (r ExecutionReceipt) Validate() error {
	if r.Schema != ExecutionReceiptSchema {
		return fmt.Errorf("experiment: unknown execution receipt schema %q", r.Schema)
	}
	for field, value := range map[string]string{"experiment_digest": r.ExperimentDigest, "authority_digest": r.AuthorityDigest, "capabilities_digest": r.CapabilitiesDigest, "schedule_digest": r.ScheduleDigest, "grants_digest": r.GrantsDigest} {
		if err := ValidateDigest(value); err != nil {
			return fmt.Errorf("experiment: receipt.%s: %w", field, err)
		}
	}
	if err := ValidateID(r.Run); err != nil {
		return err
	}
	if r.EnvironmentPolicy == "" {
		return fmt.Errorf("experiment: receipt.environment_policy must be nonempty")
	}
	if err := r.Fingerprint.Validate(); err != nil {
		return err
	}
	if r.Enforcement == nil {
		return fmt.Errorf("experiment: receipt.enforcement must be present")
	}
	last := -1
	for _, row := range r.Enforcement {
		if err := row.Validate(); err != nil {
			return err
		}
		rank := enforcementOrder[row.Kind]
		if rank <= last {
			return fmt.Errorf("experiment: receipt enforcement rows must be unique and declaration-ordered")
		}
		last = rank
	}
	if err := r.Network.Validate(); err != nil {
		return err
	}
	if len(r.Candidates) == 0 {
		return fmt.Errorf("experiment: receipt.candidates must be nonempty")
	}
	lastID := ""
	for _, c := range r.Candidates {
		if err := c.Validate(r.ExperimentDigest, r.Run); err != nil {
			return err
		}
		if lastID != "" && c.ID <= lastID {
			return fmt.Errorf("experiment: receipt candidates must be sorted by id without duplicates")
		}
		lastID = c.ID
	}
	if err := r.Versions.Validate(); err != nil {
		return err
	}
	if r.Disclosures == nil {
		return fmt.Errorf("experiment: receipt.disclosures must be present")
	}
	for i, d := range r.Disclosures {
		if err := d.Validate(); err != nil {
			return err
		}
		if i > 0 && r.Disclosures[i-1] >= d {
			return fmt.Errorf("experiment: receipt disclosures must be sorted and unique")
		}
	}
	return nil
}

func EncodeExecutionReceipt(r ExecutionReceipt) ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return canonjson.Marshal(r)
}
func DecodeExecutionReceipt(raw []byte) (ExecutionReceipt, error) {
	var r ExecutionReceipt
	if err := decodeStrictJSON(raw, &r); err != nil {
		return r, err
	}
	if err := r.Validate(); err != nil {
		return r, err
	}
	if err := requireCanonicalJSON(raw, r); err != nil {
		return r, err
	}
	return r, nil
}
func ExecutionReceiptDigest(r ExecutionReceipt) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	return canonjson.Digest(r)
}

// ValidateExecutionReceiptBinding proves the exact receipt belongs to the
// locked definition and measured run whose V2 decision it supports.
func ValidateExecutionReceiptBinding(def Definition, observations []Observation, receipt ExecutionReceipt) error {
	if err := receipt.Validate(); err != nil {
		return err
	}
	locked, err := Locked(def)
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("experiment: execution receipt binding requires a locked definition")
	}
	if err := ValidateComplete(def, observations); err != nil {
		return err
	}
	digest, err := DefinitionDigest(def)
	if err != nil {
		return err
	}
	if receipt.ExperimentDigest != digest || receipt.Run != observations[0].Run {
		return fmt.Errorf("experiment: execution receipt does not match definition/observation identity")
	}
	if receipt.EnvironmentPolicy != def.Execution.EnvironmentPolicy {
		return fmt.Errorf("experiment: execution receipt environment policy %q does not match definition %q", receipt.EnvironmentPolicy, def.Execution.EnvironmentPolicy)
	}
	if receipt.CapabilitiesDigest != def.Evaluator.CapabilitiesDigest {
		return fmt.Errorf("experiment: execution receipt capabilities digest does not match definition")
	}
	if receipt.Versions.RecommendationEngine != string(def.Algorithm) {
		return fmt.Errorf("experiment: execution receipt recommendation engine does not match definition")
	}
	registered := make(map[string]Candidate, len(def.Candidates))
	for _, candidate := range def.Candidates {
		registered[candidate.ID] = candidate
	}
	if len(receipt.Candidates) != len(registered) {
		return fmt.Errorf("experiment: execution receipt candidate set has %d rows, want %d", len(receipt.Candidates), len(registered))
	}
	for _, row := range receipt.Candidates {
		candidate, ok := registered[row.ID]
		if !ok || row.BaseCommit != candidate.Base || row.PatchDigest != candidate.Digest {
			return fmt.Errorf("experiment: execution receipt candidate %q does not match the locked definition", row.ID)
		}
	}
	return nil
}
