// Package strictweb holds the route shapes a workbench analysis must fail
// closed on: a route that reads a route table's function-typed field its
// registration did not bind, and a route that calls a function-typed field
// whose stored value no static evaluation can follow. A prefixed mount that
// does bind the row is the control.
package strictweb

import (
	"net/http"

	"example.com/synth/app"
)

type server struct {
	hook func() error
}

type row struct {
	suffix  string
	handler func(*server) http.HandlerFunc
}

func rows() []row {
	return []row{{suffix: "/x", handler: (*server).x}}
}

// Register wires the routes; hooks is whatever the caller hands in, so the
// server's hook is a value the analysis cannot follow.
func Register(mux *http.ServeMux, hooks []func() error) {
	s := &server{hook: hooks[0]}
	for _, r := range rows() {
		mux.HandleFunc("/p"+r.suffix, s.bound(r))
	}
	mux.HandleFunc("/stray", s.stray)
	mux.HandleFunc("/opaque", s.opaque)
}

func (s *server) x() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { _ = app.ReadOnly(r.Context()) }
}

// bound calls the row's handler through the row it was handed.
func (s *server) bound(r row) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) { r.handler(s)(w, req) }
}

// stray reads the route table's handler field outside any binding.
func (s *server) stray(w http.ResponseWriter, r *http.Request) { rows()[0].handler(s)(w, r) }

// opaque calls a hook whose value came from the caller of Register.
func (s *server) opaque(w http.ResponseWriter, r *http.Request) { _ = s.hook() }
