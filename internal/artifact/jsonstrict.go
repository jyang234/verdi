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
