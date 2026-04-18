# Crawl4AI OpenWebUI Proxy (for Crawl4AI 0.8.x)
This simple proxy server can be run in a docker container to let an [OpenWebUI](https://github.com/open-webui/open-webui) instance interact with a [crawl4ai](https://github.com/unclecode/crawl4ai) instance. Now compatible with the latest Crawl4AI 0.8.x version.
This makes the OpenWebUI's web search feature a lot faster and way more usable without paying for an API service. 🎉

## Usage

> **Note:** This fork is updated for compatibility with Crawl4AI v0.8.x.
> The [original repo](https://github.com/lennyerik/crawl4ai-proxy/) was built for Crawl4AI v0.6.x and is incompatible with Crawl4AI v0.8.x.

Given a `docker-compose.yml` file that looks something like this:

```yaml
services:
    crawl4ai-proxy:
        image: ghcr.io/byfebian/crawl4ai-proxy:latest
        environment:
            - LISTEN_PORT=8000
            - CRAWL4AI_ENDPOINT=http://crawl4ai:11235/crawl
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
        image: unclecode/crawl4ai:0.8.5
        shm_size: 1g
        networks:
            - openwebui
networks:
    - openwebui
```

Run docker compose up -d, visit localhost:8080 in a browser, navigate to Admin Panel → Web Search and under the "Loader" section, set:

    Web Loader Engine: external
    External Web Loader URL: http://crawl4ai-proxy:8000/crawl
    External Web Loader API Key: * (doesn't matter, but is a required field)
