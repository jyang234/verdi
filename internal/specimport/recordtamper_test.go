package specimport

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// tamperRecordOnBranch overwrites the committed import record at
// slug/digest on branch with tampered bytes, as an ORDINARY descendant
// commit — never through Apply/publish — then returns the checkout to
// main. Mirrors TestReadRecord_TamperedSource_ProvenanceMismatch's own
// out-of-band tamper pattern, applied to record.json itself rather than a
// retained source sidecar.
func tamperRecordOnBranch(t *testing.T, dir, branch, slug, digest string, tampered []byte) {
	t.Helper()
	runGitFixture(t, dir, "checkout", branch)
	if err := os.WriteFile(store.ImportRecordPath(dir, slug, digest), tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, dir, "add", "-A")
	runGitFixture(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "tamper with the import record")
	runGitFixture(t, dir, "checkout", "main")
}

// assertNoNewBranch captures refs/heads/ before/after calling retry and
// fails if anything changed — a refused retry (whatever its error) must
// never create a second branch, move the existing one, or otherwise mutate
// the repository (spec-import-contract.md Task 3 handoff: "never a created
// result and never a second branch").
func assertNoNewBranch(t *testing.T, dir string, retry func()) {
	t.Helper()
	before := runGitCapture(t, dir, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/")
	retry()
	after := runGitCapture(t, dir, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/")
	if before != after {
		t.Fatalf("refs/heads/ changed by a refused retry:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestReadRecordAndApplyRetry_TruncatedRecordJSON_NeverAPass proves a
// truncated (undecodable) record.json refuses ReadRecord with
// ErrImportRecordMissing, and that an identical Apply retry over that
// branch refuses as an ordinary collision (ErrTargetExists) — never a false
// already-created result, never a second branch.
func TestReadRecordAndApplyRetry_TruncatedRecordJSON_NeverAPass(t *testing.T) {
	repo := buildImportRepo(t)
	ctx := context.Background()
	slug := "sample-feature"
	result := applyEvidencedRequest(t, repo.Dir)

	original, err := gitx.Show(ctx, repo.Dir, result.Branch, store.ImportRecordRelPath(slug, result.PreviewDigest))
	if err != nil {
		t.Fatal(err)
	}
	if len(original) <= 10 {
		t.Fatalf("committed record.json is implausibly short (%d bytes); cannot construct a truncation fixture", len(original))
	}
	tamperRecordOnBranch(t, repo.Dir, result.Branch, slug, result.PreviewDigest, original[:10])

	if _, err := ReadRecord(ctx, repo.Dir, result.Branch, slug); !errors.Is(err, ErrImportRecordMissing) {
		t.Fatalf("ReadRecord(truncated record) = %v, want ErrImportRecordMissing", err)
	}

	svc := testService(t)
	assertNoNewBranch(t, repo.Dir, func() {
		_, err := svc.Apply(ctx, repo.Dir, evidencedRequest(), result.PreviewDigest, testAgent(t))
		if !errors.Is(err, ErrTargetExists) {
			t.Fatalf("Apply retry over a truncated record = %v, want ErrTargetExists", err)
		}
	})
}

// TestReadRecordAndApplyRetry_WrongSchemaRecord_NeverAPass proves a
// well-formed (decodable) but wrong-schema record.json refuses ReadRecord
// with ErrImportRecordMissing — invalid shape, not merely bad JSON — with
// the same Apply-retry guarantee as the truncated case.
func TestReadRecordAndApplyRetry_WrongSchemaRecord_NeverAPass(t *testing.T) {
	repo := buildImportRepo(t)
	ctx := context.Background()
	slug := "sample-feature"
	result := applyEvidencedRequest(t, repo.Dir)

	original, err := gitx.Show(ctx, repo.Dir, result.Branch, store.ImportRecordRelPath(slug, result.PreviewDigest))
	if err != nil {
		t.Fatal(err)
	}
	record, err := DecodeRecord(original)
	if err != nil {
		t.Fatal(err)
	}
	record.Schema = "verdi.spec-import-record/v2"
	tampered, err := encodeRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	tamperRecordOnBranch(t, repo.Dir, result.Branch, slug, result.PreviewDigest, tampered)

	if _, err := ReadRecord(ctx, repo.Dir, result.Branch, slug); !errors.Is(err, ErrImportRecordMissing) {
		t.Fatalf("ReadRecord(wrong schema) = %v, want ErrImportRecordMissing", err)
	}

	svc := testService(t)
	assertNoNewBranch(t, repo.Dir, func() {
		_, err := svc.Apply(ctx, repo.Dir, evidencedRequest(), result.PreviewDigest, testAgent(t))
		if !errors.Is(err, ErrTargetExists) {
			t.Fatalf("Apply retry over a wrong-schema record = %v, want ErrTargetExists", err)
		}
	})
}

// TestReadRecordAndApplyRetry_AlteredCandidateDigest_ProvenanceMismatch
// proves a decodable, correctly-shaped record whose candidate_digest has
// been altered out-of-band refuses ReadRecord with ErrProvenanceMismatch —
// never verified from the record's own self-reported hash alone — and that
// an Apply retry propagates the same provenance-mismatch refusal rather
// than a false already-created result or a second branch.
func TestReadRecordAndApplyRetry_AlteredCandidateDigest_ProvenanceMismatch(t *testing.T) {
	repo := buildImportRepo(t)
	ctx := context.Background()
	slug := "sample-feature"
	result := applyEvidencedRequest(t, repo.Dir)

	original, err := gitx.Show(ctx, repo.Dir, result.Branch, store.ImportRecordRelPath(slug, result.PreviewDigest))
	if err != nil {
		t.Fatal(err)
	}
	record, err := DecodeRecord(original)
	if err != nil {
		t.Fatal(err)
	}
	realDigest := record.CandidateDigest
	record.CandidateDigest = strings.Repeat("0", 64)
	if record.CandidateDigest == realDigest {
		t.Fatal("fixture bug: altered digest coincides with the real one")
	}
	tampered, err := encodeRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	tamperRecordOnBranch(t, repo.Dir, result.Branch, slug, result.PreviewDigest, tampered)

	if _, err := ReadRecord(ctx, repo.Dir, result.Branch, slug); !errors.Is(err, ErrProvenanceMismatch) {
		t.Fatalf("ReadRecord(altered candidate_digest) = %v, want ErrProvenanceMismatch", err)
	}

	svc := testService(t)
	assertNoNewBranch(t, repo.Dir, func() {
		_, err := svc.Apply(ctx, repo.Dir, evidencedRequest(), result.PreviewDigest, testAgent(t))
		if !errors.Is(err, ErrProvenanceMismatch) {
			t.Fatalf("Apply retry over an altered-digest record = %v, want ErrProvenanceMismatch", err)
		}
	})
}
