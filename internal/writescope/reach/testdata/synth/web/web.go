// Package web is a synthetic workbench: constant routes, a route table
// mounted twice, a closure route dispatching on its {action} wildcard, and
// an API handler whose actions dispatch in two switches.
package web

import (
	"net/http"

	"example.com/synth/app"
)

const routeAPI = "/thing/{name}/api/{action}"

type server struct{}

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

// Register wires every route onto mux.
func Register(mux *http.ServeMux) {
	s := &server{}
	mux.HandleFunc("/health", health())
	mux.HandleFunc("/static", s.static)
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
			_ = app.ViaPackageVar(r.Context())
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

func (s *server) prefixed(rt route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = app.ViaClosure(r.Context())
		rt.handler(s)(w, r)
	}
}
