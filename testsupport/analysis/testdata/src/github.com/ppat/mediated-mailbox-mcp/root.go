// Package mediatedmailbox stands for a file at the module's root, outside every component.
package mediatedmailbox

import "os"

var _ = os.Getenv("HOME") // want "reads the environment with os.Getenv outside a deployable's composition root"
