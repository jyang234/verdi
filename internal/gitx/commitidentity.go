package gitx

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// CommitIdentityAvailable reports whether git, run in dir, can mint the
// committer identity a commit made there would need — asked BEFORE a
// caller writes anything, so a checkout that cannot commit is refused
// while refusing is still free.
//
// It is `git var GIT_COMMITTER_IDENT`, which composes the ident exactly
// as `git commit` does (the GIT_COMMITTER_* environment, then the
// --local, --global and --system config scopes, then a name derived from
// the OS account) and applies the same strict rule: a name that is empty,
// or auto-detected when user.useConfigOnly forbids that, is fatal rather
// than guessed at. The state this exists for is ordinary on a CI runner —
// no global identity, and an OS account whose full-name field is empty,
// so git dies with "empty ident name (for <runner@...>) not allowed"
// where a developer's machine would have silently derived one.
//
// Three answers:
//
//   - (true, nil): git printed an ident. A commit here has an identity.
//   - (false, nil): git RAN and refused. Every reason it refuses for is a
//     reason `git commit` would refuse too, which is the question being
//     asked;
//     TestCommitIdentityAvailable_MatchesWhetherGitCanCommit pins the two
//     answers together. An unparseable config file is folded in here
//     rather than reported operational: git exits 128 for that exactly as
//     it does for an unusable ident, and telling them apart would mean
//     matching git's own localized prose, which ConfigValue's ADJ-64 rule
//     forbids. The fold is fail-closed and true — git cannot mint an
//     identity out of a configuration it cannot read.
//   - (false, err): git never ran, or was cut short (a missing binary, a
//     missing directory, a cancelled context). Nothing was learned about
//     any identity, so this is never reported as "no identity
//     configured".
//
// Scope, disclosed: the probe asks about the COMMITTER. A commit also
// needs an author, and git composes it from the same user.name/user.email
// unless GIT_AUTHOR_NAME/GIT_AUTHOR_EMAIL explicitly override them — so a
// checkout that exports GIT_AUTHOR_NAME empty while leaving the committer
// mintable still fails at commit time, and its caller falls back on
// whatever the commit's own failure discloses. Every state a host or a
// config scope decides is decided identically for both.
func CommitIdentityAvailable(ctx context.Context, dir string) (bool, error) {
	out, err := run(ctx, dir, "var", "GIT_COMMITTER_IDENT")
	if err == nil {
		// Git prints the ident on success; an empty line would mean the
		// question was not actually answered, so it is not a yes.
		return strings.TrimSpace(string(out)) != "", nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		// A probe killed mid-flight exits non-zero like a refusal. It
		// judged nothing.
		return false, fmt.Errorf("gitx: CommitIdentityAvailable(%s): %w", dir, ctxErr)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, fmt.Errorf("gitx: CommitIdentityAvailable(%s): %w", dir, err)
}
