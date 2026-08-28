package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentapp"
	"github.com/jyang234/verdi/internal/experimentrun"
	"github.com/jyang234/verdi/internal/store"
)

// vocab:identity — CLI usage/operation-name grammar (identity)
const experimentUsage = `usage: verdi experiment <operation> [flags]

operations:
  inspect
  discover-capabilities
  validate-draft
  review-registration
  status
  explain-result
  draft-definition
  capture-candidate
  reconcile-draft
  propose-registration
  start
  resume
  propose-ratification
  publish-capsule
  release-workspaces`

const experimentInputLimit int64 = 16 << 20

var experimentOperationUsage = map[string]string{
	"inspect":               "usage: verdi experiment inspect --spike <spec/id> --experiment <id> --accepted-head <sha> [--json]",                                                                                                                  // vocab:identity — CLI usage/flag grammar (identity)
	"discover-capabilities": "usage: verdi experiment discover-capabilities --spike <spec/id> --experiment <id> --accepted-head <sha> [--json]",                                                                                                    // vocab:identity — CLI usage/flag grammar (identity)
	"validate-draft":        "usage: verdi experiment validate-draft --spike <spec/id> --experiment <id> --accepted-head <sha> [--json]",                                                                                                           // vocab:identity — CLI usage/flag grammar (identity)
	"review-registration":   "usage: verdi experiment review-registration --spike <spec/id> --experiment <id> --accepted-head <sha> [--json]",                                                                                                      // vocab:identity — CLI usage/flag grammar (identity)
	"status":                "usage: verdi experiment status --spike <spec/id> --experiment <id> --accepted-head <sha> [--json]",                                                                                                                   // vocab:identity — CLI usage/flag grammar (identity)
	"explain-result":        "usage: verdi experiment explain-result --spike <spec/id> --experiment <id> --accepted-head <sha> --run <id> [--json]",                                                                                                // vocab:identity — CLI usage/flag grammar (identity)
	"draft-definition":      "usage: verdi experiment draft-definition --spike <spec/id> --experiment <id> --accepted-head <sha> --definition <path|-> --candidate-root <path> [--json]",                                                           // vocab:identity — CLI usage/flag grammar (identity)
	"capture-candidate":     "usage: verdi experiment capture-candidate --spike <spec/id> --experiment <id> --accepted-head <sha> --candidate <id> --patch <path|-> --definition <path|-> [--json]",                                                // vocab:identity — CLI usage/flag grammar (identity)
	"reconcile-draft":       "usage: verdi experiment reconcile-draft --spike <spec/id> --experiment <id> --accepted-head <sha> [--human-proof <path>] [--json]",                                                                                   // vocab:identity — CLI usage/flag grammar (identity)
	"propose-registration":  "usage: verdi experiment propose-registration --spike <spec/id> --experiment <id> --accepted-head <sha> [--human-proof <path>] [--json]",                                                                              // vocab:identity — CLI usage/flag grammar (identity)
	"propose-ratification":  "usage: verdi experiment propose-ratification --spike <spec/id> --experiment <id> --accepted-head <sha> --result <digest> --disposition <value> [--candidate <id>] [--reason <text>] [--human-proof <path>] [--json]", // vocab:identity — CLI usage/flag grammar (identity)
	"publish-capsule":       "usage: verdi experiment publish-capsule --spike <spec/id> --experiment <id> --accepted-head <sha> [--json]",                                                                                                          // vocab:identity — CLI usage/flag grammar (identity)
	"release-workspaces":    "usage: verdi experiment release-workspaces --spike <spec/id> --experiment <id> --accepted-head <sha> [--json]",                                                                                                       // vocab:identity — CLI usage/flag grammar (identity)
	"start":                 "usage: verdi experiment start --spike <spec/id> --experiment <id> --accepted-head <sha> --run <id> --inputs <path|-> [--json]",                                                                                       // vocab:identity — CLI usage/flag grammar (identity)
	"resume":                "usage: verdi experiment resume --spike <spec/id> --experiment <id> --accepted-head <sha> --run <id> --inputs <path|-> [--json]",                                                                                      // vocab:identity — CLI usage/flag grammar (identity)
}

type experimentFlags struct {
	values map[string]string
	json   bool
}

func cmdExperiment(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, experimentUsage)
		return 2
	}
	operation := args[0]
	usage, ok := experimentOperationUsage[operation]
	if !ok {
		fmt.Fprintln(stderr, experimentUsage)
		return 2
	}
	flags, err := parseExperimentFlags(operation, args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "experiment %s: %v\n%s\n", operation, err, usage)
		return 2
	}
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintf(stderr, "experiment %s: %v\n", operation, err)
		return 2
	}
	actor, err := experimentapp.NewDelegatedAgent("verdi-cli", "")
	if err != nil {
		fmt.Fprintf(stderr, "experiment %s: %v\n", operation, err)
		return 2
	}
	identity := experimentapp.Identity{
		CheckoutRoot: root, Spike: flags.values["spike"],
		ExperimentID: flags.values["experiment"], ExpectedAcceptedHEAD: flags.values["accepted-head"], Actor: actor,
	}
	service, err := newExperimentService(root)
	if err != nil {
		fmt.Fprintf(stderr, "experiment %s: %v\n", operation, err)
		return 2
	}
	ctx := context.Background()

	switch operation {
	case "inspect":
		result := service.Inspect(ctx, identity)
		return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
	case "discover-capabilities":
		result := service.DiscoverCapabilities(ctx, identity)
		return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
	case "validate-draft":
		result := service.ValidateDraft(ctx, identity)
		return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
	case "review-registration":
		result := service.ReviewRegistration(ctx, identity)
		return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
	case "status":
		result := service.Status(ctx, identity)
		return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
	case "explain-result":
		result := service.Explain(ctx, identity, experimentapp.ExplainInput{Run: flags.values["run"]})
		return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
	case "draft-definition":
		return runExperimentDraft(ctx, service, identity, flags, stdin, stdout, stderr)
	case "capture-candidate":
		return runExperimentCapture(ctx, service, identity, flags, stdin, stdout, stderr)
	case "reconcile-draft", "propose-registration", "propose-ratification":
		return runExperimentHuman(ctx, operation, service, identity, flags, stdout, stderr)
	case "publish-capsule":
		result := service.PublishRatifiedCapsule(ctx, identity)
		return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
	case "release-workspaces":
		result := service.ReleaseRatified(ctx, identity, experimentapp.ReleaseAuthority{Releaser: execworkspace.NewReleaser(root)})
		return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
	case "start", "resume":
		return runExperimentExecution(ctx, operation, service, identity, flags, stdin, stdout, stderr)
	default:
		// vocab:identity — internal invariant panic text, never a display surface
		panic("closed experiment operation escaped parser")
	}
}

func parseExperimentFlags(operation string, args []string) (experimentFlags, error) {
	allowed := map[string]bool{"spike": true, "experiment": true, "accepted-head": true, "json": true}
	switch operation {
	case "explain-result":
		allowed["run"] = true
	case "draft-definition":
		allowed["definition"], allowed["candidate-root"] = true, true
	case "capture-candidate":
		allowed["candidate"], allowed["patch"], allowed["definition"] = true, true, true
	case "reconcile-draft", "propose-registration":
		allowed["human-proof"] = true
	case "propose-ratification":
		allowed["result"], allowed["disposition"] = true, true
		allowed["candidate"], allowed["reason"], allowed["human-proof"] = true, true, true
	case "start", "resume":
		allowed["run"], allowed["inputs"] = true, true
	}
	parsed := experimentFlags{values: map[string]string{}}
	seen := map[string]bool{}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") || arg == "--" {
			return experimentFlags{}, fmt.Errorf("unexpected positional argument %q", arg)
		}
		nameValue := strings.TrimPrefix(arg, "--")
		name, value, hasEquals := strings.Cut(nameValue, "=")
		if !allowed[name] {
			return experimentFlags{}, fmt.Errorf("unknown flag --%s", name)
		}
		if seen[name] {
			return experimentFlags{}, fmt.Errorf("duplicate flag --%s", name)
		}
		seen[name] = true
		if name == "json" {
			if hasEquals {
				return experimentFlags{}, fmt.Errorf("--json does not take a value")
			}
			parsed.json = true
			continue
		}
		if !hasEquals {
			index++
			if index >= len(args) || strings.HasPrefix(args[index], "--") {
				return experimentFlags{}, fmt.Errorf("--%s requires a value", name)
			}
			value = args[index]
		}
		if value == "" {
			return experimentFlags{}, fmt.Errorf("--%s requires a value", name)
		}
		parsed.values[name] = value
	}
	required := []string{"spike", "experiment", "accepted-head"}
	switch operation {
	case "explain-result":
		required = append(required, "run")
	case "draft-definition":
		required = append(required, "definition", "candidate-root")
	case "capture-candidate":
		required = append(required, "candidate", "patch", "definition")
	case "propose-ratification":
		required = append(required, "result", "disposition")
	case "start", "resume":
		required = append(required, "run", "inputs")
	}
	for _, name := range required {
		if parsed.values[name] == "" {
			return experimentFlags{}, fmt.Errorf("--%s is required", name)
		}
	}
	return parsed, nil
}

func runExperimentDraft(ctx context.Context, service *experimentapp.Service, identity experimentapp.Identity, flags experimentFlags, stdin io.Reader, stdout, stderr io.Writer) int {
	definitionBytes, err := readExperimentInput(flags.values["definition"], stdin, experimentInputLimit)
	if err != nil {
		return experimentOperational("draft-definition", err, stderr)
	}
	definition, err := experiment.DecodeDefinition(definitionBytes)
	if err != nil {
		return experimentOperational("draft-definition", err, stderr)
	}
	patches := make(map[string][]byte, len(definition.Candidates))
	for _, candidate := range definition.Candidates {
		patchPath := filepath.Join(flags.values["candidate-root"], filepath.FromSlash(candidate.Patch))
		patch, readErr := readExperimentInput(patchPath, stdin, experimentInputLimit)
		if readErr != nil {
			return experimentOperational("draft-definition", fmt.Errorf("candidate %s: %w", candidate.ID, readErr), stderr)
		}
		patches[candidate.ID] = patch
	}
	result := service.DraftDefinition(ctx, identity, experimentapp.DraftDefinitionInput{DefinitionBytes: definitionBytes, CandidatePatches: patches})
	return renderExperimentResult("draft-definition", result.Outcome, result, flags.json, stdout, stderr)
}

func runExperimentCapture(ctx context.Context, service *experimentapp.Service, identity experimentapp.Identity, flags experimentFlags, stdin io.Reader, stdout, stderr io.Writer) int {
	if flags.values["patch"] == "-" && flags.values["definition"] == "-" {
		return experimentOperational("capture-candidate", fmt.Errorf("--patch and --definition cannot both read stdin"), stderr)
	}
	patch, err := readExperimentInput(flags.values["patch"], stdin, experimentInputLimit)
	if err != nil {
		return experimentOperational("capture-candidate", err, stderr)
	}
	definition, err := readExperimentInput(flags.values["definition"], stdin, experimentInputLimit)
	if err != nil {
		return experimentOperational("capture-candidate", err, stderr)
	}
	result := service.CaptureCandidate(ctx, identity, experimentapp.CaptureCandidateInput{
		CandidateID: flags.values["candidate"], PatchBytes: patch, DefinitionBytes: definition,
	})
	return renderExperimentResult("capture-candidate", result.Outcome, result, flags.json, stdout, stderr)
}

func runExperimentExecution(ctx context.Context, operation string, service *experimentapp.Service, identity experimentapp.Identity, flags experimentFlags, stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := readExperimentInput(flags.values["inputs"], stdin, experimentInputLimit)
	if err != nil {
		return experimentOperational(operation, err, stderr)
	}
	bindings, err := experimentrun.DecodeInputBindings(data)
	if err != nil {
		return experimentOperational(operation, fmt.Errorf("decoding --inputs: %w", err), stderr)
	}
	input := experimentapp.ExecutionInput{Run: flags.values["run"], Bindings: bindings}
	if operation == "start" {
		result := service.Start(ctx, identity, input)
		return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
	}
	result := service.Resume(ctx, identity, input)
	return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
}

func readExperimentInput(name string, stdin io.Reader, limit int64) ([]byte, error) {
	var reader io.Reader
	var closer io.Closer
	if name == "-" {
		reader = stdin
	} else {
		file, err := os.Open(name)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		reader, closer = file, file
	}
	if closer != nil {
		defer closer.Close()
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("reading %s: input exceeds %d bytes", name, limit)
	}
	return data, nil
}

func renderExperimentResult(operation string, outcome experimentapp.Outcome, result any, jsonOutput bool, stdout, stderr io.Writer) int {
	data, err := canonjson.Marshal(result)
	if err != nil {
		return experimentOperational(operation, fmt.Errorf("encoding result: %w", err), stderr)
	}
	if jsonOutput {
		if _, err := stdout.Write(data); err != nil {
			return experimentOperational(operation, fmt.Errorf("writing result: %w", err), stderr)
		}
		return outcome.ExitCode()
	}
	if _, err := fmt.Fprintf(stdout, "experiment %s: %s (%s)\n%s\n", operation, outcome.Classification, outcome.Code, outcome.Detail); err != nil {
		return experimentOperational(operation, fmt.Errorf("writing result: %w", err), stderr)
	}
	return outcome.ExitCode()
}

func experimentOperational(operation string, err error, stderr io.Writer) int {
	message := strings.ReplaceAll(err.Error(), "\n", " ")
	fmt.Fprintf(stderr, "experiment %s: %s\n", operation, message)
	return 2
}
