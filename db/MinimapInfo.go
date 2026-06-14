package db

type MinimapInfo_Dungeon_Lobby struct {
	RegionID     int    `db:"RegionID"`
	MapName      string `db:"MapName"`
	MapLocalName string `db:"MapLocalName"`
	MapFile      string `db:"MapFile"`
}

type MinimapInfo_DynamicRegion struct {
	RegionID     int    `db:"RegionID"`
	MapName      string `db:"MapName"`
	MapLocalName string `db:"MapLocalName"`
	MapFile      string `db:"MapFile"`
}

type MinimapInfo_FieldMapInfoList struct {
	RegionID     int    `db:"RegionID"`
	MapName      string `db:"MapName"`
	MapLocalName string `db:"MapLocalName"`
	MapFile      string `db:"MapFile"`
}

func NewMinimapInfo_Dungeon_Lobby(region_id int) *MinimapInfo_Dungeon_Lobby {
	var minimap MinimapInfo_Dungeon_Lobby
	_ = DB.Get(&minimap, "SELECT * FROM MinimapInfo_Dungeon_Lobby WHERE RegionID = ?", region_id)
	return &minimap
}

func NewMinimapInfo_DynamicRegion(region_id int) *MinimapInfo_DynamicRegion {
	var minimap MinimapInfo_DynamicRegion
	_ = DB.Get(&minimap, "SELECT * FROM MinimapInfo_DynamicRegion WHERE RegionID = ?", region_id)
	return &minimap
}

func NewMinimapInfo_FieldMapInfoList(region_id int) *MinimapInfo_FieldMapInfoList {
	var minimap MinimapInfo_FieldMapInfoList
	_ = DB.Get(&minimap, "SELECT * FROM MinimapInfo_FieldMapInfoList WHERE RegionID = ?", region_id)
	return &minimap
}
