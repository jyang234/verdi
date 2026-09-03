package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jyang234/verdi/internal/contextowner"
	"github.com/jyang234/verdi/internal/sealedexec"
)

// contextOwnerUsage is the exact closed grammar the VATC F12 controller-owner
// bridge correction (§2) fixes for both bridge verbs.
//
// vocab:identity — CLI usage/flag grammar (identity)
const contextOwnerUsage = "usage: verdi context owner <decode|encode> --operation OP"

// The closed diagnostic vocabulary of the owner bridge.
//
// A bridge failure names one of these classes and nothing else. The documents
// this verb refuses are controller payloads and owner replies that may carry
// any operand an ATC owner sent, so echoing a rejected value — or the seam
// that rejected it — would turn a refusal into a disclosure. The class is what
// a caller can act on; the payload is what it already has.
const (
	contextOwnerUnknownOperation = "unknown-operation"
	contextOwnerInputUnreadable  = "input-unreadable"
	contextOwnerInputTooLarge    = "input-too-large"
	contextOwnerRequestRefused   = "request-refused"
	contextOwnerReplyRefused     = "reply-refused"
	contextOwnerOutputTooLarge   = "output-too-large"
	contextOwnerOutputFailed     = "output-failed"
)

// cmdContextOwner implements `verdi context owner <decode|encode>`.
//
// The verb is effect-free in the same sense `contract` is, and for a stronger
// reason: it is invoked by a separate repository's controller for every one of
// the 22 Verdi-owned FD-3 operations, so anything it touched would be touched
// on that controller's behalf. It resolves no store root, reads no file,
// inspects no Git, opens no FD 3, reads no credential or ambient
// configuration, consults no clock or randomness, and starts no child process.
// Its whole answer is a function of the operand and the one document on stdin.
//
// It has no verdict to report. compile and conflict exit 1 when a closed state
// refusal is the answer to a question about a specification; translation asks
// no such question, so the only outcomes are the document (0) and a failure to
// produce it (2).
func cmdContextOwner(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	verb, operand, ok := parseContextOwnerArgs(args)
	if !ok {
		fmt.Fprintln(stderr, contextOwnerUsage)
		return 2
	}
	// The registry is closed and is consulted before stdin is read, so an
	// operation this build does not publish costs the caller nothing and
	// leaves its document unread.
	operation, ok := sealedexec.OwnerBridgeOperation(operand)
	if !ok {
		return contextOwnerRefusal(stderr, verb, contextOwnerUnknownOperation)
	}

	input, class := readContextOwnerDocument(stdin)
	if class != "" {
		return contextOwnerRefusal(stderr, verb, class)
	}

	var document []byte
	switch verb {
	case "decode":
		call, err := sealedexec.DecodeOwnerCall(operation, input)
		if err != nil {
			return contextOwnerRefusal(stderr, verb, contextOwnerRequestRefused)
		}
		if document, err = contextowner.EncodeCall(call); err != nil {
			return contextOwnerRefusal(stderr, verb, contextOwnerRequestRefused)
		}
	default:
		reply, err := contextowner.DecodeReply(bytes.NewReader(input))
		if err != nil {
			return contextOwnerRefusal(stderr, verb, contextOwnerReplyRefused)
		}
		if document, err = sealedexec.EncodeOwnerReply(operation, reply); err != nil {
			return contextOwnerRefusal(stderr, verb, contextOwnerReplyRefused)
		}
	}

	// The output ceiling is checked before the single write, so a document at
	// the bound is refused whole rather than delivered in fragments.
	if len(document) >= sealedexec.OwnerFrameCeiling {
		return contextOwnerRefusal(stderr, verb, contextOwnerOutputTooLarge)
	}
	// The write succeeded only if the destination accepted every byte. An
	// io.Writer that returns a short count with a nil error breaks its own
	// contract, but a destination behind a pipe, a filter, or a third-party
	// wrapper can still do it — and half of a canonical document is not a
	// document a caller could decode or tell apart from a whole one. Same
	// check sealedexec's writeControllerFrame already applies to a controller
	// frame, for the same reason.
	written, err := stdout.Write(document)
	if err != nil || written != len(document) {
		return contextOwnerRefusal(stderr, verb, contextOwnerOutputFailed)
	}
	return 0
}

// parseContextOwnerArgs accepts exactly one decode-or-encode verb followed by
// exactly one non-empty --operation value, in either spelling.
//
// Everything else — a missing, duplicated, empty, or unknown flag, a second
// verb, and any positional operand — is a shape this grammar does not honour.
// Rejecting rather than ignoring them is what keeps a mistyped invocation from
// answering a question the controller did not ask.
func parseContextOwnerArgs(args []string) (verb, operation string, ok bool) {
	if len(args) == 0 {
		return "", "", false
	}
	verb = args[0]
	if verb != "decode" && verb != "encode" {
		return "", "", false
	}
	rest := args[1:]
	seen := false
	for i := 0; i < len(rest); i++ {
		switch arg := rest[i]; {
		case arg == "--operation":
			if seen || i+1 >= len(rest) {
				return "", "", false
			}
			seen, operation = true, rest[i+1]
			i++
		case strings.HasPrefix(arg, "--operation="):
			if seen {
				return "", "", false
			}
			seen, operation = true, strings.TrimPrefix(arg, "--operation=")
		default:
			return "", "", false
		}
	}
	if !seen || operation == "" {
		return "", "", false
	}
	return verb, operation, true
}

// readContextOwnerDocument reads exactly one bounded document from stdin,
// returning a closed diagnostic class instead of bytes when it cannot.
//
// The limiter admits at most the ceiling, and a read that reached the ceiling
// is refused: the bound is exclusive, so a document at it is one operational
// failure rather than a truncated document nobody can tell apart from a whole
// one.
func readContextOwnerDocument(stdin io.Reader) ([]byte, string) {
	data, err := io.ReadAll(io.LimitReader(stdin, int64(sealedexec.OwnerFrameCeiling)))
	if err != nil {
		return nil, contextOwnerInputUnreadable
	}
	if len(data) >= sealedexec.OwnerFrameCeiling {
		return nil, contextOwnerInputTooLarge
	}
	return data, ""
}

// contextOwnerRefusal writes one closed diagnostic through the namespace's
// single framing seam and returns the operational exit code. stdout is
// untouched on every path that reaches here.
func contextOwnerRefusal(stderr io.Writer, verb, class string) int {
	printContextCommandDiagnostic(stderr, "owner "+verb, "", errors.New(class))
	return 2
}
