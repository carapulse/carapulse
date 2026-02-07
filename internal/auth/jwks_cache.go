package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

type JWKS struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksEntry struct {
	jwks      JWKS
	fetchedAt time.Time
	expiresAt time.Time

	refreshing bool
	backoff    time.Duration
}

type JWKSCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	client  *http.Client
	entries map[string]*jwksEntry
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewJWKSCache(ttl time.Duration) *JWKSCache {
	if ttl <= 0 {
		ttl = time.Hour
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &JWKSCache{
		ttl:     ttl,
		client:  &http.Client{Timeout: 10 * time.Second},
		entries: map[string]*jwksEntry{},
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (c *JWKSCache) Close() {
	if c == nil || c.cancel == nil {
		return
	}
	c.cancel()
}

func (c *JWKSCache) Get(ctx context.Context, url string) (JWKS, error) {
	if c == nil {
		return JWKS{}, errors.New("jwks cache nil")
	}
	u := strings.TrimSpace(url)
	if u == "" {
		return JWKS{}, errors.New("jwks url required")
	}

	now := time.Now()
	c.mu.RLock()
	ent := c.entries[u]
	if ent != nil && !ent.expiresAt.IsZero() && now.Before(ent.expiresAt) {
		jwks := ent.jwks
		c.mu.RUnlock()
		return jwks, nil
	}
	c.mu.RUnlock()

	jwks, err := fetchJWKS(ctx, c.client, u)
	if err != nil {
		return JWKS{}, err
	}

	c.mu.Lock()
	ent = c.entries[u]
	if ent == nil {
		ent = &jwksEntry{}
		c.entries[u] = ent
	}
	ent.jwks = jwks
	ent.fetchedAt = now
	ent.expiresAt = now.Add(c.ttl)
	if ent.backoff <= 0 {
		ent.backoff = time.Second
	}
	if !ent.refreshing {
		ent.refreshing = true
		go c.refreshLoop(u)
	}
	c.mu.Unlock()

	return jwks, nil
}

func (c *JWKSCache) refreshLoop(url string) {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		c.mu.RLock()
		ent := c.entries[url]
		ttl := c.ttl
		var fetchedAt time.Time
		var backoff time.Duration
		if ent != nil {
			fetchedAt = ent.fetchedAt
			backoff = ent.backoff
		}
		c.mu.RUnlock()

		if ttl <= 0 {
			ttl = time.Hour
		}
		if backoff <= 0 {
			backoff = time.Second
		}

		refreshAfter := ttl * 9 / 10
		if refreshAfter < 10*time.Second {
			refreshAfter = ttl / 2
		}
		next := fetchedAt.Add(refreshAfter)
		delay := time.Until(next)
		if delay < 0 {
			delay = 0
		}
		timer := time.NewTimer(delay)
		select {
		case <-c.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
		jwks, err := fetchJWKS(ctx, c.client, url)
		cancel()
		if err == nil {
			now := time.Now()
			c.mu.Lock()
			if ent := c.entries[url]; ent != nil {
				ent.jwks = jwks
				ent.fetchedAt = now
				ent.expiresAt = now.Add(c.ttl)
				ent.backoff = time.Second
			}
			c.mu.Unlock()
			continue
		}

		// Backoff on fetch failures (1s -> 2s -> ... -> 5min max).
		if backoff > 5*time.Minute {
			backoff = 5 * time.Minute
		}
		timer = time.NewTimer(backoff)
		select {
		case <-c.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		c.mu.Lock()
		if ent := c.entries[url]; ent != nil {
			ent.backoff *= 2
			if ent.backoff <= 0 {
				ent.backoff = 2 * time.Second
			}
			if ent.backoff > 5*time.Minute {
				ent.backoff = 5 * time.Minute
			}
		}
		c.mu.Unlock()
	}
}

func fetchJWKS(ctx context.Context, client *http.Client, url string) (JWKS, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return JWKS{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return JWKS{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return JWKS{}, errors.New("jwks fetch failed")
	}
	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return JWKS{}, err
	}
	return jwks, nil
}
