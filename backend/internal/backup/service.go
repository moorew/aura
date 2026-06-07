package backup

import (
	"archive/zip"
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/clevercode/sempa/internal/db"

	_ "modernc.org/sqlite"
)

const (
	appName       = "sempa"
	formatVersion = 1
	manifestName  = "manifest.json"
	dbEntryName   = "db/sempa.db"
	attachPrefix  = "attachments/"
)

// Security modes.
const (
	ModeNone           = "none"
	ModeEncrypt        = "encrypt"
	ModeExcludeSecrets = "exclude_secrets"
)

// Manifest describes a bundle. It is stored as manifest.json inside the zip.
type Manifest struct {
	App             string `json:"app"`
	FormatVersion   int    `json:"format_version"`
	SchemaVersion   string `json:"schema_version"`
	CreatedAt       string `json:"created_at"`
	Encrypted       bool   `json:"encrypted"`
	SecretsExcluded bool   `json:"secrets_excluded"`
	AttachmentCount int    `json:"attachment_count"`
}

type Service struct {
	db      *sql.DB
	store   *db.BackupStore
	dbPath  string
	blobDir string
}

func NewService(database *sql.DB, dbPath, blobDir string) *Service {
	return &Service{
		db:      database,
		store:   db.NewBackupStore(database),
		dbPath:  dbPath,
		blobDir: blobDir,
	}
}

// BuildResult is a freshly built bundle on disk. Caller must Cleanup when done.
type BuildResult struct {
	Path     string
	Filename string
	Size     int64
	tmpDir   string
}

func (b *BuildResult) Cleanup() {
	if b.tmpDir != "" {
		_ = os.RemoveAll(b.tmpDir)
	}
}

// Build produces a bundle using the given security mode. For ModeEncrypt the
// passphrase must be non-empty.
func (s *Service) Build(ctx context.Context, mode, passphrase string) (*BuildResult, error) {
	if mode == ModeEncrypt && passphrase == "" {
		return nil, fmt.Errorf("encryption mode requires a passphrase")
	}

	tmpDir, err := os.MkdirTemp("", "sempa-backup-*")
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	// 1. Consistent snapshot of the live DB (checkpoints WAL into a clean copy).
	snapPath := filepath.Join(tmpDir, "snapshot.db")
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO '"+sqlQuote(snapPath)+"'"); err != nil {
		cleanup()
		return nil, fmt.Errorf("snapshot db: %w", err)
	}

	// 2. Strip secrets from the snapshot.
	if err := sanitizeSnapshot(ctx, snapPath, mode); err != nil {
		cleanup()
		return nil, fmt.Errorf("sanitize snapshot: %w", err)
	}

	// 3. Assemble the zip.
	zipPath := filepath.Join(tmpDir, "bundle.zip")
	attachCount, err := s.writeZip(ctx, zipPath, snapPath, mode)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("write zip: %w", err)
	}
	_ = attachCount

	// 4. Optionally encrypt the whole bundle.
	ts := time.Now().Format("2006-01-02-150405")
	finalPath := zipPath
	filename := fmt.Sprintf("sempa-backup-%s.zip", ts)
	if mode == ModeEncrypt {
		encPath := filepath.Join(tmpDir, "bundle.zip.enc")
		in, err := os.Open(zipPath)
		if err != nil {
			cleanup()
			return nil, err
		}
		out, err := os.Create(encPath)
		if err != nil {
			in.Close()
			cleanup()
			return nil, err
		}
		err = EncryptStream(out, bufio.NewReaderSize(in, 1<<20), passphrase)
		in.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("encrypt: %w", err)
		}
		finalPath = encPath
		filename += ".enc"
	}

	fi, err := os.Stat(finalPath)
	if err != nil {
		cleanup()
		return nil, err
	}
	return &BuildResult{Path: finalPath, Filename: filename, Size: fi.Size(), tmpDir: tmpDir}, nil
}

func (s *Service) writeZip(ctx context.Context, zipPath, snapPath, mode string) (int, error) {
	zf, err := os.Create(zipPath)
	if err != nil {
		return 0, err
	}
	defer zf.Close()
	bw := bufio.NewWriterSize(zf, 1<<20)
	zw := zip.NewWriter(bw)

	// Attachments — copy every blob in the directory.
	attachCount := 0
	if entries, err := os.ReadDir(s.blobDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if err := addFileToZip(zw, filepath.Join(s.blobDir, e.Name()), attachPrefix+e.Name()); err != nil {
				return 0, err
			}
			attachCount++
		}
	}

	// Manifest.
	manifest := Manifest{
		App:             appName,
		FormatVersion:   formatVersion,
		SchemaVersion:   s.store.SchemaVersion(ctx),
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		Encrypted:       mode == ModeEncrypt,
		SecretsExcluded: mode == ModeExcludeSecrets,
		AttachmentCount: attachCount,
	}
	mw, err := zw.Create(manifestName)
	if err != nil {
		return 0, err
	}
	if err := json.NewEncoder(mw).Encode(manifest); err != nil {
		return 0, err
	}

	// Database snapshot.
	if err := addFileToZip(zw, snapPath, dbEntryName); err != nil {
		return 0, err
	}

	if err := zw.Close(); err != nil {
		return 0, err
	}
	return attachCount, bw.Flush()
}

func addFileToZip(zw *zip.Writer, srcPath, name string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

// sanitizeSnapshot always removes the backup passphrase + destination creds (so a
// bundle never contains the keys to itself), and in exclude-secrets mode also
// drops integration tokens and device push tokens.
func sanitizeSnapshot(ctx context.Context, snapPath, mode string) error {
	sdb, err := sql.Open("sqlite", snapPath)
	if err != nil {
		return err
	}
	defer sdb.Close()

	stmts := []string{
		`UPDATE backup_settings SET passphrase = NULL, destinations = '[]'`,
	}
	if mode == ModeExcludeSecrets {
		stmts = append(stmts,
			`DELETE FROM integration_configs`,
			`DELETE FROM device_tokens`,
		)
	}
	for _, stmt := range stmts {
		// Ignore "no such table" so the engine survives schema drift.
		if _, err := sdb.ExecContext(ctx, stmt); err != nil && !strings.Contains(err.Error(), "no such table") {
			return err
		}
	}
	return nil
}

// Tables that hold the current instance's own bookkeeping and must not be
// overwritten by a restore.
var restoreSkip = map[string]bool{
	"schema_migrations": true,
	"backup_settings":   true,
	"backup_runs":       true,
}

// Restore replaces ALL data from the bundle read from src. If the bundle is
// encrypted, passphrase must be supplied.
func (s *Service) Restore(ctx context.Context, src io.Reader, passphrase string) error {
	br := bufio.NewReaderSize(src, 1<<20)
	head, _ := br.Peek(len(magic))

	tmp, err := os.CreateTemp("", "sempa-restore-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if LooksEncrypted(head) {
		if passphrase == "" {
			tmp.Close()
			return fmt.Errorf("this backup is encrypted — a passphrase is required")
		}
		if err := DecryptStream(tmp, br, passphrase); err != nil {
			tmp.Close()
			return err
		}
	} else {
		if _, err := io.Copy(tmp, br); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	zr, err := zip.OpenReader(tmpPath)
	if err != nil {
		return fmt.Errorf("not a valid backup archive: %w", err)
	}
	defer zr.Close()

	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}

	// Validate manifest.
	mf, ok := files[manifestName]
	if !ok {
		return fmt.Errorf("backup is missing its manifest")
	}
	var manifest Manifest
	if err := readJSONFromZip(mf, &manifest); err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	if manifest.App != appName {
		return fmt.Errorf("not a Sempa backup")
	}

	// Extract the DB snapshot to a temp file.
	dbFile, ok := files[dbEntryName]
	if !ok {
		return fmt.Errorf("backup is missing its database")
	}
	snapPath := tmpPath + ".db"
	if err := extractZipFile(dbFile, snapPath); err != nil {
		return err
	}
	defer os.Remove(snapPath)

	// Replace DB contents, then attachment blobs.
	if err := s.restoreDB(ctx, snapPath); err != nil {
		return fmt.Errorf("restore database: %w", err)
	}
	if err := s.restoreAttachments(zr.File); err != nil {
		return fmt.Errorf("restore attachments: %w", err)
	}
	return nil
}

// restoreDB wipes every user table in the live DB and copies the snapshot's rows
// in, matching on common columns so it survives minor schema drift.
func (s *Service) restoreDB(ctx context.Context, snapPath string) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	escaped := sqlQuote(snapPath)
	if _, err := conn.ExecContext(ctx, "ATTACH DATABASE '"+escaped+"' AS src"); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, "DETACH DATABASE src")

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return err
	}
	// Always re-enable FKs on this pooled connection before returning it.
	defer conn.ExecContext(ctx, "PRAGMA foreign_keys = ON")

	tables, err := listTables(ctx, conn, "main")
	if err != nil {
		return err
	}
	srcTables, err := listTables(ctx, conn, "src")
	if err != nil {
		return err
	}
	srcHas := map[string]bool{}
	for _, t := range srcTables {
		srcHas[t] = true
	}

	if _, err := conn.ExecContext(ctx, "BEGIN"); err != nil {
		return err
	}
	rollback := func() { conn.ExecContext(ctx, "ROLLBACK") }

	for _, t := range tables {
		if restoreSkip[t] {
			continue
		}
		if _, err := conn.ExecContext(ctx, `DELETE FROM main."`+t+`"`); err != nil {
			rollback()
			return err
		}
		if !srcHas[t] {
			continue // table didn't exist in the backup → leave empty
		}
		mainCols, err := columns(ctx, conn, "main", t)
		if err != nil {
			rollback()
			return err
		}
		srcCols, err := columns(ctx, conn, "src", t)
		if err != nil {
			rollback()
			return err
		}
		common := intersect(mainCols, srcCols)
		if len(common) == 0 {
			continue
		}
		colList := quoteCols(common)
		stmt := fmt.Sprintf(`INSERT INTO main."%s" (%s) SELECT %s FROM src."%s"`, t, colList, colList, t)
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			rollback()
			return err
		}
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		rollback()
		return err
	}
	return nil
}

func (s *Service) restoreAttachments(files []*zip.File) error {
	if err := os.MkdirAll(s.blobDir, 0o755); err != nil {
		return err
	}
	// Clear existing blobs.
	if entries, err := os.ReadDir(s.blobDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				_ = os.Remove(filepath.Join(s.blobDir, e.Name()))
			}
		}
	}
	for _, f := range files {
		if !strings.HasPrefix(f.Name, attachPrefix) || f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		if name == "" || name == "." {
			continue
		}
		if err := extractZipFile(f, filepath.Join(s.blobDir, name)); err != nil {
			return err
		}
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func sqlQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

func listTables(ctx context.Context, conn *sql.Conn, schema string) ([]string, error) {
	rows, err := conn.QueryContext(ctx,
		fmt.Sprintf(`SELECT name FROM %s.sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%%'`, schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func columns(ctx context.Context, conn *sql.Conn, schema, table string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`PRAGMA %s.table_info("%s")`, schema, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func intersect(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range b {
		set[x] = true
	}
	var out []string
	for _, x := range a {
		if set[x] {
			out = append(out, x)
		}
	}
	return out
}

func quoteCols(cols []string) string {
	q := make([]string, len(cols))
	for i, c := range cols {
		q[i] = `"` + c + `"`
	}
	return strings.Join(q, ", ")
}

func readJSONFromZip(f *zip.File, v any) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return json.NewDecoder(rc).Decode(v)
}

func extractZipFile(f *zip.File, dstPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
