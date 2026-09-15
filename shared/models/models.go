package models

import "time"

// -------------------- hiscore_db --------------------

type Player struct {
	ID        uint          `gorm:"primaryKey" json:"id"`
	Name      string        `gorm:"uniqueIndex:idx_name_mode;size:12" json:"name"`
	Mode      string        `gorm:"uniqueIndex:idx_name_mode;size:20" json:"mode"`
	FetchedAt time.Time     `json:"fetched_at"`
	Skills    []PlayerSkill `gorm:"foreignKey:PlayerID" json:"skills"`
}

type PlayerSkill struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	PlayerID uint   `gorm:"index" json:"player_id"`
	Skill    string `gorm:"size:24;index" json:"skill"`
	Level    int    `json:"level"`
	XP       int64  `json:"xp"`
	Rank     int64  `json:"rank"`
}

// -------------------- recipe_db --------------------

// APHSource records where ActionsPerHour came from, so the frontend can
// show how much to trust a GP/h figure. The wiki rarely publishes a
// real actions-per-hour value, so most recipes fall back to a per-skill
// default — a number good enough to rank recipes against each other but
// not to quote as a literal rate.
const (
	APHSourceWiki     = "wiki"
	APHSourceDefault  = "default"
	APHSourceOverride = "override"
)

type Recipe struct {
	ID             uint          `gorm:"primaryKey" json:"id"`
	Name           string        `gorm:"uniqueIndex;size:160" json:"name"`
	OutputItemID   int64         `gorm:"index" json:"output_item_id"`
	OutputItemName string        `gorm:"size:160" json:"output_item_name"`
	OutputQty      int           `json:"output_qty"`
	Skill          string        `gorm:"size:32;index" json:"skill"`
	LevelReq       int           `json:"level_req"`
	XPPerAction    float64       `json:"xp_per_action"`
	ActionsPerHour int           `json:"actions_per_hour"`
	APHSource      string        `gorm:"size:16" json:"aph_source"`
	Members        bool          `json:"members"`
	Source         string        `gorm:"size:64" json:"source"`
	Inputs         []RecipeInput `gorm:"foreignKey:RecipeID" json:"inputs,omitempty"`
}

type RecipeInput struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	RecipeID       uint   `gorm:"index" json:"recipe_id"`
	ItemID         int64  `gorm:"index" json:"item_id"`
	ItemName       string `gorm:"size:160" json:"item_name"`
	Quantity       int    `json:"quantity"`
	IsIntermediate bool   `json:"is_intermediate"`
}

// GEIDMap maps a canonical item name to its Grand Exchange item ID.
// Populated once per day from the wiki's Module:GEIDs/data.json.
// Used to resolve recipe rows whose item_id is still 0 after
// recipe-output-name matching (ores, logs, herbs — anything that is
// gathered, not crafted).
type GEIDMap struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Name   string `gorm:"uniqueIndex;size:160" json:"name"`
	ItemID int64  `gorm:"index" json:"item_id"`
}

// TableName pins the table name. GORM's default for this struct is
// "ge_id_maps" (it splits the GEID acronym), which is easy to get wrong
// in the hand-written UPDATE ... JOIN statements that do the ID
// backfills — and a join against a non-existent table fails the whole
// backfill silently, leaving every gathered material unpriced.
func (GEIDMap) TableName() string { return "geid_maps" }

// ItemLimit is the Grand Exchange 4-hour buy limit for one item,
// from the wiki's Module:GELimits/data.json.
//
// This is the difference between a theoretical margin and a tradeable
// one: a craft that nets 500 GP on an item limited to 100 per 4 hours
// can return at most 50k GP per 4 hours no matter how fast you click.
// Without it, GP/h is an upper bound nobody can actually reach.
type ItemLimit struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Name    string `gorm:"uniqueIndex;size:160" json:"name"`
	ItemID  int64  `gorm:"index" json:"item_id"`
	Limit4h int    `gorm:"column:limit4h" json:"limit_4h"`
}

func (ItemLimit) TableName() string { return "item_limits" }

// -------------------- ge_db --------------------

// PriceSnapshot is one observation of an item's Grand Exchange price.
//
// Price is the GE *guide* price as published by Weirdgloop — the single
// number Jagex exposes. RS3 has no public instant-buy / instant-sell
// feed, so there is no real bid/ask spread in this data. Anything that
// needs a buy price and a sell price must apply an explicit, declared
// spread assumption on top of this (see calc-service). Earlier versions
// of this struct carried separate BuyPrice and SellPrice fields holding
// identical values, which read as a real spread and silently overstated
// every margin.
type PriceSnapshot struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ItemID    int64     `gorm:"index:idx_item_ts,priority:1" json:"item_id"`
	Timestamp time.Time `gorm:"index:idx_item_ts,priority:2" json:"ts"`
	Price     int64     `json:"price"`
	Volume    int64     `json:"volume"`
}

// -------------------- chat_db --------------------

type ChatUser struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:32" json:"username"`
	PasswordHash string    `gorm:"size:80" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type ChatMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Username  string    `gorm:"size:32" json:"username"`
	Body      string    `gorm:"size:512" json:"body"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
