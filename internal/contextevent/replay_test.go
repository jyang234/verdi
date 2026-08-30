package contextevent

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// TestContextEventEnvelopeContract_Behavioral is the frozen Amendment 002 §9
// behavioral envelope producer. It owns exact retry, lost acknowledgment,
// conflict, gap, restart, resume/suspension prefix, multi-flight global order,
// and the receipt detail cutoff.
func TestContextEventEnvelopeContract_Behavioral(t *testing.T) {
	t.Parallel()

	base := replayFixture(t)

	t.Run("exact-retry-returns-the-original-acknowledgment", func(t *testing.T) {
		got, err := ValidateReplay(base.event, base.ack, base.event)
		if err != nil {
			t.Fatalf("ValidateReplay(identical) error = %v", err)
		}
		if got != base.ack {
			t.Fatalf("ValidateReplay(identical) ack = %#v, want %#v", got, base.ack)
		}
	})

	t.Run("lost-live-acknowledgment-allocates-no-new-global-sequence", func(t *testing.T) {
		// The live caller lost the ack and retries the exact retained bytes.
		// §7 row 2: the original ack is returned with no write and no new
		// global sequence, so the replayed order is byte-identical.
		retried, err := ValidateReplay(base.event, base.ack, base.event)
		if err != nil {
			t.Fatalf("ValidateReplay(lost ack retry) error = %v", err)
		}
		if retried.GlobalSequence != base.ack.GlobalSequence {
			t.Fatalf("replayed global sequence = %d, want %d", retried.GlobalSequence, base.ack.GlobalSequence)
		}
		encodedOriginal, err := EncodeEventAck(base.ack)
		if err != nil {
			t.Fatalf("EncodeEventAck original: %v", err)
		}
		encodedRetried, err := EncodeEventAck(retried)
		if err != nil {
			t.Fatalf("EncodeEventAck retried: %v", err)
		}
		if !bytes.Equal(encodedOriginal, encodedRetried) {
			t.Fatal("replayed acknowledgment bytes differ from the original")
		}
	})

	t.Run("conflicting-duplicate-fails-closed", func(t *testing.T) {
		conflicts := []struct {
			name    string
			produce func(t *testing.T) Event
		}{
			{"different kind", func(t *testing.T) Event {
				other := eventFixture(t, KindToolCall, AdapterCodex)
				other.SourceSequence = base.event.SourceSequence
				other.PriorEventDigest = base.event.PriorEventDigest
				return canonicalize(t, other)
			}},
			{"different source sequence", func(t *testing.T) Event {
				shifted := base.event
				shifted.SourceSequence = base.event.SourceSequence + 1
				shifted.PriorRevision = nil
				shifted.EventDigest = ""
				return canonicalize(t, shifted)
			}},
			{"different adapter", func(t *testing.T) Event {
				adapted := base.event
				payload := *adapted.Payload.(*PromptPayload)
				adapted.Payload = &payload
				adapted.Adapter = AdapterClaude
				adapted.EventDigest = ""
				return canonicalize(t, adapted)
			}},
			{"different session", func(t *testing.T) Event {
				other := base.event
				payload := *other.Payload.(*PromptPayload)
				other.Payload = &payload
				other.Session = "session-2"
				other.EventDigest = ""
				return canonicalize(t, other)
			}},
			{"different occurred-at stamp", func(t *testing.T) Event {
				other := base.event
				payload := *other.Payload.(*PromptPayload)
				other.Payload = &payload
				other.OccurredAt = "2026-08-27T16:00:00.123456788Z"
				other.EventDigest = ""
				return canonicalize(t, other)
			}},
		}
		for _, tt := range conflicts {
			t.Run(tt.name, func(t *testing.T) {
				if _, err := ValidateReplay(base.event, base.ack, tt.produce(t)); err == nil {
					t.Fatalf("ValidateReplay(%s) error = nil, want a closed refusal", tt.name)
				}
			})
		}
	})

	t.Run("malformed-replay-operand-fails-closed", func(t *testing.T) {
		bad := base.event
		bad.EventDigest = digestA // contradicts the content-addressed digest
		if _, err := ValidateReplay(bad, base.ack, base.event); err == nil {
			t.Fatal("ValidateReplay(invalid existing) error = nil, want a closed refusal")
		}
		if _, err := ValidateReplay(base.event, base.ack, bad); err == nil {
			t.Fatal("ValidateReplay(invalid incoming) error = nil, want a closed refusal")
		}
	})

	t.Run("restart-distinguishes-committed-from-absent-prefix", func(t *testing.T) {
		committed := prefixFixture()
		committed.TerminalSourceSequence = 4
		committed.TerminalGlobalSequence = 41
		committed.TerminalEventDigest = digestB
		absent := committed
		// The ambiguous event did not commit: the acknowledged prefix ends one
		// source sequence earlier at its own terminal facts.
		absent.TerminalSourceSequence = 3
		absent.TerminalGlobalSequence = 37
		absent.TerminalEventDigest = digestC

		committedDigest, err := EventPrefixDigest(committed)
		if err != nil {
			t.Fatalf("EventPrefixDigest(committed) error = %v", err)
		}
		absentDigest, err := EventPrefixDigest(absent)
		if err != nil {
			t.Fatalf("EventPrefixDigest(absent) error = %v", err)
		}
		if committedDigest == absentDigest {
			t.Fatal("committed and absent restart prefixes share one digest")
		}
	})

	t.Run("multi-flight-global-order-is-strictly-increasing-not-consecutive", func(t *testing.T) {
		// VATC allocates one never-resetting global order across flights, so a
		// single flight observes gaps. Adjacent source order with a global gap
		// is valid; a global regression or repeat is not.
		first := ackFor(t, base.event, 7)
		second := base.ack
		second.SourceSequence = base.event.SourceSequence + 1
		second.GlobalSequence = 19
		if err := requireAdjacentAck(first, second); err != nil {
			t.Fatalf("non-consecutive global order rejected: %v", err)
		}
		for _, tt := range []struct {
			name   string
			global uint64
		}{{"regression", 6}, {"repeat", 7}} {
			t.Run(tt.name, func(t *testing.T) {
				bad := second
				bad.GlobalSequence = tt.global
				if err := requireAdjacentAck(first, bad); err == nil {
					t.Fatalf("global %s accepted, want refusal", tt.name)
				}
			})
		}
		gapped := first
		gapped.SourceSequence = base.event.SourceSequence + 3
		gapped.GlobalSequence = 25
		if err := requireAdjacentAck(second, gapped); err == nil {
			t.Fatal("source-sequence gap accepted, want refusal")
		}
	})

	t.Run("resume-and-suspension-prefix-digest-binds-eleven-fields", func(t *testing.T) {
		prefix := prefixFixture()
		digest, err := EventPrefixDigest(prefix)
		if err != nil {
			t.Fatalf("EventPrefixDigest error = %v", err)
		}
		if len(digest) != 71 || !strings.HasPrefix(digest, "sha256:") {
			t.Fatalf("EventPrefixDigest = %q, want a 71-character sha256 digest", digest)
		}
		again, err := EventPrefixDigest(prefix)
		if err != nil {
			t.Fatalf("EventPrefixDigest second error = %v", err)
		}
		if digest != again {
			t.Fatal("EventPrefixDigest is not deterministic")
		}

		t.Run("wrong schema fails closed", func(t *testing.T) {
			for _, schema := range []string{"", EventSchemaID, EventPrefixSchemaID + "x"} {
				bad := prefix
				bad.Schema = schema
				if _, err := EventPrefixDigest(bad); err == nil {
					t.Fatalf("EventPrefixDigest(schema=%q) error = nil, want refusal", schema)
				}
			}
		})

		t.Run("every field is independently bound", func(t *testing.T) {
			mutations := []struct {
				name   string
				mutate func(EventPrefix) EventPrefix
			}{
				{"flight", func(p EventPrefix) EventPrefix { p.Flight = "flight-2"; return p }},
				{"lane", func(p EventPrefix) EventPrefix { p.Lane = "reviewer"; return p }},
				{"session", func(p EventPrefix) EventPrefix { p.Session = "session-2"; return p }},
				{"epoch", func(p EventPrefix) EventPrefix { p.Epoch = "epoch-2"; return p }},
				{"manifest_revision", func(p EventPrefix) EventPrefix { p.ManifestRevision = 2; return p }},
				{"manifest_digest", func(p EventPrefix) EventPrefix { p.ManifestDigest = digestB; return p }},
				{"terminal_source_sequence", func(p EventPrefix) EventPrefix { p.TerminalSourceSequence = 11; return p }},
				{"terminal_global_sequence", func(p EventPrefix) EventPrefix { p.TerminalGlobalSequence = 21; return p }},
				{"terminal_event_digest", func(p EventPrefix) EventPrefix { p.TerminalEventDigest = digestC; return p }},
				{"completed_event_chain_root", func(p EventPrefix) EventPrefix { p.CompletedEventChainRoot = digestA; return p }},
			}
			if len(mutations)+1 != 11 {
				t.Fatalf("prefix mutation witnesses cover %d of 11 fields", len(mutations)+1)
			}
			seen := map[string]string{digest: "base"}
			for _, tt := range mutations {
				mutated, err := EventPrefixDigest(tt.mutate(prefix))
				if err != nil {
					t.Fatalf("EventPrefixDigest(%s) error = %v", tt.name, err)
				}
				if prior, ok := seen[mutated]; ok {
					t.Errorf("prefix mutation %q collides with %q", tt.name, prior)
				}
				seen[mutated] = tt.name
			}
		})
	})

	t.Run("receipt-detail-cutoff-selects-inline-or-segment", func(t *testing.T) {
		inline := exactCanonicalJSON(t, InlineDetailCeiling)
		if err := sizedInlineDetail(inline).Validate(); err != nil {
			t.Fatalf("inline detail at %d bytes rejected: %v", InlineDetailCeiling, err)
		}
		oversized := exactCanonicalJSON(t, InlineDetailCeiling+1)
		if err := sizedInlineDetail(oversized).Validate(); err == nil {
			t.Fatalf("inline detail at %d bytes accepted, want refusal", InlineDetailCeiling+1)
		}
		segment := Detail{
			Mode: DetailSegment, MediaType: MediaTypeJSON, Digest: digestOf(oversized),
			RedactionProfile: RedactionProfileStandard, ByteCount: uint64(len(oversized)),
			Reference: "controller-segment/sha256/" + strings.Repeat("a", 64),
		}
		if err := segment.Validate(); err != nil {
			t.Fatalf("segment detail at %d bytes rejected: %v", len(oversized), err)
		}
		undersized := segment
		undersized.ByteCount = InlineDetailCeiling
		if err := undersized.Validate(); err == nil {
			t.Fatalf("segment detail at %d bytes accepted, want refusal", InlineDetailCeiling)
		}
	})
}

type replayCase struct {
	event Event
	ack   EventAck
}

func replayFixture(t *testing.T) replayCase {
	t.Helper()
	event := eventFixture(t, KindPrompt, AdapterCodex)
	event.SourceSequence = 3
	event.PriorEventDigest = digestB
	canonical := canonicalize(t, event)
	return replayCase{event: canonical, ack: ackFor(t, canonical, 33)}
}

func canonicalize(t *testing.T, event Event) Event {
	t.Helper()
	encoded, err := EncodeEvent(event)
	if err != nil {
		t.Fatalf("EncodeEvent: %v", err)
	}
	decoded, err := DecodeEvent(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	return decoded
}

func ackFor(t *testing.T, event Event, global uint64) EventAck {
	t.Helper()
	ack := EventAck{
		Schema: AckSchemaID, Flight: event.Flight, Lane: event.Lane, Epoch: event.Epoch,
		Session: event.Session, ManifestRevision: event.ManifestRevision, Kind: event.Kind,
		SourceSequence: event.SourceSequence, EventDigest: event.EventDigest, GlobalSequence: global,
	}
	encoded, err := EncodeEventAck(ack)
	if err != nil {
		t.Fatalf("EncodeEventAck: %v", err)
	}
	decoded, err := DecodeEventAck(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeEventAck: %v", err)
	}
	return decoded
}

// requireAdjacentAck is the envelope-level adjacency rule: source order is
// consecutive within one active revision while global order only strictly
// increases.
func requireAdjacentAck(prior, current EventAck) error {
	if current.SourceSequence != prior.SourceSequence+1 {
		return fmt.Errorf("source sequence %d does not follow %d", current.SourceSequence, prior.SourceSequence)
	}
	if current.GlobalSequence <= prior.GlobalSequence {
		return fmt.Errorf("global sequence %d does not exceed %d", current.GlobalSequence, prior.GlobalSequence)
	}
	return nil
}

func prefixFixture() EventPrefix {
	return EventPrefix{
		Schema:                  EventPrefixSchemaID,
		Flight:                  "flight-1",
		Lane:                    "builder",
		Session:                 "session-1",
		Epoch:                   "epoch-1",
		ManifestRevision:        1,
		ManifestDigest:          digestA,
		TerminalSourceSequence:  10,
		TerminalGlobalSequence:  20,
		TerminalEventDigest:     digestB,
		CompletedEventChainRoot: digestC,
	}
}

func sizedInlineDetail(raw []byte) Detail {
	return Detail{
		Mode: DetailInline, MediaType: MediaTypeJSON, Digest: digestOf(raw),
		RedactionProfile: RedactionProfileStandard, RedactedJSON: append([]byte(nil), raw...),
	}
}

func digestOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum)
}

func exactCanonicalJSON(t *testing.T, size int) []byte {
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
