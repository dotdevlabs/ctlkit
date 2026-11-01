// Package config manages named contexts for *ctl CLIs via Viper.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
)

// Config is the top-level config file structure.
type Config struct {
	CurrentContext string             `yaml:"current_context" mapstructure:"current_context"`
	Contexts       map[string]Context `yaml:"contexts"         mapstructure:"contexts"`
}

// Context holds connection info for one named context.
type Context struct {
	BaseURL string `yaml:"base_url" mapstructure:"base_url"`
	Token   string `yaml:"token"    mapstructure:"token"`
}

// configPath returns ~/.config/atmt/<product>.yaml.
func configPath(product string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "atmt", product+".yaml"), nil
}

// Load reads the config file for product.
// Returns an empty Config (not an error) if the file does not exist.
func Load(product string) (*Config, error) {
	path, err := configPath(product)
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			return &Config{Contexts: make(map[string]Context)}, nil
		}
		// viper wraps the error; check if it is a not-found variant
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return &Config{Contexts: make(map[string]Context)}, nil
		}
		// os.IsNotExist may not match viper-wrapped errors; check path existence
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			return &Config{Contexts: make(map[string]Context)}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]Context)
	}
	return &cfg, nil
}

// Save writes the config back to disk.
// Creates parent directories if needed (mode 0700) and writes the file (mode 0600).
func Save(product string, cfg *Config) error {
	path, err := configPath(product)
	if err != nil {
		return err
	}
	//nolint:gosec // path is derived from UserHomeDir + hardcoded product name, not user input
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	//nolint:gosec // path is derived from UserHomeDir + hardcoded product name, not user input
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// ResolveOptions carries the per-invocation inputs to Resolve.
type ResolveOptions struct {
	// Context is the explicit context name (the --context flag).
	Context string
	// Token is the explicit token (the --token flag).
	Token string
	// DefaultBaseURL is the product's built-in server address.
	DefaultBaseURL string
}

// Resolve returns the active Context for the product, plus its name.
//
// Context selection: opts.Context > <PRODUCT>_CONTEXT > cfg.CurrentContext.
// A selected context must exist (otherwise a not-found error). Its BaseURL
// wins; if it has none, <PRODUCT>_BASE_URL and then opts.DefaultBaseURL apply.
// With no context selected, the base URL is <PRODUCT>_BASE_URL (an override)
// or else opts.DefaultBaseURL, and the returned name is empty.
//
// Token: opts.Token > <PRODUCT>_TOKEN > the context's token.
func Resolve(product string, cfg *Config, opts ResolveOptions) (*Context, string, error) {
	prefix := strings.ToUpper(strings.ReplaceAll(product, "-", "_"))

	ctxName := opts.Context
	if ctxName == "" {
		ctxName = os.Getenv(prefix + "_CONTEXT")
	}
	if ctxName == "" {
		ctxName = cfg.CurrentContext
	}

	var ctx Context
	if ctxName != "" {
		c, ok := cfg.Contexts[ctxName]
		if !ok {
			return nil, "", clierror.New(
				clierror.CodeNotFound,
				fmt.Sprintf("context %q not found", ctxName),
				"run 'context list' to see available contexts",
			)
		}
		ctx = c
	}

	if ctx.BaseURL == "" {
		ctx.BaseURL = os.Getenv(prefix + "_BASE_URL")
	}
	if ctx.BaseURL == "" {
		ctx.BaseURL = opts.DefaultBaseURL
	}
	if ctx.BaseURL == "" {
		return nil, "", clierror.New(
			clierror.CodeUsage,
			fmt.Sprintf("%s: no server address configured", product),
			"set "+prefix+"_BASE_URL or build the CLI with a DefaultBaseURL",
		)
	}

	if tok := os.Getenv(prefix + "_TOKEN"); tok != "" {
		ctx.Token = tok
	}
	if opts.Token != "" {
		ctx.Token = opts.Token
	}

	return &ctx, ctxName, nil
}
