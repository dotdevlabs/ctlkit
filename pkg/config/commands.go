package config

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// AuthCommand returns the "auth" cobra.Command subtree.
func AuthCommand(product string) *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication credentials",
	}
	auth.AddCommand(authLoginCommand(product))
	return auth
}

func authLoginCommand(product string) *cobra.Command {
	var (
		name       string
		url        string
		token      string
		setCurrent bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Add or update a named context with credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := Load(product)
			if err != nil {
				return err
			}
			if cfg.Contexts == nil {
				cfg.Contexts = make(map[string]Context)
			}
			cfg.Contexts[name] = Context{BaseURL: url, Token: token}
			if setCurrent || cfg.CurrentContext == "" {
				cfg.CurrentContext = name
			}
			if err := Save(product, cfg); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Context %q saved.\n", name)
			return err
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name for this context (required)")
	cmd.Flags().StringVar(&url, "url", "", "Base URL for this context (required)")
	cmd.Flags().StringVar(&token, "token", "", "Bearer token for this context (required)")
	cmd.Flags().BoolVar(&setCurrent, "set-current", false, "Set this context as current")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("token")

	return cmd
}

// ContextCommand returns the "context" cobra.Command subtree.
func ContextCommand(product string) *cobra.Command {
	ctx := &cobra.Command{
		Use:   "context",
		Short: "Manage named contexts",
	}
	ctx.AddCommand(contextListCommand(product))
	ctx.AddCommand(contextSelectCommand(product))
	return ctx
}

func contextListCommand(product string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all named contexts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := Load(product)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			// tabwriter buffers writes; errors surface only at Flush().
			_, _ = fmt.Fprintf(w, "NAME\tURL\tCURRENT\n")
			_, _ = fmt.Fprintf(w, "────\t───\t───────\n")
			for name, c := range cfg.Contexts {
				current := ""
				if name == cfg.CurrentContext {
					current = "*"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", name, c.BaseURL, current)
			}
			return w.Flush()
		},
	}
}

func contextSelectCommand(product string) *cobra.Command {
	return &cobra.Command{
		Use:   "select <name>",
		Short: "Set the current context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := Load(product)
			if err != nil {
				return err
			}
			name := args[0]
			if _, ok := cfg.Contexts[name]; !ok {
				return fmt.Errorf("context %q not found", name)
			}
			cfg.CurrentContext = name
			if err := Save(product, cfg); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Switched to context %q.\n", name)
			return err
		},
	}
}

// skipContextResolution returns true for commands that operate on the config
// file rather than the active context (auth and context subcommands).
func skipContextResolution(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch c.Name() {
		case "auth", "context", "version", "ai":
			return true
		}
	}
	return false
}

// SkipContextResolution is exported for use by pkg/root.
var SkipContextResolution = skipContextResolution
