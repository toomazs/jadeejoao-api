package media

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type fakeStorage struct {
	uploaded map[string]string // path -> content type
	deleted  []string
}

func (f *fakeStorage) Upload(_ context.Context, path, contentType string, _ io.Reader) error {
	if f.uploaded == nil {
		f.uploaded = map[string]string{}
	}
	f.uploaded[path] = contentType
	return nil
}

func (f *fakeStorage) Delete(_ context.Context, path string) error {
	f.deleted = append(f.deleted, path)
	return nil
}

func (f *fakeStorage) PublicURL(path string) string {
	return "https://proj.supabase.co/storage/v1/object/public/site-media/" + path
}

type fakeMediaRepo struct {
	rows map[uuid.UUID]Media
}

func (f *fakeMediaRepo) Insert(_ context.Context, bucketPath, publicURL string, alt *string) (Media, error) {
	if f.rows == nil {
		f.rows = map[uuid.UUID]Media{}
	}
	m := Media{ID: uuid.New(), BucketPath: bucketPath, PublicURL: publicURL, Alt: alt}
	f.rows[m.ID] = m
	return m, nil
}

func (f *fakeMediaRepo) List(_ context.Context) ([]Media, error) {
	var out []Media
	for _, m := range f.rows {
		out = append(out, m)
	}
	return out, nil
}

func (f *fakeMediaRepo) Get(_ context.Context, id uuid.UUID) (Media, error) {
	m, ok := f.rows[id]
	if !ok {
		return Media{}, ErrNotFound
	}
	return m, nil
}

func (f *fakeMediaRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.rows[id]; !ok {
		return ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

func TestUploadHappyPath(t *testing.T) {
	storage, repo := &fakeStorage{}, &fakeMediaRepo{}
	svc := NewService(repo, storage)

	m, err := svc.Upload(context.Background(), "Foto Casal.JPG", "image/jpeg", strings.NewReader("data"), nil)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !strings.HasSuffix(m.BucketPath, ".jpg") {
		t.Fatalf("extension not normalized: %s", m.BucketPath)
	}
	if !strings.HasPrefix(m.PublicURL, "https://proj.supabase.co/storage/v1/object/public/site-media/") {
		t.Fatalf("public url wrong: %s", m.PublicURL)
	}
	if len(storage.uploaded) != 1 {
		t.Fatalf("object not uploaded: %+v", storage.uploaded)
	}
}

func TestUploadRejectsNonImages(t *testing.T) {
	svc := NewService(&fakeMediaRepo{}, &fakeStorage{})

	if _, err := svc.Upload(context.Background(), "malware.exe", "application/octet-stream", strings.NewReader("x"), nil); !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("exe accepted: %v", err)
	}
	if _, err := svc.Upload(context.Background(), "doc.pdf", "application/pdf", strings.NewReader("x"), nil); !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("pdf accepted: %v", err)
	}
	// Image extension but wrong content type.
	if _, err := svc.Upload(context.Background(), "fake.png", "text/html", strings.NewReader("x"), nil); !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("mismatched content type accepted: %v", err)
	}
}

func TestDeleteRemovesObjectAndRow(t *testing.T) {
	storage, repo := &fakeStorage{}, &fakeMediaRepo{}
	svc := NewService(repo, storage)

	m, err := svc.Upload(context.Background(), "a.png", "image/png", strings.NewReader("x"), nil)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if err := svc.Delete(context.Background(), m.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(storage.deleted) != 1 || storage.deleted[0] != m.BucketPath {
		t.Fatalf("object not deleted: %+v", storage.deleted)
	}
	if len(repo.rows) != 0 {
		t.Fatalf("row not deleted: %+v", repo.rows)
	}
	if err := svc.Delete(context.Background(), m.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("double delete: got %v, want ErrNotFound", err)
	}
}
