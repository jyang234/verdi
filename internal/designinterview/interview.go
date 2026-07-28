// Package designinterview is `design start`'s flagless TTY interview
// (spec/cli-creation ac-2, ledger L-N7): given no --problem/--outcome/
// --defer-statements at all, on an attached terminal, design start
// collects the class template's own statement fields interactively —
// driven from the exact same placeholder-enumeration descriptors
// (internal/designscaffold.Fields) the board's creation form already
// reuses to validate its own submissions (spec/creation-form ac-1), one
// field contract, two front ends, never a second hand-rolled field list.
// Kept in its own package, separate from the verb plumbing in
// cmd/verdi/design.go, mirroring internal/initwizard's own split
// (interview logic vs. verb wiring, one package, one concern).
package designinterview

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jyang234/verdi/internal/designscaffold"
)

// ErrAborted is returned by RunInterview when stdin ends before every
// statement field has been answered — never silently treated as an empty
// answer (mirroring internal/initwizard.ErrAborted's identical contract
// for the init wizard's own interview).
var ErrAborted = errors.New("designinterview: interview aborted — stdin ended before every statement field was answered")

// RunInterview drives design start's flagless TTY interview against in
// (the operator's scripted or real terminal input) and out (where every
// prompt prints), returning the collected answers keyed by field name
// (e.g. "Problem", "Outcome" for the canonical templates) or ErrAborted.
// It enumerates tmpl's fields via designscaffold.Fields and asks ONLY the
// FieldStatement-kind ones, in that same enumeration order — never
// FieldInput (Title/Owners/StoryRef: design start already sources Title
// and StoryRef itself, and Owners deliberately stays out of CLI scope,
// spec/cli-creation ac-4/I-10/X-4), FieldIdentity, or FieldStructural.
// A template declaring zero statement fields (a degenerate custom
// override) returns an empty, non-nil map without reading any input at
// all — RunInterview never blocks on a field nothing asked for.
//
// Each answer is a SINGLE LINE, re-prompted (never silently accepted
// empty — statement fields are required content, spec/cli-creation ac-2)
// until non-blank. This is a disclosed v1 boundary: unlike the board
// form's free-form textarea, a CLI line-based interview has no established
// multi-line-input convention in this module to build on, and inventing
// one is out of this story's scope — multi-line statement authoring stays
// a design-branch hand-edit (spec.md itself) or the board form.
//
// It writes nothing to any filesystem itself — cmd/verdi/design.go owns
// what happens with the answers — which is what lets it be driven
// hermetically by a scripted stdin reader in tests, without any real
// terminal at all (mirroring internal/initwizard.RunInterview's own
// disclosed harness rationale).
func RunInterview(tmpl []byte, in io.Reader, out io.Writer) (map[string]string, error) {
	fields, err := designscaffold.Fields(tmpl)
	if err != nil {
		return nil, fmt.Errorf("designinterview: enumerating template fields: %w", err)
	}

	answers := map[string]string{}
	var statementFields []designscaffold.Field
	for _, f := range fields {
		if f.Kind == designscaffold.FieldStatement {
			statementFields = append(statementFields, f)
		}
	}
	if len(statementFields) == 0 {
		return answers, nil
	}

	fmt.Fprintln(out, "verdi design start — no creation flags given; collecting the statement fields interactively.")
	sc := bufio.NewScanner(in)
	for _, f := range statementFields {
		val, err := promptRequired(sc, out, f.Name)
		if err != nil {
			return nil, err
		}
		answers[f.Name] = val
	}
	return answers, nil
}

// promptRequired asks for field's value, re-prompting on a blank answer
// (never silently accepted) until one arrives non-empty or stdin ends
// (ErrAborted).
func promptRequired(sc *bufio.Scanner, out io.Writer, field string) (string, error) {
	for {
		fmt.Fprintf(out, "%s: ", field)
		if !sc.Scan() {
			return "", ErrAborted
		}
		val := strings.TrimSpace(sc.Text())
		if val == "" {
			fmt.Fprintf(out, "  %s is required, cannot be empty — try again\n", field)
			continue
		}
		return val, nil
	}
}
