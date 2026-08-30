package sealedexec

import (
	"context"
	"errors"
	"fmt"

	"github.com/jyang234/verdi/internal/contextevent"
)

// SegmentStore owns durable redacted-segment bytes and their deterministic
// controller references.
type SegmentStore interface {
	StoreRedactedSegment(context.Context, RedactedSegment) (StoredSegment, error)
	ResolveRedactedSegment(context.Context, string) (RedactedSegment, error)
}

// DetailFailureCategory is the closed classification of a DetailProcessor
// failure. Call sites map it to Amendment 002 §5's exact reason and operation
// without ever inspecting an error string.
type DetailFailureCategory string

const (
	// DetailFailureRedaction covers every inability to redact or represent the
	// canonical bytes. It is also the safe default for an unclassified failure.
	DetailFailureRedaction DetailFailureCategory = "redaction"
	// DetailFailureSegmentStore covers a refused controller segment store.
	DetailFailureSegmentStore DetailFailureCategory = "segment-store"
	// DetailFailureSegmentResolve covers a refused controller segment resolve.
	DetailFailureSegmentResolve DetailFailureCategory = "segment-resolve"
	// DetailFailureSegmentMismatch covers stored or resolved segment facts that
	// contradict the bytes or the reference metadata they claim to represent.
	DetailFailureSegmentMismatch DetailFailureCategory = "segment-mismatch"
)

// DetailFailureError carries one closed category alongside the underlying
// operational error. Its message is the wrapped error's, so existing
// diagnostics keep their exact safe text.
type DetailFailureError struct {
	Category DetailFailureCategory
	Err      error
}

func (e *DetailFailureError) Error() string { return e.Err.Error() }

func (e *DetailFailureError) Unwrap() error { return e.Err }

// DetailFailureOf reports the closed category a DetailProcessor failure
// carries. An unclassified or foreign error reports DetailFailureRedaction with
// ok false, so a caller that fails closed keeps the redaction reason.
func DetailFailureOf(err error) (DetailFailureCategory, bool) {
	var typed *DetailFailureError
	if errors.As(err, &typed) {
		return typed.Category, true
	}
	return DetailFailureRedaction, false
}

// detailFailure wraps one operational error in its closed category.
func detailFailure(category DetailFailureCategory, format string, args ...any) error {
	return &DetailFailureError{Category: category, Err: fmt.Errorf(format, args...)}
}

// DetailProcessor redacts canonical JSON and selects the exact inline or
// controller-segment representation. Its zero value is unusable.
type DetailProcessor struct {
	store SegmentStore
}

// NewDetailProcessor binds the consumer-owned segment store.
func NewDetailProcessor(store SegmentStore) (*DetailProcessor, error) {
	if nilInterface(store) {
		return nil, fmt.Errorf("sealedexec: detail processor requires a segment store")
	}
	return &DetailProcessor{store: store}, nil
}

// Process redacts raw JSON and stores bytes larger than the inline ceiling
// before returning their reference.
func (p *DetailProcessor) Process(ctx context.Context, raw []byte, protectedValues [][]byte) (contextevent.Detail, error) {
	if ctx == nil {
		return contextevent.Detail{}, fmt.Errorf("sealedexec: process detail: %w", errors.New("nil context"))
	}
	if p == nil || nilInterface(p.store) {
		return contextevent.Detail{}, fmt.Errorf("sealedexec: process detail: missing segment store")
	}
	redacted, err := contextevent.RedactStandardV1(raw, protectedValues)
	if err != nil {
		return contextevent.Detail{}, detailFailure(DetailFailureRedaction, "sealedexec: redact detail: %w", err)
	}
	digest := digestBytes(redacted)
	if len(redacted) <= contextevent.InlineDetailCeiling {
		detail := contextevent.Detail{
			Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON,
			Digest: digest, RedactionProfile: contextevent.RedactionProfileStandard,
			RedactedJSON: append([]byte{}, redacted...),
		}
		if err := detail.Validate(); err != nil {
			return contextevent.Detail{}, detailFailure(DetailFailureRedaction, "sealedexec: validate inline detail: %w", err)
		}
		return detail, nil
	}

	segment := RedactedSegment{
		Schema: redactedSegmentSchemaID, MediaType: contextevent.MediaTypeJSON,
		RedactionProfile: contextevent.RedactionProfileStandard, Digest: digest,
		ByteCount: uint64(len(redacted)), Bytes: append([]byte{}, redacted...),
	}
	stored, err := p.store.StoreRedactedSegment(ctx, segment)
	if err != nil {
		return contextevent.Detail{}, detailFailure(DetailFailureSegmentStore, "sealedexec: store redacted detail segment: %w", err)
	}
	wantReference, err := segmentReference(segment.Digest)
	if err != nil {
		return contextevent.Detail{}, detailFailure(DetailFailureSegmentStore, "sealedexec: derive detail segment reference: %w", err)
	}
	if stored.Schema != storedSegmentSchemaID || stored.Reference != wantReference ||
		stored.MediaType != segment.MediaType || stored.RedactionProfile != segment.RedactionProfile ||
		stored.Digest != segment.Digest || stored.ByteCount != segment.ByteCount {
		return contextevent.Detail{}, detailFailure(DetailFailureSegmentMismatch, "sealedexec: stored detail segment contradicts requested bytes")
	}
	detail := contextevent.Detail{
		Mode: contextevent.DetailSegment, MediaType: stored.MediaType,
		Digest: stored.Digest, RedactionProfile: stored.RedactionProfile,
		ByteCount: stored.ByteCount, Reference: stored.Reference,
	}
	if err := detail.Validate(); err != nil {
		return contextevent.Detail{}, detailFailure(DetailFailureSegmentMismatch, "sealedexec: validate segment detail: %w", err)
	}
	return detail, nil
}

// Resolve returns the exact canonical bytes represented by detail after
// revalidating every inline or controller-segment fact.
func (p *DetailProcessor) Resolve(ctx context.Context, detail contextevent.Detail) ([]byte, error) {
	if ctx == nil {
		return nil, fmt.Errorf("sealedexec: resolve detail: %w", errors.New("nil context"))
	}
	if p == nil || nilInterface(p.store) {
		return nil, fmt.Errorf("sealedexec: resolve detail: missing segment store")
	}
	if err := detail.Validate(); err != nil {
		return nil, fmt.Errorf("sealedexec: resolve invalid detail: %w", err)
	}
	if detail.Mode == contextevent.DetailInline {
		return append([]byte{}, detail.RedactedJSON...), nil
	}

	segment, err := p.store.ResolveRedactedSegment(ctx, detail.Reference)
	if err != nil {
		return nil, detailFailure(DetailFailureSegmentResolve, "sealedexec: resolve redacted detail segment: %w", err)
	}
	if _, err := redactedSegmentToWire(segment); err != nil {
		return nil, detailFailure(DetailFailureSegmentMismatch, "sealedexec: validate resolved detail segment: %w", err)
	}
	wantReference, err := segmentReference(segment.Digest)
	if err != nil {
		return nil, detailFailure(DetailFailureSegmentResolve, "sealedexec: derive resolved detail segment reference: %w", err)
	}
	if segment.Schema != redactedSegmentSchemaID || wantReference != detail.Reference ||
		segment.MediaType != detail.MediaType || segment.RedactionProfile != detail.RedactionProfile ||
		segment.Digest != detail.Digest || segment.ByteCount != detail.ByteCount {
		return nil, detailFailure(DetailFailureSegmentMismatch, "sealedexec: resolved detail segment contradicts reference metadata")
	}
	return append([]byte{}, segment.Bytes...), nil
}
