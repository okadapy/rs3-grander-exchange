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
		t.Errorf("ActionsPerHour(3, 1) = %v, want 1600", got)
	}
}

// With nothing to carry there is no trip to the bank, so the rate is the
// raw tick ceiling.
func TestActionsPerHourAllStackableInputsHitTheCeiling(t *testing.T) {
	if got := ActionsPerHour(3, 0, DefaultConfig()); got != 2000 {
		t.Errorf("ActionsPerHour(3, 0) = %v, want 2000", got)
	}
}

// Five bars per craft means five crafts an inventory, so the bank trip is
// amortised over far fewer actions and the rate drops well under the
// single-slot case.
func TestActionsPerHourFallsWhenACraftEatsMoreSlots(t *testing.T) {
	got := ActionsPerHour(3, 5, DefaultConfig())
	if got >= 1600 {
		t.Errorf("ActionsPerHour(3, 5) = %v, want below the single-slot 1600", got)
	}
	if got <= 0 {
		t.Errorf("ActionsPerHour(3, 5) = %v, want a positive rate", got)
	}
}

func TestActionsPerHourRejectsNonPositiveTicks(t *testing.T) {
	if got := ActionsPerHour(0, 1, DefaultConfig()); got != 0 {
		t.Errorf("ActionsPerHour(0, 1) = %v, want 0", got)
	}
}

// Zero-valued Config should be rejected and replaced with DefaultConfig.
func TestActionsPerHourZeroValuedConfigFallback(t *testing.T) {
	// Config{} is zero-valued; should fall back to DefaultConfig
	got := ActionsPerHour(3, 1, Config{})
	if got != 1600 {
		t.Errorf("ActionsPerHour(3, 1, Config{}) = %v, want 1600 (defaulted)", got)
	}
}

// InventorySlots <= 0 should be rejected and replaced with DefaultConfig.
func TestActionsPerHourInvalidInventorySlotsRejectAndDefault(t *testing.T) {
	got := ActionsPerHour(3, 1, Config{InventorySlots: 0, BankTripTicks: 21})
	if got != 1600 {
		t.Errorf("ActionsPerHour with InventorySlots=0 = %v, want 1600 (defaulted)", got)
	}
	got = ActionsPerHour(3, 1, Config{InventorySlots: -1, BankTripTicks: 21})
	if got != 1600 {
		t.Errorf("ActionsPerHour with InventorySlots=-1 = %v, want 1600 (defaulted)", got)
	}
}

// BankTripTicks < 0 should be rejected and replaced with DefaultConfig.
func TestActionsPerHourNegativeBankTripTicksRejectAndDefault(t *testing.T) {
	got := ActionsPerHour(3, 1, Config{InventorySlots: 28, BankTripTicks: -1})
	if got != 1600 {
		t.Errorf("ActionsPerHour with BankTripTicks=-1 = %v, want 1600 (defaulted)", got)
	}
}

// BankTripTicks = 0 is valid: it means no bank trip overhead.
// With 28 slots and 1 slot per craft, this gives the maximum rate.
func TestActionsPerHourZeroBankTripTicksIsValid(t *testing.T) {
	cfg := Config{InventorySlots: 28, BankTripTicks: 0}
	got := ActionsPerHour(3, 1, cfg)
	// craftsPerTrip = 28, tripTicks = 28*3 + 0 = 84
	// rate = 28 * 6000 / 84 = 168000 / 84 = 2000
	if got != 2000 {
		t.Errorf("ActionsPerHour(3, 1, BankTripTicks=0) = %v, want 2000 (no bank trip)", got)
	}
}

// The rate is genuinely fractional at the top of the smithing tree. An
// Elder rune platebody + 5 costs 80 bars, which ForgeTicks puts at
// 10136 ticks — about 1.7 hours for one item. Computed by integer
// division the rate truncated to zero, and calc-service's 1/aph then
// became +Inf and took the whole response with it.
func TestActionsPerHourIsFractionalForAnEightyBarCraft(t *testing.T) {
	ticks, ok := ForgeTicks(80, "Elder rune", 90, 90, Boosts{})
	if !ok {
		t.Fatal("ForgeTicks(80 Elder rune bars at 90/90) not ok")
	}

	got := ActionsPerHour(ticks, 80, DefaultConfig())
	if got <= 0 {
		t.Fatalf("ActionsPerHour(%d, 80) = %v, want a positive fractional rate", ticks, got)
	}

	// One item every ~1.7 hours. Claiming one an hour would overstate
	// its rate by seventy percent.
	hours := 1 / float64(got)
	if hours < 1.5 || hours > 2 {
		t.Errorf("ActionsPerHour(%d, 80) = %v, which is one item every %v hours; want ~1.7",
			ticks, got, hours)
	}
}
