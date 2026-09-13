// Package exa implements the bounded Exa Search API client.
package exa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"newsroom/internal/domain"
)

const defaultBaseURL = "https://api.exa.ai"

type Client struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
}

func New(apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{APIKey: apiKey, BaseURL: defaultBaseURL, HTTP: httpClient}
}

func (c *Client) Search(ctx context.Context, query string) ([]domain.SourceEvidence, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, fmt.Errorf("Exa API key is not configured")
	}
	body := map[string]any{
		"query": query, "type": "auto", "category": "news", "numResults": 5,
		"contents": map[string]any{
			"highlights": map[string]any{"query": query, "maxCharacters": 1200},
			"text":       map[string]any{"maxCharacters": 4000},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var response searchResponse
	if err := c.doJSON(ctx, "/search", payload, &response); err != nil {
		return nil, err
	}
	results := make([]domain.SourceEvidence, 0, len(response.Results))
	for _, result := range response.Results {
		if strings.TrimSpace(result.URL) == "" {
			continue
		}
		var publishedAt *time.Time
		if result.PublishedDate != "" {
			if parsed, err := time.Parse(time.RFC3339, result.PublishedDate); err == nil {
				publishedAt = &parsed
			} else if parsed, err := time.Parse("2006-01-02", result.PublishedDate); err == nil {
				publishedAt = &parsed
			}
		}
		content := result.Text
		if len(result.Highlights) > 0 {
			content = strings.Join(result.Highlights, "\n") + "\n" + content
		}
		results = append(results, domain.SourceEvidence{URL: result.URL, Title: result.Title, PublishedAt: publishedAt, RetrievedAt: time.Now().UTC(), Content: truncate(content, 6000)})
		if len(results) == 5 {
			break
		}
	}
	return results, nil
}

type searchResponse struct {
	Results []struct {
		Title, URL, PublishedDate, Text string
		Highlights                      []string
	} `json:"results"`
}

func (c *Client) doJSON(ctx context.Context, path string, payload []byte, dst any) error {
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.APIKey)
		resp, err := c.HTTP.Do(req)
		if err == nil {
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
			resp.Body.Close()
			if readErr != nil {
				return fmt.Errorf("read Exa response: %w", readErr)
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if err := json.Unmarshal(data, dst); err != nil {
					return fmt.Errorf("decode Exa response: %w", err)
				}
				return nil
			}
			lastErr = fmt.Errorf("Exa returned HTTP %d", resp.StatusCode)
			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
				return lastErr
			}
		} else {
			lastErr = fmt.Errorf("call Exa: %w", err)
		}
		if attempt == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
	}
	return lastErr
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
