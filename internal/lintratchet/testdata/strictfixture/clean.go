package strictfixture

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// errEmptyURL is a sentinel error, which the ground rules allow at package
// level.
var errEmptyURL = errors.New("empty URL")

// Get performs a request under the caller's context and wraps its errors:
// it violates none of the gated rules.
func Get(ctx context.Context, url string) (*http.Response, error) {
	if url == "" {
		return nil, errEmptyURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	return http.DefaultClient.Do(req)
}
