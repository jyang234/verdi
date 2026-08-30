package sealedexec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
)

const ProviderInputSchemaID = "verdi.sealed-provider-input/v1"

type providerInputWire struct {
	Schema       string                   `json:"schema"`
	Instructions providerInstructionsWire `json:"instructions"`
	Data         []json.RawMessage        `json:"data"`
}

type providerInstructionsWire struct {
	InstructionProjection json.RawMessage `json:"instruction_projection"`
}

// EncodeProviderInput returns the one canonical typed stdin envelope shared by
// all sealed-execution adapters. It supersedes the former codex-only encoding.
func EncodeProviderInput(input ProviderInput) ([]byte, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("sealedexec: provider input: %w", err)
	}
	projection, err := EncodeInstructionProjection(input.Instructions.Projection)
	if err != nil {
		return nil, err
	}
	data := make([]json.RawMessage, len(input.Data))
	for i, item := range input.Data {
		encoded, err := contextcompile.EncodeDataItem(item)
		if err != nil {
			return nil, fmt.Errorf("sealedexec: provider input data[%d]: %w", i, err)
		}
		data[i] = json.RawMessage(bytes.TrimSuffix(encoded, []byte("\n")))
	}
	return canonjson.Marshal(providerInputWire{
		Schema:       ProviderInputSchemaID,
		Instructions: providerInstructionsWire{InstructionProjection: json.RawMessage(bytes.TrimSuffix(projection, []byte("\n")))},
		Data:         data,
	})
}

// DecodeProviderInput strict-decodes the shared sealed-provider-input envelope.
func DecodeProviderInput(reader io.Reader) (ProviderInput, error) {
	if reader == nil {
		return ProviderInput{}, errors.New("sealedexec: decode provider input: nil reader")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return ProviderInput{}, fmt.Errorf("sealedexec: read provider input: %w", err)
	}
	if _, err := DecodeUniqueJSONObject(raw); err != nil {
		return ProviderInput{}, fmt.Errorf("sealedexec: decode provider input: %w", err)
	}
	var wire providerInputWire
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return ProviderInput{}, fmt.Errorf("sealedexec: decode provider input: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ProviderInput{}, errors.New("sealedexec: decode provider input: trailing data")
	}
	if wire.Schema != ProviderInputSchemaID || wire.Instructions.InstructionProjection == nil || wire.Data == nil {
		return ProviderInput{}, errors.New("sealedexec: decode provider input: missing or wrong mandatory field")
	}
	projectionBytes, err := canonjson.Marshal(wire.Instructions.InstructionProjection)
	if err != nil {
		return ProviderInput{}, err
	}
	projection, err := DecodeInstructionProjection(bytes.NewReader(projectionBytes))
	if err != nil {
		return ProviderInput{}, err
	}
	items := make([]contextcompile.DataItem, len(wire.Data))
	for i, encoded := range wire.Data {
		itemBytes, err := canonjson.Marshal(encoded)
		if err != nil {
			return ProviderInput{}, err
		}
		items[i], err = contextcompile.DecodeDataItem(itemBytes)
		if err != nil {
			return ProviderInput{}, fmt.Errorf("sealedexec: decode provider input data[%d]: %w", i, err)
		}
	}
	input := ProviderInput{Instructions: InstructionAuthority{Projection: projection}, Data: items}
	canonical, err := EncodeProviderInput(input)
	if err != nil {
		return ProviderInput{}, err
	}
	if !bytes.Equal(canonical, raw) {
		return ProviderInput{}, errors.New("sealedexec: provider input is not canonical")
	}
	return input, nil
}
