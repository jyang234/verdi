package policyartifact_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// TestRequiredInputNamesMatchCompiler pins the required-input witness's
// three input names to the compiler's own required-input vocabulary
// (contextcompile's RequiredInput* constants, compiler design §6). The
// strings live in both packages only because contextcompile imports
// policyartifact; this external test is the drift guard that keeps the
// two homes one vocabulary.
func TestRequiredInputNamesMatchCompiler(t *testing.T) {
	want := []string{
		contextcompile.RequiredInputBuilderReceipt,
		contextcompile.RequiredInputEvidenceBundle,
		contextcompile.RequiredInputResultDiff,
	}
	sort.Strings(want)
	if got := policyartifact.RequiredInputNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("policyartifact.RequiredInputNames() = %v, contextcompile's review inputs = %v", got, want)
	}
}
