package exa

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchUsesOfficialExaShapeAndBoundsResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/search" || r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("unexpected request")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["numResults"].(float64) != 5 || body["category"] != "news" {
			t.Fatalf("unexpected search body: %#v", body)
		}
		contents := body["contents"].(map[string]any)
		if _, ok := contents["text"]; !ok {
			t.Fatal("missing nested contents.text")
		}
		if _, ok := contents["highlights"]; !ok {
			t.Fatal("missing nested contents.highlights")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Report","url":"https://example.com/a","publishedDate":"2026-01-02","text":"evidence"}]}`))
	}))
	defer server.Close()
	client := New("test-key", server.Client())
	client.BaseURL = server.URL
	sources, err := client.Search(context.Background(), "Premier League news")
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].PublishedAt == nil || sources[0].URL != "https://example.com/a" {
		t.Fatalf("unexpected sources: %#v", sources)
	}
}

func TestSearchRejectsMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`not json`)) }))
	defer server.Close()
	client := New("key", server.Client())
	client.BaseURL = server.URL
	if _, err := client.Search(context.Background(), "query"); err == nil {
		t.Fatal("expected malformed response error")
	}
}
