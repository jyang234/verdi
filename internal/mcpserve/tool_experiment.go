// The `experiment` tool: the agent-facing MCP adapter over the shared
// comparative-experiment application core (Wave 5B, design §8; SI-145,
// SI-148). One strict tool-call decoder feeds a closed tagged operation
// union; every operation this adapter class may not invoke — the human-only
// operations and every later-wave lifecycle operation — is structurally
// refused before any store, Git, or application access. The adapter adds no
// application, policy, authorization, execution, or result semantics: it
// strict-decodes one typed request, mints its own unauthenticated agent
// attribution, invokes one application operation, and renders the returned
// typed result with its exact clean/verdict/operational classification.
package mcpserve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"testing/fstest"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentapp"
	"github.com/jyang234/verdi/internal/experimentdecision"
	"github.com/jyang234/verdi/internal/experimentevaluator"
	"github.com/jyang234/verdi/internal/experimentpolicy"
	"github.com/jyang234/verdi/internal/experimentrun"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/specstate"
)

// experimentToolAgentOperations is the closed Wave 5B agent operation union:
// exactly the operations an adapter-controlled agent may invoke, with the
// per-operation fields each one requires beyond the shared identity
// envelope. No other request field is accepted for that operation.
//
// vocab:identity — MCP operation-name/request-field grammar (identity)
var experimentToolAgentOperations = map[string]map[string]bool{
	"inspect":               {},
	"discover-capabilities": {},
	"validate-draft":        {},
	"review-registration":   {},
	"status":                {},
	"explain-result":        {"run": true},
	"draft-definition":      {"definition": true, "candidate_patches": true},
	"capture-candidate":     {"candidate": true, "patch": true, "definition": true},
	"start":                 {"run": true, "inputs": true},
	"resume":                {"run": true, "inputs": true},
}

// experimentToolRefusedOperations names every operation this adapter class
// structurally refuses: true marks a human-only operation (MCP has no human
// proof path and never manufactures reconciliation or registration
// authority), false marks a later-wave lifecycle operation that is not part
// of the Wave 5B agent surface.
//
// vocab:identity — MCP operation-name grammar (identity)
var experimentToolRefusedOperations = map[string]bool{
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

// experimentToolRequest is the one strict tool-call request. Inputs stays
// raw: its exact value bytes (plus the canonical file framing newline the
// JSON value grammar cannot carry) go to the shared experimentrun codec —
// never a second JSON grammar, path, or adapter-local map.
type experimentToolRequest struct {
	Operation        string            `json:"operation"`
	Spike            string            `json:"spike"`
	Experiment       string            `json:"experiment"`
	AcceptedHead     string            `json:"accepted_head"`
	Run              string            `json:"run"`
	Definition       string            `json:"definition"`
	Candidate        string            `json:"candidate"`
	Patch            string            `json:"patch"`
	CandidatePatches map[string]string `json:"candidate_patches"`
	Inputs           json.RawMessage   `json:"inputs"`
}

// experimentToolBaseFields is the shared identity envelope every operation
// requires.
var experimentToolBaseFields = []string{"operation", "spike", "experiment", "accepted_head"}

// decodeExperimentToolRequest is the single strict decoder: it rejects
// malformed JSON, non-object arguments, duplicate and null fields at any
// depth, unknown and trailing fields, unknown operations, and per-operation
// missing or extra fields — all before any filesystem or Git access.
func decodeExperimentToolRequest(raw json.RawMessage) (experimentToolRequest, error) {
	var request experimentToolRequest
	// encoding/json silently replaces invalid UTF-8 with U+FFFD, which
	// would rewrite exact identity, definition, or patch bytes instead of
	// refusing them — validate the raw wire bytes first.
	if !utf8.Valid(raw) {
		return request, fmt.Errorf("request arguments are not valid UTF-8")
	}
	keys, err := experimentToolRequestKeys(raw)
	if err != nil {
		return request, err
	}
	if err := strictUnmarshal(raw, &request); err != nil {
		return request, err
	}
	if !keys["operation"] {
		return request, fmt.Errorf("the operation field is required")
	}
	fields, known := experimentToolAgentOperations[request.Operation]
	if !known {
		if _, refused := experimentToolRefusedOperations[request.Operation]; refused {
			if experimentToolRefusedOperations[request.Operation] {
				return request, fmt.Errorf("operation %q is human-only and has no MCP path", request.Operation)
			}
			return request, fmt.Errorf("operation %q is not part of the Wave 5B agent surface", request.Operation)
		}
		return request, fmt.Errorf("unknown operation %q", request.Operation)
	}
	for _, field := range experimentToolBaseFields {
		if !keys[field] {
			return request, fmt.Errorf("operation %q requires the %s field", request.Operation, field)
		}
	}
	required := make([]string, 0, len(fields))
	for field := range fields {
		required = append(required, field)
	}
	sort.Strings(required)
	for _, field := range required {
		if !keys[field] {
			return request, fmt.Errorf("operation %q requires the %s field", request.Operation, field)
		}
	}
	base := map[string]bool{}
	for _, field := range experimentToolBaseFields {
		base[field] = true
	}
	for key := range keys {
		if !base[key] && !fields[key] {
			return request, fmt.Errorf("operation %q does not accept the %s field", request.Operation, key)
		}
	}
	return request, nil
}

// experimentToolRequestKeys token-scans the raw arguments value, returning
// the top-level key set and rejecting what encoding/json tolerates:
// duplicate object keys and null values, at any depth.
func experimentToolRequestKeys(raw []byte) (map[string]bool, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("malformed request: %v", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("request arguments must be a JSON object")
	}
	keys := map[string]bool{}
	if err := experimentToolScanObject(dec, keys); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing data after the request object")
	}
	return keys, nil
}

func experimentToolScanObject(dec *json.Decoder, keys map[string]bool) error {
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("malformed request: %v", err)
		}
		key, ok := tok.(string)
		if !ok {
			return fmt.Errorf("malformed request object key %v", tok)
		}
		if keys != nil {
			if keys[key] {
				return fmt.Errorf("duplicate request field %q", key)
			}
			keys[key] = true
		}
		if err := experimentToolScanValue(dec, key); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("malformed request: %v", err)
	}
	return nil
}

func experimentToolScanValue(dec *json.Decoder, field string) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("malformed request: %v", err)
	}
	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			// Nested objects track their own duplicate keys but do not add
			// to the top-level key set.
			nested := map[string]bool{}
			return experimentToolScanObject(dec, nested)
		case '[':
			for dec.More() {
				if err := experimentToolScanValue(dec, field); err != nil {
					return err
				}
			}
			if _, err := dec.Token(); err != nil {
				return fmt.Errorf("malformed request: %v", err)
			}
		}
	case nil:
		return fmt.Errorf("request field %q is null", field)
	}
	return nil
}

// Experiment is the tools/call handler for the experiment tool.
func (b *Backend) Experiment(ctx context.Context, raw json.RawMessage) map[string]any {
	request, err := decodeExperimentToolRequest(raw)
	if err != nil {
		return toolError("experiment tool: " + err.Error())
	}
	actor, err := experimentapp.NewDelegatedAgent("verdi-mcp", "")
	if err != nil {
		return toolError("experiment tool: " + err.Error())
	}
	identity := experimentapp.Identity{
		CheckoutRoot: b.Root, Spike: request.Spike,
		ExperimentID: request.Experiment, ExpectedAcceptedHEAD: request.AcceptedHead, Actor: actor,
	}
	service, err := newExperimentToolService(b.Root)
	if err != nil {
		return toolError("experiment tool: " + err.Error())
	}

	switch request.Operation {
	case "inspect":
		result := service.Inspect(ctx, identity)
		return experimentToolResult(result.Outcome, result)
	case "discover-capabilities":
		result := service.DiscoverCapabilities(ctx, identity)
		return experimentToolResult(result.Outcome, result)
	case "validate-draft":
		result := service.ValidateDraft(ctx, identity)
		return experimentToolResult(result.Outcome, result)
	case "review-registration":
		result := service.ReviewRegistration(ctx, identity)
		return experimentToolResult(result.Outcome, result)
	case "status":
		result := service.Status(ctx, identity)
		return experimentToolResult(result.Outcome, result)
	case "explain-result":
		result := service.Explain(ctx, identity, experimentapp.ExplainInput{Run: request.Run})
		return experimentToolResult(result.Outcome, result)
	case "draft-definition":
		patches := make(map[string][]byte, len(request.CandidatePatches))
		for id, patch := range request.CandidatePatches {
			patches[id] = []byte(patch)
		}
		result := service.DraftDefinition(ctx, identity, experimentapp.DraftDefinitionInput{
			DefinitionBytes: []byte(request.Definition), CandidatePatches: patches,
		})
		return experimentToolResult(result.Outcome, result)
	case "capture-candidate":
		result := service.CaptureCandidate(ctx, identity, experimentapp.CaptureCandidateInput{
			CandidateID: request.Candidate, PatchBytes: []byte(request.Patch), DefinitionBytes: []byte(request.Definition),
		})
		return experimentToolResult(result.Outcome, result)
	case "start", "resume":
		// The canonical binding-document file form ends with the canonical
		// trailing newline, which a JSON value cannot carry — re-add exactly
		// that framing byte, then let the one shared codec judge the bytes.
		bindings, err := experimentrun.DecodeInputBindings(append(append([]byte(nil), request.Inputs...), '\n'))
		if err != nil {
			return toolError("experiment tool: decoding inputs: " + err.Error())
		}
		input := experimentapp.ExecutionInput{Run: request.Run, Bindings: bindings}
		if request.Operation == "start" {
			result := service.Start(ctx, identity, input)
			return experimentToolResult(result.Outcome, result)
		}
		result := service.Resume(ctx, identity, input)
		return experimentToolResult(result.Outcome, result)
	default:
		// vocab:identity — internal invariant panic text, never a display surface
		panic("closed experiment operation escaped the decoder")
	}
}

// experimentToolResult renders one typed application result as canonical
// JSON, marking verdict and operational classifications as tool errors while
// preserving the typed projection — never a JSON-RPC framing error.
func experimentToolResult(outcome experimentapp.Outcome, result any) map[string]any {
	rendered := toolJSON(result)
	if outcome.Classification != experimentapp.ClassificationClean {
		rendered["isError"] = true
	}
	return rendered
}

// The port implementations below compose the same shared internal packages
// the CLI adapter composes (gitx plumbing, policyauthority + contextcompile
// + experimentpolicy resolution, execworkspace + experimentevaluator
// discovery, experimentdecision verification, the experimentrun delegate).
// Behavioral equivalence with the CLI wiring is pinned by
// internal/experimentapp's adapter-conformance suite.

type experimentToolGit struct{}

func (experimentToolGit) ResolveDefaultBranch(ctx context.Context, root string) (experimentapp.DefaultBranch, error) {
	branch, ok := specstate.ResolveDefaultBranch(ctx, root)
	if !ok {
		return experimentapp.DefaultBranch{}, fmt.Errorf("experiment tool: default branch is unresolved")
	}
	head, err := gitx.RevParse(ctx, root, branch.Ref)
	if err != nil {
		return experimentapp.DefaultBranch{}, err
	}
	return experimentapp.DefaultBranch{Name: branch.Name, Ref: branch.Ref, Head: head}, nil
}

func (experimentToolGit) ListTree(ctx context.Context, root, commit string) ([]experimentapp.GitTreeEntry, error) {
	entries, err := gitx.LsTreeEntries(ctx, root, commit)
	if err != nil {
		return nil, err
	}
	result := make([]experimentapp.GitTreeEntry, len(entries))
	for index, entry := range entries {
		result[index] = experimentapp.GitTreeEntry{Mode: entry.Mode, Type: entry.Type, Object: entry.Object, Path: entry.Path}
	}
	return result, nil
}

func (experimentToolGit) ReadBlob(ctx context.Context, root, commit, object, path string) ([]byte, error) {
	resolved, err := gitx.RevParse(ctx, root, commit+":"+path)
	if err != nil {
		return nil, err
	}
	if resolved != object {
		return nil, fmt.Errorf("experiment tool: blob %s at %s resolved to %s, want %s", path, commit, resolved, object)
	}
	return gitx.Show(ctx, root, commit, path)
}

type experimentToolPolicyResolver struct{}

func (experimentToolPolicyResolver) ResolvePolicy(ctx context.Context, request experimentapp.PolicyRequest) (*experimentpolicy.Decision, error) {
	var store *policyauthority.Store
	var err error
	if request.AcceptedCommit == "" {
		store, err = policyauthority.Load(request.CheckoutRoot)
	} else {
		var source fs.FS
		source, err = experimentToolAcceptedTreeFS(ctx, request.CheckoutRoot, request.AcceptedCommit)
		if err == nil {
			store, err = policyauthority.LoadFromSource(source)
		}
	}
	if err != nil {
		return nil, err
	}
	effective, err := policyauthority.Resolve(store)
	if err != nil {
		return nil, err
	}
	selection, err := contextcompile.SelectApplicablePayloads(effective, experimentpolicy.PayloadKind, contextcompile.PayloadSelectionInput{
		Request:       policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		CandidatePath: request.ExperimentPath, CandidateRef: request.Spike,
		Phase: contextcompile.PhaseDesign, Environment: "local",
	})
	if err != nil {
		return nil, err
	}
	return experimentpolicy.Resolve(selection)
}

type experimentToolCapabilityDiscoverer struct{}

func (experimentToolCapabilityDiscoverer) DiscoverCapabilities(ctx context.Context, request experimentapp.CapabilityRequest) (experimentapp.CapabilityDiscovery, error) {
	if len(request.Definition.Evaluator.Argv) == 0 {
		return experimentapp.CapabilityDiscovery{}, fmt.Errorf("experiment tool: evaluator argv is empty")
	}
	envRoot, err := os.MkdirTemp("", "verdi-experiment-env-")
	if err != nil {
		return experimentapp.CapabilityDiscovery{}, err
	}
	defer func() { _ = os.RemoveAll(envRoot) }()
	grants := execworkspace.GrantSet{Grants: []execworkspace.Grant{
		{Kind: execworkspace.GrantNetwork},
		{Kind: execworkspace.GrantProcessExecution, Argv0s: []string{request.Definition.Evaluator.Argv[0]}},
		{Kind: execworkspace.GrantTimeouts, Seconds: 30},
	}}
	profile, _, err := execworkspace.BuildProfile(request.CheckoutRoot, envRoot, grants, map[string]string{})
	if err != nil {
		return experimentapp.CapabilityDiscovery{}, err
	}
	discovery, err := experimentevaluator.Discover(ctx, profile, experimentevaluator.DiscoverInput{
		Launch:             experimentevaluator.Launch{Directory: request.CheckoutRoot, Argv: request.Definition.Evaluator.Argv, Digest: request.Definition.Evaluator.Digest},
		CapabilitiesDigest: request.Definition.Evaluator.CapabilitiesDigest,
	})
	if err != nil {
		return experimentapp.CapabilityDiscovery{}, err
	}
	return experimentapp.CapabilityDiscovery{Bytes: discovery.Bytes}, nil
}

type experimentToolResultVerifier struct{}

func (experimentToolResultVerifier) VerifyResult(definition experiment.Definition, observations []experiment.Observation, receipt *experiment.ExecutionReceipt, result experiment.Result) error {
	return experimentdecision.VerifyResult(definition, observations, receipt, result)
}

func newExperimentToolService(root string) (*experimentapp.Service, error) {
	materializer, err := execworkspace.NewMaterializer(root, root, execworkspace.NewGitReconciler(root))
	if err != nil {
		return nil, err
	}
	runner, err := experimentapp.NewRunDelegate(experimentapp.RunDependencies{
		Materializer: materializer,
		Versions:     experiment.ReceiptVersions{Verdi: "dev", RecommendationEngine: string(experiment.AlgorithmV1)},
	})
	if err != nil {
		return nil, err
	}
	return experimentapp.NewService(experimentToolPolicyResolver{}, experimentToolGit{}, experimentToolCapabilityDiscoverer{}, experimentToolResultVerifier{}, runner)
}

// experimentToolAcceptedTreeFS materializes one exact accepted Git tree as a
// read-only source, preserving each accepted entry's file kind: a
// mode-120000 entry stays a symlink (its blob data is the link target and is
// never followed) so consumers apply their own refusals, and any other
// non-regular blob mode fails closed.
func experimentToolAcceptedTreeFS(ctx context.Context, root, commit string) (fs.FS, error) {
	entries, err := gitx.LsTreeEntries(ctx, root, commit)
	if err != nil {
		return nil, err
	}
	source := fstest.MapFS{}
	for _, entry := range entries {
		if entry.Type != "blob" {
			continue
		}
		var mode fs.FileMode
		switch entry.Mode {
		case "100644", "100755":
			mode = 0o444
		case "120000":
			mode = fs.ModeSymlink | 0o444
		default:
			return nil, fmt.Errorf("experiment tool: accepted tree entry %s has unsupported blob mode %s", entry.Path, entry.Mode)
		}
		data, readErr := gitx.Show(ctx, root, commit, entry.Path)
		if readErr != nil {
			return nil, readErr
		}
		source[entry.Path] = &fstest.MapFile{Data: data, Mode: mode}
	}
	return source, nil
}
