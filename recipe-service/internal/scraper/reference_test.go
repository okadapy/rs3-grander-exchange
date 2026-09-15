package scraper

import "testing"

// The wiki reference tables mix real entries with %PERCENT_DELIMITED%
// metadata. Letting a metadata key through would create a bogus item
// named "%LAST_UPDATE%"; letting a zero through would create an entry
// that looks resolved but prices at nothing.
func TestParseWikiIDMapSkipsMetadataAndJunk(t *testing.T) {
	body := []byte(`{
		"%LAST_UPDATE%": 1789456541,
		"%LAST_UPDATE_F%": "15 September 2026 07:15:41 (UTC)",
		"Abyssal whip": 4151,
		"Cannonball": 2,
		"Quoted id": "1513",
		"Zero id": 0,
		"Negative id": -5,
		"Object value": {"nested": 1},
		"": 99
	}`)

	got, err := parseWikiIDMap(body)
	if err != nil {
		t.Fatalf("parseWikiIDMap: %v", err)
	}

	want := map[string]int64{
		"Abyssal whip": 4151,
		"Cannonball":   2,
		"Quoted id":    1513,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries %v, want %d", len(got), got, len(want))
	}
	for name, id := range want {
		if got[name] != id {
			t.Errorf("%q = %d, want %d", name, got[name], id)
		}
	}
	for _, rejected := range []string{"%LAST_UPDATE%", "Zero id", "Negative id", "Object value", ""} {
		if _, ok := got[rejected]; ok {
			t.Errorf("%q should have been skipped", rejected)
		}
	}
}

func TestParseWikiIDMapTrimsNames(t *testing.T) {
	got, err := parseWikiIDMap([]byte(`{"  Coal  ": 453}`))
	if err != nil {
		t.Fatalf("parseWikiIDMap: %v", err)
	}
	if got["Coal"] != 453 {
		t.Errorf("names should be trimmed, got %v", got)
	}
}

func TestParseWikiIDMapRejectsNonObject(t *testing.T) {
	if _, err := parseWikiIDMap([]byte(`[1,2,3]`)); err == nil {
		t.Fatal("expected an error for a non-object body")
	}
}

// Buy limits are parsed by the same routine as item IDs; this pins the
// values we verified against the live wiki table.
func TestParseWikiIDMapBuyLimitShape(t *testing.T) {
	body := []byte(`{"%LAST_UPDATE%":1,"Abyssal whip":10,"Coal":25000,"Gold bar":10000}`)
	got, err := parseWikiIDMap(body)
	if err != nil {
		t.Fatalf("parseWikiIDMap: %v", err)
	}
	if got["Abyssal whip"] != 10 {
		t.Errorf("whip limit = %d, want 10", got["Abyssal whip"])
	}
	if got["Coal"] != 25000 {
		t.Errorf("coal limit = %d, want 25000", got["Coal"])
	}
}
