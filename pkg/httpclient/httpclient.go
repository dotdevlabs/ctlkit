// Package httpclient provides a shared HTTP client with browser UA, bearer auth, retry, and envelope decoding.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
)

// BrowserUserAgent is a real browser UA set on every request.
// Periodic rotation may be needed as Cloudflare bot-detection heuristics evolve.
const BrowserUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

const maxRetries = 3

// Client is an HTTP client with bearer auth, browser UA, and retry logic.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// New creates a Client with default settings.
func New(baseURL, token string) *Client {
	return NewWithTransport(baseURL, token, http.DefaultTransport)
}

// NewWithTransport creates a Client with an injectable RoundTripper (for tests).
func NewWithTransport(baseURL, token string, transport http.RoundTripper) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

func isRetryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

func statusToCode(status int) clierror.ErrorCode {
	switch status {
	case http.StatusUnauthorized:
		return clierror.CodeUnauthorized
	case http.StatusForbidden:
		return clierror.CodeForbidden
	case http.StatusNotFound:
		return clierror.CodeNotFound
	case http.StatusConflict:
		return clierror.CodeConflict
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return clierror.CodeBadRequest
	case http.StatusServiceUnavailable:
		return clierror.CodeNotReady
	default:
		return clierror.CodeServerError
	}
}

// do executes the request with retry logic, sets browser UA and bearer auth on every attempt.
func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	url := c.baseURL + path

	var bodyBytes []byte
	if body != nil {
		bodyBytes = body
	}

	var lastStatus int
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(500*(1<<(attempt-1))) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		var bodyReader io.Reader
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return fmt.Errorf("building request: %w", err)
		}

		req.Header.Set("User-Agent", BrowserUserAgent)
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		if bodyBytes != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempt < maxRetries-1 {
				continue
			}
			return fmt.Errorf("request failed: %w", err)
		}

		lastStatus = resp.StatusCode
		respBody, readErr := io.ReadAll(resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			return fmt.Errorf("closing response body: %w", closeErr)
		}
		if readErr != nil {
			return fmt.Errorf("reading response: %w", readErr)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out != nil && len(respBody) > 0 {
				if err := json.Unmarshal(respBody, out); err != nil {
					return fmt.Errorf("decoding response: %w", err)
				}
			}
			return nil
		}

		if isRetryable(resp.StatusCode) && attempt < maxRetries-1 {
			// Respect Retry-After header when present.
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if d, parseErr := time.ParseDuration(ra + "s"); parseErr == nil {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(d):
					}
				}
			}
			continue
		}

		// Non-retryable or retries exhausted — build a CLIError.
		code := statusToCode(resp.StatusCode)
		msg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		// Try to extract message from JSON error body.
		var errBody struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if len(respBody) > 0 {
			if jsonErr := json.Unmarshal(respBody, &errBody); jsonErr == nil {
				if errBody.Message != "" {
					msg = errBody.Message
				} else if errBody.Error != "" {
					msg = errBody.Error
				}
			}
		}
		return clierror.New(code, msg, "")
	}

	return clierror.New(statusToCode(lastStatus), fmt.Sprintf("HTTP %d after %d retries", lastStatus, maxRetries), "")
}

func marshalBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}
	return b, nil
}

// Get performs an HTTP GET and decodes the JSON response into out.
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// Post performs an HTTP POST with a JSON body and decodes the response into out.
func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	b, err := marshalBody(body)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, path, b, out)
}

// Patch performs an HTTP PATCH with a JSON body and decodes the response into out.
func (c *Client) Patch(ctx context.Context, path string, body, out any) error {
	b, err := marshalBody(body)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPatch, path, b, out)
}

// Delete performs an HTTP DELETE.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
