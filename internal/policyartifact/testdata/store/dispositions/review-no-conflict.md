---
schema: verdi.policy-disposition/v1
id: policy-disposition/review-no-conflict
kind: policy-disposition
title: "Review-phase claims coexist without conflict"
owners: [platform-team]
scope: {phases: [review], environments: [], paths: [], refs: []}
witness:
  input_id: "sha256:ee41b6446f3eb0b01c96f43bdc5da0f29412469e5753ae812f20fe92df4a1ef5"
  target_digest: "sha256:690557732c5799f64393edbdb9341bfc68641940ff7855da9d208db583bea7dc"
  claims:
    - id: ac-review-approval
      digest: "sha256:8283e47538b00c9a80b75a27c5c18edade346d389f3e98cd2286fef0036e460e"
      category: acceptance-criterion
      authority_digest: "sha256:30babef8029aa29da8e43938c8c482ed249ce689df0e65d82a14bf50ffda19c8"
      scope: {phases: [review], environments: [], paths: [], refs: []}
      values: ["approved"]
    - id: policy-review-instruction
      digest: "sha256:fed9eace5c758797265b026b75c34a1534ba7ae36191012b0316ee02aeb7de9f"
      category: policy-instruction
      authority_digest: "sha256:b4334d98e9553f81da956dbd504e0a136ffe71aeceabbbbea8cb12bc23177539"
      scope: {phases: [], environments: [], paths: [], refs: []}
      values: []
  exemptions:
    - id: policy-exemption/legacy-service-go
      digest: "sha256:9d0655fb85addc86e8ceff9ec0035ebef7310f7e574e08eae806ce6fb395bef4"
conclusion: no-conflict
origin: judge-result
judgment:
  primary_digest: "sha256:b6a5c2de0c94d4d9772316059ba6e1449f1cc1f0fe471da85b7ef147402c723c"
approvals:
  - role: policy-owner
    principal: principal/github-org/YWxpY2U
template: {identity: "embedded:policy-disposition.md", digest: "sha256:c75815c9b8eb8824ccdea2456bd85db558cf49aed85ea5df7eac9663822c4c63"}
---
The primary judge found the acceptance-criterion approval claim and the
policy review instruction mutually satisfiable within the review phase;
no conflict exists in the current semantic input.
