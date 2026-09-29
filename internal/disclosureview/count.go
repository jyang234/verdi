package disclosureview

import (
	"context"

	"github.com/jyang234/verdi/internal/disclosure"
)

// Count returns the number of current disclosures for the checkout at
// root: the length of the enumeration the disclosures page shows
// (spec/index-coverage ac-3), read through the same process-wide cache
// (Cached) the page reads, with the same extras — never a second,
// separately decided tally.
func Count(ctx context.Context, root string, extras ...disclosure.Disclosure) (int, error) {
	items, err := Cached(ctx, root, extras...)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}
