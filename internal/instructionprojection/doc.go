// Package instructionprojection generates and verifies the harness
// instruction files (AGENTS.md, CLAUDE.md, and future adapter-declared
// equivalents) that a constitution store's adapters project (spec/
// context-integrity-v2 AC-1: "AGENTS.md, CLAUDE.md, and future harness
// files are generated projections of the constitution. Their content and
// digest derive from canonical policy inputs and adapter rules ... The
// adapter enumerates the harness's effective project-level instruction
// discovery chain, including nested instruction files, and requires
// every discovered project instruction to be generated and digest-
// matched. Unmanaged, shadowing, truncated, or drifted project
// instructions block authoritative launch.").
//
// A projection is NEVER authority. It is a deterministic rendering of
// the resolved effective policy internal/policyauthority already
// computed; nothing in this package reads, writes, or interprets a
// projected file as policy input, and DC-1's harness-neutral posture
// means no harness-specific behavior beyond the adapter's own declared
// paths and filenames leaks into that rendering. "Editing a projection
// does not change authority; drift is a named blocking witness until the
// projection is regenerated or the canonical policy is changed through
// governance" (AC-1) — Generate is the only writer of a managed file's
// content, and Verify is the only judge of whether the current bytes on
// disk still match what the current authority would produce; neither
// function ever treats an edited projection as a new instruction.
//
// Generate renders every adapter's managed files and one canonical
// manifest per adapter from a policyauthority.Load + Resolve pair,
// atomically writing both (internal/atomicfile). Manifests live at
// .verdi/policy/projections/<adapter-id>.json, a directory the store
// grammar admits as a GENERATED OUTPUT — policyauthority.Load recognizes
// it and deliberately never reads its entries as authority, because a
// projection derives from the constitution and can never be an input to
// it (DC-1). An unadopted store (policyauthority.ErrNotAdopted)
// generates nothing and claims nothing — adoption stays opt-in and
// reversible (DC-15). A constitution whose adapters declare the same
// managed path twice is unsatisfiable and is refused by name
// (ErrOverlappingManagedPath) before anything is written, rather than
// resolved last-writer-wins into a manifest the disk contradicts.
//
// Verify recomputes the same rendering from the CURRENT store state and
// classifies every managed file and manifest as clean, drifted (with a
// truncated subclass), missing, or manifest-drifted; enumerates the
// manifest directory so a manifest left behind by a removed or renamed
// adapter is an orphan-manifest finding rather than a record nothing
// ever checks again; and classifies the discovery walk's own findings as
// unmanaged, shadowing, or (when the walk itself could not fully
// enumerate the tree) incomplete-discovery. A discovered instruction
// file is satisfied when SOME adapter generates and digest-matches it —
// AC-1 requires each discovered instruction to be "generated and
// digest-matched", not managed by whichever adapter discovered it — so
// the ordinary layout where one harness also reads another adapter's
// file verifies clean. Every report additionally discloses the subtrees
// the walk never entered (Report.ExcludedSubtrees). A walk that cannot
// prove completeness, and a subtree that was never examined, are never
// silently treated as clean (CO-1: "silence is never a pass").
//
// Every byte this package writes is canonical: no wall-clock timestamp,
// username, local absolute path, or random identifier ever enters a
// generated file or manifest (CO-3) — only the resolved effective-
// policy digest, the governance profile's id and digest, and the
// resolved policy content itself.
package instructionprojection
