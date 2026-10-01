package strictfixture

import "net/http"

// Fetch performs a request without a context: noctx's one finding.
func Fetch(url string) (*http.Response, error) {
	return http.Get(url)
}
