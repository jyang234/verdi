package sealedexec

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

func TestExecutionHandbackService_Behavioral(t *testing.T) {
	t.Run("authoritative clean descendant fast-forwards, durably acknowledges, then releases", func(t *testing.T) {
		fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
		outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
		if err != nil {
			t.Fatalf("Apply: %v", err)
		}
		wantOrder := []string{
			"observe-runway", "observe-child", "is-ancestor", "diff",
			"fast-forward-only", "observe-runway", "persist-handback", "release",
		}
		if !reflect.DeepEqual(fixture.calls, wantOrder) {
			t.Fatalf("handback order = %q, want %q", fixture.calls, wantOrder)
		}
		if outcome.ExitCode != 0 || !outcome.Released || outcome.Handback == nil || outcome.Quarantine != nil || outcome.Abort != nil {
			t.Fatalf("successful outcome = %#v", outcome)
		}
		if !bytes.Equal(outcome.ResultBytes, fixture.completion.ResultBytes) {
			t.Fatal("successful handback did not preserve exact public result bytes")
		}
		record := *outcome.Handback
		if record.Input != (GitIdentity{Commit: fixture.request.InputCommit, Tree: fixture.request.InputTree}) ||
			record.Output != (GitIdentity{Commit: fixture.completion.Result.OutputCommit, Tree: fixture.completion.Result.OutputTree}) ||
			record.PreRunway.Head != fixture.request.InputCommit || record.PostRunway.Head != fixture.completion.Result.OutputCommit ||
			record.Receipt.Digest != fixture.completion.Receipt.Digest || record.Receipt.EventAck != fixture.completion.ReceiptEventAck {
			t.Fatalf("handback record identity mismatch: %#v", record)
		}
		if err := ValidateHandbackAck(record, outcome.ControlAck); err != nil {
			t.Fatalf("durable handback ack: %v", err)
		}
	})

	t.Run("advisory result quarantines without repository mutation or release", func(t *testing.T) {
		fixture := newHandbackFixture(t, contextevent.AuthorityAdvisory)
		outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
		assertHandbackFailure(t, fixture, outcome, err, ErrVerdict, QuarantineNonAuthoritative)
		if got, want := fixture.calls, []string{"persist-quarantine"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("advisory calls = %q, want %q", got, want)
		}
	})

	t.Run("canonical advisory completion rejects authoritative convenience-field tampering before side effects", func(t *testing.T) {
		fixture := newHandbackFixture(t, contextevent.AuthorityAdvisory)
		fixture.completion.Authority = contextevent.AuthorityAuthoritative
		outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
		if !errors.Is(err, ErrOperational) || outcome.ExitCode != 2 || outcome.Released || outcome.Handback != nil || outcome.Quarantine != nil {
			t.Fatalf("tampered completion outcome/error = %#v / %v", outcome, err)
		}
		if len(fixture.calls) != 0 {
			t.Fatalf("tampered completion reached side effects: %q", fixture.calls)
		}
	})

	precheckCases := []struct {
		name      string
		reason    QuarantineReason
		class     error
		configure func(*handbackFixture)
		wantCalls []string
	}{
		{
			name: "dirty runway", reason: QuarantineRunwayDirty, class: ErrVerdict,
			configure: func(f *handbackFixture) { f.repository.runway.Clean = false },
			wantCalls: []string{"observe-runway", "persist-quarantine"},
		},
		{
			name: "moved runway", reason: QuarantineRunwayMoved, class: ErrVerdict,
			configure: func(f *handbackFixture) {
				f.repository.runway.Commit, f.repository.runway.Tree = testSHA2, testTree2
			},
			wantCalls: []string{"observe-runway", "persist-quarantine"},
		},
		{
			name: "dirty child", reason: QuarantineChildDirty, class: ErrVerdict,
			configure: func(f *handbackFixture) { f.repository.child.Clean = false },
			wantCalls: []string{"observe-runway", "observe-child", "persist-quarantine"},
		},
		{
			name: "non-descendant", reason: QuarantineNonDescendant, class: ErrVerdict,
			configure: func(f *handbackFixture) { f.repository.descendant = false },
			wantCalls: []string{"observe-runway", "observe-child", "is-ancestor", "persist-quarantine"},
		},
		{
			name: "postcheck mismatch", reason: QuarantinePostVerificationMismatch, class: ErrVerdict,
			configure: func(f *handbackFixture) { f.repository.post.Tree = testTree1 },
			wantCalls: []string{"observe-runway", "observe-child", "is-ancestor", "diff", "fast-forward-only", "observe-runway", "persist-quarantine"},
		},
	}

	t.Run("repository proof failures durably quarantine the exact valid prefix", func(t *testing.T) {
		cases := []struct {
			name        string
			reason      QuarantineReason
			class       error
			configure   func(*handbackFixture)
			wantCalls   []string
			wantFastFwd FastForwardState
		}{
			{name: "runway observe inability", reason: QuarantineRepositoryVerificationFailed, class: ErrOperational, configure: func(f *handbackFixture) {
				f.repository.runwayObserveErr = errors.New("runway unavailable")
			}, wantCalls: []string{"observe-runway", "persist-quarantine"}, wantFastFwd: FastForwardNotAttempted},
			{name: "runway strict-state validation failure", reason: QuarantineRepositoryVerificationFailed, class: ErrOperational, configure: func(f *handbackFixture) {
				f.repository.runway.Path = "/wrong/runway"
			}, wantCalls: []string{"observe-runway", "persist-quarantine"}, wantFastFwd: FastForwardNotAttempted},
			{name: "child observe inability", reason: QuarantineRepositoryVerificationFailed, class: ErrOperational, configure: func(f *handbackFixture) {
				f.repository.childObserveErr = errors.New("child unavailable")
			}, wantCalls: []string{"observe-runway", "observe-child", "persist-quarantine"}, wantFastFwd: FastForwardNotAttempted},
			{name: "child strict-state validation failure", reason: QuarantineRepositoryVerificationFailed, class: ErrOperational, configure: func(f *handbackFixture) {
				f.repository.child.Commit = "not-a-git-object"
			}, wantCalls: []string{"observe-runway", "observe-child", "persist-quarantine"}, wantFastFwd: FastForwardNotAttempted},
			{name: "clean child contradicts durable output", reason: QuarantineChildOutputMismatch, class: ErrVerdict, configure: func(f *handbackFixture) {
				f.repository.child.Commit, f.repository.child.Tree = f.request.InputCommit, f.request.InputTree
			}, wantCalls: []string{"observe-runway", "observe-child", "persist-quarantine"}, wantFastFwd: FastForwardNotAttempted},
			{name: "ancestry inability", reason: QuarantineRepositoryVerificationFailed, class: ErrOperational, configure: func(f *handbackFixture) {
				f.repository.ancestorErr = errors.New("ancestry unavailable")
			}, wantCalls: []string{"observe-runway", "observe-child", "is-ancestor", "persist-quarantine"}, wantFastFwd: FastForwardNotAttempted},
			{name: "diff inability", reason: QuarantineRepositoryVerificationFailed, class: ErrOperational, configure: func(f *handbackFixture) {
				f.repository.diffErr = errors.New("diff unavailable")
			}, wantCalls: []string{"observe-runway", "observe-child", "is-ancestor", "diff", "persist-quarantine"}, wantFastFwd: FastForwardNotAttempted},
			{name: "post-fast-forward observe inability", reason: QuarantineRepositoryVerificationFailed, class: ErrOperational, configure: func(f *handbackFixture) {
				f.repository.postObserveErr = errors.New("post-runway unavailable")
			}, wantCalls: []string{"observe-runway", "observe-child", "is-ancestor", "diff", "fast-forward-only", "observe-runway", "persist-quarantine"}, wantFastFwd: FastForwardSucceeded},
			{name: "post-fast-forward strict-state validation failure", reason: QuarantineRepositoryVerificationFailed, class: ErrOperational, configure: func(f *handbackFixture) {
				f.repository.post.Tree = "not-a-git-object"
			}, wantCalls: []string{"observe-runway", "observe-child", "is-ancestor", "diff", "fast-forward-only", "observe-runway", "persist-quarantine"}, wantFastFwd: FastForwardSucceeded},
		}
		for _, test := range cases {
			test := test
			t.Run(test.name, func(t *testing.T) {
				fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
				test.configure(fixture)
				outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
				assertHandbackFailure(t, fixture, outcome, err, test.class, test.reason)
				if !reflect.DeepEqual(fixture.calls, test.wantCalls) {
					t.Fatalf("calls = %q, want %q", fixture.calls, test.wantCalls)
				}
				if outcome.Quarantine.Observed.FastForward != test.wantFastFwd {
					t.Fatalf("fast_forward = %q, want %q", outcome.Quarantine.Observed.FastForward, test.wantFastFwd)
				}
			})
		}

		t.Run("quarantine persistence failure is the primary operational outcome", func(t *testing.T) {
			fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
			fixture.repository.runwayObserveErr = errors.New("runway unavailable")
			fixture.control.quarantineErr = errors.New("control persistence unavailable")
			outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
			if !errors.Is(err, ErrOperational) || outcome.ExitCode != 2 || outcome.Quarantine == nil || outcome.ControlAck.Digest != "" || outcome.Released {
				t.Fatalf("persistence-failure outcome/error = %#v / %v", outcome, err)
			}
			if got, want := fixture.calls, []string{"observe-runway", "persist-quarantine"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("persistence-failure calls = %q, want %q", got, want)
			}
		})
	})

	t.Run("attempt-aware fast-forward failures use fresh runway facts", func(t *testing.T) {
		cases := []struct {
			name        string
			result      gitx.FastForwardResult
			post        RepositoryState
			postErr     error
			reason      QuarantineReason
			class       error
			wantFastFwd FastForwardState
			wantPost    RepositoryObservationState
		}{
			{name: "late dirty blocks merge", result: gitx.FastForwardResult{Category: gitx.FastForwardRunwayDirty}, post: RepositoryState{Clean: false}, reason: QuarantineRunwayDirty, class: ErrVerdict, wantFastFwd: FastForwardNotAttempted, wantPost: RepositoryObserved},
			{name: "status inspection failure with clean input", result: gitx.FastForwardResult{Category: gitx.FastForwardStatusFailed}, post: RepositoryState{Clean: true}, reason: QuarantineRepositoryVerificationFailed, class: ErrOperational, wantFastFwd: FastForwardNotAttempted, wantPost: RepositoryUnproven},
			{name: "attempted merge leaves dirty runway", result: gitx.FastForwardResult{Category: gitx.FastForwardMergeFailed, Attempted: true}, post: RepositoryState{Clean: false}, reason: QuarantineRunwayDirty, class: ErrVerdict, wantFastFwd: FastForwardFailed, wantPost: RepositoryObserved},
			{name: "attempted merge leaves clean moved runway", result: gitx.FastForwardResult{Category: gitx.FastForwardMergeFailed, Attempted: true}, post: RepositoryState{Commit: testSHA2, Tree: testTree1, Clean: true}, reason: QuarantineRunwayMoved, class: ErrVerdict, wantFastFwd: FastForwardFailed, wantPost: RepositoryObserved},
			{name: "attempted merge leaves clean input", result: gitx.FastForwardResult{Category: gitx.FastForwardMergeFailed, Attempted: true}, post: RepositoryState{Clean: true}, reason: QuarantineFastForwardFailed, class: ErrOperational, wantFastFwd: FastForwardFailed, wantPost: RepositoryObserved},
			{name: "attempted merge reaches exact output but reports failure", result: gitx.FastForwardResult{Category: gitx.FastForwardMergeFailed, Attempted: true}, post: RepositoryState{Commit: testSHA2, Tree: testTree2, Clean: true}, reason: QuarantineFastForwardFailed, class: ErrOperational, wantFastFwd: FastForwardFailed, wantPost: RepositoryObserved},
			{name: "attempted merge cannot be reobserved", result: gitx.FastForwardResult{Category: gitx.FastForwardMergeFailed, Attempted: true}, postErr: errors.New("post-runway unavailable"), reason: QuarantineRepositoryVerificationFailed, class: ErrOperational, wantFastFwd: FastForwardFailed, wantPost: RepositoryUnproven},
		}
		for _, test := range cases {
			test := test
			t.Run(test.name, func(t *testing.T) {
				fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
				fixture.repository.fastForwardResult = test.result
				fixture.repository.fastForwardErr = errors.New("guarded fast-forward failed")
				if test.post.Path == "" {
					test.post.Path = fixture.request.ATCRunway
				}
				if test.post.Commit == "" {
					test.post.Commit = fixture.request.InputCommit
				}
				if test.post.Tree == "" {
					test.post.Tree = fixture.request.InputTree
				}
				fixture.repository.post = test.post
				fixture.repository.postObserveErr = test.postErr
				outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
				assertHandbackFailure(t, fixture, outcome, err, test.class, test.reason)
				if outcome.Quarantine.Observed.FastForward != test.wantFastFwd || outcome.Quarantine.Observed.PostRunway.State != test.wantPost {
					t.Fatalf("attempt facts = %q/%q, want %q/%q", outcome.Quarantine.Observed.FastForward, outcome.Quarantine.Observed.PostRunway.State, test.wantFastFwd, test.wantPost)
				}
				wantCalls := []string{"observe-runway", "observe-child", "is-ancestor", "diff", "fast-forward-only", "observe-runway", "persist-quarantine"}
				if !reflect.DeepEqual(fixture.calls, wantCalls) {
					t.Fatalf("attempt-aware calls = %q, want %q", fixture.calls, wantCalls)
				}
			})
		}
	})
	for _, test := range precheckCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
			test.configure(fixture)
			outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
			assertHandbackFailure(t, fixture, outcome, err, test.class, test.reason)
			if !reflect.DeepEqual(fixture.calls, test.wantCalls) {
				t.Fatalf("calls = %q, want %q", fixture.calls, test.wantCalls)
			}
		})
	}

	t.Run("every committed diff status protects both rename and copy paths", func(t *testing.T) {
		cases := []struct {
			name  string
			entry gitx.DiffEntry
			want  []string
		}{
			{name: "add", entry: gitx.DiffEntry{Status: "A", Path: ".verdi/specs/new/spec.md"}, want: []string{".verdi/specs/new/spec.md"}},
			{name: "modify", entry: gitx.DiffEntry{Status: "M", Path: ".verdi/specs/active/a/spec.md"}, want: []string{".verdi/specs/active/a/spec.md"}},
			{name: "delete", entry: gitx.DiffEntry{Status: "D", Path: ".verdi/specs/archive/a/spec.md"}, want: []string{".verdi/specs/archive/a/spec.md"}},
			{name: "type change", entry: gitx.DiffEntry{Status: "T", Path: ".verdi/specs/a/spec.md"}, want: []string{".verdi/specs/a/spec.md"}},
			{name: "unmerged", entry: gitx.DiffEntry{Status: "U", Path: ".verdi/specs/a/spec.md"}, want: []string{".verdi/specs/a/spec.md"}},
			{name: "unknown status", entry: gitx.DiffEntry{Status: "X", Path: ".verdi/specs/a/spec.md"}, want: []string{".verdi/specs/a/spec.md"}},
			{name: "rename old", entry: gitx.DiffEntry{Status: "R", OldPath: ".verdi/specs/old/spec.md", Path: "docs/spec.md"}, want: []string{".verdi/specs/old/spec.md"}},
			{name: "rename new", entry: gitx.DiffEntry{Status: "R", OldPath: "docs/spec.md", Path: ".verdi/specs/new/spec.md"}, want: []string{".verdi/specs/new/spec.md"}},
			{name: "copy old and new", entry: gitx.DiffEntry{Status: "C", OldPath: ".verdi/specs/old/spec.md", Path: ".verdi/specs/new/spec.md"}, want: []string{".verdi/specs/new/spec.md", ".verdi/specs/old/spec.md"}},
		}
		for _, test := range cases {
			test := test
			t.Run(test.name, func(t *testing.T) {
				fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
				fixture.repository.diff = []gitx.DiffEntry{
					test.entry,
					{Status: "M", Path: ".verdi/specs/active/a/status.yaml"},
					{Status: "M", Path: "prefix/.verdi/specs/a/spec.md"},
				}
				outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
				assertHandbackFailure(t, fixture, outcome, err, ErrVerdict, QuarantineProtectedSpecChange)
				if got := outcome.Quarantine.Observed.ProtectedPaths; !reflect.DeepEqual(got, test.want) {
					t.Fatalf("protected paths = %q, want %q", got, test.want)
				}
				if strings.Contains(strings.Join(fixture.calls, " "), "fast-forward") {
					t.Fatalf("protected diff reached fast-forward: %q", fixture.calls)
				}
			})
		}
	})

	t.Run("real unchanged protected-source copy quarantines before fast-forward", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{
			{Files: map[string]string{".verdi/specs/example/spec.md": "protected authority\n", "README.md": "base\n"}, Message: "base"},
			{Files: map[string]string{"docs/copied.md": "protected authority\n"}, Message: "copy protected authority"},
		})
		entries, err := gitx.DiffNameStatusCopies(context.Background(), repo.Dir, repo.Heads[0], repo.Head)
		if err != nil {
			t.Fatalf("DiffNameStatusCopies: %v", err)
		}
		fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
		fixture.repository.diff = entries
		outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
		assertHandbackFailure(t, fixture, outcome, err, ErrVerdict, QuarantineProtectedSpecChange)
		if got, want := outcome.Quarantine.Observed.ProtectedPaths, []string{".verdi/specs/example/spec.md"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("protected paths from real copy diff = %q, want %q; diff %+v", got, want, entries)
		}
		if strings.Contains(strings.Join(fixture.calls, " "), "fast-forward") {
			t.Fatalf("real protected copy reached fast-forward: %q", fixture.calls)
		}
	})

	t.Run("output-write failure preserves finalized bytes and quarantines", func(t *testing.T) {
		fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
		request := fixture.requestValue()
		request.Phase = HandbackPhaseOutputWriteFailed
		outcome, err := fixture.service().Apply(context.Background(), request)
		assertHandbackFailure(t, fixture, outcome, err, ErrOperational, QuarantineOutputWriteFailed)
		if got, want := fixture.calls, []string{"persist-quarantine"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("output-write calls = %q, want %q", got, want)
		}
	})

	t.Run("terminal durability failure preserves partial bytes without release", func(t *testing.T) {
		fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
		partial := []byte("inspectable provider output\n")
		partialRef := PreservedExecutionRef{Schema: PreservedExecutionRefSchemaID, ID: "controller/partial-1", Digest: rawDigest(partial)}
		request := fixture.requestValue()
		request.Phase = HandbackPhaseTerminalDurabilityFailed
		request.Completion = nil
		request.PartialBytes = partial
		request.Preserved = PreservedExecution{State: PreservedPartial, Ref: &partialRef}
		outcome, err := fixture.service().Apply(context.Background(), request)
		assertHandbackFailure(t, fixture, outcome, err, ErrOperational, QuarantineTerminalDurabilityFailed)
		if !bytes.Equal(outcome.ResultBytes, partial) {
			t.Fatalf("partial bytes = %q, want %q", outcome.ResultBytes, partial)
		}
		if outcome.Quarantine.Session != fixture.request.Session {
			t.Fatalf("partial quarantine session = %q, want execution session %q", outcome.Quarantine.Session, fixture.request.Session)
		}
		if got, want := fixture.calls, []string{"persist-quarantine"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("terminal failure calls = %q, want %q", got, want)
		}
	})

	t.Run("execution verdict preserves an optional partial and quarantines", func(t *testing.T) {
		fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
		partial := []byte("interrupted output\n")
		partialRef := PreservedExecutionRef{Schema: PreservedExecutionRefSchemaID, ID: "controller/interrupted-1", Digest: rawDigest(partial)}
		request := fixture.requestValue()
		request.Phase = HandbackPhaseExecutionIncompleteVerdict
		request.Completion = nil
		request.PartialBytes = partial
		request.Preserved = PreservedExecution{State: PreservedPartial, Ref: &partialRef}
		outcome, err := fixture.service().Apply(context.Background(), request)
		assertHandbackFailure(t, fixture, outcome, err, ErrVerdict, QuarantineExecutionIncomplete)
	})

	t.Run("provider operational failure preserves partial output and exits operational", func(t *testing.T) {
		fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
		partial := []byte("provider failure output\n")
		partialRef := PreservedExecutionRef{Schema: PreservedExecutionRefSchemaID, ID: "controller/provider-failure-1", Digest: rawDigest(partial)}
		request := fixture.requestValue()
		request.Phase = HandbackPhaseExecutionIncompleteOperational
		request.Completion = nil
		request.PartialBytes = partial
		request.Preserved = PreservedExecution{State: PreservedPartial, Ref: &partialRef}
		outcome, err := fixture.service().Apply(context.Background(), request)
		assertHandbackFailure(t, fixture, outcome, err, ErrOperational, QuarantineExecutionIncomplete)
	})

	t.Run("incomplete run validates every present acknowledgment before persistence", func(t *testing.T) {
		t.Run("genuinely empty pre-ack run remains representable", func(t *testing.T) {
			fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
			request := fixture.requestValue()
			request.Phase = HandbackPhaseExecutionIncompleteVerdict
			request.Completion = nil
			request.Preserved = PreservedExecution{State: PreservedNone}
			request.Run.Acks = nil
			request.Run.AdapterSessionRef = ""
			outcome, err := fixture.service().Apply(context.Background(), request)
			assertHandbackFailure(t, fixture, outcome, err, ErrVerdict, QuarantineExecutionIncomplete)
		})

		mutations := []struct {
			name      string
			class     error
			configure func(*HandbackRequest)
		}{
			{name: "ack from another session", class: ErrVerdict, configure: func(request *HandbackRequest) {
				request.Run.Acks[0].Session = "other-session"
			}},
			{name: "malformed acknowledgment", class: ErrOperational, configure: func(request *HandbackRequest) {
				request.Run.Acks[0].Schema = ""
			}},
			{name: "discontinuous source order", class: ErrOperational, configure: func(request *HandbackRequest) {
				prior := request.Run.Acks[0]
				next := prior
				next.SourceSequence = prior.SourceSequence + 2
				next.GlobalSequence = prior.GlobalSequence + 1
				request.Run.Acks = append(request.Run.Acks, next)
			}},
			{name: "nonincreasing global order", class: ErrOperational, configure: func(request *HandbackRequest) {
				prior := request.Run.Acks[0]
				next := prior
				next.SourceSequence = prior.SourceSequence + 1
				request.Run.Acks = append(request.Run.Acks, next)
			}},
		}
		for _, mutation := range mutations {
			mutation := mutation
			t.Run(mutation.name, func(t *testing.T) {
				fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
				request := fixture.requestValue()
				request.Phase = HandbackPhaseExecutionIncompleteVerdict
				request.Completion = nil
				request.Preserved = PreservedExecution{State: PreservedNone}
				mutation.configure(&request)
				outcome, err := fixture.service().Apply(context.Background(), request)
				if !errors.Is(err, mutation.class) || outcome.ExitCode != handbackErrorExit(mutation.class) || outcome.Quarantine != nil {
					t.Fatalf("ack mutation outcome/error = %#v / %v", outcome, err)
				}
				if len(fixture.calls) != 0 {
					t.Fatalf("ack mutation reached control persistence: %q", fixture.calls)
				}
			})
		}
	})

	t.Run("control persistence failures and post-ack release failure retain durable state", func(t *testing.T) {
		t.Run("handback ack persistence", func(t *testing.T) {
			fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
			fixture.control.handbackErr = errors.New("controller persistence unavailable")
			outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
			if !errors.Is(err, ErrOperational) || outcome.ExitCode != 2 || outcome.Released {
				t.Fatalf("handback persistence outcome/error = %#v / %v", outcome, err)
			}
			if outcome.Handback == nil || !bytes.Equal(outcome.ResultBytes, fixture.completion.ResultBytes) {
				t.Fatalf("handback persistence lost record/result: %#v", outcome)
			}
			assertNoReleaseOrForbiddenGit(t, fixture.calls)
		})

		t.Run("quarantine persistence", func(t *testing.T) {
			fixture := newHandbackFixture(t, contextevent.AuthorityAdvisory)
			fixture.control.quarantineErr = errors.New("controller persistence unavailable")
			outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
			if !errors.Is(err, ErrOperational) || outcome.ExitCode != 2 || outcome.Released || outcome.Quarantine == nil {
				t.Fatalf("quarantine persistence outcome/error = %#v / %v", outcome, err)
			}
			assertNoReleaseOrForbiddenGit(t, fixture.calls)
		})

		t.Run("release after durable handback", func(t *testing.T) {
			fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
			fixture.releaser.err = errors.New("release marker unavailable")
			outcome, err := fixture.service().Apply(context.Background(), fixture.requestValue())
			if !errors.Is(err, ErrOperational) || outcome.ExitCode != 2 || outcome.Released || outcome.Handback == nil || outcome.ControlAck.Digest == "" {
				t.Fatalf("release failure outcome/error = %#v / %v", outcome, err)
			}
			if !bytes.Equal(outcome.ResultBytes, fixture.completion.ResultBytes) {
				t.Fatal("release failure lost finalized result bytes")
			}
		})
	})

	t.Run("explicit abort persists abort ack before release and preserves exact bytes", func(t *testing.T) {
		fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
		quarantineRequest := fixture.requestValue()
		quarantineRequest.Phase = HandbackPhaseOutputWriteFailed
		quarantined, err := fixture.service().Apply(context.Background(), quarantineRequest)
		if !errors.Is(err, ErrOperational) || quarantined.Quarantine == nil {
			t.Fatalf("prepare quarantine = %#v / %v", quarantined, err)
		}
		fixture.calls = nil
		abortRequest := fixture.requestValue()
		abortRequest.Phase = HandbackPhaseAbort
		abortRequest.Quarantine = quarantined.Quarantine
		abortRequest.OwnerDecision = LogicalRef{Schema: "verdi.owner-decision-ref/v1", ID: "decision-1", Digest: testDigest("decision")}
		outcome, err := fixture.service().Apply(context.Background(), abortRequest)
		if !errors.Is(err, ErrVerdict) || outcome.ExitCode != 1 || !outcome.Released || outcome.Abort == nil || outcome.ControlAck.Digest == "" {
			t.Fatalf("abort outcome/error = %#v / %v", outcome, err)
		}
		if got, want := fixture.calls, []string{"persist-abort", "release"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("abort order = %q, want %q", got, want)
		}
		if !bytes.Equal(outcome.ResultBytes, fixture.completion.ResultBytes) {
			t.Fatal("abort did not preserve exact finalized result bytes")
		}
		if err := ValidateAbortAgainstQuarantine(*outcome.Abort, *quarantined.Quarantine); err != nil {
			t.Fatalf("abort/quarantine binding: %v", err)
		}
	})
}

type handbackFixture struct {
	t          *testing.T
	request    ExecutionRequest
	run        ExecutionRun
	completion Completion
	preserved  PreservedExecution
	calls      []string
	repository *handbackRepositoryFake
	control    *handbackControlFake
	releaser   *handbackReleaserFake
}

func newHandbackFixture(t *testing.T, authority contextevent.Authority) *handbackFixture {
	t.Helper()
	completionFixture := newCompletionFixture(t, authority)
	completion, err := completionFixture.service().Complete(context.Background(), completionFixture.requestValue())
	if err != nil {
		t.Fatalf("prepare completion: %v", err)
	}
	fixture := &handbackFixture{t: t, request: completionFixture.request, run: completionFixture.run, completion: completion}
	finalizedRef := PreservedExecutionRef{Schema: PreservedExecutionRefSchemaID, ID: "controller/finalized-1", Digest: rawDigest(completion.ResultBytes)}
	fixture.preserved = PreservedExecution{State: PreservedFinalized, Ref: &finalizedRef}
	fixture.repository = &handbackRepositoryFake{fixture: fixture,
		runway:     RepositoryState{Path: fixture.request.ATCRunway, Commit: fixture.request.InputCommit, Tree: fixture.request.InputTree, Clean: true},
		child:      RepositoryState{Path: fixture.run.Workspace.Path, Commit: completion.Result.OutputCommit, Tree: completion.Result.OutputTree, Clean: true},
		post:       RepositoryState{Path: fixture.request.ATCRunway, Commit: completion.Result.OutputCommit, Tree: completion.Result.OutputTree, Clean: true},
		descendant: true, diff: []gitx.DiffEntry{{Status: "M", Path: "internal/example.go"}},
		fastForwardResult: gitx.FastForwardResult{Category: gitx.FastForwardSucceeded, Attempted: true},
	}
	fixture.control = &handbackControlFake{fixture: fixture}
	fixture.releaser = &handbackReleaserFake{fixture: fixture}
	return fixture
}

func (f *handbackFixture) requestValue() HandbackRequest {
	return HandbackRequest{
		Phase: HandbackPhaseCompleted, Request: f.request, Run: f.run,
		Completion: &f.completion, Preserved: f.preserved,
	}
}

func (f *handbackFixture) service() *HandbackService {
	f.t.Helper()
	service, err := NewHandbackService(HandbackPorts{Repository: f.repository, Control: f.control, Releaser: f.releaser})
	if err != nil {
		f.t.Fatalf("NewHandbackService: %v", err)
	}
	return service
}

type handbackRepositoryFake struct {
	fixture           *handbackFixture
	runway            RepositoryState
	child             RepositoryState
	post              RepositoryState
	descendant        bool
	diff              []gitx.DiffEntry
	runwayObserveErr  error
	childObserveErr   error
	postObserveErr    error
	ancestorErr       error
	diffErr           error
	fastForwardResult gitx.FastForwardResult
	fastForwardErr    error
	runwayReads       int
}

func (f *handbackRepositoryFake) Observe(_ context.Context, path string) (RepositoryState, error) {
	if path == f.fixture.request.ATCRunway {
		f.fixture.calls = append(f.fixture.calls, "observe-runway")
		f.runwayReads++
		if f.runwayReads > 1 {
			if f.postObserveErr != nil {
				return RepositoryState{}, f.postObserveErr
			}
			return f.post, nil
		}
		if f.runwayObserveErr != nil {
			return RepositoryState{}, f.runwayObserveErr
		}
		return f.runway, nil
	}
	if path == f.fixture.run.Workspace.Path {
		f.fixture.calls = append(f.fixture.calls, "observe-child")
		if f.childObserveErr != nil {
			return RepositoryState{}, f.childObserveErr
		}
		return f.child, nil
	}
	f.fixture.t.Fatalf("unexpected repository observation path %q", path)
	return RepositoryState{}, nil
}

func (f *handbackRepositoryFake) IsAncestor(_ context.Context, path, inputCommit, outputCommit string) (bool, error) {
	f.fixture.calls = append(f.fixture.calls, "is-ancestor")
	if path != f.fixture.run.Workspace.Path || inputCommit != f.fixture.request.InputCommit || outputCommit != f.fixture.completion.Result.OutputCommit {
		f.fixture.t.Fatalf("ancestry query = (%q,%q,%q)", path, inputCommit, outputCommit)
	}
	return f.descendant, f.ancestorErr
}

func (f *handbackRepositoryFake) Diff(_ context.Context, path, inputCommit, outputCommit string) ([]gitx.DiffEntry, error) {
	f.fixture.calls = append(f.fixture.calls, "diff")
	if path != f.fixture.run.Workspace.Path || inputCommit != f.fixture.request.InputCommit || outputCommit != f.fixture.completion.Result.OutputCommit {
		f.fixture.t.Fatalf("diff query = (%q,%q,%q)", path, inputCommit, outputCommit)
	}
	return append([]gitx.DiffEntry(nil), f.diff...), f.diffErr
}

func (f *handbackRepositoryFake) FastForwardOnly(_ context.Context, runway, outputCommit string) (gitx.FastForwardResult, error) {
	f.fixture.calls = append(f.fixture.calls, "fast-forward-only")
	if runway != f.fixture.request.ATCRunway || outputCommit != f.fixture.completion.Result.OutputCommit {
		f.fixture.t.Fatalf("fast-forward query = (%q,%q)", runway, outputCommit)
	}
	return f.fastForwardResult, f.fastForwardErr
}

type handbackControlFake struct {
	fixture       *handbackFixture
	handbackErr   error
	quarantineErr error
	abortErr      error
}

func (f *handbackControlFake) PersistHandback(_ context.Context, record HandbackRecord) (ControlAck, error) {
	f.fixture.calls = append(f.fixture.calls, "persist-handback")
	canonical := mustCanonicalHandback(f.fixture.t, record)
	if f.handbackErr != nil {
		return ControlAck{}, f.handbackErr
	}
	return mustCanonicalControlAck(f.fixture.t, validControlAckForHandback(canonical)), nil
}

func (f *handbackControlFake) PersistQuarantine(_ context.Context, record QuarantineRecord) (ControlAck, error) {
	f.fixture.calls = append(f.fixture.calls, "persist-quarantine")
	canonical := mustCanonicalQuarantine(f.fixture.t, record)
	if f.quarantineErr != nil {
		return ControlAck{}, f.quarantineErr
	}
	return mustCanonicalControlAck(f.fixture.t, validControlAckForQuarantine(canonical)), nil
}

func (f *handbackControlFake) PersistAbort(_ context.Context, record AbortRecord) (ControlAck, error) {
	f.fixture.calls = append(f.fixture.calls, "persist-abort")
	canonical := mustCanonicalAbort(f.fixture.t, record)
	if f.abortErr != nil {
		return ControlAck{}, f.abortErr
	}
	return mustCanonicalControlAck(f.fixture.t, validControlAckForAbort(canonical)), nil
}

type handbackReleaserFake struct {
	fixture *handbackFixture
	err     error
}

func (f *handbackReleaserFake) Release(_ context.Context, workspaceID string) error {
	f.fixture.calls = append(f.fixture.calls, "release")
	if workspaceID != f.fixture.run.Workspace.WorkspaceID {
		f.fixture.t.Fatalf("release workspace = %q", workspaceID)
	}
	return f.err
}

func assertHandbackFailure(t *testing.T, fixture *handbackFixture, outcome HandbackOutcome, err, wantClass error, reason QuarantineReason) {
	t.Helper()
	if !errors.Is(err, wantClass) {
		t.Fatalf("Apply error = %v, want class %v", err, wantClass)
	}
	wantExit := 1
	if errors.Is(wantClass, ErrOperational) {
		wantExit = 2
	}
	if outcome.ExitCode != wantExit || outcome.Released || outcome.Quarantine == nil || outcome.Quarantine.Reason != reason {
		t.Fatalf("failure outcome = %#v", outcome)
	}
	if outcome.Quarantine.Digest == "" || outcome.ControlAck.Digest == "" {
		t.Fatalf("quarantine was not durably acknowledged: %#v", outcome)
	}
	if err := ValidateQuarantineAck(*outcome.Quarantine, outcome.ControlAck); err != nil {
		t.Fatalf("durable quarantine ack: %v", err)
	}
	if outcome.Quarantine.Preserved.State == PreservedFinalized && !bytes.Equal(outcome.ResultBytes, fixture.completion.ResultBytes) {
		t.Fatal("finalized quarantine did not preserve exact public result bytes")
	}
	assertNoReleaseOrForbiddenGit(t, fixture.calls)
}

func assertNoReleaseOrForbiddenGit(t *testing.T, calls []string) {
	t.Helper()
	joined := strings.Join(calls, " ")
	for _, forbidden := range []string{"release", "reset", "--force", "update-ref", "apply-patch", "patch-copy"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("forbidden post-failure call %q in %q", forbidden, calls)
		}
	}
}
