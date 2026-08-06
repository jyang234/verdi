package experiment

import "testing"

// TestCandidatePatchFixtureDigestsMatchContent cross-checks the
// definition_test.go fixture constants against this package's own
// sha256Digest helper, so a future edit to either the content or the
// hardcoded digest constant is caught immediately rather than silently
// producing an internally inconsistent fixture.
func TestCandidatePatchFixtureDigestsMatchContent(t *testing.T) {
	if got := sha256Digest([]byte(baselinePatchContent)); got != baselinePatchDigest {
		t.Errorf("sha256Digest(baselinePatchContent) = %q, want %q", got, baselinePatchDigest)
	}
	if got := sha256Digest([]byte(factsCachePatchContent)); got != factsCachePatchDigest {
		t.Errorf("sha256Digest(factsCachePatchContent) = %q, want %q", got, factsCachePatchDigest)
	}
}

func TestValidateCandidatePatchHappyPath(t *testing.T) {
	def := mustDecodeDefinition(t, validDefinitionYAML())
	if err := ValidateCandidatePatch(def, "baseline", []byte(baselinePatchContent), "experiments/cache-placement-v1"); err != nil {
		t.Fatalf("ValidateCandidatePatch() unexpected error: %v", err)
	}
}

func TestValidateCandidatePatchUnregisteredCandidate(t *testing.T) {
	def := mustDecodeDefinition(t, validDefinitionYAML())
	if err := ValidateCandidatePatch(def, "nonexistent", []byte(baselinePatchContent), ""); err == nil {
		t.Errorf("ValidateCandidatePatch() with an unregistered candidate = nil error, want error")
	}
}

func TestValidateCandidatePatchDigestMismatch(t *testing.T) {
	def := mustDecodeDefinition(t, validDefinitionYAML())
	if err := ValidateCandidatePatch(def, "baseline", []byte("diff --git a/other b/other\n"), ""); err == nil {
		t.Errorf("ValidateCandidatePatch() with mismatched patch bytes = nil error, want error")
	}
}

func TestValidateCandidatePatchUnparseable(t *testing.T) {
	unparseable := []byte("this is not a unified diff at all\n")
	digest := sha256Digest(unparseable)
	// Register the unparseable content's own digest on baseline so the
	// digest check passes and the parse failure is isolated.
	doc := mutate(t, "digest: "+baselinePatchDigest, "digest: "+digest)
	def := mustDecodeDefinition(t, doc)
	if err := ValidateCandidatePatch(def, "baseline", unparseable, ""); err == nil {
		t.Errorf("ValidateCandidatePatch() with an unparseable patch = nil error, want error")
	}
}

func TestValidateCandidatePatchProtectedPaths(t *testing.T) {
	tests := []struct {
		name          string
		patch         string
		experimentDir string
	}{
		{
			name:  "touches explicit protected_paths entry",
			patch: "diff --git a/internal/cache/store.go b/internal/cache/store.go\n",
		},
		{
			name:  "touches explicit protected_paths entry exactly",
			patch: "diff --git a/internal/cache b/internal/cache\n",
		},
		{
			name:  "touches evaluator executable",
			patch: "diff --git a/tools/cache-evaluator b/tools/cache-evaluator\n",
		},
		{
			name:          "touches experiment's own directory",
			patch:         "diff --git a/experiments/cache-placement-v1/experiment.yaml b/experiments/cache-placement-v1/experiment.yaml\n",
			experimentDir: "experiments/cache-placement-v1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			digest := sha256Digest([]byte(tt.patch))
			doc := mutate(t, "digest: "+baselinePatchDigest, "digest: "+digest)
			def := mustDecodeDefinition(t, doc)
			if err := ValidateCandidatePatch(def, "baseline", []byte(tt.patch), tt.experimentDir); err == nil {
				t.Errorf("ValidateCandidatePatch(%s) = nil error, want error", tt.name)
			}
		})
	}
}

func TestValidateCandidatePatchAllowsUnrelatedPath(t *testing.T) {
	patch := "diff --git a/internal/other/store.go b/internal/other/store.go\n"
	digest := sha256Digest([]byte(patch))
	doc := mutate(t, "digest: "+baselinePatchDigest, "digest: "+digest)
	def := mustDecodeDefinition(t, doc)
	if err := ValidateCandidatePatch(def, "baseline", []byte(patch), "experiments/cache-placement-v1"); err != nil {
		t.Errorf("ValidateCandidatePatch() for an unrelated path = %v, want nil", err)
	}
}

func TestValidateCandidatePatchNearMissDoesNotFalsePositive(t *testing.T) {
	// "internal/cache2" must NOT be treated as touching the registered
	// "internal/cache" protected prefix — segment-boundary matching only.
	patch := "diff --git a/internal/cache2/store.go b/internal/cache2/store.go\n"
	digest := sha256Digest([]byte(patch))
	doc := mutate(t, "digest: "+baselinePatchDigest, "digest: "+digest)
	def := mustDecodeDefinition(t, doc)
	if err := ValidateCandidatePatch(def, "baseline", []byte(patch), ""); err != nil {
		t.Errorf("ValidateCandidatePatch() for a near-miss path = %v, want nil (segment-boundary matching only)", err)
	}
}

func TestValidateCandidatePatchRenameTouchesProtected(t *testing.T) {
	patch := "diff --git a/internal/other/old.go b/internal/cache/new.go\n" +
		"similarity index 100%\n" +
		"rename from internal/other/old.go\n" +
		"rename to internal/cache/new.go\n"
	digest := sha256Digest([]byte(patch))
	doc := mutate(t, "digest: "+baselinePatchDigest, "digest: "+digest)
	def := mustDecodeDefinition(t, doc)
	if err := ValidateCandidatePatch(def, "baseline", []byte(patch), ""); err == nil {
		t.Errorf("ValidateCandidatePatch() with a rename into a protected path = nil error, want error")
	}
}

func TestValidateCandidatePatchDevNullAddedFile(t *testing.T) {
	patch := "diff --git a/internal/other/new.go b/internal/other/new.go\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/internal/other/new.go\n"
	digest := sha256Digest([]byte(patch))
	doc := mutate(t, "digest: "+baselinePatchDigest, "digest: "+digest)
	def := mustDecodeDefinition(t, doc)
	if err := ValidateCandidatePatch(def, "baseline", []byte(patch), ""); err != nil {
		t.Errorf("ValidateCandidatePatch() for a /dev/null-added unrelated file = %v, want nil", err)
	}
}

func TestValidateCandidatePatchDevNullAddedFileTouchesProtected(t *testing.T) {
	patch := "diff --git a/internal/cache/new.go b/internal/cache/new.go\n" +
		"new file mode 100644\n" +
		"--- /dev/null\n" +
		"+++ b/internal/cache/new.go\n"
	digest := sha256Digest([]byte(patch))
	doc := mutate(t, "digest: "+baselinePatchDigest, "digest: "+digest)
	def := mustDecodeDefinition(t, doc)
	if err := ValidateCandidatePatch(def, "baseline", []byte(patch), ""); err == nil {
		t.Errorf("ValidateCandidatePatch() with a /dev/null-added file under a protected path = nil error, want error")
	}
}
