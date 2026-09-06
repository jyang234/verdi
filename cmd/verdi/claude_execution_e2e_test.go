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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/instructionprojection"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
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
	// bigText makes the assistant text exceed the fixed inline detail ceiling,
	// so its projected detail must become a durable controller segment.
	bigText bool
	// versionSuffix makes the fake's --version probe line print the SI-181
	// product suffix (" (Claude Code)") after version instead of the bare
	// version alone.
	versionSuffix bool
	// unknownMembers makes the fake's init frame carry the exact 8 unknown
	// members and its api_retry frame carry the 1 unknown member the real
	// Claude Code CLI 2.1.261 emits (SI-182, measured offline by the F12
	// canary track, 2026-09-06; values here are synthetic, never the measured
	// bytes). The sealed run must still proceed.
	unknownMembers bool
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
	bigText   = __BIGTEXT__
	// versionSuffix selects the SI-181 " (Claude Code)" probe suffix.
	versionSuffix = __VERSIONSUFFIX__
	// unknownMembers selects the SI-182 2.1.261-shaped unknown-member frames.
	unknownMembers = __UNKNOWNMEMBERS__
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
		if versionSuffix {
			fmt.Println(version + " (Claude Code)")
		} else {
			fmt.Println(version)
		}
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
	servers, err := mcpTransport(args)
	if err != nil {
		return err
	}
	url, authorization := servers["verdi-context"].URL, servers["verdi-context"].Headers["Authorization"]
	claimURL, claimAuthorization := servers["vatc"].URL, servers["vatc"].Headers["Authorization"]
	// Amendment 003: both required registrations complete their initialize
	// handshake before any useful work. Recording them in order is what proves
	// the ordering to the parent.
	// The record is append-only: every observation is one write to a file opened
	// once in append mode, so a row is durable the moment its record call
	// returns. The parent may interrupt this provider the moment the scoped
	// surface raises a terminal, and rewriting the whole file per observation
	// would lose every already-durable row whenever that interruption landed
	// between the truncate and the write.
	tools, err := os.OpenFile(toolsPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer tools.Close()
	record := func(row string) error {
		_, err := tools.Write([]byte(row + "\n"))
		return err
	}
	for _, handshake := range []struct{ name, url, authorization string }{
		{"vatc", claimURL, claimAuthorization},
		{"verdi-context", url, authorization},
	} {
		if _, err := post(handshake.url, handshake.authorization, ` + "`" + `{"jsonrpc":"2.0","id":0,"method":"initialize"}` + "`" + `); err != nil {
			return err
		}
		if err := record("initialize " + handshake.name); err != nil {
			return err
		}
	}
	// Amendment 002 §4/§7 provider order: a real Claude announces its session
	// before it can call a scoped tool, so the exact valid system/init frame is
	// written — and its write completed — before the scoped MCP surface is
	// exercised at all. The parent therefore reduces init into adapter-start
	// ahead of any MCP-owned context transition, which is what makes a prepared
	// resume open on resume, adapter-start.
	initFrame := map[string]any{
		"type": "system", "subtype": "init", "session_id": session, "model": model,
		"mcp_servers":    []map[string]string{{"name": "vatc", "status": "connected"}, {"name": "verdi-context", "status": "connected"}},
		"cwd":            workspace,
		"tools":          []string{},
		"permissionMode": "bypassPermissions", "apiKeySource": "ANTHROPIC_API_KEY",
		"claude_code_version": version, "slash_commands": []string{}, "output_style": "default",
		"agents": []string{}, "skills": []string{}, "plugins": []string{}, "uuid": "init-uuid-e2e",
	}
	if unknownMembers {
		// SI-182: the exact 8 unknown system/init members the real Claude Code
		// CLI 2.1.261 emits (measured offline by the F12 canary track,
		// 2026-09-06); values here are synthetic, never the measured bytes.
		initFrame["analytics_disabled"] = false
		initFrame["capabilities"] = []string{"interrupt_receipt_v1"}
		initFrame["fast_mode_disabled_reason"] = "sdk_opt_in_required"
		initFrame["fast_mode_state"] = "off"
		initFrame["memory_paths"] = map[string]string{"auto": "/synthetic/memory/"}
		initFrame["messaging_socket_path"] = "/synthetic/cc.sock"
		initFrame["product_feedback_disabled"] = false
		initFrame["terminal_slash_commands"] = []string{"doctor"}
	}
	if err := emit(initFrame); err != nil {
		return err
	}
	if unknownMembers {
		// SI-182: the one unknown system/api_retry member the real Claude Code
		// CLI 2.1.261 emits, error_status (measured offline by the F12 canary
		// track, 2026-09-06); error keeps its required object shape here, so
		// the run proceeds past this frame to the sealed retry observation.
		if err := emit(map[string]any{
			"type": "system", "subtype": "api_retry", "session_id": session,
			"attempt": 1, "max_retries": 10, "retry_delay_ms": 1, "error_status": nil,
			"error": map[string]string{"type": "unknown", "message": "synthetic retry"},
			"uuid":  "retry-uuid-e2e",
		}); err != nil {
			return err
		}
	}
	claimed, err := post(claimURL, claimAuthorization, ` + "`" + `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"claim_paths","arguments":{}}}` + "`" + `)
	if err != nil {
		return err
	}
	if err := record("claim_paths " + compact(claimed)); err != nil {
		return err
	}
	// A cross token must be refused by the server that did not mint it.
	crossStatus, err := postStatus(claimURL, authorization, ` + "`" + `{"jsonrpc":"2.0","id":2,"method":"initialize"}` + "`" + `)
	if err != nil {
		return err
	}
	if err := record(fmt.Sprintf("cross_token %d", crossStatus)); err != nil {
		return err
	}
	reverseStatus, err := postStatus(url, claimAuthorization, ` + "`" + `{"jsonrpc":"2.0","id":3,"method":"initialize"}` + "`" + `)
	if err != nil {
		return err
	}
	if err := record(fmt.Sprintf("reverse_cross_token %d", reverseStatus)); err != nil {
		return err
	}
	listed, err := post(url, authorization, ` + "`" + `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "`" + `)
	if err != nil {
		return err
	}
	if err := record("tools/list " + toolNames(listed)); err != nil {
		return err
	}
	plan, err := post(url, authorization, ` + "`" + `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_flight_plan","arguments":{}}}` + "`" + `)
	if err != nil {
		return err
	}
	if err := record("get_flight_plan " + compact(plan)); err != nil {
		return err
	}
	expansion, err := post(url, authorization, ` + "`" + `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"request_context","arguments":{"ref":"spec/feature-alpha","purpose":"sealed claude witness"}}}` + "`" + `)
	if err != nil {
		return err
	}
	if err := record("request_context " + compact(expansion)); err != nil {
		return err
	}
	if extraTool != "" {
		// The undeclared call is recorded before it is issued, so the fact that
		// this provider made it is durable. Its refusal frame arrives while the
		// scoped surface is already raising the terminal that interrupts this
		// provider, so only that last row races the interruption.
		if err := record("extra_call " + extraTool); err != nil {
			return err
		}
		refused, err := post(url, authorization, ` + "`" + `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"` + "`" + `+extraTool+` + "`" + `","arguments":{}}}` + "`" + `)
		if err != nil {
			return err
		}
		if err := record("extra " + compact(refused)); err != nil {
			return err
		}
	}
	if doCommit {
		if err := providerCommit(); err != nil {
			return err
		}
	}
	text := "Sealed witness complete."
	if bigText {
		text = strings.Repeat("a", 20000)
	}
	if err := emit(map[string]any{
		"type": "assistant", "session_id": session, "uuid": "msg-uuid-e2e",
		"message": map[string]any{
			"id": "msg_e2e", "type": "message", "role": "assistant", "model": model,
			"content": []map[string]any{{"type": "text", "text": text}},
			"usage":   map[string]any{"input_tokens": 1, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": 1},
		},
	}); err != nil {
		return err
	}
	return emit(map[string]any{
		"type": "result", "subtype": "success", "is_error": false, "result": "success",
		"session_id": session, "uuid": "result-uuid-e2e", "duration_ms": 1, "duration_api_ms": 1,
		"num_turns": 1, "total_cost_usd": 0.0,
		"usage":              map[string]any{"input_tokens": 1, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0, "output_tokens": 1},
		"permission_denials": []any{},
	})
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

// emit writes one complete stream-json frame and returns only after that write
// has actually happened, so a later scoped MCP call cannot overtake a frame the
// provider has not yet handed to the parent.
func emit(frame map[string]any) error {
	encoded, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	written, err := os.Stdout.Write(encoded)
	if err != nil {
		return err
	}
	if written != len(encoded) {
		return fmt.Errorf("short frame write: %d of %d bytes", written, len(encoded))
	}
	return nil
}

type mcpServerConfig struct {
	Type    string            ` + "`json:\"type\"`" + `
	URL     string            ` + "`json:\"url\"`" + `
	Headers map[string]string ` + "`json:\"headers\"`" + `
}

func mcpTransport(args []string) (map[string]mcpServerConfig, error) {
	path := ""
	for i, arg := range args {
		if arg == "--mcp-config" && i+1 < len(args) {
			path = args[i+1]
		}
	}
	if path == "" {
		return nil, fmt.Errorf("argv carries no --mcp-config operand")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		MCPServers map[string]mcpServerConfig ` + "`json:\"mcpServers\"`" + `
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if len(document.MCPServers) != 2 {
		return nil, fmt.Errorf("scoped MCP config does not declare exactly two servers: %s", data)
	}
	for _, name := range []string{"vatc", "verdi-context"} {
		server, ok := document.MCPServers[name]
		if !ok || server.Type != "http" || server.URL == "" || server.Headers["Authorization"] == "" {
			return nil, fmt.Errorf("scoped MCP config does not declare required server %q: %s", name, data)
		}
	}
	if document.MCPServers["vatc"].Headers["Authorization"] == document.MCPServers["verdi-context"].Headers["Authorization"] {
		return nil, fmt.Errorf("scoped MCP config reused one capability across both servers: %s", data)
	}
	return document.MCPServers, nil
}

func postStatus(url, authorization, body string) (int, error) {
	request, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	_, _ = io.ReadAll(response.Body)
	return response.StatusCode, nil
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
		"__BIGTEXT__", strconv.FormatBool(spec.bigText),
		"__VERSIONSUFFIX__", strconv.FormatBool(spec.versionSuffix),
		"__UNKNOWNMEMBERS__", strconv.FormatBool(spec.unknownMembers),
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
	// outFile routes the public result through --out. It is only a choice of
	// public output channel and never selects an execution arm.
	outFile bool
	// resume drives the sealed resume arm: the request carries the canonical
	// continuity checkpoint instead of the start arm, so the built binary must
	// take the Claude adapter's Resume and the `--resume S` argv.
	resume bool
	// oversizedDetail makes the provider emit an assistant text larger than the
	// fixed inline detail ceiling, forcing a durable controller segment.
	oversizedDetail bool
	// approvedContext makes the controller resolve the provider's real
	// `request_context` call to proven data on a proven epoch, so the embedded
	// scoped MCP compiles, acknowledges, and installs one child manifest. The
	// shared flight state therefore moves to the child revision mid-run and
	// every terminal artifact must bind that revision.
	approvedContext bool
	// claudeCodeSuffix makes the fake provider's --version probe line carry
	// the SI-181 " (Claude Code)" product suffix after the adapter version
	// instead of the bare version; the sealed launch must still succeed.
	claudeCodeSuffix bool
	// unknownMembers makes the fake provider's init and api_retry frames
	// carry the SI-182 2.1.261-shaped unknown members; the sealed run must
	// still proceed.
	unknownMembers bool
}

// serveWithAcknowledgedExpansionLedger runs the shared lifecycle controller
// wire and supplies the two durable facts the shared single-revision fixture
// cannot know in advance: once the embedded scoped MCP has actually
// acknowledged a `child-manifest`, the durable recorder carries one terminal
// revision segment per acknowledged revision, and the receipt's expansion
// ledger must name that exact acknowledged transition. Both are derived only
// from the acknowledged events and their own acknowledgments, so neither can
// assert a transition or an order the run did not make. Every other result, and
// the call-sequence bookkeeping, is the shared fixture's own.
func serveWithAcknowledgedExpansionLedger(fake *sealedLifecycleController, conn net.Conn) error {
	reader := bufio.NewReader(conn)
	var installed []contextreceipt.Expansion
	revisions := append([]contextevent.Revision{}, fake.initialRevisions...)
	for {
		frame, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) && len(frame) == 0 {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read controller frame: %w", err)
		}
		call, err := sealedexec.DecodeControllerCall(bytes.NewReader(frame))
		if err != nil {
			return fmt.Errorf("decode controller call: %w", err)
		}
		if call.CallSequence != uint64(len(fake.calls)+1) {
			return fmt.Errorf("controller call sequence %d, want %d", call.CallSequence, len(fake.calls)+1)
		}
		fake.calls = append(fake.calls, call.Operation)
		result, err := fake.result(call)
		if err != nil {
			// The shared fixture's checkpoint builder models exactly one
			// revision, so an installed expansion is the one case it cannot
			// represent. Everything else stays its own failure.
			if call.Operation != sealedexec.ControllerOperationRecorderCheckpoint || len(installed) == 0 {
				return err
			}
			result = sealedexec.ControllerResult{
				Schema: sealedexec.ControllerResultSchemaID, CallSequence: call.CallSequence, Operation: call.Operation,
				RecorderCheckpoint: sealedexec.ControllerRecorderCheckpointResult{
					Schema: "verdi.context-controller/" + string(call.Operation) + "-result/v1",
					Checkpoint: sealedexec.RecorderCheckpoint{
						Verification: sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
						Digest:       sealedTestDigest(fmt.Sprintf("checkpoint-%d", len(fake.events))),
					},
				},
			}
		}
		if call.Operation == sealedexec.ControllerOperationRecorderAppend && result.Error == nil {
			event, ack := call.RecorderAppend.Event, result.RecorderAppend.Ack
			if child, ok := event.Payload.(*contextevent.ChildManifestPayload); ok {
				installed = append(installed, contextreceipt.Expansion{
					RequestID: child.RequestID, ParentRevision: child.ParentRevision,
					ParentManifestDigest: child.ParentManifestDigest, ChildRevision: child.ChildRevision,
					ChildManifestDigest: child.ChildManifestDigest, ExpansionDigest: child.ExpansionDigest,
				})
			}
			if len(revisions) != 0 && revisions[len(revisions)-1].ManifestRevision == event.ManifestRevision {
				current := revisions[len(revisions)-1]
				current.TerminalGlobalSequence = ack.GlobalSequence
				current.TerminalSourceSequence = event.SourceSequence
				current.TerminalKind = event.Kind
				current.EventRoot = event.EventDigest
				revisions[len(revisions)-1] = current
			} else {
				revisions = append(revisions, contextevent.Revision{
					Schema: contextevent.RevisionSchemaID, ManifestRevision: event.ManifestRevision,
					ManifestDigest: event.ManifestDigest, FirstGlobalSequence: ack.GlobalSequence,
					TerminalGlobalSequence: ack.GlobalSequence, TerminalSourceSequence: event.SourceSequence,
					TerminalKind: event.Kind, EventRoot: event.EventDigest,
				})
			}
		}
		if len(installed) != 0 && result.Error == nil {
			switch call.Operation {
			case sealedexec.ControllerOperationResolveReceiptInputs:
				inputs := result.ResolveReceiptInputs.Inputs
				inputs.Expansions = append(append([]contextreceipt.Expansion(nil), inputs.Expansions...), installed...)
				result.ResolveReceiptInputs.Inputs = inputs
			case sealedexec.ControllerOperationRecorderCheckpoint:
				checkpoint := result.RecorderCheckpoint.Checkpoint
				checkpoint.Revisions = append([]contextevent.Revision(nil), revisions...)
				root, rootErr := contextevent.EventChainRoot(checkpoint.Revisions)
				if rootErr != nil {
					return fmt.Errorf("acknowledged revision chain root: %w", rootErr)
				}
				terminal := checkpoint.Revisions[len(checkpoint.Revisions)-1]
				checkpoint.EventChainRoot = root
				checkpoint.TerminalSourceSequence = terminal.TerminalSourceSequence
				checkpoint.TerminalGlobalSequence = terminal.TerminalGlobalSequence
				result.RecorderCheckpoint.Checkpoint = checkpoint
			}
		}
		encoded, err := sealedexec.EncodeControllerResult(result)
		if err != nil {
			return fmt.Errorf("encode %s result: %w", call.Operation, err)
		}
		if _, err := conn.Write(encoded); err != nil {
			return fmt.Errorf("write %s result: %w", call.Operation, err)
		}
	}
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
	claim     *fakeClaimMCP
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
	resumeCheckpoint := claudeResumeCheckpoint{}
	if options.resume {
		resumeCheckpoint = prepareClaudeResume(t, &fixture, workspaceID)
	}
	argvPath := filepath.Join(providerRoot, "argv")
	envPath := filepath.Join(providerRoot, "env.txt")
	stdinPath := filepath.Join(providerRoot, "stdin")
	toolsPath := filepath.Join(providerRoot, "tools")
	built := buildFakeClaude(t, providerRoot, fakeClaudeSpec{
		version: fixture.request.AdapterVersion, model: claudeE2EModel, session: claudeE2ESession,
		argvPath: argvPath, envPath: envPath, stdinPath: stdinPath, toolsPath: toolsPath,
		gitPath: gitPath, workspace: workspacePath, extraTool: options.extraTool, commit: true,
		bigText: options.oversizedDetail, versionSuffix: options.claudeCodeSuffix,
		unknownMembers: options.unknownMembers,
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
	if options.approvedContext {
		// A proven resolution on a proven epoch is the only difference: the
		// scoped MCP then runs its real compile/acknowledge/install path.
		fake.resolution = sealedexec.ContextResolution{
			Verification: sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
			Data:         fixture.compiled.DataItems[0],
		}
		fake.epoch = sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}}
	}
	// Amendment 003: vatc is ATC-owned, so the harness — not Verdi — hosts it.
	// Its capability is derived independently from the same canonical request
	// bytes, which is exactly how the real ATC parent authenticates the caller.
	claimServer := startFakeClaimMCP(t, fixture.requestBytes)
	fake.claimMCPURL = claimServer.url

	fake.expansionRoot = options.expansionRoot
	if options.resume {
		// The durable state the prepared continuity asserts: the completed prior
		// revision, its recorder checkpoint digest, and the installed expansion
		// ledger root the resumed flight reconstructs from.
		fake.initialRevisions = []contextevent.Revision{resumeCheckpoint.revision}
		fake.checkpointDigest = resumeCheckpoint.checkpointDigest
		fake.expansionRoot = resumeCheckpoint.expansionRoot
		fake.global = resumeCheckpoint.revision.TerminalGlobalSequence
	}

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
		if options.approvedContext {
			served <- serveWithAcknowledgedExpansionLedger(fake, controllerConn)
			return
		}
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
		fixture: fixture, fake: fake, obs: observation, claim: claimServer,
		argv:      readClaudeFixtureLines(argvPath),
		env:       readClaudeFixtureLines(envPath),
		tools:     readClaudeFixtureLines(toolsPath),
		envRoot:   envRoot,
		stdinPath: stdinPath,
		mcpConfig: filepath.Join(envRoot, "claude-mcp.json"),
		outPath:   outPath,
	}
}

// fakeClaimMCP is the ATC-owned vatc server. It derives the invocation-scoped
// capability from the same canonical request bytes Verdi uses, so a token minted
// for a different request — or the Verdi context token — is refused.
type fakeClaimMCP struct {
	url    string
	server *http.Server

	mu        sync.Mutex
	accepted  []string
	rejected  int
	initCount int
}

func (f *fakeClaimMCP) observed() ([]string, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.accepted...), f.rejected, f.initCount
}

func startFakeClaimMCP(t *testing.T, requestBytes []byte) *fakeClaimMCP {
	t.Helper()
	requestDigest, err := sealedexec.CanonicalRequestDigest(requestBytes)
	if err != nil {
		t.Fatalf("canonical request digest: %v", err)
	}
	capability, err := sealedexec.ClaimMCPCapability(requestDigest)
	if err != nil {
		t.Fatalf("claim capability: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen claim MCP: %v", err)
	}
	claim := &fakeClaimMCP{url: "http://" + listener.Addr().String() + "/mcp"}
	authorization := "Bearer " + capability
	claim.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != authorization {
			claim.mu.Lock()
			claim.rejected++
			claim.mu.Unlock()
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var frame struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&frame); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		claim.mu.Lock()
		switch frame.Method {
		case "initialize":
			claim.initCount++
		case "tools/call":
			claim.accepted = append(claim.accepted, frame.Params.Name)
		}
		claim.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch frame.Method {
		case "initialize":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%v,"result":{"protocolVersion":"2024-11-05","serverInfo":{"name":"vatc","version":"1"}}}`, frame.ID)
		case "tools/list":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%v,"result":{"tools":[{"name":"claim_paths"}]}}`, frame.ID)
		case "tools/call":
			if frame.Params.Name != "claim_paths" {
				fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%v,"error":{"code":-32601,"message":"unknown tool"}}`, frame.ID)
				return
			}
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%v,"result":{"content":[{"type":"text","text":"{\"kind\":\"claim-recorded\"}"}]}}`, frame.ID)
		default:
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%v,"error":{"code":-32601,"message":"unknown method"}}`, frame.ID)
		}
	})}
	go func() { _ = claim.server.Serve(listener) }()
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = claim.server.Shutdown(shutdownCtx)
	})
	return claim
}

// claudeResumeCheckpoint is the durable state the prepared continuity asserts.
// The controller must report exactly these facts or the built binary refuses
// the resume before any provider is launched.
type claudeResumeCheckpoint struct {
	revision         contextevent.Revision
	checkpointDigest string
	expansionRoot    string
}

// prepareClaudeResume rewrites the compiled start fixture into the sealed
// resume arm and returns the controller facts its continuity claims. The
// workspace is materialized first because a resume continues an existing
// execution workspace rather than creating one, and the continuity names
// claudeE2ESession as the adapter session ref — exactly the session the fake
// provider reports — so the binary's resumed-stream identity check and the
// `--resume S` operand both bind one real provider session.
func prepareClaudeResume(t *testing.T, fixture *claudeCompiledFixture, workspaceID string) claudeResumeCheckpoint {
	t.Helper()
	materializer, err := execworkspace.NewMaterializer(fixture.root, fixture.root, execworkspace.NewGitReconciler(fixture.root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Materialize(context.Background(), execworkspace.Request{Identity: fixture.request.ExecutionWorkspaceRequest}); err != nil {
		t.Fatalf("materialize claude resume workspace: %v", err)
	}
	workspaceDigest, err := sealedexec.ExecutionWorkspaceRequestDigest(fixture.request.ExecutionWorkspaceRequest)
	if err != nil {
		t.Fatal(err)
	}
	grantBytes, err := execworkspace.EncodeGrantSet(fixture.request.Grants)
	if err != nil {
		t.Fatal(err)
	}
	revision := contextevent.Revision{
		Schema: contextevent.RevisionSchemaID, ManifestRevision: fixture.request.ManifestRevision,
		ManifestDigest: fixture.request.ManifestDigest, FirstGlobalSequence: 1,
		TerminalGlobalSequence: 3, TerminalSourceSequence: 3,
		TerminalKind: contextevent.KindExecutionResult, EventRoot: sealedTestDigest("claude-resume-prior-event"),
	}
	revisionRoot, err := contextevent.EventChainRoot([]contextevent.Revision{revision})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := claudeResumeCheckpoint{
		revision:         revision,
		checkpointDigest: sealedTestDigest("claude-resume-checkpoint"),
		expansionRoot:    sealedTestDigest("claude-resume-expansion-root"),
	}
	continuity := sealedexec.ExecutionContinuity{
		Schema: sealedexec.ExecutionContinuitySchemaID,
		Flight: fixture.request.Flight, Lane: fixture.request.Lane, Epoch: fixture.request.Epoch, Session: fixture.request.Session,
		Adapter: fixture.request.Adapter, AdapterVersion: fixture.request.AdapterVersion, ATCRunway: fixture.root,
		InputCommit: fixture.head, InputTree: fixture.tree, CurrentCommit: fixture.head, CurrentTree: fixture.tree,
		ExecutionWorkspaceID: workspaceID, ExecutionWorkspaceRequestDigest: workspaceDigest,
		ProfileDigest: fixture.request.Profile.Digest, GrantDigest: sealedRawDigest(grantBytes),
		AuthorityVerdictDigest:  fixture.request.AuthorityVerdict.Digest,
		CurrentManifestRevision: fixture.request.ManifestRevision, CurrentManifestDigest: fixture.request.ManifestDigest,
		ProjectionDigest: fixture.request.ProjectionDigest,
		RevisionSegments: []contextevent.Revision{revision}, EventChainRoot: revisionRoot,
		ExpansionLedgerRoot:    checkpoint.expansionRoot,
		TerminalSourceSequence: revision.TerminalSourceSequence, TerminalGlobalSequence: revision.TerminalGlobalSequence,
		RecorderCheckpointDigest: checkpoint.checkpointDigest, AdapterSessionRef: claudeE2ESession,
	}
	continuityBytes, err := sealedexec.EncodeExecutionContinuity(continuity)
	if err != nil {
		t.Fatalf("encode claude resume continuity: %v", err)
	}
	if continuity, err = sealedexec.DecodeExecutionContinuity(bytes.NewReader(continuityBytes)); err != nil {
		t.Fatal(err)
	}
	fixture.request.Action = sealedexec.ActionResume
	fixture.request.Start = nil
	fixture.request.Resume = &sealedexec.ResumeArm{Continuity: continuity, ContinuityDigest: continuity.Digest}
	if fixture.requestBytes, err = sealedexec.EncodeExecutionRequest(fixture.request); err != nil {
		t.Fatalf("encode claude resume request: %v", err)
	}
	return checkpoint
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

// TestClaudeBuiltBinaryLifecycle_Behavioral drives the real built candidate
// binary through sealed Claude execution against a fake Claude executable, a
// strict FD-3 controller, and the parent-hosted scoped MCP server. It is an
// ordinary (non-frozen) behavioral test; the frozen AC-1 behavioral producer
// executes these rows by name.
func TestClaudeBuiltBinaryLifecycle_Behavioral(t *testing.T) {
	bin := buildVerdiBinary(t)

	t.Run("sealed_start_drives_the_public_claude_assembly", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{approvedContext: true})
		assertClaudeAssemblySurface(t, run)
		assertClaudeSuccessfulLifecycle(t, run)
		assertClaudeChildRevisionBinding(t, run)
	})

	// SI-181: Amendment 002 §3 (as annotated 2026-09-06) also accepts a probe
	// line carrying the real Claude Code CLI's exact product suffix; the
	// sealed launch must proceed identically to the bare-form probe above.
	t.Run("sealed_start_accepts_the_claude_code_suffixed_version_probe", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{claudeCodeSuffix: true})
		assertClaudeSuccessfulLifecycle(t, run)
	})

	// SI-182: the real Claude Code CLI 2.1.261 emits system/init and
	// system/api_retry members Amendment 002 §5's v1 tables do not list
	// (measured offline by the F12 canary track, 2026-09-06). The sealed run
	// must tolerate them, record their exact sorted dotted paths in the
	// acknowledged event stream, and proceed to the same successful
	// lifecycle. Tolerance alone is not the contract: §I-108 requires the
	// disclosure, so the recorded details are asserted, not only the exit.
	t.Run("sealed_start_tolerates_the_2_1_261_unknown_member_frames", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{unknownMembers: true})
		assertClaudeSuccessfulLifecycle(t, run)
		if countEventKindInFixture(run.fake.events, contextevent.KindRetry) == 0 {
			t.Fatalf("unknown-member arm should still produce a retry event; kinds = %v", sealedEventKinds(run.fake.events))
		}
		const wantInit = `"unknown-foreign-member":["analytics_disabled","capabilities",` +
			`"fast_mode_disabled_reason","fast_mode_state","memory_paths","messaging_socket_path",` +
			`"product_feedback_disabled","terminal_slash_commands"]`
		const wantRetry = `"unknown-foreign-member":["error_status"]`
		var initDisclosed, retryDisclosed bool
		for _, event := range run.fake.events {
			detail := eventDetail(event)
			if detail == nil {
				continue
			}
			if bytes.Contains(detail.RedactedJSON, []byte(wantInit)) {
				initDisclosed = true
			}
			if bytes.Contains(detail.RedactedJSON, []byte(wantRetry)) {
				retryDisclosed = true
			}
		}
		if !initDisclosed || !retryDisclosed {
			t.Fatalf("acknowledged details disclosed init=%v retry=%v, want both; details = %s",
				initDisclosed, retryDisclosed, sealedEventDetails(run.fake.events))
		}
	})

	// Amendment 003: every sealed session resolves exactly one ATC-owned claim
	// registration over FD 3 and hands the provider exactly two required
	// registrations, neither of which leaks its capability or bearer.
	t.Run("amendment_003_resolves_and_projects_both_required_registrations", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{approvedContext: true})
		assertClaudeSuccessfulLifecycle(t, run)
		if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperation("resolve-claim-mcp")); got != 1 {
			t.Fatalf("resolve-claim-mcp calls = %d, want exactly one", got)
		}
		assertClaudeDualRegistrationWitness(t, run)
	})

	// Amendment 002 §9 requires real built sealed-resume evidence, so this row
	// drives the resume arm itself: --out is only an orthogonal choice of public
	// output channel and never the thing that distinguishes resume from start.
	t.Run("sealed_resume_drives_the_public_claude_assembly", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{resume: true, outFile: true, approvedContext: true})
		assertClaudeAssemblySurface(t, run)
		assertClaudeSuccessfulLifecycle(t, run)
		assertClaudeResumeWitness(t, run)
		assertClaudeChildRevisionBinding(t, run)
	})

	// Amendment 002 §6: a projected detail larger than the fixed inline ceiling
	// is stored through the controller-backed segment port and the acknowledged
	// event carries only its deterministic reference, never the bytes.
	t.Run("oversized_detail_is_stored_as_a_controller_segment", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{oversizedDetail: true})
		assertClaudeSuccessfulLifecycle(t, run)
		if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperationStoreRedactedSegment); got == 0 {
			t.Fatalf("store-redacted-segment calls = %d, want at least one", got)
		}
		segmented := 0
		for _, event := range run.fake.events {
			detail := eventDetail(event)
			if detail == nil || detail.Mode != contextevent.DetailSegment {
				continue
			}
			segmented++
			if len(detail.RedactedJSON) != 0 {
				t.Fatalf("acknowledged %s segment detail inlined %d bytes", event.Kind, len(detail.RedactedJSON))
			}
			stored, ok := run.fake.segments[detail.Reference]
			if !ok || uint64(len(stored.Bytes)) != detail.ByteCount {
				t.Fatalf("acknowledged %s reference %q resolved to %d bytes, want %d", event.Kind, detail.Reference, len(stored.Bytes), detail.ByteCount)
			}
			if detail.ByteCount <= 16384 {
				t.Fatalf("segment detail is %d bytes, want more than the 16,384 inline ceiling", detail.ByteCount)
			}
		}
		if segmented == 0 {
			t.Fatalf("no acknowledged claude detail became a controller segment; kinds = %v", sealedEventKinds(run.fake.events))
		}
	})

	// Ownership witness: the execution service reads the authoritative
	// checkpoint and expansion ledger and cross-matches them before it
	// constructs the one shared flight state. A pristine checkpoint that
	// contradicts an installed expansion root must be refused; a fabricated
	// literal sequence-one state could never notice the contradiction.
	t.Run("pristine_checkpoint_contradicting_the_expansion_ledger_is_refused", func(t *testing.T) {
		run := runClaudeSealedLifecycle(t, bin, claudeLifecycleOptions{expansionRoot: sealedTestDigest("installed-expansion")})
		if run.obs.exitCode != 1 || run.obs.stdout != "" {
			t.Fatalf("contradicted reconstruction run = %#v, want a verdict refusal", run.obs)
		}
		if !strings.Contains(run.obs.stderr, "pristine start checkpoint contradicts an installed expansion ledger") {
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
		// Both required handshakes and every declared-catalogue call are durably
		// recorded before the undeclared call is issued, and the undeclared call
		// itself is recorded before it is sent. Those nine rows are proof.
		assertClaudeDurableScopedPrelude(t, run)
		if len(run.tools) < 9 || run.tools[8] != "extra_call not_a_scoped_tool" {
			t.Fatalf("undeclared scoped observations = %#v; observation=%#v", run.tools, run.obs)
		}
		// The refusal frame is the one row that races the parent's interruption:
		// the scoped surface answers `isError` and raises the terminal that kills
		// this provider in the same instant. It is asserted exactly when the
		// provider durably recorded it and never invented when it did not, and no
		// row may follow it either way.
		if len(run.tools) > 10 {
			t.Fatalf("provider recorded %d rows past the undeclared call = %#v, want at most its refusal", len(run.tools)-9, run.tools)
		}
		if len(run.tools) == 10 && !strings.Contains(run.tools[9], "unknown scoped tool") {
			t.Fatalf("undeclared scoped refusal = %q, want the scoped surface's refusal", run.tools[9])
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

// assertClaudeDurableScopedPrelude proves the eight scoped observations every
// launched provider records before it can reach any further tool call:
// Amendment 003's two required initialize handshakes first, then the ATC-owned
// claim answered on its own server, each server refusing the other's
// capability, and only then useful work against the Verdi-owned catalogue.
//
// Every one of these rows is appended by a write that returned before the next
// request was issued, so they are durable: a later parent interruption can
// neither erase nor truncate them, and an empty or short record is a real
// provider failure rather than a recording race.
func assertClaudeDurableScopedPrelude(t *testing.T, run claudeLifecycleObservation) {
	t.Helper()
	wantPrefix := []string{"initialize vatc", "initialize verdi-context"}
	if len(run.tools) < 8 || !reflect.DeepEqual(run.tools[:2], wantPrefix) ||
		!strings.HasPrefix(run.tools[2], "claim_paths ") || run.tools[3] != "cross_token 401" ||
		run.tools[4] != "reverse_cross_token 401" || run.tools[5] != "tools/list get_flight_plan,request_context" {
		t.Fatalf("provider scoped MCP observations = %#v", run.tools)
	}
	if !strings.HasPrefix(run.tools[6], "get_flight_plan ") || !strings.Contains(run.tools[6], run.fixture.request.ManifestDigest) {
		t.Fatalf("get_flight_plan observation = %q", run.tools[6])
	}
	if !strings.HasPrefix(run.tools[7], "request_context ") {
		t.Fatalf("request_context observation = %q", run.tools[7])
	}
}

// assertClaudeAssemblySurface proves the public Claude assembly reached adapter
// selection, profile activation, the version probe, the exact model and scoped
// MCP configuration, the FD-3 controller, both scoped tools, the normalized
// event reduction, and resource cleanup.
func assertClaudeAssemblySurface(t *testing.T, run claudeLifecycleObservation) {
	t.Helper()

	// An empty argv means the fake provider never ran, so the binary refused
	// before launch. Report that refusal rather than an unreadable nil diff.
	if len(run.argv) == 0 {
		t.Fatalf("provider was never launched: exit %d\nstderr: %s\nstdout: %s", run.obs.exitCode, run.obs.stderr, run.obs.stdout)
	}

	// Exact Amendment 002 §4 start argv, including the full model and the one
	// scoped MCP configuration path beneath the resolved environment root.
	wantArgv := []string{
		"--bare", "-p", "--input-format", "stream-json", "--output-format", "stream-json",
		"--verbose", "--model", claudeE2EModel, "--permission-mode", "bypassPermissions",
		"--strict-mcp-config", "--mcp-config", run.mcpConfig, "--no-chrome",
	}
	// Amendment 002 §4 resume order: the start argv with the verified provider
	// session appended as the exact `--resume S` tail.
	if run.fixture.request.Action == sealedexec.ActionResume {
		wantArgv = append(wantArgv, "--resume", run.fixture.request.Resume.Continuity.AdapterSessionRef)
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

	assertClaudeDurableScopedPrelude(t, run)

	// Exactly one typed stdin line: the Amendment 002 §4 user envelope carrying
	// the sealed provider input under its fixed marker.
	stdin := string(mustReadFile(t, run.stdinPath))
	lines := strings.Split(strings.TrimSuffix(stdin, "\n"), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], `{"message":{"content":[{"text":"VERDI_SEALED_PROVIDER_INPUT_V1`) ||
		!strings.HasSuffix(lines[0], `"role":"user"},"type":"user"}`) ||
		!strings.Contains(lines[0], run.fixture.request.ProjectionDigest) {
		t.Fatalf("claude typed stdin (%d lines) = %.200q…, want one sealed user frame", len(lines), stdin)
	}

	// I-115 ownership witness: the execution service alone reads the durable
	// checkpoint and expansion ledger, then constructs the one flight state and
	// hands the embedded Claude assembly that exact pointer. The assembly
	// therefore performs no second reconstruction read — only the service's
	// prerequisite proof and completion's terminal checkpoint remain, and only
	// the resume arm proves the expansion ledger.
	if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperationVerifyExpansion); got != 1 {
		t.Fatalf("verify-expansion calls = %d, want exactly the one service cross-match and no adapter reconstruction", got)
	}
	if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperationRecorderCheckpoint); got != 2 {
		t.Fatalf("recorder-checkpoint calls = %d, want exactly the service prerequisite and the completion terminal read\nexit %d\nstderr: %s", got, run.obs.exitCode, run.obs.stderr)
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

// assertClaudeResumeWitness proves the built candidate really took the sealed
// resume arm rather than a start that merely differs by output routing: the
// encoded request carries ActionResume with the canonical continuity, the
// provider was launched through the Claude adapter's Resume with the exact
// `--resume S` operand, the controller verified that one provider session both
// before launch and on the live re-check, and Amendment 002 §7's acknowledged
// prefix is `resume` followed by `adapter-start` continuing the checkpoint.
// assertClaudeDualRegistrationWitness proves the provider completed exactly one
// required initialize handshake against each of the two registrations before any
// useful work, and that no authorization value reached durable or public
// evidence.
func assertClaudeDualRegistrationWitness(t *testing.T, run claudeLifecycleObservation) {
	t.Helper()
	firstUseful := -1
	handshakes := map[string]int{}
	for i, row := range run.tools {
		name, isHandshake := strings.CutPrefix(row, "initialize ")
		if !isHandshake {
			if firstUseful < 0 {
				firstUseful = i
			}
			continue
		}
		if _, duplicate := handshakes[name]; duplicate {
			t.Fatalf("provider repeated the initialize handshake for %q: %v", name, run.tools)
		}
		handshakes[name] = i
	}
	if len(handshakes) != 2 {
		t.Fatalf("provider completed %d initialize handshakes, want exactly two: %v", len(handshakes), run.tools)
	}
	for _, name := range []string{"vatc", "verdi-context"} {
		index, ok := handshakes[name]
		if !ok {
			t.Fatalf("provider never completed a required initialize handshake for %q: %v", name, run.tools)
		}
		if firstUseful >= 0 && index > firstUseful {
			t.Fatalf("initialize for %q followed useful work at row %d: %v", name, firstUseful, run.tools)
		}
	}
	// The ATC-owned parent service independently observed exactly one successful
	// initialize handshake and one claim call on its own capability, and refused
	// the Verdi context capability presented to it.
	accepted, rejected, inits := run.claim.observed()
	if inits != 1 {
		t.Fatalf("claim server observed %d initialize handshakes, want exactly one", inits)
	}
	if !reflect.DeepEqual(accepted, []string{"claim_paths"}) {
		t.Fatalf("claim server tool calls = %v, want exactly [claim_paths]", accepted)
	}
	if rejected != 1 {
		t.Fatalf("claim server rejected %d cross-capability requests, want exactly one", rejected)
	}
	for _, event := range run.fake.events {
		encoded, err := contextevent.EncodeEvent(event)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(encoded, []byte("Bearer ")) {
			t.Fatalf("acknowledged event carried an authorization value: %s", encoded)
		}
	}
	if strings.Contains(run.obs.stdout, "Bearer ") || strings.Contains(run.obs.stderr, "Bearer ") {
		t.Fatalf("public output carried an authorization value: %#v", run.obs)
	}
}

func assertClaudeResumeWitness(t *testing.T, run claudeLifecycleObservation) {
	t.Helper()
	request := run.fixture.request
	if request.Action != sealedexec.ActionResume || request.Resume == nil || request.Start != nil {
		t.Fatalf("built resume row drove action %q (start arm present = %v, resume arm present = %v), want the sealed resume arm",
			request.Action, request.Start != nil, request.Resume != nil)
	}
	continuity := request.Resume.Continuity
	if request.Resume.ContinuityDigest != continuity.Digest {
		t.Fatalf("resume arm continuity digest = %q, want the continuity self-digest %q", request.Resume.ContinuityDigest, continuity.Digest)
	}
	// The continuation names exactly the provider session the fake actually
	// reports, so the binary's resumed-stream identity check has a real subject.
	if continuity.AdapterSessionRef != claudeE2ESession {
		t.Fatalf("resume continuity adapter session ref = %q, want the session the provider reports (%q)",
			continuity.AdapterSessionRef, claudeE2ESession)
	}

	// Exact `--resume S`: the verified session identity is the argv tail, never
	// a bare flag and never an option-shaped substitute.
	if n := len(run.argv); n < 2 || run.argv[n-2] != "--resume" || run.argv[n-1] != continuity.AdapterSessionRef {
		t.Fatalf("claude provider argv = %#v, want the exact `--resume %s` tail", run.argv, continuity.AdapterSessionRef)
	}

	// The controller verified the provider session before launch and again on
	// the live resume re-check; a start arm verifies none.
	if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperationVerifyProviderSession); got < 2 {
		t.Fatalf("verify-provider-session calls = %d, want the pre-launch and live resume re-checks", got)
	}

	// Amendment 002 §7 prepared-resume: the explicit `resume` acknowledgment,
	// then the session-init reduction's `adapter-start`.
	assertClaudeAcknowledgedPrefix(t, run.fake.events, []contextevent.Kind{
		contextevent.KindResume, contextevent.KindAdapterStart,
	})
	// The acknowledged prefix continues the checkpoint instead of restarting at
	// sequence one.
	if run.fake.events[0].SourceSequence != continuity.TerminalSourceSequence+1 {
		t.Fatalf("resume first acknowledged source sequence = %d, want continuation of terminal %d",
			run.fake.events[0].SourceSequence, continuity.TerminalSourceSequence)
	}
}

// assertClaudeAcknowledgedPrefix proves the acknowledged events open with
// exactly these kinds, in order, on contiguous source sequences. Unlike the
// whole-run assertSealedEventPrefix it constrains only the leading kinds,
// because the Claude stream reduction continues past them.
func assertClaudeAcknowledgedPrefix(t *testing.T, events []contextevent.Event, want []contextevent.Kind) {
	t.Helper()
	if len(events) < len(want) {
		t.Fatalf("acknowledged claude kinds = %v, want a leading %v", sealedEventKinds(events), want)
	}
	for i, kind := range want {
		if events[i].Kind != kind {
			t.Fatalf("acknowledged claude kinds = %v, want a leading %v", sealedEventKinds(events), want)
		}
		if i > 0 && events[i].SourceSequence != events[i-1].SourceSequence+1 {
			t.Fatalf("acknowledged claude source sequences = %v, want contiguous", sealedEventSequences(events))
		}
	}
}

// eventDetail returns the acknowledged event's detail, or nil for the kinds
// that carry none. It fails closed: an unrecognized payload has no detail.
// sealedEventDetails renders every acknowledged event's inline detail bytes,
// so a failing assertion can show what the recorded stream actually carried.
func sealedEventDetails(events []contextevent.Event) string {
	var parts []string
	for _, event := range events {
		detail := eventDetail(event)
		if detail == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", event.Kind, detail.RedactedJSON))
	}
	return strings.Join(parts, " ")
}

func eventDetail(event contextevent.Event) *contextevent.Detail {
	switch payload := event.Payload.(type) {
	case *contextevent.ProviderSummaryPayload:
		return &payload.Detail
	case *contextevent.ProviderMessagePayload:
		return &payload.Detail
	case *contextevent.ToolCallPayload:
		return &payload.Detail
	case *contextevent.ToolResultPayload:
		return &payload.Detail
	case *contextevent.AdapterErrorPayload:
		return &payload.Detail
	case *contextevent.PromptPayload:
		return &payload.Detail
	case *contextevent.FlightPlanPayload:
		return &payload.Detail
	case *contextevent.InstructionProjectionPayload:
		return &payload.Detail
	case *contextevent.ReceiptPayload:
		return &payload.Detail
	case *contextevent.AdapterStartPayload:
		return payload.Detail
	}
	return nil
}

// assertClaudeSuccessfulLifecycle proves the public Claude success path reaches
// completion: Amendment 002 §5 holds the exact result until the child is reaped
// and §7 then emits the advisory `provider-summary` and `adapter-stop`, after
// which shared completion appends `execution-result`, mints exactly one
// receipt, and writes the public execution-result bytes. Nothing is
// quarantined and the scoped resources are reclaimed.
func assertClaudeSuccessfulLifecycle(t *testing.T, run claudeLifecycleObservation) {
	t.Helper()
	wantStdout := run.outPath == ""
	if run.obs.exitCode != 0 || run.obs.stderr != "" || (wantStdout && run.obs.stdout == "") || (!wantStdout && run.obs.stdout != "") {
		t.Fatalf("claude success lifecycle = %#v", run.obs)
	}

	// Public execution-result bytes, on the requested public channel only.
	resultBytes := []byte(run.obs.stdout)
	if !wantStdout {
		resultBytes = mustReadFile(t, run.outPath)
	}
	result, err := sealedexec.DecodeExecutionResult(bytes.NewReader(resultBytes))
	if err != nil {
		t.Fatalf("decode claude execution result: %v\n%s", err, resultBytes)
	}
	if result.InputCommit != run.fixture.head || result.OutputCommit == "" || !result.Clean {
		t.Fatalf("claude execution result git identity = %#v", result)
	}

	// Exactly one receipt, no quarantine, and the terminal preserved state.
	if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperationAppendReceipt); got != 1 {
		t.Fatalf("append-receipt calls = %d, want exactly 1", got)
	}
	// A completed run preserves nothing: there is no quarantine record, no
	// preserved-partial bytes, and the workspace is handed back normally.
	if len(run.fake.quarantines) != 0 || len(run.fake.quarantineRecords) != 0 || len(run.fake.preservedBytes) != 0 {
		t.Fatalf("successful claude run quarantined %v / %d records / %d preserved payloads",
			run.fake.quarantines, len(run.fake.quarantineRecords), len(run.fake.preservedBytes))
	}

	// Terminal acknowledged kinds: the reaped success summary, adapter-stop,
	// then the shared execution-result.
	kinds := sealedEventKinds(run.fake.events)
	if len(kinds) < 3 {
		t.Fatalf("acknowledged claude event kinds = %v, want a terminal tail", kinds)
	}
	wantTail := []contextevent.Kind{
		contextevent.KindProviderSummary, contextevent.KindAdapterStop, contextevent.KindExecutionResult,
	}
	if !reflect.DeepEqual(kinds[len(kinds)-3:], wantTail) {
		t.Fatalf("acknowledged claude event tail = %v, want %v", kinds, wantTail)
	}
	if got := countEventKindInFixture(run.fake.events, contextevent.KindExecutionResult); got != 1 {
		t.Fatalf("execution-result acknowledgments = %d, want exactly 1", got)
	}

	// The terminal detail bytes survived redaction and the controller segment
	// store: every acknowledged detail is representable and no reference dangles.
	assertClaudeAcknowledgedDetails(t, run)

	// Cleanup: only the scoped configuration is removed and the workspace is
	// released exactly once.
	if _, err := os.Stat(run.mcpConfig); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scoped MCP config survived the successful run: %v", err)
	}
	if got := countControllerOperation(run.fake.calls, sealedexec.ControllerOperationPersistHandback); got != 1 {
		t.Fatalf("persist-handback calls = %d, want exactly 1", got)
	}
}

// assertClaudeChildRevisionBinding proves the real built row's successful
// `request_context` call actually moved the shared flight state, and that every
// terminal artifact binds the installed child rather than the dispatched request
// revision (I-115–I-117/SI-171–SI-173). It pins: the acknowledged
// `child-manifest` transition and the revision its own event carries; the next
// provider-owned event, which must open the child revision at source order one
// across the exact acknowledged bridge; the `execution-result` event and its
// payload manifest; the receipt event acknowledgment; the public result's
// terminal manifest revision and digest; and the receipt's terminal revision
// segment.
func assertClaudeChildRevisionBinding(t *testing.T, run claudeLifecycleObservation) {
	t.Helper()
	events := run.fake.events

	// Exactly one acknowledged child-manifest names the parent→child install.
	child, childIndex := (*contextevent.ChildManifestPayload)(nil), -1
	for i, event := range events {
		payload, ok := event.Payload.(*contextevent.ChildManifestPayload)
		if !ok {
			continue
		}
		if childIndex >= 0 {
			t.Fatalf("acknowledged %d child-manifest events, want exactly the one installed expansion", 2)
		}
		child, childIndex = payload, i
	}
	if childIndex < 0 {
		t.Fatalf("no acknowledged child-manifest; kinds = %v", sealedEventKinds(events))
	}
	parentRevision := run.fixture.request.ManifestRevision
	if child.ParentRevision != parentRevision || child.ParentManifestDigest != run.fixture.request.ManifestDigest {
		t.Fatalf("child-manifest parent = revision %d digest %q, want the dispatched request %d/%q",
			child.ParentRevision, child.ParentManifestDigest, parentRevision, run.fixture.request.ManifestDigest)
	}
	childRevision := parentRevision + 1
	if child.ChildRevision != childRevision || child.ChildManifestDigest == "" ||
		child.ChildManifestDigest == child.ParentManifestDigest {
		t.Fatalf("child-manifest child = revision %d digest %q, want the successor %d with its own manifest",
			child.ChildRevision, child.ChildManifestDigest, childRevision)
	}
	// The child-manifest event itself closes the parent revision.
	if events[childIndex].ManifestRevision != parentRevision {
		t.Fatalf("child-manifest event carries revision %d, want the closing parent revision %d",
			events[childIndex].ManifestRevision, parentRevision)
	}

	// The next acknowledged event is provider-owned and opens the installed
	// child revision at source order one across the exact bridge.
	if childIndex+1 >= len(events) {
		t.Fatalf("no acknowledged event follows the installed expansion; kinds = %v", sealedEventKinds(events))
	}
	next := events[childIndex+1]
	if next.ManifestRevision != childRevision || next.ManifestDigest != child.ChildManifestDigest {
		t.Fatalf("event after the expansion = revision %d digest %q, want the installed child %d/%q",
			next.ManifestRevision, next.ManifestDigest, childRevision, child.ChildManifestDigest)
	}
	if next.SourceSequence != 1 || next.PriorEventDigest != "" {
		t.Fatalf("event after the expansion = source %d prior %q, want the child revision to open at source 1 with no prior event",
			next.SourceSequence, next.PriorEventDigest)
	}
	if next.PriorRevision == nil || next.PriorRevision.ManifestRevision != parentRevision ||
		next.PriorRevision.EventRoot != events[childIndex].EventDigest ||
		next.PriorRevision.TerminalSourceSequence != events[childIndex].SourceSequence {
		t.Fatalf("event after the expansion bridges %#v, want the exact acknowledged child-manifest terminal", next.PriorRevision)
	}

	// The terminal execution-result is acknowledged on the child revision and
	// its payload binds the installed child manifest.
	terminal := events[len(events)-1]
	resultPayload, ok := terminal.Payload.(*contextevent.ExecutionResultPayload)
	if !ok {
		t.Fatalf("terminal acknowledged event = %s, want execution-result", terminal.Kind)
	}
	if terminal.ManifestRevision != childRevision || terminal.ManifestDigest != child.ChildManifestDigest ||
		resultPayload.ManifestDigest != child.ChildManifestDigest {
		t.Fatalf("execution-result = revision %d event digest %q payload digest %q, want the installed child %d/%q",
			terminal.ManifestRevision, terminal.ManifestDigest, resultPayload.ManifestDigest, childRevision, child.ChildManifestDigest)
	}

	// The public result, its receipt event acknowledgment, and the receipt's
	// terminal revision segment all bind that same child revision.
	result := claudePublicResult(t, run)
	if result.TerminalManifestRevision != childRevision || result.TerminalManifestDigest != child.ChildManifestDigest {
		t.Fatalf("public result terminal manifest = revision %d digest %q, want the installed child %d/%q",
			result.TerminalManifestRevision, result.TerminalManifestDigest, childRevision, child.ChildManifestDigest)
	}
	if result.ReceiptEventAck.ManifestRevision != childRevision {
		t.Fatalf("receipt event acknowledgment carries revision %d, want the installed child %d",
			result.ReceiptEventAck.ManifestRevision, childRevision)
	}
	if result.Receipt.TerminalManifestRevision != childRevision || result.Receipt.ManifestDigest != child.ChildManifestDigest {
		t.Fatalf("receipt terminal = revision %d manifest %q, want the installed child %d/%q",
			result.Receipt.TerminalManifestRevision, result.Receipt.ManifestDigest, childRevision, child.ChildManifestDigest)
	}
	segments := result.Receipt.RevisionSegments
	if len(segments) == 0 {
		t.Fatal("receipt carries no revision segments")
	}
	last := segments[len(segments)-1]
	if last.ManifestRevision != childRevision || last.ManifestDigest != child.ChildManifestDigest {
		t.Fatalf("receipt terminal revision segment = %d/%q, want the installed child %d/%q",
			last.ManifestRevision, last.ManifestDigest, childRevision, child.ChildManifestDigest)
	}
	if len(segments) < 2 || segments[len(segments)-2].ManifestRevision != parentRevision {
		t.Fatalf("receipt revision segments = %#v, want the parent segment preserved before the child", segments)
	}
}

// claudePublicResult decodes the public execution-result bytes from whichever
// channel the row requested.
func claudePublicResult(t *testing.T, run claudeLifecycleObservation) sealedexec.ExecutionResult {
	t.Helper()
	resultBytes := []byte(run.obs.stdout)
	if run.outPath != "" {
		resultBytes = mustReadFile(t, run.outPath)
	}
	result, err := sealedexec.DecodeExecutionResult(bytes.NewReader(resultBytes))
	if err != nil {
		t.Fatalf("decode claude execution result: %v\n%s", err, resultBytes)
	}
	return result
}

// assertClaudeAcknowledgedDetails proves every acknowledged Claude detail is a
// valid inline or controller-segment representation, and that each segment
// reference actually resolves to the exact stored bytes.
func assertClaudeAcknowledgedDetails(t *testing.T, run claudeLifecycleObservation) {
	t.Helper()
	inline, segment := 0, 0
	for _, event := range run.fake.events {
		detail := eventDetail(event)
		if detail == nil {
			continue
		}
		if err := detail.Validate(); err != nil {
			t.Fatalf("acknowledged %s detail is invalid: %v", event.Kind, err)
		}
		switch detail.Mode {
		case contextevent.DetailInline:
			inline++
		case contextevent.DetailSegment:
			segment++
			stored, ok := run.fake.segments[detail.Reference]
			if !ok {
				t.Fatalf("acknowledged %s detail references unstored segment %q", event.Kind, detail.Reference)
			}
			if stored.Digest != detail.Digest || stored.ByteCount != detail.ByteCount {
				t.Fatalf("acknowledged %s segment = digest %s/%d bytes, want %s/%d", event.Kind, stored.Digest, stored.ByteCount, detail.Digest, detail.ByteCount)
			}
		}
	}
	if inline == 0 {
		t.Fatalf("no acknowledged claude detail was inline; kinds = %v", sealedEventKinds(run.fake.events))
	}
}
