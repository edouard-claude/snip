package snip

import "embed"

//go:embed filters/*.yaml
var EmbeddedFilters embed.FS
