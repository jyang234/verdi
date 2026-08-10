package journey

import (
	"strings"
	"testing"
)

const testSelectedProfileDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestPrincipalFactsValidate_ProfileAdoptionInvariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*PrincipalFacts)
		wantErr string
	}{
		{
			name: "adopted profile",
			mutate: func(pf *PrincipalFacts) {
				pf.ProfileAdopted = true
				pf.SelectedProfileID = "solo.default_1"
				pf.SelectedProfileDigest = testSelectedProfileDigest
				pf.Disclosures = []string{"authenticated principal resolution and profile-contributed requirements remain unproven"}
			},
		},
		{
			name:   "not adopted",
			mutate: func(*PrincipalFacts) {},
		},
		{
			name: "adopted without selected profile id",
			mutate: func(pf *PrincipalFacts) {
				pf.ProfileAdopted = true
				pf.SelectedProfileDigest = testSelectedProfileDigest
			},
			wantErr: "selected_profile_id",
		},
		{
			name: "adopted with malformed selected profile id",
			mutate: func(pf *PrincipalFacts) {
				pf.ProfileAdopted = true
				pf.SelectedProfileID = "Solo Default"
				pf.SelectedProfileDigest = testSelectedProfileDigest
			},
			wantErr: "selected_profile_id",
		},
		{
			name: "adopted without selected profile digest",
			mutate: func(pf *PrincipalFacts) {
				pf.ProfileAdopted = true
				pf.SelectedProfileID = "solo-default"
			},
			wantErr: "selected_profile_digest",
		},
		{
			name: "adopted with malformed selected profile digest",
			mutate: func(pf *PrincipalFacts) {
				pf.ProfileAdopted = true
				pf.SelectedProfileID = "solo-default"
				pf.SelectedProfileDigest = "sha256:ABCDEF"
			},
			wantErr: "selected_profile_digest",
		},
		{
			name: "not adopted with selected profile id",
			mutate: func(pf *PrincipalFacts) {
				pf.SelectedProfileID = "solo-default"
			},
			wantErr: "selected_profile_id",
		},
		{
			name: "not adopted with selected profile digest",
			mutate: func(pf *PrincipalFacts) {
				pf.SelectedProfileDigest = testSelectedProfileDigest
			},
			wantErr: "selected_profile_digest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pf := validRecord(t).Principals
			tt.mutate(&pf)

			err := pf.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("PrincipalFacts.validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("PrincipalFacts.validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestCanonicalDecode_AdoptedProfileRoundTrip(t *testing.T) {
	t.Parallel()

	r := validRecord(t)
	r.Principals.ProfileAdopted = true
	r.Principals.SelectedProfileID = "solo-default"
	r.Principals.SelectedProfileDigest = testSelectedProfileDigest
	r.Principals.Disclosures = []string{"authenticated principal resolution and profile-contributed requirements remain unproven"}

	data, err := Canonical(r)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Principals.SelectedProfileID != "solo-default" {
		t.Fatalf("SelectedProfileID = %q, want solo-default", decoded.Principals.SelectedProfileID)
	}
	if decoded.Principals.SelectedProfileDigest != testSelectedProfileDigest {
		t.Fatalf("SelectedProfileDigest = %q, want %q", decoded.Principals.SelectedProfileDigest, testSelectedProfileDigest)
	}

	data2, err := Canonical(decoded)
	if err != nil {
		t.Fatalf("Canonical(decoded): %v", err)
	}
	if string(data2) != string(data) {
		t.Fatalf("canonical round trip differs:\nfirst:  %s\nsecond: %s", data, data2)
	}
}
