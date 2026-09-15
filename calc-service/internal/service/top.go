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

func rowLess(a, b TopRow, m Metric) bool {
	switch m {
	case MetricXPPerHour:
		return a.XPPerHour < b.XPPerHour
	case MetricGPPerXP:
		return a.GPPerXP < b.GPPerXP
	default: // MetricGPPerHour
		return a.GPPerHourLimited < b.GPPerHourLimited
	}
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
	return row
}

// Top ranks every priceable recipe by one metric.
//
// It walks the whole catalogue, so it answers from the cache on
// anything but the first call for a given parameter set.
func (s *Service) Top(ctx context.Context, opts TopOptions) (TopResponse, error) {
	opts.Limit = clampLimit(opts.Limit)

	key := topCacheKey(opts)
	var cached TopResponse
	if s.cc.Get(key, &cached) {
		return cached, nil
	}

	ids, err := s.recipe.AllItemIDs(ctx)
	if err != nil {
		return TopResponse{}, fmt.Errorf("list item ids: %w", err)
	}

	best := make(map[int64]TopRow, len(ids))
	for _, id := range ids {
		res, err := s.Calculate(ctx, id, opts.Calc)
		if err != nil {
			// One unpriceable item must not fail the whole ranking. It
			// is simply absent from the results, the same reasoning
			// that makes /calc/batch report failures per entry rather
			// than failing the batch.
			continue
		}
		for _, p := range res.Paths {
			row := rowFromPath(id, p)
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
		Metric: opts.Metric,
		Count:  len(rows),
		Rows:   rows,
		// Built directly from the resolved market and the caller's
		// options rather than borrowed from whichever item happened to
		// be evaluated last: assumptionsFor depends on neither an item
		// nor its price data, so every item's Assumptions block under
		// these options is identical, and computing it up front holds
		// even when every item in the catalogue fails to price.
		Assumptions: assumptionsFor(s.resolveMarket(opts.Calc), opts.Calc),
	}
	s.cc.Set(key, out)
	return out, nil
}

// topCacheKey keys on the full parameter set: the ranking metric, the
// skill filter and limit, and every Options field that changes the
// price computed for an item. Missing one here would serve one
// player's or one boost stack's ranking to another's request.
func topCacheKey(opts TopOptions) string {
	spread := "nil"
	if opts.Calc.SpreadPctOverride != nil {
		spread = fmt.Sprintf("%.4f", *opts.Calc.SpreadPctOverride)
	}
	h := sha1.New()
	fmt.Fprintf(h, "metric=%s|skill=%s|limit=%d|aph=%d|p=%s|m=%s|spread=%s|inc=%v|boosts=%s|%s",
		opts.Metric, strings.ToLower(opts.Skill), opts.Limit,
		opts.Calc.ActionsPerHourOverride, strings.ToLower(opts.Calc.Player), opts.Calc.Mode,
		spread, opts.Calc.IncludeIncomplete, strings.ToLower(opts.Calc.BoostsRaw), cacheVersion)
	return "top:" + hex.EncodeToString(h.Sum(nil))
}
