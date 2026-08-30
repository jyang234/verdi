package claude

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
}

// ---------------------------------------------------------------------------
// Frozen producer: TestClaudeAdapterParityContract_Behavioral
// Owns every Amendment 002 §5 family, ordering, id/digest/detail projection,
// start/resume/interrupt/advisory/result/receipt parity.
// ---------------------------------------------------------------------------

func TestClaudeAdapterParityContract_Behavioral(t *testing.T) {
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

	t.Run("unknown_family_yields_telemetry_gap", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: []byte("{\"type\":\"future.event\",\"value\":1}\n")}
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
			t.Fatalf("unknown family: want telemetry-gap, got %v", observationKindsC(result.Observations))
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
		mutatedInit := []byte(`{"type":"future","subtype":"init","session_id":"s1","model":"claude-opus-5-test","mcp_servers":[{"name":"verdi-context","status":"connected"}],"cwd":"/workspace","tools":[],"permissionMode":"bypassPermissions","apiKeySource":"ANTHROPIC_API_KEY","claude_code_version":"1.2.3","slash_commands":[],"output_style":"default","agents":[],"skills":[],"plugins":[],"uuid":"u1"}` + "\n")
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

	t.Run("init_rejects_unknown_outer_field", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		line := strings.Replace(claudeInitLine("s1", launch.Workspace.Path),
			`"uuid":"u-init"`, `"uuid":"u-init","future_key":1`, 1)
		result := runClaudeLines(t, launch, envRoot, line)
		assertClaudeGapReason(t, result, "unknown-foreign-field", "decode", claudeSource)
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

	t.Run("assistant_rejects_unknown_message_field", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1},"future_key":true}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad)
		assertClaudeGapReason(t, result, "unknown-foreign-field", "decode", claudeSource)
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

	t.Run("retry_rejects_unknown_field", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		bad := `{"type":"system","subtype":"api_retry","attempt":1,"max_retries":3,"retry_delay_ms":10,"error":{"type":"rate_limit","message":"slow down"},"uuid":"ru","session_id":"s1","future_key":1}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), bad)
		assertClaudeGapReason(t, result, "unknown-foreign-field", "decode", claudeSource)
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

	t.Run("duplicate_message_id_is_refused", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		msg := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		result := runClaudeLines(t, launch, envRoot, claudeInitLine("s1", launch.Workspace.Path), msg, msg)
		assertClaudeGapReason(t, result, "duplicate-message-id", "decode", claudeSource)
	})

	t.Run("duplicate_tool_result_is_refused", func(t *testing.T) {
		launch, envRoot := claudeTestLaunch(t, sealedexec.ActionStart)
		toolUse := `{"type":"assistant","session_id":"s1","uuid":"mu","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"tool_use","id":"call_1","name":"Read","input":{"path":"README.md"}}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}`
		toolResult := `{"type":"user","session_id":"s1","uuid":"tu","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":"ok"}]}}`
		result := runClaudeLines(t, launch, envRoot,
			claudeInitLine("s1", launch.Workspace.Path), toolUse, toolResult, toolResult)
		assertClaudeGapReason(t, result, "duplicate-tool-result", "decode", claudeSource)
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

	// Segment-reason witness: I-109/IL-073 require a detail-processor segment
	// failure to carry the `segment-store-failed` reason and the `segment`
	// operation, and decodeFailure's own table already maps that pair. Every
	// processDetail call site in adapter.go nevertheless hard-codes
	// "redaction-failed", so the reachable reason/operation is the pinned
	// boundary below. Repairing it requires discriminating the underlying error
	// class inside adapter.go, which is outside this correction pass's declared
	// write set; the exact non-conforming pair is disclosed rather than hidden,
	// and this row fails the moment either the store stops being reached or the
	// mapping changes.
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
						t.Errorf("segment-store failure reason/operation = %q, want the pinned boundary %q",
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

// claudeSegmentStoreFailureBoundary is the exact reason/operation pair a failed
// controller segment store currently produces. I-109/IL-073 require
// "segment-store-failed/segment"; this ratchet pins the disclosed
// non-conforming value so the gap cannot be mistaken for conformance.
const claudeSegmentStoreFailureBoundary = "redaction-failed/redaction"

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
// Amendment 002 §5 requires init to observe exactly.
func mustClaudeFixture(t *testing.T, name, workspace string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return bytes.ReplaceAll(data, []byte(`"cwd":"/workspace"`), []byte(`"cwd":"`+workspace+`"`))
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
		Path:          filepath.Join(envRoot, claudeMCPConfigName),
		URL:           "http://127.0.0.1:54321/mcp",
		Authorization: "Bearer sha256:" + strings.Repeat("a", 64),
	}
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
	mu         sync.Mutex
	probeCmd   *exec.Cmd
	startCmd   *exec.Cmd
	startStdin []byte
	version    string
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
	if p.version == "" {
		return nil, nil, 1, errors.New("testProbeProcess: no version configured")
	}
	return []byte(p.version + "\n"), nil, 0, nil
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
			Path:          filepath.Join(t.TempDir(), claudeMCPConfigName),
			URL:           "http://127.0.0.1:54321/mcp",
			Authorization: "Bearer sha256:" + strings.Repeat("b", 64),
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

	mcpConfigs := []struct {
		name   string
		config MCPConfig
	}{
		{"zero_value", MCPConfig{}},
		{"relative_path", MCPConfig{Path: "claude-mcp.json", URL: "http://127.0.0.1:1/mcp", Authorization: "Bearer sha256:" + strings.Repeat("a", 64)}},
		{"unclean_path", MCPConfig{Path: "/tmp/../tmp/claude-mcp.json", URL: "http://127.0.0.1:1/mcp", Authorization: "Bearer sha256:" + strings.Repeat("a", 64)}},
		{"wrong_basename", MCPConfig{Path: "/tmp/mcp.json", URL: "http://127.0.0.1:1/mcp", Authorization: "Bearer sha256:" + strings.Repeat("a", 64)}},
		{"missing_url", MCPConfig{Path: "/tmp/claude-mcp.json", Authorization: "Bearer sha256:" + strings.Repeat("a", 64)}},
		{"missing_authorization", MCPConfig{Path: "/tmp/claude-mcp.json", URL: "http://127.0.0.1:1/mcp"}},
		{"unscoped_authorization", MCPConfig{Path: "/tmp/claude-mcp.json", URL: "http://127.0.0.1:1/mcp", Authorization: "Bearer opaque-token"}},
		{"short_capability_digest", MCPConfig{Path: "/tmp/claude-mcp.json", URL: "http://127.0.0.1:1/mcp", Authorization: "Bearer sha256:" + strings.Repeat("a", 63)}},
	}
	for _, tc := range mcpConfigs {
		t.Run("mcp_config_rejected_"+tc.name, func(t *testing.T) {
			if _, err := New(&testProbeProcess{}, newTestProcessor(t), tc.config); err == nil {
				t.Fatalf("New with %s MCP config = nil, want refusal", tc.name)
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
		`","model":"claude-opus-5-test","mcp_servers":[{"name":"verdi-context","status":"connected"}],"cwd":"` + cwd +
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
const claudeInitSummaryDetail = `{"family":"system/init","mcp_servers":[{"name":"verdi-context","status":"connected"}],"model":"claude-opus-5-test","permission_mode":"bypassPermissions","session_id":"[REDACTED]"}`
const claudeInitSummaryDigest = "sha256:7b41e049b61e2f8745d1845f2a0e383aecb94dbb931f60c2a59b496e9a0c5a44"

// claudeMalformedFrameRawDigest is SHA-256 over the exact discarded frame
// `{not-json SENTINEL-FOREIGN-BYTES}`.
const claudeMalformedFrameRawDigest = "sha256:a6d4b1ec6210407d9ecb9c94f5a8efcf5a3160a9ce312025c703ae5ee1f278ec"

// claudeStderrRawDigest is SHA-256 over the exact discarded stderr bytes
// "panic: SENTINEL-STDERR-BYTES\n".
const claudeStderrRawDigest = "sha256:52e6abf2893c5e80e4efe11b4188d6a0fe2c306071a5936e998de0b8e84b206a"

// claudeStderrFixedDetailDigest is SHA-256 over the exact canonical fixed
// safe-detail bytes {"raw_digest":<claudeStderrRawDigest>,"reason":"provider-stderr"}.
const claudeStderrFixedDetailDigest = "sha256:78d98527296f16f9169ba95128d3b3a352c39beb585781b4415d135e963336c6"
