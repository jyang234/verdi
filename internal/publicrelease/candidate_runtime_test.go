package publicrelease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidateRuntimeRequiresMeasuredHashesForEveryRoot(t *testing.T) {
	want := strings.Repeat("a", 64)
	for _, jsonStream := range []bool{true, false} {
		t.Run(fmt.Sprint(jsonStream), func(t *testing.T) {
			encode := func(test, output string) string {
				if !jsonStream {
					return "=== RUN   " + test + "\n" + output
				}
				b, e := json.Marshal(testEvent{Action: "output", Package: "github.com/jyang234/verdi-atc/cmd/vatc", Test: test, Output: output})
				if e != nil {
					t.Fatal(e)
				}
				return string(b) + "\n"
			}
			event := func(name, hash string) string {
				return encode(name, "    helper.go:1: built vatc binary /private/temporary/vatc sha256:"+hash+"\n")
			}
			good := ""
			for _, name := range candidateRuntimeRoots {
				good += event(name, want)
			}
			for _, tc := range []struct {
				name, input string
				bad         bool
			}{
				{"exact actual helper format", good, false},
				{"existing boundary summary supplements helper", good + encode(candidateRuntimeRoots[0], "    boundary_e2e_test.go:169: boundary: built vatc binary sha256:"+want+"\n"), false},
				{"summary alone is not measured helper", encode(candidateRuntimeRoots[0], "    boundary_e2e_test.go:169: boundary: built vatc binary sha256:"+want+"\n"), true},
				{"different measured executable", strings.Replace(good, want, strings.Repeat("b", 64), 1), true},
				{"missing root", event(candidateRuntimeRoots[0], want), true},
				{"duplicate root", good + event(candidateRuntimeRoots[0], want), true},
				{"claimed success without observed build", encode(candidateRuntimeRoots[0], "PASS\n"), true},
				{"instrumentation digest cannot substitute", encode(candidateRuntimeRoots[0], "instrumented binary sha256:"+want+"\n"), true},
				{"unrelated root", strings.Replace(good, candidateRuntimeRoots[0], "TestUnrelated", 1), true},
				{"malformed measured hash", strings.Replace(good, want, "invalid", 1), true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					rows, err := candidateBuildEvents(strings.NewReader(tc.input), "actual-execution", want, candidateRuntimeRoots, jsonStream)
					if (err != nil) != tc.bad {
						t.Fatalf("observed rows %+v: %v", rows, err)
					}
					if !tc.bad && len(rows) != len(candidateRuntimeRoots) {
						t.Fatal("missing observed rows")
					}
				})
			}
			if jsonStream {
				if _, err := candidateBuildEvents(strings.NewReader(good+"}"), "run", want, candidateRuntimeRoots, true); err == nil {
					t.Fatal("malformed stream accepted")
				}
			}
		})
	}
}

func TestCandidateRuntimeJoinsAllRequiredExecutions(t *testing.T) {
	want := strings.Repeat("a", 64)
	executions := []execution{}
	for _, name := range []string{"A-cmd-vatc", "atc-race", "boundary-check"} {
		var b strings.Builder
		required := candidateRuntimeRoots
		if name == "boundary-check" {
			required = required[:4]
		}
		for _, root := range required {
			line := "    helper.go:1: built vatc binary /tmp/vatc sha256:" + want + "\n"
			if name == "boundary-check" {
				fmt.Fprintf(&b, "=== RUN   %s\n%s", root, line)
			} else {
				data, err := json.Marshal(testEvent{Action: "output", Package: "github.com/jyang234/verdi-atc/cmd/vatc", Test: root, Output: line})
				if err != nil {
					t.Fatal(err)
				}
				b.Write(data)
				b.WriteByte('\n')
			}
		}
		path := filepath.Join(t.TempDir(), "output.log")
		if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
			t.Fatal(err)
		}
		executions = append(executions, execution{Name: name, StdoutPath: path})
	}
	for _, tc := range []struct {
		name   string
		change func([]execution) []execution
		bad    bool
	}{
		{"complete", func(e []execution) []execution { return e }, false},
		{"missing execution", func(e []execution) []execution { return e[1:] }, true},
		{"duplicate execution", func(e []execution) []execution { return append(e, e[0]) }, true},
		{"failed execution", func(e []execution) []execution { e[0].Exit = 1; return e }, true},
		{"missing output", func(e []execution) []execution { e[0].StdoutPath += "missing"; return e }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := observeCandidateRuntime(tc.change(append([]execution{}, executions...)), want)
			if (err != nil) != tc.bad {
				t.Fatalf("join %+v: %v", rows, err)
			}
			if !tc.bad && len(rows) != 14 {
				t.Fatal("missing joined observations")
			}
		})
	}
}
