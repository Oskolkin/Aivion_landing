package ozon

import (
	"strings"
	"testing"
)

func TestSecretCodec_PerformanceTokenRoundTrip(t *testing.T) {
	const key = "01234567890123456789012345678901"
	codec, err := NewSecretCodec(key)
	if err != nil {
		t.Fatal(err)
	}
	plain := "performance-bearer-token-not-seller-api-key"
	enc, err := codec.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(enc, plain) {
		t.Fatal("ciphertext must not contain plaintext")
	}
	got, err := codec.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("decrypt: want %q got %q", plain, got)
	}
}
