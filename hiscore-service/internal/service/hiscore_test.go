package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/config"
)

func serviceAgainst(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cfg := &config.Config{}
	cfg.Hiscore.BaseURL = srv.URL
	return New(cfg, zap.NewNop(), nil)
}

// The hiscores CSV is positional: row N is skillOrder[N]. A shifted or
// mis-indexed parse silently attributes one skill's levels to another.
func TestFetchFromJagexParsesSkillOrder(t *testing.T) {
	body := strings.Join([]string{
		"1,2750,1000000000", // Overall
		"5,99,13034431",     // Attack
		"7,99,13034431",     // Defence
		"9,99,13034431",     // Strength
		"2,99,20000000",     // Constitution
	}, "\n")

	svc := serviceAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	})

	p, err := svc.FetchFromJagex("Zezima", "normal")
	if err != nil {
		t.Fatalf("FetchFromJagex: %v", err)
	}
	if len(p.Skills) != 5 {
		t.Fatalf("got %d skills, want 5", len(p.Skills))
	}

	want := []struct {
		skill string
		level int
		rank  int64
	}{
		{"Overall", 2750, 1},
		{"Attack", 99, 5},
		{"Defence", 99, 7},
		{"Strength", 99, 9},
		{"Constitution", 99, 2},
	}
	for i, w := range want {
		got := p.Skills[i]
		if got.Skill != w.skill || got.Level != w.level || got.Rank != w.rank {
			t.Errorf("skill[%d] = %s lvl %d rank %d, want %s lvl %d rank %d",
				i, got.Skill, got.Level, got.Rank, w.skill, w.level, w.rank)
		}
	}
	if p.Name != "Zezima" || p.Mode != "normal" {
		t.Errorf("player = %s/%s, want Zezima/normal", p.Name, p.Mode)
	}
	if p.FetchedAt.IsZero() {
		t.Error("fetched_at must be stamped so callers can judge staleness")
	}
}

func TestFetchFromJagexStopsAtKnownSkills(t *testing.T) {
	// Jagex appends activity/minigame rows after the skills; those must
	// not be recorded as skills.
	var rows []string
	for i := 0; i < len(skillOrder)+20; i++ {
		rows = append(rows, "1,50,100000")
	}
	svc := serviceAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Join(rows, "\n")))
	})

	p, err := svc.FetchFromJagex("Someone", "normal")
	if err != nil {
		t.Fatalf("FetchFromJagex: %v", err)
	}
	if len(p.Skills) != len(skillOrder) {
		t.Errorf("got %d skills, want %d", len(p.Skills), len(skillOrder))
	}
}

func TestFetchFromJagexUnknownMode(t *testing.T) {
	svc := serviceAgainst(t, func(w http.ResponseWriter, r *http.Request) {})

	if _, err := svc.FetchFromJagex("Zezima", "ultimate"); err == nil {
		t.Fatal("expected an error for an unrecognised mode")
	}
}

func TestFetchFromJagexModeMapsToTable(t *testing.T) {
	paths := map[string]string{
		"normal":   "m=hiscore/",
		"ironman":  "m=hiscore_ironman/",
		"hardcore": "m=hiscore_hardcore_ironman/",
	}
	for mode, want := range paths {
		var gotPath string
		svc := serviceAgainst(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_, _ = w.Write([]byte("1,50,100000"))
		})
		if _, err := svc.FetchFromJagex("X", mode); err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if !strings.Contains(gotPath, want) {
			t.Errorf("%s hit %q, want it to contain %q", mode, gotPath, want)
		}
	}
}

func TestFetchFromJagexNotFound(t *testing.T) {
	svc := serviceAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := svc.FetchFromJagex("Nobody", "normal")
	if err == nil {
		t.Fatal("expected an error for an unknown player")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to say the player was not found", err)
	}
}

func TestFetchFromJagexEmptyBody(t *testing.T) {
	svc := serviceAgainst(t, func(w http.ResponseWriter, r *http.Request) {})

	if _, err := svc.FetchFromJagex("Ghost", "normal"); err == nil {
		t.Fatal("expected an error when no skills could be parsed")
	}
}

func TestFetchFromJagexEscapesPlayerName(t *testing.T) {
	var gotQuery string
	svc := serviceAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("player")
		_, _ = w.Write([]byte("1,50,100000"))
	})

	if _, err := svc.FetchFromJagex("Two Words", "normal"); err != nil {
		t.Fatalf("FetchFromJagex: %v", err)
	}
	if gotQuery != "Two Words" {
		t.Errorf("player = %q, want the name round-tripped intact", gotQuery)
	}
}

func TestSkillOrderMatchesRS3(t *testing.T) {
	// The order is positional in the upstream CSV; reordering it
	// silently relabels every player's skills.
	if skillOrder[0] != "Overall" {
		t.Errorf("first entry = %q, want Overall", skillOrder[0])
	}
	if last := skillOrder[len(skillOrder)-1]; last != "Necromancy" {
		t.Errorf("last entry = %q, want Necromancy", last)
	}
	seen := map[string]bool{}
	for _, s := range skillOrder {
		if seen[s] {
			t.Errorf("duplicate skill %q", s)
		}
		seen[s] = true
	}
}
