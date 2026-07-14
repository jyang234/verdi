// Package refindex computes the directory index spec/workbench-directory
// ac-2 needs — every spec on the default branch and every unmerged design
// branch's draft — as a pure function of git ref state (spec/ref-index).
//
// ComputeIndex never switches a checkout: it depends only on a narrow,
// consumer-defined GitRunner port (dc-2, the 04 §port pattern) whose method
// set contains nothing capable of moving HEAD or writing a working tree or
// index (ac-5's static half). internal/refindex owns no HTTP handler, no
// page, and no rendering — directory-home (a sibling story under the same
// feature) is the only consumer that turns []Entry into markup (dc-1).
package refindex
