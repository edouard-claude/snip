package main

import (
	"os"

	snip "github.com/edouard-claude/snip"
	"github.com/edouard-claude/snip/internal/cli"
	"github.com/edouard-claude/snip/internal/filter"
)

func main() {
	fs := snip.EmbeddedFilters
	filter.EmbeddedFS = &fs
	exitCode := cli.Run(os.Args)
	os.Exit(exitCode)
}
