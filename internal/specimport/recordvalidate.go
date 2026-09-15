package specimport

import (
	"fmt"
	"regexp"
)

// sha256HexRe is the shape every SHA-256 lowercase-hex digest this package
// computes through sha256Hex must have (spec-import-contract.md: "digests
// are SHA-256 lowercase hex"): preview/request/candidate/config/engine
// digests and both per-source digests.
var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// gitOIDRe is the shape of a Git object id, which base_commit records. A
// Git OID is NOT a SHA-256 content digest of this package's own making: it
// is whatever width the repository's object format uses (40 hex for sha1,
// 64 for sha256 repositories), so both are accepted and neither is
// conflated with a sha256Hex digest.
var gitOIDRe = regexp.MustCompile(`^[0-9a-f]{40}$|^[0-9a-f]{64}$`)

// prefixedDigestRe is canonjson.Digest's own output shape, which
// model.Model.Digest and designprovenance.Policy both produce: the literal
// "sha256:" prefix followed by lowercase hex.
var prefixedDigestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// validSha256Hex checks one bare SHA-256 hex digest field.
func validSha256Hex(field, value string) error {
	if !sha256HexRe.MatchString(value) {
		return fmt.Errorf("%s %q is not a lowercase 64-hex sha256 digest", field, value)
	}
	return nil
}

// validFieldTarget reports whether target names a destination this package
// can actually populate: a statement (problem/outcome) or one of the four
// bare object-id kinds. It consumes the same grammar Request.Validate
// applies to an explicit Mapping target, rather than inventing a second one.
func validFieldTarget(target string) bool {
	return target == "problem" || target == "outcome" || mappingTargetIDRe.MatchString(target)
}

// validFieldOrigin reports whether origin is one of the closed Field.Origin
// values (schema.go's four Normalize origins plus Compose's
// generated-deferral).
func validFieldOrigin(origin string) bool {
	switch origin {
	case OriginCopiedSource, OriginUserEditedSrc, OriginUserAdded, OriginNative, OriginGeneratedDeferral:
		return true
	default:
		return false
	}
}

// validDisposition reports whether disposition is one of the three closed
// coverage dispositions (spec-import-contract.md: "An interval is exactly
// one of mapped/retained-only/unresolved").
func validDisposition(disposition string) bool {
	switch disposition {
	case DispositionMapped, DispositionRetained, DispositionUnresolved:
		return true
	default:
		return false
	}
}

// validateRecordDigests checks every stored digest/identity field's shape,
// honouring the format each one actually has: base_commit is a Git OID,
// model_digest and the policy digest carry canonjson's "sha256:" prefix,
// and the rest are bare sha256Hex output.
func validateRecordDigests(r Record) error {
	if !gitOIDRe.MatchString(r.BaseCommit) {
		return fmt.Errorf("record base_commit %q is not a git object id", r.BaseCommit)
	}
	if !prefixedDigestRe.MatchString(r.ModelDigest) {
		return fmt.Errorf("record model_digest %q is not a sha256:-prefixed digest", r.ModelDigest)
	}
	for _, pair := range []struct{ field, value string }{
		{"record preview_digest", r.PreviewDigest},
		{"record config_digest", r.ConfigDigest},
		{"record engine_digest", r.EngineDigest},
		{"record request_digest", r.RequestDigest},
		{"record candidate_digest", r.CandidateDigest},
	} {
		if err := validSha256Hex(pair.field, pair.value); err != nil {
			return err
		}
	}
	return nil
}

// validateRecordSources checks every retained source's identity and digest
// shapes and returns the set of ids the rest of the record may reference.
// OriginalDigest must be present and well-shaped, but it remains an
// explicitly UNVERIFIABLE fingerprint of an input this package no longer
// holds (RecordSource's own doc comment): shape is all that can honestly be
// checked here.
func validateRecordSources(r Record) (map[string]bool, error) {
	ids := make(map[string]bool, len(r.Sources))
	for i, s := range r.Sources {
		if !sourceIDRe.MatchString(s.ID) {
			return nil, fmt.Errorf("record source[%d] id %q must match %s", i, s.ID, sourceIDRe.String())
		}
		if ids[s.ID] {
			return nil, fmt.Errorf("record source id %q is duplicated", s.ID)
		}
		ids[s.ID] = true
		if !nonBlankUTF8(s.Label) {
			return nil, fmt.Errorf("record source %q label must be a nonblank valid UTF-8 string", s.ID)
		}
		if err := validSha256Hex(fmt.Sprintf("record source %q digest", s.ID), s.Digest); err != nil {
			return nil, err
		}
		if err := validSha256Hex(fmt.Sprintf("record source %q original_digest", s.ID), s.OriginalDigest); err != nil {
			return nil, err
		}
		if s.StartLine < 0 || s.EndLine < 0 || (s.EndLine != 0 && s.EndLine < s.StartLine) {
			return nil, fmt.Errorf("record source %q has an invalid line range [%d,%d]", s.ID, s.StartLine, s.EndLine)
		}
	}
	return ids, nil
}

// validateRecordFields checks every resolved field's and stored mapping's
// destination grammar, closed origin, evidence and transform vocabularies,
// and source-identity bindings.
func validateRecordFields(r Record, sourceIDs map[string]bool) error {
	for i, f := range r.Fields {
		if !validFieldTarget(f.Target) {
			return fmt.Errorf("record field %d target %q must be problem, outcome, or a valid ac-/co-/dc-/oq- object id", i, f.Target)
		}
		if !validFieldOrigin(f.Origin) {
			return fmt.Errorf("record field %q origin %q is not one of copied-source, user-edited-source, user-added, native, generated-deferral", f.Target, f.Origin)
		}
		for j, span := range f.Spans {
			if !sourceIDs[span.SourceID] {
				return fmt.Errorf("record field %q span %d names source %q, which the record does not carry", f.Target, j, span.SourceID)
			}
			if span.Start < 0 || span.End < span.Start {
				return fmt.Errorf("record field %q span %d has an invalid half-open range [%d,%d)", f.Target, j, span.Start, span.End)
			}
			if span.Transform != "" && !validTransforms[span.Transform] {
				return fmt.Errorf("record field %q span %d transform %q is not a closed transform", f.Target, j, span.Transform)
			}
		}
		if err := validateEvidence(i, f.Target, f.Evidence); err != nil {
			return fmt.Errorf("record field %q %v", f.Target, err)
		}
	}
	for i, m := range r.Mappings {
		if !validFieldTarget(m.Target) {
			return fmt.Errorf("record mapping %d target %q must be problem, outcome, or a valid ac-/co-/dc-/oq- object id", i, m.Target)
		}
		if m.SourceID != "" && !sourceIDs[m.SourceID] {
			return fmt.Errorf("record mapping %d names source %q, which the record does not carry", i, m.SourceID)
		}
		// An empty Transform is legitimate on the evidence-only and
		// user-added mapping shapes; only a declared one must be closed.
		if m.Transform != "" && !validTransforms[m.Transform] {
			return fmt.Errorf("record mapping %d (target %q) transform %q is not one of identity, trim-blank-lines, collapse-whitespace, list-item", i, m.Target, m.Transform)
		}
		if err := validateEvidence(i, m.Target, m.Evidence); err != nil {
			return fmt.Errorf("record mapping %d (target %q) %v", i, m.Target, err)
		}
	}
	return nil
}

// validateEvidence applies the request surface's own evidence rule to a
// stored field's or mapping's evidence list: the DECISION is
// validateMappingEvidence's — acceptance-criterion targets only, closed
// static/behavioral/runtime/attestation kinds, no duplicates, and an empty
// list always allowed — reused verbatim rather than restated here.
//
// Only the diagnostic is re-worded: validateMappingEvidence's own message
// is phrased for a request's mappings[i], which would misname the stored
// field or mapping this record actually carries, so the offending list is
// reported in the record's own terms.
func validateEvidence(index int, target string, evidence []string) error {
	if err := validateMappingEvidence(index, Mapping{Target: target, Evidence: evidence}); err != nil {
		return fmt.Errorf("evidence %v is not a unique list of acceptance-criterion evidence kinds (static, behavioral, runtime, attestation)", evidence)
	}
	return nil
}

// validateRecordCoverage enforces the contract's byte-accounting invariants
// on the stored coverage (spec-import-contract.md: "The coverage record
// partitions every selected source byte exactly once ... For every source
// assert TotalBytes == MappedBytes + RetainedBytes + UnresolvedBytes"):
// every recorded source has exactly one coverage entry and every entry
// names a recorded source, the three per-disposition totals sum to
// TotalBytes and are each reproduced by the intervals themselves, and the
// intervals are a contiguous, non-overlapping, ordered partition of
// [0,TotalBytes).
//
// Interval targets are checked for grammar only, and accept the native
// whole-primary pseudo-target: native content is not decomposed into
// problem/outcome/ac-N fields at all, so its one mapped interval names
// "native" rather than a real field (normalize.go's nativeCoverageTarget).
func validateRecordCoverage(r Record, sourceIDs map[string]bool) error {
	seen := make(map[string]bool, len(r.Coverage))
	for i, cov := range r.Coverage {
		if !sourceIDs[cov.SourceID] {
			return fmt.Errorf("record coverage[%d] names source %q, which the record does not carry", i, cov.SourceID)
		}
		if seen[cov.SourceID] {
			return fmt.Errorf("record coverage for source %q is duplicated", cov.SourceID)
		}
		seen[cov.SourceID] = true
		if cov.TotalBytes < 0 || cov.MappedBytes < 0 || cov.RetainedBytes < 0 || cov.UnresolvedBytes < 0 {
			return fmt.Errorf("record coverage for source %q has a negative byte total", cov.SourceID)
		}
		if sum := cov.MappedBytes + cov.RetainedBytes + cov.UnresolvedBytes; sum != cov.TotalBytes {
			return fmt.Errorf("record coverage for source %q: total_bytes %d != mapped %d + retained %d + unresolved %d (= %d)",
				cov.SourceID, cov.TotalBytes, cov.MappedBytes, cov.RetainedBytes, cov.UnresolvedBytes, sum)
		}
		byDisposition := map[string]int{}
		next := 0
		for j, interval := range cov.Intervals {
			if !validDisposition(interval.Disposition) {
				return fmt.Errorf("record coverage for source %q interval %d disposition %q is not one of mapped, retained-only, unresolved",
					cov.SourceID, j, interval.Disposition)
			}
			if interval.Start != next || interval.End <= interval.Start {
				return fmt.Errorf("record coverage for source %q intervals do not partition [0,%d) contiguously: interval %d is [%d,%d), expected a nonempty interval starting at %d",
					cov.SourceID, cov.TotalBytes, j, interval.Start, interval.End, next)
			}
			next = interval.End
			if interval.Disposition == DispositionMapped && len(interval.Targets) == 0 {
				return fmt.Errorf("record coverage for source %q interval %d is mapped but lists no destination", cov.SourceID, j)
			}
			if interval.Disposition != DispositionMapped && len(interval.Targets) != 0 {
				return fmt.Errorf("record coverage for source %q interval %d is %s but lists destinations %v", cov.SourceID, j, interval.Disposition, interval.Targets)
			}
			for _, target := range interval.Targets {
				if target != nativeCoverageTarget && !validFieldTarget(target) {
					return fmt.Errorf("record coverage for source %q interval %d names destination %q, which is not a field target", cov.SourceID, j, target)
				}
			}
			byDisposition[interval.Disposition] += interval.End - interval.Start
		}
		if next != cov.TotalBytes {
			return fmt.Errorf("record coverage for source %q intervals cover [0,%d), not the recorded total_bytes %d", cov.SourceID, next, cov.TotalBytes)
		}
		for _, want := range []struct {
			disposition string
			total       int
		}{
			{DispositionMapped, cov.MappedBytes},
			{DispositionRetained, cov.RetainedBytes},
			{DispositionUnresolved, cov.UnresolvedBytes},
		} {
			if byDisposition[want.disposition] != want.total {
				return fmt.Errorf("record coverage for source %q: intervals account %d %s byte(s), but the record claims %d",
					cov.SourceID, byDisposition[want.disposition], want.disposition, want.total)
			}
		}
	}
	// Completeness: every recorded source carries its own accounting, so a
	// retained support document cannot silently lose its byte coverage
	// while remaining a committed sidecar. Walked in r.Sources order so the
	// first missing source is reported deterministically.
	for _, s := range r.Sources {
		if !seen[s.ID] {
			return fmt.Errorf("record has no coverage entry for source %q; every recorded source carries its own byte accounting", s.ID)
		}
	}
	return nil
}
