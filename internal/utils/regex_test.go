package utils

import (
	"testing"
)

func TestLazyRegex(t *testing.T) {
	lr := NewLazyRegex(`^\d+$`)

	// First call compiles
	re := lr.Re()
	if !re.MatchString("123") {
		t.Error("expected match for '123'")
	}
	if re.MatchString("abc") {
		t.Error("expected no match for 'abc'")
	}

	// Second call returns same instance
	re2 := lr.Re()
	if re != re2 {
		t.Error("expected same regexp instance on second call")
	}
}

func TestLazyRegexConcurrent(t *testing.T) {
	lr := NewLazyRegex(`\w+`)
	done := make(chan bool, 10)

	for range 10 {
		go func() {
			re := lr.Re()
			if !re.MatchString("hello") {
				t.Error("expected match")
			}
			done <- true
		}()
	}

	for range 10 {
		<-done
	}
}
