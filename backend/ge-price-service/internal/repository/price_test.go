package repository

import (
	"testing"

	"github.com/rs3-market/backend/shared/models"
)

// The poller stamps an entire cycle with one timestamp, so a double-run
// leaves two rows tied for "newest". The join cannot break the tie, and
// a GROUP BY would be rejected under MySQL's ONLY_FULL_GROUP_BY, so the
// tie is broken here instead.
func TestDedupeByItemKeepsNewestRow(t *testing.T) {
	rows := []models.PriceSnapshot{
		{ID: 1, ItemID: 100, Price: 500},
		{ID: 2, ItemID: 200, Price: 900},
		{ID: 3, ItemID: 100, Price: 550}, // same item, later row
	}

	got := dedupeByItem(rows)
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 (one per item)", len(got))
	}
	if got[0].ItemID != 100 || got[0].Price != 550 {
		t.Errorf("item 100 = %+v, want the higher-id row (price 550)", got[0])
	}
	if got[1].ItemID != 200 {
		t.Errorf("second row = item %d, want 200 in first-seen order", got[1].ItemID)
	}
}

func TestDedupeByItemPassesThroughSmallInputs(t *testing.T) {
	if got := dedupeByItem(nil); got != nil {
		t.Errorf("nil should pass through, got %v", got)
	}
	one := []models.PriceSnapshot{{ID: 1, ItemID: 100}}
	if got := dedupeByItem(one); len(got) != 1 {
		t.Errorf("single row should pass through, got %v", got)
	}
}

func TestDedupeByItemNoDuplicatesIsIdentity(t *testing.T) {
	rows := []models.PriceSnapshot{
		{ID: 1, ItemID: 100}, {ID: 2, ItemID: 200}, {ID: 3, ItemID: 300},
	}
	got := dedupeByItem(rows)
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3", len(got))
	}
	for i := range rows {
		if got[i].ItemID != rows[i].ItemID {
			t.Errorf("order changed at %d: %d vs %d", i, got[i].ItemID, rows[i].ItemID)
		}
	}
}
