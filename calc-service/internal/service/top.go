package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
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
// reporting, not a silent fallback to one particular metric.
func ParseMetric(s string) (Metric, error) {
	switch Metric(strings.TrimSpace(s)) {
	case MetricXPPerHour:
		return MetricXPPerHour, nil
	case MetricGPPerHour:
		return MetricGPPerHour, nil
	case MetricGPPerXP:
		return MetricGPPerXP, nil
	}
	return "", fmt.Errorf("unknown metric %q: want one of xp_per_hour, gp_per_hour, gp_per_xp", s)
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

// TopRow is one ranked production path: the best path for its item
// under the requested metric.
type TopRow struct {
	ItemID    int64   `json:"item_id"`
	Name      string  `json:"name"`
	Skill     string  `json:"skill"`
	LevelReq  int     `json:"level_req"`
	XPPerHour float64 `json:"xp_per_hour"`
	GPPerHour float64 `json:"gp_per_hour"`
	// GPPerHourLimited is the rate the ranking actually sorts on for
	// gp_per_hour: GPPerHour capped by the binding input's buy limit
	// when one is known (see effectiveGPPerHour), and GPPerHour itself
	// otherwise.
	GPPerHourLimited float64 `json:"gp_per_hour_limited"`
	GPPerXP          float64 `json:"gp_per_xp"`
	ActionsPerHour   int     `json:"actions_per_hour"`
	APHSource        string  `json:"aph_source"`
	Complete         bool    `json:"complete"`
	BindingItemName  string  `json:"binding_item_name,omitempty"`

	// MeetsRequirements is nil unless a player was supplied, exactly as
	// on PathResult: with no hiscore data "you qualify" would be a
	// guess presented as fact. It is carried straight from the path
	// applySkillRequirements already annotated.
	MeetsRequirements *bool `json:"meets_requirements,omitempty"`
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

// sortTop orders rows best-first: see rowLess for the precedence.
//
// The money metric sorts on GPPerHourLimited, the buy-limit-bound rate,
// not GPPerHour. The raw figure multiplies one craft's profit by a full
// hour of actions, which floats items nobody can produce at that rate —
// a godsword nobody forges six hundred times an hour — above the ones
// people actually make.
func sortTop(rows []TopRow, m Metric) {
	sort.SliceStable(rows, func(i, j int) bool {
		return rowLess(rows[j], rows[i], m)
	})
}

// betterRow reports whether a beats b under m. Used both by sortTop
// (via rowLess) and while picking the single best path per item, so
// both stages of ranking agree on what "better" means for each metric.
func betterRow(a, b TopRow, m Metric) bool {
	return rowLess(b, a, m)
}

// rowLess is the whole ordering, and it is deliberately the same
// precedence sortPaths uses on /calc/{itemID}: a path the player cannot
// perform is not a better answer than one they can, whatever its rate,
// and neither is an incomplete path whose money figures are upper
// bounds. Two routes disagreeing about which path represents an item
// was a defect in its own right.
//
// Achievability is inert without a player — MeetsRequirements is nil on
// every row then, so every row counts as achievable — which is why
// there is no explicit check on opts.Player here. Unachievable rows are
// ordered last rather than dropped: a caller browsing without a player
// must still see the whole catalogue.
//
// ItemID breaks the final tie. Rows are collected out of a map, so
// without it sort.SliceStable preserves a randomized order and equal
// values change places between two identical requests — visible at the
// limit boundary, where it changes which item is returned at all.
func rowLess(a, b TopRow, m Metric) bool {
	if am, bm := meetsRow(a), meetsRow(b); am != bm {
		return !am
	}
	if a.Complete != b.Complete {
		return !a.Complete
	}
	if av, bv := metricValue(a, m), metricValue(b, m); av != bv {
		return av < bv
	}
	return a.ItemID > b.ItemID
}

func metricValue(r TopRow, m Metric) float64 {
	switch m {
	case MetricXPPerHour:
		return r.XPPerHour
	case MetricGPPerXP:
		return r.GPPerXP
	default: // MetricGPPerHour
		return r.GPPerHourLimited
	}
}

func meetsRow(r TopRow) bool {
	return r.MeetsRequirements == nil || *r.MeetsRequirements
}

// matchesSkill applies the skill filter. Case-insensitive because the
// filter arrives from a query string, where "smithing" and "Smithing"
// are the same request.
func matchesSkill(r TopRow, skill string) bool {
	return skill == "" || strings.EqualFold(r.Skill, skill)
}

// rowFromPath maps one evaluated PathResult onto a ranked TopRow.
//
// The root recipe's own name, skill and level requirement live on the
// last entry of Steps: evaluator.evaluate appends the parent's own step
// after every child step it depends on, so Steps[last] is always the
// item being ranked rather than one of its ingredients.
func rowFromPath(itemID int64, p PathResult) TopRow {
	row := TopRow{
		ItemID:           itemID,
		XPPerHour:        p.XPPerHour,
		GPPerHour:        p.GPPerHour,
		GPPerHourLimited: effectiveGPPerHour(p),
		GPPerXP:          p.GPPerXP,
		ActionsPerHour:   p.ActionsPerHour,
		APHSource:        p.APHSource,
		Complete:         p.Complete,
	}
	if n := len(p.Steps); n > 0 {
		root := p.Steps[n-1]
		row.Name = root.Recipe
		row.Skill = root.Skill
		row.LevelReq = root.LevelReq
	}
	if p.Throughput != nil {
		row.BindingItemName = p.Throughput.BindingItemName
	}
	row.MeetsRequirements = p.MeetsRequirements
	return row
}

// Top ranks every priceable recipe by one metric.
//
// It walks the whole catalogue, so it answers from the cache on
// anything but the first call for a given parameter set.
func (s *Service) Top(ctx context.Context, opts TopOptions) (TopResponse, error) {
	opts.Limit = clampLimit(opts.Limit)

	market := s.resolveMarket(opts.Calc)
	key := topCacheKey(opts, market)
	var cached TopResponse
	if s.cc.Get(key, &cached) {
		return cached, nil
	}

	// The skill filter goes upstream: /recipes/ids applies it in SQL,
	// so a single-skill ranking evaluates that skill's recipes instead
	// of all 5800 and discarding most of them. The per-row check below
	// still stands — an item can be produced by more than one recipe.
	ids, err := s.recipe.AllItemIDs(ctx, opts.Skill)
	if err != nil {
		return TopResponse{}, fmt.Errorf("list item ids: %w", err)
	}

	// Resolved once for the whole walk rather than per item: with a
	// player set, the per-item lookup meant one hiscore round-trip per
	// catalogue entry.
	levels := s.playerLevelsOrNil(ctx, opts.Calc)
	levelsOf := func() map[string]int { return levels }

	best := make(map[int64]TopRow, len(ids))
	for _, id := range ids {
		// A dead request must not produce a ranking. Every item left in
		// the walk would fail instantly, and the truncated result would
		// be cached under the full parameter key and served to everyone
		// else for the rest of the TTL.
		if err := ctx.Err(); err != nil {
			return TopResponse{}, err
		}

		res, err := s.calculate(ctx, id, opts.Calc, levelsOf)
		if err != nil {
			// One unpriceable item must not fail the whole ranking. It
			// is simply absent from the results, the same reasoning
			// that makes /calc/batch report failures per entry rather
			// than failing the batch.
			continue
		}
		for _, p := range res.Paths {
			row := rowFromPath(id, p)
			if !matchesSkill(row, opts.Skill) {
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
		Metric: opts.Metric,
		Count:  len(rows),
		Rows:   rows,
		// Built directly from the resolved market and the caller's
		// options rather than borrowed from whichever item happened to
		// be evaluated last: assumptionsFor depends on neither an item
		// nor its price data, so every item's Assumptions block under
		// these options is identical, and computing it up front holds
		// even when every item in the catalogue fails to price.
		Assumptions: assumptionsFor(market, opts.Calc),
	}
	s.cc.Set(key, out)
	return out, nil
}

// topCacheKey keys on the full parameter set: the ranking metric, the
// skill filter and limit, every Options field that changes the price
// computed for an item, and the resolved market rather than the
// caller's spread override. Missing one here would serve one player's
// or one boost stack's ranking to another's request; keying on the
// override rather than the resolved market let an operator change tax,
// inventory slots or bank trip ticks and invalidate every per-item
// entry (cacheKey mixes them in) while every ranking kept answering
// from the old model.
func topCacheKey(opts TopOptions, m Market) string {
	h := sha1.New()
	fmt.Fprintf(h, "metric=%s|skill=%s|limit=%d|aph=%d|p=%s|m=%s|inc=%v|boosts=%s|spread=%.4f|tax=%.4f|cap=%d|ex=%d|inv=%d|bank=%d|defaph=%d|%s",
		opts.Metric, strings.ToLower(opts.Skill), opts.Limit,
		opts.Calc.ActionsPerHourOverride, strings.ToLower(opts.Calc.Player), opts.Calc.Mode,
		opts.Calc.IncludeIncomplete, strings.ToLower(opts.Calc.BoostsRaw),
		m.SpreadPct, m.TaxPct, m.TaxCapPerItem, m.TaxExemptBelow,
		m.InventorySlots, m.BankTripTicks, m.DefaultActionsPerHour, cacheVersion)
	return "top:" + hex.EncodeToString(h.Sum(nil))
}
