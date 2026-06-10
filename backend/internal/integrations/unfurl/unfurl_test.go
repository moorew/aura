package unfurl

import (
	"net/url"
	"testing"
)

func TestParseMeta_OpenGraph(t *testing.T) {
	base, _ := url.Parse("https://docs.google.com/document/d/abc/edit")
	doc := `<html><head>
		<title>Raw &amp; Title</title>
		<meta property="og:title" content="My Spec Doc">
		<meta property="og:description" content="A short &quot;summary&quot; of the doc.">
		<meta property="og:image" content="/thumb.png">
		<meta property="og:site_name" content="Google Docs">
		<link rel="icon" href="//ssl.gstatic.com/favicon.ico">
	</head><body>ignored</body></html>`

	m := parseMeta(doc, base)
	if m.Title != "My Spec Doc" {
		t.Errorf("title = %q, want %q", m.Title, "My Spec Doc")
	}
	if m.Description != `A short "summary" of the doc.` {
		t.Errorf("description = %q", m.Description)
	}
	if m.ImageURL != "https://docs.google.com/thumb.png" {
		t.Errorf("image = %q, want resolved absolute", m.ImageURL)
	}
	if m.SiteName != "Google Docs" {
		t.Errorf("site_name = %q", m.SiteName)
	}
	if m.FaviconURL != "https://ssl.gstatic.com/favicon.ico" {
		t.Errorf("favicon = %q (should resolve scheme-relative)", m.FaviconURL)
	}
}

func TestParseMeta_FallbackTitleAndSite(t *testing.T) {
	base, _ := url.Parse("https://www.example.com/page")
	doc := `<head><title>  Plain Title  </title>
		<meta name="description" content="meta desc"></head>`
	m := parseMeta(doc, base)
	if m.Title != "Plain Title" {
		t.Errorf("fallback title = %q", m.Title)
	}
	if m.Description != "meta desc" {
		t.Errorf("desc = %q", m.Description)
	}
	if m.SiteName != "example.com" {
		t.Errorf("site_name fallback = %q, want example.com", m.SiteName)
	}
}

func TestParseMeta_TwitterImageFallback(t *testing.T) {
	base, _ := url.Parse("https://x.example/article")
	doc := `<head><meta name="twitter:image" content="https://cdn.example/og.jpg"></head>`
	m := parseMeta(doc, base)
	if m.ImageURL != "https://cdn.example/og.jpg" {
		t.Errorf("twitter image = %q", m.ImageURL)
	}
}

func TestValidate_RejectsLoopback(t *testing.T) {
	if _, err := validate("http://localhost:8080/x"); err == nil {
		t.Error("expected loopback to be rejected")
	}
	if _, err := validate("ftp://example.com"); err == nil {
		t.Error("expected non-http scheme to be rejected")
	}
}
