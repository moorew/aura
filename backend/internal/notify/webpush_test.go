package notify

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"testing"

	"github.com/clevercode/sempa/internal/db"
)

// TestWebPushEncryptRoundTrip exercises the RFC 8291 (aes128gcm) encryption by
// decrypting a server-produced payload exactly as a user agent would, proving
// the hand-rolled key derivation and framing are correct.
func TestWebPushEncryptRoundTrip(t *testing.T) {
	curve := ecdh.P256()

	// Simulate the browser: a UA key pair + 16-byte auth secret.
	uaPriv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	uaPublic := uaPriv.PublicKey().Bytes()
	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatal(err)
	}

	keys, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	sender := NewWebPushSender(keys, "mailto:test@example.com")

	sub := db.PushSubscription{
		Endpoint: "https://push.example.com/abc",
		P256dh:   b64.EncodeToString(uaPublic),
		Auth:     b64.EncodeToString(authSecret),
	}
	plaintext := []byte(`{"title":"Reminder","body":"Finish the report"}`)

	body, asPublic, err := sender.encrypt(sub, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	got, err := uaDecrypt(body, uaPriv, uaPublic, authSecret)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch:\n got: %s\nwant: %s", got, plaintext)
	}

	// The keyid in the header must be the ephemeral server public key.
	if !bytes.Equal(body[21:21+65], asPublic) {
		t.Fatal("header keyid is not the server public key")
	}
}

// TestVAPIDAuthHeader checks that the VAPID Authorization header is well-formed.
func TestVAPIDAuthHeader(t *testing.T) {
	keys, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	sender := NewWebPushSender(keys, "mailto:test@example.com")
	auth, err := sender.vapidAuth("https://push.example.com/fcm/send/xyz")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix([]byte(auth), []byte("vapid t=")) {
		t.Fatalf("unexpected auth header: %q", auth)
	}
}

// uaDecrypt reverses the RFC 8291 / RFC 8188 steps from the user-agent side.
func uaDecrypt(body []byte, uaPriv *ecdh.PrivateKey, uaPublic, authSecret []byte) ([]byte, error) {
	salt := body[0:16]
	idlen := int(body[20])
	asPublicBytes := body[21 : 21+idlen]
	ciphertext := body[21+idlen:]

	asPublic, err := ecdh.P256().NewPublicKey(asPublicBytes)
	if err != nil {
		return nil, err
	}
	ecdhSecret, err := uaPriv.ECDH(asPublic)
	if err != nil {
		return nil, err
	}

	prkKey, _ := hkdf.Extract(sha256.New, ecdhSecret, authSecret)
	keyInfo := append([]byte("WebPush: info\x00"), uaPublic...)
	keyInfo = append(keyInfo, asPublicBytes...)
	ikm, _ := hkdf.Expand(sha256.New, prkKey, string(keyInfo), 32)

	prk, _ := hkdf.Extract(sha256.New, ikm, salt)
	cek, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	out, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return bytes.TrimRight(out, "\x02"), nil
}
