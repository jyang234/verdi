package artifact

import "testing"

func TestDecodeFlowmapLoose_Happy(t *testing.T) {
	data := []byte(`version: 1
service: svcfix
classify:
  busPublish:
    - "example.com/svcfix/internal/bus#Publish"
obligations:
  - name: audit-before-publish
    require: "example.com/svcfix/internal/audit#Write"
    before: "example.com/svcfix/internal/bus#Publish"
  - name: tx-must-close
    acquire: "example.com/svcfix/internal/store#BeginTx"
    release:
      - "example.com/svcfix/internal/store#Commit"
`)
	got, err := DecodeFlowmapLoose(data)
	if err != nil {
		t.Fatalf("DecodeFlowmapLoose: %v", err)
	}
	if got.Service != "svcfix" {
		t.Fatalf("Service = %q, want %q", got.Service, "svcfix")
	}
	want := []string{"audit-before-publish", "tx-must-close"}
	if len(got.Obligations) != len(want) {
		t.Fatalf("Obligations = %v, want %v", got.Obligations, want)
	}
	for i := range want {
		if got.Obligations[i] != want[i] {
			t.Fatalf("Obligations[%d] = %q, want %q", i, got.Obligations[i], want[i])
		}
	}
}

func TestDecodeFlowmapLoose_UnknownKeysIgnored(t *testing.T) {
	// The documented exception: fields verdi doesn't consume, or doesn't
	// even know about, must not fail decode — unlike DecodeStrict.
	data := []byte(`version: 1
service: svcfix
someFutureUpstreamField:
  nested: true
  list: [1, 2, 3]
obligations:
  - name: only-obligation
    someOtherField: whatever
`)
	got, err := DecodeFlowmapLoose(data)
	if err != nil {
		t.Fatalf("DecodeFlowmapLoose: %v", err)
	}
	if got.Service != "svcfix" {
		t.Fatalf("Service = %q, want %q", got.Service, "svcfix")
	}
	if len(got.Obligations) != 1 || got.Obligations[0] != "only-obligation" {
		t.Fatalf("Obligations = %v, want [only-obligation]", got.Obligations)
	}
}

func TestDecodeFlowmapLoose_NoServiceNoObligations(t *testing.T) {
	data := []byte(`version: 1
classify:
  db: ["database/sql"]
`)
	got, err := DecodeFlowmapLoose(data)
	if err != nil {
		t.Fatalf("DecodeFlowmapLoose: %v", err)
	}
	if got.Service != "" {
		t.Fatalf("Service = %q, want empty (caller defaults to dir name)", got.Service)
	}
	if len(got.Obligations) != 0 {
		t.Fatalf("Obligations = %v, want none", got.Obligations)
	}
}

func TestDecodeFlowmapLoose_Negative(t *testing.T) {
	cases := []struct {
		name string
		data string
	}{
		{"invalid yaml", "service: [unterminated"},
		{"anchor rejected", "service: &s svcfix\nother: *s\n"},
		{"alias rejected", "a: &x foo\nservice: *x\n"},
		{"custom tag rejected", "service: !mytag svcfix\n"},
		{"service not scalar", "service: {nested: true}\n"},
		{"obligations not sequence", "obligations: {name: foo}\n"},
		{"obligations entry not mapping", "obligations:\n  - just-a-string\n"},
		{"obligations entry missing name", "obligations:\n  - acquire: foo\n"},
		{"obligations entry empty name", "obligations:\n  - name: \"\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeFlowmapLoose([]byte(tc.data)); err == nil {
				t.Fatalf("DecodeFlowmapLoose(%s): want error, got nil", tc.name)
			}
		})
	}
}
