package instagram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	return NewService(srv.URL, nil)
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
	empty := NewService("", nil)
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

// fakeUploader records what would have gone to the bucket.
type fakeUploader struct {
	path string
	body []byte
	fail error
}

func (f *fakeUploader) Upload(_ context.Context, path, _ string, body io.Reader) error {
	if f.fail != nil {
		return f.fail
	}
	f.path = path
	raw, err := io.ReadAll(body)
	f.body = raw
	return err
}

// TestReplaceDropsTheCache is the whole reason Replace lives on the service.
// Writing the manifest without forgetting the cached copy leaves the site
// serving the old photos for the rest of the cache window — a save that looks
// like it did nothing.
func TestReplaceDropsTheCache(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `[{"id":"a-%d","media_type":"IMAGE","media_url":"u","permalink":"p"}]`, hits)
	}))
	defer srv.Close()

	up := &fakeUploader{}
	svc := NewService(srv.URL, up)
	ctx := context.Background()

	if _, _, err := svc.Posts(ctx, PersonBride); err != nil {
		t.Fatalf("first read: %v", err)
	}
	if _, _, err := svc.Posts(ctx, PersonBride); err != nil {
		t.Fatalf("cached read: %v", err)
	}
	if hits != 1 {
		t.Fatalf("second read should have been cached, got %d fetches", hits)
	}

	if err := svc.Replace(ctx, PersonBride, []PostView{{ID: "novo", MediaType: "IMAGE", MediaURL: "u"}}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if up.path != "instagram/bride.json" {
		t.Fatalf("wrote to %q", up.path)
	}
	if !strings.Contains(string(up.body), `"novo"`) {
		t.Fatalf("manifest body missing the post: %s", up.body)
	}
	if _, _, err := svc.Posts(ctx, PersonBride); err != nil {
		t.Fatalf("read after replace: %v", err)
	}
	if hits != 2 {
		t.Fatalf("replace must drop the cache; fetches after save: %d", hits)
	}
}

// TestReplaceWithoutStorageSaysSo — a deploy without bucket credentials can
// still show the galleries, and must refuse to pretend it saved one.
func TestReplaceWithoutStorageSaysSo(t *testing.T) {
	if err := NewService("http://x", nil).Replace(context.Background(), PersonBride, nil); err == nil {
		t.Fatal("replace without storage should fail")
	}
}

// TestCleanFillsIdsAndRejectsBlankPhotos covers the two things the panel
// cannot get right on its own: a photo with no image is a hole in the grid,
// and a duplicate id makes two frames the same frame.
func TestCleanFillsIdsAndRejectsBlankPhotos(t *testing.T) {
	if _, err := clean([]PostView{{MediaURL: "  "}}, PersonBride); err == nil {
		t.Fatal("a post with no image should be refused")
	}
	if _, err := clean(make([]PostView, MaxPosts+1), PersonBride); err == nil {
		t.Fatal("over the ceiling should be refused")
	}
	out, err := clean([]PostView{
		{MediaURL: "a", ID: "same"},
		{MediaURL: "b", ID: "same"},
		{MediaURL: "c"},
	}, PersonBride)
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	seen := map[string]bool{}
	for i, post := range out {
		if post.ID == "" {
			t.Fatalf("post %d left without an id", i)
		}
		if seen[post.ID] {
			t.Fatalf("post %d reused id %q", i, post.ID)
		}
		seen[post.ID] = true
		if post.MediaType != "IMAGE" {
			t.Fatalf("post %d should default to IMAGE, got %q", i, post.MediaType)
		}
	}
}
