package rates

import (
	"fmt"
	"strings"
)

// Forging is not a fixed tick cost — that is why the recipe infobox says
// "varies". The player fills a progress meter by striking an anvil, and
// each strike is worth more when the item is hotter.
//
// From the wiki's Smithing page: a strike lands every 2 ticks, costs 10
// heat and adds base progress scaled by the heat tier; maximum heat is
// 300 + 3*Smithing + 3*Firemaking; reheating from empty costs 6 ticks,
// dropping at per-metal level milestones.
const (
	strikeTicks     = 2
	heatPerStrike   = 10
	baseProgress    = 10
	reheatTicksSlow = 6
	reheatTicksMid  = 4
	reheatTicksFast = 2
)

// forgeProgressPerBar is the progress one bar of each metal demands.
var forgeProgressPerBar = map[string]int{
	"Bronze":     100,
	"Iron":       200,
	"Steel":      300,
	"Mithril":    400,
	"Adamant":    500,
	"Rune":       600,
	"Orikalkum":  700,
	"Necronium":  800,
	"Bane":       900,
	"Elder rune": 1000,
	"Primal":     1100,
}

// reheatMilestones is the Smithing level at which a full reheat drops
// from 6 ticks to 4, and then to 2. Confirmed against the raw wikitext
// dump of the Smithing page (columns "Level for faster heating" and
// "Level for fastest heating") — see Task 2 Step 1. The table also
// carries a "Level for creation w/ full heat" column between those two;
// that is a separate mechanic and is not part of the reheat ladder, so
// it is deliberately not used here.
var reheatMilestones = map[string][2]int{
	"Bronze":     {3, 7},
	"Iron":       {12, 18},
	"Steel":      {22, 26},
	"Mithril":    {35, 39},
	"Adamant":    {42, 44},
	"Rune":       {54, 58},
	"Orikalkum":  {67, 69},
	"Necronium":  {73, 78},
	"Bane":       {82, 85},
	"Elder rune": {93, 96},
	"Primal":     {104, 107},
}

// Boosts is the stack the player declares. None of it is visible in a
// hiscore lookup, so it is an input rather than an assumption; the empty
// value is the conservative floor.
type Boosts struct {
	// BaseProgressBonus is added to the base 10 progress per strike.
	BaseProgressBonus int
	// DoubleProgressPct is the summed chance of a double-progress
	// strike, applied as a multiplier.
	DoubleProgressPct float64
}

type boostToken struct {
	base   int
	double float64
}

var boostTokens = map[string]boostToken{
	"smithing_cape":  {base: 5},
	"careless5":      {base: 5},
	"luminite":       {base: 1},
	"rapid4":         {double: 20},
	"tinker2":        {double: 4},
	"juju":           {double: 5},
	"varrock4":       {double: 2},
	"crystal_hammer": {double: 1},
}

// equipment20 raises two perks to their equipment-level-20 values rather
// than contributing on its own.
const equipmentToken = "equipment20"

var equipmentUpgrade = map[string]float64{
	"rapid4":  22,
	"tinker2": 4.4,
}

// ParseBoosts reads the comma-separated checkbox set. An unknown token is
// an error, not a no-op: a mistyped checkbox producing a plausible
// number is the worst outcome available here.
func ParseBoosts(csv string) (Boosts, error) {
	var b Boosts
	if strings.TrimSpace(csv) == "" {
		return b, nil
	}

	seen := map[string]bool{}
	for _, raw := range strings.Split(csv, ",") {
		token := strings.ToLower(strings.TrimSpace(raw))
		if token == "" {
			continue
		}
		if token != equipmentToken {
			if _, ok := boostTokens[token]; !ok {
				return Boosts{}, fmt.Errorf("unknown boost %q", token)
			}
		}
		seen[token] = true
	}

	equipped := seen[equipmentToken]
	for token := range seen {
		if token == equipmentToken {
			continue
		}
		t := boostTokens[token]
		b.BaseProgressBonus += t.base
		if equipped {
			if upgraded, ok := equipmentUpgrade[token]; ok {
				b.DoubleProgressPct += upgraded
				continue
			}
		}
		b.DoubleProgressPct += t.double
	}
	return b, nil
}

// ForgeTicks simulates forging one item and returns the ticks it takes.
//
// The simulation starts at full heat, strikes until the progress meter
// is full, and reheats whenever heat runs out. It models no perk, potion
// or piece of gear beyond what the caller declared, so the result is a
// floor rather than a ceiling.
func ForgeTicks(bars int, metal string, smithing, firemaking int, b Boosts) (int, bool) {
	perBar, ok := forgeProgressPerBar[metal]
	if !ok || bars <= 0 {
		return 0, false
	}
	thresholds, ok := smeltThresholds[metal]
	if !ok || smithing < thresholds[0] {
		return 0, false
	}

	required := bars * perBar
	maxHeat := 300 + 3*smithing + 3*firemaking

	base := baseProgress + b.BaseProgressBonus
	if smithing >= 99 {
		base++
	}
	multiplier := 1 + b.DoubleProgressPct/100

	reheat := reheatTicksSlow
	if m, ok := reheatMilestones[metal]; ok {
		switch {
		case smithing >= m[1]:
			reheat = reheatTicksFast
		case smithing >= m[0]:
			reheat = reheatTicksMid
		}
	}

	ticks, progress, heat := 0, 0, maxHeat
	for progress < required {
		if heat <= 0 {
			ticks += reheat
			heat = maxHeat
		}
		ticks += strikeTicks
		gain := int(float64(base) * heatMultiplier(heat, maxHeat) * multiplier)
		// ParseBoosts never produces a Boosts that drives gain non-positive,
		// but Boosts is exported and a caller can construct one directly
		// (e.g. with a negative BaseProgressBonus or DoubleProgressPct).
		// This floor guarantees progress strictly increases every
		// iteration, so the loop terminates in at most `required`
		// iterations for any Boosts, however it was built.
		if gain < 1 {
			gain = 1
		}
		progress += gain
		heat -= heatPerStrike
	}
	return ticks, true
}

// heatMultiplier is the progress multiplier for the current heat, by
// thirds of the maximum.
func heatMultiplier(heat, maxHeat int) float64 {
	if heat <= 0 || maxHeat <= 0 {
		return 1
	}
	switch share := float64(heat) / float64(maxHeat); {
	case share > 2.0/3.0:
		return 2
	case share > 1.0/3.0:
		return 1.6
	default:
		return 1.3
	}
}
