// infrastructure/db/encryption_test.go
//
// Per-user brokerage and Plaid credentials are stored AES-256-GCM encrypted.
// These tests lock in the round-trip contract and the authentication guarantees
// (tampered or malformed ciphertext must never decrypt silently).
//
// getEncryptionKey caches the key via sync.Once, so a valid key is seeded in
// TestMain before any test calls Encrypt/Decrypt.
package db

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("encryption_test: generate key: " + err.Error())
	}
	os.Setenv("PLAID_TOKEN_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
	os.Exit(m.Run())
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		plaintext string
	}{
		{"typical_token", "access-sandbox-1a2b3c4d-5e6f-7g8h"},
		{"empty_string", ""},
		{"unicode", "café—naïve—😀"},
		{"long_value", string(make([]byte, 4096))},
		{"whitespace_only", "   \t\n"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ciphertext, err := EncryptToken(tc.plaintext)
			if err != nil {
				t.Fatalf("EncryptToken() error = %v", err)
			}
			got, err := DecryptToken(ciphertext)
			if err != nil {
				t.Fatalf("DecryptToken() error = %v", err)
			}
			if got != tc.plaintext {
				t.Errorf("round-trip = %q, want %q", got, tc.plaintext)
			}
		})
	}
}

func TestEncryptTokenDoesNotLeakPlaintext(t *testing.T) {
	t.Parallel()

	plaintext := "super-secret-access-token"
	ciphertext, err := EncryptToken(plaintext)
	if err != nil {
		t.Fatalf("EncryptToken() error = %v", err)
	}
	if ciphertext == plaintext {
		t.Fatal("ciphertext equals plaintext")
	}
	// The base64 blob must not contain the raw plaintext bytes.
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatalf("ciphertext is not valid base64: %v", err)
	}
	if got := string(decoded); got == plaintext || contains(decoded, []byte(plaintext)) {
		t.Error("decoded ciphertext contains the plaintext")
	}
}

func TestEncryptTokenIsNonDeterministic(t *testing.T) {
	t.Parallel()

	const plaintext = "same-input-every-time"
	first, err := EncryptToken(plaintext)
	if err != nil {
		t.Fatalf("EncryptToken() error = %v", err)
	}
	second, err := EncryptToken(plaintext)
	if err != nil {
		t.Fatalf("EncryptToken() error = %v", err)
	}
	if first == second {
		t.Error("encrypting the same plaintext twice produced identical ciphertext (nonce not random)")
	}
	// Both must still decrypt back to the original.
	for _, c := range []string{first, second} {
		got, err := DecryptToken(c)
		if err != nil {
			t.Fatalf("DecryptToken() error = %v", err)
		}
		if got != plaintext {
			t.Errorf("decrypt = %q, want %q", got, plaintext)
		}
	}
}

func TestDecryptTokenRejectsBadInput(t *testing.T) {
	t.Parallel()

	t.Run("invalid_base64", func(t *testing.T) {
		t.Parallel()
		if _, err := DecryptToken("not!valid!base64!"); err == nil {
			t.Error("expected error for invalid base64, got nil")
		}
	})

	t.Run("too_short_blob", func(t *testing.T) {
		t.Parallel()
		short := base64.StdEncoding.EncodeToString([]byte("tiny"))
		if _, err := DecryptToken(short); err == nil {
			t.Error("expected error for too-short ciphertext, got nil")
		}
	})

	t.Run("tampered_ciphertext_fails_authentication", func(t *testing.T) {
		t.Parallel()
		ciphertext, err := EncryptToken("authentic-token")
		if err != nil {
			t.Fatalf("EncryptToken() error = %v", err)
		}
		blob, err := base64.StdEncoding.DecodeString(ciphertext)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		// Flip a bit in the last byte (inside the GCM tag / ciphertext region).
		blob[len(blob)-1] ^= 0x01
		tampered := base64.StdEncoding.EncodeToString(blob)
		if _, err := DecryptToken(tampered); err == nil {
			t.Error("expected authentication error for tampered ciphertext, got nil")
		}
	})
}

// contains reports whether sub appears within b.
func contains(b, sub []byte) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(b); i++ {
		match := true
		for j := range sub {
			if b[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
