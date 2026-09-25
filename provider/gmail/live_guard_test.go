package gmail_test

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/livecontract"
)

// proxyVariables are the proxy settings, in lower case, that the child's environment drops before it
// sets the one proxy it means, so no inherited setting, in either letter case, bypasses it.
var proxyVariables = []string{"http_proxy", "https_proxy", "no_proxy", "all_proxy"}

// The contract run against real Gmail, compiled under its build tag with credentials present,
// skips before any call to Gmail unless go tool livecontract gmail started it, and names the
// command. The credentials here are not real, so a run past the guard would fail rather than skip.
func TestTheLiveContractSkipsUnlessItsCommandStartedIt(t *testing.T) {
	for name, marker := range map[string]string{"no marker": "", "another provider's marker": "gcal"} {
		t.Run(name, func(t *testing.T) {
			env := slices.DeleteFunc(os.Environ(), func(v string) bool {
				name, _, _ := strings.Cut(v, "=")
				return name == livecontract.Marker || slices.Contains(proxyVariables, strings.ToLower(name))
			})
			for _, v := range []string{"ACCOUNT=test@example.com", "CLIENT_ID=id", "CLIENT_SECRET=secret", "REFRESH_TOKEN=1//token"} {
				env = append(env, "GMAIL"+"_TEST_"+v)
			}
			// A run past the guard would try to reach Google, and the proxy, which nothing listens on,
			// keeps it from leaving the machine.
			env = append(env, "HTTPS_PROXY=http://127.0.0.1:9")
			if marker != "" {
				env = append(env, livecontract.Marker+"="+marker)
			}
			cmd := exec.CommandContext(t.Context(), "go", "test", "-tags", "gmail"+"_live", "-count=1", "-v",
				"-run", "^TestTheAdapterPassesTheContractAgainstGmail$", ".")
			cmd.Env = env
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("the live test failed rather than skipping: %v\n%s", err, out)
			}
			for _, want := range []string{"--- SKIP: TestTheAdapterPassesTheContractAgainstGmail", "go tool livecontract gmail"} {
				if !strings.Contains(string(out), want) {
					t.Errorf("the live test's output lacks %q:\n%s", want, out)
				}
			}
		})
	}
}
