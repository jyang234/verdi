package constitutionimpact

import (
	"context"
	"strings"
	"testing"
)

func TestBuildPlanUsesSortedAcceptedProposedUnionAndRetainsRemovedRows(t *testing.T) {
	removed := testConsumer("spec/removed", "local")
	shared := testConsumer("spec/shared", "local")
	plan := testPlan(t, []Consumer{removed, shared}, []Consumer{shared}, testChangedLayer())

	consumers := plan.Consumers()
	if len(consumers) != 2 {
		t.Fatalf("union size = %d, want 2", len(consumers))
	}
	first, _ := consumers[0].Identity()
	second, _ := consumers[1].Identity()
	if first >= second {
		t.Fatalf("union not sorted: %s >= %s", first, second)
	}
	foundRemoved := false
	for _, consumer := range consumers {
		if consumer.Request.Spec == "spec/removed" {
			foundRemoved = true
		}
	}
	if !foundRemoved {
		t.Fatal("accepted-only removed consumer was lost from the union")
	}

	// Returned consumers are deep copies, including nested request state.
	consumers[0].GovernedOperations[0] = "mutated"
	consumers[0].Request.Scope.Phases[0] = "review"
	again := plan.Consumers()
	if again[0].GovernedOperations[0] != "make-verify" || again[0].Request.Scope.Phases[0] != "build" {
		t.Fatal("Plan.Consumers returned aliases into the sealed plan")
	}
}

func TestBuildPlanValidatesEachInventoryAgainstItsOwnCatalog(t *testing.T) {
	acceptedConstitution := []byte(strings.Replace(string(testConstitutionBytes(t)),
		"action: [make-verify]", "action: [legacy-action, make-verify]", 1))
	proposedConstitution := testConstitutionBytes(t)
	legacy := testConsumer("spec/legacy", "local")
	legacy.GovernedOperations = []string{"legacy-action"}
	current := testConsumer("spec/current", "local")

	plan, err := BuildPlan(context.Background(),
		ExactTree{Commit: testAcceptedCommit, Tree: testAcceptedTree, FS: testTree(testInventoryBytes(t, legacy), acceptedConstitution)},
		ExactTree{Commit: testProposedCommit, Tree: testProposedTree, FS: testTree(testInventoryBytes(t, current), proposedConstitution)},
		testChangedLayer(),
	)
	if err != nil {
		t.Fatalf("BuildPlan rejected accepted-local legacy operation: %v", err)
	}
	if len(plan.Consumers()) != 2 {
		t.Fatalf("union size = %d, want legacy and current consumers", len(plan.Consumers()))
	}

	_, err = BuildPlan(context.Background(),
		ExactTree{Commit: testAcceptedCommit, Tree: testAcceptedTree, FS: testTree(testInventoryBytes(t, current), acceptedConstitution)},
		ExactTree{Commit: testProposedCommit, Tree: testProposedTree, FS: testTree(testInventoryBytes(t, legacy), proposedConstitution)},
		testChangedLayer(),
	)
	if err == nil || !strings.Contains(err.Error(), "proposed inventory") || !strings.Contains(err.Error(), "legacy-action") {
		t.Fatalf("BuildPlan invalid proposed catalog operation error = %v", err)
	}
}

func TestBuildPlanClosedInventoryStates(t *testing.T) {
	constitution := testConstitutionBytes(t)
	t.Run("missing inventories are unproven", func(t *testing.T) {
		plan, err := BuildPlan(context.Background(),
			ExactTree{Commit: testAcceptedCommit, Tree: testAcceptedTree, FS: testTree(nil, constitution)},
			ExactTree{Commit: testProposedCommit, Tree: testProposedTree, FS: testTree(nil, constitution)},
			testChangedLayer(),
		)
		if err != nil {
			t.Fatal(err)
		}
		coverage := plan.Complete(nil, nil)
		if coverage.State != StateDisclosedUnproven || !hasReason(coverage.Reasons, ReasonAcceptedInventoryMissing, InventoryPath) || !hasReason(coverage.Reasons, ReasonProposedInventoryMissing, InventoryPath) {
			t.Fatalf("coverage = %+v", coverage)
		}
	})

	t.Run("unavailable exact tree is unproven", func(t *testing.T) {
		plan, err := BuildPlan(context.Background(),
			ExactTree{Commit: testAcceptedCommit, Tree: testAcceptedTree},
			ExactTree{Commit: testProposedCommit, Tree: testProposedTree},
			nil,
		)
		if err != nil {
			t.Fatal(err)
		}
		coverage := plan.Complete(nil, nil)
		if coverage.State != StateDisclosedUnproven || !hasReason(coverage.Reasons, ReasonAcceptedTreeUnavailable, "accepted") {
			t.Fatalf("coverage = %+v", coverage)
		}
	})

	t.Run("changed empty universe is distinct from no layer change", func(t *testing.T) {
		changed := testPlan(t, nilToEmptyConsumers(), nilToEmptyConsumers(), testChangedLayer()).Complete(nil, nil)
		if changed.State != StateDisclosedUnproven || !hasReason(changed.Reasons, ReasonConsumerUniverseEmpty, InventoryPath) {
			t.Fatalf("changed coverage = %+v", changed)
		}
		unchanged := testPlan(t, nilToEmptyConsumers(), nilToEmptyConsumers(), nil).Complete(nil, nil)
		if unchanged.State != StateProven || len(unchanged.Reasons) != 0 {
			t.Fatalf("unchanged coverage = %+v", unchanged)
		}
	})

	t.Run("malformed present inventory is operational", func(t *testing.T) {
		_, err := BuildPlan(context.Background(),
			ExactTree{Commit: testAcceptedCommit, Tree: testAcceptedTree, FS: testTree([]byte("{}\n"), constitution)},
			ExactTree{Commit: testProposedCommit, Tree: testProposedTree, FS: testTree(testInventoryBytes(t, nilToEmptyConsumers()...), constitution)},
			nil,
		)
		if err == nil || !strings.Contains(err.Error(), "loading accepted inventory") {
			t.Fatalf("BuildPlan malformed inventory error = %v", err)
		}
	})
}

func TestBuildPlanDuplicateInventoryIsViolatedAndEnvironmentIsIdentityBound(t *testing.T) {
	local := testConsumer("spec/shared", "local")
	production := testConsumer("spec/shared", "production")
	plan := testPlan(t, []Consumer{local, local, production}, []Consumer{local, production}, testChangedLayer())
	if len(plan.Consumers()) != 2 {
		t.Fatalf("union size = %d, want distinct local and production entries", len(plan.Consumers()))
	}
	result := resultForPlan(t, plan, true)
	evaluations := make([]Evaluation, 0, 2)
	for _, consumer := range plan.Consumers() {
		identity, _ := consumer.Identity()
		evaluations = append(evaluations, Evaluation{ConsumerIdentity: identity, Consumer: consumer, Result: result})
	}
	coverage := plan.Complete(evaluations, nil)
	if coverage.State != StateViolatedWithWitness || !hasReasonCode(coverage.Reasons, ReasonInventoryDuplicate) {
		t.Fatalf("coverage = %+v", coverage)
	}
}

func nilToEmptyConsumers() []Consumer { return []Consumer{} }

func hasReasonCode(reasons []Reason, code ReasonCode) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}
