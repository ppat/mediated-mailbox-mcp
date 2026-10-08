//go:build unix

package main

import (
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The private key file is written with mode 0600 and the public key file with 0644 whatever the
// process's umask, a restrictive one that would narrow the public key's mode and an empty one that
// would leave a wider private key mode as it was.
func TestTheKeyFilesModesHoldUnderAnyUmask(t *testing.T) {
	for _, umask := range []int{0o077, 0o000} {
		dir := t.TempDir()
		privatePath, publicPath := filepath.Join(dir, "credential.key"), filepath.Join(dir, "credential.pub")
		previous := syscall.Umask(umask)
		err := run([]string{"-private-key-file", privatePath, "-public-key-file", publicPath}, io.Discard)
		syscall.Umask(previous)
		if err != nil {
			t.Fatalf("run under umask %04o: %v", umask, err)
		}
		for path, mode := range map[string]os.FileMode{privatePath: 0o600, publicPath: 0o644} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != mode {
				t.Errorf("under umask %04o %s has mode %v, want %v", umask, filepath.Base(path), info.Mode().Perm(), mode)
			}
		}
	}
}
