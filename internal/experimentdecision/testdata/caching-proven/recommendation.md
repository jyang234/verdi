# Experiment cache-placement-v1

**Verdict:** proven-winner

**Winner:** facts-cache

Candidate facts-cache is the best demonstrated path among the registered candidates for this desired outcome, workload, environment, and comparison revision.

This is not a claim of universal superiority over unregistered designs or unrepresented production conditions.

## Candidates

| id | baseline | primary | eligible | violated guards |
|---|---|---|---|---|
| baseline | yes | 42 ms | true |  |
| final-cache |  | 12 ms | false | behavioral-equivalence (round 3: stale response detected after policy update in round 3) |
| facts-cache |  | 19 ms | true |  |

Definition digest: `sha256:5f40d68bb34a4384534721ae42ca7f44c109c87dd8c23aba4400fcac4687a7e8`

Result digest: `sha256:41327609fe8854f6e116a6691f28411575875318f08a6472d555d351157da3fb`

Run: `run-1`

Algorithm: `verdi.experiment-recommendation/v1`
