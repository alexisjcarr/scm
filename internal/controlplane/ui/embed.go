package ui

import "embed"

// TemplatesFS embeds the control plane UI templates into the daemon binary.
//
//go:embed templates/*.tmpl
var TemplatesFS embed.FS
