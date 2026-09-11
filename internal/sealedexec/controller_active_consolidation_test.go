package sealedexec

import (
	"github.com/jyang234/verdi/internal/contextevent"
	"testing"
)

// These cases isolate checks that broad one-field mutations can mask with an
// earlier acknowledgment-prefix failure. Every base is accepted before mutation.
func TestPublicControllerConsolidationActiveChecks(t *testing.T) {
	makeBase := func(t *testing.T, kind string) ControllerResult {
		r := controllerResultFixture(t, 1, ControllerOperationRecorderCheckpoint)
		c := &r.RecorderCheckpoint.Checkpoint
		terminal := c.Revisions[len(c.Revisions)-1]
		bridge := &contextevent.PriorRevision{ManifestRevision: terminal.ManifestRevision, ManifestDigest: terminal.ManifestDigest, EventRoot: terminal.EventRoot, TerminalSourceSequence: terminal.TerminalSourceSequence, TerminalGlobalSequence: terminal.TerminalGlobalSequence}
		a := &ActiveRevision{Revision: terminal.ManifestRevision + 1, ManifestDigest: testDigest("active-manifest"), NextSourceSequence: 1, EventAcks: []contextevent.EventAck{}, PriorRevision: bridge, LastGlobalSequence: terminal.TerminalGlobalSequence}
		c.ActiveRevision = a
		switch kind {
		case "complete":
		case "omitted":
			c.Revisions = []contextevent.Revision{}
			c.EventChainRoot = ""
			c.TerminalSourceSequence = 0
			c.TerminalGlobalSequence = 0
		case "pristine":
			c.Revisions = []contextevent.Revision{}
			c.EventChainRoot = ""
			c.TerminalSourceSequence = 0
			c.TerminalGlobalSequence = 0
			a.PriorRevision = nil
			a.LastGlobalSequence = 0
		case "events":
			a.PriorRevision = nil
			a.NextSourceSequence = 2
			a.PriorEventDigest = testDigest("active-event")
			a.LastGlobalSequence = terminal.TerminalGlobalSequence + 1
			a.EventAcks = []contextevent.EventAck{controllerActiveAck(a.Revision, 1, a.LastGlobalSequence, a.PriorEventDigest)}
		default:
			t.Fatal(kind)
		}
		return r
	}
	cases := []struct {
		name, base string
		mutate     func(*RecorderCheckpoint)
	}{
		{"invalid-ack", "events", func(c *RecorderCheckpoint) { c.ActiveRevision.EventAcks[0].Schema = "wrong" }},
		{"first-ack-not-after-complete", "events", func(c *RecorderCheckpoint) {
			a := c.ActiveRevision
			a.EventAcks[0].GlobalSequence = c.TerminalGlobalSequence
			a.LastGlobalSequence = c.TerminalGlobalSequence
		}},
		{"revision-gap", "complete", func(c *RecorderCheckpoint) { c.ActiveRevision.Revision++ }},
		{"pristine-global-advance", "pristine", func(c *RecorderCheckpoint) { c.ActiveRevision.LastGlobalSequence++ }},
		{"omitted-bridge-mismatch", "omitted", func(c *RecorderCheckpoint) { c.ActiveRevision.PriorRevision.ManifestRevision++ }},
		{"sequence-one-global-advance", "complete", func(c *RecorderCheckpoint) { c.ActiveRevision.LastGlobalSequence++ }},
		{"later-sequence-retains-bridge", "events", func(c *RecorderCheckpoint) {
			c.ActiveRevision.PriorRevision = &contextevent.PriorRevision{ManifestRevision: 0, ManifestDigest: testDigest("prior"), EventRoot: testDigest("root"), TerminalSourceSequence: 1, TerminalGlobalSequence: 1}
		}},
		{"bridge-zero-terminal", "omitted", func(c *RecorderCheckpoint) { c.ActiveRevision.PriorRevision.TerminalSourceSequence = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := makeBase(t, tc.base)
			if _, err := EncodeControllerResult(r); err != nil {
				t.Fatalf("accepted control: %v", err)
			}
			tc.mutate(&r.RecorderCheckpoint.Checkpoint)
			if _, err := EncodeControllerResult(r); err == nil {
				t.Fatal("accepted invalid active checkpoint")
			}
		})
	}
}

func TestPublicControllerConsolidationReceiptEventKind(t *testing.T) {
	call := controllerCallFixture(t, 1, ControllerOperationAppendReceipt)
	if _, err := EncodeControllerCall(call); err != nil {
		t.Fatal(err)
	}
	event, _ := controllerEventFixture(t, validExecutionRequest(t, ActionStart))
	call.AppendReceipt.Append.Event = event
	if _, err := EncodeControllerCall(call); err == nil {
		t.Fatal("accepted a valid non-receipt event as the appended receipt event")
	}
}
