// Package store implements the semantics 01-store-layout.md owns: store
// manifest decode, root discovery (I-16), the normative ref-slug mapping
// (01 §notes), the corpus tree hash and cache key naming (I-15, D4), and
// service discovery via .flowmap.yaml (01 §Store manifest,
// PLAN.md §3 "Service discovery" row). It does not walk or index committed
// artifacts — that is internal/index's job (02 §External refs, §Link
// taxonomy), built on top of what this package discovers.
package store
