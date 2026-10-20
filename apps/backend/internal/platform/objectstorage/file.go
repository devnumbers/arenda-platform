package objectstorage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nambers/arenda-planform/apps/backend/internal/shared/storage"
)

// FileStorage is the disk-backed adapter for local development (ADR 0065):
// provider=file, objects live as files under a root directory and survive
// backend restarts — unlike the in-memory fake, whose objects die with the
// process while the DB keys keep pointing into the storage (the dangling-key
// drift the serving self-heal then wipes). Same port semantics, no URLs.
//
// On-disk format: one file per object, `<content-type>\n<raw bytes>` — a
// single rename makes every Put atomic, so Open can never observe a torn
// object and report a false storage.ErrNotFound (a false 404 would trigger
// the serving self-heal and erase a live key). The port is streaming, this
// adapter buffers: it is a dev adapter, the upload seam caps objects at
// 5 MiB (ADR 0065).
type FileStorage struct {
	root string
}

// NewFileStorage creates the disk-backed adapter rooted at dir; the
// directory is created lazily on the first Put.
func NewFileStorage(dir string) *FileStorage {
	return &FileStorage{root: dir}
}

// path resolves a key under the root, rejecting keys that escape it. The
// keys are backend-minted (`photos/<uuid>.<ext>`), the guard keeps the
// contract honest against anything else.
func (s *FileStorage) path(key string) (string, error) {
	clean := filepath.Clean(key)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("object storage: key %q escapes the root", key)
	}
	return filepath.Join(s.root, clean), nil
}

// Put stores the object under the key, replacing any previous one. The
// bytes land in a temp file first: the rename is atomic, readers see either
// the old or the new object, never a torn one.
func (s *FileStorage) Put(_ context.Context, key string, data io.Reader, contentType string, _ int64) error {
	b, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("read upload data: %w", err)
	}
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return fmt.Errorf("make object dir: %w", err)
	}
	payload := make([]byte, 0, len(contentType)+1+len(b))
	payload = append(payload, contentType...)
	payload = append(payload, '\n')
	payload = append(payload, b...)
	tmp, err := os.CreateTemp(filepath.Dir(p), ".put-*")
	if err != nil {
		return fmt.Errorf("create temp object: %w", err)
	}
	tmpPath := tmp.Name()
	if _, werr := tmp.Write(payload); werr != nil {
		return errors.Join(fmt.Errorf("write temp object: %w", werr), tmp.Close(), removeTemp(tmpPath))
	}
	if cerr := tmp.Close(); cerr != nil {
		return errors.Join(fmt.Errorf("close temp object: %w", cerr), removeTemp(tmpPath))
	}
	if err := os.Rename(tmpPath, p); err != nil {
		return errors.Join(fmt.Errorf("place object: %w", err), removeTemp(tmpPath))
	}
	return nil
}

// removeTemp drops a failed Put's temp file; an already-gone file is fine.
// The cleanup error joins the primary one — a lost temp file must not mask
// why the Put failed, and a failed cleanup must not be silenced either.
func removeTemp(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove temp object: %w", err)
	}
	return nil
}

// Open returns the object's body, size and content type; a missing key is
// storage.ErrNotFound. An object file without the content-type header is a
// format the adapter never wrote — the honest ErrNotFound, the caller
// re-uploads.
func (s *FileStorage) Open(_ context.Context, key string) (body io.ReadCloser, size int64, contentType string, err error) {
	p, err := s.path(key)
	if err != nil {
		return nil, 0, "", err
	}
	raw, err := os.ReadFile(p) // #nosec G304 -- the path is derived from a key validated against root escape in path()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, "", storage.ErrNotFound
		}
		return nil, 0, "", fmt.Errorf("open object: %w", err)
	}
	header, payload, found := bytes.Cut(raw, []byte{'\n'})
	if !found {
		return nil, 0, "", storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(payload)), int64(len(payload)), string(header), nil
}

// Delete removes the object; deleting an absent key succeeds — the callers'
// orphan cleanup is best-effort and must not fail on an already-gone object.
func (s *FileStorage) Delete(_ context.Context, key string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

// HeadBucket checks the root directory is usable; the wire layer calls it
// once at startup.
func (s *FileStorage) HeadBucket(_ context.Context) error {
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return fmt.Errorf("make storage root: %w", err)
	}
	probe := filepath.Join(s.root, ".headbucket")
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		return fmt.Errorf("probe storage root: %w", err)
	}
	return os.Remove(probe)
}

var _ storage.PhotoStorage = (*FileStorage)(nil)
