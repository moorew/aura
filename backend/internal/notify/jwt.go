package notify

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// serviceAccountToken exchanges a service account JSON key for an OAuth2 access token
// using the JWT Bearer assertion flow, without any external Google SDK dependency.
func serviceAccountToken(keyPath string) (token string, expiry time.Time, err error) {
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("read key file: %w", err)
	}

	var sa struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal(data, &sa); err != nil {
		return "", time.Time{}, fmt.Errorf("parse key file: %w", err)
	}
	if sa.TokenURI == "" {
		sa.TokenURI = "https://oauth2.googleapis.com/token"
	}

	now := time.Now()
	exp := now.Add(55 * time.Minute) // tokens last 1h, refresh at 55m

	// Build JWT
	header := base64url(mustJSON(map[string]string{"alg": "RS256", "typ": "JWT"}))
	claims := base64url(mustJSON(map[string]any{
		"iss":   sa.ClientEmail,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   sa.TokenURI,
		"iat":   now.Unix(),
		"exp":   exp.Unix(),
	}))
	unsigned := header + "." + claims

	// Sign with RSA
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return "", time.Time{}, fmt.Errorf("failed to decode PEM private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return "", time.Time{}, fmt.Errorf("private key is not RSA")
	}

	hash := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(nil, rsaKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign JWT: %w", err)
	}

	jwt := unsigned + "." + base64url(sig)

	// Exchange JWT for access token
	resp, err := http.PostForm(sa.TokenURI, url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {jwt},
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", time.Time{}, fmt.Errorf("token exchange %d: %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", time.Time{}, fmt.Errorf("parse token response: %w", err)
	}

	return tokenResp.AccessToken, exp, nil
}

func base64url(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
