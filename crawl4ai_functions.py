"""
Crawl4AI Functions — OpenWebUI Integration
==========================================

This file provides an OpenWebUI Function (Pipe) that connects your OpenWebUI
instance to crawl4ai-proxy with user-friendly toggles for Deep Research,
Reading Mode, Research Depth, and Stealth Mode.

Installation
------------
1. Open your OpenWebUI instance
2. Go to  Workspace → Functions → + Add
3. Switch to the "Code" editor tab
4. Paste this entire file contents
5. Click Save
6. The toggles will appear in your chat UI automatically

Configuration
-------------
After saving, click the gear icon on the function in the Functions list to
configure the proxy URL. The default is http://crawl4ai-proxy:8000 which
matches the docker-compose service name.

Valves (User Toggles)
---------------------
- Deep Research  : ON/OFF — crawl linked pages for comprehensive coverage
- Research Depth  : Low / Medium / High — how deep to follow links (1/3/5)
- Reading Mode   : Best / Focused / All — how content is extracted
- Stealth Mode   : ON/OFF — bypass bot detection on protected sites

Reading Mode details:
  Best    → PruningContentFilter (removes boilerplate, keeps quality content)
  Focused → BM25ContentFilter   (extracts only content matching the user's query)
  All     → No filter           (returns the full page unfiltered)

When "Focused" is selected, the user's chat message is automatically used as
the BM25 search query.
"""

import json
import os
from typing import Optional

try:
    import httpx
    _USE_HTTPX = True
except ImportError:
    import urllib.request
    import urllib.error
    _USE_HTTPX = False

from pydantic import BaseModel, Field


class Pipe:
    class Valves(BaseModel):
        deep_research: bool = Field(
            default=False,
            description="Crawl linked pages for more comprehensive information",
        )
        research_depth: str = Field(
            default="Medium",
            description="Low, Medium, or High — how deep to follow links when Deep Research is on",
        )
        reading_mode: str = Field(
            default="Best",
            description="Best, Focused, or All — how content is extracted from pages",
        )
        stealth_mode: bool = Field(
            default=False,
            description="Bypass bot detection on protected websites",
        )

    # Proxy URL — set via CRAWL4AI_PROXY_URL env var, defaults to docker-compose service name
    PROXY_URL = os.environ.get("CRAWL4AI_PROXY_URL", "http://crawl4ai-proxy:8000")

    def __init__(self):
        self.valves = self.Valves()

    def pipe(
        self,
        user_message: str,
        model: str,
        messages: list,
        body: dict,
        __user__: Optional[dict] = None,
        __event_emitter__: Optional[callable] = None,
    ) -> Optional[str]:
        """
        Called by OpenWebUI when the AI decides to fetch a URL.

        The URL comes from the function call arguments that OpenWebUI passes
        through body["tool_calls"] or body["function_call"].

        Returns extracted content as a string, or None if no URL found.
        """
        url = self._extract_url(body)
        if not url:
            return None

        query = self._extract_user_query(messages)

        payload = self._build_payload(url, query)

        try:
            result = self._call_proxy(payload)
        except Exception as e:
            return f"Error fetching {url}: {str(e)}"

        if not result:
            return f"No content could be extracted from {url}."

        return result

    def _extract_url(self, body: dict) -> Optional[str]:
        """
        Extract URL from OpenWebUI function call arguments.

        OpenWebUI passes function calls in different formats depending on the
        model and version. We check all known locations.
        """
        # Format 1: tool_calls in body (newer OpenWebUI)
        tool_calls = body.get("tool_calls", [])
        for tc in tool_calls:
            if isinstance(tc, dict):
                fn = tc.get("function", {})
                args = fn.get("arguments", {})
                if isinstance(args, str):
                    try:
                        args = json.loads(args)
                    except (json.JSONDecodeError, TypeError):
                        continue
                url = args.get("url") or args.get("href") or args.get("link")
                if url:
                    return url.strip()

        # Format 2: function_call in body (older OpenWebUI / some models)
        fc = body.get("function_call", {})
        if isinstance(fc, dict):
            args = fc.get("arguments", {})
            if isinstance(args, str):
                try:
                    args = json.loads(args)
                except (json.JSONDecodeError, TypeError):
                    args = {}
            url = args.get("url") or args.get("href") or args.get("link")
            if url:
                return url.strip()

        # Format 3: direct parameters (simplest case)
        url = body.get("url") or body.get("href") or body.get("link")
        if url:
            return url.strip()

        return None

    def _extract_user_query(self, messages: list) -> str:
        """
        Extract the user's most recent message text for use as a BM25 query.
        """
        if not messages:
            return ""
        for msg in reversed(messages):
            if isinstance(msg, dict) and msg.get("role") == "user":
                content = msg.get("content", "")
                if isinstance(content, str) and content.strip():
                    return content.strip()[:500]
                elif isinstance(content, list):
                    for part in content:
                        if isinstance(part, dict) and part.get("type") == "text":
                            text = part.get("text", "").strip()
                            if text:
                                return text[:500]
        return ""

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

    def _build_payload(self, url: str, query: str) -> dict:
        """
        Build the JSON payload for crawl4ai-proxy's /crawl endpoint,
        translating Valve values into per-request override fields.
        """
        payload = {
            "urls": [url],
        }

        # Deep Research
        if self.valves.deep_research:
            payload["deep_crawl"] = True
            payload["deep_crawl_max_depth"] = self._map_research_depth(
                self.valves.research_depth
            )
            payload["deep_crawl_max_pages"] = 20

        # Reading Mode
        content_filter_type = self._map_reading_mode(self.valves.reading_mode)
        payload["content_filter_type"] = content_filter_type
        if content_filter_type == "bm25" and query:
            payload["bm25_query"] = query

        # Stealth Mode
        if self.valves.stealth_mode:
            payload["enable_stealth"] = True

        return payload

    def _call_proxy(self, payload: dict) -> Optional[str]:
        """
        Send the request to crawl4ai-proxy and return extracted content.
        Uses httpx if available, falls back to urllib (stdlib).
        """
        proxy_url = self.PROXY_URL.rstrip("/")
        endpoint = f"{proxy_url}/crawl"
        body = json.dumps(payload).encode("utf-8")

        if _USE_HTTPX:
            return self._call_proxy_httpx(endpoint, body, proxy_url)
        else:
            return self._call_proxy_urllib(endpoint, body, proxy_url)

    def _call_proxy_httpx(self, endpoint, body, proxy_url):
        """Call proxy using httpx."""
        try:
            with httpx.Client(timeout=300.0) as client:
                response = client.post(
                    endpoint,
                    content=body,
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

    def _call_proxy_urllib(self, endpoint, body, proxy_url):
        """Call proxy using urllib (stdlib fallback)."""
        req = urllib.request.Request(
            endpoint,
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=300) as resp:
                data = json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            detail = ""
            try:
                error_body = json.loads(e.read().decode("utf-8"))
                detail = error_body.get("detail", error_body.get("error", ""))
            except Exception:
                detail = str(e.read()[:200]) if e.fp else ""
            raise Exception(f"Crawl proxy returned HTTP {e.code}: {detail}")
        except urllib.error.URLError as e:
            raise Exception(
                f"Cannot connect to crawl proxy at {proxy_url}. "
                f"Make sure crawl4ai-proxy is running and the URL is correct. "
                f"Error: {e.reason}"
            )
        except TimeoutError:
            raise Exception("Request to crawl proxy timed out after 300 seconds")
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