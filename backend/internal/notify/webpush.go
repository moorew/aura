package notify

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"github.com/clevercode/sempa/internal/db"
)

// This file implements the W3C Web Push protocol from scratch using only the Go
// standard library, matching the repo's no-dependency ethos (see jwt.go for the
// same approach applied to FCM). It covers:
//
//   - VAPID (RFC 8292): an ES256 JWT identifying this application server.
//   - Message encryption (RFC 8291) with the aes128gcm content coding (RFC 8188):
//     ECDH(P-256) → HKDF-SHA256 → AES-128-GCM.
//
// Go 1.24+ provides crypto/hkdf and crypto/ecdh in the standard library, so no
// third-party crypto is needed.

// VAPIDKeys is the application-server identity used to authenticate push
// requests. Public is the uncompressed P-256 point (base64url) the browser uses
// as applicationServerKey; Private is the raw scalar D (base64url).
type VAPIDKeys struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

// GenerateVAPIDKeys creates a fresh P-256 VAPID key pair. Called once on first
// boot; the result is persisted in integration_configs.
func GenerateVAPIDKeys() (VAPIDKeys, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return VAPIDKeys{}, err
	}
	pub := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y) //nolint:staticcheck // uncompressed point is exactly what the Push API wants
	return VAPIDKeys{
		Public:  b64.EncodeToString(pub),
		Private: b64.EncodeToString(priv.D.Bytes()),
	}, nil
}

var b64 = base64.RawURLEncoding

// WebPushSender sends encrypted payloads to browser push services.
type WebPushSender struct {
	keys    VAPIDKeys
	subject string // "mailto:..." contact for the VAPID JWT `sub` claim
	client  *http.Client
}

func NewWebPushSender(keys VAPIDKeys, subject string) *WebPushSender {
	if subject == "" {
		subject = "mailto:admin@localhost"
	}
	return &WebPushSender{
		keys:    keys,
		subject: subject,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// ErrSubscriptionGone signals a 404/410 from the push service — the caller
// should delete the dead subscription.
type webPushError struct {
	status int
	body   string
}

func (e *webPushError) Error() string { return fmt.Sprintf("web push %d: %s", e.status, e.body) }

func isSubscriptionGone(err error) bool {
	if e, ok := err.(*webPushError); ok {
		return e.status == http.StatusNotFound || e.status == http.StatusGone
	}
	return false
}

// Send encrypts payload for one subscription and POSTs it to the push endpoint.
// ttlSeconds controls how long the push service retains the message if the
// device is offline.
func (s *WebPushSender) Send(sub db.PushSubscription, payload []byte, ttlSeconds int) error {
	body, asPublic, err := s.encrypt(sub, payload)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	_ = asPublic // included inside the encrypted header

	auth, err := s.vapidAuth(sub.Endpoint)
	if err != nil {
		return fmt.Errorf("vapid: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", fmt.Sprintf("%d", ttlSeconds))
	req.Header.Set("Urgency", "high")
	req.Header.Set("Authorization", auth)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return &webPushError{status: resp.StatusCode, body: string(b)}
	}
	return nil
}

// encrypt implements RFC 8291 message encryption with the aes128gcm coding.
// Returns the full body (header || ciphertext) and the ephemeral server public
// key (already embedded in the header, returned for clarity/testing).
func (s *WebPushSender) encrypt(sub db.PushSubscription, payload []byte) (body, asPublic []byte, err error) {
	uaPublicBytes, err := b64.DecodeString(sub.P256dh)
	if err != nil {
		return nil, nil, fmt.Errorf("decode p256dh: %w", err)
	}
	authSecret, err := b64.DecodeString(sub.Auth)
	if err != nil {
		return nil, nil, fmt.Errorf("decode auth: %w", err)
	}

	curve := ecdh.P256()
	uaPublic, err := curve.NewPublicKey(uaPublicBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ua public key: %w", err)
	}

	// Ephemeral application-server key pair (fresh per message).
	asPriv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	asPublicBytes := asPriv.PublicKey().Bytes() // 65-byte uncompressed point

	ecdhSecret, err := asPriv.ECDH(uaPublic)
	if err != nil {
		return nil, nil, fmt.Errorf("ecdh: %w", err)
	}

	// RFC 8291 §3.4 key derivation.
	prkKey, err := hkdf.Extract(sha256.New, ecdhSecret, authSecret)
	if err != nil {
		return nil, nil, err
	}
	keyInfo := append([]byte("WebPush: info\x00"), uaPublicBytes...)
	keyInfo = append(keyInfo, asPublicBytes...)
	ikm, err := hkdf.Expand(sha256.New, prkKey, string(keyInfo), 32)
	if err != nil {
		return nil, nil, err
	}

	// aes128gcm content coding (RFC 8188) keys, salted with a random 16 bytes.
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, nil, err
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, nil, err
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	// Single record: plaintext = payload || 0x02 (last-record delimiter).
	plaintext := append(append([]byte{}, payload...), 0x02)
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// aes128gcm header: salt(16) | rs(4) | idlen(1) | keyid(=as public key).
	const recordSize = 4096
	header := make([]byte, 0, 16+4+1+len(asPublicBytes))
	header = append(header, salt...)
	rs := make([]byte, 4)
	binary.BigEndian.PutUint32(rs, recordSize)
	header = append(header, rs...)
	header = append(header, byte(len(asPublicBytes)))
	header = append(header, asPublicBytes...)

	return append(header, ciphertext...), asPublicBytes, nil
}

// vapidAuth builds the VAPID Authorization header (RFC 8292): an ES256 JWT
// asserting this server's identity, plus the public key.
func (s *WebPushSender) vapidAuth(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	aud := u.Scheme + "://" + u.Host

	header := b64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims, _ := json.Marshal(map[string]any{
		"aud": aud,
		"exp": time.Now().Add(12 * time.Hour).Unix(),
		"sub": s.subject,
	})
	signingInput := header + "." + b64.EncodeToString(claims)

	priv, err := s.privateKey()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(signingInput))
	r, ss, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		return "", err
	}
	// JWS ES256 signature is the fixed-width concatenation r||s, not ASN.1 DER.
	sig := append(leftPad(r.Bytes(), 32), leftPad(ss.Bytes(), 32)...)
	jwt := signingInput + "." + b64.EncodeToString(sig)

	return "vapid t=" + jwt + ", k=" + s.keys.Public, nil
}

func (s *WebPushSender) privateKey() (*ecdsa.PrivateKey, error) {
	d, err := b64.DecodeString(s.keys.Private)
	if err != nil {
		return nil, fmt.Errorf("decode vapid private key: %w", err)
	}
	priv := new(ecdsa.PrivateKey)
	priv.Curve = elliptic.P256()
	priv.D = new(big.Int).SetBytes(d)
	priv.PublicKey.X, priv.PublicKey.Y = priv.Curve.ScalarBaseMult(d)
	return priv, nil
}

func leftPad(b []byte, size int) []byte {
	if len(b) >= size {
		return b
	}
	out := make([]byte, size)
	copy(out[size-len(b):], b)
	return out
}

// VAPIDPublicKey returns the base64url public key the frontend feeds to
// pushManager.subscribe() as applicationServerKey.
func (s *WebPushSender) VAPIDPublicKey() string { return s.keys.Public }
