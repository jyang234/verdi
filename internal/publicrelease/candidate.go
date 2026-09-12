package publicrelease

import (
	"context"
	"fmt"
	"path/filepath"
)

func candidateBuild(name, dir, binary, pkg string, env []string) command {
	// Match boundaryBuiltVatc exactly; only the candidate ATC uses this ordinary
	// build. Fixed baseline archive builds, Verdi and the checker retain trimpath.
	if name == "atc" {
		return command{Dir: filepath.Join(dir, "cmd/vatc"), Name: "build-" + name, Args: []string{"go", "build", "-o", binary, "."}, Env: env}
	}
	return command{Dir: dir, Name: "build-" + name, Args: []string{"go", "build", "-trimpath", "-o", binary, pkg}, Env: env}
}

func authenticateCandidate(ctx context.Context, x executor, cmd command, want string) (execution, error) {
	out, err := x.Run(ctx, cmd)
	if err != nil {
		return out, err
	}
	if out.Exit != 0 {
		return out, fmt.Errorf("%w: %s build", ErrVerdict, cmd.Name)
	}
	binary := cmd.Args[len(cmd.Args)-2]
	h, err := fileDigest(binary)
	if err != nil {
		return out, err
	}
	if h != want {
		return out, fmt.Errorf("candidate %s executable differs from exact-source rebuild", cmd.Name)
	}
	return out, nil
}
