package database

import (
	"context"
	"fmt"
	"strings"
)

// IntRange is an inclusive numeric filter. Nil bounds are unbounded.
type IntRange struct {
	Min, Max *int
}

// SearchFilter combines optional FTS text with SQL facet predicates.
type SearchFilter struct {
	Query        string
	SetIDs       []string
	Types        []string
	Supertypes   []string
	Rarities     []string
	Domains      []string
	DomainAny    bool // false = ALL (default), true = ANY
	Energy       IntRange
	Might        IntRange
	Power        IntRange
	Signature    *bool
	AlternateArt *bool
	New          *bool
	Limit        int
	Offset       int
}

// HasFacets reports whether any non-text facet is set.
func (f SearchFilter) HasFacets() bool {
	return len(f.SetIDs) > 0 ||
		len(f.Types) > 0 ||
		len(f.Supertypes) > 0 ||
		len(f.Rarities) > 0 ||
		len(f.Domains) > 0 ||
		f.Energy.Min != nil || f.Energy.Max != nil ||
		f.Might.Min != nil || f.Might.Max != nil ||
		f.Power.Min != nil || f.Power.Max != nil ||
		f.Signature != nil ||
		f.AlternateArt != nil ||
		f.New != nil
}

func (f SearchFilter) normalized() SearchFilter {
	out := f
	out.Query = strings.TrimSpace(f.Query)
	out.SetIDs = normalizeList(f.SetIDs)
	out.Types = normalizeList(f.Types)
	out.Supertypes = normalizeList(f.Supertypes)
	out.Rarities = normalizeList(f.Rarities)
	out.Domains = normalizeList(f.Domains)
	if out.Limit <= 0 {
		out.Limit = 20
	}
	if out.Offset < 0 {
		out.Offset = 0
	}
	return out
}

func normalizeList(vals []string) []string {
	var out []string
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func buildFacetSQL(f SearchFilter) (string, []any) {
	var parts []string
	var args []any

	appendIn := func(expr string, vals []string) {
		if len(vals) == 0 {
			return
		}
		placeholders := make([]string, len(vals))
		for i, v := range vals {
			placeholders[i] = "?"
			args = append(args, v)
		}
		parts = append(parts, expr+" IN ("+strings.Join(placeholders, ", ")+")")
	}

	appendIn("upper(c.set_id)", upperAll(f.SetIDs))
	appendIn("lower(c.type)", lowerAll(f.Types))
	appendIn("lower(c.rarity)", lowerAll(f.Rarities))
	appendIn("lower(COALESCE(c.supertype,''))", lowerAll(f.Supertypes))

	if len(f.Domains) > 0 {
		if f.DomainAny {
			placeholders := make([]string, len(f.Domains))
			for i, d := range f.Domains {
				placeholders[i] = "?"
				args = append(args, strings.ToLower(d))
			}
			parts = append(parts, `EXISTS (
  SELECT 1 FROM json_each(c.domains) j
  WHERE lower(j.value) IN (`+strings.Join(placeholders, ", ")+`)
)`)
		} else {
			for _, d := range f.Domains {
				parts = append(parts, `EXISTS (
  SELECT 1 FROM json_each(c.domains) j
  WHERE lower(j.value) = lower(?)
)`)
				args = append(args, d)
			}
		}
	}

	appendRange := func(col string, r IntRange) {
		if r.Min != nil {
			parts = append(parts, fmt.Sprintf("(c.%s IS NOT NULL AND c.%s >= ?)", col, col))
			args = append(args, *r.Min)
		}
		if r.Max != nil {
			parts = append(parts, fmt.Sprintf("(c.%s IS NOT NULL AND c.%s <= ?)", col, col))
			args = append(args, *r.Max)
		}
	}
	appendRange("energy", f.Energy)
	appendRange("might", f.Might)
	appendRange("power", f.Power)

	appendBool := func(col string, v *bool) {
		if v == nil {
			return
		}
		parts = append(parts, "c."+col+" = ?")
		args = append(args, boolInt(*v))
	}
	appendBool("signature", f.Signature)
	appendBool("alternate_art", f.AlternateArt)
	appendBool("new", f.New)

	if len(parts) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(parts, " AND "), args
}

func lowerAll(vals []string) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = strings.ToLower(v)
	}
	return out
}

func upperAll(vals []string) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = strings.ToUpper(v)
	}
	return out
}

// Search runs optional FTS plus facet filters.
func (db *DB) Search(ctx context.Context, f SearchFilter) ([]SearchResult, error) {
	f = f.normalized()
	facetSQL, facetArgs := buildFacetSQL(f)

	var (
		query string
		args  []any
	)
	if f.Query != "" {
		args = append(args, ftsQuery(f.Query))
		args = append(args, facetArgs...)
		args = append(args, f.Limit, f.Offset)
		query = `
			SELECT ` + prefixedCardColumns("c") + `, f.rank
			FROM cards_fts f
			JOIN cards c ON c.id = f.card_id
			WHERE cards_fts MATCH ?` + facetSQL + `
			ORDER BY f.rank
			LIMIT ? OFFSET ?
		`
	} else {
		args = append(args, facetArgs...)
		args = append(args, f.Limit, f.Offset)
		where := "1=1" + facetSQL
		query = `
			SELECT ` + prefixedCardColumns("c") + `, 0 AS rank
			FROM cards c
			WHERE ` + where + `
			ORDER BY c.set_id, c.collector_number, c.variant
			LIMIT ? OFFSET ?
		`
	}

	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SearchResult
	for rows.Next() {
		c, rank, err := scanSearchResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, SearchResult{Card: c, Rank: rank})
	}
	return out, rows.Err()
}

// CountFiltered returns how many cards match the filter (ignores Limit/Offset).
func (db *DB) CountFiltered(ctx context.Context, f SearchFilter) (int, error) {
	f = f.normalized()
	facetSQL, facetArgs := buildFacetSQL(f)

	var (
		query string
		args  []any
	)
	if f.Query != "" {
		args = append(args, ftsQuery(f.Query))
		args = append(args, facetArgs...)
		query = `
			SELECT COUNT(*)
			FROM cards_fts f
			JOIN cards c ON c.id = f.card_id
			WHERE cards_fts MATCH ?` + facetSQL
	} else {
		args = append(args, facetArgs...)
		query = `SELECT COUNT(*) FROM cards c WHERE 1=1` + facetSQL
	}

	var n int
	err := db.sql.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

// DistinctValues returns sorted unique values for browse pickers.
// column is one of: type, supertype, rarity, set_id, domain.
func (db *DB) DistinctValues(ctx context.Context, column string) ([]string, error) {
	var query string
	switch strings.ToLower(strings.TrimSpace(column)) {
	case "type":
		query = `SELECT DISTINCT type FROM cards WHERE COALESCE(type,'') <> '' ORDER BY type COLLATE NOCASE`
	case "supertype":
		query = `SELECT DISTINCT supertype FROM cards WHERE COALESCE(supertype,'') <> '' ORDER BY supertype COLLATE NOCASE`
	case "rarity":
		query = `SELECT DISTINCT rarity FROM cards WHERE COALESCE(rarity,'') <> '' ORDER BY rarity COLLATE NOCASE`
	case "set_id", "set":
		query = `SELECT DISTINCT set_id FROM cards WHERE COALESCE(set_id,'') <> '' ORDER BY set_id COLLATE NOCASE`
	case "domain", "domains":
		query = `
			SELECT DISTINCT j.value
			FROM cards c, json_each(c.domains) j
			WHERE COALESCE(j.value,'') <> ''
			ORDER BY j.value COLLATE NOCASE
		`
	default:
		return nil, fmt.Errorf("unsupported distinct column: %q", column)
	}

	rows, err := db.sql.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
