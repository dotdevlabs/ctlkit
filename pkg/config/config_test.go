package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"
)

const defURL = "https://default.example.com"

func writeTemp(t *testing.T, product, content string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "atmt")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, product+".yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadTwoContexts(t *testing.T) {
	writeTemp(t, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
  sandbox:
    base_url: https://sandbox.example.com
    token: tok-sandbox
`)
	cfg, err := config.Load("testctl")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentContext != "prod" {
		t.Errorf("CurrentContext = %q, want %q", cfg.CurrentContext, "prod")
	}
	if len(cfg.Contexts) != 2 {
		t.Errorf("len(Contexts) = %d, want 2", len(cfg.Contexts))
	}
	if cfg.Contexts["prod"].BaseURL != "https://prod.example.com" {
		t.Errorf("prod BaseURL = %q", cfg.Contexts["prod"].BaseURL)
	}
}

func TestLoadMissingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := config.Load("nonexistent")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.CurrentContext != "" {
		t.Errorf("expected empty CurrentContext")
	}
}

func TestResolvByName(t *testing.T) {
	writeTemp(t, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
  sandbox:
    base_url: https://sandbox.example.com
    token: tok-sandbox
`)
	cfg, _ := config.Load("testctl")
	ctx, name, err := config.Resolve("testctl", cfg, config.ResolveOptions{Context: "sandbox", DefaultBaseURL: defURL})
	if err != nil {
		t.Fatal(err)
	}
	if name != "sandbox" {
		t.Errorf("name = %q, want sandbox", name)
	}
	if ctx.BaseURL != "https://sandbox.example.com" {
		t.Errorf("BaseURL = %q", ctx.BaseURL)
	}
}

func TestResolveCurrentContext(t *testing.T) {
	writeTemp(t, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
`)
	cfg, _ := config.Load("testctl")
	ctx, name, err := config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if err != nil {
		t.Fatal(err)
	}
	if name != "prod" {
		t.Errorf("name = %q, want prod", name)
	}
	if ctx.Token != "tok-prod" {
		t.Errorf("token = %q", ctx.Token)
	}
}

func TestResolveEnvContext(t *testing.T) {
	writeTemp(t, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
  sandbox:
    base_url: https://sandbox.example.com
    token: tok-sandbox
`)
	t.Setenv("TESTCTL_CONTEXT", "sandbox")
	cfg, _ := config.Load("testctl")
	_, name, err := config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if err != nil {
		t.Fatal(err)
	}
	if name != "sandbox" {
		t.Errorf("name = %q, want sandbox", name)
	}
}

func TestResolveTokenEnvOverride(t *testing.T) {
	writeTemp(t, "testctl", `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
`)
	t.Setenv("TESTCTL_TOKEN", "env-token")
	cfg, _ := config.Load("testctl")
	ctx, _, err := config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Token != "env-token" {
		t.Errorf("token = %q, want env-token", ctx.Token)
	}
}

func TestResolveNoContextUsesDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TESTCTL_CONTEXT", "")
	t.Setenv("TESTCTL_BASE_URL", "")
	t.Setenv("TESTCTL_TOKEN", "")
	cfg, _ := config.Load("testctl")
	ctx, name, err := config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.BaseURL != defURL || name != "" || ctx.Token != "" {
		t.Errorf("got %+v name=%q", ctx, name)
	}
}

func TestResolveNoDefaultNoContextErrors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TESTCTL_CONTEXT", "")
	t.Setenv("TESTCTL_BASE_URL", "")
	cfg, _ := config.Load("testctl")
	_, _, err := config.Resolve("testctl", cfg, config.ResolveOptions{})
	if err == nil || !strings.Contains(err.Error(), "testctl") {
		t.Fatalf("expected error naming product, got %v", err)
	}
}

func TestResolveMissingNamedContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TESTCTL_CONTEXT", "ghost")
	cfg, _ := config.Load("testctl")
	_, _, err := config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestResolveTokenPrecedence(t *testing.T) {
	writeTemp(t, "testctl", prodCtxFile)
	cfg, _ := config.Load("testctl")
	t.Setenv("TESTCTL_TOKEN", "env-token")
	ctx, _, _ := config.Resolve("testctl", cfg, config.ResolveOptions{Token: "flag-token", DefaultBaseURL: defURL})
	if ctx.Token != "flag-token" {
		t.Errorf("flag should beat env, got %q", ctx.Token)
	}
	ctx, _, _ = config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if ctx.Token != "env-token" {
		t.Errorf("env should beat context, got %q", ctx.Token)
	}
}

func TestResolveTokenEnvNoContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TESTCTL_CONTEXT", "")
	t.Setenv("TESTCTL_BASE_URL", "")
	t.Setenv("TESTCTL_TOKEN", "env-token")
	cfg, _ := config.Load("testctl")
	ctx, _, err := config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if err != nil || ctx.BaseURL != defURL || ctx.Token != "env-token" {
		t.Errorf("got %+v err=%v", ctx, err)
	}
}

func TestResolveContextWithoutURLFallsBack(t *testing.T) {
	writeTemp(t, "testctl", "current_context: bare\ncontexts:\n  bare:\n    token: t\n")
	t.Setenv("TESTCTL_BASE_URL", "")
	cfg, _ := config.Load("testctl")
	ctx, _, _ := config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if ctx.BaseURL != defURL {
		t.Errorf("want default, got %q", ctx.BaseURL)
	}
	t.Setenv("TESTCTL_BASE_URL", "https://env.example.com")
	ctx, _, _ = config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if ctx.BaseURL != "https://env.example.com" {
		t.Errorf("want env, got %q", ctx.BaseURL)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := &config.Config{
		CurrentContext: "dev",
		Contexts: map[string]config.Context{
			"dev": {BaseURL: "https://dev.example.com", Token: "tok-dev"},
		},
	}
	if err := config.Save("testctl", cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load("testctl")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CurrentContext != "dev" {
		t.Errorf("CurrentContext = %q", loaded.CurrentContext)
	}
	if loaded.Contexts["dev"].Token != "tok-dev" {
		t.Errorf("token = %q", loaded.Contexts["dev"].Token)
	}
}

const prodCtxFile = `
current_context: prod
contexts:
  prod:
    base_url: https://prod.example.com
    token: tok-prod
`

func TestResolveBaseURLUnsetKeepsFileURL(t *testing.T) {
	writeTemp(t, "testctl", prodCtxFile)
	t.Setenv("TESTCTL_BASE_URL", "")
	cfg, _ := config.Load("testctl")
	ctx, _, err := config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.BaseURL != "https://prod.example.com" {
		t.Errorf("BaseURL = %q, want file URL", ctx.BaseURL)
	}
}

func TestResolveContextFileWinsOverEnvBaseURL(t *testing.T) {
	writeTemp(t, "testctl", prodCtxFile)
	t.Setenv("TESTCTL_BASE_URL", "https://env.example.com")
	cfg, _ := config.Load("testctl")
	ctx, name, err := config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.BaseURL != "https://prod.example.com" || ctx.Token != "tok-prod" || name != "prod" {
		t.Errorf("got %+v name=%q", ctx, name)
	}
}

func TestResolveBaseURLEnvNoContextFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TESTCTL_CONTEXT", "")
	t.Setenv("TESTCTL_BASE_URL", "https://env.example.com")
	t.Setenv("TESTCTL_TOKEN", "env-token")
	cfg, err := config.Load("testctl")
	if err != nil {
		t.Fatal(err)
	}
	ctx, name, err := config.Resolve("testctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.BaseURL != "https://env.example.com" || ctx.Token != "env-token" || name != "" {
		t.Errorf("got %+v name=%q", ctx, name)
	}
}

func TestResolveBaseURLEnvNamedContextMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TESTCTL_BASE_URL", "https://env.example.com")
	cfg, _ := config.Load("testctl")
	if _, _, err := config.Resolve("testctl", cfg, config.ResolveOptions{Context: "nope", DefaultBaseURL: defURL}); err == nil {
		t.Fatal("expected not-found error for missing named context")
	}
}

func TestResolveBaseURLEnvHyphenProduct(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MY_CTL_CONTEXT", "")
	t.Setenv("MY_CTL_BASE_URL", "https://hyphen.example.com")
	cfg, _ := config.Load("my-ctl")
	ctx, _, err := config.Resolve("my-ctl", cfg, config.ResolveOptions{DefaultBaseURL: defURL})
	if err != nil {
		t.Fatal(err)
	}
	if ctx.BaseURL != "https://hyphen.example.com" {
		t.Errorf("BaseURL = %q", ctx.BaseURL)
	}
}
