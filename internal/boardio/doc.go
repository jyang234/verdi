// Package boardio is the shared filesystem I/O for the mutable zone's
// annotation streams and board state (01 §Directory layout,
// 02 §Record schemas, 05 §Workbench). It was internal/mcpserve's private
// implementation detail (phase 9: annotationio.go, part of backend.go)
// until internal/workbench (phase 10) needed the identical
// read-every-annotation-file and locate-the-right-stream logic to render
// board stickies — CLAUDE.md's "anything used by two or more packages
// lives in a shared internal/ package" rule applies directly, so the
// annotation-stream helpers moved here verbatim and internal/mcpserve now
// delegates to them. Board *state* I/O (LoadBoardState/SaveBoardState,
// verdi.board/v1's mutable-zone document — new in phase 10) lives here
// too, since it is the same "read/write one JSON-shaped file under the
// mutable zone, atomically" concern.
//
// Caller contract (spec/fail-loud ac-4): the read-modify-write helpers —
// RepositionSticky, GraduateStickies, DeleteAnnotations — are NOT
// internally synchronized. Each one loads a file, mutates it, and writes
// it back; the caller MUST hold the store's single write lock across that
// load→write span, or two overlapping calls can lose an update (last
// writer wins). In production that lock is workbench's per-dispatch
// writeMu, held inside the one process the writer.lock file admits
// (internal/workbench/boardspec.go's M-2 window). Atomic temp+rename
// prevents a torn file either way, but does not by itself prevent a lost
// update — only the caller-held lock does.
package boardio
