package publicrelease

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

type testEvent struct {
	Time        string
	Action      string
	Package     string
	Test        string
	Elapsed     float64
	Output      string
	FailedBuild string
}

// testResults contains identities, not output text: private source printed by a
// test failure must never enter the publishable report.
type testResults struct {
	Package string   `json:"package"`
	Passed  []string `json:"passed"`
}

func readTestEvents(r io.Reader, exit int, pkg string, required []string) (testResults, error) {
	result := testResults{Package: pkg, Passed: []string{}}
	if exit != 0 {
		return result, fmt.Errorf("test process exited %d", exit)
	}
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	started, finished := false, false
	running := map[string]bool{}
	passed := map[string]bool{}
	for {
		var e testEvent
		if err := d.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			return result, fmt.Errorf("test event: %w", err)
		}
		if e.Package != pkg {
			return result, fmt.Errorf("unexpected package %q", e.Package)
		}
		switch e.Action {
		case "start":
			if started || finished {
				return result, fmt.Errorf("duplicate package start")
			}
			started = true
		case "run":
			if !started || finished || e.Test == "" || running[e.Test] || passed[e.Test] {
				return result, fmt.Errorf("invalid run %q", e.Test)
			}
			running[e.Test] = true
		case "pass":
			if e.Test == "" {
				if !started || finished || len(running) > 0 {
					return result, fmt.Errorf("incomplete package")
				}
				finished = true
				continue
			}
			if !running[e.Test] || finished {
				return result, fmt.Errorf("pass without running %q", e.Test)
			}
			delete(running, e.Test)
			passed[e.Test] = true
		case "fail", "skip", "build-fail":
			return result, fmt.Errorf("required execution %s: %s", e.Action, e.Test)
		case "output", "pause", "cont":
			if !started || finished {
				return result, fmt.Errorf("event outside package execution")
			}
		case "build-output":
			return result, fmt.Errorf("unexpected build output package")
		default:
			return result, fmt.Errorf("unknown test action %q", e.Action)
		}
	}
	if !finished || len(required) == 0 {
		return result, fmt.Errorf("missing package completion or required test inventory")
	}
	for _, name := range required {
		if !passed[name] {
			return result, fmt.Errorf("required test did not pass: %s", name)
		}
	}
	for name := range passed {
		result.Passed = append(result.Passed, name)
	}
	sort.Strings(result.Passed)
	return result, nil
}
