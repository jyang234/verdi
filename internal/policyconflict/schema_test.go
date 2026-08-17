package policyconflict

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
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
	tree, ok := generic.(map[string]any)
	if !ok {
		t.Fatalf("test setup: document root is not an object")
	}
	setAtPathIn(t, tree, path, value)
	return tree
}

// setAtPathIn is setAtPath's walker over an already-decoded tree, so a
// single case can stack several mutations before one redigest.
func setAtPathIn(t *testing.T, tree map[string]any, path []any, value any) {
	t.Helper()
	var cur any = tree
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
				return
			}
			cur = m[s]
		case int:
			arr, ok := cur.([]any)
			if !ok {
				t.Fatalf("test setup: path segment %v: not an array", seg)
			}
			if last {
				arr[s] = value
				return
			}
			cur = arr[s]
		default:
			t.Fatalf("test setup: bad path segment type %T", seg)
		}
	}
	t.Fatalf("test setup: empty path")
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

func TestDecodeJudgment_RequiresGovernanceIdentity(t *testing.T) {
	base := mustReadFixture(t, "judgment.json")
	for _, field := range []string{"profile_id", "profile_digest", "authority_digest"} {
		t.Run(field+" absent", func(t *testing.T) {
			if _, err := DecodeJudgment(withoutTopLevelField(t, base, field)); err == nil {
				t.Fatalf("DecodeJudgment accepted missing %s", field)
			}
		})
	}
	for _, field := range []string{"profile_digest", "authority_digest"} {
		t.Run(field+" is a full digest", func(t *testing.T) {
			data := withTopLevelField(t, base, field, `"`+repeatHex('a')+`"`)
			if _, err := DecodeJudgment(data); err == nil {
				t.Fatalf("DecodeJudgment accepted bare %s", field)
			}
		})
	}
}

func TestDecodeJudgment_ProfileIDGrammar(t *testing.T) {
	base := mustReadFixture(t, "judgment.json")
	tree := setAtPath(t, base, []any{"profile_id"}, "UPPER")
	if _, err := DecodeJudgment(redigestTopLevel(t, tree)); err == nil {
		t.Fatal("DecodeJudgment accepted a profile ID outside governanceprincipal's grammar")
	}

	judgment, err := DecodeJudgment(base)
	if err != nil {
		t.Fatal(err)
	}
	judgment.ProfileID = "UPPER"
	if _, err := EncodeJudgment(judgment); err == nil {
		t.Fatal("EncodeJudgment accepted a profile ID outside governanceprincipal's grammar")
	}
}

func TestDecodeJudgment_RawResultMustEqualParsedResult(t *testing.T) {
	base := mustReadFixture(t, "judgment.json")
	tree := setAtPath(t, base, []any{"exchange", "result", "recommendation"}, "inconclusive")
	setAtPathIn(t, tree, []any{"exchange", "result", "findings"}, []any{})
	data := redigestTopLevel(t, tree)
	if _, err := DecodeJudgment(data); err == nil {
		t.Fatal("DecodeJudgment accepted raw_result bytes that decode to a different result")
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
// of the embedded witness can). It passes through the exported
// SemanticClaimWitness.Validate seam that controller adjudication
// authorized in commit fe0fc401; policyconflict never duplicates
// policyartifact's private closed witness-category vocabulary.
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
// forgery. It reaches that vocabulary through the sibling
// DispositionConclusion.Validate seam authorized in the same commit.
func TestDecodeReport_DispositionConclusionClosure(t *testing.T) {
	base := mustReadFixture(t, "report.json")
	tree := setAtPath(t, base, []any{"semantic", 0, "dispositions", 0, "conclusion"}, "not-a-real-conclusion")
	data := redigestTopLevel(t, tree)
	if _, err := DecodeReport(data); err == nil {
		t.Fatalf("got nil error, want failure (out-of-vocabulary disposition conclusion, digest-consistent forgery)")
	}
}

// --- shared negative-case plumbing (F2/F3/F4/F7/F8) -------------------------

// requireErrContains asserts err is non-nil AND fails for the labelled
// reason: a negative case that passes only because some unrelated wall
// (byte-canonicality, digest mismatch) fired proves nothing about the rule
// under test.
func requireErrContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("got nil error, want failure containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err.Error(), want)
	}
}

// canonicalTree marshals a mutated tree with the real canonjson seam. The
// judge-result and request documents carry no self-digest, so this is their
// analogue of redigestTopLevel: bytes that are genuinely canonical, so a
// rejection can only come from the grammar rule under test.
func canonicalTree(t *testing.T, tree map[string]any) []byte {
	t.Helper()
	out, err := canonjson.Marshal(tree)
	if err != nil {
		t.Fatalf("test setup: marshal canonical: %v", err)
	}
	return out
}

// forgedReport applies one mutation to the report fixture and returns
// canonical bytes carrying a freshly forged, self-consistent digest.
func forgedReport(t *testing.T, base []byte, path []any, value any) []byte {
	t.Helper()
	return redigestTopLevel(t, setAtPath(t, base, path, value))
}

func sha(c byte) string { return "sha256:" + repeatHex(c) }

// validExemptionResolution is a fully conforming embedded exemption row —
// the report fixture carries `"exemptions": []`, so every exemption case
// (positive and negative alike) splices this shape in.
// validExemptionResolution is an all-five-proven resolution: authority
// design §5.5 requires it name at least one MechanicalClaimWitness, keyed by
// the composite (policy_id, claim_id) with the claim's exact digest — the
// exact current claim of the fixture's own enclosing mechanical row.
func validExemptionResolution(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		"id":     "policy-exemption/legacy-service-go",
		"digest": sha('3'),
		"resolution": map[string]any{
			"match": "proven", "freshness": "proven", "scope": "proven",
			"bound": "proven", "authorization": "proven",
		},
		"removed_claims": []any{
			map[string]any{
				"policy_id":    "go-toolchain",
				"claim_id":     "go-version",
				"claim_digest": fixtureClaimDigest(t),
			},
		},
	}
}

// rejectedExemptionResolution is the mirror case: a resolution whose
// authority is not all-proven names the mandatory-present EXPLICIT empty
// removal set, because it removed nothing.
func rejectedExemptionResolution(t *testing.T) map[string]any {
	t.Helper()
	e := validExemptionResolution(t)
	e["resolution"].(map[string]any)["bound"] = "unproven"
	e["removed_claims"] = []any{}
	return e
}

// validJudgmentExchange is a fully conforming embedded exchange for the
// named role, including a raw_digest that really is the digest of the exact
// raw_result bytes it carries.
func validJudgmentExchange(role string) map[string]any {
	raw := `{"findings":[],"recommendation":"no-conflict","schema":"verdi.policy-conflict-judge-result/v1"}` + "\n"
	return map[string]any{
		"role":           role,
		"adapter":        map[string]any{"id": "codex", "version": "1"},
		"model":          "codex-align-judge",
		"command_digest": sha('3'),
		"prompt_digest":  sha('5'),
		"input_digest":   sha('4'),
		"raw_result":     raw,
		"raw_digest":     rawContentDigest([]byte(raw)),
		"result": map[string]any{
			"schema":         JudgeResultSchema,
			"recommendation": "no-conflict",
			"findings":       []any{},
		},
	}
}

func dimensionRow(name string) map[string]any {
	return map[string]any{
		"dimension": name, "state": "disjoint",
		"left": []any{}, "right": []any{}, "intersection": []any{}, "witnesses": []any{},
	}
}

func policyEntry(kind, id string) map[string]any {
	return map[string]any{"kind": kind, "id": id, "digest": sha('e')}
}

// --- F2: closed source-category vocabulary ----------------------------------

// TestDecodeJudgeResult_CategoryVocabularyClosure proves both category-
// bearing members of a judge result are checked against the closed §6
// source-category list, not merely against the single-line-prose grammar.
// The bytes are canonical, so nothing but the vocabulary rule can reject.
func TestDecodeJudgeResult_CategoryVocabularyClosure(t *testing.T) {
	base := mustReadFixture(t, "judge-result.json")

	t.Run("valid categories accepted", func(t *testing.T) {
		if _, err := DecodeJudgeResult(base); err != nil {
			t.Fatalf("DecodeJudgeResult(fixture): %v", err)
		}
	})

	t.Run("unknown claim witness category", func(t *testing.T) {
		tree := setAtPath(t, base, []any{"findings", 0, "claims", 0, "category"}, "not-a-real-category")
		_, err := DecodeJudgeResult(canonicalTree(t, tree))
		requireErrContains(t, err, `unknown witness category "not-a-real-category"`)
	})

	// "not-a-real-category" < "policy-instruction" lexically, so the set is
	// still sorted-unique: only the vocabulary rule can reject it.
	t.Run("unknown finding category", func(t *testing.T) {
		tree := setAtPath(t, base, []any{"findings", 0, "categories"}, []any{"not-a-real-category", "policy-instruction"})
		_, err := DecodeJudgeResult(canonicalTree(t, tree))
		requireErrContains(t, err, `unknown witness category "not-a-real-category"`)
	})
}

// TestDecodeReport_MechanicalClaimWitnessIsNotSemantic pins ledger SI-105(c)
// on the wire: an exemption's removed-claim witness is a
// MechanicalClaimWitness identified by (policy_id, claim_id, claim_digest),
// NOT one of §6's prose ClaimWitness categories. The semantic spelling is
// therefore unknown-field-rejected by the strict decoder, even when the
// top-level self-digest is forged to agree with the mutation.
func TestDecodeReport_MechanicalClaimWitnessIsNotSemantic(t *testing.T) {
	base := mustReadFixture(t, "report.json")

	t.Run("semantic witness spelling rejected", func(t *testing.T) {
		exemption := validExemptionResolution(t)
		exemption["removed_claims"] = []any{map[string]any{
			"id": "policy-instruction:example-policy#example-claim", "digest": sha('1'), "category": "policy-instruction",
		}}
		data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{exemption})
		requireErrContains(t, mustDecodeReportErr(t, data), "unknown field")
	})

	t.Run("category alongside the mechanical witness rejected", func(t *testing.T) {
		exemption := validExemptionResolution(t)
		exemption["removed_claims"].([]any)[0].(map[string]any)["category"] = "policy-instruction"
		data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{exemption})
		requireErrContains(t, mustDecodeReportErr(t, data), "unknown field")
	})
}

// TestDecodeReport_ExemptionResolutionRemovedClaimsCardinality pins authority
// design §5.5's mandatory-present removal set on the wire: nonempty exactly
// for an all-five-proven resolution, explicitly empty for every rejected
// one, and never absent.
func TestDecodeReport_ExemptionResolutionRemovedClaimsCardinality(t *testing.T) {
	base := mustReadFixture(t, "report.json")

	t.Run("rejected resolution with an explicit empty set round-trips", func(t *testing.T) {
		data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{rejectedExemptionResolution(t)})
		report, err := DecodeReport(data)
		if err != nil {
			t.Fatalf("DecodeReport(rejected resolution): %v", err)
		}
		removed := report.Mechanical[0].Exemptions[0].RemovedClaims
		if removed == nil || len(removed) != 0 {
			t.Fatalf("removed_claims = %+v, want a mandatory-present empty set", removed)
		}
		out, err := EncodeReport(report)
		if err != nil {
			t.Fatalf("EncodeReport: %v", err)
		}
		if !bytes.Equal(out, data) {
			t.Fatalf("empty removal set did not survive the round trip:\n got: %s\nwant: %s", out, data)
		}
	})

	negatives := []struct {
		name string
		mut  func(e map[string]any)
		want string
	}{
		{"proven resolution removing nothing", func(e map[string]any) { e["removed_claims"] = []any{} },
			"must name at least one removed claim"},
		{"rejected resolution claiming a removal", func(e map[string]any) {
			e["resolution"].(map[string]any)["scope"] = "unproven"
		}, "must name the explicit empty removal set"},
		{"absent removal set", func(e map[string]any) { delete(e, "removed_claims") },
			"must be non-nil"},
		{"blank policy id", func(e map[string]any) {
			e["removed_claims"].([]any)[0].(map[string]any)["policy_id"] = ""
		}, "policy_id: must be non-empty"},
		{"blank claim id", func(e map[string]any) {
			e["removed_claims"].([]any)[0].(map[string]any)["claim_id"] = ""
		}, "claim_id: must be non-empty"},
		{"malformed claim digest", func(e map[string]any) {
			e["removed_claims"].([]any)[0].(map[string]any)["claim_digest"] = repeatHex('1')
		}, "is not a valid sha256"},
	}
	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			e := validExemptionResolution(t)
			tc.mut(e)
			data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{e})
			requireErrContains(t, mustDecodeReportErr(t, data), tc.want)
		})
	}
}

// TestDecodeReport_MechanicalClaimCompositeIdentity pins ledger SI-105(c) on
// the report wire: row claims sort and deduplicate by the composite
// (policy_id, claim_id), so two policies declaring byte-identical claims are
// two valid records while one repeated composite identity is a duplicate.
func TestDecodeReport_MechanicalClaimCompositeIdentity(t *testing.T) {
	base := mustReadFixture(t, "report.json")
	claimOf := func(policyID, claimID string) map[string]any {
		return reportClaimRecord(t, policyID, claimID)
	}

	t.Run("same claim bytes from two policies are two records", func(t *testing.T) {
		data := forgedReport(t, base, []any{"mechanical", 0, "claims"},
			[]any{claimOf("go-toolchain", "go-version"), claimOf("go-toolchain-overlay", "go-version")})
		report, err := DecodeReport(data)
		if err != nil {
			t.Fatalf("DecodeReport(two policies, identical claim bytes): %v", err)
		}
		if len(report.Mechanical[0].Claims) != 2 {
			t.Fatalf("claims = %+v, want both policy identities retained", report.Mechanical[0].Claims)
		}
	})

	t.Run("duplicate composite identity rejected", func(t *testing.T) {
		data := forgedReport(t, base, []any{"mechanical", 0, "claims"},
			[]any{claimOf("go-toolchain", "go-version"), claimOf("go-toolchain", "go-version")})
		requireErrContains(t, mustDecodeReportErr(t, data), "claims: duplicate identity")
	})

	t.Run("unsorted composite identities rejected", func(t *testing.T) {
		data := forgedReport(t, base, []any{"mechanical", 0, "claims"},
			[]any{claimOf("go-toolchain-overlay", "go-version"), claimOf("go-toolchain", "go-version")})
		requireErrContains(t, mustDecodeReportErr(t, data), "claims: must be sorted ascending")
	})
}

// --- SI-105 wire closure: recomputed digests and exact removal witnesses -----

// fixtureClaim is the exact policyartifact.Claim the report fixture's single
// mechanical row carries. Keeping it in code (rather than reading the
// fixture's own claim_digest back) is what makes fixtureClaimDigest an
// INDEPENDENT recomputation: a fixture carrying an untruthful digest fails
// these tests instead of defining truth for them.
func fixtureClaim() policyartifact.Claim {
	return policyartifact.Claim{
		ID: "go-version", Family: policyartifact.FamilyConfiguration,
		Operator: policyartifact.OpEquals, Subject: "go-toolchain",
		Values: []string{"1.22"},
		Scope: policyartifact.Scope{
			Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{},
		},
		Overridable: false,
	}
}

func fixtureClaimDigest(t *testing.T) string {
	t.Helper()
	d, err := policyartifact.ClaimDigest(fixtureClaim())
	if err != nil {
		t.Fatalf("test setup: ClaimDigest(fixture claim): %v", err)
	}
	return d
}

// reportClaimRecord is the wire form of one TypedClaimRecord carrying the
// fixture's claim shape under the named composite identity, with the
// TRUTHFUL canonical digest of the claim it carries.
func reportClaimRecord(t *testing.T, policyID, claimID string) map[string]any {
	t.Helper()
	c := fixtureClaim()
	c.ID = claimID
	d, err := policyartifact.ClaimDigest(c)
	if err != nil {
		t.Fatalf("test setup: ClaimDigest(%s/%s): %v", policyID, claimID, err)
	}
	return map[string]any{
		"policy_id": policyID, "policy_digest": sha('0'), "claim_digest": d,
		"claim": map[string]any{
			"family": "configuration", "id": claimID, "operator": "equals", "overridable": false,
			"scope":   map[string]any{"environments": []any{}, "paths": []any{}, "phases": []any{}, "refs": []any{}},
			"subject": "go-toolchain", "values": []any{"1.22"},
		},
	}
}

// TestDecodeReport_MechanicalClaimDigestRecomputed pins ledger SI-105 on the
// report wire: a TypedClaimRecord's claim_digest must EQUAL the canonical
// digest of the base claim it carries. The forged report is re-signed, so
// the document is internally consistent and only independent recomputation
// can reject it.
func TestDecodeReport_MechanicalClaimDigestRecomputed(t *testing.T) {
	base := mustReadFixture(t, "report.json")

	t.Run("truthful digest round-trips", func(t *testing.T) {
		report, err := DecodeReport(base)
		if err != nil {
			t.Fatalf("DecodeReport(fixture): %v", err)
		}
		if got, want := report.Mechanical[0].Claims[0].ClaimDigest, fixtureClaimDigest(t); got != want {
			t.Fatalf("fixture claim_digest = %q, want the canonical digest %q of the carried claim", got, want)
		}
	})

	t.Run("stale carried digest refused", func(t *testing.T) {
		data := forgedReport(t, base, []any{"mechanical", 0, "claims", 0, "claim_digest"}, sha('a'))
		requireErrContains(t, mustDecodeReportErr(t, data), "claims[0].claim_digest")
	})

	t.Run("mutated claim body refused", func(t *testing.T) {
		data := forgedReport(t, base, []any{"mechanical", 0, "claims", 0, "claim", "values"}, []any{"1.23"})
		requireErrContains(t, mustDecodeReportErr(t, data), "claims[0].claim_digest")
	})
}

// TestDecodeReport_ExemptionResolutionRemovalWitnessIsCurrentRowClaim pins
// the other half of SI-105 on the wire: every removed_claims witness of an
// all-five-proven resolution must equal an EXACT current claim of its
// ENCLOSING mechanical row, by policy_id, claim_id and digest alike. Both
// forgeries are re-signed, so nothing but the membership rule can reject.
func TestDecodeReport_ExemptionResolutionRemovalWitnessIsCurrentRowClaim(t *testing.T) {
	base := mustReadFixture(t, "report.json")

	t.Run("exact current witness accepted", func(t *testing.T) {
		data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{validExemptionResolution(t)})
		if _, err := DecodeReport(data); err != nil {
			t.Fatalf("DecodeReport(exact current witness): %v", err)
		}
	})

	t.Run("witness absent from the enclosing row", func(t *testing.T) {
		e := validExemptionResolution(t)
		e["removed_claims"].([]any)[0].(map[string]any)["policy_id"] = "ghost-policy"
		data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{e})
		requireErrContains(t, mustDecodeReportErr(t, data), "absent from")
	})

	t.Run("witness digest mismatching the enclosing row claim", func(t *testing.T) {
		e := validExemptionResolution(t)
		e["removed_claims"].([]any)[0].(map[string]any)["claim_digest"] = sha('a')
		data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{e})
		requireErrContains(t, mustDecodeReportErr(t, data), "current claim digests to")
	})
}

// --- F4: embedded exemption resolutions -------------------------------------

// TestDecodeReport_ExemptionResolutions exercises the embedded exemption
// row end to end: the fixture's own `"exemptions": []` never reaches this
// code, so the positive case is what proves the doc conversion round-trips
// at all, and the negatives pin each of its own rules.
func TestDecodeReport_ExemptionResolutions(t *testing.T) {
	base := mustReadFixture(t, "report.json")

	t.Run("valid exemption round-trips", func(t *testing.T) {
		data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{validExemptionResolution(t)})
		report, err := DecodeReport(data)
		if err != nil {
			t.Fatalf("DecodeReport(valid exemption): %v", err)
		}
		if got := len(report.Mechanical[0].Exemptions); got != 1 {
			t.Fatalf("exemptions = %d, want 1", got)
		}
		want := MechanicalClaimWitness{PolicyID: "go-toolchain", ClaimID: "go-version", ClaimDigest: fixtureClaimDigest(t)}
		if got := report.Mechanical[0].Exemptions[0].RemovedClaims[0]; got != want {
			t.Fatalf("removed claim = %+v, want the composite witness %+v", got, want)
		}
		out, err := EncodeReport(report)
		if err != nil {
			t.Fatalf("EncodeReport(valid exemption): %v", err)
		}
		if !bytes.Equal(out, data) {
			t.Fatalf("exemption round-trip mismatch:\n got: %s\nwant: %s", out, data)
		}
	})

	negatives := []struct {
		name string
		mut  func(e map[string]any)
		want string
	}{
		{"blank id", func(e map[string]any) { e["id"] = "" }, "id: must be non-empty"},
		{"malformed digest", func(e map[string]any) { e["digest"] = "not-a-digest" }, "digest: \"not-a-digest\" is not a valid sha256"},
		{"unknown resolution state", func(e map[string]any) {
			e["resolution"].(map[string]any)["freshness"] = "probably"
		}, "resolution.freshness"},
	}
	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			e := validExemptionResolution(t)
			tc.mut(e)
			data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{e})
			requireErrContains(t, mustDecodeReportErr(t, data), tc.want)
		})
	}

	t.Run("duplicate removed claim composite identity", func(t *testing.T) {
		e := validExemptionResolution(t)
		dup := e["removed_claims"].([]any)[0]
		e["removed_claims"] = []any{dup, dup}
		data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{e})
		requireErrContains(t, mustDecodeReportErr(t, data), "removed_claims: duplicate identity")
	})

	t.Run("unsorted removed claim composite identities", func(t *testing.T) {
		e := validExemptionResolution(t)
		first := map[string]any{"policy_id": "go-toolchain", "claim_id": "go-version", "claim_digest": fixtureClaimDigest(t)}
		second := map[string]any{"policy_id": "aardvark-policy", "claim_id": "go-version", "claim_digest": fixtureClaimDigest(t)}
		e["removed_claims"] = []any{first, second}
		data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{e})
		requireErrContains(t, mustDecodeReportErr(t, data), "removed_claims: must be sorted ascending")
	})

	// Both departures are real: the enclosing row carries BOTH policy
	// identities' byte-identical claims, so each witness names an exact
	// current row claim and only policy identity distinguishes them.
	t.Run("two policies departed from for identical claim bytes", func(t *testing.T) {
		e := validExemptionResolution(t)
		e["removed_claims"] = []any{
			map[string]any{"policy_id": "go-toolchain", "claim_id": "go-version", "claim_digest": fixtureClaimDigest(t)},
			map[string]any{"policy_id": "go-toolchain-overlay", "claim_id": "go-version", "claim_digest": fixtureClaimDigest(t)},
		}
		tree := setAtPath(t, base, []any{"mechanical", 0, "claims"}, []any{
			reportClaimRecord(t, "go-toolchain", "go-version"),
			reportClaimRecord(t, "go-toolchain-overlay", "go-version"),
		})
		setAtPathIn(t, tree, []any{"mechanical", 0, "exemptions"}, []any{e})
		data := redigestTopLevel(t, tree)
		report, err := DecodeReport(data)
		if err != nil {
			t.Fatalf("DecodeReport(two policy identities, identical claim bytes): %v", err)
		}
		if len(report.Mechanical[0].Exemptions[0].RemovedClaims) != 2 {
			t.Fatalf("removed_claims = %+v, want both policy identities retained", report.Mechanical[0].Exemptions[0].RemovedClaims)
		}
	})

	t.Run("duplicate exemption id", func(t *testing.T) {
		data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"},
			[]any{validExemptionResolution(t), validExemptionResolution(t)})
		requireErrContains(t, mustDecodeReportErr(t, data), "exemptions: duplicate identity")
	})

	t.Run("unsorted exemptions", func(t *testing.T) {
		first := validExemptionResolution(t)
		second := validExemptionResolution(t)
		second["id"] = "policy-exemption/aardvark"
		data := forgedReport(t, base, []any{"mechanical", 0, "exemptions"}, []any{first, second})
		requireErrContains(t, mustDecodeReportErr(t, data), "exemptions: must be sorted ascending")
	})
}

func mustDecodeReportErr(t *testing.T, data []byte) error {
	t.Helper()
	_, err := DecodeReport(data)
	return err
}

// --- F4/F7: semantic primary and challenger exchanges -----------------------

// TestDecodeReport_SemanticExchanges exercises the semantic row's optional
// judgment exchanges. The malformed cases must surface the exchange's OWN
// grammar error: a swallowed conversion error would leave only the generic
// "not the canonical encoding" wall, which proves nothing about why the
// exchange is illegal.
func TestDecodeReport_SemanticExchanges(t *testing.T) {
	base := mustReadFixture(t, "report.json")

	t.Run("valid primary and challenger round-trip", func(t *testing.T) {
		tree := setAtPath(t, base, []any{"semantic", 0, "primary"}, validJudgmentExchange("primary"))
		setAtPathIn(t, tree, []any{"semantic", 0, "challenger"}, validJudgmentExchange("challenger"))
		data := redigestTopLevel(t, tree)
		report, err := DecodeReport(data)
		if err != nil {
			t.Fatalf("DecodeReport(valid exchanges): %v", err)
		}
		if report.Semantic[0].Primary == nil || report.Semantic[0].Challenger == nil {
			t.Fatalf("primary/challenger = %v/%v, want both present", report.Semantic[0].Primary, report.Semantic[0].Challenger)
		}
		out, err := EncodeReport(report)
		if err != nil {
			t.Fatalf("EncodeReport(valid exchanges): %v", err)
		}
		if !bytes.Equal(out, data) {
			t.Fatalf("exchange round-trip mismatch:\n got: %s\nwant: %s", out, data)
		}
	})

	t.Run("primary carrying the challenger role", func(t *testing.T) {
		data := forgedReport(t, base, []any{"semantic", 0, "primary"}, validJudgmentExchange("challenger"))
		requireErrContains(t, mustDecodeReportErr(t, data), `primary.role: must be "primary"`)
	})

	t.Run("challenger carrying the primary role", func(t *testing.T) {
		data := forgedReport(t, base, []any{"semantic", 0, "challenger"}, validJudgmentExchange("primary"))
		requireErrContains(t, mustDecodeReportErr(t, data), `challenger.role: must be "challenger"`)
	})

	t.Run("exchange missing a mandatory member", func(t *testing.T) {
		e := validJudgmentExchange("primary")
		delete(e, "model")
		data := forgedReport(t, base, []any{"semantic", 0, "primary"}, e)
		requireErrContains(t, mustDecodeReportErr(t, data), "judgment.exchange.model is missing")
	})

	t.Run("exchange raw_digest not the digest of raw_result", func(t *testing.T) {
		e := validJudgmentExchange("primary")
		e["raw_digest"] = sha('7')
		data := forgedReport(t, base, []any{"semantic", 0, "primary"}, e)
		requireErrContains(t, mustDecodeReportErr(t, data), "does not match the exact bytes carried in raw_result")
	})

	t.Run("exchange carrying an invalid inner result", func(t *testing.T) {
		e := validJudgmentExchange("primary")
		e["result"].(map[string]any)["recommendation"] = "maybe"
		data := forgedReport(t, base, []any{"semantic", 0, "primary"}, e)
		requireErrContains(t, mustDecodeReportErr(t, data), "does not equal the strict-decoded raw_result")
	})
}

// --- F4: candidate target identity ------------------------------------------

// TestEncodeReport_CandidateTargetIdentity exercises the InputIdentity
// union's candidate arm, which no committed fixture carries: the row is
// built in code and proven by an EncodeReport/DecodeReport round-trip.
func TestEncodeReport_CandidateTargetIdentity(t *testing.T) {
	base := mustReadFixture(t, "report.json")
	report, err := DecodeReport(base)
	if err != nil {
		t.Fatalf("DecodeReport(fixture): %v", err)
	}

	candidate := &CandidateIdentity{
		Ref:           "spec/example-story",
		Path:          ".verdi/specs/active/example-story.md",
		Branch:        "feature/example",
		Head:          "0123456789abcdef0123456789abcdef01234567",
		Blob:          "89abcdef0123456789abcdef0123456789abcdef",
		ContentDigest: sha('a'),
		Scope:         policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Adapter:       contextcompile.AdapterRef{ID: "codex", Version: "1"},
		GrantDigest:   sha('b'),
	}
	report.Input.Target = TargetIdentity{Kind: TargetAcceptanceCandidate, Candidate: candidate}

	data, err := EncodeReport(report)
	if err != nil {
		t.Fatalf("EncodeReport(candidate target): %v", err)
	}
	decoded, err := DecodeReport(data)
	if err != nil {
		t.Fatalf("DecodeReport(candidate target): %v", err)
	}
	if decoded.Input.Target.Candidate == nil || decoded.Input.Target.Accepted != nil {
		t.Fatalf("decoded target = %+v, want exactly the candidate arm", decoded.Input.Target)
	}
	if decoded.Input.Target.Candidate.Blob != candidate.Blob {
		t.Fatalf("candidate blob = %q, want %q", decoded.Input.Target.Candidate.Blob, candidate.Blob)
	}

	negatives := []struct {
		name string
		mut  func(r *Report)
		want string
	}{
		{"candidate head not full 40-hex", func(r *Report) { r.Input.Target.Candidate.Head = "abc" }, "candidate.head"},
		{"candidate blob not full 40-hex", func(r *Report) { r.Input.Target.Candidate.Blob = strings.ToUpper(candidate.Blob) }, "candidate.blob"},
		{"candidate content digest malformed", func(r *Report) { r.Input.Target.Candidate.ContentDigest = "nope" }, "candidate.content_digest"},
		{"candidate grant digest malformed", func(r *Report) { r.Input.Target.Candidate.GrantDigest = "nope" }, "candidate.grant_digest"},
		{"candidate ref empty", func(r *Report) { r.Input.Target.Candidate.Ref = "" }, "candidate.ref"},
		{"candidate adapter version empty", func(r *Report) { r.Input.Target.Candidate.Adapter.Version = "" }, "candidate.adapter.version"},
		{"both arms present", func(r *Report) { r.Input.Target.Accepted = &AcceptedIdentity{ManifestDigest: sha('b')} }, "requires exactly candidate present"},
		{"neither arm present", func(r *Report) { r.Input.Target.Candidate = nil }, "requires exactly candidate present"},
	}
	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			mutated := report
			c := *candidate
			mutated.Input.Target = TargetIdentity{Kind: TargetAcceptanceCandidate, Candidate: &c}
			tc.mut(&mutated)
			_, err := EncodeReport(mutated)
			requireErrContains(t, err, tc.want)
		})
	}
}

// --- F3/F4/F8: digest-consistent forged report mutations --------------------

// TestDecodeReport_ForgedMutationMatrix mutates every nested report enum,
// digest, set, and order, each with the top-level self-digest re-forged so
// the document is internally consistent. Digest verification therefore
// cannot catch any of these; only independent domain validation can, and
// each case asserts the specific rule that must fire.
func TestDecodeReport_ForgedMutationMatrix(t *testing.T) {
	base := mustReadFixture(t, "report.json")

	cases := []struct {
		name  string
		path  []any
		value any
		want  string
	}{
		// nested enums
		{"unknown reason code", []any{"mechanical", 0, "reasons"}, []any{"bogus-reason"}, `unknown reason code "bogus-reason"`},
		{"unsorted reason codes", []any{"mechanical", 0, "reasons"}, []any{"scope-disjoint", "mechanical-conflict"}, "reasons: must be sorted ascending"},
		{"duplicate reason codes", []any{"mechanical", 0, "reasons"}, []any{"scope-disjoint", "scope-disjoint"}, "reasons: duplicate identity"},
		{"unknown semantic reason code", []any{"semantic", 0, "reasons"}, []any{"bogus-reason"}, `unknown reason code "bogus-reason"`},
		{"unknown family", []any{"mechanical", 0, "family"}, "posture", "unknown constraint family"},
		{"unknown mechanical state", []any{"mechanical", 0, "state"}, "maybe", `unknown proof state "maybe"`},
		{"unknown semantic state", []any{"semantic", 0, "state"}, "maybe", `unknown proof state "maybe"`},
		{"unknown scope-proof state", []any{"mechanical", 0, "scope", "state"}, "maybe", `unknown scope state "maybe"`},
		{"unknown dimension state", []any{"mechanical", 0, "scope", "dimensions", 0, "state"}, "maybe", `unknown scope state "maybe"`},
		{"unknown solver state", []any{"mechanical", 0, "before", "state"}, "maybe", `unknown solver state "maybe"`},
		{"unknown post-exemption solver state", []any{"mechanical", 0, "after", "state"}, "maybe", `unknown solver state "maybe"`},
		{"unknown verdict", []any{"verdict"}, "maybe", `unknown verdict "maybe"`},
		{"unknown target kind", []any{"input", "target", "kind"}, "bogus", `unknown target kind "bogus"`},
		{"unknown policy entry kind", []any{"input", "policy_entries", 0, "kind"}, "bogus", `kind: unknown value "bogus"`},
		{"unknown scope dimension name", []any{"mechanical", 0, "scope", "dimensions", 0, "dimension"}, "timezone", `dimension: unknown value "timezone"`},

		// authority-resolution sub-states, one case per member
		{"resolution.match unknown", []any{"semantic", 0, "dispositions", 0, "resolution", "match"}, "maybe", "resolution.match:"},
		{"resolution.freshness unknown", []any{"semantic", 0, "dispositions", 0, "resolution", "freshness"}, "maybe", "resolution.freshness:"},
		{"resolution.scope unknown", []any{"semantic", 0, "dispositions", 0, "resolution", "scope"}, "maybe", "resolution.scope:"},
		{"resolution.bound unknown", []any{"semantic", 0, "dispositions", 0, "resolution", "bound"}, "maybe", "resolution.bound:"},
		{"resolution.authorization unknown", []any{"semantic", 0, "dispositions", 0, "resolution", "authorization"}, "maybe", "resolution.authorization:"},

		// digest formats: case, length, and missing "sha256:" prefix
		{"constitution digest uppercase", []any{"input", "constitution_digest"}, "sha256:" + strings.ToUpper(repeatHex('c')), "constitution_digest"},
		{"effective policy digest too short", []any{"input", "effective_policy_digest"}, "sha256:abcdef", "effective_policy_digest"},
		{"policy entry digest unprefixed", []any{"input", "policy_entries", 0, "digest"}, repeatHex('e'), "policy_entries[0].digest"},
		{"profile digest unprefixed", []any{"input", "profile", "digest"}, repeatHex('f'), "profile.digest"},
		{"accepted manifest digest malformed", []any{"input", "target", "accepted", "manifest_digest"}, "sha256:" + repeatHex('b') + "bb", "accepted.manifest_digest"},
		{"typed claim policy digest unprefixed", []any{"mechanical", 0, "claims", 0, "policy_digest"}, repeatHex('0'), "claims[0].policy_digest"},
		{"typed claim digest uppercase", []any{"mechanical", 0, "claims", 0, "claim_digest"}, "sha256:" + strings.ToUpper(repeatHex('a')), "claims[0].claim_digest"},
		{"semantic input id unprefixed", []any{"semantic", 0, "input_id"}, repeatHex('2'), "input_id"},
		{"disposition digest too short", []any{"semantic", 0, "dispositions", 0, "digest"}, "sha256:aa", "dispositions[0].digest"},
		{"semantic claim digest malformed", []any{"semantic", 0, "claims", 0, "digest"}, "sha256:nothex", "is not sha256:<64 hex> form"},
		{"semantic claim authority digest malformed", []any{"semantic", 0, "claims", 0, "authority_digest"}, repeatHex('7'), "authority_digest"},

		// evaluated_on
		{"evaluated_on wrong shape", []any{"input", "evaluated_on"}, "2026-8-12", "is not YYYY-MM-DD form"},
		{"evaluated_on not a calendar date", []any{"input", "evaluated_on"}, "2026-02-31", "is not a real calendar date"},

		// solver-proof sets: sorted-unique in canonical lexical order
		{"unsorted solver values", []any{"mechanical", 0, "before", "values"}, []any{"1.23", "1.22"}, "before.values: must be sorted ascending"},
		{"duplicate solver values", []any{"mechanical", 0, "before", "values"}, []any{"1.22", "1.22"}, "before.values: duplicate identity"},
		{"unsorted solver required", []any{"mechanical", 0, "after", "required"}, []any{"beta", "alpha"}, "after.required: must be sorted ascending"},
		{"duplicate solver forbidden", []any{"mechanical", 0, "before", "forbidden"}, []any{"alpha", "alpha"}, "before.forbidden: duplicate identity"},
		{"unsorted solver witnesses", []any{"mechanical", 0, "after", "witnesses"}, []any{"w-2", "w-1"}, "after.witnesses: must be sorted ascending"},

		// dimension-proof sets
		{"unsorted dimension left", []any{"mechanical", 0, "scope", "dimensions", 0, "left"}, []any{"review", "build"}, "left: must be sorted ascending"},
		{"duplicate dimension right", []any{"mechanical", 0, "scope", "dimensions", 0, "right"}, []any{"review", "review"}, "right: duplicate identity"},
		{"unsorted dimension intersection", []any{"mechanical", 0, "scope", "dimensions", 0, "intersection"}, []any{"review", "build"}, "intersection: must be sorted ascending"},
		{"duplicate dimension witnesses", []any{"mechanical", 0, "scope", "dimensions", 0, "witnesses"}, []any{"w-1", "w-1"}, "witnesses: duplicate identity"},

		// scope-proof dimension rows: unique, known, in the fixed §4.4 order
		{"duplicate dimension row", []any{"mechanical", 0, "scope", "dimensions"}, []any{dimensionRow("phase"), dimensionRow("phase")}, "dimensions: duplicate dimension"},
		{"dimension rows out of §4.4 order", []any{"mechanical", 0, "scope", "dimensions"}, []any{dimensionRow("ref"), dimensionRow("phase")}, "dimensions: must be in phase, environment, path, ref order"},
		{"unknown-mechanical dimension rows out of order", []any{"semantic", 0, "unknown_mechanicals"}, []any{map[string]any{
			"id":     "mechanical/unresolved",
			"claims": []any{reportClaimRecord(t, "go-toolchain", "go-version")},
			"scope": map[string]any{
				"state":      "unknown",
				"dimensions": []any{dimensionRow("path"), dimensionRow("environment")},
			},
		}}, "must be in phase, environment, path, ref order"},

		// policy-entry order and duplication
		{"unsorted policy entries", []any{"input", "policy_entries"}, []any{policyEntry("policy", "go-toolchain"), policyEntry("exemption", "legacy")}, "policy_entries: must be sorted ascending"},
		{"duplicate policy entries", []any{"input", "policy_entries"}, []any{policyEntry("policy", "go-toolchain"), policyEntry("policy", "go-toolchain")}, "policy_entries: duplicate identity"},

		// disclosure order and duplication
		{"unsorted disclosure witnesses", []any{"disclosures"}, []any{map[string]any{"code": "actor-resolution-unproven", "witnesses": []any{"w-2", "w-1"}}}, "witnesses: must be sorted ascending"},
		{"duplicate disclosure witnesses", []any{"disclosures"}, []any{map[string]any{"code": "actor-resolution-unproven", "witnesses": []any{"w-1", "w-1"}}}, "witnesses: duplicate identity"},
		{"duplicate disclosure codes", []any{"disclosures"}, []any{
			map[string]any{"code": "actor-resolution-unproven", "witnesses": []any{}},
			map[string]any{"code": "actor-resolution-unproven", "witnesses": []any{}},
		}, "disclosures: duplicate identity"},

		// semantic claim ordering and embedded witness scope
		{"duplicate semantic claim ids", []any{"semantic", 0, "claims", 1, "id"}, "ac-1", "claims: duplicate identity"},
		{"unsorted semantic witness scope paths", []any{"semantic", 0, "claims", 0, "scope", "paths"}, []any{"src/", "docs/"}, "paths: must be sorted ascending"},
		{"unsorted semantic witness scope refs", []any{"semantic", 0, "claims", 0, "scope", "refs"}, []any{"spec/zeta", "spec/alpha"}, "refs: must be sorted ascending"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := forgedReport(t, base, tc.path, tc.value)
			requireErrContains(t, mustDecodeReportErr(t, data), tc.want)
		})
	}
}

// TestDecodeReport_ScopeDimensionSubsequence proves the §4.4 dimension
// order is a SUBSEQUENCE rule, not an all-four-required rule: any ordered
// subset of phase, environment, path, ref is legal.
func TestDecodeReport_ScopeDimensionSubsequence(t *testing.T) {
	base := mustReadFixture(t, "report.json")
	valid := [][]any{
		{dimensionRow("phase")},
		{dimensionRow("environment"), dimensionRow("ref")},
		{dimensionRow("phase"), dimensionRow("environment"), dimensionRow("path"), dimensionRow("ref")},
	}
	for i, rows := range valid {
		t.Run(fmt.Sprintf("subsequence/%d", i), func(t *testing.T) {
			data := forgedReport(t, base, []any{"mechanical", 0, "scope", "dimensions"}, rows)
			if _, err := DecodeReport(data); err != nil {
				t.Fatalf("DecodeReport(%d dimension rows): %v", len(rows), err)
			}
		})
	}
}

// TestValidateAuthorityResolution_DeterministicFieldSelection pins F10: when
// several members are simultaneously invalid, the reported member must be
// the first in the fixed match/freshness/scope/bound/authorization order,
// every time. A map range would pick one at random per call.
func TestValidateAuthorityResolution_DeterministicFieldSelection(t *testing.T) {
	all := AuthorityResolution{Match: "a", Freshness: "b", Scope: "c", Bound: "d", Authorization: "e"}
	for i := 0; i < 100; i++ {
		err := validateAuthorityResolution("row.resolution", all)
		requireErrContains(t, err, "row.resolution.match:")
	}

	// With match valid, freshness is next in the fixed order.
	partial := AuthorityResolution{Match: ProofProven, Freshness: "b", Scope: "c", Bound: "d", Authorization: "e"}
	for i := 0; i < 100; i++ {
		err := validateAuthorityResolution("row.resolution", partial)
		requireErrContains(t, err, "row.resolution.freshness:")
	}
}
