package specimport

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// publishedRecord runs a real import and returns the decoded record its
// commit actually carries, so every strictness case below starts from a
// record a conforming Apply produced rather than a hand-built literal.
func publishedRecord(t *testing.T, root string, req Request) (Result, Record) {
	t.Helper()
	svc := testService(t)
	ctx := context.Background()
	preview, err := svc.Preview(ctx, root, req)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !preview.Ready {
		t.Fatalf("preview not ready: %+v", preview.Findings)
	}
	result, err := svc.Apply(ctx, root, req, preview.Digest, testAgent(t))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	data, err := gitx.Show(ctx, root, result.Branch, store.ImportRecordRelPath(req.Target.Slug, result.PreviewDigest))
	if err != nil {
		t.Fatal(err)
	}
	record, err := DecodeRecord(data)
	if err != nil {
		t.Fatalf("a record this package just published must decode: %v", err)
	}
	return result, record
}

func cloneRecord(in Record) Record {
	out := in
	out.Sources = append([]RecordSource(nil), in.Sources...)
	out.Mappings = append([]Mapping(nil), in.Mappings...)
	out.Fields = append([]Field(nil), in.Fields...)
	for i := range out.Fields {
		out.Fields[i].Spans = append([]Span(nil), in.Fields[i].Spans...)
	}
	out.Coverage = append([]Coverage(nil), in.Coverage...)
	for i := range out.Coverage {
		out.Coverage[i].Intervals = append([]Interval(nil), in.Coverage[i].Intervals...)
	}
	return out
}

// TestDecodeRecord_RefusesSemanticallyImpossibleRecords holds the package's
// SOLE record decoder to the closed vocabularies, digest shapes, identity
// grammar and coverage invariants the contract states (spec-import-
// contract.md: "The import record's sole decoder belongs to specimport";
// "For every source assert TotalBytes == MappedBytes + RetainedBytes +
// UnresolvedBytes"; "An interval is exactly one of mapped/retained-only/
// unresolved"). Each case mutates exactly one fact of a really-published
// record into a state no conforming Normalize/Compose could emit, re-encodes
// with the package's own canonical encoder, and requires a refusal that
// names the violated invariant.
//
// This is bounded record-shape validation, not authenticity against a
// forged Git history: a writer who can rewrite the branch can equally write
// well-formed enums. The point is that Task 4's CLI and record route, which
// hold this decoder, never present a semantically impossible record as
// verified provenance.
func TestDecodeRecord_RefusesSemanticallyImpossibleRecords(t *testing.T) {
	repo := buildImportRepo(t)
	_, base := publishedRecord(t, repo.Dir, evidencedRequest())
	if len(base.Fields) == 0 || len(base.Sources) == 0 || len(base.Coverage) == 0 || len(base.Coverage[0].Intervals) == 0 {
		t.Fatalf("baseline record is too thin to corrupt: %+v", base)
	}
	if len(base.Mappings) == 0 || base.Mappings[0].Target != "ac-1" {
		t.Fatalf("baseline record must carry the evidence-only ac-1 mapping the nested-enum cases corrupt: %+v", base.Mappings)
	}

	cases := []struct {
		name    string
		corrupt func(*Record)
		want    string
	}{
		{
			name:    "field origin outside the closed vocabulary",
			corrupt: func(r *Record) { r.Fields[0].Origin = "authored-by-a-third-party" },
			want:    `origin "authored-by-a-third-party"`,
		},
		{
			name:    "field target is neither a statement nor an object id",
			corrupt: func(r *Record) { r.Fields[0].Target = "../../escape" },
			want:    `target "../../escape"`,
		},
		{
			name: "field span names a source the record does not carry",
			corrupt: func(r *Record) {
				r.Fields[0].Spans = []Span{{SourceID: "no-such-source", Start: 0, End: 1, Transform: TransformIdentity}}
			},
			want: `span 0 names source "no-such-source"`,
		},
		{
			name: "field evidence kind outside the closed vocabulary",
			corrupt: func(r *Record) {
				for i := range r.Fields {
					if strings.HasPrefix(r.Fields[i].Target, "ac-") {
						r.Fields[i].Evidence = []string{"invented-proof"}
						return
					}
				}
			},
			want: `record field "ac-1" evidence [invented-proof]`,
		},
		{
			name:    "mapping evidence kind outside the closed vocabulary",
			corrupt: func(r *Record) { r.Mappings[0].Evidence = []string{"invented-proof"} },
			want:    `record mapping 0 (target "ac-1") evidence [invented-proof]`,
		},
		{
			name:    "mapping transform outside the closed vocabulary",
			corrupt: func(r *Record) { r.Mappings[0].Transform = "invented-transform" },
			want:    `record mapping 0 (target "ac-1") transform "invented-transform"`,
		},
		{
			name:    "interval disposition outside the closed vocabulary",
			corrupt: func(r *Record) { r.Coverage[0].Intervals[0].Disposition = "invented-disposition" },
			want:    `disposition "invented-disposition"`,
		},
		{
			name:    "coverage totals do not balance",
			corrupt: func(r *Record) { r.Coverage[0].TotalBytes += 4096 },
			want:    "total_bytes",
		},
		{
			name: "coverage intervals do not partition the source",
			corrupt: func(r *Record) {
				r.Coverage[0].Intervals = append([]Interval{{Start: 0, End: 0, Disposition: DispositionMapped}}, r.Coverage[0].Intervals...)
			},
			want: "intervals",
		},
		{
			name:    "coverage names a source the record does not carry",
			corrupt: func(r *Record) { r.Coverage[0].SourceID = "no-such-source" },
			want:    `coverage[0] names source "no-such-source"`,
		},
		{
			name:    "source digest is not a sha256",
			corrupt: func(r *Record) { r.Sources[0].Digest = "obviously-not-a-digest" },
			want:    "digest",
		},
		{
			name:    "source original_digest absent",
			corrupt: func(r *Record) { r.Sources[0].OriginalDigest = "" },
			want:    "original_digest",
		},
		{
			name:    "candidate digest is not a sha256",
			corrupt: func(r *Record) { r.CandidateDigest = "not-a-digest" },
			want:    "candidate_digest",
		},
		{
			name:    "model digest drops its sha256 prefix",
			corrupt: func(r *Record) { r.ModelDigest = strings.TrimPrefix(r.ModelDigest, "sha256:") },
			want:    "model_digest",
		},
		{
			name:    "base commit is not a git object id",
			corrupt: func(r *Record) { r.BaseCommit = "HEAD" },
			want:    "base_commit",
		},
		{
			name:    "format outside the closed vocabulary",
			corrupt: func(r *Record) { r.Format = "yaml-v9" },
			want:    `record format "yaml-v9" is not one of`,
		},
		{
			name:    "profile_primary_digest set for a format with no bound reference profile",
			corrupt: func(r *Record) { r.ProfilePrimaryDigest = f13PrimarySHA256 },
			want:    `is set but format "markdown-v1" names no pinned reference profile`,
		},
		{
			name:    "f13-reference-v1 format missing its pinned profile digest",
			corrupt: func(r *Record) { r.Format = FormatF13Reference },
			want:    `must be exactly the "f13-reference-v1" profile's pinned primary`,
		},
		{
			name: "f13-reference-v1 format with a mismatched profile digest",
			corrupt: func(r *Record) {
				r.Format = FormatF13Reference
				r.ProfilePrimaryDigest = strings.Repeat("a", 64)
			},
			want: `must be exactly the "f13-reference-v1" profile's pinned primary`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tampered := cloneRecord(base)
			tc.corrupt(&tampered)
			data, err := encodeRecord(tampered)
			if err != nil {
				t.Fatalf("encodeRecord: %v", err)
			}
			_, err = DecodeRecord(data)
			if !errors.Is(err, ErrImportRecordMissing) {
				t.Fatalf("DecodeRecord(%s) = %v, want a refusal wrapping ErrImportRecordMissing", tc.name, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("DecodeRecord(%s) = %v, want a refusal naming %q", tc.name, err, tc.want)
			}
		})
	}
}

// multiSourceRequest adds a retained-only support document alongside the
// evidenced primary (spec-import-contract.md: "Extra selected supports can
// be retained"), so a real published record carries TWO sources and their
// two coverage entries — the shape the completeness check below needs.
func multiSourceRequest() Request {
	req := evidencedRequest()
	req.Sources = append(req.Sources, Source{
		ID:    "support",
		Label: "support.md",
		Data:  []byte("Background the operator retained alongside the primary, mapped to nothing.\n"),
	})
	return req
}

// TestDecodeRecord_AcceptsEveryConformingRecordShape is the strictness
// cases' mandatory counterweight: the record shapes a conforming Apply
// really produces — external copied/edited source, native, an explicitly
// user-added field, a generated deferral, a source-backed explicit mapping
// alongside an evidence-only one, and a retained-only support document
// beside the primary — must all still decode, and between them must
// exercise the closed origin, evidence, transform and disposition
// vocabularies and the native whole-primary coverage the checks above
// police.
func TestDecodeRecord_AcceptsEveryConformingRecordShape(t *testing.T) {
	userAdded := evidencedRequest()
	addedText := "A criterion the operator added with no source backing at all."
	userAdded.Mappings = append(userAdded.Mappings, Mapping{
		Target: "ac-9", Text: &addedText, Evidence: []string{"static", "attestation"},
	})
	deferred := evidencedRequest()
	deferred.DeferStatements = true
	// ac-1 becomes an explicit source-backed mapping over its own
	// automatic span (a real list item of validMarkdownSource), so one
	// record carries BOTH a mapping with a closed non-empty transform and
	// the evidence-only ac-2 mapping whose transform is legitimately empty.
	sourceBacked := evidencedRequest()
	sourceBacked.Mappings[0] = Mapping{
		Target: "ac-1", SourceID: "source", Start: 112, End: 126,
		Transform: TransformListItem, Evidence: []string{"static", "attestation"},
	}

	shapes := map[string]Request{
		"external":      evidencedRequest(),
		"native":        newSpecNativeRequest(),
		"user-added":    userAdded,
		"deferral":      deferred,
		"source-backed": sourceBacked,
		"multi-source":  multiSourceRequest(),
	}
	names := make([]string, 0, len(shapes))
	for name := range shapes {
		names = append(names, name)
	}
	sort.Strings(names)

	origins := map[string]bool{}
	dispositions := map[string]bool{}
	evidenceKinds := map[string]bool{}
	transforms := map[string]bool{}
	formats := map[string]bool{}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			repo := buildImportRepo(t)
			_, record := publishedRecord(t, repo.Dir, shapes[name])
			formats[record.Format] = true
			if !validFormats[record.Format] {
				t.Fatalf("record.Format = %q, want one of the closed formats", record.Format)
			}
			if record.ProfilePrimaryDigest != "" {
				t.Fatalf("record.ProfilePrimaryDigest = %q, want absent: shape %q names no reference profile", record.ProfilePrimaryDigest, name)
			}
			for _, f := range record.Fields {
				origins[f.Origin] = true
				for _, kind := range f.Evidence {
					evidenceKinds[kind] = true
				}
				for _, span := range f.Spans {
					transforms[span.Transform] = true
				}
			}
			for _, m := range record.Mappings {
				transforms[m.Transform] = true
				for _, kind := range m.Evidence {
					evidenceKinds[kind] = true
				}
			}
			for _, cov := range record.Coverage {
				for _, interval := range cov.Intervals {
					dispositions[interval.Disposition] = true
				}
			}
			if name == "native" {
				if len(record.Coverage) != 1 || len(record.Coverage[0].Intervals) != 1 {
					t.Fatalf("native coverage = %+v, want one whole-primary interval", record.Coverage)
				}
				got := record.Coverage[0].Intervals[0]
				if got.Disposition != DispositionMapped || len(got.Targets) != 1 || got.Targets[0] != nativeCoverageTarget {
					t.Fatalf("native interval = %+v, want the whole primary mapped to the native pseudo-target", got)
				}
			}
			if name == "multi-source" {
				if len(record.Sources) != 2 || len(record.Coverage) != 2 {
					t.Fatalf("multi-source record = %d source(s)/%d coverage entr(ies), want 2/2", len(record.Sources), len(record.Coverage))
				}
			}
			if name == "source-backed" {
				var sawSourceBacked, sawEvidenceOnly bool
				for _, m := range record.Mappings {
					if m.SourceID != "" && m.Transform == TransformListItem {
						sawSourceBacked = true
					}
					if m.SourceID == "" && m.Transform == "" && len(m.Evidence) > 0 {
						sawEvidenceOnly = true
					}
				}
				if !sawSourceBacked || !sawEvidenceOnly {
					t.Fatalf("mappings = %+v, want both a source-backed transform and a preserved evidence-only mapping with an empty transform", record.Mappings)
				}
			}
		})
	}

	for _, format := range []string{FormatMarkdownV1, FormatNative} {
		if !formats[format] {
			t.Errorf("no published record exercised format %q", format)
		}
	}
	for _, origin := range []string{OriginCopiedSource, OriginUserAdded, OriginGeneratedDeferral} {
		if !origins[origin] {
			t.Errorf("no published record exercised origin %q; the strictness table may be policing an unreachable vocabulary", origin)
		}
	}
	for _, disposition := range []string{DispositionMapped, DispositionRetained} {
		if !dispositions[disposition] {
			t.Errorf("no published record exercised disposition %q", disposition)
		}
	}
	for _, kind := range []string{"static", "attestation"} {
		if !evidenceKinds[kind] {
			t.Errorf("no published record exercised evidence kind %q", kind)
		}
	}
	// "" is the evidence-only/user-added mapping's legitimately absent
	// transform, which the closed-vocabulary check must keep accepting.
	for _, transform := range []string{"", TransformTrimBlankLines, TransformListItem} {
		if !transforms[transform] {
			t.Errorf("no published record exercised transform %q", transform)
		}
	}
}

// TestDecodeRecord_RequiresOneCoverageEntryPerSource pins the completeness
// half of the coverage invariant, which checking only the entries that
// happen to be present cannot reach: the contract accounts for every
// selected byte of every retained source (spec-import-contract.md: "The
// coverage record partitions every selected source byte exactly once ...
// For every source assert TotalBytes == MappedBytes + RetainedBytes +
// UnresolvedBytes"), so a record that carries a source but no coverage for
// it has dropped that source's byte accounting entirely.
//
// The baseline is a real two-source import — the evidenced primary plus a
// retained-only support document — so both a mapped and a retained-only
// source's entry can go missing independently.
func TestDecodeRecord_RequiresOneCoverageEntryPerSource(t *testing.T) {
	repo := buildImportRepo(t)
	_, base := publishedRecord(t, repo.Dir, multiSourceRequest())

	if len(base.Sources) != 2 || base.Sources[0].ID != "source" || base.Sources[1].ID != "support" {
		t.Fatalf("baseline sources = %+v, want the primary and its retained support", base.Sources)
	}
	if len(base.Coverage) != 2 {
		t.Fatalf("baseline coverage = %+v, want exactly one entry per recorded source", base.Coverage)
	}
	support := recordCoverageForSource(t, base, "support")
	if support.RetainedBytes != support.TotalBytes || support.TotalBytes == 0 {
		t.Fatalf("support coverage = %+v, want every selected byte retained-only", support)
	}

	withoutCoverageFor := func(id string) func(*Record) {
		return func(r *Record) {
			kept := make([]Coverage, 0, len(r.Coverage))
			for _, cov := range r.Coverage {
				if cov.SourceID != id {
					kept = append(kept, cov)
				}
			}
			r.Coverage = kept
		}
	}

	cases := []struct {
		name    string
		corrupt func(*Record)
		want    string
	}{
		{
			name:    "no coverage entries at all",
			corrupt: func(r *Record) { r.Coverage = nil },
			want:    `record has no coverage entry for source "source"`,
		},
		{
			name:    "the retained-only support source's entry is missing",
			corrupt: withoutCoverageFor("support"),
			want:    `record has no coverage entry for source "support"`,
		},
		{
			name:    "the primary source's entry is missing",
			corrupt: withoutCoverageFor("source"),
			want:    `record has no coverage entry for source "source"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tampered := cloneRecord(base)
			tc.corrupt(&tampered)
			data, err := encodeRecord(tampered)
			if err != nil {
				t.Fatalf("encodeRecord: %v", err)
			}
			_, err = DecodeRecord(data)
			if !errors.Is(err, ErrImportRecordMissing) {
				t.Fatalf("DecodeRecord(%s) = %v, want a refusal wrapping ErrImportRecordMissing", tc.name, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("DecodeRecord(%s) = %v, want a refusal naming %q", tc.name, err, tc.want)
			}
		})
	}

	// The untouched two-source record still decodes: the completeness rule
	// requires exactly one entry per source, never more than one.
	data, err := encodeRecord(cloneRecord(base))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeRecord(data); err != nil {
		t.Fatalf("DecodeRecord(complete two-source record) = %v, want acceptance", err)
	}
}

func recordCoverageForSource(t *testing.T, record Record, id string) Coverage {
	t.Helper()
	for _, cov := range record.Coverage {
		if cov.SourceID == id {
			return cov
		}
	}
	t.Fatalf("record carries no coverage for source %q: %+v", id, record.Coverage)
	return Coverage{}
}

// TestReadRecord_RefusesRebuiltSemanticallyInvalidRecord drives the same
// gap through the read surface Task 4 will expose. The import commit is
// rebuilt out of band with an identical write set, an identical parent and
// identical candidate/source bytes — only record.json's internals change —
// so every Git binding ReadRecord checks still holds and the record's own
// shape validation is the only thing standing between a semantically
// impossible record and a "verified proof" answer.
func TestReadRecord_RefusesRebuiltSemanticallyInvalidRecord(t *testing.T) {
	repo := buildImportRepo(t)
	ctx := context.Background()
	slug := "sample-feature"
	result, record := publishedRecord(t, repo.Dir, evidencedRequest())

	record.Fields[0].Origin = "authored-by-a-third-party"
	record.Coverage[0].TotalBytes += 4096
	record.Coverage[0].Intervals[0].Disposition = "invented-disposition"
	tampered, err := encodeRecord(record)
	if err != nil {
		t.Fatal(err)
	}

	recordRel := store.ImportRecordRelPath(slug, result.PreviewDigest)
	sourceRel := store.ImportSourceRelPath(slug, result.PreviewDigest, "source")
	specBytes, err := gitx.Show(ctx, repo.Dir, result.Branch, store.ActiveSpecRelPath(slug))
	if err != nil {
		t.Fatal(err)
	}
	sourceBytes, err := gitx.Show(ctx, repo.Dir, result.Branch, sourceRel)
	if err != nil {
		t.Fatal(err)
	}
	files := []writeFile{
		{path: store.ActiveSpecRelPath(slug), data: specBytes},
		{path: recordRel, data: tampered},
		{path: sourceRel, data: sourceBytes},
	}
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	tree := record.BaseCommit + "^{tree}"
	for _, f := range files {
		blob, err := gitx.WriteBlob(ctx, repo.Dir, f.data)
		if err != nil {
			t.Fatal(err)
		}
		if tree, err = gitx.BuildTreeWithFile(ctx, repo.Dir, tree, f.path, blob); err != nil {
			t.Fatal(err)
		}
	}
	commit, err := gitx.CommitTree(ctx, repo.Dir, tree, record.BaseCommit, "rebuilt import commit")
	if err != nil {
		t.Fatal(err)
	}
	runGitFixture(t, repo.Dir, "update-ref", "refs/heads/"+result.Branch, commit)

	view, err := ReadRecord(ctx, repo.Dir, result.Branch, slug)
	if !errors.Is(err, ErrImportRecordMissing) {
		t.Fatalf("ReadRecord(semantically impossible record) = %+v, %v; want a refusal wrapping ErrImportRecordMissing", view, err)
	}
	if view.CurrentSpecMatches || view.ImportCommit != "" || len(view.Disclosures) != 0 {
		t.Fatalf("a refused ReadRecord still reported proof: %+v", view)
	}
}
