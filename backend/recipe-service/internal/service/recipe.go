package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/recipe-service/internal/repository"
	"github.com/rs3-market/backend/shared/cache"
	"github.com/rs3-market/backend/shared/models"
)

type Service struct {
	log  *zap.Logger
	repo *repository.Repo
	c    *cache.Cache
}

func New(repo *repository.Repo, log *zap.Logger, c *cache.Cache) *Service {
	return &Service{repo: repo, log: log, c: c}
}

// Tree is what calc-service consumes.
type Tree struct {
	Root *Node `json:"root"`
}

type Node struct {
	Recipe   models.Recipe `json:"recipe"`
	Children []*Node       `json:"children"`
	Depth    int           `json:"depth"`
}

const treeTTL = time.Hour
const maxDepth = 12

// BuildTree resolves all intermediate recipes needed to produce `itemID`.
// Cached in Redis under recipe:tree:<itemID>.
func (s *Service) BuildTree(itemID int64) (*Tree, error) {
	key := fmt.Sprintf("recipe:tree:%d", itemID)
	if cached, err := s.c.Get(key); err == nil {
		var t Tree
		if json.Unmarshal([]byte(cached), &t) == nil {
			return &t, nil
		}
	}

	root, err := s.buildNode(itemID, 0, map[int64]bool{})
	if err != nil {
		return nil, err
	}
	tree := &Tree{Root: root}

	if body, err := json.Marshal(tree); err == nil {
		_ = s.c.Set(key, string(body), treeTTL)
	}
	return tree, nil
}

func (s *Service) buildNode(itemID int64, depth int, visited map[int64]bool) (*Node, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("recipe tree too deep at %d", itemID)
	}
	if visited[itemID] {
		return nil, fmt.Errorf("recipe cycle at %d", itemID)
	}
	visited[itemID] = true
	defer delete(visited, itemID)

	recipes, err := s.repo.GetByOutputItemID(itemID)
	if err != nil {
		return nil, fmt.Errorf("lookup item %d: %w", itemID, err)
	}
	if len(recipes) == 0 {
		return nil, fmt.Errorf("no recipe for item %d", itemID)
	}

	node := &Node{Recipe: pickPrimary(recipes), Depth: depth}

	// Index-based so the resolved ID is written back into the node we
	// return. Ranging by value here would resolve the ID, use it for the
	// recursion, and then throw it away — leaving item_id at 0 in the
	// tree that calc-service consumes, where an unknown ID is priced at
	// zero and the input silently becomes free.
	for i := range node.Recipe.Inputs {
		in := &node.Recipe.Inputs[i]

		if in.ItemID == 0 {
			in.ItemID = s.resolveItemID(in.ItemName)
		}
		if in.ItemID == 0 {
			// Genuinely unidentifiable. Leave it at 0 and move on;
			// calc-service treats a zero ID as an unpriced input and
			// refuses to report a margin that depends on it.
			continue
		}

		child, err := s.buildNode(in.ItemID, depth+1, visited)
		if err != nil {
			// Not craftable from anything we know about, so it is a
			// leaf: a raw material to be bought on the GE.
			continue
		}
		in.IsIntermediate = true
		node.Children = append(node.Children, child)
	}
	return node, nil
}

// resolveItemID finds an item ID for a name, first via a recipe that
// outputs it, then via the wiki's GE ID table.
func (s *Service) resolveItemID(name string) int64 {
	if name == "" {
		return 0
	}
	if rec, err := s.repo.GetByName(name); err == nil && rec.OutputItemID > 0 {
		return rec.OutputItemID
	}
	if id, err := s.repo.ItemIDByName(name); err == nil {
		return id
	}
	return 0
}

// pickPrimary chooses which recipe to use when several produce the same
// item. Highest XP wins, then fewest inputs, then name — fully ordered
// so the same catalogue always yields the same tree, which matters
// because the result is cached and compared across requests.
func pickPrimary(recipes []models.Recipe) models.Recipe {
	best := recipes[0]
	for _, r := range recipes[1:] {
		switch {
		case r.XPPerAction != best.XPPerAction:
			if r.XPPerAction > best.XPPerAction {
				best = r
			}
		case len(r.Inputs) != len(best.Inputs):
			if len(r.Inputs) < len(best.Inputs) {
				best = r
			}
		case r.Name < best.Name:
			best = r
		}
	}
	return best
}

func (s *Service) ListFiltered(skill string, maxLevel int) ([]models.Recipe, error) {
	return s.repo.ListFiltered(skill, maxLevel)
}

func (s *Service) GetByName(name string) (*models.Recipe, error) {
	return s.repo.GetByName(name)
}

// ListOutputItemIDs returns distinct output item IDs filtered by
// skill and level range. All filters are optional. Level bounds are
// inclusive.
func (s *Service) ListOutputItemIDs(skill string, minLevel, maxLevel int) ([]int64, error) {
	return s.repo.ListOutputItemIDs(skill, minLevel, maxLevel)
}

// ---- price feed ----

func (s *Service) AllRelevantItemIDs(ctx context.Context) ([]int64, error) {
	return s.repo.AllRelevantItemIDs()
}

func (s *Service) BackfillInputItemIDs() (int64, error) {
	return s.repo.BackfillInputItemIDs()
}

func (s *Service) InputIDStats() (int64, int64, error) {
	return s.repo.InputIDStats()
}

// ---- search ----

func (s *Service) SearchRecipes(q, skill string, minLevel, maxLevel, limit, offset int) ([]models.Recipe, int64, error) {
	return s.repo.SearchRecipes(q, skill, minLevel, maxLevel, limit, offset)
}

func (s *Service) SearchItems(q, source string, limit, offset int) ([]repository.ItemSummary, int64, error) {
	return s.repo.SearchItems(q, source, limit, offset)
}

// ---- buy limits ----

func (s *Service) BuyLimitsFor(ids []int64) (map[int64]int, error) {
	return s.repo.BuyLimitsFor(ids)
}
