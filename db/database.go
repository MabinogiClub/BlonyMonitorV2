package db

import (
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

var DB *sqlx.DB

func getDBPath() string {
	dbName := "mabidata.db"

	exePath, err := os.Executable()
	if err != nil {
		return dbName
	}
	return filepath.Join(filepath.Dir(exePath), dbName)
}

func InitDB() {
	dbPath := getDBPath()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		panic("[DB] ????????: " + dbPath)
	}

	var err error
	DB, err = sqlx.Open("sqlite", "file:"+dbPath)
	if err != nil {
		panic("[DB] ??????????: " + err.Error())
	}

	if err := DB.Ping(); err != nil {
		panic("[DB] ?????????: " + err.Error())
	}

	DB.SetMaxOpenConns(1)
	DB.SetMaxIdleConns(1)

	println("[DB] ??????: " + dbPath)
}
