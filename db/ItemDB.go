package db

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

func (item *ItemInfo) DisplayName() string {
	if item == nil {
		return ""
	}
	if item.NameLocal != "" {
		return item.NameLocal
	}
	return item.NameEN
}
