package lint

import (
	"context"
	"sort"
)

// RunInput bundles everything a Rule needs: the pre-built Snapshot, the
// git/CI Context, the engine Options, and a std context plus the store
// root for rules that must exec git directly (VL-003/009's commit
// realness, VL-010's diff).
type RunInput struct {
	Ctx      context.Context
	Root     string
	Snapshot *Snapshot
	LintCtx  Context
	Opts     Options
}

// Rule is one VL-xxx check.
type Rule interface {
	ID() string
	Check(in *RunInput) []Finding
}

// allRules is every VL-001..VL-022 rule, in id order.
var allRules = []Rule{
	vl001{}, vl002{}, vl003{}, vl004{}, vl005{}, vl006{}, vl007{},
	vl008{}, vl009{}, vl010{}, vl011{}, vl012{}, vl013{}, vl014{},
	vl015{}, vl016{}, vl017{}, vl018{}, vl019{}, vl020{}, vl021{},
	vl022{},
}

// Engine runs every rule over a store root and reports every finding.
type Engine struct {
	rules []Rule
}

// NewEngine returns an Engine configured with all twenty-two rules.
func NewEngine() *Engine {
	return &Engine{rules: allRules}
}

// Run builds a Snapshot for root and runs every rule over it, returning
// every finding sorted deterministically (by rule, then path, then
// message). A non-nil error is always operational (root unreadable,
// service discovery failed) — per-artifact decode/content problems are
// findings, never errors.
func (e *Engine) Run(ctx context.Context, root string, lctx Context, opts Options) ([]Finding, error) {
	snap, err := BuildSnapshot(root, opts)
	if err != nil {
		return nil, err
	}

	in := &RunInput{Ctx: ctx, Root: root, Snapshot: snap, LintCtx: lctx, Opts: opts}

	var findings []Finding
	for _, r := range e.rules {
		findings = append(findings, r.Check(in)...)
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Rule != findings[j].Rule {
			return findings[i].Rule < findings[j].Rule
		}
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Message < findings[j].Message
	})
	return findings, nil
}
