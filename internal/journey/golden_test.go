package journey

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// goldenRecord returns a fully populated, valid Record exercising every
// v1 field, including the fields added after the initial commit: an
// Action with non-empty Authority, a managed worktree fact, and an
// authenticated-attribution blocker owner. Its Canonical() output is the
// digest ratchet in testdata/canonical-record.json — any tag rename,
// field addition/removal, or ordering-rule change that is not also
// reflected there fails TestCanonicalGoldenFixture.
func goldenRecord(t *testing.T) Record {
	t.Helper()

	id, err := governanceprincipal.CanonicalPrincipalID("github", "user-123")
	if err != nil {
		t.Fatalf("CanonicalPrincipalID: %v", err)
	}
	attr, err := governanceprincipal.NewPrincipalAttribution(id)
	if err != nil {
		t.Fatalf("NewPrincipalAttribution: %v", err)
	}

	r := validRecord(t)
	r.Repository.Worktree = WorktreeFact{Managed: true, Name: "glg-journey-projection"}
	r.Blockers.Current[0].Owner = Owner{Declared: "Jane Doe", Attribution: attr}
	r.Blockers.Current = append(r.Blockers.Current, Blocker{
		ID:                "obligation-quality/ac-2/runtime",
		Reason:            ReasonObligationDesignUnresolved,
		Class:             ClassMechanical,
		Witnesses:         []string{".verdi/obligations/example/ac-2--runtime.md: unresolved-design-debt"},
		Owner:             Owner{Declared: "Jane Doe", Attribution: attr},
		ClearingCondition: "elaborate the obligation quality for ac-2/runtime",
		Transition:        "build:start",
	})
	r.Principals.ProfileAdopted = true
	r.Principals.SelectedProfileID = "solo-default"
	r.Principals.SelectedProfileDigest = testSelectedProfileDigest
	r.Principals.Disclosures = []string{profileResolutionUnprovenDisclosure}
	r.Actions.Safe[0].Authority = []RequiredRole{
		{Transition: "close", Obligation: "attestation/countersign", Count: 1, Resolution: "authenticated"},
	}
	r.Disclosures = []string{}
	return r
}

func TestCanonicalGoldenFixture(t *testing.T) {
	got, err := Canonical(goldenRecord(t))
	if err != nil {
		t.Fatalf("Canonical(goldenRecord()): %v", err)
	}

	want, err := os.ReadFile("testdata/canonical-record.json")
	if err != nil {
		t.Fatalf("reading testdata/canonical-record.json: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("Canonical(goldenRecord()) drifted from testdata/canonical-record.json (schema/tag/ordering change not reflected in the golden fixture):\ngot:\n%s\nwant:\n%s", got, want)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	digest, _ := decoded["digest"].(string)
	if !digestRe.MatchString(digest) {
		t.Errorf("golden fixture digest %q does not match sha256:<hex> form", digest)
	}
}
