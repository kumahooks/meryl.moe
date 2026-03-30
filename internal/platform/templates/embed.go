package templates

import (
	embed "embed"
)

//go:embed layouts components pages
var templateFS embed.FS
