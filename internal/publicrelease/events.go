package publicrelease

import (
	"fmt"
	"io"
	"sort"

	"github.com/jyang234/verdi/internal/gotestjson"
)

// testEvent is the one `go test -json` event shape (internal/gotestjson);
// this package's other stream readers decode it under their own posture.
type testEvent = gotestjson.Event

// testResults contains identities, not output text: private source printed by a
// test failure must never enter the publishable report.
type testResults struct {
	Package string   `json:"package"`
	Passed  []string `json:"passed"`
}

// readTestEvents applies the release policy to one package's stream, read by
// the shared reader: the process exited 0, nothing was built or failed to
// build, the package and every test in it passed, and every required test
// is among them.
func readTestEvents(r io.Reader, exit int, pkg string, required []string) (testResults, error) {
	result := testResults{Package: pkg, Passed: []string{}}
	if exit != 0 {
		return result, fmt.Errorf("test process exited %d", exit)
	}
	read, err := gotestjson.ReadPackage(r, gotestjson.Target{ImportPath: pkg})
	if err != nil {
		return result, fmt.Errorf("test event: %w", err)
	}
	if read.BuildOutput || read.BuildFailed || read.FailedBuild != "" {
		return result, fmt.Errorf("unexpected build output package")
	}
	if read.Outcome != gotestjson.ActionPass {
		return result, fmt.Errorf("required execution %s: package %s", read.Outcome, pkg)
	}
	names := make([]string, 0, len(read.Tests))
	for name := range read.Tests {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if action := read.Tests[name]; action != gotestjson.ActionPass {
			return result, fmt.Errorf("required execution %s: %s", action, name)
		}
	}
	if len(required) == 0 {
		return result, fmt.Errorf("missing package completion or required test inventory")
	}
	for _, name := range required {
		if read.Tests[name] != gotestjson.ActionPass {
			return result, fmt.Errorf("required test did not pass: %s", name)
		}
	}
	result.Passed = append(result.Passed, names...)
	return result, nil
}
