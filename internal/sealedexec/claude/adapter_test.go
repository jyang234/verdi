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
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
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
		_, err := New(nil, dp)
		if err == nil {
			t.Fatal("New(nil process) should return error")
		}
	})

	t.Run("nil_processor_rejected", func(t *testing.T) {
		_, err := New(&testProbeProcess{}, nil)
		if err == nil {
			t.Fatal("New(nil processor) should return error")
		}
	})

	t.Run("verify_adapter_identity_checks", func(t *testing.T) {
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		dp := newTestProcessor(t)
		adapter, err := New(&testProbeProcess{version: launch.Request.AdapterVersion}, dp)
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
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		adapter, err := New(pp, dp)
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
			"--model", launch.Profile.Name,
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
		adapter, err := New(pp, dp)
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
			"--model", launch.Profile.Name,
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
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		const marker = "VERDI_SEALED_PROVIDER_INPUT_V1\n"
		var outer map[string]any
		dec := json.NewDecoder(bytes.NewReader(pp.startStdin))
		dec.UseNumber()
		if err := dec.Decode(&outer); err != nil {
			t.Fatalf("stdin not valid JSON: %v\nstdin=%s", err, pp.startStdin)
		}
		if outer["type"] != "user" {
			t.Fatalf("stdin outer type = %v, want user", outer["type"])
		}
		msg, _ := outer["message"].(map[string]any)
		if msg == nil || msg["role"] != "user" {
			t.Fatalf("stdin message role wrong: %#v", msg)
		}
		content, _ := msg["content"].([]any)
		if len(content) != 1 {
			t.Fatalf("stdin content has %d blocks, want 1", len(content))
		}
		block, _ := content[0].(map[string]any)
		if block["type"] != "text" {
			t.Fatalf("stdin block type = %v, want text", block["type"])
		}
		text, _ := block["text"].(string)
		if !strings.HasPrefix(text, marker) {
			t.Fatalf("stdin text does not start with marker: %q", text[:min(80, len(text))])
		}
	})

	t.Run("api_key_not_in_policy_secrets_refuses", func(t *testing.T) {
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		launch.Profile.PolicySecretValues = [][]byte{[]byte("other-secret-not-api-key")}
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		adapter, err := New(pp, dp)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Start(context.Background(), launch)
		if err == nil || pp.startCmd != nil {
			t.Fatalf("Start with API key < 8 bytes should refuse before process start")
		}
	})

	t.Run("classification_incomplete_refuses", func(t *testing.T) {
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		launch.Profile.ClassificationComplete = false
		pp := &testProbeProcess{version: launch.Request.AdapterVersion}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Start(context.Background(), launch)
		if err == nil || pp.startCmd != nil {
			t.Fatalf("Start with incomplete classification should refuse before process start")
		}
	})

	t.Run("review_resume_refused", func(t *testing.T) {
		launch, _ := claudeTestLaunch(t, sealedexec.ActionResume)
		launch.Review = &sealedexec.ReviewLaunch{Round: "r0", PacketDigest: claudeTestDigest([]byte("packet")), Model: "some-model"}
		dp := newTestProcessor(t)
		adapter, err := New(&testProbeProcess{}, dp)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		_, err = adapter.Resume(context.Background(), launch, "claude-sess-start-001")
		if err == nil {
			t.Fatal("Resume with review should return error")
		}
	})

	t.Run("nil_context_refused", func(t *testing.T) {
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		dp := newTestProcessor(t)
		adapter, err := New(&testProbeProcess{}, dp)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		//nolint:staticcheck
		if _, err := adapter.Start(nil, launch); err == nil { //nolint:contextcheck
			t.Fatal("Start(nil ctx) should return error")
		}
	})

	t.Run("version_mismatch_refuses", func(t *testing.T) {
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: "WRONG-VERSION"}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-start.jsonl")}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-start.jsonl")}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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

	t.Run("resume_fixture_starts_with_resume_kind", func(t *testing.T) {
		launch, _ := claudeTestLaunch(t, sealedexec.ActionResume)
		session := "claude-sess-resume-001"
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-resume.jsonl")}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		if len(result.Observations) == 0 || result.Observations[0].Kind != contextevent.KindResume {
			t.Fatalf("first observation should be resume, got %v", observationKindsC(result.Observations))
		}
		wantPrefix := []contextevent.Kind{contextevent.KindResume, contextevent.KindAdapterStart, contextevent.KindProviderSummary}
		if len(result.Observations) < len(wantPrefix) || !reflect.DeepEqual(observationKindsC(result.Observations[:len(wantPrefix)]), wantPrefix) {
			t.Fatalf("resume prefix kinds = %v, want %v", observationKindsC(result.Observations), wantPrefix)
		}
	})

	t.Run("resume_fixture_thinking_yields_provider_summary", func(t *testing.T) {
		launch, _ := claudeTestLaunch(t, sealedexec.ActionResume)
		session := "claude-sess-resume-001"
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-resume.jsonl")}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-advisory.jsonl")}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: []byte("\n")}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: []byte("{not-json}\n")}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: []byte("{\"type\":\"future.event\",\"value\":1}\n")}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		resultLine := []byte(`{"type":"result","subtype":"success","is_error":false,"result":"success","session_id":"s1","uuid":"u1","duration_ms":1,"duration_api_ms":1,"num_turns":0,"total_cost_usd":0.0,"usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0},"permission_denials":[]}` + "\n")
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: resultLine}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		mutatedInit := []byte(`{"type":"future","subtype":"init","session_id":"s1","model":"claude-opus-5-test","mcp_servers":[{"name":"verdi-context","status":"connected"}],"cwd":"/workspace","tools":[],"permissionMode":"bypassPermissions","apiKeySource":"ANTHROPIC_API_KEY","claude_code_version":"1.2.3","slash_commands":[],"output_style":"default","agents":[],"skills":[],"plugins":[],"uuid":"u1"}` + "\n")
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mutatedInit}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		initLine := []byte(`{"type":"system","subtype":"init","session_id":"claude-sess-start-001","model":"claude-opus-5-test","mcp_servers":[{"name":"verdi-context","status":"connected"}],"cwd":"/workspace","tools":[],"permissionMode":"bypassPermissions","apiKeySource":"ANTHROPIC_API_KEY","claude_code_version":"1.2.3","slash_commands":[],"output_style":"default","agents":[],"skills":[],"plugins":[],"uuid":"u1"}` + "\n")
		mutatedAssistant := []byte(`{"type":"assistant","session_id":"claude-sess-start-001","uuid":"mu","message":{"id":"msg_m","type":"message","role":"assistant","model":"claude-opus-5-test","content":[{"type":"unknown_block","text":"hello"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":1}}}` + "\n")
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: append(initLine, mutatedAssistant...)}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
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

	t.Run("mutation_id_derivation_fails_message_row", func(t *testing.T) {
		// The compound message ID must be "message_id:block_index".
		// If the id were different, the message_digest would differ.
		launch, _ := claudeTestLaunch(t, sealedexec.ActionStart)
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-start.jsonl")}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Start(context.Background(), launch)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		result := collectClaudeRun(t, run)
		var msgPayload *contextevent.ProviderMessagePayload
		for _, obs := range result.Observations {
			if obs.Kind == contextevent.KindProviderMessage {
				msgPayload, _ = obs.Payload.(*contextevent.ProviderMessagePayload)
				break
			}
		}
		if msgPayload == nil {
			t.Fatal("no provider-message observation in start fixture")
		}
		// Compound ID must be "msg_001:0"
		if msgPayload.MessageID != "msg_001:0" {
			t.Fatalf("message id = %q, want msg_001:0 (message_id:block_index)", msgPayload.MessageID)
		}
		// Digest must be over D={family:"assistant/text",message_id:"msg_001",block_index:0,text:"Analysis complete."}
		wantD := buildAssistantTextDetail(t, "msg_001", 0, "Analysis complete.")
		wantDigest := claudeTestDigest(wantD)
		if msgPayload.MessageDigest != wantDigest {
			t.Fatalf("message_digest = %q, want %q (H of exact D)", msgPayload.MessageDigest, wantDigest)
		}
		// Mutation: different message_id changes digest
		mutatedD := buildAssistantTextDetail(t, "msg_WRONG", 0, "Analysis complete.")
		if claudeTestDigest(mutatedD) == wantDigest {
			t.Fatal("mutated message_id must produce different digest (independence witness)")
		}
	})

	t.Run("mutation_thinking_retention_fails_summary_row", func(t *testing.T) {
		// Thinking bytes must NOT enter the detail digest; detail is {content_type,omitted:true}.
		launch, _ := claudeTestLaunch(t, sealedexec.ActionResume)
		session := "claude-sess-resume-001"
		pp := &testProbeProcess{version: launch.Request.AdapterVersion, output: mustClaudeFixture(t, "claude-resume.jsonl")}
		dp := newTestProcessor(t)
		adapter, err := New(pp, dp)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		run, err := adapter.Resume(context.Background(), launch, session)
		if err != nil {
			t.Fatalf("Resume: %v", err)
		}
		result := collectClaudeRun(t, run)
		// Find the provider-summary for thinking (summary_id = "msg_002:0")
		var thinkingSummary *contextevent.ProviderSummaryPayload
		for _, obs := range result.Observations {
			if obs.Kind == contextevent.KindProviderSummary {
				sp, _ := obs.Payload.(*contextevent.ProviderSummaryPayload)
				if sp != nil && sp.SummaryID == "msg_002:0" {
					thinkingSummary = sp
					break
				}
			}
		}
		if thinkingSummary == nil {
			t.Fatal("no provider-summary for thinking block (msg_002:0)")
		}
		// Digest must equal H({content_type:"thinking",omitted:true})
		expectedDetailJSON := buildThinkingOmissionDetail(t, "thinking")
		wantDigest := claudeTestDigest(expectedDetailJSON)
		if thinkingSummary.SummaryDigest != wantDigest {
			t.Fatalf("thinking summary_digest = %q, want digest of {content_type:thinking,omitted:true}", thinkingSummary.SummaryDigest)
		}
		// Mutation: including thinking bytes changes digest
		mutatedJSON := buildThinkingWithBytesDetail(t, "thinking", "internal deliberation")
		if claudeTestDigest(mutatedJSON) == wantDigest {
			t.Fatal("including thinking bytes must produce different digest (independence witness)")
		}
	})
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func mustClaudeFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// claudeTestLaunch builds an AdapterLaunch for the claude adapter in tests.
// Returns the launch and the envRoot (HOME directory) for path assertions.
func claudeTestLaunch(t *testing.T, action sealedexec.Action) (sealedexec.AdapterLaunch, string) {
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
		"ANTHROPIC_API_KEY":                        apiKey,
		"CLAUDE_CONFIG_DIR":                        claudeConfigDir,
		"DISABLE_AUTOUPDATER":                      "1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"CLAUDE_CODE_AUTO_CONNECT_IDE":             "false",
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
	// profile.Name carries the model identifier.
	// (service.go lacks a Model field on ResolvedProfile; this is documented in the task-4 report.)
	return sealedexec.AdapterLaunch{
		Request: request,
		Profile: sealedexec.ResolvedProfile{
			Verification:           sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
			Ref:                    request.Profile,
			Digest:                 request.Profile.Digest,
			Name:                   "claude-opus-5-test", // model identifier
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
	p.run = &testClaudeActiveProcess{observations: claudeTestObservations(p.output)}
	return p.run, nil
}

type testClaudeActiveProcess struct {
	mu           sync.Mutex
	observations []ProcessObservation
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
	return ProcessObservation{Terminal: &ProcessResult{ExitCode: 0}}, nil
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

// ---------------------------------------------------------------------------
// Literal detail builders for mutation witnesses
// ---------------------------------------------------------------------------

// buildAssistantTextDetail builds the canonical D for an assistant text block.
// D = {family:"assistant/text", message_id, block_index, text}
func buildAssistantTextDetail(t *testing.T, messageID string, blockIndex int, text string) []byte {
	t.Helper()
	d := map[string]any{
		"block_index": float64(blockIndex),
		"family":      "assistant/text",
		"message_id":  messageID,
		"text":        text,
	}
	raw, err := canonjson.Marshal(d)
	if err != nil {
		t.Fatalf("marshal assistant text detail: %v", err)
	}
	return bytes.TrimSuffix(raw, []byte("\n"))
}

// buildThinkingOmissionDetail builds D for a thinking block: {content_type, omitted:true}.
func buildThinkingOmissionDetail(t *testing.T, contentType string) []byte {
	t.Helper()
	d := map[string]any{
		"content_type": contentType,
		"omitted":      true,
	}
	raw, err := canonjson.Marshal(d)
	if err != nil {
		t.Fatalf("marshal thinking omission detail: %v", err)
	}
	return bytes.TrimSuffix(raw, []byte("\n"))
}

// buildThinkingWithBytesDetail is the mutation: includes thinking bytes (wrong).
func buildThinkingWithBytesDetail(t *testing.T, contentType, thinkingText string) []byte {
	t.Helper()
	d := map[string]any{
		"content_type": contentType,
		"omitted":      true,
		"thinking":     thinkingText, // mutation: thinking bytes leaked
	}
	raw, err := canonjson.Marshal(d)
	if err != nil {
		t.Fatalf("marshal mutated thinking detail: %v", err)
	}
	return bytes.TrimSuffix(raw, []byte("\n"))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
