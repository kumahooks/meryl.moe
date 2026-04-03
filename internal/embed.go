package internal

import (
	"embed"
)

// templatesEmbedFS holds all templates baked into the binary at compile time:
// shared layouts, shared components, and each module's page template.
//
//go:embed platform/templates/layouts platform/templates/components modules/*/*.html
var templatesEmbedFS embed.FS
