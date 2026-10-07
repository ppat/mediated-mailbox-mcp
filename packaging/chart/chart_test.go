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
var inputs = map[string]string{
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

// podSpec is the pod spec an object runs, wherever its kind keeps it, and nil for an object that runs
// no pod.
func (o object) podSpec() object {
	var spec any
	switch o.kind() {
	case "Pod":
		spec = o.get("spec")
	case "Deployment", "StatefulSet", "Job":
		spec = o.get("spec", "template", "spec")
	case "CronJob":
		spec = o.get("spec", "jobTemplate", "spec", "template", "spec")
	}
	if m, ok := spec.(map[string]any); ok {
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

// render renders the chart with the inputs, overridden and extended by values, and returns every
// object, the Helm tests included.
func render(t *testing.T, values map[string]string) []object {
	t.Helper()
	set := maps.Clone(inputs)
	maps.Copy(set, values)
	file := filepath.Join(t.TempDir(), "values.yaml")
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

// reach maps each object that runs a pod, as its kind and component, to whether its pod mounts the
// Secret.
func reach(objects []object, secret string) map[string]bool {
	out := map[string]bool{}
	for _, o := range objects {
		spec := o.podSpec()
		if spec == nil {
			continue
		}
		out[o.kind()+" "+o.component()] = slices.Contains(mountedSecrets(spec), secret)
	}
	return out
}

// R1's part of VERIFICATIONS' row for the private key's reach. The key pair reaches the pods of the
// deployables that open a credential, the mediator, backfill however it is started, and delta sync,
// and the UI, whose one client-secret part opens with it. It reaches no other pod the chart runs, the
// migration step and the Helm tests included (ADR-0079, ADR-0081).
func TestThePrivateKeyReachesOnlyTheDeployablesThatOpenCredentials(t *testing.T) {
	want := map[string]bool{
		"Job migrate":        false,
		"Deployment mediate": true,
		"Job backfill":       true,
		"CronJob backfill":   true,
		"StatefulSet sync":   true,
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
	objects := render(t, map[string]string{"database.caSecret": "database-ca"})
	for kind, mounts := range reach(objects, migrateSecret) {
		if mounts != (kind == "Job migrate") {
			t.Errorf("%s mounts the migration role's credential: %t", kind, mounts)
		}
	}
	for _, o := range objects {
		if o.kind() == "Job" && o.component() == "migrate" {
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
}

// VERIFICATIONS' row for the migration step running first. The migration Job is a hook Helm runs to
// completion before an install's and an upgrade's other objects, so no deployable rolls before the
// chain has run, and it is the only object the chart runs that way (ADR-0048, ADR-0115).
func TestTheMigrationStepRunsBeforeTheDeployables(t *testing.T) {
	hooks := map[string]string{}
	for _, o := range render(t, nil) {
		if h := o.str("metadata", "annotations", "helm.sh/hook"); h != "" && h != "test" {
			hooks[o.kind()+" "+o.component()] = h
		}
	}
	want := map[string]string{"Job migrate": "pre-install,pre-upgrade"}
	if diff := cmp.Diff(want, hooks, compare.Options); diff != "" {
		t.Errorf("the objects Helm runs before the others (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for delta sync's one process. The chart runs it only as a StatefulSet of one
// replica whose pods start in order, so a second pod starts only once the first is gone, a rollout
// included (ADR-0103, ADR-0117).
func TestDeltaSyncRunsAsOneProcessAtATime(t *testing.T) {
	var runs []string
	for _, o := range render(t, nil) {
		spec := o.podSpec()
		if spec == nil {
			continue
		}
		for _, c := range spec.list("containers") {
			if strings.Contains(c.str("image"), "/mediated-mailbox-sync:") {
				runs = append(runs, o.kind())
				if o.kind() != "StatefulSet" {
					continue
				}
				if r, ok := o.get("spec", "replicas").(int); !ok || r != 1 {
					t.Errorf("delta sync's StatefulSet has %v replicas, want 1", o.get("spec", "replicas"))
				}
				if p := o.str("spec", "podManagementPolicy"); p != "OrderedReady" {
					t.Errorf("delta sync's pods are managed %q, want OrderedReady", p)
				}
			}
		}
	}
	if diff := cmp.Diff([]string{"StatefulSet"}, runs, compare.Options); diff != "" {
		t.Errorf("the objects that run delta sync (-want +got):\n%s", diff)
	}
}

// backfillJob returns the name of the Job that runs backfill with no manual step.
func backfillJob(t *testing.T, objects []object) string {
	t.Helper()
	var names []string
	for _, o := range objects {
		if o.kind() == "Job" && o.component() == "backfill" {
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
	for change, values := range map[string]map[string]string{
		"a release":             {"image.tag": "9.9.9"},
		"the scanner's section": {"scannerConfig": "scanner:\n  triggers:\n    en: [verify]\n"},
	} {
		if got := backfillJob(t, render(t, values)); got == base {
			t.Errorf("after %s the Job running backfill keeps its name %s, so no upgrade runs it", change, got)
		}
	}
	for change, values := range map[string]map[string]string{
		"the UI's configuration":     {"ui.config": "operator_name: someone\n"},
		"the mediator's replicas":    {"mediate.replicas": "2"},
		"delta sync's configuration": {"sync.config": "sync_interval: 1m\n"},
	} {
		if got := backfillJob(t, render(t, values)); got != base {
			t.Errorf("after a change of %s the Job running backfill is renamed %s from %s, so an upgrade runs it", change, got, base)
		}
	}
	for _, o := range render(t, nil) {
		if o.kind() == "CronJob" && o.component() == "backfill" && o.get("spec", "suspend") != true {
			t.Errorf("backfill's CronJob is scheduled, suspend is %v", o.get("spec", "suspend"))
		}
	}
}

// VERIFICATIONS' row for the pod hardening. Every pod the chart runs, the migration step, the UI and
// the Helm tests included, runs as a non-root user under the runtime's seccomp profile with no
// service account token and no injected service variables, and every container runs with a
// read-only root filesystem, no privilege escalation and every capability dropped (ADR-0028,
// ADR-0052, ADR-0078).
func TestEveryPodRunsHardened(t *testing.T) {
	pods := 0
	for _, o := range render(t, nil) {
		spec := o.podSpec()
		if spec == nil {
			continue
		}
		pods++
		at := o.kind() + " " + o.component()
		for path, want := range map[string]any{
			"automountServiceAccountToken":        false,
			"enableServiceLinks":                  false,
			"securityContext.runAsNonRoot":        true,
			"securityContext.seccompProfile.type": "RuntimeDefault",
		} {
			if got := spec.get(strings.Split(path, ".")...); got != want {
				t.Errorf("%s: %s is %v, want %v", at, path, got, want)
			}
		}
		for _, c := range spec.list("containers") {
			for path, want := range map[string]any{
				"securityContext.allowPrivilegeEscalation": false,
				"securityContext.readOnlyRootFilesystem":   true,
				"securityContext.privileged":               false,
			} {
				if got := c.get(strings.Split(path, ".")...); got != want {
					t.Errorf("%s: container %s: %s is %v, want %v", at, c.str("name"), path, got, want)
				}
			}
			if drop, ok := c.get("securityContext", "capabilities", "drop").([]any); !ok || !slices.Contains(drop, any("ALL")) {
				t.Errorf("%s: container %s drops %v, want ALL", at, c.str("name"), drop)
			}
		}
	}
	if pods != 9 {
		t.Errorf("the chart runs %d pods, want the nine the other tests name", pods)
	}
}

// VERIFICATIONS' row for scratch space. The workloads that hold bodies, the mediator, backfill and
// delta sync, write to memory-backed scratch space at /tmp, their one writable path, and the UI,
// which holds none, has none (ADR-0009).
func TestTheWorkloadsThatHoldBodiesGetMemoryScratch(t *testing.T) {
	got := map[string]bool{}
	for _, o := range render(t, nil) {
		spec := o.podSpec()
		if spec == nil || strings.HasSuffix(o.component(), "-test") || o.component() == "migrate" {
			continue
		}
		memory := map[string]bool{}
		for _, v := range spec.list("volumes") {
			if v.str("emptyDir", "medium") == "Memory" {
				memory[v.str("name")] = true
			}
		}
		mounted := false
		for _, c := range spec.list("containers") {
			for _, m := range c.list("volumeMounts") {
				mounted = mounted || (memory[m.str("name")] && m.str("mountPath") == "/tmp")
			}
		}
		got[o.kind()+" "+o.component()] = mounted
	}
	want := map[string]bool{
		"Deployment mediate": true, "Job backfill": true, "CronJob backfill": true, "StatefulSet sync": true,
		"Deployment ui": false,
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("which pods get memory-backed scratch at /tmp (-want +got):\n%s", diff)
	}
}

// templateMeta returns the metadata of the pod template an object runs, wherever its kind keeps it.
func (o object) templateMeta() object {
	var meta any
	switch o.kind() {
	case "Deployment", "StatefulSet", "Job":
		meta = o.get("spec", "template", "metadata")
	case "CronJob":
		meta = o.get("spec", "jobTemplate", "spec", "template", "metadata")
	}
	if m, ok := meta.(map[string]any); ok {
		return m
	}
	return nil
}

// configs is what one render gives each object that runs a deployable: the text of the deployable's
// ConfigMap, the checksum its pod template carries, and whether its container is given the file.
type configs map[string]struct {
	text, checksum string
	file           bool
}

// configsOf reads configs from a render, keyed on each object's kind and component, the Helm tests left
// out.
func configsOf(objects []object) configs {
	texts := map[string]string{}
	for _, o := range objects {
		if o.kind() == "ConfigMap" {
			texts[o.component()] = o.str("data", "config.yaml")
		}
	}
	out := configs{}
	for _, o := range objects {
		meta := o.templateMeta()
		if meta == nil {
			continue
		}
		c := out[o.kind()+" "+o.component()]
		c.text = texts[o.component()]
		c.checksum = meta.str("annotations", "checksum/config")
		for _, container := range o.podSpec().list("containers") {
			if args, ok := container.get("args").([]any); ok {
				c.file = c.file || slices.Contains(args, any("--config-file=/etc/mediated-mailbox/config.yaml"))
			}
		}
		out[o.kind()+" "+o.component()] = c
	}
	return out
}

// The objects that run a deployable, each with the deployable whose values configure it and whether
// the deployable masks or scans, so its file carries the shared scanner section (ADR-0096, ADR-0116).
var deployables = []struct {
	object, values string
	scans          bool
}{
	{"Deployment mediate", "mediate", true},
	{"Job backfill", "backfill", true},
	{"CronJob backfill", "backfill", true},
	{"StatefulSet sync", "sync", true},
	{"Deployment ui", "ui", false},
}

// The configuration file reaches every deployable as the text the operator wrote, comments and a word
// a re-serialisation would read as a boolean included, followed by the shared scanner section for each
// deployable that masks or scans and for no other. A change to a deployable's own text changes the
// checksum of every object running it, so its pods restart, and a change to the scanner section changes
// the checksum of the three that carry it and not the UI's. A deployable with no text gets no file,
// since a file holding no document refuses the start (ADR-0052, ADR-0078, ADR-0096).
func TestTheConfigurationFileIsPassedThroughAsWritten(t *testing.T) {
	own := "# the operator's own comment\nno_such_key_is_read: no\n"
	scanner := "scanner:\n  triggers:\n    no: [verify]\n"
	values := map[string]string{"scannerConfig": scanner}
	for _, d := range deployables {
		values[d.values+".config"] = own
	}
	before := configsOf(render(t, values))
	for _, d := range deployables {
		got := before[d.object]
		want := own
		if d.scans {
			want = own + "\n" + scanner
		}
		if got.text != want || !got.file || got.checksum == "" {
			t.Errorf("%s: the file is %q, given as an argument %t, with checksum %q, want %q given with a checksum", d.object, got.text, got.file, got.checksum, want)
		}
	}

	for _, changed := range deployables {
		edited := maps.Clone(values)
		edited[changed.values+".config"] = own + "another_key: 1\n"
		after := configsOf(render(t, edited))
		for _, d := range deployables {
			restarted := after[d.object].checksum != before[d.object].checksum
			if want := d.values == changed.values; restarted != want {
				t.Errorf("a change of %s's file: %s's checksum changed %t, want %t", changed.values, d.object, restarted, want)
			}
		}
	}

	edited := maps.Clone(values)
	edited["scannerConfig"] = scanner + "    en: [log in]\n"
	after := configsOf(render(t, edited))
	for _, d := range deployables {
		if restarted := after[d.object].checksum != before[d.object].checksum; restarted != d.scans {
			t.Errorf("a change of the scanner section: %s's checksum changed %t, want %t", d.object, restarted, d.scans)
		}
	}

	empty := configsOf(render(t, nil))
	for _, d := range deployables {
		if got := empty[d.object]; got.text != "" || got.file {
			t.Errorf("%s, given no text and no scanner section, has the file %q, given as an argument %t", d.object, got.text, got.file)
		}
	}
}
