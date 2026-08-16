package economics

import (
	"testing"

	"github.com/edouard-claude/snip/internal/config"
)

func TestActiveTiersDefaults(t *testing.T) {
	tiers := ActiveTiers(config.EconomicsConfig{})
	if len(tiers) != 4 {
		t.Fatalf("expected 4 default tiers, got %v", tiers)
	}
	want := map[string]float64{"Haiku": 1.00, "Sonnet": 3.00, "Opus": 5.00, "Fable": 10.00}
	for _, tier := range tiers {
		if want[tier.Name] != tier.PriceM {
			t.Errorf("%s: got %v, want %v", tier.Name, tier.PriceM, want[tier.Name])
		}
	}
}

func TestActiveTiersFromConfigSortedByPrice(t *testing.T) {
	tiers := ActiveTiers(config.EconomicsConfig{Tiers: map[string]float64{
		"premium": 9.99, "budget": 0.50, "standard": 2.00,
	}})
	if len(tiers) != 3 {
		t.Fatalf("expected 3 tiers, got %v", tiers)
	}
	if tiers[0].Name != "budget" || tiers[1].Name != "standard" || tiers[2].Name != "premium" {
		t.Errorf("expected price-ascending order, got %v", tiers)
	}
}

func TestFindTierCaseInsensitive(t *testing.T) {
	tiers := ActiveTiers(config.EconomicsConfig{Tiers: map[string]float64{"Negotiated": 2.75}})
	if tier := FindTier(tiers, "negotiated"); tier == nil || tier.PriceM != 2.75 {
		t.Errorf("FindTier(negotiated): got %v", tier)
	}
	if tier := FindTier(tiers, "nope"); tier != nil {
		t.Errorf("FindTier(nope): got %v, want nil", tier)
	}
}
