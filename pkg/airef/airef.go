// Package airef provides AI model reference helpers for the dotdevlabs *ctl CLIs.
package airef

import (
	"bytes"
	"encoding/json"
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
	// Enum lists the values the flag accepts (see SetFlagEnum).
	Enum []string `json:"enum,omitempty"`
	// Annotations carries the flag's other pflag annotations, multiple values
	// joined with ",". The required marker is surfaced as Required and the
	// reserved EnumAnnotation and SchemaAnnotation keys as Enum and Schema.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Schema is the JSON Schema of a flag whose value is JSON (see SetFlagSchema).
	Schema json.RawMessage `json:"schema,omitempty"`
}

// Reserved pflag annotation keys read by the reference builder.
const (
	// EnumAnnotation holds the flag's allowed values as a JSON array of strings.
	EnumAnnotation = "ctlkit.enum"
	// SchemaAnnotation holds the JSON Schema of the flag's JSON value.
	SchemaAnnotation = "ctlkit.schema"
)

// SetFlagEnum declares the allowed values of the named flag of cmd. values must
// be non-empty, without empty strings or duplicates.
func SetFlagEnum(cmd *cobra.Command, name string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("flag %q: enum must not be empty", name)
	}
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		if v == "" {
			return fmt.Errorf("flag %q: enum contains an empty value", name)
		}
		if seen[v] {
			return fmt.Errorf("flag %q: enum contains duplicate value %q", name, v)
		}
		seen[v] = true
	}
	encoded, _ := json.Marshal(values) // cannot fail for []string
	return setAnnotation(cmd, name, EnumAnnotation, []string{string(encoded)})
}

// SetFlagSchema declares the JSON Schema of the named flag's JSON value.
// schemaJSON must be valid JSON and a schema form (an object or a boolean).
func SetFlagSchema(cmd *cobra.Command, name, schemaJSON string) error {
	compact, ok := compactSchema(schemaJSON)
	if !ok {
		return fmt.Errorf("flag %q: schema must be a valid JSON object or boolean", name)
	}
	return setAnnotation(cmd, name, SchemaAnnotation, []string{compact})
}

// setAnnotation sets a pflag annotation on a local or persistent flag of cmd.
func setAnnotation(cmd *cobra.Command, name, key string, values []string) error {
	cmd.LocalFlags() // merges the persistent flags into cmd.Flags()
	return cmd.Flags().SetAnnotation(name, key, values)
}

// compactSchema returns the compacted schema and whether it is valid JSON that
// is an object or a boolean.
func compactSchema(s string) (string, bool) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err != nil {
		return "", false
	}
	c := buf.String()
	if !strings.HasPrefix(c, "{") && c != "true" && c != "false" {
		return "", false
	}
	return c, true
}

// CommandRef is the structured representation of a single command.
type CommandRef struct {
	Use         string       `json:"use"`
	Short       string       `json:"short"`
	Long        string       `json:"long,omitempty"`
	Flags       []FlagRef    `json:"flags,omitempty"`
	Subcommands []CommandRef `json:"subcommands,omitempty"`
	Examples    string       `json:"examples,omitempty"`
	// Annotations carries the command's cobra annotations verbatim, so a CLI can
	// attach metadata (e.g. API operation id, method, path).
	Annotations map[string]string `json:"annotations,omitempty"`
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
		if skipCommand(sub) {
			continue
		}
		ref.Commands = append(ref.Commands, buildCommandRef(sub))
	}
	return ref
}

// skipCommand reports whether cmd is left out of the reference: hidden
// commands and cobra's built-in help and completion commands.
func skipCommand(cmd *cobra.Command) bool {
	return cmd.Hidden || cmd.Name() == "help" || cmd.Name() == "completion"
}

func buildCommandRef(cmd *cobra.Command) CommandRef {
	ref := CommandRef{
		Use:      cmd.Use,
		Short:    cmd.Short,
		Long:     cmd.Long,
		Examples: cmd.Example,
		Flags:    extractFlags(cmd),
	}
	if len(cmd.Annotations) > 0 {
		ref.Annotations = cmd.Annotations
	}
	for _, sub := range cmd.Commands() {
		if skipCommand(sub) {
			continue
		}
		ref.Subcommands = append(ref.Subcommands, buildCommandRef(sub))
	}
	return ref
}

// extractFlags collects flag metadata from a command's local (non-inherited) flags.
func extractFlags(cmd *cobra.Command) []FlagRef {
	var refs []FlagRef
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		required := f.Annotations[cobra.BashCompOneRequiredFlag] != nil
		ref := FlagRef{
			Name:     f.Name,
			Type:     f.Value.Type(),
			Default:  f.DefValue,
			Usage:    f.Usage,
			Required: required,
		}
		for key, vals := range f.Annotations {
			switch key {
			case cobra.BashCompOneRequiredFlag:
			case EnumAnnotation:
				ref.Enum = parseEnum(vals)
			case SchemaAnnotation:
				if len(vals) == 1 {
					if c, ok := compactSchema(vals[0]); ok {
						ref.Schema = json.RawMessage(c)
					}
				}
			default:
				if ref.Annotations == nil {
					ref.Annotations = map[string]string{}
				}
				ref.Annotations[key] = strings.Join(vals, ",")
			}
		}
		refs = append(refs, ref)
	})
	return refs
}

// parseEnum decodes an EnumAnnotation value, returning nil when malformed.
func parseEnum(vals []string) []string {
	var out []string
	if len(vals) != 1 || json.Unmarshal([]byte(vals[0]), &out) != nil {
		return nil
	}
	return out
}

// NewCommand returns the "ai" cobra.Command that renders the reference. It
// prints JSON when the root command's --json persistent flag is set (read at
// run time), and markdown otherwise.
func NewCommand(product, ver string, workflows []Workflow) *cobra.Command {
	return &cobra.Command{
		Use:   "ai",
		Short: "Print AI-ingestible command reference",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ref := Build(cmd.Root(), product, ver, workflows)
			if jsonRequested(cmd) {
				return output.JSONTo(cmd.OutOrStdout(), ref)
			}
			return renderMarkdown(cmd.OutOrStdout(), ref)
		},
	}
}

// jsonRequested reports whether the --json flag is set on cmd or its root.
func jsonRequested(cmd *cobra.Command) bool {
	f := cmd.Flags().Lookup("json")
	if f == nil {
		f = cmd.Root().PersistentFlags().Lookup("json")
	}
	return f != nil && f.Value.String() == "true"
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
			usage := f.Usage
			if len(f.Enum) > 0 {
				usage += " Allowed: " + strings.Join(f.Enum, ", ")
			}
			m.printf("| --%s | %s | %s | %s | %s |\n", f.Name, f.Type, f.Default, req, usage)
		}
		m.println()
	}
	for _, sub := range cmd.Subcommands {
		renderCommandMarkdown(m, sub, depth+1)
	}
}
