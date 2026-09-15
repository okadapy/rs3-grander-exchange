package service

import (
	"math"
	"testing"

	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/rates"
)

func approx(t *testing.T, got, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestMarketFromAppliesDefaults(t *testing.T) {
	m := MarketFrom(config.MarketConf{})
	if m.DefaultActionsPerHour != 600 {
		t.Errorf("default aph = %d, want 600", m.DefaultActionsPerHour)
	}

	m = MarketFrom(config.MarketConf{SpreadPct: -5, DefaultActionsPerHour: -1})
	if m.SpreadPct != 0 {
		t.Errorf("negative spread should clamp to 0, got %v", m.SpreadPct)
	}
	if m.DefaultActionsPerHour != 600 {
		t.Errorf("non-positive aph should fall back to 600, got %d", m.DefaultActionsPerHour)
	}
}

// An unset banking model falls back to the same figures rates.DefaultConfig
// is calibrated against, so an unconfigured deployment still produces the
// timed 1600/h for smelting rather than a config-shaped zero.
func TestMarketFromDefaultsTheBankingModel(t *testing.T) {
	m := MarketFrom(config.MarketConf{})
	def := rates.DefaultConfig()
	if m.InventorySlots != def.InventorySlots {
		t.Errorf("inventory slots = %d, want %d", m.InventorySlots, def.InventorySlots)
	}
}

func TestMarketFromKeepsAConfiguredBankingModel(t *testing.T) {
	m := MarketFrom(config.MarketConf{InventorySlots: 4, BankTripTicks: 10})
	if m.InventorySlots != 4 || m.BankTripTicks != 10 {
		t.Errorf("banking model = %+v, want inventory 4 / bank trip 10", m)
	}
}

func TestRatesConfigCarriesTheBankingModel(t *testing.T) {
	m := MarketFrom(config.MarketConf{InventorySlots: 12, BankTripTicks: 7})
	cfg := m.RatesConfig()
	if cfg.InventorySlots != 12 || cfg.BankTripTicks != 7 {
		t.Errorf("RatesConfig = %+v, want inventory 12 / bank trip 7", cfg)
	}
}

// The spread is the whole reason this service does not pretend the guide
// price is a price you can transact at. Buying must cost more than the
// guide and selling must fetch less, or the model is back to claiming a
// free round trip.
func TestSpreadWidensBuyAndNarrowsSell(t *testing.T) {
	m := Market{SpreadPct: 2}

	buy := m.BuyPrice(1000)
	sell := m.SellPrice(1000)

	approx(t, buy, 1010, "buy price")
	approx(t, sell, 990, "sell price")

	if buy <= sell {
		t.Errorf("buy (%v) must exceed sell (%v) under a positive spread", buy, sell)
	}
}

func TestZeroSpreadTransactsAtGuidePrice(t *testing.T) {
	m := Market{SpreadPct: 0}
	approx(t, m.BuyPrice(1000), 1000, "buy price")
	approx(t, m.SellPrice(1000), 1000, "sell price")
}

// Rounding is deliberately pessimistic in both directions: pay a little
// more, receive a little less. An optimistic rounding error compounds
// across every step of a deep recipe tree.
func TestSpreadRoundingIsConservative(t *testing.T) {
	m := Market{SpreadPct: 3}
	approx(t, m.BuyPrice(101), 103, "buy price rounds up")    // 101 * 1.015 = 102.515
	approx(t, m.SellPrice(101), 99, "sell price rounds down") // 101 * 0.985 = 99.485
}

func TestSpreadNeverReturnsNegative(t *testing.T) {
	m := Market{SpreadPct: 100}
	if got := m.SellPrice(10); got < 0 {
		t.Errorf("sell price = %v, must never be negative", got)
	}
}

func TestPriceOfNonPositiveGuideIsZero(t *testing.T) {
	m := Market{SpreadPct: 2}
	approx(t, m.BuyPrice(0), 0, "buy price of 0")
	approx(t, m.SellPrice(-5), 0, "sell price of negative")
}

func TestTaxIsChargedOnTheWholeSale(t *testing.T) {
	m := Market{TaxPct: 2, TaxCapPerItem: 5_000_000}

	// 100 units at 1,000 GP = 100,000 GP of sales, taxed at 2%.
	approx(t, m.Tax(1000, 100), 2000, "tax")
}

func TestTaxCapAppliesPerItem(t *testing.T) {
	m := Market{TaxPct: 2, TaxCapPerItem: 5_000_000}

	// 2% of 1bn would be 20m, but the cap is 5m per item.
	approx(t, m.Tax(1_000_000_000, 1), 5_000_000, "capped tax on one item")

	// The cap scales with quantity rather than acting as a single lump.
	approx(t, m.Tax(1_000_000_000, 3), 15_000_000, "capped tax on three items")
}

func TestTaxBelowCapIsUncapped(t *testing.T) {
	m := Market{TaxPct: 2, TaxCapPerItem: 5_000_000}
	approx(t, m.Tax(1_000_000, 2), 40_000, "tax under the cap")
}

func TestTaxExemption(t *testing.T) {
	m := Market{TaxPct: 2, TaxCapPerItem: 5_000_000, TaxExemptBelow: 100}
	approx(t, m.Tax(99, 1000), 0, "tax below the exemption threshold")
	approx(t, m.Tax(100, 1000), 2000, "tax at the exemption threshold")
}

func TestTaxEdgeCases(t *testing.T) {
	m := Market{TaxPct: 2, TaxCapPerItem: 5_000_000}
	approx(t, m.Tax(1000, 0), 0, "zero quantity")
	approx(t, m.Tax(0, 10), 0, "zero price")
	approx(t, m.Tax(-1, 10), 0, "negative price")

	free := Market{TaxPct: 0}
	approx(t, free.Tax(1000, 100), 0, "zero tax rate")
}

func TestTaxRoundsDown(t *testing.T) {
	m := Market{TaxPct: 2, TaxCapPerItem: 5_000_000}
	// 2% of 1,010 is 20.2.
	approx(t, m.Tax(101, 10), 20, "tax rounds down")
}
