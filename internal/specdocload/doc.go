// Package specdocload assembles the one specdoc.Input every consumer
// renders from: the CLI verb, the board's Document tab, the docs site, and
// the MCP get_document tool. Sharing the assembly is what makes the four
// outputs byte-identical (spec/spec-documents ac-6). It reads the store
// and git; it never writes.
package specdocload
