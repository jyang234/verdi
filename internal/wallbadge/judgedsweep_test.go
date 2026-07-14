package wallbadge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// fakeResolver is a canned CoversResolver: it records the commit/relPath
// it was asked about and returns fixed values — the hermetic double for
// the git-backed implementation internal/workbench wires.
type fakeResolver struct {
	revision string
	ok       bool
	err      error

	gotCommit, gotRelPath string
}

func (f *fakeResolver) SpecDigestAtCommit(_ context.Context, commit, relPath string) (string, bool, error) {
	f.gotCommit, f.gotRelPath = commit, relPath
	return f.revision, f.ok, f.err
}

const sweepSpecName = "sweep-fixture"

// sweepSpecFM is the current spec's already-loaded frontmatter: two
// declared decisions — dc-3's set-comparison operand.
func sweepSpecFM() *artifact.SpecFrontmatter {
	return &artifact.SpecFrontmatter{
		Base: artifact.Base{ID: "spec/" + sweepSpecName},
		Decisions: []artifact.Decision{
			{ID: "dc-1", Text: "first decision"},
			{ID: "dc-2", Text: "second decision"},
		},
	}
}

const coversSHA = "96b44f049d11bfef37e017d5e8f7dcb462a58ef4"

// sweepReport renders a valid decision-conflict-report.md: one
// dispositioned judged finding, one undispositioned judged finding, and a
// sweep_provenance block scanning the ids named by scanned.
func sweepReport(scanned []string) string {
	scannedYAML := "[]"
	if len(scanned) > 0 {
		scannedYAML = "[" + strings.Join(scanned, ", ") + "]"
	}
	return `---
schema: verdi.decisionconflict/v1
covers: ` + coversSHA + `
findings:
  - { id: judged-dcf-1, kind: judged, text: "first sweep finding", disposition: no-conflict, note: "reviewed and cleared" }
  - { id: judged-dcf-2, kind: judged, text: "second sweep finding" }
sweep_provenance: { adr_corpus_digest: sha256:37517e5f3dc66819f61f5a7bb8ace1921282415f10551d2defa5c3eb0985b570, decisions_scanned: ` + scannedYAML + ` }
---
# Decision-conflict report
`
}

// writeSweepStore lays reportContent (when non-empty) into a temp store
// root at the spec's conventional report path.
func writeSweepStore(t *testing.T, reportContent string) string {
	t.Helper()
	root := t.TempDir()
	if reportContent == "" {
		return root
	}
	dir := filepath.Join(root, ".verdi", "specs", "active", sweepSpecName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "decision-conflict-report.md"), []byte(reportContent), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const wallRevision = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

func fullScan() []string {
	return []string{"spec/" + sweepSpecName + "#dc-1", "spec/" + sweepSpecName + "#dc-2"}
}

func TestJudgedSweepBadge_NoReport_NoChipNoDisclosure(t *testing.T) {
	root := writeSweepStore(t, "")
	rec, disclosure, err := JudgedSweepBadge(context.Background(), root, sweepSpecName, wallRevision, sweepSpecFM(), &fakeResolver{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec != nil {
		t.Errorf("a spec with no report must get no chip (dc-2), got %+v", rec)
	}
	if disclosure != "" {
		t.Errorf("absence of a sweep is not a finding: want no disclosure, got %q", disclosure)
	}
}

func TestJudgedSweepBadge_UndecodableReport_DisclosedNeverSilent(t *testing.T) {
	tests := []struct {
		name, content string
	}{
		{"not an artifact at all", "just prose, no frontmatter\n"},
		{"wrong schema", "---\nschema: verdi.other/v1\ncovers: " + coversSHA + "\nfindings: []\n---\n"},
		{"unknown field", "---\nschema: verdi.decisionconflict/v1\ncovers: " + coversSHA + "\nfindings: []\nbogus: 1\n---\n"},
		{"invalid covers", "---\nschema: verdi.decisionconflict/v1\ncovers: not-a-sha\nfindings: []\n---\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := writeSweepStore(t, tc.content)
			rec, disclosure, err := JudgedSweepBadge(context.Background(), root, sweepSpecName, wallRevision, sweepSpecFM(), &fakeResolver{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec != nil {
				t.Errorf("an unreadable report must never chip, got %+v", rec)
			}
			if !strings.Contains(disclosure, "judged findings are disclosed-unproven") {
				t.Errorf("want a disclosed-unproven disclosure, got %q", disclosure)
			}
		})
	}
}

func TestJudgedSweepBadge_FreshCompleteReport(t *testing.T) {
	content := sweepReport(fullScan())
	root := writeSweepStore(t, content)
	resolver := &fakeResolver{revision: wallRevision, ok: true}

	rec, disclosure, err := JudgedSweepBadge(context.Background(), root, sweepSpecName, wallRevision, sweepSpecFM(), resolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if disclosure != "" {
		t.Fatalf("unexpected disclosure: %q", disclosure)
	}
	if rec == nil {
		t.Fatal("want a case-file chip, got none")
	}

	if rec.Source != "align:judged-sweep" {
		t.Errorf("Source = %q, want align:judged-sweep", rec.Source)
	}
	if rec.Target != "" {
		t.Errorf("the judged chip is a CASE-FILE chip (dc-2): Target = %q, want empty", rec.Target)
	}
	if rec.Label != "2 judged findings" {
		t.Errorf("Label = %q, want \"2 judged findings\"", rec.Label)
	}

	wantInputs := []InputRecord{
		{Name: "covers", Path: ".verdi/specs/active/sweep-fixture/spec.md", Revision: coversSHA},
		{Name: "decision-conflict-report", Path: ".verdi/specs/active/sweep-fixture/decision-conflict-report.md", Revision: digestOf([]byte(content))},
	}
	if !reflect.DeepEqual(rec.Inputs, wantInputs) {
		t.Errorf("Inputs = %+v, want %+v", rec.Inputs, wantInputs)
	}

	wantRecords := []string{
		"judged-dcf-1 [no-conflict] first sweep finding — note: reviewed and cleared",
		"judged-dcf-2 [undispositioned] second sweep finding",
	}
	if !reflect.DeepEqual(rec.Records, wantRecords) {
		t.Errorf("Records = %q, want %q", rec.Records, wantRecords)
	}

	wantProvenance := []string{
		"sweep covers " + coversSHA,
		"adr_corpus_digest sha256:37517e5f3dc66819f61f5a7bb8ace1921282415f10551d2defa5c3eb0985b570",
		"decisions_scanned: spec/sweep-fixture#dc-1, spec/sweep-fixture#dc-2",
	}
	if !reflect.DeepEqual(rec.Provenance, wantProvenance) {
		t.Errorf("Provenance = %q, want %q", rec.Provenance, wantProvenance)
	}

	// Fresh and complete: NO mismatch lines at all (the ac-3--behavioral
	// obligation's fixture (a)).
	if len(rec.Disclosures) != 0 {
		t.Errorf("a fresh, complete sweep must wear no mismatch line, got %q", rec.Disclosures)
	}

	// The comparison consulted the pinned covers sha against the spec's
	// own store path — never some other operand.
	if resolver.gotCommit != coversSHA || resolver.gotRelPath != ".verdi/specs/active/sweep-fixture/spec.md" {
		t.Errorf("resolver asked about (%q, %q), want (covers sha, the spec's own path)", resolver.gotCommit, resolver.gotRelPath)
	}
}

func TestJudgedSweepBadge_ComparisonDisclosures(t *testing.T) {
	tests := []struct {
		name     string
		scanned  []string
		resolver *fakeResolver
		want     []string
	}{
		{
			name:     "stale covers discloses the contrast",
			scanned:  fullScan(),
			resolver: &fakeResolver{revision: "sha256:2222222222222222222222222222222222222222222222222222222222222222", ok: true},
			want:     []string{"sweep covers " + coversSHA + "; this wall renders " + wallRevision},
		},
		{
			name:     "unresolvable covers is disclosed-unproven, never a claimed mismatch",
			scanned:  fullScan(),
			resolver: &fakeResolver{ok: false},
			want:     []string{"sweep covers " + coversSHA + ", which this checkout cannot resolve; this wall renders " + wallRevision},
		},
		{
			name:     "partial sweep names each missing decision id",
			scanned:  []string{"spec/" + sweepSpecName + "#dc-1"},
			resolver: &fakeResolver{revision: wallRevision, ok: true},
			want:     []string{"dc-2 is not in decisions_scanned"},
		},
		{
			name:     "stale AND partial discloses both, covers line first",
			scanned:  nil,
			resolver: &fakeResolver{revision: "sha256:2222222222222222222222222222222222222222222222222222222222222222", ok: true},
			want: []string{
				"sweep covers " + coversSHA + "; this wall renders " + wallRevision,
				"dc-1 is not in decisions_scanned",
				"dc-2 is not in decisions_scanned",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := writeSweepStore(t, sweepReport(tc.scanned))
			rec, disclosure, err := JudgedSweepBadge(context.Background(), root, sweepSpecName, wallRevision, sweepSpecFM(), tc.resolver)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if disclosure != "" {
				t.Fatalf("unexpected disclosure: %q", disclosure)
			}
			if rec == nil {
				t.Fatal("want a chip, got none")
			}
			if !reflect.DeepEqual(rec.Disclosures, tc.want) {
				t.Errorf("Disclosures = %q, want %q", rec.Disclosures, tc.want)
			}
		})
	}
}

func TestJudgedSweepBadge_NoSweepProvenance_DisclosedNotCompared(t *testing.T) {
	content := `---
schema: verdi.decisionconflict/v1
covers: ` + coversSHA + `
findings:
  - { id: judged-dcf-1, kind: judged, text: "lone finding" }
---
`
	root := writeSweepStore(t, content)
	rec, _, err := JudgedSweepBadge(context.Background(), root, sweepSpecName, wallRevision, sweepSpecFM(), &fakeResolver{revision: wallRevision, ok: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec == nil {
		t.Fatal("want a chip, got none")
	}
	wantProvenance := []string{"sweep covers " + coversSHA}
	if !reflect.DeepEqual(rec.Provenance, wantProvenance) {
		t.Errorf("Provenance = %q, want the covers line alone", rec.Provenance)
	}
	want := []string{"sweep_provenance is absent from this report; adr_corpus_digest and decisions_scanned cannot be compared"}
	if !reflect.DeepEqual(rec.Disclosures, want) {
		t.Errorf("Disclosures = %q, want %q", rec.Disclosures, want)
	}
	if rec.Label != "1 judged finding" {
		t.Errorf("Label = %q, want the singular form", rec.Label)
	}
}

func TestJudgedSweepBadge_NilResolver_DisclosedUnproven(t *testing.T) {
	root := writeSweepStore(t, sweepReport(fullScan()))
	rec, _, err := JudgedSweepBadge(context.Background(), root, sweepSpecName, wallRevision, sweepSpecFM(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec == nil {
		t.Fatal("want a chip, got none")
	}
	if len(rec.Disclosures) != 1 || !strings.Contains(rec.Disclosures[0], "cannot resolve") {
		t.Errorf("a nil resolver must disclose the unproven comparison, got %q", rec.Disclosures)
	}
}

// TestJudgedSweepBadge_Deterministic is ac-2's same-record-same-bytes
// substrate at the compute layer: two runs over the same store produce
// byte-identical serialized records (no map-order or clock dependence).
func TestJudgedSweepBadge_Deterministic(t *testing.T) {
	root := writeSweepStore(t, sweepReport([]string{"spec/" + sweepSpecName + "#dc-1"}))
	run := func() []byte {
		rec, _, err := JudgedSweepBadge(context.Background(), root, sweepSpecName, wallRevision, sweepSpecFM(), &fakeResolver{revision: wallRevision, ok: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		j, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return j
	}
	a, b := run(), run()
	if string(a) != string(b) {
		t.Errorf("two computes over the same store differ:\n%s\n%s", a, b)
	}
}
