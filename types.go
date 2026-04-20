package main

import (
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// OpenWebUI-facing types (input and output)
// =============================================================================

type Request struct {
	Urls              []string `json:"urls"`
	ContentFilterType *string  `json:"content_filter_type,omitempty"`
	Bm25Query         *string  `json:"bm25_query,omitempty"`
	CssSelector       *string  `json:"css_selector,omitempty"`
	ExcludedTags      []string `json:"excluded_tags,omitempty"`
	TargetElements    []string `json:"target_elements,omitempty"`
	JsCode            []string `json:"js_code,omitempty"`
	WaitFor           *string  `json:"wait_for,omitempty"`
	ContentSource     *string  `json:"content_source,omitempty"`
	EnableStealth     *bool    `json:"enable_stealth,omitempty"`
	UserAgent         *string  `json:"user_agent,omitempty"`
	DeepCrawl         *bool    `json:"deep_crawl,omitempty"`
	DeepCrawlMaxDepth *int     `json:"deep_crawl_max_depth,omitempty"`
	DeepCrawlMaxPages *int     `json:"deep_crawl_max_pages,omitempty"`
}

type MdRequest struct {
	Url        string  `json:"url"`
	FilterType *string `json:"filter_type,omitempty"`
	Bm25Query  *string `json:"bm25_query,omitempty"`
}

type ScreenshotRequest struct {
	Url            string   `json:"url"`
	ScreenshotWait *float64 `json:"screenshot_wait_for,omitempty"`
	WaitForImages  *bool    `json:"wait_for_images,omitempty"`
}

type ExecuteJsRequest struct {
	Url     string   `json:"url"`
	Scripts []string `json:"scripts"`
}

type SuccessResponseItem struct {
	PageContent string            `json:"page_content"`
	Metadata    map[string]string `json:"metadata"`
}

type SuccessResponse []SuccessResponseItem

type ErrorResponse struct {
	ErrorName string `json:"error"`
	Detail    string `json:"detail,omitempty"`
}

// =============================================================================
// Crawl4AI 0.8.x response types
// =============================================================================

type MarkdownResult struct {
	RawMarkdown           string  `json:"raw_markdown"`
	MarkdownWithCitations string  `json:"markdown_with_citations"`
	ReferencesMarkdown    string  `json:"references_markdown"`
	FitMarkdown           *string `json:"fit_markdown"`
	FitHtml               *string `json:"fit_html"`
}

type CrawlResultItem struct {
	Url          string                 `json:"url"`
	Success      bool                   `json:"success"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Markdown     MarkdownResult         `json:"markdown"`
	Metadata     map[string]interface{} `json:"metadata"`
	Links        map[string]interface{} `json:"links,omitempty"`
	Media        map[string]interface{} `json:"media,omitempty"`
	Screenshot   string                 `json:"screenshot,omitempty"`
	Html         string                 `json:"html,omitempty"`
	CleanedHtml  string                 `json:"cleaned_html,omitempty"`
}

type CrawlResponse struct {
	Success bool              `json:"success"`
	Results []CrawlResultItem `json:"results"`
	Result  *CrawlResultItem  `json:"result,omitempty"`
}

func (cr *CrawlResponse) GetAllResults() []CrawlResultItem {
	if cr.Result != nil && len(cr.Results) == 0 {
		return []CrawlResultItem{*cr.Result}
	}
	return cr.Results
}

// MdResponse for /md endpoint responses from crawl4ai
type MdResponse struct {
	Success  bool   `json:"success"`
	Url      string `json:"url,omitempty"`
	Filter   string `json:"filter,omitempty"`
	Query    string `json:"query,omitempty"`
	Markdown string `json:"markdown,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ScreenshotApiResponse for /screenshot endpoint from crawl4ai
type ScreenshotApiResponse struct {
	Success    bool   `json:"success"`
	Screenshot string `json:"screenshot,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ExecuteJsApiResponse for /execute_js from crawl4ai
type ExecuteJsApiResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// =============================================================================
// ProxyConfig — typed proxy configuration
// =============================================================================

type ProxyConfig struct {
	Type     string `json:"type,omitempty"`
	Server   string `json:"server,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	IP       string `json:"ip,omitempty"`
}

// =============================================================================
// Metrics — in-memory request counters
// =============================================================================

type Metrics struct {
	TotalRequests       atomic.Int64
	TotalErrors         atomic.Int64
	UpstreamErrors      atomic.Int64
	RateLimitedRequests atomic.Int64
	CacheHits           atomic.Int64
	CacheMisses         atomic.Int64
	CacheBypasses       atomic.Int64

	mu             sync.Mutex
	RequestsByPath map[string]*int64
	TotalLatencyNs atomic.Int64
	LatencyCount   atomic.Int64
}

type MetricsSnapshot struct {
	TotalRequests       int64            `json:"total_requests"`
	TotalErrors         int64            `json:"total_errors"`
	UpstreamErrors      int64            `json:"upstream_errors"`
	RateLimitedRequests int64            `json:"rate_limited_requests"`
	CacheHits           int64            `json:"cache_hits"`
	CacheMisses         int64            `json:"cache_misses"`
	CacheBypasses       int64            `json:"cache_bypasses"`
	AvgLatencyMs        float64          `json:"avg_latency_ms"`
	RequestsByPath      map[string]int64 `json:"requests_by_path"`
}

func NewMetrics() *Metrics {
	return &Metrics{
		RequestsByPath: make(map[string]*int64),
	}
}

func (m *Metrics) RecordRequest(path string) {
	m.TotalRequests.Add(1)
	m.mu.Lock()
	if _, ok := m.RequestsByPath[path]; !ok {
		m.RequestsByPath[path] = new(int64)
	}
	m.mu.Unlock()
	atomic.AddInt64(m.RequestsByPath[path], 1)
}

func (m *Metrics) RecordLatency(d time.Duration) {
	m.TotalLatencyNs.Add(d.Nanoseconds())
	m.LatencyCount.Add(1)
}

func (m *Metrics) Snapshot(reset bool) MetricsSnapshot {
	s := MetricsSnapshot{
		TotalRequests:       m.TotalRequests.Load(),
		TotalErrors:         m.TotalErrors.Load(),
		UpstreamErrors:      m.UpstreamErrors.Load(),
		RateLimitedRequests: m.RateLimitedRequests.Load(),
		CacheHits:           m.CacheHits.Load(),
		CacheMisses:         m.CacheMisses.Load(),
		CacheBypasses:       m.CacheBypasses.Load(),
		RequestsByPath:      make(map[string]int64),
	}

	m.mu.Lock()
	for k, v := range m.RequestsByPath {
		s.RequestsByPath[k] = atomic.LoadInt64(v)
	}
	m.mu.Unlock()

	count := m.LatencyCount.Load()
	if count > 0 {
		totalNs := m.TotalLatencyNs.Load()
		s.AvgLatencyMs = float64(totalNs) / float64(count) / 1e6
	}

	if reset {
		m.TotalRequests.Store(0)
		m.TotalErrors.Store(0)
		m.UpstreamErrors.Store(0)
		m.RateLimitedRequests.Store(0)
		m.CacheHits.Store(0)
		m.CacheMisses.Store(0)
		m.CacheBypasses.Store(0)
		m.TotalLatencyNs.Store(0)
		m.LatencyCount.Store(0)
		m.mu.Lock()
		for k := range m.RequestsByPath {
			atomic.StoreInt64(m.RequestsByPath[k], 0)
		}
		m.mu.Unlock()
	}

	return s
}
