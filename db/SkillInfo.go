package db

import "strconv"

type SkillInfo struct {
	SkillID        int    `db:"SkillID"`
	SkillEngName   string `db:"SkillEngName"`
	SkillLocalName string `db:"SkillLocalName"`
	SkillDesc      string `db:"SkillDesc"`
	ImageData      string `db:"ImageData"`
}

func NewSkillInfo(skill_id int) *SkillInfo {
	var skill_info SkillInfo
	_ = DB.Get(&skill_info, "SELECT * FROM SkillInfo WHERE SkillID = ?", skill_id)
	return &skill_info
}

func (s *SkillInfo) GetName() string {
	if s.SkillLocalName != "" {
		return s.SkillLocalName
	}
	if s.SkillEngName != "" {
		return s.SkillEngName
	}
	return strconv.Itoa(s.SkillID)
}

func GetAllSkills() map[int]string {
	rows, err := DB.Queryx("SELECT * FROM SkillInfo")
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := make(map[int]string)
	for rows.Next() {
		var skill_info SkillInfo
		if err := rows.StructScan(&skill_info); err != nil {
			continue
		}
		result[skill_info.SkillID] = skill_info.GetName()
	}
	return result
}

func GetAllSkillIcons() map[int]string {
	rows, err := DB.Queryx("SELECT * FROM SkillInfo")
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := make(map[int]string)
	for rows.Next() {
		var skill_info SkillInfo
		if err := rows.StructScan(&skill_info); err != nil {
			continue
		}
		result[skill_info.SkillID] = skill_info.ImageData
	}
	return result
}
