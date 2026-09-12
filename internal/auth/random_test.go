package auth

import (
	"errors"
	"testing"
)

var errTestRandFailure = errors.New("test: rand read failed")

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errTestRandFailure }

func TestRandomToken_ReturnsErrorWhenRandReaderFails(t *testing.T) {
	orig := randReader
	randReader = failingReader{}
	defer func() { randReader = orig }()

	if _, err := randomToken(); err == nil {
		t.Fatal("randomToken() succeeded with a failing rand reader, want error")
	}
}
