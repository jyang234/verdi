package sealedexec

import (
	"encoding/json"
	"fmt"
)

func encodeClaimControllerCallPayload(call ControllerCall) (json.RawMessage, error) {
	wantSchema := controllerRequestSchema(call.Operation)

	if err := requireOnlyCallArm(call, call.ResolveClaimMCP); err != nil {
		return nil, err
	}
	if call.ResolveClaimMCP.Schema != wantSchema {
		return nil, operationSchemaError(call.Operation)
	}
	query, err := claimMCPQueryToWire(call.ResolveClaimMCP.Query)
	if err != nil {
		return nil, err
	}
	return marshalControllerPayload(struct {
		Query  claimMCPQueryWire `json:"query"`
		Schema string            `json:"schema"`
	}{query, wantSchema})
}
func decodeClaimControllerCallPayload(raw json.RawMessage, call *ControllerCall) error {
	schema := controllerRequestSchema(call.Operation)

	var wire struct {
		Query  claimMCPQueryWire `json:"query"`
		Schema string            `json:"schema"`
	}
	if err := unmarshalControllerPayload(raw, &wire); err != nil {
		return err
	}
	if wire.Schema != schema {
		return operationSchemaError(call.Operation)
	}
	query, err := claimMCPQueryFromWire(wire.Query)
	if err != nil {
		return err
	}
	call.ResolveClaimMCP = ControllerResolveClaimMCPRequest{schema, query}
	return nil
}
func encodeClaimControllerSuccessPayload(result ControllerResult) (json.RawMessage, error) {
	wantSchema := controllerResultSchema(result.Operation)
	if !controllerSuccessArmsMatch(result) {
		return nil, fmt.Errorf("sealedexec: controller success carries wrong or multiple result arms")
	}

	if result.ResolveClaimMCP.Schema != wantSchema {
		return nil, operationSchemaError(result.Operation)
	}
	registration, err := claimMCPRegistrationToWire(result.ResolveClaimMCP.Registration)
	if err != nil {
		return nil, err
	}
	return marshalControllerPayload(struct {
		Registration claimMCPRegistrationWire `json:"registration"`
		Schema       string                   `json:"schema"`
	}{registration, wantSchema})
}
func decodeClaimControllerSuccessPayload(raw json.RawMessage, result *ControllerResult) error {
	schema := controllerResultSchema(result.Operation)

	var wire struct {
		Registration claimMCPRegistrationWire `json:"registration"`
		Schema       string                   `json:"schema"`
	}
	if err := unmarshalControllerPayload(raw, &wire); err != nil {
		return err
	}
	if wire.Schema != schema {
		return operationSchemaError(result.Operation)
	}
	registration, err := claimMCPRegistrationFromWire(wire.Registration)
	if err != nil {
		return err
	}
	result.ResolveClaimMCP = ControllerResolveClaimMCPResult{schema, registration}
	return nil
}
