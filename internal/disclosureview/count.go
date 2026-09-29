package disclosureview

import (
	"context"

	"github.com/jyang234/verdi/internal/disclosure"
)

// Count returns the number of current disclosures for the checkout at
// root — the SAME enumeration Current builds (spec/index-coverage ac-3:
// "the same enumeration the disclosures page shows"), never a second,
// separately decided tally. A caller that needs only the count (the
// index page's top-bar carrier) calls this instead of re-deriving one
// from HTML or re-implementing a parallel enumeration.
func Count(ctx context.Context, root string, extras ...disclosure.Disclosure) (int, error) {
	items, err := Current(ctx, root, extras...)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}
