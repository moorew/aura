// Package unfurl fetches a web page and extracts Open Graph / link-preview
// metadata (title, description, thumbnail image, site name, favicon) for use in
// link previews. It validates URLs to prevent SSRF against private networks,
// caps the response size, and follows a limited number of (re-validated)
// redirects.
package unfurl

import (
	"context"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Meta is the metadata extracted from a page. Empty fields mean "not found".
type Meta struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	SiteName    string `json:"site_name"`
	FaviconURL  string `json:"favicon_url"`
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// validate ensures the URL is http(s) and resolves only to public IPs.
func validate(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("URL has no hostname")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return nil, fmt.Errorf("URL resolves to a private/internal address; refusing to fetch")
		}
	}
	return u, nil
}

var (
	metaRe  = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	linkRe  = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	attrRe  = regexp.MustCompile(`(?is)([a-zA-Z_:][a-zA-Z0-9_:.-]*)\s*=\s*("([^"]*)"|'([^']*)')`)
	tagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
)

func tagAttrs(tag string) map[string]string {
	m := map[string]string{}
	for _, a := range attrRe.FindAllStringSubmatch(tag, -1) {
		val := a[3]
		if val == "" {
			val = a[4]
		}
		m[strings.ToLower(a[1])] = val
	}
	return m
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return strings.TrimSpace(s[:n]) + "…"
	}
	return s
}

// Fetch downloads rawURL and returns its preview metadata plus the HTTP status.
func Fetch(ctx context.Context, rawURL string) (*Meta, int, error) {
	u, err := validate(rawURL)
	if err != nil {
		return nil, 0, err
	}

	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("stopped after 5 redirects")
			}
			// Re-validate each redirect target to block SSRF via redirect.
			if _, err := validate(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; SempaBot/1.0; +https://github.com/moorew/sempa)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if ct := strings.ToLower(resp.Header.Get("Content-Type")); ct != "" &&
		!strings.Contains(ct, "html") && !strings.Contains(ct, "xml") {
		return nil, resp.StatusCode, fmt.Errorf("not an HTML page (%s)", ct)
	}

	// OG tags live in <head>; read at most 512 KB and stop at </head>.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	return parseMeta(string(body), resp.Request.URL), resp.StatusCode, nil
}

// parseMeta extracts preview metadata from an HTML document, resolving relative
// image/favicon URLs against base. Split out from Fetch so it's unit-testable
// without a network round-trip.
func parseMeta(doc string, base *url.URL) *Meta {
	if i := strings.Index(strings.ToLower(doc), "</head>"); i > 0 {
		doc = doc[:i]
	}

	og := map[string]string{}
	for _, tag := range metaRe.FindAllString(doc, -1) {
		a := tagAttrs(tag)
		key := a["property"]
		if key == "" {
			key = a["name"]
		}
		content := strings.TrimSpace(html.UnescapeString(a["content"]))
		if key == "" || content == "" {
			continue
		}
		switch strings.ToLower(key) {
		case "og:title":
			og["title"] = content
		case "og:description":
			og["description"] = content
		case "description":
			if og["description"] == "" {
				og["description"] = content
			}
		case "og:image", "og:image:url", "og:image:secure_url":
			if og["image"] == "" {
				og["image"] = content
			}
		case "twitter:image", "twitter:image:src":
			if og["image"] == "" {
				og["image"] = content
			}
		case "og:site_name":
			og["site_name"] = content
		}
	}

	meta := &Meta{
		Title:       og["title"],
		Description: og["description"],
		ImageURL:    og["image"],
		SiteName:    og["site_name"],
	}

	if meta.Title == "" {
		if m := titleRe.FindStringSubmatch(doc); m != nil {
			meta.Title = strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(m[1], "")))
		}
	}
	for _, tag := range linkRe.FindAllString(doc, -1) {
		a := tagAttrs(tag)
		if strings.Contains(strings.ToLower(a["rel"]), "icon") && a["href"] != "" {
			meta.FaviconURL = a["href"]
			break
		}
	}

	// Resolve relative image/favicon URLs against the final page URL.
	meta.ImageURL = resolveRef(base, meta.ImageURL)
	meta.FaviconURL = resolveRef(base, meta.FaviconURL)
	if meta.SiteName == "" && base != nil {
		meta.SiteName = strings.TrimPrefix(base.Hostname(), "www.")
	}
	meta.Title = clip(meta.Title, 300)
	meta.Description = clip(meta.Description, 500)

	return meta
}

func resolveRef(base *url.URL, ref string) string {
	if ref == "" {
		return ""
	}
	r, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return ""
	}
	if base == nil {
		return r.String()
	}
	return base.ResolveReference(r).String()
}
