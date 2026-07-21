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

// Resolve returns the active Context for the product.
// Priority: ctxName arg > <PRODUCT>_CONTEXT env var > cfg.CurrentContext.
// Applies <PRODUCT>_TOKEN env var override to the resolved Context.
// Also returns the resolved context name.
func Resolve(product string, cfg *Config, ctxName string) (*Context, string, error) {
	prefix := strings.ToUpper(strings.ReplaceAll(product, "-", "_"))

	if ctxName == "" {
		if envCtx := os.Getenv(prefix + "_CONTEXT"); envCtx != "" {
			ctxName = envCtx
		}
	}
	if ctxName == "" {
		ctxName = cfg.CurrentContext
	}
	if ctxName == "" {
		return nil, "", clierror.New(
			clierror.CodeUsage,
			"no context selected",
			"use 'auth login --name <ctx>' to create one",
		)
	}

	ctx, ok := cfg.Contexts[ctxName]
	if !ok {
		return nil, "", clierror.New(
			clierror.CodeNotFound,
			fmt.Sprintf("context %q not found", ctxName),
			"run 'context list' to see available contexts",
		)
	}

	if tok := os.Getenv(prefix + "_TOKEN"); tok != "" {
		ctx.Token = tok
	}

	return &ctx, ctxName, nil
}
