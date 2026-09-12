package sealedexec

import (
	"context"
	"strings"
	"testing"
)

// A mismatched typed event payload is rejected by the owning event codec before
// the owner arm is serialized. The client must emit no partial request frame.
func TestPublicControllerConsolidationPayloadOwner(t *testing.T) {
	call := controllerCallFixture(t, 1, ControllerOperationAppendReceipt)
	other, _ := controllerEventFixture(t, validExecutionRequest(t, ActionStart))
	call.AppendReceipt.Append.Event.Payload = other.Payload
	if _, err := EncodeControllerCall(call); err == nil || !strings.Contains(err.Error(), "typed payload") {
		t.Fatalf("owning payload refusal = %v", err)
	}
	transport := &controllerMemoryTransport{}
	client, err := NewControllerClient(transport)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.AppendReceipt(context.Background(), call.AppendReceipt.Append); err == nil || !strings.Contains(err.Error(), "typed payload") {
		t.Fatalf("client owning payload refusal = %v", err)
	}
	if transport.written.Len() != 0 {
		t.Fatalf("client wrote %d bytes", transport.written.Len())
	}
}

func TestPublicControllerConsolidationDomainUnions(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		call := controllerCallFixture(t, 1, ControllerOperationVerifyAuthority)
		call.NextStamp = ControllerNextStampRequest{Schema: controllerRequestSchema(ControllerOperationNextStamp)}
		if _, err := EncodeControllerCall(call); err == nil {
			t.Fatal("accepted multiple typed request arms")
		}
	})
	t.Run("result", func(t *testing.T) {
		result := controllerResultFixture(t, 1, ControllerOperationVerifyAuthority)
		other := controllerResultFixture(t, 1, ControllerOperationNextStamp)
		result.NextStamp = other.NextStamp
		if _, err := EncodeControllerResult(result); err == nil {
			t.Fatal("accepted multiple typed result arms")
		}
	})
}
