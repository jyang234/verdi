package align

import "testing"

// TestStripJudgeFence pins the fast-path fence-strip's exact behavior
// (unchanged from the historical inner-parse): a bare object passes through,
// a ```json or ``` fence is stripped, surrounding whitespace is trimmed, and
// prose with no fence is left intact for the slow-path scan to handle.
func TestStripJudgeFence(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare object unchanged", `{"findings":[]}`, `{"findings":[]}`},
		{"json fence stripped", "```json\n{\"findings\":[]}\n```", `{"findings":[]}`},
		{"plain fence stripped", "```\n{\"findings\":[]}\n```", `{"findings":[]}`},
		{"whitespace trimmed", "   {\"findings\":[]}   ", `{"findings":[]}`},
		{"prose left intact (no fence)", `preamble {"findings":[]}`, `preamble {"findings":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripJudgeFence(tc.in); got != tc.want {
				t.Fatalf("stripJudgeFence(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestScanBalancedObject pins the string/escape-aware balanced-brace scan: it
// stops at the FIRST brace-depth-zero close, treats braces and quotes inside a
// string literal (and backslash escapes) as opaque, and fails (0,false) on an
// unbalanced or unterminated fragment.
func TestScanBalancedObject(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantOK  bool
		wantObj string // s[start:end] when ok; ignored otherwise
	}{
		{"bare object", `{"a":1}`, true, `{"a":1}`},
		{"braces inside a string are opaque", `{"t":"has {a,b} braces"}`, true, `{"t":"has {a,b} braces"}`},
		{"escaped quote inside a string is opaque", `{"t":"say \"hi\""}`, true, `{"t":"say \"hi\""}`},
		{"nested object", `{"a":{"b":1}}`, true, `{"a":{"b":1}}`},
		{"stops at first close, ignoring trailing prose", `{"a":1} then prose`, true, `{"a":1}`},
		{"brace inside string does not prematurely close", `{"t":"a}b"}`, true, `{"t":"a}b"}`},
		{"unbalanced never closes", `{"a":1`, false, ""},
		{"unterminated string", `{"a":"oops`, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			end, ok := scanBalancedObject(tc.in, 0)
			if ok != tc.wantOK {
				t.Fatalf("scanBalancedObject(%q, 0) ok = %v, want %v", tc.in, ok, tc.wantOK)
			}
			if ok {
				if got := tc.in[0:end]; got != tc.wantObj {
					t.Fatalf("scanBalancedObject(%q, 0) object = %q, want %q", tc.in, got, tc.wantObj)
				}
			}
		})
	}
}

// TestBalancedObjects pins the top-level scan: it yields only TOP-LEVEL
// balanced regions in order, never descending into a region's nested braces
// (the strictness guard — a shape-violating outer object is never rescued by a
// clean nested sub-object), skips a stray unbalanced prose brace byte-by-byte,
// and returns nothing when there is no object.
func TestBalancedObjects(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"preamble then object", "I reviewed:\n\n{\"findings\":[]}", []string{`{"findings":[]}`}},
		{"non-JSON prose brace then object", `the set {a,b} then {"findings":[]}`, []string{`{a,b}`, `{"findings":[]}`}},
		{"two top-level objects", `{"a":1} {"b":2}`, []string{`{"a":1}`, `{"b":2}`}},
		{"nested object is not a separate candidate", `{"findings":[],"x":{"findings":[]}}`, []string{`{"findings":[],"x":{"findings":[]}}`}},
		{"trailing prose after object", "{\"a\":1}\n\ntrailing", []string{`{"a":1}`}},
		{"stray unbalanced brace then object", `{ oops {"findings":[]}`, []string{`{"findings":[]}`}},
		{"no object at all", `just prose here`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := balancedObjects(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("balancedObjects(%q) = %v (len %d), want %v (len %d)", tc.in, got, len(got), tc.want, len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("balancedObjects(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestDecodeJudgeInnerJSON exercises the shared extractor directly over the
// build-branch judgeInnerResult shape: the fast path (bare/fenced), the slow
// path (preamble/postamble/prose-brace), and — load-bearing — the negatives
// that must FAIL rather than parse to a false 0 findings (refusal, unknown
// field, and an unknown-field outer whose clean nested sub-object must NOT
// rescue it).
func TestDecodeJudgeInnerJSON(t *testing.T) {
	cases := []struct {
		name         string
		raw          string
		wantErr      bool
		wantFindings int // checked only when wantErr is false
	}{
		{"bare empty findings", `{"findings":[]}`, false, 0},
		{"bare one finding", `{"findings":[{"id":"x","text":"y","confidence":0.5}]}`, false, 1},
		{"json fence", "```json\n{\"findings\":[{\"id\":\"x\",\"text\":\"y\",\"confidence\":0.5}]}\n```", false, 1},
		{"preamble then object", "I've now reviewed all five acceptance criteria:\n\n{\"findings\":[]}", false, 0},
		{"preamble then object with finding", "Here is my analysis:\n\n{\"findings\":[{\"id\":\"x\",\"text\":\"y\",\"confidence\":0.5}]}", false, 1},
		{"trailing prose (round-2 trailing data)", "{\"findings\":[]}\n\nThat concludes my review.", false, 0},
		{"preamble and postamble", "pre\n{\"findings\":[]}\npost", false, 0},
		{"non-JSON prose brace before object", `the set {a,b} then {"findings":[]}`, false, 0},
		{"refusal — no object", `I cannot analyze this request.`, true, 0},
		{"empty result", ``, true, 0},
		{"unknown field fails closed", `{"findings":[],"verdict":"clean"}`, true, 0},
		{"unknown-field outer, clean nested object does not rescue", `{"findings":[],"x":{"findings":[]}}`, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeJudgeInnerJSON[judgeInnerResult](tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("decodeJudgeInnerJSON(%q) = %+v, want an error (a genuine failure must never parse as a false 0 findings)", tc.raw, got)
				}
				if got != nil {
					t.Fatalf("decodeJudgeInnerJSON(%q) returned non-nil %+v alongside its error", tc.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeJudgeInnerJSON(%q): unexpected error %v", tc.raw, err)
			}
			if len(got.Findings) != tc.wantFindings {
				t.Fatalf("decodeJudgeInnerJSON(%q) findings = %d, want %d", tc.raw, len(got.Findings), tc.wantFindings)
			}
		})
	}
}
