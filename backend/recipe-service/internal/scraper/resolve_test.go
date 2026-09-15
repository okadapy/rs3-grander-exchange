package scraper

import (
	"errors"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// fakeResolver records which passes ran, in order.
type fakeResolver struct {
	calls []string
	fail  string
}

func (f *fakeResolver) pass(name string, rows int64) (int64, error) {
	f.calls = append(f.calls, name)
	if f.fail == name {
		return 0, errors.New(name + " failed")
	}
	return rows, nil
}

func (f *fakeResolver) BackfillOutputsFromGEIDs() (int64, error) {
	return f.pass("outputs-from-geids", 5)
}
func (f *fakeResolver) BackfillInputItemIDs() (int64, error) {
	return f.pass("inputs-from-recipes", 7)
}
func (f *fakeResolver) BackfillInputsFromGEIDs() (int64, error) {
	return f.pass("inputs-from-geids", 11)
}

type fakeScraper struct {
	upserted int
	err      error
	ran      bool
}

func (f *fakeScraper) ScrapeAll() (int, error) {
	f.ran = true
	return f.upserted, f.err
}

// Outputs have to be resolved before inputs: the inputs-from-recipes
// pass matches an input name against recipe output names and copies the
// output's ID, so an output resolved later in the run is an input the
// earlier pass could not see.
func TestResolveIDsRunsPassesInDependencyOrder(t *testing.T) {
	f := &fakeResolver{}

	if _, err := ResolveIDs(f, zap.NewNop()); err != nil {
		t.Fatalf("ResolveIDs: %v", err)
	}

	want := "outputs-from-geids,inputs-from-recipes,inputs-from-geids"
	if got := strings.Join(f.calls, ","); got != want {
		t.Errorf("passes ran %q, want %q", got, want)
	}
}

func TestResolveIDsReportsRowsPerPass(t *testing.T) {
	res, err := ResolveIDs(&fakeResolver{}, zap.NewNop())
	if err != nil {
		t.Fatalf("ResolveIDs: %v", err)
	}
	if res.Outputs != 5 || res.InputsFromRecipes != 7 || res.InputsFromGEIDs != 11 {
		t.Errorf("resolution = %+v, want {5 7 11}", res)
	}
}

// A failing pass must not be reported as a completed resolution, and
// must not leave the later passes silently unrun without a word.
func TestResolveIDsStopsOnAFailingPass(t *testing.T) {
	f := &fakeResolver{fail: "outputs-from-geids"}

	if _, err := ResolveIDs(f, zap.NewNop()); err == nil {
		t.Fatal("ResolveIDs returned nil error after a pass failed")
	}
	if got := strings.Join(f.calls, ","); got != "outputs-from-geids" {
		t.Errorf("passes ran %q, want the run to stop at the failure", got)
	}
}

// The bug this guards: a scrape rewrites every recipe's inputs from
// scratch, which clears the item IDs resolution had filled in. With
// resolution running only at startup and once a day, every scrape left
// the whole input set back at zero — and the daily reference refresh
// ran minutes before the daily scrape, so it never recovered.
func TestScrapeCycleResolvesIDsTheScrapeCleared(t *testing.T) {
	sc := &fakeScraper{upserted: 100}
	f := &fakeResolver{}

	n, res, err := RunScrapeCycle(sc, f, zap.NewNop())
	if err != nil {
		t.Fatalf("RunScrapeCycle: %v", err)
	}
	if n != 100 {
		t.Errorf("upserted = %d, want 100", n)
	}
	if !sc.ran {
		t.Fatal("the scrape never ran")
	}
	if len(f.calls) == 0 {
		t.Fatal("no IDs were resolved after the scrape cleared them")
	}
	if res.InputsFromGEIDs != 11 {
		t.Errorf("resolution = %+v, want the input passes to have run", res)
	}
}

// Nothing was rewritten, so there is nothing to re-resolve; running the
// passes anyway is pure load on the database.
func TestScrapeCycleSkipsResolutionWhenTheScrapeFails(t *testing.T) {
	sc := &fakeScraper{err: errors.New("wiki unreachable")}
	f := &fakeResolver{}

	if _, _, err := RunScrapeCycle(sc, f, zap.NewNop()); err == nil {
		t.Fatal("RunScrapeCycle hid a failing scrape")
	}
	if len(f.calls) != 0 {
		t.Errorf("resolution ran after a failed scrape: %v", f.calls)
	}
}
