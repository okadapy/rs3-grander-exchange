package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"

	"github.com/rs3-market/backend/shared/models"
)

type Repo struct{ DB *gorm.DB }

func New(db *gorm.DB) *Repo { return &Repo{DB: db} }

func (r *Repo) Migrate() error {
	return r.DB.AutoMigrate(&models.Recipe{}, &models.RecipeInput{}, &models.GEIDMap{}, &models.ItemLimit{})
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

		// Delete old inputs first so we insert a fresh set.
		if err := tx.Where("recipe_id = ?", existing.ID).Delete(&models.RecipeInput{}).Error; err != nil {
			return err
		}

		rec.ID = existing.ID
		for i := range rec.Inputs {
			rec.Inputs[i].ID = 0 // ensure insert, not update-by-PK
			rec.Inputs[i].RecipeID = existing.ID
		}

		// Omit("Inputs") prevents GORM from cascading the association
		// during Save — which was causing the duplicate-key error on
		// the second scrape run.
		if err := tx.Omit("Inputs").Save(rec).Error; err != nil {
			return err
		}
		if len(rec.Inputs) == 0 {
			return nil
		}
		return tx.Create(&rec.Inputs).Error
	})
}

func (r *Repo) GetByOutputItemID(itemID int64) ([]models.Recipe, error) {
	var out []models.Recipe
	err := r.DB.Preload("Inputs").Where("output_item_id = ?", itemID).Find(&out).Error
	return out, err
}

// ItemIDByName looks up a GE item ID from the wiki name map. Used for
// inputs that no recipe produces — ores, logs, herbs and the like.
func (r *Repo) ItemIDByName(name string) (int64, error) {
	var row models.GEIDMap
	if err := r.DB.Where("name = ?", name).First(&row).Error; err != nil {
		return 0, err
	}
	return row.ItemID, nil
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

// ListOutputItemIDs returns distinct output item IDs for recipes
// matching the optional filters. Rows with output_item_id = 0 are
// excluded so callers never receive unresolved placeholders.
//
// Filter semantics:
//   - skill == ""      -> all skills
//   - minLevel == 0    -> no lower bound
//   - maxLevel == 0    -> no upper bound
//
// Ranges are inclusive on both ends.
func (r *Repo) ListOutputItemIDs(skill string, minLevel, maxLevel int) ([]int64, error) {
	q := r.DB.Model(&models.Recipe{}).
		Distinct().
		Where("output_item_id > 0")

	if skill != "" {
		q = q.Where("skill = ?", skill)
	}
	if minLevel > 0 {
		q = q.Where("level_req >= ?", minLevel)
	}
	if maxLevel > 0 {
		q = q.Where("level_req <= ?", maxLevel)
	}

	var ids []int64
	err := q.Pluck("output_item_id", &ids).Error
	return ids, err
}

// -------------------- input ID resolution --------------------

// BackfillInputItemIDs resolves `recipe_inputs.item_id` from matching
// output item names. Anything that's craftable (bars, planks, leathers,
// etc.) gets its ID populated this way. Raw materials like ores that
// aren't outputs of any recipe remain at 0 and need mapping-based
// resolution (see: Weirdgloop /mapping endpoint).
//
// Safe to call repeatedly — only touches rows where item_id = 0.
func (r *Repo) BackfillInputItemIDs() (int64, error) {
	res := r.DB.Exec(`
		UPDATE recipe_inputs ri
		JOIN recipes r ON ri.item_name = r.output_item_name
		SET ri.item_id = r.output_item_id
		WHERE (ri.item_id = 0 OR ri.item_id IS NULL)
		  AND r.output_item_id > 0
	`)
	return res.RowsAffected, res.Error
}

// InputIDStats returns (total, resolved) — used at startup to report
// how complete the input ID coverage is.
func (r *Repo) InputIDStats() (int64, int64, error) {
	var total, resolved int64
	if err := r.DB.Model(&models.RecipeInput{}).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	if err := r.DB.Model(&models.RecipeInput{}).
		Where("item_id > 0").Count(&resolved).Error; err != nil {
		return 0, 0, err
	}
	return total, resolved, nil
}

// -------------------- poller feed --------------------

// AllRelevantItemIDs returns the union of output IDs and input IDs.
// This is what the price poller subscribes to — both sides of every
// recipe need prices for the calculator to work.
func (r *Repo) AllRelevantItemIDs() ([]int64, error) {
	seen := map[int64]struct{}{}

	var outputs []int64
	if err := r.DB.Model(&models.Recipe{}).
		Where("output_item_id > 0").
		Distinct("output_item_id").
		Pluck("output_item_id", &outputs).Error; err != nil {
		return nil, err
	}
	for _, id := range outputs {
		seen[id] = struct{}{}
	}

	var inputs []int64
	if err := r.DB.Model(&models.RecipeInput{}).
		Where("item_id > 0").
		Distinct("item_id").
		Pluck("item_id", &inputs).Error; err != nil {
		return nil, err
	}
	for _, id := range inputs {
		seen[id] = struct{}{}
	}

	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out, nil
}

// -------------------- search --------------------

// SearchRecipes returns recipes matching name substring and optional
// skill/level filters, plus the total match count for pagination.
// Inputs are NOT preloaded — this is a summary listing.
func (r *Repo) SearchRecipes(q, skill string, minLevel, maxLevel, limit, offset int) ([]models.Recipe, int64, error) {
	base := r.DB.Model(&models.Recipe{})

	if q != "" {
		like := "%" + q + "%"
		base = base.Where("name LIKE ? OR output_item_name LIKE ?", like, like)
	}
	if skill != "" {
		base = base.Where("skill = ?", skill)
	}
	if minLevel > 0 {
		base = base.Where("level_req >= ?", minLevel)
	}
	if maxLevel > 0 {
		base = base.Where("level_req <= ?", maxLevel)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var recipes []models.Recipe
	err := base.
		Order("level_req ASC, name ASC").
		Limit(limit).
		Offset(offset).
		Find(&recipes).Error
	return recipes, total, err
}

// ItemSummary is a lightweight item for search results.
type ItemSummary struct {
	ItemID  int64    `json:"item_id"`
	Name    string   `json:"name"`
	Sources []string `json:"sources"` // e.g. ["output"], ["input"], ["input","output"]
}

// SearchItems searches distinct item names appearing as either a
// recipe output or a recipe input. `source` filters to "output",
// "input", or "all" (empty defaults to all).
func (r *Repo) SearchItems(q, source string, limit, offset int) ([]ItemSummary, int64, error) {
	inner := `
		SELECT output_item_id AS item_id, output_item_name AS name, 'output' AS source
		FROM recipes WHERE output_item_id > 0
		UNION ALL
		SELECT item_id, item_name, 'input'
		FROM recipe_inputs WHERE item_id > 0
	`

	where := "1=1"
	args := []interface{}{}
	if q != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+q+"%")
	}
	if source == "output" || source == "input" {
		where += " AND source = ?"
		args = append(args, source)
	}

	var total int64
	countSQL := "SELECT COUNT(DISTINCT item_id) FROM (" + inner + ") AS combined WHERE " + where
	if err := r.DB.Raw(countSQL, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	selectSQL := "SELECT item_id, MAX(name) AS name, GROUP_CONCAT(DISTINCT source) AS sources " +
		"FROM (" + inner + ") AS combined WHERE " + where +
		" GROUP BY item_id ORDER BY name LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	var rows []struct {
		ItemID  int64
		Name    string
		Sources string
	}
	if err := r.DB.Raw(selectSQL, args...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]ItemSummary, len(rows))
	for i, row := range rows {
		out[i] = ItemSummary{
			ItemID:  row.ItemID,
			Name:    row.Name,
			Sources: strings.Split(row.Sources, ","),
		}
	}
	return out, total, nil
}

// -------------------- GEIDs resolution --------------------

// UpsertGEIDs bulk-inserts rows into geid_map. Repeated calls are
// idempotent — an existing name just has its item_id refreshed.
func (r *Repo) UpsertGEIDs(rows []models.GEIDMap) error {
	if len(rows) == 0 {
		return nil
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"item_id"}),
	}).CreateInBatches(rows, 500).Error
}

// BackfillInputsFromGEIDs resolves any remaining recipe_inputs rows
// whose item_id is 0 by joining on the canonical name map. This is
// the second pass after BackfillInputItemIDs (which handles
// intermediates that are themselves recipe outputs).
func (r *Repo) BackfillInputsFromGEIDs() (int64, error) {
	res := r.DB.Exec(`
		UPDATE recipe_inputs ri
		JOIN geid_maps g ON g.name = ri.item_name
		SET ri.item_id = g.item_id
		WHERE (ri.item_id = 0 OR ri.item_id IS NULL)
		  AND g.item_id > 0
	`)
	return res.RowsAffected, res.Error
}

// BackfillOutputsFromGEIDs resolves recipes whose own output_item_id is
// still 0 by matching the output name against the GEIDs table.
//
// The parser only finds an output ID when the page happens to carry an
// `|id=` field, which many recipe pages do not. Every unresolved recipe
// is excluded from the price poller's feed and from /recipes/ids, so
// leaving these at 0 quietly removes them from the catalogue.
func (r *Repo) BackfillOutputsFromGEIDs() (int64, error) {
	res := r.DB.Exec(`
		UPDATE recipes rc
		JOIN geid_maps g ON g.name = rc.output_item_name
		SET rc.output_item_id = g.item_id
		WHERE (rc.output_item_id = 0 OR rc.output_item_id IS NULL)
		  AND g.item_id > 0
	`)
	return res.RowsAffected, res.Error
}

// -------------------- buy limits --------------------

// UpsertItemLimits bulk-inserts the wiki buy-limit table. Idempotent:
// an existing name just has its limit refreshed.
func (r *Repo) UpsertItemLimits(rows []models.ItemLimit) error {
	if len(rows) == 0 {
		return nil
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"limit4h"}),
	}).CreateInBatches(rows, 500).Error
}

// BackfillLimitItemIDs attaches item IDs to buy-limit rows via the GEIDs
// table, so limits can be looked up by ID rather than by name.
func (r *Repo) BackfillLimitItemIDs() (int64, error) {
	res := r.DB.Exec(`
		UPDATE item_limits il
		JOIN geid_maps g ON g.name = il.name
		SET il.item_id = g.item_id
		WHERE (il.item_id = 0 OR il.item_id IS NULL)
		  AND g.item_id > 0
	`)
	return res.RowsAffected, res.Error
}

// BuyLimitsFor returns itemID -> 4-hour buy limit. Unknown items are
// absent from the map rather than present with a zero, so callers can
// tell "no limit data" apart from "limit of zero".
func (r *Repo) BuyLimitsFor(ids []int64) (map[int64]int, error) {
	out := map[int64]int{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []models.ItemLimit
	if err := r.DB.Where("item_id IN ? AND limit4h > 0", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ItemID] = row.Limit4h
	}
	return out, nil
}
