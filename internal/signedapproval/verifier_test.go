package signedapproval

import (
	"strings"
	"testing"
)

const (
	testCommit   = "0123456789abcdef0123456789abcdef01234567"
	testSnapshot = "sha256:" + "ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd"
)

func TestCommitVerificationValidate(t *testing.T) {
	good := []struct {
		name string
		v    CommitVerification
	}{
		{name: "verified", v: CommitVerification{Commit: testCommit, Available: true, Verified: true, SignerAccountID: "1001", ProviderSnapshotID: testSnapshot}},
		{name: "available but unverified", v: CommitVerification{Commit: testCommit, Available: true, ProviderSnapshotID: testSnapshot}},
		{name: "unavailable", v: CommitVerification{Commit: testCommit, UnavailableReason: "forge unreachable"}},
		{name: "sha256 commit", v: CommitVerification{Commit: strings.Repeat("a", 64), UnavailableReason: "adapter unsupported"}},
	}
	for _, tc := range good {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.v.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}

	verified := func(mut func(*CommitVerification)) CommitVerification {
		v := CommitVerification{Commit: testCommit, Available: true, Verified: true, SignerAccountID: "1001", ProviderSnapshotID: testSnapshot}
		mut(&v)
		return v
	}
	bad := []struct {
		name, wantSubstr string
		v                CommitVerification
	}{
		{name: "empty commit", wantSubstr: "commit", v: verified(func(v *CommitVerification) { v.Commit = "" })},
		{name: "short commit", wantSubstr: "commit", v: verified(func(v *CommitVerification) { v.Commit = "abc123" })},
		{name: "uppercase commit", wantSubstr: "commit", v: verified(func(v *CommitVerification) { v.Commit = strings.ToUpper(testCommit) })},
		{name: "unavailable without reason", wantSubstr: "reason", v: CommitVerification{Commit: testCommit}},
		{name: "unavailable but verified", wantSubstr: "unavailable", v: CommitVerification{Commit: testCommit, UnavailableReason: "x", Verified: true}},
		{name: "unavailable with signer", wantSubstr: "unavailable", v: CommitVerification{Commit: testCommit, UnavailableReason: "x", SignerAccountID: "1001"}},
		{name: "unavailable with snapshot", wantSubstr: "unavailable", v: CommitVerification{Commit: testCommit, UnavailableReason: "x", ProviderSnapshotID: testSnapshot}},
		{name: "available with reason", wantSubstr: "reason", v: verified(func(v *CommitVerification) { v.UnavailableReason = "x" })},
		{name: "available without snapshot", wantSubstr: "snapshot", v: verified(func(v *CommitVerification) { v.ProviderSnapshotID = "" })},
		{name: "snapshot not sha256", wantSubstr: "snapshot", v: verified(func(v *CommitVerification) { v.ProviderSnapshotID = "md5:abc" })},
		{name: "snapshot uppercase hex", wantSubstr: "snapshot", v: verified(func(v *CommitVerification) { v.ProviderSnapshotID = strings.ToUpper(testSnapshot) })},
		{name: "unverified with signer", wantSubstr: "signer", v: verified(func(v *CommitVerification) { v.Verified = false })},
		{name: "verified without signer", wantSubstr: "signer", v: verified(func(v *CommitVerification) { v.SignerAccountID = "" })},
		{name: "signer with leading zero", wantSubstr: "signer", v: verified(func(v *CommitVerification) { v.SignerAccountID = "01001" })},
		{name: "signer zero", wantSubstr: "signer", v: verified(func(v *CommitVerification) { v.SignerAccountID = "0" })},
		{name: "signer negative", wantSubstr: "signer", v: verified(func(v *CommitVerification) { v.SignerAccountID = "-5" })},
		{name: "signer login name", wantSubstr: "signer", v: verified(func(v *CommitVerification) { v.SignerAccountID = "octocat" })},
		{name: "signer beyond int64", wantSubstr: "signer", v: verified(func(v *CommitVerification) { v.SignerAccountID = "99999999999999999999" })},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.v.Validate()
			if err == nil {
				t.Fatal("Validate: want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("Validate error %q does not mention %q", err, tc.wantSubstr)
			}
		})
	}
}
