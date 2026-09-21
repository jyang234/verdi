// Command oq2decode is spike/self-governance oq-2's decode harness: it
// wraps each of the five CLAUDE.md ground rules picked by
// spec/self-governance oq-2 in the best-faith policy-claim YAML the
// kernel grammar (internal/policyartifact) admits today, decodes each
// through the real, exported policyartifact.DecodePolicy seam (never a
// hand-built Claim value — the plan calls for running the YAML "through
// the decoder"), and reports the outcome. It also probes whether a
// feature-owned payload kind named "ground_rule" is registered, to
// support the "fits with a new payload kind" verdict for the two rules
// that have no plausible claim operator at all.
//
// Evidence-only: lives under docs/spikes/self-governance/_scratch/ (the
// underscore prefix keeps it out of `go list ./...` and every gate
// except an explicit `go run` naming this directory), gofmt-clean because
// `make fmt-check` runs gofmt -l . over the whole tree regardless.
package main

import (
	"fmt"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// rule is one of the five CLAUDE.md ground rules oq-2 names, plus the
// best-faith claim YAML this program attempts on its behalf.
type rule struct {
	name string
	text string // the CLAUDE.md ground rule, verbatim from the parent spec's oq-2
	yaml string // a complete, minimal verdi.policy/v1 artifact carrying exactly one claim for this rule
}

const templateLine = `template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}`

func policyDoc(id, claimYAML string) string {
	return fmt.Sprintf(`---
schema: verdi.policy/v1
id: policy/%s
kind: policy
title: "oq-2 scratch: %s"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
  - %s
instructions: []
payloads: {}
%s
---
Scratch decode probe for spec/self-governance oq-2; not a real policy.
`, id, id, claimYAML, templateLine)
}

var rules = []rule{
	{
		name: "1-context-first-param",
		text: `context is the first parameter of anything doing I/O`,
		yaml: policyDoc("ctx-first-param", `id: ctx-first-param
    family: action
    operator: required-values
    subject: io-function-signature
    values: [context-first-parameter]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: false`),
	},
	{
		name: "2-errors-wrap-percent-w",
		text: `errors wrap with %w`,
		yaml: policyDoc("error-wrap-verb", `id: error-wrap-verb
    family: action
    operator: equals
    subject: error-wrap-format-verb
    values: ["percent-w"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: false`),
	},
	{
		name: "3-no-god-packages",
		text: `no god packages ("a package you would need two sentences to describe is two packages")`,
		yaml: policyDoc("single-responsibility-packages", `id: single-responsibility-packages
    family: configuration
    operator: equals
    subject: package-scope-discipline
    values: ["single-concern"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: false`),
	},
	{
		name: "4-every-commit-builds",
		text: `every commit builds`,
		yaml: policyDoc("commit-must-build", `id: commit-must-build
    family: action
    operator: required-values
    subject: commit-build-status
    values: ["clean-exit"]
    scope: {phases: [build], environments: [], paths: [], refs: []}
    overridable: false`),
	},
	{
		name: "5-never-bare-git-stash",
		text: `never bare git stash`,
		yaml: policyDoc("no-bare-git-stash", `id: no-bare-git-stash
    family: action
    operator: forbidden-values
    subject: git-stash-invocation
    values: ["bare"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: false`),
	},
}

func main() {
	fmt.Println("=== oq-2: five CLAUDE.md ground rules through the real policyartifact.DecodePolicy seam ===")
	for _, r := range rules {
		fmt.Printf("\n--- rule %s ---\ntext: %s\nyaml:\n%s\n", r.name, r.text, r.yaml)
		p, err := policyartifact.DecodePolicy([]byte(r.yaml))
		if err != nil {
			fmt.Printf("result: DECODE ERROR: %v\n", err)
			continue
		}
		digest, derr := p.Digest()
		if derr != nil {
			fmt.Printf("result: decoded but Digest() failed: %v\n", derr)
			continue
		}
		claim := p.Claims[0]
		fmt.Printf("result: DECODED OK — claim id=%s family=%s operator=%s subject=%s values=%v policy_digest=%s\n",
			claim.ID, claim.Family, claim.Operator, claim.Subject, claim.Values, digest)
	}

	fmt.Println("\n=== payload-kind probe: is a feature-owned \"ground_rule\" payload registered today? ===")
	probeYAML := fmt.Sprintf(`---
schema: verdi.policy/v1
id: policy/payload-probe
kind: policy
title: "oq-2 scratch: payload probe"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
payloads:
  ground_rule: {rule_id: ctx-first-param, enforcing_mechanism: "golangci-lint:contextcheck"}
%s
---
Scratch probe: is the ground_rule payload kind registered?
`, templateLine)
	fmt.Printf("yaml:\n%s\n", probeYAML)
	_, err := policyartifact.DecodePolicy([]byte(probeYAML))
	if err != nil {
		fmt.Printf("result: DECODE ERROR (expected if unregistered): %v\n", err)
	} else {
		fmt.Println("result: DECODED OK — a ground_rule payload kind IS registered today")
	}
}
