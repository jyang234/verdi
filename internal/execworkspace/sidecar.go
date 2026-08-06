package execworkspace

import (
	"bytes"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// sidecarSchema is the request-identity sidecar's schema tag (AD-2, spec
// §Workspace naming).
const sidecarSchema = "verdi.execution-request/v1"

// sidecarDoc is the on-disk JSON shape of data/execution/<workspace-id>.request
// (spec §Workspace naming): canonical JSON — sorted keys, trailing newline,
// via internal/canonjson — recording the consumer run identity, the full
// 40-hex commit SHA, and, for the base-plus-patch shape only, the full
// 64-hex patch sha256.
//
// PatchSHA256 is a *string, not string, so decode can distinguish the key
// being ABSENT (nil — the exact-SHA shape) from the key being PRESENT but
// empty (a non-nil pointer to "" — always a decode error, never silently
// read as absent or as a valid patch shape).
type sidecarDoc struct {
	Schema      string  `json:"schema"`
	RunID       string  `json:"run_id"`
	CommitSHA   string  `json:"commit_sha"`
	PatchSHA256 *string `json:"patch_sha256,omitempty"`
}

// EncodeSidecar renders id as the immutable request-identity sidecar's
// canonical JSON bytes (spec §Workspace naming). The exact-SHA shape omits
// patch_sha256 entirely; the base-plus-patch shape carries its full 64-hex
// digest.
//
// It fails closed on any identity that does not satisfy Identity.Validate:
// a malformed identity is never serialized, so a sidecar on disk — the
// materialization COMPLETION WITNESS — never records a request this package
// would refuse to hand back.
func EncodeSidecar(id Identity) ([]byte, error) {
	if err := id.Validate(); err != nil {
		return nil, fmt.Errorf("execworkspace: encoding request sidecar: %w", err)
	}
	return encodeSidecarDoc(sidecarDocFor(id))
}

// sidecarDocFor projects id onto its on-disk document shape. It is the ONE
// place the projection lives, so encode and the decode-side canonical-bytes
// gate can never drift apart.
func sidecarDocFor(id Identity) sidecarDoc {
	doc := sidecarDoc{
		Schema:    sidecarSchema,
		RunID:     id.RunID,
		CommitSHA: id.CommitSHA,
	}
	if id.Shape == BasePlusPatch {
		patch := id.PatchSHA256
		doc.PatchSHA256 = &patch
	}
	return doc
}

func encodeSidecarDoc(doc sidecarDoc) ([]byte, error) {
	data, err := canonjson.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("execworkspace: encoding request sidecar: %w", err)
	}
	return data, nil
}

// DecodeSidecar strict-decodes a request-identity sidecar's bytes into an
// Identity (spec §Workspace naming). It fails closed on: any unknown field
// or trailing data (via internal/artifact.DecodeStrictJSON); ANY departure
// from the canonical bytes this package would itself write for the decoded
// document (the canonical-bytes gate below); a schema value other than
// sidecarSchema; a missing or non-canonical commit_sha; a patch_sha256 that
// is present but not a canonical 64-hex digest — including
// present-but-empty, never conflated with the key being absent (the
// exact-SHA shape) — and any decoded identity that fails Identity.Validate.
//
// THE CANONICAL-BYTES GATE. The spec fixes the sidecar's serialization
// exactly ("canonical JSON — sorted keys, trailing newline"), so anything
// else is an UNDECODABLE sidecar, not a tolerated variant: the decoded
// document is re-encoded through the same canonical encoder EncodeSidecar
// uses and must byte-compare equal to the input. That single check closes
// every silent-reinterpretation hole encoding/json otherwise leaves open —
// an explicit `"patch_sha256": null` (which decodes to the absent/exact
// shape while the bytes say otherwise), a duplicate key (last-wins), and
// any non-canonical key order, interior whitespace, string escaping, or
// missing/extra trailing newline. A sidecar is the materialization
// completion witness and is written once and never edited, so a byte the
// writer would not have produced means the file is not this package's
// sidecar and must never be read as one.
func DecodeSidecar(data []byte) (Identity, error) {
	var doc sidecarDoc
	if err := artifact.DecodeStrictJSON(data, &doc); err != nil {
		return Identity{}, fmt.Errorf("execworkspace: decoding request sidecar: %w", err)
	}
	canonical, err := encodeSidecarDoc(doc)
	if err != nil {
		return Identity{}, fmt.Errorf("execworkspace: decoding request sidecar: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return Identity{}, fmt.Errorf(
			"execworkspace: request sidecar: bytes are not the canonical encoding of the document they decode to (want %q, got %q)",
			canonical, data,
		)
	}
	if doc.Schema != sidecarSchema {
		return Identity{}, fmt.Errorf("execworkspace: request sidecar: schema %q, want %q", doc.Schema, sidecarSchema)
	}
	if err := validateCommitSHA(doc.CommitSHA); err != nil {
		return Identity{}, fmt.Errorf("execworkspace: request sidecar: %w", err)
	}
	id := Identity{Shape: ExactSHA, RunID: doc.RunID, CommitSHA: doc.CommitSHA}
	if doc.PatchSHA256 != nil {
		if err := validatePatchSHA256(*doc.PatchSHA256); err != nil {
			return Identity{}, fmt.Errorf("execworkspace: request sidecar: %w", err)
		}
		id.Shape = BasePlusPatch
		id.PatchSHA256 = *doc.PatchSHA256
	}
	if err := id.Validate(); err != nil {
		return Identity{}, fmt.Errorf("execworkspace: request sidecar: %w", err)
	}
	return id, nil
}

// ErrIdentityMismatch is the typed hard error a caller returns when a
// proposed materialization request's identity does not byte-compare equal
// to an existing workspace's recorded sidecar identity (spec §Workspace
// naming: "a hard error naming both requests, never a silent merge — the
// RefSlug collision rule extended from the slug level to the full request
// identity"). Later lanes (materialization, reuse verification) construct
// this via VerifyIdentity.
type ErrIdentityMismatch struct {
	WorkspaceID string
	Proposed    Identity
	Recorded    Identity
}

func (e *ErrIdentityMismatch) Error() string {
	return fmt.Sprintf(
		"execworkspace: workspace id %q: proposed request %s does not match recorded request %s",
		e.WorkspaceID, e.Proposed, e.Recorded,
	)
}

// VerifyIdentity byte-compares proposed against recorded (Identity.Equal)
// and returns nil when they match. A mismatch returns *ErrIdentityMismatch
// naming workspaceID and both requests in full — never a silent merge and
// never a partial (e.g. slug-only) comparison.
func VerifyIdentity(workspaceID string, proposed, recorded Identity) error {
	if proposed.Equal(recorded) {
		return nil
	}
	return &ErrIdentityMismatch{WorkspaceID: workspaceID, Proposed: proposed, Recorded: recorded}
}
