package artifact

import "testing"

func TestDecodeWaiver_Happy(t *testing.T) {
	cases := map[string]string{
		"active":              "id: waiver/story-1482--ac-4\nkind: waiver\ntitle: \"Runtime check deferred\"\nstatus: active\nowners: [platform-team]\nreason: \"runtime probe not yet built (OQ-2)\"\nfrozen: { at: 2026-05-01, commit: 3e91ab2 }\n",
		"expired with expiry": "id: waiver/story-1482--ac-3\nkind: waiver\ntitle: \"Temporary golden gap\"\nstatus: expired\nowners: [platform-team]\nreason: \"golden flow pending\"\nexpiry: 2026-06-01\nfrozen: { at: 2026-05-01, commit: 3e91ab2 }\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeWaiver([]byte(y)); err != nil {
				t.Fatalf("DecodeWaiver: %v", err)
			}
		})
	}
}

func TestDecodeWaiver_Negative(t *testing.T) {
	cases := map[string]string{
		"missing reason": "id: waiver/story-1482--ac-4\nkind: waiver\ntitle: Foo\nstatus: active\nowners: [x]\nfrozen: { at: 2026-05-01, commit: 3e91ab2 }\n",
		"missing frozen": "id: waiver/story-1482--ac-4\nkind: waiver\ntitle: Foo\nstatus: active\nowners: [x]\nreason: bar\n",
		"bad expiry":     "id: waiver/story-1482--ac-4\nkind: waiver\ntitle: Foo\nstatus: active\nowners: [x]\nreason: bar\nexpiry: not-a-date\nfrozen: { at: 2026-05-01, commit: 3e91ab2 }\n",
		"unknown status": "id: waiver/story-1482--ac-4\nkind: waiver\ntitle: Foo\nstatus: pending\nowners: [x]\nreason: bar\nfrozen: { at: 2026-05-01, commit: 3e91ab2 }\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeWaiver([]byte(y)); err == nil {
				t.Fatalf("DecodeWaiver(%s): want error, got nil", name)
			}
		})
	}
}
