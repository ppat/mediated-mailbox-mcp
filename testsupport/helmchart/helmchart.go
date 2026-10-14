// Package helmchart renders the chart under packaging/chart with the pinned helm and hands a test the rendered
// objects, so the chart's own tests and each deployable's test of the file the chart renders for it read the
// chart the same way (ADR-0052). A missing helm fails the test rather than skipping it, because a skipped test
// passes in the job that was meant to run it.
package helmchart

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// Object is one rendered Kubernetes object, decoded as YAML maps.
type Object = map[string]any

// RequiredValues is the values file, relative to the chart's directory, that sets every value the chart requires.
const RequiredValues = "ci/required-values.yaml"

// Render runs helm template over the chart at chartDir with the required values and args, and returns every
// object it renders. A render helm refuses fails the test with helm's output.
func Render(t *testing.T, chartDir string, args ...string) []Object {
	t.Helper()
	out, err := Template(t, chartDir, args...)
	if err != nil {
		t.Fatalf("helm template failed: %v\n%s", err, out)
	}
	return Decode(t, out)
}

// Template runs helm template over the chart at chartDir with the required values and args, and returns its
// combined output and the error a refused render returns.
func Template(t *testing.T, chartDir string, args ...string) (string, error) {
	t.Helper()
	path, err := exec.LookPath("helm")
	if err != nil {
		t.Fatalf("helm is not on PATH. Run the tests through mise: %v", err)
	}
	full := append([]string{"template", "release", chartDir, "--namespace", "mailbox", "-f", filepath.Join(chartDir, RequiredValues)}, args...)
	var out bytes.Buffer
	cmd := exec.CommandContext(t.Context(), path, full...)
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running helm: %v", err)
	}
	return out.String(), err
}

// Decode splits helm's output into its objects.
func Decode(t *testing.T, out string) []Object {
	t.Helper()
	var objects []Object
	dec := yaml.NewDecoder(bytes.NewBufferString(out))
	for {
		var o Object
		err := dec.Decode(&o)
		if errors.Is(err, io.EOF) {
			return objects
		}
		if err != nil {
			t.Fatalf("reading the rendered chart: %v\n%s", err, out)
		}
		if o != nil {
			objects = append(objects, o)
		}
	}
}

// ConfigFile returns the text of one process's configuration file from the chart's ConfigMap, such as
// api.yaml.
func ConfigFile(t *testing.T, objects []Object, file string) string {
	t.Helper()
	for _, o := range objects {
		if o["kind"] != "ConfigMap" {
			continue
		}
		data, ok := o["data"].(map[string]any)
		if !ok {
			continue
		}
		if text, ok := data[file].(string); ok {
			return text
		}
	}
	t.Fatalf("no ConfigMap of the rendered chart holds %s", file)
	return ""
}

// SchemaKeys returns the property names of the object at path in the chart's values schema, sorted, such as
// SchemaKeys(t, chartDir, "api", "config").
func SchemaKeys(t *testing.T, chartDir string, path ...string) []string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(chartDir, "values.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var node map[string]any
	if err := json.Unmarshal(src, &node); err != nil {
		t.Fatal(err)
	}
	for _, p := range path {
		props, ok := node["properties"].(map[string]any)
		if !ok {
			t.Fatalf("the values schema has no %s", strings.Join(path, "."))
		}
		next, ok := props[p].(map[string]any)
		if !ok {
			t.Fatalf("the values schema has no %s", strings.Join(path, "."))
		}
		node = next
	}
	props, ok := node["properties"].(map[string]any)
	if !ok {
		t.Fatalf("the values schema's %s has no properties", strings.Join(path, "."))
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// YAMLKeys returns the YAML names of a struct type's fields, the fields of a section it embeds inline
// among them, at the same level, as the configuration library reads them, sorted.
func YAMLKeys(typ reflect.Type) []string {
	keys := make([]string, 0, typ.NumField())
	for f := range typ.Fields() {
		if f.Anonymous && f.Tag.Get("yaml") == ",inline" {
			keys = append(keys, YAMLKeys(f.Type)...)
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		keys = append(keys, name)
	}
	slices.Sort(keys)
	return keys
}
