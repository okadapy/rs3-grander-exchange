package repository

import (
	"gorm.io/gorm"

	"github.com/rs3-market/backend/shared/models"
)

type Repo struct{ DB *gorm.DB }

func New(db *gorm.DB) *Repo { return &Repo{DB: db} }

func (r *Repo) Migrate() error {
	return r.DB.AutoMigrate(&models.Recipe{}, &models.RecipeInput{})
}

// UpsertRecipe inserts or updates a recipe + its inputs atomically.
func (r *Repo) UpsertRecipe(rec *models.Recipe) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.Recipe
		err := tx.Where("name = ?", rec.Name).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			return tx.Create(rec).Error
		}
		if err != nil {
			return err
		}
		rec.ID = existing.ID
		if err := tx.Where("recipe_id = ?", existing.ID).Delete(&models.RecipeInput{}).Error; err != nil {
			return err
		}
		for i := range rec.Inputs {
			rec.Inputs[i].RecipeID = existing.ID
		}
		if err := tx.Save(rec).Error; err != nil {
			return err
		}
		return tx.Create(&rec.Inputs).Error
	})
}

func (r *Repo) GetByOutputItemID(itemID int64) ([]models.Recipe, error) {
	var out []models.Recipe
	err := r.DB.Preload("Inputs").Where("output_item_id = ?", itemID).Find(&out).Error
	return out, err
}

func (r *Repo) GetByName(name string) (*models.Recipe, error) {
	var rec models.Recipe
	if err := r.DB.Preload("Inputs").Where("name = ?", name).First(&rec).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *Repo) ListFiltered(skill string, maxLevel int) ([]models.Recipe, error) {
	q := r.DB.Preload("Inputs")
	if skill != "" {
		q = q.Where("skill = ?", skill)
	}
	if maxLevel > 0 {
		q = q.Where("level_req <= ?", maxLevel)
	}
	var out []models.Recipe
	err := q.Find(&out).Error
	return out, err
}

func (r *Repo) AllOutputItemIDs() ([]int64, error) {
	var ids []int64
	err := r.DB.Model(&models.Recipe{}).Distinct("output_item_id").Pluck("output_item_id", &ids).Error
	return ids, err
}
