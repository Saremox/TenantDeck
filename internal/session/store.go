package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// randReader is crypto/rand.Reader by default; swapped out in tests to
// exercise generateID's error path without touching the real package-level
// crypto/rand.Reader (which other tests/goroutines may rely on).
var randReader io.Reader = rand.Reader

const (
	sessionKeyPrefix = "session:"
	loginKeyPrefix   = "login:"

	// idByteLength is the entropy (before base64url encoding) of every
	// generated session ID and login transaction state value.
	idByteLength = 32
)

// Record is what's stored per logged-in session. Deliberately minimal for
// the vertical slice: refresh-token handling is not implemented yet (see
// docs/implementation-plan.md "Known blockers") and isn't stored here until
// it is, per CLAUDE.md's "don't design for hypothetical future requirements."
type Record struct {
	Subject   string    `json:"subject"`
	IDToken   string    `json:"id_token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`

	// CSRFToken is handed to the frontend once, in the /auth/session JSON
	// body (never a cookie), and must be echoed back on state-changing
	// requests like logout. See docs/spec/04-auth-session-browser-security.md
	// ("Require CSRF protection ... for state-changing local endpoints").
	CSRFToken string `json:"csrf_token"`
}

// LoginTransaction is the server-side record of one in-progress login,
// keyed by its own state value. Storing it server-side (rather than trusting
// client-supplied PKCE/nonce values) is what makes the callback verifiable.
type LoginTransaction struct {
	Nonce        string    `json:"nonce"`
	PKCEVerifier string    `json:"pkce_verifier"`
	CreatedAt    time.Time `json:"created_at"`
}

// Store layers typed, encrypted session and login-transaction operations
// over a raw backend. Every record is JSON-marshaled then sealed with
// AES-256-GCM before it ever reaches the backend.
type Store struct {
	backend backend
	enc     *encryptor
}

func NewStore(b backend, encryptionKey []byte) (*Store, error) {
	enc, err := newEncryptor(encryptionKey)
	if err != nil {
		return nil, err
	}
	return &Store{backend: b, enc: enc}, nil
}

func generateID() (string, error) {
	raw := make([]byte, idByteLength)
	if _, err := io.ReadFull(randReader, raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *Store) put(ctx context.Context, key string, v any, ttl time.Duration) error {
	plaintext, err := json.Marshal(v)
	if err != nil {
		return err
	}
	sealed, err := s.enc.seal(plaintext)
	if err != nil {
		return err
	}
	return s.backend.Set(ctx, key, sealed, ttl)
}

// CreateSession stores rec under a freshly generated, random session ID and
// returns that ID. The browser only ever learns this opaque ID, never rec's
// contents.
func (s *Store) CreateSession(ctx context.Context, rec Record, ttl time.Duration) (id string, err error) {
	id, err = generateID()
	if err != nil {
		return "", err
	}
	if err := s.put(ctx, sessionKeyPrefix+id, rec, ttl); err != nil {
		return "", err
	}
	return id, nil
}

// GetSession returns ErrNotFound if the session doesn't exist or has
// expired, and any other error if the backend itself failed (e.g.
// unreachable). Callers must treat both as "deny access" — see
// docs/spec/04-auth-session-browser-security.md on failing closed when the
// store is unavailable.
func (s *Store) GetSession(ctx context.Context, id string) (Record, error) {
	sealed, err := s.backend.Get(ctx, sessionKeyPrefix+id)
	if err != nil {
		return Record{}, err
	}
	var rec Record
	plaintext, err := s.enc.open(sealed)
	if err != nil {
		return Record{}, fmt.Errorf("session: decrypt: %w", err)
	}
	if err := json.Unmarshal(plaintext, &rec); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// DeleteSession removes a session, e.g. on logout. Deleting an
// already-absent session is not an error.
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	return s.backend.Del(ctx, sessionKeyPrefix+id)
}

// CreateLoginTransaction stores txn under a freshly generated state value
// and returns it. The state value is what the browser round-trips through
// the IdP and what the BFF binds the callback to.
func (s *Store) CreateLoginTransaction(ctx context.Context, txn LoginTransaction, ttl time.Duration) (state string, err error) {
	state, err = generateID()
	if err != nil {
		return "", err
	}
	if err := s.put(ctx, loginKeyPrefix+state, txn, ttl); err != nil {
		return "", err
	}
	return state, nil
}

// TakeLoginTransaction atomically reads and deletes the transaction for
// state, so a given state can only ever be redeemed once — required for
// the "one-use state" property in docs/spec/04-auth-session-browser-security.md.
func (s *Store) TakeLoginTransaction(ctx context.Context, state string) (LoginTransaction, error) {
	sealed, err := s.backend.GetDel(ctx, loginKeyPrefix+state)
	if err != nil {
		return LoginTransaction{}, err
	}
	var txn LoginTransaction
	plaintext, err := s.enc.open(sealed)
	if err != nil {
		return LoginTransaction{}, fmt.Errorf("session: decrypt: %w", err)
	}
	if err := json.Unmarshal(plaintext, &txn); err != nil {
		return LoginTransaction{}, err
	}
	return txn, nil
}
