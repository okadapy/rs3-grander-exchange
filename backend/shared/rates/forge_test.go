package rates

import (
	"testing"
	"time"
)

// Progress required is the bar count times a per-metal constant. Checked
// against three wiki pages: rune platebody 5 bars x 600 = 3000, rune
// dagger 2 x 600 = 1200, steel platebody 5 x 300 = 1500.
func TestForgeProgressPerBar(t *testing.T) {
	cases := map[string]int{"Bronze": 100, "Steel": 300, "Rune": 600, "Primal": 1100}
	for metal, want := range cases {
		if got := forgeProgressPerBar[metal]; got != want {
			t.Errorf("forgeProgressPerBar[%s] = %d, want %d", metal, got, want)
		}
	}
}

func TestForgeTicksUnknownMetal(t *testing.T) {
	if _, ok := ForgeTicks(5, "Cheese", 99, 99, Boosts{}); ok {
		t.Error("ForgeTicks accepted a metal that is not in the table")
	}
}

func TestForgeTicksBelowWorkingLevel(t *testing.T) {
	if _, ok := ForgeTicks(5, "Rune", 49, 99, Boosts{}); ok {
		t.Error("ForgeTicks reported a rate below the level requirement")
	}
}

// A strike is 2 ticks and lands 20 progress at high heat, so 1500
// progress is at least 75 strikes, and reheats only add ticks. The point
// is that the simulation lands in the right order of magnitude rather
// than returning something arbitrary.
func TestForgeTicksSteelPlatebodyIsPlausible(t *testing.T) {
	got, ok := ForgeTicks(5, "Steel", 99, 99, Boosts{})
	if !ok {
		t.Fatal("ForgeTicks(steel platebody at 99/99) not ok")
	}
	if got < 150 {
		t.Errorf("ForgeTicks = %d ticks, below the 150 a perfect run needs", got)
	}
	if got > 600 {
		t.Errorf("ForgeTicks = %d ticks, implausibly slow", got)
	}
}

// More heat means more strikes before a reheat, so a higher Firemaking
// level can only help.
func TestForgeTicksImproveWithFiremaking(t *testing.T) {
	low, ok1 := ForgeTicks(5, "Rune", 99, 1, Boosts{})
	high, ok2 := ForgeTicks(5, "Rune", 99, 99, Boosts{})
	if !ok1 || !ok2 {
		t.Fatal("ForgeTicks not ok")
	}
	if high > low {
		t.Errorf("ForgeTicks with Firemaking 99 = %d, slower than with 1 = %d", high, low)
	}
}

// Boosts are the whole reason the parameter exists: they must move the
// number.
func TestForgeTicksImproveWithBoosts(t *testing.T) {
	plain, _ := ForgeTicks(5, "Rune", 99, 99, Boosts{})
	boosted, _ := ForgeTicks(5, "Rune", 99, 99,
		Boosts{BaseProgressBonus: 10, DoubleProgressPct: 34.4})
	if boosted >= plain {
		t.Errorf("boosted run took %d ticks, no better than the plain %d", boosted, plain)
	}
}

func TestParseBoostsKnownTokens(t *testing.T) {
	b, err := ParseBoosts("smithing_cape,careless5,luminite")
	if err != nil {
		t.Fatalf("ParseBoosts: %v", err)
	}
	if b.BaseProgressBonus != 11 {
		t.Errorf("BaseProgressBonus = %d, want 11", b.BaseProgressBonus)
	}
}

func TestParseBoostsEquipmentLevelRaisesRapidAndTinker(t *testing.T) {
	plain, err := ParseBoosts("rapid4,tinker2")
	if err != nil {
		t.Fatalf("ParseBoosts: %v", err)
	}
	upgraded, err := ParseBoosts("rapid4,tinker2,equipment20")
	if err != nil {
		t.Fatalf("ParseBoosts: %v", err)
	}
	if upgraded.DoubleProgressPct <= plain.DoubleProgressPct {
		t.Errorf("equipment20 did not raise the double-progress chance: %v vs %v",
			upgraded.DoubleProgressPct, plain.DoubleProgressPct)
	}
}

// A mistyped checkbox must not silently produce a number that looks
// fine.
func TestParseBoostsRejectsUnknownToken(t *testing.T) {
	if _, err := ParseBoosts("rapid4,rapid9"); err == nil {
		t.Error("ParseBoosts accepted an unknown token")
	}
}

// ParseBoosts never emits a negative BaseProgressBonus or DoubleProgressPct,
// but Boosts is an exported struct, so a caller outside this package (a
// hand-built test fixture, a future call site that skips ParseBoosts) can
// construct one directly with negative fields. Without the per-strike gain
// floor in ForgeTicks, such a Boosts would drive progress-per-strike to zero
// or negative and the simulation loop would never terminate. Run the call on
// a goroutine with a timeout so a regression here fails the test instead of
// hanging the suite.
func TestForgeTicksTerminatesWithPathologicalBoosts(t *testing.T) {
	pathological := Boosts{BaseProgressBonus: -100, DoubleProgressPct: -100}

	type result struct {
		ticks int
		ok    bool
	}
	done := make(chan result, 1)
	go func() {
		ticks, ok := ForgeTicks(5, "Rune", 99, 99, pathological)
		done <- result{ticks, ok}
	}()

	select {
	case r := <-done:
		if !r.ok {
			t.Fatal("ForgeTicks(pathological Boosts) not ok")
		}
		if r.ticks <= 0 {
			t.Errorf("ForgeTicks(pathological Boosts) = %d ticks, want a positive count", r.ticks)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ForgeTicks(pathological Boosts) did not return within 5s, likely an infinite loop")
	}
}

func TestParseBoostsEmptyIsTheConservativeDefault(t *testing.T) {
	b, err := ParseBoosts("")
	if err != nil {
		t.Fatalf("ParseBoosts(\"\"): %v", err)
	}
	if b.BaseProgressBonus != 0 || b.DoubleProgressPct != 0 {
		t.Errorf("empty boosts = %+v, want zero", b)
	}
}
