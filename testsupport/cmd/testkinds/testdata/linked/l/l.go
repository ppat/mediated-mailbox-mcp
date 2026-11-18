// Package l is linked by r's recording test and embeds a file, so the filter check reads both.
package l

import _ "embed"

//go:embed rules.txt
var Rules string
