package scraper

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rs3-market/backend/shared/models"
)

// Modern RS3 infobox (current):
//
//	{{Infobox Recipe
//	|name        = Sapphire necklace
//	|members     = Yes
//	|skill       = Crafting
//	|skill1lvl   = 22
//	|skill1exp   = 55
//	|mat1        = Gold bar
//	|mat1qty     = 1
//	|mat2        = Sapphire
//	|mat2qty     = 1
//	|output1     = Sapphire necklace
//	|output1qty  = 1
//	}}
//
// The item's own page carries the item ID in a plain `|id =` field:
//
//	{{Infobox Item
//	|name = Sapphire necklace
//	|id   = 1656
//	}}
//
// We match both `|id` and `|itemid` to cover both templates.
var (
	reName    = regexp.MustCompile(`\|\s*name\s*=\s*([^\n|}]+)`)
	reMembers = regexp.MustCompile(`\|\s*members\s*=\s*([^\n|}]+)`)

	// Item ID: prefer "itemid" if present, else the plain "id" field.
	reItemID    = regexp.MustCompile(`\|\s*itemid\s*=\s*(\d+)`)
	reItemIDAlt = regexp.MustCompile(`\|\s*id\s*=\s*(\d+)`)

	reAPH = regexp.MustCompile(`\|\s*aph\s*=\s*(\d+)`)

	// Ticks needs multiline mode: `$` must match the end of the
	// `|ticks = 3` line, not just the end of the whole wikitext blob.
	reTicks    = regexp.MustCompile(`(?m)\|\s*ticks\s*=\s*(\d+)\s*$`)
	reFacility = regexp.MustCompile(`\|\s*facility\s*=\s*([^|\n]+)`)

	// Skill: matches skill, skill1, skill2 ...
	reSkill = regexp.MustCompile(`\|\s*skill\d*\s*=\s*([^\n|}]+)`)

	// Level: skilllvl, skill1lvl, level, level1 ...
	reLevel = regexp.MustCompile(`\|\s*(?:skill\d*lvl|level\d*)\s*=\s*(\d+)`)

	// XP: skillexp, skill1exp, xp, xp1 ...
	reXP = regexp.MustCompile(`\|\s*(?:skill\d*exp|xp\d*)\s*=\s*([0-9.]+)`)

	reMatN    = regexp.MustCompile(`\|\s*mat(\d+)\s*=\s*([^\n|}]+)`)
	reMatQtyN = regexp.MustCompile(`\|\s*mat(\d+)qty\s*=\s*(\d+)`)

	reOutputN    = regexp.MustCompile(`\|\s*output(\d*)\s*=\s*([^\n|}]+)`)
	reOutputQtyN = regexp.MustCompile(`\|\s*output(\d*)qty\s*=\s*(\d+)`)

	reMatLegacy = regexp.MustCompile(`(?s)\|\s*materials\s*=\s*(.*?)(?:\n\s*\||\n\s*\}\})`)
	reLink      = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)
	reBullet    = regexp.MustCompile(`^\s*\*\s*(?:(\d+)\s+)?`)
)

// Values that mean "this field was a boolean placeholder, not a skill".
var bogusSkills = map[string]bool{
	"no": true, "yes": true, "none": true, "n/a": true,
	"unknown": true, "": true, "?": true,
}

// parseWikitext extracts a recipe from a page's raw wikitext.
func parseWikitext(title, hintSkill, wt string) *models.Recipe {
	rec := &models.Recipe{
		Name:           title,
		OutputItemName: title,
		OutputQty:      1,
		Skill:          hintSkill,
		Source:         "runescape.wiki",
	}

	if m := reName.FindStringSubmatch(wt); len(m) == 2 {
		rec.Name = cleanValue(m[1])
		rec.OutputItemName = rec.Name
	}
	if m := reMembers.FindStringSubmatch(wt); len(m) == 2 {
		rec.Members = strings.EqualFold(cleanValue(m[1]), "yes")
	}
	if m := reSkill.FindStringSubmatch(wt); len(m) == 2 {
		rec.Skill = cleanValue(m[1])
	}
	if m := reLevel.FindStringSubmatch(wt); len(m) == 2 {
		rec.LevelReq, _ = strconv.Atoi(m[1])
	}
	if m := reXP.FindStringSubmatch(wt); len(m) == 2 {
		rec.XPPerAction, _ = strconv.ParseFloat(m[1], 64)
	}

	// Item ID: try both field names. Prefer explicit `itemid`.
	if m := reItemID.FindStringSubmatch(wt); len(m) == 2 {
		rec.OutputItemID, _ = strconv.ParseInt(m[1], 10, 64)
	} else if m := reItemIDAlt.FindStringSubmatch(wt); len(m) == 2 {
		rec.OutputItemID, _ = strconv.ParseInt(m[1], 10, 64)
	}

	if m := reAPH.FindStringSubmatch(wt); len(m) == 2 {
		if aph, err := strconv.Atoi(m[1]); err == nil && aph > 0 {
			rec.ActionsPerHour = aph
			rec.APHSource = models.APHSourceWiki
		}
	}

	// Only a bare number counts. "varies" means the cost depends on
	// heat, level or a minigame, and reading it as anything else would
	// invent a rate.
	if m := reTicks.FindStringSubmatch(wt); len(m) == 2 {
		if ticks, err := strconv.Atoi(m[1]); err == nil && ticks > 0 {
			rec.Ticks = ticks
		}
	}
	if m := reFacility.FindStringSubmatch(wt); len(m) == 2 {
		rec.Facility = strings.TrimSpace(m[1])
	}

	// Output name/qty overrides from numbered fields.
	if m := reOutputN.FindStringSubmatch(wt); len(m) == 3 {
		if out := resolveLink(m[2]); out != "" {
			rec.OutputItemName = out
		}
	}
	if m := reOutputQtyN.FindStringSubmatch(wt); len(m) == 3 {
		if q, err := strconv.Atoi(m[2]); err == nil && q > 0 {
			rec.OutputQty = q
		}
	}

	// Inputs.
	inputs := collectNumberedMaterials(wt)
	if len(inputs) == 0 {
		inputs = collectLegacyMaterials(wt)
	}
	rec.Inputs = inputs

	// ---------- validity guards ----------

	// Reject if skill came out as a boolean/placeholder.
	if bogusSkills[strings.ToLower(strings.TrimSpace(rec.Skill))] {
		return nil
	}

	// RS3 skills cap at 120. Anything outside 1..120 is a parse error,
	// not a real level. Most recipe pages have skill1lvl = 0 when the
	// field is a "second optional skill" placeholder.
	if rec.LevelReq < 1 || rec.LevelReq > 120 {
		return nil
	}

	// A recipe must have at least one input.
	if len(rec.Inputs) == 0 {
		return nil
	}

	return rec
}

func collectNumberedMaterials(wt string) []models.RecipeInput {
	byIdx := map[int]*models.RecipeInput{}

	for _, m := range reMatN.FindAllStringSubmatch(wt, -1) {
		idx, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		name := resolveLink(m[2])
		if name == "" {
			continue
		}
		in := byIdx[idx]
		if in == nil {
			in = &models.RecipeInput{Quantity: 1}
			byIdx[idx] = in
		}
		in.ItemName = name
	}
	for _, m := range reMatQtyN.FindAllStringSubmatch(wt, -1) {
		idx, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		qty, err := strconv.Atoi(m[2])
		if err != nil || qty <= 0 {
			continue
		}
		in := byIdx[idx]
		if in == nil {
			in = &models.RecipeInput{}
			byIdx[idx] = in
		}
		in.Quantity = qty
	}

	return sortedInputs(byIdx)
}

func collectLegacyMaterials(wt string) []models.RecipeInput {
	m := reMatLegacy.FindStringSubmatch(wt)
	if len(m) < 2 {
		return nil
	}
	var out []models.RecipeInput
	for _, line := range strings.Split(m[1], "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(strings.TrimSpace(line), "*") {
			continue
		}
		qty := 1
		if bm := reBullet.FindStringSubmatch(line); len(bm) == 2 && bm[1] != "" {
			qty, _ = strconv.Atoi(bm[1])
		}
		name := resolveLink(line)
		if name == "" {
			continue
		}
		out = append(out, models.RecipeInput{ItemName: name, Quantity: qty})
	}
	return out
}

func sortedInputs(byIdx map[int]*models.RecipeInput) []models.RecipeInput {
	keys := make([]int, 0, len(byIdx))
	for k := range byIdx {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	out := make([]models.RecipeInput, 0, len(keys))
	for _, k := range keys {
		in := byIdx[k]
		if in.ItemName == "" {
			continue
		}
		if in.Quantity <= 0 {
			in.Quantity = 1
		}
		out = append(out, *in)
	}
	return out
}

func cleanValue(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "}}")
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[[")
	s = strings.TrimSuffix(s, "]]")
	if i := strings.Index(s, "|"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func resolveLink(s string) string {
	if m := reLink.FindStringSubmatch(s); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "*")
	s = strings.TrimSpace(s)
	for _, sep := range []string{" x ", " × "} {
		if i := strings.Index(s, sep); i > 0 && i < 6 {
			if _, err := strconv.Atoi(strings.TrimSpace(s[:i])); err == nil {
				s = strings.TrimSpace(s[i+len(sep):])
			}
		}
	}
	return s
}
