package service

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/rs3-market/backend/calc-service/internal/client"
	"github.com/rs3-market/backend/shared/models"
)

// ---- fixtures ----

func recipe(name string, outID int64, outQty int, skill string, level int, xp float64, aph int, inputs ...models.RecipeInput) models.Recipe {
	return models.Recipe{
		Name:           name,
		OutputItemID:   outID,
		OutputItemName: name,
		OutputQty:      outQty,
		Skill:          skill,
		LevelReq:       level,
		XPPerAction:    xp,
		ActionsPerHour: aph,
		APHSource:      models.APHSourceWiki,
		Inputs:         inputs,
	}
}

func input(id int64, name string, qty int) models.RecipeInput {
	return models.RecipeInput{ItemID: id, ItemName: name, Quantity: qty}
}

func snapshots(prices map[int64]int64) map[int64]models.PriceSnapshot {
	out := make(map[int64]models.PriceSnapshot, len(prices))
	for id, p := range prices {
		out[id] = models.PriceSnapshot{ItemID: id, Price: p, Timestamp: time.Now().UTC()}
	}
	return out
}

func noSpread() Market {
	return Market{TaxPct: 0, DefaultActionsPerHour: 600}
}

// ---- pricing and completeness ----

// An input with no price used to be silently costed at zero, which made
// the least tradeable recipes look like the most profitable ones. The
// path must now be marked incomplete and name what it could not price.
func TestUnpricedInputMarksPathIncomplete(t *testing.T) {
	node := &client.Node{
		Recipe: recipe("Widget", 100, 1, "Crafting", 10, 5, 100,
			input(200, "Priced thing", 1),
			input(201, "Unpriced thing", 1),
		),
	}
	ev := &evaluator{
		market: noSpread(),
		prices: snapshots(map[int64]int64{100: 1000, 200: 100}),
	}

	paths := ev.expand(node)
	if len(paths) != 1 {
		t.Fatalf("got %d paths, want 1", len(paths))
	}
	p := paths[0]

	if p.Complete {
		t.Error("path with an unpriced input must not be marked complete")
	}
	if len(p.UnpricedInputs) != 1 || p.UnpricedInputs[0] != "Unpriced thing" {
		t.Errorf("unpriced_inputs = %v, want [Unpriced thing]", p.UnpricedInputs)
	}
}

func TestZeroItemIdIsUnpriced(t *testing.T) {
	node := &client.Node{
		Recipe: recipe("Widget", 100, 1, "Crafting", 10, 5, 100,
			input(0, "Unresolved", 1),
		),
	}
	ev := &evaluator{market: noSpread(), prices: snapshots(map[int64]int64{100: 1000})}

	p := ev.expand(node)[0]
	if p.Complete {
		t.Error("an input with item_id 0 cannot be priced, so the path is incomplete")
	}
	if p.BuyCost != 0 {
		t.Errorf("buy cost = %v; the unpriced input contributes nothing, which is why the path is flagged", p.BuyCost)
	}
}

func TestCompletePathArithmetic(t *testing.T) {
	node := &client.Node{
		Recipe: recipe("Widget", 100, 1, "Crafting", 10, 20, 100,
			input(200, "Bar", 2),
		),
	}
	ev := &evaluator{
		market: Market{TaxPct: 2, TaxCapPerItem: 5_000_000, DefaultActionsPerHour: 600},
		prices: snapshots(map[int64]int64{100: 1000, 200: 300}),
	}

	p := ev.expand(node)[0]
	if !p.Complete {
		t.Fatalf("expected a complete path, unpriced: %v", p.UnpricedInputs)
	}

	approx(t, p.BuyCost, 600, "buy cost")          // 2 bars at 300
	approx(t, p.SellRevenue, 1000, "sell revenue") // 1 widget at 1000
	approx(t, p.TaxPaid, 20, "tax")                // 2% of 1000
	approx(t, p.ProfitPerCraft, 380, "profit")     // 1000 - 20 - 600
	approx(t, p.TotalHours, 0.01, "hours")         // 1 / 100 aph
	approx(t, p.GPPerHour, 38000, "gp per hour")   // 380 / 0.01
	approx(t, p.XPPerHour, 2000, "xp per hour")    // 20 / 0.01
	approx(t, p.GPPerXP, 19, "gp per xp")          // 380 / 20
	approx(t, p.ROIPct, 380.0/600.0*100, "roi")
}

func TestSpreadReducesProfit(t *testing.T) {
	node := &client.Node{
		Recipe: recipe("Widget", 100, 1, "Crafting", 10, 20, 100,
			input(200, "Bar", 1),
		),
	}
	prices := snapshots(map[int64]int64{100: 1000, 200: 500})

	flat := (&evaluator{market: Market{DefaultActionsPerHour: 600}, prices: prices}).expand(node)[0]
	spread := (&evaluator{market: Market{SpreadPct: 4, DefaultActionsPerHour: 600}, prices: prices}).expand(node)[0]

	if spread.ProfitPerCraft >= flat.ProfitPerCraft {
		t.Errorf("a spread must reduce profit: %v with spread vs %v without",
			spread.ProfitPerCraft, flat.ProfitPerCraft)
	}
	approx(t, spread.BuyCost, 510, "buy cost with spread")    // 500 * 1.02
	approx(t, spread.SellRevenue, 980, "revenue with spread") // 1000 * 0.98
}

func TestOutputQuantityMultipliesRevenueAndTax(t *testing.T) {
	node := &client.Node{
		Recipe: recipe("Cannonball", 2, 4, "Smithing", 35, 25.5, 100,
			input(200, "Steel bar", 1),
		),
	}
	ev := &evaluator{
		market: Market{TaxPct: 2, TaxCapPerItem: 5_000_000, DefaultActionsPerHour: 600},
		prices: snapshots(map[int64]int64{2: 500, 200: 1500}),
	}

	p := ev.expand(node)[0]
	approx(t, p.SellRevenue, 2000, "revenue for 4 cannonballs")
	approx(t, p.TaxPaid, 40, "tax on the whole sale")
	approx(t, p.ProfitPerCraft, 460, "profit") // 2000 - 40 - 1500
}

// ---- buy versus craft enumeration ----

func TestExpandEnumeratesBuyAndCraft(t *testing.T) {
	child := &client.Node{
		Recipe: recipe("Bar", 200, 1, "Smithing", 15, 10, 200,
			input(300, "Ore", 1),
		),
	}
	node := &client.Node{
		Recipe: recipe("Widget", 100, 1, "Crafting", 10, 20, 100,
			input(200, "Bar", 1),
		),
		Children: []*client.Node{child},
	}
	ev := &evaluator{
		market: noSpread(),
		prices: snapshots(map[int64]int64{100: 5000, 200: 1000, 300: 100}),
	}

	paths := ev.expand(node)
	if len(paths) != 2 {
		t.Fatalf("got %d paths, want 2 (buy the bar, or smelt it)", len(paths))
	}

	var buyBar, craftBar *PathResult
	for i := range paths {
		if len(paths[i].Steps) == 1 {
			buyBar = &paths[i]
		} else {
			craftBar = &paths[i]
		}
	}
	if buyBar == nil || craftBar == nil {
		t.Fatal("expected one single-step path and one two-step path")
	}

	approx(t, buyBar.BuyCost, 1000, "cost of buying the bar")
	approx(t, craftBar.BuyCost, 100, "cost of smelting from ore")
	if craftBar.TotalXP <= buyBar.TotalXP {
		t.Error("crafting the intermediate should accrue its XP too")
	}
	if craftBar.TotalHours <= buyBar.TotalHours {
		t.Error("crafting the intermediate should take longer")
	}
}

func TestChildRunsScaleWithQuantityAndOutput(t *testing.T) {
	// The child produces 2 per craft; the parent needs 3, so it runs twice.
	child := &client.Node{Recipe: recipe("Bar", 200, 2, "Smithing", 15, 10, 100, input(300, "Ore", 1))}
	node := &client.Node{
		Recipe:   recipe("Widget", 100, 1, "Crafting", 10, 20, 100, input(200, "Bar", 3)),
		Children: []*client.Node{child},
	}
	ev := &evaluator{market: noSpread(), prices: snapshots(map[int64]int64{100: 9000, 200: 500, 300: 50})}

	var crafted *PathResult
	for _, p := range ev.expand(node) {
		if len(p.Steps) == 2 {
			crafted = &p
			break
		}
	}
	if crafted == nil {
		t.Fatal("expected a path that crafts the intermediate")
	}
	// 2 child runs at 50 GP of ore each.
	approx(t, crafted.BuyCost, 100, "ore cost across 2 child runs")
	approx(t, crafted.TotalXP, 40, "xp: 20 parent + 2 x 10 child")
}

func TestEnumerateRespectsCombinationBound(t *testing.T) {
	// Eight craftable inputs with two paths each would be 3^8 = 6561.
	var craftable []childPlan
	for i := 0; i < 8; i++ {
		craftable = append(craftable, childPlan{
			inputIdx: i,
			runs:     1,
			paths:    []PathResult{{}, {}},
		})
	}
	got := enumerate(craftable)

	if len(got) > maxCombinationsPerNode {
		t.Errorf("got %d combinations, want at most %d", len(got), maxCombinationsPerNode)
	}
	if len(got) == 0 {
		t.Fatal("bounding must still return the all-buy baseline")
	}
	for i, row := range got {
		if len(row) != len(craftable) {
			t.Fatalf("row %d has %d choices, want %d", i, len(row), len(craftable))
		}
	}
}

func TestEnumerateWithNoCraftableInputs(t *testing.T) {
	got := enumerate(nil)
	if len(got) != 1 || len(got[0]) != 0 {
		t.Errorf("got %v, want a single empty assignment", got)
	}
}

// ---- throughput ----

// Buy limits are the difference between a theoretical rate and one
// anybody can reach: you cannot craft 1,000/hr from a material you may
// only buy 100 of every four hours.
func TestThroughputPicksTheBindingInput(t *testing.T) {
	node := &client.Node{
		Recipe: recipe("Widget", 100, 1, "Crafting", 10, 20, 1000,
			input(200, "Plentiful", 1),
			input(201, "Scarce", 2),
		),
	}
	ev := &evaluator{
		market: noSpread(),
		prices: snapshots(map[int64]int64{100: 1000, 200: 10, 201: 10}),
		limits: map[int64]int{
			200: 25000, // 25,000 crafts per 4h
			201: 100,   // 50 crafts per 4h -- this one binds
		},
	}

	p := ev.expand(node)[0]
	if p.Throughput == nil {
		t.Fatal("expected a throughput block")
	}
	if p.Throughput.BindingItemName != "Scarce" {
		t.Errorf("binding item = %q, want Scarce", p.Throughput.BindingItemName)
	}
	if p.Throughput.MaxCraftsPer4h != 50 {
		t.Errorf("max crafts per 4h = %d, want 50", p.Throughput.MaxCraftsPer4h)
	}
	approx(t, p.Throughput.CraftsPerHour, 12.5, "crafts per hour")

	if p.Throughput.GPPerHour >= p.GPPerHour {
		t.Errorf("limited rate (%v) should be below the unconstrained rate (%v)",
			p.Throughput.GPPerHour, p.GPPerHour)
	}
}

func TestThroughputCappedByActionsPerHour(t *testing.T) {
	// A generous limit must not inflate the rate past what you can click.
	node := &client.Node{
		Recipe: recipe("Widget", 100, 1, "Crafting", 10, 20, 100,
			input(200, "Cheap", 1),
		),
	}
	ev := &evaluator{
		market: noSpread(),
		prices: snapshots(map[int64]int64{100: 1000, 200: 10}),
		limits: map[int64]int{200: 1_000_000},
	}

	p := ev.expand(node)[0]
	approx(t, p.Throughput.CraftsPerHour, 100, "crafts per hour capped by aph")
}

func TestNoThroughputWhenLimitUnknown(t *testing.T) {
	node := &client.Node{
		Recipe: recipe("Widget", 100, 1, "Crafting", 10, 20, 100, input(200, "Thing", 1)),
	}
	ev := &evaluator{market: noSpread(), prices: snapshots(map[int64]int64{100: 1000, 200: 10})}

	if p := ev.expand(node)[0]; p.Throughput != nil {
		t.Errorf("throughput should be nil when no limit is known, got %+v", p.Throughput)
	}
}

// A crafted input is produced, not purchased, so no buy limit applies.
func TestThroughputIgnoresCraftedInputs(t *testing.T) {
	child := &client.Node{Recipe: recipe("Bar", 200, 1, "Smithing", 15, 10, 100, input(300, "Ore", 1))}
	node := &client.Node{
		Recipe:   recipe("Widget", 100, 1, "Crafting", 10, 20, 100, input(200, "Bar", 1)),
		Children: []*client.Node{child},
	}
	ev := &evaluator{
		market: noSpread(),
		prices: snapshots(map[int64]int64{100: 5000, 200: 1000, 300: 100}),
		limits: map[int64]int{200: 4, 300: 100000},
	}

	for _, p := range ev.expand(node) {
		if len(p.Steps) == 2 && p.Throughput != nil && p.Throughput.BindingItemID == 200 {
			t.Error("a crafted input must not constrain throughput via its buy limit")
		}
	}
}

// ---- actions per hour ----

func TestChooseAPHPrecedence(t *testing.T) {
	r := recipe("Widget", 100, 1, "Crafting", 10, 20, 250)

	ev := &evaluator{market: Market{DefaultActionsPerHour: 600}}
	if aph, src := ev.chooseAPH(r); aph != 250 || src != models.APHSourceWiki {
		t.Errorf("recipe value = %v/%s, want 250/wiki", aph, src)
	}

	ev.opts.ActionsPerHourOverride = 900
	if aph, src := ev.chooseAPH(r); aph != 900 || src != models.APHSourceOverride {
		t.Errorf("override = %v/%s, want 900/override", aph, src)
	}

	ev.opts.ActionsPerHourOverride = 0
	r.ActionsPerHour = 0
	if aph, src := ev.chooseAPH(r); aph != 600 || src != models.APHSourceDefault {
		t.Errorf("fallback = %v/%s, want 600/default", aph, src)
	}
}

// ---- skill requirements ----

func TestApplySkillRequirements(t *testing.T) {
	p := PathResult{Steps: []Step{
		{Skill: "Smithing", LevelReq: 15},
		{Skill: "Crafting", LevelReq: 22},
		{Skill: "Crafting", LevelReq: 40}, // the higher gate wins
	}}
	applySkillRequirements(&p, map[string]int{"smithing": 20, "crafting": 30})

	if p.MeetsRequirements == nil || *p.MeetsRequirements {
		t.Error("crafting 30 does not meet the level 40 gate")
	}
	if len(p.SkillRequirements) != 2 {
		t.Fatalf("got %d requirements, want 2", len(p.SkillRequirements))
	}
	if p.SkillRequirements[0].Skill != "Crafting" || p.SkillRequirements[0].Required != 40 {
		t.Errorf("crafting requirement = %+v, want the highest gate of 40", p.SkillRequirements[0])
	}
	if !p.SkillRequirements[1].Met {
		t.Error("smithing 20 meets the level 15 gate")
	}
}

// With no hiscore data, claiming the player qualifies would be a guess
// presented as fact.
func TestApplySkillRequirementsNoPlayerLeavesFieldsNil(t *testing.T) {
	p := PathResult{Steps: []Step{{Skill: "Crafting", LevelReq: 99}}}
	applySkillRequirements(&p, nil)

	if p.MeetsRequirements != nil {
		t.Error("meets_requirements must stay nil without hiscore data")
	}
	if p.SkillRequirements != nil {
		t.Error("skill_requirements must stay nil without hiscore data")
	}
}

// ---- ranking ----

func TestSortPathsRanksAchievableFirst(t *testing.T) {
	yes, no := true, false
	paths := []PathResult{
		{Path: []string{"rich but locked"}, GPPerHour: 10000, Complete: true, MeetsRequirements: &no},
		{Path: []string{"modest but doable"}, GPPerHour: 500, Complete: true, MeetsRequirements: &yes},
	}
	sortPaths(paths)

	if paths[0].Path[0] != "modest but doable" {
		t.Errorf("first = %q, want the path the player can actually perform", paths[0].Path[0])
	}
}

func TestSortPathsPrefersCompletePaths(t *testing.T) {
	paths := []PathResult{
		{Path: []string{"incomplete"}, GPPerHour: 99999, Complete: false},
		{Path: []string{"complete"}, GPPerHour: 100, Complete: true},
	}
	sortPaths(paths)

	if paths[0].Path[0] != "complete" {
		t.Error("a complete path must outrank an incomplete one with a fictional margin")
	}
}

// Where a buy limit is known, the limited rate is the rate a trader
// would actually see, so it is what ranking should use.
func TestSortPathsUsesThroughputLimitedRate(t *testing.T) {
	paths := []PathResult{
		{Path: []string{"limited"}, GPPerHour: 1_000_000, Complete: true,
			Throughput: &Throughput{GPPerHour: 1000}},
		{Path: []string{"unlimited"}, GPPerHour: 50_000, Complete: true},
	}
	sortPaths(paths)

	if paths[0].Path[0] != "unlimited" {
		t.Errorf("first = %q; the limited path only yields 1000 GP/h in practice", paths[0].Path[0])
	}
}

// ---- helpers ----

func TestCeilDiv(t *testing.T) {
	tests := []struct{ a, b, want int }{
		{3, 2, 2}, {4, 2, 2}, {1, 4, 1}, {0, 3, 0}, {5, 0, 5}, {-1, 2, 0},
	}
	for _, tc := range tests {
		if got := ceilDiv(tc.a, tc.b); got != tc.want {
			t.Errorf("ceilDiv(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCollectItemIDsIsDeduplicatedAndSorted(t *testing.T) {
	child := &client.Node{Recipe: recipe("Bar", 200, 1, "Smithing", 15, 10, 100, input(300, "Ore", 1))}
	node := &client.Node{
		Recipe:   recipe("Widget", 100, 1, "Crafting", 10, 20, 100, input(200, "Bar", 1), input(0, "Unknown", 1)),
		Children: []*client.Node{child},
	}

	got := collectItemIDs(node)
	want := []int64{100, 200, 300}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v (zero IDs excluded, sorted, deduped)", got, want)
		}
	}
}

func TestMatchChildByIDThenName(t *testing.T) {
	byID := &client.Node{Recipe: recipe("Bar", 200, 1, "Smithing", 15, 10, 100)}
	node := &client.Node{Children: []*client.Node{byID}}

	if got := matchChild(node, input(200, "", 1)); got != byID {
		t.Error("should match on item ID")
	}
	if got := matchChild(node, input(0, "bar", 1)); got != byID {
		t.Error("should fall back to a case-insensitive name match")
	}
	if got := matchChild(node, input(999, "Nothing", 1)); got != nil {
		t.Error("should not match an unrelated input")
	}
}

func TestDedupeSortsAndRemovesDuplicates(t *testing.T) {
	got := dedupe([]string{"b", "a", "b", "c", "a"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if dedupe(nil) != nil {
		t.Error("dedupe(nil) should stay nil")
	}
}

// The cache key must change with the assumptions, or a spread or tax
// change silently serves numbers computed under the old model.
func TestCacheKeyVariesWithAssumptions(t *testing.T) {
	opts := Options{}
	base := Market{SpreadPct: 2, TaxPct: 2, TaxCapPerItem: 5_000_000}
	key := cacheKey(100, opts, base)

	if again := cacheKey(100, opts, base); again != key {
		t.Error("identical inputs must produce an identical key")
	}

	variants := map[string]Market{
		"spread": {SpreadPct: 3, TaxPct: 2, TaxCapPerItem: 5_000_000},
		"tax":    {SpreadPct: 2, TaxPct: 1, TaxCapPerItem: 5_000_000},
		"cap":    {SpreadPct: 2, TaxPct: 2, TaxCapPerItem: 1_000_000},
		// Every aph_source=default result is this number times the XP or
		// profit per action, so an operator retuning it must not keep
		// serving the old figures out of the cache.
		"default aph": {SpreadPct: 2, TaxPct: 2, TaxCapPerItem: 5_000_000, DefaultActionsPerHour: 600},
	}
	for name, m := range variants {
		if cacheKey(100, opts, m) == key {
			t.Errorf("changing %s must change the cache key", name)
		}
	}

	if cacheKey(100, Options{ActionsPerHourOverride: 900}, base) == key {
		t.Error("an actions-per-hour override must change the cache key")
	}
	if cacheKey(101, opts, base) == key {
		t.Error("a different item must change the cache key")
	}
}

func TestCacheKeyIgnoresPlayerNameCasing(t *testing.T) {
	m := Market{SpreadPct: 2}
	a := cacheKey(100, Options{Player: "Zezima"}, m)
	b := cacheKey(100, Options{Player: "zezima"}, m)
	if a != b {
		t.Error("player names differing only in case should share a cache entry")
	}
}

// A buy limit caps how often a *finished item* can be made, so it has
// to be compared against the rate the whole path completes at, not the
// root step's actions per hour. On a two-step path the finished item
// appears half as often as the root step runs, and comparing against
// the root rate made the "limited" figure come out at twice the
// unconstrained one — a cap that inflates is not a cap.
func TestThroughputCapIsTheWholePathRateNotTheRootStep(t *testing.T) {
	child := &client.Node{
		Recipe: recipe("Bar", 200, 1, "Smithing", 1, 10, 100, input(300, "Ore", 1)),
	}
	node := &client.Node{
		Recipe: recipe("Widget", 100, 1, "Crafting", 10, 20, 100,
			input(200, "Bar", 1),
			input(301, "Flux", 1),
		),
		Children: []*client.Node{child},
	}
	ev := &evaluator{
		market: noSpread(),
		prices: snapshots(map[int64]int64{100: 10000, 200: 1000, 300: 100, 301: 50}),
		// Generous enough that the limit itself never binds: what is
		// under test is which click rate it is compared against.
		limits: map[int64]int{301: 1_000_000},
	}

	p := twoStepPath(t, ev.expand(node))

	approx(t, p.TotalHours, 0.02, "total hours")   // 1/100 for each step
	approx(t, p.GPPerHour, 492_500, "gp per hour") // 9850 profit / 0.02
	approx(t, p.Throughput.CraftsPerHour, 50, "crafts per hour")
	if p.Throughput.GPPerHour > p.GPPerHour {
		t.Errorf("limited rate (%v) exceeds the unconstrained rate (%v)",
			p.Throughput.GPPerHour, p.GPPerHour)
	}
}

func twoStepPath(t *testing.T, paths []PathResult) PathResult {
	t.Helper()
	for _, p := range paths {
		if len(p.Steps) == 2 {
			return p
		}
	}
	t.Fatal("expected a path that crafts the child rather than buying it")
	return PathResult{}
}

// ---- finite figures ----

// The real Elder rune platebody + 5: 80 bars at an anvil, for a player
// who can forge them. ForgeTicks puts that at 10136 ticks, about 1.7
// hours per item, so the honest rate is a fraction of one an hour.
// Truncated to an integer it was zero, 1/aph was +Inf, json.Marshal
// refused the document, and gin answered 200 with an empty body.
func TestEightyBarForgeProducesAServeableResult(t *testing.T) {
	node := &client.Node{
		Recipe: models.Recipe{
			Name:           "Elder rune platebody + 5",
			OutputItemID:   45742,
			OutputItemName: "Elder rune platebody + 5",
			OutputQty:      1,
			Skill:          "Smithing",
			LevelReq:       90,
			XPPerAction:    80_000,
			Facility:       "Anvil",
			Inputs:         []models.RecipeInput{input(44830, "Elder rune bar", 80)},
		},
	}
	ev := &evaluator{
		market: MarketFrom(marketConfForTest()),
		prices: snapshots(map[int64]int64{45742: 50_000_000, 44830: 40_000}),
		opts:   Options{Player: "Zezima"},
		levels: map[string]int{"smithing": 90, "firemaking": 90},
	}

	paths := ev.expand(node)
	if len(paths) != 1 {
		t.Fatalf("got %d paths, want the single all-buy path", len(paths))
	}
	p := paths[0]

	if p.ActionsPerHour <= 0 {
		t.Fatalf("actions per hour = %v, want the fractional rate 80 bars really run at",
			p.ActionsPerHour)
	}
	if p.TotalHours < 1.5 || p.TotalHours > 2 {
		t.Errorf("total hours = %v, want ~1.7 for an 80-bar item", p.TotalHours)
	}
	if _, err := json.Marshal(p); err != nil {
		t.Fatalf("path does not serialise: %v", err)
	}
}

// Defence in depth for the same failure shape, independent of the
// arithmetic above: whatever produces it, a figure that is not finite
// must never reach the serialiser. A silent zero-byte 200 looks like
// success to every client and logs as success. Here the rate is zero
// because the house default was never configured — a different source
// of +Inf from the one the 80-bar case had.
func TestPathWithNonFiniteFiguresNeverReachesTheSerialiser(t *testing.T) {
	node := &client.Node{
		Recipe: recipe("Widget", 100, 1, "Crafting", 10, 20, 0, input(200, "Thing", 1)),
	}
	ev := &evaluator{
		market: Market{DefaultActionsPerHour: 0},
		prices: snapshots(map[int64]int64{100: 1000, 200: 10}),
	}

	for _, p := range ev.expand(node) {
		for label, v := range map[string]float64{
			"profit_per_craft": p.ProfitPerCraft, "gp_per_hour": p.GPPerHour,
			"xp_per_hour": p.XPPerHour, "gp_per_xp": p.GPPerXP,
			"roi_pct": p.ROIPct, "total_hours": p.TotalHours,
			"total_xp": p.TotalXP, "buy_cost": p.BuyCost,
			"sell_revenue": p.SellRevenue, "tax_paid": p.TaxPaid,
		} {
			if math.IsInf(v, 0) || math.IsNaN(v) {
				t.Errorf("%s = %v on a path handed to the client", label, v)
			}
		}
		if _, err := json.Marshal(p); err != nil {
			t.Fatalf("path does not serialise: %v", err)
		}
	}
}
