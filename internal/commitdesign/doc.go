// Package commitdesign implements the mechanical half of the
// commit-to-design ritual (05 §Workbench, PLAN.md ledger I-20): given a
// board's pins/stickies/yarn, write a draft feature spec skeleton (story,
// placeholder AC, `context:` from the board's pinned refs) on the current
// design branch, a frozen `board.json` snapshot committed alongside it
// (design provenance, one frame — never a drag history), and a
// `dispositions:` block listing every sticky as `open-question` — legal,
// honest, and VL-014-lint-clean until the commit-to-design skill (or a
// human) upgrades each entry to `incorporated` or `contradicted`.
//
// I-20 explicitly draws this split: "the binary does the mechanical half
// ... yarn promotion + prose belong to a committed skill doc outside the
// binary; VL-014 is the backstop for both." This package is the binary's
// half. It has exactly one entry point (Run), called from two places
// (05's own instruction: "the board page can call the verb's logic over
// HTTP" — meaning the WORKBENCH's HTTP handler and the CLI verb both call
// this same in-process Go function; neither shells out to the other):
//
//   - CLI: `verdi board commit <board-key> --name <spec-name> [--story-ref <scheme:key>]`
//   - workbench: `POST /board/<board-key>/commit` with the same fields as
//     a JSON body
//
// Local naming note (disclosed here since this phase's scope excludes
// editing PLAN.md's invention ledger): a board's own key (the filename
// stem under mutable/boards/, e.g. "STORY-1482" in the phase-2 corpus
// fixture) and a feature spec's `story:` field (a scheme-prefixed tracker
// ref, e.g. "jira:LOAN-1482") are two identifier spaces PLAN.md's ledger
// (I-30, I-31) explicitly declines to bridge by any heuristic. Run
// therefore takes StoryRef as an explicit input; when the caller omits
// it, Run accepts the board key itself only if it already has the
// scheme:key shape a spec's `story:` field requires — never a silent,
// fuzzy translation between the two spaces.
package commitdesign
