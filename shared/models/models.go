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
	Members        bool          `json:"members"`
	Source         string        `gorm:"size:64" json:"source"`
	Inputs         []RecipeInput `gorm:"foreignKey:RecipeID" json:"inputs"`
}

type RecipeInput struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	RecipeID       uint   `gorm:"index" json:"recipe_id"`
	ItemID         int64  `gorm:"index" json:"item_id"`
	ItemName       string `gorm:"size:160" json:"item_name"`
	Quantity       int    `json:"quantity"`
	IsIntermediate bool   `json:"is_intermediate"`
}

// -------------------- ge_db --------------------

type PriceSnapshot struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ItemID     int64     `gorm:"index:idx_item_ts,priority:1" json:"item_id"`
	Timestamp  time.Time `gorm:"index:idx_item_ts,priority:2" json:"ts"`
	BuyPrice   int64     `json:"buy"`
	SellPrice  int64     `json:"sell"`
	Volume     int64     `json:"volume"`
	Normalized bool      `json:"normalized"`
}

type DailyChange struct {
	ID        uint      `gorm:"primaryKey"`
	ItemID    int64     `gorm:"index:idx_item_day,priority:1"`
	Day       time.Time `gorm:"index:idx_item_day,priority:2"`
	ChangePct float64
	VolumeSum int64
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
