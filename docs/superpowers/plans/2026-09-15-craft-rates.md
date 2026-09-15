# Tick-Based Craft Rates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the house-default actions-per-hour with a rate derived from game mechanics, and expose XP/h, GP/h and GP/XP rankings through one route.

**Architecture:** A new dependency-free package `shared/rates` holds the formulas and the wiki's Smithing tables. `recipe-service` scrapes two more infobox fields into the recipe row. `calc-service` calls `rates` from the single existing `chooseAPH` seam, passing player levels it already fetches, and a new `/calc/top` route ranks the result over every priceable recipe behind the existing cache.

**Tech Stack:** Go 1.22, Gin, GORM, MySQL 8, Redis, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-09-15-craft-rate-design.md`

## Global Constraints

- An RS3 tick is 0.6 seconds; 6000 ticks per hour.
- Banking model defaults: `InventorySlots` 28, `BankTripTicks` 21. These
  two values must make `ActionsPerHour(ticks=3, slots=1)` return exactly
  1600 — an independently timed rate. Any change that breaks that number
  is wrong.
- `shared/rates` imports nothing but the standard library and
  `shared/models`. No database, no HTTP, no config package.
- Tests are plain `testing`, table-free unless the case list is long, and
  each test names the behaviour it protects. Match the style in
  `shared/middleware/cors_test.go`.
- Every command runs inside Docker except `go build`/`go test`, which run
  on the host: `docker compose exec -T mysql mysql -uroot -proot < file`.
- Commit messages: imperative subject, no task numbers, no AI attribution.
- Run `gofmt -l .` before every commit; it must print nothing.

---

### Task 1: `shared/rates` — tables, stackables and the banking formula

**Files:**
- Create: `shared/rates/rates.go`
- Create: `shared/rates/rates_test.go`

**Interfaces:**
- Consumes: `models.RecipeInput` from `shared/models` (fields `ItemName string`, `Quantity int`).
- Produces: `TickSeconds`, `SmeltTicks(metal string, smithing int) (int, bool)`, `BarMetal(itemName string) (string, bool)`, `IsStackable(itemName string) bool`, `SlotsPerCraft(inputs []models.RecipeInput) int`, `Config{InventorySlots, BankTripTicks int}`, `DefaultConfig() Config`, `ActionsPerHour(ticks, slotsPerCraft int, cfg Config) int`.

- [ ] **Step 1: Write the failing tests**

Create `shared/rates/rates_test.go`:

```go
package rates

import (
	"testing"

	"github.com/rs3-market/backend/shared/models"
)

// The wiki gives three thresholds per bar: the level it can be smelted
// at (5 ticks), and the levels where it drops to 4 and then 3.
func TestSmeltTicksAtSteelThresholds(t *testing.T) {
	cases := map[int]int{20: 5, 21: 4, 22: 4, 23: 3, 99: 3}
	for level, want := range cases {
		got, ok := SmeltTicks("Steel", level)
		if !ok {
			t.Errorf("SmeltTicks(Steel, %d) not ok", level)
			continue
		}
		if got != want {
			t.Errorf("SmeltTicks(Steel, %d) = %d, want %d", level, got, want)
		}
	}
}

// Below the working level there is no rate, not a slow one.
func TestSmeltTicksBelowWorkingLevel(t *testing.T) {
	if _, ok := SmeltTicks("Steel", 19); ok {
		t.Error("SmeltTicks(Steel, 19) reported a rate below the level requirement")
	}
}

func TestSmeltTicksUnknownMetal(t *testing.T) {
	if _, ok := SmeltTicks("Cheese", 99); ok {
		t.Error("SmeltTicks accepted a metal that is not in the table")
	}
}

func TestBarMetalRecognisesBars(t *testing.T) {
	if m, ok := BarMetal("Rune bar"); !ok || m != "Rune" {
		t.Errorf("BarMetal(Rune bar) = %q, %v; want Rune, true", m, ok)
	}
	if m, ok := BarMetal("Elder rune bar"); !ok || m != "Elder rune" {
		t.Errorf("BarMetal(Elder rune bar) = %q, %v; want Elder rune, true", m, ok)
	}
}

// "Rune bar" is a bar; "Rune platebody" and "Bar" are not.
func TestBarMetalRejectsNonBars(t *testing.T) {
	for _, name := range []string{"Rune platebody", "Bar", "Bronze", "Oak plank"} {
		if _, ok := BarMetal(name); ok {
			t.Errorf("BarMetal(%q) reported a bar", name)
		}
	}
}

func TestIsStackable(t *testing.T) {
	stack := []string{"Fire rune", "Law rune", "Coins", "Spirit shards",
		"Divine energy", "Gold charm", "Feather", "Adamant bolts", "Rune arrow"}
	for _, name := range stack {
		if !IsStackable(name) {
			t.Errorf("IsStackable(%q) = false, want true", name)
		}
	}
	solid := []string{"Rune bar", "Soft clay", "Yew logs", "Steel platebody"}
	for _, name := range solid {
		if IsStackable(name) {
			t.Errorf("IsStackable(%q) = true, want false", name)
		}
	}
}

// Stackables ride along without costing a slot, so a tab made from one
// clay and two runes is limited by the clay alone.
func TestSlotsPerCraftCountsOnlyNonStackables(t *testing.T) {
	inputs := []models.RecipeInput{
		{ItemName: "Soft clay", Quantity: 1},
		{ItemName: "Air rune", Quantity: 2},
		{ItemName: "Law rune", Quantity: 1},
	}
	if got := SlotsPerCraft(inputs); got != 1 {
		t.Errorf("SlotsPerCraft = %d, want 1", got)
	}
}

func TestSlotsPerCraftSumsQuantities(t *testing.T) {
	inputs := []models.RecipeInput{{ItemName: "Rune bar", Quantity: 5}}
	if got := SlotsPerCraft(inputs); got != 5 {
		t.Errorf("SlotsPerCraft = %d, want 5", got)
	}
}

// The calibration point: a full inventory of bars at 3 ticks each, with
// one trip to the bank, is a timed 1600 an hour.
func TestActionsPerHourMatchesTheTimedSmeltingRate(t *testing.T) {
	if got := ActionsPerHour(3, 1, DefaultConfig()); got != 1600 {
		t.Errorf("ActionsPerHour(3, 1) = %d, want 1600", got)
	}
}

// With nothing to carry there is no trip to the bank, so the rate is the
// raw tick ceiling.
func TestActionsPerHourAllStackableInputsHitTheCeiling(t *testing.T) {
	if got := ActionsPerHour(3, 0, DefaultConfig()); got != 2000 {
		t.Errorf("ActionsPerHour(3, 0) = %d, want 2000", got)
	}
}

// Five bars per craft means five crafts an inventory, so the bank trip is
// amortised over far fewer actions and the rate drops well under the
// single-slot case.
func TestActionsPerHourFallsWhenACraftEatsMoreSlots(t *testing.T) {
	got := ActionsPerHour(3, 5, DefaultConfig())
	if got >= 1600 {
		t.Errorf("ActionsPerHour(3, 5) = %d, want below the single-slot 1600", got)
	}
	if got <= 0 {
		t.Errorf("ActionsPerHour(3, 5) = %d, want a positive rate", got)
	}
}

func TestActionsPerHourRejectsNonPositiveTicks(t *testing.T) {
	if got := ActionsPerHour(0, 1, DefaultConfig()); got != 0 {
		t.Errorf("ActionsPerHour(0, 1) = %d, want 0", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./shared/rates/`
Expected: FAIL to build, `undefined: SmeltTicks`.

- [ ] **Step 3: Write the implementation**

Create `shared/rates/rates.go`:

```go
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
func ActionsPerHour(ticks, slotsPerCraft int, cfg Config) int {
	if ticks <= 0 {
		return 0
	}
	if cfg.InventorySlots <= 0 {
		cfg.InventorySlots = 28
	}
	if slotsPerCraft <= 0 {
		return ticksPerHour / ticks
	}

	craftsPerTrip := cfg.InventorySlots / slotsPerCraft
	if craftsPerTrip < 1 {
		craftsPerTrip = 1
	}
	tripTicks := craftsPerTrip*ticks + cfg.BankTripTicks
	return craftsPerTrip * ticksPerHour / tripTicks
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./shared/rates/ -v`
Expected: PASS, every test. In particular
`TestActionsPerHourMatchesTheTimedSmeltingRate` must show 1600 exactly.

- [ ] **Step 5: Check formatting and commit**

```bash
gofmt -l .
go test ./...
git add shared/rates
git commit -m "Add rates package for tick-derived craft throughput

Actions per hour was a flat house assumption on every recipe, so a GP/h
column reported one craft's profit multiplied by 600. This package
derives the rate instead: the wiki's per-bar smelting ticks by Smithing
level, and a banking model that charges a trip to the bank whenever the
inventory runs out.

Calibrated against a timed rate: smelting bars at 3 ticks, one bar to a
slot, comes out at exactly 1600 an hour. Stackable inputs cost no slot,
so a recipe whose inputs all stack runs at the raw tick ceiling."
```

---

### Task 2: `shared/rates` — forging simulation and boosts

**Files:**
- Create: `shared/rates/forge.go`
- Create: `shared/rates/forge_test.go`
- Modify: `docs/superpowers/specs/2026-09-15-craft-rate-design.md` only if Step 1 finds the reheat table mis-mapped.

**Interfaces:**
- Consumes: `smeltThresholds` keys from Task 1 (metal names), `BarMetal`.
- Produces: `Boosts` struct, `ParseBoosts(csv string) (Boosts, error)`, `ForgeTicks(bars int, metal string, smithing, firemaking int, b Boosts) (int, bool)`.

- [ ] **Step 1: Verify the reheat milestone columns before coding**

The raw wikitext columns are ambiguous: parsed naively, Steel reads
`faster heating = 22`, `creation with full heat = 28`, `fastest heating =
26`, and a "fastest" threshold below the "full heat" one is suspicious.

Run this and compare against the rendered table at
<https://runescape.wiki/w/Smithing>:

```bash
curl -s "http://localhost:8082/internal/dump?page=Smithing" \
  | grep -A 6 "!Level for faster heating"
```

Write the confirmed mapping into `reheatMilestones` below. If the
mapping differs from what this plan assumes, correct the constants here
and note it in the spec. Do not guess.

- [ ] **Step 2: Write the failing tests**

Create `shared/rates/forge_test.go`:

```go
package rates

import "testing"

// Progress required is the bar count times a per-metal constant. Checked
// against three wiki pages: rune platebody 5 bars x 600 = 3000, rune
// dagger 2 x 600 = 1200, steel platebody 5 x 300 = 1500.
func TestForgeProgressPerBar(t *testing.T) {
	cases := map[string]int{"Bronze": 100, "Steel": 300, "Rune": 600, "Primal": 1100}
	for metal, want := range cases {
		if got := forgeProgressPerBar[metal]; got != want {
			t.Errorf("forgeProgressPerBar[%s] = %d, want %d", metal, got, want)
		}
	}
}

func TestForgeTicksUnknownMetal(t *testing.T) {
	if _, ok := ForgeTicks(5, "Cheese", 99, 99, Boosts{}); ok {
		t.Error("ForgeTicks accepted a metal that is not in the table")
	}
}

func TestForgeTicksBelowWorkingLevel(t *testing.T) {
	if _, ok := ForgeTicks(5, "Rune", 49, 99, Boosts{}); ok {
		t.Error("ForgeTicks reported a rate below the level requirement")
	}
}

// A strike is 2 ticks and lands 20 progress at high heat, so 1500
// progress is at least 75 strikes, and reheats only add ticks. The point
// is that the simulation lands in the right order of magnitude rather
// than returning something arbitrary.
func TestForgeTicksSteelPlatebodyIsPlausible(t *testing.T) {
	got, ok := ForgeTicks(5, "Steel", 99, 99, Boosts{})
	if !ok {
		t.Fatal("ForgeTicks(steel platebody at 99/99) not ok")
	}
	if got < 150 {
		t.Errorf("ForgeTicks = %d ticks, below the 150 a perfect run needs", got)
	}
	if got > 600 {
		t.Errorf("ForgeTicks = %d ticks, implausibly slow", got)
	}
}

// More heat means more strikes before a reheat, so a higher Firemaking
// level can only help.
func TestForgeTicksImproveWithFiremaking(t *testing.T) {
	low, ok1 := ForgeTicks(5, "Rune", 99, 1, Boosts{})
	high, ok2 := ForgeTicks(5, "Rune", 99, 99, Boosts{})
	if !ok1 || !ok2 {
		t.Fatal("ForgeTicks not ok")
	}
	if high > low {
		t.Errorf("ForgeTicks with Firemaking 99 = %d, slower than with 1 = %d", high, low)
	}
}

// Boosts are the whole reason the parameter exists: they must move the
// number.
func TestForgeTicksImproveWithBoosts(t *testing.T) {
	plain, _ := ForgeTicks(5, "Rune", 99, 99, Boosts{})
	boosted, _ := ForgeTicks(5, "Rune", 99, 99,
		Boosts{BaseProgressBonus: 10, DoubleProgressPct: 34.4})
	if boosted >= plain {
		t.Errorf("boosted run took %d ticks, no better than the plain %d", boosted, plain)
	}
}

func TestParseBoostsKnownTokens(t *testing.T) {
	b, err := ParseBoosts("smithing_cape,careless5,luminite")
	if err != nil {
		t.Fatalf("ParseBoosts: %v", err)
	}
	if b.BaseProgressBonus != 11 {
		t.Errorf("BaseProgressBonus = %d, want 11", b.BaseProgressBonus)
	}
}

func TestParseBoostsEquipmentLevelRaisesRapidAndTinker(t *testing.T) {
	plain, err := ParseBoosts("rapid4,tinker2")
	if err != nil {
		t.Fatalf("ParseBoosts: %v", err)
	}
	upgraded, err := ParseBoosts("rapid4,tinker2,equipment20")
	if err != nil {
		t.Fatalf("ParseBoosts: %v", err)
	}
	if upgraded.DoubleProgressPct <= plain.DoubleProgressPct {
		t.Errorf("equipment20 did not raise the double-progress chance: %v vs %v",
			upgraded.DoubleProgressPct, plain.DoubleProgressPct)
	}
}

// A mistyped checkbox must not silently produce a number that looks
// fine.
func TestParseBoostsRejectsUnknownToken(t *testing.T) {
	if _, err := ParseBoosts("rapid4,rapid9"); err == nil {
		t.Error("ParseBoosts accepted an unknown token")
	}
}

func TestParseBoostsEmptyIsTheConservativeDefault(t *testing.T) {
	b, err := ParseBoosts("")
	if err != nil {
		t.Fatalf("ParseBoosts(\"\"): %v", err)
	}
	if b.BaseProgressBonus != 0 || b.DoubleProgressPct != 0 {
		t.Errorf("empty boosts = %+v, want zero", b)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./shared/rates/`
Expected: FAIL to build, `undefined: ForgeTicks`.

- [ ] **Step 4: Write the implementation**

Create `shared/rates/forge.go`:

```go
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
// from 6 ticks to 4, and then to 2. Confirmed against the rendered
// Smithing page — see Task 2 Step 1.
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
		progress += int(float64(base) * heatMultiplier(heat, maxHeat) * multiplier)
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
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./shared/rates/ -v`
Expected: PASS, every test.

- [ ] **Step 6: Check formatting and commit**

```bash
gofmt -l .
go test ./...
git add shared/rates docs
git commit -m "Simulate anvil forging instead of assuming a rate

A smithed item has no fixed tick cost, which is why its infobox says
varies: the player fills a progress meter and each strike is worth more
when the item is hot. The simulation follows the published mechanics —
a strike every 2 ticks costing 10 heat, progress scaled by heat tier,
maximum heat of 300 + 3x Smithing + 3x Firemaking, and reheats that get
cheaper at per-metal level milestones.

Boosts are a declared input rather than a guess, because none of the
perk and gear stack is visible in a hiscore lookup. An unknown token is
rejected: a mistyped checkbox quietly producing a plausible number is
worse than an error."
```

---

### Task 3: `recipe-service` — scrape ticks and facility

**Files:**
- Modify: `shared/models/models.go` (the `Recipe` struct)
- Modify: `recipe-service/internal/scraper/parser.go`
- Modify: `recipe-service/internal/scraper/parser_test.go`
- Create: `scripts/migrations/004_recipe_ticks_facility.sql`
- Modify: `README.md` (the migrations list)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `models.Recipe.Ticks int` and `models.Recipe.Facility string`, populated by the parser.

- [ ] **Step 1: Write the failing tests**

Append to `recipe-service/internal/scraper/parser_test.go`:

```go
func TestParseRecipeReadsTicks(t *testing.T) {
	wt := `{{Infobox Recipe
|ticks = 3
|facility = Furnace
|skill1 = Smithing
|skill1lvl = 20
|skill1exp = 75
|mat1 = Iron ore
|mat1qty = 1
|output1 = Steel bar
}}`
	rec := parseRecipe("Steel bar", wt)
	if rec == nil {
		t.Fatal("parseRecipe returned nil")
	}
	if rec.Ticks != 3 {
		t.Errorf("Ticks = %d, want 3", rec.Ticks)
	}
	if rec.Facility != "Furnace" {
		t.Errorf("Facility = %q, want Furnace", rec.Facility)
	}
}

// "varies" is the wiki's way of saying the cost depends on mechanics the
// infobox cannot express. It must not be read as a number.
func TestParseRecipeTreatsVariesAsUnknown(t *testing.T) {
	wt := `{{Infobox Recipe
|ticks = varies
|facility = Anvil
|skill1 = Smithing
|skill1lvl = 50
|skill1exp = 1200
|mat1 = Rune bar
|mat1qty = 5
|output1 = Rune platebody
}}`
	rec := parseRecipe("Rune platebody", wt)
	if rec == nil {
		t.Fatal("parseRecipe returned nil")
	}
	if rec.Ticks != 0 {
		t.Errorf("Ticks = %d, want 0 for a varies value", rec.Ticks)
	}
	if rec.Facility != "Anvil" {
		t.Errorf("Facility = %q, want Anvil", rec.Facility)
	}
}

func TestParseRecipeMissingTicksIsZero(t *testing.T) {
	wt := `{{Infobox Recipe
|skill1 = Crafting
|skill1lvl = 5
|skill1exp = 10
|mat1 = Ball of wool
|mat1qty = 1
|output1 = Wool
}}`
	rec := parseRecipe("Wool", wt)
	if rec == nil {
		t.Fatal("parseRecipe returned nil")
	}
	if rec.Ticks != 0 {
		t.Errorf("Ticks = %d, want 0", rec.Ticks)
	}
	if rec.Facility != "" {
		t.Errorf("Facility = %q, want empty", rec.Facility)
	}
}
```

Note: if `parseRecipe` has a different name or signature in
`parser.go`, use the real one — read the file first and match the
existing tests in `parser_test.go`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./recipe-service/internal/scraper/ -run TestParseRecipe`
Expected: FAIL, `rec.Ticks undefined`.

- [ ] **Step 3: Add the model fields**

In `shared/models/models.go`, inside `type Recipe struct`, after
`APHSource`:

```go
	// Ticks is the per-action tick cost from the recipe infobox. Zero
	// means the wiki did not publish one, or published "varies" because
	// the real cost depends on mechanics the infobox cannot express.
	Ticks int `gorm:"index" json:"ticks"`
	// Facility is where the recipe is made — Furnace, Anvil and so on.
	// It separates mechanics that share a skill.
	Facility string `gorm:"size:48" json:"facility,omitempty"`
```

- [ ] **Step 4: Parse the two fields**

In `recipe-service/internal/scraper/parser.go`, beside the existing
`reAPH`:

```go
	reTicks    = regexp.MustCompile(`\|\s*ticks\s*=\s*(\d+)\s*$`)
	reFacility = regexp.MustCompile(`\|\s*facility\s*=\s*([^|\n]+)`)
```

and where `reAPH` is applied, add:

```go
	// Only a bare number counts. "varies" means the cost depends on
	// heat, level or a minigame, and reading it as anything else would
	// invent a rate.
	if m := reTicks.FindStringSubmatch(wt); len(m) == 2 {
		if ticks, err := strconv.Atoi(m[1]); err == nil && ticks > 0 {
			rec.Ticks = ticks
		}
	}
	if m := reFacility.FindStringSubmatch(wt); len(m) == 2 {
		rec.Facility = strings.TrimSpace(m[1])
	}
```

The `reTicks` pattern needs `(?m)` multiline mode for `$` to match a line
end — add it to the pattern if the tests show the anchor failing.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./recipe-service/internal/scraper/ -v -run TestParseRecipe`
Expected: PASS.

- [ ] **Step 6: Write the migration**

Create `scripts/migrations/004_recipe_ticks_facility.sql`:

```sql
-- Add the recipe fields that drive tick-derived craft rates.
--
-- ticks is the per-action cost from the wiki's recipe infobox, and
-- facility separates mechanics that share a skill: smelting at a furnace
-- and forging at an anvil are both Smithing but are not the same
-- operation and do not have the same rate.
--
-- Both are populated by the next scrape. AutoMigrate creates them on a
-- fresh database from the model tags; this script is for databases that
-- already exist.
--
-- Safe to run more than once.

USE recipe_db;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA='recipe_db' AND TABLE_NAME='recipes' AND COLUMN_NAME='ticks'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE recipes ADD COLUMN ticks INT NOT NULL DEFAULT 0, ADD INDEX idx_recipes_ticks (ticks)',
  'SELECT "ticks column already present"');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA='recipe_db' AND TABLE_NAME='recipes' AND COLUMN_NAME='facility'
);
SET @sql := IF(@exists = 0,
  'ALTER TABLE recipes ADD COLUMN facility VARCHAR(48) NOT NULL DEFAULT ""',
  'SELECT "facility column already present"');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
```

- [ ] **Step 7: Apply the migration and re-scrape**

```bash
docker compose exec -T mysql mysql -uroot -proot < scripts/migrations/004_recipe_ticks_facility.sql
docker compose up -d --build recipe-service
curl -X POST localhost:8082/internal/scrape
```

Wait for `scrape complete` in `docker logs rs3-actioneer-recipe-service-1`
(about three minutes), then confirm the fields landed:

```bash
docker compose exec -T mysql mysql -uroot -proot -N -e "
SELECT COUNT(*) total, SUM(ticks>0) with_ticks, SUM(facility<>'') with_facility
FROM recipe_db.recipes;"
```

Expected: `with_ticks` is roughly half of `total` — the sampled coverage
was 55%. A result near zero means the regex is not matching; fix it
before moving on.

- [ ] **Step 8: Document the migration and commit**

Add to the migrations list in `README.md`, after the 003 entry:

```markdown
- **004** adds `recipes.ticks` and `recipes.facility`. The tick cost
  drives the craft-rate model; the facility separates smelting at a
  furnace from forging at an anvil, which share a skill but not a rate.
```

```bash
gofmt -l .
go test ./...
git add shared/models recipe-service scripts/migrations README.md
git commit -m "Scrape the recipe tick cost and facility

The rate model needs the per-action tick cost the recipe infobox
publishes, and the facility to tell smelting from forging — both are
Smithing, neither shares a rate with the other.

Only a bare number is read. The infobox writes varies when the real cost
depends on heat or level, and treating that as a value would invent
exactly the kind of number this work exists to remove."
```

---

### Task 4: `calc-service` — derive the rate at request time

**Files:**
- Modify: `shared/config/config.go` (`MarketConf`)
- Modify: `calc-service/internal/service/market.go`
- Modify: `calc-service/internal/service/calc.go` (`Options`, `Assumptions`, `chooseAPH`)
- Modify: `calc-service/internal/handler/calc.go` (`parseOpts`)
- Modify: `calc-service/config.yaml`
- Create: `calc-service/internal/service/rate_test.go`
- Modify: `shared/models/models.go` (new `APHSource*` constants)

**Interfaces:**
- Consumes: `rates.SmeltTicks`, `rates.ForgeTicks`, `rates.BarMetal`, `rates.SlotsPerCraft`, `rates.ActionsPerHour`, `rates.Config`, `rates.Boosts`, `rates.ParseBoosts` from Tasks 1-2. `models.Recipe.Ticks`, `.Facility` from Task 3.
- Produces: `chooseAPH` returning a rate and one of `override`, `ticks_level`, `ticks_forge`, `ticks`, `wiki`, `default`.

- [ ] **Step 1: Write the failing tests**

Create `calc-service/internal/service/rate_test.go`:

```go
package service

import (
	"testing"

	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/models"
	"github.com/rs3-market/backend/shared/rates"
)

func marketConfForTest() config.MarketConf {
	return config.MarketConf{
		SpreadPct:             2,
		TaxPct:                2,
		TaxCapPerItem:         5_000_000,
		DefaultActionsPerHour: 600,
		InventorySlots:        28,
		BankTripTicks:         21,
	}
}

func testEvaluator(opts Options, levels map[string]int) *evaluator {
	return &evaluator{
		opts:   opts,
		levels: levels,
		market: MarketFrom(marketConfForTest()),
		rates:  rates.DefaultConfig(),
	}
}

// An explicit override always wins: the caller asked for a specific
// number and must get it back.
func TestChooseAPHOverrideWins(t *testing.T) {
	e := testEvaluator(Options{ActionsPerHourOverride: 1234}, map[string]int{"Smithing": 99})
	rec := models.Recipe{Skill: "Smithing", Ticks: 3, Facility: "Furnace",
		OutputItemName: "Rune bar"}

	aph, src := e.chooseAPH(rec)
	if aph != 1234 || src != models.APHSourceOverride {
		t.Errorf("chooseAPH = %d, %q; want 1234, override", aph, src)
	}
}

// Smelting is level-dependent, and the infobox holds the best case, so
// the level table has to win over it.
func TestChooseAPHSmeltingUsesTheLevelTable(t *testing.T) {
	e := testEvaluator(Options{Player: "someone"}, map[string]int{"Smithing": 20})
	rec := models.Recipe{Skill: "Smithing", Ticks: 3, Facility: "Furnace",
		OutputItemName: "Steel bar",
		Inputs: []models.RecipeInput{{ItemName: "Iron ore", Quantity: 1}}}

	aph, src := e.chooseAPH(rec)
	if src != models.APHSourceTicksLevel {
		t.Fatalf("aph_source = %q, want ticks_level", src)
	}
	// Level 20 smelts steel in 5 ticks, not the infobox's 3.
	want := rates.ActionsPerHour(5, 1, rates.DefaultConfig())
	if aph != want {
		t.Errorf("chooseAPH = %d, want %d (5 ticks at level 20)", aph, want)
	}
}

func TestChooseAPHForgingSimulates(t *testing.T) {
	e := testEvaluator(Options{Player: "someone"},
		map[string]int{"Smithing": 99, "Firemaking": 99})
	rec := models.Recipe{Skill: "Smithing", Facility: "Anvil",
		OutputItemName: "Rune platebody",
		Inputs: []models.RecipeInput{{ItemName: "Rune bar", Quantity: 5}}}

	aph, src := e.chooseAPH(rec)
	if src != models.APHSourceTicksForge {
		t.Fatalf("aph_source = %q, want ticks_forge", src)
	}
	if aph <= 0 {
		t.Errorf("chooseAPH = %d, want a positive rate", aph)
	}
}

// Without a player there are no levels, so the level-dependent branches
// cannot run and the infobox value is used instead.
func TestChooseAPHWithoutPlayerFallsBackToInfoboxTicks(t *testing.T) {
	e := testEvaluator(Options{}, nil)
	rec := models.Recipe{Skill: "Crafting", Ticks: 3,
		Inputs: []models.RecipeInput{{ItemName: "Gold bar", Quantity: 1}}}

	aph, src := e.chooseAPH(rec)
	if src != models.APHSourceTicks {
		t.Fatalf("aph_source = %q, want ticks", src)
	}
	if want := rates.ActionsPerHour(3, 1, rates.DefaultConfig()); aph != want {
		t.Errorf("chooseAPH = %d, want %d", aph, want)
	}
}

// A recipe the wiki never published a tick cost for keeps the old
// behaviour, and keeps saying so.
func TestChooseAPHFallsBackToDefault(t *testing.T) {
	e := testEvaluator(Options{}, nil)
	rec := models.Recipe{Skill: "Divination", ActionsPerHour: 700,
		APHSource: models.APHSourceDefault}

	aph, src := e.chooseAPH(rec)
	if aph != 700 || src != models.APHSourceDefault {
		t.Errorf("chooseAPH = %d, %q; want 700, default", aph, src)
	}
}
```

The `evaluator` fields `levels` and `rates` are added in Step 5. If the
struct in `calc.go` names its existing fields differently, match the real
names rather than these — read the file first.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./calc-service/internal/service/ -run TestChooseAPH`
Expected: FAIL to build, `undefined: models.APHSourceTicksLevel`.

- [ ] **Step 3: Add the source constants**

In `shared/models/models.go`, beside the existing `APHSource*`:

```go
	// APHSourceTicks is a rate derived from the wiki's published tick
	// cost. APHSourceTicksLevel and APHSourceTicksForge are derived from
	// Smithing mechanics and the player's levels — the forge figure is a
	// floor, since it models no perk or potion the caller did not
	// declare.
	APHSourceTicks      = "ticks"
	APHSourceTicksLevel = "ticks_level"
	APHSourceTicksForge = "ticks_forge"
```

- [ ] **Step 4: Add the banking config**

In `shared/config/config.go`, inside `MarketConf`:

```go
	// InventorySlots and BankTripTicks model the trip to the bank when
	// the inventory runs out. Defaults of 28 and 21 put smelting bars at
	// the independently timed 1600 an hour.
	InventorySlots int `mapstructure:"inventory_slots"`
	BankTripTicks  int `mapstructure:"bank_trip_ticks"`
```

In `calc-service/internal/service/market.go`, carry them onto `Market`
in `MarketFrom`, defaulting to `rates.DefaultConfig()` values when unset,
and add a method:

```go
// RatesConfig is the banking model the throughput calculation uses.
func (m Market) RatesConfig() rates.Config {
	return rates.Config{
		InventorySlots: m.InventorySlots,
		BankTripTicks:  m.BankTripTicks,
	}
}
```

In `calc-service/config.yaml`, under `market:`:

```yaml
  inventory_slots: 28
  bank_trip_ticks: 21
```

- [ ] **Step 5: Rewrite chooseAPH**

Replace the body of `chooseAPH` in
`calc-service/internal/service/calc.go`:

```go
// chooseAPH decides how fast a recipe can actually be performed, and
// reports where the number came from. The order matters: a caller's
// override beats everything, mechanics beat the infobox because the
// infobox publishes the best case, and the house default is the last
// resort it has always been.
func (e *evaluator) chooseAPH(r models.Recipe) (int, string) {
	if e.opts.ActionsPerHourOverride > 0 {
		return e.opts.ActionsPerHourOverride, models.APHSourceOverride
	}

	slots := rates.SlotsPerCraft(r.Inputs)
	cfg := e.market.RatesConfig()

	if r.Skill == "Smithing" && e.levels != nil {
		smithing := e.levels["Smithing"]

		if r.Facility == "Furnace" {
			if metal, ok := rates.BarMetal(r.OutputItemName); ok {
				if ticks, ok := rates.SmeltTicks(metal, smithing); ok {
					return rates.ActionsPerHour(ticks, slots, cfg),
						models.APHSourceTicksLevel
				}
			}
		}

		if r.Facility == "Anvil" {
			for _, in := range r.Inputs {
				metal, ok := rates.BarMetal(in.ItemName)
				if !ok {
					continue
				}
				bars := in.Quantity
				if bars < 1 {
					bars = 1
				}
				ticks, ok := rates.ForgeTicks(bars, metal, smithing,
					e.levels["Firemaking"], e.opts.Boosts)
				if ok {
					return rates.ActionsPerHour(ticks, slots, cfg),
						models.APHSourceTicksForge
				}
			}
		}
	}

	if r.Ticks > 0 {
		return rates.ActionsPerHour(r.Ticks, slots, cfg), models.APHSourceTicks
	}
	if r.ActionsPerHour > 0 {
		src := r.APHSource
		if src == "" {
			src = models.APHSourceDefault
		}
		return r.ActionsPerHour, src
	}
	return e.market.DefaultActionsPerHour, models.APHSourceDefault
}
```

Add to `Options`:

```go
	// Boosts is the forging stack the caller declared. The zero value is
	// the conservative floor.
	Boosts rates.Boosts
	// BoostsRaw is the unparsed parameter, kept for the assumptions
	// block and the cache key.
	BoostsRaw string
```

Add `levels map[string]int` and `rates rates.Config` to `evaluator`, and
populate `levels` where `playerLevels` is already called around
`calc.go:182`.

- [ ] **Step 6: Echo the assumptions**

Add to `Assumptions` in `calc.go`:

```go
	InventorySlots int     `json:"inventory_slots"`
	BankTripTicks  int     `json:"bank_trip_ticks"`
	Boosts         string  `json:"boosts,omitempty"`
```

and fill them where `Assumptions` is built.

- [ ] **Step 7: Accept the boosts parameter**

In `calc-service/internal/handler/calc.go`, inside `parseOpts`:

```go
	boosts, err := rates.ParseBoosts(c.Query("boosts"))
	if err != nil {
		return Options{}, err
	}
	opts.Boosts = boosts
	opts.BoostsRaw = c.Query("boosts")
```

The existing `parseOpts` error path returns 400 — confirm it does, and
make it do so if it does not. An unknown boost token must be a 400.

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test ./calc-service/... -v -run TestChooseAPH`
Expected: PASS.

- [ ] **Step 9: Check it end to end**

```bash
docker compose up -d --build calc-service
curl -s "localhost:8080/calc/45543?player=okadishe" | python3 -m json.tool | grep -E "aph|actions_per_hour" | head
```

Expected: `aph_source` reads `ticks_forge` for a smithed item, and the
rate is far below the old flat 600.

- [ ] **Step 10: Commit**

```bash
gofmt -l .
go test ./...
git add shared calc-service
git commit -m "Derive actions per hour from mechanics at request time

The rate stopped being a property of the recipe the moment it became
level-dependent: smelting a steel bar takes 5 ticks at level 20 and 3 at
23, and forging depends on Smithing and Firemaking together. So it is
computed per request, from levels the calculator already fetches for the
player parameter.

Precedence runs override, then Smithing mechanics, then the infobox tick
cost, then the wiki value, then the house default. Mechanics beat the
infobox deliberately: the infobox publishes the best case, which is a
lie for anyone below the last threshold.

aph_source names the branch, so a derived rate is never mistaken for an
assumed one."
```

---

### Task 5: `GET /calc/top`

**Files:**
- Create: `calc-service/internal/service/top.go`
- Create: `calc-service/internal/service/top_test.go`
- Modify: `calc-service/internal/handler/calc.go`
- Modify: `calc-service/internal/handler/contract_test.go`
- Modify: `openapi/combined.yaml`
- Modify: `README.md`

**Interfaces:**
- Consumes: everything from Tasks 1-4, plus the existing `CalcCache` (`Get(key string, v interface{}) bool`, `Set(key string, v interface{})`) and the recipe client's item-ID listing.
- Produces: `service.Top(ctx, TopOptions) (TopResponse, error)` and the `/calc/top` route.

- [ ] **Step 1: Write the failing tests**

Create `calc-service/internal/service/top_test.go`:

```go
package service

import "testing"

// The money metric sorts on the buy-limit-bound figure. Sorting on the
// raw one floats exactly the items nobody can make 600 times an hour —
// a godsword at 938M GP/h above things people actually craft.
func TestSortTopUsesLimitBoundGPForMoney(t *testing.T) {
	rows := []TopRow{
		{ItemID: 1, GPPerHour: 900_000_000, GPPerHourLimited: 2_000_000},
		{ItemID: 2, GPPerHour: 20_000_000, GPPerHourLimited: 21_000_000},
	}
	sortTop(rows, MetricGPPerHour)
	if rows[0].ItemID != 2 {
		t.Errorf("top row is item %d, want the one with the higher limited rate", rows[0].ItemID)
	}
}

func TestSortTopByXP(t *testing.T) {
	rows := []TopRow{
		{ItemID: 1, XPPerHour: 100},
		{ItemID: 2, XPPerHour: 500},
	}
	sortTop(rows, MetricXPPerHour)
	if rows[0].ItemID != 2 {
		t.Errorf("top row is item %d, want 2", rows[0].ItemID)
	}
}

func TestSortTopByGPPerXP(t *testing.T) {
	rows := []TopRow{
		{ItemID: 1, GPPerXP: 2.5},
		{ItemID: 2, GPPerXP: 9.5},
	}
	sortTop(rows, MetricGPPerXP)
	if rows[0].ItemID != 2 {
		t.Errorf("top row is item %d, want 2", rows[0].ItemID)
	}
}

func TestParseMetricRejectsUnknown(t *testing.T) {
	if _, err := ParseMetric("profit"); err == nil {
		t.Error("ParseMetric accepted an unknown metric")
	}
}

func TestParseMetricDefaultsToNothing(t *testing.T) {
	if _, err := ParseMetric(""); err == nil {
		t.Error("ParseMetric accepted an empty metric; it is required")
	}
}

func TestClampLimit(t *testing.T) {
	if got := clampLimit(0); got != 20 {
		t.Errorf("clampLimit(0) = %d, want the default 20", got)
	}
	if got := clampLimit(5000); got != 100 {
		t.Errorf("clampLimit(5000) = %d, want the cap 100", got)
	}
	if got := clampLimit(50); got != 50 {
		t.Errorf("clampLimit(50) = %d, want 50", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./calc-service/internal/service/ -run TestSortTop`
Expected: FAIL to build, `undefined: TopRow`.

- [ ] **Step 3: Write the ranking**

Create `calc-service/internal/service/top.go`:

```go
package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Metric is what /calc/top ranks by.
type Metric string

const (
	MetricXPPerHour Metric = "xp_per_hour"
	MetricGPPerHour Metric = "gp_per_hour"
	MetricGPPerXP   Metric = "gp_per_xp"
)

const (
	defaultTopLimit = 20
	maxTopLimit     = 100
)

// ParseMetric rejects anything it does not rank. There is no default:
// asking for a ranking without saying of what is a mistake worth
// reporting.
func ParseMetric(s string) (Metric, error) {
	switch Metric(strings.TrimSpace(s)) {
	case MetricXPPerHour:
		return MetricXPPerHour, nil
	case MetricGPPerHour:
		return MetricGPPerHour, nil
	case MetricGPPerXP:
		return MetricGPPerXP, nil
	}
	return "", fmt.Errorf("unknown metric %q", s)
}

func clampLimit(n int) int {
	if n <= 0 {
		return defaultTopLimit
	}
	if n > maxTopLimit {
		return maxTopLimit
	}
	return n
}

// TopRow is one ranked production path.
type TopRow struct {
	ItemID           int64   `json:"item_id"`
	Name             string  `json:"name"`
	Skill            string  `json:"skill"`
	LevelReq         int     `json:"level_req"`
	XPPerHour        float64 `json:"xp_per_hour"`
	GPPerHour        float64 `json:"gp_per_hour"`
	GPPerHourLimited float64 `json:"gp_per_hour_limited"`
	GPPerXP          float64 `json:"gp_per_xp"`
	ActionsPerHour   int     `json:"actions_per_hour"`
	APHSource        string  `json:"aph_source"`
	Complete         bool    `json:"complete"`
	BindingItemName  string  `json:"binding_item_name,omitempty"`
}

type TopOptions struct {
	Metric Metric
	Skill  string
	Limit  int
	Calc   Options
}

type TopResponse struct {
	Metric      Metric      `json:"metric"`
	Count       int         `json:"count"`
	Rows        []TopRow    `json:"rows"`
	Assumptions Assumptions `json:"assumptions"`
}

// sortTop orders rows best-first.
//
// The money metric sorts on the buy-limit-bound rate. The raw figure
// multiplies one craft's profit by a full hour of actions, which floats
// items nobody can produce at that rate above the ones people actually
// make.
func sortTop(rows []TopRow, m Metric) {
	sort.SliceStable(rows, func(i, j int) bool {
		switch m {
		case MetricXPPerHour:
			return rows[i].XPPerHour > rows[j].XPPerHour
		case MetricGPPerXP:
			return rows[i].GPPerXP > rows[j].GPPerXP
		default:
			return rows[i].GPPerHourLimited > rows[j].GPPerHourLimited
		}
	})
}

// Top ranks every priceable recipe by one metric.
//
// It walks the whole catalogue, so it answers from the cache on anything
// but the first call for a given parameter set.
func (s *Service) Top(ctx context.Context, opts TopOptions) (TopResponse, error) {
	opts.Limit = clampLimit(opts.Limit)

	key := fmt.Sprintf("top:%s:%s:%s:%d:%t:%s",
		opts.Metric, opts.Calc.Player, opts.Skill, opts.Limit,
		opts.Calc.IncludeIncomplete, opts.Calc.BoostsRaw)

	var cached TopResponse
	if s.cc.Get(key, &cached) {
		return cached, nil
	}

	ids, err := s.recipe.AllItemIDs(ctx)
	if err != nil {
		return TopResponse{}, fmt.Errorf("list item ids: %w", err)
	}

	best := make(map[int64]TopRow, len(ids))
	var assumptions Assumptions

	for _, id := range ids {
		res, err := s.Calc(ctx, id, opts.Calc)
		if err != nil {
			// One unpriceable item must not fail the ranking.
			continue
		}
		assumptions = res.Assumptions

		for _, p := range res.Paths {
			row := rowFromPath(id, res, p)
			if opts.Skill != "" && !strings.EqualFold(row.Skill, opts.Skill) {
				continue
			}
			if cur, ok := best[id]; !ok || betterRow(row, cur, opts.Metric) {
				best[id] = row
			}
		}
	}

	rows := make([]TopRow, 0, len(best))
	for _, r := range best {
		rows = append(rows, r)
	}
	sortTop(rows, opts.Metric)
	if len(rows) > opts.Limit {
		rows = rows[:opts.Limit]
	}

	out := TopResponse{
		Metric:      opts.Metric,
		Count:       len(rows),
		Rows:        rows,
		Assumptions: assumptions,
	}
	s.cc.Set(key, out)
	return out, nil
}

// betterRow compares two paths for the same item by the ranked metric.
func betterRow(a, b TopRow, m Metric) bool {
	switch m {
	case MetricXPPerHour:
		return a.XPPerHour > b.XPPerHour
	case MetricGPPerXP:
		return a.GPPerXP > b.GPPerXP
	default:
		return a.GPPerHourLimited > b.GPPerHourLimited
	}
}
```

`rowFromPath` maps one evaluated path onto a `TopRow`. Write it against
the real `CalcResult` and `CalcPath` field names in `calc.go` — the
names above match what `/calc/{itemID}` already returns in JSON, but the
Go field names may differ. `s.recipe.AllItemIDs` likewise: if the recipe
client has no such method, add one calling `GET /recipes/ids`, which the
poller already consumes in `ge-price-service/internal/client`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./calc-service/internal/service/ -v -run "TestSortTop|TestParseMetric|TestClampLimit"`
Expected: PASS.

- [ ] **Step 5: Register the route**

In `calc-service/internal/handler/calc.go`, before the `:itemID` route so
`top` is never read as an item ID:

```go
	r.GET("/calc/batch", h.batch)
	r.GET("/calc/top", h.top)
	r.GET("/calc/:itemID", h.calc)
```

`h.top` parses `metric` (required, 400 on unknown), `player`, `skill`,
`limit`, `include_incomplete` and `boosts`, then calls `svc.Top`.

- [ ] **Step 6: Document the route in the spec**

Add `/calc/top` to `openapi/combined.yaml` with all six parameters and a
`TopResponse` schema, then regenerate and check the contract:

```bash
make openapi
go test ./calc-service/internal/handler/ -run TestRoutesMatchOpenAPISpec -v
```

Expected: PASS. The contract test fails if the spec and the routes
disagree.

- [ ] **Step 7: Check it end to end**

```bash
docker compose up -d --build calc-service gateway
curl -s "localhost:8080/calc/top?metric=xp_per_hour&player=okadishe&limit=5" | python3 -m json.tool
curl -s "localhost:8080/calc/top?metric=gp_per_hour&player=okadishe&limit=5" | python3 -m json.tool
curl -s -o /dev/null -w "%{http_code}\n" "localhost:8080/calc/top?metric=nonsense"
```

Expected: two ranked lists whose `aph_source` is mostly `ticks`,
`ticks_level` or `ticks_forge` rather than `default`, and `400` for the
unknown metric.

- [ ] **Step 8: Document and commit**

Add a `/calc/top` line to the README's local-operations block, then:

```bash
gofmt -l .
go test ./...
git add calc-service openapi README.md
git commit -m "Add /calc/top for the three headline metrics

Ranking the catalogue meant calling /calc/batch fifty ids at a time and
sorting the result by hand, which is a thing every consumer would have
rebuilt.

The money metric sorts on the buy-limit-bound rate, not the raw one.
Sorting on the raw figure puts a godsword nobody can forge six hundred
times an hour above the things people actually craft, which is the
failure this whole change set exists to fix.

The route walks every priceable recipe, so it answers from the cache on
anything but the first call."
```
