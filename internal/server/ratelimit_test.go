package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jadeejoao/jadeejoao-api/internal/content"
	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

func TestPublicPostRateLimiting(t *testing.T) {
	cfg := &platform.Config{CORSOrigins: []string{"http://localhost:5173"}}
	router := NewRouter(cfg, Deps{
		Content: content.NewService(fakeContentRepo{}),
		Auth:    fakeAuth{},
	})

	// Empty body: schema validation answers 422 without touching services,
	// so the limiter is the only stateful thing exercised.
	post := func(ip string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/guests/lookup", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests && !strings.Contains(rec.Body.String(), "Muitas tentativas") {
			t.Fatalf("429 detail must be PT-BR: %s", rec.Body.String())
		}
		return rec.Code
	}

	// Hammer one IP until the bucket runs dry.
	var got429 bool
	for i := 0; i < publicPostBurst+5; i++ {
		if post("9.9.9.9") == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatal("public POST was never rate limited")
	}

	// Ordinary GETs (everything except /guests/suggest) are never limited.
	for i := 0; i < publicPostBurst+5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/content", nil)
		req.RemoteAddr = "9.9.9.9:1234"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatal("content GET must not be rate limited")
		}
	}

	// A different IP still has budget for POSTs.
	if post("8.8.8.8") == http.StatusTooManyRequests {
		t.Fatal("distinct IPs must not share buckets")
	}
}

// TestSuggestRateLimitIsolatedFromPosts: the typeahead has its own bucket —
// it must 429 under hammering (the AD-5 tradeoff depends on it) without
// draining the write budget of the same IP.
func TestSuggestRateLimitIsolatedFromPosts(t *testing.T) {
	cfg := &platform.Config{CORSOrigins: []string{"http://localhost:5173"}}
	router := NewRouter(cfg, Deps{Content: content.NewService(fakeContentRepo{}), Auth: fakeAuth{}})

	// q too short → schema 422 before any service call, so a nil guests
	// service is safe; the limiter runs before Huma either way.
	suggest := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/guests/suggest?q=ab", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	var got429 bool
	for i := 0; i < suggestBurst+10; i++ {
		if suggest("4.4.4.4") == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatal("suggest was never rate limited")
	}

	// The SAME IP still has full POST budget: suggest chatter must not
	// starve lookup/RSVP.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/guests/lookup", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "4.4.4.4:1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Fatal("suggest bucket drained the POST budget")
	}
}

// TestRateLimitKeyIgnoresSpoofedForwardedFor: the limiter must key on the
// proxy-appended (last) X-Forwarded-For hop; a client minting random first
// entries must not get fresh buckets.
func TestRateLimitKeyIgnoresSpoofedForwardedFor(t *testing.T) {
	cfg := &platform.Config{CORSOrigins: []string{"http://localhost:5173"}}
	router := NewRouter(cfg, Deps{Content: content.NewService(fakeContentRepo{}), Auth: fakeAuth{}})

	var got429 bool
	for i := 0; i < publicPostBurst+5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/guests/lookup", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		// Random client-supplied first hop, fixed proxy-appended last hop.
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("10.0.0.%d, 6.6.6.6", i))
		req.RemoteAddr = "127.0.0.1:1234"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatal("spoofed first-hop XFF bypassed the rate limit")
	}
}
