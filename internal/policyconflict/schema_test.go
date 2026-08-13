package policyconflict

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
)

// --- fixture / mutation helpers (mirrors internal/contextcompile's own
// schema_test.go style: generic byte-level mutation over a golden fixture
// rather than bespoke construction per case) -----------------------------

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return data
}

func withTopLevelField(t *testing.T, data []byte, key, rawValue string) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("test setup: unmarshal fixture: %v", err)
	}
	m[key] = json.RawMessage(rawValue)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("test setup: remarshal fixture: %v", err)
	}
	return append(out, '\n')
}

func withoutTopLevelField(t *testing.T, data []byte, key string) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("test setup: unmarshal fixture: %v", err)
	}
	delete(m, key)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("test setup: remarshal fixture: %v", err)
	}
	return append(out, '\n')
}

// duplicateTopLevelKey returns data with key's exact top-level value
// re-emitted a second time immediately after the opening brace, so the
// document contains the key twice — exercising the shared
// artifact.DecodeExactJSON duplicate-key wall.
func duplicateTopLevelKey(t *testing.T, data []byte, key string) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("test setup: unmarshal fixture: %v", err)
	}
	raw, ok := m[key]
	if !ok {
		t.Fatalf("test setup: fixture has no top-level key %q", key)
	}
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		t.Fatalf("test setup: fixture does not start with '{'")
	}
	rest := bytes.TrimSpace(data)
	rest = rest[1:] // drop leading '{'
	dup := append([]byte("{\""+key+"\":"), raw...)
	dup = append(dup, ',')
	dup = append(dup, rest...)
	return append(dup, '\n')
}

// reorderedNoncanonically returns data's top-level object re-emitted with
// keys in descending order — guaranteed different from canonjson's
// ascending canonical order for a 2+-key document.
func reorderedNoncanonically(t *testing.T, data []byte) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("test setup: unmarshal fixture: %v", err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		b.Write(m[k])
	}
	b.WriteByte('}')
	b.WriteByte('\n')
	return b.Bytes()
}

func withTrailingData(data []byte) []byte {
	out := append([]byte(nil), data...)
	return append(out, '{', '}')
}

func withInvalidUTF8(data []byte) []byte {
	out := append([]byte(nil), data...)
	// Inject an invalid UTF-8 byte sequence inside a string value near the
	// front (0xff is never valid as a UTF-8 lead byte).
	idx := bytes.IndexByte(out, ':')
	if idx < 0 || idx+1 >= len(out) {
		return append(out, 0xff)
	}
	out2 := append([]byte(nil), out[:idx+1]...)
	out2 = append(out2, 0xff)
	out2 = append(out2, out[idx+1:]...)
	return out2
}

// setAtPath decodes data into a generic tree (numbers preserved via
// json.Number so re-encoding never perturbs formatting), walks path
// (string keys / int slice indices), sets the leaf to value, and returns
// the result marshaled with sorted keys and a trailing newline — i.e. the
// exact canonical bytes for the mutated document, ready to be re-digested
// by the caller.
func setAtPath(t *testing.T, data []byte, path []any, value any) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var generic any
	if err := dec.Decode(&generic); err != nil {
		t.Fatalf("test setup: decode generic: %v", err)
	}
	cur := generic
	for i, seg := range path {
		last := i == len(path)-1
		switch s := seg.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				t.Fatalf("test setup: path segment %v: not an object", seg)
			}
			if last {
				m[s] = value
				return generic.(map[string]any)
			}
			cur = m[s]
		case int:
			arr, ok := cur.([]any)
			if !ok {
				t.Fatalf("test setup: path segment %v: not an array", seg)
			}
			if last {
				arr[s] = value
				return generic.(map[string]any)
			}
			cur = arr[s]
		default:
			t.Fatalf("test setup: bad path segment type %T", seg)
		}
	}
	t.Fatalf("test setup: empty path")
	return nil
}

// redigestTopLevel takes a mutated generic tree carrying a top-level
// "digest" key, recomputes that digest over the digestless canonical form
// via the real canonjson seam, splices it back in, and returns final
// canonical bytes — i.e. it forges a self-consistent (digest-matches-
// content) document the way an attacker with write access to the cache or
// request path could, so decode must reject it on CONTENT grounds, not
// merely on digest-mismatch grounds.
func redigestTopLevel(t *testing.T, tree map[string]any) []byte {
	t.Helper()
	tree["digest"] = ""
	digestless, err := canonjson.Marshal(tree)
	if err != nil {
		t.Fatalf("test setup: marshal digestless: %v", err)
	}
	digest, err := canonjson.Digest(json.RawMessage(digestless))
	if err != nil {
		t.Fatalf("test setup: digest: %v", err)
	}
	tree["digest"] = digest
	out, err := canonjson.Marshal(tree)
	if err != nil {
		t.Fatalf("test setup: marshal final: %v", err)
	}
	return out
}

// --- Request --------------------------------------------------------------

func TestDecodeRequest_CanonicalRoundTrip(t *testing.T) {
	for _, name := range []string{"request-accepted.json", "request-candidate.json"} {
		t.Run(name, func(t *testing.T) {
			data := mustReadFixture(t, name)
			req, err := DecodeRequest(data)
			if err != nil {
				t.Fatalf("DecodeRequest(%s): %v", name, err)
			}
			out, err := EncodeRequest(req)
			if err != nil {
				t.Fatalf("EncodeRequest(%s): %v", name, err)
			}
			if !bytes.Equal(out, data) {
				t.Fatalf("EncodeRequest(%s) round-trip mismatch:\n got: %s\nwant: %s", name, out, data)
			}
		})
	}
}

func TestDecodeRequest_StrictDecodeMatrix(t *testing.T) {
	base := mustReadFixture(t, "request-accepted.json")

	cases := map[string][]byte{
		"unknown top-level field": withTopLevelField(t, base, "bogus", `"x"`),
		"duplicate top-level key": duplicateTopLevelKey(t, base, "schema"),
		"trailing data":           withTrailingData(base),
		"invalid utf-8":           withInvalidUTF8(base),
		"noncanonical byte order": reorderedNoncanonically(t, base),
		"wrong schema":            withTopLevelField(t, base, "schema", `"verdi.bogus/v1"`),
		"schema explicit null":    withTopLevelField(t, base, "schema", `null`),
		"schema absent":           withoutTopLevelField(t, base, "schema"),
		"target explicit null":    withTopLevelField(t, base, "target", `null`),
		"target absent":           withoutTopLevelField(t, base, "target"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeRequest(data); err == nil {
				t.Fatalf("DecodeRequest(%s): got nil error, want failure", name)
			}
		})
	}
}

func TestDecodeRequest_TargetUnion(t *testing.T) {
	accepted := mustReadFixture(t, "request-accepted.json")
	candidate := mustReadFixture(t, "request-candidate.json")

	t.Run("accepted-context valid", func(t *testing.T) {
		if _, err := DecodeRequest(accepted); err != nil {
			t.Fatalf("DecodeRequest(accepted): %v", err)
		}
	})
	t.Run("acceptance-candidate valid", func(t *testing.T) {
		if _, err := DecodeRequest(candidate); err != nil {
			t.Fatalf("DecodeRequest(candidate): %v", err)
		}
	})

	// kind=accepted-context but the accepted_context arm is explicit null.
	t.Run("selected arm explicit null", func(t *testing.T) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(accepted, &m); err != nil {
			t.Fatal(err)
		}
		var target map[string]json.RawMessage
		if err := json.Unmarshal(m["target"], &target); err != nil {
			t.Fatal(err)
		}
		target["accepted_context"] = json.RawMessage(`null`)
		tb, _ := json.Marshal(target)
		m["target"] = tb
		data, _ := json.Marshal(m)
		if _, err := DecodeRequest(append(data, '\n')); err == nil {
			t.Fatalf("DecodeRequest(explicit null arm): got nil error, want failure")
		}
	})

	// kind=accepted-context but accepted_context is absent (missing matching arm).
	t.Run("matching arm absent", func(t *testing.T) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(accepted, &m); err != nil {
			t.Fatal(err)
		}
		var target map[string]json.RawMessage
		if err := json.Unmarshal(m["target"], &target); err != nil {
			t.Fatal(err)
		}
		delete(target, "accepted_context")
		tb, _ := json.Marshal(target)
		m["target"] = tb
		data, _ := json.Marshal(m)
		if _, err := DecodeRequest(append(data, '\n')); err == nil {
			t.Fatalf("DecodeRequest(missing matching arm): got nil error, want failure")
		}
	})

	// Both arms present.
	t.Run("both arms present", func(t *testing.T) {
		var mAccepted map[string]json.RawMessage
		if err := json.Unmarshal(accepted, &mAccepted); err != nil {
			t.Fatal(err)
		}
		var targetAccepted map[string]json.RawMessage
		if err := json.Unmarshal(mAccepted["target"], &targetAccepted); err != nil {
			t.Fatal(err)
		}
		var mCandidate map[string]json.RawMessage
		if err := json.Unmarshal(candidate, &mCandidate); err != nil {
			t.Fatal(err)
		}
		var targetCandidate map[string]json.RawMessage
		if err := json.Unmarshal(mCandidate["target"], &targetCandidate); err != nil {
			t.Fatal(err)
		}
		targetAccepted["acceptance_candidate"] = targetCandidate["acceptance_candidate"]
		tb, _ := json.Marshal(targetAccepted)
		mAccepted["target"] = tb
		data, _ := json.Marshal(mAccepted)
		if _, err := DecodeRequest(append(data, '\n')); err == nil {
			t.Fatalf("DecodeRequest(both arms): got nil error, want failure")
		}
	})

	// Neither arm present.
	t.Run("neither arm present", func(t *testing.T) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(accepted, &m); err != nil {
			t.Fatal(err)
		}
		var target map[string]json.RawMessage
		if err := json.Unmarshal(m["target"], &target); err != nil {
			t.Fatal(err)
		}
		delete(target, "accepted_context")
		delete(target, "acceptance_candidate")
		tb, _ := json.Marshal(target)
		m["target"] = tb
		data, _ := json.Marshal(m)
		if _, err := DecodeRequest(append(data, '\n')); err == nil {
			t.Fatalf("DecodeRequest(neither arm): got nil error, want failure")
		}
	})

	// Unknown kind value.
	t.Run("unknown kind", func(t *testing.T) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(accepted, &m); err != nil {
			t.Fatal(err)
		}
		var target map[string]json.RawMessage
		if err := json.Unmarshal(m["target"], &target); err != nil {
			t.Fatal(err)
		}
		target["kind"] = json.RawMessage(`"bogus-kind"`)
		tb, _ := json.Marshal(target)
		m["target"] = tb
		data, _ := json.Marshal(m)
		if _, err := DecodeRequest(append(data, '\n')); err == nil {
			t.Fatalf("DecodeRequest(unknown kind): got nil error, want failure")
		}
	})

	// wrong arm for kind: kind=accepted-context but only acceptance_candidate present.
	t.Run("wrong arm for kind", func(t *testing.T) {
		var mAccepted map[string]json.RawMessage
		if err := json.Unmarshal(accepted, &mAccepted); err != nil {
			t.Fatal(err)
		}
		var targetAccepted map[string]json.RawMessage
		if err := json.Unmarshal(mAccepted["target"], &targetAccepted); err != nil {
			t.Fatal(err)
		}
		var mCandidate map[string]json.RawMessage
		if err := json.Unmarshal(candidate, &mCandidate); err != nil {
			t.Fatal(err)
		}
		var targetCandidate map[string]json.RawMessage
		if err := json.Unmarshal(mCandidate["target"], &targetCandidate); err != nil {
			t.Fatal(err)
		}
		newTarget := map[string]json.RawMessage{
			"kind":                 targetAccepted["kind"], // "accepted-context"
			"acceptance_candidate": targetCandidate["acceptance_candidate"],
		}
		tb, _ := json.Marshal(newTarget)
		mAccepted["target"] = tb
		data, _ := json.Marshal(mAccepted)
		if _, err := DecodeRequest(append(data, '\n')); err == nil {
			t.Fatalf("DecodeRequest(wrong arm for kind): got nil error, want failure")
		}
	})
}

// --- JudgeResult ------------------------------------------------------------

func TestDecodeJudgeResult_CanonicalRoundTrip(t *testing.T) {
	data := mustReadFixture(t, "judge-result.json")
	jr, err := DecodeJudgeResult(data)
	if err != nil {
		t.Fatalf("DecodeJudgeResult: %v", err)
	}
	out, err := EncodeJudgeResult(jr)
	if err != nil {
		t.Fatalf("EncodeJudgeResult: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("EncodeJudgeResult round-trip mismatch:\n got: %s\nwant: %s", out, data)
	}
}

func TestDecodeJudgeResult_StrictDecodeMatrix(t *testing.T) {
	base := mustReadFixture(t, "judge-result.json")
	cases := map[string][]byte{
		"unknown top-level field": withTopLevelField(t, base, "bogus", `"x"`),
		"duplicate top-level key": duplicateTopLevelKey(t, base, "schema"),
		"trailing data":           withTrailingData(base),
		"invalid utf-8":           withInvalidUTF8(base),
		"noncanonical byte order": reorderedNoncanonically(t, base),
		"wrong schema":            withTopLevelField(t, base, "schema", `"verdi.bogus/v1"`),
		"unknown recommendation":  withTopLevelField(t, base, "recommendation", `"bogus"`),
		"findings absent":         withoutTopLevelField(t, base, "findings"),
		"findings null":           withTopLevelField(t, base, "findings", `null`),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeJudgeResult(data); err == nil {
				t.Fatalf("DecodeJudgeResult(%s): got nil error, want failure", name)
			}
		})
	}
}

func TestDecodeJudgeResult_RecommendationCardinality(t *testing.T) {
	base := mustReadFixture(t, "judge-result.json")

	t.Run("conflict requires at least one finding", func(t *testing.T) {
		data := withTopLevelField(t, base, "findings", `[]`)
		if _, err := DecodeJudgeResult(data); err == nil {
			t.Fatalf("got nil error, want failure (conflict with zero findings)")
		}
	})

	t.Run("no-conflict requires explicit empty findings", func(t *testing.T) {
		withFindings := withTopLevelField(t, base, "recommendation", `"no-conflict"`)
		if _, err := DecodeJudgeResult(withFindings); err == nil {
			t.Fatalf("got nil error, want failure (no-conflict with nonempty findings)")
		}
		empty := withTopLevelField(t, withTopLevelField(t, base, "recommendation", `"no-conflict"`), "findings", `[]`)
		if _, err := DecodeJudgeResult(empty); err != nil {
			t.Fatalf("no-conflict with empty findings: %v", err)
		}
	})

	t.Run("inconclusive permits findings", func(t *testing.T) {
		data := withTopLevelField(t, base, "recommendation", `"inconclusive"`)
		if _, err := DecodeJudgeResult(data); err != nil {
			t.Fatalf("inconclusive with findings: %v", err)
		}
		empty := withTopLevelField(t, withTopLevelField(t, base, "recommendation", `"inconclusive"`), "findings", `[]`)
		if _, err := DecodeJudgeResult(empty); err != nil {
			t.Fatalf("inconclusive with empty findings: %v", err)
		}
	})
}

func TestDecodeJudgeResult_FindingWitnessCardinality(t *testing.T) {
	base := mustReadFixture(t, "judge-result.json")

	t.Run("single claim witness rejected", func(t *testing.T) {
		data := setAtPathAndMarshal(t, base, []any{"findings", 0, "claims"}, []any{
			map[string]any{"id": "policy-instruction:example-policy#example-claim", "digest": "sha256:" + repeatHex('1'), "category": "policy-instruction"},
		})
		if _, err := DecodeJudgeResult(data); err == nil {
			t.Fatalf("got nil error, want failure (single claim witness)")
		}
	})

	t.Run("duplicate claim witness id rejected", func(t *testing.T) {
		data := setAtPathAndMarshal(t, base, []any{"findings", 0, "claims"}, []any{
			map[string]any{"id": "policy-instruction:example-policy#example-claim", "digest": "sha256:" + repeatHex('1'), "category": "policy-instruction"},
			map[string]any{"id": "policy-instruction:example-policy#example-claim", "digest": "sha256:" + repeatHex('2'), "category": "spec-outcome"},
		})
		if _, err := DecodeJudgeResult(data); err == nil {
			t.Fatalf("got nil error, want failure (duplicate claim witness id)")
		}
	})

	t.Run("unsorted claim witnesses rejected", func(t *testing.T) {
		data := setAtPathAndMarshal(t, base, []any{"findings", 0, "claims"}, []any{
			map[string]any{"id": "spec-outcome:spec/example-story#outcome-1", "digest": "sha256:" + repeatHex('2'), "category": "spec-outcome"},
			map[string]any{"id": "policy-instruction:example-policy#example-claim", "digest": "sha256:" + repeatHex('1'), "category": "policy-instruction"},
		})
		if _, err := DecodeJudgeResult(data); err == nil {
			t.Fatalf("got nil error, want failure (unsorted claim witnesses)")
		}
	})

	t.Run("unsorted categories rejected", func(t *testing.T) {
		data := setAtPathAndMarshal(t, base, []any{"findings", 0, "categories"}, []any{"spec-outcome", "policy-instruction"})
		if _, err := DecodeJudgeResult(data); err == nil {
			t.Fatalf("got nil error, want failure (unsorted categories)")
		}
	})

	t.Run("duplicate categories rejected", func(t *testing.T) {
		data := setAtPathAndMarshal(t, base, []any{"findings", 0, "categories"}, []any{"policy-instruction", "policy-instruction"})
		if _, err := DecodeJudgeResult(data); err == nil {
			t.Fatalf("got nil error, want failure (duplicate categories)")
		}
	})

	t.Run("blank explanation rejected", func(t *testing.T) {
		data := setAtPathAndMarshal(t, base, []any{"findings", 0, "explanation"}, "   ")
		if _, err := DecodeJudgeResult(data); err == nil {
			t.Fatalf("got nil error, want failure (blank explanation)")
		}
	})

	t.Run("multiline explanation rejected", func(t *testing.T) {
		data := setAtPathAndMarshal(t, base, []any{"findings", 0, "explanation"}, "line one\nline two")
		if _, err := DecodeJudgeResult(data); err == nil {
			t.Fatalf("got nil error, want failure (multiline explanation)")
		}
	})
}

func repeatHex(b byte) string {
	out := make([]byte, 64)
	for i := range out {
		out[i] = b
	}
	return string(out)
}

// setAtPathAndMarshal mutates data at path and returns compact (not
// necessarily canonical) JSON bytes suitable as DECODE input for a
// negative test — DecodeJudgeResult et al. must reject on the underlying
// grammar violation, independent of whether the bytes are also canonical.
func setAtPathAndMarshal(t *testing.T, data []byte, path []any, value any) []byte {
	t.Helper()
	tree := setAtPath(t, data, path, value)
	out, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("test setup: marshal: %v", err)
	}
	return append(out, '\n')
}

// --- Judgment ---------------------------------------------------------------

func TestDecodeJudgment_CanonicalRoundTrip(t *testing.T) {
	data := mustReadFixture(t, "judgment.json")
	j, err := DecodeJudgment(data)
	if err != nil {
		t.Fatalf("DecodeJudgment: %v", err)
	}
	out, err := EncodeJudgment(j)
	if err != nil {
		t.Fatalf("EncodeJudgment: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("EncodeJudgment round-trip mismatch:\n got: %s\nwant: %s", out, data)
	}
}

func TestDecodeJudgment_StrictDecodeMatrix(t *testing.T) {
	base := mustReadFixture(t, "judgment.json")
	cases := map[string][]byte{
		"unknown top-level field": withTopLevelField(t, base, "bogus", `"x"`),
		"duplicate top-level key": duplicateTopLevelKey(t, base, "schema"),
		"trailing data":           withTrailingData(base),
		"invalid utf-8":           withInvalidUTF8(base),
		"noncanonical byte order": reorderedNoncanonically(t, base),
		"wrong schema":            withTopLevelField(t, base, "schema", `"verdi.bogus/v1"`),
		"exchange absent":         withoutTopLevelField(t, base, "exchange"),
		"digest absent":           withoutTopLevelField(t, base, "digest"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeJudgment(data); err == nil {
				t.Fatalf("DecodeJudgment(%s): got nil error, want failure", name)
			}
		})
	}
}

func TestDecodeJudgment_PathKeyVsRecordDigestForm(t *testing.T) {
	base := mustReadFixture(t, "judgment.json")

	t.Run("tree_hash rejects sha256 prefix", func(t *testing.T) {
		data := withTopLevelField(t, base, "tree_hash", `"sha256:`+repeatHex('a')+`"`)
		if _, err := DecodeJudgment(data); err == nil {
			t.Fatalf("got nil error, want failure (prefixed tree_hash)")
		}
	})
	t.Run("tree_hash rejects uppercase", func(t *testing.T) {
		data := withTopLevelField(t, base, "tree_hash", `"`+string(bytes.Repeat([]byte("A"), 64))+`"`)
		if _, err := DecodeJudgment(data); err == nil {
			t.Fatalf("got nil error, want failure (uppercase tree_hash)")
		}
	})
	t.Run("tree_hash rejects short value", func(t *testing.T) {
		data := withTopLevelField(t, base, "tree_hash", `"abc"`)
		if _, err := DecodeJudgment(data); err == nil {
			t.Fatalf("got nil error, want failure (short tree_hash)")
		}
	})
	t.Run("top-level input_digest rejects sha256 prefix", func(t *testing.T) {
		data := withTopLevelField(t, base, "input_digest", `"sha256:`+repeatHex('b')+`"`)
		if _, err := DecodeJudgment(data); err == nil {
			t.Fatalf("got nil error, want failure (prefixed input_digest)")
		}
	})
	t.Run("exchange.input_digest requires sha256 prefix", func(t *testing.T) {
		data := setAtPathAndMarshal(t, base, []any{"exchange", "input_digest"}, repeatHex('4'))
		if _, err := DecodeJudgment(data); err == nil {
			t.Fatalf("got nil error, want failure (bare exchange.input_digest)")
		}
	})
}

func TestDecodeJudgment_SelfDigestMutation(t *testing.T) {
	base := mustReadFixture(t, "judgment.json")

	t.Run("mutated role, stale digest", func(t *testing.T) {
		data := setAtPathAndMarshal(t, base, []any{"exchange", "role"}, "challenger")
		if _, err := DecodeJudgment(data); err == nil {
			t.Fatalf("got nil error, want digest-verification failure")
		}
	})

	t.Run("mutated role, forged consistent digest", func(t *testing.T) {
		tree := setAtPath(t, base, []any{"exchange", "role"}, "bogus-role")
		data := redigestTopLevel(t, tree)
		if _, err := DecodeJudgment(data); err == nil {
			t.Fatalf("got nil error, want failure (unknown role, even with a consistent digest)")
		}
	})
}

// --- Report -------------------------------------------------------------

func TestDecodeReport_CanonicalRoundTrip(t *testing.T) {
	data := mustReadFixture(t, "report.json")
	r, err := DecodeReport(data)
	if err != nil {
		t.Fatalf("DecodeReport: %v", err)
	}
	out, err := EncodeReport(r)
	if err != nil {
		t.Fatalf("EncodeReport: %v", err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("EncodeReport round-trip mismatch:\n got: %s\nwant: %s", out, data)
	}
}

func TestDecodeReport_StrictDecodeMatrix(t *testing.T) {
	base := mustReadFixture(t, "report.json")
	cases := map[string][]byte{
		"unknown top-level field": withTopLevelField(t, base, "bogus", `"x"`),
		"duplicate top-level key": duplicateTopLevelKey(t, base, "schema"),
		"trailing data":           withTrailingData(base),
		"invalid utf-8":           withInvalidUTF8(base),
		"noncanonical byte order": reorderedNoncanonically(t, base),
		"wrong schema":            withTopLevelField(t, base, "schema", `"verdi.bogus/v1"`),
		"unknown verdict":         withTopLevelField(t, base, "verdict", `"bogus"`),
		"mechanical absent":       withoutTopLevelField(t, base, "mechanical"),
		"semantic absent":         withoutTopLevelField(t, base, "semantic"),
		"disclosures null":        withTopLevelField(t, base, "disclosures", `null`),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeReport(data); err == nil {
				t.Fatalf("DecodeReport(%s): got nil error, want failure", name)
			}
		})
	}
}

func TestDecodeReport_SelfDigestMutation(t *testing.T) {
	base := mustReadFixture(t, "report.json")

	cases := map[string][]any{
		"mechanical[0].state": {"mechanical", 0, "state"},
		"semantic[0].state":   {"semantic", 0, "state"},
		"verdict":             {"verdict"},
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			data := setAtPathAndMarshal(t, base, path, "bogus-value")
			if _, err := DecodeReport(data); err == nil {
				t.Fatalf("got nil error, want digest-verification failure (%s)", name)
			}
		})
	}
}

// TestDecodeReport_UnsortedRows proves mechanical/semantic row ordering is
// checked, not silently accepted or re-sorted.
func TestDecodeReport_UnsortedRows(t *testing.T) {
	base := mustReadFixture(t, "report.json")

	t.Run("duplicate mechanical row id", func(t *testing.T) {
		tree := setAtPath(t, base, []any{"mechanical"}, nil)
		var m map[string]any
		dec := json.NewDecoder(bytes.NewReader(base))
		dec.UseNumber()
		if err := dec.Decode(&m); err != nil {
			t.Fatal(err)
		}
		mech := m["mechanical"].([]any)
		dup := append(append([]any{}, mech...), mech[0])
		tree["mechanical"] = dup
		data := redigestTopLevel(t, tree)
		if _, err := DecodeReport(data); err == nil {
			t.Fatalf("got nil error, want failure (duplicate mechanical row id)")
		}
	})
}

// TestDecodeReport_DisclosureVocabulary proves the disclosure code set is
// the fourteen existing contextcompile.DisclosureCode values plus exactly
// "solo-principal-collapse", and that an unknown code fails closed.
func TestDecodeReport_DisclosureVocabulary(t *testing.T) {
	base := mustReadFixture(t, "report.json")

	valid := []string{"actor-resolution-unproven", "solo-principal-collapse"}
	for _, code := range valid {
		t.Run("valid/"+code, func(t *testing.T) {
			tree := setAtPath(t, base, []any{"disclosures"}, []any{
				map[string]any{"code": code, "witnesses": []any{}},
			})
			data := redigestTopLevel(t, tree)
			if _, err := DecodeReport(data); err != nil {
				t.Fatalf("valid disclosure code %q: %v", code, err)
			}
		})
	}

	t.Run("unknown disclosure code", func(t *testing.T) {
		tree := setAtPath(t, base, []any{"disclosures"}, []any{
			map[string]any{"code": "bogus-disclosure", "witnesses": []any{}},
		})
		data := redigestTopLevel(t, tree)
		if _, err := DecodeReport(data); err == nil {
			t.Fatalf("got nil error, want failure (unknown disclosure code)")
		}
	})
}

// TestDecodeReport_SemanticClaimWitnessCategoryClosure is the pinpoint
// evidence test for the policyartifact.SemanticClaimWitness boundary: a
// Report whose embedded semantic claim witness carries a category outside
// the closed §6 source-category vocabulary, with the top-level self-digest
// forged to be internally consistent with that mutation (so digest
// verification ALONE cannot catch it — only independent domain validation
// of the embedded witness can). This is the schema.go/validate.go contract
// this package cannot satisfy without either (a) an exported validation
// seam on policyartifact.SemanticClaimWitness, or (b) duplicating
// policyartifact's private closed witness-category vocabulary, both
// forbidden by the Task 3 mandate.
func TestDecodeReport_SemanticClaimWitnessCategoryClosure(t *testing.T) {
	base := mustReadFixture(t, "report.json")
	tree := setAtPath(t, base, []any{"semantic", 0, "claims", 0, "category"}, "not-a-real-category")
	data := redigestTopLevel(t, tree)
	if _, err := DecodeReport(data); err == nil {
		t.Fatalf("got nil error, want failure (out-of-vocabulary semantic claim witness category, digest-consistent forgery)")
	}
}

// TestDecodeReport_DispositionConclusionClosure is the second pinpoint
// evidence test: a Report whose embedded DispositionResolution.Conclusion
// (policyartifact.DispositionConclusion) carries a value outside the
// closed {conflict, no-conflict} vocabulary, again with a digest-consistent
// forgery. policyartifact.DispositionConclusion exports no Validate method
// either, so this hits the identical boundary.
func TestDecodeReport_DispositionConclusionClosure(t *testing.T) {
	base := mustReadFixture(t, "report.json")
	tree := setAtPath(t, base, []any{"semantic", 0, "dispositions", 0, "conclusion"}, "not-a-real-conclusion")
	data := redigestTopLevel(t, tree)
	if _, err := DecodeReport(data); err == nil {
		t.Fatalf("got nil error, want failure (out-of-vocabulary disposition conclusion, digest-consistent forgery)")
	}
}
