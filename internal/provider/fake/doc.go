// Package fake provides an in-memory StoryProvider (04 §Testing) for
// hermetic tests: configurable stories, a recorded publish history that
// enforces idempotency on the (story, commit) key, comment-only-on-change
// bookkeeping, and on-demand simulation of the provider package's failure
// taxonomy. It is exercised by the shared contract suite in
// internal/provider/providertest and is the reference adapter that suite
// runs against in this phase.
package fake
