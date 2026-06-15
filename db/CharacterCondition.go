package db

import "strconv"

type CharacterCondition struct {
	ConditionID        int    `db:"ConditionID"`
	ConditionEngName   string `db:"ConditionEngName"`
	ConditionLocalName string `db:"ConditionLocalName"`
}

func NewCharacterCondition(condition_id int) *CharacterCondition {
	var cond CharacterCondition
	_ = DB.Get(&cond, "SELECT * FROM CharacterCondition WHERE ConditionID = ?", condition_id)
	return &cond
}

func (s *CharacterCondition) GetName() string {
	if s.ConditionLocalName != "" {
		return s.ConditionLocalName
	}
	if s.ConditionEngName != "" {
		return s.ConditionEngName
	}
	return strconv.Itoa(s.ConditionID)
}

// GetIcon 通过状态名称匹配 SkillInfo 获取图标（base64）
func GetConditionIcon(conditionId int) string {
	cond := NewCharacterCondition(conditionId)
	if cond.ConditionLocalName == "" {
		return ""
	}

	var imageData string
	err := DB.Get(&imageData, "SELECT ImageData FROM SkillInfo WHERE SkillLocalName = ? LIMIT 1", cond.ConditionLocalName)
	if err != nil {
		return ""
	}
	return imageData
}

// GetAllConditionIcons 获取所有状态图标映射（通过名称匹配技能图标）
func GetAllConditionIcons() map[int]string {
	rows, err := DB.Queryx("SELECT ConditionID, ConditionLocalName FROM CharacterCondition")
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := make(map[int]string)
	for rows.Next() {
		var cond CharacterCondition
		if err := rows.StructScan(&cond); err != nil || cond.ConditionLocalName == "" {
			continue
		}
		var imageData string
		if err := DB.Get(&imageData, "SELECT ImageData FROM SkillInfo WHERE SkillLocalName = ? LIMIT 1", cond.ConditionLocalName); err != nil {
			continue
		}
		if imageData != "" {
			result[cond.ConditionID] = imageData
		}
	}
	return result
}

func GetAllCondition() map[int]string {
	rows, err := DB.Queryx("SELECT * FROM CharacterCondition")
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := make(map[int]string)
	for rows.Next() {
		var cond_info CharacterCondition
		if err := rows.StructScan(&cond_info); err != nil {
			continue
		}
		result[cond_info.ConditionID] = cond_info.GetName()
	}
	return result
}
