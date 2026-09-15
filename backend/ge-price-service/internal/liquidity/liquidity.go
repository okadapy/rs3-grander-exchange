// Package liquidity turns a window of stored price snapshots into a
// judgement about whether an item can actually be traded at size.
//
// This exists because a margin alone is not a trading signal. A craft
// showing 5M GP/h on an item that trades twice a day is indistinguishable
// — in a plain profit listing — from one on an item that moves millions
// of units a day. The first is noise, the second is a business. Every
// number here is derived from data we already store, so it costs no
// extra upstream calls.
package liquidity

import (
	"math"
	"sort"
	"time"

	"github.com/rs3-market/backend/shared/models"
)

// Tier labels for coarse filtering in the UI.
const (
	TierHigh    = "high"
	TierMedium  = "medium"
	TierLow     = "low"
	TierUnknown = "unknown"
)

// Stats summarises one item's recent price behaviour.
type Stats struct {
	ItemID int64 `json:"item_id"`

	// Observations is how many snapshots the window contained. A score
	// built from one or two samples is not worth much, so the frontend
	// should surface this alongside the score.
	Observations   int `json:"observations"`
	DistinctPrices int `json:"distinct_prices"`

	PriceMin  int64   `json:"price_min"`
	PriceMax  int64   `json:"price_max"`
	PriceAvg  float64 `json:"price_avg"`
	PriceLast int64   `json:"price_last"`

	// VolatilityPct is the peak-to-trough swing over the window as a
	// percentage of the mean price.
	VolatilityPct float64 `json:"volatility_pct"`

	VolumeAvg  float64 `json:"volume_avg"`
	VolumeLast int64   `json:"volume_last"`

	// LastChangeAt is when the price last actually moved — not when we
	// last polled. A price that has not moved in two weeks is a price
	// nobody is trading at.
	LastChangeAt  *time.Time `json:"last_change_at"`
	StaleForHours float64    `json:"stale_for_hours"`

	// Score is 0-100; Tier buckets it for filtering.
	Score int    `json:"score"`
	Tier  string `json:"tier"`

	// BuyLimit4h is the Grand Exchange 4-hour buy limit, and
	// MaxGPPerHourAtLimit is the ceiling that limit puts on any strategy
	// with the given per-unit margin. Both are zero when unknown.
	BuyLimit4h int `json:"buy_limit_4h"`
}

// WindowFor is the default lookback used when a caller does not specify.
const WindowFor = 14 * 24 * time.Hour

// Compute derives Stats from snapshots. Snapshots need not be sorted.
// `now` is passed explicitly so the result is deterministic in tests.
//
// An empty input yields a zero-value Stats with TierUnknown rather than
// an error: "we have no data on this item" is a normal answer that the
// frontend must render, not an exceptional one.
func Compute(itemID int64, snaps []models.PriceSnapshot, buyLimit4h int, now time.Time) Stats {
	st := Stats{ItemID: itemID, Tier: TierUnknown, BuyLimit4h: buyLimit4h}
	if len(snaps) == 0 {
		return st
	}

	ordered := make([]models.PriceSnapshot, len(snaps))
	copy(ordered, snaps)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Timestamp.Before(ordered[j].Timestamp)
	})

	var (
		sumPrice   float64
		sumVolume  float64
		distinct   = map[int64]struct{}{}
		lastChange *time.Time
	)
	st.PriceMin = ordered[0].Price
	st.PriceMax = ordered[0].Price

	for i, s := range ordered {
		if s.Price < st.PriceMin {
			st.PriceMin = s.Price
		}
		if s.Price > st.PriceMax {
			st.PriceMax = s.Price
		}
		sumPrice += float64(s.Price)
		sumVolume += float64(s.Volume)
		distinct[s.Price] = struct{}{}

		if i > 0 && s.Price != ordered[i-1].Price {
			ts := s.Timestamp
			lastChange = &ts
		}
	}

	n := len(ordered)
	st.Observations = n
	st.DistinctPrices = len(distinct)
	st.PriceAvg = sumPrice / float64(n)
	st.VolumeAvg = sumVolume / float64(n)
	st.PriceLast = ordered[n-1].Price
	st.VolumeLast = ordered[n-1].Volume
	st.LastChangeAt = lastChange

	if st.PriceAvg > 0 {
		st.VolatilityPct = float64(st.PriceMax-st.PriceMin) / st.PriceAvg * 100
	}

	// Staleness is measured from the last observed price *move*. With no
	// move recorded in the window, fall back to the window's own span so
	// a flat item is reported as at least as stale as our history is long.
	ref := ordered[0].Timestamp
	if lastChange != nil {
		ref = *lastChange
	}
	if h := now.Sub(ref).Hours(); h > 0 {
		st.StaleForHours = h
	}

	st.Score = score(st)
	st.Tier = tierFor(st.Score)
	return st
}

// score blends three independent signals into 0-100.
//
//   - volume (60%): the dominant signal, and the only one sourced from
//     the exchange rather than inferred. Log-scaled because the gap
//     between 10 and 1,000 units/day matters far more than the gap
//     between 100,000 and 1,000,000.
//   - freshness (25%): how recently the price moved at all.
//   - activity (15%): how often it moves within the window, which
//     separates a genuinely traded item from one that reprices once.
//
// The weights are a judgement call, not a fitted model. They are here to
// rank items against each other, not to be read as a probability.
func score(st Stats) int {
	if st.Observations == 0 {
		return 0
	}

	// 1e6 average units and above saturates the volume component.
	volumeScore := math.Log10(st.VolumeAvg+1) / 6
	volumeScore = clamp01(volumeScore)

	// Full marks inside 48h, decaying linearly to zero at 14 days.
	const freshHours, deadHours = 48.0, 14 * 24.0
	freshness := 1.0
	if st.StaleForHours > freshHours {
		freshness = 1 - (st.StaleForHours-freshHours)/(deadHours-freshHours)
	}
	freshness = clamp01(freshness)

	activity := 0.0
	if st.Observations > 1 {
		activity = float64(st.DistinctPrices-1) / float64(st.Observations-1)
	}
	activity = clamp01(activity)

	raw := 0.60*volumeScore + 0.25*freshness + 0.15*activity
	return int(math.Round(clamp01(raw) * 100))
}

func tierFor(score int) string {
	switch {
	case score >= 66:
		return TierHigh
	case score >= 33:
		return TierMedium
	default:
		return TierLow
	}
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
