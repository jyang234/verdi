package experiment

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// measurementValueKind is which arm of the measurement-value union a
// MeasurementValue holds. The zero kind is ABSENT rather than "number 0",
// so a measurement whose value key is missing can never be mistaken for
// one that measured zero.
type measurementValueKind uint8

const (
	valueAbsent measurementValueKind = iota
	valueNumber
	valueBoolean
)

// MeasurementValue is the strict typed union a measurement's value field
// carries (invention ledger SI-45): a JSON number, or the JSON literal
// true/false for a measurement whose registered metric type is boolean.
//
// AC-3 names boolean a core metric primitive, but the artifact grammar
// carried only json.Number, so a boolean observation had to be encoded as
// 0/1 — a coercion nothing downstream could detect, and one that made a
// misdeclared metric type invisible. This type keeps the two kinds
// distinct all the way through decode, digest, and re-encode:
//
//   - The ORIGINAL literal is preserved verbatim. "18.0" re-encodes as
//     18.0 and true re-encodes as true, never 1 — the same byte-identity
//     obligation json.Number already carried (CO-3), now extended to
//     booleans, since the observations digest binds these exact bytes.
//   - Decoding fails closed on anything else: a quoted number, null, an
//     object, an array. Nothing is coerced.
//
// WHICH kind is legal for a given measurement is not this type's decision:
// it depends on the metric the definition registered for that id, so it is
// enforced in the def-aware validation path (ValidateObservations), not in
// the grammar. For AGGREGATION every value projects to one float64 through
// Float64, which maps true to 1 and false to 0 so all five registered
// aggregations stay defined (rate of a boolean is the fraction of true
// rounds).
//
// The zero value holds neither arm and is invalid; it exists so an absent
// value field is detectable.
type MeasurementValue struct {
	kind    measurementValueKind
	number  json.Number
	boolean bool
}

// NumberValue returns the numeric arm of the union carrying the exact
// literal n. It performs no validation — Measurement.Validate is where a
// literal that is not a finite JSON number is rejected — so a writer can
// build a value and have its one canonical formatting checked in one
// place.
func NumberValue(n json.Number) MeasurementValue {
	return MeasurementValue{kind: valueNumber, number: n}
}

// BoolValue returns the boolean arm of the union.
func BoolValue(b bool) MeasurementValue {
	return MeasurementValue{kind: valueBoolean, boolean: b}
}

// Present reports whether v holds either arm of the union. A false answer
// means the value field was absent, not that it measured zero or false.
func (v MeasurementValue) Present() bool { return v.kind != valueAbsent }

// IsBool reports whether v holds the boolean arm.
func (v MeasurementValue) IsBool() bool { return v.kind == valueBoolean }

// Bool returns the boolean v holds, or false when v is not the boolean arm
// (check IsBool first — false is also a legitimate boolean value).
func (v MeasurementValue) Bool() bool { return v.kind == valueBoolean && v.boolean }

// Number returns the exact numeric literal v holds, or the empty
// json.Number when v is not the numeric arm.
func (v MeasurementValue) Number() json.Number {
	if v.kind != valueNumber {
		return ""
	}
	return v.number
}

// String returns v's original JSON literal ("18.0", "true"), or "" when v
// holds no value at all.
func (v MeasurementValue) String() string {
	switch v.kind {
	case valueNumber:
		return string(v.number)
	case valueBoolean:
		if v.boolean {
			return "true"
		}
		return "false"
	}
	return ""
}

// Float64 projects v onto the single numeric scale every comparison and
// aggregation in the decision engine operates on: a number parses to
// itself, true maps to 1, and false maps to 0 (SI-45). An absent or
// malformed value is an error rather than a silent zero, because zero is
// itself a meaningful measurement.
func (v MeasurementValue) Float64() (float64, error) {
	switch v.kind {
	case valueNumber:
		f, err := v.number.Float64()
		if err != nil {
			return 0, fmt.Errorf("experiment: measurement value %q is not a JSON number: %w", string(v.number), err)
		}
		return f, nil
	case valueBoolean:
		if v.boolean {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("experiment: measurement value is missing")
}

// UnmarshalJSON accepts exactly two shapes — a JSON number and the
// literals true/false — and rejects everything else, including the quoted
// forms "18" and "true", null, objects, and arrays. Nothing is coerced:
// this is the point where a boolean measurement stops being
// indistinguishable from the number 1.
func (v *MeasurementValue) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	switch string(trimmed) {
	case "true":
		*v = BoolValue(true)
		return nil
	case "false":
		*v = BoolValue(false)
		return nil
	case "null":
		return fmt.Errorf("experiment: measurement value must be a JSON number or the literal true/false, got null")
	}

	// encoding/json accepts a QUOTED number ("18") into a json.Number, so
	// the literal's own first byte has to be checked here: a stringified
	// measurement is not a measurement, and letting it through would
	// reintroduce exactly the silent coercion this union closes.
	if len(trimmed) == 0 || (trimmed[0] != '-' && (trimmed[0] < '0' || trimmed[0] > '9')) {
		return fmt.Errorf("experiment: measurement value %s is neither a JSON number nor the literal true/false", trimmed)
	}
	var n json.Number
	if err := json.Unmarshal(trimmed, &n); err != nil {
		return fmt.Errorf("experiment: measurement value %s is neither a JSON number nor the literal true/false: %w", trimmed, err)
	}
	*v = NumberValue(n)
	return nil
}

// MarshalJSON writes the original literal back out unchanged, which is
// what makes a decode/encode round trip byte-identical. A value holding
// neither arm is an error rather than a null or a zero: an encoder that
// cannot say what was measured must not emit a document claiming
// something was.
func (v MeasurementValue) MarshalJSON() ([]byte, error) {
	switch v.kind {
	case valueNumber:
		if v.number == "" {
			return nil, fmt.Errorf("experiment: measurement value is missing")
		}
		return []byte(v.number), nil
	case valueBoolean:
		if v.boolean {
			return []byte("true"), nil
		}
		return []byte("false"), nil
	}
	return nil, fmt.Errorf("experiment: measurement value is missing")
}
