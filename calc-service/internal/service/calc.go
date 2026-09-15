package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/calc-service/internal/cache"
	"github.com/rs3-market/backend/calc-service/internal/client"
	"github.com/rs3-market/backend/shared/models"
	"github.com/rs3-market/backend/shared/rates"
)

// maxCombinationsPerNode bounds the cartesian product at each tree node.
// A recipe with many craftable inputs, each with many sub-paths, would
// otherwise blow up combinatorially. Past the bound we keep the
// combinations already enumerated and add the all-buy baseline, so the
// answer is always at least as good as "buy everything".
const maxCombinationsPerNode = 64

// cacheVersion is mixed into every cache key. Bump it whenever the
// output shape or the arithmetic changes, so results computed under the
// old model are never served after a deploy.
const cacheVersion = "v4"

type Service struct {
	log    *zap.Logger
	recipe *client.RecipeClient
	price  *client.PriceClient
	his    *client.HiscoreClient
	cc     *cache.CalcCache
	market Market
}

func New(log *zap.Logger, r *client.RecipeClient, p *client.PriceClient,
	h *client.HiscoreClient, cc *cache.CalcCache, market Market) *Service {
	return &Service{log: log, recipe: r, price: p, his: h, cc: cc, market: market}
}

type Options struct {
	ActionsPerHourOverride int
	Player                 string
	Mode                   string

	// SpreadPctOverride lets a caller model a tighter or wider spread
	// than the server default. Nil means "use the configured value".
	SpreadPctOverride *float64

	// IncludeIncomplete keeps paths that depend on an input we could not
	// price. They are excluded by default because their margin is not a
	// margin — an unpriced input is costed at nothing, which makes the
	// worst paths look like the best ones.
	IncludeIncomplete bool

	// Boosts is the forging stack the caller declared. The zero value is
	// the conservative floor.
	Boosts rates.Boosts
	// BoostsRaw is the unparsed parameter, kept for the assumptions
	// block and the cache key.
	BoostsRaw string
}

// Assumptions is echoed in every response so a number is never separated
// from the model that produced it.
type Assumptions struct {
	PriceBasis     string  `json:"price_basis"`
	SpreadPct      float64 `json:"spread_pct"`
	TaxPct         float64 `json:"tax_pct"`
	TaxCapPerItem  int64   `json:"tax_cap_per_item"`
	TaxExemptBelow int64   `json:"tax_exempt_below"`
	Player         string  `json:"player,omitempty"`
	Mode           string  `json:"mode,omitempty"`

	// InventorySlots and BankTripTicks are the banking model behind
	// every actions-per-hour figure derived from ticks.
	InventorySlots int `json:"inventory_slots"`
	BankTripTicks  int `json:"bank_trip_ticks"`
	// Boosts is the forging stack the caller declared, verbatim.
	Boosts string `json:"boosts,omitempty"`
}

// Throughput reports the ceiling the Grand Exchange buy limits put on a
// strategy, independent of how fast you can click.
type Throughput struct {
	BindingItemID   int64   `json:"binding_item_id"`
	BindingItemName string  `json:"binding_item_name"`
	BindingLimit4h  int     `json:"binding_limit_4h"`
	MaxCraftsPer4h  int     `json:"max_crafts_per_4h"`
	CraftsPerHour   float64 `json:"crafts_per_hour"`
	GPPerHour       float64 `json:"gp_per_hour"`
}

// SkillRequirement is one level gate on a path.
type SkillRequirement struct {
	Skill    string `json:"skill"`
	Required int    `json:"required"`
	Actual   int    `json:"actual"`
	Met      bool   `json:"met"`
}

type Step struct {
	Recipe      string  `json:"recipe"`
	Skill       string  `json:"skill"`
	LevelReq    int     `json:"level_req"`
	XP          float64 `json:"xp"`
	APH         int     `json:"aph"`
	APHSource   string  `json:"aph_source"`
	Runs        int     `json:"runs"`
	BuyCost     float64 `json:"buy_cost"`
	SellRevenue float64 `json:"sell_revenue"`
	Profit      float64 `json:"profit"`
}

type PathResult struct {
	Path           []string `json:"path"`
	Steps          []Step   `json:"steps"`
	ProfitPerCraft float64  `json:"profit_per_craft"`
	GPPerHour      float64  `json:"gp_per_hour"`
	XPPerHour      float64  `json:"xp_per_hour"`
	GPPerXP        float64  `json:"gp_per_xp"`
	ROIPct         float64  `json:"roi_pct"`
	TotalHours     float64  `json:"total_hours"`
	TotalXP        float64  `json:"total_xp"`
	BuyCost        float64  `json:"buy_cost"`
	SellRevenue    float64  `json:"sell_revenue"`
	TaxPaid        float64  `json:"tax_paid"`
	ActionsPerHour int      `json:"actions_per_hour"`
	APHSource      string   `json:"aph_source"`

	// Complete is false when any input on the path had no price. The
	// money figures on an incomplete path are upper bounds, not
	// estimates, because the missing inputs are costed at zero.
	Complete       bool     `json:"complete"`
	UnpricedInputs []string `json:"unpriced_inputs,omitempty"`

	// Throughput is nil when no input on the path has a known buy limit.
	Throughput *Throughput `json:"throughput,omitempty"`

	// MeetsRequirements is nil unless a player was supplied.
	MeetsRequirements *bool              `json:"meets_requirements,omitempty"`
	SkillRequirements []SkillRequirement `json:"skill_requirements,omitempty"`
}

type Result struct {
	ItemID      int64        `json:"item_id"`
	GeneratedAt time.Time    `json:"generated_at"`
	Assumptions Assumptions  `json:"assumptions"`
	Paths       []PathResult `json:"paths"`
	Cached      bool         `json:"cached"`
}

func (s *Service) Calculate(ctx context.Context, itemID int64, opts Options) (*Result, error) {
	market := s.market
	if opts.SpreadPctOverride != nil && *opts.SpreadPctOverride >= 0 {
		market.SpreadPct = *opts.SpreadPctOverride
	}

	key := cacheKey(itemID, opts, market)
	var cached Result
	if s.cc.Get(key, &cached) {
		cached.Cached = true
		return &cached, nil
	}

	tree, err := s.recipe.Tree(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if tree.Root == nil {
		return nil, fmt.Errorf("no recipe tree for item %d", itemID)
	}

	ids := collectItemIDs(tree.Root)
	prices, err := s.price.Latest(ctx, ids)
	if err != nil {
		return nil, err
	}

	// Buy limits and player levels are enrichment, not preconditions.
	// Losing either degrades the answer; it should not fail the request.
	limits := map[int64]int{}
	if l, err := s.recipe.BuyLimits(ctx, ids); err != nil {
		s.log.Debug("buy limits unavailable", zap.Error(err))
	} else {
		limits = l
	}

	var levels map[string]int
	if opts.Player != "" {
		if lv, err := s.playerLevels(ctx, opts.Player, opts.Mode); err != nil {
			s.log.Debug("hiscore lookup failed",
				zap.String("player", opts.Player), zap.Error(err))
		} else {
			levels = lv
		}
	}

	ev := &evaluator{market: market, prices: prices, limits: limits, opts: opts, levels: levels}
	paths := ev.expand(tree.Root)

	if !opts.IncludeIncomplete {
		kept := paths[:0]
		for _, p := range paths {
			if p.Complete {
				kept = append(kept, p)
			}
		}
		paths = kept
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no priceable production path for item %d", itemID)
	}

	for i := range paths {
		applySkillRequirements(&paths[i], levels)
	}
	sortPaths(paths)

	res := &Result{
		ItemID:      itemID,
		GeneratedAt: time.Now().UTC(),
		Paths:       paths,
		Assumptions: Assumptions{
			PriceBasis:     PriceBasis,
			SpreadPct:      market.SpreadPct,
			TaxPct:         market.TaxPct,
			TaxCapPerItem:  market.TaxCapPerItem,
			TaxExemptBelow: market.TaxExemptBelow,
			Player:         opts.Player,
			Mode:           opts.Mode,
			InventorySlots: market.InventorySlots,
			BankTripTicks:  market.BankTripTicks,
			Boosts:         opts.BoostsRaw,
		},
	}
	s.cc.Set(key, res)
	return res, nil
}

func (s *Service) playerLevels(ctx context.Context, name, mode string) (map[string]int, error) {
	if mode == "" {
		mode = "normal"
	}
	p, err := s.his.Get(ctx, name, mode)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(p.Skills))
	for _, sk := range p.Skills {
		out[strings.ToLower(sk.Skill)] = sk.Level
	}
	return out, nil
}

// ---- tree expansion ----

type evaluator struct {
	market Market
	prices map[int64]models.PriceSnapshot
	limits map[int64]int
	opts   Options

	// levels is the player's skill levels, lowercase-keyed as produced
	// by playerLevels, or nil when no player was supplied. chooseAPH
	// looks skills up through the level method, which lowercases the
	// query to match.
	levels map[string]int
}

// level looks up a skill in e.levels. Real data arrives lowercase-keyed
// from playerLevels — the same map applySkillRequirements reads with a
// lowercased key — so the query is lowercased here too rather than
// trying the literal case first; a fallback would let a fixture that
// happens to use the recipe's title case hide a real casing mismatch.
func (e *evaluator) level(skill string) int {
	if e.levels == nil {
		return 0
	}
	return e.levels[strings.ToLower(skill)]
}

type childPlan struct {
	inputIdx int
	runs     int
	paths    []PathResult
}

// expand returns every distinct way to produce the item at node.
//
// At each node, every craftable input independently gets a choice:
// buy it, or craft it via any one of that child's own paths. "Craft
// everything", "buy everything" and every mixture in between fall out
// of the same enumeration.
func (e *evaluator) expand(node *client.Node) []PathResult {
	if node == nil {
		return nil
	}

	var craftable []childPlan
	for i, in := range node.Recipe.Inputs {
		child := matchChild(node, in)
		if child == nil {
			continue
		}
		childPaths := e.expand(child)
		if len(childPaths) == 0 {
			continue
		}
		outQty := child.Recipe.OutputQty
		if outQty < 1 {
			outQty = 1
		}
		craftable = append(craftable, childPlan{
			inputIdx: i,
			runs:     ceilDiv(in.Quantity, outQty),
			paths:    childPaths,
		})
	}

	assignments := enumerate(craftable)
	out := make([]PathResult, 0, len(assignments))
	for _, a := range assignments {
		if pr := e.evaluate(node, craftable, a); pr != nil {
			out = append(out, *pr)
		}
	}
	return out
}

// enumerate builds the buy-vs-craft choice vectors. Index 0 means "buy
// this input"; k>0 means "craft it via the child's path k-1".
//
// When the product would exceed the bound we stop expanding and keep
// what we have, plus the all-buy baseline. Keeping the partial set
// beats discarding it: those are real, evaluated strategies.
func enumerate(craftable []childPlan) [][]int {
	assignments := [][]int{{}}
	for i, cp := range craftable {
		var next [][]int
		for _, a := range assignments {
			for choice := 0; choice <= len(cp.paths); choice++ {
				row := make([]int, len(a), len(a)+1)
				copy(row, a)
				next = append(next, append(row, choice))
			}
		}
		if len(next) > maxCombinationsPerNode {
			// Pad what we already have out to full length with "buy".
			for j := range assignments {
				for len(assignments[j]) < len(craftable) {
					assignments[j] = append(assignments[j], 0)
				}
			}
			if i == 0 {
				assignments = [][]int{make([]int, len(craftable))}
			}
			return assignments
		}
		assignments = next
	}
	return assignments
}

// evaluate computes the PathResult for one buy-vs-craft choice vector.
func (e *evaluator) evaluate(node *client.Node, craftable []childPlan, assignment []int) *PathResult {
	type chosen struct {
		plan PathResult
		runs int
	}
	crafting := map[int]chosen{}
	for i, choice := range assignment {
		if choice == 0 || i >= len(craftable) {
			continue
		}
		cp := craftable[i]
		if choice-1 >= len(cp.paths) {
			continue
		}
		crafting[cp.inputIdx] = chosen{plan: cp.paths[choice-1], runs: cp.runs}
	}

	var (
		parentCost float64
		unpriced   []string
		complete   = true
	)
	// Inputs we buy rather than craft, priced at the assumed buy side.
	bought := make([]models.RecipeInput, 0, len(node.Recipe.Inputs))
	for i, in := range node.Recipe.Inputs {
		if _, ok := crafting[i]; ok {
			continue
		}
		guide, ok := e.guidePrice(in.ItemID)
		if !ok {
			complete = false
			unpriced = append(unpriced, inputLabel(in))
			continue
		}
		parentCost += e.market.BuyPrice(guide) * float64(in.Quantity)
		bought = append(bought, in)
	}

	aph, aphSource := e.chooseAPH(node.Recipe)
	parentHours := 1.0 / float64(aph)

	outQty := node.Recipe.OutputQty
	if outQty < 1 {
		outQty = 1
	}
	outGuide, outPriced := e.guidePrice(node.Recipe.OutputItemID)
	if !outPriced {
		complete = false
		unpriced = append(unpriced, node.Recipe.OutputItemName)
	}
	unitSell := e.market.SellPrice(outGuide)
	revenue := unitSell * float64(outQty)
	tax := e.market.Tax(unitSell, outQty)

	// Fold in the children we chose to craft, scaled by run count.
	var (
		childCost  float64
		childHours float64
		childXP    float64
		childSteps []Step
		pathNames  []string
	)
	for _, ch := range crafting {
		r := float64(ch.runs)
		childCost += ch.plan.BuyCost * r
		childHours += ch.plan.TotalHours * r
		childXP += ch.plan.TotalXP * r
		if !ch.plan.Complete {
			complete = false
			unpriced = append(unpriced, ch.plan.UnpricedInputs...)
		}
		for _, st := range ch.plan.Steps {
			scaled := st
			scaled.Runs = st.Runs * ch.runs
			scaled.BuyCost = st.BuyCost * r
			scaled.SellRevenue = st.SellRevenue * r
			scaled.Profit = st.Profit * r
			childSteps = append(childSteps, scaled)
		}
		pathNames = append(pathNames, ch.plan.Path...)
	}

	buyCost := childCost + parentCost
	totalHours := childHours + parentHours
	totalXP := childXP + node.Recipe.XPPerAction
	profit := revenue - tax - buyCost

	pr := PathResult{
		Path: append(pathNames, node.Recipe.Name),
		Steps: append(childSteps, Step{
			Recipe:      node.Recipe.Name,
			Skill:       node.Recipe.Skill,
			LevelReq:    node.Recipe.LevelReq,
			XP:          node.Recipe.XPPerAction,
			APH:         aph,
			APHSource:   aphSource,
			Runs:        1,
			BuyCost:     parentCost,
			SellRevenue: revenue,
			Profit:      revenue - tax - parentCost,
		}),
		ProfitPerCraft: profit,
		BuyCost:        buyCost,
		SellRevenue:    revenue,
		TaxPaid:        tax,
		TotalHours:     totalHours,
		TotalXP:        totalXP,
		ActionsPerHour: aph,
		APHSource:      aphSource,
		Complete:       complete,
		UnpricedInputs: dedupe(unpriced),
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
	pr.Throughput = e.throughput(bought, profit, pr.GPPerHour, float64(aph))
	return &pr
}

// throughput finds the input whose 4-hour buy limit binds hardest and
// reports the GP/h that limit actually allows.
//
// Clicking speed is rarely the real constraint: you cannot craft a
// thousand items an hour out of a material you may only buy a hundred of
// every four hours. Only bought inputs count — a crafted input is
// produced, not purchased, so no limit applies to it.
func (e *evaluator) throughput(bought []models.RecipeInput, profitPerCraft, gpPerHour, aph float64) *Throughput {
	best := (*Throughput)(nil)
	for _, in := range bought {
		limit, ok := e.limits[in.ItemID]
		if !ok || limit <= 0 || in.Quantity <= 0 {
			continue
		}
		crafts4h := limit / in.Quantity
		if best == nil || crafts4h < best.MaxCraftsPer4h {
			best = &Throughput{
				BindingItemID:   in.ItemID,
				BindingItemName: in.ItemName,
				BindingLimit4h:  limit,
				MaxCraftsPer4h:  crafts4h,
			}
		}
	}
	if best == nil {
		return nil
	}
	best.CraftsPerHour = math.Min(aph, float64(best.MaxCraftsPer4h)/4)
	best.GPPerHour = profitPerCraft * best.CraftsPerHour
	return best
}

func (e *evaluator) guidePrice(itemID int64) (int64, bool) {
	if itemID <= 0 {
		return 0, false
	}
	p, ok := e.prices[itemID]
	if !ok || p.Price <= 0 {
		return 0, false
	}
	return p.Price, true
}

// chooseAPH decides how fast a recipe can actually be performed, and
// reports where the number came from. The order matters: a caller's
// override beats everything, mechanics beat the infobox because the
// infobox publishes the best case, and the house default is the last
// resort it has always been.
func (e *evaluator) chooseAPH(r models.Recipe) (int, string) {
	if e.opts.ActionsPerHourOverride > 0 {
		return e.opts.ActionsPerHourOverride, models.APHSourceOverride
	}

	slots := rates.SlotsPerCraft(r.Inputs)
	cfg := e.market.RatesConfig()

	if r.Skill == "Smithing" && e.levels != nil {
		smithing := e.level("Smithing")

		if r.Facility == "Furnace" {
			if metal, ok := rates.BarMetal(r.OutputItemName); ok {
				if ticks, ok := rates.SmeltTicks(metal, smithing); ok {
					return rates.ActionsPerHour(ticks, slots, cfg),
						models.APHSourceTicksLevel
				}
			}
		}

		if r.Facility == "Anvil" {
			for _, in := range r.Inputs {
				metal, ok := rates.BarMetal(in.ItemName)
				if !ok {
					continue
				}
				bars := in.Quantity
				if bars < 1 {
					bars = 1
				}
				ticks, ok := rates.ForgeTicks(bars, metal, smithing,
					e.level("Firemaking"), e.opts.Boosts)
				if ok {
					return rates.ActionsPerHour(ticks, slots, cfg),
						models.APHSourceTicksForge
				}
			}
		}
	}

	if r.Ticks > 0 {
		return rates.ActionsPerHour(r.Ticks, slots, cfg), models.APHSourceTicks
	}
	if r.ActionsPerHour > 0 {
		src := r.APHSource
		if src == "" {
			src = models.APHSourceDefault
		}
		return r.ActionsPerHour, src
	}
	return e.market.DefaultActionsPerHour, models.APHSourceDefault
}

// ---- post-processing ----

// applySkillRequirements annotates a path with the level gates it
// crosses. With no hiscore data the fields stay nil rather than
// defaulting to "you qualify", which would be a guess presented as fact.
func applySkillRequirements(p *PathResult, levels map[string]int) {
	if levels == nil {
		return
	}
	seen := map[string]int{}
	for _, st := range p.Steps {
		if st.Skill == "" || st.LevelReq <= 0 {
			continue
		}
		if cur, ok := seen[st.Skill]; !ok || st.LevelReq > cur {
			seen[st.Skill] = st.LevelReq
		}
	}

	reqs := make([]SkillRequirement, 0, len(seen))
	all := true
	for skill, need := range seen {
		actual := levels[strings.ToLower(skill)]
		met := actual >= need
		if !met {
			all = false
		}
		reqs = append(reqs, SkillRequirement{
			Skill: skill, Required: need, Actual: actual, Met: met,
		})
	}
	sort.Slice(reqs, func(i, j int) bool { return reqs[i].Skill < reqs[j].Skill })

	p.SkillRequirements = reqs
	p.MeetsRequirements = &all
}

// sortPaths ranks by what a player can actually achieve: paths they can
// perform come first, then limit-aware GP/h where a limit is known,
// since that is the rate they would really see.
func sortPaths(paths []PathResult) {
	sort.SliceStable(paths, func(i, j int) bool {
		a, b := paths[i], paths[j]
		if am, bm := meets(a), meets(b); am != bm {
			return am
		}
		if a.Complete != b.Complete {
			return a.Complete
		}
		return effectiveGPPerHour(a) > effectiveGPPerHour(b)
	})
}

func meets(p PathResult) bool {
	return p.MeetsRequirements == nil || *p.MeetsRequirements
}

func effectiveGPPerHour(p PathResult) float64 {
	if p.Throughput != nil {
		return p.Throughput.GPPerHour
	}
	return p.GPPerHour
}

// ---- helpers ----

func matchChild(node *client.Node, in models.RecipeInput) *client.Node {
	for _, child := range node.Children {
		if child == nil {
			continue
		}
		if in.ItemID > 0 && child.Recipe.OutputItemID == in.ItemID {
			return child
		}
		if in.ItemName == "" {
			continue
		}
		if strings.EqualFold(child.Recipe.OutputItemName, in.ItemName) ||
			strings.EqualFold(child.Recipe.Name, in.ItemName) {
			return child
		}
	}
	return nil
}

func inputLabel(in models.RecipeInput) string {
	if in.ItemName != "" {
		return in.ItemName
	}
	return fmt.Sprintf("item %d", in.ItemID)
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func ceilDiv(a, b int) int {
	if b <= 0 {
		b = 1
	}
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}

func collectItemIDs(n *client.Node) []int64 {
	set := map[int64]struct{}{}
	var visit func(*client.Node)
	visit = func(x *client.Node) {
		if x == nil {
			return
		}
		if x.Recipe.OutputItemID != 0 {
			set[x.Recipe.OutputItemID] = struct{}{}
		}
		for _, in := range x.Recipe.Inputs {
			if in.ItemID != 0 {
				set[in.ItemID] = struct{}{}
			}
		}
		for _, c := range x.Children {
			visit(c)
		}
	}
	visit(n)

	out := make([]int64, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func cacheKey(itemID int64, opts Options, m Market) string {
	h := sha1.New()
	fmt.Fprintf(h, "%d|aph=%d|p=%s|m=%s|inc=%v|spread=%.4f|tax=%.4f|cap=%d|ex=%d|inv=%d|bank=%d|boosts=%s|%s",
		itemID, opts.ActionsPerHourOverride, strings.ToLower(opts.Player), opts.Mode,
		opts.IncludeIncomplete, m.SpreadPct, m.TaxPct, m.TaxCapPerItem,
		m.TaxExemptBelow, m.InventorySlots, m.BankTripTicks,
		strings.ToLower(opts.BoostsRaw), cacheVersion)
	return "calc:" + hex.EncodeToString(h.Sum(nil))
}
