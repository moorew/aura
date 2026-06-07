package backup

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// s3Dest uploads backups to any S3-compatible store (AWS S3, MinIO, Backblaze
// B2, etc.) using raw HTTP + AWS Signature V4 — no SDK dependency. Path-style
// addressing is used for broad compatibility.
type s3Dest struct {
	bucket    string
	region    string
	prefix    string
	endpoint  string // optional custom endpoint; empty = AWS
	accessKey string
	secretKey string
}

const unsignedPayload = "UNSIGNED-PAYLOAD"
const emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func (s *s3Dest) host() string {
	if s.endpoint != "" {
		u, err := url.Parse(s.endpoint)
		if err == nil && u.Host != "" {
			return u.Host
		}
		return strings.TrimPrefix(strings.TrimPrefix(s.endpoint, "https://"), "http://")
	}
	return fmt.Sprintf("s3.%s.amazonaws.com", s.region)
}

func (s *s3Dest) scheme() string {
	if strings.HasPrefix(s.endpoint, "http://") {
		return "http"
	}
	return "https"
}

func (s *s3Dest) keyName(filename string) string {
	p := strings.Trim(s.prefix, "/")
	if p == "" {
		return filename
	}
	return p + "/" + filename
}

func (s *s3Dest) objectURL(key string) string {
	return fmt.Sprintf("%s://%s/%s/%s", s.scheme(), s.host(), s.bucket, s3EscapePath(key))
}

func (s *s3Dest) Put(ctx context.Context, filename, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	key := s.keyName(filename)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.objectURL(key), f)
	if err != nil {
		return err
	}
	req.ContentLength = fi.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	s.sign(req, unsignedPayload)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("s3 PUT failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

type s3ListResult struct {
	Contents []struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
	} `xml:"Contents"`
}

func (s *s3Dest) List(ctx context.Context) ([]RemoteFile, error) {
	q := url.Values{}
	q.Set("list-type", "2")
	prefix := strings.Trim(s.prefix, "/")
	listPrefix := backupFilePrefix
	if prefix != "" {
		listPrefix = prefix + "/" + backupFilePrefix
	}
	q.Set("prefix", listPrefix)

	rawURL := fmt.Sprintf("%s://%s/%s?%s", s.scheme(), s.host(), s.bucket, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	s.sign(req, emptyPayloadHash)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("s3 LIST failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var lr s3ListResult
	if err := xml.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, err
	}
	var out []RemoteFile
	for _, c := range lr.Contents {
		name := c.Key
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		if !strings.HasPrefix(name, backupFilePrefix) {
			continue
		}
		out = append(out, RemoteFile{ID: c.Key, Name: name, Modified: c.LastModified})
	}
	sortRemoteNewestFirst(out)
	return out, nil
}

func (s *s3Dest) Delete(ctx context.Context, id string) error {
	// id is the full object key.
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.objectURL(id), nil)
	if err != nil {
		return err
	}
	s.sign(req, emptyPayloadHash)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("s3 DELETE failed: HTTP %d", resp.StatusCode)
	}
	return nil
}

// sign applies AWS Signature V4 to req using the given payload hash.
func (s *s3Dest) sign(req *http.Request, payloadHash string) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	// Canonical headers (host, x-amz-content-sha256, x-amz-date).
	signed := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	sort.Strings(signed)
	var canonHeaders strings.Builder
	for _, h := range signed {
		var v string
		switch h {
		case "host":
			v = req.URL.Host
		default:
			v = req.Header.Get(h)
		}
		canonHeaders.WriteString(h)
		canonHeaders.WriteString(":")
		canonHeaders.WriteString(strings.TrimSpace(v))
		canonHeaders.WriteString("\n")
	}
	signedHeaders := strings.Join(signed, ";")

	canonicalQuery := canonicalizeQuery(req.URL.Query())

	canonicalRequest := strings.Join([]string{
		req.Method,
		s3CanonicalURI(req.URL.Path),
		canonicalQuery,
		canonHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")

	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := strings.Join([]string{dateStamp, s.region, "s3", "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		algorithm,
		amzDate,
		credentialScope,
		hashHex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := hmacSHA256(hmacSHA256(hmacSHA256(hmacSHA256([]byte("AWS4"+s.secretKey), []byte(dateStamp)), []byte(s.region)), []byte("s3")), []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	auth := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, s.accessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", auth)
}

func canonicalizeQuery(q url.Values) string {
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vals := q[k]
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, s3Escape(k)+"="+s3Escape(v))
		}
	}
	return strings.Join(parts, "&")
}

// s3CanonicalURI URI-encodes each path segment while preserving slashes.
func s3CanonicalURI(p string) string {
	if p == "" {
		return "/"
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = s3Escape(s)
	}
	return strings.Join(segs, "/")
}

// s3EscapePath encodes a key for use in a URL path (slashes preserved).
func s3EscapePath(key string) string {
	segs := strings.Split(key, "/")
	for i, s := range segs {
		segs[i] = s3Escape(s)
	}
	return strings.Join(segs, "/")
}

// s3Escape implements AWS's RFC 3986 encoding (unreserved chars left alone).
func s3Escape(s string) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(unreserved, c) >= 0 {
			b.WriteByte(c)
		} else {
			b.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return b.String()
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
