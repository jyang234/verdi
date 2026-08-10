package experiment

import (
	"encoding/json"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
)

func TestMeasurementValueUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name       string
		doc        string
		wantErr    bool
		wantBool   bool
		wantIsBool bool
		wantString string
	}{
		{name: "integer", doc: `18`, wantString: "18"},
		{name: "decimal", doc: `18.0`, wantString: "18.0"},
		{name: "negative", doc: `-0.5`, wantString: "-0.5"},
		{name: "exponent", doc: `1.8e1`, wantString: "1.8e1"},
		{name: "zero", doc: `0`, wantString: "0"},
		{name: "true", doc: `true`, wantIsBool: true, wantBool: true, wantString: "true"},
		{name: "false", doc: `false`, wantIsBool: true, wantBool: false, wantString: "false"},
		{name: "quoted number", doc: `"18"`, wantErr: true},
		{name: "quoted boolean", doc: `"true"`, wantErr: true},
		{name: "null", doc: `null`, wantErr: true},
		{name: "object", doc: `{"value":18}`, wantErr: true},
		{name: "array", doc: `[18]`, wantErr: true},
		{name: "capitalized boolean", doc: `True`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v MeasurementValue
			err := json.Unmarshal([]byte(tt.doc), &v)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal(%s) = nil error, want error", tt.doc)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%s) unexpected error: %v", tt.doc, err)
			}
			if !v.Present() {
				t.Fatalf("Unmarshal(%s): value reports absent", tt.doc)
			}
			if v.IsBool() != tt.wantIsBool {
				t.Fatalf("IsBool() = %v, want %v", v.IsBool(), tt.wantIsBool)
			}
			if tt.wantIsBool && v.Bool() != tt.wantBool {
				t.Errorf("Bool() = %v, want %v", v.Bool(), tt.wantBool)
			}
			if v.String() != tt.wantString {
				t.Errorf("String() = %q, want %q", v.String(), tt.wantString)
			}
		})
	}
}

// TestMeasurementValueMarshalRoundTripsLiterals is SI-45's byte-identity
// arm: the union preserves the ORIGINAL literal, so a value that decoded
// from "18.0" re-encodes as 18.0 (never 18), and true re-encodes as the
// boolean literal (never 1). Both properties matter for CO-3, since the
// observations digest binds these exact bytes.
func TestMeasurementValueMarshalRoundTripsLiterals(t *testing.T) {
	for _, literal := range []string{"18", "18.0", "-0.5", "1.8e1", "0", "true", "false"} {
		t.Run(literal, func(t *testing.T) {
			var v MeasurementValue
			if err := json.Unmarshal([]byte(literal), &v); err != nil {
				t.Fatalf("Unmarshal(%s) unexpected error: %v", literal, err)
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("Marshal() unexpected error: %v", err)
			}
			if string(out) != literal {
				t.Errorf("Marshal() = %s, want the original literal %s", out, literal)
			}
			canonical, err := canonjson.Marshal(v)
			if err != nil {
				t.Fatalf("canonjson.Marshal() unexpected error: %v", err)
			}
			if string(canonical) != literal+"\n" {
				t.Errorf("canonjson.Marshal() = %q, want %q", canonical, literal+"\n")
			}
		})
	}
}

func TestMeasurementValueMarshalRejectsMissing(t *testing.T) {
	if _, err := json.Marshal(MeasurementValue{}); err == nil {
		t.Errorf("Marshal(zero MeasurementValue) = nil error, want error")
	}
}

// TestMeasurementValueFloat64 covers SI-45's aggregation mapping: a
// boolean projects to 1 (true) or 0 (false) so every registered
// aggregation stays defined over the mapped values, while a number
// projects to itself.
func TestMeasurementValueFloat64(t *testing.T) {
	tests := []struct {
		name    string
		v       MeasurementValue
		want    float64
		wantErr bool
	}{
		{name: "number", v: NumberValue("18.5"), want: 18.5},
		{name: "integer", v: NumberValue("42"), want: 42},
		{name: "true maps to 1", v: BoolValue(true), want: 1},
		{name: "false maps to 0", v: BoolValue(false), want: 0},
		{name: "missing", v: MeasurementValue{}, wantErr: true},
		{name: "malformed literal", v: NumberValue("eighteen"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.v.Float64()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Float64() = %v, nil error, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Float64() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Float64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMeasurementValueAccessors(t *testing.T) {
	num := NumberValue("18")
	if num.IsBool() {
		t.Errorf("NumberValue().IsBool() = true, want false")
	}
	if num.Number() != json.Number("18") {
		t.Errorf("NumberValue().Number() = %q, want %q", num.Number(), "18")
	}
	if num.Bool() {
		t.Errorf("NumberValue().Bool() = true, want the false zero value for a non-boolean")
	}

	b := BoolValue(true)
	if !b.IsBool() || !b.Bool() {
		t.Errorf("BoolValue(true) = (IsBool %v, Bool %v), want (true, true)", b.IsBool(), b.Bool())
	}
	if b.Number() != "" {
		t.Errorf("BoolValue(true).Number() = %q, want the empty json.Number", b.Number())
	}

	var zero MeasurementValue
	if zero.Present() {
		t.Errorf("zero MeasurementValue reports Present() = true, want false")
	}
	if zero.String() != "" {
		t.Errorf("zero MeasurementValue String() = %q, want %q", zero.String(), "")
	}
}
