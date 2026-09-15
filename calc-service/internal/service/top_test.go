package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	calccache "github.com/rs3-market/backend/calc-service/internal/cache"
	"github.com/rs3-market/backend/calc-service/internal/client"
	sharedcache "github.com/rs3-market/backend/shared/cache"
)

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

func TestParseMetricAcceptsEveryRankedMetric(t *testing.T) {
	for _, s := range []string{"xp_per_hour", "gp_per_hour", "gp_per_xp"} {
		if _, err := ParseMetric(s); err != nil {
			t.Errorf("ParseMetric(%q) failed: %v", s, err)
		}
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

// betterRow is what picks the best path for a single item before rows
// ever reach sortTop, so it has to agree with sortTop's ordering for
// every metric or the wrong path wins before the wrong item does.
func TestBetterRowAgreesWithEachMetric(t *testing.T) {
	cases := []struct {
		metric Metric
		a, b   TopRow
	}{
		{MetricXPPerHour, TopRow{XPPerHour: 500}, TopRow{XPPerHour: 100}},
		{MetricGPPerXP, TopRow{GPPerXP: 9.5}, TopRow{GPPerXP: 2.5}},
		{MetricGPPerHour, TopRow{GPPerHourLimited: 21_000_000}, TopRow{GPPerHourLimited: 2_000_000}},
	}
	for _, c := range cases {
		if !betterRow(c.a, c.b, c.metric) {
			t.Errorf("betterRow(%s): expected the first row to win", c.metric)
		}
		if betterRow(c.b, c.a, c.metric) {
			t.Errorf("betterRow(%s): expected the second row to lose", c.metric)
		}
	}
}

// rowFromPath is the only hand-written mapping in this file; everything
// else is generic. The root recipe's name, skill and level requirement
// live on the last step, appended after every child step (see
// evaluator.evaluate) — not on the first one.
func TestRowFromPathMapsRootStepAndRates(t *testing.T) {
	p := PathResult{
		Steps: []Step{
			{Recipe: "Smelt bronze bar", Skill: "Smithing", LevelReq: 1},
			{Recipe: "Bronze bar", Skill: "Smithing", LevelReq: 1},
		},
		XPPerHour:      1234.5,
		GPPerHour:      100,
		GPPerXP:        2.5,
		ActionsPerHour: 900,
		APHSource:      "ticks",
		Complete:       true,
	}

	row := rowFromPath(42, p)

	if row.ItemID != 42 {
		t.Errorf("item id = %d, want 42", row.ItemID)
	}
	if row.Name != "Bronze bar" || row.Skill != "Smithing" || row.LevelReq != 1 {
		t.Errorf("root step fields = %+v, want the last step's", row)
	}
	if row.XPPerHour != 1234.5 || row.GPPerXP != 2.5 || row.ActionsPerHour != 900 || row.APHSource != "ticks" {
		t.Errorf("rate fields not carried through: %+v", row)
	}
	if !row.Complete {
		t.Error("complete should be carried through")
	}
}

// Without a throughput figure the raw rate is the only signal available,
// so it must still be usable as the limited rate rather than zero.
func TestRowFromPathFallsBackToRawGPPerHourWithoutThroughput(t *testing.T) {
	p := PathResult{GPPerHour: 500, Throughput: nil}
	row := rowFromPath(1, p)
	if row.GPPerHourLimited != 500 {
		t.Errorf("gp_per_hour_limited = %v, want the raw 500 as a fallback", row.GPPerHourLimited)
	}
}

func TestRowFromPathUsesThroughputWhenPresent(t *testing.T) {
	p := PathResult{
		GPPerHour:  900_000_000,
		Throughput: &Throughput{GPPerHour: 2_000_000, BindingItemName: "Rune bar"},
	}
	row := rowFromPath(1, p)
	if row.GPPerHourLimited != 2_000_000 {
		t.Errorf("gp_per_hour_limited = %v, want the throughput-limited 2000000", row.GPPerHourLimited)
	}
	if row.BindingItemName != "Rune bar" {
		t.Errorf("binding item name = %q, want Rune bar", row.BindingItemName)
	}
}

// unreachableCache backs a Service with a Redis that is not there:
// every Get misses, every Set is dropped. Enough to drive the code
// around the cache without standing one up.
func unreachableCache() *calccache.CalcCache {
	rdb := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 50 * time.Millisecond,
	})
	return calccache.New(&sharedcache.Cache{RDB: rdb, Ctx: context.Background()})
}

// A dead request must not produce a ranking. Once the context is done
// every remaining item fails instantly, so swallowing those failures
// the way a pricing miss is swallowed completes the walk and caches a
// truncated ranking under the full parameter key — which is then served
// to everyone else for the rest of the TTL.
func TestTopAbortsWhenTheRequestContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/recipes/ids" {
			_, _ = w.Write([]byte(`{"item_ids":[1,2,3]}`))
			return
		}
		// The caller hangs up part-way through the walk.
		cancel()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := New(zap.NewNop(),
		client.NewRecipeClient(srv.URL), client.NewPriceClient(srv.URL),
		client.NewHiscoreClient(srv.URL), unreachableCache(),
		Market{DefaultActionsPerHour: 600})

	res, err := svc.Top(ctx, TopOptions{Metric: MetricGPPerHour, Limit: 10})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Top returned (%+v, %v); want context.Canceled rather than a ranking built from an aborted walk", res, err)
	}
}

func boolPtr(b bool) *bool { return &b }

// betterRow picks which path represents an item, so it has to use the
// same precedence sortPaths uses on /calc/{itemID}: a path the player
// cannot perform is not the item's best path however fast it is. Two
// routes disagreeing about "best path" is how the leaderboard ended up
// representing items by paths the sibling route had already ranked last.
func TestRowOrderingPutsAchievableAndCompleteFirst(t *testing.T) {
	fast := TopRow{ItemID: 1, XPPerHour: 1_000_000, Complete: true,
		MeetsRequirements: boolPtr(false)}
	slow := TopRow{ItemID: 2, XPPerHour: 1, Complete: true,
		MeetsRequirements: boolPtr(true)}

	if !betterRow(slow, fast, MetricXPPerHour) {
		t.Error("a path the player can perform beats a faster one they cannot")
	}

	upperBound := TopRow{ItemID: 3, XPPerHour: 1_000_000, Complete: false}
	priced := TopRow{ItemID: 4, XPPerHour: 1, Complete: true}
	if !betterRow(priced, upperBound, MetricXPPerHour) {
		t.Error("a complete path beats an incomplete one whose figures are upper bounds")
	}
}

// Unachievable rows are ordered last, not dropped: a caller browsing
// without a player must still see the whole catalogue.
func TestSortTopKeepsUnachievableRowsBelowAchievableOnes(t *testing.T) {
	rows := []TopRow{
		{ItemID: 1, GPPerXP: 900, MeetsRequirements: boolPtr(false)},
		{ItemID: 2, GPPerXP: 5, MeetsRequirements: boolPtr(true)},
	}
	sortTop(rows, MetricGPPerXP)

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want both kept", len(rows))
	}
	if rows[0].ItemID != 2 {
		t.Errorf("top row is item %d, want the achievable one", rows[0].ItemID)
	}
}

// Rows are collected out of a map, so without an explicit tiebreak
// sort.SliceStable preserves a randomized order: two identical requests
// return different items at the limit boundary.
func TestSortTopBreaksTiesOnItemID(t *testing.T) {
	for i := 0; i < 20; i++ {
		rows := []TopRow{
			{ItemID: 7, XPPerHour: 100},
			{ItemID: 3, XPPerHour: 100},
			{ItemID: 5, XPPerHour: 100},
		}
		sortTop(rows, MetricXPPerHour)
		for j, want := range []int64{3, 5, 7} {
			if rows[j].ItemID != want {
				t.Fatalf("tied rows ordered %v, want ascending item id", rows)
			}
		}
	}
}

// Achievability is carried from the path applySkillRequirements already
// annotated, not recomputed: the ranking must say exactly what
// /calc/{itemID} says about the same path.
func TestRowFromPathCarriesMeetsRequirements(t *testing.T) {
	row := rowFromPath(1, PathResult{MeetsRequirements: boolPtr(false)})
	if row.MeetsRequirements == nil || *row.MeetsRequirements {
		t.Errorf("meets_requirements = %v, want the path's false", row.MeetsRequirements)
	}
	if got := rowFromPath(1, PathResult{}).MeetsRequirements; got != nil {
		t.Errorf("meets_requirements = %v, want nil with no player", got)
	}
}

// The filter arrives from a query string, where "smithing" and
// "Smithing" are the same request.
func TestMatchesSkillFiltersCaseInsensitively(t *testing.T) {
	row := TopRow{Skill: "Smithing"}
	if !matchesSkill(row, "") {
		t.Error("an empty filter must keep every row")
	}
	if !matchesSkill(row, "smithing") {
		t.Error("the skill filter must be case-insensitive")
	}
	if matchesSkill(row, "Crafting") {
		t.Error("a row from another skill must be filtered out")
	}
}

// A leaderboard is read as a list of things to go and do, so a recipe
// the player's levels refuse is not worth a place in it — the more so
// because those are the recipes whose rate falls back to the house
// default, the level gate that refuses them being the same one that
// refuses to derive a tick cost.
func TestRankableDropsWhatThePlayerCannotDo(t *testing.T) {
	yes, no := true, false

	if rankable(TopRow{Skill: "Smithing", MeetsRequirements: &no}, "") {
		t.Error("a recipe above the player's level must not be ranked")
	}
	if !rankable(TopRow{Skill: "Smithing", MeetsRequirements: &yes}, "") {
		t.Error("a recipe the player can perform must be ranked")
	}
	if !rankable(TopRow{Skill: "Smithing"}, "") {
		t.Error("with no player there is nothing to compare against, " +
			"so the whole catalogue must still rank")
	}
	if rankable(TopRow{Skill: "Smithing", MeetsRequirements: &yes}, "Crafting") {
		t.Error("the skill filter must still apply to an achievable row")
	}
}

// A ranking cached under a key that omits an input is served to a
// request that asked for something else. The per-item key already mixes
// in the resolved market; keying only on the caller's spread override
// let an operator change tax or the banking model and invalidate every
// per-item entry while every ranking kept answering from the old model.
func TestTopCacheKeyVariesWithEveryInput(t *testing.T) {
	base := TopOptions{Metric: MetricGPPerHour, Skill: "Smithing", Limit: 20}
	market := Market{SpreadPct: 2, TaxPct: 2, TaxCapPerItem: 5_000_000,
		TaxExemptBelow: 100, InventorySlots: 28, BankTripTicks: 21}
	key := topCacheKey(base, market)

	if again := topCacheKey(base, market); again != key {
		t.Error("identical inputs must produce an identical key")
	}

	spread := 4.0
	optVariants := map[string]TopOptions{
		"metric":             {Metric: MetricXPPerHour, Skill: "Smithing", Limit: 20},
		"skill":              {Metric: MetricGPPerHour, Skill: "Crafting", Limit: 20},
		"limit":              {Metric: MetricGPPerHour, Skill: "Smithing", Limit: 21},
		"aph override":       withCalc(base, Options{ActionsPerHourOverride: 900}),
		"player":             withCalc(base, Options{Player: "Zezima"}),
		"mode":               withCalc(base, Options{Mode: "ironman"}),
		"include_incomplete": withCalc(base, Options{IncludeIncomplete: true}),
		"boosts":             withCalc(base, Options{BoostsRaw: "juju"}),
		"spread override":    withCalc(base, Options{SpreadPctOverride: &spread}),
	}
	for name, o := range optVariants {
		if o.Calc.SpreadPctOverride != nil {
			// The override reaches the key through the resolved market,
			// the same way it reaches the numbers.
			if topCacheKey(o, Market{SpreadPct: spread}) == key {
				t.Errorf("changing %s must change the cache key", name)
			}
			continue
		}
		if topCacheKey(o, market) == key {
			t.Errorf("changing %s must change the cache key", name)
		}
	}

	marketVariants := map[string]Market{
		"spread":     {SpreadPct: 3, TaxPct: 2, TaxCapPerItem: 5_000_000, TaxExemptBelow: 100, InventorySlots: 28, BankTripTicks: 21},
		"tax":        {SpreadPct: 2, TaxPct: 1, TaxCapPerItem: 5_000_000, TaxExemptBelow: 100, InventorySlots: 28, BankTripTicks: 21},
		"tax cap":    {SpreadPct: 2, TaxPct: 2, TaxCapPerItem: 1_000_000, TaxExemptBelow: 100, InventorySlots: 28, BankTripTicks: 21},
		"tax exempt": {SpreadPct: 2, TaxPct: 2, TaxCapPerItem: 5_000_000, TaxExemptBelow: 50, InventorySlots: 28, BankTripTicks: 21},
		"inventory":  {SpreadPct: 2, TaxPct: 2, TaxCapPerItem: 5_000_000, TaxExemptBelow: 100, InventorySlots: 30, BankTripTicks: 21},
		"bank trip":  {SpreadPct: 2, TaxPct: 2, TaxCapPerItem: 5_000_000, TaxExemptBelow: 100, InventorySlots: 28, BankTripTicks: 30},
		// The house fallback rate is the number every aph_source=default
		// row is built from, so an operator changing it must not be
		// served the old ranking for the rest of the TTL.
		"default aph": {SpreadPct: 2, TaxPct: 2, TaxCapPerItem: 5_000_000, TaxExemptBelow: 100, InventorySlots: 28, BankTripTicks: 21, DefaultActionsPerHour: 600},
	}
	for name, m := range marketVariants {
		if topCacheKey(base, m) == key {
			t.Errorf("changing the market's %s must change the cache key", name)
		}
	}
}

func TestTopCacheKeyIgnoresCasing(t *testing.T) {
	m := Market{SpreadPct: 2}
	upper := TopOptions{Metric: MetricGPPerHour, Skill: "Smithing",
		Calc: Options{Player: "Zezima", BoostsRaw: "Juju"}}
	lower := TopOptions{Metric: MetricGPPerHour, Skill: "smithing",
		Calc: Options{Player: "zezima", BoostsRaw: "juju"}}

	if topCacheKey(upper, m) != topCacheKey(lower, m) {
		t.Error("names and skills differing only in case should share a cache entry")
	}
}

func withCalc(base TopOptions, calc Options) TopOptions {
	base.Calc = calc
	return base
}
