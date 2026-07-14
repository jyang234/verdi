package main

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestResolvePorts_Unset(t *testing.T) {
	got := resolvePorts(func(string) string { return "" })
	want := ports{workbench: "127.0.0.1:4173", dex: "127.0.0.1:4174", control: "127.0.0.1:4177", inspect: "127.0.0.1:4178"}
	if got != want {
		t.Fatalf("resolvePorts(unset) = %+v, want %+v", got, want)
	}
}

func TestResolvePorts_HappyOverride(t *testing.T) {
	got := resolvePorts(stubGetenv(portBaseEnvVar, "4300"))
	want := ports{workbench: "127.0.0.1:4300", dex: "127.0.0.1:4301", control: "127.0.0.1:4302", inspect: "127.0.0.1:4303"}
	if got != want {
		t.Fatalf("resolvePorts(4300) = %+v, want %+v", got, want)
	}
}

func TestResolvePorts_GarbageFailsClosedToDefaults(t *testing.T) {
	defaults := ports{workbench: "127.0.0.1:4173", dex: "127.0.0.1:4174", control: "127.0.0.1:4177", inspect: "127.0.0.1:4178"}

	cases := []struct {
		name string
		raw  string
	}{
		{"non-numeric", "banana"},
		{"zero", "0"},
		{"negative", "-1"},
		{"float", "4300.5"},
		{"too-large", "70000"},
		{"whitespace", " 4300"},
		{"trailing-garbage", "4300x"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			restore := captureLog(&buf)
			defer restore()

			got := resolvePorts(stubGetenv(portBaseEnvVar, tc.raw))
			if got != defaults {
				t.Fatalf("resolvePorts(%q) = %+v, want defaults %+v", tc.raw, got, defaults)
			}
			if !strings.Contains(buf.String(), portBaseEnvVar) {
				t.Fatalf("resolvePorts(%q) printed no notice; log output: %q", tc.raw, buf.String())
			}
			if !strings.Contains(buf.String(), "falling back to default ports") {
				t.Fatalf("resolvePorts(%q) notice missing fallback wording; log output: %q", tc.raw, buf.String())
			}
		})
	}
}

func TestResolvePorts_OtherEnvVarsIgnored(t *testing.T) {
	got := resolvePorts(stubGetenv("SOME_OTHER_VAR", "4300"))
	want := ports{workbench: "127.0.0.1:4173", dex: "127.0.0.1:4174", control: "127.0.0.1:4177", inspect: "127.0.0.1:4178"}
	if got != want {
		t.Fatalf("resolvePorts(irrelevant env) = %+v, want %+v", got, want)
	}
}

// stubGetenv returns a getenv func that answers key with value and every
// other lookup with "".
func stubGetenv(key, value string) func(string) string {
	return func(k string) string {
		if k == key {
			return value
		}
		return ""
	}
}

// captureLog redirects the standard logger's output into buf for the
// duration of a test, returning a restore func.
func captureLog(buf *bytes.Buffer) func() {
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	return func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	}
}
