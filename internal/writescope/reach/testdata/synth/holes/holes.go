// Package holes registers one route per function-value shape the analysis
// cannot follow, each of which must fail closed (ledger SI-318): a
// captured function value passed on as an argument rather than called, a
// channel receive, a type assertion to a function type, a dereference of
// a pointer loaded from a non-package container, and an element of a
// field-held container; a type switch to a function type (the switch
// form of the assertion rule); and, for an interface value holding a
// function (ledger SI-319), a channel of http.Handler and a field-held
// slice of http.Handler; and a function laundered through the empty
// interface or a type parameter (ledger SI-320): asserted from a captured
// any, asserted from a captured map[string]any, asserted by a helper a
// captured any is handed to, and a captured ~func type parameter passed
// on. RegisterPtrStore and RegisterPtrStoreHelper each fill a route's
// function-typed field through a pointer (ledger SI-321's probed fifth
// class), in the registration code (PS1) or in a helper it calls (PS2):
// the route's own reach holds no store, and the entry that runs the
// registration fails closed on the dereference. Control routes dereference a pointer loaded from
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
	hch      chan http.Handler
	hs       []http.Handler
}

// ptrStore holds a handler in a function-typed field that only a store
// through a pointer fills (PS1, PS2).
type ptrStore struct{ f http.HandlerFunc }

func (s *ptrStore) serve(w http.ResponseWriter, r *http.Request) { s.f(w, r) }

// setVia stores v through p (PS2).
func setVia(p *http.HandlerFunc, v http.HandlerFunc) { *p = v }

// directHandler returns a handler that mutates.
func directHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { _ = app.Direct(r.Context()) }
}

// RegisterPtrStore stores the route's handler through a pointer to its
// field in the registration code (PS1).
func RegisterPtrStore(mux *http.ServeMux) {
	ps1 := &ptrStore{}
	pf := &ps1.f
	*pf = directHandler()
	mux.HandleFunc("/hole/ptrstore", ps1.serve)
}

// RegisterPtrStoreHelper stores the route's handler through a pointer to
// its field in a helper the registration calls (PS2).
func RegisterPtrStoreHelper(mux *http.ServeMux) {
	ps2 := &ptrStore{}
	setVia(&ps2.f, directHandler())
	mux.HandleFunc("/hole/ptrstorehelper", ps2.serve)
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
	mux.HandleFunc("/hole/ifacechan", s.ifaceChan)
	mux.HandleFunc("/hole/ifaceslice", s.ifaceSlice)
	mux.HandleFunc("/control/packagederef", s.packageDeref)
	hv := func(w http.ResponseWriter, r *http.Request) { _ = app.Direct(r.Context()) }
	var anyH any = http.HandlerFunc(hv)
	mux.HandleFunc("/launder/assert", func(w http.ResponseWriter, r *http.Request) { anyH.(http.Handler).ServeHTTP(w, r) })
	m := map[string]any{"k": http.HandlerFunc(hv)}
	mux.HandleFunc("/launder/map", func(w http.ResponseWriter, r *http.Request) { m["k"].(http.HandlerFunc)(w, r) })
	mux.HandleFunc("/launder/helper", func(w http.ResponseWriter, r *http.Request) { assertAndServe(anyH, w, r) })
	registerTyped(mux, hv)
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

func (s *server) ifaceChan(w http.ResponseWriter, r *http.Request) {
	h := <-s.hch
	h.ServeHTTP(w, r)
}

func (s *server) ifaceSlice(w http.ResponseWriter, r *http.Request) {
	s.hs[0].ServeHTTP(w, r)
}

// assertAndServe asserts what it is handed to http.Handler and calls it.
func assertAndServe(v any, w http.ResponseWriter, r *http.Request) { v.(http.Handler).ServeHTTP(w, r) }

// registerTyped registers a route whose handler passes on a function held
// in a type parameter constrained to a function type.
func registerTyped[F ~func(http.ResponseWriter, *http.Request)](mux *http.ServeMux, f F) {
	mux.HandleFunc("/launder/typeparam", func(w http.ResponseWriter, r *http.Request) { callTyped(f, w, r) })
}

func callTyped[F ~func(http.ResponseWriter, *http.Request)](f F, w http.ResponseWriter, r *http.Request) {
	f(w, r)
}
