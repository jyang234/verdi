// Package aliasgitx exports an alias of a type declared in another
// package: its methods are not this package's functions, so the export
// census cannot classify them and must refuse.
package aliasgitx

import "net/http"

// Client is an alias of a standard-library type.
type Client = http.Client
