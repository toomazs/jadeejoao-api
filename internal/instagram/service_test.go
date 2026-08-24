package instagram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const manifestJSON = `[
	{"id":"1","media_type":"VIDEO","media_url":"https://cdn.example/v-thumb.jpg","permalink":"https://www.instagram.com/p/a/","timestamp":"2026-08-01T12:00:00-03:00"},
	{"id":"2","media_type":"IMAGE","media_url":"https://cdn.example/i.jpg","permalink":"https://www.instagram.com/p/b/","caption":"legenda"}
]`

func newTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewService(srv.URL)
}

func TestPostsReadsManifestAndCaches(t *testing.T) {
	var calls atomic.Int32
	s := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/bride.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(manifestJSON))
	})

	posts, exists, err := s.Posts(context.Background(), PersonBride)
	if err != nil || !exists {
		t.Fatalf("Posts: exists=%v err=%v", exists, err)
	}
	if len(posts) != 2 {
		t.Fatalf("got %d posts, want 2", len(posts))
	}
	if posts[0].MediaType != "VIDEO" || posts[0].MediaURL != "https://cdn.example/v-thumb.jpg" {
		t.Fatalf("video mapping wrong: %+v", posts[0])
	}
	if posts[1].Caption != "legenda" {
		t.Fatalf("caption lost: %+v", posts[1])
	}

	if _, _, err := s.Posts(context.Background(), PersonBride); err != nil {
		t.Fatalf("cached Posts: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 manifest fetch, got %d", calls.Load())
	}
}

func TestMissingManifestMeansNotConfigured(t *testing.T) {
	var calls atomic.Int32
	s := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		// Supabase answers 400/404 JSON for absent public objects.
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"not_found"}`))
	})

	posts, exists, err := s.Posts(context.Background(), PersonGroom)
	if err != nil || exists || posts != nil {
		t.Fatalf("missing manifest: posts=%v exists=%v err=%v", posts, exists, err)
	}
	// The miss is cached too — no hammering while nothing is imported.
	if _, _, _ = s.Posts(context.Background(), PersonGroom); calls.Load() != 1 {
		t.Fatalf("expected 1 fetch for cached miss, got %d", calls.Load())
	}
}

func TestFailureKeepsLastGoodFeed(t *testing.T) {
	var fail atomic.Bool
	s := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(manifestJSON))
	})

	good, exists, err := s.Posts(context.Background(), PersonBride)
	if err != nil || !exists || len(good) == 0 {
		t.Fatalf("seed fetch: posts=%d exists=%v err=%v", len(good), exists, err)
	}

	// Expire the cache, then break the upstream.
	fail.Store(true)
	s.mu.Lock()
	entry := s.cache[PersonBride]
	entry.fetchedAt = time.Now().Add(-2 * cacheTTL)
	s.cache[PersonBride] = entry
	s.mu.Unlock()

	posts, exists, err := s.Posts(context.Background(), PersonBride)
	if err == nil {
		t.Fatal("expected an error from the broken upstream")
	}
	if !exists || len(posts) != len(good) {
		t.Fatalf("last good feed lost: posts=%d exists=%v", len(posts), exists)
	}
}

func TestNilAndUnconfiguredService(t *testing.T) {
	var nilSvc *Service
	if posts, exists, err := nilSvc.Posts(context.Background(), PersonBride); posts != nil || exists || err != nil {
		t.Fatal("nil service must answer empty and not configured")
	}
	empty := NewService("")
	if posts, exists, err := empty.Posts(context.Background(), PersonBride); posts != nil || exists || err != nil {
		t.Fatal("empty base must answer empty and not configured")
	}
}

// A guest closing the tab mid-request (or a dev StrictMode remount) cancels
// that request's context. The manifest is shared, so the read must finish and
// cache the real feed — never the cancellation, which used to hide the grid
// from everyone for missTTL.
func TestCallerCancellationDoesNotPoisonTheCache(t *testing.T) {
	var calls atomic.Int32
	s := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(manifestJSON))
	})

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	posts, exists, err := s.Posts(cancelled, PersonBride)
	if err != nil {
		t.Fatalf("cancelled caller should still get the feed: %v", err)
	}
	if !exists || len(posts) != 2 {
		t.Fatalf("got exists=%v posts=%d, want the real feed", exists, len(posts))
	}

	// And the next visitor is served from a healthy cache, not a poisoned one.
	posts, exists, err = s.Posts(context.Background(), PersonBride)
	if err != nil || !exists || len(posts) != 2 {
		t.Fatalf("second read: posts=%d exists=%v err=%v", len(posts), exists, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected the first read to have cached, got %d fetches", calls.Load())
	}
}
