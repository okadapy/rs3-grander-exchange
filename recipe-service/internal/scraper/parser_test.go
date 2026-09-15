package scraper

import (
	"fmt"
	"testing"

	"github.com/rs3-market/backend/shared/models"
)

const modernInfobox = `{{Infobox Recipe
|name        = Sapphire necklace
|members     = Yes
|skill       = Crafting
|skill1lvl   = 22
|skill1exp   = 55
|mat1        = [[Gold bar]]
|mat1qty     = 1
|mat2        = [[Sapphire]]
|mat2qty     = 1
|output1     = [[Sapphire necklace]]
|output1qty  = 1
}}`

func TestParseWikitextModernInfobox(t *testing.T) {
	rec := parseWikitext("Sapphire necklace", "", modernInfobox)
	if rec == nil {
		t.Fatal("parseWikitext returned nil for a valid recipe")
	}

	if rec.Name != "Sapphire necklace" {
		t.Errorf("name = %q, want %q", rec.Name, "Sapphire necklace")
	}
	if rec.Skill != "Crafting" {
		t.Errorf("skill = %q, want Crafting", rec.Skill)
	}
	if rec.LevelReq != 22 {
		t.Errorf("level = %d, want 22", rec.LevelReq)
	}
	if rec.XPPerAction != 55 {
		t.Errorf("xp = %v, want 55", rec.XPPerAction)
	}
	if !rec.Members {
		t.Error("members = false, want true")
	}
	if len(rec.Inputs) != 2 {
		t.Fatalf("got %d inputs, want 2", len(rec.Inputs))
	}
	if rec.Inputs[0].ItemName != "Gold bar" || rec.Inputs[0].Quantity != 1 {
		t.Errorf("input[0] = %+v, want Gold bar x1", rec.Inputs[0])
	}
	if rec.Inputs[1].ItemName != "Sapphire" {
		t.Errorf("input[1] = %q, want Sapphire", rec.Inputs[1].ItemName)
	}
}

// Actions-per-hour provenance decides whether the frontend can present a
// GP/h figure as a rate or only as a ranking. An absent wiki value must
// not be reported as if the wiki supplied one.
func TestParseWikitextActionsPerHourProvenance(t *testing.T) {
	t.Run("absent leaves source empty", func(t *testing.T) {
		rec := parseWikitext("Sapphire necklace", "", modernInfobox)
		if rec.ActionsPerHour != 0 {
			t.Errorf("aph = %d, want 0 when the wiki has none", rec.ActionsPerHour)
		}
		if rec.APHSource != "" {
			t.Errorf("aph_source = %q, want empty", rec.APHSource)
		}
	})

	t.Run("present is marked as from the wiki", func(t *testing.T) {
		rec := parseWikitext("X", "", modernInfobox+"\n{{Recipe|aph = 1400}}")
		if rec.ActionsPerHour != 1400 {
			t.Errorf("aph = %d, want 1400", rec.ActionsPerHour)
		}
		if rec.APHSource != models.APHSourceWiki {
			t.Errorf("aph_source = %q, want %q", rec.APHSource, models.APHSourceWiki)
		}
	})
}

func TestParseWikitextItemIDFields(t *testing.T) {
	tests := []struct {
		name string
		wt   string
		want int64
	}{
		{"explicit itemid wins", modernInfobox + "\n|itemid = 1656\n|id = 999", 1656},
		{"plain id is a fallback", modernInfobox + "\n|id = 1656", 1656},
		{"absent leaves zero", modernInfobox, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := parseWikitext("Sapphire necklace", "", tc.wt)
			if rec == nil {
				t.Fatal("unexpected nil")
			}
			if rec.OutputItemID != tc.want {
				t.Errorf("output_item_id = %d, want %d", rec.OutputItemID, tc.want)
			}
		})
	}
}

func TestParseWikitextOutputQuantity(t *testing.T) {
	wt := `{{Infobox Recipe
|name = Cannonball
|skill = Smithing
|skill1lvl = 35
|skill1exp = 25.5
|mat1 = [[Steel bar]]
|mat1qty = 1
|output1 = [[Cannonball]]
|output1qty = 4
}}`
	rec := parseWikitext("Cannonball", "", wt)
	if rec == nil {
		t.Fatal("unexpected nil")
	}
	if rec.OutputQty != 4 {
		t.Errorf("output_qty = %d, want 4", rec.OutputQty)
	}
	if rec.XPPerAction != 25.5 {
		t.Errorf("xp = %v, want 25.5", rec.XPPerAction)
	}
}

// Every rejection here corresponds to a class of page that would
// otherwise enter the catalogue as a recipe with nonsense economics.
func TestParseWikitextRejections(t *testing.T) {
	tests := []struct {
		name string
		wt   string
		why  string
	}{
		{
			name: "skill is a boolean placeholder",
			wt:   "{{Infobox Recipe\n|name = Thing\n|skill = No\n|skill1lvl = 5\n|mat1 = [[Coal]]\n}}",
			why:  "a members-style yes/no landed in the skill field",
		},
		{
			name: "level zero",
			wt:   "{{Infobox Recipe\n|name = Thing\n|skill = Crafting\n|skill1lvl = 0\n|mat1 = [[Coal]]\n}}",
			why:  "0 marks an unused optional-skill slot, not a real requirement",
		},
		{
			name: "level above the RS3 cap",
			wt:   "{{Infobox Recipe\n|name = Thing\n|skill = Crafting\n|skill1lvl = 200\n|mat1 = [[Coal]]\n}}",
			why:  "RS3 skills cap at 120, so this is a misparse",
		},
		{
			name: "no inputs",
			wt:   "{{Infobox Recipe\n|name = Thing\n|skill = Crafting\n|skill1lvl = 10\n}}",
			why:  "a recipe with no materials has no cost basis",
		},
		{
			name: "no skill at all",
			wt:   "{{Infobox Item\n|name = Thing\n|id = 5\n}}",
			why:  "an item page is not a recipe",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if rec := parseWikitext("Thing", "", tc.wt); rec != nil {
				t.Errorf("expected rejection (%s), got %+v", tc.why, rec)
			}
		})
	}
}

func TestParseWikitextLevelBoundaries(t *testing.T) {
	const tmpl = "{{Infobox Recipe\n|name = Thing\n|skill = Crafting\n|skill1lvl = %d\n|mat1 = [[Coal]]\n}}"
	for _, tc := range []struct {
		level int
		ok    bool
	}{
		{1, true},
		{120, true},
		{121, false},
		{0, false},
	} {
		rec := parseWikitext("Thing", "", fmt.Sprintf(tmpl, tc.level))
		if tc.ok && rec == nil {
			t.Errorf("level %d should be accepted", tc.level)
		}
		if !tc.ok && rec != nil {
			t.Errorf("level %d should be rejected", tc.level)
		}
	}
}

func TestCollectLegacyMaterials(t *testing.T) {
	wt := `{{Recipe
|name = Old thing
|skill = Smithing
|skill1lvl = 40
|materials =
* 3 [[Iron ore]]
* [[Coal]]
* 2 [[Gold bar]]
|output = Old thing
}}`
	rec := parseWikitext("Old thing", "", wt)
	if rec == nil {
		t.Fatal("legacy materials list should parse")
	}
	if len(rec.Inputs) != 3 {
		t.Fatalf("got %d inputs, want 3", len(rec.Inputs))
	}
	want := []struct {
		name string
		qty  int
	}{{"Iron ore", 3}, {"Coal", 1}, {"Gold bar", 2}}
	for i, w := range want {
		if rec.Inputs[i].ItemName != w.name || rec.Inputs[i].Quantity != w.qty {
			t.Errorf("input[%d] = %s x%d, want %s x%d",
				i, rec.Inputs[i].ItemName, rec.Inputs[i].Quantity, w.name, w.qty)
		}
	}
}

// Material indices are sparse and out of order in real wikitext; the
// parser must key on the index rather than on document order, or
// quantities attach to the wrong material.
func TestCollectNumberedMaterialsOrdering(t *testing.T) {
	wt := `|mat3 = [[Gold bar]]
|mat1 = [[Coal]]
|mat3qty = 5
|mat1qty = 2`
	got := collectNumberedMaterials(wt)
	if len(got) != 2 {
		t.Fatalf("got %d inputs, want 2", len(got))
	}
	if got[0].ItemName != "Coal" || got[0].Quantity != 2 {
		t.Errorf("first = %s x%d, want Coal x2", got[0].ItemName, got[0].Quantity)
	}
	if got[1].ItemName != "Gold bar" || got[1].Quantity != 5 {
		t.Errorf("second = %s x%d, want Gold bar x5", got[1].ItemName, got[1].Quantity)
	}
}

func TestCollectNumberedMaterialsDefaultsQuantityToOne(t *testing.T) {
	got := collectNumberedMaterials("|mat1 = [[Coal]]")
	if len(got) != 1 {
		t.Fatalf("got %d inputs, want 1", len(got))
	}
	if got[0].Quantity != 1 {
		t.Errorf("quantity = %d, want 1", got[0].Quantity)
	}
}

func TestResolveLink(t *testing.T) {
	tests := []struct{ in, want string }{
		{"[[Gold bar]]", "Gold bar"},
		{"[[Gold bar|gold bars]]", "Gold bar"},
		{"  Gold bar  ", "Gold bar"},
		{"* [[Coal]]", "Coal"},
		{"3 x [[Iron ore]]", "Iron ore"},
	}
	for _, tc := range tests {
		if got := resolveLink(tc.in); got != tc.want {
			t.Errorf("resolveLink(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseWikitextReadsTicks(t *testing.T) {
	wt := `{{Infobox Recipe
|ticks = 3
|facility = Furnace
|skill1 = Smithing
|skill1lvl = 20
|skill1exp = 75
|mat1 = Iron ore
|mat1qty = 1
|output1 = Steel bar
}}`
	rec := parseWikitext("Steel bar", "", wt)
	if rec == nil {
		t.Fatal("parseWikitext returned nil")
	}
	if rec.Ticks != 3 {
		t.Errorf("Ticks = %d, want 3", rec.Ticks)
	}
	if rec.Facility != "Furnace" {
		t.Errorf("Facility = %q, want Furnace", rec.Facility)
	}
}

// "varies" is the wiki's way of saying the cost depends on mechanics the
// infobox cannot express. It must not be read as a number.
func TestParseWikitextTreatsVariesAsUnknown(t *testing.T) {
	wt := `{{Infobox Recipe
|ticks = varies
|facility = Anvil
|skill1 = Smithing
|skill1lvl = 50
|skill1exp = 1200
|mat1 = Rune bar
|mat1qty = 5
|output1 = Rune platebody
}}`
	rec := parseWikitext("Rune platebody", "", wt)
	if rec == nil {
		t.Fatal("parseWikitext returned nil")
	}
	if rec.Ticks != 0 {
		t.Errorf("Ticks = %d, want 0 for a varies value", rec.Ticks)
	}
	if rec.Facility != "Anvil" {
		t.Errorf("Facility = %q, want Anvil", rec.Facility)
	}
}

func TestParseWikitextMissingTicksIsZero(t *testing.T) {
	wt := `{{Infobox Recipe
|skill1 = Crafting
|skill1lvl = 5
|skill1exp = 10
|mat1 = Ball of wool
|mat1qty = 1
|output1 = Wool
}}`
	rec := parseWikitext("Wool", "", wt)
	if rec == nil {
		t.Fatal("parseWikitext returned nil")
	}
	if rec.Ticks != 0 {
		t.Errorf("Ticks = %d, want 0", rec.Ticks)
	}
	if rec.Facility != "" {
		t.Errorf("Facility = %q, want empty", rec.Facility)
	}
}

func TestCleanValue(t *testing.T) {
	tests := []struct{ in, want string }{
		{"  Crafting  ", "Crafting"},
		{"Crafting}}", "Crafting"},
		{"[[Crafting]]", "Crafting"},
		{"Crafting|extra", "Crafting"},
	}
	for _, tc := range tests {
		if got := cleanValue(tc.in); got != tc.want {
			t.Errorf("cleanValue(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
