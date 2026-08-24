package instagram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const feedJSON = `{"data":[
	{"id":"1","media_type":"VIDEO","media_url":"https://cdn.example/v.mp4","thumbnail_url":"https://cdn.example/t.jpg","permalink":"https://www.instagram.com/p/a/","timestamp":"2026-08-01T12:00:00+0000"},
	{"id":"2","media_type":"IMAGE","media_url":"https://cdn.example/i.jpg","permalink":"https://www.instagram.com/p/b/","caption":"legenda"},
	{"id":"3","media_type":"IMAGE","permalink":"https://www.instagram.com/p/c/"}
]}`

func newTestService(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	s := NewService("tok-bride", "")
	s.baseURL = srv.URL
	return s
}

func TestPostsMapsFiltersAndCaches(t *testing.T) {
	var calls atomic.Int32
	s := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/me/media" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("access_token") != "tok-bride" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(feedJSON))
	})

	posts, err := s.Posts(context.Background(), PersonBride)
	if err != nil {
		t.Fatalf("Posts: %v", err)
	}
	// Entry 3 has no media at all and must be dropped.
	if len(posts) != 2 {
		t.Fatalf("got %d posts, want 2", len(posts))
	}
	if posts[0].ThumbnailURL != "https://cdn.example/t.jpg" || posts[0].MediaType != "VIDEO" {
		t.Fatalf("video mapping wrong: %+v", posts[0])
	}
	if posts[1].Caption != "legenda" {
		t.Fatalf("caption lost: %+v", posts[1])
	}

	if _, err := s.Posts(context.Background(), PersonBride); err != nil {
		t.Fatalf("cached Posts: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 upstream call, got %d", calls.Load())
	}
}

func TestPostsKeepsLastGoodFeedOnFailure(t *testing.T) {
	var fail atomic.Bool
	s := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(feedJSON))
	})

	good, err := s.Posts(context.Background(), PersonBride)
	if err != nil || len(good) == 0 {
		t.Fatalf("seed fetch: posts=%d err=%v", len(good), err)
	}

	// Expire the cache, then break the upstream.
	fail.Store(true)
	s.mu.Lock()
	entry := s.cache[PersonBride]
	entry.fetchedAt = time.Now().Add(-2 * cacheTTL)
	s.cache[PersonBride] = entry
	s.mu.Unlock()

	posts, err := s.Posts(context.Background(), PersonBride)
	if err == nil {
		t.Fatal("expected an error from the broken upstream")
	}
	if len(posts) != len(good) {
		t.Fatalf("last good feed lost: got %d posts, want %d", len(posts), len(good))
	}

	// Within failureTTL the failure is served from cache — no hammering.
	if _, err := s.Posts(context.Background(), PersonBride); err != nil {
		t.Fatalf("backoff read should not error, got %v", err)
	}
}

func TestUnconfiguredPerson(t *testing.T) {
	s := NewService("", "")
	if s.Configured(PersonBride) || s.Configured(PersonGroom) {
		t.Fatal("no token should mean not configured")
	}
	posts, err := s.Posts(context.Background(), PersonGroom)
	if err != nil || posts != nil {
		t.Fatalf("unconfigured feed: posts=%v err=%v", posts, err)
	}
	var nilSvc *Service
	if nilSvc.Configured(PersonBride) {
		t.Fatal("nil service must report not configured")
	}
}
