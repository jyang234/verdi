package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/matrixprojection"
	"github.com/jyang234/verdi/internal/mcpserve"
)

const machineJourneyStorySpec = `---
id: spec/journey-story
kind: spec
class: story
title: "Journey story"
owners: [platform-team]
story: jira:JOURNEY-1
problem: { text: "journey target is implicit", anchor: problem }
outcome: { text: "journey target is explicit", anchor: outcome }
links:
  - { type: implements, ref: "spec/payments#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "story journey is projected", evidence: [static] }
---
# Journey story
`

func buildMachineProjectionBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "verdi")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building verdi binary: %v\n%s", err, out)
	}
	return bin
}

func runMachineProjectionBinary(t *testing.T, bin, dir string, args ...string) (int, []byte, []byte) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.Bytes(), stderr.Bytes()
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("running verdi %v: %v", args, err)
	}
	return exitErr.ExitCode(), stdout.Bytes(), stderr.Bytes()
}

func decodeToolMatrix(t *testing.T, result map[string]any) matrixprojection.Record {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshaling MCP result envelope: %v", err)
	}
	var envelope struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decoding MCP result envelope: %v", err)
	}
	if envelope.IsError || len(envelope.Content) != 1 || envelope.Content[0].Type != "text" {
		t.Fatalf("MCP get_matrix result = %s, want one successful text content item", raw)
	}
	record, err := matrixprojection.Decode([]byte(envelope.Content[0].Text))
	if err != nil {
		t.Fatalf("decoding MCP matrix record: %v\n%s", err, envelope.Content[0].Text)
	}
	return record
}

func TestMatrixProjectionContract_Behavioral(t *testing.T) {
	bin := buildMachineProjectionBinary(t)
	repo := buildCorpusRepo(t)
	copyV2FeatureFixture(t, repo.Dir,
		"specs/active/borrower-update-api",
		"specs/active/borrower-update-mobile",
		"specs/active/borrower-update-mobile-spike",
	)
	backend := &mcpserve.Backend{Root: repo.Dir}

	tests := []struct {
		name    string
		ref     string
		preview bool
	}{
		{name: "story", ref: "spec/borrower-update-api"},
		{name: "feature", ref: "spec/stale-decline"},
		{name: "preview feature", ref: "spec/stale-decline", preview: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"matrix"}
			if tc.preview {
				args = append(args, "--preview")
			}
			args = append(args, "--json", tc.ref)
			exit, first, stderr := runMachineProjectionBinary(t, bin, repo.Dir, args...)
			if exit != 0 {
				t.Fatalf("verdi %v exit = %d, want 0; stderr=%s", args, exit, stderr)
			}
			if len(first) == 0 || first[len(first)-1] != '\n' || bytes.HasSuffix(first, []byte("\n\n")) {
				t.Fatalf("verdi %v output must have exactly one trailing newline: %q", args, first)
			}
			cliRecord, err := matrixprojection.Decode(first)
			if err != nil {
				t.Fatalf("decoding CLI matrix: %v\n%s", err, first)
			}
			exit, second, stderr := runMachineProjectionBinary(t, bin, repo.Dir, args...)
			if exit != 0 || !bytes.Equal(first, second) {
				t.Fatalf("verdi %v is nondeterministic: exit=%d stderr=%s\nfirst=%s\nsecond=%s", args, exit, stderr, first, second)
			}
			mcpRecord := decodeToolMatrix(t, backend.GetMatrix(context.Background(), json.RawMessage(`{"story":"`+tc.ref+`","preview":`+map[bool]string{false: "false", true: "true"}[tc.preview]+`}`)))
			if !reflect.DeepEqual(cliRecord, mcpRecord) {
				t.Fatalf("CLI and MCP matrix records differ:\nCLI: %#v\nMCP: %#v", cliRecord, mcpRecord)
			}
		})
	}
}

func TestJourneyJSONContract_Behavioral(t *testing.T) {
	bin := buildMachineProjectionBinary(t)
	repo := buildJourneyRepo(t, map[string]string{
		".verdi/specs/active/payments/spec.md":      journeyFeatureSpecMD,
		".verdi/specs/active/journey-story/spec.md": machineJourneyStorySpec,
	})

	before := gitStatusPorcelain(t, repo.Dir)
	for _, ref := range []string{"spec/payments", "spec/journey-story"} {
		t.Run(ref, func(t *testing.T) {
			legacyExit, legacy, legacyErr := runMachineProjectionBinary(t, bin, repo.Dir, "journey", ref)
			explicitExit, explicit, explicitErr := runMachineProjectionBinary(t, bin, repo.Dir, "journey", "--json", ref)
			if legacyExit != 0 || explicitExit != 0 {
				t.Fatalf("journey exits legacy=%d explicit=%d, want 0/0; legacy stderr=%s explicit stderr=%s", legacyExit, explicitExit, legacyErr, explicitErr)
			}
			if !bytes.Equal(legacy, explicit) {
				t.Fatalf("journey output differs for %s:\nlegacy=%s\nexplicit=%s", ref, legacy, explicit)
			}
		})
	}
	if after := gitStatusPorcelain(t, repo.Dir); after != before {
		t.Fatalf("journey JSON mutated the store: before=%q after=%q", before, after)
	}
}

func TestMachineProjectionFailureContract_Behavioral(t *testing.T) {
	bin := buildMachineProjectionBinary(t)
	repo := buildCorpusRepo(t)
	derivedDir := filepath.Join(repo.Dir, ".verdi", "data", "derived", "spec--stale-decline", repo.Head)
	if err := os.MkdirAll(derivedDir, 0o755); err != nil {
		t.Fatalf("creating violated matrix fixture: %v", err)
	}
	violatedEvidence := `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"static","verdict":"fail","witness":"ci://verify/static/ac-1","provenance":{"source":"ci","pipeline":"u1","commit":"` + repo.Head + `"},"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]`
	if err := os.WriteFile(filepath.Join(derivedDir, "verdicts.json"), []byte(violatedEvidence), 0o644); err != nil {
		t.Fatalf("writing violated matrix fixture: %v", err)
	}
	exit, violatedJSON, stderr := runMachineProjectionBinary(t, bin, repo.Dir, "matrix", "--json", "spec/stale-decline")
	if exit != 0 {
		t.Fatalf("violated matrix report exit = %d, want 0; stderr=%s", exit, stderr)
	}
	violatedRecord, err := matrixprojection.Decode(violatedJSON)
	if err != nil || !violatedRecord.Violated {
		t.Fatalf("violated matrix report = %s, decode error=%v, want violated=true", violatedJSON, err)
	}
	exit, blockedText, stderr := runMachineProjectionBinary(t, bin, repo.Dir, "matrix", "spec/stale-decline")
	if exit != 0 || !bytes.Contains(blockedText, []byte("stub_reconciliation.blocked: true")) {
		t.Fatalf("blocked matrix report exit=%d stderr=%s output=%s, want exit 0 and blocked=true", exit, stderr, blockedText)
	}

	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "matrix unknown flag", args: []string{"matrix", "--wat", "--json", "spec/stale-decline"}, want: 2},
		{name: "matrix duplicate json", args: []string{"matrix", "--json", "--json", "spec/stale-decline"}, want: 2},
		{name: "matrix duplicate preview", args: []string{"matrix", "--preview", "--preview", "spec/stale-decline"}, want: 2},
		{name: "matrix malformed flag order", args: []string{"matrix", "--json", "--preview", "spec/stale-decline"}, want: 2},
		{name: "matrix missing ref", args: []string{"matrix", "--json", "spec/missing"}, want: 2},
		{name: "journey blocker report succeeds", args: []string{"journey", "--json", "spec/stale-decline"}, want: 0},
		{name: "journey unknown flag", args: []string{"journey", "--wat", "spec/stale-decline"}, want: 2},
		{name: "journey duplicate json", args: []string{"journey", "--json", "--json", "spec/stale-decline"}, want: 2},
		{name: "journey malformed flag order", args: []string{"journey", "spec/stale-decline", "--json"}, want: 2},
		{name: "journey missing ref", args: []string{"journey", "--json", "spec/missing"}, want: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exit, _, stderr := runMachineProjectionBinary(t, bin, repo.Dir, tc.args...)
			if exit != tc.want {
				t.Fatalf("verdi %v exit = %d, want %d; stderr=%s", tc.args, exit, tc.want, stderr)
			}
		})
	}

	rootless := t.TempDir()
	for _, args := range [][]string{{"matrix", "--json", "spec/stale-decline"}, {"journey", "--json", "spec/stale-decline"}} {
		exit, _, stderr := runMachineProjectionBinary(t, bin, rootless, args...)
		if exit != 2 {
			t.Fatalf("rootless verdi %v exit = %d, want 2; stderr=%s", args, exit, stderr)
		}
	}
}

func gitStatusPorcelain(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain=v1", "--untracked-files=all")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status in %s: %v\n%s", dir, err, out)
	}
	return string(out)
}
