package wechat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
)

// Client is the low-level HTTP client for the iLink Bot API.
type Client struct {
	baseURL        string
	token          string // bot_token from login
	httpClient     *http.Client
	logger         *slog.Logger
	channelVersion string
}

// NewClient creates a new iLink API client.
func NewClient(baseURL string, httpClient *http.Client, logger *slog.Logger, channelVersion string) *Client {
	return &Client{
		baseURL:        baseURL,
		httpClient:     httpClient,
		logger:         logger,
		channelVersion: channelVersion,
	}
}

// SetToken sets the bot_token for authenticated requests.
func (c *Client) SetToken(token string) {
	c.token = token
}

// Token returns the current bot_token.
func (c *Client) Token() string {
	return c.token
}

// SetBaseURL overrides the API base URL (used when login returns a custom baseurl).
func (c *Client) SetBaseURL(url string) {
	c.baseURL = url
}

// generateUIN generates the X-WECHAT-UIN header value:
// random uint32 -> decimal string -> base64 encode
func generateUIN() string {
	n, _ := rand.Int(rand.Reader, new(big.Int).SetUint64(1<<32))
	s := n.String()
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// doRequest performs an HTTP request with iLink headers.
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set iLink headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("X-WECHAT-UIN", generateUIN())
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request %s %s: %w", method, path, err)
	}

	return resp, nil
}

// doJSON performs an HTTP request and decodes the JSON response.
func (c *Client) doJSON(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	resp, err := c.doRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// Get performs an authenticated GET request and decodes JSON response.
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	return c.doJSON(ctx, http.MethodGet, path, nil, result)
}

// Post performs an authenticated POST request and decodes JSON response.
func (c *Client) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doJSON(ctx, http.MethodPost, path, body, result)
}
