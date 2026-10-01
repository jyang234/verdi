package ritualwitness

import (
	"fmt"
	"os"
	"testing"
)

// TestMain isolates the whole test binary from ambient git configuration
// (IsolateGitConfig), so tests using Build may run in parallel.
func TestMain(m *testing.M) {
	restore, err := IsolateGitConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	code := m.Run()
	restore()
	os.Exit(code)
}
