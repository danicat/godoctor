package crawl_webpage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/net/html"
)

// Register registers the crawl_webpage tool with the server.
func Register(server *mcp.Server) {
	name := "crawl_webpage"
	schema, err := jsonschema.For[CrawlWebpageParams]()
	if err != nil {
		panic(err)
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       "Crawl a website",
		Description: "Crawls a website to a specified depth, returning the text-only content of each page. This tool is useful for extracting documentation from websites.",
		InputSchema: schema,
	}, crawlWebpageHandler)
}

// CrawlWebpageParams defines the input parameters for the crawl_webpage tool.
type CrawlWebpageParams struct {
	URL      string `json:"url"`
	Level    int    `json:"level"`
	External bool   `json:"external"`
}

func crawlWebpageHandler(ctx context.Context, s *mcp.ServerSession, request *mcp.CallToolParamsFor[CrawlWebpageParams]) (*mcp.CallToolResult, error) {
	e, err := New(request.Arguments.URL, request.Arguments.Level, request.Arguments.External)
	if err != nil {
		return nil, fmt.Errorf("failed to create crawl_webpage: %w", err)
	}
	crawlResult, err := e.Crawl(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to crawl: %w", err)
	}
	b, err := json.Marshal(crawlResult)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal crawl result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(b)},
		},
	}, nil
}

// CrawlResult represents the overall result of a crawl, including successful
// results and any errors that occurred.
type CrawlResult struct {
	Results []*Result     `json:"results"`
	Errors  []*CrawlError `json:"errors"`
}

// CrawlError represents an error that occurred while crawling a single URL.
type CrawlError struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

// Result represents the crawled data for a single URL.
type Result struct {
	URL     string   `json:"url"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Refs    []string `json:"-"`
}

// CrawlWebpage is the main struct for the web crawler.
type CrawlWebpage struct {
	BaseURL  *url.URL
	External bool
	MaxLevel int
	visited  map[string]bool
	results  []*Result
	errors   []*CrawlError
}

// New creates a new CrawlWebpage instance.
func New(baseURL string, level int, external bool) (*CrawlWebpage, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	return &CrawlWebpage{
		BaseURL:  u,
		External: external,
		MaxLevel: level,
		visited:  make(map[string]bool),
		results:  []*Result{},
		errors:   []*CrawlError{},
	}, nil
}

// Crawl starts the crawling process.
func (e *CrawlWebpage) Crawl(ctx context.Context) (*CrawlResult, error) {
	queue := []struct {
		url   string
		level int
	}{
		{e.BaseURL.String(), 0},
	}
	e.visited[e.BaseURL.String()] = true

	for len(queue) > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		current := queue[0]
		queue = queue[1:]

		if current.level > e.MaxLevel {
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current.url, nil)
		if err != nil {
			e.errors = append(e.errors, &CrawlError{URL: current.url, Error: err.Error()})
			continue
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			e.errors = append(e.errors, &CrawlError{URL: current.url, Error: err.Error()})
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			e.errors = append(e.errors, &CrawlError{URL: current.url, Error: fmt.Sprintf("status code %d", resp.StatusCode)})
			continue
		}

		if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
			continue
		}

		doc, err := html.Parse(resp.Body)
		if err != nil {
			e.errors = append(e.errors, &CrawlError{URL: current.url, Error: fmt.Sprintf("failed to parse HTML: %v", err)})
			continue
		}

		result := &Result{
			URL: current.url,
		}
		var sb strings.Builder
		e.extractContent(doc, result, &sb)
		result.Content = sb.String()
		e.results = append(e.results, result)

		if current.level < e.MaxLevel {
			for _, ref := range result.Refs {
				if !e.visited[ref] {
					e.visited[ref] = true
					queue = append(queue, struct {
						url   string
						level int
					}{ref, current.level + 1})
				}
			}
		}
	}

	return &CrawlResult{Results: e.results, Errors: e.errors}, nil
}

func (e *CrawlWebpage) extractContent(n *html.Node, result *Result, sb *strings.Builder) {
	if n.Type == html.ElementNode && n.Data == "title" {
		if n.FirstChild != nil {
			result.Title = n.FirstChild.Data
		}
	}
	if n.Type == html.ElementNode && n.Data == "body" {
		e.extractBodyContent(n, result, sb)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		e.extractContent(c, result, sb)
	}
}

func (e *CrawlWebpage) extractBodyContent(n *html.Node, result *Result, sb *strings.Builder) {
	allowedParents := map[string]bool{
		"p": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
		"li": true, "td": true, "th": true, "a": true, "blockquote": true, "span": true,
		"strong": true, "em": true, "b": true, "i": true, "code": true, "pre": true,
	}

	if n.Type == html.TextNode {
		if n.Parent != nil && allowedParents[n.Parent.Data] {
			trimmed := strings.TrimSpace(n.Data)
			if trimmed != "" {
				sb.WriteString(trimmed)
				sb.WriteString(" ")
			}
		}
	}

	if n.Type == html.ElementNode && n.Data == "a" {
		for _, a := range n.Attr {
			if a.Key == "href" {
				link, err := e.resolveURL(a.Val)
				if err != nil {
					return
				}
				if e.shouldCrawl(link) {
					result.Refs = append(result.Refs, link.String())
				}
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		e.extractBodyContent(c, result, sb)
	}
}

func (e *CrawlWebpage) resolveURL(href string) (*url.URL, error) {
	rel, err := url.Parse(href)
	if err != nil {
		return nil, err
	}
	return e.BaseURL.ResolveReference(rel), nil
}

func (e *CrawlWebpage) shouldCrawl(u *url.URL) bool {
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if e.External {
		return true
	}
	return u.Host == e.BaseURL.Host
}
