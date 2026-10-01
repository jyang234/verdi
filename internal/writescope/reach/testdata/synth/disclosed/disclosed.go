// Package disclosed registers one route per reachability class ledger
// SI-321 discloses (W1-W6, to be closed by BL-136's rebuild): each hands a
// function that publishes to a route through a shape the analysis neither
// follows nor fails closed on. W1ctl, the same dependency wrapper handed
// the function-typed value directly, is followed.
package disclosed

import (
	"net/http"

	"example.com/synth/app"
	"example.com/synth/subhost"
)

// id is a generic identity (W2).
func id[T any](v T) T { return v }

// emb embeds a dependency's function type (W3, W3c).
type emb struct{ http.HandlerFunc }

// serveFn is a module function type whose ServeHTTP calls its receiver.
type serveFn func(http.ResponseWriter, *http.Request)

// ServeHTTP calls f.
func (f serveFn) ServeHTTP(w http.ResponseWriter, r *http.Request) { f(w, r) }

// emb2 embeds a module function type (W3b).
type emb2 struct{ serveFn }

// box holds a value in a T-typed field (W4).
type box[T any] struct{ v T }

func (b *box[T]) get() T { return b.v }

type boxSrv struct{ b *box[http.Handler] }

func (s *boxSrv) serve(w http.ResponseWriter, r *http.Request) { s.b.get().ServeHTTP(w, r) }

// Register wires one route per shape.
func Register(mux *http.ServeMux) {
	fv := func(w http.ResponseWriter, r *http.Request) { _ = app.Publish(r.Context()) }

	// W1: a dependency wrapper (http.StripPrefix) holding an http.Handler.
	w1in := http.Handler(http.HandlerFunc(fv))
	w1 := http.StripPrefix("/w1", w1in)
	mux.HandleFunc("/w1", func(w http.ResponseWriter, r *http.Request) { w1.ServeHTTP(w, r) })

	// W1ctl: the same wrapper handed the function-typed value directly.
	w1c := http.StripPrefix("/w1ctl", http.HandlerFunc(fv))
	mux.HandleFunc("/w1ctl", func(w http.ResponseWriter, r *http.Request) { w1c.ServeHTTP(w, r) })

	// W2: a generic identity constrained by any.
	w2 := id(fv)
	mux.HandleFunc("/w2", func(w http.ResponseWriter, r *http.Request) { w2(w, r) })

	// W3: a struct embedding http.HandlerFunc, called by its promoted method.
	w3 := emb{fv}
	mux.HandleFunc("/w3", func(w http.ResponseWriter, r *http.Request) { w3.ServeHTTP(w, r) })

	// W3c: the same struct held as an http.Handler, captured by a route.
	w3c := http.Handler(emb{fv})
	mux.HandleFunc("/w3c", func(w http.ResponseWriter, r *http.Request) { w3c.ServeHTTP(w, r) })

	// W3b: a struct embedding a module function type.
	w3b := emb2{serveFn(fv)}
	mux.HandleFunc("/w3b", func(w http.ResponseWriter, r *http.Request) { w3b.ServeHTTP(w, r) })

	// W4: an http.Handler in a generic box's T-typed field, read by a route method.
	w4 := &boxSrv{b: &box[http.Handler]{v: http.HandlerFunc(fv)}}
	mux.HandleFunc("/w4", w4.serve)

	// W5: a method-value handler of a module function type.
	mux.HandleFunc("/w5", serveFn(fv).ServeHTTP)

	// W6: a sub-ServeMux built in another module package.
	w6 := subhost.NewSub(fv)
	mux.HandleFunc("/w6", func(w http.ResponseWriter, r *http.Request) { w6.ServeHTTP(w, r) })
}
