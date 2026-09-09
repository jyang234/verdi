package artifact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// DecodeExactJSON adds duplicate-key rejection to DecodeStrictJSON's unknown
// field and trailing-data checks. It is the seam for signed and
// content-addressed JSON contracts where accepting two spellings of one
// object would make the decoded meaning parser-dependent.
func DecodeExactJSON(data []byte, out any) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("artifact: exact JSON must be valid UTF-8")
	}
	if err := checkNoDuplicateJSONKeys(data); err != nil {
		return err
	}
	return DecodeStrictJSON(data, out)
}

// DecodeClosedJSON adds explicit-null rejection to DecodeExactJSON's
// duplicate-key, unknown-field, and trailing-data checks — the complete
// closed grammar for a versioned request document, where every accepted
// spelling has to mean exactly one thing.
//
// Why nulls are refused rather than tolerated: `{"targets": null}` and
// `{"targets": []}` and an omitted key all decode to the same Go zero value,
// so a decoder that accepts all three cannot tell "I declare no targets"
// from "I meant to send targets and something upstream produced null." For a
// document whose whole job is to be an exact, versioned contract, that is a
// silent reinterpretation of the caller's meaning. Omit the key, or send an
// empty array; both say what they mean.
//
// It is a SEPARATE seam rather than a change to DecodeExactJSON because
// DecodeExactJSON's existing consumers are signed and content-addressed
// artifacts whose accepted grammar is already fixed; tightening it in place
// would change what those established contracts accept.
func DecodeClosedJSON(data []byte, out any) error {
	if err := checkNoJSONNulls(data); err != nil {
		return err
	}
	return DecodeExactJSON(data, out)
}

// checkNoJSONNulls token-walks data rejecting an explicit null literal at
// any depth. json.Decoder.Token reports a null literal — and only a null
// literal — as a nil token, so the scan needs no separate lexer.
func checkNoJSONNulls(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	for {
		token, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("artifact: scanning exact-grammar JSON: %w", err)
		}
		if token == nil {
			return fmt.Errorf("artifact: explicit null is not permitted in an exact-grammar JSON document")
		}
	}
}

type jsonFrame struct {
	object    bool
	seen      map[string]bool
	expectKey bool
}

func checkNoDuplicateJSONKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var stack []*jsonFrame
	valueDone := func() {
		if len(stack) > 0 && stack[len(stack)-1].object {
			stack[len(stack)-1].expectKey = true
		}
	}
	for {
		token, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("artifact: scanning exact JSON: %w", err)
		}
		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '{':
				stack = append(stack, &jsonFrame{object: true, seen: map[string]bool{}, expectKey: true})
			case '[':
				stack = append(stack, &jsonFrame{})
			case '}', ']':
				if len(stack) == 0 {
					return fmt.Errorf("artifact: unbalanced exact JSON")
				}
				stack = stack[:len(stack)-1]
				valueDone()
			}
			continue
		}
		if len(stack) > 0 && stack[len(stack)-1].object && stack[len(stack)-1].expectKey {
			key, ok := token.(string)
			if !ok {
				return fmt.Errorf("artifact: JSON object key is not a string")
			}
			if stack[len(stack)-1].seen[key] {
				return fmt.Errorf("artifact: duplicate JSON key %q", key)
			}
			stack[len(stack)-1].seen[key] = true
			stack[len(stack)-1].expectKey = false
			continue
		}
		valueDone()
	}
}
