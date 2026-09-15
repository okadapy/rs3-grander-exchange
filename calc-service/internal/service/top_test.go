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
