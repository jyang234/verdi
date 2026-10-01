// Package holes registers one route per function-value shape the analysis
// cannot follow, each of which must fail closed (ledger SI-318): a
// captured function value passed on as an argument rather than called, a
// channel receive, a type assertion to a function type, a dereference of
// a pointer loaded from a non-package container, and an element of a
// field-held container; and a type switch to a function type (the
// switch form of the assertion rule). Control routes dereference a pointer loaded from
// a package-level variable, use a field-held container only benignly
// (its length, its keys, a nil comparison), and compare a captured
// function to nil before calling it; the analysis follows each.
package holes

import (
	"net/http"
	"sync/atomic"

	"example.com/synth/app"
)

type server struct {
	ch       chan func() error
	anyHook  any
	handlers map[string]func() error
}

// seam is a package-level pointer to a function, filled only at
// initialization (here, never).
var seam atomic.Pointer[func() error]

// Register wires the routes; the server's containers are filled by its
// caller, outside every route's own code.
func Register(mux *http.ServeMux, s *server) {
	direct := func() error { return app.Direct(nil) }
	mux.HandleFunc("/hole/argument", func(w http.ResponseWriter, r *http.Request) { _ = invoke(direct) })
	mux.HandleFunc("/hole/receive", s.receive)
	mux.HandleFunc("/hole/assert", s.assert)
	mux.HandleFunc("/hole/deref", s.deref)
	mux.HandleFunc("/hole/element", s.element)
	mux.HandleFunc("/hole/switch", s.typeSwitch)
	mux.HandleFunc("/control/packagederef", s.packageDeref)
	mux.HandleFunc("/control/benign", s.benign)
	mux.HandleFunc("/control/capturednil", func(w http.ResponseWriter, r *http.Request) {
		if direct != nil {
			_ = direct()
		}
	})
}

func invoke(f func() error) error { return f() }

func (s *server) receive(w http.ResponseWriter, r *http.Request) {
	f := <-s.ch
	_ = f()
}

func (s *server) assert(w http.ResponseWriter, r *http.Request) {
	if f, ok := s.anyHook.(func() error); ok {
		_ = f()
	}
}

func (s *server) deref(w http.ResponseWriter, r *http.Request) {
	table := pointers()
	if p := table["k"]; p != nil {
		_ = (*p)()
	}
}

// pointers returns a map of pointers to functions: a container no
// package-level variable holds.
func pointers() map[string]*func() error { return map[string]*func() error{} }

func (s *server) element(w http.ResponseWriter, r *http.Request) {
	_ = s.handlers["k"]()
}

func (s *server) packageDeref(w http.ResponseWriter, r *http.Request) {
	if h := seam.Load(); h != nil {
		_ = (*h)()
	}
}

func (s *server) benign(w http.ResponseWriter, r *http.Request) {
	if s.handlers == nil || len(s.handlers) == 0 {
		return
	}
	for k := range s.handlers {
		_ = k
	}
}

func (s *server) typeSwitch(w http.ResponseWriter, r *http.Request) {
	switch f := s.anyHook.(type) {
	case func() error:
		_ = f()
	}
}
