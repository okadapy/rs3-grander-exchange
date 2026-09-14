package repository

import (
	"time"

	"gorm.io/gorm"

	"github.com/rs3-market/backend/shared/models"
)

type Repo struct{ DB *gorm.DB }

func New(db *gorm.DB) *Repo { return &Repo{DB: db} }

func (r *Repo) Migrate() error {
	return r.DB.AutoMigrate(&models.PriceSnapshot{}, &models.DailyChange{})
}

func (r *Repo) InsertSnapshots(items []models.PriceSnapshot) error {
	if len(items) == 0 {
		return nil
	}
	return r.DB.CreateInBatches(items, 200).Error
}

// LatestForItems returns the newest snapshot per item ID.
func (r *Repo) LatestForItems(ids []int64) ([]models.PriceSnapshot, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []models.PriceSnapshot
	// Subquery: max(ts) per item
	sub := r.DB.Model(&models.PriceSnapshot{}).
		Select("item_id, MAX(timestamp) AS max_ts").
		Where("item_id IN ?", ids).
		Group("item_id")
	err := r.DB.
		Joins("JOIN (?) AS m ON m.item_id = price_snapshots.item_id AND m.max_ts = price_snapshots.timestamp", sub).
		Where("price_snapshots.item_id IN ?", ids).
		Find(&out).Error
	return out, err
}

func (r *Repo) History(itemID int64, from, to time.Time) ([]models.PriceSnapshot, error) {
	var out []models.PriceSnapshot
	err := r.DB.
		Where("item_id = ? AND timestamp BETWEEN ? AND ?", itemID, from, to).
		Order("timestamp ASC").
		Find(&out).Error
	return out, err
}

// Price24hAgo returns the snapshot closest to 24h before now.
func (r *Repo) Price24hAgo(itemID int64, at time.Time) (*models.PriceSnapshot, error) {
	var s models.PriceSnapshot
	target := at.Add(-24 * time.Hour)
	err := r.DB.
		Where("item_id = ? AND timestamp <= ?", itemID, target).
		Order("timestamp DESC").
		Limit(1).
		First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}
