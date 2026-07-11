// Package providertest is the story-provider contract-test suite (04
// §Testing): "the port ships with a fake provider and a contract-test
// suite that every adapter must pass." Run drives a Harness through
// resolve-happy-path, resolve-not-found, publish-idempotency, and
// comment-only-on-change, observing published state and comments through
// the Harness abstraction so the same suite runs unchanged against the
// fake (this phase) and a future httptest-backed Jira adapter (PLAN.md
// phase 11).
//
// An adapter package proves it satisfies the contract with its own test
// implementing Harness and calling Run — see
// internal/provider/fake's contract_test.go for the reference instance.
package providertest
