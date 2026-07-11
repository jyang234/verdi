# testdata/forge-captures

Canned forge-comment captures (S6's committed proof artifacts, V1-P1 brief
§4 appendix), copied verbatim from `docs/spikes/v1/s6-captures/` — real
GitHub review-comments JSON (captured live against a throwaway repo) and
GitLab discussion-notes JSON (doc-derived, since `glab` auth was
unavailable during the spike — see each file's own `_capture_status` field;
that labeling is preserved unchanged). Each forge has at least one capture
carrying a `[vd:<object-id>]`-bearing comment and a token-free comment (the
comment-token grammar, 02 §Record schemas): `github/01-list-review-comments-REST.json`
and `gitlab/01-doc-derived-UNVERIFIED-list-discussions.json` both have one
of each.

Used to build `httptest` fakes for V1-P7's forge-comment contract suite and
referenced by V1-P0's spike write-up (`docs/spikes/v1/spike-s6-findings.md`).
Not consumed by any test in V1-P1 itself — this phase only lands the
copies; V1-P7 builds the httptest fakes.
