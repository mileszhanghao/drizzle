// Package media validates uploads and stores media files.
package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/mileszhanghao/socialai/backend/internal/store"
)

// Allowed content types, detected from the file's bytes. The course version
// trusted the filename extension, so any file renamed to .jpg was accepted.
var allowed = map[string]struct {
	kind store.MediaType
	ext  string
}{
	"image/jpeg": {store.MediaImage, ".jpg"},
	"image/png":  {store.MediaImage, ".png"},
	"image/gif":  {store.MediaImage, ".gif"},
	"image/webp": {store.MediaImage, ".webp"},
	"video/mp4":  {store.MediaVideo, ".mp4"},
	"video/webm": {store.MediaVideo, ".webm"},
}

var ErrUnsupported = errors.New("unsupported media type: upload a JPEG, PNG, GIF, WebP, MP4 or WebM file")

// Sniff reads the start of r to detect its real content type and returns a
// reader that replays those bytes followed by the rest.
func Sniff(r io.Reader) (io.Reader, store.MediaType, string, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, "", "", err
	}
	head = head[:n]
	contentType := http.DetectContentType(head)
	info, ok := allowed[contentType]
	if !ok {
		return nil, "", "", ErrUnsupported
	}
	return io.MultiReader(bytes.NewReader(head), r), info.kind, info.ext, nil
}

// Store saves and serves media blobs. The course version used Google Cloud
// Storage; this interface lets GCS or S3 replace the local implementation.
type Store interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Delete(ctx context.Context, key string) error
	// Handler serves stored blobs at GET /media/{key}.
	Handler() http.Handler
}

var keyPattern = regexp.MustCompile(`^[a-f0-9-]{36}\.(jpg|png|gif|webp|mp4|webm)$`)

// ValidKey reports whether key is one this service generated, which rules
// out path traversal through user-controlled names.
func ValidKey(key string) bool { return keyPattern.MatchString(key) }

// Local stores blobs in a directory on disk.
type Local struct {
	dir string
}

func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Local{dir: dir}, nil
}

func (l *Local) Put(_ context.Context, key string, r io.Reader) error {
	if !ValidKey(key) {
		return fmt.Errorf("invalid media key %q", key)
	}
	tmp, err := os.CreateTemp(l.dir, ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(l.dir, key))
}

func (l *Local) Delete(_ context.Context, key string) error {
	if !ValidKey(key) {
		return fmt.Errorf("invalid media key %q", key)
	}
	err := os.Remove(filepath.Join(l.dir, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (l *Local) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		if !ValidKey(key) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, filepath.Join(l.dir, key))
	})
}
