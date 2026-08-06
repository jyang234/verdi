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

Definition digest: `sha256:2764d29e30525accf3b0ee69568bb6da2fd71c9abfc7e0ee16079db3411512a7`

Result digest: `sha256:8f8298d8682131ce0c52de8a284694eac3ed894f4d7aedfa707a52cc8c77c6ca`

Run: `run-1`

Algorithm: `verdi.experiment-recommendation/v1`
