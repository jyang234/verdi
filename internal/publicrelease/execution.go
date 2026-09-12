package publicrelease

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrVerdict distinguishes an executed rejecting check from an operational error.
var ErrVerdict = errors.New("release check rejected")

type command struct {
	Dir              string
	Args, Env        []string
	Name             string
	CleanEnvironment bool
}
type execution struct {
	Name        string       `json:"name"`
	Args        []string     `json:"argv"`
	Environment []string     `json:"environment"`
	Exit        int          `json:"exit"`
	StdoutSHA   string       `json:"stdout_sha256"`
	StderrSHA   string       `json:"stderr_sha256"`
	StdoutPath  string       `json:"-"`
	Tests       *testResults `json:"tests,omitempty"`
}

type executor interface {
	Run(context.Context, command) (execution, error)
}
type processExecutor struct{ root string }

func (p processExecutor) Run(ctx context.Context, c command) (execution, error) {
	out := execution{Name: c.Name, Args: c.Args, Environment: c.Env}
	dir := filepath.Join(p.root, c.Name)
	if err := os.Mkdir(dir, 0700); err != nil {
		return out, err
	}
	out.StdoutPath = filepath.Join(dir, "stdout.log")
	stderrPath := filepath.Join(dir, "stderr.log")
	stdout, err := os.OpenFile(out.StdoutPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return out, err
	}
	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		_ = stdout.Close()
		return out, err
	}
	cmd := exec.CommandContext(ctx, c.Args[0], c.Args[1:]...)
	cmd.Dir = c.Dir
	baseEnv := childEnvironment()
	if c.CleanEnvironment {
		baseEnv = baselineInheritedEnvironment()
	}
	cmd.Env = overlayEnv(baseEnv, c.Env)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	fmt.Fprintln(os.Stderr, "release: starting", c.Name)
	runErr := cmd.Run()
	closeOut, closeErr := stdout.Close(), stderr.Close()
	if closeOut != nil {
		return out, closeOut
	}
	if closeErr != nil {
		return out, closeErr
	}
	if runErr != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) {
			return out, fmt.Errorf("start %s: %w", c.Name, runErr)
		}
		out.Exit = exit.ExitCode()
	}
	if out.StdoutSHA, err = fileDigest(out.StdoutPath); err != nil {
		return out, err
	}
	if out.StderrSHA, err = fileDigest(stderrPath); err != nil {
		return out, err
	}
	fmt.Fprintln(os.Stderr, "release: completed", c.Name, "exit", out.Exit)
	return out, nil
}
func overlayEnv(base, extra []string) []string {
	values := map[string]string{}
	for _, item := range append(append([]string{}, base...), extra...) {
		key, _, ok := strings.Cut(item, "=")
		if ok {
			values[key] = item
		}
	}
	out := make([]string, 0, len(values))
	for _, key := range sortedKeys(values) {
		out = append(out, values[key])
	}
	return out
}

func runTests(ctx context.Context, x executor, c command, pkg string, names []string) (execution, error) {
	out, err := x.Run(ctx, c)
	if err != nil {
		return out, err
	}
	f, err := os.Open(out.StdoutPath)
	if err != nil {
		return out, err
	}
	defer func() { _ = f.Close() }()
	result, err := readTestEvents(f, out.Exit, pkg, names)
	if err != nil {
		return out, fmt.Errorf("%w: %s: %v", ErrVerdict, c.Name, err)
	}
	out.Tests = &result
	return out, nil
}

func childEnvironment() []string {
	out := []string{}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "PUBLIC_RELEASE_ATC_BUNDLE_URL=") {
			continue
		}
		out = append(out, e)
	}
	return out
}

func baselineInheritedEnvironment() []string {
	out := []string{}
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "TMP", "TEMP", "GOCACHE", "GOMODCACHE", "GOPATH", "SYSTEMROOT"} {
		if value, ok := os.LookupEnv(key); ok {
			out = append(out, key+"="+value)
		}
	}
	return out
}
