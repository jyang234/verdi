package sealedexec

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextowner"
)

func TestControllerSnapshotValidationKeepsRequestOwners(t *testing.T) {
	call, err := DecodeControllerCall(bytes.NewReader(publicFixture(t, "verify-epoch.call.v2.json")))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := call.VerifyEpoch.Check.Snapshot
	if err = validateDomainSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	public, err := flightSnapshotToPublic(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []json.RawMessage{nil, json.RawMessage(`{"schema":"wrong"}`)} {
		bad := public
		bad.Request = request
		if err = contextowner.ValidateFlightStateSnapshotFields(bad); err != nil {
			t.Fatalf("fields-only seam unexpectedly validates request: %v", err)
		}
		if err = contextowner.ValidateFlightStateSnapshot(bad); err == nil {
			t.Fatal("full public snapshot validation skipped nested request")
		}
		owner, err := controllerCallToOwner(call)
		if err != nil {
			t.Fatal(err)
		}
		owner.VerifyEpoch.Check.Snapshot = bad
		if _, err = contextowner.RequestArm(owner); err == nil {
			t.Fatal("public operation producer skipped nested request")
		}
	}
	bad := snapshot
	bad.Request.Schema = "wrong"
	if err = validateDomainSnapshot(bad); err == nil || !strings.Contains(err.Error(), "sealedexec: execution request schema") {
		t.Fatalf("domain snapshot did not invoke execution request owner: %v", err)
	}
	// The actual nontransport compiler calls the domain snapshot helper before
	// checking later request identity and data facts.
	request := ChildCompileRequest{RequestID: "request", Ref: "ref", Purpose: "purpose", Snapshot: bad}
	if err = validateChildCompileRequest(request); err == nil || !strings.Contains(err.Error(), "invalid current flight-state snapshot: sealedexec: execution request schema") {
		t.Fatalf("compiler did not retain domain request validation: %v", err)
	}
	bad = snapshot
	bad.Key.Flight = string([]byte{0xff})
	if err = validateDomainSnapshot(bad); err == nil {
		t.Fatal("domain snapshot allowed invalid UTF-8 key")
	}
}
