// Package instagram serves the six posts shown under each chapter. They live
// in the public storage bucket as one manifest per person
// (instagram/{person}.json), written first by a one-shot import script and now
// edited from the panel. The name is history: the site never talks to
// Instagram, and nothing here does either — it is a small hand-kept gallery
// that happens to link back to the posts it came from. A missing manifest
// degrades to "not configured", never an error page.
package instagram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Person keys, mirrored by the {person} path parameter.
const (
	PersonBride = "bride"
	PersonGroom = "groom"
)

const (
	// Manifests change only when the import script reruns — cache generously,
	// but short enough that a fresh import shows up without a restart.
	cacheTTL = 10 * time.Minute
	// Misses and failures are retried sooner: right after the first import
	// the feed should appear quickly.
	missTTL = 2 * time.Minute
	// Ceiling for one manifest read, independent of the caller's deadline.
	fetchTimeout = 10 * time.Second
)

// PostView is one imported post, shaped for the public site. JSON tags match
// the manifest entries the import script writes.
type PostView struct {
	ID           string `json:"id"`
	Caption      string `json:"caption,omitempty"`
	MediaType    string `json:"media_type" example:"IMAGE" doc:"IMAGE, VIDEO or CAROUSEL_ALBUM."`
	MediaURL     string `json:"media_url,omitempty" format:"uri" doc:"Display image, served from the site's own bucket."`
	ThumbnailURL string `json:"thumbnail_url,omitempty" format:"uri" doc:"Preview image, present for VIDEO posts."`
	Permalink    string `json:"permalink" format:"uri"`
	Timestamp    string `json:"timestamp,omitempty" doc:"Publish time, ISO 8601."`
}

type cacheEntry struct {
	posts     []PostView
	exists    bool
	fetchedAt time.Time
	failed    bool
}

// Uploader is the slice of storage this package writes through.
type Uploader interface {
	Upload(ctx context.Context, path, contentType string, body io.Reader) error
}

// Service reads and caches one manifest per person, and writes them back.
type Service struct {
	// manifestBase is the public URL prefix the manifests live under, e.g.
	// {SUPABASE_URL}/storage/v1/object/public/{bucket}/instagram
	manifestBase string
	client       *http.Client
	// storage is nil in tests and in any deploy without bucket credentials;
	// reading still works, saving reports that it cannot.
	storage Uploader

	mu    sync.Mutex
	cache map[string]cacheEntry
}

// NewService builds the reader for the given public manifest prefix. Pass a
// non-nil uploader to allow the panel to save.
func NewService(manifestBase string, storage Uploader) *Service {
	return &Service{
		manifestBase: manifestBase,
		client:       &http.Client{Timeout: 8 * time.Second},
		storage:      storage,
		cache:        map[string]cacheEntry{},
	}
}

// ManifestPath is where one person's gallery lives in the bucket.
func ManifestPath(person string) string { return "instagram/" + person + ".json" }

// Editable returns the stored gallery without consulting the cache.
//
// The panel must see what is actually saved: a ten-minute-old copy would have
// the couple editing a list that no longer exists and saving it back over the
// real one.
func (s *Service) Editable(ctx context.Context, person string) ([]PostView, error) {
	if s == nil || s.manifestBase == "" {
		return nil, nil
	}
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	posts, _, err := s.fetch(fetchCtx, person)
	return posts, err
}

// Replace writes one person's gallery and drops the cached copy.
//
// Dropping the cache is the point. Without it a save is invisible for up to
// ten minutes — the couple would change a photo, look at the site, see the old
// one, and reasonably conclude the panel is broken.
func (s *Service) Replace(ctx context.Context, person string, posts []PostView) error {
	if s == nil || s.storage == nil {
		return fmt.Errorf("storage is not configured")
	}
	if posts == nil {
		posts = []PostView{}
	}
	body, err := json.Marshal(posts)
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	if err := s.storage.Upload(ctx, ManifestPath(person), "application/json", bytes.NewReader(body)); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.cache, person)
	s.mu.Unlock()
	return nil
}

// Posts returns the imported feed for person and whether an import exists.
// On a fetch failure it returns the last good state together with the error,
// and backs off retries via missTTL.
func (s *Service) Posts(ctx context.Context, person string) ([]PostView, bool, error) {
	if s == nil || s.manifestBase == "" {
		return nil, false, nil
	}
	s.mu.Lock()
	entry, ok := s.cache[person]
	s.mu.Unlock()

	ttl := cacheTTL
	if entry.failed || !entry.exists {
		ttl = missTTL
	}
	if ok && time.Since(entry.fetchedAt) < ttl {
		return entry.posts, entry.exists, nil
	}

	// The manifest is shared by every visitor, so its fetch must outlive the
	// request that happened to trigger it. A guest closing the tab (or a dev
	// StrictMode remount) cancels their own context — without this, that
	// cancellation would be cached as "no feed" for everyone until missTTL.
	fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), fetchTimeout)
	defer cancel()

	posts, exists, err := s.fetch(fetchCtx, person)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		// Keep serving the last good state while the bucket misbehaves.
		s.cache[person] = cacheEntry{posts: entry.posts, exists: entry.exists, fetchedAt: time.Now(), failed: true}
		return entry.posts, entry.exists, err
	}
	s.cache[person] = cacheEntry{posts: posts, exists: exists, fetchedAt: time.Now()}
	return posts, exists, nil
}

// fetch pulls one manifest from the public bucket. A 4xx means no import has
// been run for that person yet — reported as absence, not as an error.
func (s *Service) fetch(ctx context.Context, person string) ([]PostView, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.manifestBase+"/"+person+".json", nil)
	if err != nil {
		return nil, false, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, false, err
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("manifest fetch responded %d", resp.StatusCode)
	}
	var posts []PostView
	if err := json.Unmarshal(body, &posts); err != nil {
		return nil, false, fmt.Errorf("decode manifest: %w", err)
	}
	return posts, true, nil
}
