package unsealedprovenance

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/policyartifact"
)

const (
	cutoff40 = "0123456789abcdef0123456789abcdef01234567"
	cutoff64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

// validPayloadYAML is the complete payload grammar of SI-255: every field
// present, two inventory entries sorted by id, and an optional cutoff.
const validPayloadYAML = `permitted: true
cap: 1
inventory:
  - id: alpha-work
    story: spec/alpha-work
    admitted_by: "governed change admitting alpha-work"
  - id: vatc-machine-projections
    story: spec/vatc-machine-projections
    admitted_by: "unsealed-provenance exemption ratification (SI-244)"
cutoff:
  commit: ` + cutoff40 + `
`

func validPayload() *Payload {
	return &Payload{
		Permitted: true,
		Cap:       1,
		Inventory: []InventoryEntry{
			{ID: "alpha-work", Story: "spec/alpha-work", AdmittedBy: "governed change admitting alpha-work"},
			{ID: "vatc-machine-projections", Story: "spec/vatc-machine-projections", AdmittedBy: "unsealed-provenance exemption ratification (SI-244)"},
		},
		Cutoff: &Cutoff{Commit: cutoff40},
	}
}

func replaceOnce(t *testing.T, s, old, replacement string) string {
	t.Helper()
	if !strings.Contains(s, old) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return strings.Replace(s, old, replacement, 1)
}

func TestDecodePayload_Accepts(t *testing.T) {
	withoutCutoff := func(t *testing.T) string {
		return replaceOnce(t, validPayloadYAML, "cutoff:\n  commit: "+cutoff40+"\n", "")
	}
	tests := []struct {
		name string
		yaml func(*testing.T) string
		want func() *Payload
	}{
		{
			name: "full payload with cutoff",
			yaml: func(*testing.T) string { return validPayloadYAML },
			want: validPayload,
		},
		{
			name: "full payload without cutoff",
			yaml: withoutCutoff,
			want: func() *Payload { p := validPayload(); p.Cutoff = nil; return p },
		},
		{
			name: "64-character cutoff object id",
			yaml: func(t *testing.T) string { return replaceOnce(t, validPayloadYAML, cutoff40, cutoff64) },
			want: func() *Payload { p := validPayload(); p.Cutoff = &Cutoff{Commit: cutoff64}; return p },
		},
		{
			name: "explicitly empty inventory decodes as non-nil empty",
			yaml: func(t *testing.T) string {
				return "permitted: false\ncap: 0\ninventory: []\n"
			},
			want: func() *Payload { return &Payload{Permitted: false, Cap: 0, Inventory: []InventoryEntry{}} },
		},
		{
			name: "permission off with a non-empty inventory is valid",
			yaml: func(t *testing.T) string {
				return replaceOnce(t, validPayloadYAML, "permitted: true", "permitted: false")
			},
			want: func() *Payload { p := validPayload(); p.Permitted = false; return p },
		},
		{
			name: "canonical JSON form decodes through the strict seam",
			yaml: func(*testing.T) string {
				return `{"cap":1,"cutoff":{"commit":"` + cutoff40 + `"},"inventory":[{"admitted_by":"governed change admitting alpha-work","id":"alpha-work","story":"spec/alpha-work"},{"admitted_by":"unsealed-provenance exemption ratification (SI-244)","id":"vatc-machine-projections","story":"spec/vatc-machine-projections"}],"permitted":true}`
			},
			want: validPayload,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodePayload([]byte(tc.yaml(t)))
			if err != nil {
				t.Fatalf("DecodePayload: %v", err)
			}
			if want := tc.want(); !reflect.DeepEqual(got, want) {
				t.Fatalf("DecodePayload = %#v, want %#v", got, want)
			}
			if got.Inventory == nil {
				t.Fatal("decoded inventory is nil; it must be non-nil")
			}
			if got.PayloadKind() != PayloadKind {
				t.Fatalf("PayloadKind = %q, want %q", got.PayloadKind(), PayloadKind)
			}
		})
	}
}

func TestDecodePayload_Refuses(t *testing.T) {
	v := validPayloadYAML
	entryA := "  - id: alpha-work\n    story: spec/alpha-work\n    admitted_by: \"governed change admitting alpha-work\"\n"
	tests := []struct {
		name string
		yaml func(*testing.T) string
		want string
	}{
		// Every required field missing, then null.
		{"missing permitted", func(t *testing.T) string { return replaceOnce(t, v, "permitted: true\n", "") }, "permitted is missing or null"},
		{"missing cap", func(t *testing.T) string { return replaceOnce(t, v, "cap: 1\n", "") }, "cap is missing or null"},
		{"missing inventory", func(t *testing.T) string {
			return "permitted: true\ncap: 1\n"
		}, "inventory is missing or null"},
		{"null permitted", func(t *testing.T) string { return replaceOnce(t, v, "permitted: true", "permitted: null") }, "permitted is missing or null"},
		{"null cap", func(t *testing.T) string { return replaceOnce(t, v, "cap: 1", "cap: ~") }, "cap is missing or null"},
		{"null inventory", func(*testing.T) string { return "permitted: true\ncap: 1\ninventory: null\n" }, "inventory is missing or null"},
		{"null cutoff", func(t *testing.T) string {
			return replaceOnce(t, v, "cutoff:\n  commit: "+cutoff40+"\n", "cutoff: null\n")
		}, "cutoff is null"},
		{"empty cutoff value", func(t *testing.T) string {
			return replaceOnce(t, v, "cutoff:\n  commit: "+cutoff40+"\n", "cutoff:\n")
		}, "cutoff is null"},
		{"missing cutoff commit", func(t *testing.T) string {
			return replaceOnce(t, v, "cutoff:\n  commit: "+cutoff40+"\n", "cutoff: {}\n")
		}, "cutoff.commit is missing or null"},
		{"null cutoff commit", func(t *testing.T) string { return replaceOnce(t, v, "commit: "+cutoff40, "commit: null") }, "cutoff.commit is missing or null"},
		{"missing entry id", func(t *testing.T) string { return replaceOnce(t, v, "  - id: alpha-work\n    story:", "  - story:") }, "inventory[0] requires id, story, and admitted_by"},
		{"missing entry story", func(t *testing.T) string { return replaceOnce(t, v, "    story: spec/alpha-work\n", "") }, "inventory[0] requires id, story, and admitted_by"},
		{"missing entry admitted_by", func(t *testing.T) string {
			return replaceOnce(t, v, "    admitted_by: \"governed change admitting alpha-work\"\n", "")
		}, "inventory[0] requires id, story, and admitted_by"},
		{"null entry id", func(t *testing.T) string { return replaceOnce(t, v, "id: alpha-work", "id: null") }, "inventory[0] requires id, story, and admitted_by"},
		{"null entry story", func(t *testing.T) string { return replaceOnce(t, v, "story: spec/alpha-work", "story: ~") }, "inventory[0] requires id, story, and admitted_by"},
		{"null entry admitted_by", func(t *testing.T) string {
			return replaceOnce(t, v, "admitted_by: \"governed change admitting alpha-work\"", "admitted_by: null")
		}, "inventory[0] requires id, story, and admitted_by"},
		{"null entry", func(t *testing.T) string { return replaceOnce(t, v, entryA, "  - null\n") }, "inventory[0] requires id, story, and admitted_by"},
		// Unknown keys at every level.
		{"unknown top-level key", func(*testing.T) string { return v + "extra: true\n" }, "field extra not found"},
		{"unknown entry key", func(t *testing.T) string {
			return replaceOnce(t, v, "    story: spec/alpha-work\n", "    story: spec/alpha-work\n    note: x\n")
		}, "field note not found"},
		{"unknown cutoff key", func(t *testing.T) string {
			return replaceOnce(t, v, "  commit: "+cutoff40+"\n", "  commit: "+cutoff40+"\n  at: now\n")
		}, "field at not found"},
		// Wrong scalar and collection types.
		{"string permitted", func(t *testing.T) string { return replaceOnce(t, v, "permitted: true", `permitted: "true"`) }, "permitted must be a YAML !!bool scalar"},
		{"YAML 1.1 yes permitted", func(t *testing.T) string { return replaceOnce(t, v, "permitted: true", "permitted: yes") }, "permitted must be a YAML !!bool scalar"},
		{"integer permitted", func(t *testing.T) string { return replaceOnce(t, v, "permitted: true", "permitted: 1") }, "permitted must be a YAML !!bool scalar"},
		{"mapping permitted", func(t *testing.T) string { return replaceOnce(t, v, "permitted: true", "permitted: {x: 1}") }, "permitted must be a YAML !!bool scalar"},
		{"fractional cap", func(t *testing.T) string { return replaceOnce(t, v, "cap: 1", "cap: 1.5") }, "cap must be a YAML !!int scalar"},
		{"integral float cap", func(t *testing.T) string { return replaceOnce(t, v, "cap: 1", "cap: 1.0") }, "cap must be a YAML !!int scalar"},
		{"string cap", func(t *testing.T) string { return replaceOnce(t, v, "cap: 1", `cap: "1"`) }, "cap must be a YAML !!int scalar"},
		{"overflowing cap", func(t *testing.T) string { return replaceOnce(t, v, "cap: 1", "cap: 99999999999999999999") }, "cap must be a YAML !!int scalar"},
		{"inventory mapping", func(*testing.T) string { return "permitted: true\ncap: 1\ninventory: {id: x}\n" }, "strict decode"},
		{"scalar cutoff", func(t *testing.T) string {
			return replaceOnce(t, v, "cutoff:\n  commit: "+cutoff40+"\n", "cutoff: "+cutoff40+"\n")
		}, "decode cutoff"},
		{"sequence cutoff", func(t *testing.T) string {
			return replaceOnce(t, v, "cutoff:\n  commit: "+cutoff40+"\n", "cutoff: ["+cutoff40+"]\n")
		}, "decode cutoff"},
		{"anchor refused by the dialect", func(t *testing.T) string { return replaceOnce(t, v, "cap: 1", "cap: &c 1") }, "anchor"},
		// Semantic refusals (Validate through DecodePayload).
		{"negative cap", func(t *testing.T) string { return replaceOnce(t, v, "cap: 1", "cap: -1") }, "cap -1 must be a non-negative integer"},
		{"duplicate ids", func(t *testing.T) string {
			return replaceOnce(t, v, "id: vatc-machine-projections", "id: alpha-work")
		}, `duplicate inventory id "alpha-work"`},
		{"duplicate stories", func(t *testing.T) string {
			return replaceOnce(t, v, "story: spec/vatc-machine-projections", "story: spec/alpha-work")
		}, `names spec ref "spec/alpha-work" already named by inventory id "alpha-work"`},
		{"unsorted entries", func(t *testing.T) string {
			return replaceOnce(t, v, "id: alpha-work", "id: zulu-work")
		}, "inventory must be sorted by id"},
		{"non-spec story ref", func(t *testing.T) string { return replaceOnce(t, v, "story: spec/alpha-work", "story: adr/alpha-work") }, "must be a spec ref"},
		{"pinned story ref", func(t *testing.T) string {
			return replaceOnce(t, v, "story: spec/alpha-work", "story: spec/alpha-work@abcdef1")
		}, "must not be pinned"},
		{"fragment story ref", func(t *testing.T) string {
			return replaceOnce(t, v, "story: spec/alpha-work", "story: spec/alpha-work#ac-1")
		}, "must not carry a fragment"},
		{"malformed story ref", func(t *testing.T) string { return replaceOnce(t, v, "story: spec/alpha-work", "story: spec/Alpha") }, "must be kebab-case"},
		{"empty story ref", func(t *testing.T) string { return replaceOnce(t, v, "story: spec/alpha-work", `story: ""`) }, "missing the '/'"},
		{"upper-case id", func(t *testing.T) string { return replaceOnce(t, v, "id: alpha-work", "id: Alpha-work") }, "must be kebab-case"},
		{"underscore id", func(t *testing.T) string { return replaceOnce(t, v, "id: alpha-work", "id: alpha_work") }, "must be kebab-case"},
		{"empty id", func(t *testing.T) string { return replaceOnce(t, v, "id: alpha-work", `id: ""`) }, "must be kebab-case"},
		{"blank admitted_by", func(t *testing.T) string {
			return replaceOnce(t, v, `admitted_by: "governed change admitting alpha-work"`, `admitted_by: "   "`)
		}, "admitted_by must name the governed change"},
		{"empty admitted_by", func(t *testing.T) string {
			return replaceOnce(t, v, `admitted_by: "governed change admitting alpha-work"`, `admitted_by: ""`)
		}, "admitted_by must name the governed change"},
		{"multi-line admitted_by", func(t *testing.T) string {
			return replaceOnce(t, v, `admitted_by: "governed change admitting alpha-work"`, `admitted_by: "first line\nsecond line"`)
		}, "admitted_by must be a single line"},
		{"carriage-return admitted_by", func(t *testing.T) string {
			return replaceOnce(t, v, `admitted_by: "governed change admitting alpha-work"`, `admitted_by: "first\rsecond"`)
		}, "admitted_by must be a single line"},
		{"short cutoff id", func(t *testing.T) string { return replaceOnce(t, v, "commit: "+cutoff40, "commit: abcdef1") }, "cutoff.commit"},
		{"upper-case cutoff id", func(t *testing.T) string {
			return replaceOnce(t, v, "commit: "+cutoff40, "commit: "+strings.ToUpper(cutoff40))
		}, "cutoff.commit"},
		{"non-hex cutoff id", func(t *testing.T) string {
			return replaceOnce(t, v, "commit: "+cutoff40, "commit: "+strings.Repeat("g", 40))
		}, "cutoff.commit"},
		{"50-character cutoff id", func(t *testing.T) string {
			return replaceOnce(t, v, "commit: "+cutoff40, "commit: "+strings.Repeat("a", 50))
		}, "cutoff.commit"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodePayload([]byte(tc.yaml(t)))
			if err == nil {
				t.Fatalf("DecodePayload accepted the input: %#v", got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("DecodePayload error = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

// TestPayloadValidate covers Validate on in-memory values, including the
// states strict decoding can never produce (nil payload, nil inventory).
func TestPayloadValidate(t *testing.T) {
	tests := []struct {
		name    string
		payload func() *Payload
		want    string // empty means valid
	}{
		{"valid", validPayload, ""},
		{"valid without cutoff", func() *Payload { p := validPayload(); p.Cutoff = nil; return p }, ""},
		{"valid empty inventory", func() *Payload { return &Payload{Inventory: []InventoryEntry{}} }, ""},
		{"nil payload", func() *Payload { return nil }, "payload is nil"},
		{"nil inventory", func() *Payload { p := validPayload(); p.Inventory = nil; return p }, "inventory is missing or null"},
		{"negative cap", func() *Payload { p := validPayload(); p.Cap = -3; return p }, "cap -3 must be a non-negative integer"},
		{"duplicate id", func() *Payload { p := validPayload(); p.Inventory[1].ID = "alpha-work"; return p }, "duplicate inventory id"},
		{"duplicate story", func() *Payload { p := validPayload(); p.Inventory[1].Story = "spec/alpha-work"; return p }, "already named by inventory id"},
		{"unsorted", func() *Payload {
			p := validPayload()
			p.Inventory[0], p.Inventory[1] = p.Inventory[1], p.Inventory[0]
			return p
		}, "inventory must be sorted by id"},
		{"bad cutoff", func() *Payload { p := validPayload(); p.Cutoff = &Cutoff{Commit: "abc"}; return p }, "cutoff.commit"},
		{"empty cutoff commit", func() *Payload { p := validPayload(); p.Cutoff = &Cutoff{}; return p }, "cutoff.commit"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.payload().Validate()
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Validate: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate error = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

// TestPayloadRegisteredAsSingleton proves the kind is registered from this
// package's init through the singleton seam, so policyauthority's
// duplicate-owner check refuses a second carrier.
func TestPayloadRegisteredAsSingleton(t *testing.T) {
	cardinality, ok := policyartifact.RegisteredPayloadCardinality(PayloadKind)
	if !ok || cardinality != policyartifact.PayloadSingleton {
		t.Fatalf("registered cardinality = %q, %t; want %q, true", cardinality, ok, policyartifact.PayloadSingleton)
	}
	if PayloadKind != "unsealed-provenance" {
		t.Fatalf("PayloadKind = %q", PayloadKind)
	}
}

// TestPayloadClonesThroughRegistry proves the canonical JSON form round-trips
// through the registered decoder, which is how policyauthority.Resolve
// copies every payload into the effective policy.
func TestPayloadClonesThroughRegistry(t *testing.T) {
	tests := []struct {
		name    string
		payload *Payload
	}{
		{"with cutoff", validPayload()},
		{"without cutoff", func() *Payload { p := validPayload(); p.Cutoff = nil; return p }()},
		{"empty inventory", &Payload{Inventory: []InventoryEntry{}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cloned, err := policyartifact.ClonePayload(PayloadKind, tc.payload)
			if err != nil {
				t.Fatalf("ClonePayload: %v", err)
			}
			typed, ok := cloned.(*Payload)
			if !ok {
				t.Fatalf("clone is %T, want *Payload", cloned)
			}
			if !reflect.DeepEqual(typed, tc.payload) {
				t.Fatalf("clone = %#v, want %#v", typed, tc.payload)
			}
			if len(typed.Inventory) > 0 {
				typed.Inventory[0].ID = "mutated"
				if tc.payload.Inventory[0].ID == "mutated" {
					t.Fatal("clone aliases the original inventory")
				}
			}
			if typed.Cutoff != nil {
				typed.Cutoff.Commit = "mutated"
				if tc.payload.Cutoff.Commit == "mutated" {
					t.Fatal("clone aliases the original cutoff")
				}
			}
		})
	}
	if _, err := policyartifact.ClonePayload(PayloadKind, &Payload{Cap: -1, Inventory: []InventoryEntry{}}); err == nil {
		t.Fatal("ClonePayload accepted an invalid payload")
	}
}

func TestPayloadEntry(t *testing.T) {
	p := validPayload()
	tests := []struct {
		name    string
		payload *Payload
		id      string
		want    InventoryEntry
		found   bool
	}{
		{"present", p, "vatc-machine-projections", p.Inventory[1], true},
		{"first", p, "alpha-work", p.Inventory[0], true},
		{"absent", p, "other-work", InventoryEntry{}, false},
		{"empty id", p, "", InventoryEntry{}, false},
		{"empty inventory", &Payload{Inventory: []InventoryEntry{}}, "alpha-work", InventoryEntry{}, false},
		{"nil payload", nil, "alpha-work", InventoryEntry{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := tc.payload.Entry(tc.id)
			if found != tc.found || got != tc.want {
				t.Fatalf("Entry(%q) = %#v, %t; want %#v, %t", tc.id, got, found, tc.want, tc.found)
			}
		})
	}
}
