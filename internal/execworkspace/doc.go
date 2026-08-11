// Package execworkspace implements spec/execution-workspace (ledger SI-10,
// SI-15): the deterministic <workspace-id> identity and state grammar for
// the two materialization request shapes, and the operations over it —
// materialization, release, the gc slice, and isolation-profile
// construction.
//
// Its foundation is still the identity layer (spec §Workspace naming): the
// naming scheme, the immutable request-identity sidecar's canonical-JSON
// codec, the sibling path grammar under a store's data/execution/ root, and
// the lstat-based path typing every mutator of that grammar depends on.
// What the unit ships on top of it:
//
//   - Identity and grammar: the two request shapes, the <workspace-id>
//     scheme over internal/store's normative RefSlug, ValidWorkspaceID as
//     the package's ONE shape test, and ClassifyEntry over the sibling
//     namespace under data/execution/ (SI-10, SI-15).
//   - Sidecar codec: the immutable request-identity sidecar's canonical-JSON
//     encode/decode and the identity-mismatch verdict, plus the lstat-based
//     path typing every mutator of the grammar depends on.
//   - Materialization: Materializer.Materialize creates and reuses the unit
//     worktree for both request shapes, writing the sidecar as the
//     completion witness. Git work is NOT done inline — it goes through the
//     Reconciler port (reconcile.go's GitReconciler is the real
//     implementation), so the decision logic stays testable against a
//     hermetic fake.
//   - Locking: every mutating operation takes the unit's data/execution/
//     <workspace-id>.lock through internal/filelock, non-blocking and held
//     only for that operation.
//   - Release: Releaser.Release records the durable .released marker that
//     authorizes a later reclaim — an operational fact, never a proof or a
//     ratification.
//   - GC slice: GC decides each unit against the spec's total ordered
//     rank 0-5 set, disclosing and keeping whatever it cannot classify.
//   - Isolation: BuildProfile constructs the isolation profile and its
//     per-grant enforcement report, and Profile.Command is the package's one
//     launch-construction seam (AD-5, AD-9, SI-40) — it enforces the granted
//     argv0 allowlist and deadline while still never STARTING the consumer's
//     process, which remains the consumer's own act. It also builds the
//     default-deny network posture (SI-75, SI-76): an absent network grant
//     is a mandatory deny — on Linux, Profile.Command attaches a new
//     user+network namespace; on every other platform it is an operational
//     error, with no weaker fallback — and a present, payload-free grant is
//     an explicit ambient-allow bit. EnforcementReport's always-present
//     Network fact makes that posture, and any could-not-apply grant
//     alongside it, one disclosed, three-valued-honest report.
//   - Fingerprint: CollectFingerprint records the environment actually
//     constructed, for the consuming feature's own environment comparison.
//
// (spec/execution-workspace §Implementation seam names this unit
// "execution-workspace enforcement", one shared internal package beside
// internal/wtmanager, internal/gitx, internal/filelock; the Go package NAME
// "execworkspace" is this unit's own decision, controller decision AD-1,
// which SI-15 delegates to this unit rather than fixing itself.)
package execworkspace
