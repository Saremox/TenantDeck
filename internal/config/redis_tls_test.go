package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestCAFile(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}

	path := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestBuildRedisTLSConfig_ReturnsNilWhenTLSDisabled(t *testing.T) {
	s := SessionConfig{RedisTLS: false, RedisCAFile: "irrelevant"}

	tlsConfig, err := s.BuildRedisTLSConfig()
	if err != nil {
		t.Fatalf("BuildRedisTLSConfig: %v", err)
	}
	if tlsConfig != nil {
		t.Errorf("tlsConfig = %v, want nil", tlsConfig)
	}
}

func TestBuildRedisTLSConfig_ReturnsEmptyConfigWhenNoCAFileGiven(t *testing.T) {
	s := SessionConfig{RedisTLS: true}

	tlsConfig, err := s.BuildRedisTLSConfig()
	if err != nil {
		t.Fatalf("BuildRedisTLSConfig: %v", err)
	}
	if tlsConfig == nil {
		t.Fatal("tlsConfig = nil, want a non-nil *tls.Config using the system pool")
	}
	if tlsConfig.RootCAs != nil {
		t.Error("RootCAs set with no CA file configured, want nil (system pool)")
	}
}

func TestBuildRedisTLSConfig_LoadsConfiguredCAFile(t *testing.T) {
	s := SessionConfig{RedisTLS: true, RedisCAFile: writeTestCAFile(t)}

	tlsConfig, err := s.BuildRedisTLSConfig()
	if err != nil {
		t.Fatalf("BuildRedisTLSConfig: %v", err)
	}
	if tlsConfig.RootCAs == nil {
		t.Fatal("RootCAs = nil, want the parsed CA pool")
	}
}

func TestBuildRedisTLSConfig_ErrorsWhenCAFileIsUnreadable(t *testing.T) {
	s := SessionConfig{RedisTLS: true, RedisCAFile: filepath.Join(t.TempDir(), "does-not-exist.pem")}

	if _, err := s.BuildRedisTLSConfig(); err == nil {
		t.Fatal("BuildRedisTLSConfig succeeded with an unreadable CA file, want error")
	}
}

func TestBuildRedisTLSConfig_ErrorsWhenCAFileHasNoValidCertificates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garbage.pem")
	if err := os.WriteFile(path, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s := SessionConfig{RedisTLS: true, RedisCAFile: path}

	if _, err := s.BuildRedisTLSConfig(); err == nil {
		t.Fatal("BuildRedisTLSConfig succeeded with a CA file containing no certificates, want error")
	}
}
