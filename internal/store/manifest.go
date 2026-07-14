package store

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

const manifestSchema = "verdi.layout/v1"

// JiraConfig is the `providers.jira` block of verdi.yaml (04 §Story
// provider owns semantics; this package only decodes the shape 01 §Store
// manifest shows). Secrets never live here — only ids.
//
// Mode is spec/close-verb dc-2's config-selectable fake tracker: "" (the
// zero value) selects the real Jira adapter unchanged; "fake" selects the
// in-process internal/provider/fake adapter instead (buildProviderRegistry,
// cmd/verdi/rollup.go), keeping ac-2's publish + read-back proof hermetic
// and stopping routine verbs from egressing to a real host (D6-2).
// BaseURL/RollupField stay decodable and present under fake mode — true-
// closure dc-2 requires switching to real Jira to stay a pure config change
// (flip mode back to "" or remove it), never a code change, so this field
// must not require removing the other two.
type JiraConfig struct {
	Mode        string `yaml:"mode,omitempty"`
	BaseURL     string `yaml:"base_url"`
	RollupField string `yaml:"rollup_field"`
}

// validJiraModes is Mode's closed enum (CLAUDE.md: "unknown enum values
// fail closed").
var validJiraModes = map[string]bool{"": true, "fake": true}

// Validate checks Mode is a known value.
func (j JiraConfig) Validate() error {
	if !validJiraModes[j.Mode] {
		return fmt.Errorf("store: verdi.yaml providers.jira.mode %q is not \"\" (real) or \"fake\"", j.Mode)
	}
	return nil
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
//
// JudgeCmd is an argv ARRAY, not a shell string — a deliberate, FLAGGED
// deviation from 01 §Store manifest's example YAML (`judge_cmd: claude -p`),
// made per PLAN.md Phase 8's binding spike S5 finding: "judge_cmd is an
// argv ARRAY (no shell string) ... splitting a string on whitespace is
// FORBIDDEN as silent invention" (quoting/escaping rules would have to be
// invented, and S5 proved the real path takes an explicit argv, e.g.
// ["claude", "-p", "--output-format", "json", "--model", "<pin>"]). This is
// a manifest schema change candidate for ratification into 01 §Store
// manifest; flagged here rather than silently reconciled, per CLAUDE.md
// ("never resolve a spec ambiguity silently ... record it").
// JudgeTimeoutSeconds is an INVENTED key: 01 §Store manifest's example
// YAML shows only judge_cmd/judge_required under align: — this field has
// no spec citation, mirroring judge_cmd's own argv-array disclosed
// deviation on this same struct. Added per D6-21
// (docs/design/plans/round6-divergences.md): the judge (`claude -p`,
// exec'd as a subprocess) was timing out at internal/align's hardcoded
// 120s ceiling on every real build diff, so automated alignment coverage
// never landed; this key lets a repo raise (or lower) that ceiling via
// verdi.yaml without a code change. Zero/absent means "use
// align.DefaultJudgeTimeout unchanged" (default-unchanged guarantee); a
// negative value fails decode/validation loudly rather than being
// silently clamped or ignored (CLAUDE.md: "unknown enum values fail
// closed" — the same fail-closed posture applied to an invalid duration).
// A manifest schema change candidate for ratification into 01 §Store
// manifest, flagged here rather than silently reconciled.
type AlignConfig struct {
	JudgeCmd            []string `yaml:"judge_cmd,omitempty"`
	JudgeRequired       bool     `yaml:"judge_required"`
	JudgeTimeoutSeconds int      `yaml:"judge_timeout_seconds,omitempty"`
}

// Validate checks JudgeTimeoutSeconds is non-negative (D6-21: a negative
// timeout can never elapse meaningfully, so it fails closed rather than
// being silently coerced to the default or to zero).
func (a AlignConfig) Validate() error {
	if a.JudgeTimeoutSeconds < 0 {
		return fmt.Errorf("store: verdi.yaml align.judge_timeout_seconds %d must not be negative", a.JudgeTimeoutSeconds)
	}
	return nil
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

// AuditConfig is verdi.yaml's `audit:` block (R4-I-10, 01 §Store manifest):
// the exemption/deviation counterweight thresholds (spec-realignment
// concept §2, §3b). Both fields are tunable, both documented as
// spec-realignment concept OQ-iii watch items, and 01 documents both as
// defaulting to 3 — "the smallest reversible starting point, not a value
// derived from data." This package only decodes and shape-checks the raw
// values; applying the documented default of 3 when the block (or a field
// within it) is absent, and disambiguating an absent field from an
// explicit 0, is left to the first consuming phase (V1-P3's spec-stale
// computation for DeviationsStaleThreshold, V1-P5's exemption audit for
// ExemptsConflictThreshold per R4-I-10's phase assignment) — this phase's
// job is only to grow the manifest schema, not to invent the
// zero-vs-absent-vs-configured default-application rule 01 does not spell
// out mechanically. Flagged in the phase report as a candidate follow-up
// for 01 §Store manifest to state explicitly.
type AuditConfig struct {
	ExemptsConflictThreshold int `yaml:"exempts_conflict_threshold"`
	DeviationsStaleThreshold int `yaml:"deviations_stale_threshold"`
}

// Validate checks both thresholds are non-negative (a negative count can
// never be reached, which would make the counterweight permanently inert —
// silently accepting one would hide a manifest typo).
func (a AuditConfig) Validate() error {
	if a.ExemptsConflictThreshold < 0 {
		return fmt.Errorf("store: verdi.yaml audit.exempts_conflict_threshold %d must not be negative", a.ExemptsConflictThreshold)
	}
	if a.DeviationsStaleThreshold < 0 {
		return fmt.Errorf("store: verdi.yaml audit.deviations_stale_threshold %d must not be negative", a.DeviationsStaleThreshold)
	}
	return nil
}

// Manifest is the store manifest, `verdi.yaml`, schema verdi.layout/v1
// (01 §Store manifest). Decode is strict: unknown top-level keys fail.
type Manifest struct {
	Schema    string           `yaml:"schema"`
	Forge     string           `yaml:"forge,omitempty"`
	Providers *ProvidersConfig `yaml:"providers,omitempty"`
	Lint      *LintConfig      `yaml:"lint,omitempty"`
	Align     *AlignConfig     `yaml:"align,omitempty"`
	Audit     *AuditConfig     `yaml:"audit,omitempty"`
	// SpikePaths is the VL-016 path-glob fence a spike build branch's diff
	// must stay inside (01 §Store manifest, R4-I-10). Fails closed: an
	// absent or empty list admits no spike diffs at all — a repo must
	// explicitly declare its spike workspace and doc paths before any spike
	// diff is accepted (01: "mirroring lint.gated_generated's empty-by-
	// default posture").
	SpikePaths []string         `yaml:"spike_paths,omitempty"`
	Derived    *DerivedConfig   `yaml:"derived,omitempty"`
	Services   *ServicesConfig  `yaml:"services,omitempty"`
	Toolchain  *ToolchainConfig `yaml:"toolchain,omitempty"`
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
	if m.Audit != nil {
		if err := m.Audit.Validate(); err != nil {
			return err
		}
	}
	if m.Align != nil {
		if err := m.Align.Validate(); err != nil {
			return err
		}
	}
	if m.Providers != nil && m.Providers.Jira != nil {
		if err := m.Providers.Jira.Validate(); err != nil {
			return err
		}
	}
	return nil
}
