package ctxutil_test

import (
	"context"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
	"github.com/dotdevlabs/ctlkit/pkg/output"
)

func TestConfigRoundTrip(t *testing.T) {
	cfg := &config.Config{CurrentContext: "test"}
	ctx := ctxutil.WithConfig(context.Background(), cfg)
	got := ctxutil.ConfigFrom(ctx)
	if got == nil || got.CurrentContext != "test" {
		t.Errorf("ConfigFrom = %+v", got)
	}
}

func TestActiveContextRoundTrip(t *testing.T) {
	ac := &config.Context{BaseURL: "https://example.com", Token: "tok"}
	ctx := ctxutil.WithActiveContext(context.Background(), ac)
	got := ctxutil.ActiveContextFrom(ctx)
	if got == nil || got.BaseURL != "https://example.com" {
		t.Errorf("ActiveContextFrom = %+v", got)
	}
}

func TestClientRoundTrip(t *testing.T) {
	c := httpclient.New("https://example.com", "tok")
	ctx := ctxutil.WithClient(context.Background(), c)
	got := ctxutil.ClientFrom(ctx)
	if got == nil {
		t.Error("ClientFrom returned nil")
	}
}

func TestRendererRoundTrip(t *testing.T) {
	r := output.New(true, "", nil, nil)
	ctx := ctxutil.WithRenderer(context.Background(), r)
	got := ctxutil.RendererFrom(ctx)
	if got == nil || got.Mode != output.ModeJSON {
		t.Errorf("RendererFrom = %+v", got)
	}
}

func TestGlobalFlagsRoundTrip(t *testing.T) {
	flags := ctxutil.GlobalFlags{Context: "prod", JSON: true, DryRun: true}
	ctx := ctxutil.WithGlobalFlags(context.Background(), flags)
	got := ctxutil.GlobalFlagsFrom(ctx)
	if got.Context != "prod" || !got.JSON || !got.DryRun {
		t.Errorf("GlobalFlagsFrom = %+v", got)
	}
}

func TestMissingValues(t *testing.T) {
	ctx := context.Background()
	if ctxutil.ConfigFrom(ctx) != nil {
		t.Error("expected nil config")
	}
	if ctxutil.ActiveContextFrom(ctx) != nil {
		t.Error("expected nil active context")
	}
	if ctxutil.ClientFrom(ctx) != nil {
		t.Error("expected nil client")
	}
	if ctxutil.RendererFrom(ctx) != nil {
		t.Error("expected nil renderer")
	}
	flags := ctxutil.GlobalFlagsFrom(ctx)
	if flags.Context != "" || flags.JSON || flags.DryRun {
		t.Errorf("expected zero GlobalFlags, got %+v", flags)
	}
}
