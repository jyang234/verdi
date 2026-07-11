// Package workbench is the localhost-only HTTP surface `verdi serve`
// hosts alongside the MCP socket (05 §Workbench: "`verdi serve` binds
// localhost only"). PLAN.md Phase 9 ships only a health/index skeleton —
// "full workbench is phase 10" — but the package is structured so phase
// 10 adds PAGES here rather than rewiring how `verdi serve` hosts HTTP:
// every route is registered in one place (RegisterRoutes), and each
// page's handler lives in its own file, one file per route, matching this
// module's file-per-topic convention.
package workbench
