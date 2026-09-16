package specimport

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/store"
)

// RecordSchema is the committed record.json's one accepted schema value
// (spec-import-contract.md: "Record schema verdi.spec-import-record/v1").
const RecordSchema = "verdi.spec-import-record/v1"

// RecordSource is one retained source's normalized identity, range and
// digests — Snapshot WITHOUT Data (owner-preflight: "Record source metadata
// is normalized identities/ranges/digests; committed source payloads are
// selected Snapshot.Data. Do not embed original full Source.Data in
// record.json as a shortcut to claiming the original file digest is
// verified"). OriginalDigest is an unverifiable fingerprint of the input
// ReadRecord never claims to independently prove; Digest is the selected
// bytes actually committed at the paired sources/<id>.md sidecar, which
// verifyCommittedImport DOES independently verify against Git bytes.
type RecordSource struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	OriginalDigest string `json:"original_digest"`
	Digest         string `json:"digest"`
	StartLine      int    `json:"start_line,omitempty"`
	EndLine        int    `json:"end_line,omitempty"`
}

// RecordActor is the committed actor attribution and posture
// (spec-import-contract.md: "actor attribution and policy posture"),
// mirroring designprovenance.Entry's own Attribution/Harness/Session shape.
type RecordActor struct {
	Attribution governanceprincipal.Attribution `json:"attribution"`
	Harness     string                          `json:"harness,omitempty"`
	Session     string                          `json:"session,omitempty"`
}

// Record is the committed, non-authoritative provenance record at
// .verdi/imports/<slug>/<preview-digest>/record.json (spec-import-
// contract.md: "Record schema verdi.spec-import-record/v1 stores preview/
// base/model/config/engine/request/candidate digests, spec ref, normalized
// source identities/ranges/digests, mappings/origins, coverage, actor
// attribution and policy posture. No clock/randomness in this record. This
// new creation record supplies import provenance atomically; it does not
// forge an ASD mutation entry for a nonexistent prior draft").
//
// Format and ProfilePrimaryDigest are a uat-round-1 spec ac-4 addition
// (closing UAT-005: "no tooling exists to tell from a record which profile
// produced it"). Both are ADDITIVE and OPTIONAL-ON-READ: a record committed
// before this change carries neither key at all, decodes with both fields
// at their zero value ("" — reported as ABSENT, never fabricated as a real
// value), and remains otherwise valid — validate() imposes no requiredness
// on either field taken alone. They are REQUIRED-ON-WRITE in the narrow
// sense that every record Service.Apply produces from here forward always
// sets Format (Request.Format is itself a required, closed-enum field by
// the time Normalize has validated it), and sets ProfilePrimaryDigest
// whenever Format names a pinned reference profile. No other invariant
// loosens: validate() still enforces that a NON-EMPTY Format is one of the
// four closed Request.Format values, and that ProfilePrimaryDigest is
// present, and exactly equal to the profile's own pinned constant (see
// profilePrimaryDigestFor), whenever — and only whenever — Format names a
// reference profile.
type Record struct {
	Schema        string `json:"schema"`
	PreviewDigest string `json:"preview_digest"`
	BaseCommit    string `json:"base_commit"`
	ModelDigest   string `json:"model_digest"`
	ConfigDigest  string `json:"config_digest"`
	EngineDigest  string `json:"engine_digest"`
	RequestDigest string `json:"request_digest"`
	// Format is the Request.Format value ("native", "markdown-v1",
	// "f13-reference-v1" or "manual-v1") this record's candidate was
	// produced from. Absent ("") on a pre-ac-4 record.
	Format string `json:"format,omitempty"`
	// ProfilePrimaryDigest is the exact pinned primary SHA-256 the named
	// reference profile in Format was bound to (currently only
	// f13-reference-v1 names one). Absent ("") for every other format,
	// and for any pre-ac-4 record regardless of format.
	ProfilePrimaryDigest string                  `json:"profile_primary_digest,omitempty"`
	CandidateDigest      string                  `json:"candidate_digest"`
	SpecRef              string                  `json:"spec_ref"`
	Sources              []RecordSource          `json:"sources"`
	Fields               []Field                 `json:"fields"`
	Mappings             []Mapping               `json:"mappings,omitempty"`
	Coverage             []Coverage              `json:"coverage"`
	Actor                RecordActor             `json:"actor"`
	Policy               designprovenance.Policy `json:"policy"`
}

// validate checks Record's own closed shape: every digest/ref field present
// and correctly shaped, the closed origin/disposition/transform
// vocabularies, field and source identity grammar, the coverage invariants
// the contract states, the optional Format/ProfilePrimaryDigest pairing
// (see validateRecordFormat — both fields are absent-tolerant, so a
// pre-ac-4 record is unaffected), and (transitively, through Policy's own
// UnmarshalJSON and Actor.Attribution.Validate) the nested union/
// attribution shapes. Its checks are drawn from the same constants and
// validators Request.Validate and Normalize use, never a second grammar.
//
// It is a shape and internal-consistency check only — cross-referencing
// these claims against actual Git bytes is verifyCommittedImport's job,
// never this method's — and it is deliberately not a claim of authenticity
// against a rewritten branch: a writer who can forge history can equally
// write well-formed enums. What it guarantees is exactly the invariants
// enumerated above, no more: a record reaching a consumer through this
// package's SOLE decoder has passed those closed-vocabulary, identity,
// digest-shape and coverage-accounting checks. It is not a claim of
// semantic completeness or of truthfulness about the original source.
func (r Record) validate() error {
	if r.Schema != RecordSchema {
		return fmt.Errorf("record schema %q must be %q", r.Schema, RecordSchema)
	}
	if r.PreviewDigest == "" || r.BaseCommit == "" || r.ModelDigest == "" || r.ConfigDigest == "" ||
		r.EngineDigest == "" || r.RequestDigest == "" || r.CandidateDigest == "" || r.SpecRef == "" {
		return fmt.Errorf("record is missing a required digest or spec_ref field")
	}
	if err := validateRecordFormat(r); err != nil {
		return err
	}
	if err := validateRecordDigests(r); err != nil {
		return err
	}
	if _, err := artifact.ParseRef(r.SpecRef); err != nil {
		return fmt.Errorf("record spec_ref %q: %v", r.SpecRef, err)
	}
	if err := r.Actor.Attribution.Validate(); err != nil {
		return fmt.Errorf("record actor attribution: %w", err)
	}
	sourceIDs, err := validateRecordSources(r)
	if err != nil {
		return err
	}
	if err := validateRecordFields(r, sourceIDs); err != nil {
		return err
	}
	return validateRecordCoverage(r, sourceIDs)
}

// DecodeRecord strict-decodes record.json bytes, mirroring DecodeRequest's
// own shape (artifact.DecodeExactJSON then a closed-shape Validate), so the
// stored record is held to the identical strict-decode discipline as every
// other wire type this package owns.
func DecodeRecord(data []byte) (Record, error) {
	var record Record
	if err := artifact.DecodeExactJSON(data, &record); err != nil {
		return Record{}, fmt.Errorf("%w: decoding import record: %v", ErrImportRecordMissing, err)
	}
	if err := record.validate(); err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrImportRecordMissing, err)
	}
	return record, nil
}

// importWriteSetPaths returns the exact sorted repo-relative paths one
// import commit's write set must contain: the candidate spec, the record,
// and every retained source sidecar (spec-import-contract.md: "Commit only
// .verdi/specs/active/<slug>/spec.md plus .verdi/imports/<slug>/<preview-
// digest>/record.json and .verdi/imports/<slug>/<preview-digest>/sources/
// <source-id>.md").
func importWriteSetPaths(slug, previewDigest string, sources []RecordSource) []string {
	paths := make([]string, 0, len(sources)+2)
	paths = append(paths, store.ActiveSpecRelPath(slug), store.ImportRecordRelPath(slug, previewDigest))
	for _, s := range sources {
		paths = append(paths, store.ImportSourceRelPath(slug, previewDigest, s.ID))
	}
	sort.Strings(paths)
	return paths
}

// verifyExactWriteSet checks entries (a diff between an import commit and
// its parent) is EXACTLY want: every entry is a pure addition, and the
// sorted path sets match one-for-one — no extra file, no missing file, no
// non-add change (spec-import-contract.md: "build the base tree with the
// complete write set in sorted path order" / "exact one-parent write-set").
func verifyExactWriteSet(entries []gitx.DiffEntry, want []string) error {
	got := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Status != "A" {
			return fmt.Errorf("write set contains a non-add change %q at %q", e.Status, e.Path)
		}
		got = append(got, e.Path)
	}
	sort.Strings(got)
	if len(got) != len(want) {
		return fmt.Errorf("write set has %d path(s) %v, want exactly %d %v", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			return fmt.Errorf("write set path[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	return nil
}

// verifyCommittedImport reads slug's import record for previewDigest at
// ref, decodes it, and verifies every recorded claim against ACTUAL
// committed Git bytes: every retained source's digest, the import commit's
// exact one-parent write set, and the candidate's digest at the ORIGINAL
// import commit (spec-import-contract.md: "No claimed verification derives
// solely from self-reported record hashes without checking referenced Git
// bytes"). The original import commit is identified as the first-parent
// landing of the record.json blob currently at ref's tip — a path this
// package never rewrites after creation, so that landing is unambiguous.
//
// It never reads the caller's working tree: every fact comes from ref's
// committed Git objects, so this remains safe to call even when the
// caller's checkout/index has since become dirty.
func verifyCommittedImport(ctx context.Context, root, ref, slug, previewDigest string) (Record, string, error) {
	recordRelPath := store.ImportRecordRelPath(slug, previewDigest)
	recordBytes, err := gitx.Show(ctx, root, ref, recordRelPath)
	if err != nil {
		return Record{}, "", fmt.Errorf("%w: reading %s at %s: %v", ErrImportRecordMissing, recordRelPath, ref, err)
	}
	record, err := DecodeRecord(recordBytes)
	if err != nil {
		return Record{}, "", err
	}
	if record.PreviewDigest != previewDigest || record.SpecRef != specRef(slug) {
		return Record{}, "", fmt.Errorf("%w: record at %s does not name preview digest %q / spec %q", ErrImportRecordMissing, recordRelPath, previewDigest, specRef(slug))
	}

	for _, src := range record.Sources {
		srcPath := store.ImportSourceRelPath(slug, previewDigest, src.ID)
		data, err := gitx.Show(ctx, root, ref, srcPath)
		if err != nil {
			return Record{}, "", fmt.Errorf("%w: reading retained source %s: %v", ErrImportRecordMissing, srcPath, err)
		}
		if sha256Hex(data) != src.Digest {
			return Record{}, "", fmt.Errorf("%w: retained source %s does not match its recorded digest", ErrProvenanceMismatch, srcPath)
		}
	}

	blobOID, found, err := gitx.BlobAt(ctx, root, ref, recordRelPath)
	if err != nil {
		return Record{}, "", fmt.Errorf("%w: resolving import record blob: %v", ErrIOFailure, err)
	}
	if !found {
		return Record{}, "", fmt.Errorf("%w: %s not found at %s", ErrImportRecordMissing, recordRelPath, ref)
	}
	importCommit, found, err := gitx.FirstParentBlobLanding(ctx, root, ref, recordRelPath, blobOID)
	if err != nil {
		return Record{}, "", fmt.Errorf("%w: identifying original import commit: %v", ErrIOFailure, err)
	}
	if !found {
		return Record{}, "", fmt.Errorf("%w: could not identify the original import commit for %s", ErrImportRecordMissing, recordRelPath)
	}

	parent, err := gitx.RevParse(ctx, root, importCommit+"^")
	if err != nil {
		return Record{}, "", fmt.Errorf("%w: resolving import commit's parent: %v", ErrIOFailure, err)
	}
	if parent != record.BaseCommit {
		return Record{}, "", fmt.Errorf("%w: import commit %s parent %s does not match recorded base_commit %s", ErrProvenanceMismatch, importCommit, parent, record.BaseCommit)
	}
	if _, err := gitx.RevParse(ctx, root, importCommit+"^2"); err == nil {
		return Record{}, "", fmt.Errorf("%w: import commit %s has more than one parent", ErrProvenanceMismatch, importCommit)
	}
	diffEntries, err := gitx.DiffNameStatus(ctx, root, record.BaseCommit, importCommit)
	if err != nil {
		return Record{}, "", fmt.Errorf("%w: diffing import commit: %v", ErrIOFailure, err)
	}
	if err := verifyExactWriteSet(diffEntries, importWriteSetPaths(slug, previewDigest, record.Sources)); err != nil {
		return Record{}, "", fmt.Errorf("%w: %v", ErrProvenanceMismatch, err)
	}

	specBytes, err := gitx.Show(ctx, root, importCommit, store.ActiveSpecRelPath(slug))
	if err != nil {
		return Record{}, "", fmt.Errorf("%w: reading committed candidate at import commit: %v", ErrProvenanceMismatch, err)
	}
	if sha256Hex(specBytes) != record.CandidateDigest {
		return Record{}, "", fmt.Errorf("%w: committed candidate at import commit does not match recorded candidate_digest", ErrProvenanceMismatch)
	}

	return record, importCommit, nil
}

// discoverImportDigest finds slug's one committed preview-digest directory
// under .verdi/imports/<slug>/ at ref (ReadRecord's caller supplies no
// digest of its own, unlike Apply's retry path). A create-only ref can
// only ever have produced one such directory in ref's history; more than
// one is corruption, never a caller error to silently pick among.
func discoverImportDigest(ctx context.Context, root, ref, slug string) (string, error) {
	prefix := store.ImportDirRelPath(slug, "")
	entries, err := gitx.LsTree(ctx, root, ref, prefix)
	if err != nil {
		return "", fmt.Errorf("%w: listing %s at %s: %v", ErrImportRecordMissing, prefix, ref, err)
	}
	digests := map[string]bool{}
	for _, e := range entries {
		rest := strings.TrimPrefix(e, prefix+"/")
		segment, _, ok := strings.Cut(rest, "/")
		if !ok {
			continue
		}
		if e == store.ImportRecordRelPath(slug, segment) {
			digests[segment] = true
		}
	}
	switch len(digests) {
	case 0:
		return "", fmt.Errorf("%w: no import record for slug %q on %q", ErrImportRecordMissing, slug, ref)
	case 1:
		for d := range digests {
			return d, nil
		}
	}
	return "", fmt.Errorf("%w: %d distinct import records for slug %q on %q, which a create-only import can never produce", ErrImportRecordMissing, len(digests), slug, ref)
}

// RecordView is ReadRecord's exact result (spec-import-contract.md:
// "RecordView includes the decoded record, original import commit and an
// explicit current-spec-match boolean/disclosure"). CurrentSpecMatches is
// false, with a nonblocking current-spec-changed Disclosure, whenever the
// active spec at ref's CURRENT tip no longer matches the record's
// candidate_digest — an ordinary supported edit, not a corruption of the
// original provenance (still fully verified above).
type RecordView struct {
	Record             Record    `json:"record"`
	ImportCommit       string    `json:"import_commit"`
	CurrentSpecMatches bool      `json:"current_spec_matches"`
	Disclosures        []Finding `json:"disclosures"`
}

// ReadRecord owns committed provenance reads (spec-import-contract.md,
// "Shared internal interfaces": "ReadRecord ... owns committed provenance
// reads"). It identifies the original import commit even after ordinary
// descendant edits, independently verifies every recorded claim against
// actual Git bytes (never trusting record.json's self-reported hashes
// alone), and discloses a current-spec mismatch truthfully rather than
// reporting it as corrupted provenance.
func ReadRecord(ctx context.Context, root, branch, slug string) (RecordView, error) {
	previewDigest, err := discoverImportDigest(ctx, root, branch, slug)
	if err != nil {
		return RecordView{}, err
	}
	record, importCommit, err := verifyCommittedImport(ctx, root, branch, slug, previewDigest)
	if err != nil {
		return RecordView{}, err
	}

	currentSpec, showErr := gitx.Show(ctx, root, branch, store.ActiveSpecRelPath(slug))
	matches := showErr == nil && sha256Hex(currentSpec) == record.CandidateDigest

	var disclosures []Finding
	if !matches {
		detail := "the active spec's current bytes no longer match the recorded candidate_digest"
		if showErr != nil {
			detail = "the active spec is no longer present at its imported path on this branch (moved, archived, or removed)"
		}
		disclosures = append(disclosures, Finding{
			Code:     FindingCurrentSpecChanged,
			Target:   specRef(slug),
			Message:  fmt.Sprintf("%s; this record describes the ORIGINAL imported revision at commit %s, not current spec state — ordinary supported edits do not corrupt the original provenance", detail, importCommit),
			Blocking: false,
		})
	}

	return RecordView{Record: record, ImportCommit: importCommit, CurrentSpecMatches: matches, Disclosures: disclosures}, nil
}

// encodeRecord canonically encodes record for commit — canonjson.Marshal,
// so the committed bytes are stable regardless of Go struct/map ordering.
func encodeRecord(record Record) ([]byte, error) {
	data, err := canonjson.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("%w: encoding import record: %v", ErrIOFailure, err)
	}
	return data, nil
}
