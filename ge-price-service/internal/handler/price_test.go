package handler

import (
	"strings"
	"testing"
	"time"
)

// Silently dropping a malformed ID makes a client typo indistinguishable
// from an item that simply has no price data.
func TestParseIDs(t *testing.T) {
	t.Run("valid list", func(t *testing.T) {
		got, err := parseIDs("4151, 2 ,1513")
		if err != nil {
			t.Fatalf("parseIDs: %v", err)
		}
		want := []int64{4151, 2, 1513}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	})

	t.Run("rejections", func(t *testing.T) {
		for _, raw := range []string{"", "   ", "abc", "4151,abc", "-1", "0", "4151,0"} {
			if _, err := parseIDs(raw); err == nil {
				t.Errorf("parseIDs(%q) should have failed", raw)
			}
		}
	})

	t.Run("too many", func(t *testing.T) {
		ids := strings.TrimSuffix(strings.Repeat("1,", maxIDsPerRequest+1), ",")
		_, err := parseIDs(ids)
		if err == nil {
			t.Fatal("expected a limit on the number of IDs")
		}
		if !strings.Contains(err.Error(), "too many") {
			t.Errorf("error = %q, want it to explain the cap", err)
		}
	})

	t.Run("trailing comma tolerated", func(t *testing.T) {
		got, err := parseIDs("4151,")
		if err != nil {
			t.Fatalf("parseIDs: %v", err)
		}
		if len(got) != 1 || got[0] != 4151 {
			t.Errorf("got %v, want [4151]", got)
		}
	})
}

func TestParseWindow(t *testing.T) {
	def := 24 * time.Hour

	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", def, false},
		{"24h", 24 * time.Hour, false},
		{"168h", 168 * time.Hour, false},
		{"90m", 90 * time.Minute, false},
		{"48", 48 * time.Hour, false}, // bare number means hours
		{"0", 0, true},
		{"-5h", 0, true},
		{"7d", 0, true}, // not a Go duration
		{"nonsense", 0, true},
	}

	for _, tc := range tests {
		got, err := parseWindow(tc.in, def)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseWindow(%q) should have failed", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseWindow(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseWindow(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseTimeFallsBackToDefault(t *testing.T) {
	def := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	if got := parseTime("", def); !got.Equal(def) {
		t.Errorf("empty = %v, want the default", got)
	}
	if got := parseTime("garbage", def); !got.Equal(def) {
		t.Errorf("garbage = %v, want the default", got)
	}

	want := time.Date(2026, 9, 15, 7, 15, 41, 0, time.UTC)
	if got := parseTime("2026-09-15T07:15:41Z", def); !got.Equal(want) {
		t.Errorf("parsed = %v, want %v", got, want)
	}
}
