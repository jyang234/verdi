package artifact

import "testing"

func TestDecodeAttestation_Happy(t *testing.T) {
	y := "id: attestation/story-1482--ac-2\nkind: attestation\ntitle: \"AC-2 attested by QA lead\"\nowners: [qa-lead]\nfrozen: { at: 2026-05-01, commit: 3e91ab2 }\n"
	fm, err := DecodeAttestation([]byte(y))
	if err != nil {
		t.Fatalf("DecodeAttestation: %v", err)
	}
	if fm.Frozen == nil {
		t.Fatal("Frozen is nil")
	}
}

func TestDecodeAttestation_Negative(t *testing.T) {
	cases := map[string]string{
		"missing frozen": "id: attestation/story-1482--ac-2\nkind: attestation\ntitle: Foo\nowners: [x]\n",
		"status field present (attestations have none)": "id: attestation/story-1482--ac-2\nkind: attestation\ntitle: Foo\nowners: [x]\nstatus: active\nfrozen: { at: 2026-05-01, commit: 3e91ab2 }\n",
		"non-compound name":                             "id: attestation/story-1482\nkind: attestation\ntitle: Foo\nowners: [x]\nfrozen: { at: 2026-05-01, commit: 3e91ab2 }\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeAttestation([]byte(y)); err == nil {
				t.Fatalf("DecodeAttestation(%s): want error, got nil", name)
			}
		})
	}
}
