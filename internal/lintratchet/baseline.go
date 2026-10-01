package lintratchet

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// baselineFile is .golangci.strict-baseline.json's shape: every key the
// committed baseline allows, with how many times it allows it.
type baselineFile struct {
	Findings *[]baselineEntry `json:"findings"`
}

// baselineEntry is one allowed key. Every field must be present; pointers
// tell a missing field from a zero one.
type baselineEntry struct {
	Count   *int    `json:"count"`
	Linter  *string `json:"linter"`
	Package *string `json:"package"`
	Message *string `json:"message"`
	Source  *string `json:"source"`
}

// ParseBaseline decodes a baseline strictly. Unknown fields, trailing data, a
// missing field, an empty linter, package, or message, a count below one, and
// a key listed twice are malformed.
func ParseBaseline(data []byte) (Counts, error) {
	var file baselineFile
	if err := artifact.DecodeStrictJSON(data, &file); err != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	if file.Findings == nil {
		return nil, errors.New("baseline: no findings array")
	}
	counts := Counts{}
	for i, e := range *file.Findings {
		key, n, err := e.key()
		if err != nil {
			return nil, fmt.Errorf("baseline: entry %d: %w", i, err)
		}
		if _, dup := counts[key]; dup {
			return nil, fmt.Errorf("baseline: entry %d lists the key %+v twice", i, key)
		}
		counts[key] = n
	}
	return counts, nil
}

// key validates one entry and returns its key and count.
func (e baselineEntry) key() (Key, int, error) {
	switch {
	case e.Count == nil:
		return Key{}, 0, errors.New("no count")
	case *e.Count < 1:
		return Key{}, 0, fmt.Errorf("count %d is below one", *e.Count)
	case e.Linter == nil || *e.Linter == "":
		return Key{}, 0, errors.New("no linter")
	case e.Package == nil || *e.Package == "":
		return Key{}, 0, errors.New("no package")
	case e.Message == nil || *e.Message == "":
		return Key{}, 0, errors.New("no message")
	case e.Source == nil:
		return Key{}, 0, errors.New("no source")
	}
	return Key{Linter: *e.Linter, Package: *e.Package, Message: *e.Message, Source: *e.Source}, *e.Count, nil
}

// EncodeBaseline renders counts as the committed baseline's canonical JSON:
// entries sorted by (linter, package, message, source), object keys sorted,
// no HTML escaping, and a trailing newline.
func EncodeBaseline(counts Counts) ([]byte, error) {
	keys := sortedKeys(counts)
	entries := make([]baselineEntry, 0, len(keys))
	for _, k := range keys {
		n := counts[k]
		if n < 1 {
			return nil, fmt.Errorf("baseline: key %+v has count %d, below one", k, n)
		}
		entries = append(entries, baselineEntry{Count: &n, Linter: &k.Linter, Package: &k.Package, Message: &k.Message, Source: &k.Source})
	}
	data, err := canonjson.Marshal(baselineFile{Findings: &entries})
	if err != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	return data, nil
}

// sortedKeys returns counts' keys in (linter, package, message, source)
// order.
func sortedKeys(counts Counts) []Key {
	keys := make([]Key, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, compareKeys)
	return keys
}

// compareKeys orders keys by linter, package, message, then source.
func compareKeys(a, b Key) int {
	return cmp.Or(
		cmp.Compare(a.Linter, b.Linter),
		cmp.Compare(a.Package, b.Package),
		cmp.Compare(a.Message, b.Message),
		cmp.Compare(a.Source, b.Source),
	)
}
