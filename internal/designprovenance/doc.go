// Package designprovenance defines ASD's committed, append-only,
// non-authoritative design-provenance sidecar. Its records preserve exact
// mutation, policy, attribution, context-unavailability, and semantic-change
// identities without becoming acceptance evidence or normal build context.
package designprovenance

import "fmt"

const (
	// Schema is the sidecar entry schema's original (v1) version: strict
	// decode-only history (Wave 6 design §4.1). No writer may emit it —
	// every public writer surface refuses it (see refuseDecodeOnlyWrite),
	// while decoding, validation, and digest verification of already
	// committed v1 records stay byte-exact.
	Schema = "verdi.design-provenance/v1"
	// SchemaV2 is the sidecar entry schema every current writer emits
	// (SI-176, Wave 6 design §4.1/§6.1.1): it replaces the required
	// `policy_digest` scalar with one required, non-null `policy` union
	// (see Policy in record.go) so an explicit browser-human mutation can
	// honestly record genuine policy non-adoption instead of a fabricated
	// digest.
	SchemaV2 = "verdi.design-provenance/v2"
	// ExclusionReason is the fixed Context Integrity classifier reason.
	ExclusionReason = "design-provenance-sidecar"
	// ContextUnavailableReason is the honest v1 context identity until the
	// context compiler is delivered.
	ContextUnavailableReason = "unavailable-before-context-compiler"
	// MaxExcerptScalars is the per-excerpt Unicode scalar bound.
	MaxExcerptScalars = 600
	// MaxExcerptsPerTarget is the per-target excerpt count bound.
	MaxExcerptsPerTarget = 3
)

// refuseDecodeOnlyWrite enforces Schema's decode-only contract at every
// public writer surface: no fresh v1 bytes and no fresh v1 digest may be
// produced. v1's required `policy_digest` scalar has no not-applicable arm,
// so a v1 writer could only ever present some digest as a policy identity —
// it cannot honestly record genuine policy non-adoption. Historical v1
// records remain fully readable: decoding, validation, canonical re-encode
// comparison, and own-digest verification all run through version-neutral
// paths that deliberately do NOT call this refusal.
func refuseDecodeOnlyWrite(schema string) error {
	if schema != Schema {
		return nil
	}
	return fmt.Errorf("designprovenance: schema %q is strict decode-only history and must not be written; writers emit %q", Schema, SchemaV2)
}
