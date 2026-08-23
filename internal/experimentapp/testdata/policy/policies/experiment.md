---
schema: verdi.policy/v1
id: policy/experiment
kind: policy
title: "Experiment execution fixture policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
payloads:
  experiment_execution:
    experiment_paths: [.verdi/specs/active/**/experiments/**]
    candidate_paths: [spikes/**]
    classes: [request-path-performance]
    evaluators:
      - argv0: ./tools/evaluator
        protocols: [verdi.experiment-evaluator/v1]
    environments:
      - id: local-isolated-v1
        grants:
          schema: verdi.execution-grants/v1
          grants:
            - kind: process-execution
              argv0s: [./tools/evaluator]
            - kind: timeouts
              seconds: 30
        declared_environment: {GOMAXPROCS: "1"}
    limits: {observation_bytes: 262144, retained_artifact_bytes: 8388608}
    trusted_measurement_sources: [evaluator-measured, harness-measured]
    mandatory_guards: []
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Allow the hermetic request-path comparison fixture.
