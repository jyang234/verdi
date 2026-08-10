// Package draftmutation implements ASD's structured, authority-gated,
// crash-safe proposal mutation kernel. It applies exact ordered operations and
// commits spec bytes with their append-only design-provenance sidecar under
// the checkout-wide writer lock.
package draftmutation
