package main

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const maxDownstreamErrorBodyBytes int64 = 4096

func buildBrowserConfig(req *Request) map[string]interface{} {
	params := map[string]interface{}{
		"headless":                 true,
		"avoid_ads":                AVOID_ADS,
		"avoid_css":                AVOID_CSS,
		"memory_saving_mode":       MEMORY_SAVING_MODE,
		"max_pages_before_recycle": MAX_PAGES_BEFORE_RECYCLE,
		"ignore_https_errors":      !FEATURE_FLAGS.VerifySSL,
	}

	if FEATURE_FLAGS.TextMode {
		params["text_mode"] = true
	}
	if FEATURE_FLAGS.LightMode {
		params["light_mode"] = true
	}

	ua := EffectiveUserAgent(req)
	if ua != "" {
		params["user_agent"] = ua
	}

	if EffectiveStealth(req) {
		params["enable_stealth"] = true
	}

	return map[string]interface{}{
		"type":   "BrowserConfig",
		"params": params,
	}
}

func buildCrawlerConfig(req *Request) map[string]interface{} {
	filterType, bm25Query := EffectiveContentFilter(req)
	crawlerParams := map[string]interface{}{
		"remove_consent_popups":   REMOVE_CONSENT_POPUPS,
		"flatten_shadow_dom":      FLATTEN_SHADOW_DOM,
		"remove_overlay_elements": REMOVE_OVERLAY,
	}

	switch filterType {
	case "bm25":
		mdGen := map[string]interface{}{
			"type": "DefaultMarkdownGenerator",
			"params": map[string]interface{}{
				"content_filter": map[string]interface{}{
					"type": "BM25ContentFilter",
					"params": map[string]interface{}{
						"user_query":     bm25Query,
						"bm25_threshold": 1.0,
					},
				},
				"options": map[string]interface{}{
					"ignore_links":    false,
					"body_width":      0,
					"include_sup_sub": true,
				},
			},
		}
		crawlerParams["markdown_generator"] = mdGen
	case "none":
	default:
		crawlerParams["markdown_generator"] = map[string]interface{}{
			"type": "DefaultMarkdownGenerator",
			"params": map[string]interface{}{
				"content_filter": map[string]interface{}{
					"type": "PruningContentFilter",
					"params": map[string]interface{}{
						"threshold":          PRUNING_THRESHOLD,
						"threshold_type":     "dynamic",
						"min_word_threshold": MIN_WORD_THRESHOLD,
					},
				},
				"options": map[string]interface{}{
					"ignore_links":    false,
					"body_width":      0,
					"include_sup_sub": true,
				},
			},
		}
	}

	if cssSel := EffectiveCssSelector(req); cssSel != "" {
		crawlerParams["css_selector"] = cssSel
	}
	if tags := EffectiveExcludedTags(req); len(tags) > 0 {
		crawlerParams["excluded_tags"] = tags
	}
	if targets := EffectiveTargetElements(req); len(targets) > 0 {
		crawlerParams["target_elements"] = targets
	}

	jsCode := EffectiveJsCode(req)
	if len(jsCode) > 0 {
		crawlerParams["js_code"] = jsCode
	}
	if wf := EffectiveWaitFor(req); wf != "" {
		crawlerParams["wait_for"] = wf
	}
	if EffectiveScanFullPage() {
		crawlerParams["scan_full_page"] = true
	}
	if EffectiveScrollDelay() > 0 {
		crawlerParams["scroll_delay"] = EffectiveScrollDelay()
	}

	if MAX_RETRIES > 0 {
		crawlerParams["max_retries"] = MAX_RETRIES
	}
	if HAS_PROXY_CONFIG {
		crawlerParams["proxy_config"] = PROXY_CONFIG_PARSED
	}

	if FEATURE_FLAGS.ProcessIframes {
		crawlerParams["process_iframes"] = true
	}
	if FEATURE_FLAGS.OnlyText {
		crawlerParams["only_text"] = true
	}
	if FEATURE_FLAGS.CheckRobotsTxt {
		crawlerParams["check_robots_txt"] = true
	}
	if FEATURE_FLAGS.CaptureNetworkReqs {
		crawlerParams["capture_network_requests"] = true
	}
	if FEATURE_FLAGS.CaptureConsoleMsgs {
		crawlerParams["capture_console_messages"] = true
	}
	if FEATURE_FLAGS.PreserveHTTPSLinks {
		crawlerParams["preserve_https_for_internal_links"] = true
	}

	if EffectiveDeepCrawl(req) {
		crawlerParams["deep_crawl_strategy"] = map[string]interface{}{
			"type": "BFSDeepCrawlStrategy",
			"params": map[string]interface{}{
				"max_depth": EffectiveDeepCrawlMaxDepth(req),
				"max_pages": EffectiveDeepCrawlMaxPages(req),
			},
		}
	}

	return map[string]interface{}{
		"type":   "CrawlerRunConfig",
		"params": crawlerParams,
	}
}

func buildCrawlPayload(urls []string, req *Request) map[string]interface{} {
	return map[string]interface{}{
		"urls":           urls,
		"browser_config": buildBrowserConfig(req),
		"crawler_config": buildCrawlerConfig(req),
	}
}

func buildMdPayload(url string, filterType string, bm25Query string) map[string]interface{} {
	crawlerParams := map[string]interface{}{
		"remove_consent_popups":   REMOVE_CONSENT_POPUPS,
		"flatten_shadow_dom":      FLATTEN_SHADOW_DOM,
		"remove_overlay_elements": REMOVE_OVERLAY,
	}

	switch filterType {
	case "fit":
		crawlerParams["markdown_generator"] = map[string]interface{}{
			"type": "DefaultMarkdownGenerator",
			"params": map[string]interface{}{
				"content_filter": map[string]interface{}{
					"type": "PruningContentFilter",
					"params": map[string]interface{}{
						"threshold":          PRUNING_THRESHOLD,
						"threshold_type":     "dynamic",
						"min_word_threshold": MIN_WORD_THRESHOLD,
					},
				},
			},
		}
	case "bm25":
		crawlerParams["markdown_generator"] = map[string]interface{}{
			"type": "DefaultMarkdownGenerator",
			"params": map[string]interface{}{
				"content_filter": map[string]interface{}{
					"type": "BM25ContentFilter",
					"params": map[string]interface{}{
						"user_query":     bm25Query,
						"bm25_threshold": 1.0,
					},
				},
			},
		}
	default:
	}

	return map[string]interface{}{
		"url": url,
		"browser_config": map[string]interface{}{
			"type": "BrowserConfig",
			"params": map[string]interface{}{
				"headless":                 true,
				"avoid_ads":                AVOID_ADS,
				"avoid_css":                AVOID_CSS,
				"memory_saving_mode":       MEMORY_SAVING_MODE,
				"max_pages_before_recycle": MAX_PAGES_BEFORE_RECYCLE,
				"ignore_https_errors":      !FEATURE_FLAGS.VerifySSL,
			},
		},
		"crawler_config": map[string]interface{}{
			"type":   "CrawlerRunConfig",
			"params": crawlerParams,
		},
	}
}

func sendUpstreamRequest(client *http.Client, method string, url string, payload []byte, requestID string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to build upstream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if CRAWL4AI_AUTH_TOKEN != "" {
		authValue := CRAWL4AI_AUTH_TOKEN
		if strings.TrimSpace(CRAWL4AI_AUTH_SCHEME) != "" {
			authValue = strings.TrimSpace(CRAWL4AI_AUTH_SCHEME) + " " + CRAWL4AI_AUTH_TOKEN
		}
		req.Header.Set(CRAWL4AI_AUTH_HEADER, authValue)
	}
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}

	return client.Do(req)
}

func sendWithRetry(client *http.Client, method string, url string, payload []byte, requestID string) (*http.Response, error) {
	var lastResp *http.Response
	var lastErr error

	maxAttempts := 1 + UPSTREAM_RETRIES
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(UPSTREAM_RETRY_DELAY_MS) * time.Millisecond * time.Duration(1<<(attempt-1))
			slog.Info("retrying upstream request", "request_id", requestID, "attempt", attempt+1, "delay_ms", delay.Milliseconds())
			time.Sleep(delay)
		}

		resp, err := sendUpstreamRequest(client, method, url, payload, requestID)
		if err != nil {
			lastErr = err
			slog.Warn("upstream request error", "request_id", requestID, "attempt", attempt+1, "error", err)
			continue
		}

		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxDownstreamErrorBodyBytes))
			resp.Body.Close()
			slog.Warn("upstream server error", "request_id", requestID, "attempt", attempt+1, "status", resp.StatusCode, "body", string(body))
			lastResp = resp
			lastErr = fmt.Errorf("upstream returned status %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	if lastResp != nil {
		return lastResp, lastErr
	}
	return nil, lastErr
}

func readDownstreamError(resp *http.Response) string {
	rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxDownstreamErrorBodyBytes))
	return strings.TrimSpace(string(rawBody))
}
