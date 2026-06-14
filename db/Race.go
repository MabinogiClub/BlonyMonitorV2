package db

import "strconv"

type Race struct {
	ID          int    `db:"ID"`
	ClassName   string `db:"ClassName"`
	StringID    string `db:"StringID"`
	EnglishName string `db:"EnglishName"`
	LocalName   string `db:"LocalName"`
}

func NewRace(id int) *Race {
	var race Race
	_ = DB.Get(&race, "SELECT * FROM Race WHERE ID = ?", id)
	return &race
}

func (s *Race) GetName() string {
	if s.LocalName != "" {
		return s.LocalName
	}
	if s.EnglishName != "" {
		return s.EnglishName
	}
	if s.ClassName != "" {
		return s.ClassName
	}
	return strconv.Itoa(s.ID)
}

func GetAllRace() map[int]string {
	rows, err := DB.Queryx("SELECT * FROM Race")
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := make(map[int]string)
	for rows.Next() {
		var race_info Race
		if err := rows.StructScan(&race_info); err != nil {
			continue
		}
		result[race_info.ID] = race_info.GetName()
	}
	return result
}
