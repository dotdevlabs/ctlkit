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

const jsonAPIMediaType = "application/vnd.api+json"

// Resource is a decoded JSON:API resource object with typed attributes.
type Resource[T any] struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes T      `json:"attributes"`
}

// Links holds JSON:API top-level pagination links.
type Links struct {
	First string `json:"first,omitempty"`
	Prev  string `json:"prev,omitempty"`
	Next  string `json:"next,omitempty"`
	Last  string `json:"last,omitempty"`
}

// Collection is a decoded JSON:API collection response.
type Collection[T any] struct {
	Data  []Resource[T]  `json:"data"`
	Links Links          `json:"links"`
	Meta  map[string]any `json:"meta,omitempty"`
}

// APIError is a single JSON:API error object.
type APIError struct {
	Status string            `json:"status,omitempty"`
	Title  string            `json:"title,omitempty"`
	Detail string            `json:"detail,omitempty"`
	Source map[string]string `json:"source,omitempty"`
}

// rawResource mirrors the wire format of a JSON:API resource object.
type rawResource struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Attributes json.RawMessage `json:"attributes"`
}

type rawSingleDoc struct {
	Data   rawResource `json:"data"`
	Errors []APIError  `json:"errors"`
}

type rawCollectionDoc struct {
	Data   []rawResource  `json:"data"`
	Links  Links          `json:"links"`
	Meta   map[string]any `json:"meta,omitempty"`
	Errors []APIError     `json:"errors"`
}

// doJSONAPIRaw executes a request with JSON:API content negotiation and retry logic.
// Returns raw response bytes on success; parses JSON:API errors[] on non-2xx.
// See do() in httpclient.go for the sibling flat-envelope version.
func (c *Client) doJSONAPIRaw(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	url := c.baseURL + path

	var lastStatus int
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(500*(1<<(attempt-1))) * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("building request: %w", err)
		}

		req.Header.Set("User-Agent", BrowserUserAgent)
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", jsonAPIMediaType)
		if body != nil {
			req.Header.Set("Content-Type", jsonAPIMediaType)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attempt < maxRetries-1 {
				continue
			}
			return nil, fmt.Errorf("request failed: %w", err)
		}

		lastStatus = resp.StatusCode
		respBody, readErr := io.ReadAll(resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			return nil, fmt.Errorf("closing response body: %w", closeErr)
		}
		if readErr != nil {
			return nil, fmt.Errorf("reading response: %w", readErr)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}

		if isRetryable(resp.StatusCode) && attempt < maxRetries-1 {
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if d, parseErr := time.ParseDuration(ra + "s"); parseErr == nil {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(d):
					}
				}
			}
			continue
		}

		code := statusToCode(resp.StatusCode)
		msg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if len(respBody) > 0 {
			var errDoc struct {
				Errors []APIError `json:"errors"`
			}
			if jsonErr := json.Unmarshal(respBody, &errDoc); jsonErr == nil && len(errDoc.Errors) > 0 {
				parts := make([]string, 0, len(errDoc.Errors))
				for _, e := range errDoc.Errors {
					if e.Detail != "" {
						parts = append(parts, e.Detail)
					} else if e.Title != "" {
						parts = append(parts, e.Title)
					}
				}
				if len(parts) > 0 {
					msg = strings.Join(parts, "; ")
				}
			}
		}
		return nil, clierror.New(code, msg, "")
	}

	return nil, clierror.New(statusToCode(lastStatus), fmt.Sprintf("HTTP %d after %d retries", lastStatus, maxRetries), "")
}

func apiErrorsToError(errs []APIError) error {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		if e.Detail != "" {
			parts = append(parts, e.Detail)
		} else if e.Title != "" {
			parts = append(parts, e.Title)
		}
	}
	if len(parts) == 0 {
		return clierror.New(clierror.CodeServerError, "JSON:API error", "")
	}
	return clierror.New(clierror.CodeBadRequest, strings.Join(parts, "; "), "")
}

// GetJSONAPISingle performs a GET and decodes the response as a JSON:API single resource.
func GetJSONAPISingle[T any](ctx context.Context, c *Client, path string) (Resource[T], error) {
	raw, err := c.doJSONAPIRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return Resource[T]{}, err
	}
	var doc rawSingleDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Resource[T]{}, fmt.Errorf("decoding JSON:API document: %w", err)
	}
	if len(doc.Errors) > 0 {
		return Resource[T]{}, apiErrorsToError(doc.Errors)
	}
	res := Resource[T]{ID: doc.Data.ID, Type: doc.Data.Type}
	if len(doc.Data.Attributes) > 0 {
		if err := json.Unmarshal(doc.Data.Attributes, &res.Attributes); err != nil {
			return Resource[T]{}, fmt.Errorf("decoding attributes: %w", err)
		}
	}
	return res, nil
}

// GetJSONAPICollection performs a GET and decodes the response as a JSON:API collection.
func GetJSONAPICollection[T any](ctx context.Context, c *Client, path string) (Collection[T], error) {
	raw, err := c.doJSONAPIRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return Collection[T]{}, err
	}
	var doc rawCollectionDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Collection[T]{}, fmt.Errorf("decoding JSON:API document: %w", err)
	}
	if len(doc.Errors) > 0 {
		return Collection[T]{}, apiErrorsToError(doc.Errors)
	}
	col := Collection[T]{Links: doc.Links, Meta: doc.Meta}
	for _, r := range doc.Data {
		res := Resource[T]{ID: r.ID, Type: r.Type}
		if len(r.Attributes) > 0 {
			if err := json.Unmarshal(r.Attributes, &res.Attributes); err != nil {
				return Collection[T]{}, fmt.Errorf("decoding attributes: %w", err)
			}
		}
		col.Data = append(col.Data, res)
	}
	return col, nil
}

// PostJSONAPISingle performs a POST with a JSON body and decodes the response as a JSON:API single resource.
func PostJSONAPISingle[T any](ctx context.Context, c *Client, path string, body any) (Resource[T], error) {
	b, err := marshalBody(body)
	if err != nil {
		return Resource[T]{}, err
	}
	raw, err := c.doJSONAPIRaw(ctx, http.MethodPost, path, b)
	if err != nil {
		return Resource[T]{}, err
	}
	var doc rawSingleDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Resource[T]{}, fmt.Errorf("decoding JSON:API document: %w", err)
	}
	if len(doc.Errors) > 0 {
		return Resource[T]{}, apiErrorsToError(doc.Errors)
	}
	res := Resource[T]{ID: doc.Data.ID, Type: doc.Data.Type}
	if len(doc.Data.Attributes) > 0 {
		if err := json.Unmarshal(doc.Data.Attributes, &res.Attributes); err != nil {
			return Resource[T]{}, fmt.Errorf("decoding attributes: %w", err)
		}
	}
	return res, nil
}
