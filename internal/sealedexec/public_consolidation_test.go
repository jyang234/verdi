package sealedexec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
)

type consolidationCase struct {
	ID            string   `json:"id"`
	Fixture       string   `json:"fixture"`
	Direction     string   `json:"direction"`
	Path          []string `json:"path"`
	Mutation      string   `json:"mutation"`
	OperandSHA256 string   `json:"operand_sha256"`
	Accepted      bool     `json:"accepted"`
	OutputSHA256  string   `json:"output_sha256,omitempty"`
}
type consolidationCorpus struct {
	Baseline string              `json:"baseline"`
	Fixtures map[string]string   `json:"fixtures"`
	Cases    []consolidationCase `json:"cases"`
}

func consolidationDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func consolidationFixture(name string) ([]byte, error) {
	if strings.HasPrefix(name, "supplement/") {
		return os.ReadFile(filepath.Join("testdata", "public-consolidation", name))
	}
	return os.ReadFile(filepath.Join("..", "contextowner", "testdata", "public-contract", name))
}

// Typed mutations operate before any marshaling. The path and mutation recipe,
// including invalid UTF-8 bytes, are independent of the production encoder.
func consolidationMutateTyped(v reflect.Value, path []string, mutation string) error {
	for _, p := range path {
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() == reflect.Slice {
			n, e := strconv.Atoi(p)
			if e != nil || n < 0 || n >= v.Len() {
				return fmt.Errorf("invalid index %s", p)
			}
			v = v.Index(n)
			continue
		}
		if v.Kind() != reflect.Struct {
			return fmt.Errorf("nonstruct path %v", path)
		}
		v = v.FieldByName(p)
		if !v.IsValid() {
			return fmt.Errorf("absent field %s", p)
		}
	}
	if v.Kind() == reflect.Pointer && mutation != "nil" && mutation != "zero-pointer" && mutation != "control" {
		if v.IsNil() {
			return fmt.Errorf("nil scalar pointer %v", path)
		}
		v = v.Elem()
	}
	switch mutation {
	case "control":
		return nil
	case "oid-64":
		v.SetString(strings.Repeat("a", 64))
	case "fractional-zero-stamp":
		v.SetString("2026-01-01T00:00:00.000Z")
	case "empty":
		v.SetString("")
	case "space":
		v.SetString(" " + v.String())
	case "internal-space":
		v.SetString("two words")
	case "internal-control":
		v.SetString("two\x01words")
	case "invalid-utf8":
		v.SetString(string([]byte{0xff}))
	case "unknown":
		v.SetString("wrong")
	case "uppercase":
		v.SetString(strings.ToUpper(v.String()))
	case "relative-path":
		v.SetString("relative/path")
	case "unclean-path":
		v.SetString("/tmp/../value")
	case "zero":
		v.SetUint(0)
	case "max":
		v.SetUint(^uint64(0))
	case "zero-int":
		v.SetInt(0)
	case "negative-int":
		v.SetInt(-1)
	case "toggle":
		v.SetBool(!v.Bool())
	case "nil":
		v.SetZero()
	case "empty-slice":
		v.Set(reflect.MakeSlice(v.Type(), 0, 0))
	case "element-empty", "element-invalid-utf8", "element-space":
		x := reflect.New(v.Type().Elem()).Elem()
		switch mutation {
		case "element-invalid-utf8":
			x.SetString(string([]byte{0xff}))
		case "element-space":
			x.SetString(" witness")
		}
		if v.Len() == 0 {
			v.Set(reflect.Append(v, x))
		} else {
			v.Index(0).Set(x)
		}
	case "duplicate":
		if v.Len() == 0 {
			return fmt.Errorf("empty duplicate")
		}
		v.Set(reflect.AppendSlice(v, v))
	case "reverse":
		reflect.Swapper(v.Interface())(0, v.Len()-1)
	case "zero-pointer":
		v.Set(reflect.New(v.Type().Elem()))
	default:
		return fmt.Errorf("unknown typed mutation %s", mutation)
	}
	return nil
}

func consolidationMutateRaw(raw []byte, path []string, mutation string) ([]byte, error) {
	if mutation == "control" {
		return append([]byte(nil), raw...), nil
	}
	if len(path) == 0 {
		switch mutation {
		case "leading-space":
			return append([]byte(" "), raw...), nil
		case "missing-lf":
			return bytes.TrimSuffix(raw, []byte("\n")), nil
		case "trailing-document":
			return append(append([]byte(nil), raw...), []byte("{}\n")...), nil
		case "duplicate-member":
			return bytes.Replace(raw, []byte("{"), []byte(`{"schema":"wrong",`), 1), nil
		}
	}
	if len(path) > 0 {
		if index, e := strconv.Atoi(path[0]); e == nil {
			var values []json.RawMessage
			if err := json.Unmarshal(raw, &values); err != nil {
				return nil, err
			}
			if index < 0 || index >= len(values) {
				return nil, fmt.Errorf("invalid raw index %d", index)
			}
			if len(path) > 1 {
				next, err := consolidationMutateRaw(values[index], path[1:], mutation)
				if err != nil {
					return nil, err
				}
				values[index] = bytes.TrimSuffix(next, []byte("\n"))
			} else {
				wrapper, err := json.Marshal(map[string]json.RawMessage{"value": values[index]})
				if err != nil {
					return nil, err
				}
				changed, err := consolidationMutateRaw(wrapper, []string{"value"}, mutation)
				if err != nil {
					return nil, err
				}
				var member map[string]json.RawMessage
				if err := json.Unmarshal(changed, &member); err != nil {
					return nil, err
				}
				if value, ok := member["value"]; ok {
					values[index] = value
				} else {
					values = append(values[:index], values[index+1:]...)
				}
			}
			var out bytes.Buffer
			enc := json.NewEncoder(&out)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(values); err != nil {
				return nil, err
			}
			return out.Bytes(), nil
		}
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if len(path) == 0 {
		return nil, fmt.Errorf("missing raw path")
	}
	if len(path) > 1 {
		next, err := consolidationMutateRaw(doc[path[0]], path[1:], mutation)
		if err != nil {
			return nil, err
		}
		doc[path[0]] = bytes.TrimSuffix(next, []byte("\n"))
	} else {
		key := path[0]
		switch mutation {
		case "missing":
			delete(doc, key)
		case "null":
			doc[key] = json.RawMessage("null")
		case "wrong-type":
			doc[key] = json.RawMessage("{}")
		case "unknown-member":
			doc["future_member"] = json.RawMessage("true")
		case "negative-zero":
			doc[key] = json.RawMessage("-0")
		case "fractional":
			doc[key] = json.RawMessage("1.0")
		case "duplicate-first":
			var values []json.RawMessage
			if err := json.Unmarshal(doc[key], &values); err != nil {
				return nil, err
			}
			if len(values) == 0 {
				return nil, fmt.Errorf("empty duplicate")
			}
			values = append(values, values[0])
			b, err := canonjson.Marshal(values)
			if err != nil {
				return nil, err
			}
			doc[key] = bytes.TrimSuffix(b, []byte("\n"))
		default:
			return nil, fmt.Errorf("unknown raw mutation %s", mutation)
		}
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func consolidationExecute(row consolidationCase) (operand string, accepted bool, output string, reason string, err error) {
	raw, err := consolidationFixture(row.Fixture)
	if err != nil {
		return "", false, "", "", err
	}
	call := strings.HasSuffix(row.Fixture, ".call.v2.json")
	var encoded []byte
	var failure error
	if row.Direction == "typed" {
		var v reflect.Value
		if call {
			c, e := DecodeControllerCall(bytes.NewReader(raw))
			if e != nil {
				return "", false, "", "", e
			}
			v = reflect.ValueOf(&c).Elem()
		} else {
			r, e := DecodeControllerResult(bytes.NewReader(raw))
			if e != nil {
				return "", false, "", "", e
			}
			v = reflect.ValueOf(&r).Elem()
		}
		if err := consolidationMutateTyped(v, row.Path, row.Mutation); err != nil {
			return "", false, "", "", err
		}
		// The recipe authenticates pre-marshal typed operands, whose invalid UTF-8
		// cannot be represented losslessly as ordinary JSON text.
		recipe, e := json.Marshal(struct {
			Fixture  string
			Path     []string
			Mutation string
		}{consolidationDigest(raw), row.Path, row.Mutation})
		if e != nil {
			return "", false, "", "", e
		}
		operand = consolidationDigest(recipe)
		if call {
			encoded, failure = EncodeControllerCall(v.Interface().(ControllerCall))
		} else {
			encoded, failure = EncodeControllerResult(v.Interface().(ControllerResult))
		}
	} else {
		raw, err = consolidationMutateRaw(raw, row.Path, row.Mutation)
		if err != nil {
			return "", false, "", "", err
		}
		operand = consolidationDigest(raw)
		if call {
			v, e := DecodeControllerCall(bytes.NewReader(raw))
			failure = e
			if e == nil {
				encoded, failure = EncodeControllerCall(v)
			}
		} else {
			v, e := DecodeControllerResult(bytes.NewReader(raw))
			failure = e
			if e == nil {
				encoded, failure = EncodeControllerResult(v)
			}
		}
	}
	if failure != nil {
		return operand, false, "", failure.Error(), nil
	}
	return operand, true, consolidationDigest(encoded), "", nil
}

func TestPublicControllerConsolidationCorpus(t *testing.T) {
	consolidationReplayCorpus(t, "corpus.json", "7a684a96e75b7afbe9c450c3b51966c58615cab972940fdc2833006c27daec76", 4377, 44)
}

func consolidationReplayCorpus(t *testing.T, filename, digest string, count, fixtures int) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "public-consolidation", filename))
	if err != nil {
		t.Fatal(err)
	}
	if consolidationDigest(raw) != digest {
		t.Fatal("baseline consolidation corpus changed")
	}
	var corpus consolidationCorpus
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		t.Fatalf("trailing corpus document: %v", err)
	}
	if corpus.Baseline != "e671a326418fdfb672a92398228bafcac184c99f" || len(corpus.Cases) != count || len(corpus.Fixtures) != fixtures {
		t.Fatal("missing fixed baseline corpus")
	}
	for name, want := range corpus.Fixtures {
		b, e := consolidationFixture(name)
		if e != nil {
			t.Fatal(e)
		}
		if consolidationDigest(b) != want {
			t.Fatalf("fixture changed: %s", name)
		}
	}
	seen := map[string]bool{}
	for _, row := range corpus.Cases {
		if seen[row.ID] {
			t.Fatalf("duplicate case %s", row.ID)
		}
		seen[row.ID] = true
		t.Run(row.ID, func(t *testing.T) {
			operand, accepted, output, reason, e := consolidationExecute(row)
			if e != nil {
				t.Fatal(e)
			}
			if operand != row.OperandSHA256 {
				t.Fatalf("operand changed: %s != %s", operand, row.OperandSHA256)
			}
			if accepted != row.Accepted || output != row.OutputSHA256 {
				t.Fatalf("baseline accepted=%v output=%s; candidate accepted=%v output=%s reason=%s", row.Accepted, row.OutputSHA256, accepted, output, reason)
			}
		})
	}
}

// Supplemental seeds populate controller-owned arrays and optional facts that
// the original frozen operation fixtures leave empty. Outcomes were measured
// exclusively against the accepted pre-consolidation source.
func TestPublicControllerConsolidationSupplement(t *testing.T) {
	consolidationReplayCorpus(t, "supplement.json", "a04c12b4c8db318addb3bf4ba89e70f9a62ceffa9a495af114641d87081c1196", 3666, 16)
}
