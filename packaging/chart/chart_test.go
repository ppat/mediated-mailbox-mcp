package chart_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/helmchart"
)

// chartDir is the chart, which is this package's own directory.
const chartDir = "."

// everyOption sets every optional input that adds a mounted file: the CA certificate, two private keys during a key
// replacement, and the API server's and the UI's TLS pairs.
var everyOption = []string{
	"--set", "secrets.databaseCaCertificate.existingSecret=database-ca",
	"--set", "secrets.credentialSealingKeyPair.secretKeys.privateKeys={old.key,new.key}",
	"--set", "secrets.apiTls.existingSecret=api-tls",
	"--set", "secrets.uiTls.existingSecret=ui-tls",
}

// workload is a rendered Deployment, Job or Pod and its pod spec.
type workload struct {
	kind, name string
	spec       map[string]any
}

func workloads(t *testing.T, objects []helmchart.Object) []workload {
	t.Helper()
	var out []workload
	for _, o := range objects {
		name, kind := dig[string](o, "metadata", "name"), dig[string](o, "kind")
		switch kind {
		case "Deployment", "Job":
			out = append(out, workload{kind, name, dig[map[string]any](o, "spec", "template", "spec")})
		case "Pod":
			out = append(out, workload{"Pod", name, dig[map[string]any](o, "spec")})
		}
	}
	return out
}

// dig walks nested maps and returns the value at the path, or the zero value.
func dig[T any](v any, path ...string) T {
	var zero T
	for _, p := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return zero
		}
		v = m[p]
	}
	if got, ok := v.(T); ok {
		return got
	}
	return zero
}

// list returns v as a list, or nil when it is not one.
func list(v any) []any {
	return dig[[]any](v)
}

// secretFiles returns every secret key a pod reads, as "secret/key -> path", from every volume and every
// environment variable, so a key reaching a pod any way at all is listed.
func secretFiles(spec map[string]any) []string {
	var files []string
	for _, v := range list(spec["volumes"]) {
		if s := dig[map[string]any](v, "secret"); s != nil {
			files = append(files, fmt.Sprintf("%s/* (whole secret volume)", s["secretName"]))
		}
		for _, src := range list(dig[any](v, "projected", "sources")) {
			s := dig[map[string]any](src, "secret")
			if s == nil {
				continue
			}
			items := list(s["items"])
			if len(items) == 0 {
				files = append(files, fmt.Sprintf("%s/* (whole secret)", s["name"]))
			}
			for _, it := range items {
				files = append(files, fmt.Sprintf("%s/%s -> %s", s["name"], dig[string](it, "key"), dig[string](it, "path")))
			}
		}
	}
	for _, c := range append(list(spec["containers"]), list(spec["initContainers"])...) {
		for _, e := range list(dig[any](c, "env")) {
			if ref := dig[map[string]any](e, "valueFrom", "secretKeyRef"); ref != nil {
				files = append(files, fmt.Sprintf("%s/%s -> env %s", ref["name"], ref["key"], dig[string](e, "name")))
			}
		}
		for _, e := range list(dig[any](c, "envFrom")) {
			if ref := dig[map[string]any](e, "secretRef"); ref != nil {
				files = append(files, fmt.Sprintf("%s/* -> env", ref["name"]))
			}
		}
	}
	slices.Sort(files)
	return files
}

// TestEveryPodReadsOnlyTheSecretsItsProcessUses renders the chart with every optional secret input set and lists,
// for each pod, every secret key it can read by any means. Each process gets exactly the keys it reads (ADR-0079):
// the key pair reaches the API server, the worker and the UI and nothing else (ADR-0081), the migration role's
// password reaches only the migration step (ADR-0129), each runtime role's password only its own process, and
// the Helm test reads no secret at all. The expected lists are written out here, not derived from the chart.
func TestEveryPodReadsOnlyTheSecretsItsProcessUses(t *testing.T) {
	keyPair := []string{
		"sealing-keys/credential.pub -> credential-sealing-key-pair/credential.pub",
		"sealing-keys/new.key -> credential-sealing-key-pair/new.key",
		"sealing-keys/old.key -> credential-sealing-key-pair/old.key",
	}
	ca := "database-ca/ca.crt -> ca/ca.crt"
	want := map[string][]string{
		"Deployment/release-mediated-mailbox-api": append([]string{
			ca, "api-db/password -> database/password",
			"api-tls/tls.crt -> tls/tls.crt", "api-tls/tls.key -> tls/tls.key",
			"release-mediated-mailbox-access-credentials/api-bearer-token -> bearer-token",
		}, keyPair...),
		"Deployment/release-mediated-mailbox-ui": append([]string{
			ca, "release-mediated-mailbox-access-credentials/ui-request-token-key -> request-token-key",
			"ui-db/password -> database/password", "ui-tls/tls.crt -> tls/tls.crt", "ui-tls/tls.key -> tls/tls.key",
		}, keyPair...),
		"Deployment/release-mediated-mailbox-worker": append([]string{
			"backfill-db/password -> database/backfill/password", ca, "sync-db/password -> database/sync/password",
		}, keyPair...),
		"Job/release-mediated-mailbox-migration": {ca, "migrator-db/password -> database/password"},
		"Pod/release-mediated-mailbox-test":      nil,
	}
	got := map[string][]string{}
	for _, w := range workloads(t, helmchart.Render(t, chartDir, everyOption...)) {
		got[w.kind+"/"+w.name] = secretFiles(w.spec)
	}
	for k := range want {
		slices.Sort(want[k])
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("the secret keys each pod reads (-want +got):\n%s", diff)
	}
}

// TestTheWorkerRunsAsOnePodNeverTwo renders the worker's Deployment, which must hold exactly one replica and a
// Recreate strategy, so a rollout stops the old pod before it starts the new one (ADR-0117), and checks that no value
// can raise the count, because the schema refuses a replica count for the worker.
func TestTheWorkerRunsAsOnePodNeverTwo(t *testing.T) {
	var found bool
	for _, o := range helmchart.Render(t, chartDir) {
		if o["kind"] != "Deployment" || dig[string](o, "metadata", "name") != "release-mediated-mailbox-worker" {
			continue
		}
		found = true
		if got := dig[int](o, "spec", "replicas"); got != 1 {
			t.Errorf("the worker's replicas are %d, want 1", got)
		}
		if got := dig[string](o, "spec", "strategy", "type"); got != "Recreate" {
			t.Errorf("the worker's strategy is %q, want Recreate", got)
		}
	}
	if !found {
		t.Fatal("the chart renders no Deployment for the worker")
	}
	for _, key := range []string{"worker.replicaCount=2", "worker.replicas=2", "worker.strategy.type=RollingUpdate"} {
		if out, err := helmchart.Template(t, chartDir, "--set", key); err == nil || !strings.Contains(out, "additional properties") {
			t.Errorf("the chart accepts --set %s: %v\n%s", key, err, out)
		}
	}
}

// TestEveryProcessIsProbedForHealthAndReadiness checks that each process's Deployment probes its health at /healthz
// and its readiness at /readyz on its metrics port, the operations endpoints every process that runs until it is told
// to stop serves (ADR-0138), so a rollout completes only once the process reports ready, the worker once its job kinds
// can run. It also checks that the Helm test asks each process's Service for /readyz. The paths and the port are written out here, not derived
// from the chart.
func TestEveryProcessIsProbedForHealthAndReadiness(t *testing.T) {
	probe := func(path string) map[string]any {
		return map[string]any{"httpGet": map[string]any{"path": path, "port": "metrics"}}
	}
	var script string
	probed := map[string]bool{}
	for _, o := range helmchart.Render(t, chartDir) {
		name := dig[string](o, "metadata", "name")
		switch o["kind"] {
		case "Deployment":
			for _, c := range list(dig[any](o, "spec", "template", "spec", "containers")) {
				probed[name] = true
				if diff := cmp.Diff(probe("/healthz"), dig[map[string]any](c, "livenessProbe")); diff != "" {
					t.Errorf("%s: livenessProbe (-want +got):\n%s", name, diff)
				}
				if diff := cmp.Diff(probe("/readyz"), dig[map[string]any](c, "readinessProbe")); diff != "" {
					t.Errorf("%s: readinessProbe (-want +got):\n%s", name, diff)
				}
				var port int
				for _, p := range list(dig[any](c, "ports")) {
					if dig[string](p, "name") == "metrics" {
						port = dig[int](p, "containerPort")
					}
				}
				if port != 8080 {
					t.Errorf("%s: the metrics port is %d, want 8080", name, port)
				}
			}
		case "Pod":
			if cmd := list(dig[any](o, "spec", "containers")); len(cmd) == 1 {
				if args := list(dig[any](cmd[0], "command")); len(args) > 0 {
					script = dig[string](args[len(args)-1])
				}
			}
		}
	}
	want := map[string]bool{
		"release-mediated-mailbox-api":    true,
		"release-mediated-mailbox-ui":     true,
		"release-mediated-mailbox-worker": true,
	}
	if diff := cmp.Diff(want, probed); diff != "" {
		t.Errorf("the Deployments checked (-want +got):\n%s", diff)
	}
	for name := range want {
		if line := "check 200 http://" + name + ":8080/readyz\n"; !strings.Contains(script, line) {
			t.Errorf("the Helm test does not run %q:\n%s", strings.TrimSpace(line), script)
		}
	}
}

// checksums renders the chart from dir and returns each Deployment's configuration checksum by name.
func checksums(t *testing.T, dir string, args ...string) map[string]string {
	t.Helper()
	sums := map[string]string{}
	for _, o := range helmchart.Render(t, dir, args...) {
		if o["kind"] == "Deployment" {
			sums[dig[string](o, "metadata", "name")] = dig[string](o, "spec", "template", "metadata", "annotations", "checksum/config")
		}
	}
	return sums
}

// TestAConfigurationChangeRestartsOnlyTheProcessesThatReadIt changes one value at a time and checks which
// Deployments' configuration checksums change, since a changed checksum changes the pod template and restarts the
// pods (ADR-0052). Each checksum hashes only its own process's rendered file, so a change to one process's own value
// restarts that process alone and a shared value restarts exactly the processes it is rendered into.
func TestAConfigurationChangeRestartsOnlyTheProcessesThatReadIt(t *testing.T) {
	const api, ui, worker = "release-mediated-mailbox-api", "release-mediated-mailbox-ui", "release-mediated-mailbox-worker"
	base := checksums(t, chartDir)
	for _, sum := range base {
		if sum == "" {
			t.Fatalf("a Deployment carries no configuration checksum: %v", base)
		}
	}
	for _, c := range []struct {
		set     string
		changed []string
	}{
		{"api.config.provider_timeout=1m", []string{api}},
		{"ui.config.operator_name=owner", []string{ui}},
		{"worker.jobKinds.backfill.config.concurrency=3", []string{worker}},
		{"sharedConfig.sync_interval=2m", []string{ui, worker}},
		{"sharedConfig.scanner.window=5", []string{api, worker}},
		{"sharedConfig.log_level=debug", []string{api, ui, worker}},
	} {
		var changed []string
		for name, sum := range checksums(t, chartDir, "--set", c.set) {
			if sum != base[name] {
				changed = append(changed, name)
			}
		}
		slices.Sort(changed)
		if diff := cmp.Diff(c.changed, changed); diff != "" {
			t.Errorf("--set %s restarts (-want +got):\n%s", c.set, diff)
		}
	}
}

// TestEveryPodRunsHardened checks every pod the chart renders, the migration step and the Helm test included,
// against the restricted Pod Security Standard and the hardening of ADR-0028: a numeric non-root user, the
// runtime's default seccomp profile, no privilege escalation, every capability dropped, a read-only root
// filesystem, no ServiceAccount token, no Service environment variables, and no volume a process could write.
func TestEveryPodRunsHardened(t *testing.T) {
	ws := workloads(t, helmchart.Render(t, chartDir, everyOption...))
	if len(ws) != 5 {
		t.Fatalf("the chart renders %d pods, want the API server, the UI, the worker, the migration step and the test", len(ws))
	}
	for _, w := range ws {
		pod := dig[map[string]any](w.spec, "securityContext")
		problems := []string{}
		check := func(ok bool, what string) {
			if !ok {
				problems = append(problems, what)
			}
		}
		check(pod["runAsNonRoot"] == true, "runAsNonRoot is not true")
		check(dig[int](pod, "runAsUser") > 0, "runAsUser is not a non-zero number")
		check(dig[int](pod, "fsGroup") > 0, "fsGroup is not set")
		check(dig[string](pod, "seccompProfile", "type") == "RuntimeDefault", "seccompProfile is not RuntimeDefault")
		check(w.spec["automountServiceAccountToken"] == false, "the ServiceAccount token is mounted")
		check(w.spec["enableServiceLinks"] == false, "Service environment variables are injected")
		check(w.spec["hostNetwork"] == nil && w.spec["hostPID"] == nil && w.spec["hostIPC"] == nil, "a host namespace is shared")
		for _, c := range append(list(w.spec["containers"]), list(w.spec["initContainers"])...) {
			sc := dig[map[string]any](c, "securityContext")
			name := dig[string](c, "name")
			check(sc["allowPrivilegeEscalation"] == false, name+": privilege escalation is allowed")
			check(sc["readOnlyRootFilesystem"] == true, name+": the root filesystem is writable")
			check(sc["privileged"] == nil || sc["privileged"] == false, name+": it is privileged")
			check(slices.Equal(list(dig[any](sc, "capabilities", "drop")), []any{"ALL"}), name+": not every capability is dropped")
			check(dig[any](sc, "capabilities", "add") == nil, name+": a capability is added")
			for _, m := range list(dig[any](c, "volumeMounts")) {
				check(dig[bool](m, "readOnly"), name+": the mount "+dig[string](m, "mountPath")+" is writable")
			}
		}
		for _, v := range list(w.spec["volumes"]) {
			m := dig[map[string]any](v)
			check(m["configMap"] != nil || m["projected"] != nil, "the volume "+dig[string](m, "name")+" is neither a ConfigMap nor a projected Secret")
			if p := dig[map[string]any](m, "projected"); p != nil {
				check(dig[int](p, "defaultMode") == 0o440, "the projected volume's mode is not 0440")
			}
		}
		if len(problems) > 0 {
			t.Errorf("%s %s: %s", w.kind, w.name, strings.Join(problems, "; "))
		}
	}
}

// TestEveryPodPullsWithTheGlobalImagePullSecrets checks that global.imagePullSecrets reaches every pod the chart
// renders, the Helm test's included, so every image pulls through an authenticated registry.
func TestEveryPodPullsWithTheGlobalImagePullSecrets(t *testing.T) {
	ws := workloads(t, helmchart.Render(t, chartDir, "--set", "global.imagePullSecrets[0].name=registry-login"))
	if len(ws) != 5 {
		t.Fatalf("the chart renders %d pods, want the API server, the UI, the worker, the migration step and the test", len(ws))
	}
	want := []any{map[string]any{"name": "registry-login"}}
	for _, w := range ws {
		if diff := cmp.Diff(want, list(w.spec["imagePullSecrets"])); diff != "" {
			t.Errorf("%s %s: imagePullSecrets (-want +got):\n%s", w.kind, w.name, diff)
		}
	}
}

// TestTheMigrationStepIsAHookThatReadsNothingTheChartRenders checks the migration Job of ADR-0129: a pre-install and
// pre-upgrade hook that runs one pod at a time, replaces a pod only once it has failed, mounts nothing from a
// ConfigMap, runs under no ServiceAccount the chart creates, and sets its configuration in its own spec with no
// password, the password coming from the mounted password file. That the migrator reads that configuration is
// migrator/app's test.
func TestTheMigrationStepIsAHookThatReadsNothingTheChartRenders(t *testing.T) {
	var job helmchart.Object
	for _, o := range helmchart.Render(t, chartDir, "--set", "secrets.databaseCaCertificate.existingSecret=database-ca") {
		if o["kind"] == "Job" {
			job = o
		}
	}
	if job == nil {
		t.Fatal("the chart renders no Job")
	}
	annotations := dig[map[string]any](job, "metadata", "annotations")
	if diff := cmp.Diff(map[string]any{
		"helm.sh/hook":               "pre-install,pre-upgrade",
		"helm.sh/hook-weight":        "0",
		"helm.sh/hook-delete-policy": "before-hook-creation,hook-succeeded",
	}, annotations); diff != "" {
		t.Errorf("the Job's annotations (-want +got):\n%s", diff)
	}
	for _, c := range []struct {
		path []string
		want any
	}{
		{[]string{"spec", "parallelism"}, 1},
		{[]string{"spec", "completions"}, 1},
		{[]string{"spec", "podReplacementPolicy"}, "Failed"},
		{[]string{"spec", "template", "spec", "serviceAccountName"}, nil},
	} {
		if got := dig[any](job, c.path...); got != c.want {
			t.Errorf("%s is %v, want %v", strings.Join(c.path, "."), got, c.want)
		}
	}
	spec := dig[map[string]any](job, "spec", "template", "spec")
	for _, v := range list(spec["volumes"]) {
		if dig[any](v, "configMap") != nil {
			t.Errorf("the Job mounts a ConfigMap, which does not exist yet at pre-install")
		}
	}
	env := map[string]any{}
	for _, e := range list(dig[any](list(spec["containers"])[0], "env")) {
		if dig[any](e, "valueFrom") != nil {
			t.Errorf("the Job takes %s from outside its spec", dig[string](e, "name"))
		}
		env[dig[string](e, "name")] = dig[any](e, "value")
	}
	//nolint:gosec // A mounted file's path, not a credential.
	want := map[string]any{
		"MEDIATED_MAILBOX_LOG_LEVEL":               "info",
		"MEDIATED_MAILBOX_DATABASE__HOST":          "db.example",
		"MEDIATED_MAILBOX_DATABASE__PORT":          "5432",
		"MEDIATED_MAILBOX_DATABASE__NAME":          "mediated_mailbox",
		"MEDIATED_MAILBOX_DATABASE__USER":          "mediated_mailbox_migrator",
		"MEDIATED_MAILBOX_DATABASE__SSLMODE":       "verify-full",
		"MEDIATED_MAILBOX_DATABASE__SSLROOTCERT":   "/etc/mediated-mailbox/secrets/ca/ca.crt",
		"MEDIATED_MAILBOX_DATABASE__PASSWORD_FILE": "/etc/mediated-mailbox/secrets/database/password",
	}
	if diff := cmp.Diff(want, env); diff != "" {
		t.Errorf("the Job's environment (-want +got):\n%s", diff)
	}
}

// kinds renders the chart and counts each kind it renders.
func kinds(t *testing.T, args ...string) map[string]int {
	t.Helper()
	count := map[string]int{}
	for _, o := range helmchart.Render(t, chartDir, args...) {
		count[dig[string](o, "kind")]++
	}
	return count
}

// TestThePrometheusOperatorsResourcesFollowItsAPIAndTheSwitch renders the chart with and without the Prometheus
// Operator's kinds among the API versions, under each setting of the switches. auto renders a resource exactly where
// its kind is served, true renders it anyway and false never does (ADR-0052, ADR-0077). helm template reads the API
// versions only from its flags, so both the group-version form and the kind form are passed, and the chart must
// render on the kind form alone.
func TestThePrometheusOperatorsResourcesFollowItsAPIAndTheSwitch(t *testing.T) {
	api := []string{"--api-versions", "monitoring.coreos.com/v1", "--api-versions", "monitoring.coreos.com/v1/ServiceMonitor", "--api-versions", "monitoring.coreos.com/v1/PrometheusRule"}
	for _, c := range []struct {
		name            string
		args            []string
		monitors, rules int
	}{
		{"auto without the API", nil, 0, 0},
		{"auto with the API", api, 3, 1},
		{"auto with only the group version", []string{"--api-versions", "monitoring.coreos.com/v1"}, 0, 0},
		{"false with the API", append(slices.Clone(api), "--set", "monitoring.serviceMonitor.enabled=false", "--set", "monitoring.prometheusRule.enabled=false"), 0, 0},
		{"true without the API", []string{"--set", "monitoring.serviceMonitor.enabled=true", "--set", "monitoring.prometheusRule.enabled=true"}, 3, 1},
	} {
		got := kinds(t, c.args...)
		if got["ServiceMonitor"] != c.monitors || got["PrometheusRule"] != c.rules {
			t.Errorf("%s: %d ServiceMonitors and %d PrometheusRules, want %d and %d", c.name, got["ServiceMonitor"], got["PrometheusRule"], c.monitors, c.rules)
		}
	}
}

// TestNothingOptionalRendersByDefault renders the chart with only its required values, which must expose nothing
// outside the cluster and render no dashboard, and then switches each on.
func TestNothingOptionalRendersByDefault(t *testing.T) {
	want := map[string]int{"ConfigMap": 1, "Deployment": 3, "Job": 1, "Pod": 1, "Secret": 1, "Service": 3, "ServiceAccount": 3}
	if diff := cmp.Diff(want, kinds(t)); diff != "" {
		t.Errorf("the kinds a default render holds (-want +got):\n%s", diff)
	}
	on := kinds(t,
		"--set", "api.ingress.enabled=true", "--set", "ui.ingress.enabled=true",
		"--set", "api.httpRoute.enabled=true", "--set", "ui.httpRoute.enabled=true",
		"--set", "monitoring.dashboard.enabled=true")
	if on["Ingress"] != 2 || on["HTTPRoute"] != 2 || on["ConfigMap"] != 2 {
		t.Errorf("switched on, the chart renders %d Ingresses, %d HTTPRoutes and %d ConfigMaps, want 2, 2 and 2", on["Ingress"], on["HTTPRoute"], on["ConfigMap"])
	}
}

// TestTheRoutesShareAHostNameByPath renders both HTTPRoutes with their default rules. The UI's matches /ui and / alone,
// everything the UI serves (ADR-0137), and the API server's matches /, so on one host name a gateway sends /ui and / to
// the UI, the longer and the exact match winning, and every other path to the API server.
func TestTheRoutesShareAHostNameByPath(t *testing.T) {
	got := map[string][]string{}
	for _, o := range helmchart.Render(t, chartDir, "--set", "api.httpRoute.enabled=true", "--set", "ui.httpRoute.enabled=true") {
		if dig[string](o, "kind") != "HTTPRoute" {
			continue
		}
		name := dig[string](o, "metadata", "name")
		for _, r := range list(dig[any](o, "spec", "rules")) {
			for _, m := range list(dig[any](r, "matches")) {
				got[name] = append(got[name], dig[string](m, "path", "type")+" "+dig[string](m, "path", "value"))
			}
		}
	}
	want := map[string][]string{
		"release-mediated-mailbox-api": {"PathPrefix /"},
		"release-mediated-mailbox-ui":  {"PathPrefix /ui", "Exact /"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("the HTTPRoutes' default matches (-want +got):\n%s", diff)
	}
}

// goSoftMemoryLimits renders the chart with args and returns, for each container by name, the value of its GOMEMLIMIT
// variable, empty when it has none.
func goSoftMemoryLimits(t *testing.T, args ...string) map[string]string {
	t.Helper()
	limits := map[string]string{}
	for _, w := range workloads(t, helmchart.Render(t, chartDir, args...)) {
		for _, c := range list(w.spec["containers"]) {
			name := dig[string](c, "name")
			limits[name] = ""
			for _, e := range list(dig[any](c, "env")) {
				if dig[string](e, "name") == "GOMEMLIMIT" {
					limits[name] = dig[string](e, "value")
				}
			}
		}
	}
	return limits
}

// TestEachGoContainerGetsOnlyItsOwnSoftMemoryLimit checks ADR-0052's soft memory limit. No container has a GOMEMLIMIT
// by default, and each Go container given a soft memory limit gets exactly its own, so one component's value never
// reaches another and the Helm test never gets one.
func TestEachGoContainerGetsOnlyItsOwnSoftMemoryLimit(t *testing.T) {
	if diff := cmp.Diff(map[string]string{"api": "", "ui": "", "worker": "", "migration": "", "smoke": ""}, goSoftMemoryLimits(t)); diff != "" {
		t.Errorf("GOMEMLIMIT by default (-want +got):\n%s", diff)
	}
	limits := goSoftMemoryLimits(t,
		"--set", "api.goSoftMemoryLimit=101MiB", "--set", "ui.goSoftMemoryLimit=102MiB",
		"--set", "worker.goSoftMemoryLimit=103MiB", "--set", "migration.goSoftMemoryLimit=104MiB")
	want := map[string]string{"api": "101MiB", "ui": "102MiB", "worker": "103MiB", "migration": "104MiB", "smoke": ""}
	if diff := cmp.Diff(want, limits); diff != "" {
		t.Errorf("GOMEMLIMIT with each component's own value (-want +got):\n%s", diff)
	}
}

// accessCredentials is the Secret the chart creates for whichever of the API server's bearer token and the UI's request
// token key no Secret is named for.
const accessCredentials = "release-mediated-mailbox-access-credentials"

// bearerTokenAndRequestTokenKeyRead renders the chart with args and returns the keys of the chart's access credentials
// Secret, nil when it renders none, and what the API server and the UI read for the bearer token and the request token
// key.
func bearerTokenAndRequestTokenKeyRead(t *testing.T, args ...string) (accessCredentialsKeys []string, read map[string][]string) {
	t.Helper()
	objects := helmchart.Render(t, chartDir, args...)
	for _, o := range objects {
		if o["kind"] != "Secret" {
			continue
		}
		if name := dig[string](o, "metadata", "name"); name != accessCredentials {
			t.Errorf("the chart renders a Secret named %q, want %s", name, accessCredentials)
		}
		if got := dig[string](o, "metadata", "annotations", "helm.sh/resource-policy"); got != "keep" {
			t.Errorf("the chart's Secret has resource policy %q, want keep", got)
		}
		for k, v := range dig[map[string]any](o, "data") {
			if v == "" {
				t.Errorf("the chart's Secret holds an empty %s", k)
			}
			accessCredentialsKeys = append(accessCredentialsKeys, k)
		}
		slices.Sort(accessCredentialsKeys)
	}
	read = map[string][]string{}
	for _, w := range workloads(t, objects) {
		for _, f := range secretFiles(w.spec) {
			if strings.HasSuffix(f, "-> bearer-token") || strings.HasSuffix(f, "-> request-token-key") {
				read[w.name] = append(read[w.name], f)
			}
		}
	}
	return accessCredentialsKeys, read
}

// TestTheBearerTokenAndRequestTokenKeyComeFromTheUsersSecretsOrTheChartsOwn checks where the API server's bearer token
// and the UI's request token key come from (ADR-0052). With no Secret named for either, the chart creates one Secret,
// its access credentials Secret, kept on uninstall, holding both, and the pods read it. With a Secret named for each,
// under key names of the user's choosing, the chart creates none and the pods read exactly those keys. With a Secret
// named for only the bearer token, or only the request token key, the chart's Secret holds only the other, and each
// pod reads its value where it is.
func TestTheBearerTokenAndRequestTokenKeyComeFromTheUsersSecretsOrTheChartsOwn(t *testing.T) {
	const api, ui = "release-mediated-mailbox-api", "release-mediated-mailbox-ui"
	for _, c := range []struct {
		name              string
		args              []string
		accessCredentials []string
		read              map[string][]string
	}{
		{"neither named", nil, []string{"api-bearer-token", "ui-request-token-key"}, map[string][]string{
			api: {accessCredentials + "/api-bearer-token -> bearer-token"},
			ui:  {accessCredentials + "/ui-request-token-key -> request-token-key"},
		}},
		{"both named", []string{
			"--set", "secrets.apiBearerToken.existingSecret=api-token", "--set", "secrets.apiBearerToken.secretKeys.bearerToken=token",
			"--set", "secrets.uiRequestTokenKey.existingSecret=ui-signing", "--set", "secrets.uiRequestTokenKey.secretKeys.requestTokenKey=key",
		}, nil, map[string][]string{
			api: {"api-token/token -> bearer-token"},
			ui:  {"ui-signing/key -> request-token-key"},
		}},
		{"only the bearer token named", []string{"--set", "secrets.apiBearerToken.existingSecret=api-token"}, []string{"ui-request-token-key"}, map[string][]string{
			api: {"api-token/api-bearer-token -> bearer-token"},
			ui:  {accessCredentials + "/ui-request-token-key -> request-token-key"},
		}},
		{"only the request token key named", []string{"--set", "secrets.uiRequestTokenKey.existingSecret=ui-signing"}, []string{"api-bearer-token"}, map[string][]string{
			api: {accessCredentials + "/api-bearer-token -> bearer-token"},
			ui:  {"ui-signing/ui-request-token-key -> request-token-key"},
		}},
	} {
		gotAccessCredentials, gotRead := bearerTokenAndRequestTokenKeyRead(t, c.args...)
		if diff := cmp.Diff(c.accessCredentials, gotAccessCredentials); diff != "" {
			t.Errorf("%s: the access credentials Secret's keys (-want +got):\n%s", c.name, diff)
		}
		if diff := cmp.Diff(c.read, gotRead); diff != "" {
			t.Errorf("%s: what the pods read (-want +got):\n%s", c.name, diff)
		}
	}
}

// TestTheSchemaRefusesWhatHelmOrTheProcessesWouldChange renders values Helm's YAML 1.1 parser changes before any
// template runs, and values a process reads that the chart writes itself, and requires the schema to refuse each
// (ADR-0052, ADR-0078). An unquoted no as a language becomes the key false, and an unquoted yes as a word becomes
// true. A chart-owned key given in a process's own block, or a job kind the worker does not run, would make a
// process refuse its start or two sources write one key. Each case names the refusal it must cause, so a case refused
// for another reason, such as a misnamed block around it, fails.
func TestTheSchemaRefusesWhatHelmOrTheProcessesWouldChange(t *testing.T) {
	for _, c := range []struct {
		name, values, refusal string
	}{
		{"an unquoted no as a language", "sharedConfig:\n  scanner:\n    triggers:\n      no: [kode]\n", "invalid propertyName 'false'"},
		{"an unquoted yes as a trigger word", "sharedConfig:\n  scanner:\n    triggers:\n      en: [code, yes]\n", "got boolean, want string"},
		{"an unquoted off as a link word", "sharedConfig:\n  scanner:\n    link_words: [off]\n", "got boolean, want string"},
		{"the scanner in the API server's own block", "api:\n  config:\n    scanner: {window: 3}\n", "additional properties 'scanner' not allowed"},
		{"the listen address in the UI's own block", "ui:\n  config:\n    listen: \":1\"\n", "additional properties 'listen' not allowed"},
		{"the database in the worker's own block", "worker:\n  config:\n    database: {host: other}\n", "additional properties 'database' not allowed"},
		{"the database in the migrator's own block", "migration:\n  config:\n    database: {host: other}\n", "additional properties 'database' not allowed"},
		{"the sync interval in delta sync's own section", "worker:\n  jobKinds:\n    sync:\n      config:\n        sync_interval: 1m\n", "additional properties 'sync_interval' not allowed"},
		{"a password file in a role's block", "api:\n  database:\n    password_file: /x\n", "additional properties 'password_file' not allowed"},
		{"a job kind the worker does not run", "worker:\n  jobKinds:\n    organize:\n      database: {user: x}\n", "additional properties 'organize' not allowed"},
		{"a password for a job kind the worker does not run", "secrets:\n  databasePasswords:\n    worker:\n      organize: {existingSecret: x}\n", "additional properties 'organize' not allowed"},
		{"a misspelled key", "api:\n  config:\n    provider_timout: 1m\n", "additional properties 'provider_timout' not allowed"},
		{"a Kubernetes quantity as a Go soft memory limit", "api:\n  goSoftMemoryLimit: 460Mi\n", "does not match pattern"},
		{"a number as a Go soft memory limit", "worker:\n  goSoftMemoryLimit: 1073741824\n", "got number, want string"},
		{"a pool size of zero", "ui:\n  database:\n    poolSize: 0\n", "minimum: got 0, want 1"},
	} {
		path := filepath.Join(t.TempDir(), "values.yaml")
		if err := os.WriteFile(path, []byte(c.values), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := helmchart.Template(t, chartDir, "-f", path)
		if err == nil || !strings.Contains(out, "values don't meet the specifications of the schema") || !strings.Contains(out, c.refusal) {
			t.Errorf("%s: the schema does not refuse it with %q: %v\n%s", c.name, c.refusal, err, out)
		}
	}
	quoted := "sharedConfig:\n  scanner:\n    triggers:\n      \"no\": [kode, \"yes\"]\n"
	path := filepath.Join(t.TempDir(), "values.yaml")
	if err := os.WriteFile(path, []byte(quoted), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := helmchart.Template(t, chartDir, "-f", path); err != nil {
		t.Errorf("the quoted no and yes are refused: %v\n%s", err, out)
	}
}

// TestTheDashboardReadsOnlySeriesTheProcessesEmit checks that every series the Grafana dashboard queries is named in
// the non-test Go code of some deployable or shared library, so a renamed or removed series does not leave a panel
// silently empty.
func TestTheDashboardReadsOnlySeriesTheProcessesEmit(t *testing.T) {
	dashboard, err := os.ReadFile("dashboards/mediated-mailbox.json")
	if err != nil {
		t.Fatal(err)
	}
	var code strings.Builder
	repository := os.DirFS(filepath.Join("..", ".."))
	err = fs.WalkDir(repository, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == "testdata") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			src, err := fs.ReadFile(repository, path)
			if err != nil {
				return err
			}
			code.Write(src)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	series := regexp.MustCompile(`mediated_mailbox_[a-z_]+`).FindAllString(string(dashboard), -1)
	if len(series) == 0 {
		t.Fatal("the dashboard queries no series")
	}
	for _, s := range series {
		name := strings.TrimSuffix(s, "_bucket")
		if !strings.Contains(code.String(), `"`+name+`"`) {
			t.Errorf("the dashboard queries %s, which no process emits", s)
		}
	}
}
