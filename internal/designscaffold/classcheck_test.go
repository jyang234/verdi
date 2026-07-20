package designscaffold

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestCheckClass_Agrees proves the happy path: a decoded scaffold whose
// own Class matches the class it was resolved/requested under passes
// silently (nil error) — every real render (the canonical feature.md/
// story.md templates, or a faithful store override) takes this path.
func TestCheckClass_Agrees(t *testing.T) {
	for _, class := range []artifact.SpecClass{artifact.ClassFeature, artifact.ClassStory} {
		spec := &artifact.SpecFrontmatter{Class: class}
		if err := CheckClass(spec, class); err != nil {
			t.Fatalf("CheckClass(class=%s, want=%s) = %v, want nil", class, class, err)
		}
	}
}

// TestCheckClass_Disagrees is K1's own kernel proof: a class's resolved
// template is DATA (model.Class.Template — a store override, or a
// misconfigured model.yaml binding one class's Template filename to
// another class's template file). Neither SplitFrontmatter nor DecodeSpec
// alone catches a template that renders a well-formed spec of the WRONG
// class — both accept any legal class. CheckClass must fail closed,
// naming BOTH the class the content actually declares and the class it
// was expected to declare, so every call site's own wrapping error names
// all three facts an operator needs (template file, declared/requested
// class, rendered class).
func TestCheckClass_Disagrees(t *testing.T) {
	spec := &artifact.SpecFrontmatter{Class: artifact.ClassFeature}
	err := CheckClass(spec, artifact.ClassStory)
	if err == nil {
		t.Fatal("CheckClass(class=feature, want=story) = nil, want an error")
	}
	if !strings.Contains(err.Error(), `"feature"`) {
		t.Fatalf("CheckClass error = %q, want it to name the rendered class %q", err.Error(), "feature")
	}
	if !strings.Contains(err.Error(), `"story"`) {
		t.Fatalf("CheckClass error = %q, want it to name the wanted class %q", err.Error(), "story")
	}
}
