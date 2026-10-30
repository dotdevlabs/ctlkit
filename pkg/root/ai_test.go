package root_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/dotdevlabs/ctlkit/pkg/airef"
)

func sampleTree() *cobra.Command {
	tasks := &cobra.Command{
		Use:   "tasks",
		Short: "Manage tasks",
		Annotations: map[string]string{
			"operation_id": "listTasks", "method": "GET", "path": "/tasks",
		},
	}
	tasks.Flags().String("project-id", "", "Project ID")
	_ = tasks.MarkFlagRequired("project-id")
	tasks.AddCommand(&cobra.Command{Use: "internal", Hidden: true})
	secret := &cobra.Command{Use: "secret", Hidden: true}
	return makeRoot(tasks, secret)
}

func runAI(t *testing.T, args ...string) string {
	t.Helper()
	r := sampleTree()
	var out bytes.Buffer
	r.SetOut(&out)
	r.SetErr(&bytes.Buffer{})
	r.SetArgs(args)
	if err := r.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestAIJSON(t *testing.T) {
	for _, args := range [][]string{{"ai", "--json"}, {"--json", "ai"}} {
		out := runAI(t, args...)
		var ref airef.Reference
		if err := json.Unmarshal([]byte(out), &ref); err != nil {
			t.Fatalf("%v: invalid JSON: %v\n%s", args, err, out)
		}
		if ref.Product != "testctl" {
			t.Errorf("product = %q", ref.Product)
		}
		uses := map[string]airef.CommandRef{}
		for _, c := range ref.Commands {
			uses[c.Use] = c
		}
		for _, absent := range []string{"secret", "help", "completion"} {
			if _, ok := uses[absent]; ok {
				t.Errorf("%s should be excluded", absent)
			}
		}
		for _, present := range []string{"ai", "tasks"} {
			if _, ok := uses[present]; !ok {
				t.Errorf("%s missing", present)
			}
		}
		tasks := uses["tasks"]
		if len(tasks.Subcommands) != 0 {
			t.Errorf("hidden nested command included: %+v", tasks.Subcommands)
		}
		if tasks.Annotations["operation_id"] != "listTasks" || tasks.Annotations["path"] != "/tasks" {
			t.Errorf("annotations = %v", tasks.Annotations)
		}
		var req bool
		for _, f := range tasks.Flags {
			if f.Name == "project-id" {
				req = f.Required
			}
		}
		if !req {
			t.Error("project-id not marked required")
		}

		var raw map[string]any
		if err := json.Unmarshal([]byte(out), &raw); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"product", "version", "commands", "workflows"} {
			if _, ok := raw[k]; !ok {
				t.Errorf("missing key %q", k)
			}
		}
	}
}

func TestAIMarkdown(t *testing.T) {
	out := runAI(t, "ai")
	if !strings.Contains(out, "testctl Command Reference") {
		t.Errorf("expected markdown, got %q", out)
	}
	if json.Valid([]byte(out)) {
		t.Error("output should not be JSON")
	}
	if strings.Contains(out, "secret") {
		t.Error("hidden command rendered")
	}
}
