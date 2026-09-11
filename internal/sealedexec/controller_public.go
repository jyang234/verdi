package sealedexec

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/contextowner"
)

// These adapters retain the incumbent domain validators during Flight 1.
// Only public owner arms cross FD3; the claims exception retains its wrapper.
func encodePublicControllerCallPayload(call ControllerCall) (json.RawMessage, error) {
	private, err := encodeControllerCallPayload(call)
	if err != nil || call.Operation == ControllerOperationResolveClaimMCP {
		return private, err
	}
	public, err := republishSchema(private, controllerRequestSchema(call.Operation), contextowner.RequestSchema(contextowner.Operation(call.Operation)))
	if err != nil {
		return nil, err
	}
	if _, err := contextowner.NewCall(contextowner.Operation(call.Operation), public); err != nil {
		return nil, err
	}
	return public, nil
}

func decodePublicControllerCallPayload(public json.RawMessage, call *ControllerCall) error {
	if call.Operation == ControllerOperationResolveClaimMCP {
		return decodeControllerCallPayload(public, call)
	}
	op := contextowner.Operation(call.Operation)
	if _, err := contextowner.NewCall(op, public); err != nil {
		return err
	}
	private, err := republishSchema(public, contextowner.RequestSchema(op), controllerRequestSchema(call.Operation))
	if err != nil {
		return err
	}
	return decodeControllerCallPayload(private, call)
}

func encodePublicControllerSuccessPayload(result ControllerResult) (json.RawMessage, error) {
	private, err := encodeControllerSuccessPayload(result)
	if err != nil || result.Operation == ControllerOperationResolveClaimMCP {
		return private, err
	}
	op := contextowner.Operation(result.Operation)
	public, err := republishSchema(private, controllerResultSchema(result.Operation), contextowner.ResultSchema(op))
	if err != nil {
		return nil, err
	}
	if err := contextowner.ValidateResultArm(op, public); err != nil {
		return nil, err
	}
	return public, nil
}

func decodePublicControllerSuccessPayload(public json.RawMessage, result *ControllerResult) error {
	if result.Operation == ControllerOperationResolveClaimMCP {
		return decodeControllerSuccessPayload(public, result)
	}
	op := contextowner.Operation(result.Operation)
	if err := contextowner.ValidateResultArm(op, public); err != nil {
		return err
	}
	private, err := republishSchema(public, contextowner.ResultSchema(op), controllerResultSchema(result.Operation))
	if err != nil {
		return err
	}
	return decodeControllerSuccessPayload(private, result)
}

// readControllerReply refuses a line as soon as it reaches the exclusive
// ceiling, including a peer that never sends LF or EOF. ReadSlice retains at
// most one bufio buffer beyond the accumulated prefix.
func readControllerReply(reader *bufio.Reader) ([]byte, error) {
	var frame []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(part) >= OwnerFrameCeiling-len(frame) {
			return nil, fmt.Errorf("controller reply reaches frame ceiling")
		}
		frame = append(frame, part...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return nil, err
		}
		return frame, nil
	}
}

func decodeControllerFrame(reader io.Reader, target any) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, OwnerFrameCeiling))
	if err != nil {
		return nil, err
	}
	if len(raw) >= OwnerFrameCeiling {
		return nil, fmt.Errorf("controller frame reaches frame ceiling")
	}
	return decodeStrict(bytes.NewReader(raw), target)
}

func boundedControllerFrame(frame []byte, err error) ([]byte, error) {
	if err != nil {
		return nil, err
	}
	if len(frame) >= OwnerFrameCeiling {
		return nil, fmt.Errorf("controller frame reaches frame ceiling")
	}
	return frame, nil
}
