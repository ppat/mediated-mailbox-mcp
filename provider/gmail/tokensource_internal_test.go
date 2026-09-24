package gmail

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// These tests hand the source the bodies of Google's token responses as literal bytes, at the one
// seam between the HTTP exchange and the source, because the token endpoint is Google's and a
// stand-in for it would test this package's beliefs about it (ADR-0043). What the source would
// send next is read from the request it builds. The files they read and write are real ones.

var now = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

// account mounts the credentials of one account and names a write-back location in a directory of
// its own.
func account(t *testing.T, refreshToken string) (Files, string) {
	t.Helper()
	mounted, writable := t.TempDir(), t.TempDir()
	files := Files{
		ClientID:     filepath.Join(mounted, "client-id"),
		ClientSecret: filepath.Join(mounted, "client-secret"),
		RefreshToken: filepath.Join(mounted, "refresh-token"),
	}
	for path, value := range map[string]string{
		files.ClientID:     "client-id",
		files.ClientSecret: "client-secret",
		files.RefreshToken: refreshToken,
	} {
		if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return files, filepath.Join(writable, "refresh-token")
}

// start loads the mounted credentials the way a deployable does when it starts.
func start(t *testing.T, files Files, writeBack string, log *bytes.Buffer) *TokenSource {
	t.Helper()
	creds, err := Load(files)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return NewTokenSource(http.DefaultClient, creds, writeBack, slog.New(slog.NewJSONHandler(log, nil)))
}

// readFile returns a file's content, or "absent" when there is no file.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "absent"
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// held is the refresh token the source would send with its next refresh.
func held(src *TokenSource) string {
	return src.refreshForm().Get("refresh_token")
}

// receive hands the source the body of a refresh response.
func receive(t *testing.T, src *TokenSource, body string, at time.Time) {
	t.Helper()
	if err := src.receive([]byte(body), at); err != nil {
		t.Fatalf("receive: %v", err)
	}
}

// Bodies of Google's refresh responses, as its documentation shows them. The first rotates the
// refresh token and the second does not.
const (
	rotatingResponse = `{"access_token": "access", "expires_in": 3599, "refresh_token": "rotated-refresh-token", "scope": "https://www.googleapis.com/auth/gmail.modify", "token_type": "Bearer"}`
	plainResponse    = `{"access_token": "access", "expires_in": 3599, "scope": "https://www.googleapis.com/auth/gmail.modify", "token_type": "Bearer"}`
)

// errorRecords returns the location attribute of every error-level log record.
func errorRecords(t *testing.T, log *bytes.Buffer) []string {
	t.Helper()
	var locations []string
	for line := range strings.Lines(log.String()) {
		var rec struct {
			Level    string `json:"level"`
			Location string `json:"location"`
		}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		if rec.Level == "ERROR" {
			locations = append(locations, rec.Location)
		}
	}
	return locations
}

// F5's application half of the rotation row in docs/VERIFICATIONS.md (ADR-0039). A rotated refresh
// token is kept in memory and written to the write-back location. The secret store's sync and the
// mounted file's refresh are the platform's, stood in for here by copying the location over the
// mounted file. The restart then reads the rotated token.
func TestARotatedCredentialSurvivesARestart(t *testing.T) {
	files, writeBack := account(t, "original-refresh-token")
	var log bytes.Buffer
	src := start(t, files, writeBack, &log)

	receive(t, src, rotatingResponse, now)

	if got := held(src); got != "rotated-refresh-token" {
		t.Errorf("the running process would refresh with %q, want the rotated token", got)
	}
	if got := readFile(t, writeBack); got != "rotated-refresh-token" {
		t.Fatalf("the write-back location holds %q, want the rotated token", got)
	}
	if got := errorRecords(t, &log); len(got) != 0 {
		t.Errorf("error records %q", got)
	}

	if err := os.WriteFile(files.RefreshToken, []byte(readFile(t, writeBack)), 0o600); err != nil {
		t.Fatal(err)
	}
	restarted := start(t, files, writeBack, &log)
	if got := held(restarted); got != "rotated-refresh-token" {
		t.Errorf("after the restart the process would refresh with %q, want the rotated token", got)
	}
}

// A refresh that does not rotate the token leaves the held token as it was and writes nothing.
// Google leaves the field out when it does not rotate, and may return the same token.
func TestARefreshThatDoesNotRotateWritesNothing(t *testing.T) {
	for name, body := range map[string]string{
		"field left out": plainResponse,
		"same token":     `{"access_token": "access", "expires_in": 3599, "refresh_token": "original-refresh-token", "token_type": "Bearer"}`,
	} {
		t.Run(name, func(t *testing.T) {
			files, writeBack := account(t, "original-refresh-token")
			var log bytes.Buffer
			src := start(t, files, writeBack, &log)

			receive(t, src, body, now)

			if got := held(src); got != "original-refresh-token" {
				t.Errorf("the running process would refresh with %q, want the original token", got)
			}
			if got := readFile(t, writeBack); got != "absent" {
				t.Errorf("the write-back location holds %q, want no file", got)
			}
			if got := errorRecords(t, &log); len(got) != 0 {
				t.Errorf("error records %q", got)
			}
		})
	}
}

// A write-back that fails is logged at error level without the token, and the next refresh tries
// it again even though that refresh rotates nothing.
func TestAFailedWriteBackIsLoggedAndTriedAgain(t *testing.T) {
	files, writeBack := account(t, "original-refresh-token")
	// The location's directory is missing at first, so the write fails.
	writeBack = filepath.Join(filepath.Dir(writeBack), "missing", "refresh-token")
	var log bytes.Buffer
	src := start(t, files, writeBack, &log)

	receive(t, src, rotatingResponse, now)

	if diff := cmp.Diff([]string{writeBack}, errorRecords(t, &log), compare.Options); diff != "" {
		t.Errorf("error records naming the location (-want +got):\n%s", diff)
	}
	if strings.Contains(log.String(), "rotated-refresh-token") {
		t.Errorf("the log holds the refresh token: %s", log.String())
	}
	if got := held(src); got != "rotated-refresh-token" {
		t.Errorf("the running process would refresh with %q, want the rotated token", got)
	}

	if err := os.Mkdir(filepath.Dir(writeBack), 0o700); err != nil {
		t.Fatal(err)
	}
	receive(t, src, plainResponse, now.Add(time.Hour))

	if got := readFile(t, writeBack); got != "rotated-refresh-token" {
		t.Errorf("after the retry the write-back location holds %q, want the rotated token", got)
	}
}

// An access token is used until a minute before its stated expiry, and refreshed from then on.
func TestAnAccessTokenIsUsedUntilItNearlyExpires(t *testing.T) {
	files, writeBack := account(t, "original-refresh-token")
	var log bytes.Buffer
	src := start(t, files, writeBack, &log)
	if _, ok := src.cached(now); ok {
		t.Fatal("a source that never refreshed has a cached access token")
	}
	receive(t, src, `{"access_token": "access", "expires_in": 3600, "token_type": "Bearer"}`, now)

	for _, c := range []struct {
		after time.Duration
		want  bool
	}{
		{0, true},
		{58*time.Minute + 59*time.Second, true},
		{59 * time.Minute, false},
		{time.Hour, false},
	} {
		if _, got := src.cached(now.Add(c.after)); got != c.want {
			t.Errorf("cached %v after the refresh = %v, want %v", c.after, got, c.want)
		}
	}
}

// Starting a source removes a new file an earlier process left beside the location when it
// stopped part of the way through a write-back, since that file holds a live refresh token. A new
// file younger than ten minutes may be another deployable's write in progress, so it stays.
func TestStartingRemovesANewFileLeftBesideTheLocation(t *testing.T) {
	files, writeBack := account(t, "original-refresh-token")
	dir := filepath.Dir(writeBack)
	for name, content := range map[string]string{
		".refresh-token.1234567": "live-token-from-a-crash",
		".refresh-token.7654321": "another-deployables-write-in-progress",
		"refresh-token":          "previous-rotated-token",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-11 * time.Minute)
	for _, name := range []string{".refresh-token.1234567", "refresh-token"} {
		if err := os.Chtimes(filepath.Join(dir, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	var log bytes.Buffer
	start(t, files, writeBack, &log)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if diff := cmp.Diff([]string{".refresh-token.7654321", "refresh-token"}, names, compare.Options); diff != "" {
		t.Errorf("directory entries (-want +got):\n%s", diff)
	}
	if got := readFile(t, writeBack); got != "previous-rotated-token" {
		t.Errorf("the write-back location holds %q, want its previous content", got)
	}
}

// The refresh request carries the client and the held refresh token.
func TestTheRefreshRequest(t *testing.T) {
	files, writeBack := account(t, "original-refresh-token")
	var log bytes.Buffer
	src := start(t, files, writeBack, &log)
	want := url.Values{
		"client_id":     {"client-id"},
		"client_secret": {"client-secret"},
		"grant_type":    {"refresh_token"},
		"refresh_token": {"original-refresh-token"},
	}
	if diff := cmp.Diff(want, src.refreshForm(), compare.Options); diff != "" {
		t.Errorf("refresh request (-want +got):\n%s", diff)
	}
}

// A new file beside the location counts as left behind only once it is more than ten minutes
// old, measured from its last modification.
func TestANewFileLeftBesideTheLocationIsRemovedOnlyOnceItIsOld(t *testing.T) {
	written := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		age  time.Duration
		kept bool
	}{
		{0, true},
		{10 * time.Minute, true},
		{10*time.Minute + time.Second, false},
		{24 * time.Hour, false},
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "refresh-token")
		unfinished := filepath.Join(dir, ".refresh-token.1234567")
		if err := os.WriteFile(unfinished, []byte("token"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(unfinished, written, written); err != nil {
			t.Fatal(err)
		}
		if err := removeUnfinished(path, written.Add(c.age)); err != nil {
			t.Fatalf("removeUnfinished: %v", err)
		}
		if kept := readFile(t, unfinished) != "absent"; kept != c.kept {
			t.Errorf("a new file %v old: kept = %v, want %v", c.age, kept, c.kept)
		}
	}
}
