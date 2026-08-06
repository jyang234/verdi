package execworkspace

import (
	"strings"
	"testing"
)

func TestDecodeGrantSet_AllSixKindsHappy(t *testing.T) {
	cases := map[string]struct {
		body string
		want Grant
	}{
		"network": {
			body: `{"grants":[{"kind":"network"}],"schema":"verdi.execution-grants/v1"}` + "\n",
			want: Grant{Kind: GrantNetwork},
		},
		"path-read": {
			body: `{"grants":[{"kind":"path-read","paths":["a","b"]}],"schema":"verdi.execution-grants/v1"}` + "\n",
			want: Grant{Kind: GrantPathRead, Paths: []string{"a", "b"}},
		},
		"path-write": {
			body: `{"grants":[{"kind":"path-write","paths":["c"]}],"schema":"verdi.execution-grants/v1"}` + "\n",
			want: Grant{Kind: GrantPathWrite, Paths: []string{"c"}},
		},
		"process-execution": {
			body: `{"grants":[{"argv0s":["go","git"],"kind":"process-execution"}],"schema":"verdi.execution-grants/v1"}` + "\n",
			want: Grant{Kind: GrantProcessExecution, Argv0s: []string{"go", "git"}},
		},
		"resource-ceilings": {
			body: `{"grants":[{"ceilings":{"cpu":2,"mem":1024},"kind":"resource-ceilings"}],"schema":"verdi.execution-grants/v1"}` + "\n",
			want: Grant{Kind: GrantResourceCeilings, Ceilings: map[string]int{"cpu": 2, "mem": 1024}},
		},
		"timeouts": {
			body: `{"grants":[{"kind":"timeouts","seconds":30}],"schema":"verdi.execution-grants/v1"}` + "\n",
			want: Grant{Kind: GrantTimeouts, Seconds: 30},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := DecodeGrantSet([]byte(tc.body))
			if err != nil {
				t.Fatalf("DecodeGrantSet(%s): unexpected error: %v", name, err)
			}
			if len(got.Grants) != 1 {
				t.Fatalf("DecodeGrantSet(%s): got %d grants, want 1", name, len(got.Grants))
			}
			g := got.Grants[0]
			if g.Kind != tc.want.Kind {
				t.Fatalf("Kind = %v, want %v", g.Kind, tc.want.Kind)
			}
			if !stringSliceEqual(g.Paths, tc.want.Paths) {
				t.Fatalf("Paths = %v, want %v", g.Paths, tc.want.Paths)
			}
			if !stringSliceEqual(g.Argv0s, tc.want.Argv0s) {
				t.Fatalf("Argv0s = %v, want %v", g.Argv0s, tc.want.Argv0s)
			}
			if len(g.Ceilings) != len(tc.want.Ceilings) {
				t.Fatalf("Ceilings = %v, want %v", g.Ceilings, tc.want.Ceilings)
			}
			for k, v := range tc.want.Ceilings {
				if g.Ceilings[k] != v {
					t.Fatalf("Ceilings[%q] = %d, want %d", k, g.Ceilings[k], v)
				}
			}
			if g.Seconds != tc.want.Seconds {
				t.Fatalf("Seconds = %d, want %d", g.Seconds, tc.want.Seconds)
			}
		})
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDecodeGrantSet_EmptyGrantsListDecodesToValidEmptySet(t *testing.T) {
	body := `{"grants":[],"schema":"verdi.execution-grants/v1"}` + "\n"
	got, err := DecodeGrantSet([]byte(body))
	if err != nil {
		t.Fatalf("DecodeGrantSet: unexpected error: %v", err)
	}
	if len(got.Grants) != 0 {
		t.Fatalf("Grants = %v, want empty", got.Grants)
	}
}

func TestDecodeGrantSet_RejectsUnknownKind(t *testing.T) {
	body := `{"grants":[{"kind":"filesystem"}],"schema":"verdi.execution-grants/v1"}` + "\n"
	if _, err := DecodeGrantSet([]byte(body)); err == nil {
		t.Fatalf("DecodeGrantSet: want error for unknown kind, got nil")
	} else if !strings.Contains(err.Error(), "filesystem") {
		t.Fatalf("error = %v, want it to name the unknown kind", err)
	}
}

func TestDecodeGrantSet_RejectsDuplicateKind(t *testing.T) {
	body := `{"grants":[{"kind":"network"},{"kind":"network"}],"schema":"verdi.execution-grants/v1"}` + "\n"
	if _, err := DecodeGrantSet([]byte(body)); err == nil {
		t.Fatalf("DecodeGrantSet: want error for duplicate kind, got nil")
	} else if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error = %v, want it to mention duplicate", err)
	}
}

func TestDecodeGrantSet_RejectsUnknownField(t *testing.T) {
	cases := map[string]string{
		"top-level unknown field": `{"extra":true,"grants":[],"schema":"verdi.execution-grants/v1"}` + "\n",
		"per-grant unknown field": `{"grants":[{"kind":"network","bogus":1}],"schema":"verdi.execution-grants/v1"}` + "\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeGrantSet([]byte(body)); err == nil {
				t.Fatalf("DecodeGrantSet(%s): want error, got nil", name)
			}
		})
	}
}

func TestDecodeGrantSet_RejectsWrongSchema(t *testing.T) {
	body := `{"grants":[],"schema":"verdi.execution-grants/v0"}` + "\n"
	if _, err := DecodeGrantSet([]byte(body)); err == nil {
		t.Fatalf("DecodeGrantSet: want error for wrong schema, got nil")
	} else if !strings.Contains(err.Error(), "schema") {
		t.Fatalf("error = %v, want it to mention schema", err)
	}
}

func TestDecodeGrantSet_RejectsMissingSchema(t *testing.T) {
	body := `{"grants":[]}` + "\n"
	if _, err := DecodeGrantSet([]byte(body)); err == nil {
		t.Fatalf("DecodeGrantSet: want error for missing schema, got nil")
	}
}

func TestDecodeGrantSet_RejectsNonCanonicalBytes(t *testing.T) {
	cases := map[string]string{
		"unsorted top-level keys": `{"schema":"verdi.execution-grants/v1","grants":[]}` + "\n",
		"interior whitespace":     `{"grants": [], "schema": "verdi.execution-grants/v1"}` + "\n",
		"no trailing newline":     `{"grants":[],"schema":"verdi.execution-grants/v1"}`,
		"extra trailing newline":  `{"grants":[],"schema":"verdi.execution-grants/v1"}` + "\n\n",
		"unsorted grant keys":     `{"grants":[{"paths":["a"],"kind":"path-read"}],"schema":"verdi.execution-grants/v1"}` + "\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeGrantSet([]byte(body)); err == nil {
				t.Fatalf("DecodeGrantSet(%s): want error for non-canonical bytes, got nil", name)
			}
		})
	}
}

func TestDecodeGrantSet_RejectsGarbage(t *testing.T) {
	if _, err := DecodeGrantSet([]byte("not json")); err == nil {
		t.Fatalf("DecodeGrantSet: want error for garbage input, got nil")
	}
}

func TestDecodeGrantSet_RejectsInvalidPayloads(t *testing.T) {
	cases := map[string]string{
		"empty paths list":         `{"grants":[{"kind":"path-read","paths":[]}],"schema":"verdi.execution-grants/v1"}` + "\n",
		"empty path entry":         `{"grants":[{"kind":"path-read","paths":[""]}],"schema":"verdi.execution-grants/v1"}` + "\n",
		"empty argv0s list":        `{"grants":[{"argv0s":[],"kind":"process-execution"}],"schema":"verdi.execution-grants/v1"}` + "\n",
		"empty argv0 entry":        `{"grants":[{"argv0s":[""],"kind":"process-execution"}],"schema":"verdi.execution-grants/v1"}` + "\n",
		"empty ceilings":           `{"grants":[{"ceilings":{},"kind":"resource-ceilings"}],"schema":"verdi.execution-grants/v1"}` + "\n",
		"zero seconds":             `{"grants":[{"kind":"timeouts","seconds":0}],"schema":"verdi.execution-grants/v1"}` + "\n",
		"negative seconds":         `{"grants":[{"kind":"timeouts","seconds":-5}],"schema":"verdi.execution-grants/v1"}` + "\n",
		"network with extra field": `{"grants":[{"argv0s":["go"],"kind":"network"}],"schema":"verdi.execution-grants/v1"}` + "\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeGrantSet([]byte(body)); err == nil {
				t.Fatalf("DecodeGrantSet(%s): want error, got nil", name)
			}
		})
	}
}

func TestEncodeGrantSet_RoundTrip(t *testing.T) {
	set := GrantSet{Grants: []Grant{
		{Kind: GrantNetwork},
		{Kind: GrantPathRead, Paths: []string{"src", "docs"}},
		{Kind: GrantTimeouts, Seconds: 45},
	}}
	data, err := EncodeGrantSet(set)
	if err != nil {
		t.Fatalf("EncodeGrantSet: %v", err)
	}
	got, err := DecodeGrantSet(data)
	if err != nil {
		t.Fatalf("DecodeGrantSet: %v", err)
	}
	if len(got.Grants) != len(set.Grants) {
		t.Fatalf("round trip grant count = %d, want %d", len(got.Grants), len(set.Grants))
	}
}

func TestEncodeGrantSet_Deterministic(t *testing.T) {
	set := GrantSet{Grants: []Grant{{Kind: GrantResourceCeilings, Ceilings: map[string]int{"cpu": 1, "mem": 2, "disk": 3}}}}
	a, err := EncodeGrantSet(set)
	if err != nil {
		t.Fatalf("EncodeGrantSet: %v", err)
	}
	b, err := EncodeGrantSet(set)
	if err != nil {
		t.Fatalf("EncodeGrantSet: %v", err)
	}
	if string(a) != string(b) {
		t.Fatalf("EncodeGrantSet not deterministic: %q vs %q", a, b)
	}
}

func TestEncodeGrantSet_EmptySet(t *testing.T) {
	data, err := EncodeGrantSet(GrantSet{})
	if err != nil {
		t.Fatalf("EncodeGrantSet: %v", err)
	}
	want := `{"grants":[],"schema":"verdi.execution-grants/v1"}` + "\n"
	if string(data) != want {
		t.Fatalf("EncodeGrantSet(empty) = %q, want %q", data, want)
	}
}

func TestEncodeGrantSet_RejectsUnknownEnumValue(t *testing.T) {
	set := GrantSet{Grants: []Grant{{Kind: GrantKind(99)}}}
	if data, err := EncodeGrantSet(set); err == nil {
		t.Fatalf("EncodeGrantSet(unknown kind) = %q, want error", data)
	}
}

func TestEncodeGrantSet_RejectsInvalidGrant(t *testing.T) {
	cases := map[string]GrantSet{
		"empty paths":        {Grants: []Grant{{Kind: GrantPathRead}}},
		"zero seconds":       {Grants: []Grant{{Kind: GrantTimeouts}}},
		"duplicate kind":     {Grants: []Grant{{Kind: GrantNetwork}, {Kind: GrantNetwork}}},
		"extra field":        {Grants: []Grant{{Kind: GrantNetwork, Paths: []string{"x"}}}},
		"empty ceiling name": {Grants: []Grant{{Kind: GrantResourceCeilings, Ceilings: map[string]int{"": 1}}}},
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			if data, err := EncodeGrantSet(set); err == nil {
				t.Fatalf("EncodeGrantSet(%s) = %q, want error", name, data)
			}
		})
	}
}

func TestGrantKind_StringUnknownFailsClosed(t *testing.T) {
	got := GrantKind(99).String()
	if !strings.Contains(got, "unknown") {
		t.Fatalf("GrantKind(99).String() = %q, want it to self-identify as unknown", got)
	}
}

func TestGrantSet_Get(t *testing.T) {
	set := GrantSet{Grants: []Grant{{Kind: GrantTimeouts, Seconds: 10}}}
	g, ok := set.Get(GrantTimeouts)
	if !ok || g.Seconds != 10 {
		t.Fatalf("Get(GrantTimeouts) = %+v, %v, want Seconds=10, true", g, ok)
	}
	if _, ok := set.Get(GrantNetwork); ok {
		t.Fatalf("Get(GrantNetwork) = true, want false (not present)")
	}
}
