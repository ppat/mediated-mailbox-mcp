package gmail

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Files names the mounted credential files, one value per file (ADR-0038). The deployable's
// composition root supplies the paths. Nothing in this package reads a credential from the
// environment.
type Files struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

// Credentials are the installed-app client and the account's refresh token.
type Credentials struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

// Load reads the credentials from the mounted files. It reads those files alone, so a restart
// takes whatever the files hold at that moment. A rotated refresh token reaches them through the
// secret store (ADR-0039), never through this package reading the write-back location.
func Load(files Files) (Credentials, error) {
	var c Credentials
	var errs []error
	for _, f := range []struct {
		path string
		dst  *string
	}{
		{files.ClientID, &c.ClientID},
		{files.ClientSecret, &c.ClientSecret},
		{files.RefreshToken, &c.RefreshToken},
	} {
		value, err := readValue(f.path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		*f.dst = value
	}
	if err := errors.Join(errs...); err != nil {
		return Credentials{}, err
	}
	return c, nil
}

// readValue reads one credential file. Surrounding whitespace is dropped, since a file written by
// hand or by a tool often ends in a newline and no OAuth value holds whitespace.
func readValue(path string) (string, error) {
	if path == "" {
		return "", errors.New("gmail: a credential file path is empty")
	}
	b, err := os.ReadFile(path) //nolint:gosec // the path is the mounted credential file the composition root names
	if err != nil {
		return "", fmt.Errorf("gmail: reading a credential file: %w", err)
	}
	value := strings.TrimSpace(string(b))
	if value == "" {
		return "", fmt.Errorf("gmail: the credential file %s is empty", path)
	}
	return value, nil
}

// WriteBack writes a rotated refresh token to the write-back location, the one writable file of
// ADR-0039, replacing its content whole. The token goes to a new file in the same directory, which
// is synced and then renamed over path, so a crash or a failed write at any point leaves path
// holding either its previous content or the whole new token, never part of one. The directory
// must therefore be writable, not only the file.
//
// The new file is created with mode 0600 and owned by the deployable's user, and the rename gives
// the location that mode and owner whatever it had before. A process that syncs the location to
// the store while running as another user loses read access at the first rotation.
//
// A new file left beside the location by a process that stopped part of the way through holds a
// live refresh token outside the location, so WriteBack first removes any such file older than
// unfinishedAge. Removing and creating a file both need write access to the directory, so when
// the removal fails the write would too, and WriteBack stops there.
func WriteBack(path, refreshToken string) error {
	if refreshToken == "" {
		return errors.New("gmail: refusing to write back an empty refresh token")
	}
	if err := removeUnfinished(path, time.Now()); err != nil {
		return fmt.Errorf("gmail: writing back the refresh token: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, unfinishedPrefix(path)+"*")
	if err != nil {
		return fmt.Errorf("gmail: writing back the refresh token: %w", err)
	}
	if err := fill(tmp, refreshToken); err != nil {
		return errors.Join(fmt.Errorf("gmail: writing back the refresh token: %w", err), os.Remove(tmp.Name()))
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return errors.Join(fmt.Errorf("gmail: writing back the refresh token: %w", err), os.Remove(tmp.Name()))
	}
	// The rename is what makes the new token durable across a machine crash, so the directory
	// entry is synced too.
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("gmail: writing back the refresh token: %w", err)
	}
	return nil
}

// unfinishedPrefix is the name prefix of the new file WriteBack creates beside the location. It is
// hidden, and no location's own name can carry it.
func unfinishedPrefix(path string) string {
	return "." + filepath.Base(path) + "."
}

// unfinishedAge is how old a new file beside the location must be before it counts as left behind.
// Every deployable that calls a provider writes back to the same location (ADR-0039), so a younger
// file may be another deployable's write still in progress, and removing it would fail that write
// until its next refresh, about an hour later. A write-back creates, fills, syncs and renames a file
// of about a hundred bytes, which takes milliseconds. Ten minutes leaves room for a stalled disk and
// for clock skew between the hosts sharing the location.
//
// The cost is that a file left by a process that died part of the way through a write-back stays
// in the directory until a start or a write-back more than ten minutes after it was last written.
// A restart soon after the crash keeps it, and the next write-back comes only with the next
// rotation. Such a file holds all or part of a refresh token, and nothing in this package reads it.
const unfinishedAge = 10 * time.Minute

// removeUnfinished removes every new file an earlier write-back left beside the location, meaning
// one last modified more than unfinishedAge before now. A directory that does not exist holds none.
func removeUnfinished(path string, now time.Time) error {
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), unfinishedPrefix(path)) {
			continue
		}
		info, err := e.Info()
		if errors.Is(err, fs.ErrNotExist) {
			// Its writer renamed or removed it since the directory was read.
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if now.Sub(info.ModTime()) > unfinishedAge {
			errs = append(errs, os.Remove(filepath.Join(dir, e.Name())))
		}
	}
	return errors.Join(errs...)
}

// fill writes the token into the new file, syncs it and closes it.
func fill(f *os.File, token string) error {
	_, err := f.WriteString(token)
	if err == nil {
		err = f.Sync()
	}
	return errors.Join(err, f.Close())
}

// syncDir syncs a directory, so a rename inside it survives a crash of the machine.
func syncDir(dir string) error {
	d, err := os.Open(dir) //nolint:gosec // the directory holding the write-back location the composition root names
	if err != nil {
		return err
	}
	return errors.Join(d.Sync(), d.Close())
}
