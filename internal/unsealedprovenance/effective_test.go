package unsealedprovenance

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

const payloadPolicyPath = ".verdi/policy/policies/unsealed-provenance.md"

// storeFixture reads testdata/store into a repository-rooted MapFS so each
// case can add or drop files without writing to disk.
func storeFixture(t *testing.T) fstest.MapFS {
	t.Helper()
	root := filepath.Join("testdata", "store")
	out := fstest.MapFS{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = &fstest.MapFile{Data: data, Mode: 0o644}
		return nil
	})
	if err != nil {
		t.Fatalf("reading store fixture: %v", err)
	}
	return out
}

func resolveFixture(t *testing.T, source fs.FS) *policyauthority.EffectivePolicy {
	t.Helper()
	store, err := policyauthority.LoadFromSource(source)
	if err != nil {
		t.Fatalf("LoadFromSource: %v", err)
	}
	effective, err := policyauthority.Resolve(store)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return effective
}

func fixturePayload() *Payload {
	return &Payload{
		Permitted: true,
		Cap:       1,
		Inventory: []InventoryEntry{{
			ID: "vatc-machine-projections", Story: "spec/vatc-machine-projections",
			AdmittedBy: "unsealed-provenance exemption ratification (SI-244)",
		}},
		Cutoff: &Cutoff{Commit: cutoff40},
	}
}

func TestEffective_StoreCarryingThePayload(t *testing.T) {
	ep := resolveFixture(t, storeFixture(t))
	payload, present, err := Effective(ep)
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if !present {
		t.Fatal("present = false for a store whose policy carries the payload")
	}
	if !reflect.DeepEqual(payload, fixturePayload()) {
		t.Fatalf("Effective = %#v, want %#v", payload, fixturePayload())
	}

	// The returned value never aliases the sealed effective policy.
	payload.Inventory[0].ID = "mutated"
	payload.Cutoff.Commit = "mutated"
	again, _, err := Effective(ep)
	if err != nil {
		t.Fatalf("Effective (again): %v", err)
	}
	if !reflect.DeepEqual(again, fixturePayload()) {
		t.Fatalf("mutating a returned payload changed the effective policy: %#v", again)
	}
	if _, err := ep.Digest(); err != nil {
		t.Fatalf("effective policy seal broken after Effective: %v", err)
	}
}

func TestEffective_StoreWithoutThePayloadReadsTheDefault(t *testing.T) {
	source := storeFixture(t)
	delete(source, payloadPolicyPath)
	payload, present, err := Effective(resolveFixture(t, source))
	if err != nil {
		t.Fatalf("Effective: %v", err)
	}
	if present {
		t.Fatal("present = true for a store that carries no unsealed-provenance payload")
	}
	want := &Payload{Permitted: false, Cap: 0, Inventory: []InventoryEntry{}}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("default = %#v, want %#v", payload, want)
	}
	if payload.Inventory == nil {
		t.Fatal("default inventory is nil; it must be non-nil")
	}
}

func TestEffective_TwoCarriersFailLoad(t *testing.T) {
	source := storeFixture(t)
	second := strings.Replace(string(source[payloadPolicyPath].Data), "id: policy/unsealed-provenance", "id: policy/second-carrier", 1)
	source[".verdi/policy/policies/second-carrier.md"] = &fstest.MapFile{Data: []byte(second), Mode: 0o644}
	_, err := policyauthority.LoadFromSource(source)
	if err == nil {
		t.Fatal("LoadFromSource accepted two policies carrying the singleton payload")
	}
	if !strings.Contains(err.Error(), `payload kind "unsealed-provenance" is registered by both policy`) {
		t.Fatalf("LoadFromSource error = %v, want the singleton duplicate-owner refusal", err)
	}
}

// TestEffective_DefensiveRefusals covers effective policies Load and Resolve
// never produce; Effective stays defensive rather than trusting its caller.
func TestEffective_DefensiveRefusals(t *testing.T) {
	carrier := func(id string, payload policyartifact.Payload) policyauthority.EffectivePolicyEntry {
		return policyauthority.EffectivePolicyEntry{PolicyID: id, Payloads: map[string]policyartifact.Payload{PayloadKind: payload}}
	}
	tests := []struct {
		name string
		ep   *policyauthority.EffectivePolicy
		want string
	}{
		{"nil effective policy", nil, "effective policy is nil"},
		{"two carriers", &policyauthority.EffectivePolicy{Policies: []policyauthority.EffectivePolicyEntry{
			carrier("policy/a", fixturePayload()), carrier("policy/b", fixturePayload()),
		}}, "carried by both policy policy/a and policy policy/b"},
		{"foreign payload type", &policyauthority.EffectivePolicy{Policies: []policyauthority.EffectivePolicyEntry{
			carrier("policy/a", &policyartifact.DesignAssistancePayload{Mode: "off"}),
		}}, "is not the registered typed payload"},
		{"nil typed payload", &policyauthority.EffectivePolicy{Policies: []policyauthority.EffectivePolicyEntry{
			carrier("policy/a", (*Payload)(nil)),
		}}, "is not the registered typed payload"},
		{"invalid payload", &policyauthority.EffectivePolicy{Policies: []policyauthority.EffectivePolicyEntry{
			carrier("policy/a", &Payload{Cap: -1, Inventory: []InventoryEntry{}}),
		}}, "cap -1 must be a non-negative integer"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, present, err := Effective(tc.ep)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Effective error = %v, want one containing %q", err, tc.want)
			}
			if payload != nil || present {
				t.Fatalf("Effective returned %#v, %t alongside an error", payload, present)
			}
		})
	}
}
