// Package badweb registers a route whose pattern no static evaluation can
// resolve, so route discovery must fail closed on it.
package badweb

import "net/http"

func pattern() string { return "/dynamic" }

// Register wires one route with a computed pattern.
func Register(mux *http.ServeMux) {
	mux.HandleFunc(pattern(), func(w http.ResponseWriter, r *http.Request) {})
}
