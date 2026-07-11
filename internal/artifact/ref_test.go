package artifact

import "testing"

func TestParseRef_Happy(t *testing.T) {
	cases := []struct {
		in   string
		want Ref
	}{
		{"spec/loansvc-stale-decline", Ref{Kind: KindSpec, Name: "loansvc-stale-decline"}},
		{"adr/0012-outbox-loansvc-events", Ref{Kind: KindADR, Name: "0012-outbox-loansvc-events"}},
		{"adr/0012-outbox-loansvc-events@3e91ab2", Ref{Kind: KindADR, Name: "0012-outbox-loansvc-events", Commit: "3e91ab2"}},
		{"diagram/loansvc-topology", Ref{Kind: KindDiagram, Name: "loansvc-topology"}},
		{"conflict/stale-decline-incident", Ref{Kind: KindConflict, Name: "stale-decline-incident"}},
		{"attestation/story-1482--ac-2", Ref{Kind: KindAttestation, Name: "story-1482--ac-2"}},
		{"waiver/story-1482--ac-4", Ref{Kind: KindWaiver, Name: "story-1482--ac-4"}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseRef(tc.in)
			if err != nil {
				t.Fatalf("ParseRef(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("ParseRef(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
			if got.String() != tc.in {
				t.Fatalf("round-trip: ParseRef(%q).String() = %q", tc.in, got.String())
			}
		})
	}
}

func TestParseRef_Negative(t *testing.T) {
	cases := []string{
		"",
		"spec",                       // missing '/'
		"spec/",                      // empty name
		"spec/Loansvc-Stale-Decline", // not kebab-case (uppercase)
		"spec/loansvc_stale_decline", // underscores not allowed
		"nope/some-name",             // unknown kind
		"spec/foo@",                  // trailing '@' with no commit
		"spec/foo@xyz",               // commit not hex
		"spec/foo@abc12",             // commit too short (5 hex chars < 7)
		"attestation/story-1482",     // attestation requires compound name
		"waiver/ac-2",                // waiver requires compound name
		"attestation/story-1482--",   // compound name missing second half
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseRef(in); err == nil {
				t.Fatalf("ParseRef(%q): want error, got nil", in)
			}
		})
	}
}

func TestParsePinnedRef_Happy(t *testing.T) {
	got, err := ParsePinnedRef("spec/loansvc-stale-decline@7f3c2a1")
	if err != nil {
		t.Fatalf("ParsePinnedRef: %v", err)
	}
	if !got.Pinned() || got.Commit != "7f3c2a1" {
		t.Fatalf("ParsePinnedRef = %+v, want pinned at 7f3c2a1", got)
	}
}

func TestParsePinnedRef_Negative(t *testing.T) {
	cases := []string{
		"spec/loansvc-stale-decline", // unpinned
		"not-a-ref",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if _, err := ParsePinnedRef(in); err == nil {
				t.Fatalf("ParsePinnedRef(%q): want error, got nil", in)
			}
		})
	}
}

func TestRef_Validate_Happy(t *testing.T) {
	r := Ref{Kind: KindSpec, Name: "loansvc-stale-decline", Commit: "3e91ab2"}
	if err := r.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestRef_Validate_Negative(t *testing.T) {
	cases := []Ref{
		{Kind: "bogus", Name: "foo"},
		{Kind: KindSpec, Name: ""},
		{Kind: KindSpec, Name: "Not-Kebab"},
		{Kind: KindAttestation, Name: "not-compound"},
		{Kind: KindSpec, Name: "foo", Commit: "not-hex!!"},
	}
	for _, r := range cases {
		t.Run(r.String(), func(t *testing.T) {
			if err := r.Validate(); err == nil {
				t.Fatalf("Validate(%+v): want error, got nil", r)
			}
		})
	}
}

func TestKind_Valid(t *testing.T) {
	for _, k := range []Kind{KindSpec, KindADR, KindDiagram, KindAttestation, KindWaiver, KindConflict} {
		if !k.Valid() {
			t.Fatalf("Kind(%q).Valid() = false, want true", k)
		}
	}
	if Kind("bogus").Valid() {
		t.Fatal(`Kind("bogus").Valid() = true, want false`)
	}
}

func TestRef_String_Unpinned(t *testing.T) {
	r := Ref{Kind: KindSpec, Name: "foo"}
	if got, want := r.String(), "spec/foo"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
