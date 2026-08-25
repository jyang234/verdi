package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestObligationQualityLegacyCorpus is the exact-tree compatibility witness
// for the 282 frozen obligation artifacts present at the PR #290 adoption
// base. The digest binds every repository-relative path and byte. Every file
// must remain an absent-quality compatibility decode and must assess as legacy,
// never elaborated. The exact base contains no persisted unauthored sentinels;
// newly rendered sentinel-bearing scaffolds are covered by the writer tests.
func TestObligationQualityLegacyCorpus(t *testing.T) {
	const (
		wantFiles  = 282
		wantDigest = "sha256:0a0895886b1afbf03d5b1c08a7620f2301a052fa6c8326c323d3c37c890efb66"
	)
	root := filepath.Clean(filepath.Join("..", ".."))
	paths, err := filepath.Glob(filepath.Join(root, ".verdi", "obligations", "*", "*.md"))
	if err != nil {
		t.Fatalf("glob legacy obligations: %v", err)
	}
	h := sha256.New()
	legacyFiles := 0
	markerBodies := 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("relative path %s: %v", path, err)
		}
		rel = filepath.ToSlash(rel)
		fm, body, err := artifact.SplitFrontmatter(raw)
		if err != nil {
			t.Fatalf("split %s: %v", rel, err)
		}
		obligation, err := artifact.DecodeObligation(fm)
		if err != nil {
			t.Fatalf("decode %s: %v", rel, err)
		}
		if obligation.Quality != nil {
			continue
		}

		legacyFiles++
		_, _ = h.Write([]byte(rel))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(raw)
		_, _ = h.Write([]byte{0})
		if strings.Contains(string(body), UnauthoredObligationMarker) {
			markerBodies++
		}

		stem := strings.TrimSuffix(filepath.Base(path), ".md")
		separator := strings.LastIndex(stem, "--")
		if separator < 1 {
			t.Fatalf("legacy obligation path %s has no AC/kind separator", rel)
		}
		assessment, err := AssessObligation(context.Background(), ObligationAssessmentInput{
			StoreRoot: root,
			SpecName:  filepath.Base(filepath.Dir(path)),
			ACID:      stem[:separator],
			Kind:      artifact.EvidenceKind(stem[separator+2:]),
		})
		if err != nil {
			t.Fatalf("assess %s: %v", rel, err)
		}
		if assessment.StructuralState != ObligationLegacyUnelaborated {
			t.Fatalf("%s structural state = %q, want %q", rel, assessment.StructuralState, ObligationLegacyUnelaborated)
		}
	}
	if legacyFiles != wantFiles {
		t.Fatalf("legacy obligation files = %d, want %d", legacyFiles, wantFiles)
	}

	gotDigest := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if gotDigest != wantDigest {
		t.Fatalf("legacy obligation tree digest = %q, want %q", gotDigest, wantDigest)
	}
	if markerBodies != 0 {
		t.Fatalf("persisted unauthored marker bodies = %d, want exact-base fact 0", markerBodies)
	}
}
