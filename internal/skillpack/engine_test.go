package skillpack

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specdoc"
)

func TestEngineDigestIsStableAndBindsDocumentEngine(t *testing.T) {
	a, b := EngineDigest(), EngineDigest()
	if a != b || !strings.HasPrefix(a, "sha256:") {
		t.Fatalf("EngineDigest unstable or unprefixed: %q %q", a, b)
	}
	if got := engineDescriptorDigest(engineDescriptor{ID: engineID, Version: engineVersion, Skills: Skills(), Hosts: hostStrings(Hosts()), Document: "sha256:other"}); got == a {
		t.Fatal("EngineDigest must change when the document engine digest changes")
	}
	// task-1-review.md finding 9: the two assertions above prove
	// EngineDigest() is sensitive to the Document field, not that it
	// actually reads specdoc.EngineDigest() — a mutant that froze
	// today's specdoc value as a literal would still pass both. Assert
	// EngineDigest() equals the descriptor digest built directly from
	// specdoc.EngineDigest(), so a decoupled (frozen-literal) mutant fails.
	if want := engineDescriptorDigest(engineDescriptor{ID: engineID, Version: engineVersion, Skills: Skills(), Hosts: hostStrings(Hosts()), Document: specdoc.EngineDigest()}); want != a {
		t.Fatalf("EngineDigest() = %q, want the descriptor digest built from specdoc.EngineDigest() = %q", a, want)
	}
}

func TestTemplateDigestPerSkill(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range Skills() {
		d, err := TemplateDigest(s)
		if err != nil {
			t.Fatalf("TemplateDigest(%s): %v", s, err)
		}
		if !strings.HasPrefix(d, "sha256:") || seen[d] {
			t.Fatalf("TemplateDigest(%s) = %q: unprefixed or duplicate", s, d)
		}
		seen[d] = true
	}
	if _, err := TemplateDigest("deploy"); err == nil {
		t.Fatal("unknown skill must refuse")
	}
}
