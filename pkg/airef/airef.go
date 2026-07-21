// Package airef provides AI model reference helpers for the dotdevlabs *ctl CLIs.
package airef

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/dotdevlabs/ctlkit/pkg/output"
)

// Workflow is a hand-authored common workflow for the AI reference.
type Workflow struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
}

// FlagRef is the structured representation of a single flag.
type FlagRef struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Default  string `json:"default"`
	Usage    string `json:"usage"`
	Required bool   `json:"required"`
}

// CommandRef is the structured representation of a single command.
type CommandRef struct {
	Use         string       `json:"use"`
	Short       string       `json:"short"`
	Long        string       `json:"long,omitempty"`
	Flags       []FlagRef    `json:"flags,omitempty"`
	Subcommands []CommandRef `json:"subcommands,omitempty"`
	Examples    string       `json:"examples,omitempty"`
}

// Reference is the top-level AI reference document.
type Reference struct {
	Product   string       `json:"product"`
	Version   string       `json:"version"`
	Commands  []CommandRef `json:"commands"`
	Workflows []Workflow   `json:"workflows"`
}

// Build walks root and returns a Reference.
func Build(root *cobra.Command, product, ver string, workflows []Workflow) Reference {
	ref := Reference{
		Product:   product,
		Version:   ver,
		Workflows: workflows,
	}
	for _, sub := range root.Commands() {
		ref.Commands = append(ref.Commands, buildCommandRef(sub))
	}
	return ref
}

func buildCommandRef(cmd *cobra.Command) CommandRef {
	ref := CommandRef{
		Use:      cmd.Use,
		Short:    cmd.Short,
		Long:     cmd.Long,
		Examples: cmd.Example,
		Flags:    extractFlags(cmd),
	}
	for _, sub := range cmd.Commands() {
		ref.Subcommands = append(ref.Subcommands, buildCommandRef(sub))
	}
	return ref
}

// extractFlags collects flag metadata from a command's local (non-inherited) flags.
func extractFlags(cmd *cobra.Command) []FlagRef {
	var refs []FlagRef
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		required := f.Annotations[cobra.BashCompOneRequiredFlag] != nil
		refs = append(refs, FlagRef{
			Name:     f.Name,
			Type:     f.Value.Type(),
			Default:  f.DefValue,
			Usage:    f.Usage,
			Required: required,
		})
	})
	return refs
}

// NewCommand returns the "ai" cobra.Command that renders the reference.
func NewCommand(product, ver string, workflows []Workflow, r *output.Renderer) *cobra.Command {
	return &cobra.Command{
		Use:   "ai",
		Short: "Print AI-ingestible command reference",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := cmd.Root()
			ref := Build(root, product, ver, workflows)
			if r != nil && r.Mode == output.ModeJSON {
				return output.JSONTo(cmd.OutOrStdout(), ref)
			}
			return renderMarkdown(cmd.OutOrStdout(), ref)
		},
	}
}

// mw is a helper to write to w and capture the first error.
type mw struct {
	w   io.Writer
	err error
}

func (m *mw) printf(format string, args ...any) {
	if m.err != nil {
		return
	}
	_, m.err = fmt.Fprintf(m.w, format, args...)
}

func (m *mw) println() {
	if m.err != nil {
		return
	}
	_, m.err = fmt.Fprintln(m.w)
}

func renderMarkdown(w io.Writer, ref Reference) error {
	m := &mw{w: w}
	m.printf("# %s Command Reference\n\n", ref.Product)
	m.printf("Generated: %s  Version: %s\n\n", time.Now().Format("2006-01-02"), ref.Version)

	for _, cmd := range ref.Commands {
		renderCommandMarkdown(m, cmd, 2)
	}

	if len(ref.Workflows) > 0 {
		m.printf("## Common Workflows\n\n")
		for _, wf := range ref.Workflows {
			m.printf("### %s\n\n%s\n\n", wf.Name, wf.Description)
			for i, step := range wf.Steps {
				m.printf("%d. %s\n", i+1, step)
			}
			m.println()
		}
	}
	return m.err
}

func renderCommandMarkdown(m *mw, cmd CommandRef, depth int) {
	heading := strings.Repeat("#", depth)
	m.printf("%s %s\n\n", heading, cmd.Use)
	if cmd.Short != "" {
		m.printf("%s\n\n", cmd.Short)
	}
	if len(cmd.Flags) > 0 {
		m.printf("**Flags:**\n\n")
		m.printf("| Flag | Type | Default | Required | Description |\n")
		m.printf("|------|------|---------|----------|-------------|\n")
		for _, f := range cmd.Flags {
			req := ""
			if f.Required {
				req = "yes"
			}
			m.printf("| --%s | %s | %s | %s | %s |\n", f.Name, f.Type, f.Default, req, f.Usage)
		}
		m.println()
	}
	for _, sub := range cmd.Subcommands {
		renderCommandMarkdown(m, sub, depth+1)
	}
}
