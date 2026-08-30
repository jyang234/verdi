package contextevent

import (
	"bytes"
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
)

// ValidateReplay checks whether incoming is a byte-identical replay of existing.
// If so, it returns existingAck unchanged. Any identity or byte contradiction
// fails closed with an error. Both events are re-encoded and compared
// canonically; a malformed event in either position also fails closed.
func ValidateReplay(existing Event, existingAck EventAck, incoming Event) (EventAck, error) {
	existingBytes, err := canonjson.Marshal(existing)
	if err != nil {
		return EventAck{}, fmt.Errorf("contextevent: replay: encode existing event: %w", err)
	}
	// Re-validate existing by round-tripping through DecodeEvent.
	if _, err := DecodeEvent(bytes.NewReader(existingBytes)); err != nil {
		return EventAck{}, fmt.Errorf("contextevent: replay: existing event invalid: %w", err)
	}
	incomingBytes, err := canonjson.Marshal(incoming)
	if err != nil {
		return EventAck{}, fmt.Errorf("contextevent: replay: encode incoming event: %w", err)
	}
	// Re-validate incoming.
	if _, err := DecodeEvent(bytes.NewReader(incomingBytes)); err != nil {
		return EventAck{}, fmt.Errorf("contextevent: replay: incoming event invalid: %w", err)
	}
	if !bytes.Equal(existingBytes, incomingBytes) {
		return EventAck{}, fmt.Errorf("contextevent: replay: incoming event contradicts existing acknowledged event")
	}
	return existingAck, nil
}
