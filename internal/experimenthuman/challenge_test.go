package experimenthuman

import (
	"bytes"
	"strings"
	"testing"
)

const (
	testAcceptedHEAD = "0123456789abcdef0123456789abcdef01234567"
	testProposalHEAD = "89abcdef0123456789abcdef0123456789abcdef"
	testInputDigest  = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testHumanDigest  = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func testChallengeFacts() ChallengeFacts {
	return ChallengeFacts{
		Operation:      OperationProposeRegistration,
		Spike:          "spec/example",
		ExperimentID:   "comparison",
		AcceptedHEAD:   testAcceptedHEAD,
		ProposalHEAD:   testProposalHEAD,
		TrustSource:    "offline-human",
		InputDigest:    testInputDigest,
		ProposalDigest: testHumanDigest,
	}
}

func TestHumanChallengeCanonicalAndStrict(t *testing.T) {
	challenge, err := NewChallenge(testChallengeFacts())
	if err != nil {
		t.Fatalf("NewChallenge() error = %v", err)
	}
	got, err := challenge.Canonical()
	if err != nil {
		t.Fatalf("Canonical() error = %v", err)
	}
	want := []byte(`{"accepted_head":"0123456789abcdef0123456789abcdef01234567","experiment_id":"comparison","input_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","operation":"propose-registration","proposal_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","proposal_head":"89abcdef0123456789abcdef0123456789abcdef","schema":"verdi.experiment-human-challenge/v1","spike":"spec/example","trust_source":"offline-human"}` + "\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("Canonical() = %s, want %s", got, want)
	}
	decoded, err := DecodeChallenge(got)
	if err != nil {
		t.Fatalf("DecodeChallenge(canonical) error = %v", err)
	}
	if decoded != challenge {
		t.Fatalf("DecodeChallenge(canonical) = %+v, want %+v", decoded, challenge)
	}

	tests := []struct {
		name string
		raw  []byte
	}{
		{"unknown field", bytes.Replace(got, []byte(`{"accepted_head"`), []byte(`{"ambient_actor":"alice","accepted_head"`), 1)},
		{"duplicate field", bytes.Replace(got, []byte(`{"accepted_head"`), []byte(`{"accepted_head":"0123456789abcdef0123456789abcdef01234567","accepted_head"`), 1)},
		{"noncanonical whitespace", bytes.Replace(got, []byte("{"), []byte("{ "), 1)},
		{"missing newline", bytes.TrimSuffix(got, []byte("\n"))},
		{"unknown operation", bytes.Replace(got, []byte(`"propose-registration"`), []byte(`"approve"`), 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeChallenge(tt.raw); err == nil {
				t.Fatal("DecodeChallenge() error = nil, want strict/canonical rejection")
			}
		})
	}
}

func TestHumanChallengeClosedOperationsAndBindings(t *testing.T) {
	for _, operation := range []Operation{OperationReconcileDraft, OperationProposeRegistration, OperationProposeRatification} {
		facts := testChallengeFacts()
		facts.Operation = operation
		if _, err := NewChallenge(facts); err != nil {
			t.Errorf("NewChallenge(%q) error = %v", operation, err)
		}
	}
	bad := testChallengeFacts()
	bad.Operation = "approve"
	if _, err := NewChallenge(bad); err == nil {
		t.Fatal("NewChallenge(unknown operation) error = nil")
	}

	base, err := NewChallenge(testChallengeFacts())
	if err != nil {
		t.Fatal(err)
	}
	baseBytes, err := base.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name   string
		mutate func(*ChallengeFacts)
	}{
		{"accepted head", func(f *ChallengeFacts) { f.AcceptedHEAD = strings.Repeat("c", 40) }},
		{"proposal head", func(f *ChallengeFacts) { f.ProposalHEAD = strings.Repeat("d", 40) }},
		{"input", func(f *ChallengeFacts) { f.InputDigest = "sha256:" + strings.Repeat("e", 64) }},
		{"human artifacts", func(f *ChallengeFacts) { f.ProposalDigest = "sha256:" + strings.Repeat("f", 64) }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			facts := testChallengeFacts()
			tt.mutate(&facts)
			changed, err := NewChallenge(facts)
			if err != nil {
				t.Fatalf("NewChallenge() error = %v", err)
			}
			changedBytes, err := changed.Canonical()
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(changedBytes, baseBytes) {
				t.Fatalf("changing %s did not change canonical challenge", tt.name)
			}
		})
	}
}
