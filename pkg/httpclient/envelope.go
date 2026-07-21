package httpclient

import "context"

// Envelope is the stable API response shape.
type Envelope[T any] struct {
	Data       T          `json:"data"`
	Pagination *Paginator `json:"pagination,omitempty"`
}

// Paginator holds pagination metadata returned by list endpoints.
type Paginator struct {
	Total   int    `json:"total"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Next    string `json:"next,omitempty"`
	Prev    string `json:"prev,omitempty"`
}

// GetEnvelope performs a GET and decodes the response into an Envelope[T].
func GetEnvelope[T any](ctx context.Context, c *Client, path string) (Envelope[T], error) {
	var env Envelope[T]
	if err := c.Get(ctx, path, &env); err != nil {
		return Envelope[T]{}, err
	}
	return env, nil
}

// PostEnvelope performs a POST and decodes the response into an Envelope[T].
func PostEnvelope[T any](ctx context.Context, c *Client, path string, body any) (Envelope[T], error) {
	var env Envelope[T]
	if err := c.Post(ctx, path, body, &env); err != nil {
		return Envelope[T]{}, err
	}
	return env, nil
}
