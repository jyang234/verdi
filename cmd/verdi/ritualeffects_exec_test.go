package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/constitutionapp"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentrun"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/ritualwitness"
	"github.com/jyang234/verdi/internal/sealedexec"
	sealedclaude "github.com/jyang234/verdi/internal/sealedexec/claude"
	"github.com/jyang234/verdi/internal/specimport"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/workbench"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// The rows of TestRitualEffects_EveryDeclaredRitual whose fixture or
// driver is more than a store and one request: spec import on all three
// surfaces, the MCP tools, context execution (the built binary with its
// controller socket on file descriptor 3 and a compiled fake Claude), and
// the execution rituals (SI-348 (1)).

// ritualEffectsTable is the producer's case table, keyed by registry verb.
func ritualEffectsTable(t *testing.T, bin string) map[ws.Verb][]ritualCase {
	table := map[ws.Verb][]ritualCase{}
	for _, part := range []map[ws.Verb][]ritualCase{
		cliRitualCases(t, bin),
		workbenchRitualCases(),
		specImportCases(t, bin),
		evaluationMCPCases(t),
		contextExecutionCases(bin),
		executionWorkspaceCases(bin),
	} {
		for v, cases := range part {
			table[v] = append(table[v], cases...)
		}
	}
	return table
}

// serveMCP serves the MCP server core `verdi mcp` serves, in process, for
// the store found at root (ledger SI-341 (5)).
func serveMCP(ctx context.Context, root string, r io.Reader, w io.Writer) error {
	storeRoot, err := store.FindRoot(root)
	if err != nil {
		return err
	}
	return mcpserve.ServeConn(ctx, r, w, mcpserve.NewServer(storeRoot))
}

// mcpTool is a case driver calling tool once with args.
func mcpTool(tool string, args func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture) json.RawMessage) func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver {
	return func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture) ritualwitness.Driver {
		return ritualwitness.MCP{Serve: serveMCP, Tool: tool, Arguments: args(t, ctx, fx)}
	}
}

// specImportDirty is spec import's own dirty-context refusal in state,
// which every surface carries, preview and apply alike (ledger SI-349
// (2)), each behind its own prefix: three operator paths in SeedFull, two
// in SeedClean, whose index holds no foreign entry.
func specImportDirty(state ritualwitness.SeedState) string {
	if state == ritualwitness.SeedFull {
		return "checkout has 3 uncommitted or untracked path(s)"
	}
	return "checkout has 2 uncommitted or untracked path(s)"
}

// The surfaces' prefixes to specImportDirty, for preview and apply.
const (
	specImportCLIPreview  = "design import preview: dirty-context: "
	specImportCLIApply    = "design import apply: dirty-context: "
	specImportWorkbench   = `{"code":"dirty-context","error":"`
	specImportMCPPreview  = "import_preview: dirty-context: specimport: dirty context: "
	specImportMCPApply    = "import_apply: dirty-context: specimport: dirty context: "
	specImportDirtyAdvice = "commit or remove them before preview/apply"
)

// specImportCases are spec_import's three surfaces, a whole-tree guard
// (ledger SI-349 (2)): import refuses a checkout with any uncommitted or
// untracked path, preview and apply alike. In both seeded states each
// surface's own preview refuses (asserted while the case is prepared), and
// so does its apply, which names a digest no preview made, with the
// dirty-context words and nothing remaining. The CLI refusal exits 1 and
// the workbench and MCP refusals exit 2 (backlog BL-156); those two, under
// the scoped declaration, carry SI-348 (2)'s named allowance. Over a
// pristine tree each surface previews and then applies a ready request
// under a draft-write design-assistance policy, and completes.
func specImportCases(t *testing.T, bin string) map[ws.Verb][]ritualCase {
	policyStore := designImportPolicyFiles(t, "draft-write")
	// preview runs one surface's preview of request in fx and returns its
	// result, or, when the tree is seeded, asserts its dirty-context
	// refusal and returns nil.
	type previewer func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture, path string, request []byte, pristine bool) []byte
	cliPreview := func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture, path string, _ []byte, pristine bool) []byte {
		if pristine {
			return []byte(binaryOutput(t, ctx, bin, fx.Dir, 0, "design", "import", "preview", "--request", path))
		}
		cmd := exec.CommandContext(ctx, bin, "design", "import", "preview", "--request", path)
		cmd.Dir = fx.Dir
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), specImportCLIPreview+specImportDirty(fx.State)) {
			t.Fatalf("the CLI's import preview over the seeded tree = %v %s, want its dirty-context refusal", err, out)
		}
		return nil
	}
	workbenchPreview := func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture, _ string, request []byte, pristine bool) []byte {
		rec := httptest.NewRecorder()
		workbench.NewHandler(fx.Dir).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/design/import/preview", bytes.NewReader(request)))
		if !pristine {
			if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), specImportWorkbench+specImportDirty(fx.State)) {
				t.Fatalf("the workbench's import preview over the seeded tree = %d %s, want its dirty-context refusal", rec.Code, rec.Body.String())
			}
			return nil
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("the workbench's import preview = %d %s", rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	mcpPreview := func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture, _ string, request []byte, pristine bool) []byte {
		storeRoot, err := store.FindRoot(fx.Dir)
		if err != nil {
			t.Fatal(err)
		}
		result := mcpserve.NewServer(storeRoot).Backend.ImportPreview(ctx, encodedRequest(t, map[string]any{"request": json.RawMessage(request)}))
		if !pristine {
			if result["isError"] != true || !strings.Contains(fmt.Sprint(result["content"]), specImportMCPPreview+specImportDirty(fx.State)) {
				t.Fatalf("the MCP import preview over the seeded tree = %+v, want its dirty-context refusal", result)
			}
			return nil
		}
		return []byte(constitutionToolText(t, result))
	}
	// prepared writes the request and previews it through the same surface
	// that applies it; over a seeded tree the preview refuses, and apply
	// names a digest no preview made.
	prepared := func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture, slug string, pristine bool, preview previewer) (path string, request []byte, digest string) {
		request = designImportRequestJSON(t, designImportReadyRequest(t, slug))
		path = filepath.Join(t.TempDir(), "import-request.json")
		if err := os.WriteFile(path, request, 0o644); err != nil {
			t.Fatal(err)
		}
		body := preview(t, ctx, fx, path, request, pristine)
		if !pristine {
			return path, request, strings.Repeat("0", 64)
		}
		var result specimport.PreviewResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decoding the preview: %v", err)
		}
		if !result.Ready {
			t.Fatalf("the import preview is not ready: %+v", result.Findings)
		}
		return path, request, result.Digest
	}
	guard := func(exit int, prefix string, scoped bool) func(ritualwitness.SeedState) ritualRun {
		return func(state ritualwitness.SeedState) ritualRun {
			r := refuses(exit, prefix+specImportDirty(state), specImportDirtyAdvice)
			if scoped {
				r = allowingScopedRefusal(r)
			}
			return r
		}
	}
	surfaces := []struct {
		verb   ws.Verb
		guard  func(ritualwitness.SeedState) ritualRun
		driver func(pristine bool) func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver
	}{
		{ws.CLI("design import apply"), guard(1, specImportCLIApply, false),
			func(pristine bool) func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver {
				return func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture) ritualwitness.Driver {
					path, _, digest := prepared(t, ctx, fx, "r3c-import-cli", pristine, cliPreview)
					return ritualwitness.Binary{Path: bin, Args: []string{"design", "import", "apply", "--request", path, "--preview", digest, "--harness", "codex"}}
				}
			}},
		{ws.Workbench("/design/import/apply"), guard(2, "answered 409: "+specImportWorkbench, true),
			func(pristine bool) func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver {
				return func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture) ritualwitness.Driver {
					_, request, digest := prepared(t, ctx, fx, "r3c-import-workbench", pristine, workbenchPreview)
					return ritualwitness.Workbench{Serve: workbench.NewHandler, Method: http.MethodPost, Path: "/design/import/apply",
						Body: request, Header: http.Header{"X-Verdi-Import-Preview": {digest}}}
				}
			}},
		{ws.MCP("import_apply"), guard(2, specImportMCPApply, true),
			func(pristine bool) func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver {
				return mcpTool("import_apply", func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture) json.RawMessage {
					_, request, digest := prepared(t, ctx, fx, "r3c-import-mcp", pristine, mcpPreview)
					return encodedRequest(t, map[string]any{"harness": "codex", "preview_digest": digest, "request": json.RawMessage(request)})
				})
			}},
	}
	table := map[ws.Verb][]ritualCase{}
	for _, s := range surfaces {
		table[s.verb] = []ritualCase{
			{path: pathRoot, ritual: "spec_import", base: policyStore, driver: s.driver(false), want: s.guard},
			{path: pathRoot + pristineSuffix, ritual: "spec_import", base: policyStore, pristine: true, driver: s.driver(true), want: always(completes())},
		}
	}
	return table
}

// evaluationMCPCases is constitution_evaluation's MCP tool.
func evaluationMCPCases(t *testing.T) map[ws.Verb][]ritualCase {
	return map[ws.Verb][]ritualCase{
		ws.MCP("constitution_impact_review"): {{path: pathRoot, ritual: "constitution_evaluation", base: constitutionStoreFiles(t), want: always(completes()),
			driver: mcpTool("constitution_impact_review", func(t *testing.T, _ context.Context, _ *ritualwitness.Fixture) json.RawMessage {
				return encodedRequest(t, map[string]any{"schema": constitutionapp.ImpactReviewRequestSchema})
			})}},
	}
}

// seedOperatorWork lays state's operator work over a fixture this file
// builds itself, exactly as ritualwitness's seeded states do: a dirty
// tracked file and an untracked file in every state, and the staged
// foreign entry in SeedFull.
func seedOperatorWork(t *testing.T, dir string, state ritualwitness.SeedState) {
	t.Helper()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(ritualwitness.TrackedFile, "original content\nedited by the operator\n")
	write(ritualwitness.UntrackedFile, "the operator's new, never-added file\n")
	if state == ritualwitness.SeedFull {
		write(ritualwitness.ForeignFile, "a colleague's unrelated staged work\n")
		gitTestOutput(t, dir, "add", "--", ritualwitness.ForeignFile)
	}
}

// emptyBare is a fresh bare repository, the remote a fixture without one
// is sensed against.
func emptyBare(t *testing.T) string {
	t.Helper()
	bare := t.TempDir()
	gitTestOutput(t, bare, "init", "-q", "--bare", "--initial-branch=main")
	return bare
}

// contextExecutionCases are context execution's two cases (SI-341 (2),
// SI-348 (5)): in both seeded states the operator's work changes the
// runway the request was compiled from, so the binary's compile gate
// refuses (exit 1) before any workspace or hand-back; over a pristine
// runway the sealed run completes and hands back.
func contextExecutionCases(bin string) map[ws.Verb][]ritualCase {
	// runs holds each run's compiled fixture, by fixture directory, from its
	// fixture to its driver.
	var runs sync.Map
	fixture := func(t *testing.T, ctx context.Context, state ritualwitness.SeedState) *ritualwitness.Fixture {
		providerRoot := t.TempDir()
		claudePath := filepath.Join(providerRoot, "claude")
		compiled := buildClaudeCompiledFixtureWith(ctx, t, execworkspace.GrantSet{Grants: []execworkspace.Grant{
			{Kind: execworkspace.GrantNetwork},
			{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{claudePath}},
		}}, map[string]string{ritualwitness.TrackedFile: "original content\n"})
		seedOperatorWork(t, compiled.root, state)
		runs.Store(compiled.root, contextExecutionRun{compiled: compiled, providerRoot: providerRoot, claudePath: claudePath})
		return &ritualwitness.Fixture{Dir: compiled.root, Bare: emptyBare(t), BaseCommit: compiled.head, State: state}
	}
	driver := func(t *testing.T, ctx context.Context, fx *ritualwitness.Fixture) ritualwitness.Driver {
		v, ok := runs.Load(fx.Dir)
		if !ok {
			t.Fatalf("no compiled context-execution fixture for %s", fx.Dir)
		}
		return v.(contextExecutionRun).driver(ctx, t, bin)
	}
	return map[ws.Verb][]ritualCase{
		ws.CLI("context execution"): {
			{path: "compile-gate", ritual: "context_execution", fixture: fixture, driver: driver,
				want: always(refuses(1, "context execution: sealedexec: verdict failure: compiled context manifest does not match the public request"))},
			{path: "hand-back" + pristineSuffix, ritual: "context_execution", fixture: fixture, pristine: true, driver: driver, want: always(completes())},
		},
	}
}

// contextExecutionRun is one context-execution run's fixture: the
// compiled runway, and where its fake Claude is built.
type contextExecutionRun struct {
	compiled     claudeCompiledFixture
	providerRoot string
	claudePath   string
}

// driver builds the fake Claude and the strict controller, hands the
// controller's socket to the built binary as file descriptor 3, and
// returns the Binary driver for `context execution`. The test's cleanup
// closes the binary's end and waits, bounded, for the controller.
func (r contextExecutionRun) driver(ctx context.Context, t *testing.T, bin string) ritualwitness.Driver {
	t.Helper()
	envRoot := filepath.Join(r.providerRoot, "env")
	claudeConfigDir := filepath.Join(envRoot, "claude-config")
	if err := os.MkdirAll(claudeConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if gitPath, err = filepath.Abs(gitPath); err != nil {
		t.Fatal(err)
	}
	workspaceID, err := r.compiled.request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	if built := buildFakeClaude(t, r.providerRoot, fakeClaudeSpec{
		version: r.compiled.request.AdapterVersion, model: claudeE2EModel, session: claudeE2ESession,
		argvPath: filepath.Join(r.providerRoot, "argv"), envPath: filepath.Join(r.providerRoot, "env.txt"),
		stdinPath: filepath.Join(r.providerRoot, "stdin"), toolsPath: filepath.Join(r.providerRoot, "tools"),
		gitPath: gitPath, workspace: execworkspace.UnitPath(r.compiled.root, workspaceID), commit: true,
	}); built != r.claudePath {
		t.Fatalf("fake claude built at %q, want the granted argv0 %q", built, r.claudePath)
	}
	controller := &sealedLifecycleController{
		t: t, request: r.compiled.request, allowQuarantine: true,
		profile: sealedexec.ProfileMaterial{
			Ref: r.compiled.request.Profile, Name: "claude-fixture", AbsoluteExecutable: r.claudePath,
			AbsoluteEnvRoot: envRoot, Model: claudeE2EModel, ClaudeConfigDir: claudeConfigDir,
			AdapterVersion: r.compiled.request.AdapterVersion, DecoderProfile: sealedclaude.DecoderProfileV1,
		},
		resolution: sealedexec.ContextResolution{
			Verification: sealedexec.Verification{State: contextcompile.ResolutionUnproven, Failure: sealedexec.FailureUnproven, Witnesses: []string{"fixture context unavailable"}},
		},
	}
	controller.claimMCPURL = startFakeClaimMCP(ctx, t, r.compiled.requestBytes).url

	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	controllerFile := os.NewFile(uintptr(fds[0]), "ritual-effects-controller")
	childFile := os.NewFile(uintptr(fds[1]), "ritual-effects-child")
	conn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() {
		defer conn.Close()
		served <- controller.serve(conn)
	}()
	t.Cleanup(func() {
		_ = childFile.Close()
		select {
		case err := <-served:
			if err != nil {
				t.Errorf("the context-execution controller: %v", err)
			}
		case <-time.After(30 * time.Second):
			t.Error("the context-execution controller did not end within 30s of the run")
			_ = conn.Close()
		}
	})
	return ritualwitness.Binary{Path: bin, Args: []string{"context", "execution", "--request", "-"},
		Stdin: bytes.NewReader(r.compiled.requestBytes), ExtraFiles: []*os.File{childFile},
		Env: []string{"ANTHROPIC_API_KEY=" + claudeE2EAPIKey}}
}

// experimentBindingRefusal is the execution rituals' refusal (ledger
// SI-349 (3)): the earliest deterministic refusal that precedes every
// effect and is the same on every platform. The verb checks the run's input
// bindings against the accepted definition's locked digests before it
// resolves policy or reaches the runner, whose isolation profile is where
// the platforms part (SI-131 (c), SI-136 (c)); a binding whose workload
// digest is not the definition's is refused there, operationally (exit
// 2), with nothing written. start, resume, and the MCP tool share it.
func experimentBindingRefusal() ritualRun {
	return refuses(2, `"classification":"operational","code":"input-binding-invalid"`, "experimentapp: execution input bindings:")
}

// experimentFixture builds the registered, accepted experiment the
// execution rituals run (buildWave5CAcceptedResult's registration, with
// its inputs bound), a local bare remote, and then state's operator work.
func experimentFixture(bin string) func(*testing.T, context.Context, ritualwitness.SeedState) *ritualwitness.Fixture {
	return func(t *testing.T, ctx context.Context, state ritualwitness.SeedState) *ritualwitness.Fixture {
		privateKey := ed25519.NewKeyFromSeed(wave5CFixtureEd25519Seed[:])
		repo := buildExperimentHumanRepo(t, privateKey.Public().(ed25519.PublicKey))
		pinFixtureDefaultBranch(t, repo.Dir)
		wave5CBindProtectedInputs(t, repo)
		runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "-b", "registration")
		args := []string{"experiment", "propose-registration", "--spike", "spec/request-path-spike", "--experiment", "request-path-v2", "--accepted-head", repo.Head, "--json"}
		stdout, stderr, code := runExperimentBuiltBinary(t, bin, repo.Dir, nil, args...)
		if code != 1 || stderr != "" {
			t.Fatalf("registration challenge exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
		}
		challenge, err := decodeExperimentChallengeOutput(t, stdout).Challenge.Canonical()
		if err != nil {
			t.Fatal(err)
		}
		proof := filepath.Join(t.TempDir(), "registration.sig")
		if err := os.WriteFile(proof, ed25519.Sign(privateKey, challenge), 0o600); err != nil {
			t.Fatal(err)
		}
		if stdout, stderr, code = runExperimentBuiltBinary(t, bin, repo.Dir, nil, append(args, "--human-proof", proof)...); code != 0 || stderr != "" {
			t.Fatalf("registration proposal exit/stdout/stderr = %d/%q/%q", code, stdout, stderr)
		}
		runGitForExperimentTest(t, repo.Dir, "add", ".")
		runGitForExperimentTest(t, repo.Dir, "commit", "-q", "-m", "propose registration")
		runGitForExperimentTest(t, repo.Dir, "checkout", "-q", "main")
		runGitForExperimentTest(t, repo.Dir, "merge", "-q", "--ff-only", "registration")
		if err := os.WriteFile(filepath.Join(repo.Dir, ritualwitness.TrackedFile), []byte("original content\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitTestOutput(t, repo.Dir, "add", "--", ritualwitness.TrackedFile)
		gitCommandOK(t, ctx, repo.Dir, "commit", "-q", "-m", "a tracked file", "--", ritualwitness.TrackedFile)
		bare := emptyBare(t)
		gitTestOutput(t, repo.Dir, "remote", "add", "origin", bare)
		gitTestOutput(t, repo.Dir, "push", "-q", "origin", "main")
		gitTestOutput(t, repo.Dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
		head := strings.TrimSpace(gitTestOutput(t, repo.Dir, "rev-parse", "HEAD"))
		seedOperatorWork(t, repo.Dir, state)
		return &ritualwitness.Fixture{Dir: repo.Dir, Bare: bare, BaseCommit: head, State: state}
	}
}

// gitCommandOK runs gitCommand in dir, failing the test on error.
func gitCommandOK(t *testing.T, ctx context.Context, dir string, args ...string) {
	t.Helper()
	if out, err := gitCommand(ctx, dir, args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// experimentBindings writes input bindings for the registered definition
// whose workload digest is not the definition's (the accepted workload's
// digest with its hex digits replaced), outside the fixture, and returns
// their path and bytes: the input the execution rituals refuse before any
// effect (SI-349 (3)).
func experimentBindings(t *testing.T, root string) (string, []byte) {
	t.Helper()
	definition, err := experiment.DecodeDefinition(mustReadWave5CFile(t, wave5CDefinitionPath(root)))
	if err != nil {
		t.Fatal(err)
	}
	mismatched := "sha256:" + strings.Repeat("7", 64)
	if definition.Workload.Digest == mismatched {
		t.Fatalf("the definition's workload digest is the mismatched one %s", mismatched)
	}
	doc, err := experimentrun.EncodeInputBindings(experimentrun.InputBindings{Schema: experimentrun.InputBindingSchema, Inputs: []experimentrun.InputBinding{
		{Slot: experimentrun.InputSlotContract, ID: definition.Contract.ID, Digest: definition.Contract.Digest, Path: wave5CContractPath},
		{Slot: experimentrun.InputSlotWorkload, ID: definition.Workload.ID, Digest: mismatched, Path: wave5CWorkloadPath},
	}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bindings.json")
	if err := os.WriteFile(path, doc, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, doc
}

// executionWorkspaceDisclosure is what the execution rituals' rows do not
// prove (SI-348 (1)).
const executionWorkspaceDisclosure = "the execution rituals' completion is unproven (ledger SI-348 (1), SI-349 (3), backlog BL-155): this row asserts their earliest refusal common to every platform, a mismatched input binding, with nothing remaining, and never reads as completed"

// executionWorkspaceCases are execution_workspace's three verbs, each
// asserting experimentBindingRefusal (SI-348 (1), SI-349 (3)).
func executionWorkspaceCases(bin string) map[ws.Verb][]ritualCase {
	fixture := experimentFixture(bin)
	cliRun := func(operation string) func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver {
		return func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) ritualwitness.Driver {
			path, _ := experimentBindings(t, fx.Dir)
			return ritualwitness.Binary{Path: bin, Args: []string{"experiment", operation, "--spike", "spec/request-path-spike", "--experiment", "request-path-v2",
				"--accepted-head", fx.BaseCommit, "--run", "run-r3c", "--inputs", path, "--json"}}
		}
	}
	row := func(driver func(*testing.T, context.Context, *ritualwitness.Fixture) ritualwitness.Driver, want ritualRun) ritualCase {
		return ritualCase{path: "input-binding-refusal", ritual: "execution_workspace", fixture: fixture, driver: driver, want: always(want), disclosure: executionWorkspaceDisclosure}
	}
	return map[ws.Verb][]ritualCase{
		ws.CLI("experiment start"):  {row(cliRun("start"), experimentBindingRefusal())},
		ws.CLI("experiment resume"): {row(cliRun("resume"), experimentBindingRefusal())},
		ws.MCP("experiment"): {row(mcpTool("experiment", func(t *testing.T, _ context.Context, fx *ritualwitness.Fixture) json.RawMessage {
			_, doc := experimentBindings(t, fx.Dir)
			return encodedRequest(t, map[string]any{"operation": "start", "spike": "spec/request-path-spike", "experiment": "request-path-v2",
				"accepted_head": fx.BaseCommit, "run": "run-r3c", "inputs": json.RawMessage(bytes.TrimSuffix(doc, []byte("\n")))})
		}), experimentBindingRefusal())},
	}
}
