package disclosureview

import (
	"context"

	"github.com/jyang234/verdi/internal/disclosure"
)

// Count returns the number of current disclosures for the checkout at
// root (spec/index-coverage ac-3): the length of the enumeration the
// disclosures page shows for the same inputs and extras — the one
// enumeration, lint's disclosures plus the extras, never a separately
// decided tally. It is the index's read, served through the process-wide
// cache (Cached), which only the index reads and fills; the page itself
// computes fresh with Current and never touches the cache (SI-295).
func Count(ctx context.Context, root string, extras ...disclosure.Disclosure) (int, error) {
	items, err := Cached(ctx, root, extras...)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}
