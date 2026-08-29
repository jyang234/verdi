package contextevent_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/sealedexec"
)

func TestContextEventRedactionContinuityContract_Behavioral(t *testing.T) {
	t.Run("sensitive key normalization and hidden thinking", func(t *testing.T) {
		protected := [][]byte{[]byte("classified-value")}
		keys := []string{
			"API_KEY", "Authorization", "coo-kie", "Pass_word", "SECRET", "to-ken",
			"access_token", "Refresh-Token", "SESSION_TOKEN", "session-id", "Thinking",
			"redacted_thinking", "SIGNATURE",
		}
		for _, key := range keys {
			key := key
			t.Run(key, func(t *testing.T) {
				raw := []byte(fmt.Sprintf(`{"%s":{"nested":"must disappear"}}`, key))
				got, err := contextevent.RedactStandardV1(raw, protected)
				if err != nil {
					t.Fatal("RedactStandardV1 rejected a sensitive-key fixture")
				}
				want := []byte(fmt.Sprintf(`{"%s":"[REDACTED]"}`, key))
				if !bytes.Equal(got, want) {
					failByteMismatch(t, "sensitive-key redaction mismatch", got, want)
				}
			})
		}

		// The redactor cannot authenticate which protected member is the
		// activated API key. Sensitive-key handling remains independent of set
		// membership; activation owns the membership proof.
		got, err := contextevent.RedactStandardV1(
			[]byte(`{"api_key":"omitted-from-classified-set"}`),
			[][]byte{[]byte("different-classified-value")},
		)
		if err != nil {
			t.Fatal("RedactStandardV1 rejected an omitted API-key fixture")
		}
		if want := `{"api_key":"[REDACTED]"}`; string(got) != want {
			failByteMismatch(t, "omitted API-key redaction mismatch", got, []byte(want))
		}
	})

	t.Run("literal substring overlap recursive walk and idempotency", func(t *testing.T) {
		protected := [][]byte{
			[]byte("secret-one"),
			[]byte("abcde"),
			[]byte("cdefg"),
			[]byte("fixed-secret"),
			[]byte("secret-one"),
		}
		raw := []byte(`{"array":["secret-one",{"value":"xxfixed-secretyy"}],"number":1.25,"text":"pre secret-one and abcdefg post"}`)
		want := []byte(`{"array":["[REDACTED]",{"value":"xx[REDACTED]yy"}],"number":1.25,"text":"pre [REDACTED] and [REDACTED] post"}`)
		got, err := contextevent.RedactStandardV1(raw, protected)
		if err != nil {
			t.Fatal("RedactStandardV1 rejected a recursive overlap fixture")
		}
		if !bytes.Equal(got, want) {
			failByteMismatch(t, "recursive overlap redaction mismatch", got, want)
		}
		again, err := contextevent.RedactStandardV1(got, protected)
		if err != nil {
			t.Fatal("RedactStandardV1 rejected already-redacted bytes")
		}
		if !bytes.Equal(again, want) {
			failByteMismatch(t, "idempotent redaction mismatch", again, want)
		}
	})

	t.Run("fixed payload values refuse protected bytes", func(t *testing.T) {
		protected := [][]byte{[]byte("fixed-secret")}
		for _, value := range []string{"message-fixed-secret-id", "fixed-secret-tool-name"} {
			if err := contextevent.ValidateFixedPayloadValue(value, protected); err == nil {
				t.Fatal("ValidateFixedPayloadValue accepted protected bytes")
			} else if strings.Contains(err.Error(), "fixed-secret") || strings.Contains(err.Error(), value) {
				t.Fatal("fixed-field error disclosed protected input")
			}
		}
		if err := contextevent.ValidateFixedPayloadValue("safe-provider-id", protected); err != nil {
			t.Fatal("ValidateFixedPayloadValue rejected a safe value")
		}
	})

	t.Run("classification and protected key failures disclose no input", func(t *testing.T) {
		rows := []struct {
			name      string
			raw       []byte
			protected [][]byte
			secret    string
		}{
			{name: "classification unavailable", raw: []byte(`{"value":"safe"}`), protected: nil},
			{name: "empty classified set", raw: []byte(`{"value":"safe"}`), protected: [][]byte{}},
			{name: "empty classified member", raw: []byte(`{"value":"safe"}`), protected: [][]byte{[]byte("classified"), []byte{}}},
			{name: "secret in key", raw: []byte(`{"prefix-secret-one-suffix":"value"}`), protected: [][]byte{[]byte("secret-one")}, secret: "secret-one"},
			{name: "replacement token is protected", raw: []byte(`{"token":"value"}`), protected: [][]byte{[]byte("[REDACTED]")}, secret: "[REDACTED]"},
		}
		for _, row := range rows {
			row := row
			t.Run(row.name, func(t *testing.T) {
				if _, err := contextevent.RedactStandardV1(row.raw, row.protected); err == nil {
					t.Fatal("RedactStandardV1 accepted unsafe classification/input")
				} else if row.secret != "" && strings.Contains(err.Error(), row.secret) {
					t.Fatal("redaction error disclosed protected bytes")
				}
			})
		}
	})

	t.Run("malformed input reduces only to fixed digest and closed reason", func(t *testing.T) {
		rows := []struct {
			name   string
			raw    []byte
			reason string
		}{
			{name: "malformed UTF-8", raw: []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}, reason: "malformed-foreign-frame"},
			{name: "malformed JSON", raw: []byte(`{"value":"secret-one"`), reason: "malformed-foreign-frame"},
			{name: "duplicate key", raw: []byte(`{"value":1,"value":2}`), reason: "unknown-foreign-field"},
			{name: "trailing data", raw: []byte(`{"value":1}{}`), reason: "malformed-foreign-frame"},
		}
		for _, row := range rows {
			row := row
			t.Run(row.name, func(t *testing.T) {
				if _, err := contextevent.RedactStandardV1(row.raw, [][]byte{[]byte("secret-one")}); err == nil {
					t.Fatal("RedactStandardV1 accepted malformed/non-unique input")
				} else if strings.Contains(err.Error(), "secret-one") {
					t.Fatal("redaction error disclosed raw input")
				}

				got, err := contextevent.SafeRawDetail(row.raw, row.reason)
				if err != nil {
					t.Fatal("SafeRawDetail rejected a closed reason")
				}
				sum := sha256.Sum256(row.raw)
				want := fmt.Sprintf(`{"raw_digest":"sha256:%x","reason":%q}`, sum, row.reason)
				if string(got) != want {
					failByteMismatch(t, "safe raw-detail mismatch", got, []byte(want))
				}
				if bytes.Contains(got, row.raw) {
					t.Fatal("safe detail contains original raw bytes")
				}
			})
		}
		if _, err := contextevent.SafeRawDetail([]byte("private"), "caller-invented-reason"); err == nil {
			t.Fatal("SafeRawDetail accepted an open reason")
		} else if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "caller-invented-reason") {
			t.Fatal("SafeRawDetail error disclosed caller input")
		}
	})

	t.Run("failure diagnostics do not disclose protected fixture members", func(t *testing.T) {
		protected := [][]byte{[]byte("secret-one"), []byte("fixed-secret")}
		if os.Getenv("VERDI_TEST_REDACTION_DIAGNOSTIC_HELPER") == "1" {
			failByteMismatch(t, "forced redaction mismatch", protected[0], protected[1])
		}

		executable, err := os.Executable()
		if err != nil {
			t.Fatal("locate current test executable")
		}
		cmd := exec.Command(executable,
			"-test.run=^TestContextEventRedactionContinuityContract_Behavioral$/^failure_diagnostics_do_not_disclose_protected_fixture_members$",
			"-test.v",
		)
		cmd.Env = append(os.Environ(), "VERDI_TEST_REDACTION_DIAGNOSTIC_HELPER=1")
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatal("diagnostic helper unexpectedly passed")
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			t.Fatal("diagnostic helper did not fail through a test assertion")
		}
		for _, member := range protected {
			if bytes.Contains(output, member) {
				t.Fatal("failure diagnostic disclosed a protected fixture member")
			}
		}
	})

	t.Run("inline and segment boundary store before reference and exact resolve", func(t *testing.T) {
		store := newMemorySegmentStore()
		processor, err := sealedexec.NewDetailProcessor(store)
		if err != nil {
			t.Fatalf("NewDetailProcessor: %v", err)
		}
		protected := [][]byte{[]byte("classified-value")}

		inlineRaw := exactJSONSize(t, contextevent.InlineDetailCeiling)
		inline, err := processor.Process(context.Background(), inlineRaw, protected)
		if err != nil {
			t.Fatal("Process rejected the inline boundary fixture")
		}
		if inline.Mode != contextevent.DetailInline || !bytes.Equal(inline.RedactedJSON, inlineRaw) || store.storeCalls != 0 {
			t.Fatalf("inline detail/store calls = %#v/%d", inline, store.storeCalls)
		}
		resolvedInline, err := processor.Resolve(context.Background(), inline)
		if err != nil {
			t.Fatal("Resolve rejected the inline boundary fixture")
		}
		if !bytes.Equal(resolvedInline, inlineRaw) || store.resolveCalls != 0 {
			t.Fatalf("resolved inline/store calls mismatch")
		}

		segmentRaw := exactJSONSize(t, contextevent.InlineDetailCeiling+1)
		segment, err := processor.Process(context.Background(), segmentRaw, protected)
		if err != nil {
			t.Fatal("Process rejected the segment boundary fixture")
		}
		if !store.storeReturned || store.storeCalls != 1 {
			t.Fatal("Process returned a segment reference before the store completed")
		}
		if segment.Mode != contextevent.DetailSegment || segment.ByteCount != uint64(len(segmentRaw)) || segment.RedactedJSON != nil {
			t.Fatalf("segment detail = %#v", segment)
		}
		if err := segment.Validate(); err != nil {
			t.Fatal("Process returned invalid segment metadata")
		}
		resolved, err := processor.Resolve(context.Background(), segment)
		if err != nil {
			t.Fatal("Resolve rejected the stored segment fixture")
		}
		if !bytes.Equal(resolved, segmentRaw) || store.resolveCalls != 1 {
			t.Fatalf("resolved segment mismatch")
		}

		replayed, err := processor.Process(context.Background(), segmentRaw, protected)
		if err != nil {
			t.Fatal("Process rejected an exact segment replay")
		}
		if !reflect.DeepEqual(replayed, segment) || len(store.rows) != 1 {
			t.Fatalf("idempotent detail/row count = %#v/%d, want %#v/1", replayed, len(store.rows), segment)
		}
	})

	t.Run("detail representation boundary rejects contradictory external values", func(t *testing.T) {
		oversizedInlineJSON := exactJSONSize(t, contextevent.InlineDetailCeiling+1)
		oversizedInlineSum := sha256.Sum256(oversizedInlineJSON)
		oversizedInline := contextevent.Detail{
			Mode:             contextevent.DetailInline,
			MediaType:        contextevent.MediaTypeJSON,
			Digest:           fmt.Sprintf("sha256:%x", oversizedInlineSum),
			RedactionProfile: contextevent.RedactionProfileStandard,
			RedactedJSON:     oversizedInlineJSON,
		}
		undersizedSegment := contextevent.Detail{
			Mode:             contextevent.DetailSegment,
			MediaType:        contextevent.MediaTypeJSON,
			Digest:           "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			RedactionProfile: contextevent.RedactionProfileStandard,
			ByteCount:        contextevent.InlineDetailCeiling,
			Reference:        "controller-segment/sha256/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}

		for name, detail := range map[string]contextevent.Detail{
			"oversized inline":   oversizedInline,
			"undersized segment": undersizedSegment,
		} {
			t.Run(name, func(t *testing.T) {
				if err := detail.Validate(); err == nil {
					t.Fatal("Detail.Validate accepted a contradictory representation")
				}
			})
		}

		store := newMemorySegmentStore()
		processor, err := sealedexec.NewDetailProcessor(store)
		if err != nil {
			t.Fatal("NewDetailProcessor rejected a concrete store")
		}
		for name, detail := range map[string]contextevent.Detail{
			"oversized inline":   oversizedInline,
			"undersized segment": undersizedSegment,
		} {
			t.Run("resolve "+name, func(t *testing.T) {
				if _, err := processor.Resolve(context.Background(), detail); err == nil {
					t.Fatal("Resolve accepted a contradictory representation")
				}
			})
		}
		if store.resolveCalls != 0 {
			t.Fatal("Resolve sent an invalid segment representation to storage")
		}
	})

	t.Run("store collision and response mismatch refuse a reference", func(t *testing.T) {
		protected := [][]byte{[]byte("classified-value")}
		raw := exactJSONSize(t, contextevent.InlineDetailCeiling+1)
		rows := []struct {
			name  string
			store *memorySegmentStore
		}{
			{name: "collision", store: &memorySegmentStore{rows: map[string]sealedexec.RedactedSegment{}, storeErr: errors.New("controller collision")}},
			{name: "stored digest mismatch", store: &memorySegmentStore{rows: map[string]sealedexec.RedactedSegment{}, mismatchStoredDigest: true}},
		}
		for _, row := range rows {
			row := row
			t.Run(row.name, func(t *testing.T) {
				processor, err := sealedexec.NewDetailProcessor(row.store)
				if err != nil {
					t.Fatalf("NewDetailProcessor: %v", err)
				}
				if detail, err := processor.Process(context.Background(), raw, protected); err == nil {
					t.Fatalf("Process returned unsafe detail %#v", detail)
				}
			})
		}
	})

	t.Run("missing and mismatched resolved segments fail closed", func(t *testing.T) {
		store := newMemorySegmentStore()
		processor, err := sealedexec.NewDetailProcessor(store)
		if err != nil {
			t.Fatalf("NewDetailProcessor: %v", err)
		}
		detail, err := processor.Process(context.Background(), exactJSONSize(t, contextevent.InlineDetailCeiling+1), [][]byte{[]byte("classified-value")})
		if err != nil {
			t.Fatal("Process rejected the missing-segment setup fixture")
		}

		delete(store.rows, detail.Reference)
		if _, err := processor.Resolve(context.Background(), detail); err == nil {
			t.Fatal("Resolve accepted a missing segment row")
		}

		store = newMemorySegmentStore()
		processor, err = sealedexec.NewDetailProcessor(store)
		if err != nil {
			t.Fatalf("NewDetailProcessor replacement: %v", err)
		}
		detail, err = processor.Process(context.Background(), exactJSONSize(t, contextevent.InlineDetailCeiling+1), [][]byte{[]byte("classified-value")})
		if err != nil {
			t.Fatal("Process rejected the mismatched-segment setup fixture")
		}
		stored := store.rows[detail.Reference]
		stored.MediaType = "text/plain"
		store.rows[detail.Reference] = stored
		if _, err := processor.Resolve(context.Background(), detail); err == nil {
			t.Fatal("Resolve accepted mismatched segment metadata")
		}
	})
}

func failByteMismatch(t *testing.T, fact string, got, want []byte) {
	t.Helper()
	gotDigest := sha256.Sum256(got)
	wantDigest := sha256.Sum256(want)
	t.Fatalf("%s: got_digest=sha256:%x want_digest=sha256:%x", fact, gotDigest, wantDigest)
}

func exactJSONSize(t *testing.T, size int) []byte {
	t.Helper()
	const framing = len(`{"value":""}`)
	if size < framing {
		t.Fatalf("requested JSON size %d is below framing", size)
	}
	raw := []byte(`{"value":"` + strings.Repeat("x", size-framing) + `"}`)
	if len(raw) != size {
		t.Fatalf("fixture size = %d, want %d", len(raw), size)
	}
	return raw
}

type memorySegmentStore struct {
	rows                 map[string]sealedexec.RedactedSegment
	storeCalls           int
	resolveCalls         int
	storeReturned        bool
	storeErr             error
	mismatchStoredDigest bool
}

func newMemorySegmentStore() *memorySegmentStore {
	return &memorySegmentStore{rows: map[string]sealedexec.RedactedSegment{}}
}

func (s *memorySegmentStore) StoreRedactedSegment(_ context.Context, segment sealedexec.RedactedSegment) (sealedexec.StoredSegment, error) {
	s.storeCalls++
	if s.storeErr != nil {
		return sealedexec.StoredSegment{}, s.storeErr
	}
	reference := strings.Replace(segment.Digest, "sha256:", "controller-segment/sha256/", 1)
	if existing, ok := s.rows[reference]; ok && !equalRedactedSegment(existing, segment) {
		return sealedexec.StoredSegment{}, errors.New("controller collision")
	}
	s.rows[reference] = cloneRedactedSegment(segment)
	stored := sealedexec.StoredSegment{
		Schema: "verdi.context-redacted-segment-stored/v1", Reference: reference,
		MediaType: segment.MediaType, RedactionProfile: segment.RedactionProfile,
		Digest: segment.Digest, ByteCount: segment.ByteCount,
	}
	if s.mismatchStoredDigest {
		other := append([]byte{}, segment.Bytes...)
		other[len(other)-3] = 'y'
		sum := sha256.Sum256(other)
		stored.Digest = fmt.Sprintf("sha256:%x", sum)
	}
	s.storeReturned = true
	return stored, nil
}

func (s *memorySegmentStore) ResolveRedactedSegment(_ context.Context, reference string) (sealedexec.RedactedSegment, error) {
	s.resolveCalls++
	segment, ok := s.rows[reference]
	if !ok {
		return sealedexec.RedactedSegment{}, errors.New("segment missing")
	}
	return cloneRedactedSegment(segment), nil
}

func cloneRedactedSegment(segment sealedexec.RedactedSegment) sealedexec.RedactedSegment {
	segment.Bytes = append([]byte{}, segment.Bytes...)
	return segment
}

func equalRedactedSegment(left, right sealedexec.RedactedSegment) bool {
	return left.Schema == right.Schema && left.MediaType == right.MediaType &&
		left.RedactionProfile == right.RedactionProfile && left.Digest == right.Digest &&
		left.ByteCount == right.ByteCount && bytes.Equal(left.Bytes, right.Bytes)
}
