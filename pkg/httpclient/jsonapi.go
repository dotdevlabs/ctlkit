package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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

// doJSONAPIRaw executes a request with JSON:API content negotiation.
// Returns raw response bytes on success; parses JSON:API errors[] on non-2xx.
func (c *Client) doJSONAPIRaw(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	raw, err := c.execRaw(ctx, method, path, body, jsonAPIMediaType, jsonAPIMediaType)
	if err != nil {
		return nil, err
	}
	return raw, nil
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
