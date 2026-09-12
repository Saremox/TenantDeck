// Package web embeds the built frontend (web/dist, produced by `npm run
// build`) so the Go binary can serve it without a separate static-file
// deployment step - one production container, one browser origin, per
// docs/spec/03-architecture.md.
package web

import "embed"

// The "all:" prefix is required, not stylistic: go:embed excludes
// dot/underscore-prefixed files by default, and web/dist/.gitkeep is
// exactly that - without "all:", a fresh checkout (before `npm run
// build` has ever populated dist with anything else) embeds zero files
// and fails to compile at all ("contains no embeddable files"), which is
// the opposite of .gitkeep's whole purpose here.
//
//go:embed all:dist
var DistFS embed.FS
