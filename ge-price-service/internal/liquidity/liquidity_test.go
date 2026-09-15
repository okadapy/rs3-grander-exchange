package liquidity

import (
	"math"
	"testing"
	"time"

	"github.com/rs3-market/backend/shared/models"
)

var now = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

func snap(hoursAgo int, price, volume int64) models.PriceSnapshot {
	return models.PriceSnapshot{
		ItemID:    1,
		Timestamp: now.Add(-time.Duration(hoursAgo) * time.Hour),
		Price:     price,
		Volume:    volume,
	}
}

// "We have no data" is a normal answer the frontend must render, not an
// error and not a zero score presented as a real measurement.
func TestComputeWithNoDataIsUnknown(t *testing.T) {
	got := Compute(1, nil, 0, now)

	if got.Tier != TierUnknown {
		t.Errorf("tier = %q, want %q", got.Tier, TierUnknown)
	}
	if got.Observations != 0 {
		t.Errorf("observations = %d, want 0", got.Observations)
	}
	if got.ItemID != 1 {
		t.Errorf("item_id = %d, want 1", got.ItemID)
	}
}

func TestComputeBasicStatistics(t *testing.T) {
	snaps := []models.PriceSnapshot{
		snap(72, 100, 1000),
		snap(48, 120, 2000),
		snap(24, 110, 3000),
	}
	got := Compute(1, snaps, 0, now)

	if got.Observations != 3 {
		t.Errorf("observations = %d, want 3", got.Observations)
	}
	if got.DistinctPrices != 3 {
		t.Errorf("distinct prices = %d, want 3", got.DistinctPrices)
	}
	if got.PriceMin != 100 || got.PriceMax != 120 {
		t.Errorf("range = %d..%d, want 100..120", got.PriceMin, got.PriceMax)
	}
	if got.PriceLast != 110 {
		t.Errorf("last price = %d, want 110 (the newest, not the last in the slice)", got.PriceLast)
	}
	if got.VolumeLast != 3000 {
		t.Errorf("last volume = %d, want 3000", got.VolumeLast)
	}
	if math.Abs(got.PriceAvg-110) > 1e-9 {
		t.Errorf("average price = %v, want 110", got.PriceAvg)
	}
	if math.Abs(got.VolumeAvg-2000) > 1e-9 {
		t.Errorf("average volume = %v, want 2000", got.VolumeAvg)
	}
	// (120 - 100) / 110 * 100
	if math.Abs(got.VolatilityPct-18.1818181818) > 1e-6 {
		t.Errorf("volatility = %v, want ~18.18", got.VolatilityPct)
	}
}

func TestComputeSortsUnorderedInput(t *testing.T) {
	unordered := []models.PriceSnapshot{snap(24, 110, 1), snap(72, 100, 1), snap(48, 120, 1)}
	got := Compute(1, unordered, 0, now)

	if got.PriceLast != 110 {
		t.Errorf("last price = %d, want 110; input order must not matter", got.PriceLast)
	}
}

// Staleness must be measured from the last price *move*, not the last
// poll. A price we re-record hourly but that never changes is not fresh.
func TestLastChangeTracksMovesNotPolls(t *testing.T) {
	snaps := []models.PriceSnapshot{
		snap(96, 100, 1),
		snap(72, 150, 1), // the move
		snap(48, 150, 1),
		snap(24, 150, 1),
		snap(1, 150, 1),
	}
	got := Compute(1, snaps, 0, now)

	if got.LastChangeAt == nil {
		t.Fatal("expected a recorded price change")
	}
	if !got.LastChangeAt.Equal(now.Add(-72 * time.Hour)) {
		t.Errorf("last change = %v, want 72h ago", got.LastChangeAt)
	}
	if math.Abs(got.StaleForHours-72) > 1e-9 {
		t.Errorf("stale for %v hours, want 72", got.StaleForHours)
	}
}

func TestFlatPriceHasNoRecordedChange(t *testing.T) {
	snaps := []models.PriceSnapshot{snap(96, 100, 1), snap(48, 100, 1), snap(1, 100, 1)}
	got := Compute(1, snaps, 0, now)

	if got.LastChangeAt != nil {
		t.Errorf("last change = %v, want nil for a price that never moved", got.LastChangeAt)
	}
	// Falls back to the span of the window, so a flat item reads as at
	// least as stale as our history is long.
	if math.Abs(got.StaleForHours-96) > 1e-9 {
		t.Errorf("stale for %v hours, want 96", got.StaleForHours)
	}
}

// The whole point of the score: separate an item that genuinely trades
// from one carrying the same margin on no volume.
func TestHighVolumeScoresAboveLowVolume(t *testing.T) {
	fresh := func(vol int64) []models.PriceSnapshot {
		return []models.PriceSnapshot{
			snap(30, 100, vol), snap(20, 105, vol), snap(10, 110, vol), snap(1, 108, vol),
		}
	}
	busy := Compute(1, fresh(3_000_000), 0, now)
	quiet := Compute(1, fresh(5), 0, now)

	if busy.Score <= quiet.Score {
		t.Errorf("busy item scored %d, quiet item %d; volume must dominate",
			busy.Score, quiet.Score)
	}
	if busy.Tier != TierHigh {
		t.Errorf("a 3M-volume, actively moving item should be %q, got %q", TierHigh, busy.Tier)
	}
}

func TestStalePriceScoresBelowFreshPrice(t *testing.T) {
	const vol = 100_000
	freshSnaps := []models.PriceSnapshot{snap(30, 100, vol), snap(20, 105, vol), snap(2, 110, vol)}
	staleSnaps := []models.PriceSnapshot{snap(330, 100, vol), snap(320, 105, vol), snap(310, 110, vol)}

	fresh := Compute(1, freshSnaps, 0, now)
	stale := Compute(1, staleSnaps, 0, now)

	if stale.Score >= fresh.Score {
		t.Errorf("stale scored %d, fresh scored %d; recency must count",
			stale.Score, fresh.Score)
	}
}

func TestScoreIsBounded(t *testing.T) {
	cases := map[string][]models.PriceSnapshot{
		"huge volume":  {snap(1, 100, math.MaxInt32), snap(0, 200, math.MaxInt32)},
		"zero volume":  {snap(400, 100, 0), snap(390, 100, 0)},
		"single point": {snap(1, 100, 50)},
	}
	for name, snaps := range cases {
		got := Compute(1, snaps, 0, now)
		if got.Score < 0 || got.Score > 100 {
			t.Errorf("%s: score = %d, want 0..100", name, got.Score)
		}
		if got.Tier == "" {
			t.Errorf("%s: tier must always be set", name)
		}
	}
}

func TestTierThresholds(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{100, TierHigh}, {66, TierHigh}, {65, TierMedium},
		{33, TierMedium}, {32, TierLow}, {0, TierLow},
	}
	for _, tc := range tests {
		if got := tierFor(tc.score); got != tc.want {
			t.Errorf("tierFor(%d) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestBuyLimitPassedThrough(t *testing.T) {
	got := Compute(1, []models.PriceSnapshot{snap(1, 100, 10)}, 25000, now)
	if got.BuyLimit4h != 25000 {
		t.Errorf("buy limit = %d, want 25000", got.BuyLimit4h)
	}
}

func TestComputeDoesNotMutateInput(t *testing.T) {
	snaps := []models.PriceSnapshot{snap(1, 110, 1), snap(72, 100, 1)}
	Compute(1, snaps, 0, now)

	if snaps[0].Price != 110 {
		t.Error("Compute must not reorder the caller's slice")
	}
}

func TestClamp01(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{-1, 0}, {0, 0}, {0.5, 0.5}, {1, 1}, {2, 1}, {math.NaN(), 0},
	}
	for _, tc := range tests {
		if got := clamp01(tc.in); got != tc.want {
			t.Errorf("clamp01(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
