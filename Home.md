# Crawl4AI Proxy — Configuration Reference

All configuration is done via environment variables. This document lists every variable, its default value, accepted values, recommended value, and purpose.

## Table of Contents

- [Server & Network](#server--network)
- [Crawl4AI Connection](#crawl4ai-connection)
- [Crawl4AI Authentication](#crawl4ai-authentication)
- [Content Filtering & Extraction](#content-filtering--extraction)
- [Content Source Selection](#content-source-selection)
- [CSS & DOM Targeting](#css--dom-targeting)
- [JavaScript & Dynamic Content](#javascript--dynamic-content)
- [Stealth & Anti-Detection](#stealth--anti-detection)
- [Deep Crawling](#deep-crawling)
- [Browser Behavior](#browser-behavior)
- [Crawler Behavior](#crawler-behavior)
- [Anti-Bot & Proxy](#anti-bot--proxy)
- [Feature Flags](#feature-flags)
- [Upstream Resilience](#upstream-resilience)
- [Response Cache](#response-cache)
- [Rate Limiting](#rate-limiting)
- [Logging](#logging)

---

## Server & Network

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `LISTEN_IP` | `""` (all interfaces) | IP address or empty string | `""` | IP address to bind. Empty string listens on all interfaces. |
| `LISTEN_PORT` | `8000` | Any valid port (1–65535) | `8000` | TCP port the proxy listens on. |
| `SERVER_READ_TIMEOUT_SECONDS` | `30` | Integer ≥ 0 | `30` | Maximum seconds for reading the entire request, including the body. |
| `SERVER_WRITE_TIMEOUT_SECONDS` | `180` | Integer ≥ 0 | `180` | Maximum seconds for writing the response. Set higher than `CRAWL_TOTAL_TIMEOUT_SECONDS` because upstream crawls can take time. |
| `SERVER_IDLE_TIMEOUT_SECONDS` | `120` | Integer ≥ 0 | `120` | Maximum seconds to wait for the next request on a keepalive connection. |

## Crawl4AI Connection

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `CRAWL4AI_ENDPOINT` | `http://crawl4ai:11235/crawl` | Any HTTP(S) URL | `http://crawl4ai:11235/crawl` | Full URL of the Crawl4AI Docker API `/crawl` endpoint. Must be a valid URL or the proxy exits on startup. |
| `CRAWL_CONNECT_TIMEOUT_SECONDS` | `10` | Integer ≥ 0 | `10` | Timeout in seconds for establishing a TCP connection to Crawl4AI. |
| `CRAWL_TOTAL_TIMEOUT_SECONDS` | `120` | Integer ≥ 0 | `120` | Total timeout in seconds for the entire request to Crawl4AI, including connection and reading the response. |
| `MAX_REQUEST_BODY_BYTES` | `1048576` (1 MiB) | Integer ≥ 0 | `1048576` | Maximum request body size in bytes. Requests exceeding this are rejected with 413. |
| `MAX_URLS_PER_REQUEST` | `100` | Integer ≥ 1 | `100` | Maximum number of URLs allowed per batch request. Mirrors Crawl4AI's own limit. |

## Crawl4AI Authentication

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `CRAWL4AI_AUTH_TOKEN` | `""` (empty) | Any string | Your JWT/token | Bearer token or API key for authenticating with a secured Crawl4AI instance. Leave empty if Crawl4AI has no auth. |
| `CRAWL4AI_AUTH_SCHEME` | `Bearer` | Any string | `Bearer` | Prefix for the auth header value. Combined with `CRAWL4AI_AUTH_TOKEN` to form `Bearer <token>`. |
| `CRAWL4AI_AUTH_HEADER` | `Authorization` | Any HTTP header name | `Authorization` | HTTP header name used for downstream authentication. |

## Content Filtering & Extraction

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `CONTENT_FILTER_TYPE` | `pruning` | `pruning`, `bm25`, `none` | `pruning` | Content filter strategy applied by Crawl4AI. `pruning` removes boilerplate (nav, footers). `bm25` extracts only content relevant to a query. `none` passes raw markdown through. |
| `BM25_USER_QUERY` | `""` (empty) | Any search query string | `""` | Default query used when `CONTENT_FILTER_TYPE=bm25` and no per-request override is provided. |
| `PRUNING_THRESHOLD` | `0.48` | Float 0.0–1.0 | `0.48` | Aggressiveness of pruning. Lower = more aggressive (removes more), higher = keeps more. `0.3` is very aggressive, `0.7` is lenient. |
| `MIN_WORD_THRESHOLD` | `10` | Integer ≥ 0 | `10` | Minimum word count for a text block to survive pruning. Blocks with fewer words are discarded. |
| `INCLUDE_REFERENCES` | `true` | `true`, `false` | `true` | Whether to append link references/citations to the markdown output. |

Per-request overrides (via JSON body on `/crawl`):

| Field | Type | Purpose |
|-------|------|---------|
| `content_filter_type` | string | Override `CONTENT_FILTER_TYPE` for this request. |
| `bm25_query` | string | Override `BM25_USER_QUERY` for this request. |

## Content Source Selection

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `CONTENT_SOURCE` | `fit_html` | `fit_html`, `raw_html`, `cleaned_html` | `fit_html` | Controls which markdown field the proxy returns to OpenWebUI. `fit_html` returns `fit_markdown` (pruned, highest quality). `raw_html` returns `raw_markdown` (full unfiltered content). `cleaned_html` returns `raw_markdown` (same as raw, but signals intent for cleaned source). |

Per-request override: `content_source` field in the `/crawl` JSON body.

**Recommended**: Keep `fit_html` for best signal-to-noise ratio. Use `raw_html` only when you need the full unprocessed content.

## CSS & DOM Targeting

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `CSS_SELECTOR` | `""` (empty) | CSS selector string | `""` | Target a specific element for extraction, e.g. `"article"`, `"main"`, `"#content"`. Empty means extract the full page. |
| `EXCLUDED_TAGS` | `""` (empty) | Comma-separated HTML tag names | `""` | HTML tags to strip before extraction, e.g. `"nav,footer,aside"`. |
| `TARGET_ELEMENTS` | `""` (empty) | Comma-separated CSS selectors | `""` | Specific elements to target for extraction, e.g. `".post-content,.article-body"`. |

Per-request overrides: `css_selector`, `excluded_tags` (array), `target_elements` (array) in the `/crawl` JSON body.

## JavaScript & Dynamic Content

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `JS_CODE` | `""` (empty) | Comma-separated JS strings | `""` | JavaScript code to execute on the page before extraction. e.g. `"document.querySelector('.cookie-banner').remove()"`. |
| `WAIT_FOR` | `""` (empty) | CSS selector or JS expression | `""` | Wait for this selector to appear or JS expression to evaluate to truthy before extraction. e.g. `".content-loaded"`. |
| `SCAN_FULL_PAGE` | `false` | `true`, `false` | `false` | Scroll through the entire page before extraction. Useful for infinite-scroll pages. |
| `SCROLL_DELAY` | `0.0` | Float ≥ 0.0 | `0.0` | Seconds to wait between scroll steps when `SCAN_FULL_PAGE=true`. |

Per-request overrides: `js_code` (array), `wait_for`, in the `/crawl` JSON body.

## Stealth & Anti-Detection

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `ENABLE_STEALTH` | `false` | `true`, `false` | `false` | Apply Playwright stealth evasion scripts to avoid bot detection. Enable if sites block headless browsers. |
| `USER_AGENT` | `""` (empty, browser default) | Any User-Agent string | `""` | Override the browser User-Agent string. Leave empty for Crawl4AI's default Chromium UA. |

Per-request overrides: `enable_stealth` (boolean), `user_agent` (string) in the `/crawl` JSON body.

## Deep Crawling

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `DEEP_CRAWL` | `false` | `true`, `false` | `false` | Enable BFS deep crawl strategy. The proxy will follow links from the initial URL up to the configured depth. Only use for documentation sites or sitemap crawling. |
| `DEEP_CRAWL_MAX_DEPTH` | `1` | Integer ≥ 1 | `3` | Maximum link-follow depth when `DEEP_CRAWL=true`. Higher values crawl more pages but take longer. |
| `DEEP_CRAWL_MAX_PAGES` | `10` | Integer ≥ 1 | `50` | Maximum number of pages to crawl when `DEEP_CRAWL=true`. Caps total pages. |

Per-request overrides: `deep_crawl` (boolean), `deep_crawl_max_depth` (integer), `deep_crawl_max_pages` (integer) in the `/crawl` JSON body.

## Browser Behavior

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `REMOVE_CONSENT_POPUPS` | `true` | `true`, `false` | `true` | Auto-dismiss cookie consent/GDPR popups from 40+ CMP platforms (OneTrust, Cookiebot, Didomi, etc.). |
| `FLATTEN_SHADOW_DOM` | `true` | `true`, `false` | `true` | Flatten Shadow DOM components to extract hidden content in modern web apps. |
| `REMOVE_OVERLAY_ELEMENTS` | `true` | `true`, `false` | `true` | Remove sticky overlays, popups, and interstitials that block content. |
| `AVOID_ADS` | `true` | `true`, `false` | `true` | Block ad/tracker domains at the network level for faster, cleaner crawls. |
| `AVOID_CSS` | `false` | `true`, `false` | `false` | Block CSS file loading. Speeds up crawls but may reduce content quality. Enable only for speed-critical use cases. |
| `MEMORY_SAVING_MODE` | `true` | `true`, `false` | `true` | Aggressive memory management flags to prevent memory leaks during long sessions. |
| `MAX_PAGES_BEFORE_RECYCLE` | `100` | Integer ≥ 0 | `100` | Restart the browser after this many pages to prevent memory leaks. Set to `0` to disable recycling. |

## Crawler Behavior

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `PROCESS_IFRAMES` | `false` | `true`, `false` | `false` | Extract content from iframe elements. Enable if target content is embedded in iframes. |
| `ONLY_TEXT` | `false` | `true`, `false` | `false` | Return text-only content without links, images, or formatting. Strips all HTML markup. |
| `CHECK_ROBOTS_TXT` | `false` | `true`, `false` | `false` | Check and respect the site's robots.txt before crawling. |
| `VERIFY_SSL` | `true` | `true`, `false` | `true` | Verify SSL certificates on crawled sites. Disable only for self-signed cert environments. |
| `TEXT_MODE` | `false` | `true`, `false` | `false` | Disable images and rich content in the browser. Faster but less complete. |
| `LIGHT_MODE` | `false` | `true`, `false` | `false` | Disable background browser features for faster light-weight crawls. |
| `CAPTURE_NETWORK_REQUESTS` | `false` | `true`, `false` | `false` | Capture network request log from the browser. Useful for debugging. |
| `CAPTURE_CONSOLE_MESSAGES` | `false` | `true`, `false` | `false` | Capture browser console messages. Useful for debugging JS issues. |
| `PRESERVE_HTTPS_FOR_INTERNAL_LINKS` | `false` | `true`, `false` | `false` | Preserve HTTPS for internal links after page redirects. |

## Anti-Bot & Proxy

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `MAX_RETRIES` | `0` | Integer ≥ 0 | `0` or `2` | Number of retry attempts with proxy rotation on anti-bot blocks. Only effective with `PROXY_CONFIG_JSON`. Set to `2` if sites frequently block you. |
| `PROXY_CONFIG_JSON` | `""` (empty) | JSON string | `""` | Proxy configuration for Crawl4AI. Accepts a JSON array `["direct","http://proxy:8080"]` or a JSON object `{"server":"http://proxy:8080","username":"user","password":"pass"}`. |

## Upstream Resilience

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `UPSTREAM_RETRIES` | `0` | Integer ≥ 0 | `0` or `2` | Number of times to retry the upstream Crawl4AI request on 5xx errors. Uses exponential backoff starting at `UPSTREAM_RETRY_DELAY_MS`. |
| `UPSTREAM_RETRY_DELAY_MS` | `500` | Integer ≥ 0 | `500` | Initial delay in milliseconds between retries. Doubles on each retry (exponential backoff). |

## Response Cache

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `CACHE_ENABLED` | `true` | `true`, `false` | `true` for production, `false` for development | Enable in-memory TTL response caching. Caches successful 200 responses. Requests with per-request overrides bypass the cache automatically. |
| `CACHE_TTL_SECONDS` | `300` (5 minutes) | Integer ≥ 0 | `300` | Time-to-live for cached responses in seconds. Expired entries are pruned in the background. |
| `CACHE_MAX_ENTRIES` | `500` | Integer ≥ 1 | `500` | Maximum number of cached responses. Oldest entries are evicted when full. |

**Cache bypass**: Send `Cache-Control: no-cache` header to force a fresh request.

**Cache behavior**: Only HTTP 200 responses are cached. Per-request overrides (any of `content_filter_type`, `bm25_query`, `css_selector`, `js_code`, etc.) automatically bypass the cache.

## Rate Limiting

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `RATE_LIMIT_RPS` | `10` | Float ≥ 0 | `10` | Requests per second allowed per client IP. 0 = no rate limit. |
| `RATE_LIMIT_BURST` | `20` | Integer ≥ 1 | `20` | Burst capacity per client IP. Allows temporary spikes up to this many requests before throttling. |

When rate limited, the proxy returns HTTP 429 with a `Retry-After` header.

## Logging

| Variable | Default | Accepted Values | Recommended | Purpose |
|----------|---------|----------------|-------------|---------|
| `LOG_FORMAT` | `text` | `text`, `json` | `json` for production, `text` for development | Log output format. `json` produces structured logs suitable for log aggregation systems (ELK, Datadog, etc.). `text` produces human-readable logs. |

---

## Per-Request Overrides

All variables in the **Crawl Behavior** sections above can be overridden per-request by including fields in the `/crawl` JSON body:

```json
{
  "urls": ["https://example.com"],
  "content_filter_type": "bm25",
  "bm25_query": "artificial intelligence",
  "css_selector": "article",
  "excluded_tags": ["nav", "footer"],
  "target_elements": [".main-content"],
  "js_code": ["document.title"],
  "wait_for": ".content-loaded",
  "content_source": "raw_html",
  "enable_stealth": true,
  "user_agent": "Mozilla/5.0 (compatible; MyBot/1.0)",
  "deep_crawl": true,
  "deep_crawl_max_depth": 3,
  "deep_crawl_max_pages": 50
}
```

When a per-request field is provided, it takes precedence over the environment variable default. When omitted, the environment variable default is used.

## Quick-Reference Symptom Table

| Symptom | Likely Cause | Variable to Adjust | Suggested First Try |
|---------|-------------|---------------------|---------------------|
| Crawl fails on protected sites | Bot detection | `ENABLE_STEALTH`, `MAX_RETRIES`, `PROXY_CONFIG_JSON` | `ENABLE_STEALTH=true`, `MAX_RETRIES=2` |
| Output contains popups/overlays | Overlay cleanup not aggressive | `REMOVE_CONSENT_POPUPS`, `REMOVE_OVERLAY_ELEMENTS` | Keep both `true` |
| Crawl seems slow on heavy pages | Too many resources loaded | `AVOID_ADS`, `AVOID_CSS` | `AVOID_ADS=true`, `AVOID_CSS=true` |
| Memory grows over long runs | Browser not recycled enough | `MEMORY_SAVING_MODE`, `MAX_PAGES_BEFORE_RECYCLE` | `MEMORY_SAVING_MODE=true`, `MAX_PAGES_BEFORE_RECYCLE=50` |
| Too much boilerplate (menus/footer) | Pruning too weak | `PRUNING_THRESHOLD` | Lower to `0.40` |
| Main content disappears | Pruning too aggressive | `PRUNING_THRESHOLD` | Raise to `0.60` |
| Only need relevant content for a query | No query-based filtering | `CONTENT_FILTER_TYPE`, `BM25_USER_QUERY` | `CONTENT_FILTER_TYPE=bm25`, `BM25_USER_QUERY=your topic` |
| Need specific element only | Full page extracted | `CSS_SELECTOR` | `CSS_SELECTOR=article` or `.post-content` |
| Page requires JS rendering | JS not executed | `JS_CODE`, `WAIT_FOR` | `WAIT_FOR=.content-loaded` |
| Content from iframe not showing | iframes skipped | `PROCESS_IFRAMES` | `PROCESS_IFRAMES=true` |
| Proxy returns 502 unauthorized | Crawl4AI auth required | `CRAWL4AI_AUTH_TOKEN`, `CRAWL4AI_AUTH_SCHEME` | Set your token |
| Large request rejected | Size limit reached | `MAX_REQUEST_BODY_BYTES`, `MAX_URLS_PER_REQUEST` | Increase carefully |
| Citations missing from output | References disabled | `INCLUDE_REFERENCES` | `INCLUDE_REFERENCES=true` |
| Upstream timeouts frequently | Timeout too low | `CRAWL_TOTAL_TIMEOUT_SECONDS` | Raise to `180` or `300` |
| Repeated 502 from upstream | Transient failures | `UPSTREAM_RETRIES` | `UPSTREAM_RETRIES=2` |
| Same pages crawled repeatedly | Cache disabled | `CACHE_ENABLED`, `CACHE_TTL_SECONDS` | `CACHE_ENABLED=true`, `CACHE_TTL_SECONDS=300` |
| Too many requests overwhelming | No rate limiting | `RATE_LIMIT_RPS`, `RATE_LIMIT_BURST` | `RATE_LIMIT_RPS=10` |

---

## Recommended Configurations for OpenWebUI

These are tested profiles optimized specifically for OpenWebUI's web search / external web loader integration. Pick the one that matches your use case.

### Profile 1: General Purpose (Best for Most Users)

Balanced quality, speed, and stability. Best default for OpenWebUI chat where users search mixed content types (news, documentation, blogs).

```yaml
environment:
  - CONTENT_FILTER_TYPE=pruning
  - PRUNING_THRESHOLD=0.48
  - MIN_WORD_THRESHOLD=10
  - INCLUDE_REFERENCES=true
  - CONTENT_SOURCE=fit_html
  - REMOVE_CONSENT_POPUPS=true
  - FLATTEN_SHADOW_DOM=true
  - REMOVE_OVERLAY_ELEMENTS=true
  - AVOID_ADS=true
  - AVOID_CSS=false
  - MEMORY_SAVING_MODE=true
  - MAX_PAGES_BEFORE_RECYCLE=100
  - ENABLE_STEALTH=false
  - DEEP_CRAWL=false
  - CRAWL_TOTAL_TIMEOUT_SECONDS=120
  - CRAWL_CONNECT_TIMEOUT_SECONDS=10
  - CACHE_ENABLED=true
  - CACHE_TTL_SECONDS=300
  - RATE_LIMIT_RPS=10
  - RATE_LIMIT_BURST=20
  - LOG_FORMAT=json
```

### Profile 2: High Quality / Research

Maximum content quality for research-heavy OpenWebUI usage. Slower but extracts the most relevant content using BM25 query-based filtering. Set `BM25_USER_QUERY` to a general fallback, then override per-request with the user's actual query.

```yaml
environment:
  - CONTENT_FILTER_TYPE=bm25
  - BM25_USER_QUERY=research
  - PRUNING_THRESHOLD=0.48
  - MIN_WORD_THRESHOLD=5
  - INCLUDE_REFERENCES=true
  - CONTENT_SOURCE=fit_html
  - REMOVE_CONSENT_POPUPS=true
  - FLATTEN_SHADOW_DOM=true
  - REMOVE_OVERLAY_ELEMENTS=true
  - AVOID_ADS=true
  - AVOID_CSS=false
  - MEMORY_SAVING_MODE=true
  - MAX_PAGES_BEFORE_RECYCLE=100
  - PROCESS_IFRAMES=true
  - ENABLE_STEALTH=false
  - DEEP_CRAWL=false
  - CRAWL_TOTAL_TIMEOUT_SECONDS=180
  - CRAWL_CONNECT_TIMEOUT_SECONDS=10
  - CACHE_ENABLED=true
  - CACHE_TTL_SECONDS=300
  - RATE_LIMIT_RPS=10
  - RATE_LIMIT_BURST=20
  - LOG_FORMAT=json
```

OpenWebUI can override `content_filter_type` and `bm25_query` per-request by sending:
```json
{"urls": ["https://example.com"], "content_filter_type": "bm25", "bm25_query": "artificial intelligence"}
```

### Profile 3: Anti-Bot / Hard-to-Crawl Sites

For OpenWebUI instances that crawl sites with aggressive bot detection (Cloudflare, Datadome, PerimeterX, etc.). Uses stealth mode, retries with proxy rotation, and browser recycling.

```yaml
environment:
  - CONTENT_FILTER_TYPE=pruning
  - PRUNING_THRESHOLD=0.48
  - INCLUDE_REFERENCES=true
  - CONTENT_SOURCE=fit_html
  - REMOVE_CONSENT_POPUPS=true
  - FLATTEN_SHADOW_DOM=true
  - REMOVE_OVERLAY_ELEMENTS=true
  - AVOID_ADS=true
  - AVOID_CSS=true
  - MEMORY_SAVING_MODE=true
  - MAX_PAGES_BEFORE_RECYCLE=50
  - ENABLE_STEALTH=true
  - MAX_RETRIES=2
  - UPSTREAM_RETRIES=2
  - UPSTREAM_RETRY_DELAY_MS=500
  - PROXY_CONFIG_JSON=["direct","http://my-proxy:8080"]
  - CRAWL_TOTAL_TIMEOUT_SECONDS=180
  - CRAWL_CONNECT_TIMEOUT_SECONDS=15
  - CACHE_ENABLED=true
  - CACHE_TTL_SECONDS=600
  - RATE_LIMIT_RPS=5
  - RATE_LIMIT_BURST=10
  - LOG_FORMAT=json
```

### Profile 4: Memory-Constrained / Lightweight

For small servers or long-running OpenWebUI sessions where browser memory is a concern. Aggressive recycling, CSS blocking, and text mode for minimal overhead.

```yaml
environment:
  - CONTENT_FILTER_TYPE=pruning
  - PRUNING_THRESHOLD=0.40
  - INCLUDE_REFERENCES=false
  - CONTENT_SOURCE=fit_html
  - REMOVE_CONSENT_POPUPS=true
  - FLATTEN_SHADOW_DOM=true
  - REMOVE_OVERLAY_ELEMENTS=true
  - AVOID_ADS=true
  - AVOID_CSS=true
  - MEMORY_SAVING_MODE=true
  - MAX_PAGES_BEFORE_RECYCLE=40
  - TEXT_MODE=true
  - CRAWL_TOTAL_TIMEOUT_SECONDS=90
  - CRAWL_CONNECT_TIMEOUT_SECONDS=10
  - CACHE_ENABLED=true
  - CACHE_TTL_SECONDS=300
  - CACHE_MAX_ENTRIES=200
  - RATE_LIMIT_RPS=10
  - RATE_LIMIT_BURST=20
  - LOG_FORMAT=text
```

### Profile 5: Deep Crawl / Documentation Sites

For OpenWebUI instances that need to index entire documentation sites or knowledge bases. Uses BFS deep crawl with depth and page limits.

```yaml
environment:
  - CONTENT_FILTER_TYPE=pruning
  - PRUNING_THRESHOLD=0.48
  - INCLUDE_REFERENCES=true
  - CONTENT_SOURCE=fit_html
  - REMOVE_CONSENT_POPUPS=true
  - FLATTEN_SHADOW_DOM=true
  - REMOVE_OVERLAY_ELEMENTS=true
  - AVOID_ADS=true
  - AVOID_CSS=false
  - MEMORY_SAVING_MODE=true
  - MAX_PAGES_BEFORE_RECYCLE=100
  - DEEP_CRAWL=true
  - DEEP_CRAWL_MAX_DEPTH=3
  - DEEP_CRAWL_MAX_PAGES=50
  - CSS_SELECTOR=article
  - CRAWL_TOTAL_TIMEOUT_SECONDS=300
  - CRAWL_CONNECT_TIMEOUT_SECONDS=10
  - CACHE_ENABLED=true
  - CACHE_TTL_SECONDS=300
  - RATE_LIMIT_RPS=5
  - RATE_LIMIT_BURST=10
  - LOG_FORMAT=json
```

> **Note:** Deep crawl sends multiple pages back to OpenWebUI, one per URL. Ensure your OpenWebUI instance can handle larger responses.

### Profile 6: Maximum Speed / Minimal Content

For real-time OpenWebUI chat where speed matters more than content completeness. Strips everything non-essential for fastest possible extraction.

```yaml
environment:
  - CONTENT_FILTER_TYPE=pruning
  - PRUNING_THRESHOLD=0.35
  - MIN_WORD_THRESHOLD=15
  - INCLUDE_REFERENCES=false
  - CONTENT_SOURCE=fit_html
  - REMOVE_CONSENT_POPUPS=true
  - REMOVE_OVERLAY_ELEMENTS=true
  - AVOID_ADS=true
  - AVOID_CSS=true
  - MEMORY_SAVING_MODE=true
  - MAX_PAGES_BEFORE_RECYCLE=100
  - ONLY_TEXT=true
  - CRAWL_TOTAL_TIMEOUT_SECONDS=60
  - CRAWL_CONNECT_TIMEOUT_SECONDS=5
  - CACHE_ENABLED=true
  - CACHE_TTL_SECONDS=600
  - RATE_LIMIT_RPS=20
  - RATE_LIMIT_BURST=30
  - LOG_FORMAT=json
```

### Profile Selection Guide

| Your OpenWebUI Usage | Recommended Profile |
|----------------------|---------------------|
| General chat with web search | Profile 1: General Purpose |
| Research papers, deep analysis | Profile 2: High Quality |
| Sites block your crawls | Profile 3: Anti-Bot |
| Small server / limited RAM | Profile 4: Memory-Constrained |
| Indexing docs / knowledge bases | Profile 5: Deep Crawl |
| Fast real-time answers, minimal depth | Profile 6: Maximum Speed |

### Per-Request Variable Overrides at a Glance

When using OpenWebUI's External Web Loader URL pointing to this proxy, you can pass per-request overrides in the JSON body. This is most useful when your OpenWebUI setup allows custom web loader configurations or piping queries through the `bm25_query` field:

| Override Field | Overrides Env Var | Example Value | Use Case |
|---------------|-------------------|---------------|----------|
| `content_filter_type` | `CONTENT_FILTER_TYPE` | `"bm25"` | Switch to query-relevant extraction |
| `bm25_query` | `BM25_USER_QUERY` | `"climate change"` | Target extraction to user's question |
| `css_selector` | `CSS_SELECTOR` | `"article"` | Extract only article body |
| `excluded_tags` | `EXCLUDED_TAGS` | `["nav","footer"]` | Strip navigation and footer |
| `target_elements` | `TARGET_ELEMENTS` | `[".post-content"]` | Target specific CSS classes |
| `js_code` | `JS_CODE` | `["window.scrollTo(0,document.body.scrollHeight)"]` | Execute JS before extraction |
| `wait_for` | `WAIT_FOR` | `".content-loaded"` | Wait for dynamic content |
| `content_source` | `CONTENT_SOURCE` | `"raw_html"` | Get unfiltered markdown |
| `enable_stealth` | `ENABLE_STEALTH` | `true` | Enable stealth for specific sites |
| `user_agent` | `USER_AGENT` | `"Mozilla/5.0 ..."` | Custom UA for specific sites |
| `deep_crawl` | `DEEP_CRAWL` | `true` | Enable deep crawl for specific queries |
| `deep_crawl_max_depth` | `DEEP_CRAWL_MAX_DEPTH` | `3` | Control crawl depth per-request |
| `deep_crawl_max_pages` | `DEEP_CRAWL_MAX_PAGES` | `50` | Control page limit per-request |

---

## OpenWebUI Integration

There are **two ways** to connect this proxy to OpenWebUI. You can use either one or both together.

### Method 1: External Web Loader URL (Automatic, No UI Toggles)

This is the simplest integration. OpenWebUI's built-in web search automatically calls the proxy when it needs to fetch a URL. No toggles, no configuration per-chat — the proxy uses its environment variable defaults.

**Setup:**

1. Open your OpenWebUI instance
2. Go to **Admin Panel → Settings → Web Search**
3. Under **Web Loader Engine**, select `external`
4. Set **External Web Loader URL** to: `http://crawl4ai-proxy:8000/crawl`
5. Set **External Web Loader API Key** to any non-empty string (e.g., `proxy`) — it's a required field but the proxy doesn't use it
6. Save

**What happens:** When the AI decides to fetch a URL during web search, it calls the proxy. All settings come from your `docker-compose.yml` environment variables. No user controls in the chat UI.

### Method 2: Crawl4AI Tools (With UI Toggles)

This gives users **toggle controls** directly in the chat UI for Deep Research, Reading Mode, and Stealth Mode. The AI calls this tool when it needs to fetch a URL. Since it's a Tool (not a Function/Pipe), it can be **enabled globally** for all chats.

**Setup:**

1. Open your OpenWebUI instance
2. Go to **Workspace → Tools → + Add** (click the + icon in the Tools section)
3. Switch to the **Code** editor tab
4. Copy the entire contents of `crawl4ai_tools.py` from this repository and paste it in
5. Click **Save**
6. The tool now appears in **Admin Panel → Workspace → Tools**
7. Click the **global toggle** to enable it for all chats (no need to enable per-chat)

**What you'll see in the chat UI:**

| Toggle | Options | What it does |
|--------|---------|-------------|
| **Deep Research** | ON / OFF | Follows links from crawled pages for broader coverage. |
| **Research Depth** | Low / Medium / High | Link-follow depth. Low=1, Medium=3, High=5 levels. Only matters when Deep Research is ON. |
| **Max Pages** | Any number | Max pages to crawl per request. Mix freely with Research Depth (e.g. Medium depth + 30 pages). Default 10. |
| **Reading Mode** | Best / Focused / All | Best=pruning (removes boilerplate), Focused=bm25 (content matching your question), All=no filter (full page). |
| **Stealth Mode** | ON / OFF | Bypass bot detection on protected sites. |

**How Reading Mode "Focused" works:**

When you select "Focused", the function automatically uses your chat message as the search query. For example, if you ask *"What are the new features in Python 3.13?"* and the AI fetches `https://python.org/3.13/whatsnew`, the proxy will only extract paragraphs related to "Python 3.13 features" rather than the entire page. This dramatically improves relevance for RAG use cases.

**How both methods work together:**

| Scenario | What OpenWebUI uses | Settings source |
|----------|---------------------|-----------------|
| Normal chat with web search ON | Built-in External Web Loader | Environment variables (docker-compose) |
| User toggles Deep Research ON in chat | Crawl4AI Tool | Valve toggles (global) |
| User switches Reading Mode to "Focused" | Crawl4AI Tool | Valve toggles (global) |
| User enables Stealth Mode | Crawl4AI Tool | Valve toggles (global) |

### Valve Configuration Reference

| Valve | Type | Default | Options | Maps to Proxy Field |
|-------|------|---------|---------|-------------------|
| Deep Research | Toggle | OFF | ON / OFF | `deep_crawl: true / false` |
| Research Depth | Dropdown | Medium | Low / Medium / High | `deep_crawl_max_depth: 1 / 3 / 5` |
| Max Pages | Number | 10 | Any positive integer | `deep_crawl_max_pages` |
| Reading Mode | Dropdown | Best | Best / Focused / All | `content_filter_type: pruning / bm25 / none` |
| Stealth Mode | Toggle | OFF | ON / OFF | `enable_stealth: true / false` |

The proxy URL is configured via the `CRAWL4AI_PROXY_URL` environment variable (default: `http://crawl4ai-proxy:8000`). This matches the docker-compose service name automatically — only change it if your proxy is at a different address.

### Testing the Tool in OpenWebUI

Once you've installed the tool and enabled it globally, test it by asking the AI to fetch a URL. The tool is called automatically — you don't click anything in the chat.

**Test 1: Basic fetch (all defaults)**
> Ask: `Summarize https://example.com`

The AI should call `crawl_web` with `url="https://example.com"` and return a summary. You'll see a tool call indicator in the chat.

**Test 2: Deep Research ON**
> 1. Click the ⚙️ gear on the Crawl4AI tool in the Tools panel
> 2. Toggle **Deep Research** to ON
> 3. Set **Research Depth** to High
> 4. Ask: `What are the main topics on https://winpoin.com?`

The AI will call `crawl_web` with `deep_crawl=true`, following links from the page up to 5 levels deep, returning content from multiple pages.

**Test 3: Reading Mode Focused**
> 1. Set **Reading Mode** to Focused
> 2. Ask: `What does https://example.com say about domain registration?`

The AI will call `crawl_web` with `content_filter_type=bm25` and `bm25_query` set to your question, returning only the relevant paragraphs instead of the full page.

**Test 4: Stealth Mode**
> 1. Toggle **Stealth Mode** to ON
> 2. Ask: `Fetch https://protected-site.example.com`

The AI will call `crawl_web` with `enable_stealth=true`, which applies browser stealth evasion scripts.

**If the tool is not being called:**
1. Go to **Admin Panel → Workspace → Tools** and make sure the Crawl4AI tool toggle is ON (green)
2. Make sure your AI model supports tool/function calling (GPT-4, Claude, Llama 3.1+, etc.)
3. Check that the proxy URL in the tool gear settings matches your deployment (`http://crawl4ai-proxy:8000` by default)
4. Verify the proxy is running: `curl http://crawl4ai-proxy:8000/health` should return `{"status":"healthy"}`

**If you get connection errors:**
- Make sure the tool's proxy URL can be reached from the OpenWebUI container. If OpenWebUI and crawl4ai-proxy are in the same Docker network, use the service name (`http://crawl4ai-proxy:8000`). If they're on different networks, use the host URL (`http://host.docker.internal:8000`).
- Set `CRAWL4AI_PROXY_URL` in the OpenWebUI container environment if needed.