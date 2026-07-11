// Package provider defines the story-provider port (docs/design/specs/04
// §The port): the consumer-defined interface verdi uses to resolve story
// metadata from an external tracker and publish AC rollups back to it,
// one-way. The port itself is deliberately tiny and tracker-agnostic; a
// tracker becomes reachable by writing a small adapter package that
// implements StoryProvider and registering it under its ref scheme (04
// §Reference scheme).
//
// This package also hosts scheme parsing, a runtime scheme registry, a
// resolve-caching decorator (04 §Semantics), and the shared error
// taxonomy adapters use to report the failure table in 04. The Jira
// adapter itself is out of scope here (PLAN.md phase 11); this phase
// ships the port, the registry, the cache, the fake, and the contract
// suite (internal/provider/providertest) that every adapter — including
// the future Jira one — must pass.
package provider
