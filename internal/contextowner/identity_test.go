package contextowner

import (
	"bytes"
	"encoding/json"
	"sort"
	"testing"
)

// --- Shared JSON-mutation helpers, also used by local_test.go and
// relations_test.go: white-box tests in this package cannot import
// internal/sealedexec (that would cycle back through the bridge this
// package publishes for), so every adverse fixture below is built from the
// frozen bytes and this package's own public API, never a private codec.

// fixtureArm loads a frozen fixture file and strips its one trailing LF,
// producing the "no trailing LF" convention every local.go/identity.go
// entry point accepts for a request or result arm.
func fixtureArm(t *testing.T, name string) []byte {
	t.Helper()
	data := readPublicContractFixture(t, name)
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("%s does not end in exactly one LF", name)
	}
	return data[:len(data)-1]
}

// reorderTopLevelMembers returns document with its top-level members
// reassembled in descending key order -- deliberately not canonical
// (ascending) order whenever document has at least two distinct top-level
// members -- without changing any member's own bytes.
func reorderTopLevelMembers(t *testing.T, document []byte) []byte {
	t.Helper()
	var members map[string]json.RawMessage
	if err := json.Unmarshal(document, &members); err != nil {
		t.Fatalf("unmarshal document to reorder: %v", err)
	}
	keys := make([]string, 0, len(members))
	for key := range members {
		keys = append(keys, key)
	}
	if len(keys) < 2 {
		t.Fatalf("document has fewer than two top-level members: %s", document)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		encodedKey, err := json.Marshal(key)
		if err != nil {
			t.Fatalf("marshal key %q: %v", key, err)
		}
		buf.Write(encodedKey)
		buf.WriteByte(':')
		buf.Write(members[key])
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// addUnknownMember returns document with one additional, previously absent
// top-level member.
func addUnknownMember(t *testing.T, document []byte, key string) []byte {
	t.Helper()
	var members map[string]json.RawMessage
	if err := json.Unmarshal(document, &members); err != nil {
		t.Fatalf("unmarshal document to add member: %v", err)
	}
	if _, present := members[key]; present {
		t.Fatalf("member %q is already present in %s", key, document)
	}
	members[key] = json.RawMessage(`"probe"`)
	out, err := json.Marshal(members)
	if err != nil {
		t.Fatalf("marshal document with added member: %v", err)
	}
	return out
}

// duplicateMember returns document with one extra, byte-appended occurrence
// of key immediately before the closing brace -- a syntactically legal JSON
// object with a duplicate top-level key, which no canonical document may
// ever carry.
func duplicateMember(t *testing.T, document []byte, key string) []byte {
	t.Helper()
	if len(document) < 2 || document[0] != '{' || document[len(document)-1] != '}' {
		t.Fatalf("document is not a JSON object: %s", document)
	}
	out := append([]byte{}, document[:len(document)-1]...)
	out = append(out, []byte(`,"`+key+`":"probe"}`)...)
	return out
}

func TestLegacyRequestSchema(t *testing.T) {
	for _, op := range Operations() {
		want := legacyControllerRequestSchemaLiteral(string(op))
		if got := LegacyRequestSchema(op); got != want {
			t.Errorf("LegacyRequestSchema(%s) = %q, want %q", op, got, want)
		}
	}
	if got, want := LegacyRequestSchema(OperationInstallExpansion), "verdi.context-controller/install-expansion-request/v2"; got != want {
		t.Errorf("LegacyRequestSchema(install-expansion) = %q, want the v2 literal %q", got, want)
	}
	for _, op := range Operations() {
		if op == OperationInstallExpansion {
			continue
		}
		if got, want := LegacyRequestSchema(op), "verdi.context-controller/"+string(op)+"-request/v1"; got != want {
			t.Errorf("LegacyRequestSchema(%s) = %q, want the v1 literal %q", op, got, want)
		}
	}
	for _, op := range []Operation{"resolve-claim-mcp", "bogus-operation", ""} {
		if got := LegacyRequestSchema(op); got != "" {
			t.Errorf("LegacyRequestSchema(%s) = %q, want the empty string", op, got)
		}
	}
}

func TestLegacyRequestPreimage_MatchesFrozenFixtures(t *testing.T) {
	for _, op := range Operations() {
		op := op
		t.Run(string(op), func(t *testing.T) {
			requestArm := fixtureArm(t, string(op)+".request.json")
			want := readPublicContractFixture(t, string(op)+".legacy-request.json")
			got, err := LegacyRequestPreimage(op, requestArm)
			if err != nil {
				t.Fatalf("LegacyRequestPreimage(%s): %v", op, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("LegacyRequestPreimage(%s) =\n%s\nwant\n%s", op, got, want)
			}
		})
	}

	t.Run("verify-epoch.nonproven-resolution", func(t *testing.T) {
		requestArm := fixtureArm(t, "verify-epoch.nonproven-resolution.request.json")
		want := readPublicContractFixture(t, "verify-epoch.nonproven-resolution.legacy-request.json")
		got, err := LegacyRequestPreimage(OperationVerifyEpoch, requestArm)
		if err != nil {
			t.Fatalf("LegacyRequestPreimage(verify-epoch nonproven-resolution): %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("LegacyRequestPreimage(verify-epoch nonproven-resolution) =\n%s\nwant\n%s", got, want)
		}
	})
}

func TestControllerRequestDigest_MatchesFrozenFixtures(t *testing.T) {
	var digests map[string]string
	if err := json.Unmarshal(readPublicContractFixture(t, "digests.json"), &digests); err != nil {
		t.Fatalf("unmarshal digests.json: %v", err)
	}

	for _, op := range Operations() {
		op := op
		t.Run(string(op), func(t *testing.T) {
			requestArm := fixtureArm(t, string(op)+".request.json")
			got, err := ControllerRequestDigest(op, requestArm)
			if err != nil {
				t.Fatalf("ControllerRequestDigest(%s): %v", op, err)
			}
			if got != digests[string(op)] {
				t.Errorf("ControllerRequestDigest(%s) = %q, want digests.json value %q", op, got, digests[string(op)])
			}

			var callMembers map[string]json.RawMessage
			if err := json.Unmarshal(readPublicContractFixture(t, string(op)+".owner-call.json"), &callMembers); err != nil {
				t.Fatalf("unmarshal %s.owner-call.json: %v", op, err)
			}
			var wantFromCall string
			if err := json.Unmarshal(callMembers["controller_request_digest"], &wantFromCall); err != nil {
				t.Fatalf("unmarshal %s.owner-call.json controller_request_digest: %v", op, err)
			}
			if got != wantFromCall {
				t.Errorf("ControllerRequestDigest(%s) = %q, want %s.owner-call.json value %q", op, got, op, wantFromCall)
			}
		})
	}

	t.Run("verify-epoch.nonproven-resolution", func(t *testing.T) {
		requestArm := fixtureArm(t, "verify-epoch.nonproven-resolution.request.json")
		got, err := ControllerRequestDigest(OperationVerifyEpoch, requestArm)
		if err != nil {
			t.Fatalf("ControllerRequestDigest(verify-epoch nonproven-resolution): %v", err)
		}
		if got != digests["verify-epoch.nonproven-resolution"] {
			t.Errorf("ControllerRequestDigest(verify-epoch nonproven-resolution) = %q, want %q", got, digests["verify-epoch.nonproven-resolution"])
		}
	})
}

func TestLegacyRequestPreimage_Negatives(t *testing.T) {
	verifyAuthorityArm := fixtureArm(t, "verify-authority.request.json")
	resolveProfileArm := fixtureArm(t, "resolve-profile.request.json")
	installV2Arm := fixtureArm(t, "install-expansion.request.json")
	installV1Arm := bytes.Replace(installV2Arm,
		[]byte(`"verdi.context-owner/install-expansion-request/v2"`),
		[]byte(`"verdi.context-owner/install-expansion-request/v1"`), 1)
	if bytes.Equal(installV1Arm, installV2Arm) {
		t.Fatalf("test setup: install-expansion v1 substitution did not change the fixture")
	}

	legacyInPlaceOfPublic := bytes.Replace(verifyAuthorityArm,
		[]byte(`"`+publicOwnerRequestSchemaLiteral("verify-authority")+`"`),
		[]byte(`"`+legacyControllerRequestSchemaLiteral("verify-authority")+`"`), 1)
	if bytes.Equal(legacyInPlaceOfPublic, verifyAuthorityArm) {
		t.Fatalf("test setup: legacy-schema substitution did not change the fixture")
	}

	tests := []struct {
		name      string
		operation Operation
		arm       []byte
	}{
		{"resolve-claim-mcp", "resolve-claim-mcp", verifyAuthorityArm},
		{"unknown operation", "bogus-operation", verifyAuthorityArm},
		{"arm of a different operation", OperationVerifyAuthority, resolveProfileArm},
		{"arm with trailing LF", OperationVerifyAuthority, readPublicContractFixture(t, "verify-authority.request.json")},
		{"leading space", OperationVerifyAuthority, append([]byte(" "), verifyAuthorityArm...)},
		{"noncanonical key order", OperationResolveProfile, reorderTopLevelMembers(t, resolveProfileArm)},
		{"unknown member", OperationVerifyAuthority, addUnknownMember(t, verifyAuthorityArm, "unexpected_member")},
		{"duplicated member", OperationVerifyAuthority, duplicateMember(t, verifyAuthorityArm, "schema")},
		{"legacy schema literal in place of the public one", OperationVerifyAuthority, legacyInPlaceOfPublic},
		{"install-expansion arm with -request/v1", OperationInstallExpansion, installV1Arm},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := LegacyRequestPreimage(tc.operation, tc.arm); err == nil {
				t.Errorf("LegacyRequestPreimage(%s) accepted an arm that must be refused", tc.name)
			}
			if _, err := ControllerRequestDigest(tc.operation, tc.arm); err == nil {
				t.Errorf("ControllerRequestDigest(%s) accepted an arm that must be refused", tc.name)
			}
		})
	}
}
