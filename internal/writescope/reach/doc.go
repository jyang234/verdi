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
// in its reach therefore calls none through any of those shapes.
//
// Soundness boundaries, disclosed rather than silently assumed:
//   - reflection and unsafe are not followed; init functions and variable
//     initializers are followed as the pre-dispatch entry, never as part
//     of a verb;
//   - a dependency that type-asserts a module value received as an empty
//     interface (any) to another interface and calls it is not followed;
//   - function values (ledger SI-318): a function value is an edge from the
//     code that names it, which is in a verb's reach whenever the
//     activation that produced the value is part of the verb's execution
//     (an uncaptured parameter's value was named by a caller, an
//     uncaptured local's by its own function, a call's result by its
//     callee, a package-level variable's by its initializer). Values that
//     outlive their activation are handled explicitly: a read of a
//     function-typed struct field resolves to every value the module
//     stores in it (fields.go); a call through a function value captured
//     from an enclosing function resolves through the flow; every
//     function-typed argument of a route registration's wrapper is a root
//     of the route; and every value the analysis cannot follow fails
//     closed, naming its site, in every entry whose reach holds it: a
//     captured function value used other than by calling it (passed on as
//     an argument, assigned, returned, stored), a call through a captured
//     value the flow cannot follow, a channel receive yielding a function
//     value, a type assertion to a function type, a dereference of a
//     pointer to a function value not loaded from a package-level
//     variable, a read of a field-held container of function values, and,
//     for an entry a host dispatches, a function-typed field holding a
//     value the flow cannot follow or a route table's field its
//     registration did not bind (valuecalls.go). A package-level variable
//     reassigned outside its initializer (package-level mutable state the
//     ground rules forbid) is still attributed to the code that assigns
//     it;
//   - generic types are not candidates for interface dispatch; Build
//     refuses a module where a generic type's method set (methods promoted
//     from embedded fields included) has the methods of a module
//     interface (the tripwire), so the gap cannot pass silently. A generic
//     type converted to a dependency's interface is followed
//     (internal/governanceprincipal's byContent[T], a sort.Interface, is);
//   - each Program is one build target; the witness loads every target in
//     Targets so platform-specific files are all analyzed.
package reach
