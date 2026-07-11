package commitdesign

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// removeSecondDispositionLine drops the second "- { sticky: ..." line from
// content, modeling a hand-edit that leaves one board sticky undispositioned
// (VL-014's "board sticky ... has no dispositions[] entry" half).
func removeSecondDispositionLine(t *testing.T, content string) string {
	t.Helper()
	lines := strings.Split(content, "\n")
	count := 0
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "- { sticky:") {
			count++
			if count == 2 {
				continue
			}
		}
		out = append(out, l)
	}
	got := strings.Join(out, "\n")
	if count < 2 {
		t.Fatalf("scaffold did not contain 2 disposition lines to remove one from (found %d)", count)
	}
	return got
}

// appendDanglingDisposition inserts one extra disposition entry naming a
// sticky id that is not a real board sticky, modeling a hand-edit typo
// (VL-014's "dispositions[] names sticky ..., which is not a real sticky"
// half).
func appendDanglingDisposition(content string) string {
	const marker = "dispositions:\n"
	idx := strings.Index(content, marker)
	if idx < 0 {
		return content
	}
	insertAt := idx + len(marker)
	const danglingLine = "  - { sticky: a-01J8Z0K9ZZZZZZZZZZZZZZZZZZ, disposition: open-question }\n"
	return content[:insertAt] + danglingLine + content[insertAt:]
}

// commitAll stages every change under .verdi/ EXCEPT data/ (VL-013
// forbids ever git-tracking data/, and these tests separately keep a
// mutable board + annotation stream on disk under repo.Dir for boardio
// to read — `git add -A` would wrongly sweep those up) and commits under
// a fixed identity, mirroring internal/lint's own harness_test.go
// convention (a real `git commit`, exercised because these tests are
// proving something about a REAL git-committed tree, the way VL-014's
// own fixtures are).
func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid", "GIT_AUTHOR_DATE=1704067200 +0000",
			"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid", "GIT_COMMITTER_DATE=1704067200 +0000",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "--", ".verdi/specs")
	run("commit", "--quiet", "--no-verify", "-m", message)
}
