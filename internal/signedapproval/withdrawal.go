package signedapproval

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/gitx"
)

// withdrawal checks that no later history withdrew row r after commit c
// introduced it (SI-256): the row must be present, with the same role and
// principal, in every commit that changes the artifact on the full-history
// ancestry path from c to the head. Blame passes a row that a merge
// restores from a stale branch back to c, so blame alone cannot see a
// withdrawal followed by such a merge.
//
// A commit on that path that leaves the artifact unchanged holds the same
// copy as its parents on the path, so checking the commits that change it
// covers the whole path. In a shallow clone the path may run past the
// clone's boundary, so it cannot be shown complete and the row is unproven
// with history-boundary.
//
// It returns an empty reason when the row was never withdrawn, and a
// reason and detail when it is unproven. A git failure is an operational
// error.
func (a *authenticator) withdrawal(ctx context.Context, r approvalRow, c string) (reason, detail string, err error) {
	if c == a.in.Head {
		return "", "", nil
	}
	shallow, err := a.shallow(ctx)
	if err != nil {
		return "", "", err
	}
	if shallow {
		return ReasonHistoryBoundary, fmt.Sprintf("the repository is a shallow clone, so the ancestry path from %s to %s cannot be shown complete and a withdrawal of this row may lie beyond its boundary", c, a.in.Head), nil
	}
	changes, err := gitx.AncestryPathChanges(ctx, a.in.Root, c, a.in.Head, a.in.Path)
	if err != nil {
		return "", "", fmt.Errorf("signedapproval: listing the changes to %s from %s to %s: %w", a.in.Path, c, a.in.Head, err)
	}
	for _, x := range changes {
		present, err := gitx.PathExistsAt(ctx, a.in.Root, x, a.in.Path)
		if err != nil {
			return "", "", fmt.Errorf("signedapproval: %w", err)
		}
		if !present {
			return ReasonRowWithdrawnAfterApproval, fmt.Sprintf("commit %s, after %s approved this row, removes the artifact", x, c), nil
		}
		doc, err := gitx.Show(ctx, a.in.Root, x, a.in.Path)
		if err != nil {
			return "", "", fmt.Errorf("signedapproval: reading %s at %s: %w", a.in.Path, x, err)
		}
		rows, perr := parseApprovalRows(doc)
		if perr != nil {
			return unreadableReason(perr, ReasonRowWithdrawnAfterApproval), fmt.Sprintf("commit %s, after %s approved this row, has no readable approval rows: %v", x, c, perr), nil
		}
		if !hasRow(rows, r) {
			return ReasonRowWithdrawnAfterApproval, fmt.Sprintf("commit %s, after %s approved this row, does not carry it", x, c), nil
		}
	}
	return "", "", nil
}

// shallow reports, once per Authenticate, whether the repository is a
// shallow clone.
func (a *authenticator) shallow(ctx context.Context) (bool, error) {
	if !a.shallowKnown {
		s, err := gitx.IsShallow(ctx, a.in.Root)
		if err != nil {
			return false, fmt.Errorf("signedapproval: %w", err)
		}
		a.isShallow, a.shallowKnown = s, true
	}
	return a.isShallow, nil
}

// hasRow reports whether rows holds a row with r's role and principal.
func hasRow(rows []approvalRow, r approvalRow) bool {
	for _, o := range rows {
		if o.role == r.role && o.principal == r.principal {
			return true
		}
	}
	return false
}
