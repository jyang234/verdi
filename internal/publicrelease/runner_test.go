package publicrelease

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func completeReport(t *testing.T) (releaseReport, declarations) {
	t.Helper()
	d, err := readDeclarations(checkBytes)
	if err != nil {
		t.Fatal(err)
	}
	r := releaseReport{}
	for n := 1; n <= 5; n++ {
		r.SourceChecks = append(r.SourceChecks, SourceCheck{AC: d.Producers[(n-1)*2].AC, Name: "executed-source-inventory"})
	}
	for _, name := range []string{"build-verdi", "build-atc", "build-checker", "verdi-verify", "verdi-race", "atc-verify", "atc-race", "boundary-check"} {
		r.Executions = append(r.Executions, execution{Name: name})
	}
	for _, g := range groups(d) {
		r.Executions = append(r.Executions, execution{Name: g.Repo + "-" + replacePackage(g.Package), Tests: &testResults{Passed: g.Required}})
	}
	return r, d
}
func replacePackage(s string) string {
	out := []byte(s)
	for i, b := range out {
		if b == '/' {
			out[i] = '-'
		}
	}
	return string(out)
}
func TestTenResultsRequireTheirOwnExecutedScopesAndFullGates(t *testing.T) {
	r, d := completeReport(t)
	got, err := produce(r, d, "report-hash")
	if err != nil || len(got) != 10 {
		t.Fatalf("ten results %d %v", len(got), err)
	}
	for _, tc := range []struct {
		name   string
		change func(*releaseReport)
	}{
		{"no executions", func(r *releaseReport) { r.Executions = nil }}, {"missing full gate", func(r *releaseReport) { r.Executions = r.Executions[1:] }}, {"nonzero full gate despite pass text", func(r *releaseReport) { r.Executions[0].Exit = 1 }}, {"missing source scope", func(r *releaseReport) { r.SourceChecks = r.SourceChecks[1:] }}, {"missing required named child", func(r *releaseReport) {
			for i := range r.Executions {
				if r.Executions[i].Tests != nil {
					r.Executions[i].Tests = &testResults{Passed: []string{"TestClaimedPass"}}
					break
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, d := completeReport(t)
			tc.change(&r)
			if _, err := produce(r, d, "report-hash"); err == nil {
				t.Fatal("accepted incomplete producer proof")
			}
		})
	}
}
func TestPublishOnlyTypedReportsAndExistingEvidenceCarrier(t *testing.T) {
	r, d := completeReport(t)
	r.Inputs.Verdi.Commit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	r.Workflow.Source = artifact.SourceLocal
	r.RuntimeArtifacts = map[string]string{"private-source.go": "hash-only"}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "private"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "private", "source.go"), []byte("PRIVATE SOURCE MUST STAY PRIVATE"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := publish(root, r, d); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(filepath.Join(root, "publish"))
	if err != nil || len(files) != 12 {
		t.Fatalf("publish inventory %d %v", len(files), err)
	}
	b, err := os.ReadFile(filepath.Join(root, "publish", "verdicts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var records []artifact.Evidence
	if err = artifact.DecodeStrictJSON(b, &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 10 {
		t.Fatal("wrong record count")
	}
	for _, r := range records {
		if err = r.Validate(); err != nil {
			t.Fatal(err)
		}
		if r.Provenance.Source != artifact.SourceLocal {
			t.Fatal("local promoted to CI")
		}
	}
}
func TestSourceChecksActualPairSmoke(t *testing.T) {
	peer := os.Getenv("PUBLIC_RELEASE_SOURCECHECK_ATC")
	if peer == "" {
		t.Skip("UNPROVEN: actual paired source smoke requires PUBLIC_RELEASE_SOURCECHECK_ATC; no paired source check ran")
	} // opt-in development smoke; it never writes producer evidence.
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := SourceChecks(context.Background(), root, peer)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 5 {
		t.Fatal("incomplete scopes")
	}
	t.Logf("actual source inventory: %d named rows", len(rows))
}
