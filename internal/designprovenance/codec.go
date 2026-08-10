package designprovenance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// EncodeEntry returns one canonical JSONL record, including its newline.
func EncodeEntry(entry Entry) ([]byte, error) {
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	return canonjson.Marshal(entry)
}

// DecodeEntry strict-decodes and validates one sidecar entry.
func DecodeEntry(data []byte) (Entry, error) {
	var entry Entry
	if err := decodeStrictJSON(data, &entry); err != nil {
		return Entry{}, fmt.Errorf("designprovenance: decoding entry: %w", err)
	}
	if err := entry.Validate(); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// DecodeLog strict-decodes an entire JSONL sidecar and verifies digest
// uniqueness plus continuous or explicitly-gapped chaining.
func DecodeLog(data []byte) ([]Entry, error) {
	if len(data) == 0 {
		return []Entry{}, nil
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		return nil, fmt.Errorf("designprovenance: JSONL must end with a newline")
	}
	lines := bytes.Split(data[:len(data)-1], []byte("\n"))
	entries := make([]Entry, 0, len(lines))
	seen := map[string]bool{}
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			return nil, fmt.Errorf("designprovenance: blank JSONL line %d", i+1)
		}
		entry, err := DecodeEntry(line)
		if err != nil {
			return nil, fmt.Errorf("designprovenance: JSONL line %d: %w", i+1, err)
		}
		if seen[entry.Digest] {
			return nil, fmt.Errorf("designprovenance: duplicate entry digest %q", entry.Digest)
		}
		seen[entry.Digest] = true
		if i > 0 {
			prior := entries[i-1]
			if entry.PreviousDigest == prior.ResultDigest {
				if entry.UnclassifiedGap != nil {
					return nil, fmt.Errorf("designprovenance: unclassified gap is invalid when the typed chain is continuous")
				}
			} else if entry.UnclassifiedGap == nil {
				return nil, fmt.Errorf("designprovenance: unexplained chain break from %q to %q", prior.ResultDigest, entry.PreviousDigest)
			} else if entry.UnclassifiedGap.FromDigest != prior.ResultDigest || entry.UnclassifiedGap.ToDigest != entry.PreviousDigest {
				return nil, fmt.Errorf("designprovenance: unclassified gap does not explain the chain break")
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func decodeStrictJSON(data []byte, out any) error {
	if err := checkNoDuplicateJSONKeys(data); err != nil {
		return err
	}
	return artifact.DecodeStrictJSON(data, out)
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
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("designprovenance: scanning JSON: %w", err)
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{':
				stack = append(stack, &jsonFrame{object: true, seen: map[string]bool{}, expectKey: true})
			case '[':
				stack = append(stack, &jsonFrame{})
			case '}', ']':
				if len(stack) == 0 {
					return fmt.Errorf("designprovenance: unbalanced JSON")
				}
				stack = stack[:len(stack)-1]
				valueDone()
			}
			continue
		}
		if len(stack) > 0 && stack[len(stack)-1].object && stack[len(stack)-1].expectKey {
			key, ok := tok.(string)
			if !ok {
				return fmt.Errorf("designprovenance: JSON object key is not a string")
			}
			if stack[len(stack)-1].seen[key] {
				return fmt.Errorf("designprovenance: duplicate JSON key %q", key)
			}
			stack[len(stack)-1].seen[key] = true
			stack[len(stack)-1].expectKey = false
			continue
		}
		valueDone()
	}
}
