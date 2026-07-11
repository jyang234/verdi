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
package boardio
