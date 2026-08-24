// Package instagram proxies the couple's Instagram feeds for the public site.
// Tokens come from env (Instagram API with Instagram Login, long-lived); the
// service caches each feed in-process so guests never hit Meta directly and a
// broken token degrades to the last good feed (or an empty one), never an
// error page.
package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Person keys, mirrored by the {person} path parameter.
const (
	PersonBride = "bride"
	PersonGroom = "groom"
)

const (
	defaultBaseURL = "https://graph.instagram.com"
	feedLimit      = 12
	cacheTTL       = time.Hour
	// A failing token (expired, revoked, rate-limited) is retried at most
	// this often, and the last good feed keeps serving meanwhile.
	failureTTL = 10 * time.Minute
)

// PostView is one feed entry, shaped for the public site. JSON tags match the
// Graph API media fields, so the upstream response unmarshals directly.
type PostView struct {
	ID           string `json:"id"`
	Caption      string `json:"caption,omitempty"`
	MediaType    string `json:"media_type" example:"IMAGE" doc:"IMAGE, VIDEO or CAROUSEL_ALBUM."`
	MediaURL     string `json:"media_url,omitempty" format:"uri"`
	ThumbnailURL string `json:"thumbnail_url,omitempty" format:"uri" doc:"Preview image, present for VIDEO posts."`
	Permalink    string `json:"permalink" format:"uri"`
	Timestamp    string `json:"timestamp,omitempty" doc:"Publish time, ISO 8601."`
}

type cacheEntry struct {
	posts     []PostView
	fetchedAt time.Time
	failed    bool
}

// Service fetches and caches one feed per person.
type Service struct {
	tokens  map[string]string
	baseURL string
	client  *http.Client

	mu    sync.Mutex
	cache map[string]cacheEntry
}

// NewService builds the provider. An empty token simply marks that person's
// feed as not configured — the endpoint stays up and answers configured=false.
func NewService(brideToken, groomToken string) *Service {
	return &Service{
		tokens: map[string]string{
			PersonBride: brideToken,
			PersonGroom: groomToken,
		},
		baseURL: defaultBaseURL,
		client:  &http.Client{Timeout: 8 * time.Second},
		cache:   map[string]cacheEntry{},
	}
}

// Configured reports whether the given person's token is present.
func (s *Service) Configured(person string) bool {
	return s != nil && s.tokens[person] != ""
}

// Posts returns the feed for person, refreshing the cache when stale. On a
// fetch failure it returns the last good feed together with the error, and
// backs off retries via failureTTL.
func (s *Service) Posts(ctx context.Context, person string) ([]PostView, error) {
	if !s.Configured(person) {
		return nil, nil
	}
	s.mu.Lock()
	entry, ok := s.cache[person]
	s.mu.Unlock()

	ttl := cacheTTL
	if entry.failed {
		ttl = failureTTL
	}
	if ok && time.Since(entry.fetchedAt) < ttl {
		return entry.posts, nil
	}

	posts, err := s.fetch(ctx, person)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.cache[person] = cacheEntry{posts: entry.posts, fetchedAt: time.Now(), failed: true}
		return entry.posts, err
	}
	s.cache[person] = cacheEntry{posts: posts, fetchedAt: time.Now()}
	return posts, nil
}

// fetch pulls the recent media of the token's owner from the Graph API.
func (s *Service) fetch(ctx context.Context, person string) ([]PostView, error) {
	query := url.Values{
		"fields":       {"id,caption,media_type,media_url,thumbnail_url,permalink,timestamp"},
		"limit":        {fmt.Sprint(feedLimit)},
		"access_token": {s.tokens[person]},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/me/media?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	// Never surface the upstream body: Meta error payloads are noisy and the
	// status code is all the log needs.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram responded %d", resp.StatusCode)
	}
	var decoded struct {
		Data []PostView `json:"data"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode feed: %w", err)
	}
	posts := make([]PostView, 0, len(decoded.Data))
	for _, post := range decoded.Data {
		// A tile needs something to show and somewhere to go.
		if post.Permalink == "" || (post.MediaURL == "" && post.ThumbnailURL == "") {
			continue
		}
		posts = append(posts, post)
	}
	return posts, nil
}
