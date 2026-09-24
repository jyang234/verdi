package signedapproval

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

// approvalsKey is the top-level frontmatter key holding approval rows.
const approvalsKey = "approvals"

// approvalRow is one row of an artifact's approvals sequence with the
// 1-based, inclusive document lines it occupies.
type approvalRow struct {
	role, principal string
	first, last     int
}

// spans reports whether the row occupies document line n.
func (r approvalRow) spans(n int) bool { return r.first <= n && n <= r.last }

// parseApprovalRows locates every approval row of doc. A row must carry
// exactly a role (a governance id) and a principal (a canonical principal
// id), and no (role, principal) pair may repeat. A missing or empty
// approvals sequence yields no rows.
func parseApprovalRows(doc []byte) ([]approvalRow, error) {
	items, err := artifact.FrontmatterMappingItems(doc, approvalsKey)
	if err != nil {
		return nil, err
	}
	rows := make([]approvalRow, 0, len(items))
	seen := make(map[[2]string]bool, len(items))
	for i, it := range items {
		role, hasRole := it.Fields["role"]
		principal, hasPrincipal := it.Fields["principal"]
		if len(it.Fields) != 2 || !hasRole || !hasPrincipal {
			return nil, fmt.Errorf("approvals[%d] must carry exactly role and principal", i)
		}
		if err := gp.ValidateID(role); err != nil {
			return nil, fmt.Errorf("approvals[%d] role: %w", i, err)
		}
		if err := gp.PrincipalID(principal).Validate(); err != nil {
			return nil, fmt.Errorf("approvals[%d] principal: %w", i, err)
		}
		key := [2]string{role, principal}
		if seen[key] {
			return nil, fmt.Errorf("approvals: duplicate row (%s, %s)", role, principal)
		}
		seen[key] = true
		rows = append(rows, approvalRow{role: role, principal: principal, first: it.FirstLine, last: it.LastLine})
	}
	return rows, nil
}

// unreadableReason names why a historical copy of the artifact yields no
// approval rows: a frontmatter line break other than LF or CRLF has its own
// code, and every other parse failure takes fallback.
func unreadableReason(perr error, fallback string) string {
	if errors.Is(perr, artifact.ErrNonstandardLineBreak) {
		return ReasonNonstandardLineBreak
	}
	return fallback
}

// withoutRowLines returns doc with every line inside any row's span
// removed, each remaining line kept byte-for-byte with its terminator.
func withoutRowLines(doc []byte, rows []approvalRow) []byte {
	var out bytes.Buffer
	for i, line := range bytes.SplitAfter(doc, []byte("\n")) {
		n := i + 1
		inRow := false
		for _, r := range rows {
			if r.spans(n) {
				inRow = true
				break
			}
		}
		if !inRow {
			out.Write(line)
		}
	}
	return out.Bytes()
}
