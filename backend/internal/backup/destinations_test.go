package backup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLocalDestinationRoundTrip(t *testing.T) {
	dir := t.TempDir()
	d := &localDest{path: filepath.Join(dir, "backups")}
	src := writeTempFile(t, "sempa-backup-2026-01-01-000000.zip", "data")
	ctx := context.Background()

	if err := d.Put(ctx, "sempa-backup-2026-01-01-000000.zip", src); err != nil {
		t.Fatal(err)
	}
	if err := d.Put(ctx, "sempa-backup-2026-01-02-000000.zip", src); err != nil {
		t.Fatal(err)
	}
	files, err := d.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	// Newest first.
	if files[0].Name != "sempa-backup-2026-01-02-000000.zip" {
		t.Fatalf("expected newest first, got %s", files[0].Name)
	}
	if err := d.Delete(ctx, files[0].ID); err != nil {
		t.Fatal(err)
	}
	files, _ = d.List(ctx)
	if len(files) != 1 {
		t.Fatalf("expected 1 file after delete, got %d", len(files))
	}
}

func TestWebDAVDestination(t *testing.T) {
	var putBody, gotMethodPut, gotMethodDelete string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			gotMethodPut = r.URL.Path
			b := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(b)
			putBody = string(b)
			w.WriteHeader(201)
		case "PROPFIND":
			w.WriteHeader(207)
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
				<d:multistatus xmlns:d="DAV:">
				  <d:response><d:href>/dav/sempa-backup-2026-01-01-000000.zip</d:href>
				    <d:propstat><d:prop><d:getlastmodified>Wed, 01 Jan 2026 00:00:00 GMT</d:getlastmodified></d:prop></d:propstat>
				  </d:response>
				  <d:response><d:href>/dav/other.txt</d:href></d:response>
				</d:multistatus>`))
		case http.MethodDelete:
			gotMethodDelete = r.URL.Path
			w.WriteHeader(204)
		}
	}))
	defer srv.Close()

	d := &webdavDest{baseURL: srv.URL + "/dav", user: "me", pass: "pw"}
	ctx := context.Background()
	src := writeTempFile(t, "b.zip", "hello-webdav")

	if err := d.Put(ctx, "sempa-backup-2026-01-01-000000.zip", src); err != nil {
		t.Fatal(err)
	}
	if putBody != "hello-webdav" {
		t.Fatalf("put body mismatch: %q", putBody)
	}
	if !strings.HasSuffix(gotMethodPut, "/dav/sempa-backup-2026-01-01-000000.zip") {
		t.Fatalf("unexpected PUT path: %s", gotMethodPut)
	}
	files, err := d.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "sempa-backup-2026-01-01-000000.zip" {
		t.Fatalf("unexpected list result: %+v", files)
	}
	if err := d.Delete(ctx, files[0].ID); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(gotMethodDelete, "/sempa-backup-2026-01-01-000000.zip") {
		t.Fatalf("unexpected DELETE path: %s", gotMethodDelete)
	}
}

func TestS3DestinationSignsAndLists(t *testing.T) {
	var sawAuth, sawSha string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		sawSha = r.Header.Get("X-Amz-Content-Sha256")
		switch r.Method {
		case http.MethodPut:
			w.WriteHeader(200)
		case http.MethodGet:
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
				<ListBucketResult><Contents><Key>sempa/sempa-backup-2026-01-01-000000.zip</Key>
				<LastModified>2026-01-01T00:00:00.000Z</LastModified></Contents></ListBucketResult>`))
		case http.MethodDelete:
			w.WriteHeader(204)
		}
	}))
	defer srv.Close()

	d := &s3Dest{
		bucket: "mybucket", region: "us-east-1", prefix: "sempa",
		endpoint: srv.URL, accessKey: "AKIA", secretKey: "secret",
	}
	ctx := context.Background()
	src := writeTempFile(t, "b.zip", "s3-data")

	if err := d.Put(ctx, "sempa-backup-2026-01-01-000000.zip", src); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sawAuth, "AWS4-HMAC-SHA256 Credential=AKIA/") {
		t.Fatalf("missing/invalid SigV4 auth header: %q", sawAuth)
	}
	if sawSha != unsignedPayload {
		t.Fatalf("expected unsigned payload for PUT, got %q", sawSha)
	}
	files, err := d.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != "sempa-backup-2026-01-01-000000.zip" {
		t.Fatalf("unexpected list: %+v", files)
	}
	if files[0].ID != "sempa/sempa-backup-2026-01-01-000000.zip" {
		t.Fatalf("expected full key as ID, got %s", files[0].ID)
	}
	if err := d.Delete(ctx, files[0].ID); err != nil {
		t.Fatal(err)
	}
}
