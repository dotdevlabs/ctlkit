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

func TestAIJSONFlagEnumAnnotationsSchema(t *testing.T) {
	tasks := &cobra.Command{Use: "tasks", Short: "Manage tasks"}
	tasks.Flags().String("status", "", "Task status")
	tasks.Flags().String("filter", "", "Filter JSON")
	tasks.Flags().String("plain", "", "Plain")
	if err := airef.SetFlagEnum(tasks, "status", []string{"open", "done"}); err != nil {
		t.Fatal(err)
	}
	if err := airef.SetFlagSchema(tasks, "filter", `{"type":"object","properties":{"id":{"type":"integer"}}}`); err != nil {
		t.Fatal(err)
	}
	_ = tasks.Flags().SetAnnotation("status", "api.field", []string{"status"})
	r := makeRoot(tasks)
	var out bytes.Buffer
	r.SetOut(&out)
	r.SetErr(&bytes.Buffer{})
	r.SetArgs([]string{"ai", "--json"})
	if err := r.Execute(); err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Commands []struct {
			Use   string                       `json:"use"`
			Flags []map[string]json.RawMessage `json:"flags"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	for _, c := range raw.Commands {
		if c.Use != "tasks" {
			continue
		}
		got := map[string]map[string]json.RawMessage{}
		for _, f := range c.Flags {
			var n string
			_ = json.Unmarshal(f["name"], &n)
			got[n] = f
		}
		compact := func(m json.RawMessage) string {
			var b bytes.Buffer
			_ = json.Compact(&b, m)
			return b.String()
		}
		if compact(got["status"]["enum"]) != `["open","done"]` || compact(got["status"]["annotations"]) != `{"api.field":"status"}` {
			t.Errorf("status = %v", got["status"])
		}
		if compact(got["filter"]["schema"]) != `{"type":"object","properties":{"id":{"type":"integer"}}}` {
			t.Errorf("filter = %v", got["filter"])
		}
		for _, k := range []string{"enum", "annotations", "schema"} {
			if _, ok := got["plain"][k]; ok {
				t.Errorf("plain has %s", k)
			}
		}
		return
	}
	t.Fatal("tasks command not found")
}
