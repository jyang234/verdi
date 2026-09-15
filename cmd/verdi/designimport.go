// verdi design import source|preview|apply|record (spec-import-contract.md,
// Task 4 CLI): a thin adapter over the frozen internal/specimport service
// surface (NewService, ReadSource, DecodeRequest, ReadRecord). This file
// composes and reads strict JSON; it never parses a source, evaluates
// policy, publishes Git, or invents a finding — every one of those stays
// owned by internal/specimport and the seams it already reuses
// (internal/draftmutation's actor-policy dispatcher, internal/store's
// trusted path constructors). Mirrors designmutate.go's own bounded
// request read and diagnostic/exit conventions and designappadapter.go's
// canonical-JSON-on-success convention, without calling either: import's
// error vocabulary, request shape and CLI grammar are this contract's own,
// not draftmutation's or designapp's.
//
// Grammar (all four subcommands, spec-import-contract.md "Public
// operations and closed shapes"):
//
//	verdi design import source --root <directory> --file <relative-path> [--start-line <n> --end-line <n>]
//	verdi design import preview --request <path|->
//	verdi design import apply --request <path|-> --preview <sha256> --harness <id> [--session <id>]
//	verdi design import record --branch <branch> --spec <slug>
//
// No CLI human actor or AI is ever constructed here: apply always mints a
// delegated-agent actor from --harness/--session
// (draftmutation.NewDelegatedAgent); VERDI_ACTOR_KIND/VERDI_PRINCIPAL_ID/
// VERDI_HUMAN are never read, and --human is simply an unknown flag.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/specimport"
	"github.com/jyang234/verdi/internal/store"
)

// cmdDesignImport dispatches the four import subcommands; a missing or
// unrecognized one prints the whole design usage line, matching every
// other unrecognized `design` subcommand's convention (runDesignVerb's own
// default case) rather than a narrower "design import: usage" message.
func cmdDesignImport(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, designVerbUsage)
		return 2
	}
	switch args[0] {
	case "source":
		return cmdDesignImportSource(args[1:], stdout, stderr)
	case "preview":
		return cmdDesignImportPreview(args[1:], stdout, stderr)
	case "apply":
		return cmdDesignImportApply(args[1:], stdout, stderr)
	case "record":
		return cmdDesignImportRecord(args[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, designVerbUsage)
		return 2
	}
}

// cmdDesignImportSource reads exactly one selected file through
// specimport.ReadSource — never listing a directory or executing content —
// and prints the resulting Source as canonical JSON. It needs no store
// root: --root is any directory the operator selects.
func cmdDesignImportSource(args []string, stdout, stderr io.Writer) int {
	values, err := parseDesignImportFlags(args, "--root", "--file", "--start-line", "--end-line")
	if err != nil {
		return renderDesignImportUsageError(stderr, "source", err)
	}
	root, ok := values["--root"]
	if !ok {
		return renderDesignImportUsageError(stderr, "source", errors.New("--root <directory> is required"))
	}
	file, ok := values["--file"]
	if !ok {
		return renderDesignImportUsageError(stderr, "source", errors.New("--file <relative-path> is required"))
	}
	startRaw, hasStart := values["--start-line"]
	endRaw, hasEnd := values["--end-line"]
	if hasStart != hasEnd {
		return renderDesignImportUsageError(stderr, "source", errors.New("--start-line and --end-line must both be given or both omitted"))
	}
	var startLine, endLine int
	if hasStart {
		startLine, err = parseDesignImportPositiveLine("--start-line", startRaw)
		if err != nil {
			return renderDesignImportUsageError(stderr, "source", err)
		}
		endLine, err = parseDesignImportPositiveLine("--end-line", endRaw)
		if err != nil {
			return renderDesignImportUsageError(stderr, "source", err)
		}
	}

	source, err := specimport.ReadSource(context.Background(), root, file, startLine, endLine)
	if err != nil {
		return renderDesignImportServiceError(stderr, "source", err)
	}
	return writeDesignImportResult(stdout, stderr, "source", source)
}

// parseDesignImportPositiveLine parses raw as a positive decimal integer,
// the CLI-level shape check spec-import-contract.md's own line-range rule
// needs before ReadSource ever sees a value (non-integer or non-positive
// values refuse here, at the usage layer).
func parseDesignImportPositiveLine(flag, raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", flag, raw)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %d", flag, n)
	}
	return n, nil
}

// cmdDesignImportPreview reads a bounded request envelope, strict-decodes
// it, resolves the store root, and calls the one shared Preview
// algorithm — read-only end to end. A blocking preview is still printed
// as structured JSON on stdout (exit 1); stderr stays empty.
func cmdDesignImportPreview(args []string, stdout, stderr io.Writer) int {
	values, err := parseDesignImportFlags(args, "--request")
	if err != nil {
		return renderDesignImportUsageError(stderr, "preview", err)
	}
	requestPath, ok := values["--request"]
	if !ok {
		return renderDesignImportUsageError(stderr, "preview", errors.New("--request <path|-> is required"))
	}

	raw, err := readDesignImportRequest(requestPath, os.Stdin)
	if err != nil {
		return renderDesignImportUsageError(stderr, "preview", err)
	}
	req, err := specimport.DecodeRequest(raw)
	if err != nil {
		return renderDesignImportServiceError(stderr, "preview", err)
	}
	root, err := store.FindRoot(".")
	if err != nil {
		return renderDesignImportIOFailure(stderr, "preview", err)
	}

	result, err := specimport.NewService().Preview(context.Background(), root, req)
	if err != nil {
		return renderDesignImportServiceError(stderr, "preview", err)
	}
	if code := writeDesignImportResult(stdout, stderr, "preview", result); code != 0 {
		return code
	}
	if !result.Ready {
		return 1
	}
	return 0
}

// designImportPreviewDigestRe is the CLI-level shape check for --preview:
// exactly 64 lowercase hex characters (spec-import-contract.md's
// SHA-256-lowercase-hex digest form).
var designImportPreviewDigestRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// cmdDesignImportApply mints the one allowed delegated-agent actor from
// --harness/--session, reads and decodes the same bounded request preview
// uses, resolves the store root, and calls the one shared Apply algorithm.
// Both "created" and "already-created" print their Result as canonical
// JSON at exit 0, including statements_deferred and disclosures verbatim.
func cmdDesignImportApply(args []string, stdout, stderr io.Writer) int {
	values, err := parseDesignImportFlags(args, "--request", "--preview", "--harness", "--session")
	if err != nil {
		return renderDesignImportUsageError(stderr, "apply", err)
	}
	requestPath, ok := values["--request"]
	if !ok {
		return renderDesignImportUsageError(stderr, "apply", errors.New("--request <path|-> is required"))
	}
	previewDigest, ok := values["--preview"]
	if !ok {
		return renderDesignImportUsageError(stderr, "apply", errors.New("--preview <sha256> is required"))
	}
	if !designImportPreviewDigestRe.MatchString(previewDigest) {
		return renderDesignImportUsageError(stderr, "apply", fmt.Errorf("--preview %q must be 64 lowercase hex characters", previewDigest))
	}
	harness, ok := values["--harness"]
	if !ok {
		return renderDesignImportUsageError(stderr, "apply", errors.New("--harness <id> is required"))
	}
	session := values["--session"]

	// The one allowed CLI actor: a delegated agent from --harness/--session
	// alone. No principal claim, no human bypass, no environment-derived
	// identity — VERDI_ACTOR_KIND/VERDI_PRINCIPAL_ID/VERDI_HUMAN are never
	// read here or anywhere else in this file.
	actor, err := draftmutation.NewDelegatedAgent(harness, session)
	if err != nil {
		return renderDesignImportUsageError(stderr, "apply", err)
	}

	raw, err := readDesignImportRequest(requestPath, os.Stdin)
	if err != nil {
		return renderDesignImportUsageError(stderr, "apply", err)
	}
	req, err := specimport.DecodeRequest(raw)
	if err != nil {
		return renderDesignImportServiceError(stderr, "apply", err)
	}
	root, err := store.FindRoot(".")
	if err != nil {
		return renderDesignImportIOFailure(stderr, "apply", err)
	}

	result, err := specimport.NewService().Apply(context.Background(), root, req, previewDigest, actor)
	if err != nil {
		return renderDesignImportServiceError(stderr, "apply", err)
	}
	return writeDesignImportResult(stdout, stderr, "apply", result)
}

// cmdDesignImportRecord validates --spec with the same bare spec-name
// validator Request.Validate uses (artifact.ParseRef("spec/"+slug)) and
// requires a nonblank --branch that does not begin with "-", then reads
// committed provenance through the one shared ReadRecord algorithm. A
// truthful current-spec-changed disclosure is still exit 0: it is a
// successful read, not a refusal.
func cmdDesignImportRecord(args []string, stdout, stderr io.Writer) int {
	values, err := parseDesignImportFlags(args, "--branch", "--spec")
	if err != nil {
		return renderDesignImportUsageError(stderr, "record", err)
	}
	branch, ok := values["--branch"]
	if !ok {
		return renderDesignImportUsageError(stderr, "record", errors.New("--branch <branch> is required"))
	}
	if strings.HasPrefix(branch, "-") {
		return renderDesignImportUsageError(stderr, "record", fmt.Errorf("--branch %q must not begin with -", branch))
	}
	slug, ok := values["--spec"]
	if !ok {
		return renderDesignImportUsageError(stderr, "record", errors.New("--spec <slug> is required"))
	}
	ref, err := artifact.ParseRef("spec/" + slug)
	if err != nil {
		return renderDesignImportUsageError(stderr, "record", fmt.Errorf("--spec %q is not a valid spec name: %v", slug, err))
	}
	if ref.Pinned() || ref.Fragment() {
		return renderDesignImportUsageError(stderr, "record", fmt.Errorf("--spec %q must be a bare spec name, not a pinned (@commit) or fragment (#object-id) ref", slug))
	}

	root, err := store.FindRoot(".")
	if err != nil {
		return renderDesignImportIOFailure(stderr, "record", err)
	}
	view, err := specimport.ReadRecord(context.Background(), root, branch, slug)
	if err != nil {
		return renderDesignImportServiceError(stderr, "record", err)
	}
	return writeDesignImportResult(stdout, stderr, "record", view)
}

// parseDesignImportFlags parses a "--name value" / "--name=value" flag set
// against the closed allowed-name list names, mirroring designmutate.go's
// own parseDesignMutateFlags shape (strings.Cut on "=", then a following
// token) without calling it — this contract's own flags and required-ness
// rules are distinct per subcommand. Every accepted value must be
// nonblank. An unrecognized flag (including a bare positional argument,
// which never matches any "--name") or a repeated one is refused; the
// returned map holds only, and exactly, the flags actually given.
func parseDesignImportFlags(args []string, names ...string) (map[string]string, error) {
	allowed := make(map[string]bool, len(names))
	for _, n := range names {
		allowed[n] = true
	}
	values := make(map[string]string, len(names))
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(args[i], "=")
		if !allowed[name] {
			return nil, fmt.Errorf("unknown argument %q", args[i])
		}
		if _, seen := values[name]; seen {
			return nil, fmt.Errorf("%s given more than once", name)
		}
		if !hasValue {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("%s requires a value", name)
			}
			i++
			value = args[i]
		}
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("%s requires a nonblank value", name)
		}
		values[name] = value
	}
	return values, nil
}

// readDesignImportRequest reads request bytes from path (or stdin when
// path is "-"), bounded by specimport.MaxEnvelopeBytes+1 — this package's
// OWN bounded reader. specimport's 12 MiB JSON envelope limit is distinct
// from designmutate's own 1 MiB draft-mutation request limit
// (draftmutation.MaxRequestBytes), so this never reuses
// readDesignMutationRequest's reader or limit.
func readDesignImportRequest(path string, stdin io.Reader) (_ []byte, resultErr error) {
	var reader io.Reader
	var file *os.File
	if path == "-" {
		if stdin == nil {
			return nil, errors.New("reading request from stdin: stdin is unavailable")
		}
		reader = stdin
	} else {
		var err error
		file, err = os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening request %q: %w", path, err)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil && resultErr == nil {
				resultErr = fmt.Errorf("closing request %q: %w", path, closeErr)
			}
		}()
		reader = file
	}
	raw, err := io.ReadAll(io.LimitReader(reader, specimport.MaxEnvelopeBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading request: %w", err)
	}
	if len(raw) > specimport.MaxEnvelopeBytes {
		return nil, fmt.Errorf("request exceeds %d bytes", specimport.MaxEnvelopeBytes)
	}
	return raw, nil
}

// writeDesignImportResult prints result as this store's canonical JSON
// (internal/canonjson: sorted keys, deterministic, newline-terminated) on
// stdout and returns 0, or reports an encode/write failure as io-failure
// exit 2 — mirroring designappadapter.go's renderDesignAppResult tail.
func writeDesignImportResult(stdout, stderr io.Writer, op string, result any) int {
	encoded, err := canonjson.Marshal(result)
	if err != nil {
		fmt.Fprintf(stderr, "design import %s: io-failure: encoding result: %v\n", op, err)
		return 2
	}
	if _, err := stdout.Write(encoded); err != nil {
		fmt.Fprintf(stderr, "design import %s: io-failure: writing result: %v\n", op, err)
		return 2
	}
	return 0
}

// renderDesignImportUsageError renders a CLI-local refusal (flag parsing,
// actor construction, malformed --preview shape) that never reached
// specimport at all: these are always the operational invalid-request
// code at exit 2.
func renderDesignImportUsageError(stderr io.Writer, op string, err error) int {
	fmt.Fprintf(stderr, "design import %s: invalid-request: %s\n", op, err)
	return 2
}

// renderDesignImportIOFailure renders a store.FindRoot failure: a plain
// error, never a specimport sentinel, but the contract still calls it out
// explicitly as the operational io-failure code (main's
// task4-cli-preflight.md preflight and spec-import-contract.md's own
// "resolve the store root" step).
func renderDesignImportIOFailure(stderr io.Writer, op string, err error) int {
	fmt.Fprintf(stderr, "design import %s: io-failure: %s\n", op, err)
	return 2
}

// designImportSentinel pairs one specimport sentinel error with its
// contract wire code and CLI exit classification.
type designImportSentinel struct {
	err  error
	code string
	exit int
}

// designImportSentinels is the closed error-code table spec-import-
// contract.md's "Errors and browser behavior" section declares for
// ReadSource/DecodeRequest/Preview/Apply/ReadRecord — operational codes at
// exit 2, completed refusals at exit 1. ErrImportRecordMissing is main's
// task4-cli-preflight.md adapter mapping: an internal proof-unavailability
// sentinel, never a new public wire error code — it reports as the
// existing completed provenance-mismatch refusal, exactly like
// ErrProvenanceMismatch itself; import-record-missing remains only a
// Finding/disclosure code (specimport.FindingImportRecordMissing), never a
// second wire error here. No new codes are introduced beyond this table.
var designImportSentinels = []designImportSentinel{
	{specimport.ErrInvalidRequest, "invalid-request", 2},
	{specimport.ErrInvalidSource, "invalid-source", 2},
	{specimport.ErrUnsupportedFormat, "unsupported-format", 2},
	{specimport.ErrInvalidModel, "invalid-model", 2},
	{specimport.ErrIdentityUnavailable, "identity-unavailable", 2},
	{specimport.ErrAuthorityInvalid, "authority-invalid", 2},
	{specimport.ErrIOFailure, "io-failure", 2},
	{specimport.ErrUnresolved, "unresolved", 1},
	{specimport.ErrDirtyContext, "dirty-context", 1},
	{specimport.ErrStalePreview, "stale-preview", 1},
	{specimport.ErrTargetExists, "target-exists", 1},
	{specimport.ErrPolicyForbidden, "policy-forbidden", 1},
	{specimport.ErrActorForbidden, "actor-forbidden", 1},
	{specimport.ErrImportRecordMissing, "provenance-mismatch", 1},
	{specimport.ErrProvenanceMismatch, "provenance-mismatch", 1},
}

// renderDesignImportServiceError maps err — always an error one of this
// file's specimport calls returned — to its contract wire code and CLI
// exit classification via errors.Is against designImportSentinels,
// preserving the sentinel's own appended detail text (the shared refusal
// reasons carry the correction guidance, e.g. the policy mode that forbids
// delegated-agent writes) rather than the generic sentinel message itself.
// An error matching no sentinel is a defensive io-failure exit 2: every
// exported specimport function this file calls is documented to wrap one
// of these, so this arm is not expected to fire in practice.
func renderDesignImportServiceError(stderr io.Writer, op string, err error) int {
	for _, s := range designImportSentinels {
		if errors.Is(err, s.err) {
			detail := strings.TrimPrefix(err.Error(), s.err.Error()+": ")
			fmt.Fprintf(stderr, "design import %s: %s: %s\n", op, s.code, detail)
			return s.exit
		}
	}
	fmt.Fprintf(stderr, "design import %s: io-failure: %s\n", op, err)
	return 2
}
