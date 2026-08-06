// Package humanartifact is the shared human-artifact extension contract
// and scaffold-resolution seam spec/context-integrity-v2 AC-1 requires:
// "The operating model resolves a configurable scaffold for every
// committed human-authored artifact kind: policies, overlays,
// exemptions, feature, story, and component specs, ADRs, obligations,
// attestations, waivers, reaffirmations, and future model-registered
// human kinds. CLI, workbench, and agent-assisted creation use one
// resolver and renderer. The model may declare typed extension fields
// and presentation guidance, but a template cannot remove, rename,
// retype, or synthesize kernel fields. A created artifact records the
// resolved template identity and digest. `verdi model check` renders
// and strict-decodes every configured template and proves parity across
// creation surfaces."
//
// DC-4 draws the boundary this package enforces structurally: "Human
// artifacts optimize for authors and reviewers... Configuration may
// change human scaffold structure and add declared typed extensions. It
// cannot make proof formats ambiguous or force checkers to interpret
// project-specific layouts." CX-16/R-10 (docs/superpowers/plans/
// 2026-08-03-cross-feature-authority-audit.md) assign ownership: "CI
// policy-authority owns the immutable identity, authority, scope,
// lifecycle, ownership, and provenance kernel plus the shared renderer …
// ASD must consume this seam for agent-assisted creation and may add
// typed draft operations, but it may not create a competing template or
// policy model" — "CI's plan defines the shared extension contract, ASD's
// plan maps model descriptors onto it (R-10)".
//
// This package therefore does three things, each load-bearing for that
// later consumer:
//
//  1. Kernel (kernel.go): names, per artifact kind, the immutable
//     frontmatter field set a template extension must never shadow,
//     rename, retype, or synthesize.
//  2. Contract (contract.go, register.go): a closed, validated extension
//     grammar over a kind's kernel — structurally rejecting any
//     extension that collides with a kernel field name, case-fold
//     included — plus the init-time registry every artifact kind's
//     (currently empty) contract lives in.
//  3. Scaffold resolution and policy-family rendering (scaffold.go,
//     policy.go): ONE resolver (ResolveScaffold) every creation surface
//     shares, and the policy/overlay/exemption render-then-verify path
//     that proves the render/strict-decode kernel round trip AC-1's
//     "verdi model check ... proves parity" language requires.
//
// It wraps — never competes with — internal/designscaffold: every render
// in this package runs through designscaffold.RenderValue, the same
// parse options and error wrapping internal/designscaffold's own
// ScaffoldData-typed Render already established (no second, independently
// maintained template execution path). internal/policyartifact remains
// the frozen L1 decode/validate authority for the three constitution
// artifact kinds this package renders; this package adds resolution,
// the extension contract, and the render/decode kernel-round-trip proof
// on top, never a competing decoder.
package humanartifact
