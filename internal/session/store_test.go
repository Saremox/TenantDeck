package session

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeBackend is an in-memory backend for tests that don't need a real
// Redis. It ignores TTLs — tests that care about expiry use a real Redis
// (see store_redis_test.go) rather than reimplementing TTL semantics here.
type fakeBackend struct {
	data map[string][]byte
}

func newFakeBackend() *fakeBackend { return &fakeBackend{data: map[string][]byte{}} }

func (f *fakeBackend) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	f.data[key] = value
	return nil
}

func (f *fakeBackend) Get(_ context.Context, key string) ([]byte, error) {
	v, ok := f.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (f *fakeBackend) GetDel(ctx context.Context, key string) ([]byte, error) {
	v, err := f.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	delete(f.data, key)
	return v, nil
}

func (f *fakeBackend) Del(_ context.Context, key string) error {
	delete(f.data, key)
	return nil
}

// unavailableBackend simulates a store that cannot be reached at all —
// every operation fails, as if Redis were down.
type unavailableBackend struct{}

var errStoreUnavailable = errors.New("session: store unavailable")

func (unavailableBackend) Set(context.Context, string, []byte, time.Duration) error {
	return errStoreUnavailable
}
func (unavailableBackend) Get(context.Context, string) ([]byte, error) {
	return nil, errStoreUnavailable
}
func (unavailableBackend) GetDel(context.Context, string) ([]byte, error) {
	return nil, errStoreUnavailable
}
func (unavailableBackend) Del(context.Context, string) error {
	return errStoreUnavailable
}

func newTestStore(t *testing.T, b backend) *Store {
	t.Helper()
	s, err := NewStore(b, testKey())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

func TestStore_GetSessionRecoversWhatCreateSessionStored(t *testing.T) {
	s := newTestStore(t, newFakeBackend())
	want := Record{Subject: "alice", IDToken: "header.payload.sig", ExpiresAt: time.Now().Add(time.Hour)}

	id, err := s.CreateSession(context.Background(), want, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	got, err := s.GetSession(context.Background(), id)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Subject != want.Subject || got.IDToken != want.IDToken {
		t.Errorf("GetSession() = %+v, want %+v", got, want)
	}
}

func TestStore_CreateSessionGeneratesDistinctIDs(t *testing.T) {
	s := newTestStore(t, newFakeBackend())
	rec := Record{Subject: "alice"}

	id1, err := s.CreateSession(context.Background(), rec, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	id2, err := s.CreateSession(context.Background(), rec, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if id1 == id2 {
		t.Error("two CreateSession calls returned the same session ID")
	}
}

func TestStore_GetSessionFailsAfterDeleteSession(t *testing.T) {
	s := newTestStore(t, newFakeBackend())
	id, err := s.CreateSession(context.Background(), Record{Subject: "alice"}, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := s.DeleteSession(context.Background(), id); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if _, err := s.GetSession(context.Background(), id); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSession() after delete = %v, want ErrNotFound", err)
	}
}

// This is the fail-closed requirement from docs/spec/04-auth-session-browser-security.md:
// a store failure must surface as an error, never as a fabricated empty-but-valid session.
func TestStore_GetSessionReturnsErrorWhenStoreUnavailable(t *testing.T) {
	s := newTestStore(t, unavailableBackend{})

	_, err := s.GetSession(context.Background(), "any-id")
	if err == nil {
		t.Fatal("GetSession() succeeded against an unavailable store, want error")
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("GetSession() returned ErrNotFound for a store failure; callers can't distinguish this from an expired session, but must still deny access either way")
	}
}

func TestStore_TakeLoginTransactionIsSingleUse(t *testing.T) {
	s := newTestStore(t, newFakeBackend())
	txn := LoginTransaction{Nonce: "n1", PKCEVerifier: "v1"}

	state, err := s.CreateLoginTransaction(context.Background(), txn, 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateLoginTransaction: %v", err)
	}

	got, err := s.TakeLoginTransaction(context.Background(), state)
	if err != nil {
		t.Fatalf("first TakeLoginTransaction: %v", err)
	}
	if got.Nonce != txn.Nonce || got.PKCEVerifier != txn.PKCEVerifier {
		t.Errorf("TakeLoginTransaction() = %+v, want %+v", got, txn)
	}

	if _, err := s.TakeLoginTransaction(context.Background(), state); !errors.Is(err, ErrNotFound) {
		t.Errorf("second TakeLoginTransaction() = %v, want ErrNotFound (replay of a consumed state must fail)", err)
	}
}

func TestStore_TakeLoginTransactionFailsForUnknownState(t *testing.T) {
	s := newTestStore(t, newFakeBackend())

	if _, err := s.TakeLoginTransaction(context.Background(), "never-issued"); !errors.Is(err, ErrNotFound) {
		t.Errorf("TakeLoginTransaction() = %v, want ErrNotFound", err)
	}
}

func TestNewStore_RejectsInvalidEncryptionKey(t *testing.T) {
	_, err := NewStore(newFakeBackend(), []byte("too-short"))
	if err == nil {
		t.Fatal("NewStore() succeeded with an invalid key, want error")
	}
}

func TestStore_CreateSessionReturnsErrorWhenBackendFails(t *testing.T) {
	s := newTestStore(t, unavailableBackend{})

	if _, err := s.CreateSession(context.Background(), Record{Subject: "alice"}, time.Hour); err == nil {
		t.Fatal("CreateSession() succeeded against an unavailable backend, want error")
	}
}

func TestStore_CreateLoginTransactionReturnsErrorWhenBackendFails(t *testing.T) {
	s := newTestStore(t, unavailableBackend{})

	if _, err := s.CreateLoginTransaction(context.Background(), LoginTransaction{Nonce: "n"}, time.Minute); err == nil {
		t.Fatal("CreateLoginTransaction() succeeded against an unavailable backend, want error")
	}
}

// GetSession must error, not panic or return a zero-value Record silently,
// when the stored bytes don't decrypt/unmarshal into a Record — e.g. data
// corruption or a key mismatch after rotation.
func TestStore_GetSessionErrorsOnCorruptStoredRecord(t *testing.T) {
	b := newFakeBackend()
	s := newTestStore(t, b)
	sealed, err := s.enc.seal([]byte("not valid json"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if err := b.Set(context.Background(), sessionKeyPrefix+"broken", sealed, time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if _, err := s.GetSession(context.Background(), "broken"); err == nil {
		t.Fatal("GetSession() succeeded on a corrupt record, want error")
	}
}

func TestStore_TakeLoginTransactionErrorsOnCorruptStoredRecord(t *testing.T) {
	b := newFakeBackend()
	s := newTestStore(t, b)
	sealed, err := s.enc.seal([]byte("not valid json"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if err := b.Set(context.Background(), loginKeyPrefix+"broken", sealed, time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if _, err := s.TakeLoginTransaction(context.Background(), "broken"); err == nil {
		t.Fatal("TakeLoginTransaction() succeeded on a corrupt record, want error")
	}
}

// put must surface a marshal error rather than silently storing nothing -
// e.g. a future Record field of an unmarshalable type.
func TestStore_PutReturnsErrorWhenValueCannotBeMarshaled(t *testing.T) {
	s := newTestStore(t, newFakeBackend())

	if err := s.put(context.Background(), "k", make(chan int), time.Hour); err == nil {
		t.Fatal("put() succeeded marshaling an unmarshalable value, want error")
	}
}

func TestStore_PutReturnsErrorWhenSealFails(t *testing.T) {
	s := newTestStore(t, newFakeBackend())
	orig := randReader
	randReader = failingRandReader{}
	defer func() { randReader = orig }()

	if err := s.put(context.Background(), "k", Record{Subject: "alice"}, time.Hour); err == nil {
		t.Fatal("put() succeeded with a failing rand reader, want error")
	}
}

func TestStore_CreateSessionReturnsErrorWhenIDGenerationFails(t *testing.T) {
	s := newTestStore(t, newFakeBackend())
	orig := randReader
	randReader = failingRandReader{}
	defer func() { randReader = orig }()

	if _, err := s.CreateSession(context.Background(), Record{Subject: "alice"}, time.Hour); err == nil {
		t.Fatal("CreateSession() succeeded with a failing rand reader, want error")
	}
}

func TestStore_CreateLoginTransactionReturnsErrorWhenIDGenerationFails(t *testing.T) {
	s := newTestStore(t, newFakeBackend())
	orig := randReader
	randReader = failingRandReader{}
	defer func() { randReader = orig }()

	if _, err := s.CreateLoginTransaction(context.Background(), LoginTransaction{Nonce: "n"}, time.Minute); err == nil {
		t.Fatal("CreateLoginTransaction() succeeded with a failing rand reader, want error")
	}
}

// Distinct from the "corrupt JSON" case above: this is bytes that aren't
// even valid ciphertext (too short to contain a nonce), exercising enc.open's
// own error return from inside GetSession/TakeLoginTransaction, not json.Unmarshal's.
func TestStore_GetSessionErrorsOnUndecryptableBytes(t *testing.T) {
	b := newFakeBackend()
	s := newTestStore(t, b)
	if err := b.Set(context.Background(), sessionKeyPrefix+"broken", []byte("x"), time.Hour); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if _, err := s.GetSession(context.Background(), "broken"); err == nil {
		t.Fatal("GetSession() succeeded on undecryptable bytes, want error")
	}
}

func TestStore_TakeLoginTransactionErrorsOnUndecryptableBytes(t *testing.T) {
	b := newFakeBackend()
	s := newTestStore(t, b)
	if err := b.Set(context.Background(), loginKeyPrefix+"broken", []byte("x"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if _, err := s.TakeLoginTransaction(context.Background(), "broken"); err == nil {
		t.Fatal("TakeLoginTransaction() succeeded on undecryptable bytes, want error")
	}
}

// failingRandReader always errors, simulating crypto/rand.Read failure -
// exercises generateID's error path, which CreateSession/CreateLoginTransaction
// depend on.
type failingRandReader struct{}

func (failingRandReader) Read([]byte) (int, error) {
	return 0, errStoreUnavailable // any non-nil error; unrelated to the store, reused for brevity
}

func TestGenerateID_ReturnsErrorWhenRandReaderFails(t *testing.T) {
	orig := randReader
	randReader = failingRandReader{}
	defer func() { randReader = orig }()

	if _, err := generateID(); err == nil {
		t.Fatal("generateID() succeeded with a failing rand reader, want error")
	}
}
