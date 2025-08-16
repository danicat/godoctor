package crawl_webpage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCrawlWebpage(t *testing.T) {
	var server *httptest.Server
	var externalServer *httptest.Server

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprintf(w, `
				<html>
					<head><title>Test Page</title></head>
					<body>
						<h1>Welcome</h1>
						<a href="/page2">Page 2</a>
						<a href="%s/external">External</a>
					</body>
				</html>
			`, externalServer.URL)
		} else if r.URL.Path == "/page2" {
			w.Write([]byte(`
				<html>
					<head><title>Page 2</title></head>
					<body>
						<p>This is page 2.</p>
						<a href="/">Home</a>
					</body>
				</html>
			`))
		}
	}))
	defer server.Close()

	externalServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
			<html>
				<head><title>External</title></head>
				<body>
					<p>External page</p>
				</body>
			</html>
		`))
	}))
	defer externalServer.Close()

	t.Run("level 0", func(t *testing.T) {
		e, err := New(server.URL, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.Crawl(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(e.results) != 1 {
			t.Errorf("expected 1 result, got %d", len(e.results))
		}
	})

	t.Run("level 1, external false", func(t *testing.T) {
		e, err := New(server.URL, 1, false)
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.Crawl(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(e.results) != 2 {
			t.Errorf("expected 2 results, got %d", len(e.results))
		}
		for _, res := range e.results {
			for _, ref := range res.Refs {
				if ref == externalServer.URL+"/external" {
					t.Error("should not have crawled external link")
				}
			}
		}
	})

	t.Run("level 1, external true", func(t *testing.T) {
		e, err := New(server.URL, 1, true)
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.Crawl(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(e.results) != 3 {
			t.Errorf("expected 3 results, got %d", len(e.results))
		}
		foundExternal := false
		for _, res := range e.results {
			for _, ref := range res.Refs {
				if ref == externalServer.URL+"/external" {
					foundExternal = true
				}
			}
		}
		if !foundExternal {
			t.Error("should have found external link")
		}
	})
}
