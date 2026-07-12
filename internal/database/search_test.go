package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/slh/rift-ctl/internal/cards"
)

func TestSearchFacets(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	seed := []struct {
		id, name, typ, super, rarity, set string
		domains                           []string
		energy, might                     *int
		text                              string
	}{
		{"unl-001-1", "Fury Unit", "Unit", "Champion", "Rare", "UNL", []string{"Fury"}, intPtrVal(3), intPtrVal(4), "deal damage"},
		{"unl-002-2", "Calm Spell", "Spell", "", "Common", "UNL", []string{"Calm"}, intPtrVal(2), nil, "draw a card"},
		{"unl-003-3", "Dual Unit", "Unit", "Basic", "Epic", "UNL", []string{"Fury", "Calm"}, intPtrVal(4), intPtrVal(5), "deal damage and draw"},
		{"ogn-001-4", "Mind Unit", "Unit", "Token", "Rare", "OGN", []string{"Mind"}, intPtrVal(1), intPtrVal(2), "accelerate"},
		{"ogn-002-5", "Fury Mind", "Unit", "", "Rare", "OGN", []string{"Fury", "Mind"}, intPtrVal(5), intPtrVal(6), "deal damage"},
	}
	for _, c := range seed {
		err := db.UpsertCard(ctx, testCard(c.id, c.name, c.typ, c.super, c.rarity, c.set, c.domains, c.energy, c.might, c.text))
		if err != nil {
			t.Fatalf("upsert %s: %v", c.id, err)
		}
	}
	if err := db.RebuildSearchIndex(ctx); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	t.Run("domain ALL requires every domain", func(t *testing.T) {
		got, err := db.Search(ctx, SearchFilter{Domains: []string{"Fury", "Calm"}, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		assertIDs(t, got, "unl-003-3")
	})

	t.Run("domain ANY matches either domain", func(t *testing.T) {
		got, err := db.Search(ctx, SearchFilter{Domains: []string{"Fury", "Calm"}, DomainAny: true, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		assertIDs(t, got, "ogn-002-5", "unl-001-1", "unl-002-2", "unl-003-3")
	})

	t.Run("filter-only type and rarity", func(t *testing.T) {
		got, err := db.Search(ctx, SearchFilter{Types: []string{"Unit"}, Rarities: []string{"Rare"}, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		assertIDs(t, got, "ogn-001-4", "ogn-002-5", "unl-001-1")
	})

	t.Run("FTS plus facet", func(t *testing.T) {
		got, err := db.Search(ctx, SearchFilter{Query: "deal", Types: []string{"Unit"}, Domains: []string{"Fury"}, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		assertIDSet(t, got, "ogn-002-5", "unl-001-1", "unl-003-3")
	})

	t.Run("set and energy range", func(t *testing.T) {
		min, max := 2, 3
		got, err := db.Search(ctx, SearchFilter{SetIDs: []string{"UNL"}, Energy: IntRange{Min: &min, Max: &max}, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		assertIDs(t, got, "unl-001-1", "unl-002-2")
	})

	t.Run("CountFiltered ignores limit", func(t *testing.T) {
		n, err := db.CountFiltered(ctx, SearchFilter{Types: []string{"Unit"}, Limit: 1})
		if err != nil {
			t.Fatal(err)
		}
		if n != 4 {
			t.Fatalf("CountFiltered = %d, want 4", n)
		}
	})

	t.Run("DistinctValues domains", func(t *testing.T) {
		got, err := db.DistinctValues(ctx, "domain")
		if err != nil {
			t.Fatal(err)
		}
		assertStrings(t, got, []string{"Calm", "Fury", "Mind"})
	})

	t.Run("offset pagination", func(t *testing.T) {
		got, err := db.Search(ctx, SearchFilter{Types: []string{"Unit"}, Limit: 1, Offset: 1})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		// filter-only order: set_id, collector_number, variant → ogn-001, ogn-002, unl-001, unl-003
		if got[0].Card.ID != "ogn-002-5" {
			t.Fatalf("got %s, want ogn-002-5", got[0].Card.ID)
		}
	})
}

func TestSearchEnergyMin(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	e2, e5 := 2, 5
	if err := db.UpsertCard(ctx, testCard("a-1-1", "A", "Unit", "", "Common", "UNL", nil, &e2, nil, "")); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertCard(ctx, testCard("a-2-2", "B", "Unit", "", "Common", "UNL", nil, &e5, nil, "")); err != nil {
		t.Fatal(err)
	}
	if err := db.RebuildSearchIndex(ctx); err != nil {
		t.Fatal(err)
	}

	min := 5
	got, err := db.Search(ctx, SearchFilter{Energy: IntRange{Min: &min}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	assertIDs(t, got, "a-2-2")
}

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cards.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func testCard(id, name, typ, super, rarity, set string, domains []string, energy, might *int, text string) cards.Card {
	c := cards.Card{
		ID:              id,
		RiftcodexID:     id,
		Name:            name,
		Type:            typ,
		Supertype:       super,
		Rarity:          rarity,
		SetID:           set,
		Domains:         domains,
		Energy:          energy,
		Might:           might,
		TextPlain:       text,
		CollectorNumber: collectorFromID(id),
		APIJSON:         `{}`,
	}
	_ = c.Derive()
	return c
}

func assertIDs(t *testing.T, results []SearchResult, want ...string) {
	t.Helper()
	got := make([]string, len(results))
	for i, r := range results {
		got[i] = r.Card.ID
	}
	if len(got) != len(want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %v, want %v", got, want)
		}
	}
}

func assertIDSet(t *testing.T, results []SearchResult, want ...string) {
	t.Helper()
	got := make(map[string]struct{}, len(results))
	for _, r := range results {
		got[r.Card.ID] = struct{}{}
	}
	if len(got) != len(want) {
		ids := make([]string, 0, len(got))
		for id := range got {
			ids = append(ids, id)
		}
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for _, id := range want {
		if _, ok := got[id]; !ok {
			t.Fatalf("missing id %s in results", id)
		}
	}
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func intPtrVal(n int) *int { return &n }

func collectorFromID(id string) int {
	parts := splitID(id)
	if len(parts) < 2 {
		return 0
	}
	n := 0
	for _, ch := range parts[1] {
		if ch < '0' || ch > '9' {
			break
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

func splitID(id string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(id); i++ {
		if id[i] == '-' {
			parts = append(parts, id[start:i])
			start = i + 1
		}
	}
	parts = append(parts, id[start:])
	return parts
}
