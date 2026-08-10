package execworkspace

import (
	"math"
	"strings"
	"testing"
	"time"
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

// probeSecondsOverflowingDuration and probeSecondsWrappingToZero are the two
// GrantTimeouts values whose time.Duration conversion in BuildProfile
// silently overflows: the first wraps to a NEGATIVE duration, the second
// wraps to exactly 0s — either one turns a requested deadline into no
// enforceable deadline at all while BuildProfile still reports the timeouts
// grant as Applied. Both are declared as int64 VARIABLES (never untyped
// constants) so the int() conversions below stay runtime conversions and the
// test file still compiles where int is 32 bits wide.
var (
	probeSecondsOverflowingDuration int64 = math.MaxInt64/int64(time.Second) + 1
	probeSecondsWrappingToZero      int64 = 4611686018427387904
)

func TestGrant_Validate_RejectsSecondsOverflowingDuration(t *testing.T) {
	maxSeconds := int(int64(math.MaxInt64 / int64(time.Second)))
	cases := map[string]struct {
		seconds int
		wantErr bool
	}{
		"largest non-overflowing value is still accepted": {seconds: maxSeconds, wantErr: false},
		"one past the largest value wraps negative":       {seconds: int(probeSecondsOverflowingDuration), wantErr: true},
		"large value wraps to exactly zero":               {seconds: int(probeSecondsWrappingToZero), wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := Grant{Kind: GrantTimeouts, Seconds: tc.seconds}
			err := g.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("Grant{Seconds:%d}.Validate() = nil, want error (time.Duration conversion is %v)",
					tc.seconds, time.Duration(tc.seconds)*time.Second)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Grant{Seconds:%d}.Validate() = %v, want nil", tc.seconds, err)
			}
		})
	}
}

func TestBuildProfile_RejectsSecondsOverflowingDuration(t *testing.T) {
	for name, seconds := range map[string]int{
		"wraps negative": int(probeSecondsOverflowingDuration),
		"wraps to zero":  int(probeSecondsWrappingToZero),
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			set := GrantSet{Grants: []Grant{{Kind: GrantTimeouts, Seconds: seconds}}}
			profile, report, err := BuildProfile(dir, dir, set, nil)
			if err == nil {
				t.Fatalf("BuildProfile(seconds=%d) = timeout %v, report %+v, want error",
					seconds, profile.Timeout, report)
			}
		})
	}
}

func TestDecodeGrantSet_RejectsSecondsOverflowingDuration(t *testing.T) {
	cases := map[string]string{
		"one past the largest value": `{"grants":[{"kind":"timeouts","seconds":9223372037}],"schema":"verdi.execution-grants/v1"}` + "\n",
		"wraps to exactly zero":      `{"grants":[{"kind":"timeouts","seconds":4611686018427387904}],"schema":"verdi.execution-grants/v1"}` + "\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if got, err := DecodeGrantSet([]byte(body)); err == nil {
				t.Fatalf("DecodeGrantSet(%s) = %+v, want error", name, got)
			}
		})
	}
}

// TestDecodeGrantSet_RejectsNullGrants pins that an explicit "grants":null is
// refused. It is self-canonical (the canonical-bytes gate re-encodes the
// decoded document, and a nil Grants slice marshals straight back to null),
// so only an explicit nil check refuses it — leaving the spec's one empty
// form as "grants":[].
func TestDecodeGrantSet_RejectsNullGrants(t *testing.T) {
	body := `{"grants":null,"schema":"verdi.execution-grants/v1"}` + "\n"
	if got, err := DecodeGrantSet([]byte(body)); err == nil {
		t.Fatalf("DecodeGrantSet(null grants) = %+v, want error", got)
	}
}

func TestDecodeGrantSet_RejectsAbsentGrants(t *testing.T) {
	body := `{"schema":"verdi.execution-grants/v1"}` + "\n"
	if got, err := DecodeGrantSet([]byte(body)); err == nil {
		t.Fatalf("DecodeGrantSet(absent grants) = %+v, want error", got)
	}
}

func TestGrant_Validate_RejectsNonPositiveCeilingValues(t *testing.T) {
	cases := map[string]struct {
		ceilings map[string]int
		wantErr  bool
	}{
		"positive ceiling accepted": {ceilings: map[string]int{"mem": 1024}, wantErr: false},
		"negative ceiling rejected": {ceilings: map[string]int{"mem": -1}, wantErr: true},
		"zero ceiling rejected":     {ceilings: map[string]int{"mem": 0}, wantErr: true},
		"one bad among good":        {ceilings: map[string]int{"cpu": 2, "mem": 0}, wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := Grant{Kind: GrantResourceCeilings, Ceilings: tc.ceilings}
			err := g.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("Grant{Ceilings:%v}.Validate() = nil, want error", tc.ceilings)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Grant{Ceilings:%v}.Validate() = %v, want nil", tc.ceilings, err)
			}
		})
	}
}

func TestDecodeGrantSet_RejectsNonPositiveCeilingValues(t *testing.T) {
	cases := map[string]string{
		"negative ceiling": `{"grants":[{"ceilings":{"mem":-1},"kind":"resource-ceilings"}],"schema":"verdi.execution-grants/v1"}` + "\n",
		"zero ceiling":     `{"grants":[{"ceilings":{"mem":0},"kind":"resource-ceilings"}],"schema":"verdi.execution-grants/v1"}` + "\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if got, err := DecodeGrantSet([]byte(body)); err == nil {
				t.Fatalf("DecodeGrantSet(%s) = %+v, want error", name, got)
			}
		})
	}
}
