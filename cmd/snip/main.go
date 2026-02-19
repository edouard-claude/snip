package main

import (
	"os"

	snip "snip"
	"snip/internal/cli"
	"snip/internal/filter"
)

func main() {
	fs := snip.EmbeddedFilters
	filter.EmbeddedFS = &fs
	exitCode := cli.Run(os.Args)
	os.Exit(exitCode)
}
