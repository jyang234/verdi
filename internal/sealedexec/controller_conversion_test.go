package sealedexec

import (
	"bytes"
	"encoding/json"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextowner"
)

// These helpers exercise the production controller encode/decode boundary. They
// construct or inspect envelopes only; all semantic validation belongs to the
// production public arm codec and the owning domain artifact codecs.
func encodeResolutionForContract(value ContextResolution) (contextowner.ContextResolution, error) {
	result := ControllerResult{Schema: ControllerResultSchemaID, CallSequence: 1, Operation: ControllerOperationResolveContext}
	result.ResolveContext = ControllerResolveContextResult{Schema: controllerResultSchema(result.Operation), Resolution: value}
	frame, err := EncodeControllerResult(result)
	if err != nil {
		return contextowner.ContextResolution{}, err
	}
	var document struct {
		Payload struct {
			Result contextowner.ResolveContextResult `json:"result"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(frame, &document); err != nil {
		return contextowner.ContextResolution{}, err
	}
	return document.Payload.Result.Resolution, nil
}

func decodeResolutionForContract(value contextowner.ContextResolution) (ContextResolution, error) {
	arm, err := canonjson.Marshal(contextowner.ResolveContextResult{
		Schema: contextowner.ResultSchema(contextowner.OperationResolveContext), Resolution: value,
	})
	if err != nil {
		return ContextResolution{}, err
	}
	payload, err := canonjson.Marshal(controllerResultPayloadWire{Result: trimFrame(arm)})
	if err != nil {
		return ContextResolution{}, err
	}
	frame, err := canonjson.Marshal(controllerResultWire{
		Schema: ControllerResultSchemaID, CallSequence: 1,
		Operation: ControllerOperationResolveContext, Payload: trimFrame(payload),
	})
	if err != nil {
		return ContextResolution{}, err
	}
	result, err := DecodeControllerResult(bytes.NewReader(frame))
	if err != nil {
		return ContextResolution{}, err
	}
	return result.ResolveContext.Resolution, nil
}

func encodeEpochCheckForContract(value EpochCheck) (contextowner.EpochCheck, error) {
	call := ControllerCall{Schema: ControllerCallSchemaID, CallSequence: 1, Operation: ControllerOperationVerifyEpoch}
	call.VerifyEpoch = ControllerVerifyEpochRequest{Schema: controllerRequestSchema(call.Operation), Check: value}
	frame, err := EncodeControllerCall(call)
	if err != nil {
		return contextowner.EpochCheck{}, err
	}
	var document struct {
		Payload contextowner.VerifyEpochRequest `json:"payload"`
	}
	if err := json.Unmarshal(frame, &document); err != nil {
		return contextowner.EpochCheck{}, err
	}
	return document.Payload.Check, nil
}

func decodeEpochCheckForContract(value contextowner.EpochCheck) (EpochCheck, error) {
	arm, err := canonjson.Marshal(contextowner.VerifyEpochRequest{
		Schema: contextowner.RequestSchema(contextowner.OperationVerifyEpoch), Check: value,
	})
	if err != nil {
		return EpochCheck{}, err
	}
	frame, err := canonjson.Marshal(controllerCallWire{
		Schema: ControllerCallSchemaID, CallSequence: 1,
		Operation: ControllerOperationVerifyEpoch, Payload: trimFrame(arm),
	})
	if err != nil {
		return EpochCheck{}, err
	}
	call, err := DecodeControllerCall(bytes.NewReader(frame))
	if err != nil {
		return EpochCheck{}, err
	}
	return call.VerifyEpoch.Check, nil
}
