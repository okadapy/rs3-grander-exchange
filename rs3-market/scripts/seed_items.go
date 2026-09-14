//go:build ignore

// Sample seeder — run with: go run ./scripts/seed_items.go
// Connects directly to MySQL (recipe_db) and inserts a couple of recipes so
// the calc-service has something to work with before the wiki scrape finishes.
package main

import (
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/rs3-market/backend/shared/models"
)

func main() {
	dsn := "rs3:rs3pass@tcp(localhost:3306)/recipe_db?charset=utf8mb4&parseTime=True&loc=UTC"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Recipe{}, &models.RecipeInput{}); err != nil {
		log.Fatal(err)
	}
	recipes := []models.Recipe{
		{
			Name: "Sapphire necklace", OutputItemID: 1656, OutputItemName: "Sapphire necklace",
			OutputQty: 1, Skill: "Crafting", LevelReq: 22, XPPerAction: 55, ActionsPerHour: 1000,
			Inputs: []models.RecipeInput{
				{ItemID: 2357, ItemName: "Gold bar", Quantity: 1},
				{ItemID: 1623, ItemName: "Sapphire", Quantity: 1},
			},
		},
		{
			Name: "Gold bar", OutputItemID: 2357, OutputItemName: "Gold bar",
			OutputQty: 1, Skill: "Smithing", LevelReq: 40, XPPerAction: 22.5, ActionsPerHour: 100,
			Inputs: []models.RecipeInput{
				{ItemID: 444, ItemName: "Gold ore", Quantity: 1},
			},
		},
	}
	for _, r := range recipes {
		var existing models.Recipe
		if err := db.Where("name = ?", r.Name).First(&existing).Error; err == nil {
			continue
		}
		if err := db.Create(&r).Error; err != nil {
			log.Printf("skip %s: %v", r.Name, err)
		}
	}
	log.Println("seeded")
}
