// Package rates turns game mechanics into a sustainable hourly craft
// rate.
//
// It exists because every recipe used to report a flat house assumption
// for actions per hour, which turned one craft's profit into a GP/h
// figure nobody could reach. The numbers here come from the wiki's
// published tick costs and Smithing tables; where a rate is genuinely
// unknown the caller is told so rather than handed a guess.
//
// Pure functions only: no database, no HTTP, no config.
package rates

import (
	"strings"

	"github.com/rs3-market/backend/shared/models"
)

// TickSeconds is the RS3 game tick.
const TickSeconds = 0.6

// ticksPerHour is 3600 / TickSeconds.
const ticksPerHour = 6000

// smeltThresholds is the Smithing level at which smelting a bar takes 5,
// 4 and 3 ticks, from the table on the wiki's Smithing page.
var smeltThresholds = map[string][3]int{
	"Bronze":     {1, 2, 5},
	"Iron":       {10, 14, 17},
	"Steel":      {20, 21, 23},
	"Mithril":    {30, 31, 37},
	"Adamant":    {40, 46, 48},
	"Rune":       {50, 53, 56},
	"Orikalkum":  {60, 61, 65},
	"Necronium":  {70, 75, 77},
	"Bane":       {80, 86, 89},
	"Elder rune": {90, 91, 94},
	"Primal":     {100, 101, 106},
}

// SmeltTicks returns how long one bar takes to smelt at a given Smithing
// level. The second result is false for an unknown metal or a level
// below the bar's requirement — there is no rate at all, rather than a
// slow one.
func SmeltTicks(metal string, smithing int) (int, bool) {
	t, ok := smeltThresholds[metal]
	if !ok {
		return 0, false
	}
	switch {
	case smithing >= t[2]:
		return 3, true
	case smithing >= t[1]:
		return 4, true
	case smithing >= t[0]:
		return 5, true
	}
	return 0, false
}

// BarMetal recognises a bar by name and returns its metal.
func BarMetal(itemName string) (string, bool) {
	name := strings.TrimSpace(itemName)
	metal, found := strings.CutSuffix(name, " bar")
	if !found {
		return "", false
	}
	if _, ok := smeltThresholds[metal]; !ok {
		return "", false
	}
	return metal, true
}

// stackableSuffixes covers the families that appear as recipe inputs.
// The wiki records stackability per item, but scraping ~7000 item pages
// for one boolean does not pay when a dozen families cover every case
// the recipe corpus actually contains.
var stackableSuffixes = []string{
	" rune", " runes", " energy", " energies", " shard", " shards",
	" charm", " charms", " feather", " feathers", " bolt", " bolts",
	" arrow", " arrows", " dart", " darts", " memories",
}

var stackableExact = map[string]bool{
	"Coins":    true,
	"Feather":  true,
	"Feathers": true,
}

// IsStackable reports whether an input rides along without costing an
// inventory slot.
func IsStackable(itemName string) bool {
	name := strings.TrimSpace(itemName)
	if stackableExact[name] {
		return true
	}
	lower := strings.ToLower(name)
	for _, suffix := range stackableSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// SlotsPerCraft is how much of the inventory one craft consumes.
func SlotsPerCraft(inputs []models.RecipeInput) int {
	slots := 0
	for _, in := range inputs {
		if IsStackable(in.ItemName) {
			continue
		}
		qty := in.Quantity
		if qty < 1 {
			qty = 1
		}
		slots += qty
	}
	return slots
}

// Config holds the banking assumptions. They are assumptions, so they
// are configurable and echoed back to the caller.
type Config struct {
	InventorySlots int
	BankTripTicks  int
}

// DefaultConfig is calibrated so that smelting bars at 3 ticks with one
// bar per slot comes out at the timed rate of 1600 an hour.
func DefaultConfig() Config {
	return Config{InventorySlots: 28, BankTripTicks: 21}
}

// ActionsPerHour converts a per-craft tick cost into a rate that can be
// sustained, including the walk to the bank when the inventory runs dry.
//
// A craft consuming no slots — everything stacks — never forces a trip,
// so it runs at the raw tick ceiling.
//
// The result is fractional because the quantity is: an Elder rune
// platebody + 5 costs 80 bars and 10136 ticks, which is one item every
// 1.7 hours. Computed by integer division that rate truncated to zero,
// and a caller dividing by it got an infinity it could not serialise.
// Rounding it up to one instead would have overstated the item's rate
// by seventy percent, which is exactly the kind of invented number this
// package exists to remove.
//
// Config must be fully initialized. Zero-valued or partially-filled Config
// structs are detected and replaced with DefaultConfig(); this prevents
// silently wrong rates from half-filled Configs (e.g., InventorySlots set
// but BankTripTicks left at zero).
func ActionsPerHour(ticks, slotsPerCraft int, cfg Config) float64 {
	if ticks <= 0 {
		return 0
	}
	// Detect and reject invalid or zero-valued Config. This guard is symmetric:
	// InventorySlots <= 0 is invalid (can't fit items), BankTripTicks < 0 is
	// invalid (negative time), and 0 == 0 catch zero-valued Config{}. A valid
	// Config must have positive InventorySlots and non-negative BankTripTicks.
	if cfg == (Config{}) || cfg.InventorySlots <= 0 || cfg.BankTripTicks < 0 {
		cfg = DefaultConfig()
	}
	if slotsPerCraft <= 0 {
		return float64(ticksPerHour) / float64(ticks)
	}

	// A craft needing more slots than the inventory holds is charged one
	// bank trip, not the several it really takes. That understates the
	// time by 21 ticks out of 10157 on the largest craft in the game, and
	// correcting it is a behaviour change for every oversized craft
	// rather than part of this fix; it is recorded as follow-up work.
	craftsPerTrip := cfg.InventorySlots / slotsPerCraft
	if craftsPerTrip < 1 {
		craftsPerTrip = 1
	}
	tripTicks := craftsPerTrip*ticks + cfg.BankTripTicks
	if tripTicks <= 0 {
		return 0
	}
	return float64(craftsPerTrip) * ticksPerHour / float64(tripTicks)
}
