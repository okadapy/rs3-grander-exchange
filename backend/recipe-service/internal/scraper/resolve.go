package scraper

import (
	"fmt"

	"go.uber.org/zap"
)

// IDResolver is the set of ID-resolution passes, in the order they have
// to run. Satisfied by *repository.Repo; an interface so the ordering
// can be tested without a database.
type IDResolver interface {
	// BackfillOutputsFromGEIDs fills in recipe output IDs from the wiki
	// item ID map.
	BackfillOutputsFromGEIDs() (int64, error)
	// BackfillInputItemIDs fills in input IDs from the outputs of
	// recipes that produce them.
	BackfillInputItemIDs() (int64, error)
	// BackfillInputsFromGEIDs fills in whatever inputs are left from the
	// wiki item ID map — the gathered materials no recipe produces.
	BackfillInputsFromGEIDs() (int64, error)
}

// ScrapeRunner is the scraping half of a cycle.
type ScrapeRunner interface {
	ScrapeAll() (int, error)
}

// Resolution reports how many rows each pass filled in.
type Resolution struct {
	Outputs           int64
	InputsFromRecipes int64
	InputsFromGEIDs   int64
}

// ResolveIDs runs the three passes in dependency order.
//
// Outputs first: a recipe whose own output ID is unknown is invisible to
// the price poller and to every listing endpoint. Inputs-from-recipes
// second, because it matches input names against output names and so
// only sees the outputs already resolved. Inputs-from-GEIDs last, for
// the gathered materials no recipe produces.
func ResolveIDs(r IDResolver, log *zap.Logger) (Resolution, error) {
	var res Resolution
	var err error

	if res.Outputs, err = r.BackfillOutputsFromGEIDs(); err != nil {
		return res, fmt.Errorf("resolve outputs from GEIDs: %w", err)
	}
	if res.InputsFromRecipes, err = r.BackfillInputItemIDs(); err != nil {
		return res, fmt.Errorf("resolve inputs from recipes: %w", err)
	}
	if res.InputsFromGEIDs, err = r.BackfillInputsFromGEIDs(); err != nil {
		return res, fmt.Errorf("resolve inputs from GEIDs: %w", err)
	}

	log.Info("resolved item ids",
		zap.Int64("outputs", res.Outputs),
		zap.Int64("inputs_from_recipes", res.InputsFromRecipes),
		zap.Int64("inputs_from_geids", res.InputsFromGEIDs))
	return res, nil
}

// RunScrapeCycle scrapes and then re-resolves the IDs the scrape just
// cleared.
//
// The two belong together: upserting a recipe deletes its inputs and
// inserts them fresh, with no item ID, so a scrape that is not followed
// by resolution leaves every input at zero — unpriced, and costed as
// free by anything downstream.
func RunScrapeCycle(s ScrapeRunner, r IDResolver, log *zap.Logger) (int, Resolution, error) {
	n, err := s.ScrapeAll()
	if err != nil {
		return n, Resolution{}, err
	}

	res, err := ResolveIDs(r, log)
	if err != nil {
		return n, res, err
	}
	return n, res, nil
}
