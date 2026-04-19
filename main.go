package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// CONFIGURATION — All tunable via environment variables
// =============================================================================
var (
	LISTEN_IP   string = ""
	LISTEN_PORT int    = 8000

	// Crawl4AI backend endpoint (e.g. http://crawl4ai:11235/crawl)
	CRAWL4AI_ENDPOINT string = "http://crawl4ai:11235/crawl"

	// v0.0.3 change:
	// These auth settings let the proxy work with secured Crawl4AI deployments
	// where JWT/Bearer auth is enabled.
	CRAWL4AI_AUTH_TOKEN  string = ""
	CRAWL4AI_AUTH_SCHEME string = "Bearer"
	CRAWL4AI_AUTH_HEADER string = "Authorization"

	// How long to wait for Crawl4AI before giving up (seconds)
	CRAWL_TIMEOUT_SECONDS int = 120

	// v0.0.3 change:
	// This keeps request size bounded so malformed/huge payloads do not consume
	// unbounded memory inside this tiny proxy.
	MAX_REQUEST_BODY_BYTES int64 = 1 << 20 // 1 MiB

	// v0.0.3 change:
	// Crawl4AI Docker API limits urls list length; we mirror that to fail fast.
	MAX_URLS_PER_REQUEST int = 100

	// --- Feature flags ---
	REMOVE_CONSENT_POPUPS bool = true
	FLATTEN_SHADOW_DOM    bool = true
	REMOVE_OVERLAY        bool = true

	AVOID_ADS bool = true

	// v0.0.3 change:
	// Expose Crawl4AI's avoid_css browser feature for users who want faster,
	// cleaner extraction with minimal style resources loaded.
	AVOID_CSS bool = false

	MEMORY_SAVING_MODE       bool = true
	MAX_PAGES_BEFORE_RECYCLE int  = 100

	PRUNING_THRESHOLD  float64 = 0.48
	INCLUDE_REFERENCES bool    = true

	// v0.0.3 change:
	// Expose Crawl4AI anti-bot retry control introduced in newer 0.8.x.
	MAX_RETRIES int = 0

	// v0.0.3 change:
	// Allow advanced proxy setup pass-through from env without hardcoding a
	// specific proxy model. This can be string/object/list as Crawl4AI accepts.
	PROXY_CONFIG_JSON   string = ""
	PROXY_CONFIG_PARSED any
	HAS_PROXY_CONFIG    bool = false
)

const maxDownstreamErrorBodyBytes int64 = 4096

func isMaxBodySizeError(err error) bool {
	if err == nil {
		return false
	}
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

// =============================================================================
// ReadEnvironment — Reads env vars and overrides defaults
// =============================================================================
func ReadEnvironment() {
	if v := os.Getenv("LISTEN_IP"); strings.TrimSpace(v) != "" {
		LISTEN_IP = strings.TrimSpace(v)
	}
	if v := os.Getenv("CRAWL4AI_ENDPOINT"); strings.TrimSpace(v) != "" {
		CRAWL4AI_ENDPOINT = strings.TrimSpace(v)
	}
	if v := os.Getenv("CRAWL4AI_AUTH_TOKEN"); strings.TrimSpace(v) != "" {
		CRAWL4AI_AUTH_TOKEN = strings.TrimSpace(v)
	}
	if v := os.Getenv("CRAWL4AI_AUTH_SCHEME"); strings.TrimSpace(v) != "" {
		CRAWL4AI_AUTH_SCHEME = strings.TrimSpace(v)
	}
	if v := os.Getenv("CRAWL4AI_AUTH_HEADER"); strings.TrimSpace(v) != "" {
		CRAWL4AI_AUTH_HEADER = strings.TrimSpace(v)
	}

	LISTEN_PORT = envInt("LISTEN_PORT", LISTEN_PORT)
	CRAWL_TIMEOUT_SECONDS = envInt("CRAWL_TIMEOUT_SECONDS", CRAWL_TIMEOUT_SECONDS)
	MAX_URLS_PER_REQUEST = envInt("MAX_URLS_PER_REQUEST", MAX_URLS_PER_REQUEST)
	MAX_PAGES_BEFORE_RECYCLE = envInt("MAX_PAGES_BEFORE_RECYCLE", MAX_PAGES_BEFORE_RECYCLE)
	MAX_RETRIES = envInt("MAX_RETRIES", MAX_RETRIES)
	MAX_REQUEST_BODY_BYTES = int64(envInt("MAX_REQUEST_BODY_BYTES", int(MAX_REQUEST_BODY_BYTES)))

	PRUNING_THRESHOLD = envFloat64("PRUNING_THRESHOLD", PRUNING_THRESHOLD)

	REMOVE_CONSENT_POPUPS = envBool("REMOVE_CONSENT_POPUPS", REMOVE_CONSENT_POPUPS)
	FLATTEN_SHADOW_DOM = envBool("FLATTEN_SHADOW_DOM", FLATTEN_SHADOW_DOM)
	REMOVE_OVERLAY = envBool("REMOVE_OVERLAY_ELEMENTS", REMOVE_OVERLAY)
	AVOID_ADS = envBool("AVOID_ADS", AVOID_ADS)
	AVOID_CSS = envBool("AVOID_CSS", AVOID_CSS)
	MEMORY_SAVING_MODE = envBool("MEMORY_SAVING_MODE", MEMORY_SAVING_MODE)
	INCLUDE_REFERENCES = envBool("INCLUDE_REFERENCES", INCLUDE_REFERENCES)

	// v0.0.3 change:
	// Parse proxy config once at startup. If invalid, we log and continue with
	// no proxy override so the service remains available.
	if v := os.Getenv("PROXY_CONFIG_JSON"); strings.TrimSpace(v) != "" {
		PROXY_CONFIG_JSON = strings.TrimSpace(v)
		var decoded any
		if err := json.Unmarshal([]byte(PROXY_CONFIG_JSON), &decoded); err != nil {
			log.Printf("Invalid PROXY_CONFIG_JSON, ignoring proxy override: %v", err)
			HAS_PROXY_CONFIG = false
			PROXY_CONFIG_PARSED = nil
		} else {
			HAS_PROXY_CONFIG = true
			PROXY_CONFIG_PARSED = decoded
		}
	}
}

func envBool(name string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		log.Printf("Invalid boolean for %s=%q. Using fallback: %v", name, v, fallback)
		return fallback
	}
	return parsed
}

func envInt(name string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("Invalid integer for %s=%q. Using fallback: %d", name, v, fallback)
		return fallback
	}
	return parsed
}

func envFloat64(name string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		log.Printf("Invalid float for %s=%q. Using fallback: %.4f", name, v, fallback)
		return fallback
	}
	return parsed
}

// =============================================================================
// OpenWebUI-facing types (input and output)
// =============================================================================

// Request — What OpenWebUI sends to this proxy.
type Request struct {
	Urls []string `json:"urls"`
}

// SuccessResponseItem — What OpenWebUI expects back.
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
}

type CrawlResponse struct {
	Success bool              `json:"success"`
	Results []CrawlResultItem `json:"results"`
	Result  *CrawlResultItem  `json:"result,omitempty"`
}

// GetAllResults handles both single-result and multi-result responses.
func (cr *CrawlResponse) GetAllResults() []CrawlResultItem {
	if cr.Result != nil && len(cr.Results) == 0 {
		return []CrawlResultItem{*cr.Result}
	}
	return cr.Results
}

// =============================================================================
// Helper functions
// =============================================================================

func errorResponseFromError(name string, err error) ErrorResponse {
	return ErrorResponse{
		ErrorName: name,
		Detail:    err.Error(),
	}
}

func writeJSON(response http.ResponseWriter, statusCode int, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal response payload: %v", err)
		response.WriteHeader(http.StatusInternalServerError)
		_, _ = response.Write([]byte(`{"error":"internal server error"}`))
		return
	}
	response.WriteHeader(statusCode)
	_, _ = response.Write(encoded)
}

// validateAndNormalizeURLs performs strict input validation before forwarding
// traffic to Crawl4AI.
//
// v0.0.3 change:
// - Reject empty URL lists and oversized batches.
// - Reject dangerous schemes (file:, javascript:, data:) for defense-in-depth.
// - Allow bare hostnames (example.com) so behavior stays OpenWebUI-friendly.
func validateAndNormalizeURLs(urls []string) ([]string, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("urls must contain at least one entry")
	}
	if MAX_URLS_PER_REQUEST > 0 && len(urls) > MAX_URLS_PER_REQUEST {
		return nil, fmt.Errorf("urls length exceeds MAX_URLS_PER_REQUEST (%d)", MAX_URLS_PER_REQUEST)
	}

	normalized := make([]string, 0, len(urls))
	for idx, rawURL := range urls {
		u := strings.TrimSpace(rawURL)
		if u == "" {
			return nil, fmt.Errorf("urls[%d] must not be empty", idx)
		}

		lower := strings.ToLower(u)
		if strings.HasPrefix(lower, "file:") || strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
			return nil, fmt.Errorf("urls[%d] uses unsupported url scheme", idx)
		}

		// If URL has an explicit scheme, validate it.
		if strings.Contains(u, "://") || strings.HasPrefix(lower, "raw:") {
			parsed, err := url.Parse(u)
			if err != nil {
				return nil, fmt.Errorf("urls[%d] is invalid: %w", idx, err)
			}

			scheme := strings.ToLower(parsed.Scheme)
			switch scheme {
			case "http", "https", "raw":
				// Accepted schemes.
			default:
				return nil, fmt.Errorf("urls[%d] uses unsupported scheme %q", idx, scheme)
			}

			if (scheme == "http" || scheme == "https") && parsed.Host == "" {
				return nil, fmt.Errorf("urls[%d] is missing host", idx)
			}
		}

		normalized = append(normalized, u)
	}

	return normalized, nil
}

// appendReferencesSection avoids duplicate "## References" headers.
//
// v0.0.3 change:
// Crawl4AI already includes a references heading in many cases, so we detect
// that and avoid producing nested or duplicated headings.
func appendReferencesSection(content string, references string) string {
	references = strings.TrimSpace(references)
	if references == "" {
		return content
	}

	lowered := strings.ToLower(strings.TrimSpace(references))
	if strings.HasPrefix(lowered, "## references") || strings.HasPrefix(lowered, "# references") {
		return content + "\n\n---\n" + references
	}
	return content + "\n\n---\n## References\n" + references
}

// buildCrawlPayload creates a Crawl4AI 0.8.x request payload with structured
// type/params objects.
//
// v0.0.3 change:
// Added pass-through options for newer Crawl4AI features and anti-bot retries.
func buildCrawlPayload(urls []string) map[string]interface{} {
	browserParams := map[string]interface{}{
		"headless":                 true,
		"avoid_ads":                AVOID_ADS,
		"avoid_css":                AVOID_CSS,
		"memory_saving_mode":       MEMORY_SAVING_MODE,
		"max_pages_before_recycle": MAX_PAGES_BEFORE_RECYCLE,
	}

	crawlerParams := map[string]interface{}{
		"remove_consent_popups":   REMOVE_CONSENT_POPUPS,
		"flatten_shadow_dom":      FLATTEN_SHADOW_DOM,
		"remove_overlay_elements": REMOVE_OVERLAY,
		"markdown_generator": map[string]interface{}{
			"type": "DefaultMarkdownGenerator",
			"params": map[string]interface{}{
				"content_filter": map[string]interface{}{
					"type": "PruningContentFilter",
					"params": map[string]interface{}{
						"threshold":          PRUNING_THRESHOLD,
						"threshold_type":     "dynamic",
						"min_word_threshold": 10,
					},
				},
				"options": map[string]interface{}{
					"ignore_links":    false,
					"body_width":      0,
					"include_sup_sub": true,
				},
			},
		},
	}

	if MAX_RETRIES > 0 {
		crawlerParams["max_retries"] = MAX_RETRIES
	}
	if HAS_PROXY_CONFIG {
		crawlerParams["proxy_config"] = PROXY_CONFIG_PARSED
	}

	return map[string]interface{}{
		"urls": urls,
		"browser_config": map[string]interface{}{
			"type":   "BrowserConfig",
			"params": browserParams,
		},
		"crawler_config": map[string]interface{}{
			"type":   "CrawlerRunConfig",
			"params": crawlerParams,
		},
	}
}

// =============================================================================
// HTTP client with timeout (prevents hung requests from blocking forever)
// =============================================================================
var httpClient *http.Client

func initHTTPClient() {
	httpClient = &http.Client{
		Timeout: time.Duration(CRAWL_TIMEOUT_SECONDS) * time.Second,
	}
}

// =============================================================================
// Health check endpoint — for monitoring
// =============================================================================
func HealthEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	writeJSON(response, http.StatusOK, map[string]string{"status": "healthy"})
}

// =============================================================================
// CrawlEndpoint — Main handler that translates OpenWebUI ↔ Crawl4AI
// =============================================================================
func CrawlEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	// -------------------------------------------------------------------------
	// Only accept POST requests.
	// -------------------------------------------------------------------------
	if request.Method != http.MethodPost {
		writeJSON(response, http.StatusMethodNotAllowed, ErrorResponse{ErrorName: "method not allowed"})
		log.Printf("405 method not allowed :: %s", request.RemoteAddr)
		return
	}

	// v0.0.3 change:
	// Use media-type parsing so requests like
	// "application/json; charset=utf-8" are accepted.
	contentType := request.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "content type must be application/json"})
		log.Printf("400 invalid content type (%q) :: %s", contentType, request.RemoteAddr)
		return
	}

	limitedBody := http.MaxBytesReader(response, request.Body, MAX_REQUEST_BODY_BYTES)
	defer limitedBody.Close()

	// -------------------------------------------------------------------------
	// Parse and validate incoming OpenWebUI request.
	// -------------------------------------------------------------------------
	var requestData Request
	decoder := json.NewDecoder(limitedBody)
	if err := decoder.Decode(&requestData); err != nil {
		if isMaxBodySizeError(err) {
			writeJSON(response, http.StatusRequestEntityTooLarge, ErrorResponse{
				ErrorName: "request body too large",
				Detail:    fmt.Sprintf("request body exceeds MAX_REQUEST_BODY_BYTES (%d)", MAX_REQUEST_BODY_BYTES),
			})
			log.Printf("413 request body too large :: %s", request.RemoteAddr)
			return
		}
		writeJSON(response, http.StatusBadRequest, errorResponseFromError("invalid json", err))
		log.Printf("400 invalid json :: %s", request.RemoteAddr)
		return
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if isMaxBodySizeError(err) {
			writeJSON(response, http.StatusRequestEntityTooLarge, ErrorResponse{
				ErrorName: "request body too large",
				Detail:    fmt.Sprintf("request body exceeds MAX_REQUEST_BODY_BYTES (%d)", MAX_REQUEST_BODY_BYTES),
			})
			log.Printf("413 request body too large :: %s", request.RemoteAddr)
			return
		}
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "invalid json", Detail: "unexpected trailing content"})
		log.Printf("400 trailing json content :: %s", request.RemoteAddr)
		return
	}

	validatedURLs, err := validateAndNormalizeURLs(requestData.Urls)
	if err != nil {
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "invalid request", Detail: err.Error()})
		log.Printf("400 invalid request (%v) :: %s", err, request.RemoteAddr)
		return
	}

	log.Printf("Request to crawl %v from %s", validatedURLs, request.RemoteAddr)

	// -------------------------------------------------------------------------
	// Build Crawl4AI request payload with enriched config.
	// -------------------------------------------------------------------------
	crawlPayload := buildCrawlPayload(validatedURLs)
	payloadBytes, err := json.Marshal(crawlPayload)
	if err != nil {
		writeJSON(response, http.StatusInternalServerError, ErrorResponse{ErrorName: "internal server error", Detail: "failed to encode crawl payload"})
		log.Printf("500 failed to encode crawl payload :: %v :: %s", err, request.RemoteAddr)
		return
	}

	// -------------------------------------------------------------------------
	// Send the enriched request to Crawl4AI.
	// -------------------------------------------------------------------------
	req, err := http.NewRequest(http.MethodPost, CRAWL4AI_ENDPOINT, bytes.NewReader(payloadBytes))
	if err != nil {
		// v0.0.3 change:
		// Never panic on request construction. Return structured 500 instead.
		writeJSON(response, http.StatusInternalServerError, ErrorResponse{ErrorName: "internal server error", Detail: "failed to build upstream request"})
		log.Printf("500 failed to build upstream request :: %v :: %s", err, request.RemoteAddr)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// v0.0.3 change:
	// Optional auth forwarding for secured Crawl4AI installations.
	if CRAWL4AI_AUTH_TOKEN != "" {
		authValue := CRAWL4AI_AUTH_TOKEN
		if strings.TrimSpace(CRAWL4AI_AUTH_SCHEME) != "" {
			authValue = strings.TrimSpace(CRAWL4AI_AUTH_SCHEME) + " " + CRAWL4AI_AUTH_TOKEN
		}
		req.Header.Set(CRAWL4AI_AUTH_HEADER, authValue)
	}

	crawlResponse, err := httpClient.Do(req)
	if err != nil {
		writeJSON(response, http.StatusBadGateway, ErrorResponse{
			ErrorName: "bad gateway",
			Detail:    fmt.Sprintf("crawl4ai request failed: %v", err),
		})
		log.Printf("502 crawl4ai request error :: %v :: %s", err, request.RemoteAddr)
		return
	}
	// v0.0.3 change:
	// Always close downstream body to avoid file descriptor and connection leaks.
	defer crawlResponse.Body.Close()

	if crawlResponse.StatusCode != http.StatusOK {
		rawBody, _ := io.ReadAll(io.LimitReader(crawlResponse.Body, maxDownstreamErrorBodyBytes))
		bodySnippet := strings.TrimSpace(string(rawBody))
		detail := fmt.Sprintf("crawl4ai returned status %d", crawlResponse.StatusCode)
		if bodySnippet != "" {
			detail = detail + ": " + bodySnippet
		}
		writeJSON(response, http.StatusBadGateway, ErrorResponse{
			ErrorName: "bad gateway",
			Detail:    detail,
		})
		log.Printf("502 upstream status %d :: %s", crawlResponse.StatusCode, request.RemoteAddr)
		return
	}

	// -------------------------------------------------------------------------
	// Parse the Crawl4AI response.
	// -------------------------------------------------------------------------
	var crawlData CrawlResponse
	if err := json.NewDecoder(crawlResponse.Body).Decode(&crawlData); err != nil {
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: "invalid json received from crawl api"})
		log.Printf("502 invalid json from crawl api :: %v :: %s", err, request.RemoteAddr)
		return
	}

	results := crawlData.GetAllResults()
	if len(results) == 0 {
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: "crawl4ai returned no results"})
		log.Printf("502 no results from crawl api :: %s", request.RemoteAddr)
		return
	}

	// -------------------------------------------------------------------------
	// Convert Crawl4AI results to OpenWebUI format.
	// -------------------------------------------------------------------------
	ret := SuccessResponse{}
	for _, result := range results {
		// Prefer fit_markdown (filtered/pruned content) over raw_markdown.
		content := strings.TrimSpace(result.Markdown.RawMarkdown)
		if result.Markdown.FitMarkdown != nil && strings.TrimSpace(*result.Markdown.FitMarkdown) != "" {
			content = *result.Markdown.FitMarkdown
		}

		// v0.0.3 change:
		// When crawl fails and markdown is empty, surface the crawl error in content
		// so OpenWebUI users can understand why extraction failed.
		if content == "" && result.ErrorMessage != "" {
			content = "Crawl failed: " + result.ErrorMessage
		}

		// Append references/citations for better source attribution in OpenWebUI.
		if INCLUDE_REFERENCES && strings.TrimSpace(result.Markdown.ReferencesMarkdown) != "" {
			content = appendReferencesSection(content, result.Markdown.ReferencesMarkdown)
		}

		// Build metadata map, converting all values to strings.
		metadata := map[string]string{}
		if result.Metadata != nil {
			for key, value := range result.Metadata {
				if strVal, ok := value.(string); ok && strVal != "" {
					metadata[key] = strVal
				} else if value != nil {
					metadata[key] = fmt.Sprintf("%v", value)
				}
			}
		}
		metadata["source"] = result.Url
		metadata["crawl_success"] = strconv.FormatBool(result.Success)
		if result.ErrorMessage != "" {
			metadata["crawl_error"] = result.ErrorMessage
		}

		ret = append(ret, SuccessResponseItem{
			PageContent: content,
			Metadata:    metadata,
		})
	}

	writeJSON(response, http.StatusOK, ret)
	log.Printf("200 :: %d results :: %s", len(ret), request.RemoteAddr)
}

// =============================================================================
// Main entry point
// =============================================================================
func main() {
	ReadEnvironment()
	initHTTPClient()

	http.HandleFunc("/crawl", CrawlEndpoint)
	http.HandleFunc("/health", HealthEndpoint)

	listenAddress := fmt.Sprintf("%s:%d", LISTEN_IP, LISTEN_PORT)
	log.Printf(
		"Listening on %s (timeout=%ds, consent_popups=%v, shadow_dom=%v, overlay_removal=%v, ads=%v, css=%v, retries=%d, pruning=%.2f, downstream_auth=%v)",
		listenAddress,
		CRAWL_TIMEOUT_SECONDS,
		REMOVE_CONSENT_POPUPS,
		FLATTEN_SHADOW_DOM,
		REMOVE_OVERLAY,
		AVOID_ADS,
		AVOID_CSS,
		MAX_RETRIES,
		PRUNING_THRESHOLD,
		CRAWL4AI_AUTH_TOKEN != "",
	)

	err := http.ListenAndServe(listenAddress, nil)
	if err != nil {
		log.Println(err)
	}
}
