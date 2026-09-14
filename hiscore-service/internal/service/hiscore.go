package service

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/models"
	"github.com/rs3-market/backend/hiscore-service/internal/repository"
)

// RS3 skill order in the hiscores CSV — do not reorder.
var skillOrder = []string{
	"Overall", "Attack", "Defence", "Strength", "Constitution", "Ranged",
	"Prayer", "Magic", "Cooking", "Woodcutting", "Fletching", "Fishing",
	"Firemaking", "Crafting", "Smithing", "Mining", "Herblore", "Agility",
	"Thieving", "Slayer", "Farming", "Runecrafting", "Hunter", "Construction",
	"Summoning", "Dungeoneering", "Divination", "Invention", "Archaeology",
	"Necromancy",
}

var modeToPath = map[string]string{
	"normal":         "m=hiscore",
	"ironman":        "m=hiscore_ironman",
	"hardcore":       "m=hiscore_hardcore_ironman",
}

type Service struct {
	cfg    *config.Config
	log    *zap.Logger
	repo   *repository.Repo
	client *http.Client
}

func New(cfg *config.Config, log *zap.Logger, repo *repository.Repo) *Service {
	return &Service{
		cfg:    cfg,
		log:    log,
		repo:   repo,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *Service) FetchFromJagex(name, mode string) (*models.Player, error) {
	path, ok := modeToPath[mode]
	if !ok {
		return nil, fmt.Errorf("unknown mode %q", mode)
	}
	u := fmt.Sprintf("%s/%s/index_lite.ws?player=%s",
		s.cfg.Hiscore.BaseURL, path, url.QueryEscape(name))

	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("User-Agent", "RS3-Market-Backend/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, errors.New("player not found")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("hiscore status %d", resp.StatusCode)
	}

	r := csv.NewReader(resp.Body)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	player := &models.Player{
		Name:      strings.TrimSpace(name),
		Mode:      mode,
		FetchedAt: time.Now().UTC(),
	}

	idx := 0
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if idx >= len(skillOrder) {
			break
		}
		if len(row) < 3 {
			idx++
			continue
		}
		rank, _ := strconv.ParseInt(strings.TrimSpace(row[0]), 10, 64)
		level, _ := strconv.Atoi(strings.TrimSpace(row[1]))
		xp, _ := strconv.ParseInt(strings.TrimSpace(row[2]), 10, 64)

		player.Skills = append(player.Skills, models.PlayerSkill{
			Skill: skillOrder[idx],
			Level: level,
			XP:    xp,
			Rank:  rank,
		})
		idx++
	}

	if len(player.Skills) == 0 {
		return nil, errors.New("no skills parsed")
	}
	return player, nil
}

func (s *Service) GetOrFetch(name, mode string) (*models.Player, error) {
	p, err := s.repo.GetPlayer(name, mode)
	if err == nil && time.Since(p.FetchedAt) < s.cfg.Hiscore.CacheTTL {
		return p, nil
	}
	fresh, err := s.FetchFromJagex(name, mode)
	if err != nil {
		// fall back to stale if we have it
		if p != nil {
			s.log.Warn("hiscore fetch failed, using stale",
				zap.String("name", name), zap.Error(err))
			return p, nil
		}
		return nil, err
	}
	if err := s.repo.UpsertPlayer(fresh); err != nil {
		s.log.Error("upsert failed", zap.Error(err))
	}
	return fresh, nil
}
