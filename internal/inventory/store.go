package inventory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Item struct {
	InstanceID uint64 `json:"instanceId"`
	ItemID     uint32 `json:"itemId"`
	Name       string `json:"name"`
	Amount     int    `json:"amount"`
	PocketType uint32 `json:"pocketType"`
}

type Snapshot struct {
	HolderID, HolderType, HolderName string
	Container, SubLocation           string
	ScannedByID, ScannedByName       string
	CapturedAt                       int64
	Items                            []Item
}

type Row struct {
	ItemID             uint32 `json:"itemId"`
	Name               string `json:"name"`
	EnchantName        string `json:"enchantName,omitempty"`
	Amount             int    `json:"amount"`
	PocketType         uint32 `json:"pocketType"`
	HolderID           string `json:"holderId"`
	HolderType         string `json:"holderType"`
	HolderName         string `json:"holderName"`
	OwnerCharacterID   string `json:"ownerCharacterId,omitempty"`
	OwnerCharacterName string `json:"ownerCharacterName,omitempty"`
	ScannedByID        string `json:"scannedById,omitempty"`
	ScannedByName      string `json:"scannedByName,omitempty"`
	Container          string `json:"container"`
	SubLocation        string `json:"subLocation,omitempty"`
	UpdatedAt          int64  `json:"updatedAt"`
}

type Holder struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Name               string `json:"name"`
	OwnerCharacterID   string `json:"ownerCharacterId,omitempty"`
	OwnerCharacterName string `json:"ownerCharacterName,omitempty"`
	UpdatedAt          int64  `json:"updatedAt"`
}

type State struct {
	DatabasePath string   `json:"databasePath"`
	Rows         []Row    `json:"rows"`
	Holders      []Holder `json:"holders"`
}

type Store struct {
	db   *sql.DB
	path string
}

func DefaultPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("BLONY_INVENTORY_DB")); override != "" {
		return filepath.Clean(override), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(dir, "BlonyMonitorV2", "inventory.db"), nil
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create inventory data directory: %w", err)
	}
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	store := &Store{db: database, path: path}
	if err := store.init(); err != nil {
		database.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) init() error {
	_, err := s.db.Exec(`
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS holders (
 holder_id TEXT PRIMARY KEY, holder_type TEXT NOT NULL, name TEXT NOT NULL,
 owner_character_id TEXT NOT NULL DEFAULT '', owner_character_name TEXT NOT NULL DEFAULT '', updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS snapshots (
 id INTEGER PRIMARY KEY AUTOINCREMENT, holder_id TEXT NOT NULL, container TEXT NOT NULL,
 sub_location TEXT NOT NULL DEFAULT '', scanned_by_id TEXT NOT NULL DEFAULT '',
 scanned_by_name TEXT NOT NULL DEFAULT '', updated_at INTEGER NOT NULL,
 UNIQUE(holder_id, container, sub_location), FOREIGN KEY(holder_id) REFERENCES holders(holder_id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS items (
 snapshot_id INTEGER NOT NULL, instance_id TEXT NOT NULL, item_id INTEGER NOT NULL,
 name TEXT NOT NULL, amount INTEGER NOT NULL, pocket_type INTEGER NOT NULL,
 FOREIGN KEY(snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_inventory_items_item_id ON items(item_id);
CREATE INDEX IF NOT EXISTS idx_inventory_items_name ON items(name);
`)
	if err != nil {
		return err
	}
	if err := ensureColumn(s.db, "snapshots", "scanned_by_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return ensureColumn(s.db, "snapshots", "scanned_by_name", "TEXT NOT NULL DEFAULT ''")
}

func ensureColumn(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
	return err
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) ReplaceSnapshots(snapshots []Snapshot) error {
	if s == nil || s.db == nil || len(snapshots) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, snapshot := range snapshots {
		if err := replaceSnapshot(tx, snapshot); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func replaceSnapshot(tx *sql.Tx, snapshot Snapshot) error {
	if snapshot.HolderID == "" || snapshot.Container == "" {
		return fmt.Errorf("inventory snapshot requires holder and container")
	}
	if snapshot.CapturedAt <= 0 {
		snapshot.CapturedAt = time.Now().UnixMilli()
	}
	_, err := tx.Exec(`INSERT INTO holders(holder_id, holder_type, name, updated_at)
VALUES(?, ?, ?, ?)
ON CONFLICT(holder_id) DO UPDATE SET holder_type=excluded.holder_type,
name=CASE WHEN excluded.name <> '' THEN excluded.name ELSE holders.name END,
updated_at=excluded.updated_at`, snapshot.HolderID, snapshot.HolderType, snapshot.HolderName, snapshot.CapturedAt)
	if err != nil {
		return fmt.Errorf("upsert inventory holder: %w", err)
	}
	_, err = tx.Exec(`INSERT INTO snapshots(holder_id, container, sub_location, scanned_by_id, scanned_by_name, updated_at)
VALUES(?, ?, ?, ?, ?, ?)
ON CONFLICT(holder_id, container, sub_location) DO UPDATE SET
scanned_by_id=excluded.scanned_by_id, scanned_by_name=excluded.scanned_by_name, updated_at=excluded.updated_at`,
		snapshot.HolderID, snapshot.Container, snapshot.SubLocation,
		snapshot.ScannedByID, snapshot.ScannedByName, snapshot.CapturedAt)
	if err != nil {
		return fmt.Errorf("upsert inventory snapshot: %w", err)
	}
	var snapshotID int64
	if err := tx.QueryRow(`SELECT id FROM snapshots WHERE holder_id=? AND container=? AND sub_location=?`,
		snapshot.HolderID, snapshot.Container, snapshot.SubLocation).Scan(&snapshotID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM items WHERE snapshot_id=?`, snapshotID); err != nil {
		return err
	}
	statement, err := tx.Prepare(`INSERT INTO items(snapshot_id, instance_id, item_id, name, amount, pocket_type) VALUES(?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, item := range snapshot.Items {
		if item.ItemID == 0 {
			continue
		}
		if item.Amount <= 0 {
			item.Amount = 1
		}
		if _, err := statement.Exec(snapshotID, fmt.Sprint(item.InstanceID), item.ItemID, item.Name, item.Amount, item.PocketType); err != nil {
			return fmt.Errorf("insert inventory item: %w", err)
		}
	}
	return nil
}

func (s *Store) UpdatePetOwner(petID, petName, ownerID, ownerName string, updatedAt int64) error {
	if s == nil || s.db == nil || petID == "" || ownerID == "" {
		return nil
	}
	if updatedAt <= 0 {
		updatedAt = time.Now().UnixMilli()
	}
	_, err := s.db.Exec(`INSERT INTO holders(holder_id, holder_type, name, owner_character_id, owner_character_name, updated_at)
VALUES(?, 'pet', ?, ?, ?, ?)
ON CONFLICT(holder_id) DO UPDATE SET holder_type='pet',
name=CASE WHEN excluded.name <> '' THEN excluded.name ELSE holders.name END,
owner_character_id=excluded.owner_character_id,
owner_character_name=CASE WHEN excluded.owner_character_name <> '' THEN excluded.owner_character_name ELSE holders.owner_character_name END,
updated_at=MAX(holders.updated_at, excluded.updated_at)`, petID, petName, ownerID, ownerName, updatedAt)
	return err
}

func (s *Store) State() (State, error) {
	state := State{DatabasePath: s.path, Rows: []Row{}, Holders: []Holder{}}
	holderRows, err := s.db.Query(`SELECT holder_id, holder_type, name, owner_character_id, owner_character_name, updated_at FROM holders ORDER BY holder_type, name`)
	if err != nil {
		return state, err
	}
	for holderRows.Next() {
		var holder Holder
		if err := holderRows.Scan(&holder.ID, &holder.Type, &holder.Name, &holder.OwnerCharacterID, &holder.OwnerCharacterName, &holder.UpdatedAt); err != nil {
			holderRows.Close()
			return state, err
		}
		state.Holders = append(state.Holders, holder)
	}
	holderRows.Close()

	rows, err := s.db.Query(`SELECT i.item_id, i.name, SUM(i.amount), i.pocket_type, h.holder_id, h.holder_type, h.name,
h.owner_character_id, h.owner_character_name, sn.scanned_by_id, sn.scanned_by_name,
sn.container, sn.sub_location, sn.updated_at
FROM items i JOIN snapshots sn ON sn.id=i.snapshot_id JOIN holders h ON h.holder_id=sn.holder_id
GROUP BY i.item_id, i.name, i.pocket_type, h.holder_id, h.holder_type, h.name, h.owner_character_id,
h.owner_character_name, sn.scanned_by_id, sn.scanned_by_name, sn.container, sn.sub_location, sn.updated_at`)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.ItemID, &row.Name, &row.Amount, &row.PocketType, &row.HolderID, &row.HolderType, &row.HolderName,
			&row.OwnerCharacterID, &row.OwnerCharacterName, &row.ScannedByID, &row.ScannedByName,
			&row.Container, &row.SubLocation, &row.UpdatedAt); err != nil {
			return state, err
		}
		state.Rows = append(state.Rows, row)
	}
	sort.Slice(state.Rows, func(i, j int) bool {
		if state.Rows[i].Name != state.Rows[j].Name {
			return state.Rows[i].Name < state.Rows[j].Name
		}
		if state.Rows[i].HolderName != state.Rows[j].HolderName {
			return state.Rows[i].HolderName < state.Rows[j].HolderName
		}
		return state.Rows[i].Container < state.Rows[j].Container
	})
	return state, rows.Err()
}
