package sealedexec

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextevent"
)

func TestDetailProcessorConstructionAndOperationalFailures(t *testing.T) {
	t.Run("constructor requires a concrete store", func(t *testing.T) {
		var typedNil *detailStoreStub
		for name, store := range map[string]SegmentStore{"nil": nil, "typed nil": typedNil} {
			t.Run(name, func(t *testing.T) {
				if processor, err := NewDetailProcessor(store); err == nil || processor != nil {
					t.Fatalf("NewDetailProcessor = %#v, %v; want nil, error", processor, err)
				}
			})
		}
		if processor, err := NewDetailProcessor(&detailStoreStub{}); err != nil || processor == nil {
			t.Fatalf("NewDetailProcessor concrete store = %#v, %v", processor, err)
		}
	})

	t.Run("nil contexts and invalid detail fail before store I/O", func(t *testing.T) {
		store := &detailStoreStub{}
		processor, err := NewDetailProcessor(store)
		if err != nil {
			t.Fatalf("NewDetailProcessor: %v", err)
		}
		//nolint:staticcheck // deliberately exercise the fail-closed nil-context seam.
		if _, err := processor.Process(nil, []byte(`{"value":1}`), [][]byte{[]byte("classified")}); err == nil {
			t.Fatal("Process accepted nil context")
		}
		//nolint:staticcheck // deliberately exercise the fail-closed nil-context seam.
		if _, err := processor.Resolve(nil, contextevent.Detail{}); err == nil {
			t.Fatal("Resolve accepted nil context")
		}
		if _, err := processor.Resolve(context.Background(), contextevent.Detail{}); err == nil {
			t.Fatal("Resolve accepted invalid detail")
		}
		if store.storeCalls != 0 || store.resolveCalls != 0 {
			t.Fatalf("invalid inputs reached store: store=%d resolve=%d", store.storeCalls, store.resolveCalls)
		}
	})

	t.Run("store and resolve errors are operational", func(t *testing.T) {
		store := &detailStoreStub{storeErr: errors.New("store unavailable"), resolveErr: errors.New("resolve unavailable")}
		processor, err := NewDetailProcessor(store)
		if err != nil {
			t.Fatalf("NewDetailProcessor: %v", err)
		}
		raw := []byte(`{"value":"` + strings.Repeat("x", contextevent.InlineDetailCeiling) + `"}`)
		if _, err := processor.Process(context.Background(), raw, [][]byte{[]byte("classified")}); err == nil {
			t.Fatal("Process accepted store failure")
		}

		detail := contextevent.Detail{
			Mode: contextevent.DetailSegment, MediaType: contextevent.MediaTypeJSON,
			RedactionProfile: contextevent.RedactionProfileStandard,
			Digest:           "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			ByteCount:        contextevent.InlineDetailCeiling + 1,
			Reference:        "controller-segment/sha256/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}
		if _, err := processor.Resolve(context.Background(), detail); err == nil {
			t.Fatal("Resolve accepted resolve failure")
		}
	})
}

type detailStoreStub struct {
	storeCalls     int
	resolveCalls   int
	storeErr       error
	resolveErr     error
	contradict     bool
	corruptResolve bool
	segments       map[string]RedactedSegment
}

func (s *detailStoreStub) StoreRedactedSegment(_ context.Context, seg RedactedSegment) (StoredSegment, error) {
	s.storeCalls++
	if s.storeErr != nil {
		return StoredSegment{}, s.storeErr
	}
	reference, err := segmentReference(seg.Digest)
	if err != nil {
		return StoredSegment{}, err
	}
	if s.segments != nil {
		s.segments[reference] = seg
	}
	stored := StoredSegment{
		Schema: storedSegmentSchemaID, Reference: reference, MediaType: seg.MediaType,
		RedactionProfile: seg.RedactionProfile, Digest: seg.Digest, ByteCount: seg.ByteCount,
	}
	if s.contradict {
		stored.ByteCount++
	}
	return stored, nil
}

func (s *detailStoreStub) ResolveRedactedSegment(_ context.Context, reference string) (RedactedSegment, error) {
	s.resolveCalls++
	if s.resolveErr != nil {
		return RedactedSegment{}, s.resolveErr
	}
	seg, ok := s.segments[reference]
	if !ok {
		return RedactedSegment{}, errors.New("detailStoreStub: unknown reference " + reference)
	}
	if s.corruptResolve {
		seg.ByteCount++
	}
	return seg, nil
}

// TestDetailProcessorFailureCategories proves every DetailProcessor failure
// carries one closed typed category, so call sites map Amendment 002 §5's
// redaction and segment reasons without ever matching an error string.
func TestDetailProcessorFailureCategories(t *testing.T) {
	oversized := []byte(`{"value":"` + strings.Repeat("x", contextevent.InlineDetailCeiling) + `"}`)
	classified := [][]byte{[]byte("classified")}

	t.Run("store failure is categorized segment-store", func(t *testing.T) {
		processor := mustDetailProcessor(t, &detailStoreStub{storeErr: errors.New("store unavailable")})
		_, err := processor.Process(context.Background(), oversized, classified)
		assertDetailCategory(t, err, DetailFailureSegmentStore, true)
	})

	t.Run("contradicting stored facts are categorized segment-mismatch", func(t *testing.T) {
		processor := mustDetailProcessor(t, &detailStoreStub{contradict: true})
		_, err := processor.Process(context.Background(), oversized, classified)
		assertDetailCategory(t, err, DetailFailureSegmentMismatch, true)
	})

	t.Run("resolve failure is categorized segment-resolve", func(t *testing.T) {
		processor := mustDetailProcessor(t, &detailStoreStub{resolveErr: errors.New("resolve unavailable")})
		detail := contextevent.Detail{
			Mode: contextevent.DetailSegment, MediaType: contextevent.MediaTypeJSON,
			RedactionProfile: contextevent.RedactionProfileStandard,
			Digest:           "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			ByteCount:        contextevent.InlineDetailCeiling + 1,
			Reference:        "controller-segment/sha256/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}
		_, err := processor.Resolve(context.Background(), detail)
		assertDetailCategory(t, err, DetailFailureSegmentResolve, true)
	})

	t.Run("resolved segment contradiction is categorized segment-mismatch", func(t *testing.T) {
		store := &detailStoreStub{segments: map[string]RedactedSegment{}}
		processor := mustDetailProcessor(t, store)
		detail, err := processor.Process(context.Background(), oversized, classified)
		if err != nil {
			t.Fatalf("Process: %v", err)
		}
		store.corruptResolve = true
		_, err = processor.Resolve(context.Background(), detail)
		assertDetailCategory(t, err, DetailFailureSegmentMismatch, true)
	})

	t.Run("redaction inability is categorized redaction", func(t *testing.T) {
		processor := mustDetailProcessor(t, &detailStoreStub{})
		_, err := processor.Process(context.Background(), []byte("not json"), classified)
		assertDetailCategory(t, err, DetailFailureRedaction, true)
	})

	t.Run("unclassified operational failures default to redaction", func(t *testing.T) {
		processor := mustDetailProcessor(t, &detailStoreStub{})
		//nolint:staticcheck // deliberately exercise the fail-closed nil-context seam.
		_, err := processor.Process(nil, []byte(`{"value":1}`), classified)
		assertDetailCategory(t, err, DetailFailureRedaction, false)
		// A foreign error is never classified, and nil is never a failure.
		assertDetailCategory(t, errors.New("unrelated"), DetailFailureRedaction, false)
		assertDetailCategory(t, nil, DetailFailureRedaction, false)
	})

	t.Run("inline success carries no failure category", func(t *testing.T) {
		processor := mustDetailProcessor(t, &detailStoreStub{})
		if _, err := processor.Process(context.Background(), []byte(`{"value":1}`), classified); err != nil {
			t.Fatalf("Process inline: %v", err)
		}
	})
}

func mustDetailProcessor(t *testing.T, store SegmentStore) *DetailProcessor {
	t.Helper()
	processor, err := NewDetailProcessor(store)
	if err != nil {
		t.Fatalf("NewDetailProcessor: %v", err)
	}
	return processor
}

func assertDetailCategory(t *testing.T, err error, want DetailFailureCategory, wantClassified bool) {
	t.Helper()
	if wantClassified && err == nil {
		t.Fatal("expected a failure, got nil")
	}
	got, classified := DetailFailureOf(err)
	if classified != wantClassified || got != want {
		t.Fatalf("DetailFailureOf(%v) = %q, %t; want %q, %t", err, got, classified, want, wantClassified)
	}
}
