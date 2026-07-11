package artifact

import "testing"

const adrProposedYAML = `
id: adr/0003-retry-policy
kind: adr
title: "Retry policy for outbox publishers"
status: proposed
owners: [platform-team]
`

const adrAcceptedYAML = `
id: adr/0002-outbox-events
kind: adr
title: "Outbox pattern for domain events"
status: accepted
owners: [platform-team]
decided: 2026-04-01
frozen: { at: 2026-04-01, commit: 3e91ab2 }
`

func TestDecodeADR_Happy(t *testing.T) {
	cases := map[string]string{
		"proposed": adrProposedYAML,
		"accepted": adrAcceptedYAML,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			fm, err := DecodeADR([]byte(y))
			if err != nil {
				t.Fatalf("DecodeADR: %v", err)
			}
			if fm.ID == "" {
				t.Fatal("DecodeADR: empty id")
			}
		})
	}
}

func TestDecodeADR_Negative(t *testing.T) {
	cases := map[string]string{
		"unknown status":                "id: adr/0001-foo\nkind: adr\ntitle: Foo\nstatus: rejected\nowners: [x]\n",
		"missing decided when accepted": "id: adr/0001-foo\nkind: adr\ntitle: Foo\nstatus: accepted\nowners: [x]\nfrozen: { at: 2026-01-01, commit: 3e91ab2 }\n",
		"missing frozen when accepted":  "id: adr/0001-foo\nkind: adr\ntitle: Foo\nstatus: accepted\nowners: [x]\ndecided: 2026-01-01\n",
		"frozen while proposed":         "id: adr/0001-foo\nkind: adr\ntitle: Foo\nstatus: proposed\nowners: [x]\nfrozen: { at: 2026-01-01, commit: 3e91ab2 }\n",
		"decided while proposed":        "id: adr/0001-foo\nkind: adr\ntitle: Foo\nstatus: proposed\nowners: [x]\ndecided: 2026-01-01\n",
		"unknown field":                 "id: adr/0001-foo\nkind: adr\ntitle: Foo\nstatus: proposed\nowners: [x]\nbogus: true\n",
		"wrong kind":                    "id: adr/0001-foo\nkind: spec\ntitle: Foo\nstatus: proposed\nowners: [x]\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeADR([]byte(y)); err == nil {
				t.Fatalf("DecodeADR(%s): want error, got nil", name)
			}
		})
	}
}
