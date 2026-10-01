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
	// hook is an interface-typed field Register fills with a function
	// value (ledger SI-319).
	hook http.Handler
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
	// A second address for the API handler that wraps it in a literal: no
	// action is derived for it, so the API's actions are not its own.
	mux.HandleFunc("/quick/thing/{name}/api/{action}", func(w http.ResponseWriter, r *http.Request) {
		s.api()(w, r)
	})
	// The API handler reached through a local the registration captured
	// (N1a), through a wrapper's argument (N1b), and through a wrapper
	// handed a literal that calls it (N1c).
	api := s.api()
	mux.HandleFunc("/captured/thing/{name}/api/{action}", func(w http.ResponseWriter, r *http.Request) { api(w, r) })
	mux.HandleFunc("/wrapped/thing/{name}/api/{action}", withTrace(s.api()))
	mux.HandleFunc("/wrappedlit/thing/{name}/api/{action}", withTrace(func(w http.ResponseWriter, r *http.Request) { s.api()(w, r) }))
	// The API handler held in an interface value (ledger SI-319): captured
	// through a conversion to http.Handler (H6), through a module function
	// type whose ServeHTTP calls it (H6b), and set in an interface-typed
	// field the route reads. A struct behind the same interface is the
	// control: class-hierarchy analysis dispatches its method.
	h6 := http.Handler(http.HandlerFunc(s.api()))
	mux.HandleFunc("/iface/thing/{name}/api/{action}", func(w http.ResponseWriter, r *http.Request) { h6.ServeHTTP(w, r) })
	h6b := http.Handler(serveFunc(s.api()))
	mux.HandleFunc("/ifacemod/thing/{name}/api/{action}", func(w http.ResponseWriter, r *http.Request) { h6b.ServeHTTP(w, r) })
	s.hook = s.api()
	mux.HandleFunc("/ifield/thing/{name}/api/{action}", s.ifield)
	sh := http.Handler(readOnlyHandler{})
	mux.HandleFunc("/ifacestruct", func(w http.ResponseWriter, r *http.Request) { sh.ServeHTTP(w, r) })
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

// serveFunc is a module function type whose ServeHTTP calls its receiver.
type serveFunc func(http.ResponseWriter, *http.Request)

// ServeHTTP calls f.
func (f serveFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) { f(w, r) }

// readOnlyHandler is a struct behind http.Handler.
type readOnlyHandler struct{}

// ServeHTTP only reads.
func (readOnlyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = app.ReadOnly(r.Context())
}

// ifield calls the handler Register stored in the server's interface field.
func (s *server) ifield(w http.ResponseWriter, r *http.Request) { s.hook.ServeHTTP(w, r) }

// withTrace wraps a handler: the wrapped handler is its argument, named by
// the registration, outside the route's own code.
func withTrace(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { next(w, r) }
}

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
