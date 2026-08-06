package governanceprincipal

import (
	"reflect"
	"strings"
	"testing"
)

func TestAttributionConstructors(t *testing.T) {
	id, err := CanonicalPrincipalID("github", "user-123")
	if err != nil {
		t.Fatalf("CanonicalPrincipalID: %v", err)
	}

	a, err := NewPrincipalAttribution(id)
	if err != nil {
		t.Fatalf("NewPrincipalAttribution: %v", err)
	}
	if a.PrincipalID != id || a.Unauthenticated {
		t.Errorf("principal attribution = %+v, want principal_id %q and unauthenticated=false", a, id)
	}
	if err := a.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}

	if _, err := NewPrincipalAttribution("not-a-principal-id"); err == nil {
		t.Errorf("NewPrincipalAttribution accepted a malformed principal ID")
	}
	if _, err := NewPrincipalAttribution(""); err == nil {
		t.Errorf("NewPrincipalAttribution accepted an empty principal ID")
	}

	u := NewUnauthenticatedAttribution()
	if u.PrincipalID != "" || !u.Unauthenticated {
		t.Errorf("unauthenticated attribution = %+v, want explicit unauthenticated marker only", u)
	}
	if err := u.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// TestAttributionExclusiveUnion: exactly one of principal_id and
// unauthenticated must be set.
func TestAttributionExclusiveUnion(t *testing.T) {
	id, err := CanonicalPrincipalID("github", "user-123")
	if err != nil {
		t.Fatalf("CanonicalPrincipalID: %v", err)
	}
	tests := []struct {
		name    string
		a       Attribution
		wantSub string
	}{
		{"neither set", Attribution{}, "exactly one"},
		{"both set", Attribution{PrincipalID: id, Unauthenticated: true}, "exactly one"},
		{"malformed principal id", Attribution{PrincipalID: "bare-string"}, "principal"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.a.Validate()
			if err == nil {
				t.Fatalf("Validate(%+v): expected error, got nil", tt.a)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

// TestAttributionFromResolution: genuine authenticated resolutions may
// carry a principal attribution; genuine violated or unproven resolutions
// only the explicit unauthenticated marker.
func TestAttributionFromResolution(t *testing.T) {
	id, err := CanonicalPrincipalID("github", "user-123")
	if err != nil {
		t.Fatalf("CanonicalPrincipalID: %v", err)
	}
	claim := PrincipalClaim{TrustSource: "github", Subject: "user-123"}

	a, err := AttributionFromResolution(authedRes(t, "user-123"))
	if err != nil {
		t.Fatalf("AttributionFromResolution(authenticated): %v", err)
	}
	if a.PrincipalID != id || a.Unauthenticated {
		t.Errorf("attribution = %+v, want principal_id %q", a, id)
	}

	for _, state := range []ResolutionState{ResolutionViolated, ResolutionUnproven} {
		a, err := AttributionFromResolution(failedRes(t, "user-123", state))
		if err != nil {
			t.Fatalf("AttributionFromResolution(%s): %v", state, err)
		}
		if a.PrincipalID != "" || !a.Unauthenticated {
			t.Errorf("attribution for %s resolution = %+v, want unauthenticated marker only", state, a)
		}
	}

	inconsistent := []struct {
		name string
		res  PrincipalResolution
	}{
		{"unknown state", PrincipalResolution{Claim: claim, State: "certified"}},
		{"authenticated without principal id", PrincipalResolution{Claim: claim, State: ResolutionAuthenticated}},
		{"authenticated with malformed principal id", PrincipalResolution{Claim: claim, State: ResolutionAuthenticated, PrincipalID: "bare"}},
		{"violated with principal id", PrincipalResolution{Claim: claim, State: ResolutionViolated, PrincipalID: id}},
	}
	for _, tt := range inconsistent {
		t.Run(tt.name, func(t *testing.T) {
			if a, err := AttributionFromResolution(tt.res); err == nil {
				t.Errorf("AttributionFromResolution(%+v) = %+v, want error", tt.res, a)
			}
		})
	}
}

// TestAttributionCarriesNoAuthority: the attribution record is
// permanently advisory — it has exactly the two union fields and no
// role, resolution state, authorization result, or trust-source
// algorithm to smuggle authority through (GLG DC-19).
func TestAttributionCarriesNoAuthority(t *testing.T) {
	typ := reflect.TypeOf(Attribution{})
	if typ.NumField() != 2 {
		t.Fatalf("Attribution has %d fields, want exactly the principal_id/unauthenticated union", typ.NumField())
	}
	if f := typ.Field(0); f.Name != "PrincipalID" {
		t.Errorf("field 0 = %s, want PrincipalID", f.Name)
	}
	if f := typ.Field(1); f.Name != "Unauthenticated" || f.Type.Kind() != reflect.Bool {
		t.Errorf("field 1 = %s (%s), want Unauthenticated bool", f.Name, f.Type)
	}
}
