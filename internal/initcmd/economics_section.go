package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/edouard-claude/snip/internal/config"
)

// anthropicAgents are the agents whose provider is Anthropic, so the
// economics template can carry real list prices. Every other agent gets a
// generic template rather than invented provider rates.
var anthropicAgents = map[string]bool{
	"claude-code": true,
}

const anthropicEconomicsTemplate = `
[economics.tiers]
# Prices in $ per 1M input tokens, used by ` + "`snip cc-economics`" + `.
# Anthropic list prices as of 2026-08; uncomment and adjust to your
# negotiated rates. Names are free-form.
# haiku = 1.00
# sonnet = 3.00
# opus = 5.00
# fable = 10.00
`

const genericEconomicsTemplate = `
[economics.tiers]
# Prices in $ per 1M input tokens, used by ` + "`snip cc-economics`" + `.
# Rates vary by provider and contract: uncomment and adjust to the models
# you actually use. Names are free-form.
# budget = 1.00
# standard = 3.00
# premium = 5.00
`

// ensureEconomicsSection appends a commented [economics.tiers] template to
// the user config so cc-economics pricing is discoverable and adjustable.
// Existing content is preserved verbatim; a config that already has an
// economics section is left untouched.
func ensureEconomicsSection(agent string) error {
	path := config.Path()

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if strings.Contains(string(data), "[economics") {
			return nil
		}
	case os.IsNotExist(err):
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
			return fmt.Errorf("create config dir: %w", mkErr)
		}
	default:
		return fmt.Errorf("read config: %w", err)
	}

	tmpl := genericEconomicsTemplate
	if anthropicAgents[agent] {
		tmpl = anthropicEconomicsTemplate
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	defer func() { _ = f.Close() }()

	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		tmpl = "\n" + tmpl
	}
	if _, err := f.WriteString(tmpl); err != nil {
		return fmt.Errorf("append economics section: %w", err)
	}
	return nil
}
