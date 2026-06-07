// Package blob stores attachment file bytes on the local filesystem.
// Each blob is a single file named by its attachment ID (no extension);
// the MIME type and original filename live in the attachments DB table.
package blob

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Store struct{ dir string }

// New creates the blob directory if it does not exist and returns a Store.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create blob dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Dir returns the root directory holding all blobs (used by the backup engine).
func (s *Store) Dir() string { return s.dir }

// Path returns the on-disk path for a blob ID.
func (s *Store) Path(id string) string { return filepath.Join(s.dir, id) }

// Create streams r into a new blob file and returns the number of bytes written.
// The file is removed if the copy fails partway through.
func (s *Store) Create(id string, r io.Reader) (int64, error) {
	path := s.Path(id)
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create blob: %w", err)
	}
	n, err := io.Copy(f, r)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return 0, fmt.Errorf("write blob: %w", err)
	}
	return n, nil
}

// Open opens a blob for reading. Caller must Close.
func (s *Store) Open(id string) (*os.File, error) {
	return os.Open(s.Path(id))
}

// Remove deletes a blob file. Missing files are not an error.
func (s *Store) Remove(id string) error {
	err := os.Remove(s.Path(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
