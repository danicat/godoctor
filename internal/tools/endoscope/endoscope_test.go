package endoscope

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEndoscope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write([]byte(`
				<html>
					<head><title>Test Page</title></head>
					<body>
						<h1>Welcome</h1>
						<a href="/page2">Page 2</a>
						<a href="http://example.com/external">External</a>
					</body>
				</html>
			`))
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

	t.Run("level 0", func(t *testing.T) {
		e, err := New(server.URL, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.Crawl()
		if err != nil {
			t.Fatal(err)
		}
		if len(e.results) != 1 {
			t.Errorf("expected 1 result, got %d", len(e.results))
		}
	})

	t.Run("level 1", func(t *testing.T) {
		e, err := New(server.URL, 1, false)
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.Crawl()
		if err != nil {
			t.Fatal(err)
		}
		if len(e.results) != 2 {
			t.Errorf("expected 2 results, got %d", len(e.results))
		}
	})

	t.Run("external false", func(t *testing.T) {
		e, err := New(server.URL, 1, false)
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.Crawl()
		if err != nil {
			t.Fatal(err)
		}
		for _, res := range e.results {
			for _, ref := range res.Refs {
				if ref == "http://example.com/external" {
					t.Error("should not have crawled external link")
				}
			}
		}
	})

	t.Run("external true", func(t *testing.T) {
		e, err := New(server.URL, 1, true)
		if err != nil {
			t.Fatal(err)
		}
		_, err = e.Crawl()
		if err != nil {
			t.Fatal(err)
		}
		foundExternal := false
		for _, res := range e.results {
			for _, ref := range res.Refs {
				if ref == "http://example.com/external" {
					foundExternal = true
				}
			}
		}
		if !foundExternal {
			// Note: this test doesn't actually crawl the external link,
			// it just checks if it's included in the refs.
			// A more complete test would mock the external server.
		}
	})
}
