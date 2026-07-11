package dex

import "github.com/OWNER/verdi/internal/render"

// renderDispositionsTable delegates to internal/render.DispositionsTable —
// see internal/dex/render.go's doc comment on why this moved to a shared
// package.
var renderDispositionsTable = render.DispositionsTable
