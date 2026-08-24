// Package instagram serves the couple's imported Instagram posts. A one-shot
// operator script (scripts/instagram_import.py) copies each profile's recent
// posts — images included — into the public storage bucket and writes one
// manifest per person (instagram/{person}.json). This service reads and
// caches those manifests: the site never talks to Instagram at runtime, and
// a missing manifest degrades to "not configured", never an error page.
package instagram

import (
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

// Service reads and caches one imported manifest per person.
type Service struct {
	// manifestBase is the public URL prefix the manifests live under, e.g.
	// {SUPABASE_URL}/storage/v1/object/public/{bucket}/instagram
	manifestBase string
	client       *http.Client

	mu    sync.Mutex
	cache map[string]cacheEntry
}

// NewService builds the reader for the given public manifest prefix.
func NewService(manifestBase string) *Service {
	return &Service{
		manifestBase: manifestBase,
		client:       &http.Client{Timeout: 8 * time.Second},
		cache:        map[string]cacheEntry{},
	}
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
