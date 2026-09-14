package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/cache"
	"github.com/rs3-market/backend/shared/models"
	"github.com/rs3-market/backend/recipe-service/internal/repository"
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
	if err != nil || len(recipes) == 0 {
		return nil, fmt.Errorf("no recipe for item %d", itemID)
	}
	// Pick the "best" recipe for an item: highest XP or fewest inputs.
	primary := recipes[0]
	for _, r := range recipes[1:] {
		if r.XPPerAction > primary.XPPerAction {
			primary = r
		}
	}

	node := &Node{Recipe: primary, Depth: depth}
	for _, in := range primary.Inputs {
		if in.ItemID == 0 {
			// Item ID unknown (scraper didn't resolve). Try by name.
			rec, err := s.repo.GetByName(in.ItemName)
			if err != nil {
				continue
			}
			in.ItemID = rec.OutputItemID
		}
		child, err := s.buildNode(in.ItemID, depth+1, visited)
		if err != nil {
			// Leaf — input is a raw material
			continue
		}
		node.Children = append(node.Children, child)
	}
	return node, nil
}

func (s *Service) ListFiltered(skill string, maxLevel int) ([]models.Recipe, error) {
	return s.repo.ListFiltered(skill, maxLevel)
}

func (s *Service) GetByName(name string) (*models.Recipe, error) {
	return s.repo.GetByName(name)
}

func (s *Service) AllOutputItemIDs(ctx context.Context) ([]int64, error) {
	return s.repo.AllOutputItemIDs()
}

// helper for handlers
func ParseBoolDefault(s string, def bool) bool {
	if s == "" {
		return def
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return def
	}
	return v
}
