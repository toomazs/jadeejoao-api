// Package media owns couple-managed content images: uploads go through the
// API into a public Supabase Storage bucket, the media table records path +
// public URL, and browsers load images straight from the CDN (AD-8). Brand
// assets are NOT media — they ship inside the frontend repos.
package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNotFound: media id unknown.
	ErrNotFound = errors.New("media not found")
	// ErrUnsupportedType: not an allowed image upload.
	ErrUnsupportedType = errors.New("unsupported media type")
)

// allowedExtensions is the closed set of accepted image file extensions.
var allowedExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true, ".avif": true,
}

// Media is one uploaded content image.
type Media struct {
	ID         uuid.UUID
	BucketPath string
	PublicURL  string
	Alt        *string
	CreatedAt  time.Time
}

// Storage is the object-store surface the service needs (implemented by
// platform.Storage; faked in tests).
type Storage interface {
	Upload(ctx context.Context, path, contentType string, body io.Reader) error
	Delete(ctx context.Context, path string) error
	PublicURL(path string) string
}

// Repo is the persistence surface for the media table.
type Repo interface {
	Insert(ctx context.Context, bucketPath, publicURL string, alt *string) (Media, error)
	List(ctx context.Context) ([]Media, error)
	Get(ctx context.Context, id uuid.UUID) (Media, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Service implements the media pipeline.
type Service struct {
	repo    Repo
	storage Storage
}

// NewService wires the media service.
func NewService(repo Repo, storage Storage) *Service {
	return &Service{repo: repo, storage: storage}
}

// Upload validates the file, writes it to the bucket under a fresh UUID name,
// and records it in the media table.
func (s *Service) Upload(ctx context.Context, filename, contentType string, body io.Reader, alt *string) (Media, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExtensions[ext] || !strings.HasPrefix(contentType, "image/") {
		return Media{}, ErrUnsupportedType
	}
	path := fmt.Sprintf("%s%s", uuid.NewString(), ext)
	if err := s.storage.Upload(ctx, path, contentType, body); err != nil {
		return Media{}, err
	}
	return s.repo.Insert(ctx, path, s.storage.PublicURL(path), alt)
}

// List returns every uploaded image, newest first.
func (s *Service) List(ctx context.Context) ([]Media, error) {
	return s.repo.List(ctx)
}

// Delete removes the object from the bucket and the row from the table.
// Object deletion is idempotent, so a missing object never strands the row.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.storage.Delete(ctx, m.BucketPath); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}
