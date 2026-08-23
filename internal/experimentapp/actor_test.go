package experimentapp

import "testing"

func TestActorSealRejectsMutation(t *testing.T) {
	actor, err := NewDelegatedAgent("codex", "session-1")
	if err != nil {
		t.Fatalf("NewDelegatedAgent() error = %v", err)
	}
	if err := actor.validate(); err != nil {
		t.Fatalf("fresh actor validate() error = %v", err)
	}
	actor.session = "mutated"
	if err := actor.validate(); err == nil {
		t.Fatal("mutated actor validate() succeeded")
	}
}
