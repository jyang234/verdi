// Package strictfixture is TestLintStrict_ReportsGroundRuleFindings's fixture
// module (spec/strict-lint-gate ac-1): each file named after a gated linter
// holds exactly that linter's one violation, and clean.go holds none. It is a
// module of its own under testdata/, so the repository's ./... never lints it.
package strictfixture
