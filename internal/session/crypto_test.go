package session

import "testing"

func testKey() []byte {
	return []byte("01234567890123456789012345678901"[:32])
}

func TestEncryptor_OpenRecoversOriginalPlaintext(t *testing.T) {
	enc, err := newEncryptor(testKey())
	if err != nil {
		t.Fatalf("newEncryptor: %v", err)
	}
	want := []byte(`{"subject":"alice"}`)

	sealed, err := enc.seal(want)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	got, err := enc.open(sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("open() = %q, want %q", got, want)
	}
}

func TestEncryptor_OpenRejectsTamperedCiphertext(t *testing.T) {
	enc, err := newEncryptor(testKey())
	if err != nil {
		t.Fatalf("newEncryptor: %v", err)
	}
	sealed, err := enc.seal([]byte("secret"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	sealed[len(sealed)-1] ^= 0xFF // flip a bit in the authenticated ciphertext

	if _, err := enc.open(sealed); err == nil {
		t.Fatal("open() succeeded on tampered ciphertext, want error")
	}
}

func TestEncryptor_OpenRejectsWrongKey(t *testing.T) {
	enc1, _ := newEncryptor(testKey())
	otherKey := []byte("98765432109876543210987654321098"[:32])
	enc2, _ := newEncryptor(otherKey)

	sealed, err := enc1.seal([]byte("secret"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := enc2.open(sealed); err == nil {
		t.Fatal("open() succeeded with the wrong key, want error")
	}
}

func TestNewEncryptor_RejectsWrongKeyLength(t *testing.T) {
	if _, err := newEncryptor([]byte("too-short")); err == nil {
		t.Fatal("newEncryptor() succeeded with a short key, want error")
	}
}

func TestEncryptor_OpenRejectsCiphertextShorterThanNonce(t *testing.T) {
	enc, err := newEncryptor(testKey())
	if err != nil {
		t.Fatalf("newEncryptor: %v", err)
	}
	if _, err := enc.open([]byte("short")); err == nil {
		t.Fatal("open() succeeded on ciphertext shorter than the nonce, want error")
	}
}

func TestEncryptor_SealReturnsErrorWhenRandReaderFails(t *testing.T) {
	orig := randReader
	randReader = failingRandReader{}
	defer func() { randReader = orig }()

	enc, err := newEncryptor(testKey())
	if err != nil {
		t.Fatalf("newEncryptor: %v", err)
	}
	if _, err := enc.seal([]byte("secret")); err == nil {
		t.Fatal("seal() succeeded with a failing rand reader, want error")
	}
}
