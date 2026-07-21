// Package ctxutil provides type-safe Go context.Context helpers for *ctl CLIs.
package ctxutil

import (
	"context"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
	"github.com/dotdevlabs/ctlkit/pkg/output"
)

type contextKey int

const (
	keyConfig contextKey = iota
	keyActive
	keyClient
	keyRenderer
	keyGlobalFlags
)

// GlobalFlags holds the resolved global flag values from the root command.
type GlobalFlags struct {
	Context string
	JSON    bool
	Format  string
	DryRun  bool
	Verbose bool
}

// WithConfig stores cfg in ctx.
func WithConfig(ctx context.Context, cfg *config.Config) context.Context {
	return context.WithValue(ctx, keyConfig, cfg)
}

// ConfigFrom retrieves the config from ctx.
func ConfigFrom(ctx context.Context) *config.Config {
	v, _ := ctx.Value(keyConfig).(*config.Config)
	return v
}

// WithActiveContext stores the active named context in ctx.
func WithActiveContext(ctx context.Context, c *config.Context) context.Context {
	return context.WithValue(ctx, keyActive, c)
}

// ActiveContextFrom retrieves the active named context from ctx.
func ActiveContextFrom(ctx context.Context) *config.Context {
	v, _ := ctx.Value(keyActive).(*config.Context)
	return v
}

// WithClient stores the HTTP client in ctx.
func WithClient(ctx context.Context, c *httpclient.Client) context.Context {
	return context.WithValue(ctx, keyClient, c)
}

// ClientFrom retrieves the HTTP client from ctx.
func ClientFrom(ctx context.Context) *httpclient.Client {
	v, _ := ctx.Value(keyClient).(*httpclient.Client)
	return v
}

// WithRenderer stores the output renderer in ctx.
func WithRenderer(ctx context.Context, r *output.Renderer) context.Context {
	return context.WithValue(ctx, keyRenderer, r)
}

// RendererFrom retrieves the output renderer from ctx.
func RendererFrom(ctx context.Context) *output.Renderer {
	v, _ := ctx.Value(keyRenderer).(*output.Renderer)
	return v
}

// WithGlobalFlags stores global flags in ctx.
func WithGlobalFlags(ctx context.Context, f GlobalFlags) context.Context {
	return context.WithValue(ctx, keyGlobalFlags, f)
}

// GlobalFlagsFrom retrieves global flags from ctx.
func GlobalFlagsFrom(ctx context.Context) GlobalFlags {
	v, _ := ctx.Value(keyGlobalFlags).(GlobalFlags)
	return v
}
