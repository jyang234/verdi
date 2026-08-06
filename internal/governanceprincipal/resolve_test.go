package governanceprincipal

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// readerFunc is the hermetic fake TrustFactReader used across resolver
// tests.
type readerFunc func(ctx context.Context, source TrustSource, claim PrincipalClaim) (TrustFact, error)

func (f readerFunc) ReadTrustFact(ctx context.Context, source TrustSource, claim PrincipalClaim) (TrustFact, error) {
	return f(ctx, source, claim)
}

const testDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// staticFact returns a reader that always reports fact.
func staticFact(fact TrustFact) TrustFactReader {
	return readerFunc(func(context.Context, TrustSource, PrincipalClaim) (TrustFact, error) {
		return fact, nil
	})
}

func githubFact(subjects ...string) TrustFact {
	return TrustFact{
		SourceID:       "github",
		SourceKind:     TrustSourceForge,
		Subjects:       subjects,
		EvidenceDigest: testDigest,
		Available:      true,
		Valid:          true,
	}
}

func TestResolveAuthenticated(t *testing.T) {
	profile := mustDecode(t, profileYAML())
	var gotSource TrustSource
	var gotClaim PrincipalClaim
	r := NewResolver(readerFunc(func(_ context.Context, source TrustSource, claim PrincipalClaim) (TrustFact, error) {
		gotSource, gotClaim = source, claim
		return githubFact("user-123", "user-456"), nil
	}))

	claim := PrincipalClaim{TrustSource: "github", Subject: "user-123"}
	res, err := r.Resolve(context.Background(), profile, claim)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if gotSource.ID != "github" || gotSource.Kind != TrustSourceForge {
		t.Errorf("reader received source %+v, want github/forge", gotSource)
	}
	if gotClaim != claim {
		t.Errorf("reader received claim %+v, want %+v", gotClaim, claim)
	}
	if res.State != ResolutionAuthenticated {
		t.Errorf("State = %q, want %q", res.State, ResolutionAuthenticated)
	}
	if res.Claim != claim {
		t.Errorf("Claim = %+v, want original claim %+v", res.Claim, claim)
	}
	want, err := CanonicalPrincipalID("github", "user-123")
	if err != nil {
		t.Fatalf("CanonicalPrincipalID: %v", err)
	}
	if res.PrincipalID != want {
		t.Errorf("PrincipalID = %q, want %q", res.PrincipalID, want)
	}
	if len(res.Witnesses) != 1 {
		t.Fatalf("Witnesses = %+v, want exactly one", res.Witnesses)
	}
	w := res.Witnesses[0]
	if w.Code != ReasonTrustSubjectVerified || w.SourceID != "github" || w.EvidenceDigest != testDigest {
		t.Errorf("witness = %+v, want code %q source github digest %q", w, ReasonTrustSubjectVerified, testDigest)
	}
}

func TestResolveVerdicts(t *testing.T) {
	tests := []struct {
		name      string
		claim     PrincipalClaim
		fact      TrustFact
		wantState ResolutionState
		wantCode  string
	}{
		{
			"forbidden trust source",
			PrincipalClaim{TrustSource: "bitbucket", Subject: "user-123"},
			TrustFact{}, // reader must not be consulted
			ResolutionViolated,
			ReasonTrustSourceForbidden,
		},
		{
			"unavailable evidence",
			PrincipalClaim{TrustSource: "github", Subject: "user-123"},
			TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Available: false, Valid: false, Reason: "forge api unreachable"},
			ResolutionUnproven,
			ReasonTrustEvidenceUnavailable,
		},
		{
			"invalid evidence",
			PrincipalClaim{TrustSource: "github", Subject: "user-123"},
			TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Available: true, Valid: false, EvidenceDigest: testDigest, Reason: "signature did not verify"},
			ResolutionViolated,
			ReasonTrustEvidenceInvalid,
		},
		{
			"subject mismatch",
			PrincipalClaim{TrustSource: "github", Subject: "user-123"},
			githubFact("user-456", "user-789"),
			ResolutionViolated,
			ReasonTrustSubjectMismatch,
		},
	}
	profile := mustDecode(t, profileYAML())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewResolver(staticFact(tt.fact))
			res, err := r.Resolve(context.Background(), profile, tt.claim)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if res.State != tt.wantState {
				t.Errorf("State = %q, want %q", res.State, tt.wantState)
			}
			if res.PrincipalID != "" {
				t.Errorf("PrincipalID = %q, want empty outside authenticated", res.PrincipalID)
			}
			if len(res.Witnesses) != 1 || res.Witnesses[0].Code != tt.wantCode {
				t.Errorf("Witnesses = %+v, want one witness with code %q", res.Witnesses, tt.wantCode)
			}
			if res.Claim != tt.claim {
				t.Errorf("Claim = %+v, want %+v", res.Claim, tt.claim)
			}
		})
	}
}

// TestResolveForbiddenSourceSkipsReader: a claim naming a source outside
// the profile is judged without consulting the port.
func TestResolveForbiddenSourceSkipsReader(t *testing.T) {
	profile := mustDecode(t, profileYAML())
	r := NewResolver(readerFunc(func(context.Context, TrustSource, PrincipalClaim) (TrustFact, error) {
		t.Fatal("reader consulted for a forbidden trust source")
		return TrustFact{}, nil
	}))
	res, err := r.Resolve(context.Background(), profile, PrincipalClaim{TrustSource: "bitbucket", Subject: "u"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.State != ResolutionViolated {
		t.Errorf("State = %q, want %q", res.State, ResolutionViolated)
	}
}

func TestResolveOperationalErrors(t *testing.T) {
	profile := mustDecode(t, profileYAML())
	portErr := errors.New("adapter exploded")
	goodClaim := PrincipalClaim{TrustSource: "github", Subject: "user-123"}

	tests := []struct {
		name    string
		reader  TrustFactReader
		claim   PrincipalClaim
		wantSub string
		wantIs  error
	}{
		{"port error", readerFunc(func(context.Context, TrustSource, PrincipalClaim) (TrustFact, error) {
			return TrustFact{}, portErr
		}), goodClaim, "adapter exploded", portErr},
		{"malformed claim: empty subject", staticFact(githubFact("user-123")), PrincipalClaim{TrustSource: "github", Subject: ""}, "subject", nil},
		{"malformed claim: invalid source id", staticFact(githubFact("user-123")), PrincipalClaim{TrustSource: "GitHub", Subject: "user-123"}, "invalid id", nil},
		{"malformed claim: invalid utf-8 subject", staticFact(githubFact("user-123")), PrincipalClaim{TrustSource: "github", Subject: "\xff"}, "UTF-8", nil},
		{"fact source mismatch", staticFact(TrustFact{SourceID: "gitlab", SourceKind: TrustSourceForge, Subjects: []string{"user-123"}, EvidenceDigest: testDigest, Available: true, Valid: true}), goodClaim, "source", nil},
		{"fact kind mismatch", staticFact(TrustFact{SourceID: "github", SourceKind: TrustSourceOwnership, Subjects: []string{"user-123"}, EvidenceDigest: testDigest, Available: true, Valid: true}), goodClaim, "kind", nil},
		{"unavailable with subjects", staticFact(TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Subjects: []string{"user-123"}, Available: false, Reason: "down"}), goodClaim, "subjects", nil},
		{"unavailable without reason", staticFact(TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Available: false}), goodClaim, "reason", nil},
		{"unavailable but valid", staticFact(TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Available: false, Valid: true, Reason: "down"}), goodClaim, "valid", nil},
		{"unavailable with digest", staticFact(TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Available: false, Reason: "down", EvidenceDigest: testDigest}), goodClaim, "digest", nil},
		{"available without digest", staticFact(TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Subjects: []string{"user-123"}, Available: true, Valid: true}), goodClaim, "digest", nil},
		{"available with malformed digest", staticFact(TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Subjects: []string{"user-123"}, Available: true, Valid: true, EvidenceDigest: "sha256:xyz"}), goodClaim, "digest", nil},
		{"invalid without reason", staticFact(TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Available: true, Valid: false, EvidenceDigest: testDigest}), goodClaim, "reason", nil},
		{"empty subject in fact", staticFact(TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Subjects: []string{""}, Available: true, Valid: true, EvidenceDigest: testDigest}), goodClaim, "subject", nil},
		{"duplicate subject in fact", staticFact(TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Subjects: []string{"user-123", "user-123"}, Available: true, Valid: true, EvidenceDigest: testDigest}), goodClaim, "duplicate", nil},
		{"invalid utf-8 subject in fact", staticFact(TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Subjects: []string{"user-\xff"}, Available: true, Valid: true, EvidenceDigest: testDigest}), goodClaim, "UTF-8", nil},
		{"nil reader", nil, goodClaim, "reader", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r Resolver
			if tt.reader != nil {
				r = NewResolver(tt.reader)
			}
			_, err := r.Resolve(context.Background(), profile, tt.claim)
			if err == nil {
				t.Fatalf("Resolve: expected operational error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Errorf("errors.Is(err, port error) = false, want wrapped port error")
			}
		})
	}
}

// TestResolveCancellationPropagates: a canceled context surfaces through
// the fact reader as a wrapped operational error.
func TestResolveCancellationPropagates(t *testing.T) {
	profile := mustDecode(t, profileYAML())
	r := NewResolver(readerFunc(func(ctx context.Context, _ TrustSource, _ PrincipalClaim) (TrustFact, error) {
		return TrustFact{}, ctx.Err()
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Resolve(ctx, profile, PrincipalClaim{TrustSource: "github", Subject: "user-123"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Resolve error = %v, want wrapped context.Canceled", err)
	}
}

// TestSortWitnesses: witness ordering is total over field content.
func TestSortWitnesses(t *testing.T) {
	shuffled := []Witness{
		{Code: "b", SourceID: "s2"},
		{Code: "a", SourceID: "s2", Detail: "y"},
		{Code: "a", SourceID: "s2", Detail: "x"},
		{Code: "a", SourceID: "s1"},
		{Code: "a", SourceID: "s2", EvidenceDigest: testDigest, Detail: "x"},
	}
	sortWitnesses(shuffled)
	want := []Witness{
		{Code: "a", SourceID: "s1"},
		{Code: "a", SourceID: "s2", Detail: "x"},
		{Code: "a", SourceID: "s2", Detail: "y"},
		{Code: "a", SourceID: "s2", EvidenceDigest: testDigest, Detail: "x"},
		{Code: "b", SourceID: "s2"},
	}
	if !reflect.DeepEqual(shuffled, want) {
		t.Errorf("sortWitnesses = %+v, want %+v", shuffled, want)
	}
}
