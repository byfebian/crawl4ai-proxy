# Crawl4AI OpenWebUI Proxy (for Crawl4AI 0.8.x)

A lightweight proxy server that lets [OpenWebUI](https://github.com/open-webui/open-webui) interact with [Crawl4AI](https://github.com/unclecode/crawl4ai), making web search faster and more usable without paying for an API service.

Forked from [lennyerik/crawl4ai-proxy](https://github.com/lennyerik/crawl4ai-proxy/) and updated for Crawl4AI **0.8.x** (tested against **0.8.6**). Compatible with Open WebUI **0.8.x** and **0.9.x**.

**Full docs:** [`WIKI`](https://github.com/byfebian/crawl4ai-proxy/wiki) — configuration reference, API examples, profiles, troubleshooting, and changelog.

## What This Proxy Does

OpenWebUI sends `{"urls": [...]}`. Crawl4AI expects a richer format and returns a complex response. This proxy:

1. Receives `{"urls": [...]}` from OpenWebUI
2. Enriches the request for Crawl4AI 0.8.x
3. Forwards to Crawl4AI and converts the response back
4. Prefers `fit_markdown` (pruned, high-quality content) over `raw_markdown`

Also ships an **OpenWebUI Tool** (`crawl4ai_tools.py`) for Deep Research, Reading Mode, and Stealth Mode toggles in the chat UI.

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/crawl` | POST | Main proxy endpoint — accepts `{"urls": [...]}` from OpenWebUI |
| `/md` | POST | Markdown-only extraction |
| `/screenshot` | POST | Screenshot capture |
| `/execute_js` | POST | Execute JavaScript on a page |
| `/health` | GET | Health check (`?deep=true` checks upstream) |
| `/metrics` | GET | Request metrics (`?reset=true` resets counters) |

See [`API Reference`](https://github.com/byfebian/crawl4ai-proxy/wiki/API-Reference) for full details and examples.

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
        networks:
            - openwebui

    crawl4ai:
        image: unclecode/crawl4ai:0.8.6
        shm_size: 1g
        networks:
            - openwebui

networks:
    openwebui:
```

### 2. Start the services

```bash
docker compose up -d
```

> **Note:** Crawl4AI takes 2-3 minutes on first start to download Playwright browser binaries.

### 3. Configure OpenWebUI

Go to **Admin Panel → Web Search** and under **Loader** set:

| Setting | Value |
|---------|-------|
| Web Loader Engine | `external` |
| External Web Loader URL | `http://crawl4ai-proxy:8000/crawl` |
| External Web Loader API Key | `*` (required field, any value works) |

### 4. (Optional) Add Crawl4AI Tools for UI Toggles

1. Go to **Workspace → Tools → + Add**
2. Switch to the **Code** editor tab
3. Copy [`crawl4ai_tools.py`](crawl4ai_tools.py) and paste it in  
   *(Requires OpenWebUI 0.9.x+; for OpenWebUI 0.8.x, use the legacy [`crawl4ai_tools_0.8.x.py`](crawl4ai_tools_0.8.x.py) instead)*
4. Click **Save**, then enable the tool globally in **Admin Panel → Workspace → Tools**

You'll get 5 controls in every chat:

| Toggle | Options | What it does |
|--------|---------|-------------|
| **Deep Research** | ON / OFF | Crawl linked pages for broader coverage |
| **Research Depth** | Low / Medium / High | Link-follow depth (1 / 3 / 5 levels) |
| **Max Pages** | Any number | Max pages to crawl (default 10) |
| **Reading Mode** | Best / Focused / All | Best=pruning, Focused=bm25, All=no filter |
| **Stealth Mode** | ON / OFF | Bypass bot detection on protected sites |

## Configuration

All features are configurable via environment variables. Defaults are optimized for OpenWebUI.

See the [`Configuration Reference`](https://github.com/byfebian/crawl4ai-proxy/wiki#configuration-reference) for every variable, defaults, accepted values, and recommended settings.

For ready-made docker-compose profiles (General Purpose, Research, Anti-Bot, Memory-Constrained, Deep Crawl, Maximum Speed), see [`Recommended Configurations`](https://github.com/byfebian/crawl4ai-proxy/wiki#recommended-configurations-for-openwebui).

For troubleshooting, see [`Symptom Table`](https://github.com/byfebian/crawl4ai-proxy/wiki#quick-reference-symptom-table).

## Credits

- Original proxy by [lennyerik](https://github.com/lennyerik/crawl4ai-proxy/) (Crawl4AI v0.6.x)
- [Crawl4AI](https://github.com/unclecode/crawl4ai) by [unclecode](https://github.com/unclecode)
- [OpenWebUI](https://github.com/open-webui/open-webui)
