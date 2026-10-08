// Package settings stands for a shared library.
package settings

import "os"

func Load() []string {
	return os.Environ() // want "reads the environment with os.Environ"
}
