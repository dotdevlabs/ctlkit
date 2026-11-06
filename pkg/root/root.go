// Package root provides the Cobra root command builder for *ctl CLIs.
package root

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/airef"
	"github.com/dotdevlabs/ctlkit/pkg/config"
	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
	"github.com/dotdevlabs/ctlkit/pkg/output"
	"github.com/dotdevlabs/ctlkit/pkg/version"
)

// BuildConfig configures the root command builder.
type BuildConfig struct {
	Product string
	// ContractVersion is the API contract this CLI was built against; empty disables the advisory.
	ContractVersion string
	// UpgradeInstructions tells users how to upgrade when contracts differ.
	UpgradeInstructions string
	// DefaultBaseURL is the product's built-in server address. Required:
	// an absolute https:// URL. Build/New refuse to build without it.
	DefaultBaseURL string
	Short          string
	Version        version.Info
	Commands       []*cobra.Command
	Workflows      []airef.Workflow
}

// New builds the root command like Build, but panics if the config is
// invalid so a CLI without a valid DefaultBaseURL cannot start.
func New(cfg BuildConfig) *cobra.Command {
	cmd, err := Build(cfg)
	if err != nil {
		panic(err)
	}
	return cmd
}

// validateDefaultBaseURL checks that the product has an absolute https URL.
func validateDefaultBaseURL(product, raw string) error {
	if raw == "" {
		return fmt.Errorf("%s: DefaultBaseURL is required (set BuildConfig.DefaultBaseURL to the product's https:// server address)", product)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || strings.ContainsAny(raw, " \t\r\n") {
		return fmt.Errorf("%s: DefaultBaseURL %q is invalid: must be an absolute https:// URL with a host", product, raw)
	}
	return nil
}

// Build builds and returns the root cobra.Command for the product.
// Always includes: auth, context, version, ai subcommands.
// Returns an error naming the product if DefaultBaseURL is empty or invalid.
func Build(cfg BuildConfig) (*cobra.Command, error) {
	if err := validateDefaultBaseURL(cfg.Product, cfg.DefaultBaseURL); err != nil {
		return nil, err
	}
	if cfg.ContractVersion != "" && strings.TrimSpace(cfg.UpgradeInstructions) == "" {
		return nil, fmt.Errorf("%s: UpgradeInstructions is required with ContractVersion", cfg.Product)
	}
	var (
		flagToken   string
		flagContext string
		flagJSON    bool
		flagFormat  string
		flagDryRun  bool
		flagVerbose bool
	)

	root := &cobra.Command{
		Use:   cfg.Product,
		Short: cfg.Short,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip context resolution for config-management commands.
			if config.SkipContextResolution(cmd) {
				return nil
			}

			flags := ctxutil.GlobalFlags{
				Context: flagContext,
				JSON:    flagJSON,
				Format:  flagFormat,
				DryRun:  flagDryRun,
				Verbose: flagVerbose,
			}

			productCfg, err := config.Load(cfg.Product)
			if err != nil {
				return err
			}

			activeCtx, _, err := config.Resolve(cfg.Product, productCfg, config.ResolveOptions{
				Context:        flagContext,
				Token:          flagToken,
				DefaultBaseURL: cfg.DefaultBaseURL,
			})
			if err != nil {
				return err
			}

			client := httpclient.New(activeCtx.BaseURL, activeCtx.Token)
			renderer := output.New(flagJSON, flagFormat, os.Stdout, os.Stderr)

			ctx := cmd.Context()
			ctx = ctxutil.WithGlobalFlags(ctx, flags)
			ctx = ctxutil.WithConfig(ctx, productCfg)
			ctx = ctxutil.WithActiveContext(ctx, activeCtx)
			ctx = ctxutil.WithClient(ctx, client)
			ctx = ctxutil.WithRenderer(ctx, renderer)
			cmd.SetContext(ctx)
			checkContract(cmd, cfg, activeCtx.BaseURL, activeCtx.Token)
			return nil
		},
	}

	// Global persistent flags.
	root.PersistentFlags().StringVar(&flagContext, "context", "", "Named context to use")
	root.PersistentFlags().StringVar(&flagToken, "token", "", "API token (overrides <PRODUCT>_TOKEN and the context token)")
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output as JSON envelope")
	root.PersistentFlags().StringVar(&flagFormat, "format", "", "Go-template for output")
	root.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "Preview without API writes")
	root.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Verbose/debug output")

	// Built-in subcommands.
	root.AddCommand(config.AuthCommand(cfg.Product))
	root.AddCommand(config.ContextCommand(cfg.Product))
	root.AddCommand(version.NewCommand(cfg.Product, nil))
	root.AddCommand(airef.NewCommand(cfg.Product, cfg.Version.Version, cfg.Workflows))

	// Product-specific resource commands.
	for _, cmd := range cfg.Commands {
		root.AddCommand(cmd)
	}

	return root, nil
}
