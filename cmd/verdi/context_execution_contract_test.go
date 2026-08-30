package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/execworkspace"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/sealedexec"
	sealedcodex "github.com/jyang234/verdi/internal/sealedexec/codex"
)

func TestContextExecutionPublicContract_Behavioral(t *testing.T) {
	bin := buildVerdiBinary(t)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "missing request"},
		{name: "request missing value", args: []string{"--request"}},
		{name: "request consumes no flag", args: []string{"--request", "--out", "result"}},
		{name: "duplicate request", args: []string{"--request", "one", "--request", "two"}},
		{name: "out missing value", args: []string{"--request", "one", "--out"}},
		{name: "out consumes no flag", args: []string{"--out", "--request", "one"}},
		{name: "duplicate out", args: []string{"--request", "one", "--out", "a", "--out", "b"}},
		{name: "unknown flag", args: []string{"--request", "one", "--controller", "ambient"}},
		{name: "extra argument", args: []string{"--request", "one", "extra"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"context", "execution"}, tc.args...)
			observation := runSealedContextBinary(t, bin, t.TempDir(), nil, args...)
			if observation.exitCode != 2 || observation.stdout != "" || observation.stderr != "usage: verdi context execution --request <path|-> [--out <path>]\n" {
				t.Fatalf("execution usage = %#v, want exact usage-first operational refusal", observation)
			}
		})
	}

	requestBytes := sealedCanonicalExecutionRequest(t, t.TempDir())
	t.Run("missing inherited descriptor", func(t *testing.T) {
		observation := runSealedContextBinary(t, bin, t.TempDir(), requestBytes, "context", "execution", "--request", "-")
		if observation.exitCode != 2 || observation.stdout != "" || !strings.Contains(observation.stderr, "FD 3") {
			t.Fatalf("missing FD3 = %#v, want operational refusal", observation)
		}
	})
	for _, tc := range []struct {
		name      string
		pathMode  bool
		wrongKind bool
	}{
		{name: "path request reaches inherited controller", pathMode: true},
		{name: "stdin request reaches inherited controller"},
		{name: "wrong-kind inherited descriptor", pathMode: true, wrongKind: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			requestArg := "-"
			stdin := requestBytes
			if tc.pathMode {
				requestArg = filepath.Join(dir, "request.json")
				if err := os.WriteFile(requestArg, requestBytes, 0o600); err != nil {
					t.Fatal(err)
				}
				stdin = nil
			}
			if tc.wrongKind {
				file, err := os.Open(requestArg)
				if err != nil {
					t.Fatal(err)
				}
				observation := runSealedContextBinaryWithFiles(t, bin, dir, stdin, []*os.File{file}, "context", "execution", "--request", requestArg)
				if observation.exitCode != 2 || observation.stdout != "" || !strings.Contains(observation.stderr, "FD 3") || !strings.Contains(observation.stderr, "socket") {
					t.Fatalf("wrong-kind FD3 = %#v, want socket-only operational refusal", observation)
				}
				return
			}
			observation, call := runWithRefusingController(t, bin, dir, stdin, "context", "execution", "--request", requestArg)
			if call.Operation != sealedexec.ControllerOperationVerifyAuthority || call.CallSequence != 1 {
				t.Fatalf("first controller call = %#v, want sequence 1 verify-authority", call)
			}
			if observation.exitCode != 2 || observation.stdout != "" || !strings.Contains(observation.stderr, string(sealedexec.ControllerErrorUnavailable)) {
				t.Fatalf("controller refusal = %#v, want operational controller error", observation)
			}
			if strings.Contains(observation.stderr, dir) {
				t.Fatalf("stderr leaked absolute request/runway path: %s", observation.stderr)
			}
		})
	}
	t.Run("disconnected inherited socket", func(t *testing.T) {
		files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
		if err != nil {
			t.Fatal(err)
		}
		_ = syscall.Close(files[0])
		child := os.NewFile(uintptr(files[1]), "disconnected-child")
		observation := runSealedContextBinaryWithFiles(t, bin, t.TempDir(), requestBytes, []*os.File{child}, "context", "execution", "--request", "-")
		if observation.exitCode != 2 || observation.stdout != "" || !strings.Contains(observation.stderr, "controller") {
			t.Fatalf("disconnected FD3 = %#v, want operational controller refusal", observation)
		}
	})

	t.Run("controller negative proof remains a verdict", func(t *testing.T) {
		observation, call := runWithControllerReply(t, bin, t.TempDir(), requestBytes, []string{"context", "execution", "--request", "-"}, func(call sealedexec.ControllerCall) []byte {
			request := call.VerifyAuthority.Request
			result := sealedexec.ControllerResult{
				Schema: sealedexec.ControllerResultSchemaID, CallSequence: call.CallSequence, Operation: call.Operation,
				VerifyAuthority: sealedexec.ControllerVerifyAuthorityResult{
					Schema: "verdi.context-controller/verify-authority-result/v1",
					Facts: sealedexec.AuthorityFacts{Verification: sealedexec.Verification{
						State: contextcompile.ResolutionViolatedWithWitness, Failure: sealedexec.FailureRejected,
						Witnesses: []string{"fixture authority contradiction"},
					}, ManifestRevision: request.ManifestRevision, ManifestDigest: request.ManifestDigest,
						ProjectionDigest: request.ProjectionDigest, AuthorityDigest: request.AuthorityVerdict.Digest,
						AcceptedSpecCommit: request.Manifest.AcceptedSpec.Commit},
				},
			}
			encoded, err := sealedexec.EncodeControllerResult(result)
			if err != nil {
				t.Fatalf("encode negative authority proof: %v", err)
			}
			return encoded
		})
		if call.Operation != sealedexec.ControllerOperationVerifyAuthority || observation.exitCode != 1 || observation.stdout != "" || !strings.Contains(observation.stderr, "not proven") {
			t.Fatalf("negative authority proof = %#v/%#v, want verdict refusal", call, observation)
		}
	})

	t.Run("controller result envelope fails closed", func(t *testing.T) {
		for _, mutation := range []string{"schema", "operation", "call sequence", "unknown field"} {
			t.Run(mutation, func(t *testing.T) {
				observation, call := runWithControllerReply(t, bin, t.TempDir(), requestBytes, []string{"context", "execution", "--request", "-"}, func(call sealedexec.ControllerCall) []byte {
					return malformedControllerReply(t, call, mutation)
				})
				if call.Operation != sealedexec.ControllerOperationVerifyAuthority || observation.exitCode != 2 || observation.stdout != "" || observation.stderr == "" {
					t.Fatalf("controller %s mutation = %#v/%#v, want operational refusal", mutation, call, observation)
				}
			})
		}
	})

	t.Run("signal watcher registers once and joins cleanly", func(t *testing.T) {
		notifier := &sealedTestSignalNotifier{}
		interrupted := make(chan struct{}, 1)
		watcher := startContextExecutionSignalWatcher(notifier, func() error {
			interrupted <- struct{}{}
			return nil
		})
		notifier.channel <- syscall.SIGTERM
		select {
		case <-interrupted:
		case <-time.After(time.Second):
			t.Fatal("signal watcher did not invoke the normalized interrupt callback")
		}
		watcher.Stop()
		if notifier.notifyCalls != 1 || notifier.stopCalls != 1 || !reflect.DeepEqual(notifier.signals, []os.Signal{os.Interrupt, syscall.SIGTERM}) {
			t.Fatalf("signal registration notify/stop/signals = %d/%d/%v", notifier.notifyCalls, notifier.stopCalls, notifier.signals)
		}

		idleNotifier := &sealedTestSignalNotifier{}
		idleCalls := 0
		idleWatcher := startContextExecutionSignalWatcher(idleNotifier, func() error {
			idleCalls++
			return nil
		})
		idleWatcher.Stop()
		if idleCalls != 0 || idleNotifier.notifyCalls != 1 || idleNotifier.stopCalls != 1 {
			t.Fatalf("idle watcher calls/notify/stop = %d/%d/%d, want 0/1/1", idleCalls, idleNotifier.notifyCalls, idleNotifier.stopCalls)
		}

		retryNotifier := &sealedTestSignalNotifier{}
		attempted := make(chan int, 2)
		attempts := 0
		retryWatcher := startContextExecutionSignalWatcher(retryNotifier, func() error {
			attempts++
			attempted <- attempts
			if attempts == 1 {
				return sealedexec.ErrInterruptNotActive
			}
			return nil
		})
		retryNotifier.channel <- syscall.SIGTERM
		if attempt := <-attempted; attempt != 1 {
			t.Fatalf("pre-active interrupt attempt = %d, want 1", attempt)
		}
		retryNotifier.channel <- os.Interrupt
		if attempt := <-attempted; attempt != 2 {
			t.Fatalf("active interrupt attempt = %d, want 2", attempt)
		}
		retryWatcher.Stop()
		if retryNotifier.notifyCalls != 1 || retryNotifier.stopCalls != 1 {
			t.Fatalf("retry watcher notify/stop = %d/%d, want 1/1", retryNotifier.notifyCalls, retryNotifier.stopCalls)
		}

		permanentNotifier := &sealedTestSignalNotifier{}
		permanentAttempted := make(chan struct{}, 1)
		permanentWatcher := startContextExecutionSignalWatcher(permanentNotifier, func() error {
			permanentAttempted <- struct{}{}
			return sealedexec.ErrVerdict
		})
		permanentNotifier.channel <- syscall.SIGTERM
		<-permanentAttempted
		permanentWatcher.Stop()
		if permanentNotifier.notifyCalls != 1 || permanentNotifier.stopCalls != 1 {
			t.Fatalf("permanent watcher notify/stop = %d/%d, want 1/1", permanentNotifier.notifyCalls, permanentNotifier.stopCalls)
		}
	})
}

func TestScopedContextMCPContract_Behavioral(t *testing.T) {
	bin := buildVerdiBinary(t)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "missing request"},
		{name: "request missing value", args: []string{"--request"}},
		{name: "request consumes no flag", args: []string{"--request", "--out"}},
		{name: "duplicate request", args: []string{"--request", "one", "--request", "two"}},
		{name: "out forbidden", args: []string{"--request", "one", "--out", "result"}},
		{name: "controller forbidden", args: []string{"--request", "one", "--controller", "ambient"}},
		{name: "extra argument", args: []string{"--request", "one", "extra"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"context", "mcp"}, tc.args...)
			observation := runSealedContextBinary(t, bin, t.TempDir(), nil, args...)
			if observation.exitCode != 2 || observation.stdout != "" || observation.stderr != "usage: verdi context mcp --request <path|->\n" {
				t.Fatalf("scoped MCP usage = %#v, want exact usage-first operational refusal", observation)
			}
		})
	}

	requestBytes := sealedCanonicalExecutionRequest(t, t.TempDir())
	t.Run("malformed bootstrap is refused before controller access", func(t *testing.T) {
		for _, mutation := range []struct {
			name string
			data []byte
		}{
			{name: "unknown field", data: append([]byte(`{"unexpected":true,`), requestBytes[1:]...)},
			{name: "duplicate field", data: append([]byte(`{"schema":"verdi.context-execution-request/v1",`), requestBytes[1:]...)},
			{name: "null document", data: []byte("null\n")},
		} {
			t.Run(mutation.name, func(t *testing.T) {
				observation := runSealedContextBinary(t, bin, t.TempDir(), mutation.data, "context", "mcp", "--request", "-")
				if observation.exitCode != 2 || observation.stdout != "" || !strings.Contains(observation.stderr, "decoding request") || strings.Contains(observation.stderr, "FD 3") {
					t.Fatalf("MCP %s bootstrap = %#v, want strict refusal before controller", mutation.name, observation)
				}
			})
		}
	})
	for _, tc := range []struct {
		name     string
		pathMode bool
	}{
		{name: "path bootstrap leaves protocol stdin", pathMode: true},
		{name: "stdin bootstrap consumes one request frame"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			requestArg := "-"
			stdin := append(append([]byte(nil), requestBytes...), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n")...)
			if tc.pathMode {
				requestArg = filepath.Join(dir, "request.json")
				if err := os.WriteFile(requestArg, requestBytes, 0o600); err != nil {
					t.Fatal(err)
				}
				stdin = []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
			}
			observation, call := runWithRefusingController(t, bin, dir, stdin, "context", "mcp", "--request", requestArg)
			if call.Operation != sealedexec.ControllerOperationVerifyAuthority || call.CallSequence != 1 {
				t.Fatalf("first scoped MCP controller call = %#v, want sequence 1 verify-authority", call)
			}
			if observation.exitCode != 2 || observation.stdout != "" || !strings.Contains(observation.stderr, string(sealedexec.ControllerErrorUnavailable)) {
				t.Fatalf("scoped MCP controller refusal = %#v, want operational controller error", observation)
			}
		})
	}

	t.Run("bootstrap cross-matches installed expansion state", func(t *testing.T) {
		requestBytes := sealedCanonicalExecutionRequest(t, t.TempDir())
		request, err := sealedexec.DecodeExecutionRequest(bytes.NewReader(requestBytes))
		if err != nil {
			t.Fatal(err)
		}
		workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
		if err != nil {
			t.Fatal(err)
		}
		workspaceDigest, err := sealedexec.ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
		if err != nil {
			t.Fatal(err)
		}
		workspace := sealedexec.WorkspaceFacts{
			Verification: sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
			WorkspaceID:  workspaceID, Path: "/sealed/workspace", Request: request.ExecutionWorkspaceRequest,
			RequestDigest: workspaceDigest, CurrentCommit: request.InputCommit, CurrentTree: request.InputTree, Clean: true,
		}
		revision := contextevent.Revision{
			Schema: contextevent.RevisionSchemaID, ManifestRevision: request.ManifestRevision,
			ManifestDigest: request.ManifestDigest, FirstGlobalSequence: 1, TerminalGlobalSequence: 1,
			TerminalSourceSequence: 1, TerminalKind: contextevent.KindExecutionResult,
			EventRoot: sealedTestDigest("current-event"),
		}
		checkpoint := sealedexec.RecorderCheckpoint{
			Verification: sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
			Digest:       sealedTestDigest("checkpoint"), Revisions: []contextevent.Revision{},
		}
		expansion := sealedexec.ExpansionFacts{Verification: sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}}, Root: ""}
		fresh, err := buildMCPFlightSnapshot(request, workspace, checkpoint, expansion)
		if err != nil || fresh.Revision != request.ManifestRevision || fresh.ManifestDigest != request.ManifestDigest || fresh.ExpansionRoot != "" || fresh.NextSourceSequence != 1 {
			t.Fatalf("fresh snapshot = %#v/%v", fresh, err)
		}

		// I-86/I-110 active tail: the reconstruction continues the durable append
		// position and carries the invalidation fact, so the parent-hosted Claude
		// surface cannot present an invalidated or resumed flight as sequence one.
		activeCheckpoint := checkpoint
		activeCheckpoint.ActiveRevision = &sealedexec.ActiveRevision{
			Revision: request.ManifestRevision, ManifestDigest: request.ManifestDigest,
			NextSourceSequence: 4, PriorEventDigest: sealedTestDigest("active-prior"),
			LastGlobalSequence: 3, EventAcks: []contextevent.EventAck{},
		}
		tail, err := buildMCPFlightSnapshot(request, workspace, activeCheckpoint, expansion)
		if err != nil || tail.NextSourceSequence != 4 || tail.PriorEventDigest != sealedTestDigest("active-prior") ||
			tail.LastGlobalSequence != 3 || tail.Invalidated {
			t.Fatalf("active-tail snapshot = %#v/%v", tail, err)
		}
		invalidatedCheckpoint := activeCheckpoint
		invalidatedCheckpoint.ActiveRevision = cloneSealedActiveRevision(activeCheckpoint.ActiveRevision)
		invalidatedCheckpoint.ActiveRevision.Invalidated = true
		invalidated, err := buildMCPFlightSnapshot(request, workspace, invalidatedCheckpoint, expansion)
		if err != nil || !invalidated.Invalidated {
			t.Fatalf("invalidated snapshot = %#v/%v, want the invalidation carried into scoped MCP", invalidated, err)
		}

		laterCheckpoint := checkpoint
		laterCheckpoint.Revisions = []contextevent.Revision{revision}
		laterCheckpoint.EventChainRoot, err = contextevent.EventChainRoot(laterCheckpoint.Revisions)
		if err != nil {
			t.Fatal(err)
		}
		laterCheckpoint.TerminalSourceSequence = revision.TerminalSourceSequence
		laterCheckpoint.TerminalGlobalSequence = revision.TerminalGlobalSequence
		laterCheckpoint.ActiveRevision = &sealedexec.ActiveRevision{
			Revision: request.ManifestRevision + 1, ManifestDigest: sealedTestDigest("child-manifest"), NextSourceSequence: 1,
			PriorRevision: &contextevent.PriorRevision{
				ManifestRevision: revision.ManifestRevision, ManifestDigest: revision.ManifestDigest, EventRoot: revision.EventRoot,
				TerminalSourceSequence: revision.TerminalSourceSequence, TerminalGlobalSequence: revision.TerminalGlobalSequence,
			},
			LastGlobalSequence: revision.TerminalGlobalSequence, Invalidated: true,
		}
		laterExpansion := expansion
		laterExpansion.Root = sealedTestDigest("installed-expansion-root")
		later, err := buildMCPFlightSnapshot(request, workspace, laterCheckpoint, laterExpansion)
		if err != nil || later.Revision != laterCheckpoint.ActiveRevision.Revision || later.ManifestDigest != laterCheckpoint.ActiveRevision.ManifestDigest || later.ExpansionRoot != laterExpansion.Root ||
			later.NextSourceSequence != 1 || later.PriorEventDigest != "" || !reflect.DeepEqual(later.PriorRevision, laterCheckpoint.ActiveRevision.PriorRevision) || later.LastGlobalSequence != revision.TerminalGlobalSequence || !later.Invalidated {
			t.Fatalf("later snapshot = %#v/%v", later, err)
		}
		later.PriorRevision.ManifestDigest = sealedTestDigest("mutated-copy")
		if laterCheckpoint.ActiveRevision.PriorRevision.ManifestDigest == later.PriorRevision.ManifestDigest {
			t.Fatal("snapshot aliases the recorder active-revision bridge")
		}
		completedCheckpoint := laterCheckpoint
		completedCheckpoint.ActiveRevision = nil
		expansionClosedCheckpoint := checkpoint
		expansionClosedCheckpoint.ActiveRevision = &sealedexec.ActiveRevision{
			Revision: request.ManifestRevision + 1, ManifestDigest: sealedTestDigest("expansion-child-manifest"), NextSourceSequence: 1,
			PriorRevision: &contextevent.PriorRevision{
				ManifestRevision: request.ManifestRevision, ManifestDigest: request.ManifestDigest,
				EventRoot: sealedTestDigest("expansion-parent-event"), TerminalSourceSequence: 3, TerminalGlobalSequence: 7,
			},
			LastGlobalSequence: 7,
		}
		expansionClosed, err := buildMCPFlightSnapshot(request, workspace, expansionClosedCheckpoint, laterExpansion)
		if err != nil || expansionClosed.PriorRevision == nil || expansionClosed.PriorRevision.ManifestRevision != request.ManifestRevision {
			t.Fatalf("expansion-closed predecessor snapshot = %#v/%v", expansionClosed, err)
		}

		for _, mutation := range []struct {
			name       string
			checkpoint sealedexec.RecorderCheckpoint
			expansion  sealedexec.ExpansionFacts
		}{
			{name: "fresh with installed root", checkpoint: checkpoint, expansion: laterExpansion},
			{name: "later without installed root", checkpoint: laterCheckpoint, expansion: expansion},
			{name: "completed checkpoint without active tail", checkpoint: completedCheckpoint, expansion: expansion},
			{name: "unproven expansion", checkpoint: checkpoint, expansion: sealedexec.ExpansionFacts{Verification: sealedexec.Verification{State: contextcompile.ResolutionUnproven, Failure: sealedexec.FailureUnproven, Witnesses: []string{"unavailable"}}}},
			{name: "terminal sequence mismatch", checkpoint: func() sealedexec.RecorderCheckpoint {
				bad := laterCheckpoint
				bad.TerminalSourceSequence++
				return bad
			}(), expansion: laterExpansion},
			{name: "later sequence one without predecessor bridge", checkpoint: func() sealedexec.RecorderCheckpoint {
				bad := expansionClosedCheckpoint
				bad.ActiveRevision = cloneSealedActiveRevision(bad.ActiveRevision)
				bad.ActiveRevision.PriorRevision = nil
				bad.ActiveRevision.LastGlobalSequence = 0
				return bad
			}(), expansion: laterExpansion},
			{name: "later sequence one with wrong predecessor bridge", checkpoint: func() sealedexec.RecorderCheckpoint {
				bad := expansionClosedCheckpoint
				bad.ActiveRevision = cloneSealedActiveRevision(bad.ActiveRevision)
				bad.ActiveRevision.PriorRevision.ManifestRevision++
				return bad
			}(), expansion: laterExpansion},
		} {
			t.Run(mutation.name, func(t *testing.T) {
				if _, err := buildMCPFlightSnapshot(request, workspace, mutation.checkpoint, mutation.expansion); err == nil {
					t.Fatal("bootstrap accepted contradictory checkpoint/expansion facts")
				}
			})
		}
	})

	t.Run("real scoped MCP serves the exact registry in both request modes", func(t *testing.T) {
		for _, pathMode := range []bool{false, true} {
			name := "stdin request"
			if pathMode {
				name = "path request"
			}
			t.Run(name, func(t *testing.T) {
				runSuccessfulScopedMCP(t, bin, pathMode)
			})
		}
	})

	t.Run("real scoped MCP cross-matches an expansion-closed predecessor bridge", func(t *testing.T) {
		for _, test := range []struct {
			name     string
			mutation string
			wantExit int
		}{
			{name: "valid later bridge"},
			{name: "later bridge missing", mutation: "missing", wantExit: 1},
			{name: "later bridge contradicts request", mutation: "wrong-manifest", wantExit: 1},
		} {
			t.Run(test.name, func(t *testing.T) {
				runScopedMCPLaterCheckpoint(t, bin, test.mutation, test.wantExit)
			})
		}
	})

	t.Run("real scoped MCP records context decisions and terminal invalidation", func(t *testing.T) {
		for _, scenario := range []struct {
			name     string
			kind     sealedexec.InspectionKind
			exitCode int
		}{
			{name: "approved", kind: sealedexec.InspectionContextApproved},
			{name: "denied", kind: sealedexec.InspectionContextDenied},
			{name: "epoch invalidated", kind: sealedexec.InspectionEpochInvalidated, exitCode: 1},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				runScopedMCPContextScenario(t, bin, scenario.kind, scenario.exitCode)
			})
		}
	})

	t.Run("malformed protocol is framed before operational exit", func(t *testing.T) {
		runScopedMCPProtocolFailure(t, bin)
	})

	t.Run("recorder rejection is framed before operational exit", func(t *testing.T) {
		runScopedMCPRecorderRejection(t, bin)
	})

	t.Run("controller loss is operational before protocol service", func(t *testing.T) {
		requestBytes := sealedCanonicalExecutionRequest(t, t.TempDir())
		observation, call := runWithControllerReply(t, bin, t.TempDir(), requestBytes, []string{"context", "mcp", "--request", "-"}, func(sealedexec.ControllerCall) []byte { return nil })
		if call.Operation != sealedexec.ControllerOperationVerifyAuthority || observation.exitCode != 2 || observation.stdout != "" || !strings.Contains(observation.stderr, "controller") {
			t.Fatalf("scoped MCP controller loss = %#v/%#v", call, observation)
		}
	})

	t.Run("undeclared tool is framed without mutation", func(t *testing.T) {
		runScopedMCPUndeclaredTool(t, bin)
	})
}

func TestContextExecutionCompletionContract_Behavioral(t *testing.T) {
	bin := buildVerdiBinary(t)
	t.Run("built binary completes sealed start lifecycle", func(t *testing.T) {
		for _, test := range []struct {
			name string
			out  bool
		}{{name: "stdout"}, {name: "atomic output file", out: true}} {
			t.Run(test.name, func(t *testing.T) {
				runSuccessfulSealedStart(t, bin, test.out, false, "", false, false, false, "", false)
			})
		}
	})
	t.Run("built binary completes sealed resume lifecycle", func(t *testing.T) {
		runSuccessfulSealedResume(t, bin)
	})
	t.Run("profile and recorder contradictions remain verdicts", func(t *testing.T) {
		for _, mismatch := range []string{"profile", "recorder"} {
			t.Run(mismatch, func(t *testing.T) {
				runSealedControllerContradiction(t, bin, mismatch)
			})
		}
	})
	t.Run("profile activation failure redacts controller paths", func(t *testing.T) {
		runSealedControllerContradiction(t, bin, "profile environment")
	})
	t.Run("atomic output failure is durably quarantined", func(t *testing.T) {
		runSuccessfulSealedStart(t, bin, true, true, "", false, false, false, "", false)
	})
	t.Run("receipt rejection is durably quarantined before public output", func(t *testing.T) {
		runSuccessfulSealedStart(t, bin, false, false, sealedexec.ControllerOperationAppendReceipt, false, false, false, "", false)
	})
	t.Run("result rejection is durably quarantined before public output", func(t *testing.T) {
		runSuccessfulSealedStart(t, bin, false, false, sealedexec.ControllerOperationRecorderAppend, false, false, false, "", false)
	})
	t.Run("handback rejection occurs only after public receipt output and prevents release", func(t *testing.T) {
		runSuccessfulSealedStart(t, bin, false, false, sealedexec.ControllerOperationPersistHandback, false, false, false, "", false)
	})
	t.Run("release failure follows durable handback and public receipt output", func(t *testing.T) {
		runSuccessfulSealedStart(t, bin, false, false, "", true, false, false, "", false)
	})
	t.Run("provider failure is durably quarantined without public output", func(t *testing.T) {
		runSuccessfulSealedStart(t, bin, false, false, "", false, true, false, "", false)
	})
	t.Run("SIGTERM requests one normalized stop and durable quarantine", func(t *testing.T) {
		runInterruptedSealedStart(t, bin, false)
	})
	t.Run("pre-active SIGTERM preserves the watcher for the active run", func(t *testing.T) {
		runInterruptedSealedStart(t, bin, true)
	})
	t.Run("provider launch failure redacts profile paths", func(t *testing.T) {
		runSuccessfulSealedStart(t, bin, false, false, "", false, false, false, "", true)
	})
	t.Run("advisory completion never upgrades or hands back", func(t *testing.T) {
		runSuccessfulSealedStart(t, bin, false, false, "", false, false, true, "", false)
	})
	t.Run("protected spec add rename and copy-old are durably quarantined", func(t *testing.T) {
		for _, change := range []string{"add", "rename", "copy-old"} {
			t.Run(change, func(t *testing.T) {
				runSuccessfulSealedStart(t, bin, false, false, "", false, false, false, change, false)
			})
		}
	})
	observation := runSealedContextBinary(t, bin, t.TempDir(), nil, "context", "execution", "--request", "missing.json")
	if observation.exitCode != 2 || observation.stdout != "" || !strings.HasPrefix(observation.stderr, "context execution: reading request:") {
		t.Fatalf("execution request handling = %#v, want command-owned read failure", observation)
	}

	requestBytes := sealedCanonicalExecutionRequest(t, t.TempDir())
	for _, mutation := range []struct {
		name string
		data []byte
	}{
		{name: "unknown field", data: append([]byte(`{"unexpected":true,`), requestBytes[1:]...)},
		{name: "duplicate field", data: append([]byte(`{"schema":"verdi.execution-request/v1",`), requestBytes[1:]...)},
		{name: "null document", data: []byte("null\n")},
		{name: "trailing document", data: append(append([]byte(nil), requestBytes...), []byte("{}\n")...)},
	} {
		t.Run("strict request rejects "+mutation.name, func(t *testing.T) {
			observation := runSealedContextBinary(t, bin, t.TempDir(), mutation.data, "context", "execution", "--request", "-")
			if observation.exitCode != 2 || observation.stdout != "" || !strings.Contains(observation.stderr, "decoding request") || strings.Contains(observation.stderr, "FD 3") {
				t.Fatalf("%s request = %#v, want strict refusal before controller", mutation.name, observation)
			}
		})
	}

	t.Run("provider receives no inherited files", func(t *testing.T) {
		file, err := os.Open(os.DevNull)
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command("provider-must-not-start")
		command.ExtraFiles = []*os.File{file}
		if _, err := (commandCodexProcess{}).Start(context.Background(), command, nil); err == nil || !strings.Contains(err.Error(), "must not inherit extra files") {
			t.Fatalf("provider ExtraFiles error = %v, want pre-launch rejection", err)
		}
	})

	t.Run("quarantine is durable only after its controller acknowledgment", func(t *testing.T) {
		for _, test := range []struct {
			name    string
			outcome sealedexec.HandbackOutcome
			want    bool
		}{
			{name: "quarantine and acknowledgment", outcome: sealedexec.HandbackOutcome{Quarantine: &sealedexec.QuarantineRecord{}, ControlAck: sealedexec.ControlAck{Digest: sealedTestDigest("control-ack")}}, want: true},
			{name: "quarantine without acknowledgment", outcome: sealedexec.HandbackOutcome{Quarantine: &sealedexec.QuarantineRecord{}}},
			{name: "acknowledgment without quarantine", outcome: sealedexec.HandbackOutcome{ControlAck: sealedexec.ControlAck{Digest: sealedTestDigest("control-ack")}}},
		} {
			t.Run(test.name, func(t *testing.T) {
				if got := sealedQuarantineDurable(test.outcome); got != test.want {
					t.Fatalf("sealedQuarantineDurable() = %v, want %v", got, test.want)
				}
			})
		}
	})
}

type sealedContextObservation struct {
	exitCode int
	stdout   string
	stderr   string
}

type sealedTestSignalNotifier struct {
	channel     chan<- os.Signal
	signals     []os.Signal
	notifyCalls int
	stopCalls   int
}

func (n *sealedTestSignalNotifier) Notify(channel chan<- os.Signal, signals ...os.Signal) {
	n.channel = channel
	n.signals = append([]os.Signal(nil), signals...)
	n.notifyCalls++
}

func (n *sealedTestSignalNotifier) Stop(channel chan<- os.Signal) {
	if n.channel != channel {
		panic("signal notifier stopped a different channel")
	}
	n.stopCalls++
}

func cloneSealedActiveRevision(active *sealedexec.ActiveRevision) *sealedexec.ActiveRevision {
	if active == nil {
		return nil
	}
	clone := *active
	if active.PriorRevision != nil {
		bridge := *active.PriorRevision
		clone.PriorRevision = &bridge
	}
	return &clone
}

type sealedCompiledExecutionFixture struct {
	root         string
	head         string
	tree         string
	compiled     contextcompile.Result
	request      sealedexec.ExecutionRequest
	requestBytes []byte
}

func buildCompiledExecutionFixture(t *testing.T, grants execworkspace.GrantSet) sealedCompiledExecutionFixture {
	t.Helper()
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
		".gitignore": ".verdi/data/\n",
	})
	compileRequest, err := contextcompile.DecodeRequest(contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	if err != nil {
		t.Fatal(err)
	}
	compileRequest.Grants = grants
	compiled, err := contextcompile.NewCompiler().Compile(context.Background(), repo.Dir, compileRequest)
	if err != nil {
		t.Fatalf("compile lifecycle fixture: %v", err)
	}
	projection := sealedexec.InstructionProjection{Schema: sealedexec.InstructionProjectionSchemaID, Files: make([]sealedexec.InstructionFile, len(compiled.ProjectionFiles))}
	for i, file := range compiled.ProjectionFiles {
		projection.Files[i] = sealedexec.InstructionFile{Path: file.Path, ContentDigest: file.Digest, Content: string(file.Content)}
	}
	projectionBytes, err := sealedexec.EncodeInstructionProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	projection, err = sealedexec.DecodeInstructionProjection(bytes.NewReader(projectionBytes))
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(contextGitOutput(t, repo.Dir, "rev-parse", "HEAD"))
	tree := strings.TrimSpace(contextGitOutput(t, repo.Dir, "rev-parse", "HEAD^{tree}"))
	workspaceRequest, err := sealedexec.NewExecutionWorkspaceRequest("flight-compiled", "lane-compiled", "epoch-compiled", "session-compiled", head)
	if err != nil {
		t.Fatal(err)
	}
	request := sealedexec.ExecutionRequest{
		Schema: sealedexec.ExecutionRequestSchemaID, Action: sealedexec.ActionStart,
		Flight: "flight-compiled", Lane: "lane-compiled", Epoch: "epoch-compiled", Session: "session-compiled",
		ManifestRevision: 0, ATCRunway: repo.Dir, InputCommit: head, InputTree: tree,
		Manifest: compiled.Manifest, ManifestDigest: compiled.Manifest.Digest,
		InstructionProjection: projection, ProjectionDigest: projection.Digest,
		ExecutionWorkspaceRequest: workspaceRequest, Adapter: contextevent.Adapter(compiled.Manifest.Adapter.ID), AdapterVersion: compiled.Manifest.Adapter.Version,
		Profile: sealedexec.LogicalRef{Schema: sealedexec.ProjectProfileRefSchemaID, ID: "compiled-profile", Digest: sealedTestDigest("compiled-profile")},
		Grants:  compiled.Manifest.Capabilities, AuthorityVerdict: sealedCanonicalAuthorityReport(t, compiled.Manifest.Digest, head),
		RecorderEndpoint: sealedexec.LogicalRef{Schema: sealedexec.RecorderEndpointRefSchemaID, ID: "compiled-recorder", Digest: sealedTestDigest("compiled-recorder")},
		Start:            &sealedexec.StartArm{ExpectedSourceSequence: 1},
	}
	requestBytes, err := sealedexec.EncodeExecutionRequest(request)
	if err != nil {
		t.Fatalf("encode compiled execution request: %v", err)
	}
	return sealedCompiledExecutionFixture{root: repo.Dir, head: head, tree: tree, compiled: compiled, request: request, requestBytes: requestBytes}
}

func runSuccessfulSealedStart(t *testing.T, bin string, outputFile, outputFailure bool, failOperation sealedexec.ControllerOperation, releaseFailure, providerFailure, advisory bool, protectedChange string, providerLaunchFailure bool) {
	t.Helper()
	providerRoot := t.TempDir()
	providerPath := filepath.Join(providerRoot, "codex-fixture")
	fixture := buildCompiledExecutionFixture(t, execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantNetwork},
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{providerPath}},
	}})
	root, compiled, request, requestBytes := fixture.root, fixture.compiled, fixture.request, fixture.requestBytes
	head, tree, workspaceRequest := fixture.head, fixture.tree, request.ExecutionWorkspaceRequest
	workspaceID, err := workspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}

	argvPath := filepath.Join(providerRoot, "argv")
	envPath := filepath.Join(providerRoot, "env")
	stdinPath := filepath.Join(providerRoot, "stdin")
	fd3Path := filepath.Join(providerRoot, "fd3")
	codexHome := filepath.Join(providerRoot, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	gitPath, err = filepath.Abs(gitPath)
	if err != nil {
		t.Fatal(err)
	}
	releaseCommand := ""
	if releaseFailure {
		releaseCommand = fmt.Sprintf("/bin/mkdir -p %q\n", execworkspace.ReleasedPath(root, workspaceID))
	}
	changeCommand := fmt.Sprintf("printf 'provider change\\n' > provider-output.txt\n%q add provider-output.txt\n", gitPath)
	protectedPath := ".verdi/specs/active/feature-alpha/spec.md"
	switch protectedChange {
	case "add":
		addedPath := ".verdi/specs/active/provider-added/spec.md"
		changeCommand = fmt.Sprintf("/bin/mkdir -p .verdi/specs/active/provider-added\nprintf 'provider protected addition\\n' > %q\n%q add %q\n", addedPath, gitPath, addedPath)
	case "rename":
		changeCommand = fmt.Sprintf("%q mv %q provider-renamed-spec.md\n", gitPath, protectedPath)
	case "copy-old":
		changeCommand = fmt.Sprintf("/bin/cp %q provider-copied-spec.md\n%q add provider-copied-spec.md\n", protectedPath, gitPath)
	}
	providerAction := fmt.Sprintf("%sGIT_AUTHOR_DATE='2001-02-03T04:05:06Z' GIT_COMMITTER_DATE='2001-02-03T04:05:06Z' %q -c user.name='Fixture Provider' -c user.email='provider@example.invalid' commit -m 'provider output' >/dev/null 2>&1\n%sprintf '%%s\\n' '{\"type\":\"thread.started\",\"thread_id\":\"0199a213-81c0-7800-8aa1-bbab2a035a53\"}' '{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}'\n", changeCommand, gitPath, releaseCommand)
	if providerFailure {
		providerAction = "printf '%s\\n' '{\"type\":\"thread.started\",\"thread_id\":\"0199a213-81c0-7800-8aa1-bbab2a035a53\"}'\nexit 7\n"
	}
	script := fmt.Sprintf("#!/bin/sh\nset -eu\nif [ -e /dev/fd/3 ]; then\n  if [ -S /dev/fd/3 ]; then printf 'socket\\n' > %q; else printf 'open\\n' > %q; fi\n  exit 97\nfi\nprintf 'closed\\n' > %q\nprintf '%%s\\n' \"$@\" > %q\n/usr/bin/env > %q\n/bin/cat > %q\n%s", fd3Path, fd3Path, fd3Path, argvPath, envPath, stdinPath, providerAction)
	providerMode := os.FileMode(0o755)
	if providerLaunchFailure {
		providerMode = 0o644
	}
	if err := os.WriteFile(providerPath, []byte(script), providerMode); err != nil {
		t.Fatal(err)
	}
	profile := sealedexec.ProfileMaterial{
		Ref: request.Profile, Name: "compiled-fixture", AbsoluteExecutable: providerPath,
		AbsoluteEnvRoot: providerRoot, AbsoluteCodexHome: codexHome,
		AdapterVersion: request.AdapterVersion, DecoderProfile: sealedcodex.DecoderProfileV1,
	}

	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "lifecycle-controller")
	childFile := os.NewFile(uintptr(files[1]), "lifecycle-child")
	controllerConn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatal(err)
	}
	fake := &sealedLifecycleController{t: t, request: request, profile: profile, fail: failOperation, advisory: advisory, allowQuarantine: outputFailure || failOperation != "" || providerFailure || providerLaunchFailure || advisory || protectedChange != ""}
	if failOperation == sealedexec.ControllerOperationRecorderAppend {
		fake.failEventKind = contextevent.KindExecutionResult
	}
	served := make(chan error, 1)
	go func() {
		defer controllerConn.Close()
		served <- fake.serve(controllerConn)
	}()
	t.Setenv("VERDI_HOSTILE_AMBIENT_SECRET", "must-not-cross-provider-boundary")
	requestArg := "-"
	requestInput := requestBytes
	if outputFile {
		requestArg = filepath.Join(providerRoot, "request.json")
		if err := os.WriteFile(requestArg, requestBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		requestInput = nil
	}
	args := []string{"context", "execution", "--request", requestArg}
	resultPath := filepath.Join(providerRoot, "result.json")
	if outputFile {
		if outputFailure {
			resultPath = providerRoot
		}
		args = append(args, "--out", resultPath)
	}
	observation := runSealedContextBinaryWithFiles(t, bin, root, requestInput, []*os.File{childFile}, args...)
	if err := <-served; err != nil {
		t.Fatalf("lifecycle controller: %v; observation=%#v", err, observation)
	}
	if !providerLaunchFailure {
		fd3 := strings.TrimSpace(string(mustReadFile(t, fd3Path)))
		if fd3 != "closed" {
			t.Fatalf("provider descriptor 3 = %q, want no inherited controller capability", fd3)
		}
	}
	for _, operation := range fake.calls {
		if operation == sealedexec.ControllerOperationPersistAbort {
			t.Fatalf("Task3 public execution invented an owner-decision abort call: %v", fake.calls)
		}
	}
	if outputFailure {
		if observation.exitCode != 2 || observation.stdout != "" || !strings.Contains(observation.stderr, "writing public result") {
			t.Fatalf("output failure lifecycle = %#v", observation)
		}
		if !reflect.DeepEqual(fake.quarantines, []sealedexec.QuarantineReason{sealedexec.QuarantineOutputWriteFailed}) {
			t.Fatalf("output failure quarantines = %v", fake.quarantines)
		}
		assertLifecyclePreservation(t, fake, sealedexec.PreservedFinalized)
		if _, err := os.Stat(execworkspace.ReleasedPath(root, workspaceID)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("output failure release marker exists: %v", err)
		}
		return
	}
	if providerFailure || providerLaunchFailure {
		if observation.exitCode != 2 || observation.stdout != "" || observation.stderr == "" {
			t.Fatalf("provider failure lifecycle = %#v", observation)
		}
		if strings.Contains(observation.stderr, providerRoot) || strings.Contains(observation.stderr, providerPath) {
			t.Fatalf("provider failure leaked an absolute profile path: %s", observation.stderr)
		}
		wantQuarantines := []sealedexec.QuarantineReason{sealedexec.QuarantineExecutionIncomplete}
		if !reflect.DeepEqual(fake.quarantines, wantQuarantines) {
			t.Fatalf("provider failure quarantines = %v, want %v", fake.quarantines, wantQuarantines)
		}
		assertLifecyclePreservation(t, fake, sealedexec.PreservedPartial)
		if _, err := os.Stat(execworkspace.ReleasedPath(root, workspaceID)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("provider failure release marker exists: %v", err)
		}
		return
	}
	if advisory {
		if observation.exitCode != 1 || observation.stdout == "" || observation.stderr == "" {
			t.Fatalf("advisory lifecycle = %#v", observation)
		}
		result, err := sealedexec.DecodeExecutionResult(strings.NewReader(observation.stdout))
		if err != nil || result.Authority != contextevent.AuthorityAdvisory {
			t.Fatalf("advisory public result = %#v/%v", result, err)
		}
		if !reflect.DeepEqual(fake.quarantines, []sealedexec.QuarantineReason{sealedexec.QuarantineNonAuthoritative}) {
			t.Fatalf("advisory quarantines = %v", fake.quarantines)
		}
		if _, err := os.Stat(execworkspace.ReleasedPath(root, workspaceID)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("advisory release marker exists: %v", err)
		}
		return
	}
	if protectedChange != "" {
		if observation.exitCode != 1 || observation.stdout == "" || observation.stderr == "" {
			t.Fatalf("protected %s lifecycle = %#v", protectedChange, observation)
		}
		if !reflect.DeepEqual(fake.quarantines, []sealedexec.QuarantineReason{sealedexec.QuarantineProtectedSpecChange}) {
			t.Fatalf("protected %s quarantines = %v", protectedChange, fake.quarantines)
		}
		if runwayHead := strings.TrimSpace(contextGitOutput(t, root, "rev-parse", "HEAD")); runwayHead != head {
			t.Fatalf("protected %s moved runway to %s", protectedChange, runwayHead)
		}
		if _, err := os.Stat(execworkspace.ReleasedPath(root, workspaceID)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("protected %s released workspace: %v", protectedChange, err)
		}
		return
	}
	if failOperation == sealedexec.ControllerOperationPersistHandback {
		if observation.exitCode != 2 || observation.stdout == "" || !strings.Contains(observation.stderr, string(sealedexec.QuarantineHandbackDurabilityFailed)) {
			t.Fatalf("failed handback lifecycle = %#v", observation)
		}
		if !reflect.DeepEqual(fake.quarantines, []sealedexec.QuarantineReason{sealedexec.QuarantineHandbackDurabilityFailed}) ||
			len(fake.calls) == 0 || fake.calls[len(fake.calls)-1] != sealedexec.ControllerOperationPersistQuarantine {
			t.Fatalf("failed handback quarantine/call tail = %v/%v", fake.quarantines, fake.calls)
		}
		assertLifecyclePreservation(t, fake, sealedexec.PreservedFinalized)
		if _, err := os.Stat(execworkspace.ReleasedPath(root, workspaceID)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed handback release marker exists: %v", err)
		}
		return
	}
	if releaseFailure {
		if observation.exitCode != 2 || observation.stdout == "" || !strings.Contains(observation.stderr, "release execution workspace after handback") {
			t.Fatalf("failed release lifecycle = %#v", observation)
		}
		if len(fake.quarantines) != 0 || len(fake.calls) == 0 || fake.calls[len(fake.calls)-1] != sealedexec.ControllerOperationPersistHandback {
			t.Fatalf("failed release controller tail/quarantines = %v/%v", fake.calls, fake.quarantines)
		}
		return
	}
	if failOperation != "" {
		if observation.exitCode != 2 || observation.stdout != "" || !strings.Contains(observation.stderr, string(sealedexec.ControllerErrorUnavailable)) {
			t.Fatalf("failed completion lifecycle = %#v", observation)
		}
		if !reflect.DeepEqual(fake.quarantines, []sealedexec.QuarantineReason{sealedexec.QuarantineTerminalDurabilityFailed}) {
			t.Fatalf("failed completion quarantines = %v", fake.quarantines)
		}
		assertLifecyclePreservation(t, fake, sealedexec.PreservedPartial)
		if _, err := os.Stat(execworkspace.ReleasedPath(root, workspaceID)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed completion release marker exists: %v", err)
		}
		return
	}
	if observation.exitCode != 0 || observation.stderr != "" || (!outputFile && observation.stdout == "") || (outputFile && observation.stdout != "") {
		t.Fatalf("successful lifecycle = %#v", observation)
	}
	resultBytes := []byte(observation.stdout)
	if outputFile {
		resultBytes = mustReadFile(t, resultPath)
	}
	result, err := sealedexec.DecodeExecutionResult(bytes.NewReader(resultBytes))
	if err != nil {
		t.Fatalf("decode execution result: %v", err)
	}
	if result.InputCommit != head || result.OutputCommit == head || result.InputTree != tree || result.OutputTree == tree || !result.Clean {
		t.Fatalf("execution result git identity = %#v", result)
	}
	if runwayHead := strings.TrimSpace(contextGitOutput(t, root, "rev-parse", "HEAD")); runwayHead != result.OutputCommit {
		t.Fatalf("runway HEAD = %s, want handback output %s", runwayHead, result.OutputCommit)
	}
	marker, err := os.ReadFile(execworkspace.ReleasedPath(root, workspaceID))
	if err != nil || len(marker) != 0 {
		t.Fatalf("release marker = %q/%v", marker, err)
	}
	argvBytes, err := os.ReadFile(argvPath)
	if err != nil {
		t.Fatal(err)
	}
	wantArgv := strings.Join([]string{"exec", "--json", "--strict-config", "--ignore-user-config", "--ignore-rules", "--profile", "compiled-fixture", "--sandbox", "workspace-write", "--cd", execworkspace.UnitPath(root, workspaceID), "-", ""}, "\n")
	if string(argvBytes) != wantArgv {
		t.Fatalf("provider argv = %q, want %q", argvBytes, wantArgv)
	}
	envBytes, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(envBytes), "VERDI_HOSTILE_AMBIENT_SECRET") || !strings.Contains(string(envBytes), "CODEX_HOME="+codexHome) {
		t.Fatalf("provider environment crossed ambient boundary: %s", envBytes)
	}
	providerInput, err := sealedexec.DecodeProviderInput(bytes.NewReader(mustReadFile(t, stdinPath)))
	if err != nil {
		t.Fatalf("provider input: %v", err)
	}
	if !reflect.DeepEqual(providerInput.Instructions.Projection, request.InstructionProjection) || !reflect.DeepEqual(providerInput.Data, compiled.DataItems) {
		t.Fatalf("provider input lost compiled authority")
	}
	fake.assertSequence()
}

func runInterruptedSealedStart(t *testing.T, bin string, signalBeforeActivation bool) {
	t.Helper()
	providerRoot := t.TempDir()
	providerPath := filepath.Join(providerRoot, "codex-interrupt-fixture")
	pidPath := filepath.Join(providerRoot, "provider.pid")
	fd3Path := filepath.Join(providerRoot, "fd3")
	fixture := buildCompiledExecutionFixture(t, execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantNetwork},
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{providerPath}},
	}})
	workspaceID, err := fixture.request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	codexHome := filepath.Join(providerRoot, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\nset -eu\nif [ -e /dev/fd/3 ]; then\n  if [ -S /dev/fd/3 ]; then printf 'socket\\n' > %q; else printf 'open\\n' > %q; fi\n  exit 97\nfi\nprintf 'closed\\n' > %q\nprintf '%%s\\n' \"$$\" > %q\nprintf '%%s\\n' '{\"type\":\"thread.started\",\"thread_id\":\"0199a213-81c0-7800-8aa1-bbab2a035a53\"}'\nexec /bin/sleep 60\n", fd3Path, fd3Path, fd3Path, pidPath)
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	profile := sealedexec.ProfileMaterial{
		Ref: fixture.request.Profile, Name: "interrupt-fixture", AbsoluteExecutable: providerPath,
		AbsoluteEnvRoot: providerRoot, AbsoluteCodexHome: codexHome,
		AdapterVersion: fixture.request.AdapterVersion, DecoderProfile: sealedcodex.DecoderProfileV1,
	}

	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "interrupt-controller")
	childFile := os.NewFile(uintptr(files[1]), "interrupt-child")
	controllerConn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatal(err)
	}
	eventObserved := make(chan contextevent.Kind, 4)
	operationPaused := make(chan sealedexec.ControllerOperation, 1)
	operationRelease := make(chan struct{})
	releasedOperation := false
	defer func() {
		if signalBeforeActivation && !releasedOperation {
			close(operationRelease)
		}
	}()
	fake := &sealedLifecycleController{
		t: t, request: fixture.request, profile: profile, allowQuarantine: true,
		eventObserved: eventObserved,
	}
	if signalBeforeActivation {
		fake.pauseBeforeReply = sealedexec.ControllerOperationResolveProfile
		fake.operationPaused = operationPaused
		fake.operationRelease = operationRelease
	}
	served := make(chan error, 1)
	go func() {
		defer controllerConn.Close()
		served <- fake.serve(controllerConn)
	}()

	cmd := exec.Command(bin, "context", "execution", "--request", "-")
	cmd.Dir = fixture.root
	cmd.Stdin = bytes.NewReader(fixture.requestBytes)
	cmd.ExtraFiles = []*os.File{childFile}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		_ = childFile.Close()
		t.Fatal(err)
	}
	_ = childFile.Close()
	var providerPID int
	t.Cleanup(func() {
		if providerPID != 0 && providerPID != os.Getpid() {
			_ = syscall.Kill(providerPID, syscall.SIGKILL)
		}
	})
	if signalBeforeActivation {
		select {
		case operation := <-operationPaused:
			if operation != sealedexec.ControllerOperationResolveProfile {
				t.Fatalf("pre-active pause = %q, want resolve-profile", operation)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("execution did not reach the pre-active controller boundary")
		}
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatalf("send pre-active signal: %v", err)
		}
		time.Sleep(250 * time.Millisecond)
		close(operationRelease)
		releasedOperation = true
	}
	select {
	case kind := <-eventObserved:
		if kind != contextevent.KindAdapterStart {
			t.Fatalf("first provider event = %q, want adapter-start", kind)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not reach acknowledged adapter-start")
	}
	pidBytes := mustReadFile(t, pidPath)
	providerPID, err = strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil || providerPID <= 0 {
		t.Fatalf("provider pid = %q/%v", pidBytes, err)
	}
	if fd3 := strings.TrimSpace(string(mustReadFile(t, fd3Path))); fd3 != "closed" {
		t.Fatalf("provider descriptor 3 = %q, want closed", fd3)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal Verdi process: %v", err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case err = <-waited:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("signaled Verdi process did not terminate")
	}
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("wait signaled Verdi process: %v", err)
		}
		code = exitErr.ExitCode()
	}
	select {
	case serveErr := <-served:
		if serveErr != nil {
			t.Fatalf("interrupt controller: %v; stdout=%q stderr=%q", serveErr, stdout.String(), stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("interrupt controller did not observe connection closure")
	}
	if code != 1 || stdout.String() != "" || !strings.Contains(stderr.String(), "interrupted") {
		t.Fatalf("interrupted execution = code %d stdout %q stderr %q, want verdict/no output", code, stdout.String(), stderr.String())
	}
	if !reflect.DeepEqual(fake.quarantines, []sealedexec.QuarantineReason{sealedexec.QuarantineExecutionIncomplete}) {
		t.Fatalf("interrupt quarantines = %v", fake.quarantines)
	}
	// Amendment 002 §7: the active interrupt/owner transition emits `suspension`
	// then `adapter-stop`, so the exact acknowledged prefix is start, summary,
	// suspension, stop — one suspension and one stop, never a fabricated pair.
	assertSealedEventPrefix(t, fake.events, []contextevent.Kind{
		contextevent.KindAdapterStart, contextevent.KindProviderSummary,
		contextevent.KindSuspension, contextevent.KindAdapterStop,
	})
	if countEventKindInFixture(fake.events, contextevent.KindAdapterStop) != 1 ||
		countEventKindInFixture(fake.events, contextevent.KindSuspension) != 1 {
		t.Fatalf("interrupt events = %#v, want exactly one suspension and one adapter-stop", fake.events)
	}
	if got := fake.calls; countControllerOperation(got, sealedexec.ControllerOperationPersistQuarantine) != 1 ||
		countControllerOperation(got, sealedexec.ControllerOperationPersistAbort) != 0 || countControllerOperation(got, sealedexec.ControllerOperationPersistHandback) != 0 {
		t.Fatalf("interrupt controller operations = %v", got)
	}
	if _, err := os.Stat(execworkspace.ReleasedPath(fixture.root, workspaceID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupt released workspace: %v", err)
	}
	if err := syscall.Kill(providerPID, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("provider process %d survived normalized stop: %v", providerPID, err)
	}
	providerPID = 0
	assertLifecyclePreservation(t, fake, sealedexec.PreservedPartial)
	partial, err := sealedexec.DecodeExecutionPartial(bytes.NewReader(fake.preservedBytes[0]))
	if err != nil {
		t.Fatalf("decode interrupted controller-preserved partial: %v", err)
	}
	if !reflect.DeepEqual(partial.EventAcks, fake.eventAcks) {
		t.Fatalf("interrupted preserved event acknowledgments = %#v, want exact controller-issued sequence %#v", partial.EventAcks, fake.eventAcks)
	}
	for i := range fake.eventAcks {
		got, err := contextevent.EncodeEventAck(partial.EventAcks[i])
		if err != nil {
			t.Fatalf("encode interrupted preserved event_acks[%d]: %v", i, err)
		}
		want, err := contextevent.EncodeEventAck(fake.eventAcks[i])
		if err != nil {
			t.Fatalf("encode controller-issued event_acks[%d]: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("interrupted preserved event_acks[%d] bytes = %q, want exact controller-issued bytes %q", i, got, want)
		}
	}
}

func countControllerOperation(operations []sealedexec.ControllerOperation, want sealedexec.ControllerOperation) int {
	count := 0
	for _, operation := range operations {
		if operation == want {
			count++
		}
	}
	return count
}

func countEventKindInFixture(events []contextevent.Event, want contextevent.Kind) int {
	count := 0
	for _, event := range events {
		if event.Kind == want {
			count++
		}
	}
	return count
}

func runSuccessfulSealedResume(t *testing.T, bin string) {
	t.Helper()
	providerRoot := t.TempDir()
	providerPath := filepath.Join(providerRoot, "codex-resume-fixture")
	fixture := buildCompiledExecutionFixture(t, execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantNetwork},
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{providerPath}},
	}})
	materializer, err := execworkspace.NewMaterializer(fixture.root, fixture.root, execworkspace.NewGitReconciler(fixture.root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Materialize(context.Background(), execworkspace.Request{Identity: fixture.request.ExecutionWorkspaceRequest}); err != nil {
		t.Fatalf("materialize resume workspace: %v", err)
	}
	workspaceID, err := fixture.request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatal(err)
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
		Schema: contextevent.RevisionSchemaID, ManifestRevision: fixture.request.ManifestRevision, ManifestDigest: fixture.request.ManifestDigest,
		FirstGlobalSequence: 1, TerminalGlobalSequence: 3, TerminalSourceSequence: 3,
		TerminalKind: contextevent.KindExecutionResult, EventRoot: sealedTestDigest("resume-prior-event"),
	}
	revisionRoot, err := contextevent.EventChainRoot([]contextevent.Revision{revision})
	if err != nil {
		t.Fatal(err)
	}
	const sessionRef = "0199a213-81c0-7800-8aa1-bbab2a035a53"
	checkpointDigest := sealedTestDigest("resume-checkpoint")
	expansionRoot := sealedTestDigest("resume-expansion-root")
	continuity := sealedexec.ExecutionContinuity{
		Schema: sealedexec.ExecutionContinuitySchemaID,
		Flight: fixture.request.Flight, Lane: fixture.request.Lane, Epoch: fixture.request.Epoch, Session: fixture.request.Session,
		Adapter: fixture.request.Adapter, AdapterVersion: fixture.request.AdapterVersion, ATCRunway: fixture.root,
		InputCommit: fixture.head, InputTree: fixture.tree, CurrentCommit: fixture.head, CurrentTree: fixture.tree,
		ExecutionWorkspaceID: workspaceID, ExecutionWorkspaceRequestDigest: workspaceDigest,
		ProfileDigest: fixture.request.Profile.Digest, GrantDigest: sealedRawDigest(grantBytes), AuthorityVerdictDigest: fixture.request.AuthorityVerdict.Digest,
		CurrentManifestRevision: fixture.request.ManifestRevision, CurrentManifestDigest: fixture.request.ManifestDigest, ProjectionDigest: fixture.request.ProjectionDigest,
		RevisionSegments: []contextevent.Revision{revision}, EventChainRoot: revisionRoot, ExpansionLedgerRoot: expansionRoot,
		TerminalSourceSequence: revision.TerminalSourceSequence, TerminalGlobalSequence: revision.TerminalGlobalSequence,
		RecorderCheckpointDigest: checkpointDigest, AdapterSessionRef: sessionRef,
	}
	continuityBytes, err := sealedexec.EncodeExecutionContinuity(continuity)
	if err != nil {
		t.Fatalf("encode resume continuity: %v", err)
	}
	continuity, err = sealedexec.DecodeExecutionContinuity(bytes.NewReader(continuityBytes))
	if err != nil {
		t.Fatal(err)
	}
	request := fixture.request
	request.Action = sealedexec.ActionResume
	request.Start = nil
	request.Resume = &sealedexec.ResumeArm{Continuity: continuity, ContinuityDigest: continuity.Digest}
	requestBytes, err := sealedexec.EncodeExecutionRequest(request)
	if err != nil {
		t.Fatalf("encode resume request: %v", err)
	}

	argvPath := filepath.Join(providerRoot, "argv")
	stdinPath := filepath.Join(providerRoot, "stdin")
	codexHome := filepath.Join(providerRoot, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	gitPath, err = filepath.Abs(gitPath)
	if err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\nset -eu\nprintf '%%s\\n' \"$@\" > %q\n/bin/cat > %q\nprintf 'resume provider change\\n' > resume-output.txt\n%q add resume-output.txt\nGIT_AUTHOR_DATE='2001-02-03T05:06:07Z' GIT_COMMITTER_DATE='2001-02-03T05:06:07Z' %q -c user.name='Fixture Provider' -c user.email='provider@example.invalid' commit -m 'resume provider output' >/dev/null 2>&1\nprintf '%%s\\n' '{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}'\n", argvPath, stdinPath, gitPath, gitPath)
	if err := os.WriteFile(providerPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	profile := sealedexec.ProfileMaterial{
		Ref: request.Profile, Name: "compiled-fixture", AbsoluteExecutable: providerPath,
		AbsoluteEnvRoot: providerRoot, AbsoluteCodexHome: codexHome,
		AdapterVersion: request.AdapterVersion, DecoderProfile: sealedcodex.DecoderProfileV1,
	}
	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "resume-controller")
	childFile := os.NewFile(uintptr(files[1]), "resume-child")
	controllerConn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatal(err)
	}
	fake := &sealedLifecycleController{
		t: t, request: request, profile: profile, initialRevisions: []contextevent.Revision{revision},
		checkpointDigest: checkpointDigest, expansionRoot: expansionRoot, global: revision.TerminalGlobalSequence,
	}
	served := make(chan error, 1)
	go func() {
		defer controllerConn.Close()
		served <- fake.serve(controllerConn)
	}()
	observation := runSealedContextBinaryWithFiles(t, bin, fixture.root, requestBytes, []*os.File{childFile}, "context", "execution", "--request", "-")
	if err := <-served; err != nil {
		t.Fatalf("resume controller: %v; observation=%#v", err, observation)
	}
	if observation.exitCode != 0 || observation.stderr != "" || observation.stdout == "" {
		t.Fatalf("resume lifecycle = %#v", observation)
	}
	result, err := sealedexec.DecodeExecutionResult(strings.NewReader(observation.stdout))
	if err != nil {
		t.Fatal(err)
	}
	if result.InputCommit != fixture.head || result.OutputCommit == fixture.head || !result.Clean {
		t.Fatalf("resume result = %#v", result)
	}
	argvBytes := mustReadFile(t, argvPath)
	wantArgv := strings.Join([]string{"exec", "resume", "--json", "--strict-config", "--ignore-user-config", "--ignore-rules", sessionRef, "-", ""}, "\n")
	if string(argvBytes) != wantArgv {
		t.Fatalf("resume provider argv = %q, want %q", argvBytes, wantArgv)
	}
	if _, err := sealedexec.DecodeProviderInput(bytes.NewReader(mustReadFile(t, stdinPath))); err != nil {
		t.Fatalf("resume provider input: %v", err)
	}
	marker, err := os.ReadFile(execworkspace.ReleasedPath(fixture.root, workspaceID))
	if err != nil || len(marker) != 0 {
		t.Fatalf("resume release marker = %q/%v", marker, err)
	}
	wantCalls := []sealedexec.ControllerOperation{
		sealedexec.ControllerOperationVerifyAuthority,
		sealedexec.ControllerOperationResolveProfile,
		sealedexec.ControllerOperationVerifyConflict,
		sealedexec.ControllerOperationResolveRecorder,
		sealedexec.ControllerOperationRecorderCheckpoint,
		sealedexec.ControllerOperationVerifyExpansion,
		sealedexec.ControllerOperationVerifyProviderSession,
		sealedexec.ControllerOperationVerifyOpaqueBoundary,
		// Amendment 002 §7 prepared-resume: `resume` then `adapter-start`, each
		// stamped and acknowledged before the provider stream is reduced.
		sealedexec.ControllerOperationNextStamp,
		sealedexec.ControllerOperationRecorderAppend,
		sealedexec.ControllerOperationNextStamp,
		sealedexec.ControllerOperationRecorderAppend,
		sealedexec.ControllerOperationVerifyProviderSession,
		sealedexec.ControllerOperationNextStamp,
		sealedexec.ControllerOperationRecorderAppend,
		sealedexec.ControllerOperationRecorderCheckpoint,
		sealedexec.ControllerOperationResolveReceiptInputs,
		sealedexec.ControllerOperationNextStamp,
		sealedexec.ControllerOperationAppendReceipt,
		sealedexec.ControllerOperationPersistHandback,
	}
	if !reflect.DeepEqual(fake.calls, wantCalls) {
		t.Fatalf("resume controller sequence = %v, want %v", fake.calls, wantCalls)
	}
	// The extra acknowledgment is Amendment 002 §7's explicit `resume` event,
	// not a duplicated adapter-start. This Codex fixture's provider stream
	// carries no session-init family, so no adapter-start follows it.
	assertSealedEventPrefix(t, fake.events, []contextevent.Kind{
		contextevent.KindResume, contextevent.KindProviderSummary, contextevent.KindExecutionResult,
	})
	if fake.events[0].SourceSequence != revision.TerminalSourceSequence+1 {
		t.Fatalf("resume first acknowledged source sequence = %d, want continuation of %d", fake.events[0].SourceSequence, revision.TerminalSourceSequence)
	}
}

// assertSealedEventPrefix proves the acknowledged events are exactly the given
// kinds in order, on contiguous source sequences starting at the first
// acknowledged sequence. It fails on a missing, extra, reordered, or
// resequenced acknowledgment.
func assertSealedEventPrefix(t *testing.T, events []contextevent.Event, want []contextevent.Kind) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("acknowledged events = %d kinds %v, want %d kinds %v", len(events), sealedEventKinds(events), len(want), want)
	}
	for i, kind := range want {
		if events[i].Kind != kind {
			t.Fatalf("acknowledged event kinds = %v, want %v", sealedEventKinds(events), want)
		}
		if i > 0 && events[i].SourceSequence != events[i-1].SourceSequence+1 {
			t.Fatalf("acknowledged source sequences = %v, want contiguous", sealedEventSequences(events))
		}
	}
}

func sealedEventKinds(events []contextevent.Event) []contextevent.Kind {
	kinds := make([]contextevent.Kind, len(events))
	for i, event := range events {
		kinds[i] = event.Kind
	}
	return kinds
}

func sealedEventSequences(events []contextevent.Event) []uint64 {
	sequences := make([]uint64, len(events))
	for i, event := range events {
		sequences[i] = event.SourceSequence
	}
	return sequences
}

func runSuccessfulScopedMCP(t *testing.T, bin string, pathMode bool) {
	t.Helper()
	fixture := buildCompiledExecutionFixture(t, execworkspace.GrantSet{Grants: []execworkspace.Grant{}})
	materializer, err := execworkspace.NewMaterializer(fixture.root, fixture.root, execworkspace.NewGitReconciler(fixture.root))
	if err != nil {
		t.Fatal(err)
	}
	materialized, err := materializer.Materialize(context.Background(), execworkspace.Request{Identity: fixture.request.ExecutionWorkspaceRequest})
	if err != nil {
		t.Fatalf("materialize scoped MCP workspace: %v", err)
	}
	if materialized.Path == "" {
		t.Fatal("materialized scoped MCP workspace has no path")
	}

	frames := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_flight_plan","arguments":{}}}`,
	}, "\n") + "\n"
	requestArg := "-"
	stdin := append(append([]byte(nil), fixture.requestBytes...), []byte(frames)...)
	if pathMode {
		requestArg = writeContextRequestFile(t, t.TempDir(), "request.json", fixture.requestBytes)
		stdin = []byte(frames)
	}

	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "mcp-controller")
	childFile := os.NewFile(uintptr(files[1]), "mcp-child")
	controllerConn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatal(err)
	}
	fake := &sealedLifecycleController{t: t, request: fixture.request}
	served := make(chan error, 1)
	go func() {
		defer controllerConn.Close()
		served <- fake.serve(controllerConn)
	}()
	observation := runSealedContextBinaryWithFiles(t, bin, fixture.root, stdin, []*os.File{childFile}, "context", "mcp", "--request", requestArg)
	if err := <-served; err != nil {
		t.Fatalf("scoped MCP controller: %v; observation=%#v", err, observation)
	}
	if observation.exitCode != 0 || observation.stderr != "" {
		t.Fatalf("scoped MCP lifecycle = %#v", observation)
	}
	lines := strings.Split(strings.TrimSpace(observation.stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("scoped MCP response frames = %d\n%s", len(lines), observation.stdout)
	}
	var listed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &listed); err != nil {
		t.Fatal(err)
	}
	wantTools := []string{sealedexec.ToolGetFlightPlan, sealedexec.ToolRequestContext}
	gotTools := make([]string, len(listed.Result.Tools))
	for i, tool := range listed.Result.Tools {
		gotTools[i] = tool.Name
	}
	if !reflect.DeepEqual(gotTools, wantTools) {
		t.Fatalf("scoped MCP tools = %v, want %v", gotTools, wantTools)
	}
	var called struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &called); err != nil {
		t.Fatal(err)
	}
	if len(called.Result.Content) != 1 {
		t.Fatalf("get_flight_plan content = %#v", called.Result.Content)
	}
	inspection, err := sealedexec.DecodeInspectionResult(bytes.NewReader([]byte(called.Result.Content[0].Text)))
	if err != nil {
		t.Fatalf("decode get_flight_plan: %v", err)
	}
	if inspection.Kind != sealedexec.InspectionFlightPlan || inspection.FlightPlan.ManifestRevision != fixture.request.ManifestRevision || inspection.FlightPlan.ExpansionRoot != "" {
		t.Fatalf("get_flight_plan = %#v", inspection)
	}
	wantCalls := []sealedexec.ControllerOperation{
		sealedexec.ControllerOperationVerifyAuthority,
		sealedexec.ControllerOperationResolveRecorder,
		sealedexec.ControllerOperationRecorderCheckpoint,
		sealedexec.ControllerOperationVerifyExpansion,
	}
	if !reflect.DeepEqual(fake.calls, wantCalls) {
		t.Fatalf("scoped MCP controller sequence = %v, want %v", fake.calls, wantCalls)
	}
}

func runScopedMCPContextScenario(t *testing.T, bin string, kind sealedexec.InspectionKind, wantExit int) {
	t.Helper()
	fixture := buildCompiledExecutionFixture(t, execworkspace.GrantSet{Grants: []execworkspace.Grant{}})
	materializer, err := execworkspace.NewMaterializer(fixture.root, fixture.root, execworkspace.NewGitReconciler(fixture.root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Materialize(context.Background(), execworkspace.Request{Identity: fixture.request.ExecutionWorkspaceRequest}); err != nil {
		t.Fatalf("materialize scoped MCP workspace: %v", err)
	}
	if len(fixture.compiled.DataItems) == 0 {
		t.Fatal("compiled MCP fixture contains no context data")
	}
	proven := sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}}
	fake := &sealedLifecycleController{
		t: t, request: fixture.request,
		resolution: sealedexec.ContextResolution{Verification: proven, Data: fixture.compiled.DataItems[0]},
		epoch:      proven,
	}
	switch kind {
	case sealedexec.InspectionContextDenied:
		fake.resolution = sealedexec.ContextResolution{Verification: sealedexec.Verification{State: contextcompile.ResolutionUnproven, Failure: sealedexec.FailureUnproven, Witnesses: []string{"fixture context unavailable"}}, Data: fixture.compiled.DataItems[0]}
	case sealedexec.InspectionEpochInvalidated:
		fake.epoch = sealedexec.Verification{State: contextcompile.ResolutionViolatedWithWitness, Failure: sealedexec.FailureRejected, Witnesses: []string{"fixture epoch moved"}}
	}

	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "mcp-context-controller")
	childFile := os.NewFile(uintptr(files[1]), "mcp-context-child")
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
	frame := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"request_context","arguments":{"purpose":"fixture expansion","ref":"spec/extra"}}}` + "\n"
	stdin := append(append([]byte(nil), fixture.requestBytes...), []byte(frame)...)
	observation := runSealedContextBinaryWithFiles(t, bin, fixture.root, stdin, []*os.File{childFile}, "context", "mcp", "--request", "-")
	if err := <-served; err != nil {
		t.Fatalf("scoped MCP context controller: %v; observation=%#v", err, observation)
	}
	if observation.exitCode != wantExit || observation.stderr != "" {
		t.Fatalf("scoped MCP context scenario = %#v, want exit %d", observation, wantExit)
	}
	var response struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(observation.stdout)), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Result.Content) != 1 {
		t.Fatalf("context response content = %#v", response.Result.Content)
	}
	inspection, err := sealedexec.DecodeInspectionResult(bytes.NewReader([]byte(response.Result.Content[0].Text)))
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Kind != kind {
		t.Fatalf("context inspection kind = %q, want %q", inspection.Kind, kind)
	}
	wantKinds := []contextevent.Kind{contextevent.KindContextRequest, contextevent.KindContextDecision}
	if kind == sealedexec.InspectionContextApproved {
		wantKinds = append(wantKinds, contextevent.KindChildManifest)
		if inspection.Context.Data.Digest == "" || inspection.Context.ChildRevision != fixture.request.ManifestRevision+1 {
			t.Fatalf("approved context inspection = %#v", inspection)
		}
	}
	gotKinds := make([]contextevent.Kind, len(fake.events))
	for i, event := range fake.events {
		gotKinds[i] = event.Kind
	}
	if !reflect.DeepEqual(gotKinds, wantKinds) {
		t.Fatalf("context event kinds = %v, want %v", gotKinds, wantKinds)
	}
	if kind == sealedexec.InspectionContextApproved && fake.calls[len(fake.calls)-1] != sealedexec.ControllerOperationInstallExpansion {
		t.Fatalf("approved context did not install after terminal ack: %v", fake.calls)
	}
}

func runScopedMCPLaterCheckpoint(t *testing.T, bin, mutation string, wantExit int) {
	t.Helper()
	fixture := buildCompiledExecutionFixture(t, execworkspace.GrantSet{Grants: []execworkspace.Grant{}})
	materializer, err := execworkspace.NewMaterializer(fixture.root, fixture.root, execworkspace.NewGitReconciler(fixture.root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Materialize(context.Background(), execworkspace.Request{Identity: fixture.request.ExecutionWorkspaceRequest}); err != nil {
		t.Fatal(err)
	}
	bridge := &contextevent.PriorRevision{
		ManifestRevision: fixture.request.ManifestRevision, ManifestDigest: fixture.request.ManifestDigest,
		EventRoot: sealedTestDigest("expansion-closed-event"), TerminalSourceSequence: 3, TerminalGlobalSequence: 7,
	}
	active := &sealedexec.ActiveRevision{
		Revision: fixture.request.ManifestRevision + 1, ManifestDigest: sealedTestDigest("child-manifest"),
		NextSourceSequence: 1, PriorRevision: bridge, LastGlobalSequence: bridge.TerminalGlobalSequence,
		// I-110 grew the active tail with the exact acknowledged prefix; a
		// sequence-one tail carries an empty, never null, acknowledgment array.
		EventAcks: []contextevent.EventAck{},
	}
	switch mutation {
	case "":
	case "missing":
		active.PriorRevision = nil
		active.LastGlobalSequence = 0
	case "wrong-manifest":
		active.PriorRevision.ManifestDigest = sealedTestDigest("wrong-parent-manifest")
	default:
		t.Fatalf("unknown later checkpoint mutation %q", mutation)
	}
	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "mcp-later-controller")
	childFile := os.NewFile(uintptr(files[1]), "mcp-later-child")
	controllerConn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatal(err)
	}
	fake := &sealedLifecycleController{
		t: t, request: fixture.request, activeRevision: active,
		expansionRoot: sealedTestDigest("installed-expansion-root"),
	}
	served := make(chan error, 1)
	go func() {
		defer controllerConn.Close()
		served <- fake.serve(controllerConn)
	}()
	frame := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	stdin := append(append([]byte(nil), fixture.requestBytes...), []byte(frame)...)
	observation := runSealedContextBinaryWithFiles(t, bin, fixture.root, stdin, []*os.File{childFile}, "context", "mcp", "--request", "-")
	if err := <-served; err != nil {
		t.Fatalf("later checkpoint controller: %v; observation=%#v", err, observation)
	}
	if observation.exitCode != wantExit {
		t.Fatalf("later checkpoint observation = %#v, want exit %d", observation, wantExit)
	}
	if wantExit == 0 {
		if observation.stderr != "" || !strings.Contains(observation.stdout, `"get_flight_plan"`) || !strings.Contains(observation.stdout, `"request_context"`) {
			t.Fatalf("valid later checkpoint observation = %#v", observation)
		}
	} else if observation.stdout != "" || !strings.Contains(observation.stderr, "predecessor bridge") {
		t.Fatalf("invalid later checkpoint observation = %#v", observation)
	}
}

func runScopedMCPProtocolFailure(t *testing.T, bin string) {
	t.Helper()
	fixture := buildCompiledExecutionFixture(t, execworkspace.GrantSet{Grants: []execworkspace.Grant{}})
	materializer, err := execworkspace.NewMaterializer(fixture.root, fixture.root, execworkspace.NewGitReconciler(fixture.root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Materialize(context.Background(), execworkspace.Request{Identity: fixture.request.ExecutionWorkspaceRequest}); err != nil {
		t.Fatal(err)
	}
	mutatingParams := `"params":{"name":"request_context","arguments":{"ref":"spec/extra","purpose":"needed"}}`
	for _, test := range []struct {
		name  string
		frame string
	}{
		{name: "unparseable", frame: `{not-json}`},
		{name: "missing jsonrpc", frame: `{"id":1,"method":"tools/call",` + mutatingParams + `}`},
		{name: "wrong jsonrpc", frame: `{"jsonrpc":"1.0","id":1,"method":"tools/call",` + mutatingParams + `}`},
		{name: "unknown envelope field", frame: `{"jsonrpc":"2.0","id":1,"method":"tools/call","unexpected":true,` + mutatingParams + `}`},
		{name: "duplicate envelope field", frame: `{"jsonrpc":"2.0","id":1,"method":"tools/list","method":"tools/call",` + mutatingParams + `}`},
		{name: "duplicate call name", frame: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_flight_plan","name":"request_context","arguments":{"ref":"spec/extra","purpose":"needed"}}}`},
		{name: "duplicate call arguments", frame: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"request_context","arguments":{},"arguments":{"ref":"spec/extra","purpose":"needed"}}}`},
		{name: "trailing envelope value", frame: `{"jsonrpc":"2.0","id":1,"method":"tools/call",` + mutatingParams + `}{"jsonrpc":"2.0"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
			if err != nil {
				t.Fatal(err)
			}
			controllerFile := os.NewFile(uintptr(files[0]), "mcp-protocol-controller")
			childFile := os.NewFile(uintptr(files[1]), "mcp-protocol-child")
			controllerConn, err := net.FileConn(controllerFile)
			_ = controllerFile.Close()
			if err != nil {
				_ = childFile.Close()
				t.Fatal(err)
			}
			fake := &sealedLifecycleController{t: t, request: fixture.request}
			served := make(chan error, 1)
			go func() {
				defer controllerConn.Close()
				served <- fake.serve(controllerConn)
			}()
			stdin := append(append([]byte(nil), fixture.requestBytes...), []byte(test.frame+"\n")...)
			observation := runSealedContextBinaryWithFiles(t, bin, fixture.root, stdin, []*os.File{childFile}, "context", "mcp", "--request", "-")
			if err := <-served; err != nil {
				t.Fatalf("scoped MCP protocol controller: %v; observation=%#v", err, observation)
			}
			if observation.exitCode != 2 || observation.stderr != "" || !strings.Contains(observation.stdout, `"code":-32700`) {
				t.Fatalf("malformed scoped MCP protocol = %#v", observation)
			}
			if len(fake.events) != 0 || countControllerOperation(fake.calls, sealedexec.ControllerOperationInstallExpansion) != 0 {
				t.Fatalf("malformed scoped MCP mutated events/install calls = %v/%v", fake.events, fake.calls)
			}
		})
	}
}

func runScopedMCPRecorderRejection(t *testing.T, bin string) {
	t.Helper()
	fixture := buildCompiledExecutionFixture(t, execworkspace.GrantSet{Grants: []execworkspace.Grant{}})
	materializer, err := execworkspace.NewMaterializer(fixture.root, fixture.root, execworkspace.NewGitReconciler(fixture.root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Materialize(context.Background(), execworkspace.Request{Identity: fixture.request.ExecutionWorkspaceRequest}); err != nil {
		t.Fatal(err)
	}
	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "mcp-recorder-controller")
	childFile := os.NewFile(uintptr(files[1]), "mcp-recorder-child")
	controllerConn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatal(err)
	}
	proven := sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}}
	fake := &sealedLifecycleController{
		t: t, request: fixture.request, fail: sealedexec.ControllerOperationRecorderAppend,
		resolution: sealedexec.ContextResolution{Verification: proven, Data: fixture.compiled.DataItems[0]}, epoch: proven,
	}
	served := make(chan error, 1)
	go func() {
		defer controllerConn.Close()
		served <- fake.serve(controllerConn)
	}()
	frame := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"request_context","arguments":{"purpose":"fixture expansion","ref":"spec/extra"}}}` + "\n"
	stdin := append(append([]byte(nil), fixture.requestBytes...), []byte(frame)...)
	observation := runSealedContextBinaryWithFiles(t, bin, fixture.root, stdin, []*os.File{childFile}, "context", "mcp", "--request", "-")
	if err := <-served; err != nil {
		t.Fatalf("scoped MCP recorder controller: %v; observation=%#v", err, observation)
	}
	if observation.exitCode != 2 || observation.stderr != "" || !strings.Contains(observation.stdout, `"isError":true`) || !strings.Contains(observation.stdout, string(sealedexec.ControllerErrorUnavailable)) {
		t.Fatalf("scoped MCP recorder rejection = %#v", observation)
	}
	if len(fake.events) != 0 {
		t.Fatalf("rejected recorder append mutated fake event ledger: %v", fake.events)
	}
}

func runScopedMCPUndeclaredTool(t *testing.T, bin string) {
	t.Helper()
	fixture := buildCompiledExecutionFixture(t, execworkspace.GrantSet{Grants: []execworkspace.Grant{}})
	materializer, err := execworkspace.NewMaterializer(fixture.root, fixture.root, execworkspace.NewGitReconciler(fixture.root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := materializer.Materialize(context.Background(), execworkspace.Request{Identity: fixture.request.ExecutionWorkspaceRequest}); err != nil {
		t.Fatal(err)
	}
	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "mcp-undeclared-controller")
	childFile := os.NewFile(uintptr(files[1]), "mcp-undeclared-child")
	controllerConn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatal(err)
	}
	fake := &sealedLifecycleController{t: t, request: fixture.request}
	served := make(chan error, 1)
	go func() {
		defer controllerConn.Close()
		served <- fake.serve(controllerConn)
	}()
	frame := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"undeclared_tool","arguments":{}}}` + "\n"
	stdin := append(append([]byte(nil), fixture.requestBytes...), []byte(frame)...)
	observation := runSealedContextBinaryWithFiles(t, bin, fixture.root, stdin, []*os.File{childFile}, "context", "mcp", "--request", "-")
	if err := <-served; err != nil {
		t.Fatalf("scoped MCP undeclared-tool controller: %v; observation=%#v", err, observation)
	}
	if observation.exitCode != 2 || observation.stderr != "" || !strings.Contains(observation.stdout, `"isError":true`) || !strings.Contains(observation.stdout, "unknown scoped tool") {
		t.Fatalf("undeclared scoped MCP tool = %#v", observation)
	}
	if len(fake.events) != 0 {
		t.Fatalf("undeclared scoped MCP tool mutated event ledger: %v", fake.events)
	}
	wantCalls := []sealedexec.ControllerOperation{
		sealedexec.ControllerOperationVerifyAuthority,
		sealedexec.ControllerOperationResolveRecorder,
		sealedexec.ControllerOperationRecorderCheckpoint,
		sealedexec.ControllerOperationVerifyExpansion,
	}
	if !reflect.DeepEqual(fake.calls, wantCalls) {
		t.Fatalf("undeclared scoped MCP controller sequence = %v, want %v", fake.calls, wantCalls)
	}
}

func runSealedControllerContradiction(t *testing.T, bin, mismatch string) {
	t.Helper()
	providerRoot := t.TempDir()
	providerPath := filepath.Join(providerRoot, "provider-must-not-run")
	startedPath := filepath.Join(providerRoot, "started")
	if err := os.WriteFile(providerPath, []byte(fmt.Sprintf("#!/bin/sh\nprintf started > %q\n", startedPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	codexHome := filepath.Join(providerRoot, "codex-home")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture := buildCompiledExecutionFixture(t, execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantNetwork},
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{providerPath}},
	}})
	profile := sealedexec.ProfileMaterial{
		Ref: fixture.request.Profile, Name: "contradiction-fixture", AbsoluteExecutable: providerPath,
		AbsoluteEnvRoot: providerRoot, AbsoluteCodexHome: codexHome,
		AdapterVersion: fixture.request.AdapterVersion, DecoderProfile: sealedcodex.DecoderProfileV1,
	}
	if mismatch == "profile environment" {
		blocked := filepath.Join(providerRoot, "not-a-directory")
		if err := os.WriteFile(blocked, []byte("blocked\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		profile.AbsoluteEnvRoot = filepath.Join(blocked, "environment")
	}
	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "contradiction-controller")
	childFile := os.NewFile(uintptr(files[1]), "contradiction-child")
	controllerConn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatal(err)
	}
	fake := &sealedLifecycleController{t: t, request: fixture.request, profile: profile, profileMismatch: mismatch == "profile", recorderMismatch: mismatch == "recorder"}
	served := make(chan error, 1)
	go func() {
		defer controllerConn.Close()
		served <- fake.serve(controllerConn)
	}()
	observation := runSealedContextBinaryWithFiles(t, bin, fixture.root, fixture.requestBytes, []*os.File{childFile}, "context", "execution", "--request", "-")
	if err := <-served; err != nil {
		t.Fatalf("%s contradiction controller: %v; observation=%#v", mismatch, err, observation)
	}
	wantExit := 1
	if mismatch == "profile environment" {
		wantExit = 2
	}
	if observation.exitCode != wantExit || observation.stdout != "" || observation.stderr == "" || strings.Contains(observation.stderr, fixture.root) || strings.Contains(observation.stderr, providerRoot) {
		t.Fatalf("%s contradiction = %#v, want redacted verdict refusal", mismatch, observation)
	}
	if _, err := os.Stat(startedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s contradiction started provider: %v", mismatch, err)
	}
	workspaceID, err := fixture.request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(execworkspace.ReleasedPath(fixture.root, workspaceID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s contradiction released workspace: %v", mismatch, err)
	}
	wantCalls := []sealedexec.ControllerOperation{sealedexec.ControllerOperationVerifyAuthority, sealedexec.ControllerOperationResolveProfile}
	if mismatch == "recorder" {
		wantCalls = append(wantCalls, sealedexec.ControllerOperationVerifyConflict, sealedexec.ControllerOperationResolveRecorder)
	}
	if !reflect.DeepEqual(fake.calls, wantCalls) {
		t.Fatalf("%s contradiction controller sequence = %v, want %v", mismatch, fake.calls, wantCalls)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type sealedLifecycleController struct {
	t                 *testing.T
	request           sealedexec.ExecutionRequest
	profile           sealedexec.ProfileMaterial
	resolution        sealedexec.ContextResolution
	epoch             sealedexec.Verification
	fail              sealedexec.ControllerOperation
	failEventKind     contextevent.Kind
	advisory          bool
	profileMismatch   bool
	recorderMismatch  bool
	allowQuarantine   bool
	quarantines       []sealedexec.QuarantineReason
	quarantineRecords []sealedexec.QuarantineRecord
	preservedBytes    [][]byte
	initialRevisions  []contextevent.Revision
	checkpointDigest  string
	expansionRoot     string
	calls             []sealedexec.ControllerOperation
	events            []contextevent.Event
	eventAcks         []contextevent.EventAck
	global            uint64
	eventObserved     chan<- contextevent.Kind
	activeRevision    *sealedexec.ActiveRevision
	pauseBeforeReply  sealedexec.ControllerOperation
	operationPaused   chan<- sealedexec.ControllerOperation
	operationRelease  <-chan struct{}
}

func (f *sealedLifecycleController) serve(conn net.Conn) error {
	reader := bufio.NewReader(conn)
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
		if call.CallSequence != uint64(len(f.calls)+1) {
			return fmt.Errorf("controller call sequence %d, want %d", call.CallSequence, len(f.calls)+1)
		}
		f.calls = append(f.calls, call.Operation)
		if call.Operation == f.pauseBeforeReply {
			if f.operationPaused != nil {
				f.operationPaused <- call.Operation
			}
			if f.operationRelease != nil {
				<-f.operationRelease
			}
		}
		result, err := f.result(call)
		if err != nil {
			return err
		}
		encoded, err := sealedexec.EncodeControllerResult(result)
		if err != nil {
			return fmt.Errorf("encode %s result: %w", call.Operation, err)
		}
		if _, err := conn.Write(encoded); err != nil {
			return fmt.Errorf("write %s result: %w", call.Operation, err)
		}
		if f.eventObserved != nil && call.Operation == sealedexec.ControllerOperationRecorderAppend {
			f.eventObserved <- call.RecorderAppend.Event.Kind
		}
	}
}

func (f *sealedLifecycleController) result(call sealedexec.ControllerCall) (sealedexec.ControllerResult, error) {
	result := sealedexec.ControllerResult{Schema: sealedexec.ControllerResultSchemaID, CallSequence: call.CallSequence, Operation: call.Operation}
	if call.Operation == f.fail && (call.Operation != sealedexec.ControllerOperationRecorderAppend || f.failEventKind == "" || call.RecorderAppend.Event.Kind == f.failEventKind) {
		result.Error = &sealedexec.ControllerError{
			Schema: sealedexec.ControllerErrorSchemaID, Class: sealedexec.ControllerErrorClassOperational,
			Code: sealedexec.ControllerErrorUnavailable, Witnesses: []string{"fixture controller rejection"},
		}
		return result, nil
	}
	schema := "verdi.context-controller/" + string(call.Operation) + "-result/v1"
	proven := sealedexec.Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}}
	switch call.Operation {
	case sealedexec.ControllerOperationVerifyAuthority:
		if !reflect.DeepEqual(call.VerifyAuthority.Request, f.request) {
			return result, errors.New("verify-authority request mismatch")
		}
		result.VerifyAuthority = sealedexec.ControllerVerifyAuthorityResult{Schema: schema, Facts: sealedexec.AuthorityFacts{
			Verification: proven, ManifestRevision: f.request.ManifestRevision, ManifestDigest: f.request.ManifestDigest,
			ProjectionDigest: f.request.ProjectionDigest, AuthorityDigest: f.request.AuthorityVerdict.Digest,
			AcceptedSpecCommit: f.request.Manifest.AcceptedSpec.Commit,
		}}
	case sealedexec.ControllerOperationResolveProfile:
		material := f.profile
		if f.profileMismatch {
			material.AdapterVersion = "2.0.0"
		}
		result.ResolveProfile = sealedexec.ControllerResolveProfileResult{Schema: schema, Material: material}
	case sealedexec.ControllerOperationVerifyConflict:
		verification := proven
		if f.advisory {
			verification = sealedexec.Verification{State: contextcompile.ResolutionUnproven, Failure: sealedexec.FailureUnproven, Witnesses: []string{"fixture conflict proof unavailable"}}
		}
		result.VerifyConflict = sealedexec.ControllerVerifyConflictResult{Schema: schema, Facts: sealedexec.ConflictFacts{Verification: verification, Report: call.VerifyConflict.Report}}
	case sealedexec.ControllerOperationResolveRecorder:
		ref := call.ResolveRecorder.Ref
		verification := proven
		if f.recorderMismatch {
			verification = sealedexec.Verification{
				State: contextcompile.ResolutionViolatedWithWitness, Failure: sealedexec.FailureRejected,
				Witnesses: []string{"fixture recorder contradiction"},
			}
		}
		result.ResolveRecorder = sealedexec.ControllerResolveRecorderResult{Schema: schema, Facts: sealedexec.RecorderFacts{Verification: verification, Ref: ref}}
	case sealedexec.ControllerOperationRecorderCheckpoint:
		checkpoint, err := f.checkpoint(proven)
		if err != nil {
			return result, err
		}
		result.RecorderCheckpoint = sealedexec.ControllerRecorderCheckpointResult{Schema: schema, Checkpoint: checkpoint}
	case sealedexec.ControllerOperationVerifyOpaqueBoundary:
		rows := make([]sealedexec.OpaqueIdentity, len(call.VerifyOpaqueBoundary.Rows))
		for i, row := range call.VerifyOpaqueBoundary.Rows {
			rows[i] = sealedexec.OpaqueIdentity{ID: row.ID, Kind: row.Kind, AdapterID: row.Adapter.ID, AdapterVersion: row.Adapter.Version}
		}
		result.VerifyOpaqueBoundary = sealedexec.ControllerVerifyOpaqueBoundaryResult{Schema: schema, Facts: sealedexec.OpaqueBoundaryFacts{Verification: proven, Rows: rows}}
	case sealedexec.ControllerOperationNextStamp:
		result.NextStamp = sealedexec.ControllerNextStampResult{Schema: schema, Stamp: fmt.Sprintf("2026-08-27T12:00:%02dZ", len(f.calls))}
	case sealedexec.ControllerOperationRecorderAppend:
		event := call.RecorderAppend.Event
		f.global++
		f.events = append(f.events, event)
		ack := contextevent.EventAck{
			Schema: contextevent.AckSchemaID, Flight: event.Flight, Lane: event.Lane, Epoch: event.Epoch, Session: event.Session,
			ManifestRevision: event.ManifestRevision, Kind: event.Kind, SourceSequence: event.SourceSequence,
			EventDigest: event.EventDigest, GlobalSequence: f.global,
		}
		f.eventAcks = append(f.eventAcks, ack)
		result.RecorderAppend = sealedexec.ControllerRecorderAppendResult{Schema: schema, Ack: ack}
	case sealedexec.ControllerOperationStoreAdapterSession:
		result.StoreAdapterSession = sealedexec.ControllerStoreAdapterSessionResult{Schema: schema}
	case sealedexec.ControllerOperationResolveReceiptInputs:
		result.ResolveReceiptInputs = sealedexec.ControllerResolveReceiptInputsResult{Schema: schema, Inputs: sealedexec.ReceiptInputs{
			Expansions: []contextreceipt.Expansion{}, Obligations: []contextreceipt.Obligation{}, Evidence: []contextreceipt.Evidence{}, ReviewInputs: []contextreceipt.ReviewInput{},
			RunnerPrincipal: sealedLifecyclePrincipal(f.t),
		}}
	case sealedexec.ControllerOperationVerifyExpansion:
		result.VerifyExpansion = sealedexec.ControllerVerifyExpansionResult{Schema: schema, Facts: sealedexec.ExpansionFacts{Verification: proven, Root: f.expansionRoot}}
	case sealedexec.ControllerOperationVerifyProviderSession:
		check := call.VerifyProviderSession.Check
		result.VerifyProviderSession = sealedexec.ControllerVerifyProviderSessionResult{Schema: schema, Facts: sealedexec.ProviderSessionFacts{
			Verification: proven, SessionRef: check.SessionRef, AdapterVersion: check.AdapterVersion,
			ProfileDigest: check.ProfileDigest, WorkspaceID: check.WorkspaceID,
		}}
	case sealedexec.ControllerOperationResolveContext:
		resolution := f.resolution
		resolution.Ref = call.ResolveContext.Query.Ref
		result.ResolveContext = sealedexec.ControllerResolveContextResult{Schema: schema, Resolution: resolution}
	case sealedexec.ControllerOperationVerifyEpoch:
		result.VerifyEpoch = sealedexec.ControllerVerifyEpochResult{Schema: schema, Verification: f.epoch}
	case sealedexec.ControllerOperationInstallExpansion:
		result.InstallExpansion = sealedexec.ControllerInstallExpansionResult{Schema: schema}
	case sealedexec.ControllerOperationAppendReceipt:
		event := call.AppendReceipt.Append.Event
		f.global++
		result.AppendReceipt = sealedexec.ControllerAppendReceiptResult{Schema: schema, Ack: contextevent.ReceiptEventAck{
			Schema: contextevent.ReceiptAckSchemaID, Flight: event.Flight, Lane: event.Lane, Epoch: event.Epoch, Session: event.Session,
			ManifestRevision: event.ManifestRevision, Kind: event.Kind, SourceSequence: event.SourceSequence,
			EventDigest: event.EventDigest, GlobalSequence: f.global, ReceiptDigest: call.AppendReceipt.Append.Receipt.Digest,
		}}
	case sealedexec.ControllerOperationPersistHandback:
		ack, err := f.controlAck(call.PersistHandback.Record)
		if err != nil {
			return result, err
		}
		result.PersistHandback = sealedexec.ControllerPersistHandbackResult{Schema: schema, Ack: ack}
	case sealedexec.ControllerOperationPersistQuarantine:
		if !f.allowQuarantine {
			return result, fmt.Errorf("unexpected lifecycle quarantine %q: %#v", call.PersistQuarantine.Record.Reason, call.PersistQuarantine.Record.Observed)
		}
		if err := sealedexec.ValidateQuarantinePreservation(call.PersistQuarantine.Record, call.PersistQuarantine.PreservedBytes); err != nil {
			return result, fmt.Errorf("invalid lifecycle quarantine record/bytes pair: %w", err)
		}
		f.quarantines = append(f.quarantines, call.PersistQuarantine.Record.Reason)
		f.quarantineRecords = append(f.quarantineRecords, call.PersistQuarantine.Record)
		f.preservedBytes = append(f.preservedBytes, append([]byte{}, call.PersistQuarantine.PreservedBytes...))
		ack, err := f.quarantineAck(call.PersistQuarantine.Record)
		if err != nil {
			return result, err
		}
		result.PersistQuarantine = sealedexec.ControllerPersistQuarantineResult{Schema: schema, Ack: ack}
	default:
		return result, fmt.Errorf("unexpected lifecycle controller operation %q", call.Operation)
	}
	return result, nil
}

func assertLifecyclePreservation(t *testing.T, fake *sealedLifecycleController, state sealedexec.PreservedState) {
	t.Helper()
	if len(fake.quarantineRecords) != 1 || len(fake.preservedBytes) != 1 {
		t.Fatalf("controller stored record/bytes pairs = %d/%d, want 1/1", len(fake.quarantineRecords), len(fake.preservedBytes))
	}
	record, preserved := fake.quarantineRecords[0], fake.preservedBytes[0]
	if record.Preserved.State != state || record.Preserved.Ref == nil || len(preserved) == 0 {
		t.Fatalf("controller preservation = %#v / %q, want nonempty %s bytes", record.Preserved, preserved, state)
	}
	wantDigest := sealedRawDigest(preserved)
	wantID := "controller-preserved/sha256/" + strings.TrimPrefix(wantDigest, "sha256:")
	if record.Preserved.Ref.Digest != wantDigest || record.Preserved.Ref.ID != wantID {
		t.Fatalf("controller preservation ref = %#v, want id %q digest %q", record.Preserved.Ref, wantID, wantDigest)
	}
	switch state {
	case sealedexec.PreservedPartial:
		partial, err := sealedexec.DecodeExecutionPartial(bytes.NewReader(preserved))
		if err != nil {
			t.Fatalf("decode controller-preserved partial: %v", err)
		}
		if partial.Flight != record.Flight || partial.Lane != record.Lane || partial.Epoch != record.Epoch || partial.Session != record.Session || partial.WorkspaceID != record.WorkspaceID {
			t.Fatalf("controller-preserved partial identity = %#v, record = %#v", partial, record)
		}
	case sealedexec.PreservedFinalized:
		result, err := sealedexec.DecodeExecutionResult(bytes.NewReader(preserved))
		if err != nil {
			t.Fatalf("decode controller-preserved result: %v", err)
		}
		if result.Flight != record.Flight || result.Lane != record.Lane || result.Epoch != record.Epoch || result.Session != record.Session ||
			result.ExecutionWorkspaceID != record.WorkspaceID || result.InputCommit != record.Repository.Input.Commit || result.OutputCommit != record.Repository.Output.Commit {
			t.Fatalf("controller-preserved result identity = %#v, record = %#v", result, record)
		}
	}
}

func (f *sealedLifecycleController) checkpoint(proven sealedexec.Verification) (sealedexec.RecorderCheckpoint, error) {
	digest := sealedTestDigest(fmt.Sprintf("checkpoint-%d", len(f.events)))
	if len(f.events) == 0 && f.checkpointDigest != "" {
		digest = f.checkpointDigest
	}
	checkpoint := sealedexec.RecorderCheckpoint{Verification: proven, Digest: digest, Revisions: append([]contextevent.Revision{}, f.initialRevisions...), ActiveRevision: cloneSealedActiveRevision(f.activeRevision)}
	if len(f.events) == 0 {
		if len(checkpoint.Revisions) != 0 {
			root, err := contextevent.EventChainRoot(checkpoint.Revisions)
			if err != nil {
				return checkpoint, err
			}
			terminal := checkpoint.Revisions[len(checkpoint.Revisions)-1]
			checkpoint.EventChainRoot = root
			checkpoint.TerminalSourceSequence = terminal.TerminalSourceSequence
			checkpoint.TerminalGlobalSequence = terminal.TerminalGlobalSequence
		}
		return checkpoint, nil
	}
	terminal := f.events[len(f.events)-1]
	if terminal.Kind != contextevent.KindExecutionResult {
		return checkpoint, fmt.Errorf("checkpoint requested before execution-result terminal: %s", terminal.Kind)
	}
	revision := contextevent.Revision{Schema: contextevent.RevisionSchemaID, ManifestRevision: terminal.ManifestRevision, ManifestDigest: terminal.ManifestDigest, FirstGlobalSequence: 1}
	if len(checkpoint.Revisions) != 0 {
		revision = checkpoint.Revisions[len(checkpoint.Revisions)-1]
	}
	revision.TerminalGlobalSequence = f.global
	revision.TerminalSourceSequence = terminal.SourceSequence
	revision.TerminalKind = terminal.Kind
	revision.EventRoot = terminal.EventDigest
	if len(checkpoint.Revisions) == 0 {
		checkpoint.Revisions = []contextevent.Revision{revision}
	} else {
		checkpoint.Revisions[len(checkpoint.Revisions)-1] = revision
	}
	root, err := contextevent.EventChainRoot(checkpoint.Revisions)
	if err != nil {
		return checkpoint, err
	}
	checkpoint.EventChainRoot = root
	checkpoint.TerminalSourceSequence = revision.TerminalSourceSequence
	checkpoint.TerminalGlobalSequence = revision.TerminalGlobalSequence
	return checkpoint, nil
}

func (f *sealedLifecycleController) controlAck(record sealedexec.HandbackRecord) (sealedexec.ControlAck, error) {
	f.global++
	ack := sealedexec.ControlAck{
		Schema: sealedexec.ExecutionControlAckSchemaID, RecordSchema: record.Schema, RecordDigest: record.Digest,
		Flight: record.Flight, Lane: record.Lane, Epoch: record.Epoch, Session: record.Session,
		WorkspaceID: record.WorkspaceID, Disposition: record.Disposition, ControllerGlobalSequence: f.global,
	}
	encoded, err := sealedexec.EncodeControlAck(ack)
	if err != nil {
		return sealedexec.ControlAck{}, err
	}
	return sealedexec.DecodeControlAck(bytes.NewReader(encoded))
}

func (f *sealedLifecycleController) quarantineAck(record sealedexec.QuarantineRecord) (sealedexec.ControlAck, error) {
	f.global++
	ack := sealedexec.ControlAck{
		Schema: sealedexec.ExecutionControlAckSchemaID, RecordSchema: record.Schema, RecordDigest: record.Digest,
		Flight: record.Flight, Lane: record.Lane, Epoch: record.Epoch, Session: record.Session,
		WorkspaceID: record.WorkspaceID, Disposition: sealedexec.ControlDispositionQuarantined, ControllerGlobalSequence: f.global,
	}
	encoded, err := sealedexec.EncodeControlAck(ack)
	if err != nil {
		return sealedexec.ControlAck{}, err
	}
	return sealedexec.DecodeControlAck(bytes.NewReader(encoded))
}

func sealedLifecyclePrincipal(t *testing.T) gp.PrincipalResolution {
	t.Helper()
	claim := gp.PrincipalClaim{TrustSource: "fixture", Subject: "runner-1"}
	principalID, err := gp.CanonicalPrincipalID(claim.TrustSource, claim.Subject)
	if err != nil {
		t.Fatal(err)
	}
	return gp.PrincipalResolution{
		State: gp.ResolutionAuthenticated, Claim: claim, PrincipalID: principalID,
		Witnesses: []gp.Witness{{Code: "authenticated", SourceID: "fixture", EvidenceDigest: sealedTestDigest("principal")}},
	}
}

func (f *sealedLifecycleController) assertSequence() {
	f.t.Helper()
	want := []sealedexec.ControllerOperation{
		sealedexec.ControllerOperationVerifyAuthority,
		sealedexec.ControllerOperationResolveProfile,
		sealedexec.ControllerOperationVerifyConflict,
		sealedexec.ControllerOperationResolveRecorder,
		sealedexec.ControllerOperationRecorderCheckpoint,
		sealedexec.ControllerOperationVerifyOpaqueBoundary,
		sealedexec.ControllerOperationNextStamp,
		sealedexec.ControllerOperationRecorderAppend,
		sealedexec.ControllerOperationStoreAdapterSession,
		sealedexec.ControllerOperationNextStamp,
		sealedexec.ControllerOperationRecorderAppend,
		sealedexec.ControllerOperationNextStamp,
		sealedexec.ControllerOperationRecorderAppend,
		sealedexec.ControllerOperationNextStamp,
		sealedexec.ControllerOperationRecorderAppend,
		sealedexec.ControllerOperationRecorderCheckpoint,
		sealedexec.ControllerOperationResolveReceiptInputs,
		sealedexec.ControllerOperationNextStamp,
		sealedexec.ControllerOperationAppendReceipt,
		sealedexec.ControllerOperationPersistHandback,
	}
	if !reflect.DeepEqual(f.calls, want) {
		f.t.Fatalf("controller operation sequence = %v, want %v", f.calls, want)
	}
}

func runSealedContextBinary(t *testing.T, bin, dir string, stdin []byte, args ...string) sealedContextObservation {
	return runSealedContextBinaryWithFiles(t, bin, dir, stdin, nil, args...)
}

func runSealedContextBinaryWithFiles(t *testing.T, bin, dir string, stdin []byte, extraFiles []*os.File, args ...string) sealedContextObservation {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.ExtraFiles = extraFiles
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Start()
	for _, file := range extraFiles {
		_ = file.Close()
	}
	if err == nil {
		err = cmd.Wait()
	}
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run verdi %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	return sealedContextObservation{exitCode: code, stdout: stdout.String(), stderr: stderr.String()}
}

func runWithControllerReply(t *testing.T, bin, dir string, stdin []byte, args []string, reply func(sealedexec.ControllerCall) []byte) (sealedContextObservation, sealedexec.ControllerCall) {
	t.Helper()
	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "reply-controller")
	childFile := os.NewFile(uintptr(files[1]), "reply-child")
	controllerConn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatalf("controller FileConn: %v", err)
	}
	result := make(chan struct {
		call sealedexec.ControllerCall
		err  error
	}, 1)
	go func() {
		defer controllerConn.Close()
		frame, err := bufio.NewReader(controllerConn).ReadBytes('\n')
		var call sealedexec.ControllerCall
		if err == nil {
			call, err = sealedexec.DecodeControllerCall(bytes.NewReader(frame))
		}
		if err == nil {
			encoded := reply(call)
			if len(encoded) != 0 {
				_, err = controllerConn.Write(encoded)
			}
		}
		result <- struct {
			call sealedexec.ControllerCall
			err  error
		}{call: call, err: err}
	}()
	observation := runSealedContextBinaryWithFiles(t, bin, dir, stdin, []*os.File{childFile}, args...)
	got := <-result
	if got.err != nil {
		t.Fatalf("strict reply controller: %v; observation=%#v", got.err, observation)
	}
	return observation, got.call
}

func malformedControllerReply(t *testing.T, call sealedexec.ControllerCall, mutation string) []byte {
	t.Helper()
	result := sealedexec.ControllerResult{
		Schema: sealedexec.ControllerResultSchemaID, CallSequence: call.CallSequence, Operation: call.Operation,
		Error: &sealedexec.ControllerError{
			Schema: sealedexec.ControllerErrorSchemaID, Class: sealedexec.ControllerErrorClassOperational,
			Code: sealedexec.ControllerErrorUnavailable, Witnesses: []string{"fixture controller refusal"},
		},
	}
	encoded, err := sealedexec.EncodeControllerResult(result)
	if err != nil {
		t.Fatal(err)
	}
	switch mutation {
	case "schema":
		encoded = bytes.Replace(encoded, []byte(`"schema":"`+sealedexec.ControllerResultSchemaID+`"`), []byte(`"schema":"wrong"`), 1)
	case "operation":
		encoded = bytes.Replace(encoded, []byte(`"operation":"`+string(call.Operation)+`"`), []byte(`"operation":"resolve-profile"`), 1)
	case "call sequence":
		encoded = bytes.Replace(encoded, []byte(`"call_sequence":1`), []byte(`"call_sequence":2`), 1)
	case "unknown field":
		encoded = append(append([]byte(nil), encoded[:len(encoded)-2]...), []byte(",\"unexpected\":true}\n")...)
	default:
		t.Fatalf("unknown controller reply mutation %q", mutation)
	}
	return encoded
}

func runWithRefusingController(t *testing.T, bin, dir string, stdin []byte, args ...string) (sealedContextObservation, sealedexec.ControllerCall) {
	t.Helper()
	files, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	controllerFile := os.NewFile(uintptr(files[0]), "fixture-controller")
	childFile := os.NewFile(uintptr(files[1]), "fixture-child")
	controllerConn, err := net.FileConn(controllerFile)
	_ = controllerFile.Close()
	if err != nil {
		_ = childFile.Close()
		t.Fatalf("controller FileConn: %v", err)
	}
	result := make(chan struct {
		call sealedexec.ControllerCall
		err  error
	}, 1)
	go func() {
		defer controllerConn.Close()
		frame, err := bufio.NewReader(controllerConn).ReadBytes('\n')
		var call sealedexec.ControllerCall
		if err == nil {
			call, err = sealedexec.DecodeControllerCall(bytes.NewReader(frame))
		}
		if err == nil {
			reply := sealedexec.ControllerResult{
				Schema: sealedexec.ControllerResultSchemaID, CallSequence: call.CallSequence, Operation: call.Operation,
				Error: &sealedexec.ControllerError{
					Schema: sealedexec.ControllerErrorSchemaID, Class: sealedexec.ControllerErrorClassOperational,
					Code: sealedexec.ControllerErrorUnavailable, Witnesses: []string{"fixture controller refusal"},
				},
			}
			var encoded []byte
			encoded, err = sealedexec.EncodeControllerResult(reply)
			if err == nil {
				_, err = controllerConn.Write(encoded)
			}
		}
		result <- struct {
			call sealedexec.ControllerCall
			err  error
		}{call: call, err: err}
	}()
	observation := runSealedContextBinaryWithFiles(t, bin, dir, stdin, []*os.File{childFile}, args...)
	got := <-result
	if got.err != nil {
		t.Fatalf("strict fake controller: %v; observation=%#v", got.err, observation)
	}
	return observation, got.call
}

func sealedCanonicalExecutionRequest(t *testing.T, runway string) []byte {
	t.Helper()
	const commit = "1111111111111111111111111111111111111111"
	const tree = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	projection := sealedCanonicalProjection(t)
	manifest := sealedCanonicalManifest(t, projection, commit)
	workspace, err := sealedexec.NewExecutionWorkspaceRequest("flight-1", "lane-1", "epoch-1", "session-1", commit)
	if err != nil {
		t.Fatal(err)
	}
	request := sealedexec.ExecutionRequest{
		Schema: sealedexec.ExecutionRequestSchemaID, Action: sealedexec.ActionStart,
		Flight: "flight-1", Lane: "lane-1", Epoch: "epoch-1", Session: "session-1",
		ManifestRevision: 0, ATCRunway: runway, InputCommit: commit, InputTree: tree,
		Manifest: manifest, ManifestDigest: manifest.Digest,
		InstructionProjection: projection, ProjectionDigest: projection.Digest,
		ExecutionWorkspaceRequest: workspace, Adapter: contextevent.AdapterCodex, AdapterVersion: "1.0.0",
		Profile:          sealedexec.LogicalRef{Schema: sealedexec.ProjectProfileRefSchemaID, ID: "project-profile", Digest: sealedTestDigest("profile")},
		Grants:           execworkspace.GrantSet{Grants: []execworkspace.Grant{}},
		AuthorityVerdict: sealedCanonicalAuthorityReport(t, manifest.Digest, commit),
		RecorderEndpoint: sealedexec.LogicalRef{Schema: sealedexec.RecorderEndpointRefSchemaID, ID: "vatc-recorder", Digest: sealedTestDigest("recorder")},
		Start:            &sealedexec.StartArm{ExpectedSourceSequence: 1},
	}
	encoded, err := sealedexec.EncodeExecutionRequest(request)
	if err != nil {
		t.Fatalf("EncodeExecutionRequest fixture: %v", err)
	}
	return encoded
}

func sealedCanonicalProjection(t *testing.T) sealedexec.InstructionProjection {
	t.Helper()
	content := "sealed instructions\n"
	projection := sealedexec.InstructionProjection{
		Schema: sealedexec.InstructionProjectionSchemaID,
		Files:  []sealedexec.InstructionFile{{Path: "AGENTS.md", ContentDigest: sealedRawDigest([]byte(content)), Content: content}},
	}
	encoded, err := sealedexec.EncodeInstructionProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := sealedexec.DecodeInstructionProjection(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func sealedCanonicalManifest(t *testing.T, projection sealedexec.InstructionProjection, commit string) contextcompile.Manifest {
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
		Adapter:        contextcompile.AdapterRef{ID: "codex", Version: "1.0.0"},
		Revisions:      contextcompile.Revisions{Authority: sealedTestDigest("revision-authority"), Context: 1},
		AcceptedSpec:   contextcompile.AcceptedSpec{Ref: "spec/test", Path: ".verdi/specs/active/test/spec.md", Blob: commit, Commit: commit, ContentDigest: sealedTestDigest("accepted-spec")},
		ParentFeatures: []contextcompile.ParentFeature{}, Decisions: []contextcompile.DecisionRef{}, Obligations: []contextcompile.Obligation{},
		Repository: contextcompile.RepositoryFacts{
			RemoteOrigin: contextcompile.StringFact{Known: true, Value: "origin"}, Branch: contextcompile.StringFact{Known: true, Value: "feature/test"},
			Head: contextcompile.StringFact{Known: true, Value: commit}, DefaultBranch: contextcompile.DefaultBranchFact{Known: true, Name: "main", Ref: "refs/heads/main", Head: commit},
			Relationship: contextcompile.RelationshipEqual, Dirty: contextcompile.BoolFact{Known: true}, Staged: contextcompile.BoolFact{Known: true},
			Worktree: contextcompile.WorktreeFact{Managed: true, Name: "test-worktree"}, Source: contextcompile.RepoSourceHead, Disclosures: []contextcompile.DisclosureCode{},
		},
		Policy: contextcompile.PolicySection{EffectiveDigest: sealedTestDigest("effective-policy"), ConstitutionDigest: sealedTestDigest("constitution"), ProfileID: "profile", ProfileDigest: sealedTestDigest("policy-profile"), Entries: []contextcompile.PolicyEntry{}},
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

func sealedCanonicalAuthorityReport(t *testing.T, manifestDigest, commit string) policyconflict.Report {
	t.Helper()
	report := policyconflict.Report{
		Schema: policyconflict.ReportSchema,
		Input: policyconflict.InputIdentity{
			Target:             policyconflict.TargetIdentity{Kind: policyconflict.TargetAcceptedContext, Accepted: &policyconflict.AcceptedIdentity{ManifestDigest: manifestDigest}},
			ConstitutionDigest: sealedTestDigest("constitution"), EffectivePolicyDigest: sealedTestDigest("effective-policy"),
			PolicyEntries: []policyconflict.PolicyEntryIdentity{}, Profile: policyconflict.ProfileIdentity{ID: "profile", Class: string(gp.ClassSolo), Digest: sealedTestDigest("governance-profile")},
			EvaluatedOn: "2026-08-27",
		},
		Mechanical: []policyconflict.MechanicalEvaluation{}, Semantic: []policyconflict.SemanticEvaluation{}, Disclosures: []policyconflict.Disclosure{},
		Verdict: policyconflict.VerdictPass,
	}
	repository := fmt.Sprintf(`{"remote_origin":{"known":true,"value":"origin"},"branch":{"known":true,"value":"feature/test"},"head":{"known":true,"value":"%s"},"default_branch":{"known":true,"name":"main","ref":"refs/heads/main","head":"%s"},"relationship":"equal","dirty":{"known":true,"value":false},"staged":{"known":true,"value":false},"worktree":{"managed":true,"name":"test-worktree"},"source":"head"}`, commit, commit)
	if err := json.Unmarshal([]byte(repository), &report.Input.Repository); err != nil {
		t.Fatal(err)
	}
	encoded, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := policyconflict.DecodeReport(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func sealedTestDigest(value string) string { return sealedRawDigest([]byte(value)) }

func sealedRawDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// TestResolvedProfileFromMaterialArms proves the consumer-local resolved
// profile copies Amendment 002 §3's Claude model and configuration directory
// exactly from the controller material, declares the Amendment's fixed Claude
// environment rows, and never substitutes the logical profile name for a
// model.
func TestResolvedProfileFromMaterialArms(t *testing.T) {
	// Not parallel: the Claude arm binds ANTHROPIC_API_KEY through t.Setenv,
	// which Go forbids inside a parallel test tree.

	newMaterial := func(t *testing.T, claude bool) (sealedexec.ProfileMaterial, string, string, execworkspace.GrantSet) {
		t.Helper()
		envRoot := t.TempDir()
		workspacePath := filepath.Join(envRoot, "workspace")
		if err := os.MkdirAll(workspacePath, 0o755); err != nil {
			t.Fatal(err)
		}
		executable := filepath.Join(envRoot, "provider")
		if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		grants := execworkspace.GrantSet{Grants: []execworkspace.Grant{
			// Network is granted because deny is unconfigurable on darwin.
			{Kind: execworkspace.GrantNetwork},
			{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{executable}},
		}}
		ref := sealedexec.LogicalRef{Digest: "sha256:" + strings.Repeat("c", 64)}
		material := sealedexec.ProfileMaterial{
			Ref: ref, Name: "sealed-project", AbsoluteExecutable: executable,
			AbsoluteEnvRoot: envRoot, AbsoluteCodexHome: filepath.Join(envRoot, "codex-home"),
			AdapterVersion: "1.2.3", DecoderProfile: "codex-jsonl-v1",
		}
		if claude {
			material.AbsoluteCodexHome = ""
			material.Model = "claude-model-full-id"
			material.ClaudeConfigDir = filepath.Join(envRoot, "claude-config")
			material.DecoderProfile = "claude-stream-json-v1"
		}
		return material, workspacePath, envRoot, grants
	}

	t.Run("claude_arm_copied_and_declared", func(t *testing.T) {
		// Amendment 002 §3 activates the API key from the binary's own process
		// environment, so this arm binds it explicitly instead of depending on
		// an ambient credential (and therefore cannot run in parallel).
		t.Setenv("ANTHROPIC_API_KEY", "sk-ant-arms-fixture-key")
		material, workspacePath, _, grants := newMaterial(t, true)
		resolved, err := resolvedProfileFromMaterial(material, material.Ref, workspacePath, grants)
		if err != nil {
			t.Fatalf("resolvedProfileFromMaterial(claude) = %v, want nil", err)
		}
		if resolved.Model != material.Model {
			t.Fatalf("Model = %q, want exactly %q", resolved.Model, material.Model)
		}
		if resolved.ClaudeConfigDir != material.ClaudeConfigDir {
			t.Fatalf("ClaudeConfigDir = %q, want exactly %q", resolved.ClaudeConfigDir, material.ClaudeConfigDir)
		}
		if resolved.Name == resolved.Model {
			t.Fatal("the logical profile name must never be the provider model")
		}
		if resolved.CodexHome != "" {
			t.Fatalf("CodexHome = %q, want empty on the Claude arm", resolved.CodexHome)
		}
		env := resolved.Profile.Env()
		for name, want := range map[string]string{
			"CLAUDE_CONFIG_DIR":                        material.ClaudeConfigDir,
			"DISABLE_AUTOUPDATER":                      "1",
			"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
			"CLAUDE_CODE_AUTO_CONNECT_IDE":             "false",
		} {
			if got := testEnvValue(env, name); got != want {
				t.Fatalf("activated %s = %q, want %q", name, got, want)
			}
		}
		if testEnvValue(env, "CODEX_HOME") != "" {
			t.Fatal("the Claude arm must not declare CODEX_HOME")
		}
	})

	t.Run("codex_arm_unchanged", func(t *testing.T) {
		t.Parallel()
		material, workspacePath, _, grants := newMaterial(t, false)
		resolved, err := resolvedProfileFromMaterial(material, material.Ref, workspacePath, grants)
		if err != nil {
			t.Fatalf("resolvedProfileFromMaterial(codex) = %v, want nil", err)
		}
		if resolved.Model != "" || resolved.ClaudeConfigDir != "" {
			t.Fatalf("codex arm carries Claude fields: model=%q dir=%q", resolved.Model, resolved.ClaudeConfigDir)
		}
		env := resolved.Profile.Env()
		if got := testEnvValue(env, "CODEX_HOME"); got != material.AbsoluteCodexHome {
			t.Fatalf("activated CODEX_HOME = %q, want %q", got, material.AbsoluteCodexHome)
		}
		if testEnvValue(env, "CLAUDE_CONFIG_DIR") != "" {
			t.Fatal("the Codex arm must not declare CLAUDE_CONFIG_DIR")
		}
	})

	negatives := []struct {
		name   string
		claude bool
		mutate func(*sealedexec.ProfileMaterial)
	}{
		{"both_arms_selected", true, func(m *sealedexec.ProfileMaterial) { m.AbsoluteCodexHome = "/abs/codex-home" }},
		{"no_arm_selected", false, func(m *sealedexec.ProfileMaterial) { m.AbsoluteCodexHome = "" }},
		{"claude_missing_model", true, func(m *sealedexec.ProfileMaterial) { m.Model = "" }},
		{"claude_relative_config_dir", true, func(m *sealedexec.ProfileMaterial) { m.ClaudeConfigDir = "claude-config" }},
		{"claude_config_dir_outside_env_root", true, func(m *sealedexec.ProfileMaterial) { m.ClaudeConfigDir = "/tmp/elsewhere/claude-config" }},
		{"claude_config_dir_is_env_root", true, func(m *sealedexec.ProfileMaterial) { m.ClaudeConfigDir = m.AbsoluteEnvRoot }},
		{"codex_relative_home", false, func(m *sealedexec.ProfileMaterial) { m.AbsoluteCodexHome = "codex-home" }},
		{"relative_executable", false, func(m *sealedexec.ProfileMaterial) { m.AbsoluteExecutable = "provider" }},
	}
	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			material, workspacePath, _, grants := newMaterial(t, tc.claude)
			tc.mutate(&material)
			_, err := resolvedProfileFromMaterial(material, material.Ref, workspacePath, grants)
			if err == nil {
				t.Fatalf("resolvedProfileFromMaterial(%s) = nil, want a refusal", tc.name)
			}
		})
	}

	t.Run("material_ref_mismatch_refused", func(t *testing.T) {
		t.Parallel()
		material, workspacePath, _, grants := newMaterial(t, true)
		other := sealedexec.LogicalRef{Digest: "sha256:" + strings.Repeat("d", 64)}
		if _, err := resolvedProfileFromMaterial(material, other, workspacePath, grants); err == nil {
			t.Fatal("resolvedProfileFromMaterial with a contradicting ref = nil, want a refusal")
		}
	})
}

func testEnvValue(env []string, name string) string {
	prefix := name + "="
	for _, row := range env {
		if strings.HasPrefix(row, prefix) {
			return strings.TrimPrefix(row, prefix)
		}
	}
	return ""
}
