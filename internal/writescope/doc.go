// Package writescope is the write-scope registry (spec/write-scope-registry
// ac-1, parent spec/ritual-write-scope-v3 ac-1): as data in source, the
// write scope of every verb that mutates a git repository, in a closed,
// typed grammar (Declaration, Validate), the classification of every
// exported internal/gitx function and every function outside gitx that
// writes under a repository's git directory (Classification), and the
// named, counted list of rituals still awaiting the fix that makes them
// conform (AwaitingFixes).
//
// Registry, Classification, and AwaitingFixes return fresh values from
// functions, never package variables (docs/ground-rules.md), the same shape
// as the CLI-verb and MCP-tool inventories (parent dc-4). Check compares
// them with what a static analysis of the module observed (Facts); the
// gate test TestRegistry_CoversEveryMutatingVerb computes those facts with
// internal/writescope/reach and fails on any finding.
package writescope
