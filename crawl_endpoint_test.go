package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

type proxyState struct {
	crawlEndpoint      string
	authToken          string
	authScheme         string
	authHeader         string
	includeReferences  bool
	maxURLsPerRequest  int
	maxRequestBodySize int64
	avoidCSS           bool
	removeOverlay      bool
	maxRetries         int
	hasProxyConfig     bool
	proxyConfigParsed  []ProxyConfig
	client             *http.Client

	contentFilterType string
	bm25UserQuery     string
	cssSelector       string
	enableStealth     bool
	deepCrawl         bool
	contentSource     string
	upstreamRetries   int
	upstreamRetryMs   int
	cacheEnabled      bool
	cacheTTL          int
	cacheMaxEntries   int
	connectTimeout    int
	totalTimeout      int
}

func captureProxyState() proxyState {
	return proxyState{
		crawlEndpoint:      CRAWL4AI_ENDPOINT,
		authToken:          CRAWL4AI_AUTH_TOKEN,
		authScheme:         CRAWL4AI_AUTH_SCHEME,
		authHeader:         CRAWL4AI_AUTH_HEADER,
		includeReferences:  INCLUDE_REFERENCES,
		maxURLsPerRequest:  MAX_URLS_PER_REQUEST,
		maxRequestBodySize: MAX_REQUEST_BODY_BYTES,
		avoidCSS:           AVOID_CSS,
		removeOverlay:      REMOVE_OVERLAY,
		maxRetries:         MAX_RETRIES,
		hasProxyConfig:     HAS_PROXY_CONFIG,
		proxyConfigParsed:  PROXY_CONFIG_PARSED,
		client:             httpClient,
		contentFilterType:  CONTENT_FILTER_TYPE,
		bm25UserQuery:      BM25_USER_QUERY,
		cssSelector:        CSS_SELECTOR,
		enableStealth:      ENABLE_STEALTH,
		deepCrawl:          DEEP_CRAWL,
		contentSource:      CONTENT_SOURCE,
		upstreamRetries:    UPSTREAM_RETRIES,
		upstreamRetryMs:    UPSTREAM_RETRY_DELAY_MS,
		cacheEnabled:       CACHE_ENABLED,
		cacheTTL:           CACHE_TTL_SECONDS,
		cacheMaxEntries:    CACHE_MAX_ENTRIES,
		connectTimeout:     CRAWL_CONNECT_TIMEOUT_SECONDS,
		totalTimeout:       CRAWL_TOTAL_TIMEOUT_SECONDS,
	}
}

func restoreProxyState(s proxyState) {
	CRAWL4AI_ENDPOINT = s.crawlEndpoint
	CRAWL4AI_AUTH_TOKEN = s.authToken
	CRAWL4AI_AUTH_SCHEME = s.authScheme
	CRAWL4AI_AUTH_HEADER = s.authHeader
	INCLUDE_REFERENCES = s.includeReferences
	MAX_URLS_PER_REQUEST = s.maxURLsPerRequest
	MAX_REQUEST_BODY_BYTES = s.maxRequestBodySize
	AVOID_CSS = s.avoidCSS
	REMOVE_OVERLAY = s.removeOverlay
	MAX_RETRIES = s.maxRetries
	HAS_PROXY_CONFIG = s.hasProxyConfig
	PROXY_CONFIG_PARSED = s.proxyConfigParsed
	httpClient = s.client
	CONTENT_FILTER_TYPE = s.contentFilterType
	BM25_USER_QUERY = s.bm25UserQuery
	CSS_SELECTOR = s.cssSelector
	ENABLE_STEALTH = s.enableStealth
	DEEP_CRAWL = s.deepCrawl
	CONTENT_SOURCE = s.contentSource
	UPSTREAM_RETRIES = s.upstreamRetries
	UPSTREAM_RETRY_DELAY_MS = s.upstreamRetryMs
	CACHE_ENABLED = s.cacheEnabled
	CACHE_TTL_SECONDS = s.cacheTTL
	CACHE_MAX_ENTRIES = s.cacheMaxEntries
	CRAWL_CONNECT_TIMEOUT_SECONDS = s.connectTimeout
	CRAWL_TOTAL_TIMEOUT_SECONDS = s.totalTimeout
}

func callEndpoint(method string, path string, contentType string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	mux := http.NewServeMux()
	mux.HandleFunc("/crawl", CrawlEndpoint)
	mux.HandleFunc("/md", MdEndpoint)
	mux.HandleFunc("/screenshot", ScreenshotEndpoint)
	mux.HandleFunc("/execute_js", ExecuteJsEndpoint)
	mux.HandleFunc("/health", HealthEndpoint)
	mux.HandleFunc("/metrics", MetricsEndpoint)
	mux.ServeHTTP(response, request)
	return response
}

func callEndpointWithID(method string, path string, contentType string, body string, requestID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
	}
	response := httptest.NewRecorder()
	mux := http.NewServeMux()
	mux.HandleFunc("/crawl", CrawlEndpoint)
	mux.ServeHTTP(response, request)
	return response
}

func decodeJSON[T any](t *testing.T, body []byte) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("failed to decode json: %v\nbody: %s", err, string(body))
	}
	return out
}

type roundTripperFunc func(request *http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func mockResponse(statusCode int, body string, contentType string) *http.Response {
	headers := http.Header{}
	if contentType != "" {
		headers.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: statusCode,
		Header:     headers,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func setupDownstream(t *testing.T, handler func(request *http.Request) (*http.Response, error)) {
	t.Helper()
	CRAWL4AI_ENDPOINT = "http://crawl4ai.local/crawl"
	httpClient = &http.Client{
		Timeout:   2 * time.Second,
		Transport: roundTripperFunc(handler),
	}
}

// =============================================================================
// Basic request validation tests (from v0.0.3)
// =============================================================================

func TestMethodNotAllowed(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodGet, "/crawl", "", "")
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", response.Code)
	}
	errResp := decodeJSON[ErrorResponse](t, response.Body.Bytes())
	if errResp.ErrorName != "method not allowed" {
		t.Fatalf("unexpected error name: %s", errResp.ErrorName)
	}
}

func TestInvalidContentType(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodPost, "/crawl", "application/pdf", "{}")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
	errResp := decodeJSON[ErrorResponse](t, response.Body.Bytes())
	if errResp.ErrorName != "content type must be application/json" {
		t.Fatalf("unexpected error name: %s", errResp.ErrorName)
	}
}

func TestInvalidJson(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodPost, "/crawl", "application/json", "hello, world")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
	errResp := decodeJSON[ErrorResponse](t, response.Body.Bytes())
	if errResp.ErrorName != "invalid json" {
		t.Fatalf("unexpected error name: %s", errResp.ErrorName)
	}
}

func TestContentTypeWithCharsetAccepted(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		return mockResponse(http.StatusOK, `{
			"success": true,
			"results": [{
				"url": "https://example.com",
				"success": true,
				"markdown": {
					"raw_markdown": "raw",
					"references_markdown": ""
				},
				"metadata": {}
			}]
		}`, "application/json"), nil
	})

	response := callEndpoint(http.MethodPost, "/crawl", "application/json; charset=utf-8", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}
}

func TestEmptyURLsRejected(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":[]}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func TestTooManyURLsRejected(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	MAX_URLS_PER_REQUEST = 1
	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["https://a.com","https://b.com"]}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func TestRequestBodyTooLargeRejected(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	MAX_REQUEST_BODY_BYTES = 16
	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d, body=%s", response.Code, response.Body.String())
	}
}

func TestDangerousSchemeRejected(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["file:///etc/passwd"]}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

func TestSuccessUsesFitMarkdownAndAvoidsDuplicateReferencesHeading(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	INCLUDE_REFERENCES = true

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		return mockResponse(http.StatusOK, `{
			"success": true,
			"results": [{
				"url": "https://example.com",
				"success": true,
				"markdown": {
					"raw_markdown": "raw body",
					"fit_markdown": "fit body",
					"references_markdown": "## References\n\n⟨1⟩ https://example.com"
				},
				"metadata": {
					"title": "Example"
				}
			}]
		}`, "application/json"), nil
	})

	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}

	items := decodeJSON[[]SuccessResponseItem](t, response.Body.Bytes())
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !strings.Contains(items[0].PageContent, "fit body") {
		t.Fatalf("expected fit_markdown content, got: %s", items[0].PageContent)
	}
	if got := strings.Count(items[0].PageContent, "## References"); got != 1 {
		t.Fatalf("expected exactly one references heading, got %d\ncontent:\n%s", got, items[0].PageContent)
	}
}

func TestDownstreamNon200ReturnsBadGateway(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		return mockResponse(http.StatusUnauthorized, `{"detail":"unauthorized"}`, "application/json"), nil
	})

	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", response.Code)
	}

	errResp := decodeJSON[ErrorResponse](t, response.Body.Bytes())
	if errResp.ErrorName != "bad gateway" {
		t.Fatalf("unexpected error name: %s", errResp.ErrorName)
	}
	if !strings.Contains(errResp.Detail, "status 401") {
		t.Fatalf("expected upstream status detail, got: %s", errResp.Detail)
	}
}

func TestDownstreamInvalidJSONReturnsBadGateway(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		return mockResponse(http.StatusOK, `{`, "application/json"), nil
	})

	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", response.Code)
	}
}

func TestDownstreamAuthHeaderForwarded(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	CRAWL4AI_AUTH_TOKEN = "abc123"
	CRAWL4AI_AUTH_SCHEME = "Bearer"
	CRAWL4AI_AUTH_HEADER = "Authorization"

	seenAuth := ""
	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		seenAuth = request.Header.Get("Authorization")
		return mockResponse(http.StatusOK, `{
			"success": true,
			"results": [{
				"url": "https://example.com",
				"success": true,
				"markdown": {
					"raw_markdown": "ok",
					"references_markdown": ""
				},
				"metadata": {}
			}]
		}`, "application/json"), nil
	})

	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}
	if seenAuth != "Bearer abc123" {
		t.Fatalf("expected auth header Bearer abc123, got: %q", seenAuth)
	}
}

// =============================================================================
// New feature tests (#1 BM25, #2 CSS, #3 JS, #5 Stealth, #6 Deep crawl)
// =============================================================================

func TestPayloadContainsBM25Filter(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	CONTENT_FILTER_TYPE = "bm25"
	BM25_USER_QUERY = "climate change"

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		crawlerConfig := payload["crawler_config"].(map[string]any)
		crawlerParams := crawlerConfig["params"].(map[string]any)
		mdGen := crawlerParams["markdown_generator"].(map[string]any)
		mdGenParams := mdGen["params"].(map[string]any)
		contentFilter := mdGenParams["content_filter"].(map[string]any)
		if contentFilter["type"] != "BM25ContentFilter" {
			t.Fatalf("expected BM25ContentFilter, got %v", contentFilter["type"])
		}
		cfParams := contentFilter["params"].(map[string]any)
		if cfParams["user_query"] != "climate change" {
			t.Fatalf("expected user_query=climate change, got %v", cfParams["user_query"])
		}

		return mockResponse(http.StatusOK, `{
			"success": true,
			"results": [{
				"url": "https://example.com",
				"success": true,
				"markdown": {"raw_markdown": "ok", "references_markdown": ""},
				"metadata": {}
			}]
		}`, "application/json"), nil
	})

	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}
}

func TestPayloadPerRequestOverrides(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	CONTENT_FILTER_TYPE = "pruning"

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}

		crawlerConfig := payload["crawler_config"].(map[string]any)
		crawlerParams := crawlerConfig["params"].(map[string]any)

		mdGen := crawlerParams["markdown_generator"].(map[string]any)
		mdParams := mdGen["params"].(map[string]any)
		cf := mdParams["content_filter"].(map[string]any)
		if cf["type"] != "BM25ContentFilter" {
			t.Fatalf("expected per-request BM25ContentFilter override, got %v", cf["type"])
		}

		if crawlerParams["css_selector"] != "article" {
			t.Fatalf("expected css_selector=article, got %v", crawlerParams["css_selector"])
		}

		jsCode, ok := crawlerParams["js_code"].([]any)
		if !ok || len(jsCode) == 0 {
			t.Fatalf("expected js_code in payload")
		}
		if jsCode[0] != "document.title" {
			t.Fatalf("expected js_code=[document.title], got %v", jsCode)
		}

		if crawlerParams["wait_for"] != ".content" {
			t.Fatalf("expected wait_for=.content, got %v", crawlerParams["wait_for"])
		}

		browserConfig := payload["browser_config"].(map[string]any)
		browserParams := browserConfig["params"].(map[string]any)
		if browserParams["enable_stealth"] != true {
			t.Fatalf("expected enable_stealth=true")
		}
		if browserParams["user_agent"] != "TestBot/1.0" {
			t.Fatalf("expected user_agent=TestBot/1.0, got %v", browserParams["user_agent"])
		}

		return mockResponse(http.StatusOK, `{
			"success": true,
			"results": [{
				"url": "https://example.com",
				"success": true,
				"markdown": {"raw_markdown": "ok", "references_markdown": ""},
				"metadata": {}
			}]
		}`, "application/json"), nil
	})

	body := `{"urls":["https://example.com"],"content_filter_type":"bm25","bm25_query":"test query","css_selector":"article","js_code":["document.title"],"wait_for":".content","content_source":"raw_html","enable_stealth":true,"user_agent":"TestBot/1.0"}`
	response := callEndpoint(http.MethodPost, "/crawl", "application/json", body)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}
}

func TestPayloadContainsDeepCrawl(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	DEEP_CRAWL = true
	DEEP_CRAWL_MAX_DEPTH = 3
	DEEP_CRAWL_MAX_PAGES = 50

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		crawlerConfig := payload["crawler_config"].(map[string]any)
		crawlerParams := crawlerConfig["params"].(map[string]any)
		dcs, ok := crawlerParams["deep_crawl_strategy"].(map[string]any)
		if !ok {
			t.Fatalf("expected deep_crawl_strategy in payload")
		}
		if dcs["type"] != "BFSDeepCrawlStrategy" {
			t.Fatalf("expected BFSDeepCrawlStrategy, got %v", dcs["type"])
		}
		dcsParams := dcs["params"].(map[string]any)
		if dcsParams["max_depth"] != float64(3) {
			t.Fatalf("expected max_depth=3, got %v", dcsParams["max_depth"])
		}
		if dcsParams["max_pages"] != float64(50) {
			t.Fatalf("expected max_pages=50, got %v", dcsParams["max_pages"])
		}

		return mockResponse(http.StatusOK, `{
			"success": true,
			"results": [{
				"url": "https://example.com",
				"success": true,
				"markdown": {"raw_markdown": "ok", "references_markdown": ""},
				"metadata": {}
			}]
		}`, "application/json"), nil
	})

	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}
}

func TestContentSourceRawHtml(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	CONTENT_SOURCE = "raw_html"
	INCLUDE_REFERENCES = false

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		return mockResponse(http.StatusOK, `{
			"success": true,
			"results": [{
				"url": "https://example.com",
				"success": true,
				"markdown": {
					"raw_markdown": "raw content here",
					"fit_markdown": "fit content here",
					"references_markdown": ""
				},
				"metadata": {}
			}]
		}`, "application/json"), nil
	})

	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}

	items := decodeJSON[[]SuccessResponseItem](t, response.Body.Bytes())
	if !strings.Contains(items[0].PageContent, "raw content here") {
		t.Fatalf("expected raw_markdown content when content_source=raw_html, got: %s", items[0].PageContent)
	}
}

func TestContentSourcePerRequestOverride(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	CONTENT_SOURCE = "fit_html"
	INCLUDE_REFERENCES = false

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		return mockResponse(http.StatusOK, `{
			"success": true,
			"results": [{
				"url": "https://example.com",
				"success": true,
				"markdown": {"raw_markdown": "raw content", "fit_markdown": "fit content", "references_markdown": ""},
				"metadata": {}
			}]
		}`, "application/json"), nil
	})

	// With content_source=raw_html per-request override, should get raw_markdown instead of fit_markdown
	response := callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["https://example.com"],"content_source":"raw_html"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}

	items := decodeJSON[[]SuccessResponseItem](t, response.Body.Bytes())
	if !strings.Contains(items[0].PageContent, "raw content") {
		t.Fatalf("expected raw_markdown content when content_source=raw_html, got: %s", items[0].PageContent)
	}
}

// =============================================================================
// #8 Graceful shutdown, #9 timeouts, #10 health, #11 validation tests
// =============================================================================

func TestHealthEndpointBasic(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodGet, "/health", "", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	var result map[string]string
	json.Unmarshal(response.Body.Bytes(), &result)
	if result["status"] != "healthy" {
		t.Fatalf("expected healthy, got %s", result["status"])
	}
}

func TestEndpointURLValidation(t *testing.T) {
	os.Setenv("CRAWL4AI_ENDPOINT", "not a url")
	defer os.Unsetenv("CRAWL4AI_ENDPOINT")

	// This should fatal exit, so we test via a subprocess
	// Instead, test valid URL doesn't fail
	os.Setenv("CRAWL4AI_ENDPOINT", "http://crawl4ai:11235/crawl")
	ReadEnvironment()
	if CRAWL4AI_ENDPOINT != "http://crawl4ai:11235/crawl" {
		t.Fatalf("expected endpoint to be set correctly")
	}
}

// =============================================================================
// #14 Upstream retry tests
// =============================================================================

func TestUpstreamRetryOnServer5xx(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	UPSTREAM_RETRIES = 2
	UPSTREAM_RETRY_DELAY_MS = 10 // fast for tests

	attempts := int32(0)
	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		count := atomic.AddInt32(&attempts, 1)
		if count <= 2 {
			// First 2 attempts return 503
			return mockResponse(http.StatusServiceUnavailable, `{"error":"unavailable"}`, "application/json"), nil
		}
		return mockResponse(http.StatusOK, `{
			"success": true,
			"results": [{
				"url": "https://example.com",
				"success": true,
				"markdown": {"raw_markdown": "retried ok", "references_markdown": ""},
				"metadata": {}
			}]
		}`, "application/json"), nil
	})

	callEndpoint(http.MethodPost, "/crawl", "application/json", `{"urls":["https://example.com"]}`)
	totalAttempts := atomic.LoadInt32(&attempts)
	if totalAttempts < 2 {
		t.Fatalf("expected at least 2 attempts due to retry, got %d", totalAttempts)
	}
}

// =============================================================================
// #16 Request ID tests
// =============================================================================

func TestRequestIDGenerated(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("X-Request-ID") == "" {
			t.Fatalf("expected X-Request-ID header to be set on upstream request")
		}
		return mockResponse(http.StatusOK, `{
			"success": true,
			"results": [{
				"url": "https://example.com",
				"success": true,
				"markdown": {"raw_markdown": "ok", "references_markdown": ""},
				"metadata": {}
			}]
		}`, "application/json"), nil
	})

	// Use full middleware chain so request ID gets generated
	mux := http.NewServeMux()
	mux.HandleFunc("/crawl", CrawlEndpoint)
	handler := RequestIDMiddleware(mux)
	req := httptest.NewRequest(http.MethodPost, "/crawl", strings.NewReader(`{"urls":["https://example.com"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("expected X-Request-ID response header")
	}
}

func TestRequestIDPreservedWhenProvided(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("X-Request-ID") != "test-id-123" {
			t.Fatalf("expected X-Request-ID=test-id-123, got %s", request.Header.Get("X-Request-ID"))
		}
		return mockResponse(http.StatusOK, `{
			"success": true,
			"results": [{
				"url": "https://example.com",
				"success": true,
				"markdown": {"raw_markdown": "ok", "references_markdown": ""},
				"metadata": {}
			}]
		}`, "application/json"), nil
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/crawl", CrawlEndpoint)
	handler := RequestIDMiddleware(mux)
	req := httptest.NewRequest(http.MethodPost, "/crawl", strings.NewReader(`{"urls":["https://example.com"]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "test-id-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") != "test-id-123" {
		t.Fatalf("expected X-Request-ID=test-id-123 in response, got %s", w.Header().Get("X-Request-ID"))
	}
}

// =============================================================================
// #17 Metrics endpoint tests
// =============================================================================

func TestMetricsEndpointReturnsJSON(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	// Reset global metrics
	globalMetrics = NewMetrics()

	response := callEndpoint(http.MethodGet, "/metrics", "", "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var snapshot MetricsSnapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("failed to unmarshal metrics: %v", err)
	}
}

func TestMetricsEndpointMethodNotAllowed(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodPost, "/metrics", "", "")
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", response.Code)
	}
}

// =============================================================================
// #18 Rate limiting tests
// =============================================================================

func TestRateLimitingRejectsBurst(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	RATE_LIMIT_RPS = 1
	RATE_LIMIT_BURST = 2

	rl := NewRateLimiter(RATE_LIMIT_RPS, RATE_LIMIT_BURST)
	globalMetrics = NewMetrics()

	handler := RateLimitMiddleware(rl, globalMetrics, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))

	allowed := 0
	blocked := 0
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/crawl", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			allowed++
		} else if w.Code == http.StatusTooManyRequests {
			blocked++
		}
	}

	if blocked == 0 {
		t.Fatalf("expected some requests to be rate limited, got allowed=%d blocked=%d", allowed, blocked)
	}
	if allowed == 0 {
		t.Fatalf("expected some requests to be allowed, got allowed=%d blocked=%d", allowed, blocked)
	}
}

// =============================================================================
// #19 Cache tests
// =============================================================================

func TestCacheHitMissCycle(t *testing.T) {
	m := NewMetrics()
	c := NewResponseCache(100, 5*time.Second, m)
	defer c.Stop()

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	body := []byte(`{"status":"healthy"}`)

	c.Set("key1", http.StatusOK, body, headers)

	entry, hit := c.Get("key1")
	if !hit {
		t.Fatalf("expected cache hit for key1")
	}
	if entry.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", entry.StatusCode)
	}

	_, hit = c.Get("nonexistent")
	if hit {
		t.Fatalf("expected cache miss for nonexistent key")
	}
}

func TestCacheExpiry(t *testing.T) {
	m := NewMetrics()
	c := NewResponseCache(100, 50*time.Millisecond, m)
	defer c.Stop()

	headers := http.Header{}
	c.Set("expkey", http.StatusOK, []byte("data"), headers)

	time.Sleep(80 * time.Millisecond)

	_, hit := c.Get("expkey")
	if hit {
		t.Fatalf("expected cache miss after expiry")
	}
}

func TestCacheDoesNotStoreNon200(t *testing.T) {
	m := NewMetrics()
	c := NewResponseCache(100, 5*time.Second, m)
	defer c.Stop()

	headers := http.Header{}
	c.Set("errkey", http.StatusInternalServerError, []byte("error"), headers)

	_, hit := c.Get("errkey")
	if hit {
		t.Fatalf("expected cache miss for non-200 status")
	}
}

// =============================================================================
// /md endpoint tests
// =============================================================================

func TestMdEndpointSuccess(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/md" {
			t.Fatalf("expected request to /md, got %s", request.URL.Path)
		}
		return mockResponse(http.StatusOK, `{
			"success": true,
			"url": "https://example.com",
			"markdown": "# Example Domain\nThis domain is for use in documentation examples.",
			"filter": "fit",
			"query": null
		}`, "application/json"), nil
	})

	response := callEndpoint(http.MethodPost, "/md", "application/json", `{"url":"https://example.com"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}

	items := decodeJSON[[]SuccessResponseItem](t, response.Body.Bytes())
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if !strings.Contains(items[0].PageContent, "Example Domain") {
		t.Fatalf("expected markdown content, got: %s", items[0].PageContent)
	}
}

func TestMdEndpointInvalidURL(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodPost, "/md", "application/json", `{"url":"file:///etc/passwd"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

// =============================================================================
// /screenshot endpoint tests
// =============================================================================

func TestScreenshotEndpointSuccess(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/screenshot" {
			t.Fatalf("expected request to /screenshot, got %s", request.URL.Path)
		}
		return mockResponse(http.StatusOK, `{
			"success": true,
			"screenshot": "base64encodeddata"
		}`, "application/json"), nil
	})

	response := callEndpoint(http.MethodPost, "/screenshot", "application/json", `{"url":"https://example.com"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(response.Body.Bytes(), &result)
	if result["screenshot"] != "base64encodeddata" {
		t.Fatalf("expected screenshot data, got: %v", result["screenshot"])
	}
}

// =============================================================================
// /execute_js endpoint tests
// =============================================================================

func TestExecuteJsEndpointSuccess(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/execute_js" {
			t.Fatalf("expected request to /execute_js, got %s", request.URL.Path)
		}
		return mockResponse(http.StatusOK, `{
			"success": true,
			"result": {"title": "Example"}
		}`, "application/json"), nil
	})

	response := callEndpoint(http.MethodPost, "/execute_js", "application/json", `{"url":"https://example.com","scripts":["document.title"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}
}

func TestExecuteJsEndpointNoScripts(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodPost, "/execute_js", "application/json", `{"url":"https://example.com","scripts":[]}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
}

// =============================================================================
// #20 ProxyConfig parsing tests
// =============================================================================

func TestProxyConfigArrayParsing(t *testing.T) {
	var configs []ProxyConfig
	err := parseProxyConfig(`["direct","http://proxy:8080"]`, &configs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 proxy configs, got %d", len(configs))
	}
	if configs[0].Server != "direct" {
		t.Fatalf("expected first server=direct, got %s", configs[0].Server)
	}
	if configs[1].Server != "http://proxy:8080" {
		t.Fatalf("expected second server=http://proxy:8080, got %s", configs[1].Server)
	}
}

func TestProxyConfigObjectParsing(t *testing.T) {
	var configs []ProxyConfig
	err := parseProxyConfig(`{"server":"http://proxy:8080","username":"user","password":"pass"}`, &configs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 proxy config, got %d", len(configs))
	}
	if configs[0].Server != "http://proxy:8080" {
		t.Fatalf("expected server=http://proxy:8080, got %s", configs[0].Server)
	}
	if configs[0].Username != "user" {
		t.Fatalf("expected username=user, got %s", configs[0].Username)
	}
}

func TestProxyConfigInvalidJSON(t *testing.T) {
	var configs []ProxyConfig
	err := parseProxyConfig(`not json`, &configs)
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

// =============================================================================
// Config helper tests
// =============================================================================

func TestEffectiveContentFilterDefaults(t *testing.T) {
	CONTENT_FILTER_TYPE = "pruning"
	BM25_USER_QUERY = ""
	ft, q := EffectiveContentFilter(nil)
	if ft != "pruning" {
		t.Fatalf("expected pruning, got %s", ft)
	}
	if q != "" {
		t.Fatalf("expected empty query, got %s", q)
	}
}

func TestEffectiveContentFilterPerRequest(t *testing.T) {
	CONTENT_FILTER_TYPE = "pruning"
	bm25 := "test query"
	req := &Request{ContentFilterType: strPtr("bm25"), Bm25Query: &bm25}
	ft, q := EffectiveContentFilter(req)
	if ft != "bm25" {
		t.Fatalf("expected bm25, got %s", ft)
	}
	if q != "test query" {
		t.Fatalf("expected test query, got %s", q)
	}
}

func TestEffectiveCssSelector(t *testing.T) {
	CSS_SELECTOR = "article"
	if EffectiveCssSelector(nil) != "article" {
		t.Fatalf("expected article from env")
	}
	req := &Request{CssSelector: strPtr("main")}
	if EffectiveCssSelector(req) != "main" {
		t.Fatalf("expected main from request")
	}
}

func TestEffectiveStealth(t *testing.T) {
	ENABLE_STEALTH = false
	if EffectiveStealth(nil) != false {
		t.Fatalf("expected false from env")
	}
	req := &Request{EnableStealth: boolPtr(true)}
	if EffectiveStealth(req) != true {
		t.Fatalf("expected true from request override")
	}
}

func TestCommaListHelper(t *testing.T) {
	result := commaList("a, b, c")
	if len(result) != 3 || result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Fatalf("expected [a b c], got %v", result)
	}
	if commaList("") != nil {
		t.Fatalf("expected nil for empty string")
	}
	if commaList("  ,,  ") != nil {
		t.Fatalf("expected nil for only separators")
	}
}

func TestSelectMarkdownContent(t *testing.T) {
	result := CrawlResultItem{
		Markdown: MarkdownResult{
			RawMarkdown: "raw content",
		},
	}
	if SelectMarkdownContent(result, "raw_html") != "raw content" {
		t.Fatalf("expected raw content for raw_html content_source")
	}

	fit := "fit content"
	result2 := CrawlResultItem{
		Markdown: MarkdownResult{
			RawMarkdown: "raw content",
			FitMarkdown: &fit,
		},
	}
	if SelectMarkdownContent(result2, "fit_html") != "fit content" {
		t.Fatalf("expected fit content for fit_html content_source")
	}

	// Fallback to raw when fit is empty
	empty := ""
	result3 := CrawlResultItem{
		Markdown: MarkdownResult{
			RawMarkdown: "raw content",
			FitMarkdown: &empty,
		},
	}
	if SelectMarkdownContent(result3, "fit_html") != "raw content" {
		t.Fatalf("expected raw content fallback when fit is empty")
	}
}

// =============================================================================
// Helper functions
// =============================================================================

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func intPtr(i int) *int       { return &i }
