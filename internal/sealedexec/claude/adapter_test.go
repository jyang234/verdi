package claude

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/sealedexec"
)

// ---------------------------------------------------------------------------
// Frozen producer: TestClaudeAdapterParityContract_Static
// Owns argv/env/version/stdin/session/fallback matrix.
// ---------------------------------------------------------------------------

func TestClaudeAdapterParityContract_Static(t *testing.T) {
	t.Run("amendment 003 writes both required rows into the scoped configuration", func(t *testing.T) {
		envRoot := t.TempDir()
		listener := listenScopedMCP(t)
		config, _, closeMCP, err := StartScopedMCP(context.Background(), listener, envRoot, testCanonicalRequest, testProfileDigest, testWorkspaceID, claudeTestClaimMCP(), &scopedHTTPTestHandler{})
		if err != nil {
			t.Fatalf("StartScopedMCP: %v", err)
		}
		registerScopedMCPCleanup(t, closeMCP)
		configBytes, err := os.ReadFile(config.Path)
		if err != nil {
			t.Fatalf("read scoped MCP config: %v", err)
		}
		for _, want := range []string{`"vatc":{`, `"verdi-context":{`} {
			if !strings.Contains(string(configBytes), want) {
				t.Fatalf("scoped MCP config %s lacks required row %s", configBytes, want)
			}
		}
	})

	t.Run("claude_fixtures_record_the_committed_dual_inventory", func(t *testing.T) {
		// Amendment 003 fixes the accepted init inventory as a *provider*
		// observation, so the committed captures must record it themselves. The
		// loader is proven to add nothing but the deterministic workspace binding:
		// reversing that binding reproduces the committed bytes exactly. No
		// fixture-driven assertion can therefore be satisfied by a synthesized
		// inventory, and a capture that regressed to one row would fail here.
		type mcpRow struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
		want := []mcpRow{{Name: "vatc", Status: "connected"}, {Name: "verdi-context", Status: "connected"}}
		workspace := t.TempDir()
		for _, name := range claudeFixtureNames() {
			t.Run(name, func(t *testing.T) {
				onDisk, err := os.ReadFile(filepath.Join("testdata", name))
				if err != nil {
					t.Fatalf("read committed capture: %v", err)
				}
				if !bytes.Contains(onDisk, []byte(claudeFixtureInventory)) {
					t.Fatalf("committed capture %s does not record %s", name, claudeFixtureInventory)
				}
				var recorded struct {
					MCPServers []mcpRow `json:"mcp_servers"`
				}
				if err := json.Unmarshal(bytes.SplitN(onDisk, []byte{'\n'}, 2)[0], &recorded); err != nil {
					t.Fatalf("decode committed init row: %v", err)
				}
				if !reflect.DeepEqual(recorded.MCPServers, want) {
					t.Fatalf("committed init inventory = %v, want %v", recorded.MCPServers, want)
				}
				loaded := mustClaudeFixture(t, name, workspace)
				if bytes.Contains(loaded, []byte(`"cwd":"/workspace"`)) || !bytes.Contains(loaded, []byte(`"cwd":"`+workspace+`"`)) {
					t.Fatalf("loader left the workspace placeholder unbound in %s", name)
				}
				restored := bytes.ReplaceAll(loaded, []byte(`"cwd":"`+workspace+`"`), []byte(`"cwd":"/workspace"`))
				if !bytes.Equal(restored, onDisk) {
					t.Fatalf("loader synthesized bytes beyond the workspace binding in %s:\n%s\nwant\n%s", name, restored, onDisk)
				}
			})
		}
	})

	t.Run("decoder_profile_literal", func(t *testing.T) {
		if DecoderProfileV1 != "claude-stream-json-v1" {
			t.Fatalf("DecoderProfileV1 = %q, want %q", DecoderProfileV1, "claude-stream-json-v1")
		}
	})

	t.Run("nil_process_rejected", func(t *testing.T) {
		dp := newTestProcessor(t)
		_, err := New(nil, dp, claudeTestMCPConfig(t.TempDir()))
		if err == nil {
			t.Fatal("New(nil process) should return error")
		}
	})

	t.Run("nil_processor_rejected", func(t *testing.T) {
		_, err := New(&testProbeProcess{}, nil, claudeTestMCPConfig(t.TempDir()))
		if err == nil {
			t.Fatal("New(nil processor) should return error")
		}
	})

	t.Run("verify_adapter_identity_checks", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, &testProbeProcess{version: launch.Request.AdapterVersion}, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.VerifyAdapter(context.Background(), sealedexec.AdapterCheck{
			Request:   launch.Request,
			Profile:   launch.Profile,
			Workspace: launch.Workspace,
		})
		if err != nil {
			t.Fatalf("VerifyAdapter valid: %v", err)
		}
		// Wrong adapter type
		bad := launch
		bad.Request.Adapter = contextevent.AdapterCodex
		_, err = adapter.VerifyAdapter(context.Background(), sealedexec.AdapterCheck{
			Request:   bad.Request,
			Profile:   bad.Profile,
			Workspace: bad.Workspace,
		})
		if err == nil {
			t.Fatal("VerifyAdapter with codex adapter should return error")
		}
	})

	t.Run("version_probe_argv", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if pp.probeCmd == nil {
			t.Fatal("Probe was not called before Start")
		}
		wantProbeArgs := []string{launch.Profile.Executable, "--version"}
		if !reflect.DeepEqual(pp.probeCmd.Args, wantProbeArgs) {
			t.Fatalf("probe argv = %v, want %v", pp.probeCmd.Args, wantProbeArgs)
		}
		if !reflect.DeepEqual(pp.probeCmd.Env, launch.Profile.Profile.Env()) {
			t.Fatalf("probe env not isolated profile env")
		}
	})

	t.Run("start_exact_argv", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if pp.startCmd == nil {
			t.Fatal("Start never called process.Start")
		}
		mcpConfigPath := filepath.Join(envRoot, "claude-mcp.json")
		wantArgs := []string{
			launch.Profile.Executable,
			"--bare", "-p",
			"--input-format", "stream-json",
			"--output-format", "stream-json",
			"--verbose",
			"--model", launch.Profile.Model,
			"--permission-mode", "bypassPermissions",
			"--strict-mcp-config",
			"--mcp-config", mcpConfigPath,
			"--no-chrome",
		}
		if !reflect.DeepEqual(pp.startCmd.Args, wantArgs) {
			t.Fatalf("start argv = %v, want %v", pp.startCmd.Args, wantArgs)
		}
		if pp.startCmd.Dir != launch.Workspace.Path {
			t.Fatalf("start dir = %q, want %q", pp.startCmd.Dir, launch.Workspace.Path)
		}
		for _, forbidden := range []string{"--continue", "-c", "--last", "--fork-session", "--fallback-model",
			"--include-partial-messages", "--replay-user-messages", "--plugin-dir", "--add-dir",
			"--settings", "--setting-sources", "--ephemeral", "--resume"} {
			if containsStr(pp.startCmd.Args, forbidden) {
				t.Fatalf("start argv contains forbidden flag %q: %v", forbidden, pp.startCmd.Args)
			}
		}
	})

	t.Run("resume_exact_argv", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionResume)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		session := "claude-sess-start-001"
		_, err = adapter.Resume(context.Background(), launch, session)
		if err != nil {
			t.Fatalf("Resume: %v", err)
		}
		if pp.startCmd == nil {
			t.Fatal("Resume never called process.Start")
		}
		mcpConfigPath := filepath.Join(envRoot, "claude-mcp.json")
		wantArgs := []string{
			launch.Profile.Executable,
			"--bare", "-p",
			"--input-format", "stream-json",
			"--output-format", "stream-json",
			"--verbose",
			"--model", launch.Profile.Model,
			"--permission-mode", "bypassPermissions",
			"--strict-mcp-config",
			"--mcp-config", mcpConfigPath,
			"--no-chrome",
			"--resume", session,
		}
		if !reflect.DeepEqual(pp.startCmd.Args, wantArgs) {
			t.Fatalf("resume argv = %v, want %v", pp.startCmd.Args, wantArgs)
		}
	})

	t.Run("stdin_exact_format", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		// Amendment 002 §4 fixes the whole line. The oracle is assembled from
		// hand-written canonical literals and a hand-written string escaper,
		// never from canonjson or the shared provider-input encoder.
		wantStdin := `{"message":{"content":[{"text":"` +
			claudeQuoteJSONString(claudeSealedInputMarker+claudeExpectedProviderInput) +
			`","type":"text"}],"role":"user"},"type":"user"}` + "\n"
		if string(pp.startStdin) != wantStdin {
			t.Fatalf("stdin bytes =\n%s\nwant\n%s", pp.startStdin, wantStdin)
		}
		if bytes.Count(pp.startStdin, []byte("\n")) != 1 || pp.startStdin[len(pp.startStdin)-1] != '\n' {
			t.Fatalf("stdin is not exactly one LF-terminated line: %q", pp.startStdin)
		}
	})

	t.Run("api_key_not_in_policy_secrets_refuses", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		launch.Profile.PolicySecretValues = [][]byte{[]byte("other-secret-not-api-key")}
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Start(context.Background(), launch)
		if err == nil || pp.startCmd != nil {
			t.Fatalf("Start with unclassified API key should refuse before process start")
		}
	})

	t.Run("api_key_too_short_refuses", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		shortKey := "1234567" // 7 bytes — below minimum 8
		claudeConfigDir := envValueStr(launch.Profile.Profile.Env(), "CLAUDE_CONFIG_DIR")
		grants := launch.Profile.Grants
		executable := launch.Profile.Executable
		declaredEnv := map[string]string{
			"ANTHROPIC_API_KEY":                        shortKey,
			"CLAUDE_CONFIG_DIR":                        claudeConfigDir,
			"DISABLE_AUTOUPDATER":                      "1",
			"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
			"CLAUDE_CODE_AUTO_CONNECT_IDE":             "false",
		}
		newProfile, newReport, buildErr := execworkspace.BuildProfile(launch.Workspace.Path, envRoot, grants, declaredEnv)
		if buildErr != nil {
			t.Fatalf("BuildProfile: %v", buildErr)
		}
		launch.Profile.Profile = newProfile
		launch.Profile.Enforcement = *newReport
		launch.Profile.PolicySecretValues = [][]byte{[]byte(shortKey)}
		launch.Profile.Executable = executable
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Start(context.Background(), launch)
		if err == nil || pp.startCmd != nil {
			t.Fatalf("Start with API key < 8 bytes should refuse before process start")
		}
	})

	t.Run("classification_incomplete_refuses", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		launch.Profile.ClassificationComplete = false
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Start(context.Background(), launch)
		if err == nil || pp.startCmd != nil {
			t.Fatalf("Start with incomplete classification should refuse before process start")
		}
	})

	t.Run("review_resume_refused", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionResume)
		launch.Review = &sealedexec.ReviewLaunch{Round: "r0", PacketDigest: claudeTestDigest([]byte("packet")), Model: "some-model"}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, &testProbeProcess{}, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Resume(context.Background(), launch, "claude-sess-start-001")
		if err == nil {
			t.Fatal("Resume with review should return error")
		}
	})

	t.Run("nil_context_refused", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, &testProbeProcess{}, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		//nolint:staticcheck
		if _, err := adapter.Start(nil, launch); err == nil { //nolint:contextcheck
			t.Fatal("Start(nil ctx) should return error")
		}
	})

	t.Run("version_mismatch_refuses", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: "WRONG-VERSION"}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Start(context.Background(), launch)
		if err == nil || pp.startCmd != nil {
			t.Fatalf("Start with version mismatch should refuse before process start")
		}
	})

	// SI-181: Amendment 002 §3 (as annotated 2026-09-06) accepts the probe
	// line iff it equals the requested adapter version exactly, or equals
	// that version plus exactly the one fixed suffix " (Claude Code)" (the
	// real Claude Code CLI 2.1.261 prints "2.1.261 (Claude Code)"). Every
	// other variant is refused.
	t.Run("version_probe_accepts_bare_or_claude_code_suffixed_form", func(t *testing.T) {
		probeLaunch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		version := probeLaunch.Request.AdapterVersion
		const differentVersion = "9.9.9-not-the-requested-version"

		cases := []struct {
			name           string
			probeLine      string
			emptyProbeLine bool
			secondLine     bool
			accept         bool
		}{
			{name: "bare version accepted", probeLine: version, accept: true},
			{name: "version plus Claude Code suffix accepted", probeLine: version + " (Claude Code)", accept: true},
			{name: "lowercase suffix refused", probeLine: version + " (claude code)"},
			{name: "double space before suffix refused", probeLine: version + "  (Claude Code)"},
			{name: "suffix prefixed instead of appended refused", probeLine: "(Claude Code)" + version},
			{name: "trailing content after suffix refused", probeLine: version + " (Claude Code) extra"},
			{name: "different version with suffix refused", probeLine: differentVersion + " (Claude Code)"},
			{name: "empty probe line refused", emptyProbeLine: true},
			// SI-181 review F1: the suffix alone, with no version at all, is
			// neither accepted form and must refuse like any other variant.
			{name: "suffix without a version refused", probeLine: " (Claude Code)"},
			// SI-181 review F1: an otherwise-accepted suffixed line followed by
			// a second stdout line is still more than the one required line.
			{name: "accepted suffixed line plus a second line refused", probeLine: version + " (Claude Code)", secondLine: true},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
				pp := &testProbeProcess{version: tc.probeLine, emptyProbeLine: tc.emptyProbeLine, secondLine: tc.secondLine}
				dp := newTestProcessor(t)
				adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
				if err != nil {
					t.Fatalf("New: %v", err)
				}
				_, err = adapter.Start(context.Background(), launch)
				if tc.accept {
					if err != nil {
						t.Fatalf("Start with probe line %q should be accepted, got error: %v", tc.probeLine, err)
					}
					if pp.startCmd == nil {
						t.Fatal("Start with an accepted probe line should have launched the process")
					}
				} else {
					if err == nil {
						t.Fatalf("Start with probe line %q should refuse", tc.probeLine)
					}
					if pp.startCmd != nil {
						t.Fatalf("Start with probe line %q should refuse before process start", tc.probeLine)
					}
				}
			})
		}
	})
}

// ---------------------------------------------------------------------------
// Frozen producer: TestClaudeAdapterParityContract_Behavioral
// Owns every Amendment 002 §5 family, ordering, id/digest/detail projection,
// start/resume/interrupt/advisory/result/receipt parity.
// ---------------------------------------------------------------------------

func TestClaudeAdapterParityContract_Behavioral(t *testing.T) {
	t.Run("amendment 003 admits exactly the two connected required rows in either order", func(t *testing.T) {
		row := func(name, status string) claudeMCPRow {
			return claudeMCPRow{Name: &name, Status: &status}
		}
		vatc := row("vatc", "connected")
		verdi := row("verdi-context", "connected")
		for _, accepted := range [][]claudeMCPRow{{vatc, verdi}, {verdi, vatc}} {
			if reason := validateInitMCPServers(accepted); reason != "" {
				t.Fatalf("dual connected inventory %v refused as %q", accepted, reason)
			}
		}
		for name, rejected := range map[string][]claudeMCPRow{
			"missing claim":   {verdi},
			"missing context": {vatc},
			"duplicate":       {vatc, vatc},
			"extra":           {vatc, verdi, row("third", "connected")},
			"disconnected":    {vatc, row("verdi-context", "disconnected")},
			"renamed":         {vatc, row("verdi_context", "connected")},
		} {
			if reason := validateInitMCPServers(rejected); reason != "mcp-mismatch" {
				t.Fatalf("%s inventory reason = %q, want mcp-mismatch", name, reason)
			}
		}
	})

	t.Run("start_fixture_observation_kinds", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-start.jsonl", launch.Workspace.Path)}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeRun(t, run)
		if result.ObservedSessionRef != "claude-sess-start-001" {
			t.Fatalf("observed session = %q, want claude-sess-start-001", result.ObservedSessionRef)
		}
		wantKinds := []contextevent.Kind{
			contextevent.KindAdapterStart, contextevent.KindProviderSummary, // init
			contextevent.KindProviderMessage, // assistant text
			contextevent.KindProviderSummary, // terminal result
			contextevent.KindAdapterStop,     // process terminal
		}
		gotKinds := observationKindsC(result.Observations)
		if !reflect.DeepEqual(gotKinds, wantKinds) {
			t.Fatalf("observation kinds = %v, want %v", gotKinds, wantKinds)
		}
	})

	t.Run("start_fixture_adapter_start_has_no_detail", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-start.jsonl", launch.Workspace.Path)}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeRun(t, run)
		startPayload, ok := result.Observations[0].Payload.(*contextevent.AdapterStartPayload)
		if !ok {
			t.Fatalf("first observation not AdapterStartPayload: %T", result.Observations[0].Payload)
		}
		if startPayload.Detail != nil {
			t.Fatal("builder adapter-start should have nil detail (not a review)")
		}
		if startPayload.Adapter != contextevent.AdapterClaude {
			t.Fatalf("adapter-start adapter = %q, want claude", startPayload.Adapter)
		}
	})

	t.Run("resume_fixture_prefix_is_adapter_start_then_summary", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionResume)
		session := "claude-sess-resume-001"
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-resume.jsonl", launch.Workspace.Path)}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Resume(context.Background(), launch, session)
		if err != nil {
			t.Fatalf("Resume: %v", err)
		}
		result := collectClaudeRun(t, run)
		if result.ObservedSessionRef != session {
			t.Fatalf("observed session = %q, want %q", result.ObservedSessionRef, session)
		}
		// Amendment 002 §7 / Codex ruling 3: Task 5 owns the exact
		// acknowledged-prefix resume observation. The adapter never invents one.
		if hasKindC(result.Observations, contextevent.KindResume) {
			t.Fatalf("adapter must not invent a resume observation, got %v", observationKindsC(result.Observations))
		}
		wantPrefix := []contextevent.Kind{contextevent.KindAdapterStart, contextevent.KindProviderSummary}
		if len(result.Observations) < len(wantPrefix) || !reflect.DeepEqual(observationKindsC(result.Observations[:len(wantPrefix)]), wantPrefix) {
			t.Fatalf("resume prefix kinds = %v, want %v", observationKindsC(result.Observations), wantPrefix)
		}
	})

	t.Run("resume_fixture_thinking_yields_provider_summary", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionResume)
		session := "claude-sess-resume-001"
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-resume.jsonl", launch.Workspace.Path)}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Resume(context.Background(), launch, session)
		if err != nil {
			t.Fatalf("Resume: %v", err)
		}
		result := collectClaudeRun(t, run)
		summaryCount := 0
		for _, obs := range result.Observations {
			if obs.Kind == contextevent.KindProviderSummary {
				summaryCount++
			}
		}
		// init summary + thinking summary + terminal result summary = 3
		if summaryCount != 3 {
			t.Fatalf("provider-summary count = %d, want 3 (init + thinking + terminal)", summaryCount)
		}
	})

	t.Run("advisory_fixture_retry_and_error_kinds", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-advisory.jsonl", launch.Workspace.Path)}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeRunAll(t, run)
		if !hasKindC(result.Observations, contextevent.KindRetry) {
			t.Fatalf("advisory fixture should produce retry observation, got: %v", observationKindsC(result.Observations))
		}
		// error_max_turns result => adapter-error
		if !hasKindC(result.Observations, contextevent.KindAdapterError) {
			t.Fatalf("advisory fixture error_max_turns should produce adapter-error, got: %v", observationKindsC(result.Observations))
		}
	})

	t.Run("empty_line_yields_telemetry_gap", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: []byte("\n")}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeUntilBoundary(t, run)
		if !hasKindC(result.Observations, contextevent.KindTelemetryGap) || !hasKindC(result.Observations, contextevent.KindAdapterError) || !blocksAuthorityC(result.Observations) {
			t.Fatalf("empty line: got %v, want telemetry-gap + adapter-error blocking authority", observationKindsC(result.Observations))
		}
	})

	t.Run("malformed_json_yields_telemetry_gap", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: []byte("{not-json}\n")}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeUntilBoundary(t, run)
		if !hasKindC(result.Observations, contextevent.KindTelemetryGap) {
			t.Fatalf("malformed json: want telemetry-gap, got %v", observationKindsC(result.Observations))
		}
	})

	// SI-187, superseding this row's pre-SI-187 name and assertion: a bare
	// unrecognized type — even a dotted, non-identifier-shaped one, and even
	// carrying an extra unread field — is advisory telemetry, not a stream
	// contradiction. It produces no telemetry-gap of its own; the frame
	// carries no observation, and the run proceeds to the following
	// assistant frame, whose detail discloses the tolerated family.
	t.Run("unrecognized_type_with_extra_field_is_tolerated_advisory_telemetry", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		assistant := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: []byte(
			claudeInitLine("s1", launch.Workspace.Path) + "\n" +
				`{"type":"future.event","value":1}` + "\n" +
				assistant + "\n" +
				claudeResultLine("s1", "success", false) + "\n",
		)}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeUntilBoundary(t, run)
		if hasKindC(result.Observations, contextevent.KindTelemetryGap) {
			t.Fatalf("unrecognized type: want no telemetry-gap, got %v", observationKindsC(result.Observations))
		}
		text := claudeFindKind(t, result.Observations, contextevent.KindProviderMessage)
		const want = `"unknown-foreign-family":["future.event"]`
		if !bytes.Contains(text.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("assistant detail = %s, want it to contain %s", text.ForeignDetail.RedactedJSON, want)
		}
	})

	t.Run("result_before_init_yields_gap", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		resultLine := []byte(`{"type":"result","subtype":"success","is_error":false,"result":"success","session_id":"s1","uuid":"u1","duration_ms":1,"duration_api_ms":1,"num_turns":0,"total_cost_usd":0.0,"usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0},"permission_denials":[]}` + "\n")
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: resultLine}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeUntilBoundary(t, run)
		if !blocksAuthorityC(result.Observations) {
			t.Fatalf("result before init: should block authority, got %v", observationKindsC(result.Observations))
		}
	})

	// --- Four literal mutation witnesses ---

	t.Run("mutation_outer_tag_fails_init_row", func(t *testing.T) {
		// Mutate "system" outer type to "future" — init start row must not succeed
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		mutatedInit := []byte(`{"type":"future","subtype":"init","session_id":"s1","model":"claude-opus-5-test","mcp_servers":[{"name":"vatc","status":"connected"},{"name":"verdi-context","status":"connected"}],"cwd":"/workspace","tools":[],"permissionMode":"bypassPermissions","apiKeySource":"ANTHROPIC_API_KEY","claude_code_version":"1.2.3","slash_commands":[],"output_style":"default","agents":[],"skills":[],"plugins":[],"uuid":"u1"}` + "\n")
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mutatedInit}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeUntilBoundary(t, run)
		if !blocksAuthorityC(result.Observations) {
			t.Fatal("mutated outer tag should block authority (unknown family)")
		}
	})

	t.Run("mutation_block_tag_fails_assistant_text_row", func(t *testing.T) {
		// Mutate "text" block type to "unknown_block" — provider-message must not be produced
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		initLine := []byte(claudeInitLine("claude-sess-start-001", launch.Workspace.Path) + "\n")
		mutatedAssistant := []byte(`{"type":"assistant","session_id":"claude-sess-start-001","uuid":"mu","message":{"id":"msg_m","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"unknown_block","text":"hello"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}` + "\n")
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: append(initLine, mutatedAssistant...)}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeRunAll(t, run)
		if hasKindC(result.Observations, contextevent.KindProviderMessage) {
			t.Fatal("mutated block tag should not produce provider-message")
		}
		if !hasKindC(result.Observations, contextevent.KindAdapterError) {
			t.Fatal("mutated block tag should produce adapter-error")
		}
	})

	t.Run("assistant_text_detail_and_id_are_exact_literals", func(t *testing.T) {
		// §5 assistant text: D={family,message_id,block_index,text}; the
		// provider-message id is exactly "<message_id>:<block_index>".
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-start.jsonl", launch.Workspace.Path)}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeRun(t, run)
		var message sealedexec.NormalizedObservation
		var found bool
		for _, obs := range result.Observations {
			if string(obs.Kind) == "provider-message" {
				message, found = obs, true
				break
			}
		}
		if !found {
			t.Fatalf("no provider-message observation; kinds = %v", observationKindsC(result.Observations))
		}
		payload, ok := message.Payload.(*contextevent.ProviderMessagePayload)
		if !ok {
			t.Fatalf("provider-message payload type = %T", message.Payload)
		}
		if payload.MessageID != "msg_001:0" || payload.Role != "assistant" {
			t.Fatalf("provider-message id/role = %q/%q, want msg_001:0/assistant", payload.MessageID, payload.Role)
		}
		if string(message.ForeignDetail.RedactedJSON) != claudeAssistantTextDetail {
			t.Fatalf("assistant text detail bytes =\n%s\nwant\n%s", message.ForeignDetail.RedactedJSON, claudeAssistantTextDetail)
		}
		if string(payload.Detail.RedactedJSON) != claudeAssistantTextDetail {
			t.Fatalf("payload detail bytes =\n%s\nwant\n%s", payload.Detail.RedactedJSON, claudeAssistantTextDetail)
		}
		if payload.MessageDigest != claudeAssistantTextDigest || message.ForeignDetail.Digest != claudeAssistantTextDigest {
			t.Fatalf("message_digest/detail digest = %q/%q, want %q over the exact literal preimage",
				payload.MessageDigest, message.ForeignDetail.Digest, claudeAssistantTextDigest)
		}
		if message.ForeignDetail.Mode != contextevent.DetailInline ||
			message.ForeignDetail.MediaType != contextevent.MediaTypeJSON ||
			message.ForeignDetail.RedactionProfile != contextevent.RedactionProfileStandard {
			t.Fatalf("detail union = %+v, want inline application/json verdi.redaction/standard-v1", message.ForeignDetail)
		}
	})

	t.Run("thinking_omission_detail_is_an_exact_literal", func(t *testing.T) {
		// §5 thinking/redacted thinking: D={content_type,omitted:true}; hidden
		// bytes are inputs to neither R nor H.
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionResume)
		session := "claude-sess-resume-001"
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-resume.jsonl", launch.Workspace.Path)}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Resume(context.Background(), launch, session)
		if err != nil {
			t.Fatalf("Resume: %v", err)
		}
		result := collectClaudeRun(t, run)
		var summary sealedexec.NormalizedObservation
		var payload *contextevent.ProviderSummaryPayload
		for _, obs := range result.Observations {
			if string(obs.Kind) != "provider-summary" {
				continue
			}
			candidate, _ := obs.Payload.(*contextevent.ProviderSummaryPayload)
			if candidate != nil && candidate.SummaryID == "msg_002:0" {
				summary, payload = obs, candidate
				break
			}
		}
		if payload == nil {
			t.Fatal("no provider-summary for the thinking block (msg_002:0)")
		}
		if string(payload.Authority) != "advisory" {
			t.Fatalf("thinking summary authority = %q, want advisory", payload.Authority)
		}
		if string(summary.ForeignDetail.RedactedJSON) != claudeThinkingOmissionDetail {
			t.Fatalf("thinking detail bytes = %s, want exactly %s", summary.ForeignDetail.RedactedJSON, claudeThinkingOmissionDetail)
		}
		if payload.SummaryDigest != claudeThinkingOmissionDigest {
			t.Fatalf("thinking summary_digest = %q, want %q over the exact literal preimage", payload.SummaryDigest, claudeThinkingOmissionDigest)
		}
		for _, obs := range result.Observations {
			if bytes.Contains(obs.ForeignDetail.RedactedJSON, []byte("internal deliberation")) ||
				bytes.Contains(obs.ForeignDetail.RedactedJSON, []byte("thinking-signature")) {
				t.Fatalf("observation %q retained hidden thinking bytes", obs.Kind)
			}
		}
	})

	t.Run("provider_session_is_protected_before_any_detail", func(t *testing.T) {
		// §5: the private observed session joins the protected-value set before
		// I is redacted. Prosecuted in a non-sensitive position (assistant text)
		// so key-class redaction cannot mask a missing protected value.
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		const session = "claude-sess-protected-001"
		assistant := `{"type":"assistant","session_id":"` + session +
			`","uuid":"u-a","message":{"id":"msg_p","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"session ` + session +
			` leaked"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLines(t, launch, envRoot,
			claudeInitLine(session, launch.Workspace.Path), assistant)
		const wantDetail = `{"block_index":0,"family":"assistant/text","message_id":"msg_p","text":"session [REDACTED] leaked"}`
		var found bool
		for _, obs := range result.Observations {
			if string(obs.Kind) != "provider-message" {
				continue
			}
			found = true
			if string(obs.ForeignDetail.RedactedJSON) != wantDetail {
				t.Fatalf("assistant detail =\n%s\nwant\n%s", obs.ForeignDetail.RedactedJSON, wantDetail)
			}
		}
		if !found {
			t.Fatalf("no provider-message observation; kinds = %v", observationKindsC(result.Observations))
		}
		for _, obs := range result.Observations {
			if bytes.Contains(obs.ForeignDetail.RedactedJSON, []byte(session)) {
				t.Fatalf("observation %q disclosed the provider session", obs.Kind)
			}
		}
	})

	t.Run("init_summary_detail_is_an_exact_literal", func(t *testing.T) {
		// §5 init start: I={family,model,mcp_servers,permission_mode,session_id}
		// with the observed provider session already in the protected set.
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-start.jsonl", launch.Workspace.Path)}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeRun(t, run)
		var summary sealedexec.NormalizedObservation
		var payload *contextevent.ProviderSummaryPayload
		for _, obs := range result.Observations {
			if string(obs.Kind) != "provider-summary" {
				continue
			}
			candidate, _ := obs.Payload.(*contextevent.ProviderSummaryPayload)
			if candidate != nil && candidate.SummaryID == "system/init" {
				summary, payload = obs, candidate
				break
			}
		}
		if payload == nil {
			t.Fatalf("no system/init provider-summary; kinds = %v", observationKindsC(result.Observations))
		}
		if string(payload.Authority) != "advisory" {
			t.Fatalf("init summary authority = %q, want advisory", payload.Authority)
		}
		if string(summary.ForeignDetail.RedactedJSON) != claudeInitSummaryDetail {
			t.Fatalf("init detail bytes =\n%s\nwant\n%s", summary.ForeignDetail.RedactedJSON, claudeInitSummaryDetail)
		}
		if payload.SummaryDigest != claudeInitSummaryDigest {
			t.Fatalf("init summary_digest = %q, want %q over the exact literal preimage", payload.SummaryDigest, claudeInitSummaryDigest)
		}
		for _, obs := range result.Observations {
			if bytes.Contains(obs.ForeignDetail.RedactedJSON, []byte("claude-sess-start-001")) {
				t.Fatalf("observation %q disclosed the provider session", obs.Kind)
			}
		}
	})

	// -----------------------------------------------------------------
	// Wave B: closed stream grammar (C1), safe malformed reduction (C2),
	// stderr-bearing process terminal (C3), exact process reason/operation/
	// source/precedence (C6), and I1-I5.
	// -----------------------------------------------------------------

	// SI-182: an unknown member at the init frame's own object level is
	// tolerated (never read) and its dotted path is recorded once, under the
	// closed code `unknown-foreign-member`, in the init provider-summary
	// detail — replacing the pre-SI-182 refusal this row used to prove.
	t.Run("init_tolerates_unknown_top_level_member_and_records_witness", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeInitLine("s1", launch.Workspace.Path),
			`"uuid":"u-init"`, `"uuid":"u-init","future_key":1`, 1)
		result := runClaudeLines(t, launch, envRoot, line, claudeResultLine("s1", "success", false))
		summary := claudeFindProviderSummary(t, result.Observations, "system/init")
		const want = `"unknown-foreign-member":["future_key"]`
		if !bytes.Contains(summary.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("init detail = %s, want it to contain %s", summary.ForeignDetail.RedactedJSON, want)
		}
	})

	// SI-182 test row (a): the exact 8 unknown init members the real Claude
	// Code CLI 2.1.261 emits (measured offline by the F12 canary track,
	// 2026-09-06), reproduced here with synthetic values — never the measured
	// bytes — all decode and are listed sorted and deduplicated.
	t.Run("init_tolerates_the_measured_2_1_261_unknown_member_set", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeInitLine("s1", launch.Workspace.Path), `"uuid":"u-init"}`,
			`"analytics_disabled":false,"capabilities":["interrupt_receipt_v1"],`+
				`"fast_mode_disabled_reason":"sdk_opt_in_required","fast_mode_state":"off",`+
				`"memory_paths":{"auto":"/synthetic/memory/"},"messaging_socket_path":"/synthetic/cc.sock",`+
				`"product_feedback_disabled":false,"terminal_slash_commands":["doctor"],"uuid":"u-init"}`, 1)
		result := runClaudeLines(t, launch, envRoot, line, claudeResultLine("s1", "success", false))
		summary := claudeFindProviderSummary(t, result.Observations, "system/init")
		const want = `"unknown-foreign-member":["analytics_disabled","capabilities","fast_mode_disabled_reason",` +
			`"fast_mode_state","memory_paths","messaging_socket_path","product_feedback_disabled","terminal_slash_commands"]`
		if !bytes.Contains(summary.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("init detail = %s, want it to contain %s", summary.ForeignDetail.RedactedJSON, want)
		}
	})

	// SI-182 test row (i): a clean fixture with no unknown members discloses
	// no witness entry anywhere in the run.
	t.Run("no_unknown_members_yields_no_witness_entry", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-start.jsonl", launch.Workspace.Path)}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeRun(t, run)
		for _, obs := range result.Observations {
			if bytes.Contains(obs.ForeignDetail.RedactedJSON, []byte(unknownMemberCode)) {
				t.Fatalf("observation %s disclosed a witness over a clean fixture: %s", obs.Kind, obs.ForeignDetail.RedactedJSON)
			}
			// SI-187: a fixture whose stream carries only known families is
			// byte-identical to its pre-SI-187 output — its witnesses gain no
			// unknown-foreign-family entry either.
			if bytes.Contains(obs.ForeignDetail.RedactedJSON, []byte(unknownFamilyCode)) {
				t.Fatalf("observation %s disclosed a family witness over a clean fixture: %s", obs.Kind, obs.ForeignDetail.RedactedJSON)
			}
		}
	})

	// SI-187: a "system" frame naming no subtype at all names no family the
	// decoder can be tolerant of (there is nothing to record as advisory
	// telemetry) and stays refused exactly as before SI-187.
	t.Run("system_frame_with_no_subtype_still_refused", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		result := runClaudeLines(t, launch, envRoot, `{"type":"system"}`)
		assertClaudeGapReason(t, result, "unknown-foreign-family", "decode", claudeSource)
	})

	// SI-187, superseding the pre-SI-187 "unknown_system_subtype_still_refused"
	// row: an unrecognized subtype of a known "system" type is now advisory
	// provider telemetry rather than a refusal. The frame is skipped — no
	// projection, no digest, no observation of its own — and its family is
	// recorded once, attached to the next accepted observation (here, the
	// following assistant frame's own detail), which keeps its ordinary
	// fixed message id.
	t.Run("unknown_system_subtype_is_tolerated_and_recorded", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		assistant := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path),
			`{"type":"system","subtype":"heartbeat"}`, assistant, claudeResultLine("s1", "success", false))
		if result.OperationalFailure != "" {
			t.Fatalf("operational failure = %q, want none (unknown family is advisory telemetry)", result.OperationalFailure)
		}
		text := claudeFindKind(t, result.Observations, contextevent.KindProviderMessage)
		payload, ok := text.Payload.(*contextevent.ProviderMessagePayload)
		if !ok || payload.MessageID != "msg_1:0" {
			t.Fatalf("provider-message id = %+v, want msg_1:0 (unaffected by the skipped frame)", text.Payload)
		}
		const want = `"unknown-foreign-family":["system/heartbeat"]`
		if !bytes.Contains(text.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("assistant detail = %s, want it to contain %s", text.ForeignDetail.RedactedJSON, want)
		}
	})

	// SI-187: system/thinking_tokens between init and the first assistant
	// frame — the exact shape that interrupted the F12 canary's eighth
	// flight — is tolerated and recorded; the assistant frame that follows
	// keeps its ordinary fixed id.
	t.Run("system_thinking_tokens_between_init_and_first_assistant_is_tolerated", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		assistant := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path),
			`{"type":"system","subtype":"thinking_tokens"}`, assistant, claudeResultLine("s1", "success", false))
		if result.OperationalFailure != "" {
			t.Fatalf("operational failure = %q, want none (unknown family is advisory telemetry)", result.OperationalFailure)
		}
		text := claudeFindKind(t, result.Observations, contextevent.KindProviderMessage)
		payload, ok := text.Payload.(*contextevent.ProviderMessagePayload)
		if !ok || payload.MessageID != "msg_1:0" {
			t.Fatalf("provider-message id = %+v, want msg_1:0 (unaffected by the skipped frame)", text.Payload)
		}
		const want = `"unknown-foreign-family":["system/thinking_tokens"]`
		if !bytes.Contains(text.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("assistant detail = %s, want it to contain %s", text.ForeignDetail.RedactedJSON, want)
		}
	})

	// SI-187: tool_progress, status, rate_limit_event and keep_alive are
	// further families the real Claude Code CLI 2.1.261 emits; every one is
	// tolerated. Each distinct family is recorded exactly once, in the
	// first-seen order they arrived (not sorted, unlike SI-182's member
	// paths), and a repeated family (tool_progress recurs here) is never
	// disclosed a second time.
	t.Run("further_unknown_families_are_tolerated_and_recorded_once_each", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		assistant := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path),
			`{"type":"tool_progress"}`, `{"type":"status"}`, `{"type":"rate_limit_event"}`, `{"type":"keep_alive"}`,
			`{"type":"tool_progress"}`, assistant, claudeResultLine("s1", "success", false))
		if result.OperationalFailure != "" {
			t.Fatalf("operational failure = %q, want none (unknown families are advisory telemetry)", result.OperationalFailure)
		}
		text := claudeFindKind(t, result.Observations, contextevent.KindProviderMessage)
		const want = `"unknown-foreign-family":["tool_progress","status","rate_limit_event","keep_alive"]`
		if !bytes.Contains(text.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("assistant detail = %s, want it to contain %s (first-seen order, deduplicated)", text.ForeignDetail.RedactedJSON, want)
		}
	})

	// SI-187: an unknown family discovered with no further accepted frame
	// afterward — the stream ends before any result frame arrives — has no
	// ordinary observation left to carry it. The terminal must drain it into
	// an advisory summary of its own rather than dropping the disclosure.
	t.Run("unknown_family_pending_at_process_exit_is_disclosed_in_a_terminal_summary", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), `{"type":"system","subtype":"thinking_tokens"}`)
		if result.OperationalFailure != "missing-terminal-result" {
			t.Fatalf("operational failure = %q, want missing-terminal-result (no result frame ever arrived)", result.OperationalFailure)
		}
		const want = `"unknown-foreign-family":["system/thinking_tokens"]`
		var disclosed bool
		for _, obs := range result.Observations {
			payload, ok := obs.Payload.(*contextevent.ProviderSummaryPayload)
			if !ok || !strings.HasPrefix(payload.SummaryID, "unknown-families/") {
				continue
			}
			if bytes.Contains(obs.ForeignDetail.RedactedJSON, []byte(want)) {
				disclosed = true
			}
		}
		if !disclosed {
			t.Fatalf("no terminal unknown-families summary disclosed %s; observations = %v", want, observationKindsC(result.Observations))
		}
	})

	// SI-187: a frame with no `type` string at all names no family and stays
	// refused exactly as before.
	t.Run("frame_with_no_type_is_still_refused", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		result := runClaudeLines(t, launch, envRoot, `{"subtype":"init"}`)
		assertClaudeGapReason(t, result, "missing-foreign-field", "decode", claudeSource)
	})

	// SI-187: a malformed frame of a known family — here, an assistant frame
	// missing its required message — stays refused exactly as before; SI-187
	// only tolerates a family the decoder does not recognize at all.
	t.Run("malformed_frame_of_a_known_family_still_refused", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"assistant","session_id":"s1","uuid":"mu"}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad)
		assertClaudeGapReason(t, result, "missing-foreign-field", "decode", claudeSource)
	})

	// SI-182 test row (f): a duplicate JSON key stays refused exactly as
	// before — SI-182 tolerates unknown members, never duplicate ones.
	t.Run("duplicate_key_still_refused", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeInitLine("s1", launch.Workspace.Path),
			`"session_id":"s1"`, `"session_id":"s1","session_id":"s1"`, 1)
		result := runClaudeLines(t, launch, envRoot, line)
		assertClaudeGapReason(t, result, "malformed-foreign-frame", "decode", claudeSource)
	})

	// SI-182 test row (g): trailing data after one complete frame value stays
	// refused exactly as before.
	t.Run("trailing_data_still_refused", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := claudeInitLine("s1", launch.Workspace.Path) + `{"extra":true}`
		result := runClaudeLines(t, launch, envRoot, line)
		assertClaudeGapReason(t, result, "malformed-foreign-frame", "decode", claudeSource)
	})

	t.Run("init_rejects_missing_required_field", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeInitLine("s1", launch.Workspace.Path),
			`,"uuid":"u-init"`, ``, 1)
		result := runClaudeLines(t, launch, envRoot, line)
		assertClaudeGapReason(t, result, "missing-foreign-field", "decode", claudeSource)
	})

	t.Run("init_rejects_foreign_cwd", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", "/elsewhere"))
		assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
	})

	t.Run("init_rejects_version_contradiction", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeInitLine("s1", launch.Workspace.Path),
			`"claude_code_version":"1.2.3"`, `"claude_code_version":"9.9.9"`, 1)
		result := runClaudeLines(t, launch, envRoot, line)
		assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
	})

	t.Run("init_rejects_foreign_api_key_source", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeInitLine("s1", launch.Workspace.Path),
			`"apiKeySource":"ANTHROPIC_API_KEY"`, `"apiKeySource":"/login"`, 1)
		result := runClaudeLines(t, launch, envRoot, line)
		assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
	})

	t.Run("init_rejects_duplicate_tool_names", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeInitLine("s1", launch.Workspace.Path),
			`"tools":["Task"]`, `"tools":["Task","Task"]`, 1)
		result := runClaudeLines(t, launch, envRoot, line)
		assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
	})

	// SI-182: an unknown member of the assistant frame's "message" object is
	// tolerated and its dotted path ("message.<key>") is recorded once,
	// attached to the message's provider-message detail — replacing the
	// pre-SI-182 refusal this row used to prove.
	t.Run("assistant_tolerates_unknown_message_field_and_records_witness", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1},"future_key":true}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad, claudeResultLine("s1", "success", false))
		message := claudeFindKind(t, result.Observations, contextevent.KindProviderMessage)
		const want = `"unknown-foreign-member":["message.future_key"]`
		if !bytes.Contains(message.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("assistant text detail = %s, want it to contain %s", message.ForeignDetail.RedactedJSON, want)
		}
	})

	// SI-182 test row (h): an unknown member nested inside
	// assistant.message.usage — a known nested struct — is tolerated and
	// recorded with its full dotted path.
	t.Run("assistant_tolerates_unknown_usage_member_and_records_witness", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"cache_write_tokens":2}}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad, claudeResultLine("s1", "success", false))
		message := claudeFindKind(t, result.Observations, contextevent.KindProviderMessage)
		const want = `"unknown-foreign-member":["message.usage.cache_write_tokens"]`
		if !bytes.Contains(message.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("assistant text detail = %s, want it to contain %s", message.ForeignDetail.RedactedJSON, want)
		}
	})

	// SI-182: a content-block member is recorded at the path it actually
	// occupies in the frame — message.content.<key> — never at a frame-level
	// content.<key> that names no object the frame contains.
	t.Run("assistant_records_a_content_block_member_under_message_content", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi","future_key":true}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad, claudeResultLine("s1", "success", false))
		message := claudeFindKind(t, result.Observations, contextevent.KindProviderMessage)
		const want = `"unknown-foreign-member":["message.content.future_key"]`
		if !bytes.Contains(message.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("assistant text detail = %s, want it to contain %s", message.ForeignDetail.RedactedJSON, want)
		}
	})

	// The same rooting for the user family's tool_result blocks.
	t.Run("tool_result_records_a_content_block_member_under_message_content", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		toolUse := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"tool_use","id":"call_1","name":"Read","input":{"path":"README.md"}}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		toolResult := `{"type":"user","session_id":"s1","uuid":"tu","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":"ok","future_key":true}]}}`
		result := runClaudeLines(t, launch, envRoot,
			claudeInitLine("s1", launch.Workspace.Path), toolUse, toolResult, claudeResultLine("s1", "success", false))
		toolResultObs := claudeFindKind(t, result.Observations, contextevent.KindToolResult)
		const want = `"unknown-foreign-member":["message.content.future_key"]`
		if !bytes.Contains(toolResultObs.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("tool-result detail = %s, want it to contain %s", toolResultObs.ForeignDetail.RedactedJSON, want)
		}
	})

	// SI-182/SI-184: modelUsage is keyed by model, so a member unknown to the
	// per-model camelCase usage shape is recorded at
	// modelUsage.<model>.<key>. A bare modelUsage.<key> would name a path the
	// frame does not contain. costUSD is deliberately NOT used as the
	// unknown member here: SI-184 makes it a known optional member of this
	// shape (see the SI-184 tests below), so an outsider member is used
	// instead.
	t.Run("result_records_a_per_model_usage_member_under_its_model_key", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeResultLine("s1", "success", false), `,"permission_denials":[]`,
			`,"permission_denials":[],"modelUsage":{"claude-opus-5-test":{"inputTokens":1,"outputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"latencyMs":9999}}`, 1)
		result := runClaudeLinesToTerminal(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
		summary := claudeFindProviderSummary(t, result.Observations, "terminal-result")
		const want = `"unknown-foreign-member":["modelUsage.claude-opus-5-test.latencyMs"]`
		if !bytes.Contains(summary.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("result detail = %s, want it to contain %s", summary.ForeignDetail.RedactedJSON, want)
		}
	})

	// SI-182 records the path, full stop — the obligation does not depend on
	// what else the frame happens to carry. An accepted assistant frame whose
	// content is empty has no content-block detail to attach the disclosure
	// to, so it gets a summary of its own instead of dropping it silently.
	t.Run("assistant_with_empty_content_still_records_its_unknown_members", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"assistant","session_id":"s1","uuid":"mu","future_frame_key":1,"message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1},"future_key":true}}`
		result := runClaudeLinesToTerminal(t, launch, envRoot,
			claudeInitLine("s1", launch.Workspace.Path), bad, claudeResultLine("s1", "success", false))
		// The assistant frame is the second source line of the stream.
		summary := claudeFindProviderSummary(t, result.Observations, "unknown-members/2")
		const want = `{"family":"assistant","unknown-foreign-member":["future_frame_key","message.future_key"]}`
		if got := string(summary.ForeignDetail.RedactedJSON); got != want {
			t.Fatalf("empty-content disclosure detail = %s, want %s", got, want)
		}
	})

	// The same obligation on the user family, whose content is likewise
	// allowed to be empty.
	t.Run("user_frame_with_empty_content_still_records_its_unknown_members", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"user","session_id":"s1","uuid":"tu","future_frame_key":1,"message":{"role":"user","content":[],"future_key":true}}`
		result := runClaudeLinesToTerminal(t, launch, envRoot,
			claudeInitLine("s1", launch.Workspace.Path), bad, claudeResultLine("s1", "success", false))
		summary := claudeFindProviderSummary(t, result.Observations, "unknown-members/2")
		const want = `{"family":"user","unknown-foreign-member":["future_frame_key","message.future_key"]}`
		if got := string(summary.ForeignDetail.RedactedJSON); got != want {
			t.Fatalf("empty-content disclosure detail = %s, want %s", got, want)
		}
	})

	// The disclosure summary exists only for the disclosure: a clean
	// empty-content frame still produces no observation of its own, exactly
	// as it did before SI-182.
	t.Run("clean_empty_content_frame_yields_no_disclosure_summary", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		clean := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLinesToTerminal(t, launch, envRoot,
			claudeInitLine("s1", launch.Workspace.Path), clean, claudeResultLine("s1", "success", false))
		for _, obs := range result.Observations {
			if summary, ok := obs.Payload.(*contextevent.ProviderSummaryPayload); ok &&
				strings.HasPrefix(summary.SummaryID, "unknown-members/") {
				t.Fatalf("clean empty-content frame emitted %q: %s", summary.SummaryID, obs.ForeignDetail.RedactedJSON)
			}
			if bytes.Contains(obs.ForeignDetail.RedactedJSON, []byte(unknownMemberCode)) {
				t.Fatalf("clean run disclosed a witness on %s: %s", obs.Kind, obs.ForeignDetail.RedactedJSON)
			}
		}
	})

	// A frame that does carry content keeps disclosing on its first block, so
	// the disclosure is recorded exactly once either way.
	t.Run("nonempty_content_keeps_the_disclosure_on_its_first_block", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"assistant","session_id":"s1","uuid":"mu","future_frame_key":1,"message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLinesToTerminal(t, launch, envRoot,
			claudeInitLine("s1", launch.Workspace.Path), bad, claudeResultLine("s1", "success", false))
		message := claudeFindKind(t, result.Observations, contextevent.KindProviderMessage)
		const want = `"unknown-foreign-member":["future_frame_key"]`
		if !bytes.Contains(message.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("assistant text detail = %s, want it to contain %s", message.ForeignDetail.RedactedJSON, want)
		}
		for _, obs := range result.Observations {
			if summary, ok := obs.Payload.(*contextevent.ProviderSummaryPayload); ok &&
				strings.HasPrefix(summary.SummaryID, "unknown-members/") {
				t.Fatalf("a frame with content blocks must not also emit %q", summary.SummaryID)
			}
		}
	})

	// SI-182's other half: a tolerated member is NEVER READ. The result
	// detail is rebuilt from the typed decode, so a clean frame's projection
	// is byte-identical to the passthrough it replaced — this row is the
	// byte-identity ratchet the "never read" rebuild must not disturb.
	t.Run("clean_result_detail_is_an_exact_literal", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		result := runClaudeLinesToTerminal(t, launch, envRoot,
			claudeInitLine("s1", launch.Workspace.Path), claudeResultLine("s1", "success", false))
		assertClaudeResultDetail(t, result, claudeCleanResultDetail)
	})

	// SI-182: an unknown member of the result frame's usage object is
	// recorded and its value never reaches the detail or the hashed digest.
	// The whole detail is asserted, so the member's absence is proven, not
	// sampled: 4242 appears nowhere.
	t.Run("result_never_projects_a_tolerated_usage_member", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeResultLine("s1", "success", false),
			`"output_tokens":1}`, `"output_tokens":1,"server_tool_use":{"web_search_requests":4242}}`, 1)
		result := runClaudeLinesToTerminal(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
		assertClaudeResultDetail(t, result,
			`{"duration_api_ms":9,"duration_ms":10,"family":"result","is_error":false,"num_turns":1,`+
				`"permission_denials":[],"result":"done","subtype":"success","total_cost_usd":0.001,`+
				`"unknown-foreign-member":["usage.server_tool_use"],`+
				`"usage":{"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"input_tokens":1,"output_tokens":1}}`)
	})

	// The same for an unknown member of a permission-denial row: the row is
	// rebuilt from its three accepted members, so the foreign value cannot
	// ride along.
	t.Run("result_never_projects_a_tolerated_permission_denial_member", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeResultLine("s1", "success", false), `"permission_denials":[]`,
			`"permission_denials":[{"tool_name":"Bash","tool_use_id":"call_9","tool_input":{"command":"ls"},"future_row_key":"leaked-7777"}]`, 1)
		result := runClaudeLinesToTerminal(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
		assertClaudeResultDetail(t, result,
			`{"duration_api_ms":9,"duration_ms":10,"family":"result","is_error":false,"num_turns":1,`+
				`"permission_denials":[{"tool_input":{"command":"ls"},"tool_name":"Bash","tool_use_id":"call_9"}],`+
				`"result":"done","subtype":"success","total_cost_usd":0.001,`+
				`"unknown-foreign-member":["permission_denials.future_row_key"],`+
				`"usage":{"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"input_tokens":1,"output_tokens":1}}`)
	})

	// And for the per-model usage object: modelUsage is rebuilt from the one
	// accepted model key and its typed camelCase usage (SI-184).
	t.Run("result_never_projects_a_tolerated_model_usage_member", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeResultLine("s1", "success", false), `,"permission_denials":[]`,
			`,"permission_denials":[],"modelUsage":{"claude-opus-5-test":{"inputTokens":1,"outputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"latencyMs":9999}}`, 1)
		result := runClaudeLinesToTerminal(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
		assertClaudeResultDetail(t, result,
			`{"duration_api_ms":9,"duration_ms":10,"family":"result","is_error":false,`+
				`"modelUsage":{"claude-opus-5-test":{"cacheCreationInputTokens":0,"cacheReadInputTokens":0,"inputTokens":1,"outputTokens":1}},`+
				`"num_turns":1,"permission_denials":[],"result":"done","subtype":"success","total_cost_usd":0.001,`+
				`"unknown-foreign-member":["modelUsage.claude-opus-5-test.latencyMs"],`+
				`"usage":{"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"input_tokens":1,"output_tokens":1}}`)
	})

	// SI-184: the per-model modelUsage.<model> value's own camelCase shape —
	// required inputTokens, outputTokens, cacheReadInputTokens,
	// cacheCreationInputTokens; optional webSearchRequests, costUSD,
	// contextWindow, maxOutputTokens — accepts a real-shaped value carrying
	// all eight members and projects it byte-exactly under those same names.
	// costUSD keeps its exact source formatting (proved as a JSON number,
	// never rounded through a Go float).
	t.Run("result_accepts_the_real_shaped_eight_member_model_usage_value", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeResultLine("s1", "success", false), `,"permission_denials":[]`,
			`,"permission_denials":[],"modelUsage":{"claude-opus-5-test":{"inputTokens":1200,"outputTokens":340,`+
				`"cacheReadInputTokens":50,"cacheCreationInputTokens":25,"webSearchRequests":2,"costUSD":0.0456,`+
				`"contextWindow":200000,"maxOutputTokens":8192}}`, 1)
		result := runClaudeLinesToTerminal(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
		assertClaudeResultDetail(t, result,
			`{"duration_api_ms":9,"duration_ms":10,"family":"result","is_error":false,`+
				`"modelUsage":{"claude-opus-5-test":{"cacheCreationInputTokens":25,"cacheReadInputTokens":50,`+
				`"contextWindow":200000,"costUSD":0.0456,"inputTokens":1200,"maxOutputTokens":8192,`+
				`"outputTokens":340,"webSearchRequests":2}},`+
				`"num_turns":1,"permission_denials":[],"result":"done","subtype":"success","total_cost_usd":0.001,`+
				`"usage":{"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"input_tokens":1,"output_tokens":1}}`)
	})

	// The four required members alone (no optional member present) are
	// likewise accepted, with only those four projected.
	t.Run("result_accepts_the_four_member_minimal_model_usage_value", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeResultLine("s1", "success", false), `,"permission_denials":[]`,
			`,"permission_denials":[],"modelUsage":{"claude-opus-5-test":{"inputTokens":7,"outputTokens":3,`+
				`"cacheReadInputTokens":0,"cacheCreationInputTokens":0}}`, 1)
		result := runClaudeLinesToTerminal(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
		assertClaudeResultDetail(t, result,
			`{"duration_api_ms":9,"duration_ms":10,"family":"result","is_error":false,`+
				`"modelUsage":{"claude-opus-5-test":{"cacheCreationInputTokens":0,"cacheReadInputTokens":0,`+
				`"inputTokens":7,"outputTokens":3}},`+
				`"num_turns":1,"permission_denials":[],"result":"done","subtype":"success","total_cost_usd":0.001,`+
				`"usage":{"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"input_tokens":1,"output_tokens":1}}`)
	})

	// SI-184 makes modelUsage.<model> its own shape, distinct from the v1
	// usage shape: a snake_case object at that position now leaves every
	// required camelCase member absent and refuses missing-foreign-field —
	// it is no longer silently accepted under the old spelling.
	t.Run("result_rejects_a_snake_case_model_usage_value", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeResultLine("s1", "success", false), `,"permission_denials":[]`,
			`,"permission_denials":[],"modelUsage":{"claude-opus-5-test":{"input_tokens":1,`+
				`"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}`, 1)
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
		assertClaudeGapReason(t, result, "missing-foreign-field", "decode", claudeSource)
	})

	// A known member of the modelUsage.<model> shape with the wrong JSON
	// type still refuses invalid-foreign-field.
	t.Run("result_rejects_a_wrong_typed_model_usage_member", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeResultLine("s1", "success", false), `,"permission_denials":[]`,
			`,"permission_denials":[],"modelUsage":{"claude-opus-5-test":{"inputTokens":"seven","outputTokens":1,`+
				`"cacheReadInputTokens":0,"cacheCreationInputTokens":0}}`, 1)
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
		assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
	})

	// An accepted optional member of the known usage shape still survives the
	// rebuild: "never read" applies to unknown members, not to §5's own.
	t.Run("result_projects_the_accepted_optional_service_tier", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeResultLine("s1", "success", false),
			`"output_tokens":1}`, `"output_tokens":1,"service_tier":"priority"}`, 1)
		result := runClaudeLinesToTerminal(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
		assertClaudeResultDetail(t, result,
			`{"duration_api_ms":9,"duration_ms":10,"family":"result","is_error":false,"num_turns":1,`+
				`"permission_denials":[],"result":"done","subtype":"success","total_cost_usd":0.001,`+
				`"usage":{"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"input_tokens":1,"output_tokens":1,"service_tier":"priority"}}`)
	})

	t.Run("assistant_rejects_missing_usage", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}]}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad)
		assertClaudeGapReason(t, result, "missing-foreign-field", "decode", claudeSource)
	})

	t.Run("assistant_rejects_unknown_service_tier", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1,"service_tier":"platinum"}}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad)
		assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
	})

	// SI-182: an unknown member of the api_retry frame's own object level is
	// tolerated and its dotted path is recorded once, attached to the retry's
	// provider-summary detail — replacing the pre-SI-182 refusal this row
	// used to prove.
	t.Run("retry_tolerates_unknown_field_and_records_witness", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"system","subtype":"api_retry","attempt":1,"max_retries":3,"retry_delay_ms":10,"error":{"type":"rate_limit","message":"slow down"},"uuid":"ru","session_id":"s1","future_key":1}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad, claudeResultLine("s1", "success", false))
		summary := claudeFindProviderSummary(t, result.Observations, "api-retry/1")
		const want = `"unknown-foreign-member":["future_key"]`
		if !bytes.Contains(summary.ForeignDetail.RedactedJSON, []byte(want)) {
			t.Fatalf("retry detail = %s, want it to contain %s", summary.ForeignDetail.RedactedJSON, want)
		}
	})

	// SI-183 closes the residual risk the SI-182 row above once flagged: the
	// real Claude Code CLI 2.1.261's measured system/api_retry frame carries
	// error as a bare string ("unknown") beside the one unknown member
	// error_status (both measured offline by the F12 canary track,
	// 2026-09-06; session/uuid values here are synthetic). Both are now
	// accepted end to end: error_status still records as an unlisted member
	// and the bare error string projects as error_category "unknown" with
	// retry reason code provider-api-unknown.
	t.Run("retry_tolerates_the_measured_error_status_member", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"system","subtype":"api_retry","attempt":1,"max_retries":10,"retry_delay_ms":510,"error_status":null,"error":"unknown","uuid":"ru","session_id":"s1"}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad, claudeResultLine("s1", "success", false))
		summary := claudeFindProviderSummary(t, result.Observations, "api-retry/1")
		for _, want := range []string{`"unknown-foreign-member":["error_status"]`, `"error_category":"unknown"`} {
			if !bytes.Contains(summary.ForeignDetail.RedactedJSON, []byte(want)) {
				t.Fatalf("retry detail = %s, want it to contain %s", summary.ForeignDetail.RedactedJSON, want)
			}
		}
		retryObs := claudeFindKind(t, result.Observations, contextevent.KindRetry)
		retryPayload, ok := retryObs.Payload.(*contextevent.RetryPayload)
		if !ok {
			t.Fatalf("retry payload = %#v, want a retry payload", retryObs.Payload)
		}
		if retryPayload.ReasonCode != "provider-api-unknown" {
			t.Fatalf("retry reason code = %q, want provider-api-unknown", retryPayload.ReasonCode)
		}
	})

	// SI-183: the system/api_retry frame's error member is accepted either as
	// the v1 object {type,message} (Amendment 002 §5, unchanged) or as a bare
	// string from the real Claude Code CLI's closed eleven-value enum (read
	// offline from the bundle's zod schema, measured 2026-09-06). Any other
	// string — including one outside the enum and the empty string — and any
	// non-string non-object value refuse invalid-foreign-field exactly as
	// every non-object error value did before this amendment; an explicit
	// JSON null continues to surface as the plain absent-key case,
	// missing-foreign-field, unchanged.
	//
	// B-F1 closure review, Minor (B-F4): every accept row pins both the exact
	// error_category projection and the exact provider-api-<value> retry
	// reason code, not just acceptance — a projection that normalized or
	// remapped any of the ten values besides "unknown" would previously have
	// kept every row green.
	t.Run("retry_error_member_accepts_the_closed_string_enum_and_the_v1_object", func(t *testing.T) {
		cases := []struct {
			name         string
			errorJSON    string
			accept       bool
			wantCategory string // only read when accept
			wantReason   string // only read when !accept; "" means invalid-foreign-field
		}{
			{name: "authentication_failed accepted", errorJSON: `"authentication_failed"`, accept: true, wantCategory: "authentication_failed"},
			{name: "oauth_org_not_allowed accepted", errorJSON: `"oauth_org_not_allowed"`, accept: true, wantCategory: "oauth_org_not_allowed"},
			{name: "account_on_hold accepted", errorJSON: `"account_on_hold"`, accept: true, wantCategory: "account_on_hold"},
			{name: "billing_error accepted", errorJSON: `"billing_error"`, accept: true, wantCategory: "billing_error"},
			{name: "rate_limit accepted", errorJSON: `"rate_limit"`, accept: true, wantCategory: "rate_limit"},
			{name: "overloaded accepted", errorJSON: `"overloaded"`, accept: true, wantCategory: "overloaded"},
			{name: "invalid_request accepted", errorJSON: `"invalid_request"`, accept: true, wantCategory: "invalid_request"},
			{name: "model_not_found accepted", errorJSON: `"model_not_found"`, accept: true, wantCategory: "model_not_found"},
			{name: "server_error accepted", errorJSON: `"server_error"`, accept: true, wantCategory: "server_error"},
			{name: "unknown accepted", errorJSON: `"unknown"`, accept: true, wantCategory: "unknown"},
			{name: "max_output_tokens accepted", errorJSON: `"max_output_tokens"`, accept: true, wantCategory: "max_output_tokens"},
			{name: "v1 object form still accepted", errorJSON: `{"type":"rate_limit","message":"slow down"}`, accept: true, wantCategory: "rate_limit"},
			{name: "outsider string refused", errorJSON: `"quota_exceeded"`},
			{name: "empty string refused", errorJSON: `""`},
			{name: "bare number refused", errorJSON: `1`},
			{name: "explicit null still reads as the absent key", errorJSON: `null`, wantReason: "missing-foreign-field"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
				line := `{"type":"system","subtype":"api_retry","attempt":1,"max_retries":3,"retry_delay_ms":10,` +
					`"error":` + tc.errorJSON + `,"uuid":"ru","session_id":"s1"}`
				if tc.accept {
					result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line, claudeResultLine("s1", "success", false))
					retryObs := claudeFindKind(t, result.Observations, contextevent.KindRetry)
					retryPayload, ok := retryObs.Payload.(*contextevent.RetryPayload)
					if !ok {
						t.Fatalf("retry payload = %#v, want a retry payload", retryObs.Payload)
					}
					wantReasonCode := "provider-api-" + tc.wantCategory
					if retryPayload.ReasonCode != wantReasonCode {
						t.Fatalf("error %s: retry reason code = %q, want %q", tc.errorJSON, retryPayload.ReasonCode, wantReasonCode)
					}
					wantCategory := `"error_category":"` + tc.wantCategory + `"`
					if !bytes.Contains(retryObs.ForeignDetail.RedactedJSON, []byte(wantCategory)) {
						t.Fatalf("error %s: retry detail = %s, want it to contain %s", tc.errorJSON, retryObs.ForeignDetail.RedactedJSON, wantCategory)
					}
					return
				}
				wantReason := tc.wantReason
				if wantReason == "" {
					wantReason = "invalid-foreign-field"
				}
				result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
				assertClaudeGapReason(t, result, wantReason, "decode", claudeSource)
			})
		}
	})

	// B-F1 (closure review, Important): scanKnownObject's shape-walk boundary
	// is shared by every known frame kind, but the sibling json.Unmarshal
	// falls back to a case-insensitive field match whenever a JSON member has
	// no exact-cased counterpart in the struct's tags. Before this fix, a
	// member differing from a known name only by case was filed as
	// unknown-foreign-member (never read, by the disclosure's own promise)
	// while ALSO being read into the corresponding typed field by that
	// fallback — so its value reached the projected detail and both digests
	// despite the disclosure swearing it never would. Every location below
	// now refuses invalid-foreign-field instead of tolerating the collision.
	// Falsify by reverting scanKnownObject to an exact-only map lookup.
	t.Run("case_variant_of_a_known_member_is_refused_not_tolerated", func(t *testing.T) {
		t.Run("api_retry frame-level collision", func(t *testing.T) {
			launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
			bad := `{"type":"system","subtype":"api_retry","Attempt":1,"max_retries":3,"retry_delay_ms":10,"error":"unknown","uuid":"ru","session_id":"s1"}`
			result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad)
			assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
		})

		t.Run("system/init frame-level collision", func(t *testing.T) {
			launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
			bad := strings.Replace(claudeInitLine("s1", launch.Workspace.Path), `"session_id":"s1"`, `"SESSION_ID":"s1"`, 1)
			result := runClaudeLines(t, launch, envRoot, bad)
			assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
		})

		// The result frame's top-level `usage` member keeps validateUsage's v1
		// snake_case shape; a case variant of one of its members is refused
		// the same way.
		t.Run("result top-level usage collision", func(t *testing.T) {
			launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
			line := strings.Replace(claudeResultLine("s1", "success", false),
				`"output_tokens":1}`, `"output_tokens":1,"INPUT_TOKENS":4242}`, 1)
			result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
			assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
		})

		// Sharpest form: both the correctly-cased and the colliding key are
		// present at once. DecodeUniqueJSONObject admits both (byte-distinct
		// keys), so before this fix the projection silently reported whichever
		// one the JSON object happened to order last — chosen by exactly the
		// member the disclosure swore was never read.
		t.Run("modelUsage.<model> collision beside the correctly-cased key", func(t *testing.T) {
			launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
			line := strings.Replace(claudeResultLine("s1", "success", false), `,"permission_denials":[]`,
				`,"permission_denials":[],"modelUsage":{"claude-opus-5-test":{"inputTokens":1,"InputTokens":9999,`+
					`"outputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0}}`, 1)
			result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
			assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
		})
	})

	// SI-182 test row (e): a known member of the wrong JSON type still
	// refuses the frame as invalid-foreign-field. This guards the tolerant
	// decode's removal of DisallowUnknownFields — the typed unmarshal is the
	// only thing left refusing a wrong-typed known member — so the mistyped
	// member is retry_delay_ms and nothing else. encoding/json allocates the
	// *uint64 before it reports the type error, so a swallowed decode error
	// leaves a non-nil zero behind, and retry_delay_ms is the one required
	// api_retry member whose value no later check reads: with the unmarshal
	// error swallowed this frame is ACCEPTED and this row goes red. (A
	// mistyped attempt would not bite — its surviving zero fails the
	// *frame.Attempt == 0 check and yields the same reason either way.)
	t.Run("retry_rejects_wrong_typed_known_field", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"system","subtype":"api_retry","attempt":1,"max_retries":3,"retry_delay_ms":"soon","error":{"type":"rate_limit","message":"slow down"},"uuid":"ru","session_id":"s1"}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad)
		assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
	})

	t.Run("malformed_frame_detail_is_digest_only", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		raw := `{not-json SENTINEL-FOREIGN-BYTES}`
		result := runClaudeLines(t, launch, envRoot, raw)
		gap := claudeGapPayload(t, result.Observations)
		if gap.ReasonCode != "malformed-foreign-frame" {
			t.Fatalf("reason = %q, want malformed-foreign-frame", gap.ReasonCode)
		}
		detail := result.Observations[0].ForeignDetail
		if detail.Mode != contextevent.DetailInline {
			t.Fatalf("malformed detail mode = %q, want inline", detail.Mode)
		}
		const want = `{"raw_digest":"` + claudeMalformedFrameRawDigest + `","reason":"malformed-foreign-frame"}`
		if string(detail.RedactedJSON) != want {
			t.Fatalf("malformed detail = %s, want %s", detail.RedactedJSON, want)
		}
		if bytes.Contains(detail.RedactedJSON, []byte("SENTINEL-FOREIGN-BYTES")) {
			t.Fatal("malformed detail disclosed raw foreign bytes")
		}
	})

	t.Run("truncated_final_line_uses_malformed_foreign_frame", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: []byte(`{"type":"system"`)}
		result := runClaudeProcess(t, launch, envRoot, pp)
		assertClaudeGapReason(t, result, "malformed-foreign-frame", "decode", claudeSource)
	})

	t.Run("success_result_with_stderr_is_terminal_failure", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{
			version: launch.Request.AdapterVersion,
			output:  mustClaudeFixture(t, "claude-start.jsonl", launch.Workspace.Path),
			stderr:  []byte("panic: SENTINEL-STDERR-BYTES\n"),
		}
		result := runClaudeProcess(t, launch, envRoot, pp)
		if result.OperationalFailure != "provider-stderr" {
			t.Fatalf("operational failure = %q, want provider-stderr", result.OperationalFailure)
		}
		for _, obs := range result.Observations {
			if summary, ok := obs.Payload.(*contextevent.ProviderSummaryPayload); ok && summary.SummaryID == "terminal-result" {
				t.Fatal("nonempty stderr must not admit the success terminal-result summary")
			}
			if bytes.Contains(obs.ForeignDetail.RedactedJSON, []byte("SENTINEL-STDERR-BYTES")) {
				t.Fatal("diagnostics disclosed stderr bytes")
			}
		}
		gap := claudeGapPayload(t, result.Observations)
		if gap.Source != "claude-process" || gap.ReasonCode != "provider-stderr" {
			t.Fatalf("process gap = %+v, want source claude-process reason provider-stderr", gap)
		}
		errPayload := claudeErrorPayload(t, result.Observations)
		if errPayload.Operation != "process" || errPayload.ReasonCode != "provider-stderr" {
			t.Fatalf("adapter-error = operation %q reason %q, want process/provider-stderr", errPayload.Operation, errPayload.ReasonCode)
		}
		stop := claudeStopPayload(t, result.Observations)
		if stop.ReasonCode != "provider-stderr" {
			t.Fatalf("adapter-stop reason = %q, want provider-stderr", stop.ReasonCode)
		}
		// §5 fixes the only safe stderr detail to exactly {raw_digest,reason}
		// over the discarded stderr bytes, and fixes the kind literals.
		const wantStderrDetail = `{"raw_digest":"` + claudeStderrRawDigest + `","reason":"provider-stderr"}`
		var sawGap, sawError bool
		for _, obs := range result.Observations {
			switch string(obs.Kind) {
			case "telemetry-gap":
				sawGap = true
			case "adapter-error":
				sawError = true
			default:
				continue
			}
			if string(obs.ForeignDetail.RedactedJSON) != wantStderrDetail {
				t.Fatalf("%s stderr detail = %s, want exactly %s", obs.Kind, obs.ForeignDetail.RedactedJSON, wantStderrDetail)
			}
		}
		if !sawGap || !sawError {
			t.Fatalf("kinds = %v, want both telemetry-gap and adapter-error", observationKindsC(result.Observations))
		}
		if errPayload.ErrorDigest != claudeStderrFixedDetailDigest {
			t.Fatalf("error_digest = %q, want %q over the exact fixed safe-detail bytes", errPayload.ErrorDigest, claudeStderrFixedDetailDigest)
		}
	})

	t.Run("nonzero_exit_uses_provider_exit_nonzero_at_result_sequence", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{
			version:  launch.Request.AdapterVersion,
			output:   mustClaudeFixture(t, "claude-start.jsonl", launch.Workspace.Path),
			exitCode: 7,
		}
		result := runClaudeProcess(t, launch, envRoot, pp)
		if result.OperationalFailure != "provider-exit-nonzero" {
			t.Fatalf("operational failure = %q, want provider-exit-nonzero", result.OperationalFailure)
		}
		gap := claudeGapPayload(t, result.Observations)
		if gap.Source != "claude-process" || gap.FromSequence != 3 || gap.ToSequence != 3 {
			t.Fatalf("process gap = %+v, want source claude-process range 3..3 (terminal result sequence)", gap)
		}
		errPayload := claudeErrorPayload(t, result.Observations)
		if errPayload.Operation != "process" {
			t.Fatalf("adapter-error operation = %q, want process", errPayload.Operation)
		}
		stop := claudeStopPayload(t, result.Observations)
		if stop.ExitCode != 7 || stop.ReasonCode != "provider-exit-nonzero" {
			t.Fatalf("adapter-stop = %+v, want exit 7 reason provider-exit-nonzero", stop)
		}
	})

	t.Run("stderr_outranks_nonzero_exit", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{
			version:  launch.Request.AdapterVersion,
			output:   mustClaudeFixture(t, "claude-start.jsonl", launch.Workspace.Path),
			exitCode: 7,
			stderr:   []byte("boom\n"),
		}
		result := runClaudeProcess(t, launch, envRoot, pp)
		if result.OperationalFailure != "provider-stderr" {
			t.Fatalf("operational failure = %q, want provider-stderr (stderr outranks exit)", result.OperationalFailure)
		}
		if claudeStopPayload(t, result.Observations).ExitCode != 7 {
			t.Fatal("adapter-stop must carry the actual exit code")
		}
	})

	t.Run("missing_terminal_result_gap_uses_next_foreign_sequence", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path))
		if result.OperationalFailure != "missing-terminal-result" {
			t.Fatalf("operational failure = %q, want missing-terminal-result", result.OperationalFailure)
		}
		gap := claudeGapPayload(t, result.Observations)
		if gap.Source != "claude-process" || gap.FromSequence != 2 || gap.ToSequence != 2 {
			t.Fatalf("process gap = %+v, want source claude-process range 2..2 (next foreign sequence)", gap)
		}
		if claudeErrorPayload(t, result.Observations).Operation != "decode" {
			t.Fatal("missing-terminal-result adapter-error operation must be decode")
		}
	})

	t.Run("eof_with_incomplete_tool_call_is_operational", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		toolUse := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"tool_use","id":"call_1","name":"Read","input":{"path":"README.md"}}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLines(t, launch, envRoot,
			claudeInitLine("s1", launch.Workspace.Path), toolUse, claudeResultLine("s1", "success", false))
		if result.OperationalFailure != "incomplete-tool-call" {
			t.Fatalf("operational failure = %q, want incomplete-tool-call", result.OperationalFailure)
		}
		for _, obs := range result.Observations {
			if summary, ok := obs.Payload.(*contextevent.ProviderSummaryPayload); ok && summary.SummaryID == "terminal-result" {
				t.Fatal("an incomplete tool call must never admit the success terminal-result summary")
			}
		}
		if claudeErrorPayload(t, result.Observations).Operation != "decode" {
			t.Fatal("incomplete-tool-call adapter-error operation must be decode")
		}
	})

	// SI-185: the real Claude Code CLI 2.1.261 emits one assistant frame per
	// content block, every frame of one message carrying the same
	// message.id (F12 canary flight 5). Frames sharing one id are the
	// successive blocks of that one message — the block index continues
	// across them, so fixed ids stay <message-id>:<block-index> and unique
	// across the run. The only remaining message-id contradiction is a
	// frame naming a message id a later, different message id has already
	// closed.
	// A message id already closed by a later, different id stays refused
	// on the closed id alone, whether the reappearing frame's content
	// happens to repeat the closed message's own block (this row) or
	// offers a genuinely new one (its sibling, interleaving) — the resent
	// block is renumbered to a fresh index like any other, so this is not
	// detection of a repeated (message id, block index) pair.
	t.Run("duplicate_message_id_is_refused", func(t *testing.T) {
		for name, reopenedText := range map[string]string{
			"resends_the_closed_messages_own_block": "hi",
			"offers_a_fresh_block_instead":          "hi-again",
		} {
			t.Run(name, func(t *testing.T) {
				launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
				first := `{"type":"assistant","session_id":"s1","uuid":"mu-1","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
				closesIt := `{"type":"assistant","session_id":"s1","uuid":"mu-2","message":{"id":"msg_2","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"other"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
				reopened := `{"type":"assistant","session_id":"s1","uuid":"mu-3","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"` + reopenedText + `"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
				result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), first, closesIt, reopened)
				assertClaudeGapReason(t, result, "duplicate-message-id", "decode", claudeSource)
			})
		}
	})

	t.Run("repeated_message_id_without_interleaving_continues_the_message", func(t *testing.T) {
		// The flight-5 bug this ticket fixes: two consecutive frames
		// sharing one message id, with no other id between them, are the
		// successive blocks of one message — not a contradiction — even
		// when the second frame's block is byte-identical to the first.
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		first := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_same","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		second := `{"type":"assistant","session_id":"s1","uuid":"mu-2","message":{"id":"msg_same","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), first, second)
		if result.OperationalFailure != "missing-terminal-result" {
			t.Fatalf("operational failure = %q, want missing-terminal-result (both blocks accepted, no duplicate)", result.OperationalFailure)
		}
		var ids []string
		for _, obs := range result.Observations {
			if payload, ok := obs.Payload.(*contextevent.ProviderMessagePayload); ok {
				ids = append(ids, payload.MessageID)
			}
		}
		if len(ids) != 2 || ids[0] != "msg_same:0" || ids[1] != "msg_same:1" {
			t.Fatalf("provider-message ids = %v, want [msg_same:0 msg_same:1]", ids)
		}
	})

	t.Run("assistant_message_blocks_continue_across_frames_sharing_one_id", func(t *testing.T) {
		// F12 canary flight 5's exact shape: a thinking block (frame 1,
		// block 0) followed by a text block (frame 2, block 1) of the same
		// message, both frames carrying message.id msg_multi.
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		thinkingFrame := `{"type":"assistant","session_id":"s1","uuid":"mu-1","message":{"id":"msg_multi","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"thinking","thinking":"hidden","signature":"sig"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		textFrame := `{"type":"assistant","session_id":"s1","uuid":"mu-2","message":{"id":"msg_multi","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLinesToTerminal(t, launch, envRoot,
			claudeInitLine("s1", launch.Workspace.Path), thinkingFrame, textFrame, claudeResultLine("s1", "success", false))
		thinkingSummary := claudeFindProviderSummary(t, result.Observations, "msg_multi:0")
		if string(thinkingSummary.ForeignDetail.RedactedJSON) != claudeThinkingOmissionDetail {
			t.Fatalf("continued thinking detail = %s, want %s", thinkingSummary.ForeignDetail.RedactedJSON, claudeThinkingOmissionDetail)
		}
		message := claudeFindKind(t, result.Observations, contextevent.KindProviderMessage)
		payload, ok := message.Payload.(*contextevent.ProviderMessagePayload)
		if !ok {
			t.Fatalf("provider-message payload type = %T", message.Payload)
		}
		if payload.MessageID != "msg_multi:1" {
			t.Fatalf("continued text message id = %q, want msg_multi:1", payload.MessageID)
		}
		const wantDetail = `{"block_index":1,"family":"assistant/text","message_id":"msg_multi","text":"hi"}`
		if string(message.ForeignDetail.RedactedJSON) != wantDetail {
			t.Fatalf("continued text detail = %s, want %s", message.ForeignDetail.RedactedJSON, wantDetail)
		}
	})

	t.Run("assistant_message_continues_through_a_third_frame_with_tool_use", func(t *testing.T) {
		// A third frame sharing the same message id continues to block
		// index 2, tool_use included.
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		thinkingFrame := `{"type":"assistant","session_id":"s1","uuid":"mu-1","message":{"id":"msg_three","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"thinking","thinking":"hidden","signature":"sig"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		textFrame := `{"type":"assistant","session_id":"s1","uuid":"mu-2","message":{"id":"msg_three","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"about to read"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		toolUseFrame := `{"type":"assistant","session_id":"s1","uuid":"mu-3","message":{"id":"msg_three","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"tool_use","id":"call_three","name":"Read","input":{"path":"README.md"}}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), thinkingFrame, textFrame, toolUseFrame)
		toolCall := claudeFindKind(t, result.Observations, contextevent.KindToolCall)
		payload, ok := toolCall.Payload.(*contextevent.ToolCallPayload)
		if !ok {
			t.Fatalf("tool-call payload type = %T", toolCall.Payload)
		}
		if payload.CallID != "call_three" {
			t.Fatalf("tool-call id = %q, want call_three", payload.CallID)
		}
		if !bytes.Contains(toolCall.ForeignDetail.RedactedJSON, []byte(`"block_index":2`)) {
			t.Fatalf("third-frame tool_use detail = %s, want block_index 2", toolCall.ForeignDetail.RedactedJSON)
		}
	})

	t.Run("assistant_message_with_two_blocks_then_one_more_continues_the_index", func(t *testing.T) {
		// A frame may carry more than one new block at once: two blocks in
		// frame 1 (indices 0,1) then one more in frame 2 (index 2).
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		twoBlocks := `{"type":"assistant","session_id":"s1","uuid":"mu-1","message":{"id":"msg_pair","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"thinking","thinking":"hidden","signature":"sig"},{"type":"text","text":"first"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		oneMore := `{"type":"assistant","session_id":"s1","uuid":"mu-2","message":{"id":"msg_pair","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"second"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), twoBlocks, oneMore)
		claudeFindProviderSummary(t, result.Observations, "msg_pair:0")
		var ids []string
		for _, obs := range result.Observations {
			if payload, ok := obs.Payload.(*contextevent.ProviderMessagePayload); ok {
				ids = append(ids, payload.MessageID)
			}
		}
		if len(ids) != 2 || ids[0] != "msg_pair:1" || ids[1] != "msg_pair:2" {
			t.Fatalf("provider-message ids = %v, want [msg_pair:1 msg_pair:2]", ids)
		}
	})

	t.Run("duplicate_tool_result_is_refused", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		toolUse := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"tool_use","id":"call_1","name":"Read","input":{"path":"README.md"}}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		toolResult := `{"type":"user","session_id":"s1","uuid":"tu","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":"ok"}]}}`
		result := runClaudeLines(t, launch, envRoot,
			claudeInitLine("s1", launch.Workspace.Path), toolUse, toolResult, toolResult)
		assertClaudeGapReason(t, result, "duplicate-tool-result", "decode", claudeSource)
	})

	// Amendment 002 §6 / I-109 / SI-174: a provider-derived string bound for a
	// fixed payload field is checked against the run's complete classified set
	// before the event is built. A match is refused with the closed
	// `protected-fixed-field` reduction — the fixed field is never rewritten,
	// the value never enters a diagnostic, detail, state map, or durable
	// payload, and no normalized observation carrying it is produced.
	t.Run("classified_secret_in_a_fixed_provider_field_is_refused", func(t *testing.T) {
		// The exact key claudeTestLaunch activates as PolicySecretValues.
		const apiKey = "test-api-key-1234567890"
		safeToolUse := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"tool_use","id":"call_1","name":"Read","input":{"path":"README.md"}}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		assistantWith := func(blocks string) string {
			return `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"MSGID","type":"message","role":"assistant","model":"claude-opus-5-test","content":[` + blocks +
				`],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		}
		for name, frames := range map[string][]string{
			// The assistant message id lands in the fixed provider-message id.
			"assistant_message_id": {
				strings.Replace(assistantWith(`{"type":"text","text":"hi"}`), "MSGID", "msg_"+apiKey, 1),
			},
			// The same id derives the omission summary's fixed summary id.
			"omission_summary_id": {
				strings.Replace(assistantWith(`{"type":"thinking","thinking":"hidden","signature":"sig"}`), "MSGID", "msg_"+apiKey, 1),
			},
			// The tool-use call id lands in the fixed tool-call id.
			"tool_use_call_id": {
				strings.Replace(assistantWith(`{"type":"tool_use","id":"call_`+apiKey+`","name":"Read","input":{"path":"README.md"}}`), "MSGID", "msg_1", 1),
			},
			// The tool name lands in the fixed tool-call name.
			"tool_use_tool_name": {
				strings.Replace(assistantWith(`{"type":"tool_use","id":"call_1","name":"Read`+apiKey+`","input":{"path":"README.md"}}`), "MSGID", "msg_1", 1),
			},
			// The tool-result identity is refused before it is even probed
			// against the open-call state map, so its reduction is the fixed
			// protected refusal rather than an unmatched-call verdict.
			"tool_result_identity": {
				safeToolUse,
				`{"type":"user","session_id":"s1","uuid":"tu","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_` + apiKey + `","content":"ok"}]}}`,
			},
		} {
			t.Run(name, func(t *testing.T) {
				launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
				lines := append([]string{claudeInitLine("s1", launch.Workspace.Path)}, frames...)
				result := runClaudeLines(t, launch, envRoot, lines...)
				// Containment first: no normalized observation — fixed field,
				// detail, witness, or diagnostic — may carry the value.
				assertNoClaudePlaintext(t, result.Observations, apiKey)
				assertClaudeGapReason(t, result, "protected-fixed-field", "redaction", claudeSource)
			})
		}

		// The same classified set leaves every safe fixed byte untouched.
		t.Run("safe_identities_keep_their_exact_bytes", func(t *testing.T) {
			launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
			toolResult := `{"type":"user","session_id":"s1","uuid":"tu","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":"ok"}]}}`
			result := runClaudeLines(t, launch, envRoot,
				claudeInitLine("s1", launch.Workspace.Path), safeToolUse, toolResult,
				claudeResultLine("s1", "success", false))
			if result.OperationalFailure != "" {
				t.Fatalf("safe identities were refused: %q", result.OperationalFailure)
			}
			call, callFound := (*contextevent.ToolCallPayload)(nil), false
			out, outFound := (*contextevent.ToolResultPayload)(nil), false
			for _, obs := range result.Observations {
				switch payload := obs.Payload.(type) {
				case *contextevent.ToolCallPayload:
					call, callFound = payload, true
				case *contextevent.ToolResultPayload:
					out, outFound = payload, true
				}
			}
			if !callFound || !outFound {
				t.Fatalf("safe stream lost its tool observations: %v", observationKindsC(result.Observations))
			}
			if call.CallID != "call_1" || call.ToolName != "Read" || out.CallID != "call_1" || out.ToolName != "Read" {
				t.Fatalf("safe fixed identities = %q/%q and %q/%q, want the exact provider bytes",
					call.CallID, call.ToolName, out.CallID, out.ToolName)
			}
		})
	})

	t.Run("user_non_tool_result_block_is_refused", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		userText := `{"type":"user","session_id":"s1","uuid":"tu","message":{"role":"user","content":[{"type":"text","text":"prose"}]}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), userText)
		assertClaudeGapReason(t, result, "unknown-content-block", "decode", claudeSource)
	})

	t.Run("terminal_result_rejects_unknown_subtype", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		result := runClaudeLines(t, launch, envRoot,
			claudeInitLine("s1", launch.Workspace.Path), claudeResultLine("s1", "error_unlisted", true))
		assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
	})

	t.Run("terminal_result_rejects_missing_permission_denials", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeResultLine("s1", "success", false), `,"permission_denials":[]`, ``, 1)
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
		assertClaudeGapReason(t, result, "missing-foreign-field", "decode", claudeSource)
	})

	t.Run("terminal_result_rejects_foreign_model_usage_key", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeResultLine("s1", "success", false), `,"permission_denials":[]`,
			`,"permission_denials":[],"modelUsage":{"some-other-model":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}`, 1)
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), line)
		assertClaudeGapReason(t, result, "invalid-foreign-field", "decode", claudeSource)
	})

	t.Run("safe_detail_failure_propagates_operational_error", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		// A classified secret colliding with a fixed safe-detail object key
		// makes the fixed reduction itself unrepresentable.
		launch.Profile.PolicySecretValues = append(launch.Profile.PolicySecretValues, []byte("reason"))
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: []byte("{not-json}\n")}
		dp := newTestProcessor(t)
		adapter, err := newClaudeTestAdapter(t, pp, dp, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		got, err := run.Next(context.Background())
		if err == nil {
			t.Fatalf("Next must propagate the lower-layer failure, got result %+v", got)
		}
		if len(got.Observations) != 0 {
			t.Fatal("no replacement detail or observation may be fabricated")
		}
	})

	// Fold: environment-table and forbidden-name mutations — Amendment 002 §3.
	// These prove the closed environment table before the process boundary.
	envMutations := []struct {
		name   string
		mutate func(map[string]string)
	}{
		{"missing_claude_config_dir", func(e map[string]string) { delete(e, "CLAUDE_CONFIG_DIR") }},
		{"unbound_claude_config_dir", func(e map[string]string) { e["CLAUDE_CONFIG_DIR"] = "/other/claude-config" }},
		{"relative_claude_config_dir", func(e map[string]string) { e["CLAUDE_CONFIG_DIR"] = "relative/claude" }},
		{"missing_autoupdater_control", func(e map[string]string) { delete(e, "DISABLE_AUTOUPDATER") }},
		{"wrong_autoupdater_control", func(e map[string]string) { e["DISABLE_AUTOUPDATER"] = "0" }},
		{"missing_nonessential_traffic_control", func(e map[string]string) {
			delete(e, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC")
		}},
		{"wrong_nonessential_traffic_control", func(e map[string]string) {
			e["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = "true"
		}},
		{"missing_ide_control", func(e map[string]string) { delete(e, "CLAUDE_CODE_AUTO_CONNECT_IDE") }},
		{"wrong_ide_control", func(e map[string]string) { e["CLAUDE_CODE_AUTO_CONNECT_IDE"] = "true" }},
		{"forbidden_extra_anthropic_name", func(e map[string]string) { e["ANTHROPIC_MODEL"] = "claude-alias" }},
		{"forbidden_extra_claude_name", func(e map[string]string) { e["CLAUDE_CODE_USE_BEDROCK"] = "1" }},
		{"forbidden_cloud_provider_name", func(e map[string]string) { e["AWS_PROFILE"] = "default" }},
		{"forbidden_proxy_name", func(e map[string]string) { e["HTTPS_PROXY"] = "http://127.0.0.1:8080" }},
		{"forbidden_lowercase_proxy_name", func(e map[string]string) { e["https_proxy"] = "http://127.0.0.1:8080" }},
		{"forbidden_shell_startup_name", func(e map[string]string) { e["BASH_ENV"] = "/tmp/startup.sh" }},
		{"forbidden_ide_name", func(e map[string]string) { e["VSCODE_PID"] = "1" }},
		{"forbidden_plugin_name", func(e map[string]string) { e["MY_PLUGIN_DIR"] = "/tmp/plugins" }},
		{"forbidden_hook_name", func(e map[string]string) { e["PRE_TOOL_HOOK"] = "/tmp/hook.sh" }},
		{"forbidden_telemetry_export_name", func(e map[string]string) { e["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://127.0.0.1:4317" }},
		{"forbidden_model_selection_name", func(e map[string]string) { e["DEFAULT_MODEL_ID"] = "claude-alias" }},
	}
	for _, tc := range envMutations {
		tc := tc
		t.Run("environment_mutation_"+tc.name, func(t *testing.T) {
			launch, envRoot := claudeTestLaunchEnv(t, sealedexec.ActionStart, tc.mutate)
			pp := &testProbeProcess{version: launch.Request.AdapterVersion}
			adapter, err := newClaudeTestAdapter(t, pp, newTestProcessor(t), envRoot)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if _, err := adapter.Start(context.Background(), launch); err == nil {
				t.Fatalf("Start with %s = nil, want refusal", tc.name)
			}
			if pp.probeCmd != nil || pp.startCmd != nil {
				t.Fatalf("%s reached the process boundary before validation", tc.name)
			}
		})
	}

	// Amendment 002 §5 holds the exact success result until process termination
	// and §7 makes the reaped child emit the advisory `provider-summary` then
	// `adapter-stop`. The shared U4 stream loop refuses any non-terminal
	// AdapterResult carrying no normalized observation
	// (internal/sealedexec/service.go, "non-terminal result has no normalized
	// observations"), so the buffered success frame must never reach Service as
	// its own observation-free result.
	t.Run("buffered_success_result_never_surfaces_an_empty_nonterminal", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-start.jsonl", launch.Workspace.Path)}
		adapter, err := newClaudeTestAdapter(t, pp, newTestProcessor(t), envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		var last sealedexec.AdapterResult
		for step := 0; ; step++ {
			result, err := run.Next(context.Background())
			if err != nil {
				t.Fatalf("Next[%d]: %v", step, err)
			}
			if result.Terminal == nil && result.Stopped == nil && len(result.Observations) == 0 {
				t.Fatalf("Next[%d] returned an observation-free non-terminal result %#v; the shared service refuses that shape", step, result)
			}
			last = result
			if result.Terminal != nil || result.Stopped != nil {
				break
			}
		}
		// The reaped success emits exactly the advisory terminal-result summary
		// followed by adapter-stop with exit 0 and reason "completed".
		if last.OperationalFailure != "" {
			t.Fatalf("successful reap operational failure = %q, want none", last.OperationalFailure)
		}
		if got := observationKindsC(last.Observations); !reflect.DeepEqual(got, []contextevent.Kind{contextevent.KindProviderSummary, contextevent.KindAdapterStop}) {
			t.Fatalf("terminal observation kinds = %v, want [provider-summary adapter-stop]", got)
		}
		summary, ok := last.Observations[0].Payload.(*contextevent.ProviderSummaryPayload)
		if !ok || summary.SummaryID != "terminal-result" || summary.Authority != contextevent.AuthorityAdvisory {
			t.Fatalf("terminal summary payload = %#v, want advisory terminal-result", last.Observations[0].Payload)
		}
		stop := claudeStopPayload(t, last.Observations)
		if stop.ExitCode != 0 || stop.ReasonCode != "completed" {
			t.Fatalf("adapter-stop = exit %d reason %q, want exit 0 reason completed", stop.ExitCode, stop.ReasonCode)
		}
		if last.Terminal == nil || last.Terminal.ExitCode != 0 {
			t.Fatalf("terminal = %#v, want exit 0", last.Terminal)
		}
	})

	// Segment-reason witness: Amendment 002 §5's closed table maps a segment
	// store/resolve/mismatch failure to `segment-store-failed` /
	// `segment-resolve-failed` / `segment-mismatch` over the `segment`
	// operation, and a redaction inability to `redaction-failed` over
	// `redaction`. The call site must read the processor's typed failure
	// category — never an error string — so the reachable pair is exact. This
	// row fails both if the store is never reached and if the mapping changes.
	t.Run("segment_store_failure_maps_to_segment_operation", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		errStore := &errorSegmentStore{err: errors.New("segment-store-failed: simulated")}
		proc, err := sealedexec.NewDetailProcessor(errStore)
		if err != nil {
			t.Fatalf("NewDetailProcessor: %v", err)
		}
		// A genuine start stream whose assistant text exceeds the fixed
		// 16,384-byte inline ceiling, so its projected detail must be stored as
		// a controller segment and the failing store is actually reached.
		pp := &testProbeProcess{
			version: launch.Request.AdapterVersion,
			output:  claudeOversizedStartStream(t, launch.Workspace.Path),
		}
		adapter, err := newClaudeTestAdapter(t, pp, proc, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		// Collect until terminal. The row fails unless the expected adapter-error
		// is actually observed: a stream that never reaches the segment store
		// would otherwise pass vacuously.
		observedReasons := []string{}
		segmentObserved := false
		for {
			result, err := run.Next(context.Background())
			if err != nil {
				break
			}
			for _, obs := range result.Observations {
				if obs.Kind != contextevent.KindAdapterError {
					continue
				}
				payload, ok := obs.Payload.(*contextevent.AdapterErrorPayload)
				if !ok {
					t.Fatalf("adapter-error payload = %T, want *contextevent.AdapterErrorPayload", obs.Payload)
				}
				observedReasons = append(observedReasons, payload.ReasonCode+"/"+payload.Operation)
				if payload.ReasonCode == "segment-store-failed" || payload.ReasonCode == "redaction-failed" {
					segmentObserved = true
					if payload.ReasonCode+"/"+payload.Operation != claudeSegmentStoreFailureBoundary {
						t.Errorf("segment-store failure reason/operation = %q, want %q",
							payload.ReasonCode+"/"+payload.Operation, claudeSegmentStoreFailureBoundary)
					}
				}
			}
			if result.Terminal != nil {
				break
			}
		}
		if !segmentObserved {
			t.Fatalf("segment-store failure never produced its adapter-error; observed reasons = %v", observedReasons)
		}
	})

	// A store that acknowledges different bytes than it was handed is a segment
	// mismatch, not a store failure and not a redaction inability.
	t.Run("segment_store_contradiction_maps_to_segment_mismatch", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		proc, err := sealedexec.NewDetailProcessor(&contradictingSegmentStore{})
		if err != nil {
			t.Fatalf("NewDetailProcessor: %v", err)
		}
		pp := &testProbeProcess{
			version: launch.Request.AdapterVersion,
			output:  claudeOversizedStartStream(t, launch.Workspace.Path),
		}
		adapter, err := newClaudeTestAdapter(t, pp, proc, envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		observedReasons := []string{}
		mismatchObserved := false
		for {
			result, err := run.Next(context.Background())
			if err != nil {
				break
			}
			for _, obs := range result.Observations {
				if obs.Kind != contextevent.KindAdapterError {
					continue
				}
				payload, ok := obs.Payload.(*contextevent.AdapterErrorPayload)
				if !ok {
					t.Fatalf("adapter-error payload = %T, want *contextevent.AdapterErrorPayload", obs.Payload)
				}
				observedReasons = append(observedReasons, payload.ReasonCode+"/"+payload.Operation)
				if payload.ReasonCode == "segment-mismatch" || payload.ReasonCode == "redaction-failed" || payload.ReasonCode == "segment-store-failed" {
					mismatchObserved = true
					if got := payload.ReasonCode + "/" + payload.Operation; got != "segment-mismatch/segment" {
						t.Errorf("segment contradiction reason/operation = %q, want %q", got, "segment-mismatch/segment")
					}
				}
			}
			if result.Terminal != nil {
				break
			}
		}
		if !mismatchObserved {
			t.Fatalf("segment contradiction never produced its adapter-error; observed reasons = %v", observedReasons)
		}
	})

	// I-112/Amendment 002 §9: this frozen AC-1 behavioral producer owns the built
	// candidate binary. Building the binary here proves it compiles; the sealed
	// Claude start/resume surface itself is driven by the named cmd/verdi rows,
	// which host the compiled runway, FD-3 controller, scoped MCP server, and
	// fake Claude executable that cannot be reconstructed inside this package
	// without duplicating that fixture. Executing them here — and failing on any
	// missing or failing named row — keeps the evidence in one frozen producer
	// and adds no eighth producer.
	t.Run("built_binary_claude_sealed_start_and_resume", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipped in short mode: binary build required")
		}
		if bin := claudeTestBuildBinary(t); bin == "" {
			t.Fatal("candidate binary path is empty")
		}
		rows := []string{
			"sealed_start_drives_the_public_claude_assembly",
			"sealed_resume_drives_the_public_claude_assembly",
			"oversized_detail_is_stored_as_a_controller_segment",
			"pristine_checkpoint_contradicting_the_expansion_ledger_is_refused",
			"undeclared_scoped_tool_ends_the_run_operationally",
			"missing_api_key_refuses_with_the_exact_safe_classification_diagnostic",
		}
		command := exec.Command("go", "test", "./cmd/verdi",
			"-run", "^TestClaudeBuiltBinaryLifecycle_Behavioral$/^("+strings.Join(rows, "|")+")$",
			"-count=1", "-timeout=600s", "-v")
		command.Dir = claudeTestModuleRoot(t)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("built-binary claude lifecycle rows failed: %v\n%s", err, out)
		}
		for _, row := range rows {
			if !bytes.Contains(out, []byte("--- PASS: TestClaudeBuiltBinaryLifecycle_Behavioral/"+row)) {
				t.Fatalf("named built-binary row %q did not run:\n%s", row, out)
			}
		}
	})

	// I-112 and the Amendment 002 coverage witness: the accepted promotion
	// inventory is 33/33 source units and 39/39 destination kinds. The tables
	// below are the literal accepted inventory; the destination column is bound
	// to the shared contextevent constants, so a removed, renamed, duplicated,
	// or added kind fails this producer rather than a prose citation.
	t.Run("amendment_002_coverage_totals", func(t *testing.T) {
		if got := len(amendment002SourceUnits); got != 33 {
			t.Fatalf("Amendment 002 source coverage = %d/33 units", got)
		}
		seenSource := map[int]struct{}{}
		for _, unit := range amendment002SourceUnits {
			if unit.number < 1 || unit.number > 33 {
				t.Fatalf("source unit number %d is outside 1..33", unit.number)
			}
			if _, duplicate := seenSource[unit.number]; duplicate {
				t.Fatalf("source unit %d is listed twice", unit.number)
			}
			seenSource[unit.number] = struct{}{}
			if unit.name == "" || unit.destination == "" {
				t.Fatalf("source unit %d has no name or destination: %#v", unit.number, unit)
			}
		}
		if got := len(amendment002DestinationKinds); got != 39 {
			t.Fatalf("Amendment 002 destination coverage = %d/39 kinds", got)
		}
		seenKind := map[contextevent.Kind]struct{}{}
		for kind, owner := range amendment002DestinationKinds {
			if kind == "" || owner == "" {
				t.Fatalf("destination kind %q has no normalization owner", kind)
			}
			if _, duplicate := seenKind[kind]; duplicate {
				t.Fatalf("destination kind %q is listed twice", kind)
			}
			seenKind[kind] = struct{}{}
		}
	})
}

// amendment002SourceUnit is one row of the accepted Amendment 002 source
// coverage and losslessness witness (33/33 units, zero silent omissions).
type amendment002SourceUnit struct {
	number      int
	name        string
	destination string
}

// amendment002SourceUnits is the literal accepted 33-unit source inventory:
// 14 frozen story clauses, 7 frozen obligations, and 12 directly inherited
// authority units.
var amendment002SourceUnits = []amendment002SourceUnit{
	{1, "problem", "Amendment §§1, 5-7"},
	{2, "outcome", "Amendment §§1, 7-10"},
	{3, "AC-1", "Amendment §§3-5, 9"},
	{4, "AC-2", "Amendment §7; inherited I-63/I-95/I-96"},
	{5, "AC-3", "Amendment §5 and the destination table"},
	{6, "AC-4", "Amendment §§6-8"},
	{7, "DC-1", "Amendment §§2-4, 9"},
	{8, "DC-2", "the destination table"},
	{9, "DC-3", "Amendment §7"},
	{10, "DC-4", "Amendment §§2, 6, 8"},
	{11, "DC-5", "Amendment §5"},
	{12, "CO-1", "Amendment §§3, 6"},
	{13, "CO-2", "Amendment §§3-8"},
	{14, "CO-3", "Amendment §9"},
	{15, "AC-1 static obligation", "Amendment §§3-4 and §9 producer row 1"},
	{16, "AC-1 behavioral obligation", "Amendment §§5, 7 and §9 producer row 2, including built binary"},
	{17, "AC-2 static obligation", "Amendment §7 and §9 producer row 3"},
	{18, "AC-2 behavioral obligation", "Amendment §7 and §9 producer row 4"},
	{19, "AC-3 static obligation", "the destination table and §9 producer row 5"},
	{20, "AC-3 behavioral obligation", "Amendment §5, the destination table, and §9 producer row 6"},
	{21, "AC-4 behavioral obligation", "Amendment §§6-8 and §9 producer row 7"},
	{22, "I-63 separate source/VATC order and receipt cutoff", "Amendment §7"},
	{23, "I-64 accepted detail media/profile union", "Amendment §6; root I-109"},
	{24, "I-70 sealed Codex process/provider-input boundary", "Amendment §4; root I-107"},
	{25, "I-71 pinned foreign JSONL pattern", "Amendment §§3-5"},
	{26, "I-75 receipt detail/finalization", "Amendment §§6, 8"},
	{27, "I-80 exact controller registry", "Amendment §6; root I-109"},
	{28, "I-82 receipt digest domains", "Amendment §8"},
	{29, "I-86 active-revision checkpoint", "Amendment §7; root I-110"},
	{30, "I-88 durable partial preservation", "Amendment §7"},
	{31, "I-95 event/ack pairing", "Amendment §7"},
	{32, "I-96 interleaved VATC globals", "Amendment §7"},
	{33, "I-102 adapter-start detail and shared result", "Amendment §5"},
}

// amendment002DestinationKinds is the literal accepted 39-kind destination
// ownership table. Keys are the shared registry constants, so the inventory
// cannot silently drift from the event vocabulary it claims to cover.
var amendment002DestinationKinds = map[contextevent.Kind]string{
	contextevent.KindFlightPlan:            "shared sealed execution",
	contextevent.KindInstructionProjection: "shared sealed execution",
	contextevent.KindChildManifest:         "scoped MCP/shared expansion",
	contextevent.KindPrompt:                "Claude/Codex adapter",
	contextevent.KindProviderMessage:       "Claude/Codex adapter",
	contextevent.KindProviderSummary:       "Claude/Codex adapter",
	contextevent.KindToolCall:              "Claude/Codex adapter",
	contextevent.KindToolResult:            "Claude/Codex adapter",
	contextevent.KindRead:                  "shared workspace observer",
	contextevent.KindWrite:                 "shared workspace observer",
	contextevent.KindEditDenied:            "shared workspace/grant guard",
	contextevent.KindContextRequest:        "scoped MCP",
	contextevent.KindContextDecision:       "scoped MCP/controller",
	contextevent.KindClaimRequest:          "shared VATC claim client",
	contextevent.KindClaimDecision:         "shared VATC claim client",
	contextevent.KindClaimWait:             "shared VATC claim client",
	contextevent.KindClaimRelease:          "shared VATC claim client",
	contextevent.KindCommand:               "shared execution observer",
	contextevent.KindTest:                  "shared gate/test observer",
	contextevent.KindResource:              "shared execution observer",
	contextevent.KindTimeout:               "shared execution observer",
	contextevent.KindGitStatus:             "shared repository observer",
	contextevent.KindGitDiff:               "shared repository observer",
	contextevent.KindGitCommit:             "shared repository observer",
	contextevent.KindForgeChange:           "shared forge observer",
	contextevent.KindGateInput:             "shared gate service",
	contextevent.KindGateVerdict:           "shared gate service",
	contextevent.KindWitness:               "shared evidence service",
	contextevent.KindFlightPlanDeviation:   "shared policy service",
	contextevent.KindAdjudication:          "shared policy service",
	contextevent.KindExecutionResult:       "shared completion service",
	contextevent.KindReceipt:               "shared receipt service/controller",
	contextevent.KindRetry:                 "provider adapter",
	contextevent.KindResume:                "adapter/shared lifecycle",
	contextevent.KindSuspension:            "shared interruption lifecycle",
	contextevent.KindTelemetryGap:          "adapter/shared lifecycle",
	contextevent.KindAdapterStart:          "provider adapter",
	contextevent.KindAdapterStop:           "provider adapter",
	contextevent.KindAdapterError:          "provider adapter",
}

// claudeSegmentStoreFailureBoundary is the exact Amendment 002 §5
// reason/operation pair a failed controller segment store must produce.
const claudeSegmentStoreFailureBoundary = "segment-store-failed/segment"

// claudeOversizedStartStream returns the committed start fixture with its
// assistant text expanded past the fixed 16,384-byte inline detail ceiling.
func claudeOversizedStartStream(t *testing.T, workspace string) []byte {
	t.Helper()
	lines := bytes.Split(bytes.TrimSuffix(mustClaudeFixture(t, "claude-start.jsonl", workspace), []byte{'\n'}), []byte{'\n'})
	if len(lines) != 3 {
		t.Fatalf("claude-start.jsonl has %d frames, want 3", len(lines))
	}
	const marker = "Analysis complete."
	if !bytes.Contains(lines[1], []byte(marker)) {
		t.Fatalf("claude-start.jsonl assistant frame no longer carries %q", marker)
	}
	lines[1] = bytes.Replace(lines[1], []byte(marker), bytes.Repeat([]byte("a"), 16500), 1)
	return append(bytes.Join(lines, []byte{'\n'}), '\n')
}

// errorSegmentStore is a segment store stub that always returns an error, used
// to prove segment-store failures collapse to the "segment" operation.
type errorSegmentStore struct{ err error }

func (s *errorSegmentStore) StoreRedactedSegment(_ context.Context, _ sealedexec.RedactedSegment) (sealedexec.StoredSegment, error) {
	return sealedexec.StoredSegment{}, s.err
}
func (s *errorSegmentStore) ResolveRedactedSegment(_ context.Context, _ string) (sealedexec.RedactedSegment, error) {
	return sealedexec.RedactedSegment{}, s.err
}

// contradictingSegmentStore acknowledges a segment whose stored facts differ
// from the bytes it was handed, which Amendment 002 §5 reduces to
// "segment-mismatch" over the "segment" operation.
type contradictingSegmentStore struct{}

func (s *contradictingSegmentStore) StoreRedactedSegment(_ context.Context, seg sealedexec.RedactedSegment) (sealedexec.StoredSegment, error) {
	return sealedexec.StoredSegment{
		Schema:           testSegmentSchemaStored,
		Reference:        testSegmentRefPrefix + strings.TrimPrefix(seg.Digest, "sha256:"),
		MediaType:        seg.MediaType,
		RedactionProfile: seg.RedactionProfile,
		Digest:           seg.Digest,
		ByteCount:        seg.ByteCount + 1,
	}, nil
}

func (s *contradictingSegmentStore) ResolveRedactedSegment(_ context.Context, _ string) (sealedexec.RedactedSegment, error) {
	return sealedexec.RedactedSegment{}, errors.New("contradictingSegmentStore: resolve is not used")
}

// claudeTestBuildBinary builds ./cmd/verdi from the module root and returns
// the path of the compiled binary. The binary is built into t.TempDir so it
// is cleaned up automatically. Build results are NOT cached across sub-tests.
func claudeTestBuildBinary(t *testing.T) string {
	t.Helper()
	root := claudeTestModuleRoot(t)
	bin := filepath.Join(t.TempDir(), "verdi")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/verdi")
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("building verdi binary: %v\n%s", err, out.String())
	}
	return bin
}

// claudeTestModuleRoot resolves the verdi module root from this file's compiled
// path, independent of the test binary's working directory.
func claudeTestModuleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate module root")
	}
	// This file: <moduleRoot>/internal/sealedexec/claude/adapter_test.go
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolving module root from %s: %v", file, err)
	}
	return root
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// mustClaudeFixture loads a committed fixture and binds its deterministic
// "/workspace" cwd placeholder to the launch's real execution workspace, which
// Amendment 002 §5 requires init to observe exactly. The recorded init
// inventory is never rewritten: the captures themselves record Amendment 003's
// dual inventory, so every fixture-driven assertion runs against the exact
// committed provider bytes. `claude_fixtures_record_the_committed_dual_inventory`
// pins both halves of that property.
func mustClaudeFixture(t *testing.T, name, workspace string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return bytes.ReplaceAll(data, []byte(`"cwd":"/workspace"`), []byte(`"cwd":"`+workspace+`"`))
}

// claudeFixtureInventory is the exact accepted two-row init inventory, in the
// canonical provider observation order the three committed captures record.
const claudeFixtureInventory = `"mcp_servers":[{"name":"vatc","status":"connected"},{"name":"verdi-context","status":"connected"}]`

// claudeFixtureNames is every committed provider capture the adapter tests
// consume.
func claudeFixtureNames() []string {
	return []string{"claude-start.jsonl", "claude-resume.jsonl", "claude-advisory.jsonl"}
}

// claudeTestLaunch builds an AdapterLaunch for the claude adapter in tests.
// Returns the launch and the envRoot (HOME directory) for path assertions.
func claudeTestLaunch(t *testing.T, action sealedexec.Action) (sealedexec.AdapterLaunch, string) {
	t.Helper()
	return claudeTestLaunchEnv(t, action, nil)
}

// claudeTestLaunchEnv builds the same launch as claudeTestLaunch and applies
// mutate to the Amendment 002 §3 environment table before activation.
func claudeTestLaunchEnv(t *testing.T, action sealedexec.Action, mutate func(map[string]string)) (sealedexec.AdapterLaunch, string) {
	t.Helper()
	envRoot := t.TempDir()
	workspace := filepath.Join(envRoot, "data", "execution", "workspace-1")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	executable := "/usr/bin/claude-test"
	claudeConfigDir := filepath.Join(envRoot, ".config", "claude")
	if err := os.MkdirAll(claudeConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir claude config: %v", err)
	}
	apiKey := "test-api-key-1234567890" // >= 8 bytes
	grants := execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantNetwork},
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{executable}},
		{Kind: execworkspace.GrantTimeouts, Seconds: 30},
	}}
	declaredEnv := map[string]string{
		"ANTHROPIC_API_KEY":   apiKey,
		"CLAUDE_CONFIG_DIR":   claudeConfigDir,
		"PATH":                "/usr/bin:/bin",
		"DISABLE_AUTOUPDATER": "1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"CLAUDE_CODE_AUTO_CONNECT_IDE":             "false",
	}
	if mutate != nil {
		mutate(declaredEnv)
	}
	profile, report, err := execworkspace.BuildProfile(workspace, envRoot, grants, declaredEnv)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	projection := sealedexec.InstructionProjection{
		Schema: sealedexec.InstructionProjectionSchemaID,
		Files:  []sealedexec.InstructionFile{{Path: "AGENTS.md", Content: "sealed\n", ContentDigest: claudeTestDigest([]byte("sealed\n"))}},
	}
	projBytes, err := sealedexec.EncodeInstructionProjection(projection)
	if err != nil {
		t.Fatalf("EncodeInstructionProjection: %v", err)
	}
	projection, err = sealedexec.DecodeInstructionProjection(bytes.NewReader(projBytes))
	if err != nil {
		t.Fatalf("DecodeInstructionProjection: %v", err)
	}
	_, encoded, err := contextcompile.BuildDataItem(
		contextcompile.Candidate{ID: "path:README.md", Source: contextcompile.SourceHeadTree, Path: "README.md"},
		contextcompile.IncludedRepositoryFile, []byte("CONTEXT DATA"))
	if err != nil {
		t.Fatalf("BuildDataItem: %v", err)
	}
	item, err := contextcompile.DecodeDataItem(encoded)
	if err != nil {
		t.Fatalf("DecodeDataItem: %v", err)
	}
	adapterVersion := "1.2.3"
	request := sealedexec.ExecutionRequest{
		Action:         action,
		Adapter:        contextevent.AdapterClaude,
		AdapterVersion: adapterVersion,
		Session:        "verdi-session-1",
		Profile:        sealedexec.LogicalRef{Digest: claudeTestDigest([]byte("claude-profile"))},
	}
	return sealedexec.AdapterLaunch{
		Request: request,
		Profile: sealedexec.ResolvedProfile{
			Verification:           sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
			Ref:                    request.Profile,
			Digest:                 request.Profile.Digest,
			Name:                   "sealed-project",
			Model:                  "claude-opus-5-test",
			ClaudeConfigDir:        claudeConfigDir,
			Executable:             executable,
			AdapterVersion:         adapterVersion,
			DecoderProfile:         DecoderProfileV1,
			WorkspacePath:          workspace,
			Profile:                profile,
			Grants:                 grants,
			Enforcement:            *report,
			PolicySecretValues:     [][]byte{[]byte(apiKey)},
			ClassificationComplete: true,
		},
		Workspace: sealedexec.WorkspaceFacts{
			Verification:  sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
			WorkspaceID:   "workspace-1",
			Path:          workspace,
			RequestDigest: claudeTestDigest([]byte("workspace-request")),
			CurrentCommit: "1111111111111111111111111111111111111111",
			CurrentTree:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Clean:         true,
		},
		Input: sealedexec.ProviderInput{
			Instructions: sealedexec.InstructionAuthority{Projection: projection},
			Data:         []contextcompile.DataItem{item},
		},
	}, envRoot
}

// claudeTestMCPConfig is the exact Task 3 scoped configuration a per-command
// adapter receives; the adapter never derives it from HOME.
func claudeTestMCPConfig(envRoot string) MCPConfig {
	return MCPConfig{
		Path:    filepath.Join(envRoot, claudeMCPConfigName),
		Servers: claudeTestMCPServers(),
	}
}

// claudeTestMCPServers is Amendment 003's exact pair of required registrations
// with fixed ports and capabilities, so configuration bytes are comparable.
func claudeTestMCPServers() sealedexec.RequiredMCPSet {
	return sealedexec.RequiredMCPSet{
		Claim:   claudeTestClaimMCP(),
		Context: sealedexec.RequiredMCP{Name: "verdi-context", URL: "http://127.0.0.1:54321/mcp", Authorization: "Bearer sha256:" + strings.Repeat("a", 64), Tools: []string{"get_flight_plan", "request_context"}},
	}
}

// claudeTestClaimMCP is the ATC-owned registration the controller resolves.
func claudeTestClaimMCP() sealedexec.RequiredMCP {
	return sealedexec.RequiredMCP{Name: "vatc", URL: "http://127.0.0.1:54322/mcp", Authorization: "Bearer sha256:" + strings.Repeat("b", 64), Tools: []string{"claim_paths"}}
}

func newClaudeTestAdapter(t *testing.T, process Process, processor *sealedexec.DetailProcessor, envRoot string) (*Adapter, error) {
	t.Helper()
	return New(process, processor, claudeTestMCPConfig(envRoot))
}

func claudeTestDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func observationKindsC(rows []sealedexec.NormalizedObservation) []contextevent.Kind {
	out := make([]contextevent.Kind, len(rows))
	for i, row := range rows {
		out[i] = row.Kind
	}
	return out
}

func hasKindC(rows []sealedexec.NormalizedObservation, kind contextevent.Kind) bool {
	for _, row := range rows {
		if row.Kind == kind {
			return true
		}
	}
	return false
}

func blocksAuthorityC(rows []sealedexec.NormalizedObservation) bool {
	for _, row := range rows {
		if row.BlocksAuthority {
			return true
		}
	}
	return false
}

func containsStr(rows []string, value string) bool {
	for _, row := range rows {
		if row == value {
			return true
		}
	}
	return false
}

func envValueStr(env []string, name string) string {
	prefix := name + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix)
		}
	}
	return ""
}

func collectClaudeRun(t *testing.T, run sealedexec.ActiveAdapterRun) sealedexec.AdapterResult {
	t.Helper()
	var collected sealedexec.AdapterResult
	for {
		result, err := run.Next(context.Background())
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		mergeClaudeResult(&collected, result)
		if result.Terminal != nil {
			return collected
		}
	}
}

func collectClaudeRunAll(t *testing.T, run sealedexec.ActiveAdapterRun) sealedexec.AdapterResult {
	t.Helper()
	var collected sealedexec.AdapterResult
	for {
		result, err := run.Next(context.Background())
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		mergeClaudeResult(&collected, result)
		if result.Terminal != nil || result.Stopped != nil {
			return collected
		}
	}
}

func collectClaudeUntilBoundary(t *testing.T, run sealedexec.ActiveAdapterRun) sealedexec.AdapterResult {
	t.Helper()
	var collected sealedexec.AdapterResult
	for {
		result, err := run.Next(context.Background())
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		mergeClaudeResult(&collected, result)
		if result.Terminal != nil || result.OperationalFailure != "" || blocksAuthorityC(result.Observations) {
			return collected
		}
	}
}

func mergeClaudeResult(target *sealedexec.AdapterResult, result sealedexec.AdapterResult) {
	if result.ObservedSessionRef != "" {
		target.ObservedSessionRef = result.ObservedSessionRef
	}
	target.Observations = append(target.Observations, result.Observations...)
	if target.OperationalFailure == "" {
		target.OperationalFailure = result.OperationalFailure
	}
	if result.Terminal != nil {
		target.Terminal = result.Terminal
	}
	if result.Stopped != nil {
		target.Stopped = result.Stopped
	}
}

// ---------------------------------------------------------------------------
// Fake process implementations
// ---------------------------------------------------------------------------

// testProbeProcess simulates the Claude process for tests.
// Probe returns the configured version; Start records the command and streams output.
type testProbeProcess struct {
	mu             sync.Mutex
	probeCmd       *exec.Cmd
	startCmd       *exec.Cmd
	startStdin     []byte
	version        string
	emptyProbeLine bool
	// secondLine appends an extra stdout line after version (SI-181 review
	// F1): the probe must refuse whenever it prints more than the one
	// required line, even when that first line is itself an accepted form.
	secondLine bool
	output     []byte
	stderr     []byte
	exitCode   int
	err        error
	run        *testClaudeActiveProcess
}

func (p *testProbeProcess) Probe(_ context.Context, cmd *exec.Cmd) (stdout, stderr []byte, exitCode int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.probeCmd = cmd
	if p.err != nil {
		return nil, nil, 1, p.err
	}
	if p.emptyProbeLine {
		// A well-formed probe: exit 0, empty stderr, one empty stdout line.
		return []byte("\n"), nil, 0, nil
	}
	if p.version == "" {
		return nil, nil, 1, errors.New("testProbeProcess: no version configured")
	}
	out := p.version
	if p.secondLine {
		out += "\nunexpected-second-line"
	}
	return []byte(out + "\n"), nil, 0, nil
}

func (p *testProbeProcess) Start(_ context.Context, cmd *exec.Cmd, stdin []byte) (ActiveProcess, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startCmd = cmd
	p.startStdin = append([]byte(nil), stdin...)
	if p.err != nil {
		return nil, p.err
	}
	p.run = &testClaudeActiveProcess{
		observations: claudeTestObservations(p.output),
		stderr:       append([]byte(nil), p.stderr...),
		exitCode:     p.exitCode,
	}
	return p.run, nil
}

type testClaudeActiveProcess struct {
	mu           sync.Mutex
	observations []ProcessObservation
	stderr       []byte
	exitCode     int
	terminal     bool
}

func (p *testClaudeActiveProcess) Next(_ context.Context) (ProcessObservation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.observations) != 0 {
		obs := p.observations[0]
		p.observations = p.observations[1:]
		return obs, nil
	}
	if p.terminal {
		return ProcessObservation{}, errors.New("testClaudeActiveProcess: next after terminal")
	}
	p.terminal = true
	return ProcessObservation{Terminal: &ProcessResult{ExitCode: p.exitCode, Stderr: append([]byte(nil), p.stderr...)}}, nil
}

func (p *testClaudeActiveProcess) Stop(_ context.Context) (ProcessStopResult, error) {
	return ProcessStopResult{ExitCode: 130, ReasonCode: "interrupted"}, nil
}

func claudeTestObservations(output []byte) []ProcessObservation {
	if len(output) == 0 {
		return nil
	}
	complete := output[len(output)-1] == '\n'
	parts := bytes.Split(output, []byte("\n"))
	if complete {
		parts = parts[:len(parts)-1]
	}
	observations := make([]ProcessObservation, len(parts))
	for i, part := range parts {
		foreignJSON := make([]byte, len(part))
		copy(foreignJSON, part)
		observations[i] = ProcessObservation{ForeignJSON: foreignJSON, Complete: complete || i < len(parts)-1}
	}
	return observations
}

// ---------------------------------------------------------------------------
// Fake segment store for DetailProcessor
// ---------------------------------------------------------------------------

const testSegmentSchemaStored = "verdi.context-redacted-segment-stored/v1"
const testSegmentRefPrefix = "controller-segment/sha256/"

type testSegmentStore struct {
	mu       sync.Mutex
	segments map[string]sealedexec.RedactedSegment
}

func (s *testSegmentStore) StoreRedactedSegment(_ context.Context, seg sealedexec.RedactedSegment) (sealedexec.StoredSegment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.segments[seg.Digest] = seg
	ref := testSegmentRefPrefix + strings.TrimPrefix(seg.Digest, "sha256:")
	return sealedexec.StoredSegment{
		Schema:           testSegmentSchemaStored,
		Reference:        ref,
		MediaType:        seg.MediaType,
		RedactionProfile: seg.RedactionProfile,
		Digest:           seg.Digest,
		ByteCount:        seg.ByteCount,
	}, nil
}

func (s *testSegmentStore) ResolveRedactedSegment(_ context.Context, ref string) (sealedexec.RedactedSegment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	digest := "sha256:" + strings.TrimPrefix(ref, testSegmentRefPrefix)
	seg, ok := s.segments[digest]
	if !ok {
		return sealedexec.RedactedSegment{}, errors.New("segment not found: " + ref)
	}
	return seg, nil
}

func newTestProcessor(t *testing.T) *sealedexec.DetailProcessor {
	t.Helper()
	store := &testSegmentStore{segments: make(map[string]sealedexec.RedactedSegment)}
	proc, err := sealedexec.NewDetailProcessor(store)
	if err != nil {
		t.Fatalf("NewDetailProcessor: %v", err)
	}
	return proc
}

// TestClaudeClosedFrameShapeFieldNames pins jsonFieldNames, the single point
// that keeps SI-182's tolerant walk in sync with the strict typed decode. A
// field shape encoding/json reads under a name the walk does not know would
// invert SI-182 — the member would be recorded as unknown and still read — so
// the helper must skip exactly what encoding/json skips and refuse everything
// it cannot mirror, at construction.
func TestClaudeClosedFrameShapeFieldNames(t *testing.T) {
	t.Run("declared_names_are_known_and_skipped_fields_are_not", func(t *testing.T) {
		type shape struct {
			Named    *string `json:"named"`
			Optional string  `json:"optional,omitempty"`
			Skipped  string  `json:"-"`
			Dashed   string  `json:"-,"`
			hidden   string  //nolint:unused // proves an unexported field names no member
		}
		got := jsonFieldNames(reflect.TypeOf(shape{}))
		want := map[string]struct{}{"named": {}, "optional": {}, "-": {}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("jsonFieldNames = %v, want %v", got, want)
		}
		// The `json:"-"` field must not be known under its Go name either: a
		// member named "Skipped" is unknown, and the walk must say so.
		if _, known := got["Skipped"]; known {
			t.Fatal(`a json:"-" field must contribute no known member name`)
		}
	})

	// Negative path: every shape encoding/json would read under a name the
	// helper cannot derive is refused where the closed shapes are built.
	t.Run("unmirrorable_field_shapes_are_refused_at_construction", func(t *testing.T) {
		type promoted struct {
			Promoted string `json:"promoted"`
		}
		for _, row := range []struct {
			name string
			typ  reflect.Type
			want string
		}{
			{
				name: "untagged_exported_field",
				typ:  reflect.TypeOf(struct{ Untagged string }{}),
				want: "explicit json tag",
			},
			{
				name: "empty_json_name",
				typ: reflect.TypeOf(struct {
					Empty string `json:""`
				}{}),
				want: "explicit json name",
			},
			{
				name: "omitempty_without_a_name",
				typ: reflect.TypeOf(struct {
					Nameless string `json:",omitempty"`
				}{}),
				want: "explicit json name",
			},
			{
				name: "embedded_struct",
				typ:  reflect.TypeOf(struct{ promoted }{}),
				want: "must not embed a struct",
			},
		} {
			t.Run(row.name, func(t *testing.T) {
				defer func() {
					recovered := recover()
					message, ok := recovered.(string)
					if !ok {
						t.Fatalf("jsonFieldNames(%s) returned without refusing; recover = %v", row.name, recovered)
					}
					if !strings.Contains(message, row.want) {
						t.Fatalf("refusal = %q, want it to mention %q", message, row.want)
					}
				}()
				got := jsonFieldNames(row.typ)
				t.Fatalf("jsonFieldNames(%s) = %v, want a refusal", row.name, got)
			})
		}
	})

	// The seventeen closed shapes themselves must satisfy the invariant: they
	// are built at package initialization, so a violation would already have
	// panicked, but this row states the expectation the guard exists for.
	t.Run("every_closed_frame_shape_declares_only_mirrorable_fields", func(t *testing.T) {
		for _, typ := range []reflect.Type{
			reflect.TypeOf(claudeInitFrame{}), reflect.TypeOf(claudeMCPRow{}),
			reflect.TypeOf(claudeRetryFrame{}), reflect.TypeOf(claudeRetryError{}),
			reflect.TypeOf(claudeAssistantFrame{}), reflect.TypeOf(claudeAssistantMessage{}),
			reflect.TypeOf(claudeUsage{}), reflect.TypeOf(claudeModelUsage{}),
			reflect.TypeOf(claudeUserFrame{}),
			reflect.TypeOf(claudeUserMessage{}), reflect.TypeOf(claudeResultFrame{}),
			reflect.TypeOf(claudePermissionDenial{}), reflect.TypeOf(claudeTextBlock{}),
			reflect.TypeOf(claudeToolUseBlock{}), reflect.TypeOf(claudeThinkingBlock{}),
			reflect.TypeOf(claudeRedactedThinkingBlock{}), reflect.TypeOf(claudeToolResultBlock{}),
		} {
			names := jsonFieldNames(typ)
			if len(names) != typ.NumField() {
				t.Fatalf("%s: %d known member names for %d fields", typ.Name(), len(names), typ.NumField())
			}
		}
	})
}

// TestClaudeAdapterProfileAndCommandAuthority proves Amendment 002 §3/§4
// profile and command authority: the model comes from the resolved profile
// (never its logical name), the MCP configuration path is the exact
// per-command Task 3 value (never derived from HOME), the environment table
// is closed against missing rows and forbidden names, and the version probe
// runs the launch executable, environment, and working directory.
func TestClaudeAdapterProfileAndCommandAuthority(t *testing.T) {
	t.Run("argv_uses_resolved_model_and_supplied_mcp_path", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		// The supplied configuration deliberately lives outside the profile
		// HOME parent so a HOME-derived path cannot reproduce it.
		supplied := MCPConfig{
			Path:    filepath.Join(t.TempDir(), claudeMCPConfigName),
			Servers: claudeTestMCPServers(),
		}
		if supplied.Path == filepath.Join(envRoot, claudeMCPConfigName) {
			t.Fatal("fixture: supplied config path must differ from the env-root derivation")
		}
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		adapter, err := New(pp, newTestProcessor(t), supplied)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if _, err := adapter.Start(context.Background(), launch); err != nil {
			t.Fatalf("Start: %v", err)
		}
		wantArgs := []string{
			launch.Profile.Executable, "--bare", "-p",
			"--input-format", "stream-json", "--output-format", "stream-json", "--verbose",
			"--model", launch.Profile.Model,
			"--permission-mode", "bypassPermissions",
			"--strict-mcp-config", "--mcp-config", supplied.Path, "--no-chrome",
		}
		if !reflect.DeepEqual(pp.startCmd.Args, wantArgs) {
			t.Fatalf("start argv = %v, want %v", pp.startCmd.Args, wantArgs)
		}
		for _, arg := range pp.startCmd.Args {
			if arg == launch.Profile.Name {
				t.Fatalf("the logical profile name %q reached argv", launch.Profile.Name)
			}
		}
	})

	t.Run("resume_argv_uses_resolved_model_and_supplied_mcp_path", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionResume)
		supplied := claudeTestMCPConfig(envRoot)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		adapter, err := New(pp, newTestProcessor(t), supplied)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if _, err := adapter.Resume(context.Background(), launch, "claude-sess-resume-001"); err != nil {
			t.Fatalf("Resume: %v", err)
		}
		wantTail := []string{"--mcp-config", supplied.Path, "--no-chrome", "--resume", "claude-sess-resume-001"}
		got := pp.startCmd.Args[len(pp.startCmd.Args)-len(wantTail):]
		if !reflect.DeepEqual(got, wantTail) {
			t.Fatalf("resume argv tail = %v, want %v", got, wantTail)
		}
	})

	t.Run("missing_resolved_model_refuses_launch", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		launch.Profile.Model = ""
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		adapter, err := newClaudeTestAdapter(t, pp, newTestProcessor(t), envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if _, err := adapter.Start(context.Background(), launch); err == nil {
			t.Fatal("Start without a resolved model = nil, want refusal")
		}
		if pp.startCmd != nil {
			t.Fatal("a process was launched without a resolved model")
		}
	})

	// Amendment 003: the supplied configuration must be the exact scoped file
	// path plus exactly two separately owned registrations. Every path defect
	// and every per-server defect fails closed at construction.
	mcpConfigs := map[string]func(*MCPConfig){
		"zero_value":              func(c *MCPConfig) { *c = MCPConfig{} },
		"relative_path":           func(c *MCPConfig) { c.Path = "claude-mcp.json" },
		"unclean_path":            func(c *MCPConfig) { c.Path = "/tmp/../tmp/claude-mcp.json" },
		"wrong_basename":          func(c *MCPConfig) { c.Path = "/tmp/mcp.json" },
		"missing_context_url":     func(c *MCPConfig) { c.Servers.Context.URL = "" },
		"missing_claim_url":       func(c *MCPConfig) { c.Servers.Claim.URL = "" },
		"missing_authorization":   func(c *MCPConfig) { c.Servers.Context.Authorization = "" },
		"unscoped_authorization":  func(c *MCPConfig) { c.Servers.Claim.Authorization = "Bearer opaque-token" },
		"short_capability_digest": func(c *MCPConfig) { c.Servers.Context.Authorization = "Bearer sha256:" + strings.Repeat("a", 63) },
		"missing_claim_row":       func(c *MCPConfig) { c.Servers.Claim = sealedexec.RequiredMCP{} },
		"missing_context_row":     func(c *MCPConfig) { c.Servers.Context = sealedexec.RequiredMCP{} },
		"renamed_claim_row":       func(c *MCPConfig) { c.Servers.Claim.Name = "vatc-shadow" },
		"renamed_context_row":     func(c *MCPConfig) { c.Servers.Context.Name = "verdi_context" },
		"duplicated_row":          func(c *MCPConfig) { c.Servers.Claim = c.Servers.Context },
		"shared_capability":       func(c *MCPConfig) { c.Servers.Claim.Authorization = c.Servers.Context.Authorization },
		"shared_origin":           func(c *MCPConfig) { c.Servers.Claim.URL = c.Servers.Context.URL },
		"overlapping_catalogues":  func(c *MCPConfig) { c.Servers.Context.Tools = []string{"get_flight_plan", "claim_paths"} },
		"non_loopback_origin":     func(c *MCPConfig) { c.Servers.Claim.URL = "http://198.51.100.7:1/mcp" },
		"url_fragment":            func(c *MCPConfig) { c.Servers.Context.URL = "http://127.0.0.1:1/mcp#f" },
	}
	for name, mutate := range mcpConfigs {
		t.Run("mcp_config_rejected_"+name, func(t *testing.T) {
			config := claudeTestMCPConfig(t.TempDir())
			mutate(&config)
			if _, err := New(&testProbeProcess{}, newTestProcessor(t), config); err == nil {
				t.Fatalf("New with %s MCP config = nil, want refusal", name)
			}
		})
	}

	t.Run("version_probe_shares_launch_executable_environment_and_directory", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		adapter, err := newClaudeTestAdapter(t, pp, newTestProcessor(t), envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if _, err := adapter.Start(context.Background(), launch); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if pp.probeCmd.Dir != launch.Workspace.Path {
			t.Fatalf("probe working directory = %q, want the launch workspace %q", pp.probeCmd.Dir, launch.Workspace.Path)
		}
		if pp.probeCmd.Dir != pp.startCmd.Dir {
			t.Fatalf("probe directory %q != launch directory %q", pp.probeCmd.Dir, pp.startCmd.Dir)
		}
		if pp.probeCmd.Path != pp.startCmd.Path {
			t.Fatalf("probe executable %q != launch executable %q", pp.probeCmd.Path, pp.startCmd.Path)
		}
		if !reflect.DeepEqual(pp.probeCmd.Env, pp.startCmd.Env) {
			t.Fatal("probe environment differs from the launch environment")
		}
	})

	environments := []struct {
		name   string
		mutate func(map[string]string)
	}{
		{"missing_claude_config_dir", func(e map[string]string) { delete(e, "CLAUDE_CONFIG_DIR") }},
		{"unbound_claude_config_dir", func(e map[string]string) { e["CLAUDE_CONFIG_DIR"] = "/other/claude-config" }},
		{"relative_claude_config_dir", func(e map[string]string) { e["CLAUDE_CONFIG_DIR"] = "relative/claude" }},
		{"missing_path", func(e map[string]string) { delete(e, "PATH") }},
		{"empty_path", func(e map[string]string) { e["PATH"] = "" }},
		{"relative_path_entry", func(e map[string]string) { e["PATH"] = "/usr/bin:tools" }},
		{"empty_path_entry", func(e map[string]string) { e["PATH"] = "/usr/bin:" }},
		{"missing_autoupdater_control", func(e map[string]string) { delete(e, "DISABLE_AUTOUPDATER") }},
		{"wrong_autoupdater_control", func(e map[string]string) { e["DISABLE_AUTOUPDATER"] = "0" }},
		{"missing_nonessential_traffic_control", func(e map[string]string) {
			delete(e, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC")
		}},
		{"wrong_nonessential_traffic_control", func(e map[string]string) {
			e["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = "true"
		}},
		{"missing_ide_control", func(e map[string]string) { delete(e, "CLAUDE_CODE_AUTO_CONNECT_IDE") }},
		{"wrong_ide_control", func(e map[string]string) { e["CLAUDE_CODE_AUTO_CONNECT_IDE"] = "true" }},
		{"forbidden_extra_anthropic_name", func(e map[string]string) { e["ANTHROPIC_MODEL"] = "claude-alias" }},
		{"forbidden_extra_claude_name", func(e map[string]string) { e["CLAUDE_CODE_USE_BEDROCK"] = "1" }},
		{"forbidden_cloud_provider_name", func(e map[string]string) { e["AWS_PROFILE"] = "default" }},
		{"forbidden_proxy_name", func(e map[string]string) { e["HTTPS_PROXY"] = "http://127.0.0.1:8080" }},
		{"forbidden_lowercase_proxy_name", func(e map[string]string) { e["https_proxy"] = "http://127.0.0.1:8080" }},
		{"forbidden_shell_startup_name", func(e map[string]string) { e["BASH_ENV"] = "/tmp/startup.sh" }},
		{"forbidden_ide_name", func(e map[string]string) { e["VSCODE_PID"] = "1" }},
		{"forbidden_plugin_name", func(e map[string]string) { e["MY_PLUGIN_DIR"] = "/tmp/plugins" }},
		{"forbidden_hook_name", func(e map[string]string) { e["PRE_TOOL_HOOK"] = "/tmp/hook.sh" }},
		{"forbidden_telemetry_export_name", func(e map[string]string) { e["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://127.0.0.1:4317" }},
		{"forbidden_model_selection_name", func(e map[string]string) { e["DEFAULT_MODEL_ID"] = "claude-alias" }},
	}
	for _, tc := range environments {
		t.Run("environment_"+tc.name, func(t *testing.T) {
			launch, envRoot := claudeTestLaunchEnv(t, sealedexec.ActionStart, tc.mutate)
			pp := &testProbeProcess{version: launch.Request.AdapterVersion}
			adapter, err := newClaudeTestAdapter(t, pp, newTestProcessor(t), envRoot)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if _, err := adapter.Start(context.Background(), launch); err == nil {
				t.Fatalf("Start with %s = nil, want refusal", tc.name)
			}
			if pp.probeCmd != nil || pp.startCmd != nil {
				t.Fatalf("%s reached the process boundary before validation", tc.name)
			}
		})
	}

	t.Run("baseline_environment_names_remain_admitted", func(t *testing.T) {
		launch, envRoot := claudeTestLaunchEnv(t, sealedexec.ActionStart, func(e map[string]string) {
			e["LANG"] = "C"
			e["SOURCE_DATE_EPOCH"] = "0"
		})
		adapter, err := newClaudeTestAdapter(t, &testProbeProcess{version: launch.Request.AdapterVersion}, newTestProcessor(t), envRoot)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if _, err := adapter.Start(context.Background(), launch); err != nil {
			t.Fatalf("Start with deterministic baseline names = %v, want nil", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Wave B literal line builders and assertions
// ---------------------------------------------------------------------------

// claudeInitLine is the exact Amendment 002 §5 system/init accepted-key set,
// written as an independent literal rather than produced by the decoder.
func claudeInitLine(session, cwd string) string {
	return `{"type":"system","subtype":"init","session_id":"` + session +
		`","model":"claude-opus-5-test","mcp_servers":[{"name":"vatc","status":"connected"},{"name":"verdi-context","status":"connected"}],"cwd":"` + cwd +
		`","tools":["Task"],"permissionMode":"bypassPermissions","apiKeySource":"ANTHROPIC_API_KEY","claude_code_version":"1.2.3","slash_commands":[],"output_style":"default","agents":[],"skills":[],"plugins":[],"uuid":"u-init"}`
}

// claudeResultLine is the exact terminal result accepted-key set.
func claudeResultLine(session, subtype string, isError bool) string {
	return `{"type":"result","subtype":"` + subtype + `","is_error":` + strconv.FormatBool(isError) +
		`,"result":"done","session_id":"` + session +
		`","uuid":"u-result","duration_ms":10,"duration_api_ms":9,"num_turns":1,"total_cost_usd":0.001,"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1},"permission_denials":[]}`
}

// runClaudeLines streams the exact LF-delimited lines and collects until the
// first authority boundary or terminal.
func runClaudeLines(t *testing.T, launch sealedexec.AdapterLaunch, envRoot string, lines ...string) sealedexec.AdapterResult {
	t.Helper()
	var output []byte
	for _, line := range lines {
		output = append(output, line...)
		output = append(output, '\n')
	}
	return runClaudeProcess(t, launch, envRoot, &testProbeProcess{version: launch.Request.AdapterVersion, output: output})
}

// runClaudeLinesToTerminal streams the exact lines and collects through the
// reaped terminal, so the held success terminal-result summary is included.
func runClaudeLinesToTerminal(t *testing.T, launch sealedexec.AdapterLaunch, envRoot string, lines ...string) sealedexec.AdapterResult {
	t.Helper()
	var output []byte
	for _, line := range lines {
		output = append(output, line...)
		output = append(output, '\n')
	}
	pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: output}
	adapter, err := newClaudeTestAdapter(t, pp, newTestProcessor(t), envRoot)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	run, err := adapter.Start(context.Background(), launch)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return collectClaudeRun(t, run)
}

func runClaudeProcess(t *testing.T, launch sealedexec.AdapterLaunch, envRoot string, pp *testProbeProcess) sealedexec.AdapterResult {
	t.Helper()
	adapter, err := newClaudeTestAdapter(t, pp, newTestProcessor(t), envRoot)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	run, err := adapter.Start(context.Background(), launch)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return collectClaudeUntilBoundary(t, run)
}

func claudeGapPayload(t *testing.T, rows []sealedexec.NormalizedObservation) *contextevent.TelemetryGapPayload {
	t.Helper()
	for _, obs := range rows {
		if payload, ok := obs.Payload.(*contextevent.TelemetryGapPayload); ok {
			return payload
		}
	}
	t.Fatalf("no telemetry-gap observation in %v", observationKindsC(rows))
	return nil
}

func claudeErrorPayload(t *testing.T, rows []sealedexec.NormalizedObservation) *contextevent.AdapterErrorPayload {
	t.Helper()
	for _, obs := range rows {
		if payload, ok := obs.Payload.(*contextevent.AdapterErrorPayload); ok {
			return payload
		}
	}
	t.Fatalf("no adapter-error observation in %v", observationKindsC(rows))
	return nil
}

func claudeStopPayload(t *testing.T, rows []sealedexec.NormalizedObservation) *contextevent.AdapterStopPayload {
	t.Helper()
	for _, obs := range rows {
		if payload, ok := obs.Payload.(*contextevent.AdapterStopPayload); ok {
			return payload
		}
	}
	t.Fatalf("no adapter-stop observation in %v", observationKindsC(rows))
	return nil
}

// claudeFindProviderSummary returns the one provider-summary observation with
// the exact summary id (SI-182 tests inspect its detail for the disclosure).
func claudeFindProviderSummary(t *testing.T, rows []sealedexec.NormalizedObservation, summaryID string) sealedexec.NormalizedObservation {
	t.Helper()
	for _, obs := range rows {
		if payload, ok := obs.Payload.(*contextevent.ProviderSummaryPayload); ok && payload.SummaryID == summaryID {
			return obs
		}
	}
	t.Fatalf("no provider-summary %q in %v", summaryID, observationKindsC(rows))
	return sealedexec.NormalizedObservation{}
}

// claudeFindKind returns the first observation of kind (SI-182 tests inspect
// its detail for the disclosure).
func claudeFindKind(t *testing.T, rows []sealedexec.NormalizedObservation, kind contextevent.Kind) sealedexec.NormalizedObservation {
	t.Helper()
	for _, obs := range rows {
		if obs.Kind == kind {
			return obs
		}
	}
	t.Fatalf("no %s observation in %v", kind, observationKindsC(rows))
	return sealedexec.NormalizedObservation{}
}

// assertNoClaudePlaintext proves the classified value appears nowhere in the
// emitted observations: not in a fixed payload field, not in a detail, and not
// in a witness or diagnostic string. It serializes each observation's complete
// payload and detail rather than inspecting selected fields.
func assertNoClaudePlaintext(t *testing.T, rows []sealedexec.NormalizedObservation, secret string) {
	t.Helper()
	for i, obs := range rows {
		encoded, err := json.Marshal(struct {
			Kind    contextevent.Kind
			Payload any
			Detail  contextevent.Detail
			Witness string
		}{Kind: obs.Kind, Payload: obs.Payload, Detail: obs.ForeignDetail, Witness: obs.Witness})
		if err != nil {
			t.Fatalf("marshal observation %d: %v", i, err)
		}
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatalf("observation %d (%s) carries the classified value in plaintext: %s", i, obs.Kind, encoded)
		}
	}
}

func assertClaudeGapReason(t *testing.T, result sealedexec.AdapterResult, reason, operation, source string) {
	t.Helper()
	if result.OperationalFailure != reason {
		t.Fatalf("operational failure = %q, want %q", result.OperationalFailure, reason)
	}
	if !blocksAuthorityC(result.Observations) {
		t.Fatalf("%s must block authority: %v", reason, observationKindsC(result.Observations))
	}
	gap := claudeGapPayload(t, result.Observations)
	if gap.ReasonCode != reason || gap.Source != source {
		t.Fatalf("telemetry-gap = reason %q source %q, want %q/%q", gap.ReasonCode, gap.Source, reason, source)
	}
	errPayload := claudeErrorPayload(t, result.Observations)
	if errPayload.ReasonCode != reason || errPayload.Operation != operation {
		t.Fatalf("adapter-error = reason %q operation %q, want %q/%q", errPayload.ReasonCode, errPayload.Operation, reason, operation)
	}
}

// ---------------------------------------------------------------------------
// I7: independent literal byte oracles.
//
// Every constant below is transcribed from Amendment 002 §4/§5 and hashed with
// an out-of-process tool; none is produced by canonjson, the provider-input
// encoder, or any other production code under test.
// ---------------------------------------------------------------------------

// claudeSealedInputMarker is §4's exact sealed-input prefix.
const claudeSealedInputMarker = "VERDI_SEALED_PROVIDER_INPUT_V1\n"

// claudeExpectedProviderInput is the exact canonical
// verdi.sealed-provider-input/v1 document for claudeTestLaunch's input,
// without its trailing LF.
const claudeExpectedProviderInput = `{"data":[{"classification":"non-authoritative-data","content":"CONTEXT DATA","content_digest":"sha256:12a3dc6b79a66a834aed37dfc11d649b18b74941176aef52d50653aaaf8e2abd","digest":"sha256:b973228560c1dd662f3da4151b7c57c4a67728328c1202f550e36d84e24386c8","id":"path:README.md","kind":"repository-file","path":"README.md","schema":"verdi.context-data-item/v1","source":"head-tree"}],"instructions":{"instruction_projection":{"digest":"sha256:0f0039804d2cf11a17f0299a1bf9e8ff633f6c1866e125e2d3e8859f1ccd4e3e","files":[{"content":"sealed\n","content_digest":"sha256:24f2f924f16716eeae930dfc7ca01dd50e4b58754997d9ac3c7e630a0c9d3b71","path":"AGENTS.md"}],"schema":"verdi.instruction-projection/v1"}},"schema":"verdi.sealed-provider-input/v1"}`

// claudeQuoteJSONString is a deliberately independent minimal JSON string
// escaper. It exists so the stdin oracle never routes through the encoder it
// is meant to police; it covers exactly the characters the sealed input can
// contain.
func claudeQuoteJSONString(value string) string {
	var out []byte
	for i := 0; i < len(value); i++ {
		switch c := value[i]; c {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		default:
			if c < 0x20 {
				panic("claudeQuoteJSONString: unexpected control byte in oracle input")
			}
			out = append(out, c)
		}
	}
	return string(out)
}

// claudeAssistantTextDetail is §5's exact assistant-text detail source D for
// the committed start fixture. SHA-256 over these 100 bytes, computed with an
// out-of-process shasum, is claudeAssistantTextDigest.
const claudeAssistantTextDetail = `{"block_index":0,"family":"assistant/text","message_id":"msg_001","text":"Analysis complete."}`
const claudeAssistantTextDigest = "sha256:5f7d796ec32d0d3397919563bb24c25f5d59c2ce3d653706608010c261313200"

// claudeThinkingOmissionDetail is §5's exact thinking/redacted-thinking detail
// source D. Hidden bytes never appear in it.
const claudeThinkingOmissionDetail = `{"content_type":"thinking","omitted":true}`
const claudeThinkingOmissionDigest = "sha256:0ddb70430062cc063da194de71119fc48335d399c9cc6ffe22c31050a761ebb6"

// claudeInitSummaryDetail is §5's exact init detail source I for the committed
// start fixture, with the observed provider session already redacted by §6.
const claudeInitSummaryDetail = `{"family":"system/init","mcp_servers":[{"name":"vatc","status":"connected"},{"name":"verdi-context","status":"connected"}],"model":"claude-opus-5-test","permission_mode":"bypassPermissions","session_id":"[REDACTED]"}`

// Amendment 003 ratchet: the digest of the exact two-row init projection above.
const claudeInitSummaryDigest = "sha256:46dcd234620ebd3679a7c82a9a2de8e58f7e73aa542f2814e941d5473a9cb195"

// claudeCleanResultDetail is §5's exact terminal-result detail source for
// claudeResultLine("s1", "success", false) with no unknown member anywhere.
// SI-182's rebuild of usage / permission_denials / modelUsage from the typed
// decode must leave these bytes untouched.
const claudeCleanResultDetail = `{"duration_api_ms":9,"duration_ms":10,"family":"result","is_error":false,"num_turns":1,"permission_denials":[],"result":"done","subtype":"success","total_cost_usd":0.001,"usage":{"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"input_tokens":1,"output_tokens":1}}`

// assertClaudeResultDetail proves the terminal-result detail is exactly want
// and that both the payload's hashed summary digest and the detail's own
// digest are SHA-256 over those same bytes. Anything absent from want was
// therefore neither projected nor hashed.
func assertClaudeResultDetail(t *testing.T, result sealedexec.AdapterResult, want string) {
	t.Helper()
	summary := claudeFindProviderSummary(t, result.Observations, "terminal-result")
	if got := string(summary.ForeignDetail.RedactedJSON); got != want {
		t.Fatalf("result detail = %s, want %s", got, want)
	}
	payload, ok := summary.Payload.(*contextevent.ProviderSummaryPayload)
	if !ok {
		t.Fatalf("terminal summary payload = %#v, want a provider-summary payload", summary.Payload)
	}
	digest := claudeTestDigest([]byte(want))
	if payload.SummaryDigest != digest || summary.ForeignDetail.Digest != digest {
		t.Fatalf("summary digest = %q, detail digest = %q, want %q over the exact detail bytes",
			payload.SummaryDigest, summary.ForeignDetail.Digest, digest)
	}
}

// claudeMalformedFrameRawDigest is SHA-256 over the exact discarded frame
// `{not-json SENTINEL-FOREIGN-BYTES}`.
const claudeMalformedFrameRawDigest = "sha256:a6d4b1ec6210407d9ecb9c94f5a8efcf5a3160a9ce312025c703ae5ee1f278ec"

// claudeStderrRawDigest is SHA-256 over the exact discarded stderr bytes
// "panic: SENTINEL-STDERR-BYTES\n".
const claudeStderrRawDigest = "sha256:52e6abf2893c5e80e4efe11b4188d6a0fe2c306071a5936e998de0b8e84b206a"

// claudeStderrFixedDetailDigest is SHA-256 over the exact canonical fixed
// safe-detail bytes {"raw_digest":<claudeStderrRawDigest>,"reason":"provider-stderr"}.
const claudeStderrFixedDetailDigest = "sha256:78d98527296f16f9169ba95128d3b3a352c39beb585781b4415d135e963336c6"
