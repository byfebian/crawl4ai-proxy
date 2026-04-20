package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

const (
	CacheBypassHeader = "Cache-Control"
	CacheBypassValue  = "no-cache"
	CacheHitHeader    = "X-Cache"
	CacheHit          = "HIT"
	CacheMiss         = "MISS"
	CacheBypass       = "BYPASS"
)

type CacheEntry struct {
	ResponseBody []byte
	StatusCode   int
	Headers      http.Header
	ExpiresAt    time.Time
}

type ResponseCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	maxSize int
	ttl     time.Duration
	metrics *Metrics
	stopCh  chan struct{}
}

func NewResponseCache(maxSize int, ttl time.Duration, metrics *Metrics) *ResponseCache {
	c := &ResponseCache{
		entries: make(map[string]*CacheEntry, maxSize),
		maxSize: maxSize,
		ttl:     ttl,
		metrics: metrics,
		stopCh:  make(chan struct{}),
	}
	go c.pruneLoop()
	return c
}

func (c *ResponseCache) Stop() {
	close(c.stopCh)
}

func (c *ResponseCache) pruneLoop() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-c.stopCh:
			return
		}
	}
}

func (c *ResponseCache) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.entries {
		if now.After(v.ExpiresAt) {
			delete(c.entries, k)
		}
	}
}

func (c *ResponseCache) Key(r *http.Request, body []byte) string {
	h := sha256.New()
	h.Write([]byte(r.Method))
	h.Write([]byte(r.URL.Path))
	h.Write(body)
	return fmt.Sprintf("%x", h.Sum(nil))[:32]
}

func (c *ResponseCache) Get(key string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry, true
}

func (c *ResponseCache) Set(key string, statusCode int, body []byte, headers http.Header) {
	if statusCode != http.StatusOK {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxSize {
		now := time.Now()
		oldestKey := ""
		oldestTime := now.Add(c.ttl * 2)
		for k, v := range c.entries {
			if v.ExpiresAt.Before(oldestTime) {
				oldestTime = v.ExpiresAt
				oldestKey = k
			}
		}
		if oldestKey != "" {
			delete(c.entries, oldestKey)
		}
	}

	c.entries[key] = &CacheEntry{
		ResponseBody: body,
		StatusCode:   statusCode,
		Headers:      headers.Clone(),
		ExpiresAt:    time.Now().Add(c.ttl),
	}
}

func ShouldBypassCache(r *http.Request) bool {
	return r.Header.Get(CacheBypassHeader) == CacheBypassValue
}

func HasPerRequestOverrides(req *Request) bool {
	return req.ContentFilterType != nil ||
		req.Bm25Query != nil ||
		req.CssSelector != nil ||
		len(req.ExcludedTags) > 0 ||
		len(req.TargetElements) > 0 ||
		len(req.JsCode) > 0 ||
		req.WaitFor != nil ||
		req.ContentSource != nil ||
		req.EnableStealth != nil ||
		req.UserAgent != nil ||
		req.DeepCrawl != nil ||
		req.DeepCrawlMaxDepth != nil ||
		req.DeepCrawlMaxPages != nil
}

func ComputeCacheKey(r *http.Request, reqBody []byte) string {
	h := sha256.New()
	h.Write([]byte(r.Method))
	h.Write([]byte(r.URL.Path))
	var sortedReq Request
	if err := json.Unmarshal(reqBody, &sortedReq); err == nil {
		urls := make([]string, len(sortedReq.Urls))
		copy(urls, sortedReq.Urls)
		sort.Strings(urls)
		for _, u := range urls {
			h.Write([]byte(u))
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:32]
}

type CacheRecorder struct {
	http.ResponseWriter
	body   []byte
	status int
}

func (cr *CacheRecorder) WriteHeader(code int) {
	cr.status = code
	cr.ResponseWriter.WriteHeader(code)
}

func (cr *CacheRecorder) Write(b []byte) (int, error) {
	if cr.status == 0 {
		cr.status = http.StatusOK
	}
	cr.body = append(cr.body, b...)
	return cr.ResponseWriter.Write(b)
}
