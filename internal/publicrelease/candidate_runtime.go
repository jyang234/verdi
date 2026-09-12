package publicrelease

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type candidateRuntime struct {
	Execution string `json:"execution"`
	Test      string `json:"test"`
	BinarySHA string `json:"binary_sha256"`
}

var candidateRuntimeRoots = []string{
	"TestBoundaryBuiltBinaryProducesACandidateHandoff",
	"TestBoundaryBuiltBinarySuspendsAnUnorderedRuntimeGrant",
	"TestBoundaryRecoveryFiveProcessCuts",
	"TestBoundaryRecoveryControlsAndCopiedCompletedStores",
	"TestAssemblyContractMismatchBuiltBinaryHasNoEffects",
}

// candidateRuntimeBuilds joins hashes measured by the unchanged ATC helper
// immediately after its build to the authenticated candidate. Instrumented
// executables and test drivers have their own provenance and are not matched.
func candidateRuntimeBuilds(out execution, want string, required []string, jsonStream bool) ([]candidateRuntime, error) {
	f, err := os.Open(out.StdoutPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return candidateBuildEvents(f, out.Name, want, required, jsonStream)
}

func candidateBuildEvents(r io.Reader, executionName, want string, required []string, jsonStream bool) ([]candidateRuntime, error) {
	pattern := regexp.MustCompile(`built vatc binary (.* )?sha256:([0-9a-f]{64})\s*$`)
	observed := map[string]string{}
	add := func(test, output string) error {
		if !strings.Contains(output, "built vatc binary ") {
			return nil
		}
		match := pattern.FindStringSubmatch(output)
		if len(match) != 3 || match[2] != want {
			return fmt.Errorf("observed candidate ATC executable differs from bound binary in %s/%s", executionName, test)
		}
		// Existing built-flight tests also print a pathless summary. Check
		// its hash, but only the helper's path-bearing measurement is evidence.
		if match[1] == "" {
			return nil
		}
		if observed[test] != "" {
			return fmt.Errorf("duplicate candidate ATC build in %s/%s", executionName, test)
		}
		observed[test] = match[2]
		return nil
	}
	if jsonStream {
		d := json.NewDecoder(r)
		d.DisallowUnknownFields()
		for {
			var e testEvent
			err := d.Decode(&e)
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if e.Action == "output" && e.Package == "github.com/jyang234/verdi-atc/cmd/vatc" {
				if err = add(e.Test, e.Output); err != nil {
					return nil, err
				}
			}
		}
	} else {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 4096), 4<<20)
		current := ""
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "=== RUN   ") {
				current = strings.TrimSpace(strings.TrimPrefix(line, "=== RUN   "))
			}
			if err := add(current, line); err != nil {
				return nil, err
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}
	for _, name := range required {
		if observed[name] == "" {
			return nil, fmt.Errorf("missing observed candidate ATC build in %s/%s", executionName, name)
		}
	}
	rows := make([]candidateRuntime, 0, len(observed))
	for _, name := range sortedKeys(observed) {
		rows = append(rows, candidateRuntime{executionName, name, observed[name]})
	}
	return rows, nil
}

func observeCandidateRuntime(executions []execution, want string) ([]candidateRuntime, error) {
	rows := []candidateRuntime{}
	for _, name := range []string{"A-cmd-vatc", "atc-race", "boundary-check"} {
		found := false
		for _, out := range executions {
			if out.Name != name {
				continue
			}
			if found || out.Exit != 0 {
				return nil, fmt.Errorf("invalid candidate runtime execution %s", name)
			}
			found = true
			required := candidateRuntimeRoots
			jsonStream := name != "boundary-check"
			if !jsonStream {
				required = required[:4]
			}
			observed, err := candidateRuntimeBuilds(out, want, required, jsonStream)
			if err != nil {
				return nil, err
			}
			rows = append(rows, observed...)
		}
		if !found {
			return nil, fmt.Errorf("missing candidate runtime execution %s", name)
		}
	}
	return rows, nil
}
