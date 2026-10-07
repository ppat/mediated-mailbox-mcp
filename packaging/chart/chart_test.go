// The chart's render tests. Each renders the chart with helm, as the pinned tool runs it, and reads the
// rendered objects, so a test sees what a cluster would be given. What each object must hold is
// written here, never read from the chart. A cluster's view of the same objects is the chainsaw
// suite's, under packaging/tests/chainsaw.
package chart_test

import (
	"bytes"
	"errors"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.yaml.in/yaml/v3"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The Secrets the inputs name, one per secret the chart mounts.
const (
	keysSecret    = "sealing-keys"
	migrateSecret = "database-migrate"
)

// inputs are the chart's required inputs, every Secret named apart, so a test can tell which pod
// mounts which.
var inputs = map[string]any{
	"database.host":                   "postgres",
	"database.name":                   "mailbox",
	"keys.secretName":                 keysSecret,
	"migrate.passwordFileSecret.name": migrateSecret,
	"mediate.passwordSecret.name":     "database-mediate",
	"mediate.tokenSecret.name":        "bearer-token",
	"mediate.tls.secretName":          "tls-mediate",
	"backfill.passwordSecret.name":    "database-backfill",
	"sync.passwordSecret.name":        "database-sync",
	"ui.passwordSecret.name":          "database-ui",
	"ui.tls.secretName":               "tls-ui",
}

// object is one rendered object, read as the generic tree YAML decodes to.
type object map[string]any

// get walks a path of map keys, returning nil where the path ends early.
func (o object) get(path ...string) any {
	var v any = map[string]any(o)
	for _, k := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

func (o object) str(path ...string) string {
	if s, ok := o.get(path...).(string); ok {
		return s
	}
	return ""
}

func (o object) kind() string { return o.str("kind") }
func (o object) name() string { return o.str("metadata", "name") }

// component is the component label the chart gives an object.
func (o object) component() string { return o.str("metadata", "labels", "app.kubernetes.io/component") }

// at is an object's kind and component, how the tests name it.
func (o object) at() string { return o.kind() + " " + o.component() }

// podTemplate is the pod template an object runs, its metadata and spec, wherever its kind keeps
// it, and nil for an object that runs no pod.
func (o object) podTemplate() object {
	var tmpl any
	switch o.kind() {
	case "Pod":
		tmpl = map[string]any(o)
	case "Deployment", "StatefulSet", "Job":
		tmpl = o.get("spec", "template")
	case "CronJob":
		tmpl = o.get("spec", "jobTemplate", "spec", "template")
	}
	if m, ok := tmpl.(map[string]any); ok {
		return m
	}
	return nil
}

// podSpec is the pod spec an object runs, and nil for an object that runs no pod.
func (o object) podSpec() object {
	if m, ok := o.podTemplate().get("spec").(map[string]any); ok {
		return m
	}
	return nil
}

// list returns the value at path as a list of objects.
func (o object) list(path ...string) []object {
	items, ok := o.get(path...).([]any)
	if !ok {
		return nil
	}
	out := make([]object, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// mountedSecrets returns the Secrets a pod spec's containers mount, by name. A volume that no
// container mounts reaches no process, so it is not counted.
func mountedSecrets(spec object) []string {
	mounted := map[string]bool{}
	for _, c := range spec.list("containers") {
		for _, m := range c.list("volumeMounts") {
			mounted[m.str("name")] = true
		}
	}
	var out []string
	for _, v := range spec.list("volumes") {
		if s := v.str("secret", "secretName"); s != "" && mounted[v.str("name")] {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}

// run runs a pinned tool and returns its combined output. A missing tool fails the test rather than
// skipping it, because a skipped test passes in the job that was meant to run it.
func run(t *testing.T, tool string, args ...string) (string, error) {
	t.Helper()
	path, err := exec.LookPath(tool)
	if err != nil {
		t.Fatalf("%s is not on PATH. Run the tests through mise: %v", tool, err)
	}
	var out bytes.Buffer
	cmd := exec.CommandContext(t.Context(), path, args...)
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running %s: %v", tool, err)
	}
	return out.String(), err
}

// render renders the chart with the inputs, overridden and extended by values keyed by their dotted
// paths, and returns every object, the Helm tests included.
func render(t *testing.T, values map[string]any) []object {
	t.Helper()
	set := maps.Clone(inputs)
	maps.Copy(set, values)
	tree := map[string]any{}
	for _, key := range slices.Sorted(maps.Keys(set)) {
		node := tree
		parts := strings.Split(key, ".")
		for _, p := range parts[:len(parts)-1] {
			next, ok := node[p].(map[string]any)
			if !ok {
				next = map[string]any{}
				node[p] = next
			}
			node = next
		}
		node[parts[len(parts)-1]] = set[key]
	}
	text, err := yaml.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "values.yaml")
	if err := os.WriteFile(file, text, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, "helm", "template", "release", ".", "--values", file)
	if err != nil {
		t.Fatalf("helm template failed: %v\n%s", err, out)
	}
	var objects []object
	dec := yaml.NewDecoder(strings.NewReader(out))
	for {
		var o map[string]any
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

// reach maps each object that runs a pod to whether its pod mounts the Secret.
func reach(objects []object, secret string) map[string]bool {
	out := map[string]bool{}
	for _, o := range objects {
		if spec := o.podSpec(); spec != nil {
			out[o.at()] = slices.Contains(mountedSecrets(spec), secret)
		}
	}
	return out
}

// VERIFICATIONS' row for the private key's reach, its chart part. The key pair reaches the pods of the
// deployables that open a credential, the mediator, backfill however it is started, and delta sync,
// and the UI, whose one client-secret part opens with it. It reaches no other pod the chart runs, the
// migration step and the Helm tests included (ADR-0079, ADR-0081).
func TestThePrivateKeyReachesOnlyTheDeployablesThatOpenCredentials(t *testing.T) {
	want := map[string]bool{
		"Job migrate":        false,
		"Deployment mediate": true,
		"Job backfill":       true,
		"CronJob backfill":   true,
		"Deployment sync":    true,
		"Deployment ui":      true,
		"Pod mediate-test":   false,
		"Pod ui-test":        false,
		"Pod sync-test":      false,
	}
	if diff := cmp.Diff(want, reach(render(t, nil), keysSecret), compare.Options); diff != "" {
		t.Errorf("which pods mount the key pair (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for the migration role's credential. It reaches the migration step and no other
// pod, and the migration step mounts nothing else but the database's CA, so a runtime role's
// password, the key pair and every other secret stay out of the one container holding the
// schema-owning role (ADR-0048, ADR-0079, ADR-0115).
func TestTheMigrationCredentialReachesOnlyTheMigrationStep(t *testing.T) {
	objects := render(t, map[string]any{"database.caSecret": "database-ca"})
	for at, mounts := range reach(objects, migrateSecret) {
		if mounts != (at == "Job migrate") {
			t.Errorf("%s mounts the migration role's credential: %t", at, mounts)
		}
	}
	for _, o := range objects {
		if o.at() != "Job migrate" {
			continue
		}
		if diff := cmp.Diff([]string{"database-ca", migrateSecret}, mountedSecrets(o.podSpec()), compare.Options); diff != "" {
			t.Errorf("the Secrets the migration step mounts (-want +got):\n%s", diff)
		}
		for _, c := range o.podSpec().list("containers") {
			for _, e := range c.list("env") {
				if e.get("valueFrom") != nil {
					t.Errorf("the migration step takes %s from another object rather than from its mounted file", e.str("name"))
				}
			}
		}
	}
}

// VERIFICATIONS' row for the migration step running first. The migration Job is a hook Helm runs to
// completion before an install's and an upgrade's other objects, so no deployable rolls before the
// chain has run, and it is the only object the chart runs that way (ADR-0048, ADR-0115).
func TestTheMigrationStepRunsBeforeTheDeployables(t *testing.T) {
	hooks := map[string]string{}
	for _, o := range render(t, nil) {
		if h := o.str("metadata", "annotations", "helm.sh/hook"); h != "" && h != "test" {
			hooks[o.at()] = h
		}
	}
	want := map[string]string{"Job migrate": "pre-install,pre-upgrade"}
	if diff := cmp.Diff(want, hooks, compare.Options); diff != "" {
		t.Errorf("the objects Helm runs before the others (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for delta sync's one process. The chart runs it only as a Deployment of one
// replica whose Recreate strategy ends the old pod before it starts the new one, so a rollout never
// runs two (ADR-0103).
func TestDeltaSyncRunsAsOneProcessAtATime(t *testing.T) {
	var runs []string
	for _, o := range render(t, map[string]any{"mediate.replicaCount": 3, "ui.replicaCount": 2}) {
		spec := o.podSpec()
		if spec == nil {
			continue
		}
		for _, c := range spec.list("containers") {
			if !strings.Contains(c.str("image"), "/mediated-mailbox-sync:") {
				continue
			}
			runs = append(runs, o.kind())
			if r, ok := o.get("spec", "replicas").(int); !ok || r != 1 {
				t.Errorf("delta sync's %s has %v replicas, want 1", o.kind(), o.get("spec", "replicas"))
			}
			if s := o.str("spec", "strategy", "type"); s != "Recreate" {
				t.Errorf("delta sync's %s rolls out by %q, want Recreate", o.kind(), s)
			}
		}
	}
	if diff := cmp.Diff([]string{"Deployment"}, runs, compare.Options); diff != "" {
		t.Errorf("the objects that run delta sync (-want +got):\n%s", diff)
	}
}

// backfillJob returns the name of the Job that runs backfill with no manual step.
func backfillJob(t *testing.T, objects []object) string {
	t.Helper()
	var names []string
	for _, o := range objects {
		if o.at() == "Job backfill" {
			names = append(names, o.name())
		}
	}
	if len(names) != 1 {
		t.Fatalf("the chart renders %d Jobs running backfill, want one: %v", len(names), names)
	}
	return names[0]
}

// VERIFICATIONS' row for backfill's runs. The Job that runs backfill with no manual step takes a new
// name, which an upgrade runs, when a release changes the images and when the scanner's section
// changes, and keeps its name, so an upgrade runs nothing, when a change reaches only another
// deployable. The CronJob it is started from by hand is never scheduled (ADR-0022, ADR-0096,
// ADR-0098, ADR-0116).
func TestBackfillRunsAgainAfterAReleaseOrAScannerChangeOnly(t *testing.T) {
	base := backfillJob(t, render(t, nil))
	for change, values := range map[string]map[string]any{
		"a release":             {"backfill.image.tag": "9.9.9"},
		"the scanner's section": {"scanner": map[string]any{"triggers": map[string]any{"en": []string{"verify"}}}},
	} {
		if got := backfillJob(t, render(t, values)); got == base {
			t.Errorf("after %s the Job running backfill keeps its name %s, so no upgrade runs it", change, got)
		}
	}
	for change, values := range map[string]map[string]any{
		"the UI's configuration":     {"ui.config": map[string]any{"operator_name": "someone"}},
		"the mediator's replicas":    {"mediate.replicaCount": 2},
		"delta sync's configuration": {"sync.config": map[string]any{"first_window": "72h"}},
	} {
		if got := backfillJob(t, render(t, values)); got != base {
			t.Errorf("after a change of %s the Job running backfill is renamed %s from %s, so an upgrade runs it", change, got, base)
		}
	}
	for _, o := range render(t, nil) {
		if o.at() == "CronJob backfill" && o.get("spec", "suspend") != true {
			t.Errorf("backfill's CronJob is scheduled, suspend is %v", o.get("spec", "suspend"))
		}
	}
}

// VERIFICATIONS' row for the pod hardening. Every pod the chart runs, the migration step, the UI and
// the Helm tests included, runs as a non-root user under the runtime's seccomp profile with no
// service account token and no injected service variables, and every container runs with a
// read-only root filesystem, no privilege escalation and every capability dropped, unless a value
// overrides them (ADR-0028, ADR-0052, ADR-0078).
func TestEveryPodRunsHardened(t *testing.T) {
	pods := 0
	for _, o := range render(t, nil) {
		spec := o.podSpec()
		if spec == nil {
			continue
		}
		pods++
		for path, want := range map[string]any{
			"automountServiceAccountToken":        false,
			"enableServiceLinks":                  false,
			"securityContext.runAsNonRoot":        true,
			"securityContext.seccompProfile.type": "RuntimeDefault",
		} {
			if got := spec.get(strings.Split(path, ".")...); got != want {
				t.Errorf("%s: %s is %v, want %v", o.at(), path, got, want)
			}
		}
		for _, c := range spec.list("containers") {
			for path, want := range map[string]any{
				"securityContext.allowPrivilegeEscalation": false,
				"securityContext.readOnlyRootFilesystem":   true,
				"securityContext.privileged":               false,
			} {
				if got := c.get(strings.Split(path, ".")...); got != want {
					t.Errorf("%s: container %s: %s is %v, want %v", o.at(), c.str("name"), path, got, want)
				}
			}
			if drop, ok := c.get("securityContext", "capabilities", "drop").([]any); !ok || !slices.Contains(drop, any("ALL")) {
				t.Errorf("%s: container %s drops %v, want ALL", o.at(), c.str("name"), drop)
			}
		}
	}
	if pods != 9 {
		t.Errorf("the chart runs %d pods, want the nine the other tests name", pods)
	}
}

// VERIFICATIONS' row for nowhere to spill. No pod the chart runs mounts a writable volume: every volume
// is a Secret or the configuration ConfigMap, both read-only, so with the read-only root filesystem a
// process holding a body has nowhere to write it, and a spill fails loudly (ADR-0009).
func TestNoPodMountsAWritableVolume(t *testing.T) {
	for _, o := range render(t, map[string]any{"database.caSecret": "database-ca", "ui.tokenKeySecret.name": "token-key"}) {
		spec := o.podSpec()
		if spec == nil {
			continue
		}
		for _, v := range spec.list("volumes") {
			if v.get("secret") == nil && v.get("configMap") == nil {
				t.Errorf("%s mounts volume %s, which is neither a Secret nor a ConfigMap: %v", o.at(), v.str("name"), map[string]any(v))
			}
		}
	}
}

// The objects that run a deployable, each with the deployable whose values configure it.
var deployables = []struct{ object, component string }{
	{"Deployment mediate", "mediate"},
	{"Job backfill", "backfill"},
	{"CronJob backfill", "backfill"},
	{"Deployment sync", "sync"},
	{"Deployment ui", "ui"},
}

// rendered is what one render gives the configuration: each deployable's file, decoded, and, per
// object running a deployable, the checksum its pod template carries and the ConfigMap keys its pod
// mounts.
type rendered struct {
	files      map[string]map[string]any
	checksums  map[string]string
	mounted    map[string][]string
	configMaps int
}

func configOf(t *testing.T, objects []object) rendered {
	t.Helper()
	r := rendered{files: map[string]map[string]any{}, checksums: map[string]string{}, mounted: map[string][]string{}}
	for _, o := range objects {
		if o.kind() != "ConfigMap" {
			continue
		}
		r.configMaps++
		data, ok := o.get("data").(map[string]any)
		if !ok {
			continue
		}
		for key, text := range data {
			body, ok := text.(string)
			if !ok {
				t.Fatalf("%s is not text: %v", key, text)
			}
			var file map[string]any
			if err := yaml.Unmarshal([]byte(body), &file); err != nil {
				t.Fatalf("%s: %v", key, err)
			}
			r.files[strings.TrimSuffix(key, ".yaml")] = file
		}
	}
	for _, o := range objects {
		tmpl := o.podTemplate()
		if tmpl == nil || o.kind() == "Pod" {
			continue
		}
		r.checksums[o.at()] = tmpl.str("metadata", "annotations", "checksum/config")
		for _, v := range o.podSpec().list("volumes") {
			if v.get("configMap") == nil {
				continue
			}
			for _, item := range v.list("configMap", "items") {
				r.mounted[o.at()] = append(r.mounted[o.at()], item.str("key"))
			}
		}
	}
	return r
}

// keysOf returns a file's top-level keys, sorted.
func keysOf(file map[string]any) []string { return slices.Sorted(maps.Keys(file)) }

// VERIFICATIONS' row for the configuration. One ConfigMap holds a file per deployable, rendered from
// the structured values, each holding exactly the keys its deployable declares that the values set:
// its own section, the database, its key files, the shared scanner section for the three that mask or
// scan, and the paths of what the chart mounts. A word YAML would read as a boolean stays the string
// the values gave. Each object running a deployable mounts only its own file, and its checksum
// changes with that file and with nothing else (ADR-0052, ADR-0078, ADR-0096).
func TestEachDeployableReadsOnlyItsOwnRenderedFile(t *testing.T) {
	values := map[string]any{
		"scanner":        map[string]any{"triggers": map[string]any{"no": []string{"no", "verify"}}},
		"mediate.config": map[string]any{"provider_timeout": "20s"},
		"sync.config":    map[string]any{"sync_interval": "30s", "decisions_per_tick": 50},
		"ui.config":      map[string]any{"operator_name": "someone"},
	}
	base := configOf(t, render(t, values))
	if base.configMaps != 1 {
		t.Errorf("the chart renders %d ConfigMaps, want one", base.configMaps)
	}
	want := map[string][]string{
		"mediate":  {"credential", "database", "provider_timeout", "scanner", "tls_cert", "tls_key", "token_file"},
		"backfill": {"credential", "database", "scanner"},
		"sync":     {"credential", "database", "decisions_per_tick", "scanner", "sync_interval"},
		"ui":       {"database", "operator_name", "private_key_files", "seal_public_key_file", "sync_interval", "tls_cert", "tls_key"},
	}
	got := map[string][]string{}
	for component, file := range base.files {
		got[component] = keysOf(file)
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the keys of each deployable's file (-want +got):\n%s", diff)
	}
	for _, component := range []string{"mediate", "backfill", "sync"} {
		triggers := object(base.files[component]).get("scanner", "triggers", "no")
		if diff := cmp.Diff([]any{"no", "verify"}, triggers, compare.Options); diff != "" {
			t.Errorf("%s's scanner triggers for no (-want +got):\n%s", component, diff)
		}
	}
	for _, d := range deployables {
		if diff := cmp.Diff([]string{d.component + ".yaml"}, base.mounted[d.object], compare.Options); diff != "" {
			t.Errorf("%s mounts these files of the ConfigMap (-want +got):\n%s", d.object, diff)
		}
		if base.checksums[d.object] == "" {
			t.Errorf("%s carries no configuration checksum", d.object)
		}
	}

	for change, edit := range map[string]struct {
		values  map[string]any
		changes []string
	}{
		"the mediator's own key":  {map[string]any{"mediate.config": map[string]any{"provider_timeout": "25s"}}, []string{"mediate"}},
		"delta sync's tick count": {map[string]any{"sync.config": map[string]any{"sync_interval": "30s", "decisions_per_tick": 60}}, []string{"sync"}},
		"delta sync's interval":   {map[string]any{"sync.config": map[string]any{"sync_interval": "1m", "decisions_per_tick": 50}}, []string{"sync", "ui"}},
		"the UI's own key":        {map[string]any{"ui.config": map[string]any{"operator_name": "another"}}, []string{"ui"}},
		"the scanner's section":   {map[string]any{"scanner": map[string]any{"window": 6}}, []string{"mediate", "backfill", "sync"}},
		"the database's host":     {map[string]any{"database.host": "elsewhere"}, []string{"mediate", "backfill", "sync", "ui"}},
	} {
		edited := maps.Clone(values)
		maps.Copy(edited, edit.values)
		after := configOf(t, render(t, edited))
		for _, d := range deployables {
			moved := after.checksums[d.object] != base.checksums[d.object]
			if want := slices.Contains(edit.changes, d.component); moved != want {
				t.Errorf("a change of %s: %s's checksum changed %t, want %t", change, d.object, moved, want)
			}
		}
	}
}

// VERIFICATIONS' row for exposing the surfaces. The mediator and the UI each render an Ingress and a
// Gateway API HTTPRoute only when its value switches it on, so by default the chart needs nothing
// beyond core Kubernetes, and each points at its own component's Service and port (ADR-0052).
func TestIngressAndRoutesRenderOnlyWhenSwitchedOn(t *testing.T) {
	for _, o := range render(t, nil) {
		if o.kind() == "Ingress" || o.kind() == "HTTPRoute" {
			t.Errorf("the chart with its defaults renders %s %s", o.kind(), o.name())
		}
	}
	values := map[string]any{"mediate.service.port": 8443, "ui.service.port": 9443}
	for _, component := range []string{"mediate", "ui"} {
		values[component+".ingress.enabled"] = true
		values[component+".httpRoute.enabled"] = true
	}
	services := map[string]string{}
	objects := render(t, values)
	for _, o := range objects {
		if o.kind() == "Service" {
			services[o.component()] = o.name()
		}
	}
	got := map[string]string{}
	for _, o := range objects {
		switch o.kind() {
		case "Ingress":
			for _, rule := range o.list("spec", "rules") {
				for _, p := range rule.list("http", "paths") {
					got[o.at()] = p.str("backend", "service", "name") + ":" + intString(p.get("backend", "service", "port", "number"))
				}
			}
		case "HTTPRoute":
			for _, rule := range o.list("spec", "rules") {
				for _, ref := range rule.list("backendRefs") {
					got[o.at()] = ref.str("name") + ":" + intString(ref.get("port"))
				}
			}
		}
	}
	want := map[string]string{
		"Ingress mediate":   services["mediate"] + ":8443",
		"HTTPRoute mediate": services["mediate"] + ":8443",
		"Ingress ui":        services["ui"] + ":9443",
		"HTTPRoute ui":      services["ui"] + ":9443",
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("where each Ingress and HTTPRoute sends its traffic (-want +got):\n%s", diff)
	}
}

func intString(v any) string {
	if n, ok := v.(int); ok {
		return strconv.Itoa(n)
	}
	return "?"
}
