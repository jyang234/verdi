package governanceprincipal

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"
)

// principalIDPrefix is the fixed first segment of every canonical
// principal identifier.
const principalIDPrefix = "principal"

// PrincipalID is a canonical principal identifier:
//
//	principal/<trust-source-id>/<base64url-without-padding(subject UTF-8)>
//
// It is derived by the kernel only (CanonicalPrincipalID) and never
// accepted pre-built from an adapter; adapters supply a trust-source ID
// and a stable subject.
type PrincipalID string

// CanonicalPrincipalID derives the canonical principal identifier for a
// trust source and stable adapter subject. The source ID must satisfy the
// profile-local ID grammar; the subject must be nonempty valid UTF-8. The
// subject segment is base64url-encoded without padding, so distinct
// (source, subject) pairs always derive distinct identifiers.
func CanonicalPrincipalID(sourceID, subject string) (PrincipalID, error) {
	if err := ValidateID(sourceID); err != nil {
		return "", fmt.Errorf("governanceprincipal: principal trust-source id: %w", err)
	}
	if subject == "" {
		return "", fmt.Errorf("governanceprincipal: principal subject must be nonempty")
	}
	if !utf8.ValidString(subject) {
		return "", fmt.Errorf("governanceprincipal: principal subject is not valid UTF-8")
	}
	enc := base64.RawURLEncoding.EncodeToString([]byte(subject))
	return PrincipalID(principalIDPrefix + "/" + sourceID + "/" + enc), nil
}

// Validate checks the full canonical grammar: exactly three segments, the
// fixed prefix, a valid trust-source ID, and an unpadded base64url subject
// segment that decodes to nonempty valid UTF-8.
func (id PrincipalID) Validate() error {
	parts := strings.Split(string(id), "/")
	if len(parts) != 3 || parts[0] != principalIDPrefix {
		return fmt.Errorf("governanceprincipal: principal id %q: want principal/<trust-source-id>/<base64url-subject>", string(id))
	}
	if err := ValidateID(parts[1]); err != nil {
		return fmt.Errorf("governanceprincipal: principal id %q: %w", string(id), err)
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("governanceprincipal: principal id %q: subject segment is not unpadded base64url: %w", string(id), err)
	}
	// Only the exact CanonicalPrincipalID output validates: strict decoding
	// rejects nonzero trailing padding bits, and the re-encode equality
	// closes every remaining noncanonical spelling of the same bytes.
	if base64.RawURLEncoding.EncodeToString(raw) != parts[2] {
		return fmt.Errorf("governanceprincipal: principal id %q: subject segment is not the canonical base64url encoding of its bytes", string(id))
	}
	if len(raw) == 0 {
		return fmt.Errorf("governanceprincipal: principal id %q: subject must be nonempty", string(id))
	}
	if !utf8.Valid(raw) {
		return fmt.Errorf("governanceprincipal: principal id %q: subject does not decode to valid UTF-8", string(id))
	}
	return nil
}
