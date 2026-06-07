package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RemoteFile is a backup file that lives at a destination.
type RemoteFile struct {
	ID       string // destination-specific identifier (key, path, file id)
	Name     string
	Modified string // RFC3339 if known
}

// Destination is a place backups can be pushed to and pruned from.
type Destination interface {
	// Put uploads the bundle at localPath under the given filename.
	Put(ctx context.Context, filename, localPath string) error
	// List returns previously uploaded Sempa backups, newest first.
	List(ctx context.Context) ([]RemoteFile, error)
	// Delete removes a backup by its destination ID.
	Delete(ctx context.Context, id string) error
}

// DestConfig is the persisted, type-tagged configuration for one destination.
type DestConfig struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // 'local' | 'webdav' | 's3' | 'drive'
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`

	// local
	Path string `json:"path,omitempty"`

	// webdav
	URL      string `json:"url,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	// s3 (also reuses URL as a custom endpoint, Username unused)
	Bucket          string `json:"bucket,omitempty"`
	Region          string `json:"region,omitempty"`
	Prefix          string `json:"prefix,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`

	// drive
	FolderID string `json:"folder_id,omitempty"`
}

// secretFields lists per-type config keys that must be redacted to the client
// and merged back from storage when the client sends an empty placeholder.
var secretFields = map[string][]string{
	"webdav": {"password"},
	"s3":     {"secret_access_key"},
}

const backupFilePrefix = "sempa-backup-"

// ParseDestinations decodes the stored destinations JSON.
func ParseDestinations(raw string) ([]DestConfig, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out []DestConfig
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RedactDestinations blanks secret fields for safe transmission to the client.
func RedactDestinations(raw string) (string, error) {
	dests, err := ParseDestinations(raw)
	if err != nil {
		return "[]", err
	}
	for i := range dests {
		switch dests[i].Type {
		case "webdav":
			if dests[i].Password != "" {
				dests[i].Password = ""
			}
		case "s3":
			if dests[i].SecretAccessKey != "" {
				dests[i].SecretAccessKey = ""
			}
		}
	}
	b, err := json.Marshal(dests)
	if err != nil {
		return "[]", err
	}
	return string(b), nil
}

// MergeDestinations validates the incoming destinations and copies any redacted
// secret back from the existing stored copy (matched by id).
func MergeDestinations(existingJSON string, incomingRaw []byte) (string, error) {
	var incoming []DestConfig
	if err := json.Unmarshal(incomingRaw, &incoming); err != nil {
		return "", fmt.Errorf("invalid destinations: %w", err)
	}
	existing, _ := ParseDestinations(existingJSON)
	byID := map[string]DestConfig{}
	for _, d := range existing {
		byID[d.ID] = d
	}

	for i := range incoming {
		d := &incoming[i]
		switch d.Type {
		case "local", "webdav", "s3", "drive":
		default:
			return "", fmt.Errorf("unknown destination type %q", d.Type)
		}
		if d.ID == "" {
			return "", fmt.Errorf("destination missing id")
		}
		prev, ok := byID[d.ID]
		if !ok {
			continue
		}
		// Restore redacted secrets.
		if d.Type == "webdav" && d.Password == "" {
			d.Password = prev.Password
		}
		if d.Type == "s3" && d.SecretAccessKey == "" {
			d.SecretAccessKey = prev.SecretAccessKey
		}
	}
	b, err := json.Marshal(incoming)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// NewDestination builds a live Destination from its config. driveToken supplies
// the OAuth access token for the 'drive' type (resolved by the caller).
func NewDestination(d DestConfig, driveToken DriveTokenFunc) (Destination, error) {
	switch d.Type {
	case "local":
		return &localDest{path: d.Path}, nil
	case "webdav":
		return &webdavDest{baseURL: strings.TrimRight(d.URL, "/"), user: d.Username, pass: d.Password}, nil
	case "s3":
		ep := d.Endpoint
		return &s3Dest{
			bucket: d.Bucket, region: orDefault(d.Region, "us-east-1"),
			prefix: d.Prefix, endpoint: ep,
			accessKey: d.AccessKeyID, secretKey: d.SecretAccessKey,
		}, nil
	case "drive":
		return &driveDest{folderID: d.FolderID, token: driveToken}, nil
	}
	return nil, fmt.Errorf("unknown destination type %q", d.Type)
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// ── Local folder ─────────────────────────────────────────────────────────────

type localDest struct{ path string }

func (l *localDest) Put(ctx context.Context, filename, localPath string) error {
	if l.path == "" {
		return fmt.Errorf("local destination has no path")
	}
	if err := os.MkdirAll(l.path, 0o755); err != nil {
		return err
	}
	in, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer in.Close()
	dst := filepath.Join(l.path, filename)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (l *localDest) List(ctx context.Context) ([]RemoteFile, error) {
	entries, err := os.ReadDir(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []RemoteFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), backupFilePrefix) {
			continue
		}
		info, err := e.Info()
		mod := ""
		if err == nil {
			mod = info.ModTime().UTC().Format("2006-01-02T15:04:05Z")
		}
		out = append(out, RemoteFile{ID: e.Name(), Name: e.Name(), Modified: mod})
	}
	sortRemoteNewestFirst(out)
	return out, nil
}

func (l *localDest) Delete(ctx context.Context, id string) error {
	// id is the filename; guard against traversal.
	if strings.Contains(id, "/") || strings.Contains(id, "..") {
		return fmt.Errorf("invalid id")
	}
	err := os.Remove(filepath.Join(l.path, id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// sortRemoteNewestFirst sorts by Name descending; backup filenames embed a
// sortable timestamp so lexical order == chronological order.
func sortRemoteNewestFirst(files []RemoteFile) {
	sort.Slice(files, func(i, j int) bool { return files[i].Name > files[j].Name })
}
