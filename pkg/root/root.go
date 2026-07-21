// Package root provides the Cobra root command builder for *ctl CLIs.
package root

import (
	"os"

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
	Product   string
	Short     string
	Version   version.Info
	Commands  []*cobra.Command
	Workflows []airef.Workflow
}

// New builds and returns the root cobra.Command for the product.
// Always includes: auth, context, version, ai subcommands.
func New(cfg BuildConfig) *cobra.Command {
	var (
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

			activeCtx, _, err := config.Resolve(cfg.Product, productCfg, flagContext)
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
			return nil
		},
	}

	// Global persistent flags.
	root.PersistentFlags().StringVar(&flagContext, "context", "", "Named context to use")
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output as JSON envelope")
	root.PersistentFlags().StringVar(&flagFormat, "format", "", "Go-template for output")
	root.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "Preview without API writes")
	root.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Verbose/debug output")

	// Built-in subcommands.
	root.AddCommand(config.AuthCommand(cfg.Product))
	root.AddCommand(config.ContextCommand(cfg.Product))
	root.AddCommand(version.NewCommand(cfg.Product, nil))
	root.AddCommand(airef.NewCommand(cfg.Product, cfg.Version.Version, cfg.Workflows, nil))

	// Product-specific resource commands.
	for _, cmd := range cfg.Commands {
		root.AddCommand(cmd)
	}

	return root
}
