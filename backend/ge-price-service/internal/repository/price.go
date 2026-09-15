package repository

import (
	"time"

	"gorm.io/gorm"

	"github.com/rs3-market/backend/shared/models"
)

type Repo struct{ DB *gorm.DB }

func New(db *gorm.DB) *Repo { return &Repo{DB: db} }

func (r *Repo) Migrate() error {
	return r.DB.AutoMigrate(&models.PriceSnapshot{})
}

func (r *Repo) InsertSnapshots(items []models.PriceSnapshot) error {
	if len(items) == 0 {
		return nil
	}
	return r.DB.CreateInBatches(items, 200).Error
}

// LatestForItems returns the newest snapshot per item ID.
//
// The join can yield more than one row for an item when two snapshots
// share the same maximum timestamp — the poller stamps a whole cycle
// with a single `now`, so a double-run leaves exact ties. Deduplication
// happens in Go rather than via GROUP BY because selecting non-aggregated
// columns alongside a GROUP BY is rejected under MySQL 8's default
// ONLY_FULL_GROUP_BY mode.
func (r *Repo) LatestForItems(ids []int64) ([]models.PriceSnapshot, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []models.PriceSnapshot
	// Subquery: max(ts) per item.
	sub := r.DB.Model(&models.PriceSnapshot{}).
		Select("item_id, MAX(timestamp) AS max_ts").
		Where("item_id IN ?", ids).
		Group("item_id")
	err := r.DB.
		Joins("JOIN (?) AS m ON m.item_id = price_snapshots.item_id AND m.max_ts = price_snapshots.timestamp", sub).
		Where("price_snapshots.item_id IN ?", ids).
		Order("price_snapshots.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return dedupeByItem(rows), nil
}

// dedupeByItem keeps the highest-id row per item, preserving the order
// items first appear so results stay stable across calls.
func dedupeByItem(rows []models.PriceSnapshot) []models.PriceSnapshot {
	if len(rows) < 2 {
		return rows
	}
	index := make(map[int64]int, len(rows))
	out := make([]models.PriceSnapshot, 0, len(rows))
	for _, row := range rows {
		if at, seen := index[row.ItemID]; seen {
			if row.ID > out[at].ID {
				out[at] = row
			}
			continue
		}
		index[row.ItemID] = len(out)
		out = append(out, row)
	}
	return out
}

func (r *Repo) History(itemID int64, from, to time.Time) ([]models.PriceSnapshot, error) {
	var out []models.PriceSnapshot
	err := r.DB.
		Where("item_id = ? AND timestamp BETWEEN ? AND ?", itemID, from, to).
		Order("timestamp ASC").
		Find(&out).Error
	return out, err
}

// PriceAt returns the snapshot closest to (but not after) the given
// instant. Used for change-over-time comparisons.
func (r *Repo) PriceAt(itemID int64, at time.Time) (*models.PriceSnapshot, error) {
	var s models.PriceSnapshot
	err := r.DB.
		Where("item_id = ? AND timestamp <= ?", itemID, at).
		Order("timestamp DESC").
		Limit(1).
		First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}
