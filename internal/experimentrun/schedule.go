// Package experimentrun derives and validates the sealed inputs to one CSE
// execution run. It owns no process, workspace, policy, or persistence logic.
package experimentrun

import (
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/experiment"
)

// ScheduledAttempt is one candidate invocation in the registered logical
// schedule. Warmup attempts precede measured attempts and cycle numbers are
// one-based within their kind.
type ScheduledAttempt struct {
	Candidate string                    `json:"candidate"`
	Cycle     experiment.EvaluatorCycle `json:"cycle"`
}

// DeriveSchedule returns def's complete deterministic warmup and measured
// schedule. The returned slice has no mutable state shared with def.
func DeriveSchedule(def experiment.Definition) ([]ScheduledAttempt, error) {
	if err := def.Validate(); err != nil {
		return nil, fmt.Errorf("experimentrun: derive schedule: %w", err)
	}
	count := len(def.Candidates)
	schedule := make([]ScheduledAttempt, 0, count*(def.Execution.Warmups+def.Execution.Rounds))
	cycleIndex := 0
	for _, cycle := range []struct {
		kind  experiment.CycleKind
		count int
	}{
		{kind: experiment.CycleWarmup, count: def.Execution.Warmups},
		{kind: experiment.CycleMeasured, count: def.Execution.Rounds},
	} {
		for number := 1; number <= cycle.count; number++ {
			for offset := 0; offset < count; offset++ {
				candidate := def.Candidates[(cycleIndex+offset)%count]
				schedule = append(schedule, ScheduledAttempt{
					Candidate: candidate.ID,
					Cycle:     experiment.EvaluatorCycle{Kind: cycle.kind, Number: number},
				})
			}
			cycleIndex++
		}
	}
	return schedule, nil
}

// ScheduleDigest returns the canonical digest of a complete logical schedule.
func ScheduleDigest(schedule []ScheduledAttempt) (string, error) {
	if len(schedule) == 0 {
		return "", fmt.Errorf("experimentrun: schedule is empty")
	}
	for i, attempt := range schedule {
		if err := experiment.ValidateID(attempt.Candidate); err != nil {
			return "", fmt.Errorf("experimentrun: schedule attempt %d candidate: %w", i, err)
		}
		if err := attempt.Cycle.Validate(); err != nil {
			return "", fmt.Errorf("experimentrun: schedule attempt %d cycle: %w", i, err)
		}
	}
	digest, err := canonjson.Digest(schedule)
	if err != nil {
		return "", fmt.Errorf("experimentrun: digest schedule: %w", err)
	}
	return digest, nil
}
