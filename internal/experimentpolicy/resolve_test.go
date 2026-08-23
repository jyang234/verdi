package experimentpolicy

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

func TestExperimentPolicyResolveMonotoneRefinement(t *testing.T) {
	organization := string(readPayloadFixture(t, "organization.yaml"))
	project := string(readPayloadFixture(t, "project.yaml"))
	decision := resolvePayloadLayers(t, organization, project)
	effective, err := decision.Payload()
	if err != nil {
		t.Fatalf("decision.Payload() error = %v", err)
	}
	if !reflect.DeepEqual(effective.ExperimentPaths, []string{".verdi/specs/active/**/experiments/**"}) {
		t.Fatalf("experiment paths = %#v", effective.ExperimentPaths)
	}
	if !reflect.DeepEqual(effective.CandidatePaths, []string{"spikes/**"}) {
		t.Fatalf("candidate paths = %#v", effective.CandidatePaths)
	}
	if !reflect.DeepEqual(effective.Classes, []string{"request-path-performance"}) {
		t.Fatalf("classes = %#v", effective.Classes)
	}
	if !reflect.DeepEqual(effective.Evaluators, []EvaluatorAllowance{{
		Argv0: "./tools/evaluator", Protocols: []string{experiment.EvaluatorProtocolSchema},
	}}) {
		t.Fatalf("evaluator/protocol intersection = %#v", effective.Evaluators)
	}
	if effective.Limits.ObservationBytes != 262144 || effective.Limits.RetainedArtifactBytes != 8388608 {
		t.Fatalf("minimum limits = %+v", effective.Limits)
	}
	if len(effective.Environments) != 1 || effective.Environments[0].ID != "local-isolated-v1" {
		t.Fatalf("environments = %#v", effective.Environments)
	}
	if got := effective.MandatoryGuards[0].Guards; !reflect.DeepEqual(got, []string{"contract-correct", "integrity-guard"}) {
		t.Fatalf("mandatory guard union = %#v", got)
	}
	if !reflect.DeepEqual(effective.TrustedMeasurementSources, []experiment.Source{
		experiment.SourceEvaluatorMeasured, experiment.SourceHarnessMeasured,
	}) {
		t.Fatalf("trusted measurement source intersection = %#v", effective.TrustedMeasurementSources)
	}
	nonRestored, err := resolvePayloadLayers(t, organization, project, organization).Payload()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(nonRestored, effective) {
		t.Fatalf("later broad layer restored a narrowed allowance or byte ceiling:\ngot  %#v\nwant %#v", nonRestored, effective)
	}

	// Every outward value is a copy; mutating one snapshot cannot alter the
	// sealed reducer result or a later authorization projection.
	effective.Classes[0] = "mutated"
	effective.Environments[0].Grants[0] ^= 0xff
	effective.MandatoryGuards[0].Guards[0] = "mutated"
	again, err := decision.Payload()
	if err != nil {
		t.Fatal(err)
	}
	if again.Classes[0] != "request-path-performance" || again.MandatoryGuards[0].Guards[0] != "contract-correct" {
		t.Fatalf("sealed decision changed through returned snapshot: %#v", again)
	}
	if _, err := decision.Digest(); err != nil {
		t.Fatalf("decision.Digest() after snapshot mutation = %v", err)
	}
}

func TestExperimentPolicyDecisionExposesSealedEffectivePolicyDigest(t *testing.T) {
	selection := selectPayloadLayers(t, string(readPayloadFixture(t, "project.yaml")))
	want, err := selection.EffectiveDigest()
	if err != nil {
		t.Fatalf("selection.EffectiveDigest() error = %v", err)
	}
	decision, err := Resolve(selection)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	got, err := decision.EffectivePolicyDigest()
	if err != nil {
		t.Fatalf("decision.EffectivePolicyDigest() error = %v", err)
	}
	decisionDigest, err := decision.Digest()
	if err != nil {
		t.Fatalf("decision.Digest() error = %v", err)
	}
	if got != want {
		t.Fatalf("effective policy digest = %q, want selection authority %q", got, want)
	}
	if got == decisionDigest {
		t.Fatalf("effective policy digest unexpectedly equals decision identity %q", got)
	}

	decision.authorityDigest = strings.Repeat("0", len(decision.authorityDigest))
	if _, err := decision.EffectivePolicyDigest(); err == nil {
		t.Fatal("EffectivePolicyDigest() accepted a modified decision")
	}
}

func TestExperimentPolicyResolveIsCommutative(t *testing.T) {
	organization := string(readPayloadFixture(t, "organization.yaml"))
	project := string(readPayloadFixture(t, "project.yaml"))
	left := resolveNamedPayloadLayers(t, map[string]string{"alpha": organization, "zulu": project})
	right := resolveNamedPayloadLayers(t, map[string]string{"alpha": project, "zulu": organization})
	leftPayload, err := left.Payload()
	if err != nil {
		t.Fatal(err)
	}
	rightPayload, err := right.Payload()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(leftPayload, rightPayload) {
		t.Fatalf("commutative reduction differs:\nleft=%#v\nright=%#v", leftPayload, rightPayload)
	}
}

func TestExperimentPolicyResolveEnvironmentEqualityUsesCompleteIntersection(t *testing.T) {
	base := string(readPayloadFixture(t, "project.yaml"))
	environment := func(id string, timeout int, value string) string {
		return fmt.Sprintf(`  - id: %s
    grants:
      schema: verdi.execution-grants/v1
      grants:
        - kind: timeouts
          seconds: %d
    declared_environment: {GOMAXPROCS: %q}
`, id, timeout, value)
	}
	withEnvironments := func(environments ...string) string {
		return replaceYAMLSection(base, "environments:", "limits:", "environments:\n"+strings.Join(environments, ""))
	}
	layers := map[string]string{
		"A": withEnvironments(environment("drop", 10, "1"), environment("keep", 30, "1")),
		"B": withEnvironments(environment("drop", 20, "2"), environment("keep", 30, "1")),
		"C": withEnvironments(environment("keep", 30, "1")),
	}
	permutations := [][]string{
		{"A", "B", "C"},
		{"A", "C", "B"},
		{"B", "A", "C"},
		{"B", "C", "A"},
		{"C", "A", "B"},
		{"C", "B", "A"},
	}
	for _, order := range permutations {
		t.Run(strings.Join(order, "-"), func(t *testing.T) {
			decision := resolveNamedPayloadLayers(t, map[string]string{
				"alpha":   layers[order[0]],
				"bravo":   layers[order[1]],
				"charlie": layers[order[2]],
			})
			payload, err := decision.Payload()
			if err != nil {
				t.Fatalf("decision.Payload() error = %v", err)
			}
			if len(payload.Environments) != 1 || payload.Environments[0].ID != "keep" {
				t.Fatalf("environments = %#v, want only keep", payload.Environments)
			}
			if got := payload.Environments[0].DeclaredEnvironment; !reflect.DeepEqual(got, map[string]string{"GOMAXPROCS": "1"}) {
				t.Fatalf("keep declared environment = %#v", got)
			}
		})
	}
}

func TestExperimentPolicyResolveRefusesSurvivingEnvironmentMismatch(t *testing.T) {
	base := string(readPayloadFixture(t, "project.yaml"))
	environment := func(id string, timeout int, value string) string {
		return fmt.Sprintf(`  - id: %s
    grants:
      schema: verdi.execution-grants/v1
      grants:
        - kind: timeouts
          seconds: %d
    declared_environment: {GOMAXPROCS: %q}
`, id, timeout, value)
	}
	withEnvironments := func(environments ...string) string {
		return replaceYAMLSection(base, "environments:", "limits:", "environments:\n"+strings.Join(environments, ""))
	}
	tests := []struct {
		name       string
		mismatched string
		want       string
	}{
		{name: "grant bytes", mismatched: environment("keep", 31, "1"), want: "grant bytes differ"},
		{name: "declared values", mismatched: environment("keep", 30, "2"), want: "declared environment differs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection := selectPayloadLayers(t,
				withEnvironments(environment("drop", 10, "1"), environment("keep", 30, "1")),
				withEnvironments(environment("drop", 10, "1"), tt.mismatched),
				withEnvironments(environment("keep", 30, "1")),
			)
			_, err := Resolve(selection)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Resolve() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestExperimentPolicyResolveRefusalsAndNonRestoration(t *testing.T) {
	organization := string(readPayloadFixture(t, "organization.yaml"))
	project := string(readPayloadFixture(t, "project.yaml"))
	tests := []struct {
		name   string
		layers []string
		want   string
	}{
		{
			name:   "empty class intersection is denial",
			layers: []string{organization, strings.Replace(project, "classes: [request-path-performance]", "classes: [storage-throughput]", 1)},
			want:   "classes intersection is empty",
		},
		{
			name:   "same id grant mismatch",
			layers: []string{organization, strings.Replace(project, "seconds: 30", "seconds: 20", 1)},
			want:   "grant bytes differ",
		},
		{
			name:   "same id declared values mismatch",
			layers: []string{organization, strings.Replace(project, `GOMAXPROCS: "1"`, `GOMAXPROCS: "2"`, 1)},
			want:   "declared environment differs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selection := selectPayloadLayers(t, tt.layers...)
			_, err := Resolve(selection)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Resolve() error = %v, want containing %q", err, tt.want)
			}
		})
	}

	// A syntactically valid empty environment layer is a dominating denial;
	// a later broad layer cannot restore the removed id.
	denying := replaceYAMLSection(project, "environments:", "limits:", "environments: []\n")
	selection := selectPayloadLayers(t, organization, denying, organization)
	_, err := Resolve(selection)
	if err == nil || !strings.Contains(err.Error(), "environments intersection is empty") {
		t.Fatalf("Resolve(non-restoration) error = %v", err)
	}

	missing := selectNamedPayloadLayers(t, map[string]string{"empty": ""})
	_, err = Resolve(missing)
	if err == nil || !strings.Contains(err.Error(), "no applicable experiment_execution payload") {
		t.Fatalf("Resolve(missing payload) error = %v", err)
	}
}

func TestExperimentPolicyResolveTypesOnlyValidDenialOutcomes(t *testing.T) {
	organization := string(readPayloadFixture(t, "organization.yaml"))
	project := string(readPayloadFixture(t, "project.yaml"))
	tests := []struct {
		name        string
		layers      []string
		wantRefusal bool
	}{
		{
			name: "disjoint classes",
			layers: []string{organization, strings.Replace(project,
				"classes: [request-path-performance]", "classes: [storage-throughput]", 1)},
			wantRefusal: true,
		},
		{
			name: "disjoint environments",
			layers: []string{organization, strings.Replace(project,
				"id: local-isolated-v1", "id: other-isolated-v1", 1)},
			wantRefusal: true,
		},
		{
			name: "malformed surviving environment refinement",
			layers: []string{organization, strings.Replace(project,
				"seconds: 30", "seconds: 20", 1)},
			wantRefusal: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(selectPayloadLayers(t, tt.layers...))
			if err == nil {
				t.Fatal("Resolve() error = nil")
			}
			wrapped := fmt.Errorf("policy adapter: %w", err)
			var refusal *RefusalError
			if got := errors.As(wrapped, &refusal); got != tt.wantRefusal {
				t.Fatalf("errors.As(Resolve() error, *RefusalError) = %v, want %v: %v", got, tt.wantRefusal, err)
			}
		})
	}

	_, err := Resolve(selectNamedPayloadLayers(t, map[string]string{"empty": ""}))
	var refusal *RefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("missing applicable payload error = %v, want *RefusalError", err)
	}
	_, err = Resolve(nil)
	refusal = nil
	if errors.As(err, &refusal) {
		t.Fatalf("nil-selection integrity error = %v, must not be a RefusalError", err)
	}
}

func TestExperimentPolicyAuthorizationProjectionIsExact(t *testing.T) {
	decision := resolvePayloadLayers(t,
		string(readPayloadFixture(t, "organization.yaml")),
		string(readPayloadFixture(t, "project.yaml")),
	)
	def, capabilities := authorizationOperands(t)
	authorization, err := Authorize(decision, AuthorizationInput{
		Definition:     def,
		Capabilities:   capabilities,
		ExperimentPath: ".verdi/specs/active/request-path-spike/experiments/request-path-v2/experiment.yaml",
		CandidatePaths: []string{"spikes/cache.go"},
	})
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	effective, err := decision.Payload()
	if err != nil {
		t.Fatal(err)
	}
	environment := effective.Environments[0]
	if authorization.EnvironmentPolicy != "local-isolated-v1" || authorization.AuthorityDigest == "" {
		t.Fatalf("authorization identity = %+v", authorization)
	}
	if !bytes.Equal(authorization.GrantBytes, environment.Grants) {
		t.Fatalf("grant projection differs from exact policy bytes")
	}
	if !reflect.DeepEqual(authorization.DeclaredEnv, map[string]string{"GOMAXPROCS": "1"}) {
		t.Fatalf("declared env = %#v", authorization.DeclaredEnv)
	}
	if authorization.ObservationBytes != 262144 {
		t.Fatalf("observation bytes = %d, want 262144", authorization.ObservationBytes)
	}
	authorization.GrantBytes[0] ^= 0xff
	authorization.DeclaredEnv["GOMAXPROCS"] = "ambient-or-mutated"
	again, err := Authorize(decision, AuthorizationInput{
		Definition: def, Capabilities: capabilities,
		ExperimentPath: ".verdi/specs/active/request-path-spike/experiments/request-path-v2/experiment.yaml",
		CandidatePaths: []string{"spikes/cache.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if again.DeclaredEnv["GOMAXPROCS"] != "1" || bytes.Equal(again.GrantBytes, authorization.GrantBytes) {
		t.Fatalf("authorization output aliases caller mutation: %+v", again)
	}
}

func TestExperimentPolicyAuthorizationRefusesUnknownOperandsAndMissingGuards(t *testing.T) {
	organization := string(readPayloadFixture(t, "organization.yaml"))
	project := string(readPayloadFixture(t, "project.yaml"))
	base := resolvePayloadLayers(t, organization, project)
	def, capabilities := authorizationOperands(t)
	valid := AuthorizationInput{
		Definition: def, Capabilities: capabilities,
		ExperimentPath: ".verdi/specs/active/request-path-spike/experiments/request-path-v2/experiment.yaml",
		CandidatePaths: []string{"spikes/cache.go"},
	}
	tests := []struct {
		name   string
		change func(*AuthorizationInput)
		want   string
	}{
		{"unknown class", func(in *AuthorizationInput) { in.Definition.Class = "unregistered-class" }, "class"},
		{"unknown evaluator", func(in *AuthorizationInput) { in.Definition.Evaluator.Argv[0] = "./tools/other" }, "evaluator"},
		{"unknown environment", func(in *AuthorizationInput) { in.Definition.Execution.EnvironmentPolicy = "missing-environment" }, "environment"},
		{"experiment path outside allowance", func(in *AuthorizationInput) { in.ExperimentPath = "tmp/experiment.yaml" }, "experiment path"},
		{"candidate path outside allowance", func(in *AuthorizationInput) { in.CandidatePaths = []string{"internal/secret.go"} }, "candidate path"},
		{"missing mandatory guard", func(in *AuthorizationInput) { in.Definition.Decision.Guards = in.Definition.Decision.Guards[:1] }, "mandatory guard"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := valid
			in.Definition = cloneDefinitionForPolicyTest(valid.Definition)
			in.Capabilities = valid.Capabilities
			in.CandidatePaths = append([]string(nil), valid.CandidatePaths...)
			tt.change(&in)
			_, err := Authorize(base, in)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Authorize() error = %v, want containing %q", err, tt.want)
			}
		})
	}

	harnessOnly := strings.Replace(project,
		"trusted_measurement_sources: [evaluator-measured, harness-measured]",
		"trusted_measurement_sources: [harness-measured]", 1)
	decision := resolvePayloadLayers(t, organization, harnessOnly)
	_, err := Authorize(decision, valid)
	if err == nil || !strings.Contains(err.Error(), "measurement source") {
		t.Fatalf("Authorize(untrusted evaluator measurements) error = %v", err)
	}
}

func authorizationOperands(t *testing.T) (experiment.Definition, experiment.Capabilities) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "experiment", "testdata", "definition-v2.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	def, err := experiment.DecodeDefinition(data)
	if err != nil {
		t.Fatal(err)
	}
	def.Decision.Guards = []experiment.Guard{{ID: "contract-correct"}, {ID: "integrity-guard"}}
	capabilities := experiment.Capabilities{
		Schema:           experiment.CapabilitiesSchemaV2,
		EvaluatorVersion: "fixture/v1",
		ProtocolVersions: []string{experiment.EvaluatorProtocolSchema, experiment.ObservationSchemaV2},
		Metrics: []experiment.CapabilityMetric{{
			ID: "request-latency", Type: experiment.MetricDuration, Unit: "ms", Direction: experiment.DirectionLower,
		}},
		Guards:      []string{"contract-correct", "integrity-guard"},
		Environment: []string{"GOMAXPROCS"},
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := capabilities.Validate(); err != nil {
		t.Fatalf("capabilities: %v", err)
	}
	return def, capabilities
}

func cloneDefinitionForPolicyTest(in experiment.Definition) experiment.Definition {
	out := in
	out.Evaluator.Argv = append([]string(nil), in.Evaluator.Argv...)
	out.Decision.Guards = append([]experiment.Guard(nil), in.Decision.Guards...)
	return out
}

func resolvePayloadLayers(t *testing.T, payloads ...string) *Decision {
	t.Helper()
	selection := selectPayloadLayers(t, payloads...)
	decision, err := Resolve(selection)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	return decision
}

func resolveNamedPayloadLayers(t *testing.T, payloads map[string]string) *Decision {
	t.Helper()
	selection := selectNamedPayloadLayers(t, payloads)
	decision, err := Resolve(selection)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	return decision
}

func selectPayloadLayers(t *testing.T, payloads ...string) *contextcompile.ApplicablePayloadSelection {
	t.Helper()
	named := make(map[string]string, len(payloads))
	for i, payload := range payloads {
		named[string(rune('a'+i))] = payload
	}
	return selectNamedPayloadLayers(t, named)
}

func selectNamedPayloadLayers(t *testing.T, payloads map[string]string) *contextcompile.ApplicablePayloadSelection {
	t.Helper()
	root := t.TempDir()
	copyAuthorityFixture(t, root, "constitution.md")
	copyAuthorityFixture(t, root, "profiles/solo-default.md")
	names := make([]string, 0, len(payloads))
	for name := range payloads {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		payload := payloads[name]
		payloadBlock := "payloads: {}"
		if payload != "" {
			payloadBlock = "payloads:\n  " + PayloadKind + ":\n" + indentPayload(payload, 4)
		}
		doc := `---
schema: verdi.policy/v1
id: policy/` + name + `
kind: policy
title: "Experiment policy ` + name + `"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
` + payloadBlock + `
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Deterministic experiment policy layer.
`
		path := filepath.Join(root, ".verdi", "policy", "policies", name+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	store, err := policyauthority.Load(root)
	if err != nil {
		t.Fatalf("policyauthority.Load() error = %v", err)
	}
	effective, err := policyauthority.Resolve(store)
	if err != nil {
		t.Fatalf("policyauthority.Resolve() error = %v", err)
	}
	selection, err := contextcompile.SelectApplicablePayloads(effective, PayloadKind, contextcompile.PayloadSelectionInput{
		Request: policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Phase:   contextcompile.PhaseBuild,
	})
	if err != nil {
		t.Fatalf("SelectApplicablePayloads() error = %v", err)
	}
	return selection
}

func copyAuthorityFixture(t *testing.T, root, rel string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "policyartifact", "testdata", "store", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, ".verdi", "policy", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func indentPayload(payload string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(strings.TrimSuffix(payload, "\n"), "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n") + "\n"
}

func replaceYAMLSection(input, start, end, replacement string) string {
	startIndex := strings.Index(input, start)
	endIndex := strings.Index(input[startIndex:], end)
	if startIndex < 0 || endIndex < 0 {
		return input
	}
	return input[:startIndex] + replacement + input[startIndex+endIndex:]
}
