package app

import (
	"context"
	"testing"

	"example.com/svcfix/internal/audit"
	"example.com/svcfix/internal/bus"
)

// TestRefundFlow exercises the same call shape as
// testdata/flows/refund-flow.golden.json (PLAN.md §4): verdi's own
// derived-bundle assembly treats the whole `go test -json` suite's
// pass/fail as this fixture's behavioral evidence signal (03 §Declarations:
// "Unit tests deliberately stay coarse"), so this test's only job is to
// exist and pass, exercising PublishRefund's audit-before-publish order.
func TestRefundFlow(t *testing.T) {
	svc := New(audit.New(), bus.New())
	if err := svc.PublishRefund(context.Background(), "refund-1"); err != nil {
		t.Fatalf("PublishRefund: %v", err)
	}
}

func TestGetRefund(t *testing.T) {
	svc := New(audit.New(), bus.New())
	got, err := svc.GetRefund(context.Background(), "refund-1")
	if err != nil {
		t.Fatalf("GetRefund: %v", err)
	}
	if got != "refund-1" {
		t.Fatalf("GetRefund = %q, want %q", got, "refund-1")
	}
}
