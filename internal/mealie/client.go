// Package mealie provides a small, typed client for the Mealie API.
package mealie

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/joeysaladino/homelab-mcp/internal/config"
)

const maxErrorBodyBytes = 4 << 10

// Client calls the Mealie API using a server-side API token.
type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
}

// NewClient constructs a Mealie API client from validated application
// configuration. A nil HTTP client uses http.DefaultClient.
func NewClient(cfg config.MealieConfig, httpClient *http.Client) (*Client, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("create mealie client: base URL is required")
	}
	if strings.TrimSpace(cfg.Token()) == "" {
		return nil, fmt.Errorf("create mealie client: token is required")
	}

	baseURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("create mealie client: parse base URL: %w", err)
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, fmt.Errorf("create mealie client: base URL scheme must be http or https")
	}
	if baseURL.Host == "" {
		return nil, fmt.Errorf("create mealie client: base URL host is required")
	}
	if baseURL.RawQuery != "" || baseURL.Fragment != "" {
		return nil, fmt.Errorf("create mealie client: base URL cannot contain a query or fragment")
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	baseURL.Path = strings.TrimRight(baseURL.Path, "/")
	baseURL.RawPath = ""

	return &Client{
		baseURL:    baseURL,
		token:      strings.TrimSpace(cfg.Token()),
		httpClient: httpClient,
	}, nil
}

func (c *Client) request(ctx context.Context, method, resource string, query url.Values, body io.Reader) (*http.Request, error) {
	if ctx == nil {
		return nil, fmt.Errorf("create mealie request: context is nil")
	}

	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + resource
	endpoint.RawPath = ""
	endpoint.RawQuery = query.Encode()
	endpoint.Fragment = ""

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create mealie request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) getJSON(ctx context.Context, resource string, query url.Values, target any) error {
	return c.doJSON(ctx, http.MethodGet, resource, query, nil, target)
}

func (c *Client) postJSON(ctx context.Context, resource string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode mealie API request: %w", err)
	}
	return c.doJSON(ctx, http.MethodPost, resource, nil, bytes.NewReader(body), target)
}

func (c *Client) doJSON(ctx context.Context, method, resource string, query url.Values, body io.Reader, target any) error {
	req, err := c.request(ctx, method, resource, query, body)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call mealie API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		if readErr != nil {
			return fmt.Errorf("read mealie API error response: %w", readErr)
		}

		return &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(body)),
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode mealie API response: %w", err)
	}
	return nil
}

// HTTPError describes a non-2xx response from Mealie.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("mealie API returned %s", e.Status)
	}
	return fmt.Sprintf("mealie API returned %s: %s", e.Status, e.Body)
}
