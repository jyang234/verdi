// Package ritualwitness is the ritual effect harness (spec/ritual-effect-
// witness ac-1): it runs a ritual through its real entry point against a
// fixturegit repository with a local bare remote, in one of two seeded
// states, observes refs, HEAD, the index, the working tree, the linked-
// worktree list, and every commit the ritual creates, and reports each
// observed effect against internal/writescope's Declaration grammar as
// within the declaration, outside it, or unattributable.
//
// Like internal/fixturegit, this is a Go test helper, not a production
// package (PLAN.md §4) — its own tests are its first and, for this story
// (ac-1), only consumer; spec/ritual-effect-witness's later acceptance
// criteria (ac-2 onward) and the drivers Driver is shaped for beyond
// InProcess (dc-1: the built binary, workbench handlers, MCP) are future
// lanes' work.
//
// The pieces: Build (fixture.go) seeds a fixturegit repository; Capture
// (sensor.go) takes a Snapshot of everything the obligation's sensors
// cover; Driver and InProcess (driver.go) run a ritual and report its exit
// classification and git command log; Evaluate (evaluate.go) is the pure
// function comparing a before/after Snapshot pair and a command log
// against a writescope.Declaration; Run (harness.go) composes all four for
// one ritual-and-state pair.
package ritualwitness
