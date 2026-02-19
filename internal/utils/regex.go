package utils

import (
	"regexp"
	"sync"
)

// LazyRegex compiles a regex pattern on first use and caches the result.
type LazyRegex struct {
	pattern string
	once    sync.Once
	re      *regexp.Regexp
}

// NewLazyRegex creates a LazyRegex that will compile pattern on first use.
func NewLazyRegex(pattern string) *LazyRegex {
	return &LazyRegex{pattern: pattern}
}

// Re returns the compiled regexp, compiling it on first call.
// Panics if the pattern is invalid.
func (lr *LazyRegex) Re() *regexp.Regexp {
	lr.once.Do(func() {
		lr.re = regexp.MustCompile(lr.pattern)
	})
	return lr.re
}
