package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/slh/rift-ctl/internal/cards"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS cards (
	id TEXT PRIMARY KEY,
	riftcodex_id TEXT NOT NULL,
	name TEXT NOT NULL,
	tcgplayer_id TEXT,
	collector_number INTEGER NOT NULL,
	orientation TEXT,
	energy INTEGER,
	might INTEGER,
	power INTEGER,
	type TEXT,
	supertype TEXT,
	rarity TEXT,
	domains TEXT NOT NULL DEFAULT '[]',
	text_plain TEXT NOT NULL DEFAULT '',
	text_rich TEXT NOT NULL DEFAULT '',
	flavour TEXT,
	set_id TEXT NOT NULL,
	set_label TEXT,
	image_url TEXT,
	artist TEXT,
	accessibility_text TEXT,
	tags TEXT NOT NULL DEFAULT '[]',
	clean_name TEXT,
	source_updated_at TEXT,
	alternate_art INTEGER NOT NULL DEFAULT 0,
	overnumbered INTEGER NOT NULL DEFAULT 0,
	signature INTEGER NOT NULL DEFAULT 0,
	new INTEGER NOT NULL DEFAULT 0,
	api_json TEXT NOT NULL,
	variant TEXT NOT NULL DEFAULT '',
	faction TEXT NOT NULL DEFAULT '',
	image_path TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cards_set_num ON cards(set_id, collector_number, variant);
CREATE INDEX IF NOT EXISTS idx_cards_name ON cards(name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_cards_clean_name ON cards(clean_name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_cards_type ON cards(type);
CREATE INDEX IF NOT EXISTS idx_cards_rarity ON cards(rarity);
CREATE INDEX IF NOT EXISTS idx_cards_supertype ON cards(supertype);

CREATE TABLE IF NOT EXISTS meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS cards_fts USING fts5(
	card_id UNINDEXED,
	id,
	riftcodex_id,
	name,
	clean_name,
	set_id,
	set_label,
	rarity,
	faction,
	type,
	supertype,
	orientation,
	text_plain,
	text_rich,
	accessibility_text,
	flavour,
	tags,
	tcgplayer_id,
	artist,
	meta_flags,
	tokenize='trigram'
);
`

const cardColumns = `
	id, riftcodex_id, name, tcgplayer_id, collector_number, orientation,
	energy, might, power, type, supertype, rarity, domains, text_plain,
	text_rich, flavour, set_id, set_label, image_url, artist,
	accessibility_text, tags, clean_name, source_updated_at, alternate_art,
	overnumbered, signature, new, api_json, variant, faction, image_path, updated_at
`

type DB struct {
	sql *sql.DB
}

type SearchResult struct {
	Card cards.Card
	Rank float64
}

type ImageRecord struct {
	ID        string
	ImageURL  string
	ImagePath string
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "cards.db"
	}
	return filepath.Join(home, ".rift-ctl", "cards.db")
}

func ImageCacheDir(dbPath string) string {
	if dbPath == "" {
		dbPath = DefaultPath()
	}
	return filepath.Join(filepath.Dir(dbPath), "images")
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	sqlDB, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// SQLite allows one writer; keep a single connection so concurrent
	// image workers queue instead of racing into SQLITE_BUSY.
	sqlDB.SetMaxOpenConns(1)
	db := &DB{sql: sqlDB}
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) migrate() error {
	_, err := db.sql.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func (db *DB) UpsertCard(ctx context.Context, c cards.Card) error {
	domains, err := json.Marshal(c.Domains)
	if err != nil {
		return err
	}
	tags, err := json.Marshal(c.Tags)
	if err != nil {
		return err
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = time.Now().UTC()
	}

	_, err = db.sql.ExecContext(ctx, `
		INSERT INTO cards (
			id, riftcodex_id, name, tcgplayer_id, collector_number, orientation,
			energy, might, power, type, supertype, rarity, domains, text_plain,
			text_rich, flavour, set_id, set_label, image_url, artist,
			accessibility_text, tags, clean_name, source_updated_at, alternate_art,
			overnumbered, signature, new, api_json, variant, faction, image_path, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			riftcodex_id=excluded.riftcodex_id,
			name=excluded.name,
			tcgplayer_id=excluded.tcgplayer_id,
			collector_number=excluded.collector_number,
			orientation=excluded.orientation,
			energy=excluded.energy,
			might=excluded.might,
			power=excluded.power,
			type=excluded.type,
			supertype=excluded.supertype,
			rarity=excluded.rarity,
			domains=excluded.domains,
			text_plain=excluded.text_plain,
			text_rich=excluded.text_rich,
			flavour=excluded.flavour,
			set_id=excluded.set_id,
			set_label=excluded.set_label,
			image_url=excluded.image_url,
			artist=excluded.artist,
			accessibility_text=excluded.accessibility_text,
			tags=excluded.tags,
			clean_name=excluded.clean_name,
			source_updated_at=excluded.source_updated_at,
			alternate_art=excluded.alternate_art,
			overnumbered=excluded.overnumbered,
			signature=excluded.signature,
			new=excluded.new,
			api_json=excluded.api_json,
			variant=excluded.variant,
			faction=excluded.faction,
			image_path=CASE WHEN cards.image_url = excluded.image_url THEN cards.image_path ELSE '' END,
			updated_at=excluded.updated_at
	`, c.ID, c.RiftcodexID, c.Name, nullString(c.TCGPlayerID), c.CollectorNumber, nullString(c.Orientation),
		intPtr(c.Energy), intPtr(c.Might), intPtr(c.Power), nullString(c.Type), nullString(c.Supertype),
		nullString(c.Rarity), string(domains), c.TextPlain, c.TextRich, nullString(c.Flavour), c.SetID,
		nullString(c.SetLabel), nullString(c.ImageURL), nullString(c.Artist), nullString(c.AccessibilityText),
		string(tags), nullString(c.CleanName), nullString(c.SourceUpdatedAt), boolInt(c.AlternateArt),
		boolInt(c.Overnumbered), boolInt(c.Signature), boolInt(c.New), c.APIJSON, c.Variant, c.Faction, c.ImagePath,
		c.UpdatedAt.Format(time.RFC3339))
	return err
}

func (db *DB) RebuildSearchIndex(ctx context.Context) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM cards_fts`); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+cardColumns+` FROM cards`)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO cards_fts (
			card_id, id, riftcodex_id, name, clean_name, set_id, set_label, rarity,
			faction, type, supertype, orientation, text_plain, text_rich,
			accessibility_text, flavour, tags, tcgplayer_id, artist, meta_flags
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return err
		}
		if _, err := stmt.ExecContext(ctx,
			c.ID, c.ID, c.RiftcodexID, c.Name, c.CleanName, c.SetID, c.SetLabel, c.Rarity,
			c.Faction, c.Type, c.Supertype, c.Orientation, c.TextPlain, c.TextRich,
			c.AccessibilityText, c.Flavour, strings.Join(c.Tags, ", "), c.TCGPlayerID,
			c.Artist, metaFlags(c),
		); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) Count(ctx context.Context) (int, error) {
	var n int
	err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM cards`).Scan(&n)
	return n, err
}

func (db *DB) GetByID(ctx context.Context, id string) (cards.Card, error) {
	return scanCard(db.sql.QueryRowContext(ctx, `SELECT `+cardColumns+` FROM cards WHERE lower(id) = lower(?)`, id))
}

func (db *DB) GetByRef(ctx context.Context, ref cards.Ref) (cards.Card, error) {
	if ref.IsRune {
		prefix := fmt.Sprintf("%s-r%02d", strings.ToLower(ref.SetID), ref.CollectorNumber)
		return scanCard(db.sql.QueryRowContext(ctx, `SELECT `+cardColumns+` FROM cards WHERE lower(id) = lower(?) LIMIT 1`, prefix))
	}
	return scanCard(db.sql.QueryRowContext(ctx, `SELECT `+cardColumns+` FROM cards WHERE upper(set_id) = upper(?) AND collector_number = ? AND variant = ? LIMIT 1`, ref.SetID, ref.CollectorNumber, ref.Variant))
}

func (db *DB) ListImageRecords(ctx context.Context) ([]ImageRecord, error) {
	rows, err := db.sql.QueryContext(ctx, `SELECT id, COALESCE(image_url, ''), COALESCE(image_path, '') FROM cards WHERE COALESCE(image_url, '') <> '' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ImageRecord
	for rows.Next() {
		var rec ImageRecord
		if err := rows.Scan(&rec.ID, &rec.ImageURL, &rec.ImagePath); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (db *DB) SetImagePath(ctx context.Context, id, imagePath string) error {
	_, err := db.sql.ExecContext(ctx, `UPDATE cards SET image_path = ? WHERE id = ?`, imagePath, id)
	return err
}

func (db *DB) SetMeta(ctx context.Context, key, value string) error {
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO meta(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, key, value)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCard(s scanner) (cards.Card, error) {
	var c cards.Card
	var tcg, orientation, supertype, rarity, flavour, setLabel, imageURL, artist, access, cleanName, sourceUpdated, imagePath sql.NullString
	var energy, might, power sql.NullInt64
	var domainsJSON, tagsJSON string
	var alternateArt, overnumbered, signature, newCard int
	var updatedAt string

	err := s.Scan(
		&c.ID, &c.RiftcodexID, &c.Name, &tcg, &c.CollectorNumber, &orientation,
		&energy, &might, &power, &c.Type, &supertype, &rarity, &domainsJSON, &c.TextPlain,
		&c.TextRich, &flavour, &c.SetID, &setLabel, &imageURL, &artist,
		&access, &tagsJSON, &cleanName, &sourceUpdated, &alternateArt,
		&overnumbered, &signature, &newCard, &c.APIJSON, &c.Variant, &c.Faction, &imagePath, &updatedAt,
	)
	if err != nil {
		return c, err
	}

	c.TCGPlayerID = fromNull(tcg)
	c.Orientation = fromNull(orientation)
	c.Supertype = fromNull(supertype)
	c.Rarity = fromNull(rarity)
	c.Flavour = fromNull(flavour)
	c.SetLabel = fromNull(setLabel)
	c.ImageURL = fromNull(imageURL)
	c.Artist = fromNull(artist)
	c.AccessibilityText = fromNull(access)
	c.CleanName = fromNull(cleanName)
	c.SourceUpdatedAt = fromNull(sourceUpdated)
	c.ImagePath = fromNull(imagePath)
	c.Energy = fromNullInt(energy)
	c.Might = fromNullInt(might)
	c.Power = fromNullInt(power)
	c.AlternateArt = alternateArt != 0
	c.Overnumbered = overnumbered != 0
	c.Signature = signature != 0
	c.New = newCard != 0
	_ = json.Unmarshal([]byte(domainsJSON), &c.Domains)
	_ = json.Unmarshal([]byte(tagsJSON), &c.Tags)
	if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		c.UpdatedAt = t
	}
	return c, nil
}

func scanSearchResult(s scanner) (cards.Card, float64, error) {
	c, rank, err := scanCardWithRank(s)
	if err != nil {
		return c, 0, err
	}
	return c, rank, nil
}

func scanCardWithRank(s scanner) (cards.Card, float64, error) {
	var c cards.Card
	var tcg, orientation, supertype, rarity, flavour, setLabel, imageURL, artist, access, cleanName, sourceUpdated, imagePath sql.NullString
	var energy, might, power sql.NullInt64
	var domainsJSON, tagsJSON string
	var alternateArt, overnumbered, signature, newCard int
	var updatedAt string
	var rank float64

	err := s.Scan(
		&c.ID, &c.RiftcodexID, &c.Name, &tcg, &c.CollectorNumber, &orientation,
		&energy, &might, &power, &c.Type, &supertype, &rarity, &domainsJSON, &c.TextPlain,
		&c.TextRich, &flavour, &c.SetID, &setLabel, &imageURL, &artist,
		&access, &tagsJSON, &cleanName, &sourceUpdated, &alternateArt,
		&overnumbered, &signature, &newCard, &c.APIJSON, &c.Variant, &c.Faction, &imagePath, &updatedAt,
		&rank,
	)
	if err != nil {
		return c, 0, err
	}
	c.TCGPlayerID = fromNull(tcg)
	c.Orientation = fromNull(orientation)
	c.Supertype = fromNull(supertype)
	c.Rarity = fromNull(rarity)
	c.Flavour = fromNull(flavour)
	c.SetLabel = fromNull(setLabel)
	c.ImageURL = fromNull(imageURL)
	c.Artist = fromNull(artist)
	c.AccessibilityText = fromNull(access)
	c.CleanName = fromNull(cleanName)
	c.SourceUpdatedAt = fromNull(sourceUpdated)
	c.ImagePath = fromNull(imagePath)
	c.Energy = fromNullInt(energy)
	c.Might = fromNullInt(might)
	c.Power = fromNullInt(power)
	c.AlternateArt = alternateArt != 0
	c.Overnumbered = overnumbered != 0
	c.Signature = signature != 0
	c.New = newCard != 0
	_ = json.Unmarshal([]byte(domainsJSON), &c.Domains)
	_ = json.Unmarshal([]byte(tagsJSON), &c.Tags)
	if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		c.UpdatedAt = t
	}
	return c, rank, nil
}

func prefixedCardColumns(prefix string) string {
	cols := strings.Split(strings.ReplaceAll(cardColumns, "\n", " "), ",")
	for i, col := range cols {
		cols[i] = prefix + "." + strings.TrimSpace(col)
	}
	return strings.Join(cols, ", ")
}

func ftsQuery(q string) string {
	q = strings.TrimSpace(q)
	q = strings.ReplaceAll(q, `"`, `""`)
	return `"` + q + `"`
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func intPtr(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func fromNull(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func fromNullInt(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func metaFlags(c cards.Card) string {
	var flags []string
	if c.AlternateArt {
		flags = append(flags, "alternate_art")
	}
	if c.Overnumbered {
		flags = append(flags, "overnumbered")
	}
	if c.Signature {
		flags = append(flags, "signature")
	}
	if c.New {
		flags = append(flags, "new")
	}
	return strings.Join(flags, " ")
}
