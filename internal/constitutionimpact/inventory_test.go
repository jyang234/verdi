package constitutionimpact

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyartifact"
)

func TestInventoryCanonicalRoundTripAndIdentity(t *testing.T) {
	raw, err := os.ReadFile("testdata/inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := DecodeInventory(raw)
	if err != nil {
		t.Fatalf("DecodeInventory: %v", err)
	}
	encoded, err := EncodeInventory(inventory)
	if err != nil {
		t.Fatalf("EncodeInventory: %v", err)
	}
	if !bytes.Equal(encoded, raw) {
		t.Fatalf("round trip changed bytes\n got: %s\nwant: %s", encoded, raw)
	}
	localID, err := inventory.Consumers[0].Identity()
	if err != nil {
		t.Fatal(err)
	}
	production := cloneConsumer(inventory.Consumers[0])
	production.Environment = "production"
	production.Request.Scope.Environments = []string{"production"}
	productionID, err := production.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if localID == productionID {
		t.Fatal("consumer identity did not bind the declared environment")
	}

	// Decoder results do not alias their input or another returned field.
	inventory.Consumers[0].GovernedOperations[0] = "mutated"
	inventory.Consumers[0].Request.Scope.Phases[0] = "review"
	again, err := DecodeInventory(raw)
	if err != nil {
		t.Fatal(err)
	}
	if again.Consumers[0].GovernedOperations[0] != "make-verify" || again.Consumers[0].Request.Scope.Phases[0] != "build" {
		t.Fatal("DecodeInventory returned aliased nested values")
	}
}

func TestInventoryDeepCopiesExecutionGrantsAndPreservesExplicitEmptySets(t *testing.T) {
	consumer := testConsumer("spec/granted", "local")
	consumer.Request.Grants.Grants = []execworkspace.Grant{{Kind: execworkspace.GrantPathRead, Paths: []string{"docs"}}}
	consumer.GovernedOperations = []string{}
	raw, err := EncodeInventory(Inventory{Schema: InventorySchema, Consumers: []Consumer{consumer}})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"governed_operations":null`)) {
		t.Fatalf("explicit empty operation set encoded as null: %s", raw)
	}
	decoded, err := DecodeInventory(raw)
	if err != nil {
		t.Fatal(err)
	}
	planRows, _, err := plannedConsumers("test", decoded)
	if err != nil {
		t.Fatal(err)
	}
	plan := Plan{consumers: planRows}
	first := plan.Consumers()
	first[0].Request.Grants.Grants[0].Paths[0] = "mutated"
	again := plan.Consumers()
	if again[0].Request.Grants.Grants[0].Paths[0] != "docs" || again[0].GovernedOperations == nil {
		t.Fatal("consumer clone aliased grant payload or lost an explicit empty operation set")
	}
}

func TestDecodeInventoryStrictNegatives(t *testing.T) {
	raw, err := os.ReadFile("testdata/inventory.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"unknown top-level":      bytes.Replace(raw, []byte(`"schema":`), []byte(`"unknown":true,"schema":`), 1),
		"unknown nested request": bytes.Replace(raw, []byte(`"adapter":`), []byte(`"unknown":true,"adapter":`), 1),
		"duplicate key":          bytes.Replace(raw, []byte(`"environment":"local"`), []byte(`"environment":"local","environment":"local"`), 1),
		"explicit null":          bytes.Replace(raw, []byte(`"environment":"local"`), []byte(`"environment":null`), 1),
		"trailing data":          append(append([]byte(nil), raw...), []byte(`{}`)...),
		"noncanonical":           bytes.Replace(raw, []byte(`{"consumers":`), []byte(`{ "consumers":`), 1),
		"unsorted operations":    bytes.Replace(raw, []byte(`["make-verify"]`), []byte(`["z-action","a-action"]`), 1),
		"omitted operations":     bytes.Replace(raw, []byte(`,"governed_operations":["make-verify"]`), nil, 1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeInventory(input); err == nil {
				t.Fatal("DecodeInventory unexpectedly succeeded")
			}
		})
	}
}

func TestEncodeInventorySortsConsumersAndRefusesInvalidFields(t *testing.T) {
	first := testConsumer("spec/zeta", "local")
	second := testConsumer("spec/alpha", "local")
	encoded, err := EncodeInventory(Inventory{Schema: InventorySchema, Consumers: []Consumer{first, second}})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeInventory(encoded)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := decoded.Consumers[0].Identity()
	right, _ := decoded.Consumers[1].Identity()
	if left >= right {
		t.Fatalf("consumer identities are not sorted: %s >= %s", left, right)
	}

	bad := cloneConsumer(first)
	bad.GovernedOperations = []string{"z-action", "a-action"}
	if _, err := EncodeInventory(Inventory{Schema: InventorySchema, Consumers: []Consumer{bad}}); err == nil || !strings.Contains(err.Error(), "sorted and unique") {
		t.Fatalf("EncodeInventory unsorted operations error = %v", err)
	}
	if _, err := EncodeInventory(Inventory{Schema: InventorySchema}); err == nil {
		t.Fatal("EncodeInventory accepted nil consumers")
	}
}

func TestEncodeInventoryRefusesEnvironmentOutsideExactRequestScope(t *testing.T) {
	consumer := testConsumer("spec/registered", "local")
	consumer.Request.Scope.Environments = []string{"production"}
	if _, err := EncodeInventory(Inventory{Schema: InventorySchema, Consumers: []Consumer{consumer}}); err == nil || !strings.Contains(err.Error(), "environment") {
		t.Fatalf("EncodeInventory environment mismatch error = %v", err)
	}
}

// TestInventory_TemplateFieldRoundTrips proves the inventory's optional
// template record (ac-10, SI-204) round-trips through Encode/Decode,
// participates in the canonical encoding only when present (no "template"
// key when absent), and fails closed on a malformed record before the
// canonical-bytes check ever runs.
func TestInventory_TemplateFieldRoundTrips(t *testing.T) {
	rec := &policyartifact.TemplateRecord{Identity: "embedded:constitution-consumers.json", Digest: "sha256:" + strings.Repeat("b", 64)}
	enc, err := EncodeInventory(Inventory{Schema: InventorySchema, Consumers: []Consumer{}, Template: rec})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"consumers":[],"schema":"verdi.constitution-consumer-inventory/v1","template":{"digest":"sha256:` + strings.Repeat("b", 64) + `","identity":"embedded:constitution-consumers.json"}}` + "\n"
	if string(enc) != want {
		t.Fatalf("encoded = %s", enc)
	}
	dec, err := DecodeInventory(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Template == nil || *dec.Template != *rec {
		t.Fatalf("decoded template = %+v", dec.Template)
	}
	// Absent template still encodes exactly as before (no "template" key).
	enc0, _ := EncodeInventory(Inventory{Schema: InventorySchema, Consumers: []Consumer{}})
	if string(enc0) != `{"consumers":[],"schema":"verdi.constitution-consumer-inventory/v1"}`+"\n" {
		t.Fatalf("no-template encoding changed: %s", enc0)
	}
	// A bad record fails closed before the canonical-bytes check.
	if _, err := DecodeInventory([]byte(`{"consumers":[],"schema":"verdi.constitution-consumer-inventory/v1","template":{"digest":"nope","identity":"x"}}` + "\n")); err == nil {
		t.Fatal("bad template digest accepted")
	}
}
