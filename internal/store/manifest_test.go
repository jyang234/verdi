package store

import (
	"os"
	"path/filepath"
	"testing"
)

const validManifestYAML = `schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
lint:
  gated_generated: []
align:
  judge_cmd: [claude, -p]
  judge_required: false
derived:
  retention_days: 14
services:
  discovery: flowmap
toolchain:
  module: github.com/jyang234/golang-code-graph
  commit: cd38b1a56bb782177a207d741a39807821cf2c1c
`

func TestDecodeManifest_Happy(t *testing.T) {
	m, err := DecodeManifest([]byte(validManifestYAML))
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if m.Schema != manifestSchema {
		t.Fatalf("Schema = %q, want %q", m.Schema, manifestSchema)
	}
	if m.Forge != "gitlab" {
		t.Fatalf("Forge = %q, want gitlab", m.Forge)
	}
	if m.Providers == nil || m.Providers.Jira == nil || m.Providers.Jira.RollupField != "customfield_00000" {
		t.Fatalf("Providers.Jira = %+v, unexpected", m.Providers)
	}
	if m.Services == nil || m.Services.Discovery != "flowmap" {
		t.Fatalf("Services = %+v, want discovery=flowmap", m.Services)
	}
	if m.Toolchain == nil || m.Toolchain.Commit != "cd38b1a56bb782177a207d741a39807821cf2c1c" {
		t.Fatalf("Toolchain = %+v, unexpected", m.Toolchain)
	}
	if m.Align == nil || len(m.Align.JudgeCmd) != 2 || m.Align.JudgeCmd[0] != "claude" || m.Align.JudgeCmd[1] != "-p" {
		t.Fatalf("Align.JudgeCmd = %+v, want [claude -p]", m.Align)
	}
}

// TestDecodeManifest_JudgeCmdStringRejected proves align.judge_cmd is an
// argv array, not a shell string (PLAN.md Phase 8 spike S5, binding):
// splitting a scalar string on whitespace to recover an argv would be
// silent invention (quoting/escaping semantics verdi does not own), so a
// bare scalar must fail strict decode rather than be silently coerced.
func TestDecodeManifest_JudgeCmdStringRejected(t *testing.T) {
	data := "schema: verdi.layout/v1\nalign:\n  judge_cmd: claude -p\n"
	if _, err := DecodeManifest([]byte(data)); err == nil {
		t.Fatal("DecodeManifest(judge_cmd as bare string): want error, got nil")
	}
}

// TestDecodeManifest_ThisRepoOwnManifest proves DecodeManifest reads this
// module's own self-hosted .verdi/verdi.yaml (PLAN.md A7/00 §index: "this
// repo's own self-hosted .verdi/"), not just a synthetic fixture.
func TestDecodeManifest_ThisRepoOwnManifest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".verdi", "verdi.yaml"))
	if err != nil {
		t.Fatalf("reading this repo's .verdi/verdi.yaml: %v", err)
	}
	if _, err := DecodeManifest(data); err != nil {
		t.Fatalf("DecodeManifest(this repo's verdi.yaml): %v", err)
	}
}

func TestDecodeManifest_MinimalRequiredOnly(t *testing.T) {
	m, err := DecodeManifest([]byte("schema: verdi.layout/v1\n"))
	if err != nil {
		t.Fatalf("DecodeManifest(minimal): %v", err)
	}
	if m.Forge != "" {
		t.Fatalf("Forge = %q, want empty (auto-detect)", m.Forge)
	}
}

func TestDecodeManifest_Negative(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"unknown top-level key", "schema: verdi.layout/v1\nbogus: true\n"},
		{"wrong schema", "schema: verdi.layout/v0\n"},
		{"missing schema", "forge: gitlab\n"},
		{"bad forge enum", "schema: verdi.layout/v1\nforge: bitbucket\n"},
		{"dialect anchor", "schema: verdi.layout/v1\nforge: &f gitlab\n"},
		{"not yaml", "not: [valid"},
		{"unknown nested key", "schema: verdi.layout/v1\nservices:\n  discovery: flowmap\n  bogus: true\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeManifest([]byte(tc.data)); err == nil {
				t.Fatalf("DecodeManifest(%s): want error, got nil", tc.name)
			}
		})
	}
}
