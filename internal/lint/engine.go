package lint

import (
	"context"
	"sort"

	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
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
	// Model is the store's resolved operating model, best-effort (Run
	// leaves it nil when the store's config cannot be opened — findings
	// then speak bare ids via model.Model's nil-receiver fallback; never
	// a lint failure, since lint must still run over a store whose
	// verdi.yaml is broken and reports that through its own rules, not
	// through a display lookup). Used ONLY to route class display words
	// in finding prose (ledger L-M13a(6) work order); no rule DECISION
	// ever reads it.
	Model *model.Model
	// Projector is the shared git-derived-state resolver every rule that
	// needs one resolves candidates through (Task 4: VL-004's legacy-draft
	// compatibility disclosure; also VL-015's merge-signaled predecessor
	// baseline read, for a predecessor carrying no frozen: stamp) —
	// constructed ONCE per Run call, here,
	// rather than once per rule/per document, so a full lint pass never
	// builds more than the one real-git-backed Projector NewProjector
	// already returns cheaply (Projector itself is stateless per call;
	// ResolveMany's own per-call successor-corpus memoization is
	// unaffected by sharing the value). Rules consume
	// SpecStateResolver.Resolve/specstate.Result — never reimplement
	// reachability (CLAUDE.md, PLAN.md file-ownership map).
	Projector SpecStateResolver
}

// SpecStateResolver is the internal/specstate.Projector consumption
// surface every git-derived-state rule needs, narrowed to exactly Resolve
// (04 §port pattern: define the interface at the consumer, accept
// interfaces, return structs). specstate.Projector satisfies this
// structurally with no adapter — Run wires the real, git-backed
// specstate.NewProjector() below unconditionally; no other production
// value is ever assigned. The interface exists ONLY so a rule's own test
// can substitute a fake implementing this one method, driving a
// specstate.Result shape that specstate's own gitReader-level fakes can
// reach (internal/specstate/resolve_test.go) but that real git cannot
// practically reconstruct through ordinary repository operations — e.g. a
// blob whose bytes match the default branch but whose first-parent
// landing commit cannot be proven (specstate/resolve.go's own
// "no first-parent landing commit could be proven" branch: BlobAt and
// FirstParentBlobLanding walk the exact same ref, so a real repository's
// tip commit always self-proves its own landing; only a stubbed
// FirstParentBlobLanding can force the mismatch).
type SpecStateResolver interface {
	Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error)
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

	// Display-only, best-effort (see RunInput.Model's doc comment): a
	// store with no openable config lints exactly as before, bare ids.
	var mdl *model.Model
	if cfg, err := store.Open(root); err == nil {
		mdl = cfg.Model
	}

	in := &RunInput{Ctx: ctx, Root: root, Snapshot: snap, LintCtx: lctx, Opts: opts, Model: mdl, Projector: specstate.NewProjector()}

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
