// Package web is a synthetic workbench: constant routes, a route table
// mounted twice (the prefix mount hands each row to a dispatcher that
// calls the row's handler through its function-typed field), a closure
// route dispatching on its {action} wildcard, an API handler whose actions
// dispatch in two switches, and a route that calls a collaborator through
// a function-typed field its registration stored.
package web

import (
	"context"
	"net/http"

	"example.com/synth/app"
)

const routeAPI = "/thing/{name}/api/{action}"

type server struct {
	// audit is stored by Register, outside every route's own code.
	audit func(context.Context) error
}

type route struct {
	suffix  string
	handler func(*server) http.HandlerFunc
}

func routes() []route {
	return []route{
		{suffix: "/thing/{name}", handler: (*server).page},
		{suffix: routeAPI, handler: (*server).api},
	}
}

// holder carries a function value one field away from the server's.
type holder struct {
	run func(context.Context) error
}

func newServer(audit func(context.Context) error) *server { return &server{audit: audit} }

// Register wires every route onto mux.
func Register(mux *http.ServeMux) {
	h := holder{run: app.Direct}
	s := newServer(h.run)
	mux.HandleFunc("/health", health())
	mux.HandleFunc("/static", s.static)
	mux.HandleFunc("/audit", s.audited)
	for _, rt := range routes() {
		mux.HandleFunc(rt.suffix, rt.handler(s))
	}
	for _, rt := range routes() {
		mux.HandleFunc("/b/{branch}"+rt.suffix, s.prefixed(rt))
	}
	mux.HandleFunc("/legacy/{key}/{action}", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("action") {
		case "commit":
			_ = app.Direct(r.Context())
		case "save":
			_ = app.ReadOnly(r.Context())
		}
	})
}

func health() http.HandlerFunc { return func(w http.ResponseWriter, r *http.Request) {} }

func (s *server) static(w http.ResponseWriter, r *http.Request) {}

// audited reaches gitx.Mutate only through s.audit, which Register filled.
func (s *server) audited(w http.ResponseWriter, r *http.Request) { _ = s.audit(r.Context()) }

func (s *server) page() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { _ = app.ReadOnly(r.Context()) }
}

func (s *server) api() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		action := r.PathValue("action")
		if s.ops(r, action) {
			return
		}
		switch action {
		case "push":
			_ = app.Publish(r.Context())
		case "look":
			_ = app.ReadOnly(r.Context())
		}
	}
}

func (s *server) ops(r *http.Request, action string) bool {
	switch action {
	case "mutate":
		_ = app.ViaMethodValue(r.Context())
	case "peek":
		_ = app.ReadOnly(r.Context())
	default:
		return false
	}
	return true
}

// prefixed wraps one row for the /b/{branch} mount: its own work (gitx.Mutate,
// standing in for the managed worktree it ensures), then the row's handler,
// called through the row's function-typed field.
func (s *server) prefixed(rt route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = app.ViaClosure(r.Context())
		rt.handler(s)(w, r)
	}
}
