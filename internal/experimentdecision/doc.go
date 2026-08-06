// Package experimentdecision implements the comparative-spike-experiments
// (CSE) closed deterministic recommendation engine
// (spec/comparative-spike-experiments AC-2, DC-4, DC-5, DC-6, DC-12,
// DC-14, CO-1, CO-3, CO-5, CO-7): given one locked
// github.com/jyang234/verdi/internal/experiment.Definition and a complete,
// integrity-valid observation set, Evaluate deterministically produces
// exactly one experiment.Result expressing proven-winner,
// violated-with-witness, or disclosed-unproven, in the registered
// evaluation order. RenderResult and RenderRecommendation turn a Result
// into its canonical result.json bytes and a deterministic
// recommendation.md explanation.
//
// This package owns the decision function and its rendering only. It
// consumes internal/experiment, internal/canonjson, and the standard
// library exclusively: no weighted score, dynamic metric selection,
// threshold mutation, or tie-breaker exists anywhere in it, and it has no
// configuration beyond the locked definition itself (DC-4).
package experimentdecision
