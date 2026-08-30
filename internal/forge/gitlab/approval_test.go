package gitlab

import "testing"

func TestDerivedApprovalID(t *testing.T) {
	first, err := derivedApprovalID("42", "9", "7", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "2026-08-26T15:00:00Z")
	if err != nil {
		t.Fatalf("derivedApprovalID: %v", err)
	}
	second, err := derivedApprovalID("42", "9", "7", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "2026-08-26T15:00:00Z")
	if err != nil {
		t.Fatalf("derivedApprovalID second: %v", err)
	}
	changed, err := derivedApprovalID("42", "9", "7", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "2026-08-26T15:00:00Z")
	if err != nil {
		t.Fatalf("derivedApprovalID changed: %v", err)
	}
	if first != second || first == changed {
		t.Fatalf("ids: first=%q second=%q changed=%q", first, second, changed)
	}
}
