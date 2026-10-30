package airef_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/airef"
)

func makeTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "testctl", Short: "Test CLI"}
	sub := &cobra.Command{Use: "tasks", Short: "Manage tasks"}
	sub.Flags().String("project-id", "", "Project ID")
	sub.Flags().Bool("verbose", false, "Verbose output")
	_ = sub.MarkFlagRequired("project-id")
	root.AddCommand(sub)
	return root
}

func TestBuildCommandRefs(t *testing.T) {
	root := makeTestRoot()
	ref := airef.Build(root, "testctl", "v1.0.0", nil)
	if ref.Product != "testctl" {
		t.Errorf("Product = %q", ref.Product)
	}
	if len(ref.Commands) == 0 {
		t.Fatal("expected at least one command")
	}
	var found bool
	for _, c := range ref.Commands {
		if c.Use == "tasks" {
			found = true
			if c.Short != "Manage tasks" {
				t.Errorf("Short = %q", c.Short)
			}
		}
	}
	if !found {
		t.Error("expected 'tasks' command in reference")
	}
}

func TestBuildFlagIntrospection(t *testing.T) {
	root := makeTestRoot()
	ref := airef.Build(root, "testctl", "v1.0.0", nil)

	var taskCmd *airef.CommandRef
	for i := range ref.Commands {
		if ref.Commands[i].Use == "tasks" {
			taskCmd = &ref.Commands[i]
		}
	}
	if taskCmd == nil {
		t.Fatal("tasks command not found")
	}

	var projectFlag, verboseFlag *airef.FlagRef
	for i := range taskCmd.Flags {
		switch taskCmd.Flags[i].Name {
		case "project-id":
			projectFlag = &taskCmd.Flags[i]
		case "verbose":
			verboseFlag = &taskCmd.Flags[i]
		}
	}

	if projectFlag == nil {
		t.Fatal("project-id flag not found")
	}
	if projectFlag.Type != "string" {
		t.Errorf("project-id type = %q, want string", projectFlag.Type)
	}
	if !projectFlag.Required {
		t.Error("project-id should be required")
	}

	if verboseFlag == nil {
		t.Fatal("verbose flag not found")
	}
	if verboseFlag.Type != "bool" {
		t.Errorf("verbose type = %q, want bool", verboseFlag.Type)
	}
}

func TestNewCommandJSON(t *testing.T) {
	root := makeTestRoot()
	var out, errOut bytes.Buffer
	_ = errOut
	root.PersistentFlags().Bool("json", false, "json")
	aiCmd := airef.NewCommand("testctl", "v1.0.0", nil)
	aiCmd.SetOut(&out)
	root.AddCommand(aiCmd)

	root.SetArgs([]string{"ai", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	var ref airef.Reference
	if err := json.Unmarshal(out.Bytes(), &ref); err != nil {
		t.Fatalf("output is not valid JSON: %v\ngot: %s", err, out.String())
	}
	if ref.Product != "testctl" {
		t.Errorf("Product = %q", ref.Product)
	}
}

func TestNewCommandMarkdown(t *testing.T) {
	root := makeTestRoot()
	var out, errOut bytes.Buffer
	_ = errOut
	root.PersistentFlags().Bool("json", false, "json")
	aiCmd := airef.NewCommand("testctl", "v1.0.0", []airef.Workflow{
		{Name: "List tasks", Description: "How to list tasks", Steps: []string{"Run tasks command"}},
	})
	aiCmd.SetOut(&out)
	root.AddCommand(aiCmd)

	root.SetArgs([]string{"ai"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	if !strings.Contains(got, "testctl Command Reference") {
		t.Errorf("missing title in output: %q", got)
	}
	if !strings.Contains(got, "Common Workflows") {
		t.Errorf("missing workflows section: %q", got)
	}
	if !strings.Contains(got, "List tasks") {
		t.Errorf("missing workflow name: %q", got)
	}
}

func TestWorkflowsInReference(t *testing.T) {
	root := makeTestRoot()
	workflows := []airef.Workflow{
		{Name: "Create task", Description: "Steps to create a task", Steps: []string{"Step 1", "Step 2"}},
	}
	ref := airef.Build(root, "testctl", "v1.0.0", workflows)
	if len(ref.Workflows) != 1 {
		t.Errorf("len(Workflows) = %d, want 1", len(ref.Workflows))
	}
	if ref.Workflows[0].Name != "Create task" {
		t.Errorf("workflow name = %q", ref.Workflows[0].Name)
	}
}

func TestAnnotations(t *testing.T) {
	root := makeTestRoot()
	root.Commands()[0].Annotations = map[string]string{"operation_id": "listTasks", "method": "GET"}
	root.AddCommand(&cobra.Command{Use: "plain", Short: "no annotations"})
	ref := airef.Build(root, "testctl", "v1", nil)
	for _, c := range ref.Commands {
		switch c.Use {
		case "tasks":
			if c.Annotations["operation_id"] != "listTasks" || c.Annotations["method"] != "GET" {
				t.Errorf("annotations = %v", c.Annotations)
			}
		case "plain":
			if c.Annotations != nil {
				t.Errorf("annotations = %v, want nil", c.Annotations)
			}
		}
	}
	b, _ := json.Marshal(ref)
	if !strings.Contains(string(b), `"annotations":{"method":"GET","operation_id":"listTasks"}`) {
		t.Errorf("json missing annotations: %s", b)
	}
	if strings.Count(string(b), `"annotations"`) != 1 {
		t.Errorf("annotations should be omitted when empty: %s", b)
	}
}

func TestHiddenHelpCompletionExcluded(t *testing.T) {
	root := makeTestRoot()
	root.AddCommand(&cobra.Command{Use: "secret", Hidden: true})
	root.AddCommand(&cobra.Command{Use: "help"})
	root.AddCommand(&cobra.Command{Use: "completion"})
	parent := &cobra.Command{Use: "parent"}
	parent.AddCommand(&cobra.Command{Use: "shown"}, &cobra.Command{Use: "nested-hidden", Hidden: true})
	root.AddCommand(parent)
	ref := airef.Build(root, "testctl", "v1", nil)
	var uses []string
	for _, c := range ref.Commands {
		uses = append(uses, c.Use)
		if c.Use == "parent" {
			if len(c.Subcommands) != 1 || c.Subcommands[0].Use != "shown" {
				t.Errorf("subcommands = %+v", c.Subcommands)
			}
		}
	}
	if got := strings.Join(uses, ","); got != "tasks,parent" && got != "parent,tasks" {
		t.Errorf("commands = %s", got)
	}
}

func TestNewCommandWithoutJSONFlagIsMarkdown(t *testing.T) {
	root := makeTestRoot()
	var out bytes.Buffer
	aiCmd := airef.NewCommand("testctl", "v1.0.0", nil)
	aiCmd.SetOut(&out)
	root.AddCommand(aiCmd)
	root.SetArgs([]string{"ai"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "testctl Command Reference") {
		t.Errorf("expected markdown, got %q", out.String())
	}
}

func flagTestCmd() *cobra.Command {
	root := &cobra.Command{Use: "testctl"}
	sub := &cobra.Command{Use: "tasks", Short: "Manage tasks"}
	sub.Flags().String("status", "", "Status")
	sub.Flags().String("filter", "", "Filter JSON")
	sub.Flags().String("plain", "", "Plain")
	sub.Flags().String("req", "", "Required")
	_ = sub.MarkFlagRequired("req")
	sub.PersistentFlags().String("pers", "", "Persistent")
	root.AddCommand(sub)
	return root
}

func TestSetFlagEnumValidation(t *testing.T) {
	sub, _, _ := flagTestCmd().Find([]string{"tasks"})
	if err := airef.SetFlagEnum(sub, "status", []string{"open", "done"}); err != nil {
		t.Fatal(err)
	}
	if err := airef.SetFlagEnum(sub, "pers", []string{"a"}); err != nil {
		t.Fatalf("persistent flag: %v", err)
	}
	for name, tc := range map[string]struct {
		flag string
		vals []string
	}{
		"missing": {"nope", []string{"a"}},
		"empty":   {"status", nil},
		"blank":   {"status", []string{"a", ""}},
		"dup":     {"status", []string{"a", "a"}},
	} {
		if err := airef.SetFlagEnum(sub, tc.flag, tc.vals); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestSetFlagSchemaValidation(t *testing.T) {
	sub, _, _ := flagTestCmd().Find([]string{"tasks"})
	if err := airef.SetFlagSchema(sub, "filter", "{ \"type\": \"object\" }"); err != nil {
		t.Fatal(err)
	}
	if got := sub.Flags().Lookup("filter").Annotations[airef.SchemaAnnotation][0]; got != `{"type":"object"}` {
		t.Errorf("schema not compacted: %s", got)
	}
	if err := airef.SetFlagSchema(sub, "filter", "true"); err != nil {
		t.Errorf("boolean schema: %v", err)
	}
	for _, bad := range []string{"", "{", `"x"`, "[1]", "42"} {
		if err := airef.SetFlagSchema(sub, "filter", bad); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
	if err := airef.SetFlagSchema(sub, "nope", "{}"); err == nil {
		t.Error("missing flag: expected error")
	}
}

func TestBuildFlagEnumAnnotationsSchema(t *testing.T) {
	root := flagTestCmd()
	sub, _, _ := root.Find([]string{"tasks"})
	if err := airef.SetFlagEnum(sub, "status", []string{"open", "a,b"}); err != nil {
		t.Fatal(err)
	}
	if err := airef.SetFlagSchema(sub, "filter", `{"type":"object"}`); err != nil {
		t.Fatal(err)
	}
	_ = sub.Flags().SetAnnotation("status", "custom", []string{"x", "y"})
	// Malformed hand-set reserved annotations are skipped.
	_ = sub.Flags().SetAnnotation("plain", airef.EnumAnnotation, []string{"not json"})
	_ = sub.Flags().SetAnnotation("plain", airef.SchemaAnnotation, []string{"{broken"})
	_ = sub.Flags().SetAnnotation("pers", airef.SchemaAnnotation, []string{"{}", "{}"})

	ref := airef.Build(root, "testctl", "v1", nil)
	b, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	flags := map[string]airef.FlagRef{}
	for _, f := range ref.Commands[0].Flags {
		flags[f.Name] = f
	}
	st := flags["status"]
	if len(st.Enum) != 2 || st.Enum[1] != "a,b" || st.Annotations["custom"] != "x,y" || len(st.Annotations) != 1 {
		t.Errorf("status = %+v", st)
	}
	if string(flags["filter"].Schema) != `{"type":"object"}` {
		t.Errorf("schema = %s", flags["filter"].Schema)
	}
	if r := flags["req"]; !r.Required || r.Annotations != nil {
		t.Errorf("req = %+v", r)
	}
	for _, n := range []string{"plain", "pers"} {
		if f := flags[n]; f.Enum != nil || f.Schema != nil || f.Annotations != nil {
			t.Errorf("%s = %+v", n, f)
		}
	}

	var raw struct {
		Commands []struct {
			Flags []map[string]json.RawMessage `json:"flags"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, f := range raw.Commands[0].Flags {
		var name string
		_ = json.Unmarshal(f["name"], &name)
		for _, k := range []string{"name", "type", "default", "usage", "required"} {
			if _, ok := f[k]; !ok {
				t.Errorf("%s missing %s", name, k)
			}
		}
		_, hasEnum := f["enum"]
		_, hasSchema := f["schema"]
		_, hasAnn := f["annotations"]
		switch name {
		case "status":
			if !hasEnum || hasSchema || !hasAnn {
				t.Errorf("status keys: %v", f)
			}
		case "filter":
			if hasEnum || !hasSchema || hasAnn {
				t.Errorf("filter keys: %v", f)
			}
		default:
			if hasEnum || hasSchema || hasAnn {
				t.Errorf("%s has new keys: %v", name, f)
			}
		}
	}
}

func TestMarkdownAllowedValues(t *testing.T) {
	root := flagTestCmd()
	sub, _, _ := root.Find([]string{"tasks"})
	_ = airef.SetFlagEnum(sub, "status", []string{"open", "done"})
	root.AddCommand(airef.NewCommand("testctl", "v1", nil))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"ai"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "| Status Allowed: open, done |") {
		t.Errorf("missing allowed values:\n%s", s)
	}
	if !strings.Contains(s, "| Plain |") {
		t.Errorf("plain usage changed:\n%s", s)
	}
}
