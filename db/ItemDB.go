package db

import (
	"fmt"
	"strings"
)

// ItemInfo 是客户端物品数据库中用于装备面板的基础信息。
type ItemInfo struct {
	ID          int    `db:"ID"`
	NameEN      string `db:"Text_Name0"`
	NameLocal   string `db:"Text_Name1"`
	Description string `db:"Text_Desc1"`
	Icon        string `db:"File_InvImage"`
}

func GetItemInfo(id int) *ItemInfo {
	if DB == nil || id <= 0 {
		return nil
	}
	var item ItemInfo
	if err := DB.Get(&item, "SELECT ID, Text_Name0, Text_Name1, Text_Desc1, File_InvImage FROM ItemDB WHERE ID = ?", id); err != nil {
		return nil
	}
	return &item
}

// GetItemInfoBatch loads item definitions with bounded IN queries. SQLite has
// a limit on bind parameters, so callers can safely pass large inventories.
func GetItemInfoBatch(ids []uint32) map[uint32]*ItemInfo {
	result := make(map[uint32]*ItemInfo, len(ids))
	if DB == nil || len(ids) == 0 {
		return result
	}
	unique := make([]uint32, 0, len(ids))
	seen := make(map[uint32]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	const batchSize = 500
	for start := 0; start < len(unique); start += batchSize {
		end := start + batchSize
		if end > len(unique) {
			end = len(unique)
		}
		args := make([]any, end-start)
		placeholders := make([]string, end-start)
		for index, id := range unique[start:end] {
			args[index] = id
			placeholders[index] = "?"
		}
		var items []ItemInfo
		query := fmt.Sprintf("SELECT ID, Text_Name0, Text_Name1, Text_Desc1, File_InvImage FROM ItemDB WHERE ID IN (%s)", strings.Join(placeholders, ","))
		if err := DB.Select(&items, query, args...); err != nil {
			continue
		}
		for index := range items {
			result[uint32(items[index].ID)] = &items[index]
		}
	}
	return result
}

func (item *ItemInfo) DisplayName() string {
	if item == nil {
		return ""
	}
	if item.NameLocal != "" {
		return item.NameLocal
	}
	return item.NameEN
}

// GetEnchantName resolves an OptionSet/enchant ID when the generated data
// database contains the optional OptionSet table. Older databases simply
// return an empty string and callers can retain the numeric ID.
func GetEnchantName(id int) string {
	if DB == nil || id <= 0 {
		return ""
	}
	var name string
	if err := DB.Get(&name, "SELECT LocalName FROM OptionSet WHERE ID = ? LIMIT 1", id); err == nil {
		return strings.TrimSpace(name)
	}
	return ""
}
