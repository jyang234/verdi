// The self-hosted evidence producer (spec/close-verb ac-3, dc-1): verdi is
// not a flowmap service of itself (D6-4), so `sync --produce`'s regular
// per-service regenerate() path (sync_regen.go) always assembles an EMPTY
// verdicts.json for this repo — nothing has ever bound evidence to verdi's
// OWN self-hosted stories, so a verdi story declaring [static, behavioral]
// evidence could never fold past pending. This file closes that gap: a
// store-ROOT `verdi.bindings.yaml` sidecar (NOT service-scoped — 03
// §Pluggable evidence licenses "any test suite" as a behavioral producer
// "behind the same sidecar seam", and this is that seam exercised at the
// store root instead of a discovered .flowmap.yaml service root) binds two
// fixed, declared producer ids — one static, one behavioral — to one or
// more self-hosted stories' ACs, using artifact.ResolveBindingAC's
// fragment-qualified form when more than one story is bound from the same
// file (e.g. remote-and-ci#ac-1 alongside close-verb#ac-1/#ac-3).
//
// HONESTY (dc-1). This producer performs NO test execution and NO
// verification of its own: it emits verdict: pass purely because it was
// INVOKED AT ALL, and it is invoked (runProduce, sync.go) only as a step in
// verify.yml strictly AFTER `make verify` has already exited 0 in the SAME
// CI job (see verify.yml's own comment for the wiring this repo settled on,
// and why: running make verify a second time, independently, in a
// different job would risk exactly the "divergent re-run" this decision
// forbids — a flake in either direction could make this producer disagree
// with the real, blocking gate). Reaching this step IS the evidence; there
// is no second test run anywhere in this file — mirroring dc-1's own
// framing for --produce as a whole ("authoritative solely because ... not
// because the flag was passed"): here, solely because this step in THIS
// job, after that step, was reached.
//
// DIRECTORY CONVENTION — RECONCILED by true-closure. The fold's canonical
// local key is the OWNING SPEC's ref slug, derived/<spec-ref-slug>/<commit>/
// (store.RefSlug(spec.ID)) — what every fold consumer reads (gate.go,
// closuregate.go, rollup.go, matrix.go, featurematrix.go, close.go) and what
// this producer writes. The pieces that formerly disagreed are now aligned
// with it: `verdi sync`'s forge-PULL path preserves the fetched artifact's
// per-spec subdirs verbatim (writeDerivedTree) instead of collapsing them to
// one branch-ref-keyed bundle, so exactly the per-spec records this producer
// uploads in CI land back where the readers look; and baseline.go keys its
// advisory local baseline by the spec ref too. The one convention still
// branch-ref-keyed is regenerate()'s WHOLE-BRANCH per-service bundle
// (sync_regen.go, the transport/gc unit, 01 §gc) — for THIS repo (no flowmap
// services, D6-4) that bundle is always empty, so the reachable fold records
// are solely the per-spec ones this producer writes. Per-service-spec demux
// of a real multi-service regenerate() is the disclosed follow-up (see the
// controller's residual-hardening note); it does not affect verdi's own
// self-hosted closure, which folds entirely on this producer's output.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// selfHostedBindingsRelPath is the STORE ROOT path (sibling of .verdi/, not
// nested inside any discovered .flowmap.yaml service root) the self-hosted
// producer reads its bindings sidecar from.
const selfHostedBindingsRelPath = "verdi.bindings.yaml"

// The self-hosted producer's two fixed, declared producer ids (03
// §Declarations: "the producer's own binding is declared, not inferred" —
// close-verb ac-3's static evidence requirement) — bound in
// verdi.bindings.yaml, never invented per-call.
const (
	selfHostedStaticProducer     = "verdi-verify-static"
	selfHostedBehavioralProducer = "verdi-verify-behavioral"
)

// produceSelfHostedEvidence reads root's own verdi.bindings.yaml, if
// present, and writes the evidence records it implies directly into each
// bound spec's own derived/<spec-ref-slug>/<commit>/verdicts.json (see this
// file's package doc for why that directory, not sync's regular
// branch-keyed bundle). A repo with no self-hosted bindings file yet is a
// silent no-op, not an error — most repos ARE real flowmap services and
// never need this producer at all.
func produceSelfHostedEvidence(root, commit string, prov artifact.EvidenceProvenance) error {
	bySpec, err := selfHostedEvidence(root, prov)
	if err != nil {
		return err
	}
	if len(bySpec) == 0 {
		return nil
	}
	return writeSelfHostedEvidence(root, commit, bySpec)
}

// selfHostedEvidence computes the evidence records root's verdi.bindings.yaml
// implies, grouped by the spec ref each record targets (one record per
// (kind, spec) pair, evidence_for restricted to that spec's own bare AC
// ids). Every kind other than static/behavioral is a hard error (no
// self-hosted producer for runtime/attestation, mirroring
// internal/bundle.BuildVerdicts's same refusal). Dangling bindings — an AC
// id the named spec does not actually declare — fail loudly (03
// §Declarations: "a misspelled ac-3 must never surface as a silent
// no-signal"), checked here since this store-root sidecar sits outside
// VL-003's existing (service-scoped) bindings discovery.
func selfHostedEvidence(root string, prov artifact.EvidenceProvenance) (map[string][]artifact.Evidence, error) {
	path := filepath.Join(root, selfHostedBindingsRelPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("self-hosted evidence: reading %s: %w", path, err)
	}
	bindings, err := artifact.DecodeBindings(data)
	if err != nil {
		return nil, fmt.Errorf("self-hosted evidence: decoding %s: %w", path, err)
	}

	// specRef -> kind -> set of bare ac ids.
	bySpecKind := map[string]map[artifact.EvidenceKind]map[string]bool{}

	for _, b := range bindings.Bindings {
		if b.Kind != artifact.EvidenceStatic && b.Kind != artifact.EvidenceBehavioral {
			return nil, fmt.Errorf("self-hosted evidence: binding %q: kind %q has no self-hosted producer (static/behavioral only)", b.Producer, b.Kind)
		}
		for _, entry := range b.ACs {
			specRef, acID, err := artifact.ResolveBindingAC(bindings.Spec, entry)
			if err != nil {
				return nil, fmt.Errorf("self-hosted evidence: binding %q: %w", b.Producer, err)
			}
			if err := verifySelfHostedACDeclared(root, specRef, acID); err != nil {
				return nil, fmt.Errorf("self-hosted evidence: binding %q: %w", b.Producer, err)
			}
			if bySpecKind[specRef] == nil {
				bySpecKind[specRef] = map[artifact.EvidenceKind]map[string]bool{}
			}
			if bySpecKind[specRef][b.Kind] == nil {
				bySpecKind[specRef][b.Kind] = map[string]bool{}
			}
			bySpecKind[specRef][b.Kind][acID] = true
		}
	}

	specRefs := make([]string, 0, len(bySpecKind))
	for s := range bySpecKind {
		specRefs = append(specRefs, s)
	}
	sort.Strings(specRefs)

	out := make(map[string][]artifact.Evidence, len(specRefs))
	for _, specRef := range specRefs {
		var records []artifact.Evidence
		for _, kind := range []artifact.EvidenceKind{artifact.EvidenceStatic, artifact.EvidenceBehavioral} {
			acSet := bySpecKind[specRef][kind]
			if len(acSet) == 0 {
				continue
			}
			acs := make([]string, 0, len(acSet))
			for ac := range acSet {
				acs = append(acs, ac)
			}
			sort.Strings(acs)

			rec := artifact.Evidence{
				Schema:      "verdi.evidence/v1",
				EvidenceFor: acs,
				Kind:        kind,
				Verdict:     artifact.VerdictPass,
				Witness:     selfHostedWitness(kind),
				Producer:    selfHostedProducer(kind),
				Provenance:  prov,
			}
			digest, err := selfHostedDigest(rec)
			if err != nil {
				return nil, err
			}
			rec.Digest = digest
			records = append(records, rec)
		}
		out[specRef] = records
	}
	return out, nil
}

func selfHostedProducer(kind artifact.EvidenceKind) string {
	if kind == artifact.EvidenceStatic {
		return selfHostedStaticProducer
	}
	return selfHostedBehavioralProducer
}

func selfHostedWitness(kind artifact.EvidenceKind) string {
	if kind == artifact.EvidenceStatic {
		return "make verify: build + vet clean"
	}
	return "make verify: go test + e2e passed"
}

// selfHostedDigest hashes rec's declared content (kind, producer, and the
// AC set it attests) — recomputable from those pinned inputs, mirroring
// internal/bundle's own recordDigest posture (02 §Generated artifacts and
// digests): a content-address of the upstream fact this record asserts,
// not of wall-clock or provenance metadata.
func selfHostedDigest(rec artifact.Evidence) (string, error) {
	keyed := struct {
		Kind        artifact.EvidenceKind `json:"kind"`
		Producer    string                `json:"producer"`
		EvidenceFor []string              `json:"evidence_for"`
	}{Kind: rec.Kind, Producer: rec.Producer, EvidenceFor: rec.EvidenceFor}
	data, err := canonjson.Marshal(keyed)
	if err != nil {
		return "", fmt.Errorf("self-hosted evidence: computing digest: %w", err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// verifySelfHostedACDeclared fails loudly when specRef does not resolve in
// this store, or resolves but does not declare acID (03 §Declarations:
// dangling bindings are errors, never a silent empty cell) — the same
// discipline internal/bundle.BuildVerdicts applies via its SpecACs check,
// reimplemented here because this store-root sidecar is not one
// sync_regen.go's regenerateService already resolves a spec for.
func verifySelfHostedACDeclared(root, specRef, acID string) error {
	ref, err := artifact.ParseRef(specRef)
	if err != nil || ref.Kind != artifact.KindSpec {
		return fmt.Errorf("spec ref %q is not a valid spec ref", specRef)
	}
	spec, err := storyresolve.LoadSpec(root, ref.Name)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", specRef, err)
	}
	if spec == nil {
		return fmt.Errorf("spec %q does not resolve to a spec in this store (dangling binding)", specRef)
	}
	for _, ac := range spec.AcceptanceCriteria {
		if ac.ID == acID {
			return nil
		}
	}
	return fmt.Errorf("%s does not declare ac %q (dangling binding, 03 §Declarations)", specRef, acID)
}

// writeSelfHostedEvidence merges bySpec's records into each named spec's own
// derived/<spec-ref-slug>/<commit>/verdicts.json (store.RefSlug(specRef)) —
// the directory convention every fold consumer (gate/rollup/matrix/
// closuregate) actually reads (see this file's package doc). Existing
// records sharing a new record's producer id are replaced (idempotent
// across same-commit CI re-runs); every other existing record is preserved.
func writeSelfHostedEvidence(root, commit string, bySpec map[string][]artifact.Evidence) error {
	specRefs := make([]string, 0, len(bySpec))
	for s := range bySpec {
		specRefs = append(specRefs, s)
	}
	sort.Strings(specRefs)

	for _, specRef := range specRefs {
		dir := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(specRef), commit)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("self-hosted evidence: mkdir %s: %w", dir, err)
		}
		path := filepath.Join(dir, "verdicts.json")
		existing, err := readExistingEvidenceRecords(path)
		if err != nil {
			return err
		}
		merged := mergeEvidenceByProducer(existing, bySpec[specRef])
		data, err := canonjson.Marshal(merged)
		if err != nil {
			return fmt.Errorf("self-hosted evidence: marshaling %s: %w", path, err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("self-hosted evidence: writing %s: %w", path, err)
		}
	}
	return nil
}

// readExistingEvidenceRecords reads and strict-decodes an already present
// verdi.evidence/v1 array file at path, if any — (nil, nil) when the file
// does not exist yet (the ordinary first-write case). Generic over which
// derived-tree file it reads: produceSelfHostedEvidence uses it for
// verdicts.json, runtimeprobe.go's writeRuntimeRecord reuses it unchanged
// for runtime.json (dc-2's sibling file) — both are the same schema, one
// array of records (03 §Evidence records), so one reader serves both
// producers rather than each copy-pasting its own (CLAUDE.md: shared code
// is never copy-pasted across call sites in the same package either).
func readExistingEvidenceRecords(path string) ([]artifact.Evidence, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshaling %s: %w", path, err)
	}
	out := make([]artifact.Evidence, 0, len(raw))
	for i, rm := range raw {
		rec, err := artifact.DecodeEvidence(rm)
		if err != nil {
			return nil, fmt.Errorf("%s record %d: %w", path, i, err)
		}
		out = append(out, *rec)
	}
	return out, nil
}

// mergeEvidenceByProducer replaces any existing record sharing an incoming
// record's Producer, keeps every other existing record unchanged, and
// appends the incoming records — idempotent across same-commit CI re-runs
// (03 §The fold already takes "the latest run's verdict" as authoritative;
// this additionally keeps the FILE itself from growing unboundedly across
// retries of the same job on the same commit).
func mergeEvidenceByProducer(existing, incoming []artifact.Evidence) []artifact.Evidence {
	replaced := make(map[string]bool, len(incoming))
	for _, r := range incoming {
		replaced[r.Producer] = true
	}
	out := make([]artifact.Evidence, 0, len(existing)+len(incoming))
	for _, r := range existing {
		if replaced[r.Producer] {
			continue
		}
		out = append(out, r)
	}
	out = append(out, incoming...)
	return out
}
