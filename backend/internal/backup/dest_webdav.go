package backup

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

// webdavDest pushes backups to a WebDAV server (e.g. Nextcloud). baseURL points
// at the target collection (folder); files are stored directly inside it.
type webdavDest struct {
	baseURL string
	user    string
	pass    string
}

func (d *webdavDest) fileURL(filename string) string {
	return d.baseURL + "/" + filename
}

func (d *webdavDest) auth(req *http.Request) {
	if d.user != "" || d.pass != "" {
		req.SetBasicAuth(d.user, d.pass)
	}
}

func (d *webdavDest) Put(ctx context.Context, filename, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, d.fileURL(filename), f)
	if err != nil {
		return err
	}
	req.ContentLength = fi.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	d.auth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("webdav PUT failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

type davMultistatus struct {
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href     string `xml:"href"`
	Propstat struct {
		Prop struct {
			LastModified string `xml:"getlastmodified"`
		} `xml:"prop"`
	} `xml:"propstat"`
}

func (d *webdavDest) List(ctx context.Context) ([]RemoteFile, error) {
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", d.baseURL+"/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Depth", "1")
	d.auth(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webdav PROPFIND failed: HTTP %d", resp.StatusCode)
	}

	var ms davMultistatus
	if err := xml.NewDecoder(resp.Body).Decode(&ms); err != nil {
		return nil, err
	}
	var out []RemoteFile
	for _, r := range ms.Responses {
		name := path.Base(strings.TrimRight(r.Href, "/"))
		if !strings.HasPrefix(name, backupFilePrefix) {
			continue
		}
		out = append(out, RemoteFile{ID: name, Name: name, Modified: r.Propstat.Prop.LastModified})
	}
	sortRemoteNewestFirst(out)
	return out, nil
}

func (d *webdavDest) Delete(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, d.fileURL(id), nil)
	if err != nil {
		return err
	}
	d.auth(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("webdav DELETE failed: HTTP %d", resp.StatusCode)
	}
	return nil
}
