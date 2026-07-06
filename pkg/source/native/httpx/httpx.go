// Package httpx is the one polite HTTP client every native provider shares.
package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	userAgent   = "shirabe/1.0 (+https://github.com/tamnd/shirabe)"
	maxBody     = 4 << 20
	maxRedirect = 5
)

// Client wraps http.Client with the house rules: identifying UA, redirect
// cap, response size cap, sane timeout.
type Client struct {
	HTTP *http.Client
}

func New() *Client {
	return &Client{HTTP: &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirect {
				return fmt.Errorf("stopped after %d redirects", maxRedirect)
			}
			return nil
		},
	}}
}

// Get fetches url and returns at most 4 MB of body. Non-2xx is an error.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/html;q=0.9, */*;q=0.5")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBody))
}
