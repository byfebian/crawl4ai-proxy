package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "strconv"
    "time"
)

// =============================================================================
// CONFIGURATION — All tunable via environment variables
// =============================================================================
var (
    LISTEN_IP   string  = ""
    LISTEN_PORT int     = 8000

    // Crawl4AI backend endpoint (e.g. http://crawl4ai:11235/crawl)
    CRAWL4AI_ENDPOINT string = "http://crawl4ai:11235/crawl"

    // How long to wait for Crawl4AI before giving up (seconds)
    CRAWL_TIMEOUT_SECONDS int = 120

    // --- Feature flags (set to "false" env var to disable) ---

    // REMOVE_CONSENT_POPUPS: Auto-dismiss cookie/consent banners
    REMOVE_CONSENT_POPUPS bool = true

    // FLATTEN_SHADOW_DOM: Extract content hidden inside Shadow DOM components.
    FLATTEN_SHADOW_DOM bool = true

    // AVOID_ADS: Block ad trackers at the network level for faster, cleaner crawls.
    AVOID_ADS bool = true

    // MEMORY_SAVING_MODE: Aggressive cache/V8 heap flags to prevent memory leaks
    // during long-running sessions.
    MEMORY_SAVING_MODE bool = true

    // MAX_PAGES_BEFORE_RECYCLE: Auto-restart the browser after this many pages
    // to prevent memory leaks. Only relevant for long-running sessions.
    MAX_PAGES_BEFORE_RECYCLE int = 100

    // PRUNING_THRESHOLD: Controls how aggressively the PruningContentFilter removes
    // boilerplate (nav bars, footers, sidebars). Lower = more aggressive removal.
    // 0.3 = very aggressive, 0.48 = balanced, 0.7 = keep most content.
    PRUNING_THRESHOLD float64 = 0.48

    // INCLUDE_REFERENCES: Append link references/citations to the markdown output.
    // Gives OpenWebUI better source attribution.
    INCLUDE_REFERENCES bool = true
)

// =============================================================================
// ReadEnvironment — Reads env vars and overrides defaults
// =============================================================================
func ReadEnvironment() {
    if v := os.Getenv("LISTEN_IP"); v != "" {
        LISTEN_IP = v
    }
    if v := os.Getenv("LISTEN_PORT"); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            LISTEN_PORT = i
        }
    }
    if v := os.Getenv("CRAWL4AI_ENDPOINT"); v != "" {
        CRAWL4AI_ENDPOINT = v
    }
    if v := os.Getenv("CRAWL_TIMEOUT_SECONDS"); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            CRAWL_TIMEOUT_SECONDS = i
        }
    }
    if v := os.Getenv("REMOVE_CONSENT_POPUPS"); v == "false" {
        REMOVE_CONSENT_POPUPS = false
    }
    if v := os.Getenv("FLATTEN_SHADOW_DOM"); v == "false" {
        FLATTEN_SHADOW_DOM = false
    }
    if v := os.Getenv("AVOID_ADS"); v == "false" {
        AVOID_ADS = false
    }
    if v := os.Getenv("MEMORY_SAVING_MODE"); v == "false" {
        MEMORY_SAVING_MODE = false
    }
    if v := os.Getenv("MAX_PAGES_BEFORE_RECYCLE"); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            MAX_PAGES_BEFORE_RECYCLE = i
        }
    }
    if v := os.Getenv("PRUNING_THRESHOLD"); v != "" {
        if f, err := strconv.ParseFloat(v, 64); err == nil {
            PRUNING_THRESHOLD = f
        }
    }
    if v := os.Getenv("INCLUDE_REFERENCES"); v == "false" {
        INCLUDE_REFERENCES = false
    }
}

// =============================================================================
// OpenWebUI-facing types (input and output)
// =============================================================================

// Request — What OpenWebUI sends to this proxy
type Request struct {
    Urls []string `json:"urls"`
}

// SuccessResponseItem — What OpenWebUI expects back
type SuccessResponseItem struct {
    PageContent string           `json:"page_content"`
    Metadata    map[string]string `json:"metadata"`
}

type SuccessResponse []SuccessResponseItem

type ErrorResponse struct {
    ErrorName string `json:"error"`
    Detail    string `json:"detail,omitempty"`
}

// =============================================================================
// Crawl4AI 0.8.x response types
// =============================================================================

type MarkdownResult struct {
    RawMarkdown           string  `json:"raw_markdown"`
    MarkdownWithCitations string  `json:"markdown_with_citations"`
    ReferencesMarkdown    string  `json:"references_markdown"`
    FitMarkdown           *string `json:"fit_markdown"`
    FitHtml               *string `json:"fit_html"`
}

type CrawlResultItem struct {
    Url      string                 `json:"url"`
    Success  bool                   `json:"success"`
    Markdown MarkdownResult         `json:"markdown"`
    Metadata map[string]interface{} `json:"metadata"`
}

type CrawlResponse struct {
    Success bool             `json:"success"`
    Results []CrawlResultItem `json:"results"`
    Result  *CrawlResultItem `json:"result,omitempty"`
}

// GetAllResults handles both single-result and multi-result responses
func (cr *CrawlResponse) GetAllResults() []CrawlResultItem {
    if cr.Result != nil && len(cr.Results) == 0 {
        return []CrawlResultItem{*cr.Result}
    }
    return cr.Results
}

// =============================================================================
// Helper functions
// =============================================================================

func errorResponseFromError(name string, err error) ErrorResponse {
    return ErrorResponse{
        ErrorName: name,
        Detail:    err.Error(),
    }
}

func jsonEncodeInfallible(object any) []byte {
    encoded, err := json.Marshal(object)
    if err != nil {
        panic(err)
    }
    return encoded
}

// =============================================================================
// HTTP client with timeout (prevents hung requests from blocking forever)
// =============================================================================
var httpClient *http.Client

func initHTTPClient() {
    httpClient = &http.Client{
        Timeout: time.Duration(CRAWL_TIMEOUT_SECONDS) * time.Second,
    }
}

// =============================================================================
// Health check endpoint — for monitoring
// =============================================================================
func HealthEndpoint(response http.ResponseWriter, request *http.Request) {
    response.Header().Set("Content-Type", "application/json")
    response.WriteHeader(200)
    response.Write([]byte(`{"status":"healthy"}`))
}

// =============================================================================
// CrawlEndpoint — Main handler that translates OpenWebUI ↔ Crawl4AI
// =============================================================================
func CrawlEndpoint(response http.ResponseWriter, request *http.Request) {
    response.Header().Set("Content-Type", "application/json")

    // -------------------------------------------------------------------------
    // Only accept POST requests
    // -------------------------------------------------------------------------
    if request.Method != "POST" {
        response.WriteHeader(405)
        response.Write(jsonEncodeInfallible(ErrorResponse{ErrorName: "method not allowed"}))
        log.Printf("405 method not allowed :: %s\n", request.RemoteAddr)
        return
    }

    if request.Header.Get("Content-Type") != "application/json" {
        response.WriteHeader(400)
        response.Write(jsonEncodeInfallible(ErrorResponse{ErrorName: "content type must be application/json"}))
        log.Printf("400 invalid content type :: %s\n", request.RemoteAddr)
        return
    }

    // -------------------------------------------------------------------------
    // Parse the incoming OpenWebUI request
    // -------------------------------------------------------------------------
    var requestData Request
    err := json.NewDecoder(request.Body).Decode(&requestData)
    if err != nil {
        response.WriteHeader(400)
        resp := errorResponseFromError("invalid json", err)
        response.Write(jsonEncodeInfallible(resp))
        log.Printf("400 invalid json :: %s\n", request.RemoteAddr)
        return
    }

    log.Printf("Request to crawl %v from %s\n", requestData.Urls, request.RemoteAddr)

    // -------------------------------------------------------------------------
    // Build the enriched Crawl4AI request payload
    // Instead of just sending {"urls": [...]}, we now send a full configuration
    // that unlocks Crawl4AI 0.8.5's features.
    // -------------------------------------------------------------------------
    crawlPayload := map[string]interface{}{
        "urls": requestData.Urls,

        // ---------------------------------------------------------------------
        // browser_config: Controls how Chromium is launched.
        // Reference: https://docs.crawl4ai.com/api/parameters/
        // ---------------------------------------------------------------------
        "browser_config": map[string]interface{}{
            "type": "BrowserConfig",
            "params": map[string]interface{}{
                "headless": true,

                // avoid_ads: Blocks ad trackers (doubleclick, google-analytics, etc.)
                // at the network level. Faster crawls + cleaner content.
                "avoid_ads": AVOID_ADS,

                // memory_saving_mode: Aggressive cache/V8 heap flags to prevent
                // memory leaks during long-running sessions.
                "memory_saving_mode": MEMORY_SAVING_MODE,

                // max_pages_before_recycle: Auto-restart browser after N pages to
                // prevent memory leaks.
                "max_pages_before_recycle": MAX_PAGES_BEFORE_RECYCLE,
            },
        },

        // ---------------------------------------------------------------------
        // crawler_config: Controls how each page is crawled and processed.
        // Reference: https://docs.crawl4ai.com/api/parameters/
        // ---------------------------------------------------------------------
        "crawler_config": map[string]interface{}{
            "type": "CrawlerRunConfig",
            "params": map[string]interface{}{
                // remove_consent_popups: Auto-dismiss cookie/consent banners
                "remove_consent_popups": REMOVE_CONSENT_POPUPS,

                // flatten_shadow_dom: Extract content hidden inside Shadow DOM
                // components.
                "flatten_shadow_dom": FLATTEN_SHADOW_DOM,

                // markdown_generator: Configures how HTML → Markdown conversion works.
                // We use PruningContentFilter to remove boilerplate (nav bars,
                // footers, sidebars) and produce a much cleaner fit_markdown.
                "markdown_generator": map[string]interface{}{
                    "type": "DefaultMarkdownGenerator",
                    "params": map[string]interface{}{
                        "content_filter": map[string]interface{}{
                            "type": "PruningContentFilter",
                            "params": map[string]interface{}{
                                // threshold: Score boundary. Blocks below this score
                                // get removed. 0.48 is balanced.
                                // Lower (0.3) = more aggressive removal.
                                // Higher (0.7) = keep more content.
                                "threshold": PRUNING_THRESHOLD,

                                // threshold_type: "dynamic" adjusts the threshold
                                // based on the page's content structure.
                                "threshold_type": "dynamic",

                                // min_word_threshold: Discard blocks under N words
                                // as likely too short/unhelpful.
                                "min_word_threshold": 10,
                            },
                        },
                        "options": map[string]interface{}{
                            // ignore_links: false — keep links so OpenWebUI can see URLs
                            "ignore_links": false,

                            // body_width: 0 means no line wrapping (better for LLMs)
                            "body_width": 0,

                            // include_sup_sub: handle <sup>/<sub> tags better
                            "include_sup_sub": true,
                        },
                    },
                },
            },
        },
    }

    // -------------------------------------------------------------------------
    // Send the enriched request to Crawl4AI
    // -------------------------------------------------------------------------
    req, err := http.NewRequest("POST", CRAWL4AI_ENDPOINT, bytes.NewReader(jsonEncodeInfallible(crawlPayload)))
    if err != nil {
        panic(err)
    }
    req.Header.Set("Content-Type", "application/json")

    crawlResponse, err := httpClient.Do(req)
    if err != nil || crawlResponse.StatusCode != 200 {
        statusCode := 502
        if crawlResponse != nil {
            statusCode = crawlResponse.StatusCode
        }
        response.WriteHeader(statusCode)
        response.Write(jsonEncodeInfallible(ErrorResponse{
            ErrorName: "bad gateway",
            Detail:    fmt.Sprintf("crawl4ai returned status %d", statusCode),
        }))
        log.Printf("%d bad gateway :: %s\n", statusCode, request.RemoteAddr)
        return
    }

    // -------------------------------------------------------------------------
    // Parse the Crawl4AI response
    // -------------------------------------------------------------------------
    var crawlData CrawlResponse
    err = json.NewDecoder(crawlResponse.Body).Decode(&crawlData)
    if err != nil {
        response.WriteHeader(502)
        resp := ErrorResponse{ErrorName: "bad gateway", Detail: "invalid json received from crawl api"}
        response.Write(jsonEncodeInfallible(resp))
        log.Printf("502 bad gateway - invalid json from crawl api :: %s\n", request.RemoteAddr)
        return
    }

    // -------------------------------------------------------------------------
    // Convert Crawl4AI results to OpenWebUI format
    // -------------------------------------------------------------------------
    ret := SuccessResponse{}
    for _, result := range crawlData.GetAllResults() {
        // Prefer fit_markdown (filtered/pruned content) over raw_markdown.
        // fit_markdown has boilerplate removed by PruningContentFilter.
        content := result.Markdown.RawMarkdown
        if result.Markdown.FitMarkdown != nil && *result.Markdown.FitMarkdown != "" {
            content = *result.Markdown.FitMarkdown
        }

        // Append references/citations for better source attribution in OpenWebUI.
        // This adds a "## References" section with the source links.
        if INCLUDE_REFERENCES && result.Markdown.ReferencesMarkdown != "" {
            content += "\n\n---\n## References\n" + result.Markdown.ReferencesMarkdown
        }

        // Build metadata map, converting all values to strings.
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

        ret = append(ret, SuccessResponseItem{
            PageContent: content,
            Metadata:    metadata,
        })
    }

    response.WriteHeader(200)
    response.Write(jsonEncodeInfallible(ret))
    log.Printf("200 :: %d results :: %s\n", len(ret), request.RemoteAddr)
}

// =============================================================================
// Main entry point
// =============================================================================
func main() {
    ReadEnvironment()
    initHTTPClient()

    http.HandleFunc("/crawl", CrawlEndpoint)
    http.HandleFunc("/health", HealthEndpoint)

    listenAddress := fmt.Sprintf("%s:%d", LISTEN_IP, LISTEN_PORT)
    log.Printf("Listening on %s (timeout: %ds, consent_popups: %v, shadow_dom: %v, ads: %v, pruning: %.2f)\n",
        listenAddress, CRAWL_TIMEOUT_SECONDS, REMOVE_CONSENT_POPUPS, FLATTEN_SHADOW_DOM, AVOID_ADS, PRUNING_THRESHOLD)

    err := http.ListenAndServe(listenAddress, nil)
    if err != nil {
        log.Println(err)
    }
}
