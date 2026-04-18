package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "strconv"
)

var (
    LISTEN_IP      string = ""
    LISTEN_PORT    int    = 8000
    CRAWL4AI_ENDPOINT      = "http://crawl4ai:11235/crawl"
)

func ReadEnvironment() {
    portStr := os.Getenv("LISTEN_PORT")
    port, err := strconv.Atoi(portStr)
    if err == nil {
        LISTEN_PORT = port
    }
    ip := os.Getenv("LISTEN_IP")
    if ip != "" {
        LISTEN_IP = ip
    }
    endpoint := os.Getenv("CRAWL4AI_ENDPOINT")
    if endpoint != "" {
        CRAWL4AI_ENDPOINT = endpoint
    }
}

// OpenWebUI-facing request
type Request struct {
    Urls []string `json:"urls"`
}

// OpenWebUI-facing response
type SuccessResponseItem struct {
    PageContent string           `json:"page_content"`
    Metadata    map[string]string `json:"metadata"`
}

type SuccessResponse []SuccessResponseItem

type ErrorResponse struct {
    ErrorName string `json:"error"`
    Detail    string `json:"detail,omitempty"`
}

// Crawl4AI v0.8.x response format
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
    Markdown MarkdownResult        `json:"markdown"`
    Metadata map[string]interface{} `json:"metadata"`
}

type CrawlResponse struct {
    Success bool              `json:"success"`
    Results []CrawlResultItem `json:"results"`
    Result  *CrawlResultItem  `json:"result,omitempty"`
}

// GetAllResults handles both single-result and multi-result responses
func (cr *CrawlResponse) GetAllResults() []CrawlResultItem {
    if cr.Result != nil && len(cr.Results) == 0 {
        return []CrawlResultItem{*cr.Result}
    }
    return cr.Results
}

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

func CrawlEndpoint(response http.ResponseWriter, request *http.Request) {
    response.Header().Set("Content-Type", "application/json")

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

    var requestData Request
    err := json.NewDecoder(request.Body).Decode(&requestData)
    if err != nil {
        response.WriteHeader(400)
        resp := errorResponseFromError("invalid json", err)
        response.Write(jsonEncodeInfallible(resp))
        log.Printf("400 invalid json :: %s\n", request.RemoteAddr)
        return
    }

    log.Printf("Request to crawl %s from %s\n", requestData.Urls, request.RemoteAddr)

    // Forward request to crawl4ai (v0.8.x accepts {"urls": [...]} format)
    req, err := http.NewRequest("POST", CRAWL4AI_ENDPOINT, bytes.NewReader(jsonEncodeInfallible(requestData)))
    if err != nil {
        panic(err)
    }

    crawlResponse, err := http.DefaultClient.Do(req)
    if err != nil || crawlResponse.StatusCode != 200 {
        statusCode := 502
        if crawlResponse != nil {
            statusCode = crawlResponse.StatusCode
        }
        response.WriteHeader(statusCode)
        response.Write(jsonEncodeInfallible(ErrorResponse{ErrorName: "bad gateway", Detail: fmt.Sprintf("crawl4ai returned status %d", statusCode)}))
        log.Printf("%d bad gateway :: %s\n", statusCode, request.RemoteAddr)
        return
    }

    var crawlData CrawlResponse
    err = json.NewDecoder(crawlResponse.Body).Decode(&crawlData)
    if err != nil {
        response.WriteHeader(502)
        resp := ErrorResponse{ErrorName: "bad gateway", Detail: "invalid json received from crawl api"}
        response.Write(jsonEncodeInfallible(resp))
        log.Printf("502 bad gateway - invalid json from crawl api :: %s\n", request.RemoteAddr)
        return
    }

    ret := SuccessResponse{}
    for _, result := range crawlData.GetAllResults() {
        // Prefer fit_markdown (filtered/pruned content) over raw_markdown
        content := result.Markdown.RawMarkdown
        if result.Markdown.FitMarkdown != nil && *result.Markdown.FitMarkdown != "" {
            content = *result.Markdown.FitMarkdown
        }

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
    log.Printf("200 :: %s\n", request.RemoteAddr)
}

func main() {
    ReadEnvironment()
    http.HandleFunc("/crawl", CrawlEndpoint)
    listenAddress := fmt.Sprintf("%s:%d", LISTEN_IP, LISTEN_PORT)
    log.Printf("Listening on %s\n", listenAddress)
    err := http.ListenAndServe(listenAddress, nil)
    if err != nil {
        log.Println(err)
    }
}
