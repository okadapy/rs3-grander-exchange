package service

import (
	"math"

	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/rates"
)

// Market holds the trading assumptions applied on top of raw Grand
// Exchange guide prices.
//
// None of this is scraped fact. The exchange publishes one number per
// item — a guide price — and every figure this service reports is that
// number plus a set of modelling choices. Keeping those choices in one
// named struct means they can be echoed back in the API response, so a
// GP/h number never travels without the assumptions that produced it.
type Market struct {
	// SpreadPct is the assumed round-trip gap between what you pay to
	// buy and what you receive to sell, as a percentage of the guide
	// price. Buys are modelled at price*(1+spread/2), sells at
	// price*(1-spread/2). Zero means "assume you transact exactly at
	// the guide price", which is optimistic to the point of fiction.
	SpreadPct float64

	// TaxPct is charged on the whole sale value, TaxCapPerItem caps the
	// charge per item sold, and TaxExemptBelow skips it entirely for
	// items priced under the threshold.
	TaxPct         float64
	TaxCapPerItem  int64
	TaxExemptBelow int64

	DefaultActionsPerHour int

	// InventorySlots and BankTripTicks model the trip to the bank when
	// the inventory runs out, feeding rates.ActionsPerHour.
	InventorySlots int
	BankTripTicks  int
}

// PriceBasis names the source of the underlying number, so the frontend
// can label a chart honestly instead of implying live bid/ask.
const PriceBasis = "ge_guide_price"

func MarketFrom(cfg config.MarketConf) Market {
	m := Market{
		SpreadPct:             cfg.SpreadPct,
		TaxPct:                cfg.TaxPct,
		TaxCapPerItem:         cfg.TaxCapPerItem,
		TaxExemptBelow:        cfg.TaxExemptBelow,
		DefaultActionsPerHour: cfg.DefaultActionsPerHour,
		InventorySlots:        cfg.InventorySlots,
		BankTripTicks:         cfg.BankTripTicks,
	}
	if m.DefaultActionsPerHour <= 0 {
		m.DefaultActionsPerHour = 600
	}
	if m.SpreadPct < 0 {
		m.SpreadPct = 0
	}
	// InventorySlots <= 0 means the config was never set (there is no
	// legitimate zero-slot inventory); BankTripTicks < 0 is the same
	// signal for that field alone. Zero BankTripTicks is left alone —
	// it validly means "no bank trip overhead" — matching the guard
	// rates.ActionsPerHour itself applies to a Config.
	def := rates.DefaultConfig()
	if m.InventorySlots <= 0 {
		m.InventorySlots = def.InventorySlots
	}
	if m.BankTripTicks < 0 {
		m.BankTripTicks = def.BankTripTicks
	}
	return m
}

// RatesConfig is the banking model the throughput calculation uses.
func (m Market) RatesConfig() rates.Config {
	return rates.Config{
		InventorySlots: m.InventorySlots,
		BankTripTicks:  m.BankTripTicks,
	}
}

// BuyPrice is what we assume one unit actually costs to acquire.
// Rounded up: paying a fraction of a GP more is the conservative error.
func (m Market) BuyPrice(guide int64) float64 {
	if guide <= 0 {
		return 0
	}
	return math.Ceil(float64(guide) * (1 + m.SpreadPct/200))
}

// SellPrice is what we assume one unit actually fetches, before tax.
// Rounded down, and never below zero even for an absurd spread.
func (m Market) SellPrice(guide int64) float64 {
	if guide <= 0 {
		return 0
	}
	v := math.Floor(float64(guide) * (1 - m.SpreadPct/200))
	if v < 0 {
		return 0
	}
	return v
}

// Tax is the Grand Exchange charge on selling `qty` units at
// `unitPrice` each. The percentage applies to the whole sale value; the
// cap applies per item sold, so a stack of expensive items is capped
// proportionally rather than as a single lump.
func (m Market) Tax(unitPrice float64, qty int) float64 {
	if qty <= 0 || unitPrice <= 0 || m.TaxPct <= 0 {
		return 0
	}
	if m.TaxExemptBelow > 0 && unitPrice < float64(m.TaxExemptBelow) {
		return 0
	}
	total := unitPrice * float64(qty)
	tax := math.Floor(total * m.TaxPct / 100)

	if m.TaxCapPerItem > 0 {
		if cap := float64(m.TaxCapPerItem) * float64(qty); tax > cap {
			return cap
		}
	}
	return tax
}
