package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivezHandler_AlwaysReturnsOK(t *testing.T) {
	rec := httptest.NewRecorder()
	livezHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

type fakeStorePinger struct{ err error }

func (f fakeStorePinger) Ping(context.Context) error { return f.err }

func TestReadyzHandler_ReturnsOKWhenStoreIsReachable(t *testing.T) {
	rec := httptest.NewRecorder()
	readyzHandler(fakeStorePinger{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestReadyzHandler_ReturnsServiceUnavailableWhenStoreIsUnreachable(t *testing.T) {
	rec := httptest.NewRecorder()
	readyzHandler(fakeStorePinger{err: errors.New("store down")}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
