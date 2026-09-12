package auth

import (
	"crypto/rand"
	"encoding/base64"
	"io"
)

// randomTokenBytes is the entropy (before encoding) of every generated
// nonce and PKCE verifier.
const randomTokenBytes = 32

// randReader is swapped out in tests to exercise random-generation failure
// paths without touching the real crypto/rand.Reader.
var randReader io.Reader = rand.Reader

func randomToken() (string, error) {
	raw := make([]byte, randomTokenBytes)
	if _, err := io.ReadFull(randReader, raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
