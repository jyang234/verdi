// Package reach is the static analysis behind the write-scope witness
// (spec/write-scope-registry dc-2): it loads the module's source with the
// standard library's go/parser and go/types (Load), derives the verbs the
// CLI dispatcher, the MCP tool switch, and the workbench's route
// registrations define (CLIEntries, SwitchEntries, RouteEntries), and the
// pre-dispatch code every verb runs outside its arm (PreDispatchEntry),
// builds a call graph with interface calls resolved to every
// implementation (Build), reports which target functions each verb reaches
// (Reach), and finds the functions outside internal/gitx that write under
// a repository's git directory (GitDirWriters).
//
// A verb's reach stops only where the mutation belongs to another entry
// (ledger SI-314 (2)): at its own descendants (a dispatcher's subcommands,
// a route's actions) and, for a host verb the caller names (ledger SI-317
// (1): serve, mcp, context mcp), at the entries of another surface whose
// site (route registration, MCP tool switch) its code reaches. Every other
// call, into another verb's code or a hosted entry included, is traversed.
//
// The graph over-approximates: every use of a function, method value, or
// function literal is an edge from the code that writes it, every
// interface method resolves to every module type that implements the
// interface, and a module type converted to a dependency's interface is an
// edge to the methods that interface names. A verb that shows no target
// in its reach therefore calls none through any of those shapes, except
// through the classes ledger SI-321 discloses (below).
//
// Soundness boundaries, disclosed rather than silently assumed:
//   - reflection and unsafe are not followed; init functions and variable
//     initializers are followed as the pre-dispatch entry, never as part
//     of a verb;
//   - a dependency that type-asserts a module value received as an empty
//     interface (any) to another interface and calls it is not followed;
//   - function values (ledger SI-318, SI-319, SI-320): what follows applies to
//     a function value and to a value that carries one: an interface a named
//     function type with methods implements (a method of that type, such as
//     http.HandlerFunc's ServeHTTP, calls the function with no assertion), a
//     type parameter whose constraint has a function type among its terms, and
//     a pointer, slice, array, map, or channel of these. The empty interface
//     does not count (SI-320), on the premise that a value of that type
//     reaches a call in module code only through a type assertion or type
//     switch, which fails closed whenever its target carries a function;
//     generic instantiation refutes that premise (a type parameter constrained
//     by any carries a function with no assertion), a class disclosed below
//     (SI-321). A function value is an edge from the code that names it, which
//     is in a verb's reach whenever the activation that produced the value is
//     part of the verb's execution (an uncaptured parameter's value was named
//     by a caller, an uncaptured local's by its own function, a call's result
//     by its callee, a package-level variable's by its initializer). Values
//     that outlive their activation are handled explicitly: a read of a named
//     function-typed or function-carrying interface field resolves to every
//     value the module stores in it (fields.go; an embedded field does not, a
//     class disclosed below); a call through a captured function value, or a
//     method called on a captured value that carries one, resolves through the
//     flow, conversions transparent; every function-typed or function-carrying
//     interface argument of a route registration's wrapper is a root of the
//     route; and every value the analysis cannot follow, except in the
//     classes ledger SI-321 discloses (below), fails closed, naming its
//     site, in every entry whose reach holds it: a captured function value
//     used other than by calling it (passed on as an argument, assigned,
//     returned, stored), a call through a captured value the flow cannot
//     follow, a channel receive yielding a function value, a type assertion or
//     type switch to a type that carries one, a dereference of a pointer to a
//     function value not loaded from a package-level variable, a read of a
//     field-held container of function values, and, for an entry a host
//     dispatches, a function-typed field holding a value the flow cannot
//     follow or a route table's field its registration did not bind
//     (valuecalls.go). A package-level variable reassigned outside its
//     initializer (package-level mutable state the ground rules forbid) is
//     still attributed to the code that assigns it;
//   - reachability classes ledger SI-321 discloses: in each, a function value
//     reaches a route by a path the analysis neither follows nor fails closed
//     on, and the witness passed while the built binary committed and pushed.
//     TestSynth/DisclosedBoundaries pins one witnessed shape of each until
//     BL-136 rebuilds reachability on SSA with a VTA call graph. (1) A value
//     held by dependency code, since the flow follows only function-typed
//     dependency arguments: http.StripPrefix over an http.Handler, or a
//     sub-ServeMux built in another module package and served by a route. (2)
//     A value passed through a type parameter constrained by any: a generic
//     identity func id[T any](v T) T, or a generic box's T-typed field. (3) A
//     struct embedding a function type, whose promoted method is dropped:
//     struct{ http.HandlerFunc } called as ServeHTTP, or a struct embedding a
//     module function type. (4) A method value of a module function type used
//     as a handler, whose bound receiver is never rooted: mux.HandleFunc(p,
//     serveFn(fv).ServeHTTP). A fifth suspected class, a store through a
//     pointer to a function-typed field (p := &s.f; *p = fv), is not a
//     recorded field store, so a route that reads the field reaches nothing
//     through it; both probes, the store in the registration code (PS1) and
//     in a helper it calls (PS2), fail closed through the dereference rule in
//     the entry that runs the store, and TestSynth pins both. Hence
//     spec/write-scope-registry ac-1 (a verb that reaches a mutating gitx
//     function with no declaration fails the witness) is proven for the
//     module's current code and the pinned evasion corpus, and
//     disclosed-as-unproven beyond them until BL-136 lands, so its producer
//     still fails on a violation but abstains, never passes, while the
//     classes stay open (ledger SI-367 (1));
//   - generic types are not candidates for interface dispatch; Build
//     refuses a module where a generic type's method set (methods promoted
//     from embedded fields included) has the methods of a module
//     interface (the tripwire), so the gap cannot pass silently. A generic
//     type converted to a dependency's interface is followed
//     (internal/governanceprincipal's byContent[T], a sort.Interface, is);
//   - each Program is one build target; the witness loads every target in
//     Targets so platform-specific files are all analyzed.
package reach
