package experimentdecision

import (
	"bytes"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/experiment"
)

// VerifyResult decides whether res, as found AT REST, really is the closed
// engine's output for the locked def and the complete observation set obs
// (invention ledger SI-43). It is the implementation
// experiment.DeriveState's ResultVerifier port expects, wired here because
// the import direction only runs this way.
//
// It answers by RECOMPUTE-EQUALITY, not by inspection: res is authority
// only when re-running the closed computation over the same evidence
// produces canonically byte-identical content. Shape, enum, digest, and
// algorithm checks — everything experiment.DecodeResult already does —
// cannot reach this question, because a handwritten result.json can pass
// every one of them while naming any winner it likes (the direct Git-edit
// mutation surface AC-5/AC-6 leave open). Recomputation is what makes the
// forgery detectable.
//
// The comparison is over canonical JSON bytes (RenderResult), which is
// exactly the identity experiment.ResultDigest binds: two results agreeing
// on every value but spelling one number differently are NOT equal here,
// deliberately, because CO-3's byte identity is the property at stake.
//
// Every failure is an operational error (CO-1) — an unlocked or tampered
// definition, incomplete or integrity-violating observations, an invalid
// res, or a genuine mismatch. None of them is a verdict: a result that
// does not recompute makes no statement about candidates at all.
//
// It takes NO environment attestation, unlike Evaluate: an attestation is
// the execution layer's assertion about the run that produced obs (SI-42),
// which no at-rest reader can make. That conjunct stays disclosed-unproven
// at rest until spec/execution-workspace's durable receipt lands (SI-44).
func VerifyResult(def experiment.Definition, obs []experiment.Observation, receipt *experiment.ExecutionReceipt, res experiment.Result) error {
	locked, err := experiment.Locked(def)
	if err != nil {
		return errfWrap("verifying result: checking definition lock", err)
	}
	if !locked {
		return errf("verifying result: definition %q is not locked", def.ID)
	}
	if err := experiment.ValidateComplete(def, obs); err != nil {
		return errfWrap("verifying result: validating observations", err)
	}

	recomputed, err := compute(def, obs)
	if err != nil {
		return errfWrap("verifying result: recomputing the decision", err)
	}
	if res.Schema == experiment.ResultSchemaV2 {
		if receipt == nil {
			return errf("verifying result: result v2 requires an execution receipt")
		}
		if err := res.Validate(); err != nil {
			return errfWrap("verifying result: validating the stored result", err)
		}
		if err := experiment.ValidateWarmupDiagnosticsOrder(def, res.Execution.WarmupDiagnostics); err != nil {
			return errfWrap("verifying result: validating warmup diagnostic order", err)
		}
		if err := experiment.ValidateExecutionReceiptBinding(def, obs, *receipt); err != nil {
			return errfWrap("verifying result: validating execution receipt identity", err)
		}
		decision, err := experiment.DecisionFromResult(recomputed, obs)
		if err != nil {
			return errfWrap("verifying result: projecting the recomputed decision", err)
		}
		want, err := canonjson.Marshal(decision)
		if err != nil {
			return errfWrap("verifying result: rendering the recomputed decision", err)
		}
		got, err := canonjson.Marshal(*res.Decision)
		if err != nil {
			return errfWrap("verifying result: rendering the stored decision", err)
		}
		if !bytes.Equal(want, got) {
			return errf("verifying result: the stored decision is not this engine's own output for this definition and observation set\nstored:\n%s\nrecomputed:\n%s", got, want)
		}
		if err := experiment.ValidateResultReceipt(*receipt, res); err != nil {
			return errfWrap("verifying result: validating execution receipt binding", err)
		}
		return nil
	}
	if receipt != nil {
		return errf("verifying result: result v1 forbids an execution receipt")
	}
	want, err := RenderResult(recomputed)
	if err != nil {
		return errfWrap("verifying result: rendering the recomputed decision", err)
	}
	got, err := RenderResult(res)
	if err != nil {
		return errfWrap("verifying result: rendering the stored result", err)
	}
	if !bytes.Equal(want, got) {
		return errf("verifying result: the stored result is not this engine's own output for this definition and observation set\nstored:\n%s\nrecomputed:\n%s", got, want)
	}
	return nil
}
