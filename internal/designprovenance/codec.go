package designprovenance

import (
	"bytes"
	"fmt"

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
		if i == 0 {
			if entry.UnclassifiedGap != nil {
				return nil, fmt.Errorf("designprovenance: first JSONL entry cannot declare an unclassified gap")
			}
		} else {
			prior := entries[i-1]
			if entry.Spec != prior.Spec {
				return nil, fmt.Errorf("designprovenance: JSONL spec identity changed from %q to %q", prior.Spec, entry.Spec)
			}
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
	return artifact.DecodeExactJSON(data, out)
}
