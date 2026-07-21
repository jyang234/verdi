package align

import (
	"encoding/base64"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

const hex64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd"

func TestVerifyIntegrity_NoJudgedExchange(t *testing.T) {
	fm := &artifact.DeviationFrontmatter{}
	if err := VerifyIntegrity(fm); err != nil {
		t.Fatalf("VerifyIntegrity(no judged exchange at all): %v", err)
	}
}

// TestVerifyIntegrity_IntegrityOnlyIsUnverifiable proves an older/hand-
// authored report carrying Integrity with no persisted JudgeIntegrity
// record (internal/artifact.DeviationFrontmatter.Validate allows this
// one-directionally) is honestly reported as unverifiable, not silently
// accepted.
func TestVerifyIntegrity_IntegrityOnlyIsUnverifiable(t *testing.T) {
	fm := &artifact.DeviationFrontmatter{Integrity: "sha256:" + hex64}
	if err := VerifyIntegrity(fm); err == nil {
		t.Fatal("VerifyIntegrity(integrity only, no judge_integrity): want error, got nil")
	}
}

func TestVerifyIntegrity_RoundTrips(t *testing.T) {
	stdin := []byte("the exact stdin bytes")
	rawResult := `{"findings":[{"id":"j-1","text":"t","confidence":0.9}]}`
	fm := &artifact.DeviationFrontmatter{
		Integrity: computeIntegrity(stdin, rawResult),
		JudgeIntegrity: &artifact.JudgeIntegrity{
			StdinB64:  base64.StdEncoding.EncodeToString(stdin),
			RawResult: rawResult,
		},
	}
	if err := VerifyIntegrity(fm); err != nil {
		t.Fatalf("VerifyIntegrity: %v", err)
	}
}

func TestComputeDigest_Deterministic(t *testing.T) {
	findings := []artifact.Finding{{ID: "f-1", Kind: artifact.FindingComputed, Text: "t"}}
	diffs := []ServiceBoundaryDiff{{Service: "loansvc", Skipped: true, SkipReason: "r"}}

	d1, err := ComputeDigest("abc123", findings, diffs)
	if err != nil {
		t.Fatalf("ComputeDigest: %v", err)
	}
	d2, err := ComputeDigest("abc123", findings, diffs)
	if err != nil {
		t.Fatalf("ComputeDigest: %v", err)
	}
	if d1 != d2 {
		t.Fatalf("ComputeDigest not deterministic: %q vs %q", d1, d2)
	}

	// Disposition/Note must not affect the digest — it is human state, not
	// a pinned input.
	dispositioned := []artifact.Finding{{ID: "f-1", Kind: artifact.FindingComputed, Text: "t", Disposition: artifact.FindingFixed}}
	d3, err := ComputeDigest("abc123", dispositioned, diffs)
	if err != nil {
		t.Fatalf("ComputeDigest: %v", err)
	}
	if d1 != d3 {
		t.Fatalf("ComputeDigest changed when only Disposition changed: %q vs %q", d1, d3)
	}

	// A different covers sha must change the digest.
	d4, err := ComputeDigest("def456", findings, diffs)
	if err != nil {
		t.Fatalf("ComputeDigest: %v", err)
	}
	if d1 == d4 {
		t.Fatal("ComputeDigest did not change when covers changed")
	}
}

// TestComputeDigest_CarriedFromExcluded is spec/finding-identity ac-2's
// digest-purity pin, at the exact mechanism level: CarriedFrom (like
// Disposition/Note above) is human/provenance state, never a pinned input —
// digestInput's own findingIdentityOnly type only ever copies (id, kind,
// text), so a finding carrying carried-from: produces the byte-identical
// digest a finding without it would. Proven directly against
// artifact.FindingComputed (the only kind ComputeDigest's own signature
// ever reads) rather than only at the higher VerifyDigest/FreezeInPlace
// level, so this pin cannot be satisfied by coincidence — it fails the
// moment digestInput ever starts reading the field.
func TestComputeDigest_CarriedFromExcluded(t *testing.T) {
	covers := "abc123"
	base := []artifact.Finding{{ID: "f-1", Kind: artifact.FindingComputed, Text: "t", Disposition: artifact.FindingFixed}}
	carried := []artifact.Finding{{ID: "f-1", Kind: artifact.FindingComputed, Text: "t", Disposition: artifact.FindingFixed, CarriedFrom: "0123456789abcdef0123456789abcdef01234567"}}

	d1, err := ComputeDigest(covers, base, nil)
	if err != nil {
		t.Fatalf("ComputeDigest: %v", err)
	}
	d2, err := ComputeDigest(covers, carried, nil)
	if err != nil {
		t.Fatalf("ComputeDigest: %v", err)
	}
	if d1 != d2 {
		t.Fatalf("ComputeDigest changed when only CarriedFrom was set: %q vs %q — carried-from must be excluded from the digest (VerifyDigest must stay unaffected on every existing frozen archive)", d1, d2)
	}
}
