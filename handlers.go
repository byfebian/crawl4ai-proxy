package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// =============================================================================
// Shared helpers
// =============================================================================

func isMaxBodySizeError(err error) bool {
	if err == nil {
		return false
	}
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

func errorResponseFromError(name string, err error) ErrorResponse {
	return ErrorResponse{
		ErrorName: name,
		Detail:    err.Error(),
	}
}

func writeJSON(response http.ResponseWriter, statusCode int, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal response payload", "error", err)
		response.WriteHeader(http.StatusInternalServerError)
		_, _ = response.Write([]byte(`{"error":"internal server error"}`))
		return
	}
	response.WriteHeader(statusCode)
	_, _ = response.Write(encoded)
}

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

		if strings.Contains(u, "://") || strings.HasPrefix(lower, "raw:") {
			parsed, err := url.Parse(u)
			if err != nil {
				return nil, fmt.Errorf("urls[%d] is invalid: %w", idx, err)
			}
			scheme := strings.ToLower(parsed.Scheme)
			switch scheme {
			case "http", "https", "raw":
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

func validateURL(rawURL string) error {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return fmt.Errorf("url must not be empty")
	}
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "file:") || strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return fmt.Errorf("url uses unsupported scheme")
	}
	if strings.Contains(u, "://") {
		parsed, err := url.Parse(u)
		if err != nil {
			return fmt.Errorf("invalid url: %w", err)
		}
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != "http" && scheme != "https" {
			return fmt.Errorf("unsupported scheme %q", scheme)
		}
		if parsed.Host == "" {
			return fmt.Errorf("url is missing host")
		}
	}
	return nil
}

// =============================================================================
// Health endpoint
// =============================================================================

func HealthEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	if request.Method != http.MethodGet {
		writeJSON(response, http.StatusMethodNotAllowed, ErrorResponse{ErrorName: "method not allowed"})
		return
	}

	deep := request.URL.Query().Get("deep") == "true"
	if !deep {
		writeJSON(response, http.StatusOK, map[string]string{"status": "healthy"})
		return
	}

	baseURL := strings.TrimSuffix(CRAWL4AI_ENDPOINT, "/crawl")
	healthURL := baseURL + "/health"
	req, err := http.NewRequest(http.MethodGet, healthURL, nil)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{
			"status":   "degraded",
			"upstream": "unreachable",
			"detail":   err.Error(),
		})
		return
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{
			"status":   "degraded",
			"upstream": "unreachable",
			"detail":   err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{
			"status":   "degraded",
			"upstream": fmt.Sprintf("returned status %d", resp.StatusCode),
		})
		return
	}

	writeJSON(response, http.StatusOK, map[string]string{"status": "healthy", "upstream": "reachable"})
}

// =============================================================================
// Metrics endpoint
// =============================================================================

func MetricsEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	if request.Method != http.MethodGet {
		writeJSON(response, http.StatusMethodNotAllowed, ErrorResponse{ErrorName: "method not allowed"})
		return
	}

	reset := request.URL.Query().Get("reset") == "true"
	snapshot := globalMetrics.Snapshot(reset)
	writeJSON(response, http.StatusOK, snapshot)
}

// =============================================================================
// /crawl endpoint
// =============================================================================

func CrawlEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	if request.Method != http.MethodPost {
		writeJSON(response, http.StatusMethodNotAllowed, ErrorResponse{ErrorName: "method not allowed"})
		slog.Warn("405 method not allowed", "method", request.Method, "path", request.URL.Path, "remote_addr", ClientIP(request))
		return
	}

	contentType := request.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "content type must be application/json"})
		slog.Warn("400 invalid content type", "content_type", contentType, "remote_addr", ClientIP(request))
		return
	}

	limitedBody := http.MaxBytesReader(response, request.Body, MAX_REQUEST_BODY_BYTES)
	defer limitedBody.Close()

	var requestData Request
	decoder := json.NewDecoder(limitedBody)
	if err := decoder.Decode(&requestData); err != nil {
		if isMaxBodySizeError(err) {
			writeJSON(response, http.StatusRequestEntityTooLarge, ErrorResponse{
				ErrorName: "request body too large",
				Detail:    fmt.Sprintf("request body exceeds MAX_REQUEST_BODY_BYTES (%d)", MAX_REQUEST_BODY_BYTES),
			})
			slog.Warn("413 request body too large", "remote_addr", ClientIP(request))
			return
		}
		writeJSON(response, http.StatusBadRequest, errorResponseFromError("invalid json", err))
		slog.Warn("400 invalid json", "remote_addr", ClientIP(request), "error", err)
		return
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if isMaxBodySizeError(err) {
			writeJSON(response, http.StatusRequestEntityTooLarge, ErrorResponse{
				ErrorName: "request body too large",
				Detail:    fmt.Sprintf("request body exceeds MAX_REQUEST_BODY_BYTES (%d)", MAX_REQUEST_BODY_BYTES),
			})
			slog.Warn("413 request body too large (trailing)", "remote_addr", ClientIP(request))
			return
		}
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "invalid json", Detail: "unexpected trailing content"})
		slog.Warn("400 trailing json content", "remote_addr", ClientIP(request))
		return
	}

	validatedURLs, err := validateAndNormalizeURLs(requestData.Urls)
	if err != nil {
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "invalid request", Detail: err.Error()})
		slog.Warn("400 invalid request", "error", err, "remote_addr", ClientIP(request))
		return
	}

	slog.Info("crawl request", "urls", validatedURLs, "remote_addr", ClientIP(request), "request_id", RequestIDFromContext(request.Context()))

	crawlPayload := buildCrawlPayload(validatedURLs, &requestData)
	payloadBytes, err := json.Marshal(crawlPayload)
	if err != nil {
		writeJSON(response, http.StatusInternalServerError, ErrorResponse{ErrorName: "internal server error", Detail: "failed to encode crawl payload"})
		slog.Error("500 failed to encode crawl payload", "error", err)
		return
	}

	crawlResponse, err := sendWithRetry(httpClient, http.MethodPost, CRAWL4AI_ENDPOINT, payloadBytes, RequestIDFromContext(request.Context()))
	if err != nil {
		if crawlResponse != nil {
			crawlResponse.Body.Close()
		}
		writeJSON(response, http.StatusBadGateway, ErrorResponse{
			ErrorName: "bad gateway",
			Detail:    fmt.Sprintf("crawl4ai request failed: %v", err),
		})
		slog.Error("502 crawl4ai request error", "error", err, "remote_addr", ClientIP(request))
		return
	}
	defer crawlResponse.Body.Close()

	if crawlResponse.StatusCode != http.StatusOK {
		bodySnippet := readDownstreamError(crawlResponse)
		detail := fmt.Sprintf("crawl4ai returned status %d", crawlResponse.StatusCode)
		if bodySnippet != "" {
			detail = detail + ": " + bodySnippet
		}
		writeJSON(response, http.StatusBadGateway, ErrorResponse{
			ErrorName: "bad gateway",
			Detail:    detail,
		})
		slog.Warn("502 upstream status", "status", crawlResponse.StatusCode, "remote_addr", ClientIP(request))
		return
	}

	var crawlData CrawlResponse
	if err := json.NewDecoder(crawlResponse.Body).Decode(&crawlData); err != nil {
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: "invalid json received from crawl api"})
		slog.Error("502 invalid json from crawl api", "error", err, "remote_addr", ClientIP(request))
		return
	}

	results := crawlData.GetAllResults()
	if len(results) == 0 {
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: "crawl4ai returned no results"})
		slog.Warn("502 no results from crawl api", "remote_addr", ClientIP(request))
		return
	}

	contentSource := EffectiveContentSource(&requestData)
	ret := SuccessResponse{}
	for _, result := range results {
		content := SelectMarkdownContent(result, contentSource)

		if content == "" && result.ErrorMessage != "" {
			content = "Crawl failed: " + result.ErrorMessage
		}

		if INCLUDE_REFERENCES && strings.TrimSpace(result.Markdown.ReferencesMarkdown) != "" {
			content = appendReferencesSection(content, result.Markdown.ReferencesMarkdown)
		}

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
	slog.Info("200 crawl success", "results", len(ret), "remote_addr", ClientIP(request))
}

// =============================================================================
// /md endpoint
// =============================================================================

func MdEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	if request.Method != http.MethodPost {
		writeJSON(response, http.StatusMethodNotAllowed, ErrorResponse{ErrorName: "method not allowed"})
		return
	}

	contentType := request.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "content type must be application/json"})
		return
	}

	limitedBody := http.MaxBytesReader(response, request.Body, MAX_REQUEST_BODY_BYTES)
	defer limitedBody.Close()

	var mdReq MdRequest
	if err := json.NewDecoder(limitedBody).Decode(&mdReq); err != nil {
		if isMaxBodySizeError(err) {
			writeJSON(response, http.StatusRequestEntityTooLarge, ErrorResponse{
				ErrorName: "request body too large",
				Detail:    fmt.Sprintf("request body exceeds MAX_REQUEST_BODY_BYTES (%d)", MAX_REQUEST_BODY_BYTES),
			})
			return
		}
		writeJSON(response, http.StatusBadRequest, errorResponseFromError("invalid json", err))
		return
	}

	if err := validateURL(mdReq.Url); err != nil {
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "invalid url", Detail: err.Error()})
		return
	}

	filterType := "fit"
	if mdReq.FilterType != nil && *mdReq.FilterType != "" {
		filterType = *mdReq.FilterType
	}
	bm25Query := ""
	if mdReq.Bm25Query != nil {
		bm25Query = *mdReq.Bm25Query
	}

	mdURL := strings.TrimSuffix(CRAWL4AI_ENDPOINT, "/crawl") + "/md"
	payload := buildMdPayload(mdReq.Url, filterType, bm25Query)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		writeJSON(response, http.StatusInternalServerError, ErrorResponse{ErrorName: "internal server error", Detail: "failed to encode payload"})
		return
	}

	resp, err := sendWithRetry(httpClient, http.MethodPost, mdURL, payloadBytes, RequestIDFromContext(request.Context()))
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: fmt.Sprintf("crawl4ai request failed: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodySnippet := readDownstreamError(resp)
		detail := fmt.Sprintf("crawl4ai returned status %d", resp.StatusCode)
		if bodySnippet != "" {
			detail += ": " + bodySnippet
		}
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: detail})
		return
	}

	var mdResp MdResponse
	if err := json.NewDecoder(resp.Body).Decode(&mdResp); err != nil {
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: "invalid json from crawl api"})
		return
	}

	content := strings.TrimSpace(mdResp.Markdown)
	if content == "" && mdResp.Error != "" {
		content = "Extraction failed: " + mdResp.Error
	}

	metadata := map[string]string{"source": mdResp.Url}

	result := []SuccessResponseItem{{
		PageContent: content,
		Metadata:    metadata,
	}}
	writeJSON(response, http.StatusOK, result)
}

// =============================================================================
// /screenshot endpoint
// =============================================================================

func ScreenshotEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	if request.Method != http.MethodPost {
		writeJSON(response, http.StatusMethodNotAllowed, ErrorResponse{ErrorName: "method not allowed"})
		return
	}

	contentType := request.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "content type must be application/json"})
		return
	}

	limitedBody := http.MaxBytesReader(response, request.Body, MAX_REQUEST_BODY_BYTES)
	defer limitedBody.Close()

	var scrReq ScreenshotRequest
	if err := json.NewDecoder(limitedBody).Decode(&scrReq); err != nil {
		if isMaxBodySizeError(err) {
			writeJSON(response, http.StatusRequestEntityTooLarge, ErrorResponse{
				ErrorName: "request body too large",
				Detail:    fmt.Sprintf("request body exceeds MAX_REQUEST_BODY_BYTES (%d)", MAX_REQUEST_BODY_BYTES),
			})
			return
		}
		writeJSON(response, http.StatusBadRequest, errorResponseFromError("invalid json", err))
		return
	}

	if err := validateURL(scrReq.Url); err != nil {
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "invalid url", Detail: err.Error()})
		return
	}

	screenshotURL := strings.TrimSuffix(CRAWL4AI_ENDPOINT, "/crawl") + "/screenshot"
	payload := map[string]interface{}{
		"url": scrReq.Url,
	}
	if scrReq.ScreenshotWait != nil {
		payload["screenshot_wait_for"] = *scrReq.ScreenshotWait
	}
	if scrReq.WaitForImages != nil {
		payload["wait_for_images"] = *scrReq.WaitForImages
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		writeJSON(response, http.StatusInternalServerError, ErrorResponse{ErrorName: "internal server error"})
		return
	}

	resp, err := sendWithRetry(httpClient, http.MethodPost, screenshotURL, payloadBytes, RequestIDFromContext(request.Context()))
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: fmt.Sprintf("crawl4ai request failed: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodySnippet := readDownstreamError(resp)
		detail := fmt.Sprintf("crawl4ai returned status %d", resp.StatusCode)
		if bodySnippet != "" {
			detail += ": " + bodySnippet
		}
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: detail})
		return
	}

	var scrResp ScreenshotApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&scrResp); err != nil {
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: "invalid json from crawl api"})
		return
	}

	if !scrResp.Success {
		writeJSON(response, http.StatusBadGateway, ErrorResponse{
			ErrorName: "bad gateway",
			Detail:    scrResp.Error,
		})
		return
	}

	writeJSON(response, http.StatusOK, map[string]interface{}{
		"screenshot": scrResp.Screenshot,
		"metadata":   map[string]string{"source": scrReq.Url},
	})
}

// =============================================================================
// /execute_js endpoint
// =============================================================================

func ExecuteJsEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")

	if request.Method != http.MethodPost {
		writeJSON(response, http.StatusMethodNotAllowed, ErrorResponse{ErrorName: "method not allowed"})
		return
	}

	contentType := request.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "content type must be application/json"})
		return
	}

	limitedBody := http.MaxBytesReader(response, request.Body, MAX_REQUEST_BODY_BYTES)
	defer limitedBody.Close()

	var jsReq ExecuteJsRequest
	if err := json.NewDecoder(limitedBody).Decode(&jsReq); err != nil {
		if isMaxBodySizeError(err) {
			writeJSON(response, http.StatusRequestEntityTooLarge, ErrorResponse{
				ErrorName: "request body too large",
			})
			return
		}
		writeJSON(response, http.StatusBadRequest, errorResponseFromError("invalid json", err))
		return
	}

	if err := validateURL(jsReq.Url); err != nil {
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "invalid url", Detail: err.Error()})
		return
	}
	if len(jsReq.Scripts) == 0 {
		writeJSON(response, http.StatusBadRequest, ErrorResponse{ErrorName: "invalid request", Detail: "scripts must contain at least one entry"})
		return
	}

	jsURL := strings.TrimSuffix(CRAWL4AI_ENDPOINT, "/crawl") + "/execute_js"
	payload := map[string]interface{}{
		"url":     jsReq.Url,
		"scripts": jsReq.Scripts,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		writeJSON(response, http.StatusInternalServerError, ErrorResponse{ErrorName: "internal server error"})
		return
	}

	resp, err := sendWithRetry(httpClient, http.MethodPost, jsURL, payloadBytes, RequestIDFromContext(request.Context()))
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: fmt.Sprintf("crawl4ai request failed: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodySnippet := readDownstreamError(resp)
		detail := fmt.Sprintf("crawl4ai returned status %d", resp.StatusCode)
		if bodySnippet != "" {
			detail += ": " + bodySnippet
		}
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: detail})
		return
	}

	var jsResp ExecuteJsApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&jsResp); err != nil {
		writeJSON(response, http.StatusBadGateway, ErrorResponse{ErrorName: "bad gateway", Detail: "invalid json from crawl api"})
		return
	}

	if !jsResp.Success {
		writeJSON(response, http.StatusBadGateway, ErrorResponse{
			ErrorName: "bad gateway",
			Detail:    jsResp.Error,
		})
		return
	}

	resultJSON, _ := json.Marshal(jsResp.Result)
	writeJSON(response, http.StatusOK, map[string]interface{}{
		"result":   string(resultJSON),
		"metadata": map[string]string{"source": jsReq.Url},
	})
}
