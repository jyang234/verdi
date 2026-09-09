package gitx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// AheadBehind counts the commits reachable from left but not right
// (ahead) and from right but not left (behind) — `git rev-list
// --left-right --count left...right`. The Wave 6 posture header consumes
// it for the worktree-vs-accepted divergence facts (design §4.2:
// "ahead/behind/diverged posture when available").
func AheadBehind(ctx context.Context, dir, left, right string) (ahead, behind int, err error) {
	out, err := run(ctx, dir, "rev-list", "--left-right", "--count", left+"..."+right)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("gitx: rev-list --left-right --count returned %q", strings.TrimSpace(string(out)))
	}
	ahead, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("gitx: parsing ahead count %q: %w", fields[0], err)
	}
	behind, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("gitx: parsing behind count %q: %w", fields[1], err)
	}
	return ahead, behind, nil
}
