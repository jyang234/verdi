// verdi context constitution <inspect|propose|validate|impact-review|
// submit-preparation> --request <path|-> [--out <path>] (Wave 6 Task 3;
// docs/superpowers/plans/2026-08-29-wave-6-workbench-presentation.md "Task
// 3: Complete the Constitution application core"; spec/context-integrity-v2
// AC-6): the complete human CLI surface over internal/constitutionapp's five
// operations, added to the existing `context` namespace exactly as `design`
// gained board/context/capabilities/provenance/review subcommands in Wave 6
// Task 1 — no new top-level verb, so internal/specalign's top-level CLI-verb
// inventory needs no change.
//
// This file's flag grammar and diagnostic-redaction plumbing reuse
// context.go's own extractContextCompileFlags/canonicalOutPath/
// validateContextOutputStoreZone/sameFileArg/hasDotDotElement helpers
// byte-for-byte (the exact --request/--out grammar `context compile` and
// `context conflict` already share) — never a second flag parser or a
// second checkout-path redaction rule.
//
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go/journey.go
// convention, so dispatch.go's diff for wiring this subcommand in stays a
// one-line change (cmdContext's own switch, context.go).
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/constitutionapp"
	"github.com/jyang234/verdi/internal/store"
)

// constitutionUsage is the exact invocation grammar every unrecognized/
// absent `context constitution` subcommand prints.
//
// vocab:identity — CLI usage/flag grammar (identity)
const constitutionUsage = "usage: verdi context constitution <inspect|propose|validate|impact-review|submit-preparation> --request <path|-> [--out <path>]"

// cmdContextConstitution dispatches `verdi context constitution <op>`.
// CLI exposes the complete human workflow (design §7.1: "CLI and workbench
// may expose the complete human workflow"); MCP registers only the three
// read/validate/review tools (internal/mcpserve/tool_constitution_*.go) —
// propose and submit-preparation have no MCP tool at all, so no MCP request
// can reach them.
func cmdContextConstitution(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, constitutionUsage)
		return 2
	}
	op := args[0]
	switch op {
	case "inspect", "propose", "validate", "impact-review", "submit-preparation":
	default:
		fmt.Fprintln(stderr, constitutionUsage)
		return 2
	}
	return runConstitutionOp(op, args[1:], stdin, stdout, stderr)
}

func runConstitutionOp(op string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	requestArg, hasRequest, outArg, hasOut, rest, err := extractContextCompileFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, "context constitution "+op+":", err)
		return 2
	}
	if len(rest) != 0 {
		fmt.Fprintln(stderr, "context constitution "+op+": unexpected positional argument(s):", rest)
		return 2
	}
	if !hasRequest || requestArg == "" {
		fmt.Fprintln(stderr, "context constitution "+op+": --request is required")
		return 2
	}
	if hasOut && outArg == "" {
		fmt.Fprintln(stderr, "context constitution "+op+": --out requires a value")
		return 2
	}
	if hasOut && hasDotDotElement(outArg) {
		fmt.Fprintln(stderr, "context constitution "+op+":", errContextOutDotDot)
		return 2
	}
	if hasOut && requestArg != "-" && sameFileArg(requestArg, outArg) {
		fmt.Fprintln(stderr, "context constitution "+op+": --request and --out must not name the same path")
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "context constitution "+op+":", err)
		return 2
	}

	var outCanon string
	if hasOut {
		outCanon, err = canonicalOutPath(root, outArg)
		if err != nil {
			printContextCommandDiagnostic(stderr, "constitution "+op, root, err)
			return 2
		}
		if err := validateContextOutputStoreZone(root, outCanon, requestArg); err != nil {
			printContextCommandDiagnostic(stderr, "constitution "+op, root, err)
			return 2
		}
	}

	data, err := readContextRequest(requestArg, stdin)
	if err != nil {
		printContextCommandDiagnostic(stderr, "constitution "+op, root, err)
		return 2
	}

	svc := constitutionapp.NewService()
	ctx := context.Background()

	resultBytes, landed, typed, decodeErr := dispatchConstitutionOp(ctx, svc, root, op, data)
	if decodeErr != nil {
		printContextCommandDiagnostic(stderr, "constitution "+op, root, decodeErr)
		return 2
	}
	if typed != nil {
		// Project the application error as-is: Failure carries the optional
		// repository-effects disclosure for any refusal after mutation began.
		// Reconstructing a fresh envelope here would hide that state.
		failureBytes, encErr := constitutionapp.EncodeResult(typed.Failure())
		if encErr != nil {
			printContextCommandDiagnostic(stderr, "constitution "+op, root, encErr)
			return 2
		}
		if _, err := stdout.Write(failureBytes); err != nil {
			printConstitutionFailureOutputFailure(stderr, op, root, failureBytes, fmt.Errorf("writing failure to stdout: %w", err))
			return 2
		}
		return typed.ExitCode()
	}

	if !hasOut {
		if _, err := stdout.Write(resultBytes); err != nil {
			printConstitutionOutputFailure(stderr, op, root, landed, fmt.Errorf("writing result to stdout: %w", err))
			return 2
		}
		return 0
	}
	if err := atomicfile.Write(outCanon, resultBytes, 0o644); err != nil {
		printConstitutionOutputFailure(stderr, op, root, landed, fmt.Errorf("writing result: %w", err))
		return 2
	}
	return 0
}

// landedEffect names the durable repository effect an operation had ALREADY
// COMMITTED before its result was rendered. Only `propose` mutates anything;
// every other operation on this surface reports nil.
type landedEffect struct {
	Schema     string `json:"schema"`
	Operation  string `json:"operation"`
	Branch     string `json:"branch"`
	Commit     string `json:"commit,omitempty"`
	ZeroEffect bool   `json:"zero_effect"`
}

const landedEffectSchema = "verdi.constitution-landed-effect/v1"

// printConstitutionFailureOutputFailure preserves an application Failure when
// its primary stdout delivery fails. canonicalFailure was encoded before the
// write attempt, so stderr carries those exact bytes beside the new transport
// diagnostic rather than reconstructing or reclassifying the application
// outcome. Exit selection remains the caller's operational 2.
func printConstitutionFailureOutputFailure(stderr io.Writer, op, root string, canonicalFailure []byte, err error) {
	printContextCommandDiagnostic(stderr, "constitution "+op, root, err)
	fmt.Fprint(stderr, string(canonicalFailure))
}

// printConstitutionOutputFailure renders an output-destination failure that
// happened AFTER the application call returned. When that call had already
// landed a durable effect (a real branch and commit), the failure must never
// be reported as a bare exit 2: the caller would be told the command failed
// while a proposal sits in its repository, discoverable only by inspecting
// Git by hand — silence standing in for a fact (root CLAUDE.md's three-valued
// honesty rule: "silence is never a pass"). The Wave 6 design states the same
// rule for the browser in §4.3's operational row ("no partial action effect
// may be hidden"), and internal/workbench's writeUnrefreshedOutcome is its
// implementation for the ASD mutation path; this is that shape for the CLI —
// the transport-level failure rides BESIDE the landed facts rather than
// replacing them.
//
// The exit code is unchanged (2: the result genuinely was not delivered);
// only the diagnostic gains the landed identity, as one canonical JSON
// document so a script can read it as reliably as a human. It carries branch,
// commit, and zero_effect only — never a filesystem path — so the existing
// checkout-root redaction rule needs no exception.
func printConstitutionOutputFailure(stderr io.Writer, op, root string, landed *landedEffect, err error) {
	printContextCommandDiagnostic(stderr, "constitution "+op, root, err)
	if landed == nil {
		return
	}
	disclosure, encErr := constitutionapp.EncodeResult(landed)
	if encErr != nil {
		// Canonical-encoding a struct of strings and one bool cannot fail;
		// disclose in plain text regardless rather than losing the facts.
		fmt.Fprintf(stderr, "context constitution %s: the %s landed on branch %s at commit %s (zero_effect=%t) before this failure\n",
			op, landed.Operation, landed.Branch, landed.Commit, landed.ZeroEffect)
		return
	}
	fmt.Fprint(stderr, string(disclosure))
}

// dispatchConstitutionOp strict-decodes data against op's own request shape,
// calls the matching constitutionapp.Service method, and encodes whichever
// of (result, error) came back. decodeErr is non-nil only for a malformed
// request document (operational, exit 2, before any application call);
// typed is constitutionapp's own typed application failure, encoded and
// exit-mapped by the caller via its own ExitCode/Failure methods — this
// function never re-derives that classification.
//
// landed is non-nil only for the one operation that mutates the repository
// (propose) and only when it returned a result, so the caller can disclose
// an already-committed proposal if rendering that result then fails. It is
// deliberately reported for a zero-effect propose too: "the branch is at this
// commit and nothing new was written" is itself the fact a caller needs when
// the result never reached it.
func dispatchConstitutionOp(ctx context.Context, svc constitutionapp.Service, root, op string, data []byte) (resultBytes []byte, landed *landedEffect, typed *constitutionapp.Error, decodeErr error) {
	switch op {
	case "inspect":
		req, err := constitutionapp.DecodeInspectRequest(data)
		if err != nil {
			return nil, nil, nil, err
		}
		result, typed := svc.Inspect(ctx, root, req)
		if typed != nil {
			return nil, nil, typed, nil
		}
		bytesOut, encErr := constitutionapp.EncodeResult(result)
		return bytesOut, nil, nil, encErr
	case "validate":
		req, err := constitutionapp.DecodeValidateRequest(data)
		if err != nil {
			return nil, nil, nil, err
		}
		result, typed := svc.Validate(ctx, root, req)
		if typed != nil {
			return nil, nil, typed, nil
		}
		bytesOut, encErr := constitutionapp.EncodeResult(result)
		return bytesOut, nil, nil, encErr
	case "propose":
		req, err := constitutionapp.DecodeProposeRequest(data)
		if err != nil {
			return nil, nil, nil, err
		}
		result, typed := svc.Propose(ctx, root, req)
		if typed != nil {
			return nil, nil, typed, nil
		}
		effect := &landedEffect{
			Schema:     landedEffectSchema,
			Operation:  "propose",
			Branch:     result.Identity.Branch,
			Commit:     result.Commit,
			ZeroEffect: result.ZeroEffect,
		}
		bytesOut, encErr := constitutionapp.EncodeResult(result)
		return bytesOut, effect, nil, encErr
	case "impact-review":
		req, err := constitutionapp.DecodeImpactReviewRequest(data)
		if err != nil {
			return nil, nil, nil, err
		}
		result, typed := svc.ImpactReview(ctx, root, req)
		if typed != nil {
			return nil, nil, typed, nil
		}
		bytesOut, encErr := constitutionapp.EncodeResult(result)
		return bytesOut, nil, nil, encErr
	case "submit-preparation":
		req, err := constitutionapp.DecodeSubmitPreparationRequest(data)
		if err != nil {
			return nil, nil, nil, err
		}
		result, typed := svc.SubmitPreparation(ctx, root, req)
		if typed != nil {
			return nil, nil, typed, nil
		}
		bytesOut, encErr := constitutionapp.EncodeResult(result)
		return bytesOut, nil, nil, encErr
	default:
		return nil, nil, nil, fmt.Errorf("context constitution: unreachable: unhandled operation %q", op)
	}
}
