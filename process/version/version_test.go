package version_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

// The release cut writes one version into the manifest release-please reads, this package and the
// chart's version and appVersion, so the three agree in every commit. A misconfigured updater, or a
// hand edit of one of them, leaves them apart and fails here on the release pull request itself.
func TestTheVersionIsTheManifestsAndTheCharts(t *testing.T) {
	raw, err := os.ReadFile("../../.release-please-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]string
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	want := manifest["."]
	if want == "" {
		t.Fatalf("the manifest names no version for the root package: %s", raw)
	}
	if version.Number != want {
		t.Errorf("version.Number is %q, and the manifest's version is %q", version.Number, want)
	}

	chart, err := os.ReadFile("../../packaging/chart/Chart.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{}
	for _, line := range strings.Split(string(chart), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(key, "#") || strings.HasPrefix(key, " ") {
			continue
		}
		value, _, _ = strings.Cut(value, "#")
		fields[key] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	for _, key := range []string{"version", "appVersion"} {
		if fields[key] != want {
			t.Errorf("the chart's %s is %q, and the manifest's version is %q", key, fields[key], want)
		}
	}
}
