package filter

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"snip/internal/utils"
)

// Registry of built-in actions.
var actions = map[string]ActionFunc{
	"keep_lines":      keepLines,
	"remove_lines":    removeLines,
	"truncate_lines":  truncateLines,
	"strip_ansi":      stripANSI,
	"head":            head,
	"tail":            tail,
	"group_by":        groupBy,
	"dedup":           dedup,
	"json_extract":    jsonExtract,
	"json_schema":     jsonSchema,
	"ndjson_stream":   ndjsonStream,
	"regex_extract":   regexExtract,
	"state_machine":   stateMachine,
	"aggregate":       aggregate,
	"format_template": formatTemplate,
	"compact_path":    compactPath,
}

// GetAction returns the ActionFunc for the given action name.
func GetAction(name string) (ActionFunc, bool) {
	fn, ok := actions[name]
	return fn, ok
}

// --- helpers ---

func getStr(params map[string]any, key string) string {
	v, ok := params[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func getInt(params map[string]any, key string, def int) int {
	v, ok := params[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case int64:
		return int(n)
	default:
		return def
	}
}

func compilePattern(params map[string]any, key string) (*regexp.Regexp, error) {
	p := getStr(params, key)
	if p == "" {
		return nil, fmt.Errorf("missing %q param", key)
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, fmt.Errorf("compile %q: %w", p, err)
	}
	return re, nil
}

// --- actions ---

func keepLines(input ActionResult, params map[string]any) (ActionResult, error) {
	re, err := compilePattern(params, "pattern")
	if err != nil {
		return input, err
	}
	var out []string
	for _, line := range input.Lines {
		if re.MatchString(line) {
			out = append(out, line)
		}
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

func removeLines(input ActionResult, params map[string]any) (ActionResult, error) {
	re, err := compilePattern(params, "pattern")
	if err != nil {
		return input, err
	}
	var out []string
	for _, line := range input.Lines {
		if !re.MatchString(line) {
			out = append(out, line)
		}
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

func truncateLines(input ActionResult, params map[string]any) (ActionResult, error) {
	max := getInt(params, "max", 80)
	ellipsis := getStr(params, "ellipsis")
	if ellipsis == "" {
		ellipsis = "..."
	}
	ellipsisLen := len([]rune(ellipsis))
	if max <= ellipsisLen {
		max = ellipsisLen + 1
	}
	out := make([]string, len(input.Lines))
	for i, line := range input.Lines {
		if len([]rune(line)) > max {
			runes := []rune(line)
			out[i] = string(runes[:max-ellipsisLen]) + ellipsis
		} else {
			out[i] = line
		}
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

func stripANSI(input ActionResult, params map[string]any) (ActionResult, error) {
	out := make([]string, len(input.Lines))
	for i, line := range input.Lines {
		out[i] = utils.StripANSI(line)
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

func head(input ActionResult, params map[string]any) (ActionResult, error) {
	n := getInt(params, "n", 10)
	if len(input.Lines) <= n {
		return input, nil
	}
	out := make([]string, n)
	copy(out, input.Lines[:n])
	remaining := len(input.Lines) - n
	msg := getStr(params, "overflow_msg")
	if msg == "" {
		msg = fmt.Sprintf("+%d more lines", remaining)
	}
	out = append(out, msg)
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

func tail(input ActionResult, params map[string]any) (ActionResult, error) {
	n := getInt(params, "n", 10)
	if len(input.Lines) <= n {
		return input, nil
	}
	start := len(input.Lines) - n
	out := make([]string, n)
	copy(out, input.Lines[start:])
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

func groupBy(input ActionResult, params map[string]any) (ActionResult, error) {
	re, err := compilePattern(params, "pattern")
	if err != nil {
		return input, err
	}
	top := getInt(params, "top", 0)
	fmtStr := getStr(params, "format")
	if fmtStr == "" {
		fmtStr = "{{.Key}}: {{.Count}}"
	}

	groups := make(map[string]int)
	var order []string
	for _, line := range input.Lines {
		m := re.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		key := m[1]
		if groups[key] == 0 {
			order = append(order, key)
		}
		groups[key]++
	}

	// Sort by count descending
	sort.Slice(order, func(i, j int) bool {
		return groups[order[i]] > groups[order[j]]
	})

	if top > 0 && len(order) > top {
		order = order[:top]
	}

	tmpl, err := template.New("group").Parse(fmtStr)
	if err != nil {
		return input, fmt.Errorf("format template: %w", err)
	}

	var out []string
	for _, key := range order {
		var buf strings.Builder
		if err := tmpl.Execute(&buf, map[string]any{"Key": key, "Count": groups[key]}); err != nil {
			return input, fmt.Errorf("group_by template: %w", err)
		}
		out = append(out, buf.String())
	}

	meta := copyMeta(input.Metadata)
	meta["groups"] = groups
	return ActionResult{Lines: out, Metadata: meta}, nil
}

func dedup(input ActionResult, params map[string]any) (ActionResult, error) {
	top := getInt(params, "top", 0)

	// Build normalize patterns
	var normalizers []*regexp.Regexp
	if raw, ok := params["normalize"]; ok {
		if list, ok := raw.([]any); ok {
			for _, item := range list {
				if s, ok := item.(string); ok {
					if re, err := regexp.Compile(s); err == nil {
						normalizers = append(normalizers, re)
					}
				}
			}
		}
	}

	type entry struct {
		normalized string
		count      int
	}

	seen := make(map[string]*entry)
	var order []string

	for _, line := range input.Lines {
		norm := line
		for _, re := range normalizers {
			norm = re.ReplaceAllString(norm, "")
		}
		norm = strings.TrimSpace(norm)

		if e, ok := seen[norm]; ok {
			e.count++
		} else {
			seen[norm] = &entry{normalized: norm, count: 1}
			order = append(order, norm)
		}
	}

	sort.Slice(order, func(i, j int) bool {
		return seen[order[i]].count > seen[order[j]].count
	})

	if top > 0 && len(order) > top {
		order = order[:top]
	}

	var out []string
	for _, key := range order {
		e := seen[key]
		if e.count > 1 {
			out = append(out, fmt.Sprintf("%s (x%d)", e.normalized, e.count))
		} else {
			out = append(out, e.normalized)
		}
	}

	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

func jsonExtract(input ActionResult, params map[string]any) (ActionResult, error) {
	fieldsRaw, ok := params["fields"]
	if !ok {
		return input, fmt.Errorf("json_extract: missing 'fields' param")
	}
	fields, ok := toStringSlice(fieldsRaw)
	if !ok {
		return input, fmt.Errorf("json_extract: 'fields' must be a list of strings")
	}
	fmtStr := getStr(params, "format")

	joined := strings.Join(input.Lines, "\n")
	var data map[string]any
	if err := json.Unmarshal([]byte(joined), &data); err != nil {
		return input, fmt.Errorf("json_extract: parse: %w", err)
	}

	if fmtStr != "" {
		tmpl, err := template.New("json").Parse(fmtStr)
		if err != nil {
			return input, err
		}
		extracted := make(map[string]any)
		for _, f := range fields {
			extracted[f] = data[f]
		}
		var buf strings.Builder
		if err := tmpl.Execute(&buf, extracted); err != nil {
			return input, fmt.Errorf("json_extract template: %w", err)
		}
		return ActionResult{Lines: strings.Split(buf.String(), "\n"), Metadata: input.Metadata}, nil
	}

	var out []string
	for _, f := range fields {
		if v, ok := data[f]; ok {
			out = append(out, fmt.Sprintf("%s: %v", f, v))
		}
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

func jsonSchema(input ActionResult, params map[string]any) (ActionResult, error) {
	maxDepth := getInt(params, "max_depth", 3)

	joined := strings.Join(input.Lines, "\n")
	var data any
	if err := json.Unmarshal([]byte(joined), &data); err != nil {
		return input, fmt.Errorf("json_schema: parse: %w", err)
	}

	schema := extractSchema(data, 0, maxDepth)
	lines := strings.Split(schema, "\n")
	return ActionResult{Lines: lines, Metadata: input.Metadata}, nil
}

func extractSchema(v any, depth, maxDepth int) string {
	if depth >= maxDepth {
		return "..."
	}
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			return "{}"
		}
		var parts []string
		for k, child := range val {
			parts = append(parts, fmt.Sprintf("%s%s: %s", strings.Repeat("  ", depth+1), k, extractSchema(child, depth+1, maxDepth)))
		}
		sort.Strings(parts)
		return "{\n" + strings.Join(parts, "\n") + "\n" + strings.Repeat("  ", depth) + "}"
	case []any:
		if len(val) == 0 {
			return "[]"
		}
		return "[" + extractSchema(val[0], depth, maxDepth) + "]"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func ndjsonStream(input ActionResult, params map[string]any) (ActionResult, error) {
	groupField := getStr(params, "group_by")
	fmtStr := getStr(params, "format")

	type group struct {
		key    string
		events []map[string]any
	}

	groups := make(map[string]*group)
	var order []string

	for _, line := range input.Lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		key := ""
		if groupField != "" {
			if v, ok := obj[groupField]; ok {
				key = fmt.Sprintf("%v", v)
			}
		}

		if _, ok := groups[key]; !ok {
			groups[key] = &group{key: key}
			order = append(order, key)
		}
		groups[key].events = append(groups[key].events, obj)
	}

	var out []string
	if fmtStr != "" {
		tmpl, err := template.New("ndjson").Parse(fmtStr)
		if err != nil {
			return input, err
		}
		for _, key := range order {
			g := groups[key]
			var buf strings.Builder
			if err := tmpl.Execute(&buf, map[string]any{
				"Key":    g.key,
				"Count":  len(g.events),
				"Events": g.events,
			}); err != nil {
				return input, fmt.Errorf("ndjson_stream template: %w", err)
			}
			out = append(out, buf.String())
		}
	} else {
		for _, key := range order {
			g := groups[key]
			out = append(out, fmt.Sprintf("%s: %d events", g.key, len(g.events)))
		}
	}

	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

func regexExtract(input ActionResult, params map[string]any) (ActionResult, error) {
	re, err := compilePattern(params, "pattern")
	if err != nil {
		return input, err
	}
	fmtStr := getStr(params, "format")

	var out []string
	for _, line := range input.Lines {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if fmtStr != "" {
			result := fmtStr
			for i, match := range m {
				result = strings.ReplaceAll(result, fmt.Sprintf("$%d", i), match)
			}
			out = append(out, result)
		} else {
			if len(m) > 1 {
				out = append(out, strings.Join(m[1:], " "))
			} else {
				out = append(out, m[0])
			}
		}
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

func stateMachine(input ActionResult, params map[string]any) (ActionResult, error) {
	statesRaw, ok := params["states"]
	if !ok {
		return input, fmt.Errorf("state_machine: missing 'states' param")
	}
	statesMap, ok := statesRaw.(map[string]any)
	if !ok {
		return input, fmt.Errorf("state_machine: 'states' must be a map")
	}

	type stateConfig struct {
		until *regexp.Regexp
		keep  *regexp.Regexp
		next  string
	}

	states := make(map[string]stateConfig)
	for name, rawCfg := range statesMap {
		cfgMap, ok := rawCfg.(map[string]any)
		if !ok {
			continue
		}
		sc := stateConfig{}
		if u, ok := cfgMap["until"].(string); ok {
			sc.until, _ = regexp.Compile(u)
		}
		if k, ok := cfgMap["keep"].(string); ok {
			sc.keep, _ = regexp.Compile(k)
		}
		if n, ok := cfgMap["next"].(string); ok {
			sc.next = n
		}
		states[name] = sc
	}

	currentState := "start"
	if _, ok := states[currentState]; !ok {
		// Use alphabetically first state for determinism
		var names []string
		for name := range states {
			names = append(names, name)
		}
		sort.Strings(names)
		if len(names) > 0 {
			currentState = names[0]
		}
	}

	var out []string
	for _, line := range input.Lines {
		sc, ok := states[currentState]
		if !ok {
			break
		}
		// Check transition first — transition line is NOT kept unless explicitly matched by keep
		if sc.until != nil && sc.until.MatchString(line) {
			if sc.next != "" {
				currentState = sc.next
			}
			continue
		}
		// Apply keep filter
		if sc.keep == nil || sc.keep.MatchString(line) {
			out = append(out, line)
		}
	}

	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

func aggregate(input ActionResult, params map[string]any) (ActionResult, error) {
	patternsRaw, ok := params["patterns"]
	if !ok {
		return input, fmt.Errorf("aggregate: missing 'patterns' param")
	}
	patternsMap, ok := patternsRaw.(map[string]any)
	if !ok {
		return input, fmt.Errorf("aggregate: 'patterns' must be a map")
	}

	type patCount struct {
		name  string
		re    *regexp.Regexp
		count int
	}

	var patterns []patCount
	for name, rawPattern := range patternsMap {
		p, _ := rawPattern.(string)
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		patterns = append(patterns, patCount{name: name, re: re})
	}

	for _, line := range input.Lines {
		for i := range patterns {
			if patterns[i].re.MatchString(line) {
				patterns[i].count++
			}
		}
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].name < patterns[j].name
	})

	stats := make(map[string]int)
	var out []string
	fmtStr := getStr(params, "format")
	for _, p := range patterns {
		stats[p.name] = p.count
		if fmtStr == "" {
			out = append(out, fmt.Sprintf("%s: %d", p.name, p.count))
		}
	}

	if fmtStr != "" {
		tmpl, err := template.New("agg").Parse(fmtStr)
		if err != nil {
			return input, err
		}
		var buf strings.Builder
		if err := tmpl.Execute(&buf, stats); err != nil {
			return input, fmt.Errorf("aggregate template: %w", err)
		}
		out = strings.Split(buf.String(), "\n")
	}

	meta := copyMeta(input.Metadata)
	meta["stats"] = stats
	return ActionResult{Lines: out, Metadata: meta}, nil
}

func formatTemplate(input ActionResult, params map[string]any) (ActionResult, error) {
	tmplStr := getStr(params, "template")
	if tmplStr == "" {
		return input, fmt.Errorf("format_template: missing 'template' param")
	}

	tmpl, err := template.New("fmt").Parse(tmplStr)
	if err != nil {
		return input, fmt.Errorf("format_template: %w", err)
	}

	data := map[string]any{
		"lines":  strings.Join(input.Lines, "\n"),
		"count":  len(input.Lines),
		"groups": input.Metadata["groups"],
		"stats":  input.Metadata["stats"],
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return input, fmt.Errorf("format_template execute: %w", err)
	}

	result := buf.String()
	result = strings.TrimRight(result, "\n")
	lines := strings.Split(result, "\n")
	return ActionResult{Lines: lines, Metadata: input.Metadata}, nil
}

func compactPath(input ActionResult, params map[string]any) (ActionResult, error) {
	out := make([]string, len(input.Lines))
	for i, line := range input.Lines {
		out[i] = utils.CompactPath(line)
	}
	return ActionResult{Lines: out, Metadata: input.Metadata}, nil
}

// --- helpers ---

func copyMeta(m map[string]any) map[string]any {
	out := make(map[string]any)
	if m != nil {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

func toStringSlice(v any) ([]string, bool) {
	switch val := v.(type) {
	case []string:
		return val, true
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}
