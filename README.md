# Crawl4AI OpenWebUI Proxy (for Crawl4AI 0.8.x)

A lightweight proxy server that lets an [OpenWebUI](https://github.com/open-webui/open-webui) instance interact with a [Crawl4AI](https://github.com/unclecode/crawl4ai) instance, making OpenWebUI's web search feature faster and more usable without paying for an API service. 🎉

Forked from [lennyerik/crawl4ai-proxy](https://github.com/lennyerik/crawl4ai-proxy/) and updated for compatibility with Crawl4AI **0.8.x** (tested against **0.8.6**).

## What This Proxy Does

OpenWebUI's External Web Loader sends a simple `{"urls": [...]}` request. Crawl4AI's Docker API expects a richer request format and returns a complex response. This proxy sits between them and:

1. Receives `{"urls": [...]}` from OpenWebUI
2. Enriches the request with Crawl4AI 0.8.6-ready features (consent popup removal, shadow DOM flattening, ad/CSS blocking, content pruning, retries, proxy pass-through)
3. Forwards the enriched request to Crawl4AI
4. Converts Crawl4AI's response back into OpenWebUI's expected format
5. Prefers `fit_markdown` (pruned, high-quality content) over `raw_markdown`

## Features

### v0.0.3 — Hardening + Crawl4AI 0.8.6 Alignment

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
| **Body Size Guard** | `MAX_REQUEST_BODY_BYTES` | `1048576` | Reject overly large request bodies |
| **Strict Body Limit Enforcement** | `MAX_REQUEST_BODY_BYTES` | `1048576` | Enforced with `http.MaxBytesReader`; oversized payloads now return `413` and cannot bypass limits via trailing bytes |
| **HTTP Robustness** | — | — | Accepts `application/json; charset=utf-8`, avoids panics, closes downstream body, returns clearer gateway errors |

### v0.0.2 — Crawl4AI 0.8.5 Feature Integration

| Feature | Environment Variable | Default | Description |
|---------|---------------------|---------|-------------|
| **Consent Popup Removal** | `REMOVE_CONSENT_POPUPS` | `true` | Auto-dismisses cookie/consent banners from 40+ CMP platforms (OneTrust, Cookiebot, Didomi, etc.) |
| **Shadow DOM Flattening** | `FLATTEN_SHADOW_DOM` | `true` | Extracts content hidden inside Shadow DOM components (modern web apps) |
| **Ad Blocking** | `AVOID_ADS` | `true` | Blocks ad trackers at the network level for faster, cleaner crawls |
| **Memory Saving Mode** | `MEMORY_SAVING_MODE` | `true` | Aggressive cache/V8 heap flags to prevent memory leaks during long sessions |
| **Browser Recycling** | `MAX_PAGES_BEFORE_RECYCLE` | `100` | Auto-restarts the browser after N pages to prevent memory leaks |
| **Content Pruning** | `PRUNING_THRESHOLD` | `0.48` | Removes boilerplate (nav bars, footers, sidebars) using PruningContentFilter. Lower = more aggressive, higher = keep more content |
| **References** | `INCLUDE_REFERENCES` | `true` | Appends link references/citations to markdown output for better source attribution |
| **Request Timeout** | `CRAWL_TIMEOUT_SECONDS` | `120` | Prevents hung requests from blocking forever |
| **Health Check** | — | — | `/health` endpoint for monitoring |

### v0.0.1 — Crawl4AI 0.8.x Compatibility

- Updated response struct for Crawl4AI 0.8.x `MarkdownGenerationResult` format
- Changed metadata type from `map[string]string` to `map[string]interface{}`
- Added support for `fit_markdown` (higher quality filtered content)
- Handle both single result and results array responses

## Quick Start

### 1. Docker Compose

```yaml
services:
    crawl4ai-proxy:
        image: ghcr.io/byfebian/crawl4ai-proxy:0.0.3
        environment:
            - LISTEN_PORT=8000
            - CRAWL4AI_ENDPOINT=http://crawl4ai:11235/crawl
            # Optional: uncomment to change defaults
            # - REMOVE_CONSENT_POPUPS=true
            # - FLATTEN_SHADOW_DOM=true
            # - REMOVE_OVERLAY_ELEMENTS=true
            # - AVOID_ADS=true
            # - AVOID_CSS=false
            # - MEMORY_SAVING_MODE=true
            # - MAX_PAGES_BEFORE_RECYCLE=100
            # - PRUNING_THRESHOLD=0.48
            # - INCLUDE_REFERENCES=true
            # - MAX_RETRIES=0
            # - PROXY_CONFIG_JSON=["direct","http://my-proxy:8080"]
            # - MAX_URLS_PER_REQUEST=100
            # - MAX_REQUEST_BODY_BYTES=1048576
            # - CRAWL_TIMEOUT_SECONDS=120
            # Optional downstream auth:
            # - CRAWL4AI_AUTH_TOKEN=your-jwt-or-token
            # - CRAWL4AI_AUTH_SCHEME=Bearer
            # - CRAWL4AI_AUTH_HEADER=Authorization
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

Visit `localhost:8080` in a browser, navigate to **Admin Panel → Web Search** and under the **Loader** section, set:

| Setting | Value |
|---------|-------|
| Web Loader Engine | `external` |
| External Web Loader URL | `http://crawl4ai-proxy:8000/crawl` |
| External Web Loader API Key | `*` (doesn't matter, but is a required field) |

## Configuration

All features are configurable via environment variables. Defaults are optimized for OpenWebUI use — most users don't need to change anything.

### Feature Flags

Set to `true` to enable, `false` to disable:

```yaml
environment:
    # Auto-dismiss cookie/consent banners (OneTrust, Cookiebot, Didomi, etc.)
    - REMOVE_CONSENT_POPUPS=true

    # Extract content from Shadow DOM components
    - FLATTEN_SHADOW_DOM=true

    # Block ad trackers at the network level
    - AVOID_ADS=true

    # Block CSS resources at the network level
    - AVOID_CSS=false

    # Remove sticky overlays/popups that block content
    - REMOVE_OVERLAY_ELEMENTS=true

    # Prevent memory leaks in long-running sessions
    - MEMORY_SAVING_MODE=true

    # Append link references/citations to output
    - INCLUDE_REFERENCES=true

    # Crawl4AI anti-bot retry count
    - MAX_RETRIES=0
```

### Tuning Parameters

```yaml
environment:
    # How aggressively to remove boilerplate.
    # 0.3 = very aggressive (may remove too much)
    # 0.48 = balanced (default)
    # 0.7 = keep most content
    - PRUNING_THRESHOLD=0.48

    # Auto-restart browser after N pages (prevents memory leaks)
    - MAX_PAGES_BEFORE_RECYCLE=100

    # Reject requests with too many URLs
    - MAX_URLS_PER_REQUEST=100

    # Reject oversized request bodies (bytes)
    - MAX_REQUEST_BODY_BYTES=1048576

    # How long to wait for Crawl4AI before giving up (seconds)
    - CRAWL_TIMEOUT_SECONDS=120
```

### Optional: Secured Crawl4AI (JWT/Bearer)

If your Crawl4AI server requires authentication, configure proxy-to-Crawl4AI auth forwarding:

```yaml
environment:
    - CRAWL4AI_AUTH_TOKEN=your-jwt-or-token
    - CRAWL4AI_AUTH_SCHEME=Bearer
    - CRAWL4AI_AUTH_HEADER=Authorization
```

### Optional: Advanced Proxy Rotation Pass-through

For advanced proxy config accepted by Crawl4AI, pass raw JSON via:

```yaml
environment:
    - PROXY_CONFIG_JSON=["direct","http://my-proxy:8080"]
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/crawl` | POST | Main proxy endpoint — accepts `{"urls": [...]}` from OpenWebUI |
| `/health` | GET | Health check — returns `{"status":"healthy"}` |

### Example: Crawl a URL

```bash
curl -X POST http://crawl4ai-proxy:8000/crawl \
  -H "Content-Type: application/json" \
  -d '{"urls":["https://example.com"]}'
```

### Example: Health Check

```bash
curl http://crawl4ai-proxy:8000/health
# {"status":"healthy"}
```

## Version History

| Version | Date | Changes |
|---------|------|---------|
| v0.0.3 | Apr 2026 | Crawl4AI 0.8.6 alignment + proxy hardening: downstream auth forwarding, stricter request validation, strict `MAX_REQUEST_BODY_BYTES` enforcement with `413` on oversized bodies, content-type compatibility (`application/json; charset=utf-8`), safer upstream error handling, `avoid_css`, `remove_overlay_elements`, `max_retries`, `proxy_config` pass-through, larger test coverage, Docker/CI improvements |
| v0.0.2 | Apr 2026 | Crawl4AI 0.8.5 feature integration: consent popup removal, shadow DOM flattening, ad blocking, PruningContentFilter, memory saving mode, browser recycling, request timeout, health check endpoint, references/citations in output, configurable environment variables |
| v0.0.1 | Apr 2026 | Crawl4AI 0.8.x compatibility: updated response struct, fit_markdown support, metadata type fix |

## Credits

- Original proxy by [lennyerik](https://github.com/lennyerik/crawl4ai-proxy/) (Crawl4AI v0.6.x)
- [Crawl4AI](https://github.com/unclecode/crawl4ai) by [unclecode](https://github.com/unclecode)
- [OpenWebUI](https://github.com/open-webui/open-webui)
