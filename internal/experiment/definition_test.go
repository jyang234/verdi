package experiment

import (
	"strings"
	"testing"
)

// base40 is the shared well-formed 40-hex commit value every definition
// fixture in this file reuses.
var base40 = repeat("a", 40)

// testExperimentDir is the canonical repo-relative directory the fixture
// experiment lives in — the coordinate every protected-input check and
// every DeriveState call in this package's tests uses.
const testExperimentDir = "experiments/cache-placement-v1"

func digestOf(char string) string {
	return "sha256:" + repeat(char, 64)
}

// baselinePatchContent and factsCachePatchContent are the exact patch
// bytes validDefinitionYAML registers digests for. They are real (if
// minimal) git-unified-diff bodies — one "diff --git" header naming a
// single changed path — so commit 3's ValidateCandidatePatch tests can
// write this same content to disk and have it verify against the
// registered digest below, rather than an arbitrary placeholder.
const (
	baselinePatchContent   = "diff --git a/x b/x\n"
	factsCachePatchContent = "diff --git a/y b/y\n"
	// baselinePatchDigest and factsCachePatchDigest are
	// sha256Digest([]byte(baselinePatchContent)) and
	// sha256Digest([]byte(factsCachePatchContent)) respectively (verified
	// independently against `shasum -a 256`; also cross-checked by
	// TestCandidatePatchFixtureDigestsMatchContent).
	baselinePatchDigest   = "sha256:1a059963bbf3198857755a48c741d351e21515186ce951464b89a0de0797c081"
	factsCachePatchDigest = "sha256:0e02748d5d09549294f0c12f8356792fe75c43c7c5640a636039f9b1a9214b8d"
)

// validDefinitionYAML returns a complete, decode-and-validate-clean
// experiment.yaml document extending the spec's own AC-1 example with
// every field this package's contract requires.
func validDefinitionYAML() string {
	return "schema: verdi.experiment/v1\n" +
		"id: cache-placement-v1\n" +
		"spike: spec/cache-placement-spike\n" +
		"question: spec/request-path#oq-cache-placement\n" +
		"base_commit: " + base40 + "\n" +
		"\n" +
		"candidates:\n" +
		"  - id: baseline\n" +
		"    patch: candidates/baseline.patch\n" +
		"    digest: " + baselinePatchDigest + "\n" +
		"    base: " + base40 + "\n" +
		"  - id: facts-cache\n" +
		"    patch: candidates/facts-cache.patch\n" +
		"    digest: " + factsCachePatchDigest + "\n" +
		"    base: " + base40 + "\n" +
		"\n" +
		"evaluator:\n" +
		"  argv: [\"./tools/cache-evaluator\", \"run\"]\n" +
		"  digest: " + digestOf("3") + "\n" +
		"  capabilities_digest: " + digestOf("4") + "\n" +
		"\n" +
		"workload:\n" +
		"  id: representative-request-mix\n" +
		"  digest: " + digestOf("5") + "\n" +
		"\n" +
		"fixtures:\n" +
		"  - id: request-log\n" +
		"    digest: " + digestOf("6") + "\n" +
		"\n" +
		"contract:\n" +
		"  id: behavioral-equivalence-contract\n" +
		"  digest: " + digestOf("7") + "\n" +
		"\n" +
		"decision:\n" +
		"  primary_metric:\n" +
		"    id: request-latency\n" +
		"    type: duration\n" +
		"    unit: ms\n" +
		"    aggregation: p95\n" +
		"    direction: lower\n" +
		"  baseline: baseline\n" +
		"  baseline_improvement:\n" +
		"    relative: 0.25\n" +
		"  candidate_separation:\n" +
		"    relative: 0.05\n" +
		"  guards:\n" +
		"    - id: behavioral-equivalence\n" +
		"    - id: tenant-isolation\n" +
		"    - id: peak-rss\n" +
		"      maximum_relative_to_baseline: 0.15\n" +
		"  variability:\n" +
		"    max_relative_spread: 0.1\n" +
		"\n" +
		"execution:\n" +
		"  warmups: 3\n" +
		"  rounds: 10\n" +
		"  order: deterministic-rotation\n" +
		"  timeout_per_round: 30s\n" +
		"  environment_policy: local-isolated-v1\n" +
		"\n" +
		"algorithm: verdi.experiment-recommendation/v1\n" +
		"retention_policy: standard-retention-v1\n" +
		"\n" +
		"policy:\n" +
		"  ref: experiment-policy/standard\n" +
		"\n" +
		"protected_paths:\n" +
		"  - internal/cache\n"
}

// validDefinitionV2YAML upgrades the shared predecessor fixture without
// changing any v1 field. reproductionBlock is either empty or a complete
// top-level reproduction mapping.
func validDefinitionV2YAML(reproductionBlock string) string {
	doc := strings.Replace(validDefinitionYAML(),
		"schema: verdi.experiment/v1\n",
		"schema: verdi.experiment/v2\nclass: request-path-performance\n", 1)
	if reproductionBlock == "" {
		return doc
	}
	return doc + reproductionBlock
}

func TestDefinitionV2DecodeAndVersionCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		doc         string
		wantSchema  string
		wantClass   string
		wantMinimum int
	}{
		{
			name:       "v1 remains decode compatible",
			doc:        validDefinitionYAML(),
			wantSchema: DefinitionSchemaV1,
		},
		{
			name:       "v2 accepts an absent reproduction rule",
			doc:        validDefinitionV2YAML(""),
			wantSchema: DefinitionSchemaV2,
			wantClass:  "request-path-performance",
		},
		{
			name:        "v2 accepts a registered reproduction rule",
			doc:         validDefinitionV2YAML("reproduction:\n  minimum_valid_runs: 2\n"),
			wantSchema:  DefinitionSchemaV2,
			wantClass:   "request-path-performance",
			wantMinimum: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := DecodeDefinition([]byte(tt.doc))
			if err != nil {
				t.Fatalf("DecodeDefinition() unexpected error: %v", err)
			}
			if def.Schema != tt.wantSchema || def.Class != tt.wantClass {
				t.Fatalf("decoded schema/class = %q/%q, want %q/%q", def.Schema, def.Class, tt.wantSchema, tt.wantClass)
			}
			if tt.wantMinimum == 0 {
				if def.Reproduction != nil {
					t.Fatalf("decoded reproduction = %+v, want absent", def.Reproduction)
				}
				return
			}
			if def.Reproduction == nil || def.Reproduction.MinimumValidRuns != tt.wantMinimum {
				t.Fatalf("decoded reproduction = %+v, want minimum_valid_runs=%d", def.Reproduction, tt.wantMinimum)
			}
		})
	}
}

func TestDefinitionV1VersionSpecificFieldPresence(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantErr string
	}{
		{
			name: "ordinary historical v1",
			doc:  validDefinitionYAML(),
		},
		{
			name:    "explicit empty class",
			doc:     strings.Replace(validDefinitionYAML(), "schema: verdi.experiment/v1\n", "schema: verdi.experiment/v1\nclass: \"\"\n", 1),
			wantErr: "definition v1 forbids class",
		},
		{
			name:    "explicit empty reproduction",
			doc:     validDefinitionYAML() + "reproduction: {}\n",
			wantErr: "definition v1 forbids reproduction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeDefinition([]byte(tt.doc))
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("DecodeDefinition() unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("DecodeDefinition() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestDefinitionV2RejectsInvalidVersionedFields(t *testing.T) {
	validV2 := validDefinitionV2YAML("reproduction:\n  minimum_valid_runs: 2\n")
	tests := []struct {
		name string
		doc  string
	}{
		{"v2 missing class", strings.Replace(validV2, "class: request-path-performance\n", "", 1)},
		{"v2 noncanonical class", strings.Replace(validV2, "class: request-path-performance", "class: Request_Path_Performance", 1)},
		{"v2 minimum one", strings.Replace(validV2, "minimum_valid_runs: 2", "minimum_valid_runs: 1", 1)},
		{"v2 minimum zero", strings.Replace(validV2, "minimum_valid_runs: 2", "minimum_valid_runs: 0", 1)},
		{"v2 minimum negative", strings.Replace(validV2, "minimum_valid_runs: 2", "minimum_valid_runs: -1", 1)},
		{"v2 unknown top-level field", validV2 + "confidence: 0.95\n"},
		{"v2 unknown reproduction field", strings.Replace(validV2, "  minimum_valid_runs: 2\n", "  minimum_valid_runs: 2\n  environment_diversity: 2\n", 1)},
		{"v2 duplicate class", strings.Replace(validV2, "class: request-path-performance\n", "class: request-path-performance\nclass: request-path-performance\n", 1)},
		{"v2 duplicate minimum", strings.Replace(validV2, "  minimum_valid_runs: 2\n", "  minimum_valid_runs: 2\n  minimum_valid_runs: 3\n", 1)},
		{"v2 null class", strings.Replace(validV2, "class: request-path-performance", "class: null", 1)},
		{"v2 null reproduction", strings.Replace(validV2, "reproduction:\n  minimum_valid_runs: 2", "reproduction: null", 1)},
		{"v2 null minimum", strings.Replace(validV2, "minimum_valid_runs: 2", "minimum_valid_runs: null", 1)},
		{"v1 forbids class", strings.Replace(validDefinitionYAML(), "schema: verdi.experiment/v1\n", "schema: verdi.experiment/v1\nclass: request-path-performance\n", 1)},
		{"v1 forbids reproduction", validDefinitionYAML() + "reproduction:\n  minimum_valid_runs: 2\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if def, err := DecodeDefinition([]byte(tt.doc)); err == nil {
				t.Fatalf("DecodeDefinition() = %+v, nil error", def)
			}
		})
	}
}

func TestDecodeDefinitionHappyPath(t *testing.T) {
	def, err := DecodeDefinition([]byte(validDefinitionYAML()))
	if err != nil {
		t.Fatalf("DecodeDefinition() unexpected error: %v", err)
	}
	if def.ID != "cache-placement-v1" {
		t.Errorf("def.ID = %q, want %q", def.ID, "cache-placement-v1")
	}
	if len(def.Candidates) != 2 {
		t.Fatalf("len(def.Candidates) = %d, want 2", len(def.Candidates))
	}
	if def.Lock != nil {
		t.Errorf("def.Lock = %+v, want nil (no lock block in the fixture)", def.Lock)
	}
}

// mutate returns validDefinitionYAML with old replaced by replacement
// (exactly once), failing the test if old is not found — every negative
// case below mutates from a document already proven to decode and
// validate cleanly, so each test isolates exactly one broken field.
func mutate(t *testing.T, old, replacement string) string {
	t.Helper()
	doc := validDefinitionYAML()
	if !strings.Contains(doc, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(doc, old, replacement, 1)
}

func oneCandidateYAML(t *testing.T) string {
	t.Helper()
	block := "  - id: facts-cache\n    patch: candidates/facts-cache.patch\n    digest: " + factsCachePatchDigest + "\n    base: " + base40 + "\n"
	return mutate(t, block, "")
}

func duplicateCandidateIDsYAML(t *testing.T) string {
	t.Helper()
	return mutate(t, "id: facts-cache", "id: baseline")
}

func duplicateFixtureIDsYAML(t *testing.T) string {
	t.Helper()
	old := "fixtures:\n  - id: request-log\n    digest: " + digestOf("6") + "\n"
	replacement := "fixtures:\n  - id: request-log\n    digest: " + digestOf("6") + "\n  - id: request-log\n    digest: " + digestOf("6") + "\n"
	return mutate(t, old, replacement)
}

func TestDecodeDefinitionRejects(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"unknown schema", mutate(t, "schema: verdi.experiment/v1", "schema: verdi.experiment/v2")},
		{"unknown field", validDefinitionYAML() + "unknown_field: true\n"},
		// A bare trailing scalar breaks mapping syntax — genuine trailing
		// content the parser cannot place. A second "---" YAML document,
		// which go-yaml's single-document decode silently ignores, is
		// covered separately by strictdecode_test.go's trailing-document
		// probes.
		{"trailing data", validDefinitionYAML() + "trailing-garbage-not-a-key\n"},
		{"yaml anchor", mutate(t, "id: baseline", "id: &anchor baseline")},
		{"yaml alias", validDefinitionYAML() + "alias_ref: *nonexistent\n"},
		{"custom tag", mutate(t, "retention_policy: standard-retention-v1", "retention_policy: !custom standard-retention-v1")},
		{"missing schema", strings.Replace(validDefinitionYAML(), "schema: verdi.experiment/v1\n", "", 1)},
		{"bad id grammar", mutate(t, "id: cache-placement-v1", "id: Cache_Placement_V1")},
		{"bad spike grammar", mutate(t, "spike: spec/cache-placement-spike", "spike: cache-placement-spike")},
		{"bad question grammar (no anchor)", mutate(t, "question: spec/request-path#oq-cache-placement", "question: spec/request-path")},
		{"bad base_commit grammar", mutate(t, "base_commit: "+base40, "base_commit: not-a-commit")},
		{"unknown metric type", mutate(t, "type: duration", "type: percentage")},
		{"unknown aggregation", mutate(t, "aggregation: p95", "aggregation: p99")},
		{"unknown direction", mutate(t, "direction: lower", "direction: sideways")},
		{"unknown execution order", mutate(t, "order: deterministic-rotation", "order: random")},
		{"unknown algorithm version", mutate(t, "algorithm: verdi.experiment-recommendation/v1", "algorithm: verdi.experiment-recommendation/v2")},
		{"malformed unit (whitespace)", mutate(t, "unit: ms", "unit: \"m s\"")},
		{"malformed timeout duration", mutate(t, "timeout_per_round: 30s", "timeout_per_round: not-a-duration")},
		{"zero timeout duration", mutate(t, "timeout_per_round: 30s", "timeout_per_round: 0s")},
		{"negative warmups", mutate(t, "warmups: 3", "warmups: -1")},
		{"zero rounds", mutate(t, "rounds: 10", "rounds: 0")},
		{"only one candidate", oneCandidateYAML(t)},
		{"duplicate candidate ids", duplicateCandidateIDsYAML(t)},
		{"patch path mismatch", mutate(t, "patch: candidates/baseline.patch", "patch: candidates/wrong.patch")},
		{"differing candidate base", mutate(t, "    base: "+base40+"\n  - id: facts-cache", "    base: "+repeat("b", 40)+"\n  - id: facts-cache")},
		{"baseline names unregistered candidate", mutate(t, "baseline: baseline", "baseline: nonexistent")},
		{"threshold both arms set", mutate(t, "  baseline_improvement:\n    relative: 0.25\n", "  baseline_improvement:\n    relative: 0.25\n    absolute: 5\n")},
		{"threshold no arm set", mutate(t, "  baseline_improvement:\n    relative: 0.25\n", "  baseline_improvement: {}\n")},
		{"threshold nonpositive relative", mutate(t, "    relative: 0.25\n  candidate_separation", "    relative: -0.25\n  candidate_separation")},
		{"threshold NaN relative", mutate(t, "    relative: 0.25\n  candidate_separation", "    relative: .nan\n  candidate_separation")},
		{"threshold +Inf relative", mutate(t, "    relative: 0.25\n  candidate_separation", "    relative: .inf\n  candidate_separation")},
		{"threshold -Inf relative", mutate(t, "    relative: 0.25\n  candidate_separation", "    relative: -.inf\n  candidate_separation")},
		{"threshold NaN absolute", mutate(t, "  baseline_improvement:\n    relative: 0.25\n", "  baseline_improvement:\n    absolute: .nan\n")},
		{"threshold +Inf absolute", mutate(t, "  baseline_improvement:\n    relative: 0.25\n", "  baseline_improvement:\n    absolute: .inf\n")},
		{"threshold -Inf absolute", mutate(t, "  baseline_improvement:\n    relative: 0.25\n", "  baseline_improvement:\n    absolute: -.inf\n")},
		{"duplicate guard ids", mutate(t, "    - id: tenant-isolation\n", "    - id: behavioral-equivalence\n")},
		{"guard id equals primary metric id", mutate(t, "    - id: behavioral-equivalence\n", "    - id: request-latency\n")},
		{"guard bound nonpositive", mutate(t, "maximum_relative_to_baseline: 0.15", "maximum_relative_to_baseline: -0.15")},
		{"guard bound NaN", mutate(t, "maximum_relative_to_baseline: 0.15", "maximum_relative_to_baseline: .nan")},
		{"guard bound +Inf", mutate(t, "maximum_relative_to_baseline: 0.15", "maximum_relative_to_baseline: .inf")},
		{"guard bound -Inf", mutate(t, "maximum_relative_to_baseline: 0.15", "maximum_relative_to_baseline: -.inf")},
		{"variability nonpositive", mutate(t, "max_relative_spread: 0.1", "max_relative_spread: 0")},
		{"variability NaN", mutate(t, "max_relative_spread: 0.1", "max_relative_spread: .nan")},
		{"variability +Inf", mutate(t, "max_relative_spread: 0.1", "max_relative_spread: .inf")},
		{"variability -Inf", mutate(t, "max_relative_spread: 0.1", "max_relative_spread: -.inf")},
		{"duplicate fixture ids", duplicateFixtureIDsYAML(t)},
		{"protected path leading slash", mutate(t, "  - internal/cache\n", "  - /internal/cache\n")},
		{"protected path traversal", mutate(t, "  - internal/cache\n", "  - internal/../cache\n")},
		{"duplicate protected paths", mutate(t, "protected_paths:\n  - internal/cache\n", "protected_paths:\n  - internal/cache\n  - internal/cache\n")},
		{"tampered lock digest grammar", mutate(t, "protected_paths:\n  - internal/cache\n", "protected_paths:\n  - internal/cache\nlock:\n  definition_digest: not-a-digest\n")},
		{"policy ref empty", mutate(t, "policy:\n  ref: experiment-policy/standard\n", "policy:\n  ref: \"\"\n")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeDefinition([]byte(tt.doc)); err == nil {
				t.Errorf("DecodeDefinition(%s) = nil error, want error", tt.name)
			}
		})
	}
}

func TestExecutionValidateRequiresPositiveIntegralSecondTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		wantErr bool
	}{
		{name: "one second", timeout: "1s"},
		{name: "integral seconds expressed as milliseconds", timeout: "2000ms"},
		{name: "fractional seconds", timeout: "1500ms", wantErr: true},
		{name: "subsecond", timeout: "1ns", wantErr: true},
		{name: "fraction above an integral second", timeout: "1000000001ns", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execution := Execution{
				Rounds:            1,
				Order:             OrderDeterministicRotation,
				TimeoutPerRound:   test.timeout,
				EnvironmentPolicy: "isolated-v1",
			}
			err := execution.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Execution.Validate() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestDefinitionWithLockDecodes(t *testing.T) {
	doc := validDefinitionYAML() + "lock:\n  definition_digest: " + digestOf("9") + "\n"
	def, err := DecodeDefinition([]byte(doc))
	if err != nil {
		t.Fatalf("DecodeDefinition() unexpected error: %v", err)
	}
	if def.Lock == nil || def.Lock.DefinitionDigest != digestOf("9") {
		t.Errorf("def.Lock = %+v, want definition_digest %q", def.Lock, digestOf("9"))
	}
}

// evaluatorArgvYAML returns validDefinitionYAML with argv[0] replaced by
// argv0, keeping the rest of the registration intact.
func evaluatorArgvYAML(t *testing.T, argv0 string) string {
	t.Helper()
	return mutate(t, `  argv: ["./tools/cache-evaluator", "run"]`, `  argv: ["`+argv0+`", "run"]`)
}

// TestDefinitionEvaluatorArgvClasses pins the three classes of
// evaluator.argv[0] and, for the repo-relative class, that only a
// canonical spelling registers. A repo-path-like argv[0] the
// protected-input matcher could not recognize is rejected at REGISTRATION,
// so the patch-side check never has to guess.
func TestDefinitionEvaluatorArgvClasses(t *testing.T) {
	tests := []struct {
		name    string
		argv0   string
		wantErr bool
	}{
		{"repo-relative with ./ prefix", "./tools/cache-evaluator", false},
		{"repo-relative without ./ prefix", "tools/cache-evaluator", false},
		{"absolute external executable", "/usr/bin/env", false},
		{"bare PATH-resolved command", "env", false},
		{"repo-relative with ./ prefix and traversal", "./tools/../tools/cache-evaluator", true},
		{"repo-relative with traversal", "tools/../tools/cache-evaluator", true},
		{"repo-relative with a dot segment", "./tools/./cache-evaluator", true},
		{"repo-relative with an empty segment", "tools//cache-evaluator", true},
		{"repo-relative with a trailing slash", "./tools/cache-evaluator/", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeDefinition([]byte(evaluatorArgvYAML(t, tt.argv0)))
			if tt.wantErr && err == nil {
				t.Errorf("DecodeDefinition() with argv[0] %q = nil error, want error", tt.argv0)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("DecodeDefinition() with argv[0] %q = %v, want nil", tt.argv0, err)
			}
		})
	}
}

func TestDefinitionEvaluatorArgvNoShellString(t *testing.T) {
	doc := mutate(t, "  argv: [\"./tools/cache-evaluator\", \"run\"]\n", "  argv: []\n")
	if _, err := DecodeDefinition([]byte(doc)); err == nil {
		t.Errorf("DecodeDefinition() with empty argv = nil error, want error")
	}
}

func TestDefinitionEvaluatorArgvEndsInRunOperation(t *testing.T) {
	tests := []struct {
		name string
		argv string
	}{
		{"executable only", `["./tools/cache-evaluator"]`},
		{"wrong operation", `["./tools/cache-evaluator", "describe"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeDefinition([]byte(evaluatorArgvYAML(t, tt.argv))); err == nil {
				t.Fatalf("DecodeDefinition() = nil error, want final run-operation error")
			}
		})
	}
}
