package constitutionimpact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
)

type inventoryDoc struct {
	Schema    *string        `json:"schema"`
	Consumers *[]consumerDoc `json:"consumers"`
}

type consumerDoc struct {
	Request            json.RawMessage `json:"request"`
	Environment        *string         `json:"environment"`
	GovernedOperations *[]string       `json:"governed_operations"`
}

// DecodeInventory strict-decodes canonical inventory bytes. Duplicate
// consumer identities are intentionally retained for coverage classification.
func DecodeInventory(data []byte) (Inventory, error) {
	var doc inventoryDoc
	if err := artifact.DecodeClosedJSON(data, &doc); err != nil {
		return Inventory{}, fmt.Errorf("constitutionimpact: decoding inventory: %w", err)
	}
	if doc.Schema == nil || doc.Consumers == nil {
		return Inventory{}, fmt.Errorf("constitutionimpact: decoding inventory: schema and consumers are mandatory")
	}
	if *doc.Schema != InventorySchema {
		return Inventory{}, fmt.Errorf("constitutionimpact: decoding inventory: schema %q, want %q", *doc.Schema, InventorySchema)
	}
	consumers := make([]Consumer, len(*doc.Consumers))
	for i, row := range *doc.Consumers {
		consumer, err := consumerFromDoc(row)
		if err != nil {
			return Inventory{}, fmt.Errorf("constitutionimpact: decoding inventory: consumers[%d]: %w", i, err)
		}
		consumers[i] = consumer
	}
	inventory := Inventory{Schema: InventorySchema, Consumers: consumers}
	canonical, err := EncodeInventory(inventory)
	if err != nil {
		return Inventory{}, fmt.Errorf("constitutionimpact: decoding inventory: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return Inventory{}, fmt.Errorf("constitutionimpact: decoding inventory: input bytes are not canonical")
	}
	return cloneInventory(inventory), nil
}

// EncodeInventory returns canonical bytes for a strict v1 inventory.
func EncodeInventory(inventory Inventory) ([]byte, error) {
	if inventory.Schema != InventorySchema {
		return nil, fmt.Errorf("constitutionimpact: encoding inventory: schema %q, want %q", inventory.Schema, InventorySchema)
	}
	if inventory.Consumers == nil {
		return nil, fmt.Errorf("constitutionimpact: encoding inventory: consumers must be non-nil")
	}
	rows := make([]consumerDoc, len(inventory.Consumers))
	for i, consumer := range inventory.Consumers {
		row, err := consumerDocFor(consumer)
		if err != nil {
			return nil, fmt.Errorf("constitutionimpact: encoding inventory: consumers[%d]: %w", i, err)
		}
		rows[i] = row
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, _ := consumerIdentityFromDoc(rows[i])
		right, _ := consumerIdentityFromDoc(rows[j])
		return left < right
	})
	schema := InventorySchema
	return canonjson.Marshal(inventoryDoc{Schema: &schema, Consumers: &rows})
}

// Identity returns the canonical digest of request, environment, and sorted
// governed operations together.
func (c Consumer) Identity() (string, error) {
	doc, err := consumerDocFor(c)
	if err != nil {
		return "", err
	}
	return consumerIdentityFromDoc(doc)
}

func consumerFromDoc(doc consumerDoc) (Consumer, error) {
	if len(doc.Request) == 0 || doc.Environment == nil || doc.GovernedOperations == nil {
		return Consumer{}, fmt.Errorf("request, environment, and governed_operations are mandatory")
	}
	requestBytes := append(append([]byte(nil), doc.Request...), '\n')
	request, err := contextcompile.DecodeRequest(requestBytes)
	if err != nil {
		return Consumer{}, fmt.Errorf("request: %w", err)
	}
	consumer := Consumer{
		Request:            request,
		Environment:        *doc.Environment,
		GovernedOperations: cloneStrings(*doc.GovernedOperations),
	}
	if err := validateConsumer(consumer); err != nil {
		return Consumer{}, err
	}
	return consumer, nil
}

func consumerDocFor(consumer Consumer) (consumerDoc, error) {
	if err := validateConsumer(consumer); err != nil {
		return consumerDoc{}, err
	}
	request, err := contextcompile.EncodeRequest(consumer.Request)
	if err != nil {
		return consumerDoc{}, fmt.Errorf("request: %w", err)
	}
	environment := consumer.Environment
	operations := cloneStrings(consumer.GovernedOperations)
	return consumerDoc{
		Request:            json.RawMessage(bytes.TrimSuffix(request, []byte{'\n'})),
		Environment:        &environment,
		GovernedOperations: &operations,
	}, nil
}

func consumerIdentityFromDoc(doc consumerDoc) (string, error) {
	return canonjson.Digest(struct {
		Request            json.RawMessage `json:"request"`
		Environment        string          `json:"environment"`
		GovernedOperations []string        `json:"governed_operations"`
	}{Request: doc.Request, Environment: *doc.Environment, GovernedOperations: *doc.GovernedOperations})
}

func validateConsumer(consumer Consumer) error {
	if _, err := contextcompile.EncodeRequest(consumer.Request); err != nil {
		return fmt.Errorf("request: %w", err)
	}
	if consumer.Environment == "" {
		return fmt.Errorf("environment must be non-empty")
	}
	if consumer.GovernedOperations == nil {
		return fmt.Errorf("governed_operations must be non-nil")
	}
	for i, operation := range consumer.GovernedOperations {
		if operation == "" {
			return fmt.Errorf("governed_operations[%d] must be non-empty", i)
		}
		if i > 0 && strings.Compare(consumer.GovernedOperations[i-1], operation) >= 0 {
			return fmt.Errorf("governed_operations must be sorted and unique")
		}
	}
	return nil
}

func cloneInventory(in Inventory) Inventory {
	out := Inventory{Schema: in.Schema, Consumers: make([]Consumer, len(in.Consumers))}
	for i := range in.Consumers {
		out.Consumers[i] = cloneConsumer(in.Consumers[i])
	}
	return out
}

func cloneConsumer(in Consumer) Consumer {
	out := in
	if in.Request.Expected != nil {
		expected := *in.Request.Expected
		out.Request.Expected = &expected
	}
	out.Request.Scope.Phases = cloneStrings(in.Request.Scope.Phases)
	out.Request.Scope.Environments = cloneStrings(in.Request.Scope.Environments)
	out.Request.Scope.Paths = cloneStrings(in.Request.Scope.Paths)
	out.Request.Scope.Refs = cloneStrings(in.Request.Scope.Refs)
	out.Request.Grants.Grants = make([]execworkspace.Grant, len(in.Request.Grants.Grants))
	for i, grant := range in.Request.Grants.Grants {
		out.Request.Grants.Grants[i] = grant
		out.Request.Grants.Grants[i].Paths = cloneStrings(grant.Paths)
		out.Request.Grants.Grants[i].Argv0s = cloneStrings(grant.Argv0s)
		if grant.Ceilings != nil {
			out.Request.Grants.Grants[i].Ceilings = make(map[string]int, len(grant.Ceilings))
			for name, limit := range grant.Ceilings {
				out.Request.Grants.Grants[i].Ceilings[name] = limit
			}
		}
	}
	out.GovernedOperations = cloneStrings(in.GovernedOperations)
	return out
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
