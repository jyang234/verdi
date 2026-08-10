# Experiment cache-placement-v2

**Verdict:** disclosed-unproven

## Reasons

- `insufficient-separation` candidate=facts-cache detail="insufficient separation from candidate edge-cache"

## Candidates

| id | baseline | primary | eligible | violated guards |
|---|---|---|---|---|
| baseline | yes | 42 ms | true |  |
| facts-cache |  | 19 ms | true |  |
| edge-cache |  | 19.5 ms | true |  |

Definition digest: `sha256:fa5ce7dc638a306e23a6792449c06f1dec066956bf1321fbedf4254994bfe127`

Result digest: `sha256:44f6e1141e36100a309baba240bd8d56ba4653e7c095452fea84eb952115a922`

Run: `run-1`

Algorithm: `verdi.experiment-recommendation/v1`
