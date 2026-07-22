// Accept's post-flip rollback registry (spec/obligation-seam ac-3,
// judged-postflip-rollback-window): everything accept overwrites in place or
// stages after scaffolding begins — the spec's own draft->accepted-pending-build
// flip, any predecessor status flips, and the index paths it stages — is
// recorded here BEFORE the mutation, so that ANY refusal or operational error
// after the first on-disk flip restores the working tree and index to exactly
// their pre-accept state: a pristine tree, no partial commit. The scaffolded
// obligation stubs are creations, not overwrites, and are cleaned separately
// by unlinkScaffoldedObligations (acceptobligation.go); this type owns the
// overwrite-and-stage half of "leaves everything else exactly as it found it".
//
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go/attest.go
// convention, so accept.go's own diff for wiring this in stays small.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jyang234/verdi/internal/gitx"
)

// acceptRollback captures the pre-write bytes of each in-place file accept
// overwrites (the spec flip and any predecessor flips) and the index paths it
// stages, so restore can put both back on accept's failure path.
type acceptRollback struct {
	files  []fileSnapshot
	staged []string
}

// fileSnapshot is one overwritten file's path and its pre-write content.
type fileSnapshot struct {
	path     string
	original []byte
}

// recordFile snapshots path's pre-write content, called at the moment just
// before that file is overwritten. original is copied, so a caller may reuse
// its buffer afterward. Its signature is exactly the recorder callback
// supersede.go's flip threads through, so a predecessor flip records its own
// pre-flip bytes the same way the spec flip does.
func (r *acceptRollback) recordFile(path string, original []byte) {
	r.files = append(r.files, fileSnapshot{path: path, original: append([]byte(nil), original...)})
}

// stage records the index paths accept is about to stage (its scoped addPaths
// set), so restore unstages exactly them.
func (r *acceptRollback) stage(paths ...string) {
	r.staged = append(r.staged, paths...)
}

// restore rewrites every recorded file back to its pre-write bytes and resets
// the staged index entries to HEAD (a mixed reset — the working tree is put
// back by the file rewrites above, never by git). It is best-effort: a
// restore failure is disclosed to stderr but never changes accept's own exit
// code, since the caller already has the real refusal or error to report.
// Resetting a path that was never actually staged is a harmless no-op, so the
// staged set may safely be the whole intended addPaths set even if AddPaths
// itself failed partway. Called only on accept's failure path.
func (r *acceptRollback) restore(ctx context.Context, root string, stderr io.Writer) {
	for _, f := range r.files {
		if err := os.WriteFile(f.path, f.original, 0o644); err != nil {
			fmt.Fprintf(stderr, "accept: warning: restoring %s after refusal: %v\n", f.path, err)
		}
	}
	if len(r.staged) > 0 {
		if err := gitx.ResetPaths(ctx, root, r.staged...); err != nil {
			fmt.Fprintf(stderr, "accept: warning: unstaging after refusal: %v\n", err)
		}
	}
}
