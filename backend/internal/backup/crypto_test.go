package backup

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	sizes := []int{
		0,
		1,
		chunkSize - 1,
		chunkSize,
		chunkSize + 1,
		3 * chunkSize,
		3*chunkSize + 123,
		1 << 20, // 1 MiB
	}
	for _, n := range sizes {
		plain := make([]byte, n)
		if _, err := rand.Read(plain); err != nil {
			t.Fatal(err)
		}
		var enc bytes.Buffer
		if err := EncryptStream(&enc, bytes.NewReader(plain), "correct horse"); err != nil {
			t.Fatalf("size %d: encrypt: %v", n, err)
		}
		if !LooksEncrypted(enc.Bytes()[:len(magic)]) {
			t.Fatalf("size %d: magic not detected", n)
		}
		var dec bytes.Buffer
		if err := DecryptStream(&dec, bytes.NewReader(enc.Bytes()), "correct horse"); err != nil {
			t.Fatalf("size %d: decrypt: %v", n, err)
		}
		if !bytes.Equal(plain, dec.Bytes()) {
			t.Fatalf("size %d: round trip mismatch", n)
		}
	}
}

func TestDecryptWrongPassphrase(t *testing.T) {
	plain := []byte("secret data here")
	var enc bytes.Buffer
	if err := EncryptStream(&enc, bytes.NewReader(plain), "right"); err != nil {
		t.Fatal(err)
	}
	var dec bytes.Buffer
	if err := DecryptStream(&dec, bytes.NewReader(enc.Bytes()), "wrong"); err == nil {
		t.Fatal("expected decryption to fail with wrong passphrase")
	}
}

func TestDecryptTruncated(t *testing.T) {
	plain := make([]byte, 3*chunkSize)
	_, _ = rand.Read(plain)
	var enc bytes.Buffer
	if err := EncryptStream(&enc, bytes.NewReader(plain), "pw"); err != nil {
		t.Fatal(err)
	}
	truncated := enc.Bytes()[:enc.Len()-100]
	var dec bytes.Buffer
	if err := DecryptStream(&dec, bytes.NewReader(truncated), "pw"); err == nil {
		t.Fatal("expected decryption of truncated stream to fail")
	}
}
