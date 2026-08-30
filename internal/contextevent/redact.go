package contextevent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
)

const redactedValue = "[REDACTED]"

var sensitiveKeys = map[string]struct{}{
	"apikey": {}, "authorization": {}, "cookie": {}, "password": {},
	"secret": {}, "token": {}, "accesstoken": {}, "refreshtoken": {},
	"sessiontoken": {}, "sessionid": {}, "thinking": {},
	"redactedthinking": {}, "signature": {},
}

var safeRawDetailReasons = map[string]struct{}{
	"empty-foreign-record": {}, "malformed-foreign-frame": {},
	"unknown-foreign-family": {}, "unknown-content-block": {},
	"unknown-foreign-field": {}, "missing-foreign-field": {},
	"invalid-foreign-field": {}, "session-mismatch": {}, "model-mismatch": {},
	"mcp-mismatch": {}, "duplicate-init": {}, "duplicate-result": {},
	"duplicate-message-id": {}, "duplicate-call-id": {},
	"result-before-init": {}, "observation-after-result": {},
	"unmatched-tool-result": {}, "duplicate-tool-result": {},
	"incomplete-tool-call": {}, "missing-terminal-result": {},
	"secret-classification-unavailable": {}, "redaction-failed": {},
	"protected-fixed-field": {}, "provider-stderr": {},
	"provider-exit-nonzero": {}, "segment-store-failed": {},
	"segment-resolve-failed": {}, "segment-mismatch": {},
	"recorder-append-failed": {}, "recorder-ack-invalid": {},
	"recorder-replay-conflict": {}, "unconfirmed-initial-session": {},
}

// RedactStandardV1 applies the sole accepted recursive redaction profile to
// one unique-key JSON value and returns canonical JSON without a trailing LF.
func RedactStandardV1(raw []byte, protectedValues [][]byte) ([]byte, error) {
	if err := validateProtectedValues(protectedValues); err != nil {
		return nil, err
	}
	value, err := decodeUniqueJSONValue(raw)
	if err != nil {
		return nil, fmt.Errorf("contextevent: redaction input is not one unique-key JSON value")
	}
	redacted, err := redactJSONValue(value, protectedValues)
	if err != nil {
		return nil, err
	}
	canonical, err := canonjson.Marshal(redacted)
	if err != nil {
		return nil, fmt.Errorf("contextevent: canonicalize redacted JSON: %w", err)
	}
	canonical = bytes.TrimSuffix(canonical, []byte("\n"))
	if containsProtectedBytes(canonical, protectedValues) {
		return nil, fmt.Errorf("contextevent: final redacted JSON contains protected bytes")
	}
	return canonical, nil
}

// ValidateFixedPayloadValue refuses a provider-derived fixed payload value
// containing classified bytes. Fixed fields are never rewritten.
func ValidateFixedPayloadValue(value string, protectedValues [][]byte) error {
	if err := validateProtectedValues(protectedValues); err != nil {
		return err
	}
	if !utf8.ValidString(value) || containsProtectedBytes([]byte(value), protectedValues) {
		return fmt.Errorf("contextevent: fixed payload value is not safe to emit")
	}
	return nil
}

// SafeRawDetail reduces discarded foreign bytes to the sole fixed digest and
// closed-reason object. It never converts or embeds the original bytes.
func SafeRawDetail(raw []byte, reason string) ([]byte, error) {
	if _, ok := safeRawDetailReasons[reason]; !ok {
		return nil, fmt.Errorf("contextevent: unsafe raw-detail reason")
	}
	sum := sha256.Sum256(raw)
	detail := struct {
		RawDigest string `json:"raw_digest"`
		Reason    string `json:"reason"`
	}{
		RawDigest: "sha256:" + hex.EncodeToString(sum[:]),
		Reason:    reason,
	}
	canonical, err := canonjson.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("contextevent: encode safe raw detail: %w", err)
	}
	return bytes.TrimSuffix(canonical, []byte("\n")), nil
}

func validateProtectedValues(protectedValues [][]byte) error {
	if protectedValues == nil {
		return fmt.Errorf("contextevent: protected-value classification is unavailable")
	}
	if len(protectedValues) == 0 {
		return fmt.Errorf("contextevent: protected-value set is empty")
	}
	for _, protected := range protectedValues {
		if len(protected) == 0 {
			return fmt.Errorf("contextevent: protected-value set contains an empty member")
		}
	}
	return nil
}

func redactJSONValue(value any, protectedValues [][]byte) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, child := range typed {
			if containsProtectedBytes([]byte(key), protectedValues) {
				return nil, fmt.Errorf("contextevent: JSON object key contains protected bytes")
			}
			if _, sensitive := sensitiveKeys[normalizeSensitiveKey(key)]; sensitive {
				redacted[key] = redactedValue
				continue
			}
			walked, err := redactJSONValue(child, protectedValues)
			if err != nil {
				return nil, err
			}
			redacted[key] = walked
		}
		return redacted, nil
	case []any:
		redacted := make([]any, len(typed))
		for i, child := range typed {
			walked, err := redactJSONValue(child, protectedValues)
			if err != nil {
				return nil, err
			}
			redacted[i] = walked
		}
		return redacted, nil
	case string:
		return redactString(typed, protectedValues), nil
	case nil, bool, json.Number:
		return typed, nil
	default:
		return nil, fmt.Errorf("contextevent: decoded JSON contains an unsupported value")
	}
}

func normalizeSensitiveKey(key string) string {
	normalized := make([]byte, 0, len(key))
	for _, current := range []byte(key) {
		switch {
		case current == '_' || current == '-':
			continue
		case current >= 'A' && current <= 'Z':
			normalized = append(normalized, current+'a'-'A')
		default:
			normalized = append(normalized, current)
		}
	}
	return string(normalized)
}

type byteRange struct {
	start int
	end   int
}

func redactString(value string, protectedValues [][]byte) string {
	original := []byte(value)
	var matches []byteRange
	for _, protected := range protectedValues {
		for start := 0; start+len(protected) <= len(original); {
			relative := bytes.Index(original[start:], protected)
			if relative < 0 {
				break
			}
			matchStart := start + relative
			matches = append(matches, byteRange{start: matchStart, end: matchStart + len(protected)})
			start = matchStart + 1
		}
	}
	if len(matches) == 0 {
		return value
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].start != matches[j].start {
			return matches[i].start < matches[j].start
		}
		return matches[i].end < matches[j].end
	})
	coalesced := matches[:1]
	for _, match := range matches[1:] {
		last := &coalesced[len(coalesced)-1]
		if match.start < last.end {
			if match.end > last.end {
				last.end = match.end
			}
			continue
		}
		coalesced = append(coalesced, match)
	}

	var redacted bytes.Buffer
	lastEnd := 0
	for _, match := range coalesced {
		redacted.Write(original[lastEnd:match.start])
		redacted.WriteString(redactedValue)
		lastEnd = match.end
	}
	redacted.Write(original[lastEnd:])
	return redacted.String()
}

func containsProtectedBytes(value []byte, protectedValues [][]byte) bool {
	for _, protected := range protectedValues {
		if bytes.Contains(value, protected) {
			return true
		}
	}
	return false
}
