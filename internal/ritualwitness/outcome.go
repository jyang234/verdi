package ritualwitness

// RunOutcome is one ritual run's three-valued result.
type RunOutcome string

// The three outcomes: proven within its declaration, violated with a
// witness, or disclosed as unproven.
const (
	Pass     RunOutcome = "pass"
	Fail     RunOutcome = "fail"
	Unproven RunOutcome = "unproven"
)

// Outcome folds a run's verdicts into its result: any outside verdict
// fails it; otherwise any unattributable verdict leaves it unproven;
// otherwise it passes. No verdicts at all is unproven, never a pass:
// Evaluate always reports the index carry, so an empty list means nothing
// was evaluated.
func Outcome(verdicts []Verdict) RunOutcome {
	if len(verdicts) == 0 {
		return Unproven
	}
	out := Pass
	for _, v := range verdicts {
		switch v.Status {
		case Outside:
			return Fail
		case Within:
		default:
			out = Unproven
		}
	}
	return out
}
