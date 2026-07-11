package gitx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// run execs `git <args...>` with its working directory set to dir, returning
// stdout on success. A non-zero exit becomes an error naming the command and
// stderr, never a silent empty result.
func run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gitx: git %s (dir %s): %w: %s", strings.Join(args, " "), dir, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
