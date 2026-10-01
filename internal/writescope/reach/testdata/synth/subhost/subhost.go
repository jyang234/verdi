// Package subhost builds a sub-mux in another package of the synthetic
// module (ledger SI-321's W6 shape): the handler it mounts is held by
// dependency code the analysis does not see.
package subhost

import "net/http"

// NewSub mounts h on a fresh mux.
func NewSub(h http.HandlerFunc) *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("/w6", h)
	return m
}
