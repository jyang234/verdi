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
		if _, err := processor.Process(nil, []byte(`{"value":1}`), [][]byte{[]byte("classified")}); err == nil {
			t.Fatal("Process accepted nil context")
		}
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
	storeCalls   int
	resolveCalls int
	storeErr     error
	resolveErr   error
}

func (s *detailStoreStub) StoreRedactedSegment(_ context.Context, _ RedactedSegment) (StoredSegment, error) {
	s.storeCalls++
	return StoredSegment{}, s.storeErr
}

func (s *detailStoreStub) ResolveRedactedSegment(_ context.Context, _ string) (RedactedSegment, error) {
	s.resolveCalls++
	return RedactedSegment{}, s.resolveErr
}
