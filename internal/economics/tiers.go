package economics

import (
	"sort"
	"strings"

	"github.com/edouard-claude/snip/internal/config"
)

// defaultTiers are current Anthropic list prices, $ per 1M input tokens
// (as of 2026-08). Overridable via [economics.tiers] in config.toml for
// negotiated or non-Anthropic rates.
var defaultTiers = []Tier{
	{Name: "Haiku", PriceM: 1.00},
	{Name: "Sonnet", PriceM: 3.00},
	{Name: "Opus", PriceM: 5.00},
	{Name: "Fable", PriceM: 10.00},
}

// ActiveTiers returns the pricing tiers to report on: the [economics.tiers]
// entries from the config when present (sorted by ascending price), the
// built-in defaults otherwise.
func ActiveTiers(cfg config.EconomicsConfig) []Tier {
	if len(cfg.Tiers) == 0 {
		return defaultTiers
	}
	tiers := make([]Tier, 0, len(cfg.Tiers))
	for name, price := range cfg.Tiers {
		tiers = append(tiers, Tier{Name: name, PriceM: price})
	}
	sort.Slice(tiers, func(i, j int) bool {
		if tiers[i].PriceM != tiers[j].PriceM {
			return tiers[i].PriceM < tiers[j].PriceM
		}
		return tiers[i].Name < tiers[j].Name
	})
	return tiers
}

// FindTier returns the tier matching name (case-insensitive), or nil.
func FindTier(tiers []Tier, name string) *Tier {
	for i := range tiers {
		if strings.EqualFold(tiers[i].Name, name) {
			return &tiers[i]
		}
	}
	return nil
}

// TierNames returns the lowercase tier names, for error messages.
func TierNames(tiers []Tier) []string {
	names := make([]string, len(tiers))
	for i, t := range tiers {
		names[i] = strings.ToLower(t.Name)
	}
	return names
}
