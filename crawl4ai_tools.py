"""
title: Deep Research
author: byfebian
version: 0.0.1
license: MIT
description: Crawl4AI Tools — OpenWebUI Integration (requires OpenWebUI 0.9.x+)

======================================

This file provides an OpenWebUI Tool that connects your OpenWebUI
instance to crawl4ai-proxy with user-friendly toggles for Deep Research,
Research Depth, Max Pages, Reading Mode, and Stealth Mode.

Installation
------------
1. Open your OpenWebUI instance
2. Go to  Workspace → Tools → + Add (or click the + icon in Tools)
3. Switch to the "Code" editor tab
4. Paste this entire file contents
5. Click Save

Configuration
-------------
After saving, click the gear icon on the tool to configure:

Valves (Admin-visible, apply to all chats):
  - Deep Research  : ON/OFF — crawl linked pages for comprehensive coverage
  - Research Depth : Low / Medium / High — link-follow depth (1 / 3 / 5 levels)
  - Max Pages      : Number — max pages to crawl when Deep Research is ON (default 10)
  - Reading Mode   : Best / Focused / All — how content is extracted
  - Stealth Mode   : ON/OFF — bypass bot detection on protected sites

How it works
------------
When the AI decides to fetch a URL, it calls this tool instead of the
built-in web loader. The tool forwards the request to crawl4ai-proxy
with the Valve settings applied.

Reading Mode details:
  Best    → PruningContentFilter (removes boilerplate, keeps quality content)
  Focused → BM25ContentFilter   (extracts only content matching the user's question)
  All     → No filter           (returns the full page unfiltered)

When "Focused" is selected, the user's last chat message is automatically
used as the BM25 search query.

The proxy URL is set via the CRAWL4AI_PROXY_URL environment variable.
Default: http://crawl4ai-proxy:8000 (matches docker-compose service name).
"""

import json
import os
from typing import Optional

try:
    import httpx

    _USE_HTTPX = True
except ImportError:
    _USE_HTTPX = False

from pydantic import BaseModel, Field


class Tools:
    class Valves(BaseModel):
        deep_research: bool = Field(
            default=False,
            description="Crawl linked pages for comprehensive coverage",
        )
        research_depth: str = Field(
            default="Medium",
            description="Link-follow depth. Low=1, Medium=3, High=5 levels",
        )
        max_pages: int = Field(
            default=10,
            description="Max pages to crawl when Deep Research is ON",
        )
        reading_mode: str = Field(
            default="Best",
            description="Best=pruning, Focused=bm25, All=no filter",
        )
        stealth_mode: bool = Field(
            default=False,
            description="Bypass bot detection on protected sites",
        )

    PROXY_URL = os.environ.get("CRAWL4AI_PROXY_URL", "http://crawl4ai-proxy:8000")

    def __init__(self):
        self.valves = self.Valves()

    async def crawl_web(self, url: str, __event_emitter__: Optional[callable] = None) -> str:
        """
        Crawl a web page and extract its content. Use this tool when you need to
        read or summarize content from a URL. Supports Deep Research for
        comprehensive multi-page crawling and different Reading Modes for content
        extraction quality.

        :param url: The URL to crawl and extract content from.
        :return: Extracted text content from the page.
        """
        if not url or not url.strip():
            return "Error: No URL provided."

        url = url.strip()
        if not url.startswith(("http://", "https://")):
            return f"Error: Invalid URL: {url}. Must start with http:// or https://."

        payload = self._build_payload(url)

        try:
            result = await self._call_proxy(payload)
        except Exception as e:
            return f"Error fetching {url}: {str(e)}"

        if not result:
            return f"No content could be extracted from {url}."

        return result

    def _map_reading_mode(self, reading_mode: str) -> str:
        """Map user-friendly Reading Mode to proxy content_filter_type."""
        mapping = {
            "best": "pruning",
            "focused": "bm25",
            "all": "none",
        }
        return mapping.get(reading_mode.lower(), "pruning")

    def _map_research_depth(self, depth: str) -> int:
        """Map user-friendly Research Depth to max_depth integer."""
        mapping = {
            "low": 1,
            "medium": 3,
            "high": 5,
        }
        return mapping.get(depth.lower(), 3)

    def _build_payload(self, url: str) -> dict:
        """
        Build the JSON payload for crawl4ai-proxy's /crawl endpoint,
        translating Valve values into per-request override fields.
        """
        payload = {
            "urls": [url],
        }

        # Deep Research
        if self.valves.deep_research:
            depth = self.valves.research_depth.lower()
            payload["deep_crawl"] = True
            payload["deep_crawl_max_depth"] = self._map_research_depth(depth)
            payload["deep_crawl_max_pages"] = self.valves.max_pages

        # Reading Mode
        content_filter_type = self._map_reading_mode(self.valves.reading_mode)
        payload["content_filter_type"] = content_filter_type

        # Stealth Mode
        if self.valves.stealth_mode:
            payload["enable_stealth"] = True

        return payload

    async def _call_proxy(self, payload: dict) -> Optional[str]:
        """
        Send the request to crawl4ai-proxy and return extracted content.
        Uses httpx AsyncClient (required for async event loop compatibility).
        """
        if not _USE_HTTPX:
            raise Exception(
                "httpx is required for async HTTP in Open WebUI 0.9.0+. "
                "Please install httpx: pip install httpx"
            )

        proxy_url = self.PROXY_URL.rstrip("/")
        endpoint = f"{proxy_url}/crawl"

        try:
            async with httpx.AsyncClient(timeout=300.0) as client:
                response = await client.post(
                    endpoint,
                    json=payload,
                    headers={"Content-Type": "application/json"},
                )
                response.raise_for_status()
                data = response.json()
        except httpx.TimeoutException:
            raise Exception("Request to crawl proxy timed out after 300 seconds")
        except httpx.HTTPStatusError as e:
            detail = ""
            try:
                error_body = e.response.json()
                detail = error_body.get("detail", error_body.get("error", ""))
            except Exception:
                detail = str(e.response.text[:200]) if e.response else ""
            raise Exception(
                f"Crawl proxy returned HTTP {e.response.status_code}: {detail}"
            )
        except httpx.ConnectError:
            raise Exception(
                f"Cannot connect to crawl proxy at {proxy_url}. "
                f"Make sure crawl4ai-proxy is running and the URL is correct."
            )

        return self._parse_response(data)

    def _parse_response(self, data) -> Optional[str]:
        """
        Parse the proxy response and return combined content.
        Response format: [{"page_content": "...", "metadata": {...}}, ...]
        """
        if not isinstance(data, list) or len(data) == 0:
            return None

        # Combine content from all results (important for deep crawl which returns multiple pages)
        contents = []
        for item in data:
            if isinstance(item, dict):
                content = item.get("page_content", "")
                source = item.get("metadata", {}).get("source", "")
                if content and content.strip():
                    if source and len(data) > 1:
                        contents.append(f"---\nSource: {source}\n\n{content.strip()}")
                    else:
                        contents.append(content.strip())

        if not contents:
            return None

        return "\n\n".join(contents)
