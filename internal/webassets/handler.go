// Package webassets serves the embedded built frontend (web.DistFS), with
// the standard single-page-app fallback: any path that isn't a real built
// file gets index.html instead of a 404, so client-side routes don't break
// on a hard refresh.
package webassets

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/Saremox/TenantDeck/web"
)

// Handler serves the real embedded frontend build.
func Handler() (http.Handler, error) {
	dist, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		return nil, err
	}
	return HandlerFromFS(dist), nil
}

// HandlerFromFS builds the same handler over an arbitrary fs.FS, so tests
// can exercise the SPA-fallback logic against a small in-memory filesystem
// instead of the real build output.
func HandlerFromFS(dist fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !fileExists(dist, r.URL.Path) {
			r = requestForRoot(r)
		}
		fileServer.ServeHTTP(w, r)
	})
}

func fileExists(dist fs.FS, urlPath string) bool {
	clean := strings.TrimPrefix(path.Clean(urlPath), "/")
	if clean == "." {
		clean = "index.html"
	}
	info, err := fs.Stat(dist, clean)
	return err == nil && !info.IsDir()
}

// requestForRoot returns a shallow clone of r with its path replaced by
// "/", so the wrapped file server resolves it to index.html.
func requestForRoot(r *http.Request) *http.Request {
	clone := r.Clone(r.Context())
	u := *r.URL
	u.Path = "/"
	clone.URL = &u
	return clone
}
