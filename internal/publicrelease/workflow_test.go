package publicrelease

import (
	"os"
	"strings"
	"testing"
)

func TestManualWorkflowPreservesPrivateSourceAndClosedJob(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/public-execution-contract-release.yml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, required := range []string{"workflow_dispatch:", "  " + Job + ":", "environment: " + Job, "secrets.ATC_SOURCE_BUNDLE_URL", "fetch-depth: 0", "ref: ${{ inputs.verdi_commit }}", "--bootstrap", "go-version: '1.25'", "node-version: '22'", "golangci-lint@v2.5.0", "publish/verdicts.json"} {
		if !strings.Contains(s, required) {
			t.Fatalf("missing %q", required)
		}
	}
	for _, forbidden := range []string{"  push:", "  pull_request:", "publish/*", "/private/", "/bootstrap-private/", "run: ${{ inputs.", "run: ${{ secrets."} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("unexpected workflow scope/upload %q", forbidden)
		}
	}
	if strings.Count(s, "/run/publish/") != 12 {
		t.Fatal("upload must enumerate exactly twelve typed reports")
	}
}
