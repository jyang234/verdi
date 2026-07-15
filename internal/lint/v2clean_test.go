package lint

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// This file is the "go run ./cmd/verdi lint exits 0 on the v2 fixture
// corpus" exit criterion: it builds the real examples/showcase v2 fixture
// files (the rung-4 supersession pair, the round-four object-model
// feature, its three stories/spike, the outcome attestation, and the
// reaffirmation) into ONE git-real, fully-linted repo.
//
// This is genuinely more work than reading the files verbatim: the v2
// fixtures cite two different families of commit SHA, neither of which
// this package's own fixturegit-built test repos can reproduce unchanged.
//
//   - loan-workflow / loan-workflow-v2 cite goldenShaA/goldenShaB
//     (internal/artifact/v2fixture_test.go's dedicated, UNCHAINED 2-layer
//     history — built from an empty repo, not after the v0 corpus). Since
//     a git commit's SHA depends on its parent, chaining that same content
//     after this package's corpus+setup layers produces DIFFERENT SHAs.
//     This test rebuilds that 2-layer sub-sequence itself, chained after
//     the v0 corpus (so it participates in the SAME repo everything else
//     lives in), and substitutes the freshly-computed SHAs for the golden
//     literals wherever they're cited.
//   - accepted-pending-build / borrower-update-* / the outcome attestation
//     cite "93ddc5bbbb398cf747151e1c466afb83114398df" — goldenHeads[2],
//     the v0 corpus's OWN layer-3 head (reused deliberately, per
//     corpus_test.go's goldenHeads comment) — which IS already real,
//     unchanged, once chained after the same v0 corpus layers.txt content
//     this package's buildLintRepo already reproduces. No substitution
//     needed for that family.
//
// accepted-pending-build's context: pin (adr/0002-outbox-events@<v0 layer
// 1 head>) is likewise already real for the same reason.

var (
	frozenLineRe = regexp.MustCompile(`(?m)^frozen:.*\n`)
)

// draftVariant strips a frozen: line and flips status: accepted-pending-build
// to status: draft, producing the pre-freeze draft content fixturegit needs
// to build the layer before the one that freezes it.
func draftVariant(content string) string {
	content = frozenLineRe.ReplaceAllString(content, "")
	return strings.Replace(content, "status: accepted-pending-build\n", "status: draft\n", 1)
}

// readV2CorpusFile reads a examples/showcase file (relative to this
// package's own examples/showcase, mirroring corpusDir).
func readV2CorpusFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(corpusDir, rel))
	if err != nil {
		t.Fatalf("reading corpus file %s: %v", rel, err)
	}
	return string(data)
}

const (
	goldenShaAToken = "b5117ecc69b6779ad75cde60d4aec206ece0950b"
	goldenShaBToken = "06a3f4cabb226fe9344e1645e27c344493b6b62b"
)

// buildV2FixtureCorpusRepo builds the full v2 fixture corpus (v0's 3 golden
// layers + this package's lint-test setup layer, then the rung-4
// supersession pair as two more layers, then a final layer with every
// remaining v2 fixture file) into one git-real repo, per this file's own
// doc comment.
func buildV2FixtureCorpusRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()

	base := parseCorpusLayers(t)
	base = append(base, setupLayer())

	v1Draft := draftVariant(readV2CorpusFile(t, ".verdi/specs/active/loan-workflow/spec.md"))
	layerA := fixturegit.Layer{
		Files:   map[string]string{".verdi/specs/active/loan-workflow/spec.md": v1Draft},
		Message: "v2 corpus: loan-workflow v1 draft",
	}
	repoA := fixturegit.Build(t, append(append([]fixturegit.Layer{}, base...), layerA))
	shaA := repoA.Heads[len(repoA.Heads)-1]

	v1Frozen := strings.Replace(readV2CorpusFile(t, ".verdi/specs/active/loan-workflow/spec.md"), goldenShaAToken, shaA, 1)
	v2Draft := draftVariant(readV2CorpusFile(t, ".verdi/specs/active/loan-workflow-v2/spec.md"))
	layerB := fixturegit.Layer{
		Files: map[string]string{
			".verdi/specs/active/loan-workflow/spec.md":    v1Frozen,
			".verdi/specs/active/loan-workflow-v2/spec.md": v2Draft,
		},
		Message: "v2 corpus: loan-workflow v1 frozen + loan-workflow-v2 draft",
	}
	repoB := fixturegit.Build(t, append(append([]fixturegit.Layer{}, base...), layerA, layerB))
	shaB := repoB.Heads[len(repoB.Heads)-1]

	sub := func(rel string) string {
		content := readV2CorpusFile(t, rel)
		content = strings.ReplaceAll(content, goldenShaAToken, shaA)
		content = strings.ReplaceAll(content, goldenShaBToken, shaB)
		return content
	}

	layerC := fixturegit.Layer{
		Files: map[string]string{
			".verdi/specs/active/loan-workflow-v2/spec.md":             sub(".verdi/specs/active/loan-workflow-v2/spec.md"),
			".verdi/specs/active/accepted-pending-build/spec.md":       sub(".verdi/specs/active/accepted-pending-build/spec.md"),
			".verdi/specs/active/accepted-pending-build/layout.json":   sub(".verdi/specs/active/accepted-pending-build/layout.json"),
			".verdi/specs/active/borrower-update-api/spec.md":          sub(".verdi/specs/active/borrower-update-api/spec.md"),
			".verdi/specs/active/borrower-update-mobile/spec.md":       sub(".verdi/specs/active/borrower-update-mobile/spec.md"),
			".verdi/specs/active/borrower-update-mobile-spike/spec.md": sub(".verdi/specs/active/borrower-update-mobile-spike/spec.md"),
			".verdi/attestations/accepted-pending-build/ac-1.md":       sub(".verdi/attestations/accepted-pending-build/ac-1.md"),
			".verdi/reaffirmations/jira-loan-1483/ac-1.md":             sub(".verdi/reaffirmations/jira-loan-1483/ac-1.md"),
		},
		Message: "v2 corpus: loan-workflow-v2 frozen + accepted-pending-build cluster + reaffirmation",
	}

	layers := append(append([]fixturegit.Layer{}, base...), layerA, layerB, layerC)
	repo := fixturegit.Build(t, layers)
	writeLoansvcFixture(t, repo.Dir)
	provisionMutableZone(t, repo.Dir)
	return repo
}

// TestV2FixtureCorpus_LintsClean is the "go run ./cmd/verdi lint exits 0
// on the v2 fixture corpus" exit criterion.
func TestV2FixtureCorpus_LintsClean(t *testing.T) {
	repo := buildV2FixtureCorpusRepo(t)
	findings, err := NewEngine().Run(context.Background(), repo.Dir, Context{}, Options{})
	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("v2 fixture corpus: got %d findings, want 0:\n%s", len(findings), findingsString(findings))
	}
}

// TestV2FixtureCorpus_BareClone_OnlyVL017Disclosures models a CI clone of a
// real repo carrying new-class specs: the mutable zone (data/mutable/) is
// never committed (01 §Zones), so on a bare clone VL-017 cannot prove the
// open-question check and reports it disclosed-unproven for each new-class
// spec. Adjudicated at W2 wave close: those reports are SeverityDisclosure
// notices — printed, never silent (constitution 2), but NOT verdict
// failures. This test proves the run is neither a vacuous green (findings
// are present) nor a red (every finding is a disclosure, so the CLI's
// exit-code decision — see runLintVerb — stays 0).
func TestV2FixtureCorpus_BareClone_OnlyVL017Disclosures(t *testing.T) {
	repo := buildV2FixtureCorpusRepo(t)
	if err := os.RemoveAll(filepath.Join(repo.Dir, ".verdi", "data", "mutable")); err != nil {
		t.Fatalf("removing mutable zone: %v", err)
	}

	findings, err := NewEngine().Run(context.Background(), repo.Dir, Context{}, Options{})
	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("bare clone produced 0 findings — a vacuous green; want VL-017 disclosed-unproven notices")
	}
	for _, f := range findings {
		if f.Rule != "VL-017" || f.Severity != SeverityDisclosure {
			t.Fatalf("unexpected finding on a bare clone (want only VL-017 disclosures): %s (severity %v)", f.String(), f.Severity)
		}
	}
}
