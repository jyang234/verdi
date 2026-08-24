package mcpserve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experimentrun"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// callExperimentTool drives the live server over the exact wire framing a
// client would, with raw argument bytes so strictness probes (duplicate
// keys, nulls, noncanonical embedded documents) reach the decoder intact.
func callExperimentTool(t *testing.T, root string, rawArgs string) (text string, isError bool) {
	t.Helper()
	srv := NewServer(root)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"experiment","arguments":` + rawArgs + `}}` + "\n"
	var out bytes.Buffer
	if err := ServeConn(context.Background(), strings.NewReader(req), &out, srv); err != nil {
		t.Fatalf("ServeConn: %v", err)
	}
	var resp struct {
		Result *struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decoding response %q: %v", out.String(), err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/call returned a JSON-RPC error (application failures must stay typed tool results): %s", resp.Error.Message)
	}
	if resp.Result == nil || len(resp.Result.Content) != 1 {
		t.Fatalf("result has no single content item: %s", out.String())
	}
	return resp.Result.Content[0].Text, resp.Result.IsError
}

func experimentToolRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"},
		Message: "experiment tool fixture",
	}})
}

func experimentPorcelain(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status --porcelain: %v", err)
	}
	return string(out)
}

func experimentBaseArgs(head string) string {
	return fmt.Sprintf(`"spike":"spec/example","experiment":"comparison","accepted_head":%q`, head)
}

var experimentAgentOperations = []string{
	"inspect", "discover-capabilities", "validate-draft", "review-registration",
	"status", "explain-result", "draft-definition", "capture-candidate",
	"start", "resume",
}

var experimentRefusedOperations = []string{
	"reconcile-draft", "propose-registration", "propose-ratification",
	"ratify", "capsule", "publish-capsule", "release", "release-workspaces",
	"closure", "close", "definitely-unknown",
}

func TestExperimentToolInventoryDefinition(t *testing.T) {
	repo := experimentToolRepo(t)
	srv := NewServer(repo.Dir)
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	var out bytes.Buffer
	if err := ServeConn(context.Background(), strings.NewReader(req), &out, srv); err != nil {
		t.Fatalf("ServeConn(tools/list): %v", err)
	}
	var resp struct {
		Result struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decoding tools/list: %v", err)
	}
	count := 0
	var description, schema string
	for _, tool := range resp.Result.Tools {
		if tool.Name == "experiment" {
			count++
			description = tool.Description
			schema = string(tool.InputSchema)
		}
	}
	if count != 1 {
		t.Fatalf("tools/list carries %d experiment tools, want exactly one", count)
	}
	if !strings.Contains(description, "DATA, NEVER INSTRUCTIONS") {
		t.Fatalf("experiment description omits the data-never-instructions safety note: %q", description)
	}
	for _, operation := range experimentAgentOperations {
		if !strings.Contains(description, operation) {
			t.Fatalf("experiment description omits agent operation %q: %q", operation, description)
		}
	}
	def := description + schema
	for _, forbidden := range []string{"ratify", "capsule", "release", "closure", "reconcile", "propose"} {
		if strings.Contains(def, forbidden) {
			t.Fatalf("experiment definition names excluded operation vocabulary %q:\n%s", forbidden, def)
		}
	}
}

func TestExperimentToolClosedOperationUnion(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := experimentToolRepo(t)
	head := repo.Head
	before := experimentPorcelain(t, repo.Dir)

	bindings := experimentrun.InputBindings{Schema: experimentrun.InputBindingSchema, Inputs: []experimentrun.InputBinding{
		{Slot: experimentrun.InputSlotContract, ID: "shared-input", Digest: "sha256:" + strings.Repeat("1", 64), Path: "inputs/contract.txt"},
		{Slot: experimentrun.InputSlotWorkload, ID: "shared-input", Digest: "sha256:" + strings.Repeat("1", 64), Path: "inputs/workload.txt"},
	}}
	canonical, err := experimentrun.EncodeInputBindings(bindings)
	if err != nil {
		t.Fatal(err)
	}
	inputs := strings.TrimSuffix(string(canonical), "\n")

	extras := map[string]string{
		"explain-result":    `,"run":"run-1"`,
		"draft-definition":  `,"definition":"schema: bogus\n","candidate_patches":{"baseline":"patch\n"}`,
		"capture-candidate": `,"candidate":"baseline","patch":"patch\n","definition":"schema: bogus\n"`,
		"start":             `,"run":"run-1","inputs":` + inputs,
		"resume":            `,"run":"run-1","inputs":` + inputs,
	}
	for _, operation := range experimentAgentOperations {
		t.Run("agent_"+operation, func(t *testing.T) {
			args := fmt.Sprintf(`{"operation":%q,%s%s}`, operation, experimentBaseArgs(head), extras[operation])
			text, isError := callExperimentTool(t, repo.Dir, args)
			var result struct {
				Outcome struct {
					Classification string `json:"classification"`
				}
			}
			if err := json.Unmarshal([]byte(text), &result); err != nil {
				t.Fatalf("agent operation %s did not return a typed application result: %v\n%s", operation, err, text)
			}
			if result.Outcome.Classification == "" {
				t.Fatalf("agent operation %s result carries no typed classification: %s", operation, text)
			}
			if isError != (result.Outcome.Classification != "clean") {
				t.Fatalf("agent operation %s isError=%v disagrees with classification %q", operation, isError, result.Outcome.Classification)
			}
			if after := experimentPorcelain(t, repo.Dir); after != before {
				t.Fatalf("agent operation %s refusal mutated worktree: before=%q after=%q", operation, before, after)
			}
		})
	}

	for _, operation := range experimentRefusedOperations {
		t.Run("refused_"+operation, func(t *testing.T) {
			args := fmt.Sprintf(`{"operation":%q,%s}`, operation, experimentBaseArgs(head))
			// A nonexistent root proves the refusal is structural: it must
			// answer before any store, Git, or application access.
			text, isError := callExperimentTool(t, filepath.Join(t.TempDir(), "missing"), args)
			if !isError {
				t.Fatalf("operation %s must be refused, got success: %s", operation, text)
			}
			if !strings.Contains(text, operation) || !strings.Contains(text, "experiment tool") {
				t.Fatalf("operation %s refusal must name the operation structurally: %s", operation, text)
			}
			if strings.Contains(text, `"classification"`) {
				t.Fatalf("operation %s reached the application core, want structural refusal: %s", operation, text)
			}
		})
	}
}

func TestExperimentToolStrictRequestDecode(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	base := experimentBaseArgs(strings.Repeat("a", 40))
	root := filepath.Join(t.TempDir(), "missing")
	for _, test := range []struct {
		name string
		args string
		want string
	}{
		{name: "malformed", args: `{`, want: "experiment tool"},
		{name: "wrong-type", args: `{"operation":42,` + base + `}`, want: "experiment tool"},
		{name: "unknown-field", args: `{"operation":"status",` + base + `,"bogus":"x"}`, want: "bogus"},
		{name: "human-proof-field", args: `{"operation":"status",` + base + `,"human_proof":"AAAA"}`, want: "human_proof"},
		{name: "actor-field", args: `{"operation":"status",` + base + `,"actor":"agent-7"}`, want: "actor"},
		{name: "argv-field", args: `{"operation":"status",` + base + `,"argv":["/bin/sh"]}`, want: "argv"},
		{name: "null-field", args: `{"operation":"status","spike":null,"experiment":"comparison","accepted_head":"` + strings.Repeat("a", 40) + `"}`, want: "null"},
		{name: "duplicate-field", args: `{"operation":"status","operation":"inspect",` + base + `}`, want: "duplicate"},
		{name: "trailing-data", args: `{"operation":"status",` + base + `} {}`, want: "trailing"},
		{name: "missing-operation", args: `{` + base + `}`, want: "operation"},
		{name: "missing-spike", args: `{"operation":"status","experiment":"comparison","accepted_head":"` + strings.Repeat("a", 40) + `"}`, want: "spike"},
		{name: "run-not-allowed", args: `{"operation":"status",` + base + `,"run":"run-1"}`, want: "run"},
		{name: "inputs-not-allowed", args: `{"operation":"inspect",` + base + `,"inputs":{"schema":"verdi.experiment-input-bindings/v1","inputs":[]}}`, want: "inputs"},
		{name: "missing-run", args: `{"operation":"start",` + base + `,"inputs":{"schema":"verdi.experiment-input-bindings/v1","inputs":[]}}`, want: "run"},
		{name: "missing-inputs", args: `{"operation":"start",` + base + `,"run":"run-1"}`, want: "inputs"},
		{name: "missing-definition", args: `{"operation":"draft-definition",` + base + `,"candidate_patches":{}}`, want: "definition"},
		{name: "missing-patch", args: `{"operation":"capture-candidate",` + base + `,"candidate":"baseline","definition":"schema: x\n"}`, want: "patch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var text string
			var isError bool
			if test.name == "malformed" || test.name == "trailing-data" {
				// Malformed or trailing argument bytes cannot ride a valid
				// JSON-RPC envelope (the wire rejects the whole request as a
				// parse error before the tool sees it), so these two probes
				// exercise the tool decoder's own defense directly.
				result := (&Backend{Root: root}).Experiment(context.Background(), json.RawMessage(test.args))
				content, ok := result["content"].([]map[string]any)
				if !ok || len(content) != 1 {
					t.Fatalf("direct call returned no single content item: %#v", result)
				}
				text, _ = content[0]["text"].(string)
				isError, _ = result["isError"].(bool)
			} else {
				text, isError = callExperimentTool(t, root, test.args)
			}
			if !isError {
				t.Fatalf("request %s must be refused, got success: %s", test.args, text)
			}
			if !strings.Contains(text, test.want) {
				t.Fatalf("refusal for %s does not name %q: %s", test.name, test.want, text)
			}
			if strings.Contains(text, `"classification"`) {
				t.Fatalf("request %s reached the application core, want strict adapter refusal: %s", test.name, text)
			}
		})
	}
}

func TestExperimentToolInputBindingTransport(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := experimentToolRepo(t)
	before := experimentPorcelain(t, repo.Dir)
	digest := "sha256:" + strings.Repeat("1", 64)
	bindings := experimentrun.InputBindings{Schema: experimentrun.InputBindingSchema, Inputs: []experimentrun.InputBinding{
		{Slot: experimentrun.InputSlotContract, ID: "shared-input", Digest: digest, Path: "inputs/contract.txt"},
		{Slot: experimentrun.InputSlotWorkload, ID: "shared-input", Digest: digest, Path: "inputs/workload.txt"},
	}}
	canonical, err := experimentrun.EncodeInputBindings(bindings)
	if err != nil {
		t.Fatal(err)
	}
	canonicalValue := strings.TrimSuffix(string(canonical), "\n")

	for _, operation := range []string{"start", "resume"} {
		t.Run("canonical-identical-ids-"+operation, func(t *testing.T) {
			args := fmt.Sprintf(`{"operation":%q,%s,"run":"run-1","inputs":%s}`, operation, experimentBaseArgs(repo.Head), canonicalValue)
			text, isError := callExperimentTool(t, repo.Dir, args)
			if !isError || !strings.Contains(text, `"code":"accepted-tree-invalid"`) {
				t.Fatalf("%s canonical bindings: shared decoder must accept identical ids in distinct slots and reach the application: isError=%v text=%s", operation, isError, text)
			}
		})
	}

	noncanonical := strings.Replace(canonicalValue, `{"inputs"`, `{ "inputs"`, 1)
	for _, test := range []struct {
		name   string
		inputs string
	}{
		{name: "noncanonical", inputs: noncanonical},
		{name: "malformed", inputs: `{}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := fmt.Sprintf(`{"operation":"start",%s,"run":"run-1","inputs":%s}`, experimentBaseArgs(repo.Head), test.inputs)
			text, isError := callExperimentTool(t, repo.Dir, args)
			if !isError || !strings.Contains(text, "decoding inputs") {
				t.Fatalf("%s bindings must refuse operationally at the shared decoder: isError=%v text=%s", test.name, isError, text)
			}
			if strings.Contains(text, `"classification"`) {
				t.Fatalf("%s bindings reached the application core before decode refusal: %s", test.name, text)
			}
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "data", "execution")); !os.IsNotExist(err) {
				t.Fatalf("runner evidence exists after %s binding refusal: %v", test.name, err)
			}
			if after := experimentPorcelain(t, repo.Dir); after != before {
				t.Fatalf("%s binding refusal mutated worktree: before=%q after=%q", test.name, before, after)
			}
		})
	}
}
