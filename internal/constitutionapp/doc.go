// Package constitutionapp is the sole application consumer for the
// Constitution's five browser-neutral operations (Wave 6 Task 3;
// docs/superpowers/plans/2026-08-29-wave-6-workbench-presentation.md
// "Task 3: Complete the Constitution application core";
// docs/superpowers/specs/2026-08-29-wave-6-workbench-presentation-design.md
// §7.1, spec/context-integrity-v2 AC-1/AC-2/AC-3/AC-6):
//
//   - Inspect      — the effective constitution, its exact source layers,
//     and the accepted/proposed Git identity, without merging or flattening
//     provenance.
//   - Propose      — create or amend one Git-backed policy/overlay/exemption
//     proposal artifact on an explicit branch, guarded by an exact expected
//     branch/HEAD precondition.
//   - Validate     — strict-decode and cross-validate the proposed
//     constitution store, exactly as internal/policyauthority.Load/Resolve
//     already do, reporting a three-valued proof outcome rather than a bare
//     error.
//   - ImpactReview — diff the accepted and proposed effective policies
//     (added/removed/changed policies, exemptions, and dispositions) and run
//     mechanical/semantic conflict evaluation over caller-declared governed
//     targets through internal/policyconflict, never a second conflict
//     evaluator.
//   - SubmitPreparation — compose Validate and ImpactReview into one
//     submission-readiness packet. It writes nothing: merge and approval
//     remain outside this package, entirely the normal Git pull-request
//     boundary (design §7.1: "prepare submission without merging or
//     inventing approval").
//
// Service composes the existing schema/authority/conflict owners
// (internal/policyartifact, internal/policyauthority, internal/contextcompile,
// internal/policyconflict, internal/gitx, internal/specstate) behind
// consumer-owned ports (the 04 §port pattern: interfaces live here, at the
// consumer, never in the owner packages) rather than reimplementing any of
// their algorithms — mirroring internal/designapp's own Wave 6 Task 1
// architecture. CLI (cmd/verdi/context_constitution*.go) and MCP
// (internal/mcpserve's constitution_*.go tool files) adapters both route
// through these exact five methods.
//
// MCP exposes only Inspect, Validate, and ImpactReview (design §7.1: "MCP
// exposes only read, validation, and review projection and structurally
// refuses commit, submission, approval, exemption ownership, and semantic
// disposition"). Propose and SubmitPreparation have no MCP tool registration
// at all — there is no code path by which an MCP request could reach them,
// which is what "structurally refuses ... before store access" means here:
// absence, not a runtime guard.
//
// This package adds no rule grammar, precedence rule, approval record,
// applicability operator, or conflict semantics of its own (the Task 3 stop
// gate). Every applicability/effective-rule computation is
// internal/policyauthority.Resolve's own EffectivePolicy; every conflict
// witness is internal/policyconflict.Service.Evaluate's own Report.
package constitutionapp
