// Package roottest provides test helpers for CLIs built on pkg/root.
package roottest

import (
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/root"
)

// AssertBuilds fails t unless the root command builds from cfg (so a valid
// DefaultBaseURL is set) and a clean environment resolves to that default.
// Every CLI should call it from its tests.
func AssertBuilds(t testing.TB, cfg root.BuildConfig) {
	t.Helper()
	if _, err := root.Build(cfg); err != nil {
		t.Fatalf("root does not build: %v", err)
		return
	}

	// Resolve with no context file and no environment overrides.
	t.Setenv("HOME", t.TempDir())
	prefix := strings.ToUpper(strings.ReplaceAll(cfg.Product, "-", "_"))
	for _, suffix := range []string{"_CONTEXT", "_BASE_URL", "_TOKEN"} {
		t.Setenv(prefix+suffix, "")
	}
	loaded, err := config.Load(cfg.Product)
	if err != nil {
		t.Fatalf("loading config: %v", err)
		return
	}
	ctx, _, err := config.Resolve(cfg.Product, loaded, config.ResolveOptions{DefaultBaseURL: cfg.DefaultBaseURL})
	if err != nil {
		t.Fatalf("resolving default: %v", err)
		return
	}
	if ctx.BaseURL != cfg.DefaultBaseURL {
		t.Fatalf("%s: resolved base URL %q, want default %q", cfg.Product, ctx.BaseURL, cfg.DefaultBaseURL)
	}
}
