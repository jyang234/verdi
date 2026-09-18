package skillpack

import (
	"strings"
	"testing"
)

func TestEngineDigestIsStableAndBindsDocumentEngine(t *testing.T) {
	a, b := EngineDigest(), EngineDigest()
	if a != b || !strings.HasPrefix(a, "sha256:") {
		t.Fatalf("EngineDigest unstable or unprefixed: %q %q", a, b)
	}
	if got := engineDescriptorDigest(engineDescriptor{ID: engineID, Version: engineVersion, Skills: Skills(), Hosts: hostStrings(Hosts()), Document: "sha256:other"}); got == a {
		t.Fatal("EngineDigest must change when the document engine digest changes")
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
