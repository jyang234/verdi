package publicrelease

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActualProcessExitIsNeverHiddenByPrintedPass(t *testing.T) {
	x := processExecutor{root: t.TempDir()}
	out, err := x.Run(context.Background(), command{Name: "failure", Dir: t.TempDir(), Args: []string{"sh", "-c", "printf PASS; exit 7"}})
	if err != nil || out.Exit != 7 || out.StdoutSHA != digest([]byte("PASS")) {
		t.Fatalf("execution %+v %v", out, err)
	}
	if _, err = x.Run(context.Background(), command{Name: "unavailable", Args: []string{filepath.Join(t.TempDir(), "absent")}}); err == nil {
		t.Fatal("missing command accepted")
	}
}
func TestChildEnvironmentNeverCarriesProtectedURL(t *testing.T) {
	t.Setenv("PUBLIC_RELEASE_ATC_BUNDLE_URL", "secret")
	for _, item := range childEnvironment() {
		if strings.HasPrefix(item, "PUBLIC_RELEASE_ATC_BUNDLE_URL=") {
			t.Fatal("secret inherited")
		}
	}
	got := overlayEnv([]string{"A=old", "B=kept"}, []string{"A=new"})
	if strings.Join(got, ",") != "A=new,B=kept" {
		t.Fatal(got)
	}
}
func TestRaceEventsDiscloseOnlyHistoricalSkips(t *testing.T) {
	stream := `{"Action":"start","Package":"github.com/jyang234/verdi-atc/internal/verdiproto"}
{"Action":"run","Package":"github.com/jyang234/verdi-atc/internal/verdiproto","Test":"TestIntegrationStateMatrixJourneyParity"}
{"Action":"output","Package":"github.com/jyang234/verdi-atc/internal/verdiproto","Test":"TestIntegrationStateMatrixJourneyParity","Output":"requires historical fixed pin\n"}
{"Action":"skip","Package":"github.com/jyang234/verdi-atc/internal/verdiproto","Test":"TestIntegrationStateMatrixJourneyParity"}
{"Action":"pass","Package":"github.com/jyang234/verdi-atc/internal/verdiproto"}
`
	skips, err := raceEvents(strings.NewReader(stream), true)
	if err != nil || len(skips) != 1 || skips[0].Reason == "" {
		t.Fatalf("skips %+v %v", skips, err)
	}
	if _, err = raceEvents(strings.NewReader(stream), false); err == nil {
		t.Fatal("unexpected Verdi skip accepted")
	}
	if _, err = raceEvents(strings.NewReader(strings.ReplaceAll(stream, "TestIntegrationStateMatrixJourneyParity", "TestCurrentPair")), true); err == nil {
		t.Fatal("current pair skip accepted")
	}
}
func TestFailureLeavesNoPublishedPasses(t *testing.T) {
	root := filepath.Join(t.TempDir(), "run")
	err := Run(context.Background(), Config{OutputDir: root})
	if err == nil {
		t.Fatal("missing source accepted")
	}
	if _, err = os.Stat(filepath.Join(root, "publish")); !os.IsNotExist(err) {
		t.Fatal("published incomplete result")
	}
}
