package internal

import (
	embed "embed"
)

// assetsFS holds all templates baked into the binary at compile time:
// shared layouts, shared components, and each module's page template.
//
//go:embed platform/templates/layouts platform/templates/components modules/*/*.html
var assetsFS embed.FS
