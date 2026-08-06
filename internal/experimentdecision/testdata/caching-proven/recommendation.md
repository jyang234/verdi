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

Definition digest: `sha256:b2aaf140bf90d8d15ccefb06c7b8fa2210eeeecb8237862bd285bbb396747f58`

Result digest: `sha256:63100ca6a2bfb67dddddcbaede96e4130ab8b274cc7e6cc2cb02a35c3c071041`

Run: `run-1`

Algorithm: `verdi.experiment-recommendation/v1`
