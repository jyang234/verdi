package experiment

import "testing"

func mustDecodeDefinition(t *testing.T, doc string) Definition {
	t.Helper()
	def, err := DecodeDefinition([]byte(doc))
	if err != nil {
		t.Fatalf("DecodeDefinition() unexpected error: %v", err)
	}
	return def
}

func TestNormalizeDefinitionExcludesLock(t *testing.T) {
	doc := validDefinitionYAML() + "lock:\n  definition_digest: " + digestOf("9") + "\n"
	def := mustDecodeDefinition(t, doc)

	n, err := NormalizeDefinition(def)
	if err != nil {
		t.Fatalf("NormalizeDefinition() unexpected error: %v", err)
	}
	if n.ID != def.ID {
		t.Errorf("n.ID = %q, want %q", n.ID, def.ID)
	}
	// NormalizedDefinition carries no Lock field at all — a compile-time
	// guarantee — but assert the digest is unaffected by the lock block's
	// presence as the behavioral proof.
	unlocked := mustDecodeDefinition(t, validDefinitionYAML())
	dLocked, err := DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest(locked) unexpected error: %v", err)
	}
	dUnlocked, err := DefinitionDigest(unlocked)
	if err != nil {
		t.Fatalf("DefinitionDigest(unlocked) unexpected error: %v", err)
	}
	if dLocked != dUnlocked {
		t.Errorf("DefinitionDigest with lock = %q, without lock = %q, want equal (lock excluded from projection)", dLocked, dUnlocked)
	}
}

func TestDefinitionDigestDeterministic(t *testing.T) {
	doc := validDefinitionYAML()
	def1 := mustDecodeDefinition(t, doc)
	def2 := mustDecodeDefinition(t, doc)

	d1, err := DefinitionDigest(def1)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	d2, err := DefinitionDigest(def2)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	if d1 != d2 {
		t.Errorf("two decodes of identical bytes produced different digests: %q vs %q", d1, d2)
	}
	if err := ValidateDigest(d1); err != nil {
		t.Errorf("DefinitionDigest() = %q is not a well-formed digest: %v", d1, err)
	}
}

func TestDefinitionV2NormalizationBindsOperandsAndClones(t *testing.T) {
	def := mustDecodeDefinition(t, validDefinitionV2YAML("reproduction:\n  minimum_valid_runs: 2\n"))
	normalized, err := NormalizeDefinition(def)
	if err != nil {
		t.Fatalf("NormalizeDefinition(): %v", err)
	}
	baseDigest, err := DefinitionDigest(def)
	if err != nil {
		t.Fatalf("DefinitionDigest(): %v", err)
	}

	changedClass := def
	changedClass.Class = "request-path-throughput"
	classDigest, err := DefinitionDigest(changedClass)
	if err != nil {
		t.Fatalf("DefinitionDigest(changed class): %v", err)
	}
	changedRule := def
	changedRule.Reproduction = &ReproductionRule{MinimumValidRuns: 3}
	ruleDigest, err := DefinitionDigest(changedRule)
	if err != nil {
		t.Fatalf("DefinitionDigest(changed rule): %v", err)
	}
	if baseDigest == classDigest || baseDigest == ruleDigest {
		t.Fatalf("v2 operand change did not change digest: base=%q class=%q rule=%q", baseDigest, classDigest, ruleDigest)
	}

	// Mutate every slice owned by the source definition. A normalized value is
	// a custody boundary and must retain the exact pre-mutation projection.
	def.Candidates[0].ID = "mutated-candidate"
	def.Evaluator.Argv[0] = "mutated-evaluator"
	def.Fixtures[0].ID = "mutated-fixture"
	def.Decision.Guards[0].ID = "mutated-guard"
	def.ProtectedPaths[0] = "mutated/path"
	if normalized.Candidates[0].ID != "baseline" ||
		normalized.Evaluator.Argv[0] != "./tools/cache-evaluator" ||
		normalized.Fixtures[0].ID != "request-log" ||
		normalized.Decision.Guards[0].ID != "behavioral-equivalence" ||
		normalized.ProtectedPaths[0] != "internal/cache" {
		t.Fatalf("NormalizeDefinition() retained mutable source aliases: %+v", normalized)
	}
}

func TestDefinitionDigestChangesWithRegisteredInput(t *testing.T) {
	base := mustDecodeDefinition(t, validDefinitionYAML())
	baseDigest, err := DefinitionDigest(base)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}

	mutations := []struct {
		name string
		doc  string
	}{
		{"candidate digest changed", mutate(t, "digest: "+baselinePatchDigest, "digest: "+digestOf("f"))},
		{"threshold changed", mutate(t, "    relative: 0.25\n  candidate_separation", "    relative: 0.30\n  candidate_separation")},
		{"rounds changed", mutate(t, "rounds: 10", "rounds: 20")},
		{"environment_policy changed", mutate(t, "environment_policy: local-isolated-v1", "environment_policy: local-isolated-v2")},
	}
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			def := mustDecodeDefinition(t, m.doc)
			digest, err := DefinitionDigest(def)
			if err != nil {
				t.Fatalf("DefinitionDigest() unexpected error: %v", err)
			}
			if digest == baseDigest {
				t.Errorf("DefinitionDigest() unchanged after mutating %s: still %q", m.name, digest)
			}
		})
	}
}

func TestLocked(t *testing.T) {
	unlocked := mustDecodeDefinition(t, validDefinitionYAML())
	locked, err := Locked(unlocked)
	if err != nil {
		t.Fatalf("Locked(unlocked) unexpected error: %v", err)
	}
	if locked {
		t.Errorf("Locked(unlocked) = true, want false")
	}

	digest, err := DefinitionDigest(unlocked)
	if err != nil {
		t.Fatalf("DefinitionDigest() unexpected error: %v", err)
	}
	lockedDoc := validDefinitionYAML() + "lock:\n  definition_digest: " + digest + "\n"
	lockedDef := mustDecodeDefinition(t, lockedDoc)
	locked, err = Locked(lockedDef)
	if err != nil {
		t.Fatalf("Locked(properly locked) unexpected error: %v", err)
	}
	if !locked {
		t.Errorf("Locked(properly locked) = false, want true")
	}
}

func TestLockedTamperedDigestIsError(t *testing.T) {
	// A lock block present with a digest that does NOT match the computed
	// definition digest is a hard error — a tampered or stale lock — never
	// silently "unlocked".
	tamperedDoc := validDefinitionYAML() + "lock:\n  definition_digest: " + digestOf("9") + "\n"
	def := mustDecodeDefinition(t, tamperedDoc)
	if _, err := Locked(def); err == nil {
		t.Errorf("Locked(tampered lock) = nil error, want error")
	}
}

func TestResultDigestDeterministicAndSensitive(t *testing.T) {
	res1, err := DecodeResult([]byte(validResultJSON()))
	if err != nil {
		t.Fatalf("DecodeResult() unexpected error: %v", err)
	}
	res2, err := DecodeResult([]byte(validResultJSON()))
	if err != nil {
		t.Fatalf("DecodeResult() unexpected error: %v", err)
	}
	d1, err := ResultDigest(res1)
	if err != nil {
		t.Fatalf("ResultDigest() unexpected error: %v", err)
	}
	d2, err := ResultDigest(res2)
	if err != nil {
		t.Fatalf("ResultDigest() unexpected error: %v", err)
	}
	if d1 != d2 {
		t.Errorf("two decodes of identical result bytes produced different digests: %q vs %q", d1, d2)
	}

	other, err := DecodeResult([]byte(validInconclusiveResultJSON()))
	if err != nil {
		t.Fatalf("DecodeResult() unexpected error: %v", err)
	}
	dOther, err := ResultDigest(other)
	if err != nil {
		t.Fatalf("ResultDigest() unexpected error: %v", err)
	}
	if dOther == d1 {
		t.Errorf("ResultDigest() for a materially different result equals the original: %q", dOther)
	}
}
