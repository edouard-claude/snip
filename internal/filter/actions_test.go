package filter

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func lines(s ...string) ActionResult {
	return ActionResult{Lines: s, Metadata: make(map[string]any)}
}

func TestKeepLines(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		pattern string
		want    int
	}{
		{"keep non-blank", []string{"hello", "", "world", ""}, `\S`, 2},
		{"keep digits", []string{"abc", "123", "def", "456"}, `^\d+$`, 2},
		{"empty input", nil, `\S`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := keepLines(lines(tt.input...), map[string]any{"pattern": tt.pattern})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Lines) != tt.want {
				t.Errorf("got %d lines, want %d", len(res.Lines), tt.want)
			}
		})
	}
}

func TestRemoveLines(t *testing.T) {
	input := lines("Compiling foo", "Running test", "Compiling bar", "test result: ok")
	res, err := removeLines(input, map[string]any{"pattern": `^Compiling`})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Errorf("got %d lines, want 2", len(res.Lines))
	}
}

func TestTruncateLines(t *testing.T) {
	input := lines("short", "this is a very long line that should be truncated at some point")
	res, err := truncateLines(input, map[string]any{"max": 20, "ellipsis": "..."})
	if err != nil {
		t.Fatal(err)
	}
	if len([]rune(res.Lines[1])) > 20 {
		t.Errorf("line not truncated: %q (len=%d)", res.Lines[1], len([]rune(res.Lines[1])))
	}
	if !strings.HasSuffix(res.Lines[1], "...") {
		t.Errorf("missing ellipsis: %q", res.Lines[1])
	}
	if res.Lines[0] != "short" {
		t.Errorf("short line modified: %q", res.Lines[0])
	}
}

func TestTruncateBytesUTF8Boundary(t *testing.T) {
	// "héllo" is h(1) é(2) l(1) l(1) o(1) = 6 bytes.
	// Cutting at max=2 would land mid-rune through é (bytes 1-2).
	// We must back off to byte 1 so we never emit invalid UTF-8.
	input := lines("héllo")
	res, err := truncateBytes(input, map[string]any{"max": 2})
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(res.Lines, "\n")
	if !utf8.ValidString(out) {
		t.Errorf("truncated output is not valid UTF-8: %q (% x)", out, out)
	}
	if out != "h" {
		t.Errorf("got %q, want %q", out, "h")
	}
}

func TestTruncateBytesNoCut(t *testing.T) {
	input := lines("short")
	res, err := truncateBytes(input, map[string]any{"max": 100})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Lines, "\n") != "short" {
		t.Errorf("input mutated: %v", res.Lines)
	}
}

func TestTruncateBytesEmoji(t *testing.T) {
	// Emoji "🎉" is 4 bytes (F0 9F 8E 89). max=3 lands mid-emoji.
	input := lines("ab🎉cd")
	res, err := truncateBytes(input, map[string]any{"max": 4})
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(res.Lines, "\n")
	if !utf8.ValidString(out) {
		t.Errorf("truncated output is not valid UTF-8: %q (% x)", out, out)
	}
	if out != "ab" {
		t.Errorf("got %q, want %q", out, "ab")
	}
}

func TestStripANSI(t *testing.T) {
	input := lines("\x1b[31mred\x1b[0m", "normal")
	res, err := stripANSI(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines[0] != "red" {
		t.Errorf("ANSI not stripped: %q", res.Lines[0])
	}
}

func TestHead(t *testing.T) {
	input := lines("1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11")
	res, err := head(input, map[string]any{"n": 5})
	if err != nil {
		t.Fatal(err)
	}
	// 5 lines + overflow message
	if len(res.Lines) != 6 {
		t.Errorf("got %d lines, want 6", len(res.Lines))
	}
	if !strings.Contains(res.Lines[5], "+6 more") {
		t.Errorf("overflow msg: %q", res.Lines[5])
	}
}

func TestHeadNoOverflow(t *testing.T) {
	input := lines("1", "2", "3")
	res, err := head(input, map[string]any{"n": 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 3 {
		t.Errorf("got %d lines, want 3", len(res.Lines))
	}
}

func TestTail(t *testing.T) {
	input := lines("1", "2", "3", "4", "5")
	res, err := tail(input, map[string]any{"n": 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Errorf("got %d lines, want 2", len(res.Lines))
	}
	if res.Lines[0] != "4" || res.Lines[1] != "5" {
		t.Errorf("got %v", res.Lines)
	}
}

func TestGroupBy(t *testing.T) {
	input := lines("ERR foo", "WARN bar", "ERR baz", "ERR qux", "WARN quux")
	res, err := groupBy(input, map[string]any{
		"pattern": `^(\w+)`,
		"format":  "{{.Key}}: {{.Count}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("got %d groups, want 2: %v", len(res.Lines), res.Lines)
	}
	// ERR has 3, should be first
	if !strings.HasPrefix(res.Lines[0], "ERR") {
		t.Errorf("expected ERR first, got %q", res.Lines[0])
	}
}

func TestGroupByAppend(t *testing.T) {
	input := lines("file.go", "file.json", "other.go", "data.json", "lib.go")
	res, err := groupBy(input, map[string]any{
		"pattern": `\.(\w+)$`,
		"format":  "{{.Key}}: {{.Count}}",
		"append":  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should have original 5 lines + 2 group lines
	if len(res.Lines) != 7 {
		t.Fatalf("got %d lines, want 7: %v", len(res.Lines), res.Lines)
	}
	// Original lines preserved at the start
	if res.Lines[0] != "file.go" {
		t.Errorf("first line should be original, got %q", res.Lines[0])
	}
	// Group summary appended at the end
	if !strings.HasPrefix(res.Lines[5], "go") {
		t.Errorf("expected 'go' group, got %q", res.Lines[5])
	}
}

func TestGroupByAppendAliasingIsolation(t *testing.T) {
	// Verify that the backing array fix prevents mutation of the original
	// input slice from leaking into the groupBy output.
	original := []string{"a.go", "b.json", "c.go"}
	copyForInput := make([]string, len(original))
	copy(copyForInput, original)

	input := ActionResult{Lines: copyForInput, Metadata: nil}
	res, err := groupBy(input, map[string]any{
		"pattern": `\.(\w+)$`,
		"format":  "{{.Key}}: {{.Count}}",
		"append":  true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Mutate the original backing array — should not affect result
	copyForInput[0] = "CORRUPTED"
	copyForInput[1] = "CORRUPTED"

	// Original lines in result must still be intact
	for i, want := range original {
		if res.Lines[i] != want {
			t.Errorf("backing array aliasing at line %d: got %q, want %q", i, res.Lines[i], want)
		}
	}
}

func TestDedup(t *testing.T) {
	input := lines("error: foo", "error: foo", "error: foo", "warn: bar", "warn: bar")
	res, err := dedup(input, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("got %d lines: %v", len(res.Lines), res.Lines)
	}
	if !strings.Contains(res.Lines[0], "x3") {
		t.Errorf("expected x3: %q", res.Lines[0])
	}
}

func TestRegexExtract(t *testing.T) {
	input := lines("file: main.go line: 42", "file: utils.go line: 10")
	res, err := regexExtract(input, map[string]any{
		"pattern": `file: (\S+) line: (\d+)`,
		"format":  "$1:$2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("got %d lines", len(res.Lines))
	}
	if res.Lines[0] != "main.go:42" {
		t.Errorf("got %q", res.Lines[0])
	}
}

func TestAggregate(t *testing.T) {
	input := lines("PASS foo", "FAIL bar", "PASS baz", "PASS qux", "FAIL quux")
	res, err := aggregate(input, map[string]any{
		"patterns": map[string]any{
			"pass": `^PASS`,
			"fail": `^FAIL`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should have "fail: 2" and "pass: 3" (sorted alphabetically)
	if len(res.Lines) != 2 {
		t.Fatalf("got %d lines: %v", len(res.Lines), res.Lines)
	}
}

func TestAggregateAppend(t *testing.T) {
	input := lines("PASS foo", "FAIL bar", "PASS baz")
	res, err := aggregate(input, map[string]any{
		"patterns": map[string]any{
			"pass": `^PASS`,
			"fail": `^FAIL`,
		},
		"append": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should have original 3 lines + 2 aggregate lines
	if len(res.Lines) != 5 {
		t.Fatalf("got %d lines, want 5: %v", len(res.Lines), res.Lines)
	}
	if res.Lines[0] != "PASS foo" {
		t.Errorf("first line should be original, got %q", res.Lines[0])
	}
}

func TestAggregateAppendAliasingIsolation(t *testing.T) {
	original := []string{"PASS a", "FAIL b", "PASS c"}
	copyForInput := make([]string, len(original))
	copy(copyForInput, original)

	input := ActionResult{Lines: copyForInput, Metadata: nil}
	res, err := aggregate(input, map[string]any{
		"patterns": map[string]any{
			"pass": `^PASS`,
			"fail": `^FAIL`,
		},
		"append": true,
	})
	if err != nil {
		t.Fatal(err)
	}

	copyForInput[0] = "CORRUPTED"
	copyForInput[1] = "CORRUPTED"

	for i, want := range original {
		if res.Lines[i] != want {
			t.Errorf("backing array aliasing at line %d: got %q, want %q", i, res.Lines[i], want)
		}
	}
}

func TestFormatTemplate(t *testing.T) {
	input := ActionResult{
		Lines:    []string{"a", "b", "c"},
		Metadata: map[string]any{},
	}
	res, err := formatTemplate(input, map[string]any{
		"template": "{{.count}} items:\n{{.lines}}",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "3 items") {
		t.Errorf("missing count: %q", joined)
	}
}

func TestCompactPath(t *testing.T) {
	input := lines("src/main.go", "lib/utils.js", "README.md")
	res, err := compactPath(input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines[0] != "main.go" {
		t.Errorf("got %q", res.Lines[0])
	}
	if res.Lines[2] != "README.md" {
		t.Errorf("got %q", res.Lines[2])
	}
}

func TestJsonExtract(t *testing.T) {
	input := lines(`{"name":"snip","version":"0.1","count":42}`)
	res, err := jsonExtract(input, map[string]any{
		"fields": []any{"name", "version"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("got %d lines: %v", len(res.Lines), res.Lines)
	}
}

func TestNdjsonStream(t *testing.T) {
	input := lines(
		`{"action":"run","pkg":"foo"}`,
		`{"action":"pass","pkg":"foo"}`,
		`{"action":"run","pkg":"bar"}`,
		`{"action":"fail","pkg":"bar"}`,
	)
	res, err := ndjsonStream(input, map[string]any{"group_by": "pkg"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("got %d groups: %v", len(res.Lines), res.Lines)
	}
}

func TestStateMachine(t *testing.T) {
	input := lines(
		"running tests...",
		"test foo: ok",
		"test bar: FAILED",
		"--- failures ---",
		"bar: assertion error",
		"--- end ---",
	)
	res, err := stateMachine(input, map[string]any{
		"states": map[string]any{
			"start": map[string]any{
				"keep":  `^test`,
				"until": `^--- failures`,
				"next":  "failures",
			},
			"failures": map[string]any{
				"keep":  `.`,
				"until": `^--- end`,
				"next":  "done",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should keep "test foo: ok", "test bar: FAILED", "bar: assertion error"
	if len(res.Lines) != 3 {
		t.Errorf("got %d lines: %v", len(res.Lines), res.Lines)
	}
}

func TestEmptyInput(t *testing.T) {
	empty := ActionResult{Lines: nil, Metadata: make(map[string]any)}
	actionTests := []struct {
		name   string
		fn     ActionFunc
		params map[string]any
	}{
		{"keepLines", keepLines, map[string]any{"pattern": `\S`}},
		{"removeLines", removeLines, map[string]any{"pattern": `\S`}},
		{"truncateLines", truncateLines, map[string]any{"max": 80}},
		{"stripANSI", stripANSI, nil},
		{"head", head, map[string]any{"n": 5}},
		{"tail", tail, map[string]any{"n": 5}},
		{"compactPath", compactPath, nil},
	}
	for _, tt := range actionTests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.fn(empty, tt.params)
			if err != nil {
				t.Fatalf("unexpected error on empty input: %v", err)
			}
			if len(res.Lines) != 0 {
				t.Errorf("expected empty output, got %d lines", len(res.Lines))
			}
		})
	}
}

func TestReplace(t *testing.T) {
	input := lines("make[1]: building foo", "make[2]: entering dir", "gcc -o main main.c")
	res, err := replace(input, map[string]any{
		"pattern":     `^make\[\d+\]: `,
		"replacement": "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines[0] != "building foo" {
		t.Errorf("got %q, want %q", res.Lines[0], "building foo")
	}
	if res.Lines[2] != "gcc -o main main.c" {
		t.Errorf("unmatched line modified: %q", res.Lines[2])
	}
}

func TestReplaceWithBackref(t *testing.T) {
	input := lines("error[E0308]: mismatched types", "warning[W0001]: unused var")
	res, err := replace(input, map[string]any{
		"pattern":     `^(error|warning)\[(\w+)\]: `,
		"replacement": "$1 $2: ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines[0] != "error E0308: mismatched types" {
		t.Errorf("got %q", res.Lines[0])
	}
}

func TestMatchOutput(t *testing.T) {
	input := lines("To github.com:user/repo.git", "abc1234..def5678 main -> main")
	res, err := matchOutput(input, map[string]any{
		"pattern": `.*->.*`,
		"message": "ok pushed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 1 || res.Lines[0] != "ok pushed" {
		t.Errorf("got %v, want [ok pushed]", res.Lines)
	}
}

func TestMatchOutputUnless(t *testing.T) {
	input := lines("error: failed to push some refs", "hint: update rejected")
	res, err := matchOutput(input, map[string]any{
		"pattern": `.*`,
		"unless":  `(?i)error|fatal|rejected`,
		"message": "ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	// unless matched, so output should be unchanged
	if len(res.Lines) != 2 {
		t.Errorf("expected passthrough, got %v", res.Lines)
	}
}

func TestMatchOutputNoMatch(t *testing.T) {
	input := lines("some random output")
	res, err := matchOutput(input, map[string]any{
		"pattern": `^SPECIFIC_PATTERN$`,
		"message": "ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 1 || res.Lines[0] != "some random output" {
		t.Errorf("expected passthrough, got %v", res.Lines)
	}
}

func TestOnEmpty(t *testing.T) {
	empty := ActionResult{Lines: nil, Metadata: make(map[string]any)}
	res, err := onEmpty(empty, map[string]any{"message": "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 1 || res.Lines[0] != "ok" {
		t.Errorf("got %v, want [ok]", res.Lines)
	}
}

func TestOnEmptyWithBlanks(t *testing.T) {
	input := lines("", "  ", "\t")
	res, err := onEmpty(input, map[string]any{"message": "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 1 || res.Lines[0] != "ok" {
		t.Errorf("got %v, want [ok]", res.Lines)
	}
}

func TestOnEmptyWithContent(t *testing.T) {
	input := lines("actual content")
	res, err := onEmpty(input, map[string]any{"message": "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 1 || res.Lines[0] != "actual content" {
		t.Errorf("expected passthrough, got %v", res.Lines)
	}
}

func TestOnEmptyDefaultMessage(t *testing.T) {
	empty := ActionResult{Lines: nil, Metadata: make(map[string]any)}
	res, err := onEmpty(empty, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Lines) != 1 || res.Lines[0] != "ok" {
		t.Errorf("got %v, want [ok]", res.Lines)
	}
}

func TestGetAction(t *testing.T) {
	for _, name := range []string{"keep_lines", "remove_lines", "head", "format_template", "replace", "match_output", "on_empty"} {
		if _, ok := GetAction(name); !ok {
			t.Errorf("action %q not found", name)
		}
	}
	if _, ok := GetAction("nonexistent"); ok {
		t.Error("expected nonexistent action to not be found")
	}
}
