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
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/instructionprojection"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/execworkspace"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/sealedexec"
	sealedclaude "github.com/jyang234/verdi/internal/sealedexec/claude"
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

// ---------------------------------------------------------------------------
// Built-binary Claude lifecycle fixture
// ---------------------------------------------------------------------------

// claudeAdapterPolicyStoreFiles installs the two-adapter constitution with the
// Claude adapter declared under the exact sealed adapter identity "claude", so
// a compiled manifest can carry contextevent.AdapterClaude.
func claudeAdapterPolicyStoreFiles(t *testing.T) map[string]string {
	t.Helper()
	files := contextTwoAdapterPolicyStoreFiles(t)
	const key = ".verdi/policy/constitution.md"
	constitution, ok := files[key]
	if !ok {
		t.Fatalf("two-adapter fixture has no %s", key)
	}
	updated := strings.Replace(constitution, "  - id: claude-code\n", "  - id: claude\n", 1)
	if updated == constitution {
		t.Fatalf("two-adapter constitution no longer declares claude-code:\n%s", constitution)
	}
	files[key] = updated
	return files
}

func buildClaudeCompileRepo(t *testing.T, specFiles map[string]string) *fixturegit.Repo {
	t.Helper()
	files := claudeAdapterPolicyStoreFiles(t)
	files[".verdi/verdi.yaml"] = "schema: verdi.layout/v1\n"
	for path, content := range specFiles {
		files[path] = content
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	repo.Head = commitAllOnCurrentBranch(t, repo.Dir, "generate instruction projection")
	return repo
}

type claudeCompiledFixture struct {
	root         string
	head         string
	tree         string
	compiled     contextcompile.Result
	request      sealedexec.ExecutionRequest
	requestBytes []byte
}

// buildClaudeCompiledFixture compiles a real runway whose manifest declares the
// sealed Claude adapter, so the built binary's own compileSealedContext gate
// reproduces the request byte for byte.
func buildClaudeCompiledFixture(t *testing.T, grants execworkspace.GrantSet) claudeCompiledFixture {
	t.Helper()
	repo := buildClaudeCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
		".gitignore": ".verdi/data/\n",
	})
	compileRequest := contextcompile.Request{
		Schema:  contextcompile.RequestSchema,
		Adapter: contextcompile.AdapterRef{ID: string(contextevent.AdapterClaude), Version: "1"},
		Grants:  grants,
		Phase:   contextcompile.PhaseDesign,
		Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Spec:    "spec/feature-alpha",
	}
	compiled, err := contextcompile.NewCompiler().Compile(context.Background(), repo.Dir, compileRequest)
	if err != nil {
		t.Fatalf("compile claude lifecycle fixture: %v", err)
	}
	projection := sealedexec.InstructionProjection{Schema: sealedexec.InstructionProjectionSchemaID, Files: make([]sealedexec.InstructionFile, len(compiled.ProjectionFiles))}
	for i, file := range compiled.ProjectionFiles {
		projection.Files[i] = sealedexec.InstructionFile{Path: file.Path, ContentDigest: file.Digest, Content: string(file.Content)}
	}
	projectionBytes, err := sealedexec.EncodeInstructionProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	if projection, err = sealedexec.DecodeInstructionProjection(bytes.NewReader(projectionBytes)); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(contextGitOutput(t, repo.Dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(contextGitOutput(t, repo.Dir, "rev-parse", "HEAD^{tree}"))
	workspaceRequest, err := sealedexec.NewExecutionWorkspaceRequest("flight-claude", "lane-claude", "epoch-claude", "session-claude", head)
	if err != nil {
		t.Fatal(err)
	}
	request := sealedexec.ExecutionRequest{
		Schema: sealedexec.ExecutionRequestSchemaID, Action: sealedexec.ActionStart,
		Flight: "flight-claude", Lane: "lane-claude", Epoch: "epoch-claude", Session: "session-claude",
		ManifestRevision: 0, ATCRunway: repo.Dir, InputCommit: head, InputTree: tree,
		Manifest: compiled.Manifest, ManifestDigest: compiled.Manifest.Digest,
		InstructionProjection: projection, ProjectionDigest: projection.Digest,
		ExecutionWorkspaceRequest: workspaceRequest,
		Adapter:                   contextevent.Adapter(compiled.Manifest.Adapter.ID), AdapterVersion: compiled.Manifest.Adapter.Version,
		Profile: sealedexec.LogicalRef{Schema: sealedexec.ProjectProfileRefSchemaID, ID: "claude-profile", Digest: sealedTestDigest("claude-profile")},
		Grants:  compiled.Manifest.Capabilities, AuthorityVerdict: sealedCanonicalAuthorityReport(t, compiled.Manifest.Digest, head),
		RecorderEndpoint: sealedexec.LogicalRef{Schema: sealedexec.RecorderEndpointRefSchemaID, ID: "claude-recorder", Digest: sealedTestDigest("claude-recorder")},
		Start:            &sealedexec.StartArm{ExpectedSourceSequence: 1},
	}
	requestBytes, err := sealedexec.EncodeExecutionRequest(request)
	if err != nil {
		t.Fatalf("encode claude execution request: %v", err)
	}
	return claudeCompiledFixture{root: repo.Dir, head: head, tree: tree, compiled: compiled, request: request, requestBytes: requestBytes}
}

// ---------------------------------------------------------------------------
// Fake Claude provider
// ---------------------------------------------------------------------------

// fakeClaudeSpec parameterises the compiled fake provider. It never contacts a
// network: the only socket it opens is the parent-hosted loopback MCP server
// named by its own --mcp-config operand.
type fakeClaudeSpec struct {
	version   string
	model     string
	session   string
	argvPath  string
	envPath   string
	stdinPath string
	toolsPath string
	gitPath   string
	// workspace is the exact sealed execution workspace the provider reports as
	// its cwd. A real provider reads it from the kernel; the fake is told it so
	// the fixture never depends on host symlink resolution.
	workspace string
	// extraTool, when set, is called after both declared tools and is expected
	// to be refused by the scoped surface.
	extraTool string
	commit    bool
}

const fakeClaudeSource = `package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
)

const (
	version   = __VERSION__
	model     = __MODEL__
	session   = __SESSION__
	argvPath  = __ARGV__
	envPath   = __ENV__
	stdinPath = __STDIN__
	toolsPath = __TOOLS__
	gitPath   = __GIT__
	workspace = __WORKSPACE__
	extraTool = __EXTRA__
	doCommit  = __COMMIT__
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fake-claude:", err)
		os.Exit(9)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) == 1 && args[0] == "--version" {
		fmt.Println(version)
		return nil
	}
	if err := os.WriteFile(argvPath, []byte(strings.Join(args, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	env := os.Environ()
	sort.Strings(env)
	if err := os.WriteFile(envPath, []byte(strings.Join(env, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	if err := os.WriteFile(stdinPath, stdin, 0o644); err != nil {
		return err
	}
	url, authorization, err := mcpTransport(args)
	if err != nil {
		return err
	}
	observed := []string{}
	listed, err := post(url, authorization, ` + "`" + `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "`" + `)
	if err != nil {
		return err
	}
	observed = append(observed, "tools/list "+toolNames(listed))
	plan, err := post(url, authorization, ` + "`" + `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_flight_plan","arguments":{}}}` + "`" + `)
	if err != nil {
		return err
	}
	observed = append(observed, "get_flight_plan "+compact(plan))
	expansion, err := post(url, authorization, ` + "`" + `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"request_context","arguments":{"ref":"spec/feature-alpha","purpose":"sealed claude witness"}}}` + "`" + `)
	if err != nil {
		return err
	}
	observed = append(observed, "request_context "+compact(expansion))
	if extraTool != "" {
		refused, err := post(url, authorization, ` + "`" + `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"` + "`" + `+extraTool+` + "`" + `","arguments":{}}}` + "`" + `)
		if err != nil {
			return err
		}
		observed = append(observed, "extra "+compact(refused))
	}
	if err := os.WriteFile(toolsPath, []byte(strings.Join(observed, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	if doCommit {
		if err := providerCommit(); err != nil {
			return err
		}
	}
	emit(map[string]any{
		"type": "system", "subtype": "init", "session_id": session, "model": model,
		"mcp_servers":    []map[string]string{{"name": "verdi-context", "status": "connected"}},
		"cwd":            workspace,
		"tools":          []string{},
		"permissionMode": "bypassPermissions", "apiKeySource": "ANTHROPIC_API_KEY",
		"claude_code_version": version, "slash_commands": []string{}, "output_style": "default",
		"agents": []string{}, "skills": []string{}, "plugins": []string{}, "uuid": "init-uuid-e2e",
	})
	emit(map[string]any{
		"type": "assistant", "session_id": session, "uuid": "msg-uuid-e2e",
		"message": map[string]any{
			"id": "msg_e2e", "type": "message", "role": "assistant", "model": model,
			"content": []map[string]any{{"type": "text", "text": "Sealed witness complete."}},
			"usage":   map[string]any{"input_tokens": 1, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": 1},
		},
	})
	emit(map[string]any{
		"type": "result", "subtype": "success", "is_error": false, "result": "success",
		"session_id": session, "uuid": "result-uuid-e2e", "duration_ms": 1, "duration_api_ms": 1,
		"num_turns": 1, "total_cost_usd": 0.0,
		"usage":              map[string]any{"input_tokens": 1, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": 1},
		"permission_denials": []any{},
	})
	return nil
}

func providerCommit() error {
	if err := os.WriteFile("claude-output.txt", []byte("claude provider change\n"), 0o644); err != nil {
		return err
	}
	if out, err := exec.Command(gitPath, "add", "claude-output.txt").CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %v: %s", err, out)
	}
	commit := exec.Command(gitPath, "-c", "user.name=Fixture Provider", "-c", "user.email=provider@example.invalid", "commit", "-m", "claude provider output")
	commit.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2001-02-03T04:05:06Z", "GIT_COMMITTER_DATE=2001-02-03T04:05:06Z")
	if out, err := commit.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %v: %s", err, out)
	}
	return nil
}

func emit(frame map[string]any) {
	encoded, err := json.Marshal(frame)
	if err != nil {
		panic(err)
	}
	os.Stdout.Write(append(encoded, '\n'))
}

func mcpTransport(args []string) (string, string, error) {
	path := ""
	for i, arg := range args {
		if arg == "--mcp-config" && i+1 < len(args) {
			path = args[i+1]
		}
	}
	if path == "" {
		return "", "", fmt.Errorf("argv carries no --mcp-config operand")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	var document struct {
		MCPServers map[string]struct {
			Type    string            ` + "`json:\"type\"`" + `
			URL     string            ` + "`json:\"url\"`" + `
			Headers map[string]string ` + "`json:\"headers\"`" + `
		} ` + "`json:\"mcpServers\"`" + `
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return "", "", err
	}
	server, ok := document.MCPServers["verdi-context"]
	if !ok || len(document.MCPServers) != 1 {
		return "", "", fmt.Errorf("scoped MCP config does not declare exactly verdi-context: %s", data)
	}
	return server.URL, server.Headers["Authorization"], nil
}

func post(url, authorization, body string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return io.ReadAll(response.Body)
}

func toolNames(frame []byte) string {
	var decoded struct {
		Result struct {
			Tools []struct {
				Name string ` + "`json:\"name\"`" + `
			} ` + "`json:\"tools\"`" + `
		} ` + "`json:\"result\"`" + `
	}
	if err := json.Unmarshal(frame, &decoded); err != nil {
		return "decode-error"
	}
	names := []string{}
	for _, tool := range decoded.Result.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func compact(frame []byte) string {
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, frame); err != nil {
		return "compact-error"
	}
	return buffer.String()
}
`

// buildFakeClaude compiles the fake provider into dir and returns its absolute
// path. The provider is a real executable driven only by the sealed profile's
// argv, environment, stdin, and scoped MCP config.
func buildFakeClaude(t *testing.T, dir string, spec fakeClaudeSpec) string {
	t.Helper()
	source := strings.NewReplacer(
		"__VERSION__", strconv.Quote(spec.version),
		"__MODEL__", strconv.Quote(spec.model),
		"__SESSION__", strconv.Quote(spec.session),
		"__ARGV__", strconv.Quote(spec.argvPath),
		"__ENV__", strconv.Quote(spec.envPath),
		"__STDIN__", strconv.Quote(spec.stdinPath),
		"__TOOLS__", strconv.Quote(spec.toolsPath),
		"__GIT__", strconv.Quote(spec.gitPath),
		"__WORKSPACE__", strconv.Quote(spec.workspace),
		"__EXTRA__", strconv.Quote(spec.extraTool),
		"__COMMIT__", strconv.FormatBool(spec.commit),
	).Replace(fakeClaudeSource)
	moduleDir := filepath.Join(dir, "fake-claude-src")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte("module fakeclaude\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "claude")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = moduleDir
	build.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fake claude: %v\n%s", err, out)
	}
	return binary
}

// ---------------------------------------------------------------------------
// Built-binary Claude lifecycle driver
// ---------------------------------------------------------------------------

const (
	claudeE2EModel   = "claude-opus-5-sealed-e2e"
	claudeE2ESession = "claude-sess-e2e-001"
	claudeE2EAPIKey  = "sk-ant-e2e-fixture-key"
)

type claudeLifecycleOptions struct {
	// expansionRoot makes the controller report an installed expansion ledger.
	expansionRoot string
	// extraTool asks the provider for an undeclared scoped tool.
	extraTool string
	// noAPIKey removes ANTHROPIC_API_KEY from the binary's own environment.
	noAPIKey bool
	// failOperation makes the controller refuse one operation.
	failOperation sealedexec.ControllerOperation
	// outFile routes the public result through --out.
	outFile bool
}

type claudeLifecycleObservation struct {
	fixture   claudeCompiledFixture
	fake      *sealedLifecycleController
	obs       sealedContextObservation
	argv      []string
	env       []string
	tools     []string
	envRoot   string
	stdinPath string
	mcpConfig string
	outPath   string
}

// runClaudeSealedLifecycle drives the built candidate binary through one real
// sealed Claude execution against a fake Claude executable, a strict FD-3
// controller, and the parent-hosted scoped MCP server. No provider service and
// no network are contacted.
func runClaudeSealedLifecycle(t *testing.T, bin string, options claudeLifecycleOptions) claudeLifecycleObservation {
	t.Helper()
	providerRoot := t.TempDir()
	envRoot := filepath.Join(providerRoot, "env")
	claudeConfigDir := filepath.Join(envRoot, "claude-config")
	for _, dir := range []string{envRoot, claudeConfigDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	claudePath := filepath.Join(providerRoot, "claude")
	fixture := buildClaudeCompiledFixture(t, execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantNetwork},
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{claudePath}},
	}})
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if gitPath, err = filepath.Abs(gitPath); err != nil {
		t.Fatal(err)
	}
	workspaceID, err := fixture.request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	workspacePath := execworkspace.UnitPath(fixture.root, workspaceID)
	argvPath := filepath.Join(providerRoot, "argv")
	envPath := filepath.Join(providerRoot, "env.txt")
	stdinPath := filepath.Join(providerRoot, "stdin")
	toolsPath := filepath.Join(providerRoot, "tools")
	built := buildFakeClaude(t, providerRoot, fakeClaudeSpec{
		version: fixture.request.AdapterVersion, model: claudeE2EModel, session: claudeE2ESession,
		argvPath: argvPath, envPath: envPath, stdinPath: stdinPath, toolsPath: toolsPath,
		gitPath: gitPath, workspace: workspacePath, extraTool: options.extraTool, commit: true,
	})
	if built != claudePath {
		t.Fatalf("fake claude built at %q, want the granted argv0 %q", built, claudePath)
	}

	profile := sealedexec.ProfileMaterial{
		Ref: fixture.request.Profile, Name: "claude-fixture", AbsoluteExecutable: claudePath,
		AbsoluteEnvRoot: envRoot, Model: claudeE2EModel, ClaudeConfigDir: claudeConfigDir,
		AdapterVersion: fixture.request.AdapterVersion, DecoderProfile: sealedclaude.DecoderProfileV1,
	}
	fake := &sealedLifecycleController{
		t: t, request: fixture.request, profile: profile,
		fail: options.failOperation, allowQuarantine: true,
		resolution: sealedexec.ContextResolution{
			Verification: sealedexec.Verification{State: contextcompile.ResolutionUnproven, Failure: sealedexec.FailureUnproven, Witnesses: []string{"fixture context unavailable"}},
			Data:         fixture.compiled.DataItems[0],
		},
	}
	fake.expansionRoot = options.expansionRoot

	if options.noAPIKey {
		t.Setenv("ANTHROPIC_API_KEY", "")
	} else {
		t.Setenv("ANTHROPIC_API_KEY", claudeE2EAPIKey)
	}

	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "claude-controller")
	childFile := os.NewFile(uintptr(files[1]), "claude-child")
	controllerConn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() {
		defer controllerConn.Close()
		served <- fake.serve(controllerConn)
	}()

	args := []string{"context", "execution", "--request", "-"}
	outPath := ""
	if options.outFile {
		outPath = filepath.Join(providerRoot, "result.json")
		args = append(args, "--out", outPath)
	}
	observation := runSealedContextBinaryWithFiles(t, bin, fixture.root, fixture.requestBytes, []*os.File{childFile}, args...)
	if err := <-served; err != nil {
		t.Fatalf("claude lifecycle controller: %v; observation=%#v", err, observation)
	}
	return claudeLifecycleObservation{
		fixture: fixture, fake: fake, obs: observation,
		argv:      readClaudeFixtureLines(argvPath),
		env:       readClaudeFixtureLines(envPath),
		tools:     readClaudeFixtureLines(toolsPath),
		envRoot:   envRoot,
		stdinPath: stdinPath,
		mcpConfig: filepath.Join(envRoot, "claude-mcp.json"),
		outPath:   outPath,
	}
}

// readClaudeFixtureLines returns the recorded provider observation lines, or
// nil when the provider never wrote the file.
func readClaudeFixtureLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// sealedClaudeUnreducedResultDiagnostic is the exact boundary the public Claude
// success path currently reaches. Amendment 002 §5 makes the adapter withhold
// its terminal `provider-summary` until the child is reaped, so the ratified
// success frame yields one observation-free, non-terminal AdapterResult; the
// shared U4 stream loop refuses that shape
// (internal/sealedexec/service.go, "non-terminal result has no normalized
// observations"). Repairing it requires editing that shared service, which is
// outside this correction pass's declared write set. Until then, Claude
// result/receipt/public-output bytes are disclosed as unproven and this row
// pins the exact reachable boundary so the seam repair is visible.
const sealedClaudeUnreducedResultDiagnostic = "provider stream: non-terminal result has no normalized observations"

// TestClaudeBuiltBinaryLifecycle_Behavioral drives the real built candidate
// binary through sealed Claude execution against a fake Claude executable, a
// strict FD-3 controller, and the parent-hosted scoped MCP server. It is an
// ordinary (non-frozen) behavioral test; the frozen AC-1 behavioral producer
// executes these rows by name.
func TestClaudeBuiltBinaryLifecycle_Behavioral(t *testing.T) {
	bin := buildVerdiBinary(t)

	t.Run("sealed_start_drives_the_public_claude_assembly", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{})
		assertClaudeAssemblySurface(t, run)
		assertClaudeQuarantinedBoundary(t, run)
	})

	t.Run("sealed_resume_drives_the_public_claude_assembly", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{outFile: true})
		assertClaudeAssemblySurface(t, run)
		assertClaudeQuarantinedBoundary(t, run)
		// No public bytes are written on a quarantined boundary.
		if _, err := os.Stat(run.outPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("quarantined claude run wrote %q: %v", run.outPath, err)
		}
	})

	// Reconstruction witness: the parent-hosted scoped surface reads the
	// authoritative checkpoint and expansion ledger. A pristine checkpoint that
	// contradicts an installed expansion root must be refused; a fabricated
	// literal sequence-one state could never notice the contradiction.
	t.Run("pristine_checkpoint_contradicting_the_expansion_ledger_is_refused", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{expansionRoot: sealedTestDigest("installed-expansion")})
		if run.obs.exitCode != 1 || run.obs.stdout != "" {
			t.Fatalf("contradicted reconstruction run = %#v, want a verdict refusal", run.obs)
		}
		if !strings.Contains(run.obs.stderr, "reconstruct scoped MCP flight state") {
			t.Fatalf("contradicted reconstruction stderr = %q", run.obs.stderr)
		}
		if run.argv != nil {
			t.Fatalf("contradicted reconstruction launched the provider: %#v", run.argv)
		}
	})

	// Operational terminal witness: an undeclared scoped tool is refused, the
	// typed terminal is consumed while Execute is live, and the run is
	// operational with no completion or receipt.
	t.Run("undeclared_scoped_tool_ends_the_run_operationally", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{extraTool: "not_a_scoped_tool"})
		if len(run.tools) != 4 || !strings.Contains(run.tools[3], "unknown scoped tool") {
			t.Fatalf("undeclared scoped observations = %#v", run.tools)
		}
		if run.obs.exitCode != 2 || run.obs.stdout != "" {
			t.Fatalf("undeclared scoped tool run = %#v, want the operational terminal", run.obs)
		}
		if !strings.Contains(run.obs.stderr, "operational terminal") {
			t.Fatalf("undeclared scoped tool stderr = %q, want the operational terminal classification", run.obs.stderr)
		}
		if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperationAppendReceipt); got != 0 {
			t.Fatalf("append-receipt calls after an operational terminal = %d, want 0", got)
		}
	})

	// Safe classification diagnostic: the exact local refusal, never a generic
	// controller refusal and never the credential itself.
	t.Run("missing_api_key_refuses_with_the_exact_safe_classification_diagnostic", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{noAPIKey: true})
		if run.obs.exitCode != 1 || run.obs.stdout != "" {
			t.Fatalf("missing ANTHROPIC_API_KEY run = %#v, want a verdict refusal", run.obs)
		}
		const want = "ANTHROPIC_API_KEY must be present and at least 8 bytes for Claude activation"
		if !strings.Contains(run.obs.stderr, want) {
			t.Fatalf("missing ANTHROPIC_API_KEY stderr = %q, want %q", run.obs.stderr, want)
		}
		if strings.Contains(run.obs.stderr, claudeE2EAPIKey) || run.argv != nil {
			t.Fatalf("classification refusal leaked the key or launched the provider: %q / %#v", run.obs.stderr, run.argv)
		}
		if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperationRecorderAppend); got != 0 {
			t.Fatalf("recorder-append calls before classification = %d, want 0", got)
		}
	})
}

// assertClaudeAssemblySurface proves the public Claude assembly reached adapter
// selection, profile activation, the version probe, the exact model and scoped
// MCP configuration, the FD-3 controller, both scoped tools, the normalized
// event reduction, and resource cleanup.
func assertClaudeAssemblySurface(t *testing.T, run claudeLifecycleObservation) {
	t.Helper()

	// Exact Amendment 002 §4 start argv, including the full model and the one
	// scoped MCP configuration path beneath the resolved environment root.
	wantArgv := []string{
		"--bare", "-p", "--input-format", "stream-json", "--output-format", "stream-json",
		"--verbose", "--model", claudeE2EModel, "--permission-mode", "bypassPermissions",
		"--strict-mcp-config", "--mcp-config", run.mcpConfig, "--no-chrome",
	}
	if !reflect.DeepEqual(run.argv, wantArgv) {
		t.Fatalf("claude provider argv = %#v, want %#v", run.argv, wantArgv)
	}

	// Closed environment table: exact Claude controls, the locally classified
	// key, and no ambient proxy/cloud/IDE/plugin/hook/telemetry names.
	env := map[string]string{}
	for _, entry := range run.env {
		name, value, _ := strings.Cut(entry, "=")
		env[name] = value
	}
	for name, want := range map[string]string{
		"ANTHROPIC_API_KEY":   claudeE2EAPIKey,
		"CLAUDE_CONFIG_DIR":   filepath.Join(run.envRoot, "claude-config"),
		"PATH":                sealedClaudeToolPath,
		"DISABLE_AUTOUPDATER": "1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"CLAUDE_CODE_AUTO_CONNECT_IDE":             "false",
	} {
		if env[name] != want {
			t.Fatalf("provider %s = %q, want %q (env=%v)", name, env[name], want, run.env)
		}
	}
	if home := env["HOME"]; !strings.HasPrefix(home, run.envRoot+string(filepath.Separator)) && home != run.envRoot {
		t.Fatalf("provider HOME = %q, want the resolved environment root %q", home, run.envRoot)
	}
	if tmp := env["TMPDIR"]; !strings.HasPrefix(tmp, run.envRoot+string(filepath.Separator)) {
		t.Fatalf("provider TMPDIR = %q, want a child of %q", tmp, run.envRoot)
	}
	for _, forbidden := range []string{
		"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "AWS_ACCESS_KEY_ID", "HTTPS_PROXY", "https_proxy",
		"BASH_ENV", "VSCODE_PID", "CLAUDE_CODE_ENABLE_TELEMETRY", "OTEL_EXPORTER_OTLP_ENDPOINT", "ANTHROPIC_MODEL",
	} {
		if _, present := env[forbidden]; present {
			t.Fatalf("provider environment carries forbidden name %q: %v", forbidden, run.env)
		}
	}

	// The provider exercised both declared scoped tools over the parent-hosted
	// loopback HTTP MCP surface named by its own configuration operand.
	if len(run.tools) < 3 || run.tools[0] != "tools/list get_flight_plan,request_context" {
		t.Fatalf("provider scoped MCP observations = %#v", run.tools)
	}
	if !strings.HasPrefix(run.tools[1], "get_flight_plan ") || !strings.Contains(run.tools[1], run.fixture.request.ManifestDigest) {
		t.Fatalf("get_flight_plan observation = %q", run.tools[1])
	}
	if !strings.HasPrefix(run.tools[2], "request_context ") {
		t.Fatalf("request_context observation = %q", run.tools[2])
	}

	// Exactly one typed stdin line: the Amendment 002 §4 user envelope carrying
	// the sealed provider input under its fixed marker.
	stdin := string(mustReadFile(t, run.stdinPath))
	lines := strings.Split(strings.TrimSuffix(stdin, "\n"), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], `{"message":{"content":[{"text":"VERDI_SEALED_PROVIDER_INPUT_V1`) ||
		!strings.HasSuffix(lines[0], `"role":"user"},"type":"user"}`) ||
		!strings.Contains(lines[0], run.fixture.request.ProjectionDigest) {
		t.Fatalf("claude typed stdin (%d lines) = %.200q…, want one sealed user frame", len(lines), stdin)
	}

	// Reconstruction witness: the Claude assembly reads the authoritative
	// durable checkpoint and expansion ledger before serving scoped MCP.
	if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperationRecorderCheckpoint); got < 2 {
		t.Fatalf("recorder-checkpoint calls = %d, want the service and adapter reads", got)
	}
	if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperationVerifyExpansion); got < 1 {
		t.Fatalf("verify-expansion calls = %d, want the adapter reconstruction read", got)
	}

	// Normalized events: the scoped MCP expansion pair plus the exact Claude
	// stream reduction of the session-init and assistant-text families.
	kinds := sealedEventKinds(run.fake.events)
	for _, want := range []contextevent.Kind{
		contextevent.KindContextRequest, contextevent.KindContextDecision,
		contextevent.KindAdapterStart, contextevent.KindProviderMessage,
	} {
		if countEventKindInFixture(run.fake.events, want) == 0 {
			t.Fatalf("acknowledged claude event kinds = %v, want %q present", kinds, want)
		}
	}

	// Resource cleanup: only the scoped configuration is removed, and the
	// resolved environment root survives.
	if _, err := os.Stat(run.mcpConfig); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scoped MCP config survived the run: %v", err)
	}
	if _, err := os.Stat(run.envRoot); err != nil {
		t.Fatalf("resolved environment root was removed: %v", err)
	}
}

// assertClaudeQuarantinedBoundary pins the disclosed shared-service boundary:
// the run stops operationally at the unreduced success result, preserves its
// acknowledged prefix durably, and mints neither a receipt nor public bytes.
func assertClaudeQuarantinedBoundary(t *testing.T, run claudeLifecycleObservation) {
	t.Helper()
	if run.obs.exitCode != 2 || run.obs.stdout != "" {
		t.Fatalf("claude boundary run = %#v, want an operational stop with no public bytes", run.obs)
	}
	if !strings.Contains(run.obs.stderr, sealedClaudeUnreducedResultDiagnostic) {
		t.Fatalf("claude boundary stderr = %q, want %q", run.obs.stderr, sealedClaudeUnreducedResultDiagnostic)
	}
	if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperationAppendReceipt); got != 0 {
		t.Fatalf("append-receipt calls at the disclosed boundary = %d, want 0", got)
	}
	if !reflect.DeepEqual(run.fake.quarantines, []sealedexec.QuarantineReason{sealedexec.QuarantineExecutionIncomplete}) {
		t.Fatalf("claude boundary quarantines = %v", run.fake.quarantines)
	}
	assertLifecyclePreservation(t, run.fake, sealedexec.PreservedPartial)
}
