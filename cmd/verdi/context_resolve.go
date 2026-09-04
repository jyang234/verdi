package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/contextresolve"
	"github.com/jyang234/verdi/internal/sealedexec"
	"github.com/jyang234/verdi/internal/store"
)

// contextResolveUsage is the exact closed grammar the VATC F12 controller-
// owner bridge correction (§2.2) fixes: one flag, one spelling, one operand,
// and that operand is always stdin.
//
// vocab:identity — CLI usage/flag grammar (identity)
const contextResolveUsage = "usage: verdi context resolve --request -"

// The closed diagnostic vocabulary of the context-item resolver.
//
// A refusal names one of these classes and nothing else. The document this
// verb refuses is an authority owner's view of a live flight — a manifest, an
// installed lineage, and the exact data items a provider has already been
// shown — so echoing a rejected value, the path it was resolved against, or
// the seam that rejected it would turn a refusal into a disclosure. The class
// is what a caller can act on; the payload is what it already has.
const (
	contextResolveInputUnreadable  = "input-unreadable"
	contextResolveInputTooLarge    = "input-too-large"
	contextResolveRequestRefused   = "request-refused"
	contextResolveStoreUnavailable = "store-unavailable"
	contextResolveUnresolvable     = "unresolvable"
	contextResolveOutputTooLarge   = "output-too-large"
	contextResolveOutputFailed     = "output-failed"
)

// cmdContextResolve implements `verdi context resolve --request -`.
//
// Unlike the owner bridge, this verb is not effect-FREE — it is effect-free in
// the only direction that matters. It legitimately reads the named store and
// its Git history through the compiler's existing trusted read-only ports,
// because a fresh data item is precisely what it exists to produce. What it
// never does is write: no file, no store record, no expansion, no profile
// activation, no credential read, no provider launch, no FD 3 traffic, no
// network, no clock or randomness.
//
// It has all three exits. A resolved item is 0. A ref that is absent,
// ambiguous, inapplicable, stale, or backed by an inconsistent lineage is the
// verdict 1 — the query looked, and the answer is that nothing is proven —
// and the canonical non-proven document is still delivered, because "no" is
// an answer the caller must be able to decode. Everything else is the
// operational 2 with an empty stdout: a caller that cannot get an answer must
// never be handed a document that looks like one.
func cmdContextResolve(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if !parseContextResolveArgs(args) {
		fmt.Fprintln(stderr, contextResolveUsage)
		return 2
	}

	input, class := readContextResolveDocument(stdin)
	if class != "" {
		return contextResolveRefusal(stderr, class)
	}
	request, err := contextresolve.DecodeRequest(input)
	if err != nil {
		return contextResolveRefusal(stderr, contextResolveRequestRefused)
	}

	// The store root is resolved only after the grammar and the document are
	// both accepted, so a mistyped invocation never touches a live checkout.
	root, err := store.FindRoot(".")
	if err != nil {
		return contextResolveRefusal(stderr, contextResolveStoreUnavailable)
	}

	result, err := contextresolve.NewService().Resolve(context.Background(), root, request)
	if err != nil {
		return contextResolveRefusal(stderr, contextResolveUnresolvable)
	}
	document, err := contextresolve.EncodeResult(result)
	if err != nil {
		return contextResolveRefusal(stderr, contextResolveUnresolvable)
	}

	// The output ceiling is checked before the single write, so a document at
	// the bound is refused whole rather than delivered in fragments.
	if len(document) >= sealedexec.OwnerFrameCeiling {
		return contextResolveRefusal(stderr, contextResolveOutputTooLarge)
	}
	// The write succeeded only if the destination accepted every byte. An
	// io.Writer that returns a short count with a nil error breaks its own
	// contract, but a destination behind a pipe, a filter, or a third-party
	// wrapper can still do it — and half of a canonical result is not a result
	// a caller could decode or tell apart from a whole one.
	written, err := stdout.Write(document)
	if err != nil || written != len(document) {
		return contextResolveRefusal(stderr, contextResolveOutputFailed)
	}
	if result.State == contextresolve.StateProven {
		return 0
	}
	return 1
}

// parseContextResolveArgs accepts exactly the two-token sequence
// `--request -` and nothing else.
//
// The `=`-joined spelling is refused deliberately. Every other flag in this
// binary honours both spellings, so a reader could reasonably expect this one
// to as well — which is precisely why §2.2 closes it: an external controller
// invokes this verb programmatically across a repository pin, and two
// spellings of one invocation is one more thing that pin has to agree about.
//
// There is no second flag, no positional operand, and no destination: §2.2
// gives this verb one input and one output, both streams. Rejecting rather
// than ignoring anything else is what keeps a mistyped invocation from
// answering a question the owner did not ask.
func parseContextResolveArgs(args []string) bool {
	return len(args) == 2 && args[0] == "--request" && args[1] == "-"
}

// readContextResolveDocument reads exactly one bounded document from stdin,
// returning a closed diagnostic class instead of bytes when it cannot.
//
// The limiter admits at most the ceiling, and a read that reached the ceiling
// is refused: the bound is exclusive, so a document at it is one operational
// failure rather than a truncated document nobody can tell apart from a whole
// one.
func readContextResolveDocument(stdin io.Reader) ([]byte, string) {
	data, err := io.ReadAll(io.LimitReader(stdin, int64(sealedexec.OwnerFrameCeiling)))
	if err != nil {
		return nil, contextResolveInputUnreadable
	}
	if len(data) >= sealedexec.OwnerFrameCeiling {
		return nil, contextResolveInputTooLarge
	}
	return data, ""
}

// contextResolveRefusal writes one closed diagnostic through the namespace's
// single framing seam and returns the operational exit code. stdout is
// untouched on every path that reaches here.
func contextResolveRefusal(stderr io.Writer, class string) int {
	printContextCommandDiagnostic(stderr, "resolve", "", errors.New(class))
	return 2
}
