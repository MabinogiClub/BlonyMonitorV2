package db

type DungeonDB struct {
	Name      string `db:"name"`
	LocalName string `db:"localname"`
}

func NewDungeonDB(name string) *DungeonDB {
	var dungeon_db DungeonDB
	_ = DB.Get(&dungeon_db, "SELECT * FROM DungeonDB WHERE name = ?", name)
	return &dungeon_db
}
