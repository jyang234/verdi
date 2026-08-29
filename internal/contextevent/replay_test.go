package contextevent

import (
	"bytes"
	"testing"
)

func TestValidateReplay_Behavioral(t *testing.T) {
	t.Parallel()

	// Build a canonical event and ack fixture.
	ev := eventFixture(t, KindPrompt, AdapterCodex)
	ev.SourceSequence = 3
	ev.PriorEventDigest = digestB
	encoded, err := EncodeEvent(ev)
	if err != nil {
		t.Fatalf("EncodeEvent fixture: %v", err)
	}
	decoded, err := DecodeEvent(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeEvent fixture: %v", err)
	}
	ack := EventAck{
		Schema: AckSchemaID, Flight: decoded.Flight, Lane: decoded.Lane,
		Epoch: decoded.Epoch, Session: decoded.Session,
		ManifestRevision: decoded.ManifestRevision, Kind: decoded.Kind,
		SourceSequence: decoded.SourceSequence, EventDigest: decoded.EventDigest,
		GlobalSequence: 33,
	}

	t.Run("byte-identical replay returns original ack", func(t *testing.T) {
		got, err := ValidateReplay(decoded, ack, decoded)
		if err != nil {
			t.Fatalf("ValidateReplay(identical) error = %v", err)
		}
		if got != ack {
			t.Fatalf("ValidateReplay(identical) ack = %#v, want %#v", got, ack)
		}
	})

	t.Run("conflicting kind fails closed", func(t *testing.T) {
		other := eventFixture(t, KindToolCall, AdapterCodex)
		other.SourceSequence = decoded.SourceSequence
		other.PriorEventDigest = decoded.PriorEventDigest
		otherEnc, err := EncodeEvent(other)
		if err != nil {
			t.Fatalf("EncodeEvent other: %v", err)
		}
		otherDec, err := DecodeEvent(bytes.NewReader(otherEnc))
		if err != nil {
			t.Fatalf("DecodeEvent other: %v", err)
		}
		if _, err := ValidateReplay(decoded, ack, otherDec); err == nil {
			t.Fatal("ValidateReplay(conflicting kind) error = nil, want error")
		}
	})

	t.Run("differing source sequence fails closed", func(t *testing.T) {
		shifted := decoded
		shifted.SourceSequence = decoded.SourceSequence + 1
		shifted.PriorRevision = nil
		shifted.EventDigest = ""
		shiftedEnc, err := EncodeEvent(shifted)
		if err != nil {
			t.Fatalf("EncodeEvent shifted: %v", err)
		}
		shiftedDec, err := DecodeEvent(bytes.NewReader(shiftedEnc))
		if err != nil {
			t.Fatalf("DecodeEvent shifted: %v", err)
		}
		if _, err := ValidateReplay(decoded, ack, shiftedDec); err == nil {
			t.Fatal("ValidateReplay(shifted sequence) error = nil, want error")
		}
	})

	t.Run("differing adapter fails closed", func(t *testing.T) {
		adapted := decoded
		adapted.Adapter = AdapterClaude
		payload := *adapted.Payload.(*PromptPayload)
		adapted.Payload = &payload
		adapted.EventDigest = ""
		adaptedEnc, err := EncodeEvent(adapted)
		if err != nil {
			t.Fatalf("EncodeEvent adapted: %v", err)
		}
		adaptedDec, err := DecodeEvent(bytes.NewReader(adaptedEnc))
		if err != nil {
			t.Fatalf("DecodeEvent adapted: %v", err)
		}
		if _, err := ValidateReplay(decoded, ack, adaptedDec); err == nil {
			t.Fatal("ValidateReplay(differing adapter) error = nil, want error")
		}
	})

	t.Run("invalid existing event fails closed", func(t *testing.T) {
		bad := decoded
		bad.EventDigest = digestA // wrong digest — will fail EncodeEvent's digest check
		if _, err := ValidateReplay(bad, ack, decoded); err == nil {
			t.Fatal("ValidateReplay(invalid existing digest) error = nil, want error")
		}
	})

	t.Run("invalid incoming event fails closed", func(t *testing.T) {
		bad := decoded
		bad.EventDigest = digestA // wrong digest
		if _, err := ValidateReplay(decoded, ack, bad); err == nil {
			t.Fatal("ValidateReplay(invalid incoming digest) error = nil, want error")
		}
	})
}

func TestEventPrefixDigest_Behavioral(t *testing.T) {
	t.Parallel()

	t.Run("prefix digest is sha256 and deterministic", func(t *testing.T) {
		prefix := EventPrefix{
			Schema:                  EventPrefixSchemaID,
			Flight:                  "flight-a",
			Lane:                    "builder",
			Session:                 "session-a",
			Epoch:                   1,
			ManifestRevision:        0,
			ManifestDigest:          digestA,
			TerminalSourceSequence:  5,
			TerminalGlobalSequence:  10,
			TerminalEventDigest:     digestB,
			CompletedEventChainRoot: digestC,
		}
		d, err := EventPrefixDigest(prefix)
		if err != nil {
			t.Fatalf("EventPrefixDigest error = %v", err)
		}
		if len(d) != 71 || d[:7] != "sha256:" {
			t.Fatalf("EventPrefixDigest = %q, want sha256:... (71 chars)", d)
		}
		d2, err := EventPrefixDigest(prefix)
		if err != nil {
			t.Fatalf("EventPrefixDigest second error = %v", err)
		}
		if d != d2 {
			t.Fatal("EventPrefixDigest is not deterministic")
		}
	})

	t.Run("independent field mutation witnesses", func(t *testing.T) {
		base := EventPrefix{
			Schema:                  EventPrefixSchemaID,
			Flight:                  "flight-1",
			Lane:                    "builder",
			Session:                 "session-1",
			Epoch:                   2,
			ManifestRevision:        1,
			ManifestDigest:          digestA,
			TerminalSourceSequence:  10,
			TerminalGlobalSequence:  20,
			TerminalEventDigest:     digestB,
			CompletedEventChainRoot: digestC,
		}
		baseDigest, err := EventPrefixDigest(base)
		if err != nil {
			t.Fatalf("EventPrefixDigest base error = %v", err)
		}
		mutations := []struct {
			name   string
			mutate func(EventPrefix) EventPrefix
		}{
			{"flight", func(p EventPrefix) EventPrefix { p.Flight = "flight-2"; return p }},
			{"lane", func(p EventPrefix) EventPrefix { p.Lane = "reviewer"; return p }},
			{"session", func(p EventPrefix) EventPrefix { p.Session = "session-2"; return p }},
			{"epoch", func(p EventPrefix) EventPrefix { p.Epoch = 3; return p }},
			{"manifest_revision", func(p EventPrefix) EventPrefix { p.ManifestRevision = 2; return p }},
			{"manifest_digest", func(p EventPrefix) EventPrefix { p.ManifestDigest = digestB; return p }},
			{"terminal_source_sequence", func(p EventPrefix) EventPrefix { p.TerminalSourceSequence = 11; return p }},
			{"terminal_global_sequence", func(p EventPrefix) EventPrefix { p.TerminalGlobalSequence = 21; return p }},
			{"terminal_event_digest", func(p EventPrefix) EventPrefix { p.TerminalEventDigest = digestC; return p }},
			{"completed_event_chain_root", func(p EventPrefix) EventPrefix { p.CompletedEventChainRoot = digestA; return p }},
		}
		for _, tt := range mutations {
			mutated := tt.mutate(base)
			d, err := EventPrefixDigest(mutated)
			if err != nil {
				t.Fatalf("EventPrefixDigest(%s) error = %v", tt.name, err)
			}
			if d == baseDigest {
				t.Errorf("EventPrefixDigest mutation %q did not change digest", tt.name)
			}
		}
	})
}
