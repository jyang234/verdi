package execworkspace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func TestCollectFingerprint_GoldenBytes(t *testing.T) {
	dir := t.TempDir()
	profile, _, err := BuildProfile(dir, GrantSet{}, map[string]string{"FOO": "bar"})
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}

	inputs := FingerprintInputs{
		ToolVersions: map[string]string{"go": "1.25.0"},
		EnvVarNames:  []string{"FOO", "MISSING"},
		InputDigests: map[string]string{"fixture": "deadbeef"},
	}
	got, err := CollectFingerprint(profile, inputs)
	if err != nil {
		t.Fatalf("CollectFingerprint: %v", err)
	}

	want := fmt.Sprintf(
		`{"arch":%q,"env":{"FOO":"bar","MISSING":null},"input_digests":{"fixture":"deadbeef"},"os":%q,"tool_versions":{"go":"1.25.0"}}`+"\n",
		runtime.GOARCH, runtime.GOOS,
	)
	if string(got) != want {
		t.Fatalf("CollectFingerprint =\n%q\nwant\n%q", got, want)
	}

	var generic map[string]interface{}
	if err := json.Unmarshal(got, &generic); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := generic["schema"]; ok {
		t.Fatalf("output carries a schema field, want none (AD-8): %s", got)
	}
}

func TestCollectFingerprint_DeterministicUnderHostileMapOrdering(t *testing.T) {
	dir := t.TempDir()
	profile, _, err := BuildProfile(dir, GrantSet{}, map[string]string{"A": "1", "B": "2", "C": "3"})
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}

	first, err := CollectFingerprint(profile, FingerprintInputs{
		ToolVersions: map[string]string{"k1": "v1", "k2": "v2", "k3": "v3", "k4": "v4", "k5": "v5"},
		EnvVarNames:  []string{"A", "B", "C"},
		InputDigests: map[string]string{"d1": "aa", "d2": "bb", "d3": "cc", "d4": "dd", "d5": "ee"},
	})
	if err != nil {
		t.Fatalf("CollectFingerprint: %v", err)
	}
	for i := 0; i < 50; i++ {
		got, err := CollectFingerprint(profile, FingerprintInputs{
			ToolVersions: map[string]string{"k1": "v1", "k2": "v2", "k3": "v3", "k4": "v4", "k5": "v5"},
			EnvVarNames:  []string{"A", "B", "C"},
			InputDigests: map[string]string{"d1": "aa", "d2": "bb", "d3": "cc", "d4": "dd", "d5": "ee"},
		})
		if err != nil {
			t.Fatalf("CollectFingerprint: %v", err)
		}
		if !bytes.Equal(first, got) {
			t.Fatalf("CollectFingerprint not deterministic across calls (iteration %d): %q vs %q", i, first, got)
		}
	}
}

func TestCollectFingerprint_AbsentEnvNameRecordedExplicitly(t *testing.T) {
	dir := t.TempDir()
	profile, _, err := BuildProfile(dir, GrantSet{}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	got, err := CollectFingerprint(profile, FingerprintInputs{EnvVarNames: []string{"NEVER_DECLARED"}})
	if err != nil {
		t.Fatalf("CollectFingerprint: %v", err)
	}
	var generic map[string]interface{}
	if err := json.Unmarshal(got, &generic); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	env, ok := generic["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("env field is not an object: %v", generic["env"])
	}
	value, present := env["NEVER_DECLARED"]
	if !present {
		t.Fatalf("env object omits requested name entirely, want it present and explicitly null: %v", env)
	}
	if value != nil {
		t.Fatalf("env[%q] = %v, want explicit null for an absent-from-profile name", "NEVER_DECLARED", value)
	}
}

func TestCollectFingerprint_RejectsInvalidToolVersion(t *testing.T) {
	dir := t.TempDir()
	profile, _, err := BuildProfile(dir, GrantSet{}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	cases := map[string]FingerprintInputs{
		"empty name":    {ToolVersions: map[string]string{"": "1.0"}},
		"empty version": {ToolVersions: map[string]string{"go": ""}},
	}
	for name, inputs := range cases {
		t.Run(name, func(t *testing.T) {
			if data, err := CollectFingerprint(profile, inputs); err == nil {
				t.Fatalf("CollectFingerprint(%s) = %q, want error", name, data)
			}
		})
	}
}

func TestCollectFingerprint_RejectsInvalidDigest(t *testing.T) {
	dir := t.TempDir()
	profile, _, err := BuildProfile(dir, GrantSet{}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	cases := map[string]FingerprintInputs{
		"empty digest name":  {InputDigests: map[string]string{"": "deadbeef"}},
		"empty digest value": {InputDigests: map[string]string{"fixture": ""}},
		"non-hex digest":     {InputDigests: map[string]string{"fixture": "not-hex!!"}},
	}
	for name, inputs := range cases {
		t.Run(name, func(t *testing.T) {
			if data, err := CollectFingerprint(profile, inputs); err == nil {
				t.Fatalf("CollectFingerprint(%s) = %q, want error", name, data)
			}
		})
	}
}

func TestCollectFingerprint_RejectsEmptyEnvVarName(t *testing.T) {
	dir := t.TempDir()
	profile, _, err := BuildProfile(dir, GrantSet{}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	if data, err := CollectFingerprint(profile, FingerprintInputs{EnvVarNames: []string{""}}); err == nil {
		t.Fatalf("CollectFingerprint(empty env var name) = %q, want error", data)
	}
}

func TestCollectFingerprint_OSArchMatchRuntime(t *testing.T) {
	dir := t.TempDir()
	profile, _, err := BuildProfile(dir, GrantSet{}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	got, err := CollectFingerprint(profile, FingerprintInputs{})
	if err != nil {
		t.Fatalf("CollectFingerprint: %v", err)
	}
	var generic map[string]interface{}
	if err := json.Unmarshal(got, &generic); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if generic["os"] != runtime.GOOS {
		t.Fatalf("os = %v, want %q", generic["os"], runtime.GOOS)
	}
	if generic["arch"] != runtime.GOARCH {
		t.Fatalf("arch = %v, want %q", generic["arch"], runtime.GOARCH)
	}
}

func TestCollectFingerprint_EmptyInputsStillProduceObjects(t *testing.T) {
	dir := t.TempDir()
	profile, _, err := BuildProfile(dir, GrantSet{}, nil)
	if err != nil {
		t.Fatalf("BuildProfile: %v", err)
	}
	got, err := CollectFingerprint(profile, FingerprintInputs{})
	if err != nil {
		t.Fatalf("CollectFingerprint: %v", err)
	}
	for _, field := range []string{`"tool_versions":{}`, `"env":{}`, `"input_digests":{}`} {
		if !strings.Contains(string(got), field) {
			t.Fatalf("output = %q, want it to contain %q", got, field)
		}
	}
}
