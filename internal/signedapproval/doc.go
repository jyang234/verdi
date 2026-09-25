// Package signedapproval authenticates the approval rows of one governed
// artifact against forge-verified signed commits (R-PB-1; ledger SI-250,
// SI-256).
//
// An approval row (role, principal) in an artifact's frontmatter
// `approvals` sequence is only a claim until Authenticate proves, at the
// head H:
//
//   - git blame at H attributes every line of the row to one commit C,
//     which is not a history boundary, and no line of any other row to C
//     (one act per row);
//   - C's own copy of the artifact, at the same path, carries exactly this
//     row on exactly the lines blame maps it from;
//   - the artifact at C and at H, each with its own approval-row lines
//     removed, are byte-identical, so the approval binds exactly the
//     content at H;
//   - every commit that changes the artifact on the full-history ancestry
//     path from C to H still carries the row, so a withdrawal cannot be
//     undone by a merge that restores a stale copy (in a shallow clone
//     that path cannot be shown complete);
//   - the frontmatter keeps one line-break convention, UTF-8 with only LF
//     and CRLF, so YAML's line numbers are Git's (at H a violation is an
//     operational error);
//   - the forge, read through the consumer-defined CommitVerifier port,
//     reports C's signature verified and attributes it to an account whose
//     canonical principal under the governing profile's signed-commit
//     trust source equals the row's principal. The forge names only an
//     account id, so a profile with more than one signed-commit source
//     authenticates no row.
//
// Every other row is unproven with a closed reason code. The only identity
// this package reads is the verifier's signer account id; it never reads
// git configuration, the environment, or the commit's author fields.
//
// AuthorizationInputs turns one authenticated Artifact into governance-
// kernel inputs for that artifact alone: only authenticated rows become
// approval records, and each principal gets one resolution from a trust-
// fact reader scoped to that principal's authenticated rows.
package signedapproval
