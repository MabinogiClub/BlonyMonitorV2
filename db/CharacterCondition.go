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
