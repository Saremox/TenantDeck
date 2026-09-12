package session

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"io"
)

// SessionKeySize is the required length, in bytes, of the session
// encryption key (AES-256-GCM). Exported so callers building that key
// (internal/config) and tests elsewhere share one source of truth for it.
const SessionKeySize = 32

// encryptor seals/opens session payloads with AES-256-GCM. The key is
// supplied by the caller (from config, never from the store itself) so that
// compromising the session store alone never yields usable session data.
type encryptor struct {
	gcm cipher.AEAD
}

func newEncryptor(key []byte) (*encryptor, error) {
	if len(key) != SessionKeySize {
		return nil, fmt.Errorf("session: encryption key must be %d bytes, got %d", SessionKeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &encryptor{gcm: gcm}, nil
}

// seal encrypts plaintext, prepending a fresh random nonce to the output.
func (e *encryptor) seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(randReader, nonce); err != nil {
		return nil, err
	}
	return e.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

var errCiphertextTooShort = errors.New("session: ciphertext shorter than nonce")

// open decrypts a value produced by seal. It returns an error on any
// tampering or key mismatch rather than partial/best-effort data.
func (e *encryptor) open(ciphertext []byte) ([]byte, error) {
	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errCiphertextTooShort
	}
	nonce, data := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return e.gcm.Open(nil, nonce, data, nil)
}
