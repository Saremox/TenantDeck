// Package web embeds the built frontend (web/dist, produced by `npm run
// build`) so the Go binary can serve it without a separate static-file
// deployment step - one production container, one browser origin, per
// docs/spec/03-architecture.md.
package web

import "embed"

//go:embed dist
var DistFS embed.FS
