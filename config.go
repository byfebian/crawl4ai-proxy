package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// =============================================================================
// CONFIGURATION — All tunable via environment variables
// =============================================================================

var (
	LISTEN_IP   string = ""
	LISTEN_PORT int    = 8000

	CRAWL4AI_ENDPOINT string = "http://crawl4ai:11235/crawl"

	CRAWL4AI_AUTH_TOKEN  string = ""
	CRAWL4AI_AUTH_SCHEME string = "Bearer"
	CRAWL4AI_AUTH_HEADER string = "Authorization"

	CRAWL_CONNECT_TIMEOUT_SECONDS int = 10
	CRAWL_TOTAL_TIMEOUT_SECONDS   int = 120

	MAX_REQUEST_BODY_BYTES int64 = 1 << 20

	MAX_URLS_PER_REQUEST int = 100

	REMOVE_CONSENT_POPUPS bool = true
	FLATTEN_SHADOW_DOM    bool = true
	REMOVE_OVERLAY        bool = true

	AVOID_ADS bool = true
	AVOID_CSS bool = false

	MEMORY_SAVING_MODE       bool = true
	MAX_PAGES_BEFORE_RECYCLE int  = 100

	PRUNING_THRESHOLD  float64 = 0.48
	INCLUDE_REFERENCES bool    = true

	MAX_RETRIES int = 0

	PROXY_CONFIG_JSON   string        = ""
	PROXY_CONFIG_PARSED []ProxyConfig = nil
	HAS_PROXY_CONFIG    bool          = false

	CONTENT_FILTER_TYPE string = "pruning"
	BM25_USER_QUERY     string = ""
	MIN_WORD_THRESHOLD  int    = 10

	CSS_SELECTOR    string = ""
	EXCLUDED_TAGS   string = ""
	TARGET_ELEMENTS string = ""

	JS_CODE        string  = ""
	WAIT_FOR       string  = ""
	SCAN_FULL_PAGE bool    = false
	SCROLL_DELAY   float64 = 0.0

	CONTENT_SOURCE string = "fit_html"

	ENABLE_STEALTH bool   = false
	USER_AGENT     string = ""

	DEEP_CRAWL           bool = false
	DEEP_CRAWL_MAX_DEPTH int  = 1
	DEEP_CRAWL_MAX_PAGES int  = 10

	VERIFY_SSL bool = true

	UPSTREAM_RETRIES        int = 0
	UPSTREAM_RETRY_DELAY_MS int = 500

	CACHE_ENABLED     bool = true
	CACHE_TTL_SECONDS int  = 300
	CACHE_MAX_ENTRIES int  = 500

	RATE_LIMIT_RPS   float64 = 10
	RATE_LIMIT_BURST int     = 20

	SERVER_READ_TIMEOUT_SECONDS  int = 30
	SERVER_WRITE_TIMEOUT_SECONDS int = 180
	SERVER_IDLE_TIMEOUT_SECONDS  int = 120

	LOG_FORMAT string = "text"

	FEATURE_FLAGS FeatureFlags
)

type FeatureFlags struct {
	ProcessIframes     bool
	OnlyText           bool
	CheckRobotsTxt     bool
	VerifySSL          bool
	TextMode           bool
	LightMode          bool
	CaptureNetworkReqs bool
	CaptureConsoleMsgs bool
	PreserveHTTPSLinks bool
}

var defaultFeatureFlags = FeatureFlags{
	ProcessIframes:     false,
	OnlyText:           false,
	CheckRobotsTxt:     false,
	VerifySSL:          true,
	TextMode:           false,
	LightMode:          false,
	CaptureNetworkReqs: false,
	CaptureConsoleMsgs: false,
	PreserveHTTPSLinks: false,
}

func envBool(name string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		slog.Warn("invalid boolean, using fallback", "name", name, "value", v, "fallback", fallback)
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
		slog.Warn("invalid integer, using fallback", "name", name, "value", v, "fallback", fallback)
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
		slog.Warn("invalid float, using fallback", "name", name, "value", v, "fallback", fallback)
		return fallback
	}
	return parsed
}

func envString(name string, fallback string) string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	return v
}

func commaList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

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
	CRAWL_CONNECT_TIMEOUT_SECONDS = envInt("CRAWL_CONNECT_TIMEOUT_SECONDS", CRAWL_CONNECT_TIMEOUT_SECONDS)
	CRAWL_TOTAL_TIMEOUT_SECONDS = envInt("CRAWL_TOTAL_TIMEOUT_SECONDS", CRAWL_TOTAL_TIMEOUT_SECONDS)
	MAX_URLS_PER_REQUEST = envInt("MAX_URLS_PER_REQUEST", MAX_URLS_PER_REQUEST)
	MAX_PAGES_BEFORE_RECYCLE = envInt("MAX_PAGES_BEFORE_RECYCLE", MAX_PAGES_BEFORE_RECYCLE)
	MAX_RETRIES = envInt("MAX_RETRIES", MAX_RETRIES)
	MAX_REQUEST_BODY_BYTES = int64(envInt("MAX_REQUEST_BODY_BYTES", int(MAX_REQUEST_BODY_BYTES)))
	UPSTREAM_RETRIES = envInt("UPSTREAM_RETRIES", UPSTREAM_RETRIES)
	UPSTREAM_RETRY_DELAY_MS = envInt("UPSTREAM_RETRY_DELAY_MS", UPSTREAM_RETRY_DELAY_MS)
	CACHE_TTL_SECONDS = envInt("CACHE_TTL_SECONDS", CACHE_TTL_SECONDS)
	CACHE_MAX_ENTRIES = envInt("CACHE_MAX_ENTRIES", CACHE_MAX_ENTRIES)
	DEEP_CRAWL_MAX_DEPTH = envInt("DEEP_CRAWL_MAX_DEPTH", DEEP_CRAWL_MAX_DEPTH)
	DEEP_CRAWL_MAX_PAGES = envInt("DEEP_CRAWL_MAX_PAGES", DEEP_CRAWL_MAX_PAGES)
	MIN_WORD_THRESHOLD = envInt("MIN_WORD_THRESHOLD", MIN_WORD_THRESHOLD)
	SERVER_READ_TIMEOUT_SECONDS = envInt("SERVER_READ_TIMEOUT_SECONDS", SERVER_READ_TIMEOUT_SECONDS)
	SERVER_WRITE_TIMEOUT_SECONDS = envInt("SERVER_WRITE_TIMEOUT_SECONDS", SERVER_WRITE_TIMEOUT_SECONDS)
	SERVER_IDLE_TIMEOUT_SECONDS = envInt("SERVER_IDLE_TIMEOUT_SECONDS", SERVER_IDLE_TIMEOUT_SECONDS)
	RATE_LIMIT_BURST = envInt("RATE_LIMIT_BURST", RATE_LIMIT_BURST)

	PRUNING_THRESHOLD = envFloat64("PRUNING_THRESHOLD", PRUNING_THRESHOLD)
	SCROLL_DELAY = envFloat64("SCROLL_DELAY", SCROLL_DELAY)
	RATE_LIMIT_RPS = envFloat64("RATE_LIMIT_RPS", RATE_LIMIT_RPS)

	REMOVE_CONSENT_POPUPS = envBool("REMOVE_CONSENT_POPUPS", REMOVE_CONSENT_POPUPS)
	FLATTEN_SHADOW_DOM = envBool("FLATTEN_SHADOW_DOM", FLATTEN_SHADOW_DOM)
	REMOVE_OVERLAY = envBool("REMOVE_OVERLAY_ELEMENTS", REMOVE_OVERLAY)
	AVOID_ADS = envBool("AVOID_ADS", AVOID_ADS)
	AVOID_CSS = envBool("AVOID_CSS", AVOID_CSS)
	MEMORY_SAVING_MODE = envBool("MEMORY_SAVING_MODE", MEMORY_SAVING_MODE)
	INCLUDE_REFERENCES = envBool("INCLUDE_REFERENCES", INCLUDE_REFERENCES)
	ENABLE_STEALTH = envBool("ENABLE_STEALTH", ENABLE_STEALTH)
	SCAN_FULL_PAGE = envBool("SCAN_FULL_PAGE", SCAN_FULL_PAGE)
	DEEP_CRAWL = envBool("DEEP_CRAWL", DEEP_CRAWL)
	CACHE_ENABLED = envBool("CACHE_ENABLED", CACHE_ENABLED)
	VERIFY_SSL = envBool("VERIFY_SSL", defaultFeatureFlags.VerifySSL)

	CONTENT_FILTER_TYPE = envString("CONTENT_FILTER_TYPE", CONTENT_FILTER_TYPE)
	BM25_USER_QUERY = envString("BM25_USER_QUERY", BM25_USER_QUERY)
	CSS_SELECTOR = envString("CSS_SELECTOR", CSS_SELECTOR)
	EXCLUDED_TAGS = envString("EXCLUDED_TAGS", EXCLUDED_TAGS)
	TARGET_ELEMENTS = envString("TARGET_ELEMENTS", TARGET_ELEMENTS)
	JS_CODE = envString("JS_CODE", JS_CODE)
	WAIT_FOR = envString("WAIT_FOR", WAIT_FOR)
	CONTENT_SOURCE = envString("CONTENT_SOURCE", CONTENT_SOURCE)
	USER_AGENT = envString("USER_AGENT", USER_AGENT)
	LOG_FORMAT = envString("LOG_FORMAT", LOG_FORMAT)

	FEATURE_FLAGS = FeatureFlags{
		ProcessIframes:     envBool("PROCESS_IFRAMES", defaultFeatureFlags.ProcessIframes),
		OnlyText:           envBool("ONLY_TEXT", defaultFeatureFlags.OnlyText),
		CheckRobotsTxt:     envBool("CHECK_ROBOTS_TXT", defaultFeatureFlags.CheckRobotsTxt),
		VerifySSL:          envBool("VERIFY_SSL", defaultFeatureFlags.VerifySSL),
		TextMode:           envBool("TEXT_MODE", defaultFeatureFlags.TextMode),
		LightMode:          envBool("LIGHT_MODE", defaultFeatureFlags.LightMode),
		CaptureNetworkReqs: envBool("CAPTURE_NETWORK_REQUESTS", defaultFeatureFlags.CaptureNetworkReqs),
		CaptureConsoleMsgs: envBool("CAPTURE_CONSOLE_MESSAGES", defaultFeatureFlags.CaptureConsoleMsgs),
		PreserveHTTPSLinks: envBool("PRESERVE_HTTPS_FOR_INTERNAL_LINKS", defaultFeatureFlags.PreserveHTTPSLinks),
	}

	if v := os.Getenv("PROXY_CONFIG_JSON"); strings.TrimSpace(v) != "" {
		PROXY_CONFIG_JSON = strings.TrimSpace(v)
		var decoded []ProxyConfig
		if err := parseProxyConfig(PROXY_CONFIG_JSON, &decoded); err != nil {
			slog.Warn("invalid PROXY_CONFIG_JSON, ignoring proxy override", "error", err)
			HAS_PROXY_CONFIG = false
			PROXY_CONFIG_PARSED = nil
		} else {
			HAS_PROXY_CONFIG = true
			PROXY_CONFIG_PARSED = decoded
		}
	}

	parsedURL, err := url.Parse(CRAWL4AI_ENDPOINT)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		slog.Error("CRAWL4AI_ENDPOINT is not a valid URL", "endpoint", CRAWL4AI_ENDPOINT)
		os.Exit(1)
	}
}

func parseProxyConfig(raw string, target *[]ProxyConfig) error {
	var rawAny any
	if err := json.Unmarshal([]byte(raw), &rawAny); err != nil {
		return fmt.Errorf("proxy config parse: %w", err)
	}

	switch v := rawAny.(type) {
	case []any:
		result := make([]ProxyConfig, 0, len(v))
		for _, item := range v {
			switch elem := item.(type) {
			case string:
				result = append(result, ProxyConfig{Server: elem})
			case map[string]any:
				pc := ProxyConfig{}
				if s, ok := elem["server"].(string); ok {
					pc.Server = s
				}
				if s, ok := elem["username"].(string); ok {
					pc.Username = s
				}
				if s, ok := elem["password"].(string); ok {
					pc.Password = s
				}
				if s, ok := elem["type"].(string); ok {
					pc.Type = s
				}
				if s, ok := elem["ip"].(string); ok {
					pc.IP = s
				}
				result = append(result, pc)
			default:
				return fmt.Errorf("unsupported proxy config item type: %T", elem)
			}
		}
		*target = result
		return nil
	case map[string]any:
		pc := ProxyConfig{}
		if s, ok := v["server"].(string); ok {
			pc.Server = s
		}
		if s, ok := v["username"].(string); ok {
			pc.Username = s
		}
		if s, ok := v["password"].(string); ok {
			pc.Password = s
		}
		if s, ok := v["type"].(string); ok {
			pc.Type = s
		}
		if s, ok := v["ip"].(string); ok {
			pc.IP = s
		}
		*target = []ProxyConfig{pc}
		return nil
	default:
		return fmt.Errorf("unsupported proxy config type: %T", v)
	}
}

func EffectiveContentFilter(req *Request) (filterType string, bm25Query string) {
	filterType = CONTENT_FILTER_TYPE
	bm25Query = BM25_USER_QUERY
	if req != nil {
		if req.ContentFilterType != nil && *req.ContentFilterType != "" {
			filterType = *req.ContentFilterType
		}
		if req.Bm25Query != nil && *req.Bm25Query != "" {
			bm25Query = *req.Bm25Query
		}
	}
	return filterType, bm25Query
}

func EffectiveCssSelector(req *Request) string {
	if req != nil && req.CssSelector != nil && *req.CssSelector != "" {
		return *req.CssSelector
	}
	return CSS_SELECTOR
}

func EffectiveExcludedTags(req *Request) []string {
	if req != nil && len(req.ExcludedTags) > 0 {
		return req.ExcludedTags
	}
	return commaList(EXCLUDED_TAGS)
}

func EffectiveTargetElements(req *Request) []string {
	if req != nil && len(req.TargetElements) > 0 {
		return req.TargetElements
	}
	return commaList(TARGET_ELEMENTS)
}

func EffectiveJsCode(req *Request) []string {
	if req != nil && len(req.JsCode) > 0 {
		return req.JsCode
	}
	if JS_CODE != "" {
		return commaList(JS_CODE)
	}
	return nil
}

func EffectiveWaitFor(req *Request) string {
	if req != nil && req.WaitFor != nil && *req.WaitFor != "" {
		return *req.WaitFor
	}
	return WAIT_FOR
}

func EffectiveContentSource(req *Request) string {
	if req != nil && req.ContentSource != nil && *req.ContentSource != "" {
		return *req.ContentSource
	}
	return CONTENT_SOURCE
}

func EffectiveStealth(req *Request) bool {
	if req != nil && req.EnableStealth != nil {
		return *req.EnableStealth
	}
	return ENABLE_STEALTH
}

func EffectiveUserAgent(req *Request) string {
	if req != nil && req.UserAgent != nil && *req.UserAgent != "" {
		return *req.UserAgent
	}
	return USER_AGENT
}

func EffectiveDeepCrawl(req *Request) bool {
	if req != nil && req.DeepCrawl != nil {
		return *req.DeepCrawl
	}
	return DEEP_CRAWL
}

func EffectiveDeepCrawlMaxDepth(req *Request) int {
	if req != nil && req.DeepCrawlMaxDepth != nil && *req.DeepCrawlMaxDepth > 0 {
		return *req.DeepCrawlMaxDepth
	}
	return DEEP_CRAWL_MAX_DEPTH
}

func EffectiveDeepCrawlMaxPages(req *Request) int {
	if req != nil && req.DeepCrawlMaxPages != nil && *req.DeepCrawlMaxPages > 0 {
		return *req.DeepCrawlMaxPages
	}
	return DEEP_CRAWL_MAX_PAGES
}

func EffectiveScanFullPage() bool {
	return SCAN_FULL_PAGE
}

func EffectiveScrollDelay() float64 {
	return SCROLL_DELAY
}

func SelectMarkdownContent(result CrawlResultItem, contentSource string) string {
	switch contentSource {
	case "raw_html":
		return strings.TrimSpace(result.Markdown.RawMarkdown)
	case "fit_html":
		if result.Markdown.FitMarkdown != nil && strings.TrimSpace(*result.Markdown.FitMarkdown) != "" {
			return *result.Markdown.FitMarkdown
		}
		return strings.TrimSpace(result.Markdown.RawMarkdown)
	case "cleaned_html":
		return strings.TrimSpace(result.Markdown.RawMarkdown)
	default:
		if result.Markdown.FitMarkdown != nil && strings.TrimSpace(*result.Markdown.FitMarkdown) != "" {
			return *result.Markdown.FitMarkdown
		}
		return strings.TrimSpace(result.Markdown.RawMarkdown)
	}
}
