package contextowner

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
)

func TestNewCall_MatchesFrozenFixtures(t *testing.T) {
	for _, op := range Operations() {
		op := op
		t.Run(string(op), func(t *testing.T) {
			requestArm := fixtureArm(t, string(op)+".request.json")
			call, err := NewCall(op, requestArm)
			if err != nil {
				t.Fatalf("NewCall(%s): %v", op, err)
			}
			got, err := EncodeCall(call)
			if err != nil {
				t.Fatalf("EncodeCall(NewCall(%s)): %v", op, err)
			}
			want := readPublicContractFixture(t, string(op)+".owner-call.json")
			if !bytes.Equal(got, want) {
				t.Errorf("EncodeCall(NewCall(%s)) =\n%s\nwant\n%s", op, got, want)
			}
			arm, err := RequestArm(call)
			if err != nil {
				t.Fatalf("RequestArm(NewCall(%s)): %v", op, err)
			}
			if !bytes.Equal(arm, requestArm) {
				t.Errorf("RequestArm(NewCall(%s)) =\n%s\nwant\n%s", op, arm, requestArm)
			}
		})
	}

	t.Run("verify-epoch.nonproven-resolution", func(t *testing.T) {
		requestArm := fixtureArm(t, "verify-epoch.nonproven-resolution.request.json")
		call, err := NewCall(OperationVerifyEpoch, requestArm)
		if err != nil {
			t.Fatalf("NewCall(verify-epoch nonproven-resolution): %v", err)
		}
		resolution := call.VerifyEpoch.Check.Resolution
		if resolution.State != contextcompile.ResolutionUnproven || len(resolution.Data) != 0 {
			t.Fatalf("decoded epoch resolution = %#v, want unproven without data", resolution)
		}
		got, err := EncodeCall(call)
		if err != nil {
			t.Fatalf("EncodeCall: %v", err)
		}
		want := readPublicContractFixture(t, "verify-epoch.nonproven-resolution.owner-call.json")
		if !bytes.Equal(got, want) {
			t.Errorf("EncodeCall(NewCall(verify-epoch nonproven-resolution)) =\n%s\nwant\n%s", got, want)
		}
		arm, err := RequestArm(call)
		if err != nil {
			t.Fatalf("RequestArm: %v", err)
		}
		if !bytes.Equal(arm, requestArm) {
			t.Errorf("RequestArm(verify-epoch nonproven-resolution) =\n%s\nwant\n%s", arm, requestArm)
		}
	})
}

func TestNewReply_MatchesFrozenFixtures(t *testing.T) {
	for _, op := range Operations() {
		op := op
		t.Run(string(op), func(t *testing.T) {
			call, err := NewCall(op, fixtureArm(t, string(op)+".request.json"))
			if err != nil {
				t.Fatalf("NewCall(%s): %v", op, err)
			}
			resultArm := fixtureArm(t, string(op)+".result.json")
			reply, err := NewReply(call, resultArm)
			if err != nil {
				t.Fatalf("NewReply(%s): %v", op, err)
			}
			got, err := EncodeReply(reply)
			if err != nil {
				t.Fatalf("EncodeReply(NewReply(%s)): %v", op, err)
			}
			want := readPublicContractFixture(t, string(op)+".owner-reply.json")
			if !bytes.Equal(got, want) {
				t.Errorf("EncodeReply(NewReply(%s)) =\n%s\nwant\n%s", op, got, want)
			}
			arm, err := ResultArm(reply)
			if err != nil {
				t.Fatalf("ResultArm(NewReply(%s)): %v", op, err)
			}
			if !bytes.Equal(arm, resultArm) {
				t.Errorf("ResultArm(NewReply(%s)) =\n%s\nwant\n%s", op, arm, resultArm)
			}
		})
	}

	t.Run("resolve-context.nonproven via resolve-context's call", func(t *testing.T) {
		call, err := NewCall(OperationResolveContext, fixtureArm(t, "resolve-context.request.json"))
		if err != nil {
			t.Fatalf("NewCall(resolve-context): %v", err)
		}
		resultArm := fixtureArm(t, "resolve-context.nonproven.result.json")
		reply, err := NewReply(call, resultArm)
		if err != nil {
			t.Fatalf("NewReply(resolve-context.nonproven): %v", err)
		}
		got, err := EncodeReply(reply)
		if err != nil {
			t.Fatalf("EncodeReply: %v", err)
		}
		want := readPublicContractFixture(t, "resolve-context.nonproven.owner-reply.json")
		if !bytes.Equal(got, want) {
			t.Errorf("EncodeReply(NewReply(resolve-context.nonproven)) =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("verify-epoch.nonproven-resolution with the ordinary verify-epoch result", func(t *testing.T) {
		call, err := NewCall(OperationVerifyEpoch, fixtureArm(t, "verify-epoch.nonproven-resolution.request.json"))
		if err != nil {
			t.Fatalf("NewCall(verify-epoch nonproven-resolution): %v", err)
		}
		resultArm := fixtureArm(t, "verify-epoch.result.json")
		reply, err := NewReply(call, resultArm)
		if err != nil {
			t.Fatalf("NewReply(verify-epoch nonproven-resolution variant, ordinary result): %v", err)
		}
		got, err := EncodeReply(reply)
		if err != nil {
			t.Fatalf("EncodeReply: %v", err)
		}
		want := readPublicContractFixture(t, "verify-epoch.nonproven-resolution.owner-reply.json")
		if !bytes.Equal(got, want) {
			t.Errorf("EncodeReply(...) =\n%s\nwant\n%s", got, want)
		}
	})
}

// TestNewReply_TypedNonProvenVerdictsSucceed proves contract §2.1's rule that
// "a typed negative or unproven verification remains a successful protocol
// result": NewReply must not treat a non-proven or violated-with-witness
// state as an error as long as the reply stays structurally valid and, where
// a relation applies, the identities still agree.
func TestNewReply_TypedNonProvenVerdictsSucceed(t *testing.T) {
	t.Run("verify-epoch violated-with-witness", func(t *testing.T) {
		call, err := NewCall(OperationVerifyEpoch, fixtureArm(t, "verify-epoch.request.json"))
		if err != nil {
			t.Fatalf("NewCall: %v", err)
		}
		armBytes, err := encodeResultArm(Reply{
			Call: call,
			VerifyEpoch: VerifyEpochResult{
				Schema: ResultSchema(OperationVerifyEpoch),
				Verification: Verification{
					State:     contextcompile.ResolutionViolatedWithWitness,
					Failure:   FailureMismatch,
					Witnesses: []string{"epoch state has changed"},
				},
			},
		})
		if err != nil {
			t.Fatalf("encodeResultArm: %v", err)
		}
		reply, err := NewReply(call, armBytes)
		if err != nil {
			t.Fatalf("NewReply(verify-epoch violated-with-witness): %v", err)
		}
		if reply.VerifyEpoch.Verification.State != contextcompile.ResolutionViolatedWithWitness {
			t.Errorf("NewReply did not preserve the violated-with-witness verification state")
		}
	})

	t.Run("verify-authority unproven facts with matching identities", func(t *testing.T) {
		call, err := NewCall(OperationVerifyAuthority, fixtureArm(t, "verify-authority.request.json"))
		if err != nil {
			t.Fatalf("NewCall: %v", err)
		}
		var provenResult VerifyAuthorityResult
		if err := json.Unmarshal(fixtureArm(t, "verify-authority.result.json"), &provenResult); err != nil {
			t.Fatalf("unmarshal proven result: %v", err)
		}
		facts := provenResult.Facts
		facts.State = contextcompile.ResolutionUnproven
		facts.Failure = FailureUnavailable
		facts.Witnesses = []string{"authority verifier unavailable"}
		armBytes, err := encodeResultArm(Reply{
			Call:            call,
			VerifyAuthority: VerifyAuthorityResult{Schema: ResultSchema(OperationVerifyAuthority), Facts: facts},
		})
		if err != nil {
			t.Fatalf("encodeResultArm: %v", err)
		}
		reply, err := NewReply(call, armBytes)
		if err != nil {
			t.Fatalf("NewReply(verify-authority unproven facts with matching identities): %v", err)
		}
		if reply.VerifyAuthority.Facts.State != contextcompile.ResolutionUnproven {
			t.Errorf("NewReply did not preserve the unproven authority state")
		}
	})

	t.Run("resolve-context non-proven without data", func(t *testing.T) {
		call, err := NewCall(OperationResolveContext, fixtureArm(t, "resolve-context.request.json"))
		if err != nil {
			t.Fatalf("NewCall: %v", err)
		}
		resultArm := fixtureArm(t, "resolve-context.nonproven.result.json")
		reply, err := NewReply(call, resultArm)
		if err != nil {
			t.Fatalf("NewReply(resolve-context non-proven): %v", err)
		}
		if reply.ResolveContext.Resolution.State == contextcompile.ResolutionProven {
			t.Fatalf("test fixture is not actually non-proven")
		}
		if len(reply.ResolveContext.Resolution.Data) != 0 {
			t.Errorf("non-proven resolution unexpectedly carries data")
		}
	})
}

func TestNewReply_StructuralNegatives(t *testing.T) {
	call, err := NewCall(OperationVerifyAuthority, fixtureArm(t, "verify-authority.request.json"))
	if err != nil {
		t.Fatalf("NewCall: %v", err)
	}
	validResultArm := fixtureArm(t, "verify-authority.result.json")
	publicSchema := publicOwnerResultSchemaLiteral("verify-authority")

	tests := []struct {
		name string
		arm  []byte
	}{
		{"result arm of another operation", fixtureArm(t, "resolve-profile.result.json")},
		{"wrong schema", bytes.Replace(validResultArm, []byte(`"`+publicSchema+`"`), []byte(`"verdi.context-owner/bogus-result/v1"`), 1)},
		{"unknown schema", bytes.Replace(validResultArm, []byte(`"`+publicSchema+`"`), []byte(`"verdi.context-owner/unknown-result/v9"`), 1)},
		{"trailing LF", readPublicContractFixture(t, "verify-authority.result.json")},
		{"noncanonical key order", reorderTopLevelMembers(t, validResultArm)},
		{"extra whitespace", append([]byte(" "), validResultArm...)},
		{"unknown member", addUnknownMember(t, validResultArm, "unexpected_member")},
		{"duplicate member", duplicateMember(t, validResultArm, "schema")},
		{"invalid enum value", bytes.Replace(validResultArm, []byte(`"proven"`), []byte(`"maybe"`), 1)},
		{
			"invalid digest value",
			bytes.Replace(validResultArm,
				[]byte(`"manifest_digest":"sha256:77a2c3dc80d395e7b382dc4198f4d5608f5fc410761bf527ce638784226bfb82"`),
				[]byte(`"manifest_digest":"not-a-digest"`), 1),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if bytes.Equal(tc.arm, validResultArm) {
				t.Fatalf("test setup: %s did not change the fixture", tc.name)
			}
			reply, err := NewReply(call, tc.arm)
			if err == nil {
				t.Fatalf("NewReply accepted %s: %+v", tc.name, reply)
			}
			if errors.Is(err, ErrRelationMismatch) {
				t.Fatalf("NewReply(%s) reported a relation mismatch for a structural failure: %v", tc.name, err)
			}
		})
	}

	t.Run("install-expansion v1 request", func(t *testing.T) {
		v2Arm := fixtureArm(t, "install-expansion.request.json")
		v1Arm := bytes.Replace(v2Arm,
			[]byte(`"verdi.context-owner/install-expansion-request/v2"`),
			[]byte(`"verdi.context-owner/install-expansion-request/v1"`), 1)
		if bytes.Equal(v1Arm, v2Arm) {
			t.Fatalf("test setup: v1 substitution did not change the fixture")
		}
		if _, err := NewCall(OperationInstallExpansion, v1Arm); err == nil {
			t.Fatalf("NewCall accepted an install-expansion v1 request arm")
		}
	})

	t.Run("caller-supplied digest refused", func(t *testing.T) {
		tampered := call
		tampered.ControllerRequestDigest = digestBytes([]byte("another-preimage-entirely"))
		if tampered.ControllerRequestDigest == call.ControllerRequestDigest {
			t.Fatalf("test setup: tampered digest collided with the genuine one")
		}
		if _, err := NewReply(tampered, validResultArm); err == nil {
			t.Fatalf("NewReply accepted a call whose controller_request_digest was replaced by another canonical digest")
		}
	})
}

// TestNewReply_NonProvenResolutionDataPresence is N1: contract §2.1's data-
// presence rule -- "a non-proven context resolution omits data; a proven one
// requires it, also when embedded in an epoch check" -- proved directly
// against the frozen fixtures and NewCall/NewReply, independent of the
// pre-existing coverage over EncodeCall/EncodeReply/DecodeCall/DecodeReply.
func TestNewReply_NonProvenResolutionDataPresence(t *testing.T) {
	t.Run("frozen resolve-context.nonproven result carries no data member", func(t *testing.T) {
		if arm := fixtureArm(t, "resolve-context.nonproven.result.json"); bytes.Contains(arm, []byte(`"data"`)) {
			t.Fatalf("resolve-context.nonproven.result.json unexpectedly names a data member: %s", arm)
		}
	})

	t.Run("frozen verify-epoch.nonproven-resolution request carries no data member at its resolution", func(t *testing.T) {
		if arm := fixtureArm(t, "verify-epoch.nonproven-resolution.request.json"); bytes.Contains(arm, []byte(`"data"`)) {
			t.Fatalf("verify-epoch.nonproven-resolution.request.json unexpectedly names a data member: %s", arm)
		}
	})

	resolveContextCall, err := NewCall(OperationResolveContext, fixtureArm(t, "resolve-context.request.json"))
	if err != nil {
		t.Fatalf("NewCall(resolve-context): %v", err)
	}

	t.Run("adding data to a non-proven standalone resolution is refused", func(t *testing.T) {
		armBytes := addDataToResolutionMember(t,
			fixtureArm(t, "resolve-context.nonproven.result.json"), "resolution",
			dataMemberOf(t, fixtureArm(t, "resolve-context.result.json"), "resolution"))
		if _, err := NewReply(resolveContextCall, armBytes); err == nil {
			t.Fatalf("NewReply accepted a non-proven standalone resolution carrying data")
		} else if errors.Is(err, ErrRelationMismatch) {
			t.Fatalf("adding data to a non-proven resolution reported a relation mismatch, want a structural refusal: %v", err)
		}
	})

	t.Run("removing data from the proven resolve-context result is refused", func(t *testing.T) {
		armBytes := removeMemberFrom(t, fixtureArm(t, "resolve-context.result.json"), "resolution", "data")
		if _, err := NewReply(resolveContextCall, armBytes); err == nil {
			t.Fatalf("NewReply accepted a proven resolution with data removed")
		} else if errors.Is(err, ErrRelationMismatch) {
			t.Fatalf("removing data from a proven resolution reported a relation mismatch, want a structural refusal: %v", err)
		}
	})

	t.Run("adding data to the non-proven resolution embedded in the epoch check is refused", func(t *testing.T) {
		provenEpochArm := fixtureArm(t, "verify-epoch.request.json")
		data := dataMemberOf(t, memberOf(t, provenEpochArm, "check"), "resolution")
		nonProvenEpochArm := fixtureArm(t, "verify-epoch.nonproven-resolution.request.json")
		armBytes := addDataToNestedResolutionMember(t, nonProvenEpochArm, "check", "resolution", data)
		if _, err := NewCall(OperationVerifyEpoch, armBytes); err == nil {
			t.Fatalf("NewCall accepted a non-proven embedded resolution carrying data")
		}
	})

	t.Run("non-proven resolution embedded in the epoch check with data:null is refused", func(t *testing.T) {
		armBytes := setNestedResolutionDataNull(t, fixtureArm(t, "verify-epoch.nonproven-resolution.request.json"), "check", "resolution")
		if _, err := NewCall(OperationVerifyEpoch, armBytes); err == nil {
			t.Fatalf("NewCall accepted a non-proven embedded resolution with data:null")
		}
	})
}

// memberOf decodes document (a JSON object) and returns member's raw bytes.
func memberOf(t *testing.T, document []byte, member string) json.RawMessage {
	t.Helper()
	var members map[string]json.RawMessage
	if err := json.Unmarshal(document, &members); err != nil {
		t.Fatalf("unmarshal document for member %q: %v", member, err)
	}
	value, ok := members[member]
	if !ok {
		t.Fatalf("document has no member %q: %s", member, document)
	}
	return value
}

// dataMemberOf returns resolutionParent[resolutionField]["data"].
func dataMemberOf(t *testing.T, document []byte, resolutionField string) json.RawMessage {
	t.Helper()
	return memberOf(t, memberOf(t, document, resolutionField), "data")
}

// addDataToResolutionMember returns document with data assigned as the
// "data" member of document[resolutionField].
func addDataToResolutionMember(t *testing.T, document []byte, resolutionField string, data json.RawMessage) []byte {
	t.Helper()
	var members map[string]json.RawMessage
	if err := json.Unmarshal(document, &members); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	var resolution map[string]json.RawMessage
	if err := json.Unmarshal(members[resolutionField], &resolution); err != nil {
		t.Fatalf("unmarshal %s: %v", resolutionField, err)
	}
	resolution["data"] = data
	resolutionBytes, err := json.Marshal(resolution)
	if err != nil {
		t.Fatalf("marshal mutated %s: %v", resolutionField, err)
	}
	members[resolutionField] = resolutionBytes
	out, err := json.Marshal(members)
	if err != nil {
		t.Fatalf("marshal mutated document: %v", err)
	}
	return out
}

// addDataToNestedResolutionMember returns document with data assigned as the
// "data" member of document[parentField][resolutionField].
func addDataToNestedResolutionMember(t *testing.T, document []byte, parentField, resolutionField string, data json.RawMessage) []byte {
	t.Helper()
	var members map[string]json.RawMessage
	if err := json.Unmarshal(document, &members); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	mutatedParent := addDataToResolutionMember(t, members[parentField], resolutionField, data)
	members[parentField] = mutatedParent
	out, err := json.Marshal(members)
	if err != nil {
		t.Fatalf("marshal mutated document: %v", err)
	}
	return out
}

// removeMemberFrom returns document with member removed from
// document[parentField].
func removeMemberFrom(t *testing.T, document []byte, parentField, member string) []byte {
	t.Helper()
	var members map[string]json.RawMessage
	if err := json.Unmarshal(document, &members); err != nil {
		t.Fatalf("unmarshal document: %v", err)
	}
	var parent map[string]json.RawMessage
	if err := json.Unmarshal(members[parentField], &parent); err != nil {
		t.Fatalf("unmarshal %s: %v", parentField, err)
	}
	if _, ok := parent[member]; !ok {
		t.Fatalf("%s has no member %q to remove", parentField, member)
	}
	delete(parent, member)
	parentBytes, err := json.Marshal(parent)
	if err != nil {
		t.Fatalf("marshal mutated %s: %v", parentField, err)
	}
	members[parentField] = parentBytes
	out, err := json.Marshal(members)
	if err != nil {
		t.Fatalf("marshal mutated document: %v", err)
	}
	return out
}

// setNestedResolutionDataNull returns document with document[parentField]
// [resolutionField]["data"] set to an explicit JSON null.
func setNestedResolutionDataNull(t *testing.T, document []byte, parentField, resolutionField string) []byte {
	t.Helper()
	return addDataToNestedResolutionMember(t, document, parentField, resolutionField, json.RawMessage("null"))
}

func TestValidateResultArm(t *testing.T) {
	for _, op := range Operations() {
		t.Run(string(op), func(t *testing.T) {
			raw := fixtureArm(t, string(op)+".result.json")
			if err := ValidateResultArm(op, raw); err != nil {
				t.Fatal(err)
			}
			for name, bad := range map[string][]byte{"empty": nil, "null": []byte("null"), "trailing LF": append(append([]byte(nil), raw...), '\n'), "whitespace": append([]byte(" "), raw...), "unknown member": bytes.Replace(raw, []byte("{"), []byte(`{"extra":true,`), 1)} {
				t.Run(name, func(t *testing.T) {
					if err := ValidateResultArm(op, bad); err == nil {
						t.Fatal("accepted invalid result arm")
					}
				})
			}
		})
	}
	if err := ValidateResultArm(Operation("unknown"), []byte(`{}`)); err == nil {
		t.Fatal("accepted unknown operation")
	}
}
