package store

import (
	"fmt"

	"github.com/OWNER/verdi/internal/artifact"
)

const manifestSchema = "verdi.layout/v1"

// JiraConfig is the `providers.jira` block of verdi.yaml (04 §Story
// provider owns semantics; this package only decodes the shape 01 §Store
// manifest shows). Secrets never live here — only ids.
type JiraConfig struct {
	BaseURL     string `yaml:"base_url"`
	RollupField string `yaml:"rollup_field"`
}

// ProvidersConfig is verdi.yaml's `providers:` block.
type ProvidersConfig struct {
	Jira *JiraConfig `yaml:"jira,omitempty"`
}

// LintConfig is verdi.yaml's `lint:` block.
type LintConfig struct {
	GatedGenerated []string `yaml:"gated_generated"`
}

// AlignConfig is verdi.yaml's `align:` block (I-9).
type AlignConfig struct {
	JudgeCmd      string `yaml:"judge_cmd"`
	JudgeRequired bool   `yaml:"judge_required"`
}

// DerivedConfig is verdi.yaml's `derived:` block.
type DerivedConfig struct {
	RetentionDays int `yaml:"retention_days"`
}

// ServicesConfig is verdi.yaml's `services:` block.
type ServicesConfig struct {
	Discovery string `yaml:"discovery"`
}

// ToolchainConfig is verdi.yaml's `toolchain:` block (I-4): the pinned
// verdi-go module and commit.
type ToolchainConfig struct {
	Module string `yaml:"module"`
	Commit string `yaml:"commit"`
}

// Manifest is the store manifest, `verdi.yaml`, schema verdi.layout/v1
// (01 §Store manifest). Decode is strict: unknown top-level keys fail.
type Manifest struct {
	Schema    string           `yaml:"schema"`
	Forge     string           `yaml:"forge,omitempty"`
	Providers *ProvidersConfig `yaml:"providers,omitempty"`
	Lint      *LintConfig      `yaml:"lint,omitempty"`
	Align     *AlignConfig     `yaml:"align,omitempty"`
	Derived   *DerivedConfig   `yaml:"derived,omitempty"`
	Services  *ServicesConfig  `yaml:"services,omitempty"`
	Toolchain *ToolchainConfig `yaml:"toolchain,omitempty"`
}

var validForges = map[string]bool{"": true, "gitlab": true, "github": true}

// DecodeManifest strict-decodes and validates verdi.yaml. Decode goes
// through internal/artifact's exported strict-decode seam (DecodeStrict):
// this package never imports yaml.v3 directly (CLAUDE.md's single import
// seam, enforced module-wide by artifact.TestYAMLImportSeam).
func DecodeManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := artifact.DecodeStrict(data, &m); err != nil {
		return nil, fmt.Errorf("store: decoding verdi.yaml: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// ConfiguredStorySchemes returns the set of story-ref schemes verdi.yaml's
// providers: block configures (VL-005's "a configured scheme"; `verdi
// design start`'s I-10 scheme-configured check, 04 §Reference scheme).
// Only "jira" is modeled today (JiraConfig); a nil Manifest or an absent
// providers: block configures no scheme at all. A pointer receiver so a
// nil *Manifest (a legitimately absent manifest, e.g. lint's
// Snapshot.Manifest before a store has one) is safe to call directly.
func (m *Manifest) ConfiguredStorySchemes() map[string]bool {
	schemes := map[string]bool{}
	if m == nil || m.Providers == nil {
		return schemes
	}
	if m.Providers.Jira != nil {
		schemes["jira"] = true
	}
	return schemes
}

// Validate checks the schema literal and the forge enum ("" auto-detects
// per 01 §Store manifest).
func (m Manifest) Validate() error {
	if m.Schema != manifestSchema {
		return fmt.Errorf("store: verdi.yaml schema %q, want %q", m.Schema, manifestSchema)
	}
	if !validForges[m.Forge] {
		return fmt.Errorf("store: verdi.yaml forge %q is not gitlab, github, or empty (auto-detect)", m.Forge)
	}
	return nil
}
