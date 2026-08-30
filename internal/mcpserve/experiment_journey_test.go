package mcpserve

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experimentrun"
)

func TestExperimentToolRegistryJourney(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	wantAgentOperations := []string{
		"capture-candidate", "discover-capabilities", "draft-definition", "explain-result", "inspect",
		"resume", "review-registration", "start", "status", "validate-draft",
	}
	registeredAgentOperations := make([]string, 0, len(experimentToolAgentOperations))
	for operation := range experimentToolAgentOperations {
		registeredAgentOperations = append(registeredAgentOperations, operation)
	}
	sort.Strings(registeredAgentOperations)
	if !reflect.DeepEqual(registeredAgentOperations, wantAgentOperations) {
		t.Fatalf("production MCP experiment agent registry=%v, want fixed authority set %v", registeredAgentOperations, wantAgentOperations)
	}

	wantRefusedOperations := map[string]bool{
		"reconcile-draft":      true,
		"propose-registration": true,
		"propose-ratification": true,
		"ratify":               false,
		"capsule":              false,
		"publish-capsule":      false,
		"release":              false,
		"release-workspaces":   false,
		"closure":              false,
		"close":                false,
	}
	if !reflect.DeepEqual(experimentToolRefusedOperations, wantRefusedOperations) {
		t.Fatalf("production MCP experiment refusal registry=%v, want fixed authority map %v", experimentToolRefusedOperations, wantRefusedOperations)
	}

	repo := experimentToolRepo(t)
	before := experimentPorcelain(t, repo.Dir)
	bindings := experimentrun.InputBindings{Schema: experimentrun.InputBindingSchema, Inputs: []experimentrun.InputBinding{
		{Slot: experimentrun.InputSlotContract, ID: "shared-input", Digest: "sha256:" + strings.Repeat("1", 64), Path: "inputs/contract.txt"},
		{Slot: experimentrun.InputSlotWorkload, ID: "shared-input", Digest: "sha256:" + strings.Repeat("1", 64), Path: "inputs/workload.txt"},
	}}
	canonical, err := experimentrun.EncodeInputBindings(bindings)
	if err != nil {
		t.Fatal(err)
	}
	extras := map[string]string{
		"explain-result":    `,"run":"run-1"`,
		"draft-definition":  `,"definition":"schema: bogus\n","candidate_patches":{"baseline":"patch\n"}`,
		"capture-candidate": `,"candidate":"baseline","patch":"patch\n","definition":"schema: bogus\n"`,
		"start":             `,"run":"run-1","inputs":` + strings.TrimSuffix(string(canonical), "\n"),
		"resume":            `,"run":"run-1","inputs":` + strings.TrimSuffix(string(canonical), "\n"),
	}
	for _, operation := range registeredAgentOperations {
		args := fmt.Sprintf(`{"operation":%q,%s%s}`, operation, experimentBaseArgs(repo.Head), extras[operation])
		text, isError := callExperimentTool(t, repo.Dir, args)
		var result struct {
			Outcome struct {
				Classification string `json:"classification"`
			} `json:"outcome"`
		}
		if err := json.Unmarshal([]byte(text), &result); err != nil || result.Outcome.Classification == "" {
			t.Fatalf("registered agent operation %s did not reach a typed application result: err=%v text=%s", operation, err, text)
		}
		if isError != (result.Outcome.Classification != "clean") {
			t.Fatalf("registered agent operation %s isError=%v disagrees with classification %q", operation, isError, result.Outcome.Classification)
		}
		if after := experimentPorcelain(t, repo.Dir); after != before {
			t.Fatalf("registered agent operation %s mutated worktree: before=%q after=%q", operation, before, after)
		}
	}

	registeredRefusals := make([]string, 0, len(experimentToolRefusedOperations))
	for operation := range experimentToolRefusedOperations {
		registeredRefusals = append(registeredRefusals, operation)
	}
	sort.Strings(registeredRefusals)
	for _, operation := range registeredRefusals {
		args := fmt.Sprintf(`{"operation":%q,%s}`, operation, experimentBaseArgs(repo.Head))
		text, isError := callExperimentTool(t, filepath.Join(t.TempDir(), "missing"), args)
		wantReason := "Wave 5B agent surface"
		if experimentToolRefusedOperations[operation] {
			wantReason = "human-only"
		}
		if !isError || !strings.Contains(text, operation) || !strings.Contains(text, wantReason) {
			t.Fatalf("registered refusal %s isError=%v text=%q, want structural %q refusal", operation, isError, text, wantReason)
		}
		if strings.Contains(text, `"classification"`) {
			t.Fatalf("registered refusal %s reached the application core: %s", operation, text)
		}
	}
}
