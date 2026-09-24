package signedapproval

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
)

// CommitVerifier is the consumer-defined port (04 §port pattern) for the
// forge's own verification of one commit. A returned Go error is an
// operational failure only; an unreachable forge or an adapter that cannot
// answer is a CommitVerification with Available false.
type CommitVerifier interface {
	VerifyCommit(ctx context.Context, commit string) (CommitVerification, error)
}

// CommitVerification is the forge's report on one commit's signature.
type CommitVerification struct {
	// Commit echoes the full lowercase object id that was asked about.
	Commit string
	// Available is false when the forge is unreachable or the adapter
	// does not support the read; UnavailableReason is then required and
	// every other field is empty.
	Available         bool
	UnavailableReason string
	// Verified reports that the forge verified the signature with reason
	// "valid".
	Verified bool
	// SignerAccountID is the canonical positive decimal forge account id
	// the verified signature is attributed to; empty unless Verified.
	SignerAccountID string
	// ProviderSnapshotID is the sha256:<64 hex> identity of the forge
	// facts; required when Available.
	ProviderSnapshotID string
}

// objectIDRe is a full lowercase SHA-1 or SHA-256 object id.
var objectIDRe = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// snapshotIDRe is the canonical provider snapshot identity.
var snapshotIDRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Validate enforces the port contract. A violation means the adapter is
// broken, which is an operational error, never a verdict.
func (v CommitVerification) Validate() error {
	if !objectIDRe.MatchString(v.Commit) {
		return fmt.Errorf("signedapproval: commit verification: commit %q is not a full lowercase object id", v.Commit)
	}
	if !v.Available {
		if v.UnavailableReason == "" {
			return fmt.Errorf("signedapproval: commit verification for %s: an unavailable report must carry a reason", v.Commit)
		}
		if v.Verified || v.SignerAccountID != "" || v.ProviderSnapshotID != "" {
			return fmt.Errorf("signedapproval: commit verification for %s: an unavailable report must carry no verification, signer, or snapshot", v.Commit)
		}
		return nil
	}
	if v.UnavailableReason != "" {
		return fmt.Errorf("signedapproval: commit verification for %s: an available report must carry no unavailable reason", v.Commit)
	}
	if !snapshotIDRe.MatchString(v.ProviderSnapshotID) {
		return fmt.Errorf("signedapproval: commit verification for %s: provider snapshot id %q is not sha256:<64 lowercase hex>", v.Commit, v.ProviderSnapshotID)
	}
	if !v.Verified {
		if v.SignerAccountID != "" {
			return fmt.Errorf("signedapproval: commit verification for %s: an unverified report must carry no signer account id", v.Commit)
		}
		return nil
	}
	if !canonicalAccountID(v.SignerAccountID) {
		return fmt.Errorf("signedapproval: commit verification for %s: signer account id %q is not a canonical positive decimal", v.Commit, v.SignerAccountID)
	}
	return nil
}

// canonicalAccountID reports whether s is a canonical positive base-10
// forge account id, the form forge approval subjects already use.
func canonicalAccountID(s string) bool {
	n, err := strconv.ParseInt(s, 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == s
}
