package humanartifact

import (
	"strings"
	"testing"
)

func TestContractValidate_Happy(t *testing.T) {
	c := Contract{Kind: "policy", Extensions: []ExtensionField{
		{Name: "priority", Type: ExtensionInt},
		{Name: "tags", Type: ExtensionStringList},
		{Name: "urgent_flag", Type: ExtensionBool},
		{Name: "kebab-name", Type: ExtensionString},
	}}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestContractValidate_NoExtensions(t *testing.T) {
	c := Contract{Kind: "story"}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// TestContractValidate_Negative is the anti-synthesis proof's own
// register: exact and case-folded kernel-name collisions, duplicate
// extension names (exact and case-folded), unknown types, and bad name
// grammar all fail closed (AC-1: "a template cannot remove, rename,
// retype, or synthesize kernel fields").
func TestContractValidate_Negative(t *testing.T) {
	tests := []struct {
		name    string
		c       Contract
		wantSub string
	}{
		{"empty kind", Contract{Kind: ""}, "kind is required"},
		{"unknown kind", Contract{Kind: "no-such-kind"}, "unrecognized artifact family"},
		{"kernel collision exact", Contract{Kind: "policy", Extensions: []ExtensionField{{Name: "title", Type: ExtensionString}}}, "shadows kernel field"},
		{"kernel collision case-fold", Contract{Kind: "policy", Extensions: []ExtensionField{{Name: "Title", Type: ExtensionString}}}, "shadows kernel field"},
		{"kernel collision spec kind", Contract{Kind: "adr", Extensions: []ExtensionField{{Name: "status", Type: ExtensionString}}}, "shadows kernel field"},
		{"kernel collision feature class", Contract{Kind: "feature", Extensions: []ExtensionField{{Name: "class", Type: ExtensionString}}}, "shadows kernel field"},
		{"kernel collision story custom", Contract{Kind: "story", Extensions: []ExtensionField{{Name: "custom", Type: ExtensionString}}}, "shadows kernel field"},
		{"kernel collision reaffirmation hash", Contract{Kind: "reaffirmation", Extensions: []ExtensionField{{Name: "hash", Type: ExtensionString}}}, "shadows kernel field"},
		{"kernel collision obligation for_kind", Contract{Kind: "obligation", Extensions: []ExtensionField{{Name: "for_kind", Type: ExtensionString}}}, "shadows kernel field"},
		{"kernel collision waiver reason", Contract{Kind: "waiver", Extensions: []ExtensionField{{Name: "reason", Type: ExtensionString}}}, "shadows kernel field"},
		{"duplicate extension exact", Contract{Kind: "policy", Extensions: []ExtensionField{{Name: "foo", Type: ExtensionString}, {Name: "foo", Type: ExtensionInt}}}, "duplicate"},
		{"duplicate extension case-fold", Contract{Kind: "policy", Extensions: []ExtensionField{{Name: "foo", Type: ExtensionString}, {Name: "FOO", Type: ExtensionInt}}}, "duplicate"},
		{"unknown type", Contract{Kind: "policy", Extensions: []ExtensionField{{Name: "foo", Type: ExtensionType("float")}}}, "unknown type"},
		{"empty name", Contract{Kind: "policy", Extensions: []ExtensionField{{Name: "", Type: ExtensionString}}}, "must not be empty"},
		{"bad grammar dot", Contract{Kind: "policy", Extensions: []ExtensionField{{Name: "foo.bar", Type: ExtensionString}}}, "identifier"},
		{"bad grammar leading dash", Contract{Kind: "policy", Extensions: []ExtensionField{{Name: "-foo", Type: ExtensionString}}}, "identifier"},
		{"bad grammar space", Contract{Kind: "policy", Extensions: []ExtensionField{{Name: "foo bar", Type: ExtensionString}}}, "identifier"},
		{"bad grammar double separator", Contract{Kind: "policy", Extensions: []ExtensionField{{Name: "foo--bar", Type: ExtensionString}}}, "identifier"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil error, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantSub)
			}
		})
	}
}

// TestContractFor_PreRegisteredKinds proves every constitution and
// spec-store kind AC-1 names has a registered contract with no
// extensions initially — the contract's existence and its kernel-
// collision validation are the deliverable; AI-assisted-spec-design
// later maps model descriptors onto it.
func TestContractFor_PreRegisteredKinds(t *testing.T) {
	kinds := []string{
		"feature", "story", "adr", "attestation", "waiver", "reaffirmation", "obligation",
		"policy", "policy-overlay", "policy-exemption",
	}
	for _, k := range kinds {
		t.Run(k, func(t *testing.T) {
			c, ok := ContractFor(k)
			if !ok {
				t.Fatalf("ContractFor(%q) ok = false, want true (pre-registered)", k)
			}
			if c.Kind != k {
				t.Fatalf("ContractFor(%q).Kind = %q", k, c.Kind)
			}
			if len(c.Extensions) != 0 {
				t.Fatalf("ContractFor(%q).Extensions = %v, want empty (no extensions initially)", k, c.Extensions)
			}
		})
	}
}

func TestContractFor_Unknown(t *testing.T) {
	if _, ok := ContractFor("no-such-kind"); ok {
		t.Fatal("ContractFor(unknown) ok = true, want false")
	}
}

// TestContractFor_ComponentSpec_Unrecognized pins register.go's own
// disclosed posture: AC-1 names "component specs" among the human-
// authored kinds an operating model resolves a scaffold for, but this
// package carries no kernel-field table or Contract for it yet (no
// shipped scaffold consumer — internal/model's canonical default
// registers only feature and story). Requesting "component" fails
// closed rather than silently admitting an unproven kernel boundary.
func TestContractFor_ComponentSpec_Unrecognized(t *testing.T) {
	if _, ok := ContractFor("component"); ok {
		t.Fatal(`ContractFor("component") ok = true, want false (no kernel table yet — see register.go's own doc comment)`)
	}
	if _, ok := KernelFields("component"); ok {
		t.Fatal(`KernelFields("component") ok = true, want false`)
	}
}

// TestRegisterContract_PanicsOnDuplicate proves the registry mirrors
// policyartifact.RegisterPayloadKind's posture: a kind registered twice
// is a programming error and panics — "policy" is already registered by
// this package's own init.
func TestRegisterContract_PanicsOnDuplicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("RegisterContract(duplicate kind) did not panic")
		}
	}()
	RegisterContract(Contract{Kind: "policy"})
}

// TestRegisterContract_PanicsOnInvalid proves an invalid contract
// (unrecognized kind, so Validate fails) also panics rather than
// registering silently.
func TestRegisterContract_PanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("RegisterContract(invalid contract) did not panic")
		}
	}()
	RegisterContract(Contract{Kind: "totally-unrecognized-kind-xyz"})
}
