package publicrelease

import (
	"archive/tar"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapMissingApprovedInputsRefusesBeforeDownload(t *testing.T) {
	c := Config{VerdiCommit: strings.Repeat("a", 40), ATCCommit: strings.Repeat("b", 40)}
	env := map[string]string{"PUBLIC_RELEASE_BASELINE_ATC_COMMIT": baselineATC, "PUBLIC_RELEASE_ATC_BUNDLE_SHA256": strings.Repeat("c", 64), "PUBLIC_RELEASE_ATC_BUNDLE_URL": "https://example.invalid/private?secret=never-print"}
	get := func(k string) string { return env[k] }
	if err := bootstrapInputs(c, get); err != nil {
		t.Fatal(err)
	}
	for key := range env {
		t.Run(key, func(t *testing.T) {
			old := env[key]
			env[key] = ""
			defer func() { env[key] = old }()
			err := bootstrapInputs(c, get)
			if err == nil || strings.Contains(err.Error(), "never-print") {
				t.Fatalf("invalid refusal: %v", err)
			}
		})
	}
	for _, bad := range []string{"http://example.invalid/x", "https://user:password@example.invalid/x", ""} {
		env["PUBLIC_RELEASE_ATC_BUNDLE_URL"] = bad
		if err := bootstrapInputs(c, get); err == nil {
			t.Fatal("unsafe source URL accepted")
		}
	}
}
func TestPrivateBundleDownloadOnceAndRedactsErrors(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; _, _ = w.Write([]byte("bundle")) }))
	defer server.Close()
	for _, tc := range []struct {
		name, want string
		bad        bool
	}{{"approved", digest([]byte("bundle")), false}, {"wrong hash", strings.Repeat("0", 64), true}} {
		t.Run(tc.name, func(t *testing.T) {
			before := calls
			err := downloadBundle(context.Background(), server.URL+"/?token=never-print", filepath.Join(t.TempDir(), "bundle"), tc.want, server.Client())
			if (err != nil) != tc.bad || calls != before+1 {
				t.Fatalf("download count/error %d %v", calls, err)
			}
			if err != nil && strings.Contains(err.Error(), "never-print") {
				t.Fatal("secret leaked")
			}
		})
	}
}
func TestExtractArchiveRejectsTraversalAndLinks(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind byte
		bad  bool
	}{{"source.go", tar.TypeReg, false}, {"../escape", tar.TypeReg, true}, {"link", tar.TypeSymlink, true}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			archive := filepath.Join(root, "source.tar")
			f, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			w := tar.NewWriter(f)
			if err = w.WriteHeader(&tar.Header{Name: tc.name, Mode: 0600, Typeflag: tc.kind}); err != nil {
				t.Fatal(err)
			}
			if err = w.Close(); err != nil {
				t.Fatal(err)
			}
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}
			err = extractArchive(archive, filepath.Join(root, "source"))
			if (err != nil) != tc.bad {
				t.Fatalf("extract: %v", err)
			}
		})
	}
}
