package publicrelease

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Bootstrap builds isolated exact sources from the approved private bundle.
// The secret URL is used once and is never included in errors or artifacts.
func Bootstrap(ctx context.Context, verdiRepo, root string, getenv func(string) string) (Config, error) {
	c := Config{VerdiCommit: getenv("PUBLIC_RELEASE_VERDI_COMMIT"), ATCCommit: getenv("PUBLIC_RELEASE_ATC_COMMIT"), WorkflowRun: true, BundleSHA: getenv("PUBLIC_RELEASE_ATC_BUNDLE_SHA256")}
	if err := bootstrapInputs(c, getenv); err != nil {
		return c, err
	}
	if !filepath.IsAbs(root) || !filepath.IsAbs(verdiRepo) {
		return c, fmt.Errorf("absolute bootstrap paths required")
	}
	if err := os.Mkdir(root, 0700); err != nil {
		return c, fmt.Errorf("fresh bootstrap directory: %w", err)
	}
	private := filepath.Join(root, "bootstrap-private")
	if err := os.Mkdir(private, 0700); err != nil {
		return c, err
	}
	bundle := filepath.Join(private, "atc.bundle")
	if err := downloadBundle(ctx, getenv("PUBLIC_RELEASE_ATC_BUNDLE_URL"), bundle, getenv("PUBLIC_RELEASE_ATC_BUNDLE_SHA256"), http.DefaultClient); err != nil {
		return c, err
	}
	if err := os.Unsetenv("PUBLIC_RELEASE_ATC_BUNDLE_URL"); err != nil {
		return c, fmt.Errorf("cannot clear protected download input")
	}
	x := processExecutor{root: private}
	c.VerdiDir = filepath.Join(root, "verdi")
	c.ATCDir = filepath.Join(root, "atc")
	for _, row := range []struct{ name, from, to, commit string }{{"verdi", verdiRepo, c.VerdiDir, c.VerdiCommit}, {"atc", bundle, c.ATCDir, c.ATCCommit}} {
		for _, step := range []command{{Dir: root, Name: "clone-" + row.name, Args: []string{"git", "clone", "--no-hardlinks", "--no-checkout", row.from, row.to}}, {Dir: row.to, Name: "checkout-" + row.name, Args: []string{"git", "checkout", "--detach", row.commit}}} {
			out, err := x.Run(ctx, step)
			if err != nil {
				return c, err
			}
			if out.Exit != 0 {
				return c, fmt.Errorf("exact source bootstrap failed: %s", step.Name)
			}
		}
	}
	mainRef, err := commandBytes(ctx, verdiRepo, "git", "rev-parse", "refs/remotes/origin/main")
	if err != nil {
		return c, fmt.Errorf("source origin/main history is required for ordinary downgrade gate")
	}
	ref := strings.TrimSpace(string(mainRef))
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(ref) {
		return c, fmt.Errorf("invalid source main ref")
	}
	fetched, e := x.Run(ctx, command{Dir: c.VerdiDir, Name: "preserve-main-history", Args: []string{"git", "fetch", "--no-tags", verdiRepo, ref + ":refs/remotes/origin/main"}})
	if e != nil {
		return c, e
	}
	if fetched.Exit != 0 {
		return c, fmt.Errorf("cannot preserve ordinary main history")
	}
	verify, err := x.Run(ctx, command{Dir: c.ATCDir, Name: "verify-bundle", Args: []string{"git", "bundle", "verify", bundle}})
	if err != nil {
		return c, err
	}
	if verify.Exit != 0 {
		return c, fmt.Errorf("private bundle verification failed")
	}
	if _, err = source(ctx, c.VerdiDir, c.VerdiCommit); err != nil {
		return c, err
	}
	if _, err = source(ctx, c.ATCDir, c.ATCCommit); err != nil {
		return c, err
	}
	c.BaselineSource = filepath.Join(root, "atc-baseline")
	verdiBaseline := filepath.Join(root, "verdi-baseline")
	for _, row := range []struct{ name, repo, commit, dest string }{{"verdi", c.VerdiDir, baselineVerdi, verdiBaseline}, {"atc", c.ATCDir, baselineATC, c.BaselineSource}} {
		archive := filepath.Join(private, row.name+"-baseline.tar")
		out, e := x.Run(ctx, command{Dir: row.repo, Name: "archive-" + row.name, Args: []string{"git", "archive", "--format=tar", "--output", archive, row.commit}})
		if e != nil {
			return c, e
		}
		if out.Exit != 0 {
			return c, fmt.Errorf("fixed baseline commit unavailable")
		}
		if e = extractArchive(archive, row.dest); e != nil {
			return c, e
		}
	}
	// Both baseline modules are prepared before authentication's offline rebuild.
	for i, dir := range []string{c.VerdiDir, c.ATCDir, verdiBaseline, c.BaselineSource} {
		out, e := x.Run(ctx, command{Dir: dir, Name: fmt.Sprintf("dependencies-%d", i), Args: []string{"go", "mod", "download"}, Env: []string{"GOWORK=off", "GOFLAGS="}})
		if e != nil {
			return c, e
		}
		if out.Exit != 0 {
			return c, fmt.Errorf("dependency preparation failed")
		}
	}
	c.VerdiBinary = filepath.Join(root, "verdi-bin")
	c.ATCBinary = filepath.Join(root, "vatc-bin")
	c.BaselineVerdi = filepath.Join(root, "verdi-baseline-bin")
	c.BaselineATC = filepath.Join(root, "vatc-baseline-bin")
	c.OutputDir = filepath.Join(root, "run")
	for _, row := range []struct {
		name, dir, path, pkg string
		baseline             bool
	}{{"verdi", c.VerdiDir, c.VerdiBinary, "./cmd/verdi", false}, {"atc", c.ATCDir, c.ATCBinary, "./cmd/vatc", false}, {"verdi-baseline", verdiBaseline, c.BaselineVerdi, "./cmd/verdi", true}, {"atc-baseline", c.BaselineSource, c.BaselineATC, "./cmd/vatc", true}} {
		env := []string{"GOWORK=off", "GOFLAGS=", "GOPROXY=off"}
		if row.baseline {
			env = baselineEnvironment()
		}
		cmd := candidateBuild(row.name, row.dir, row.path, row.pkg, env)
		cmd.CleanEnvironment = row.baseline
		out, e := x.Run(ctx, cmd)
		if e != nil {
			return c, e
		}
		if out.Exit != 0 {
			return c, fmt.Errorf("exact source build failed: %s", row.name)
		}
	}
	return c, nil
}
func bootstrapInputs(c Config, getenv func(string) string) error {
	commit := regexp.MustCompile(`^[0-9a-f]{40}$`)
	sha := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !commit.MatchString(c.VerdiCommit) || !commit.MatchString(c.ATCCommit) || getenv("PUBLIC_RELEASE_BASELINE_ATC_COMMIT") != baselineATC || !sha.MatchString(getenv("PUBLIC_RELEASE_ATC_BUNDLE_SHA256")) {
		return fmt.Errorf("missing or invalid approved source identities")
	}
	u, err := url.Parse(getenv("PUBLIC_RELEASE_ATC_BUNDLE_URL"))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("protected HTTPS source URL required")
	}
	return nil
}
func downloadBundle(ctx context.Context, secret, path, want string, client *http.Client) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, secret, nil)
	if err != nil {
		return fmt.Errorf("invalid private source request")
	}
	// No redirect can leak the protected query/token to another authority.
	safe := *client
	safe.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := safe.Do(req)
	if err != nil {
		return fmt.Errorf("private source download failed")
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("private source download refused with HTTP %d", res.StatusCode)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, res.Body)
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("private source body read failed")
	}
	if closeErr != nil {
		return closeErr
	}
	got, err := fileDigest(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("private bundle SHA-256 mismatch")
	}
	return nil
}
func extractArchive(path, dest string) error {
	if err := os.Mkdir(dest, 0700); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	r := tar.NewReader(f)
	for {
		h, e := r.Next()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		name := filepath.Clean(h.Name)
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("archive path escapes source")
		}
		target := filepath.Join(dest, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if e = os.MkdirAll(target, os.FileMode(h.Mode)&0777); e != nil {
				return e
			}
		case tar.TypeReg:
			if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
				return e
			}
			out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(h.Mode)&0777)
			if e != nil {
				return e
			}
			_, copyErr := io.Copy(out, r)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeXGlobalHeader:
			continue
		default:
			return fmt.Errorf("unsupported source archive entry type")
		}
	}
}
func baselineEnvironment() []string {
	return []string{"CGO_ENABLED=0", "GOFLAGS=", "GOENV=off", "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOEXPERIMENT=", "GOOS=" + runtime.GOOS, "GOARCH=" + runtime.GOARCH, "GOAMD64=v1", "GOARM=7", "GOARM64=v8.0", "GO386=sse2", "GODEBUG="}
}
