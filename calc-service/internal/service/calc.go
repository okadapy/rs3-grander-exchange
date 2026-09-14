package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/models"
	"github.com/rs3-market/backend/calc-service/internal/cache"
	"github.com/rs3-market/backend/calc-service/internal/client"
)

const geTaxPct = 0.02
const geTaxCap = 5_000_000

type Service struct {
	log    *zap.Logger
	recipe *client.RecipeClient
	price  *client.PriceClient
	his    *client.HiscoreClient
	cc     *cache.CalcCache
}

func New(log *zap.Logger, r *client.RecipeClient, p *client.PriceClient, h *client.HiscoreClient, cc *cache.CalcCache) *Service {
	return &Service{log: log, recipe: r, price: p, his: h, cc: cc}
}

type Options struct {
	ActionsPerHourOverride int  // 0 = use recipe value
	Player                 string
	Mode                   string
	Normalized             bool
}

type PathResult struct {
	Path             []string `json:"path"`
	Steps            []Step   `json:"steps"`
	ProfitPerTrade   float64  `json:"profit_per_trade"`
	GPPerHour        float64  `json:"gp_per_hour"`
	XPPerHour        float64  `json:"xp_per_hour"`
	GPPerXP          float64  `json:"gp_per_xp"`
	ROIPct           float64  `json:"roi_pct"`
	TotalHours       float64  `json:"total_hours"`
	TotalXP          float64  `json:"total_xp"`
	BuyCost          float64  `json:"buy_cost"`
	SellRevenue      float64  `json:"sell_revenue"`
	TaxPaid          float64  `json:"tax_paid"`
	ActionsPerHour   int      `json:"actions_per_hour"`
}

type Step struct {
	Recipe       string  `json:"recipe"`
	Skill        string  `json:"skill"`
	LevelReq     int     `json:"level_req"`
	XP           float64 `json:"xp"`
	APH          int     `json:"aph"`
	BuyCost      float64 `json:"buy_cost"`
	SellRevenue  float64 `json:"sell_revenue"`
	Profit       float64 `json:"profit"`
}

type Result struct {
	ItemID int64        `json:"item_id"`
	Paths  []PathResult `json:"paths"`
	Cached bool         `json:"cached"`
}

func (s *Service) Calculate(ctx context.Context, itemID int64, opts Options) (*Result, error) {
	key := cacheKey(itemID, opts)
	var cached Result
	if s.cc.Get(key, &cached) {
		cached.Cached = true
		return &cached, nil
	}

	tree, err := s.recipe.Tree(ctx, itemID)
	if err != nil {
		return nil, err
	}

	// collect all item IDs used anywhere in the tree
	ids := collectItemIDs(tree.Root)
	ids = append(ids, itemID)
	prices, err := s.price.Latest(ctx, ids)
	if err != nil {
		return nil, err
	}

	var paths []PathResult
	s.walk(tree.Root, nil, &paths, prices, opts)

	sort.Slice(paths, func(i, j int) bool {
		return paths[i].GPPerHour > paths[j].GPPerHour
	})

	res := &Result{ItemID: itemID, Paths: paths}
	s.cc.Set(key, res)
	return res, nil
}

// walk enumerates every crafting path from leaves up. Each recursion
// produces a distinct PathResult so we can rank all the ways to make
// the requested item.
func (s *Service) walk(node *client.Node, prefix []string, out *[]PathResult, prices map[int64]models.PriceSnapshot, opts Options) {
	if node == nil {
		return
	}

	// Path A: buy all inputs, do this recipe only.
	buyAllPath := append(append([]string{}, prefix...), node.Recipe.Name)
	prBuyAll := s.evaluatePath(node.Recipe.Name, []*client.Node{node}, prices, opts)
	prBuyAll.Path = buyAllPath
	*out = append(*out, prBuyAll)

	// Path B..N: for each child, recursively craft it first, then craft node.
	for _, child := range node.Children {
		childPaths := []PathResult{}
		s.walk(child, append(prefix, node.Recipe.Name), &childPaths, prices, opts)
		for _, cp := range childPaths {
			combined := s.combine(node, cp, prices, opts)
			*out = append(*out, combined)
		}
	}
}

func (s *Service) evaluatePath(name string, nodes []*client.Node, prices map[int64]models.PriceSnapshot, opts Options) PathResult {
	var totalHours, totalXP, buyCost, revenue, tax float64
	var steps []Step

	for _, n := range nodes {
		aph := chooseAPH(n.Recipe.ActionsPerHour, opts.ActionsPerHourOverride)
		hours := 1.0 / float64(aph)

		outPrice := priceOf(n.Recipe.OutputItemID, prices)
		stepRev := float64(outPrice) * float64(n.Recipe.OutputQty)
		stepTax := taxFor(stepRev)

		var stepCost float64
		for _, in := range n.Recipe.Inputs {
			p := priceOf(in.ItemID, prices)
			stepCost += float64(p) * float64(in.Quantity)
		}

		stepProfit := stepRev - stepTax - stepCost
		step := Step{
			Recipe:      n.Recipe.Name,
			Skill:       n.Recipe.Skill,
			LevelReq:    n.Recipe.LevelReq,
			XP:          n.Recipe.XPPerAction,
			APH:         aph,
			BuyCost:     stepCost,
			SellRevenue: stepRev,
			Profit:      stepProfit,
		}
		steps = append(steps, step)

		totalHours += hours
		totalXP += n.Recipe.XPPerAction
		buyCost += stepCost
		revenue += stepRev
		tax += stepTax
	}

	profit := revenue - tax - buyCost
	pr := PathResult{
		Path:           []string{name},
		Steps:          steps,
		ProfitPerTrade: profit,
		BuyCost:        buyCost,
		SellRevenue:    revenue,
		TaxPaid:        tax,
		TotalHours:     totalHours,
		TotalXP:        totalXP,
	}
	if totalHours > 0 {
		pr.GPPerHour = profit / totalHours
		pr.XPPerHour = totalXP / totalHours
	}
	if totalXP > 0 {
		pr.GPPerXP = profit / totalXP
	}
	if buyCost > 0 {
		pr.ROIPct = profit / buyCost * 100.0
	}
	if len(steps) > 0 {
		pr.ActionsPerHour = steps[0].APH
	}
	return pr
}

func (s *Service) combine(root *client.Node, child PathResult, prices map[int64]models.PriceSnapshot, opts Options) PathResult {
	// root step: use intermediate input at cost 0 (we crafted it ourselves)
	aph := chooseAPH(root.Recipe.ActionsPerHour, opts.ActionsPerHourOverride)
	hours := 1.0 / float64(aph)

	outPrice := priceOf(root.Recipe.OutputItemID, prices)
	stepRev := float64(outPrice) * float64(root.Recipe.OutputQty)
	stepTax := taxFor(stepRev)

	// cost = all inputs except the ones we crafted (approximated: subtract child outputs)
	var stepCost float64
	for _, in := range root.Recipe.Inputs {
		p := priceOf(in.ItemID, prices)
		stepCost += float64(p) * float64(in.Quantity)
	}
	// Replace buy cost of any input the child produced with the child's buy cost.
	// Simple version: subtract child's sell revenue (what it would cost to buy).
	stepCost = math.Max(0, stepCost-child.SellRevenue)

	rootStep := Step{
		Recipe:      root.Recipe.Name,
		Skill:       root.Recipe.Skill,
		LevelReq:    root.Recipe.LevelReq,
		XP:          root.Recipe.XPPerAction,
		APH:         aph,
		BuyCost:     stepCost,
		SellRevenue: stepRev,
		Profit:      stepRev - stepTax - stepCost,
	}

	steps := append([]Step{}, child.Steps...)
	steps = append(steps, rootStep)

	buyCost := child.BuyCost + stepCost
	revenue := stepRev // final output only
	tax := stepTax
	totalXP := child.TotalXP + root.Recipe.XPPerAction
	totalHours := child.TotalHours + hours

	profit := revenue - tax - buyCost
	pr := PathResult{
		Path:           append(append([]string{}, child.Path...), root.Recipe.Name),
		Steps:          steps,
		ProfitPerTrade: profit,
		BuyCost:        buyCost,
		SellRevenue:    revenue,
		TaxPaid:        tax,
		TotalHours:     totalHours,
		TotalXP:        totalXP,
		ActionsPerHour: aph,
	}
	if totalHours > 0 {
		pr.GPPerHour = profit / totalHours
		pr.XPPerHour = totalXP / totalHours
	}
	if totalXP > 0 {
		pr.GPPerXP = profit / totalXP
	}
	if buyCost > 0 {
		pr.ROIPct = profit / buyCost * 100.0
	}
	return pr
}

// ----------------------------- helpers -----------------------------

func chooseAPH(stored, override int) int {
	if override > 0 {
		return override
	}
	if stored > 0 {
		return stored
	}
	return 600
}

func taxFor(rev float64) float64 {
	t := rev * geTaxPct
	if t > geTaxCap {
		return geTaxCap
	}
	return t
}

func priceOf(itemID int64, prices map[int64]models.PriceSnapshot) int64 {
	if p, ok := prices[itemID]; ok {
		if p.BuyPrice > 0 {
			return p.BuyPrice
		}
		return p.SellPrice
	}
	return 0
}

func collectItemIDs(n *client.Node) []int64 {
	set := map[int64]struct{}{}
	var visit func(*client.Node)
	visit = func(x *client.Node) {
		if x == nil {
			return
		}
		set[x.Recipe.OutputItemID] = struct{}{}
		for _, in := range x.Recipe.Inputs {
			set[in.ItemID] = struct{}{}
		}
		for _, c := range x.Children {
			visit(c)
		}
	}
	visit(n)
	out := make([]int64, 0, len(set))
	for id := range set {
		if id != 0 {
			out = append(out, id)
		}
	}
	return out
}

func cacheKey(itemID int64, opts Options) string {
	h := sha1.New()
	fmt.Fprintf(h, "%d|aph=%d|p=%s|m=%s|n=%v",
		itemID, opts.ActionsPerHourOverride, opts.Player, opts.Mode, opts.Normalized)
	return "calc:" + hex.EncodeToString(h.Sum(nil))
}

var _ = time.Second
