package artifact

import "testing"

func TestDecodeConflict_Happy(t *testing.T) {
	cases := map[string]string{
		"open":       "id: conflict/stale-decline-incident\nkind: conflict\ntitle: \"Prod incident contradicts ac-2\"\nstatus: open\nowners: [platform-team]\nlinks:\n  - { type: challenges, ref: spec/stale-decline }\n",
		"superseded": "id: conflict/stale-decline-incident\nkind: conflict\ntitle: \"Prod incident contradicts ac-2\"\nstatus: superseded\nowners: [platform-team]\nlinks:\n  - { type: challenges, ref: spec/stale-decline }\nfrozen: { at: 2026-06-01, commit: 3e91ab2 }\n",
		"dismissed":  "id: conflict/stale-decline-incident\nkind: conflict\ntitle: \"Prod incident contradicts ac-2\"\nstatus: dismissed\nowners: [platform-team]\nlinks:\n  - { type: challenges, ref: spec/stale-decline }\nfrozen: { at: 2026-06-01, commit: 3e91ab2 }\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeConflict([]byte(y)); err != nil {
				t.Fatalf("DecodeConflict: %v", err)
			}
		})
	}
}

func TestDecodeConflict_Negative(t *testing.T) {
	cases := map[string]string{
		"missing challenges link":      "id: conflict/foo\nkind: conflict\ntitle: Foo\nstatus: open\nowners: [x]\n",
		"frozen while open":            "id: conflict/foo\nkind: conflict\ntitle: Foo\nstatus: open\nowners: [x]\nlinks:\n  - { type: challenges, ref: spec/bar }\nfrozen: { at: 2026-01-01, commit: 3e91ab2 }\n",
		"missing frozen when resolved": "id: conflict/foo\nkind: conflict\ntitle: Foo\nstatus: dismissed\nowners: [x]\nlinks:\n  - { type: challenges, ref: spec/bar }\n",
		"unknown status":               "id: conflict/foo\nkind: conflict\ntitle: Foo\nstatus: closed\nowners: [x]\nlinks:\n  - { type: challenges, ref: spec/bar }\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeConflict([]byte(y)); err == nil {
				t.Fatalf("DecodeConflict(%s): want error, got nil", name)
			}
		})
	}
}
