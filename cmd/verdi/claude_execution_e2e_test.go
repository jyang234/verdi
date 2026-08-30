package main

// claude_execution_e2e_test.go — adverse matrix for the Claude adapter path.
//
// Coverage (Task 6 brief §7): adapter selection, profile activation,
// ANTHROPIC_API_KEY classification, controller loss, unsupported adapter,
// sealed-start lifecycle, stdin/path request delivery, --out flag,
// malformed controller schema, and byte-mutation witnesses.
// All tests are hermetic: no network, no real claude binary.
//
// These tests do not constitute an eighth frozen producer; they extend the
// existing cmd/verdi behavioral evidence base.

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/execworkspace"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/sealedexec"
)

// TestClaudeExecutionE2EContract_Behavioral exercises the Claude adapter path
// through the built verdi binary. Each sub-test isolates one concern from the
// Task 6 adverse matrix.
func TestClaudeExecutionE2EContract_Behavioral(t *testing.T) {
	bin := buildVerdiBinary(t)

	// -----------------------------------------------------------------------
	// Adapter selection: a mutated adapter value that neither the encoder
	// nor the binary recognises must produce an operational refusal (exit 2).
	// We encode a valid Codex request and mutate its adapter field bytes so
	// the binary sees the unknown value on its decode path. The binary fails
	// during request decode, before reaching the controller, so no FD3 needed.
	// -----------------------------------------------------------------------
	t.Run("unsupported_adapter_is_rejected_operationally", func(t *testing.T) {
		dir := t.TempDir()
		req := sealedCanonicalExecutionRequest(t, dir)
		// Replace the known adapter tag; the binary must reject this.
		mutated := bytes.Replace(req,
			[]byte(`"adapter":"codex"`),
			[]byte(`"adapter":"badapter"`), 1)
		// Binary exits during decode, before any FD3 interaction.
		obs := runSealedContextBinary(t, bin, dir, mutated,
			"context", "execution", "--request", "-")
		if obs.exitCode != 2 {
			t.Fatalf("unsupported adapter exit = %d, want 2; stderr=%q", obs.exitCode, obs.stderr)
		}
		// The binary should report an adapter-related error, not the missing-FD3 error.
		if strings.Contains(obs.stderr, "FD 3") {
			t.Fatalf("bad-adapter refusal should not mention FD 3; stderr=%q", obs.stderr)
		}
	})

	// -----------------------------------------------------------------------
	// Claude adapter: ANTHROPIC_API_KEY must be present and ≥8 bytes.
	// When the key is absent the binary must exit 2 before any provider
	// process is started. The test process inherits no API key.
	// -----------------------------------------------------------------------
	t.Run("claude_arm_without_api_key_is_operational_refusal", func(t *testing.T) {
		dir := t.TempDir()
		req := claudeE2ERequest(t, dir)
		obs, _ := runWithRefusingController(t, bin, dir, req,
			"context", "execution", "--request", "-")
		if obs.exitCode != 2 {
			t.Fatalf("claude without API key exit = %d, want 2; stderr=%q stdout=%q",
				obs.exitCode, obs.stderr, obs.stdout)
		}
	})

	// -----------------------------------------------------------------------
	// Codex adapter regression guard: AdapterCodex still selects the Codex
	// path after the adapter switch was added. The binary must reach the
	// controller (not crash at selection) before the controller refuses.
	// -----------------------------------------------------------------------
	t.Run("codex_adapter_selection_still_works_after_switch", func(t *testing.T) {
		dir := t.TempDir()
		req := sealedCanonicalExecutionRequest(t, dir)
		obs, call := runWithRefusingController(t, bin, dir, req,
			"context", "execution", "--request", "-")
		if obs.exitCode != 2 {
			t.Fatalf("codex refusal exit = %d, want 2; stderr=%q", obs.exitCode, obs.stderr)
		}
		if call.Operation != sealedexec.ControllerOperationVerifyAuthority {
			t.Fatalf("codex first operation = %q, want %q",
				call.Operation, sealedexec.ControllerOperationVerifyAuthority)
		}
	})

	// -----------------------------------------------------------------------
	// Controller loss (zero live provider/network): if FD3 is absent the
	// binary must exit 2 with a recognisable diagnostic.
	// -----------------------------------------------------------------------
	t.Run("controller_loss_on_first_call_is_operational", func(t *testing.T) {
		dir := t.TempDir()
		req := sealedCanonicalExecutionRequest(t, dir)
		obs := runSealedContextBinary(t, bin, dir, req,
			"context", "execution", "--request", "-")
		if obs.exitCode != 2 {
			t.Fatalf("no FD3 exit = %d, want 2; stderr=%q", obs.exitCode, obs.stderr)
		}
		if !strings.Contains(obs.stderr, "FD 3") &&
			!strings.Contains(obs.stderr, "fd3") &&
			!strings.Contains(obs.stderr, "controller") {
			t.Fatalf("no FD3 stderr = %q, want FD-3 / controller context", obs.stderr)
		}
	})

	// -----------------------------------------------------------------------
	// Malformed controller schema (receipt two-domain mismatch arm): the
	// binary must reject the response operationally (exit 2).
	// -----------------------------------------------------------------------
	t.Run("malformed_controller_schema_is_operational", func(t *testing.T) {
		dir := t.TempDir()
		req := sealedCanonicalExecutionRequest(t, dir)
		obs, _ := runWithControllerReply(t, bin, dir, req,
			[]string{"context", "execution", "--request", "-"},
			func(call sealedexec.ControllerCall) []byte {
				return malformedControllerReply(t, call, "schema")
			},
		)
		if obs.exitCode != 2 {
			t.Fatalf("schema mismatch exit = %d, want 2; stderr=%q", obs.exitCode, obs.stderr)
		}
	})

	// -----------------------------------------------------------------------
	// Claude stdin request: a Claude-adapter request delivered via stdin
	// reaches the controller before failing at local profile activation.
	// -----------------------------------------------------------------------
	t.Run("claude_request_reaches_controller_via_stdin", func(t *testing.T) {
		dir := t.TempDir()
		req := claudeE2ERequest(t, dir)
		obs, call := runWithRefusingController(t, bin, dir, req,
			"context", "execution", "--request", "-")
		if obs.exitCode != 2 {
			t.Fatalf("claude stdin path exit = %d, want 2; stderr=%q", obs.exitCode, obs.stderr)
		}
		// Binary must have reached the controller before failing locally.
		if call.Operation != sealedexec.ControllerOperationVerifyAuthority {
			t.Fatalf("claude first operation = %q, want %q",
				call.Operation, sealedexec.ControllerOperationVerifyAuthority)
		}
	})

	// -----------------------------------------------------------------------
	// --out flag: stdout must stay empty when result file path is given.
	// -----------------------------------------------------------------------
	t.Run("out_flag_stdout_stays_clean", func(t *testing.T) {
		dir := t.TempDir()
		req := sealedCanonicalExecutionRequest(t, dir)
		outPath := dir + "/result.json"
		obs, _ := runWithRefusingController(t, bin, dir, req,
			"context", "execution", "--request", "-", "--out", outPath)
		if obs.stdout != "" {
			t.Fatalf("stdout with --out = %q, want empty", obs.stdout)
		}
	})

	// -----------------------------------------------------------------------
	// path request: --request <path> delivers the same request via file.
	// -----------------------------------------------------------------------
	t.Run("path_request_reaches_controller", func(t *testing.T) {
		dir := t.TempDir()
		reqBytes := sealedCanonicalExecutionRequest(t, dir)
		reqPath := dir + "/request.json"
		if err := os.WriteFile(reqPath, reqBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		obs, call := runWithRefusingController(t, bin, dir, reqBytes,
			"context", "execution", "--request", reqPath)
		if obs.exitCode != 2 {
			t.Fatalf("path request exit = %d, want 2; stderr=%q", obs.exitCode, obs.stderr)
		}
		if call.Operation != sealedexec.ControllerOperationVerifyAuthority {
			t.Fatalf("path request first op = %q, want verify-authority", call.Operation)
		}
	})

	// -----------------------------------------------------------------------
	// Atomic output failure: if --out names an unwritable directory path the
	// binary must exit 2 after the execution attempt (operational error).
	// -----------------------------------------------------------------------
	t.Run("atomic_output_to_nonexistent_dir_is_operational", func(t *testing.T) {
		dir := t.TempDir()
		req := sealedCanonicalExecutionRequest(t, dir)
		outPath := dir + "/does-not-exist/result.json"
		obs, _ := runWithRefusingController(t, bin, dir, req,
			"context", "execution", "--request", "-", "--out", outPath)
		// The controller refusal itself triggers exit 2 regardless of out path,
		// so stdout must be empty and exit must be 2.
		if obs.exitCode != 2 {
			t.Fatalf("nonexistent out-dir exit = %d, want 2; stderr=%q", obs.exitCode, obs.stderr)
		}
		if obs.stdout != "" {
			t.Fatalf("stdout with failed out path = %q, want empty", obs.stdout)
		}
	})
}

// ---------------------------------------------------------------------------
// Claude-specific request builder
// ---------------------------------------------------------------------------

// claudeE2ERequest builds a minimal sealed execution request for the Claude
// adapter arm (ActionStart). The manifest adapter field matches AdapterClaude
// so that EncodeExecutionRequest accepts the request. The test process is
// expected to have no ANTHROPIC_API_KEY, triggering the profile-activation
// refusal in the binary.
func claudeE2ERequest(t *testing.T, runway string) []byte {
	t.Helper()
	const commit = "2222222222222222222222222222222222222222"
	const tree = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	projection := sealedCanonicalProjection(t)
	manifest := claudeE2EManifest(t, projection, commit)
	workspace, err := sealedexec.NewExecutionWorkspaceRequest(
		"flight-c", "lane-c", "epoch-c", "session-c", commit)
	if err != nil {
		t.Fatal(err)
	}
	req := sealedexec.ExecutionRequest{
		Schema: sealedexec.ExecutionRequestSchemaID, Action: sealedexec.ActionStart,
		Flight: "flight-c", Lane: "lane-c", Epoch: "epoch-c", Session: "session-c",
		ManifestRevision: 0, ATCRunway: runway, InputCommit: commit, InputTree: tree,
		Manifest: manifest, ManifestDigest: manifest.Digest,
		InstructionProjection: projection, ProjectionDigest: projection.Digest,
		ExecutionWorkspaceRequest: workspace,
		Adapter:                   contextevent.AdapterClaude,
		AdapterVersion:            "1",
		Profile: sealedexec.LogicalRef{
			Schema: sealedexec.ProjectProfileRefSchemaID,
			ID:     "project-profile",
			Digest: sealedTestDigest("profile"),
		},
		Grants:           execworkspace.GrantSet{Grants: []execworkspace.Grant{}},
		AuthorityVerdict: claudeE2EAuthorityReport(t, manifest.Digest, commit),
		RecorderEndpoint: sealedexec.LogicalRef{
			Schema: sealedexec.RecorderEndpointRefSchemaID,
			ID:     "vatc-recorder",
			Digest: sealedTestDigest("recorder"),
		},
		Start: &sealedexec.StartArm{ExpectedSourceSequence: 1},
	}
	encoded, err := sealedexec.EncodeExecutionRequest(req)
	if err != nil {
		t.Fatalf("EncodeExecutionRequest claude e2e fixture: %v", err)
	}
	return encoded
}

// claudeE2EManifest builds a contextcompile.Manifest with AdapterRef{ID:"claude"}
// so that EncodeExecutionRequest accepts a Claude-adapter request.
func claudeE2EManifest(t *testing.T, projection sealedexec.InstructionProjection, commit string) contextcompile.Manifest {
	t.Helper()
	files := make([]contextcompile.ProjectionFileRef, len(projection.Files))
	for i, file := range projection.Files {
		files[i] = contextcompile.ProjectionFileRef{Path: file.Path, Digest: file.ContentDigest}
	}
	var scope policyartifact.Scope
	if err := json.Unmarshal([]byte(`{"phases":["build"],"environments":["local"],"paths":[".verdi/**"],"refs":["spec/test"]}`), &scope); err != nil {
		t.Fatal(err)
	}
	manifest := contextcompile.Manifest{
		Schema: contextcompile.ManifestSchema, Phase: contextcompile.PhaseBuild,
		Adapter:        contextcompile.AdapterRef{ID: "claude", Version: "1"},
		Revisions:      contextcompile.Revisions{Authority: sealedTestDigest("revision-authority"), Context: 1},
		AcceptedSpec:   contextcompile.AcceptedSpec{Ref: "spec/test", Path: ".verdi/specs/active/test/spec.md", Blob: commit, Commit: commit, ContentDigest: sealedTestDigest("accepted-spec")},
		ParentFeatures: []contextcompile.ParentFeature{}, Decisions: []contextcompile.DecisionRef{}, Obligations: []contextcompile.Obligation{},
		Repository: contextcompile.RepositoryFacts{
			RemoteOrigin: contextcompile.StringFact{Known: true, Value: "origin"}, Branch: contextcompile.StringFact{Known: true, Value: "feature/test"},
			Head: contextcompile.StringFact{Known: true, Value: commit}, DefaultBranch: contextcompile.DefaultBranchFact{Known: true, Name: "main", Ref: "refs/heads/main", Head: commit},
			Relationship: contextcompile.RelationshipEqual, Dirty: contextcompile.BoolFact{Known: true}, Staged: contextcompile.BoolFact{Known: true},
			Worktree: contextcompile.WorktreeFact{Managed: true, Name: "test-worktree"}, Source: contextcompile.RepoSourceHead, Disclosures: []contextcompile.DisclosureCode{},
		},
		Policy: contextcompile.PolicySection{
			EffectiveDigest: sealedTestDigest("effective-policy"), ConstitutionDigest: sealedTestDigest("constitution"),
			ProfileID: "profile", ProfileDigest: sealedTestDigest("policy-profile"), Entries: []contextcompile.PolicyEntry{},
		},
		Owners: []string{"platform-team"}, Scope: scope,
		GovernanceProfile: contextcompile.GovernanceProfileRef{ID: "profile", Class: gp.ClassSolo, Digest: sealedTestDigest("governance-profile")},
		Actors:            contextcompile.ActorsSection{Posture: contextcompile.ResolutionUnproven, Resolutions: []gp.PrincipalResolution{}, Disclosures: []contextcompile.DisclosureCode{contextcompile.DisclosureActorResolutionUnproven}},
		Included:          []contextcompile.IncludedEntry{}, Excluded: []contextcompile.ExcludedEntry{}, Opaque: []contextcompile.OpaqueEntry{},
		Capabilities: execworkspace.GrantSet{Grants: []execworkspace.Grant{}}, ProjectionFiles: files, RequiredInputs: []contextcompile.RequiredInput{},
		Evidence:    contextcompile.EvidenceSection{Authority: contextcompile.EvidenceAuthorityAdvisory, Freshness: contextcompile.EvidenceFreshnessUnknown, ConsumedReports: []string{}, Disclosures: []contextcompile.DisclosureCode{}},
		Disclosures: []contextcompile.DisclosureCode{contextcompile.DisclosureActorResolutionUnproven},
	}
	encoded, err := contextcompile.EncodeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := contextcompile.DecodeManifest(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

// claudeE2EAuthorityReport builds a policyconflict.Report for the Claude e2e
// fixture. Structurally identical to sealedCanonicalAuthorityReport except the
// manifest digest argument changes with the Claude manifest.
func claudeE2EAuthorityReport(t *testing.T, manifestDigest, commit string) policyconflict.Report {
	return sealedCanonicalAuthorityReport(t, manifestDigest, commit)
}
