package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// DriveTokenFunc returns a fresh Google Drive access token. The caller wires
// this to the stored OAuth credentials (refreshing as needed).
type DriveTokenFunc func(ctx context.Context) (string, error)

// driveDest uploads backups to Google Drive using the drive.file scope. With
// drive.file the app can only see files it created, so List/Delete only ever
// touch Sempa's own backups.
type driveDest struct {
	folderID string
	token    DriveTokenFunc
}

func (d *driveDest) accessToken(ctx context.Context) (string, error) {
	if d.token == nil {
		return "", fmt.Errorf("Google Drive is not connected")
	}
	return d.token(ctx)
}

func (d *driveDest) Put(ctx context.Context, filename, localPath string) error {
	tok, err := d.accessToken(ctx)
	if err != nil {
		return err
	}
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}

	// 1. Start a resumable session.
	meta := map[string]any{"name": filename}
	if d.folderID != "" {
		meta["parents"] = []string{d.folderID}
	}
	metaBytes, _ := json.Marshal(meta)

	startReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://www.googleapis.com/upload/drive/v3/files?uploadType=resumable", bytes.NewReader(metaBytes))
	if err != nil {
		return err
	}
	startReq.Header.Set("Authorization", "Bearer "+tok)
	startReq.Header.Set("Content-Type", "application/json; charset=UTF-8")
	startReq.Header.Set("X-Upload-Content-Type", "application/octet-stream")
	startReq.Header.Set("X-Upload-Content-Length", strconv.FormatInt(fi.Size(), 10))

	startResp, err := http.DefaultClient.Do(startReq)
	if err != nil {
		return err
	}
	defer startResp.Body.Close()
	if startResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(startResp.Body, 2048))
		return fmt.Errorf("drive resumable start failed: HTTP %d: %s", startResp.StatusCode, strings.TrimSpace(string(body)))
	}
	sessionURI := startResp.Header.Get("Location")
	if sessionURI == "" {
		return fmt.Errorf("drive did not return an upload session URI")
	}

	// 2. Upload the bytes in one PUT (the client streams from disk).
	upReq, err := http.NewRequestWithContext(ctx, http.MethodPut, sessionURI, f)
	if err != nil {
		return err
	}
	upReq.ContentLength = fi.Size()
	upReq.Header.Set("Content-Type", "application/octet-stream")

	upResp, err := http.DefaultClient.Do(upReq)
	if err != nil {
		return err
	}
	defer upResp.Body.Close()
	if upResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(upResp.Body, 2048))
		return fmt.Errorf("drive upload failed: HTTP %d: %s", upResp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

type driveListResult struct {
	Files []struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ModifiedTime string `json:"modifiedTime"`
	} `json:"files"`
}

func (d *driveDest) List(ctx context.Context) ([]RemoteFile, error) {
	tok, err := d.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	q := fmt.Sprintf("name contains '%s' and trashed = false", backupFilePrefix)
	if d.folderID != "" {
		q = fmt.Sprintf("'%s' in parents and ", d.folderID) + q
	}
	params := url.Values{}
	params.Set("q", q)
	params.Set("fields", "files(id,name,modifiedTime)")
	params.Set("pageSize", "1000")
	params.Set("orderBy", "name desc")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://www.googleapis.com/drive/v3/files?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("drive list failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var lr driveListResult
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return nil, err
	}
	var out []RemoteFile
	for _, fl := range lr.Files {
		out = append(out, RemoteFile{ID: fl.ID, Name: fl.Name, Modified: fl.ModifiedTime})
	}
	sortRemoteNewestFirst(out)
	return out, nil
}

func (d *driveDest) Delete(ctx context.Context, id string) error {
	tok, err := d.accessToken(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		"https://www.googleapis.com/drive/v3/files/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("drive delete failed: HTTP %d", resp.StatusCode)
	}
	return nil
}
