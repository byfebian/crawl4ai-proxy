package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func init() {
	log.SetOutput(io.Discard)
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
	proxyConfigJSON    string
	proxyConfigParsed  any
	client             *http.Client
}

// captureProxyState / restoreProxyState ensure tests remain isolated even though
// the service uses package-level config variables.
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
		proxyConfigJSON:    PROXY_CONFIG_JSON,
		proxyConfigParsed:  PROXY_CONFIG_PARSED,
		client:             httpClient,
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
	PROXY_CONFIG_JSON = s.proxyConfigJSON
	PROXY_CONFIG_PARSED = s.proxyConfigParsed
	httpClient = s.client
}

func callEndpoint(method string, contentType string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/crawl", strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	CrawlEndpoint(response, request)
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

func TestMethodNotAllowed(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodGet, "", "")
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

	response := callEndpoint(http.MethodPost, "application/pdf", "{}")
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

	response := callEndpoint(http.MethodPost, "application/json", "hello, world")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
	errResp := decodeJSON[ErrorResponse](t, response.Body.Bytes())
	if errResp.ErrorName != "invalid json" {
		t.Fatalf("unexpected error name: %s", errResp.ErrorName)
	}
}

// v0.0.3 regression test:
// Verify we now accept JSON content type with charset parameter.
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

	response := callEndpoint(http.MethodPost, "application/json; charset=utf-8", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}
}

func TestEmptyURLsRejected(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodPost, "application/json", `{"urls":[]}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
	errResp := decodeJSON[ErrorResponse](t, response.Body.Bytes())
	if errResp.ErrorName != "invalid request" {
		t.Fatalf("unexpected error name: %s", errResp.ErrorName)
	}
}

func TestTooManyURLsRejected(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	MAX_URLS_PER_REQUEST = 1
	response := callEndpoint(http.MethodPost, "application/json", `{"urls":["https://a.com","https://b.com"]}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
	errResp := decodeJSON[ErrorResponse](t, response.Body.Bytes())
	if errResp.ErrorName != "invalid request" {
		t.Fatalf("unexpected error name: %s", errResp.ErrorName)
	}
}

func TestRequestBodyTooLargeRejected(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	MAX_REQUEST_BODY_BYTES = 16

	response := callEndpoint(http.MethodPost, "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d, body=%s", response.Code, response.Body.String())
	}

	errResp := decodeJSON[ErrorResponse](t, response.Body.Bytes())
	if errResp.ErrorName != "request body too large" {
		t.Fatalf("unexpected error name: %s", errResp.ErrorName)
	}
	if !strings.Contains(errResp.Detail, "MAX_REQUEST_BODY_BYTES") {
		t.Fatalf("unexpected detail: %s", errResp.Detail)
	}
}

func TestDangerousSchemeRejected(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	response := callEndpoint(http.MethodPost, "application/json", `{"urls":["file:///etc/passwd"]}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
	}
	errResp := decodeJSON[ErrorResponse](t, response.Body.Bytes())
	if errResp.ErrorName != "invalid request" {
		t.Fatalf("unexpected error name: %s", errResp.ErrorName)
	}
}

// v0.0.3 regression test:
// Ensure references are appended once and fit_markdown has priority.
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

	response := callEndpoint(http.MethodPost, "application/json", `{"urls":["https://example.com"]}`)
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

	response := callEndpoint(http.MethodPost, "application/json", `{"urls":["https://example.com"]}`)
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

	response := callEndpoint(http.MethodPost, "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", response.Code)
	}

	errResp := decodeJSON[ErrorResponse](t, response.Body.Bytes())
	if errResp.ErrorName != "bad gateway" {
		t.Fatalf("unexpected error name: %s", errResp.ErrorName)
	}
}

// v0.0.3 regression test:
// Verify optional auth forwarding is applied on upstream requests.
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

	response := callEndpoint(http.MethodPost, "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}
	if seenAuth != "Bearer abc123" {
		t.Fatalf("expected auth header Bearer abc123, got: %q", seenAuth)
	}
}

// v0.0.3 regression test:
// Ensure newer Crawl4AI settings are forwarded in request payload.
func TestPayloadContainsNewFeatureFlags(t *testing.T) {
	state := captureProxyState()
	t.Cleanup(func() { restoreProxyState(state) })

	AVOID_CSS = true
	REMOVE_OVERLAY = true
	MAX_RETRIES = 2
	HAS_PROXY_CONFIG = true
	PROXY_CONFIG_PARSED = []any{"direct", "http://proxy.local:8080"}

	setupDownstream(t, func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}

		browserConfig := payload["browser_config"].(map[string]any)
		browserParams := browserConfig["params"].(map[string]any)
		if browserParams["avoid_css"] != true {
			t.Fatalf("expected avoid_css=true in browser_config params")
		}

		crawlerConfig := payload["crawler_config"].(map[string]any)
		crawlerParams := crawlerConfig["params"].(map[string]any)
		if crawlerParams["remove_overlay_elements"] != true {
			t.Fatalf("expected remove_overlay_elements=true in crawler_config params")
		}
		if crawlerParams["max_retries"] != float64(2) {
			t.Fatalf("expected max_retries=2 in crawler_config params, got %v", crawlerParams["max_retries"])
		}
		if _, exists := crawlerParams["proxy_config"]; !exists {
			t.Fatalf("expected proxy_config to be forwarded in crawler_config params")
		}

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

	response := callEndpoint(http.MethodPost, "application/json", `{"urls":["https://example.com"]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", response.Code, response.Body.String())
	}
}
