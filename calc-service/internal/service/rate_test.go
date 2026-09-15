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

// market.RatesConfig() is the single source for the banking model — the
// evaluator does not carry its own copy, so there is nothing here for it
// to drift from.
func testEvaluator(opts Options, levels map[string]int) *evaluator {
	return &evaluator{
		opts:   opts,
		levels: levels,
		market: MarketFrom(marketConfForTest()),
	}
}

// An explicit override always wins: the caller asked for a specific
// number and must get it back.
func TestChooseAPHOverrideWins(t *testing.T) {
	e := testEvaluator(Options{ActionsPerHourOverride: 1234}, map[string]int{"smithing": 99})
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
	e := testEvaluator(Options{Player: "someone"}, map[string]int{"smithing": 20})
	rec := models.Recipe{Skill: "Smithing", Ticks: 3, Facility: "Furnace",
		OutputItemName: "Steel bar",
		Inputs:         []models.RecipeInput{{ItemName: "Iron ore", Quantity: 1}}}

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
		map[string]int{"smithing": 99, "firemaking": 99})
	rec := models.Recipe{Skill: "Smithing", Facility: "Anvil",
		OutputItemName: "Rune platebody",
		Inputs:         []models.RecipeInput{{ItemName: "Rune bar", Quantity: 5}}}

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
