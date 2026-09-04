package farmstats

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Usage struct {
	AccountID   string           `json:"accountId"`
	CoinsSpent  int64            `json:"coinsSpent"`
	EnergySpent int64            `json:"energySpent"`
	CropCounts  map[string]int64 `json:"cropCounts"`
	UpdatedAt   int64            `json:"updatedAt"`
}

type DailyUsage struct {
	Date        string
	CoinsSpent  int64
	EnergySpent int64
	CropCounts  map[string]int64
}

type CurrencyUsage struct {
	CharacterID  string `json:"characterId"`
	Currency     string `json:"currency"`
	Added        int64  `json:"added"`
	Spent        int64  `json:"spent"`
	Balance      int64  `json:"balance"`
	BalanceKnown bool   `json:"balanceKnown"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type DailyCurrencyUsage struct {
	Date    string
	Added   int64
	Spent   int64
	Balance int64
}

type Store struct {
	db   *sql.DB
	path string
}

func DefaultPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("BLONY_FARM_STATS_DB")); override != "" {
		return filepath.Clean(override), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(dir, "BlonyMonitorV2", "farm_stats.db"), nil
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
		return nil, fmt.Errorf("create farm statistics directory: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db, path: path}
	if _, err = db.Exec(`
PRAGMA journal_mode = WAL;
CREATE TABLE IF NOT EXISTS farm_usage_daily (
 account_id TEXT NOT NULL,
 usage_date TEXT NOT NULL,
 action_type TEXT NOT NULL,
 item_key TEXT NOT NULL DEFAULT '',
 action_count INTEGER NOT NULL DEFAULT 0,
 coin_cost INTEGER NOT NULL DEFAULT 0,
 energy_spent INTEGER NOT NULL DEFAULT 0,
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(account_id, usage_date, action_type, item_key)
);
CREATE INDEX IF NOT EXISTS idx_farm_usage_account_date ON farm_usage_daily(account_id, usage_date);
CREATE TABLE IF NOT EXISTS currency_usage_daily (
 character_id TEXT NOT NULL,
 usage_date TEXT NOT NULL,
 currency TEXT NOT NULL,
 added INTEGER NOT NULL DEFAULT 0,
 spent INTEGER NOT NULL DEFAULT 0,
 balance INTEGER NOT NULL DEFAULT 0,
 balance_known INTEGER NOT NULL DEFAULT 0,
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(character_id, usage_date, currency)
);
CREATE TABLE IF NOT EXISTS currency_balance (
 character_id TEXT NOT NULL,
 currency TEXT NOT NULL,
 balance INTEGER NOT NULL,
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(character_id, currency)
);
`); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) RecordCurrencyDelta(characterID, currency string, delta int64, at time.Time) error {
	if s == nil || s.db == nil || !isCharacterKey(characterID) || strings.TrimSpace(currency) == "" || delta == 0 {
		return nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	added, spent := int64(0), int64(0)
	if delta > 0 {
		added = delta
	} else {
		spent = -delta
	}
	_, err := s.db.Exec(`INSERT INTO currency_usage_daily(character_id, usage_date, currency, added, spent, updated_at)
VALUES(?, ?, ?, ?, ?, ?)
ON CONFLICT(character_id, usage_date, currency) DO UPDATE SET
added=added+excluded.added, spent=spent+excluded.spent, updated_at=excluded.updated_at`,
		characterID, usageDate(at), currency, added, spent, at.UnixMilli())
	return err
}

// SyncCurrencyBalance stores an authoritative balance. After the first
// baseline, differences are recorded as inferred additions or expenditures.
func (s *Store) SyncCurrencyBalance(characterID, currency string, balance int64, at time.Time) error {
	if s == nil || s.db == nil || !isCharacterKey(characterID) || strings.TrimSpace(currency) == "" || balance < 0 {
		return nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	var previous int64
	err := s.db.QueryRow(`SELECT balance FROM currency_balance WHERE character_id=? AND currency=?`, characterID, currency).Scan(&previous)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	hasBaseline := err == nil
	if !hasBaseline {
		// A stale daily row without a balance baseline cannot be classified as
		// a real change. Remove it so the first authoritative sync is baseline
		// only, even after an interrupted/old-version write.
		if _, err := s.db.Exec(`DELETE FROM currency_usage_daily WHERE character_id=? AND currency=?`, characterID, currency); err != nil {
			return err
		}
	}
	if hasBaseline && previous != balance {
		if err := s.RecordCurrencyDelta(characterID, currency, balance-previous, at); err != nil {
			return err
		}
	}
	_, err = s.db.Exec(`INSERT INTO currency_balance(character_id, currency, balance, updated_at) VALUES(?, ?, ?, ?)
ON CONFLICT(character_id, currency) DO UPDATE SET balance=excluded.balance, updated_at=excluded.updated_at`,
		characterID, currency, balance, at.UnixMilli())
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO currency_usage_daily(character_id, usage_date, currency, balance, balance_known, updated_at)
VALUES(?, ?, ?, ?, 1, ?)
ON CONFLICT(character_id, usage_date, currency) DO UPDATE SET balance=excluded.balance, balance_known=1, updated_at=excluded.updated_at`,
		characterID, usageDate(at), currency, balance, at.UnixMilli())
	return err
}

func (s *Store) CurrencyUsage(characterID, currency string) (CurrencyUsage, error) {
	u := CurrencyUsage{CharacterID: characterID, Currency: currency}
	if s == nil || s.db == nil || !isCharacterKey(characterID) || currency == "" {
		return u, nil
	}
	err := s.db.QueryRow(`SELECT balance FROM currency_balance WHERE character_id=? AND currency=?`, characterID, currency).Scan(&u.Balance)
	u.BalanceKnown = err == nil
	row := s.db.QueryRow(`SELECT COALESCE(SUM(added),0), COALESCE(SUM(spent),0), COALESCE(MAX(updated_at),0)
FROM currency_usage_daily WHERE character_id=? AND currency=?`, characterID, currency)
	if err := row.Scan(&u.Added, &u.Spent, &u.UpdatedAt); err != nil {
		return u, err
	}
	return u, nil
}

func (s *Store) CurrencyUsageHistory(characterID, currency string, at time.Time, days int) ([]DailyCurrencyUsage, error) {
	if days <= 0 {
		days = 30
	}
	if days > 30 {
		days = 30
	}
	currentDate := usageDate(at)
	location := time.FixedZone("CST", 8*60*60)
	current, err := time.ParseInLocation("2006-01-02", currentDate, location)
	if err != nil {
		return nil, err
	}
	start := current.AddDate(0, 0, -(days - 1))
	result := make([]DailyCurrencyUsage, days)
	indexes := make(map[string]int, days)
	for i := range result {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		result[i].Date = date
		indexes[date] = i
	}
	if s == nil || s.db == nil || !isCharacterKey(characterID) || currency == "" {
		return result, nil
	}
	rows, err := s.db.Query(`SELECT usage_date, added, spent, balance FROM currency_usage_daily
WHERE character_id=? AND currency=? AND usage_date>=? AND usage_date<=?`, characterID, currency, start.Format("2006-01-02"), currentDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var date string
		var index int
		var row DailyCurrencyUsage
		if err := rows.Scan(&date, &row.Added, &row.Spent, &row.Balance); err != nil {
			return nil, err
		}
		index, _ = indexes[date]
		result[index].Added += row.Added
		result[index].Spent += row.Spent
		result[index].Balance = row.Balance
	}
	return result, rows.Err()
}

func isCharacterKey(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "character:") && len(strings.TrimPrefix(strings.TrimSpace(value), "character:")) > 0
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func usageDate(at time.Time) string {
	if at.IsZero() {
		at = time.Now()
	}
	// Farm statistics roll over at 07:00 in China Standard Time, matching
	// the game's daily reset, rather than at calendar midnight.
	location := time.FixedZone("CST", 8*60*60)
	local := at.In(location)
	if local.Hour() < 7 {
		local = local.AddDate(0, 0, -1)
	}
	return local.Format("2006-01-02")
}

func (s *Store) RecordPlant(accountID, cropName string, coinCost int64, at time.Time) error {
	if s == nil || s.db == nil || strings.TrimSpace(accountID) == "" || strings.TrimSpace(cropName) == "" || coinCost < 0 {
		return nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	_, err := s.db.Exec(`INSERT INTO farm_usage_daily(
account_id, usage_date, action_type, item_key, action_count, coin_cost, updated_at)
VALUES(?, ?, 'plant', ?, 1, ?, ?)
ON CONFLICT(account_id, usage_date, action_type, item_key) DO UPDATE SET
action_count=action_count+1, coin_cost=coin_cost+excluded.coin_cost, updated_at=excluded.updated_at`,
		accountID, usageDate(at), cropName, coinCost, at.UnixMilli())
	return err
}

func (s *Store) RecordEnergy(accountID string, spent int, at time.Time) error {
	if s == nil || s.db == nil || strings.TrimSpace(accountID) == "" || spent <= 0 {
		return nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	_, err := s.db.Exec(`INSERT INTO farm_usage_daily(
account_id, usage_date, action_type, energy_spent, updated_at)
VALUES(?, ?, 'energy', ?, ?)
ON CONFLICT(account_id, usage_date, action_type, item_key) DO UPDATE SET
energy_spent=energy_spent+excluded.energy_spent, updated_at=excluded.updated_at`,
		accountID, usageDate(at), spent, at.UnixMilli())
	return err
}

func (s *Store) Merge(fromID, toID string) error {
	if s == nil || s.db == nil || fromID == "" || toID == "" || fromID == toID {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT INTO farm_usage_daily(
account_id, usage_date, action_type, item_key, action_count, coin_cost, energy_spent, updated_at)
SELECT ?, usage_date, action_type, item_key, action_count, coin_cost, energy_spent, updated_at
FROM farm_usage_daily WHERE account_id=?
ON CONFLICT(account_id, usage_date, action_type, item_key) DO UPDATE SET
action_count=action_count+excluded.action_count,
coin_cost=coin_cost+excluded.coin_cost,
energy_spent=energy_spent+excluded.energy_spent,
updated_at=MAX(updated_at, excluded.updated_at)`, toID, fromID)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM farm_usage_daily WHERE account_id=?`, fromID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Usage(accountID string) (Usage, error) {
	u := Usage{AccountID: accountID, CropCounts: map[string]int64{}}
	if s == nil || s.db == nil || accountID == "" {
		return u, nil
	}
	rows, err := s.db.Query(`SELECT action_type, item_key, SUM(action_count), SUM(coin_cost),
SUM(energy_spent), MAX(updated_at) FROM farm_usage_daily
WHERE account_id=? GROUP BY action_type, item_key`, accountID)
	if err != nil {
		return u, err
	}
	defer rows.Close()
	for rows.Next() {
		var typ, key string
		var count, coins, energy, updated int64
		if err := rows.Scan(&typ, &key, &count, &coins, &energy, &updated); err != nil {
			return u, err
		}
		u.CoinsSpent += coins
		u.EnergySpent += energy
		if updated > u.UpdatedAt {
			u.UpdatedAt = updated
		}
		if typ == "plant" {
			u.CropCounts[key] += count
		}
	}
	return u, rows.Err()
}

// UsageHistory returns one row for each statistics day in the requested range,
// including days with no observed actions. The range ends at the current farm
// day and uses the same 07:00 CST rollover as usageDate.
func (s *Store) UsageHistory(accountID string, at time.Time, days int) ([]DailyUsage, error) {
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}
	currentDate := usageDate(at)
	location := time.FixedZone("CST", 8*60*60)
	current, err := time.ParseInLocation("2006-01-02", currentDate, location)
	if err != nil {
		return nil, err
	}
	start := current.AddDate(0, 0, -(days - 1))
	result := make([]DailyUsage, days)
	byDate := make(map[string]int)
	for index := range result {
		date := start.AddDate(0, 0, index).Format("2006-01-02")
		result[index] = DailyUsage{Date: date, CropCounts: map[string]int64{}}
		byDate[date] = index
	}
	if s == nil || s.db == nil || strings.TrimSpace(accountID) == "" {
		return result, nil
	}
	rows, err := s.db.Query(`SELECT usage_date, action_type, item_key,
SUM(action_count), SUM(coin_cost), SUM(energy_spent)
FROM farm_usage_daily
WHERE account_id=? AND usage_date>=? AND usage_date<=?
GROUP BY usage_date, action_type, item_key`, accountID, start.Format("2006-01-02"), currentDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var date, actionType, itemKey string
		var count, coins, energy int64
		if err := rows.Scan(&date, &actionType, &itemKey, &count, &coins, &energy); err != nil {
			return nil, err
		}
		index, ok := byDate[date]
		if !ok {
			continue
		}
		result[index].CoinsSpent += coins
		result[index].EnergySpent += energy
		if actionType == "plant" {
			result[index].CropCounts[itemKey] += count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
