package scraper

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/rs3-market/backend/shared/models"
)

// parseWikitext extracts the important fields from an item's page.
// This is intentionally a light-weight parser — the wiki maintains
// a very consistent infobox template for craftable items.
//
// Example field we read:
//   {{Infobox Item
//   |name = Sapphire necklace
//   |members = Yes
//   |skill = Crafting
//   |skilllvl = 22
//   |skillexp = 55
//   |materials = * 1 [[gold bar]]\n* 1 [[sapphire]]
//   |itemid = 1656
//   }}
var (
	reName     = regexp.MustCompile(`\|\s*name\s*=\s*([^\n|]+)`)
	reMembers  = regexp.MustCompile(`\|\s*members\s*=\s*([^\n|]+)`)
	reSkill    = regexp.MustCompile(`\|\s*skill\s*=\s*([^\n|]+)`)
	reLevel    = regexp.MustCompile(`\|\s*skilllvl\s*=\s*([0-9]+)`)
	reXP       = regexp.MustCompile(`\|\s*skillexp\s*=\s*([0-9.]+)`)
	reMat      = regexp.MustCompile(`\|\s*materials\s*=\s*([^\n]+(?:\n\*[^\n]+)*)`)
	reItemID   = regexp.MustCompile(`\|\s*itemid\s*=\s*([0-9]+)`)
	reLink     = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)
	reQty      = regexp.MustCompile(`^\s*(\d+)\s+`)
	reAph      = regexp.MustCompile(`\|\s*aph\s*=\s*([0-9]+)`)
)

func parseWikitext(title, hintSkill, wt string) *models.Recipe {
	rec := &models.Recipe{
		Name:           title,
		OutputItemName: title,
		OutputQty:      1,
		Skill:          hintSkill,
		Source:         "runescape.wiki",
	}

	if m := reName.FindStringSubmatch(wt); len(m) == 2 {
		rec.Name = strings.TrimSpace(m[1])
		rec.OutputItemName = rec.Name
	}
	if m := reMembers.FindStringSubmatch(wt); len(m) == 2 {
		rec.Members = strings.EqualFold(strings.TrimSpace(m[1]), "yes")
	}
	if m := reSkill.FindStringSubmatch(wt); len(m) == 2 {
		rec.Skill = strings.TrimSpace(m[1])
	}
	if m := reLevel.FindStringSubmatch(wt); len(m) == 2 {
		rec.LevelReq, _ = strconv.Atoi(m[1])
	}
	if m := reXP.FindStringSubmatch(wt); len(m) == 2 {
		rec.XPPerAction, _ = strconv.ParseFloat(m[1], 64)
	}
	if m := reItemID.FindStringSubmatch(wt); len(m) == 2 {
		rec.OutputItemID, _ = strconv.ParseInt(m[1], 10, 64)
	}
	if m := reAph.FindStringSubmatch(wt); len(m) == 2 {
		rec.ActionsPerHour, _ = strconv.Atoi(m[1])
	}

	// materials — a bulleted list of the form "* 1 [[item]]"
	if m := reMat.FindStringSubmatch(wt); len(m) == 2 {
		for _, line := range strings.Split(m[1], "\n") {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "*")
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			qty := 1
			if qm := reQty.FindStringSubmatch(line); len(qm) == 2 {
				qty, _ = strconv.Atoi(qm[1])
				line = strings.TrimSpace(line[len(qm[1]):])
			}
			links := reLink.FindStringSubmatch(line)
			if len(links) < 2 {
				continue
			}
			itemName := strings.TrimSpace(links[1])
			rec.Inputs = append(rec.Inputs, models.RecipeInput{
				ItemName: itemName,
				Quantity: qty,
				// intermediate flag resolved later by the tree builder
				IsIntermediate: false,
			})
		}
	}

	if rec.LevelReq == 0 && rec.XPPerAction == 0 && len(rec.Inputs) == 0 {
		// Not a real recipe page
		return nil
	}
	return rec
}
