package repository

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/rs3-market/backend/shared/models"
)

type Repo struct{ DB *gorm.DB }

func New(db *gorm.DB) *Repo { return &Repo{DB: db} }

func (r *Repo) Migrate() error {
	return r.DB.AutoMigrate(&models.Player{}, &models.PlayerSkill{})
}

// UpsertPlayer replaces skills for a (name, mode) pair atomically.
func (r *Repo) UpsertPlayer(p *models.Player) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.Player
		err := tx.Where("name = ? AND mode = ?", p.Name, p.Mode).First(&existing).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == gorm.ErrRecordNotFound {
			if err := tx.Create(p).Error; err != nil {
				return err
			}
			return nil
		}
		// delete old skills, update player
		if err := tx.Where("player_id = ?", existing.ID).Delete(&models.PlayerSkill{}).Error; err != nil {
			return err
		}
		existing.FetchedAt = time.Now().UTC()
		if err := tx.Model(&existing).Update("fetched_at", existing.FetchedAt).Error; err != nil {
			return err
		}
		for i := range p.Skills {
			p.Skills[i].PlayerID = existing.ID
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&p.Skills).Error
	})
}

func (r *Repo) GetPlayer(name, mode string) (*models.Player, error) {
	var p models.Player
	err := r.DB.Preload("Skills").Where("name = ? AND mode = ?", name, mode).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}
