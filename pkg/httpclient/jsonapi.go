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
	SelfLink   string `json:"self_link,omitempty"` // populated from links.self
}

// Links holds JSON:API top-level pagination links.
type Links struct {
	Self  string `json:"self,omitempty"`
	First string `json:"first,omitempty"`
	Prev  string `json:"prev,omitempty"`
	Next  string `json:"next,omitempty"`
	Last  string `json:"last,omitempty"`
}

// PageMeta holds the standard JSON:API pagination metadata fields.
// Fields are zero if the server does not include them.
type PageMeta struct {
	Total   int `json:"total"`
	PerPage int `json:"per_page"`
	Page    int `json:"page"`
}

// Collection is a decoded JSON:API collection response.
type Collection[T any] struct {
	Data     []Resource[T]  `json:"data"`
	Links    Links          `json:"links"`
	Meta     map[string]any `json:"meta,omitempty"`
	PageMeta PageMeta       // typed subset; zero if not present
}

// APIError is a single JSON:API error object.
type APIError struct {
	Status string            `json:"status,omitempty"`
	Title  string            `json:"title,omitempty"`
	Detail string            `json:"detail,omitempty"`
	Source map[string]string `json:"source,omitempty"`
}

// rawResourceLinks holds the wire format of a JSON:API resource links object.
type rawResourceLinks struct {
	Self string `json:"self,omitempty"`
}

// rawResource mirrors the wire format of a JSON:API resource object.
type rawResource struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Attributes json.RawMessage  `json:"attributes"`
	Links      rawResourceLinks `json:"links"`
}

type rawSingleDoc struct {
	Data   rawResource `json:"data"`
	Errors []APIError  `json:"errors"`
}

type rawCollectionDoc struct {
	Data   []rawResource   `json:"data"`
	Links  Links           `json:"links"`
	Meta   json.RawMessage `json:"meta,omitempty"`
	Errors []APIError      `json:"errors"`
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

// resolveLink returns the full URL for a JSON:API link value.
// Absolute URLs are used unchanged; relative paths are prepended with baseURL.
func resolveLink(baseURL, link string) string {
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		return link
	}
	return baseURL + link
}

// doJSONAPIRawLink fetches an arbitrary JSON:API link (relative or absolute URL).
func (c *Client) doJSONAPIRawLink(ctx context.Context, link string) ([]byte, error) {
	url := resolveLink(c.baseURL, link)
	return c.execRawURL(ctx, http.MethodGet, url, nil, jsonAPIMediaType, jsonAPIMediaType)
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

// parseCollectionDoc decodes a raw JSON:API collection document into a Collection[T].
func parseCollectionDoc[T any](raw []byte) (Collection[T], error) {
	var doc rawCollectionDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Collection[T]{}, fmt.Errorf("decoding JSON:API document: %w", err)
	}
	if len(doc.Errors) > 0 {
		return Collection[T]{}, apiErrorsToError(doc.Errors)
	}
	col := Collection[T]{Links: doc.Links}
	if len(doc.Meta) > 0 {
		if err := json.Unmarshal(doc.Meta, &col.Meta); err != nil {
			return Collection[T]{}, fmt.Errorf("decoding meta: %w", err)
		}
		_ = json.Unmarshal(doc.Meta, &col.PageMeta) // best-effort typed view
	}
	for _, r := range doc.Data {
		res := Resource[T]{ID: r.ID, Type: r.Type, SelfLink: r.Links.Self}
		if len(r.Attributes) > 0 {
			if err := json.Unmarshal(r.Attributes, &res.Attributes); err != nil {
				return Collection[T]{}, fmt.Errorf("decoding attributes: %w", err)
			}
		}
		col.Data = append(col.Data, res)
	}
	return col, nil
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
	res := Resource[T]{ID: doc.Data.ID, Type: doc.Data.Type, SelfLink: doc.Data.Links.Self}
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
	return parseCollectionDoc[T](raw)
}

// CollectAllJSONAPI fetches all pages starting at path, following links.next until absent,
// and returns all resources concatenated in order.
func CollectAllJSONAPI[T any](ctx context.Context, c *Client, path string) ([]Resource[T], error) {
	raw, err := c.doJSONAPIRaw(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	col, err := parseCollectionDoc[T](raw)
	if err != nil {
		return nil, err
	}
	all := make([]Resource[T], 0, len(col.Data))
	all = append(all, col.Data...)

	for col.Links.Next != "" {
		raw, err = c.doJSONAPIRawLink(ctx, col.Links.Next)
		if err != nil {
			return nil, err
		}
		col, err = parseCollectionDoc[T](raw)
		if err != nil {
			return nil, err
		}
		all = append(all, col.Data...)
	}
	return all, nil
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
	res := Resource[T]{ID: doc.Data.ID, Type: doc.Data.Type, SelfLink: doc.Data.Links.Self}
	if len(doc.Data.Attributes) > 0 {
		if err := json.Unmarshal(doc.Data.Attributes, &res.Attributes); err != nil {
			return Resource[T]{}, fmt.Errorf("decoding attributes: %w", err)
		}
	}
	return res, nil
}
