package webassets

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testDist() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>spa-shell</html>")},
		"assets/app.js": {Data: []byte("console.log('app')")},
		"favicon.ico":   {Data: []byte("icon-bytes")},
	}
}

func TestHandlerFromFS_ServesAnExistingFile(t *testing.T) {
	h := HandlerFromFS(testDist())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "console.log('app')" {
		t.Errorf("body = %q, want the real file content", rec.Body.String())
	}
}

func TestHandlerFromFS_ServesIndexAtRoot(t *testing.T) {
	h := HandlerFromFS(testDist())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "<html>spa-shell</html>" {
		t.Errorf("body = %q, want index.html's content", rec.Body.String())
	}
}

// This is the actual SPA-fallback behavior: a path that isn't a real built
// file (a client-side route) must still get index.html, not a 404, so a
// hard refresh on a future client-routed page works.
func TestHandlerFromFS_FallsBackToIndexForUnknownPath(t *testing.T) {
	h := HandlerFromFS(testDist())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/namespaces/tenant-a-dev", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "<html>spa-shell</html>" {
		t.Errorf("body = %q, want index.html's content as the SPA fallback", rec.Body.String())
	}
}

func TestHandlerFromFS_DoesNotFallBackForADirectoryPath(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":    {Data: []byte("<html>spa-shell</html>")},
		"assets/app.js": {Data: []byte("console.log('app')")},
	}
	h := HandlerFromFS(dist)
	rec := httptest.NewRecorder()
	// "assets" exists as a directory, not a file - must still fall back to
	// the SPA shell rather than erroring on it.
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "<html>spa-shell</html>" {
		t.Errorf("body = %q, want the SPA shell for a directory path", rec.Body.String())
	}
}

func TestHandler_BuildsAWorkingHandlerFromTheRealEmbeddedAssets(t *testing.T) {
	h, err := Handler()
	if err != nil {
		t.Fatalf("Handler(): %v", err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d serving the real embedded web.DistFS", rec.Code, http.StatusOK)
	}
}
