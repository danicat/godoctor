package endoscope

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/danicat/godoctor/internal/mcp/result"
	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/net/html"
)

// Register registers the endoscope tool with the server.
func Register(server *mcp.Server, namespace string) {
	name := "endoscope"
	if namespace != "" {
		name = namespace + ":" + name
	}
	schema, err := jsonschema.For[EndoscopeParams]()
	if err != nil {
		panic(err)
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       "Crawl a website",
		Description: "Crawls a website to a specified depth, returning the text-only content of each page. This tool is useful for summarizing web pages, analyzing content, or answering questions about a website's content.",
		InputSchema: schema,
	}, endoscopeHandler)
}

// EndoscopeParams defines the input parameters for the endoscope tool.
type EndoscopeParams struct {
	URL      string `json:"url"`
	Level    int    `json:"level"`
	External bool   `json:"external"`
}

func endoscopeHandler(_ context.Context, _ *mcp.ServerSession, request *mcp.CallToolParamsFor[EndoscopeParams]) (*mcp.CallToolResult, error) {
	e, err := New(request.Arguments.URL, request.Arguments.Level, request.Arguments.External)
	if err != nil {
		return nil, fmt.Errorf("failed to create endoscope: %w", err)
	}
	s, err := e.Crawl()
	if err != nil {
		return nil, fmt.Errorf("failed to crawl: %w", err)
	}
	return result.NewText(s), nil
}

// Result represents the crawled data for a single URL.
type Result struct {
	URL     string   `json:"url"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Refs    []string `json:"refs"`
}

// Endoscope is the main struct for the web crawler.
type Endoscope struct {
	BaseURL  *url.URL
	External bool
	MaxLevel int
	visited  map[string]bool
	results  []*Result
}

// New creates a new Endoscope instance.
func New(baseURL string, level int, external bool) (*Endoscope, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	return &Endoscope{
		BaseURL:  u,
		External: external,
		MaxLevel: level,
		visited:  make(map[string]bool),
		results:  []*Result{},
	}, nil
}

// Crawl starts the crawling process.
func (e *Endoscope) Crawl() (string, error) {
	queue := []struct {
		url   string
		level int
	}{
		{e.BaseURL.String(), 0},
	}
	e.visited[e.BaseURL.String()] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.level > e.MaxLevel {
			continue
		}

		resp, err := http.Get(current.url)
		if err != nil {
			return "", fmt.Errorf("failed to get URL %s: %w", current.url, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to get URL %s: status code %d", current.url, resp.StatusCode)
		}

		doc, err := html.Parse(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to parse HTML from %s: %w", current.url, err)
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

	output, err := json.MarshalIndent(e.results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (e *Endoscope) extractContent(n *html.Node, result *Result, sb *strings.Builder) {
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

func (e *Endoscope) extractBodyContent(n *html.Node, result *Result, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(strings.TrimSpace(n.Data))
		sb.WriteString(" ")
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

func (e *Endoscope) resolveURL(href string) (*url.URL, error) {
	rel, err := url.Parse(href)
	if err != nil {
		return nil, err
	}
	return e.BaseURL.ResolveReference(rel), nil
}

func (e *Endoscope) shouldCrawl(u *url.URL) bool {
	if e.External {
		return true
	}
	return u.Host == e.BaseURL.Host
}
