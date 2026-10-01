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
// a route's actions) and at the entries of another surface it serves (a
// verb whose code reaches the workbench's route registrations or the MCP
// tool switch serves those entries). Every other call, into another verb's
// code included, is traversed.
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
//   - a function value stored by code outside an entry's reach and called
//     inside it is attributed to the code that stored it, except through a
//     function-typed struct field: a read of one resolves to every value
//     the module stores in it (fields.go), and an entry a host dispatches
//     (a workbench route or action, an MCP tool) fails closed on a field
//     whose stored value cannot be followed, or on a route table's field
//     its registration did not bind; values in maps, slices, and channels,
//     and a variable assigned through a pointer, are still attributed to
//     the code that stored them;
//   - generic types are not candidates for interface dispatch (the module
//     declares no generic type with methods at the time of writing);
//   - each Program is one build target; the witness loads every target in
//     Targets so platform-specific files are all analyzed.
package reach
