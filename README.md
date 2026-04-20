# Crawl4AI OpenWebUI Proxy (for Crawl4AI 0.8.x)

A lightweight proxy server that lets an [OpenWebUI](https://github.com/open-webui/open-webui) instance interact with a [Crawl4AI](https://github.com/unclecode/crawl4ai) instance, making OpenWebUI's web search feature faster and more usable without paying for an API service.

Forked from [lennyerik/crawl4ai-proxy](https://github.com/lennyerik/crawl4ai-proxy/) and updated for compatibility with Crawl4AI **0.8.x** (tested against **0.8.6**). See [`WIKI`](https://github.com/byfebian/crawl4ai-proxy/wiki) for full configuration reference and troubleshooting.

## What This Proxy Does

OpenWebUI's External Web Loader sends a simple `{"urls": [...]}` request. Crawl4AI's Docker API expects a richer request format and returns a complex response. This proxy sits between them and:

1. Receives `{"urls": [...]}` from OpenWebUI
2. Enriches the request with Crawl4AI 0.8.x-ready features
3. Forwards the request to Crawl4AI
4. Converts Crawl4AI's response back into OpenWebUI's expected format
5. Prefers `fit_markdown` (pruned, high-quality content) over `raw_markdown`

This proxy also ships with an **OpenWebUI Tool** (`crawl4ai_tools.py`) that adds user-friendly toggles for Deep Research, Reading Mode, and Stealth Mode directly in the chat UI.

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/crawl` | POST | Main proxy endpoint - accepts `{"urls": [...]}` from OpenWebUI |
| `/md` | POST | Markdown-only extraction - accepts `{"url": "...", "filter_type": "fit"\|"raw"\|"bm25", "bm25_query": "..."}` |
| `/screenshot` | POST | Screenshot capture - accepts `{"url": "..."}` |
| `/execute_js` | POST | Execute JavaScript - accepts `{"url": "...", "scripts": ["..."]}` |
| `/health` | GET | Health check - returns `{"status": "healthy"}`; add `?deep=true` to check upstream |
| `/metrics` | GET | Request metrics - returns JSON counters; add `?reset=true` to reset |

## Features

### v0.0.4 - Feature Expansion + OpenWebUI Tools Integration

| Feature | Environment Variable | Default | Description |
|---------|---------------------|---------|-------------|
| **Content Filter Type** | `CONTENT_FILTER_TYPE` | `pruning` | Filter type: `pruning`, `bm25`, or `none` |
| **BM25 User Query** | `BM25_USER_QUERY` | `""` | Default query for BM25 content filter |
| **Min Word Threshold** | `MIN_WORD_THRESHOLD` | `10` | Minimum word threshold for pruning filter |
| **CSS Selector** | `CSS_SELECTOR` | `""` | Target specific elements for extraction |
| **Excluded Tags** | `EXCLUDED_TAGS` | `""` | Comma-separated HTML tags to exclude |
| **Target Elements** | `TARGET_ELEMENTS` | `""` | Comma-separated CSS selectors for targeted extraction |
| **JavaScript Code** | `JS_CODE` | `""` | JavaScript to execute on page (comma-separated) |
| **Wait For** | `WAIT_FOR` | `""` | CSS/XPath selector or JS to wait for before extraction |
| **Scan Full Page** | `SCAN_FULL_PAGE` | `false` | Scroll through entire page before extraction |
| **Scroll Delay** | `SCROLL_DELAY` | `0.0` | Delay between scrolls (seconds) |
| **Content Source** | `CONTENT_SOURCE` | `fit_html` | Source for markdown: `fit_html`, `raw_html`, or `cleaned_html` |
| **Stealth Mode** | `ENABLE_STEALTH` | `false` | Apply playwright-stealth anti-detection |
| **User Agent** | `USER_AGENT` | `""` | Custom browser User-Agent string |
| **Deep Crawl** | `DEEP_CRAWL` | `false` | Enable BFS deep crawl strategy |
| **Deep Crawl Max Depth** | `DEEP_CRAWL_MAX_DEPTH` | `1` | Maximum depth for deep crawl |
| **Deep Crawl Max Pages** | `DEEP_CRAWL_MAX_PAGES` | `10` | Maximum pages for deep crawl |
| **Connect Timeout** | `CRAWL_CONNECT_TIMEOUT_SECONDS` | `10` | Connection timeout to upstream |
| **Total Timeout** | `CRAWL_TOTAL_TIMEOUT_SECONDS` | `120` | Total request timeout to upstream |
| **Upstream Retries** | `UPSTREAM_RETRIES` | `0` | Retry count on 5xx upstream errors |
| **Upstream Retry Delay** | `UPSTREAM_RETRY_DELAY_MS` | `500` | Initial retry delay in ms (exponential backoff) |
| **Cache Enabled** | `CACHE_ENABLED` | `true` | Enable response caching |
| **Cache TTL** | `CACHE_TTL_SECONDS` | `300` | Cache entry time-to-live in seconds |
| **Cache Max Entries** | `CACHE_MAX_ENTRIES` | `500` | Maximum number of cached responses |
| **Rate Limit RPS** | `RATE_LIMIT_RPS` | `10` | Requests per second per client IP |
| **Rate Limit Burst** | `RATE_LIMIT_BURST` | `20` | Burst capacity per client IP |
| **Server Read Timeout** | `SERVER_READ_TIMEOUT_SECONDS` | `30` | HTTP server read timeout |
| **Server Write Timeout** | `SERVER_WRITE_TIMEOUT_SECONDS` | `180` | HTTP server write timeout |
| **Server Idle Timeout** | `SERVER_IDLE_TIMEOUT_SECONDS` | `120` | HTTP server idle timeout |
| **Log Format** | `LOG_FORMAT` | `text` | Log format: `text` or `json` |
| **Request ID** | (automatic) | - | `X-Request-ID` auto-generated and forwarded upstream |
| **Feature Flags** | `PROCESS_IFRAMES`, `ONLY_TEXT`, `CHECK_ROBOTS_TXT`, `VERIFY_SSL`, `TEXT_MODE`, `LIGHT_MODE`, `CAPTURE_NETWORK_REQUESTS`, `CAPTURE_CONSOLE_MESSAGES`, `PRESERVE_HTTPS_FOR_INTERNAL_LINKS` | see defaults | Various crawl4ai feature toggles |

### v0.0.3 - Hardening + Crawl4AI 0.8.6 Alignment

| Feature | Environment Variable | Default | Description |
|---------|---------------------|---------|-------------|
| **Downstream Auth Token** | `CRAWL4AI_AUTH_TOKEN` | `""` | Optional token for secured Crawl4AI instances |
| **Downstream Auth Scheme** | `CRAWL4AI_AUTH_SCHEME` | `Bearer` | Prefix used when building auth header value |
| **Downstream Auth Header** | `CRAWL4AI_AUTH_HEADER` | `Authorization` | Header name used for downstream auth |
| **Overlay Removal** | `REMOVE_OVERLAY_ELEMENTS` | `true` | Enables Crawl4AI overlay removal |
| **CSS Blocking** | `AVOID_CSS` | `false` | Blocks CSS resources for lighter/faster crawls |
| **Anti-Bot Retries** | `MAX_RETRIES` | `0` | Retry count for Crawl4AI anti-bot flow |
| **Proxy Pass-through** | `PROXY_CONFIG_JSON` | `""` | Raw JSON passed to `crawler_config.proxy_config` |
| **Batch Guard** | `MAX_URLS_PER_REQUEST` | `100` | Reject oversized URL batches early |
| **Body Size Guard** | `MAX_REQUEST_BODY_BYTES` | `1048576` | Reject overly large request bodies (1 MiB) |

### v0.0.2 - Crawl4AI 0.8.5 Feature Integration

| Feature | Environment Variable | Default | Description |
|---------|---------------------|---------|-------------|
| **Consent Popup Removal** | `REMOVE_CONSENT_POPUPS` | `true` | Auto-dismisses cookie/consent banners |
| **Shadow DOM Flattening** | `FLATTEN_SHADOW_DOM` | `true` | Extracts content hidden inside Shadow DOM |
| **Ad Blocking** | `AVOID_ADS` | `true` | Blocks ad trackers at the network level |
| **Memory Saving Mode** | `MEMORY_SAVING_MODE` | `true` | Aggressive cache/V8 heap flags |
| **Browser Recycling** | `MAX_PAGES_BEFORE_RECYCLE` | `100` | Auto-restart browser after N pages |
| **Content Pruning** | `PRUNING_THRESHOLD` | `0.48` | Boilerplate removal threshold |
| **References** | `INCLUDE_REFERENCES` | `true` | Append link references to markdown output |

## Per-Request Overrides

The `/crawl` endpoint accepts optional fields that override environment variable defaults:

```json
{
  "urls": ["https://example.com"],
  "content_filter_type": "bm25",
  "bm25_query": "climate change",
  "css_selector": "article",
  "excluded_tags": ["nav", "footer"],
  "target_elements": [".main-content"],
  "js_code": ["document.title"],
  "wait_for": ".content",
  "content_source": "raw_html",
  "enable_stealth": true,
  "user_agent": "MyBot/1.0",
  "deep_crawl": true,
  "deep_crawl_max_depth": 3,
  "deep_crawl_max_pages": 50
}
```

## Quick Start

### 1. Docker Compose

```yaml
services:
    crawl4ai-proxy:
        image: ghcr.io/byfebian/crawl4ai-proxy:0.0.4
        environment:
            - LISTEN_PORT=8000
            - CRAWL4AI_ENDPOINT=http://crawl4ai:11235/crawl
            - CONTENT_FILTER_TYPE=pruning
            - INCLUDE_REFERENCES=true
            - CACHE_ENABLED=true
            - CACHE_TTL_SECONDS=300
            - RATE_LIMIT_RPS=10
            - LOG_FORMAT=json
        networks:
            - openwebui

    openwebui:
        image: ghcr.io/open-webui/open-webui:ollama
        ports:
            - "8080:8080"
        deploy:
            resources:
                reservations:
                    devices:
                        - driver: nvidia
                          count: all
                          capabilities: [gpu]
        networks:
            - openwebui

    crawl4ai:
        image: unclecode/crawl4ai:0.8.6
        shm_size: 1g
        networks:
            - openwebui

networks:
    - openwebui
```

### 2. Start the services

```bash
docker compose up -d
```

> **Note:** Crawl4AI takes 2-3 minutes on first start to download Playwright browser binaries.

### 3. Configure OpenWebUI

Visit `localhost:8080` in a browser, navigate to **Admin Panel -> Web Search** and under the **Loader** section, set:

| Setting | Value |
|---------|-------|
| Web Loader Engine | `external` |
| External Web Loader URL | `http://crawl4ai-proxy:8000/crawl` |
| External Web Loader API Key | `*` (doesn't matter, but is a required field) |

### 4. (Optional) Add Crawl4AI Tools for UI Toggles

For Deep Research, Reading Mode, and Stealth Mode toggles in the chat UI:

1. Go to **Workspace → Tools → + Add**
2. Switch to the **Code** editor tab
3. Copy the contents of [`crawl4ai_tools.py`](crawl4ai_tools.py) and paste it in
4. Click **Save**
5. Enable the tool globally using the toggle in Admin Panel → Workspace → Tools

You'll get 5 controls in every chat:

| Toggle | Type | Options | What it does |
|--------|------|---------|-------------|
| **Deep Research** | ON/OFF | ON / OFF | Crawl linked pages for comprehensive coverage |
| **Research Depth** | Dropdown | Low / Medium / High | Link-follow depth. Low=1, Medium=3, High=5 levels |
| **Max Pages** | Number | Any number | Max pages to crawl when Deep Research is ON (default 10) |
| **Reading Mode** | Dropdown | Best / Focused / All | Best=pruning, Focused=bm25, All=no filter |
| **Stealth Mode** | ON/OFF | ON / OFF | Bypass bot detection on protected sites |

## Configuration

All features are configurable via environment variables. Defaults are optimized for OpenWebUI use.

### Crawl Features

```yaml
environment:
    - CONTENT_FILTER_TYPE=pruning
    - BM25_USER_QUERY=
    - MIN_WORD_THRESHOLD=10
    - CSS_SELECTOR=
    - EXCLUDED_TAGS=
    - TARGET_ELEMENTS=
    - JS_CODE=
    - WAIT_FOR=
    - SCAN_FULL_PAGE=false
    - SCROLL_DELAY=0.0
    - CONTENT_SOURCE=fit_html
    - ENABLE_STEALTH=false
    - USER_AGENT=
    - DEEP_CRAWL=false
    - DEEP_CRAWL_MAX_DEPTH=1
    - DEEP_CRAWL_MAX_PAGES=10
```

### Feature Flags

```yaml
environment:
    - REMOVE_CONSENT_POPUPS=true
    - FLATTEN_SHADOW_DOM=true
    - REMOVE_OVERLAY_ELEMENTS=true
    - AVOID_ADS=true
    - AVOID_CSS=false
    - MEMORY_SAVING_MODE=true
    - MAX_PAGES_BEFORE_RECYCLE=100
    - PROCESS_IFRAMES=false
    - ONLY_TEXT=false
    - CHECK_ROBOTS_TXT=false
    - VERIFY_SSL=true
    - TEXT_MODE=false
    - LIGHT_MODE=false
    - CAPTURE_NETWORK_REQUESTS=false
    - CAPTURE_CONSOLE_MESSAGES=false
    - PRESERVE_HTTPS_FOR_INTERNAL_LINKS=false
```

### Resilience & Performance

```yaml
environment:
    - CRAWL_CONNECT_TIMEOUT_SECONDS=10
    - CRAWL_TOTAL_TIMEOUT_SECONDS=120
    - UPSTREAM_RETRIES=0
    - UPSTREAM_RETRY_DELAY_MS=500
    - CACHE_ENABLED=true
    - CACHE_TTL_SECONDS=300
    - CACHE_MAX_ENTRIES=500
    - RATE_LIMIT_RPS=10
    - RATE_LIMIT_BURST=20
    - SERVER_READ_TIMEOUT_SECONDS=30
    - SERVER_WRITE_TIMEOUT_SECONDS=180
    - SERVER_IDLE_TIMEOUT_SECONDS=120
```

### Tuning Parameters

```yaml
environment:
    - PRUNING_THRESHOLD=0.48
    - MAX_URLS_PER_REQUEST=100
    - MAX_REQUEST_BODY_BYTES=1048576
    - CRAWL_TIMEOUT_SECONDS=120
```

### Optional: Secured Crawl4AI (JWT/Bearer)

```yaml
environment:
    - CRAWL4AI_AUTH_TOKEN=your-jwt-or-token
    - CRAWL4AI_AUTH_SCHEME=Bearer
    - CRAWL4AI_AUTH_HEADER=Authorization
```

### Optional: Advanced Proxy Rotation

```yaml
environment:
    - PROXY_CONFIG_JSON=["direct","http://my-proxy:8080"]
```

### Optional: Logging

```yaml
environment:
    - LOG_FORMAT=json
```

Set to `json` for structured JSON logs (better for container environments), `text` for human-readable logs.

## API Examples

### Crawl URLs

```bash
curl -X POST http://crawl4ai-proxy:8000/crawl \
  -H "Content-Type: application/json" \
  -d '{"urls":["https://example.com"]}'
```

### Crawl with BM25 query

```bash
curl -X POST http://crawl4ai-proxy:8000/crawl \
  -H "Content-Type: application/json" \
  -d '{"urls":["https://example.com"],"content_filter_type":"bm25","bm25_query":"artificial intelligence"}'
```

### Get Markdown Only

```bash
curl -X POST http://crawl4ai-proxy:8000/md \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com","filter_type":"bm25","bm25_query":"climate change"}'
```

### Take Screenshot

```bash
curl -X POST http://crawl4ai-proxy:8000/screenshot \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}'
```

### Execute JavaScript

```bash
curl -X POST http://crawl4ai-proxy:8000/execute_js \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com","scripts":["document.title"]}'
```

### Health Check

```bash
curl http://crawl4ai-proxy:8000/health
# {"status":"healthy"}

# Deep health check (tests upstream connectivity)
curl http://crawl4ai-proxy:8000/health?deep=true
# {"status":"healthy","upstream":"reachable"}
```

### Metrics

```bash
curl http://crawl4ai-proxy:8000/metrics
# {"total_requests":42,"total_errors":1,...}

# Reset counters after reading
curl http://crawl4ai-proxy:8000/metrics?reset=true
```

## Version History

| Version | Date | Changes |
|---------|------|---------|
| v0.0.4 | Apr 2026 | Feature expansion, code refactored into multi-file layout, add crawl4ai_tools.py to integrate with OpenWebUI tools |
| v0.0.3 | Apr 2026 | Crawl4AI 0.8.6 alignment + proxy hardening |
| v0.0.2 | Apr 2026 | Crawl4AI 0.8.5 feature integration |
| v0.0.1 | Apr 2026 | Crawl4AI 0.8.x compatibility |

## Credits

- Original proxy by [lennyerik](https://github.com/lennyerik/crawl4ai-proxy/) (Crawl4AI v0.6.x)
- [Crawl4AI](https://github.com/unclecode/crawl4ai) by [unclecode](https://github.com/unclecode)
- [OpenWebUI](https://github.com/open-webui/open-webui)
