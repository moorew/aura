package db

import (
	"database/sql"
	"time"
)

// LinkUnfurl is cached Open Graph / link-preview metadata for a single URL.
type LinkUnfurl struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	SiteName    string `json:"site_name"`
	FaviconURL  string `json:"favicon_url"`
	OK          bool   `json:"ok"`
	Status      int    `json:"-"`
	FetchedAt   string `json:"fetched_at"`
}

type UnfurlStore struct {
	db *sql.DB
}

func NewUnfurlStore(database *sql.DB) *UnfurlStore {
	return &UnfurlStore{db: database}
}

func (s *UnfurlStore) Get(url string) (*LinkUnfurl, error) {
	row := s.db.QueryRow(`
		SELECT url, title, description, image_url, site_name, favicon_url, status, ok, fetched_at
		FROM link_unfurls WHERE url = ?`, url)
	var u LinkUnfurl
	var okInt int
	if err := row.Scan(&u.URL, &u.Title, &u.Description, &u.ImageURL, &u.SiteName,
		&u.FaviconURL, &u.Status, &okInt, &u.FetchedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.OK = okInt == 1
	return &u, nil
}

func (s *UnfurlStore) Upsert(u *LinkUnfurl) error {
	okInt := 0
	if u.OK {
		okInt = 1
	}
	if u.FetchedAt == "" {
		u.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(`
		INSERT INTO link_unfurls (url, title, description, image_url, site_name, favicon_url, status, ok, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET
			title = excluded.title, description = excluded.description,
			image_url = excluded.image_url, site_name = excluded.site_name,
			favicon_url = excluded.favicon_url, status = excluded.status,
			ok = excluded.ok, fetched_at = excluded.fetched_at`,
		u.URL, u.Title, u.Description, u.ImageURL, u.SiteName, u.FaviconURL, u.Status, okInt, u.FetchedAt)
	return err
}
